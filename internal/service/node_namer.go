package service

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// 直连节点默认模板；中转节点默认模板（落地旗帜 落地名 via 入口名，中转地域挤在 fwd 后）。
const (
	defaultNamingTemplate      = "{flag}{region}_{agent_name}{serial}"
	defaultRelayNamingTemplate = "{exit_flag} {exit_name} via {entry_name}{fwd}"
)

// 窄接口：NodeNamer 只需要这三个只读查询，具体 repo 隐式满足，测试用 stub 即可。
type namingSpecStore interface {
	FindByID(ctx context.Context, id int64) (*repository.InboundSpec, error)
}

type namingPathStore interface {
	GetByID(ctx context.Context, id int64) (*repository.RelayPath, error)
}

type namingHostStore interface {
	FindByID(ctx context.Context, id int64) (*repository.AgentHost, error)
}

// NodeNamer generates auto-names for server nodes based on a configurable
// template (e.g. "{flag}{region}_{agent_name}{serial}") and manages per-agent
// serial counting so that the first inbound on an agent gets no serial suffix
// and subsequent inbounds get -02, -03, etc.
//
// 中继链路入口上报的节点走中转命名：落地（链路最后一跳）旗帜+名称 via 入口名称，
// 中间跳地域小写按序挤在 fwd 后（如 " fwd hk jp"，无中间跳则为空）。
type NodeNamer struct {
	settings AdminSystemSettingsService
	specs    namingSpecStore
	paths    namingPathStore
	hosts    namingHostStore
	mu       sync.Mutex
	serials  map[int64]int // agentHostID -> next serial (1-indexed, 1 = first = no suffix)
}

// NewNodeNamer creates a NodeNamer backed by the given settings service.
func NewNodeNamer(settings AdminSystemSettingsService) *NodeNamer {
	return &NodeNamer{
		settings: settings,
		serials:  make(map[int64]int),
	}
}

// WithRelay 注入中转命名所需的只读查询；不调用则中转解析跳过（行为与原来一致）。
func (n *NodeNamer) WithRelay(specs namingSpecStore, paths namingPathStore, hosts namingHostStore) *NodeNamer {
	n.specs = specs
	n.paths = paths
	n.hosts = hosts
	return n
}

// FlagEmoji converts a 2-letter ISO 3166-1 alpha-2 country code to its
// corresponding regional indicator flag emoji using standard Unicode offset calculation.
// Returns the input code as fallback if it is not a valid 2-letter uppercase ASCII code.
func FlagEmoji(countryCode string) string {
	code := strings.ToUpper(strings.TrimSpace(countryCode))
	if len(code) != 2 {
		return countryCode
	}
	r1, r2 := rune(code[0]), rune(code[1])
	if r1 >= 'A' && r1 <= 'Z' && r2 >= 'A' && r2 <= 'Z' {
		// Regional Indicator Symbol Letter A is U+1F1E6 (127462)
		const base = 0x1F1E6
		return string([]rune{base + (r1 - 'A'), base + (r2 - 'A')})
	}
	return countryCode
}

// IsNamingEnabled reads the node_naming_enabled setting (category "naming").
// Returns true when the setting is absent, empty, or "1".
func (n *NodeNamer) IsNamingEnabled(ctx context.Context) bool {
	v, err := n.settings.Get(ctx, "node_naming_enabled")
	if err != nil || v == "" {
		return true // default enabled
	}
	return v == "1"
}

// namingTemplate reads the node_naming_template setting (category "naming").
func (n *NodeNamer) namingTemplate(ctx context.Context) string {
	v, err := n.settings.Get(ctx, "node_naming_template")
	if err != nil || v == "" {
		return defaultNamingTemplate
	}
	return v
}

// relayTemplate reads the node_naming_relay_template setting (category "naming").
func (n *NodeNamer) relayTemplate(ctx context.Context) string {
	v, err := n.settings.Get(ctx, "node_naming_relay_template")
	if err != nil || v == "" {
		return defaultRelayNamingTemplate
	}
	return v
}

// relayNameInfo 描述中转命名所需信息：落地（链路最后一跳）+ 中间跳地域 + 入口。
type relayNameInfo struct {
	exitFlag   string   // 落地国旗 emoji
	exitRegion string   // 落地地区代码（大写）
	exitName   string   // 落地 agent 名
	entryName  string   // 入口 agent 名
	fwdRegions []string // 中间跳地域小写，按链路顺序
}

// inboundSpecIDFromCode 从 server.Code（agent 上报文件名 inbound-<specID>-<tag>.json）
// 解析 specID；mesh-inbound 等非 spec 文件返回 0。
var inboundSpecIDPattern = regexp.MustCompile(`^inbound-(\d+)-`)

func inboundSpecIDFromCode(code string) int64 {
	m := inboundSpecIDPattern.FindStringSubmatch(strings.TrimSpace(code))
	if len(m) != 2 {
		return 0
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// resolveRelay 解析 server 是否为某条中继链路的入口：Code→spec→relay_path→全跳 agent。
// 任一步缺失返回 nil（调用方回落直连命名）。
func (n *NodeNamer) resolveRelay(ctx context.Context, host *repository.AgentHost, srv *repository.Server) *relayNameInfo {
	if n == nil || n.specs == nil || n.paths == nil || n.hosts == nil || srv == nil {
		return nil
	}
	specID := inboundSpecIDFromCode(srv.Code)
	if specID <= 0 {
		return nil // mesh-inbound 等临时文件无 spec，走直连
	}
	spec, err := n.specs.FindByID(ctx, specID)
	if err != nil || spec == nil || spec.RelayPathID == nil || *spec.RelayPathID <= 0 {
		return nil // spec 不存在或未绑中继链路，走直连
	}
	path, err := n.paths.GetByID(ctx, *spec.RelayPathID)
	if err != nil || path == nil || !path.Enabled || len(path.Nodes) < 2 {
		return nil
	}
	nodes := append([]repository.RelayPathNode(nil), path.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Sequence < nodes[j].Sequence })
	exit := nodes[len(nodes)-1]
	// 入口必须是链路第一跳，否则不是本入口的中转节点（串了别的链路时回落直连）
	if nodes[0].AgentHostID != host.ID {
		return nil
	}
	exitHost, err := n.hosts.FindByID(ctx, exit.AgentHostID)
	if err != nil || exitHost == nil {
		return nil
	}
	info := &relayNameInfo{
		exitFlag:   FlagEmoji(exitHost.Country),
		exitRegion: strings.ToUpper(strings.TrimSpace(exitHost.Country)),
		exitName:   exitHost.Name,
		entryName:  host.Name,
	}
	for _, hop := range nodes[1 : len(nodes)-1] {
		h, err := n.hosts.FindByID(ctx, hop.AgentHostID)
		if err != nil || h == nil {
			continue
		}
		if region := strings.ToLower(strings.TrimSpace(h.Country)); region != "" {
			info.fwdRegions = append(info.fwdRegions, region)
		}
	}
	return info
}

// BuildName computes a server node name from the template and the given
// inputs without mutating the receiver's serial state.
func (n *NodeNamer) BuildName(ctx context.Context, host *repository.AgentHost, srv *repository.Server, serial int) string {
	if !n.IsNamingEnabled(ctx) {
		return srv.Name
	}
	if info := n.resolveRelay(ctx, host, srv); info != nil {
		return n.buildRelayName(ctx, info)
	}
	tpl := n.namingTemplate(ctx)
	if tpl == "" {
		return srv.Name
	}

	result := tpl

	flag := FlagEmoji(host.Country)
	result = strings.ReplaceAll(result, "{flag}", flag)

	region := host.Country
	result = strings.ReplaceAll(result, "{region}", region)

	agentName := host.Name
	result = strings.ReplaceAll(result, "{agent_name}", agentName)

	if serial > 0 {
		result = strings.ReplaceAll(result, "{serial}", fmt.Sprintf("-%02d", serial))
	} else {
		// serial == 0 means first server, no serial suffix
		result = strings.ReplaceAll(result, "{serial}", "")
		// Also clean up any trailing separator that would end up doubled
		result = cleanTrailingSeparator(result)
	}

	srvType := srv.Type
	result = strings.ReplaceAll(result, "{type}", srvType)

	return result
}

// cleanTrailingSeparator removes a trailing dash, underscore, space, or
// hyphen sequence that would remain after an empty {serial} substitution.
func cleanTrailingSeparator(s string) string {
	// Common separators that may trail an empty serial
	for strings.HasSuffix(s, "-") || strings.HasSuffix(s, "_") || strings.HasSuffix(s, " ") {
		s = strings.TrimSuffix(s, "-")
		s = strings.TrimSuffix(s, "_")
		s = strings.TrimSuffix(s, " ")
	}
	return s
}

// buildRelayName 按中转模板展开：{exit_flag} {exit_name} via {entry_name}{fwd}。
// {fwd} 无中间跳时为空，有则为 " fwd hk jp"（前导空格，模板里直接贴在 entry 后）。
func (n *NodeNamer) buildRelayName(ctx context.Context, info *relayNameInfo) string {
	tpl := n.relayTemplate(ctx)
	if tpl == "" {
		tpl = defaultRelayNamingTemplate
	}
	result := tpl
	result = strings.ReplaceAll(result, "{exit_flag}", info.exitFlag)
	result = strings.ReplaceAll(result, "{exit_region}", info.exitRegion)
	result = strings.ReplaceAll(result, "{exit_name}", info.exitName)
	result = strings.ReplaceAll(result, "{entry_name}", info.entryName)
	fwd := ""
	if len(info.fwdRegions) > 0 {
		fwd = " fwd " + strings.Join(info.fwdRegions, " ")
	}
	result = strings.ReplaceAll(result, "{fwd}", fwd)
	return result
}

// ShouldRename returns true only when the server's current name equals the
// original protocol tag, indicating it has not been manually renamed by an
// admin. Manually-named servers are preserved.
func (n *NodeNamer) ShouldRename(srv *repository.Server, originalTag string) bool {
	return srv.Name == originalTag
}

// Apply computes a new name for the server, increments the per-agent serial
// counter, and returns the new name. The caller is responsible for persisting
// the name to the database.
//
// Apply does NOT check ShouldRename; callers should gate on it themselves
// so they control when the rename is skipped.
func (n *NodeNamer) Apply(ctx context.Context, host *repository.AgentHost, srv *repository.Server) (string, error) {
	if !n.IsNamingEnabled(ctx) {
		return srv.Name, nil
	}

	n.mu.Lock()
	n.serials[host.ID]++
	counter := n.serials[host.ID]
	n.mu.Unlock()

	// counter == 1 => first server on this agent => no serial suffix displayed
	// counter >= 2 => displayed as -02, -03, ...
	var displaySerial int
	if counter > 1 {
		displaySerial = counter
	}
	newName := n.BuildName(ctx, host, srv, displaySerial)

	return newName, nil
}

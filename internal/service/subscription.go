// 文件路径: internal/service/subscription.go
// 模块说明: 这是 internal 模块里的 subscription 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"log/slog"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/async"
	"github.com/creamcroissant/mgpanel/internal/cache"
	"github.com/creamcroissant/mgpanel/internal/protocol"
	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// SubscriptionService 负责生成客户端订阅响应。
type SubscriptionService interface {
	Subscribe(ctx context.Context, userID string, params SubscriptionParams) (*SubscriptionResult, error)
	SetCache(c cache.Store)
}

// SubscriptionParams 用于承接客户端传入的过滤参数。
type SubscriptionParams struct {
	Lang         string
	Types        string
	Filter       string
	Flag         string
	UserAgent    string
	Host         string
	Scheme       string
	URL          string
	Tags         string // 按标签过滤节点，逗号分隔
	ShowUserInfo bool   // 是否在节点名称中显示用户信息
	TemplateID   int64  // 用户指定的订阅模板ID
	ClientIP     string // 客户端真实 IP（订阅日志用；空值落 unknown）
}

// SubscriptionResult 包含订阅内容与元数据。
type SubscriptionResult struct {
	Payload     []byte
	ContentType string
	ETag        string
	Headers     map[string]string
}

// subscriptionService 负责订阅生成所需的仓储与依赖。
// subscriptionFilteredResult 是 filterForSubscription 的可序列化输出缓存。
type subscriptionFilteredResult struct {
	Servers    []*repository.Server
	SourceNodes []protocol.Node
}

// filteredResult 是无缓存实现的内部返回类型。
type filteredResult struct {
	servers    []*repository.Server
	nodes      []protocol.Node
}

func subscriptionCacheKey(user *repository.User, allowedTypes map[string]struct{}, keywords []string, tagsFilter []string) string {
	key := "nodes:" + fmt.Sprint(user.ID)
	if len(allowedTypes) > 0 {
		types := make([]string, 0, len(allowedTypes))
		for t := range allowedTypes {
			types = append(types, t)
		}
		sort.Strings(types)
		key += ":t=" + strings.Join(types, ",")
	}
	if len(keywords) > 0 {
		sorted := make([]string, len(keywords))
		copy(sorted, keywords)
		sort.Strings(sorted)
		key += ":k=" + strings.Join(sorted, ",")
	}
	if len(tagsFilter) > 0 {
		sorted := make([]string, len(tagsFilter))
		copy(sorted, tagsFilter)
		sort.Strings(sorted)
		key += ":tg=" + strings.Join(sorted, ",")
	}
	return key
}

type subscriptionService struct {
	users     repository.UserRepository
	servers   repository.ServerRepository
	settings  repository.SettingRepository
	plans     repository.PlanRepository
	templates repository.SubscriptionTemplateRepository
	sources   SubscriptionSourceService
	filter    SubscriptionFilterService
	protocols *protocol.Manager
	telemetry ServerTelemetryService
	subLogs   *async.SubscriptionLogQueue
	obfuscate bool
	selection UserServerSelectionService
	i18n      *i18n.Manager
	cache     cache.Store
}

// SetCache 注入缓存存储，用于订阅渲染结果的短 TTL 缓存。
func (s *subscriptionService) SetCache(c cache.Store) {
	s.cache = c
}

// protocolSettings 保存订阅模板与前端展示配置。
type protocolSettings struct {
	AppName         string
	AppURL          string
	ClashTemplate   string
	SurgeTemplate   string
	SingboxTemplate string
}

// NewSubscriptionService 组装订阅服务依赖。
func NewSubscriptionService(users repository.UserRepository, servers repository.ServerRepository, settings repository.SettingRepository, plans repository.PlanRepository, templates repository.SubscriptionTemplateRepository, sources SubscriptionSourceService, manager *protocol.Manager, telemetry ServerTelemetryService, subLogs *async.SubscriptionLogQueue, obfuscate bool, selection UserServerSelectionService, i18nMgr *i18n.Manager, filters ...SubscriptionFilterService) SubscriptionService {
	var filter SubscriptionFilterService
	if len(filters) > 0 {
		filter = filters[0]
	}
	return &subscriptionService{users: users, servers: servers, settings: settings, plans: plans, templates: templates, sources: sources, filter: filter, protocols: manager, telemetry: telemetry, subLogs: subLogs, obfuscate: obfuscate, selection: selection, i18n: i18nMgr}
}

// queryServers 根据用户显式选择、用户分组与套餐分组决定可用节点。
func (s *subscriptionService) queryServers(ctx context.Context, user *repository.User, lang string) ([]*repository.Server, error) {
	if s.servers == nil {
		return nil, s.translateError(lang, "subscription.error.repo_unavailable", "server repository unavailable / 节点仓库不可用")
	}
	if user == nil {
		return []*repository.Server{}, nil
	}

	// 0. 先收集用户与套餐关联分组（用于校验显式选择节点权限）
	groupIDs := make([]int64, 0, 4)
	if user.GroupID > 0 {
		groupIDs = append(groupIDs, user.GroupID)
	}
	if user.PlanID > 0 && s.plans != nil {
		planGroups, err := s.plans.GetGroups(ctx, user.PlanID)
		if err != nil {
			// 分组信息影响访问控制，查询失败时直接返回错误
			return nil, err
		}
		if len(planGroups) > 0 {
			groupIDs = append(groupIDs, planGroups...)
		}
	}

	// 1. 优先处理用户显式选中的节点
	if s.selection != nil {
		selectedIDs, err := s.selection.GetSelection(ctx, user.ID)
		if err == nil && len(selectedIDs) > 0 {
			// 用户显式选择节点时，仅返回被选中的可见节点
			// TODO: ServerRepository 可增加批量查询以优化循环
			var selectedServers []*repository.Server
			for _, id := range selectedIDs {
				server, err := s.servers.FindByID(ctx, id)
				if err == nil && server != nil && server.Show == 1 {
					if len(groupIDs) > 0 && !containsGroupID(groupIDs, server.GroupID) {
						continue
					}
					selectedServers = append(selectedServers, server)
				}
			}
			return selectedServers, nil
		}
	}

	// 3. 若存在分组限制，则仅返回分组内节点
	if len(groupIDs) > 0 {
		return s.servers.FindByGroupIDs(ctx, groupIDs)
	}

	// 4. 无分组限制时回退为所有可见节点
	// NOTE: 旧逻辑在无用户分组时返回所有节点，这里继续保持一致
	return s.servers.FindAllVisible(ctx)
}

func (s *subscriptionService) filterForSubscription(ctx context.Context, user *repository.User, allowedTypes map[string]struct{}, keywords []string, tagsFilter []string, lang string) ([]*repository.Server, []protocol.Node, error) {
	if s.cache != nil {
		key := subscriptionCacheKey(user, allowedTypes, keywords, tagsFilter)
		var cached subscriptionFilteredResult
		if ok, _ := s.cache.Namespace("subscription").GetJSON(ctx, key, &cached); ok && len(cached.Servers) > 0 {
			return cached.Servers, cached.SourceNodes, nil
		}
	}
	result, err := s.filterForSubscriptionUncached(ctx, user, allowedTypes, keywords, tagsFilter, lang)
	if err != nil {
		return nil, nil, err
	}
	if s.cache != nil {
		_ = s.cache.Namespace("subscription").SetJSON(ctx, subscriptionCacheKey(user, allowedTypes, keywords, tagsFilter), subscriptionFilteredResult{Servers: result.servers, SourceNodes: result.nodes}, 3*time.Second)
	}
	return result.servers, result.nodes, nil
}

// filterForSubscriptionUncached 是 filterForSubscription 的无缓存实现。
func (s *subscriptionService) filterForSubscriptionUncached(ctx context.Context, user *repository.User, allowedTypes map[string]struct{}, keywords []string, tagsFilter []string, lang string) (filteredResult, error) {
	if s.filter != nil {
		// 高频轮询路径不持久化过滤原因（写放大优化）；低频重建经
		// SubscriptionFilterService.SetPersistReasons 显式开启。
		result, err := s.filter.Filter(ctx, SubscriptionFilterRequest{User: user, AllowedTypes: allowedTypes, Keywords: keywords, Tags: tagsFilter, PersistReasons: false})
		if err == nil && result != nil {
			return filteredResult{servers: result.Servers, nodes: result.SourceNodes}, nil
		}
		if err != nil && err != ErrNotImplemented {
			return filteredResult{}, err
		}
	}

	servers, err := s.queryServers(ctx, user, lang)
	if err != nil {
		return filteredResult{}, err
	}
	if len(tagsFilter) > 0 {
		servers = filterServersByTags(servers, tagsFilter)
	}
	filtered, rejected := filterSubscriptionServers(servers, allowedTypes, keywords)
	if rejected > 0 {
		slog.Warn("subscription: filter rejected servers", "rejected", rejected)
	}
	if s.telemetry != nil {
		online := make([]*repository.Server, 0, len(filtered))
		for _, server := range filtered {
			if s.telemetry.IsNodeOnline(ctx, server) {
				online = append(online, server)
			}
		}
		filtered = online
	}

	sourceNodes := []protocol.Node{}
	if s.sources != nil {
		nodes, err := s.sources.BuildEnabledNodes(ctx)
		if err != nil {
			return filteredResult{}, err
		}
		sourceNodes = filterSubscriptionNodes(nodes, allowedTypes, keywords, tagsFilter)
	}
	return filteredResult{servers: filtered, nodes: sourceNodes}, nil
}

// Subscribe 生成用户订阅内容，按类型/关键词/标签过滤并套用协议模板。
func (s *subscriptionService) Subscribe(ctx context.Context, userID string, params SubscriptionParams) (*SubscriptionResult, error) {
	lang := strings.TrimSpace(params.Lang)
	if lang == "" {
		lang = requestctx.GetLanguage(ctx)
	}

	if s == nil || s.users == nil || s.servers == nil || s.protocols == nil {
		return nil, s.translateError(lang, "subscription.error.not_configured", "subscription service not fully configured / 订阅服务未完整配置")
	}
	user, err := loadServerUser(ctx, s.users, userID)
	if err != nil {
		return nil, err
	}
	if !isServerAccessAllowed(user) {
		return nil, ErrUserNotEligible
	}
	allowedTypes := parseRequestedTypes(params.Types)
	keywords := parseFilterKeywords(params.Filter)
	tagsFilter := parseTagsFilter(params.Tags)
	servers, sourceNodes, err := s.filterForSubscription(ctx, user, allowedTypes, keywords, tagsFilter, lang)
	if err != nil {
		return nil, err
	}

	hooked := applyProtocolServerHooks(ctx, servers, user)
	clientInfo := detectClientInfo(params.Flag, params.UserAgent, s.protocols.Flags())
	if s.obfuscate && clientInfo.Name == "" {
		return nil, ErrNotFound
	}
	pl := s.loadProtocolSettings(ctx)

	// 若用户指定模板，则覆盖默认模板
	if params.TemplateID > 0 && s.templates != nil {
		if tpl, err := s.templates.FindByID(ctx, params.TemplateID); err == nil && tpl != nil {
			switch strings.ToLower(tpl.Type) {
			case "clash":
				pl.ClashTemplate = tpl.Content
			case "singbox", "sing-box":
				pl.SingboxTemplate = tpl.Content
			case "surge":
				pl.SurgeTemplate = tpl.Content
			}
		}
	}

	// 构建节点列表并应用个性化显示
	nodes := buildProtocolNodes(hooked, user)
	nodes = append(nodes, sourceNodes...)
	nodes = personalizeNodeNames(nodes, user, params.ShowUserInfo, lang, s.i18n)

	request := protocol.BuildRequest{
		Context:       ctx,
		User:          user,
		Nodes:         nodes,
		Flag:          resolveClientFlag(params),
		UserAgent:     params.UserAgent,
		ClientName:    clientInfo.Name,
		ClientVersion: clientInfo.Version,
		Host:          params.Host,
		AppName:       pl.AppName,
		AppURL:        pl.AppURL,
		SubscribeURL:  s.resolveSubscribeURL(params, user),
		Templates: map[string]string{
			"clash":    pl.ClashTemplate,
			"surge":    pl.SurgeTemplate,
			"sing-box": pl.SingboxTemplate,
		},
		Lang: lang,
		I18n: s.i18n,
	}
	protoResult, err := s.protocols.Build(request)
	if err != nil {
		return nil, err
	}
	if protoResult == nil {
		return nil, s.translateError(lang, "subscription.error.build_empty", "protocol build result is empty / 协议构建结果为空")
	}

	// 异步记录订阅访问日志
	if s.subLogs != nil {
		clientIP := strings.TrimSpace(params.ClientIP)
		if clientIP == "" {
			clientIP = "unknown"
		}
		s.subLogs.Enqueue(&repository.SubscriptionLog{
			UserID:    user.ID,
			IP:        clientIP,
			UserAgent: params.UserAgent,
			Type:      clientInfo.Name,
			URL:       params.URL,
		})
	}

	return &SubscriptionResult{
		Payload:     protoResult.Payload,
		ContentType: protoResult.ContentType,
		ETag:        computeSubscriptionETag(protoResult.Payload),
		Headers:     protoResult.Headers,
	}, nil
}

// loadProtocolSettings 读取订阅相关的系统配置。
func (s *subscriptionService) loadProtocolSettings(ctx context.Context) protocolSettings {
	return protocolSettings{
		AppName:         s.settingString(ctx, "app_name", "MGPanel"),
		AppURL:          s.settingString(ctx, "app_url", ""),
		ClashTemplate:   s.settingString(ctx, "subscribe_template_clash", ""),
		SurgeTemplate:   s.settingString(ctx, "subscribe_template_surge", ""),
		SingboxTemplate: s.settingString(ctx, "subscribe_template_singbox", ""),
	}
}

// settingString 读取设置项并提供默认值回退。
func (s *subscriptionService) settingString(ctx context.Context, key, def string) string {
	if s == nil || s.settings == nil {
		return def
	}
	entry, err := s.settings.Get(ctx, key)
	if err != nil || entry == nil {
		return def
	}
	trimmed := strings.TrimSpace(entry.Value)
	if trimmed == "" {
		return def
	}
	return entry.Value
}

// resolveSubscribeURL 计算订阅链接（优先使用请求中的 URL）。
func (s *subscriptionService) resolveSubscribeURL(params SubscriptionParams, user *repository.User) string {
	if trimmed := strings.TrimSpace(params.URL); trimmed != "" {
		return trimmed
	}
	scheme := strings.TrimSpace(params.Scheme)
	if scheme == "" {
		scheme = "https"
	}
	host := strings.TrimSpace(params.Host)
	if host == "" {
		return ""
	}
	path := "/api/v1/client/subscribe"
	token := ""
	if user != nil {
		token = strings.TrimSpace(user.Token)
	}
	if token == "" {
		return fmt.Sprintf("%s://%s%s", scheme, host, path)
	}
	return fmt.Sprintf("%s://%s%s?token=%s", scheme, host, path, url.QueryEscape(token))
}

// filterSubscriptionServers 按协议类型与关键词过滤节点。
func filterSubscriptionServers(servers []*repository.Server, allowedTypes map[string]struct{}, keywords []string) ([]*repository.Server, int) {
	if len(servers) == 0 {
		return []*repository.Server{}, 0
	}
	var (
		filtered []*repository.Server
		rejected int
	)
	for _, server := range servers {
		if server == nil {
			continue
		}
		if !typeAllowed(server.Type, allowedTypes) {
			rejected++
			continue
		}
		if len(keywords) > 0 && !matchesKeywords(server, keywords) {
			rejected++
			continue
		}
		filtered = append(filtered, server)
	}
	return filtered, rejected
}

// typeAllowed 判断节点类型是否在允许列表中。
func typeAllowed(serverType string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return true
	}
	normalized := strings.ToLower(strings.TrimSpace(serverType))
	_, ok := allowed[normalized]
	return ok
}

// matchesKeywords 判断节点名称或标签是否匹配关键词。
func matchesKeywords(server *repository.Server, keywords []string) bool {
	name := strings.ToLower(server.Name)
	tags := decodeStringArray(server.Tags)
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		if strings.Contains(name, keyword) {
			return true
		}
		for _, tag := range tags {
			if strings.Contains(strings.ToLower(tag), keyword) {
				return true
			}
		}
	}
	return false
}

// parseRequestedTypes 解析客户端请求的协议类型过滤。
func parseRequestedTypes(raw string) map[string]struct{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.EqualFold(trimmed, "all") {
		return nil
	}
	tokens := splitTokens(trimmed)
	allowed := make(map[string]struct{})
	for _, token := range tokens {
		normalized := strings.ToLower(token)
		if _, ok := validServerTypes[normalized]; ok {
			allowed[normalized] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

// parseFilterKeywords 解析节点筛选关键词（最多 20 字符）。
func parseFilterKeywords(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if len([]rune(trimmed)) > 20 {
		return nil
	}
	tokens := splitTokens(trimmed)
	var keywords []string
	for _, token := range tokens {
		keyword := strings.ToLower(strings.TrimSpace(token))
		if keyword != "" {
			keywords = append(keywords, keyword)
		}
	}
	if len(keywords) == 0 {
		return nil
	}
	return keywords
}

// splitTokens 按常见分隔符拆分参数并去空白。
func splitTokens(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '|', '｜':
			return true
		default:
			return false
		}
	})
	var cleaned []string
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

// resolveClientFlag 优先使用显式 Flag，否则回退为 UserAgent。
func resolveClientFlag(params SubscriptionParams) string {
	if strings.TrimSpace(params.Flag) != "" {
		return strings.TrimSpace(params.Flag)
	}
	return strings.TrimSpace(params.UserAgent)
}

type clientDescriptor struct {
	Name    string
	Version string
}

// detectClientInfo 通过 Flag/UserAgent 识别客户端名称与版本。
func detectClientInfo(rawFlag, userAgent string, known []string) clientDescriptor {
	info := clientDescriptor{}
	combined := strings.ToLower(strings.TrimSpace(rawFlag))
	if combined == "" {
		combined = strings.ToLower(strings.TrimSpace(userAgent))
	}
	if combined == "" {
		return info
	}
	normalizedFlags := normalizeFlags(known)
	if name, version := regexMatchClient(combined, normalizedFlags); name != "" {
		info.Name = name
		info.Version = version
	} else if name := substringMatchClient(combined, normalizedFlags); name != "" {
		info.Name = name
		info.Version = extractVersionForFlag(combined, name)
	}
	if info.Version == "" {
		info.Version = extractStandaloneVersion(combined)
	}
	return info
}

// normalizeFlags 归一化客户端标识，并按长度降序排序以避免短标识误匹配。
func normalizeFlags(flags []string) []string {
	set := make(map[string]struct{})
	for _, flag := range flags {
		candidate := strings.ToLower(strings.TrimSpace(flag))
		if candidate == "" {
			continue
		}
		set[candidate] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for flag := range set {
		result = append(result, flag)
	}
	sort.Slice(result, func(i, j int) bool {
		if len(result[i]) == len(result[j]) {
			return result[i] < result[j]
		}
		return len(result[i]) > len(result[j])
	})
	return result
}

// regexMatchClient 通过正则匹配“客户端/版本”模式。
func regexMatchClient(input string, flags []string) (string, string) {
	re := regexp.MustCompile(`([a-z0-9\-_]+)[/\s]+(v?[0-9]+(?:\.[0-9]+){0,2})`)
	if matches := re.FindStringSubmatch(input); len(matches) == 3 {
		candidate := matches[1]
		if containsFlag(candidate, flags) {
			return candidate, trimVersionPrefix(matches[2])
		}
	}
	return "", ""
}

// substringMatchClient 通过子串命中客户端标识。
func substringMatchClient(input string, flags []string) string {
	for _, flag := range flags {
		if strings.Contains(input, flag) {
			return flag
		}
	}
	return ""
}

// extractVersionForFlag 在包含客户端标识的字符串中提取版本号。
func extractVersionForFlag(input, flag string) string {
	if flag == "" {
		return ""
	}
	pattern := fmt.Sprintf(`%s[/\s]+(v?[0-9]+(?:\.[0-9]+){0,2})`, regexp.QuoteMeta(flag))
	re := regexp.MustCompile(pattern)
	if matches := re.FindStringSubmatch(input); len(matches) == 2 {
		return trimVersionPrefix(matches[1])
	}
	return ""
}

// extractStandaloneVersion 提取独立版本号（不依赖客户端标识）。
func extractStandaloneVersion(input string) string {
	re := regexp.MustCompile(`/v?(\d+(?:\.\d+){0,2})`)
	if matches := re.FindStringSubmatch(input); len(matches) == 2 {
		return trimVersionPrefix(matches[1])
	}
	return ""
}

// trimVersionPrefix 去掉版本号前缀 v。
func trimVersionPrefix(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

// containsFlag 判断命中的客户端标识是否在允许列表中。
func containsFlag(target string, flags []string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, flag := range flags {
		if flag == target {
			return true
		}
	}
	return false
}

func containsGroupID(groupIDs []int64, target int64) bool {
	for _, id := range groupIDs {
		if id == target {
			return true
		}
	}
	return false
}

// computeSubscriptionETag 用于生成订阅内容的 ETag。
func computeSubscriptionETag(payload []byte) string {
	sum := sha1.Sum(payload)
	return hex.EncodeToString(sum[:])
}

// validServerTypes 允许的协议类型白名单。
var validServerTypes = map[string]struct{}{
	"hysteria":    {},
	"vless":       {},
	"shadowsocks": {},
	"vmess":       {},
	"trojan":      {},
	"tuic":        {},
	"socks":       {},
	"anytls":      {},
	"naive":       {},
	"http":        {},
	"mieru":       {},
}

func (s *subscriptionService) translateError(lang, key, fallback string) error {
	if s == nil || s.i18n == nil {
		return fmt.Errorf("%s", fallback)
	}
	msg := s.i18n.Translate(lang, key)
	if strings.TrimSpace(msg) == "" || msg == key {
		return fmt.Errorf("%s", fallback)
	}
	return fmt.Errorf("%s", msg)
}

func formatI18n(i18nMgr *i18n.Manager, lang, key string, args ...any) string {
	if i18nMgr == nil {
		return key
	}
	return i18nMgr.Translate(lang, key, args...)
}

// personalizeNodeNames 为节点名称添加用户个性化信息（剩余时间/剩余流量）。
func personalizeNodeNames(nodes []protocol.Node, user *repository.User, showUserInfo bool, lang string, i18nMgr *i18n.Manager) []protocol.Node {
	if !showUserInfo || user == nil {
		return nodes
	}

	now := time.Now().Unix()
	var suffix string

	// 按套餐到期时间生成提示后缀
	if user.ExpiredAt > 0 {
		daysLeft := (user.ExpiredAt - now) / 86400
		if daysLeft <= 0 {
			suffix = " | " + formatI18n(i18nMgr, lang, "subscription.node.expired")
		} else if daysLeft <= 7 {
			suffix = " | " + formatI18n(i18nMgr, lang, "subscription.node.days_left", daysLeft)
		}
	}

	// 如果未提示到期，再按剩余流量生成提示
	if user.TransferEnable > 0 && suffix == "" {
		used := user.U + user.D
		remaining := user.TransferEnable - used
		if remaining <= 0 {
			suffix = " | " + formatI18n(i18nMgr, lang, "subscription.node.exhausted")
		} else {
			remainingLabel := formatI18n(i18nMgr, lang, "subscription.node.remaining", formatTrafficSize(remaining))
			suffix = " | " + remainingLabel
		}
	}

	if suffix == "" {
		return nodes
	}

	result := make([]protocol.Node, len(nodes))
	for i, node := range nodes {
		result[i] = node
		result[i].Name = node.Name + suffix
	}
	return result
}

// formatTrafficSize 将字节数转换为可读的流量格式。
func formatTrafficSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1fTB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// parseTagsFilter 解析标签过滤参数（逗号/竖线分隔）。
func parseTagsFilter(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	tokens := splitTokens(trimmed)
	var tags []string
	for _, token := range tokens {
		tag := strings.ToLower(strings.TrimSpace(token))
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// filterServersByTags 按标签过滤服务器列表。
func filterServersByTags(servers []*repository.Server, requiredTags []string) []*repository.Server {
	if len(requiredTags) == 0 {
		return servers
	}

	var filtered []*repository.Server
	for _, server := range servers {
		if server == nil {
			continue
		}
		serverTags := decodeStringArray(server.Tags)
		if matchesAnyTag(serverTags, requiredTags) {
			filtered = append(filtered, server)
		}
	}
	return filtered
}

// matchesAnyTag 检查服务器标签是否命中任意过滤标签。
func matchesAnyTag(serverTags, requiredTags []string) bool {
	for _, required := range requiredTags {
		for _, tag := range serverTags {
			if strings.EqualFold(tag, required) {
				return true
			}
		}
	}
	return false
}

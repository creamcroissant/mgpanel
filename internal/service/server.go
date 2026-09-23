// 文件路径: internal/service/server.go
// 模块说明: 这是 internal 模块里的 server 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// ServerService 提供按用户权限过滤的节点查询能力。
type ServerService interface {
	ListForUser(ctx context.Context, userID string) (*ServerListResult, error)
	Heartbeat(ctx context.Context, nodeID int) error
}

// ServerListResult 表示用户节点列表的返回结果。
type ServerListResult struct {
	Nodes []ServerNode
	ETag  string
}

// ServerNode 对齐旧 PHP 栈使用的 NodeResource 数据结构。
type ServerNode struct {
	ID          int64    `json:"id"`
	Type        string   `json:"type"`
	Version     *int     `json:"version,omitempty"`
	Name        string   `json:"name"`
	Rate        string   `json:"rate"`
	Tags        []string `json:"tags"`
	IsOnline    int      `json:"is_online"`
	Status      int      `json:"status"`
	Country     string   `json:"country,omitempty"`
	Region      string   `json:"region,omitempty"`
	CacheKey    string   `json:"cache_key"`
	LastCheckAt int64    `json:"last_check_at"`
}

type serverService struct {
	users      repository.UserRepository
	servers    repository.ServerRepository
	agentHosts repository.AgentHostRepository
	deny       UserServerDenyService
}

// NewServerService 组装基于 repository 的依赖。
func NewServerService(users repository.UserRepository, servers repository.ServerRepository, deny UserServerDenyService, agentHosts ...repository.AgentHostRepository) ServerService {
	var hosts repository.AgentHostRepository
	if len(agentHosts) > 0 {
		hosts = agentHosts[0]
	}
	return &serverService{users: users, servers: servers, agentHosts: hosts, deny: deny}
}

func (s *serverService) ListForUser(ctx context.Context, userID string) (*ServerListResult, error) {
	if s == nil || s.users == nil || s.servers == nil {
		return nil, fmt.Errorf("server service not fully configured / 节点服务未完整配置")
	}
	user, err := loadServerUser(ctx, s.users, userID)
	if err != nil {
		return nil, err
	}
	if !isServerAccessAllowed(user) {
		return &ServerListResult{Nodes: []ServerNode{}, ETag: computeETag(nil)}, nil
	}

	// 用户直接通过分组访问节点；
	// 仅管理员默认看全部分组可见节点，普通用户未分配分组则返回空列表。
	var nodes []*repository.Server
	if user.IsAdmin {
		nodes, err = s.servers.FindAllVisible(ctx)
		if err != nil {
			return nil, err
		}
	} else if user.GroupID > 0 {
		nodes, err = s.servers.FindByGroupIDs(ctx, []int64{user.GroupID})
		if err != nil {
			return nil, err
		}
	} else {
		// 普通用户未分配分组：默认禁用所有节点，返回空列表
		nodes = []*repository.Server{}
	}
	// 节点黑名单：被禁节点不展示给用户前台
	if deniedIDs := s.deniedServerIDs(ctx, user.ID); len(deniedIDs) > 0 && len(nodes) > 0 {
		var kept []*repository.Server
		for _, node := range nodes {
			if node != nil && !isServerDenied(deniedIDs, node.ID) {
				kept = append(kept, node)
			}
		}
		nodes = kept
	}
	now := time.Now().Unix()
	hostMap := make(map[int64]*repository.AgentHost)
	if s.agentHosts != nil {
		for _, node := range nodes {
			if node.AgentHostID > 0 {
				if _, ok := hostMap[node.AgentHostID]; !ok {
					if host, err := s.agentHosts.FindByID(ctx, node.AgentHostID); err == nil && host != nil {
						hostMap[node.AgentHostID] = host
					}
				}
			}
		}
	}

	views := make([]ServerNode, 0, len(nodes))
	cacheKeys := make([]string, 0, len(nodes))
	for _, node := range nodes {
		var host *repository.AgentHost
		if node.AgentHostID > 0 {
			host = hostMap[node.AgentHostID]
		}
		view := transformServerNode(node, host, now)
		views = append(views, view)
		cacheKeys = append(cacheKeys, view.CacheKey)
	}
	return &ServerListResult{Nodes: views, ETag: computeETag(cacheKeys)}, nil
}

func (s *serverService) deniedServerIDs(ctx context.Context, userID int64) map[int64]struct{} {
	if s == nil || s.deny == nil {
		return nil
	}
	ids, err := s.deny.GetUserDeniedServerIDs(ctx, userID)
	if err != nil || len(ids) == 0 {
		return nil
	}
	denied := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id > 0 {
			denied[id] = struct{}{}
		}
	}
	return denied
}

func (s *serverService) Heartbeat(ctx context.Context, nodeID int) error {
	return s.servers.UpdateHeartbeat(ctx, int64(nodeID), time.Now().Unix())
}

func transformServerNode(server *repository.Server, host *repository.AgentHost, now int64) ServerNode {
	if server == nil {
		return ServerNode{}
	}
	tags := decodeStringArray(server.Tags)
	version := extractVersion(server.Settings)

	status := 0
	if server.Status == 0 {
		status = 0
	} else if host != nil {
		hbDiff := now - host.LastHeartbeatAt
		if host.LastHeartbeatAt > 0 && hbDiff >= 0 && hbDiff <= 600 {
			if host.Status == 1 && hbDiff <= 180 {
				status = 1
			} else if host.Status == 2 || (host.Status == 1 && hbDiff > 180) {
				status = 2
			} else {
				status = 0
			}
		} else {
			status = 0
		}
	} else {
		if server.LastHeartbeatAt > 0 {
			diff := now - server.LastHeartbeatAt
			if diff >= 0 && diff <= 300 {
				status = 1
			} else if diff > 300 && diff <= 900 {
				status = 2
			} else {
				status = 0
			}
		} else {
			if server.Status > 0 {
				status = 1
			} else {
				status = 0
			}
		}
	}

	isOnline := 0
	if status > 0 {
		isOnline = 1
	}

	var country, region string
	if host != nil {
		country = host.Country
		region = host.Region
	}
	if country == "" {
		extractedCountry, extractedRegion := extractCountryAndRegion(server.Name)
		if extractedCountry != "" {
			country = extractedCountry
		}
		if region == "" && extractedRegion != "" {
			region = extractedRegion
		}
	}

	cacheKey := fmt.Sprintf("%s-%d-%d-%d-%d", server.Type, server.ID, server.UpdatedAt, isOnline, status)
	lastCheck := server.UpdatedAt
	return ServerNode{
		ID:          server.ID,
		Type:        server.Type,
		Version:     version,
		Name:        server.Name,
		Rate:        server.Rate,
		Tags:        tags,
		IsOnline:    isOnline,
		Status:      status,
		Country:     country,
		Region:      region,
		CacheKey:    cacheKey,
		LastCheckAt: lastCheck,
	}
}

// extractCountryAndRegion attempts to extract ISO 2-letter country code and region from a server name.
func extractCountryAndRegion(name string) (string, string) {
	if name == "" {
		return "", ""
	}

	var country, region string

	// 1. Check for Regional Indicator Symbol flag emoji (U+1F1E6 - U+1F1FF).
	runes := []rune(name)
	for i := 0; i < len(runes)-1; i++ {
		r1, r2 := runes[i], runes[i+1]
		if r1 >= 0x1F1E6 && r1 <= 0x1F1FF && r2 >= 0x1F1E6 && r2 <= 0x1F1FF {
			c1 := byte('A' + (r1 - 0x1F1E6))
			c2 := byte('A' + (r2 - 0x1F1E6))
			country = string([]byte{c1, c2})
			break
		}
	}

	// 2. Look for Chinese keywords for country and region (specific cities before country names).
	type geoMatch struct {
		keyword string
		country string
		region  string
	}
	chineseLocations := []geoMatch{
		// Cities / Regions first
		{"东京", "JP", "Tokyo"},
		{"大阪", "JP", "Osaka"},
		{"洛杉矶", "US", "Los Angeles"},
		{"硅谷", "US", "Silicon Valley"},
		{"西雅图", "US", "Seattle"},
		{"纽约", "US", "New York"},
		{"伦敦", "GB", "London"},
		{"法兰克福", "DE", "Frankfurt"},
		{"巴黎", "FR", "Paris"},
		{"首尔", "KR", "Seoul"},
		{"温哥华", "CA", "Vancouver"},
		{"多伦多", "CA", "Toronto"},
		{"悉尼", "AU", "Sydney"},
		{"墨尔本", "AU", "Melbourne"},
		{"台北", "TW", "Taipei"},
		{"莫斯科", "RU", "Moscow"},
		{"阿姆斯特丹", "NL", "Amsterdam"},
		{"伊斯坦布尔", "TR", "Istanbul"},
		{"圣保罗", "BR", "Sao Paulo"},
		{"布宜诺斯艾利斯", "AR", "Buenos Aires"},
		{"吉隆坡", "MY", "Kuala Lumpur"},
		{"曼谷", "TH", "Bangkok"},
		{"胡志明", "VN", "Ho Chi Minh"},
		{"马尼拉", "PH", "Manila"},

		// Countries / Territories
		{"香港", "HK", "Hong Kong"},
		{"日本", "JP", "Japan"},
		{"新加坡", "SG", "Singapore"},
		{"狮城", "SG", "Singapore"},
		{"美国", "US", "United States"},
		{"英国", "GB", "United Kingdom"},
		{"德国", "DE", "Germany"},
		{"法国", "FR", "France"},
		{"韩国", "KR", "Korea"},
		{"加拿大", "CA", "Canada"},
		{"澳大利亚", "AU", "Australia"},
		{"澳洲", "AU", "Australia"},
		{"台湾", "TW", "Taiwan"},
		{"俄罗斯", "RU", "Russia"},
		{"印度", "IN", "India"},
		{"荷兰", "NL", "Netherlands"},
		{"土耳其", "TR", "Turkey"},
		{"巴西", "BR", "Brazil"},
		{"阿根廷", "AR", "Argentina"},
		{"马来西亚", "MY", "Malaysia"},
		{"泰国", "TH", "Thailand"},
		{"越南", "VN", "Vietnam"},
		{"菲律宾", "PH", "Philippines"},
	}

	for _, loc := range chineseLocations {
		if strings.Contains(name, loc.keyword) {
			if country == "" {
				country = loc.country
			}
			if region == "" {
				region = loc.region
			}
			if country != "" && region != "" && region != "Japan" && region != "United States" && region != "Germany" && region != "Korea" && region != "Australia" && region != "Canada" && region != "United Kingdom" {
				break
			}
		}
	}

	// 3. Look for ASCII country codes / tokens delimited by non-alphanumeric chars.
	asciiRegions := []geoMatch{
		{"SiliconValley", "US", "Silicon Valley"},
		{"LosAngeles", "US", "Los Angeles"},
		{"Tokyo", "JP", "Tokyo"},
		{"Osaka", "JP", "Osaka"},
		{"London", "GB", "London"},
		{"Frankfurt", "DE", "Frankfurt"},
		{"Seoul", "KR", "Seoul"},
		{"Sydney", "AU", "Sydney"},
		{"Singapore", "SG", "Singapore"},
		{"HongKong", "HK", "Hong Kong"},
	}
	for _, loc := range asciiRegions {
		if strings.Contains(strings.ReplaceAll(name, " ", ""), loc.keyword) {
			if country == "" {
				country = loc.country
			}
			if region == "" || region == "United States" || region == "Japan" {
				region = loc.region
			}
			break
		}
	}

	// 3. Look for ASCII country codes / tokens delimited by non-alphanumeric chars.
	if country == "" {
		asciiCodes := map[string]struct {
			country string
			region  string
		}{
			"HK":  {"HK", "Hong Kong"},
			"JP":  {"JP", "Japan"},
			"SG":  {"SG", "Singapore"},
			"US":  {"US", "United States"},
			"USA": {"US", "United States"},
			"UK":  {"GB", "United Kingdom"},
			"GB":  {"GB", "United Kingdom"},
			"DE":  {"DE", "Germany"},
			"FR":  {"FR", "France"},
			"KR":  {"KR", "Korea"},
			"CA":  {"CA", "Canada"},
			"AU":  {"AU", "Australia"},
			"TW":  {"TW", "Taiwan"},
			"RU":  {"RU", "Russia"},
			"IN":  {"IN", "India"},
			"NL":  {"NL", "Netherlands"},
			"TR":  {"TR", "Turkey"},
			"BR":  {"BR", "Brazil"},
			"AR":  {"AR", "Argentina"},
			"MY":  {"MY", "Malaysia"},
			"TH":  {"TH", "Thailand"},
			"VN":  {"VN", "Vietnam"},
			"PH":  {"PH", "Philippines"},
		}

		tokens := strings.FieldsFunc(name, func(r rune) bool {
			return r == ' ' || r == '-' || r == '_' || r == '[' || r == ']' || r == '(' || r == ')' || r == '|' || r == '/' || r == ':' || r == '.' || r == '#' || r == '@'
		})
		for _, token := range tokens {
			upper := strings.ToUpper(token)
			if match, ok := asciiCodes[upper]; ok {
				country = match.country
				if region == "" {
					region = match.region
				}
				break
			}
		}
	}

	return country, region
}

func decodeStringArray(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	return nil
}

func extractVersion(raw json.RawMessage) *int {
	if len(raw) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	if val, ok := payload["version"]; ok {
		switch v := val.(type) {
		case float64:
			n := int(v)
			return &n
		case int:
			n := v
			return &n
		case int64:
			n := int(v)
			return &n
		}
	}
	return nil
}

func computeETag(keys []string) string {
	if len(keys) == 0 {
		keys = []string{}
	}
	data, err := json.Marshal(keys)
	if err != nil {
		data = []byte{}
	}
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}

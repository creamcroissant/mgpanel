package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	agentsubscribe "github.com/creamcroissant/mgpanel/internal/agent/protocol/subscribe"
	"github.com/creamcroissant/mgpanel/internal/protocol"
	"github.com/creamcroissant/mgpanel/internal/repository"
)

const (
	SubscriptionSourceTypeSelfHosted = "self_hosted"
	SubscriptionSourceTypeImported   = "imported_subscription"
	SubscriptionSourceTypeCustom     = "custom_node"

	maxImportedSubscriptionBytes = 10 << 20
)

type SubscriptionSourceService interface {
	Create(ctx context.Context, req UpsertSubscriptionSourceRequest) (*repository.SubscriptionSource, error)
	Update(ctx context.Context, id int64, req UpsertSubscriptionSourceRequest) (*repository.SubscriptionSource, error)
	Delete(ctx context.Context, id int64) error
	Get(ctx context.Context, id int64) (*repository.SubscriptionSource, error)
	List(ctx context.Context, req ListSubscriptionSourcesRequest) (*SubscriptionSourceListResult, error)
	SyncImported(ctx context.Context, id int64) (*SubscriptionSourceSyncResult, error)
	BuildEnabledNodes(ctx context.Context) ([]protocol.Node, error)
}

type SubscriptionSourceFetcher interface {
	Fetch(ctx context.Context, rawURL string) ([]byte, error)
}

type SubscriptionSourceServiceOptions struct {
	Fetcher SubscriptionSourceFetcher
	Now     func() time.Time
}

type UpsertSubscriptionSourceRequest struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	Content string `json:"content,omitempty"`
	Enabled bool   `json:"enabled"`
}

type ListSubscriptionSourcesRequest struct {
	Type    string `json:"type,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
	Keyword string `json:"keyword,omitempty"`
	Limit   int    `json:"limit,omitempty"`
	Offset  int    `json:"offset,omitempty"`
}

type SubscriptionSourceListResult struct {
	Sources []*repository.SubscriptionSource `json:"sources"`
	Total   int64                            `json:"total"`
}

type SubscriptionSourceSyncResult struct {
	Source    *repository.SubscriptionSource `json:"source"`
	Success   bool                           `json:"success"`
	NodeCount int                            `json:"node_count"`
	Error     string                         `json:"error,omitempty"`
	SyncedAt  int64                          `json:"synced_at"`
}

type subscriptionSourceService struct {
	sources repository.SubscriptionSourceRepository
	fetcher SubscriptionSourceFetcher
	now     func() time.Time
}

func NewSubscriptionSourceService(sources repository.SubscriptionSourceRepository, options SubscriptionSourceServiceOptions) SubscriptionSourceService {
	fetcher := options.Fetcher
	if fetcher == nil {
		fetcher = &httpSubscriptionSourceFetcher{client: &http.Client{Timeout: 15 * time.Second}}
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &subscriptionSourceService{sources: sources, fetcher: fetcher, now: now}
}

func (s *subscriptionSourceService) Create(ctx context.Context, req UpsertSubscriptionSourceRequest) (*repository.SubscriptionSource, error) {
	if s == nil || s.sources == nil {
		return nil, ErrNotImplemented
	}
	source, err := buildSubscriptionSourceForWrite(0, req)
	if err != nil {
		return nil, err
	}
	created, err := s.sources.Create(ctx, source)
	if err != nil {
		return nil, mapSubscriptionSourceRepoErr(err)
	}
	return created, nil
}

func (s *subscriptionSourceService) Update(ctx context.Context, id int64, req UpsertSubscriptionSourceRequest) (*repository.SubscriptionSource, error) {
	if s == nil || s.sources == nil {
		return nil, ErrNotImplemented
	}
	if id <= 0 {
		return nil, ErrBadRequest
	}
	existing, err := s.sources.FindByID(ctx, id)
	if err != nil {
		return nil, mapSubscriptionSourceRepoErr(err)
	}
	source, err := buildSubscriptionSourceForWrite(id, req)
	if err != nil {
		return nil, err
	}
	source.LastSyncAt = existing.LastSyncAt
	source.LastSyncErr = existing.LastSyncErr
	if source.Type == SubscriptionSourceTypeImported && source.Content == "" {
		source.Content = existing.Content
	}
	if err := s.sources.Update(ctx, source); err != nil {
		return nil, mapSubscriptionSourceRepoErr(err)
	}
	return s.Get(ctx, id)
}

func (s *subscriptionSourceService) Delete(ctx context.Context, id int64) error {
	if s == nil || s.sources == nil {
		return ErrNotImplemented
	}
	if id <= 0 {
		return ErrBadRequest
	}
	return mapSubscriptionSourceRepoErr(s.sources.Delete(ctx, id))
}

func (s *subscriptionSourceService) Get(ctx context.Context, id int64) (*repository.SubscriptionSource, error) {
	if s == nil || s.sources == nil {
		return nil, ErrNotImplemented
	}
	if id <= 0 {
		return nil, ErrBadRequest
	}
	source, err := s.sources.FindByID(ctx, id)
	if err != nil {
		return nil, mapSubscriptionSourceRepoErr(err)
	}
	return source, nil
}

func (s *subscriptionSourceService) List(ctx context.Context, req ListSubscriptionSourcesRequest) (*SubscriptionSourceListResult, error) {
	if s == nil || s.sources == nil {
		return nil, ErrNotImplemented
	}
	filter, err := buildSubscriptionSourceFilter(req)
	if err != nil {
		return nil, err
	}
	sources, err := s.sources.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	total, err := s.sources.Count(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &SubscriptionSourceListResult{Sources: sources, Total: total}, nil
}

func (s *subscriptionSourceService) SyncImported(ctx context.Context, id int64) (*SubscriptionSourceSyncResult, error) {
	if s == nil || s.sources == nil || s.fetcher == nil {
		return nil, ErrNotImplemented
	}
	source, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if normalizeSubscriptionSourceType(source.Type) != SubscriptionSourceTypeImported {
		return nil, ErrBadRequest
	}
	syncedAt := s.now().Unix()
	content, err := s.fetcher.Fetch(ctx, source.URL)
	if err != nil {
		syncErr := sanitizeSubscriptionSourceSyncError(err, source.URL)
		if updateErr := s.sources.UpdateSyncResult(ctx, id, source.Content, syncErr, syncedAt); updateErr != nil {
			return nil, mapSubscriptionSourceRepoErr(updateErr)
		}
		updated, getErr := s.Get(ctx, id)
		if getErr != nil {
			return nil, getErr
		}
		return &SubscriptionSourceSyncResult{Source: updated, Success: false, Error: syncErr, SyncedAt: syncedAt}, nil
	}

	nodes, parseErr := parseImportedSubscriptionNodes(string(content))
	if parseErr != nil {
		syncErr := sanitizeSubscriptionSourceSyncError(parseErr, source.URL)
		if updateErr := s.sources.UpdateSyncResult(ctx, id, source.Content, syncErr, syncedAt); updateErr != nil {
			return nil, mapSubscriptionSourceRepoErr(updateErr)
		}
		updated, getErr := s.Get(ctx, id)
		if getErr != nil {
			return nil, getErr
		}
		return &SubscriptionSourceSyncResult{Source: updated, Success: false, Error: syncErr, SyncedAt: syncedAt}, nil
	}

	if err := s.sources.UpdateSyncResult(ctx, id, string(content), "", syncedAt); err != nil {
		return nil, mapSubscriptionSourceRepoErr(err)
	}
	updated, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &SubscriptionSourceSyncResult{Source: updated, Success: true, NodeCount: len(nodes), SyncedAt: syncedAt}, nil
}

func (s *subscriptionSourceService) BuildEnabledNodes(ctx context.Context) ([]protocol.Node, error) {
	if s == nil || s.sources == nil {
		return []protocol.Node{}, nil
	}
	enabled := true
	sources, err := s.sources.List(ctx, repository.SubscriptionSourceFilter{Enabled: &enabled, Limit: 1000})
	if err != nil {
		return nil, err
	}
	nodes := make([]protocol.Node, 0, len(sources))
	for _, source := range sources {
		if source == nil || !source.Enabled {
			continue
		}
		sourceNodes, err := buildSubscriptionSourceNodes(source)
		if err != nil {
			continue
		}
		nodes = append(nodes, sourceNodes...)
	}
	return nodes, nil
}

func buildSubscriptionSourceForWrite(id int64, req UpsertSubscriptionSourceRequest) (*repository.SubscriptionSource, error) {
	sourceType := normalizeSubscriptionSourceType(req.Type)
	if !validSubscriptionSourceType(sourceType) {
		return nil, ErrBadRequest
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrBadRequest
	}
	urlValue := strings.TrimSpace(req.URL)
	if sourceType == SubscriptionSourceTypeImported && urlValue == "" {
		return nil, ErrBadRequest
	}
	if urlValue != "" {
		if err := validateSubscriptionSourceURLSyntax(urlValue); err != nil {
			return nil, err
		}
	}
	return &repository.SubscriptionSource{
		ID:      id,
		Type:    sourceType,
		Name:    name,
		URL:     urlValue,
		Content: strings.TrimSpace(req.Content),
		Enabled: req.Enabled,
	}, nil
}

func buildSubscriptionSourceFilter(req ListSubscriptionSourcesRequest) (repository.SubscriptionSourceFilter, error) {
	filter := repository.SubscriptionSourceFilter{
		Enabled: req.Enabled,
		Keyword: strings.TrimSpace(req.Keyword),
		Limit:   req.Limit,
		Offset:  req.Offset,
	}
	if trimmed := strings.TrimSpace(req.Type); trimmed != "" {
		sourceType := normalizeSubscriptionSourceType(trimmed)
		if !validSubscriptionSourceType(sourceType) {
			return filter, ErrBadRequest
		}
		filter.Type = &sourceType
	}
	return filter, nil
}

func buildSubscriptionSourceNodes(source *repository.SubscriptionSource) ([]protocol.Node, error) {
	if source == nil {
		return []protocol.Node{}, nil
	}
	switch normalizeSubscriptionSourceType(source.Type) {
	case SubscriptionSourceTypeImported:
		return parseImportedSubscriptionNodes(source.Content)
	case SubscriptionSourceTypeCustom:
		return parseCustomSubscriptionNodes(source.Content)
	default:
		return []protocol.Node{}, nil
	}
}

func parseImportedSubscriptionNodes(content string) ([]protocol.Node, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return []protocol.Node{}, nil
	}
	data := []byte(trimmed)
	parsers := []func([]byte) ([]agentsubscribe.ClientConfig, error){
		agentsubscribe.ParseSingBoxOutbounds,
		agentsubscribe.ParseClash,
		agentsubscribe.ParseV2RayN,
	}
	var firstErr error
	for _, parser := range parsers {
		configs, err := parser(data)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		nodes := subscriptionClientConfigsToNodes(configs)
		if len(nodes) > 0 {
			return nodes, nil
		}
	}
	if firstErr != nil {
		return []protocol.Node{}, firstErr
	}
	return []protocol.Node{}, nil
}

type customSubscriptionNode struct {
	ID               int64             `json:"id"`
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	Protocol         string            `json:"protocol"`
	Host             string            `json:"host"`
	Server           string            `json:"server"`
	CustomHost       string            `json:"custom_host"`
	RelayHost        string            `json:"relay_host"`
	Port             int               `json:"port"`
	ServerPort       int               `json:"server_port"`
	Rate             string            `json:"rate"`
	Tags             []string          `json:"tags"`
	Ports            string            `json:"ports"`
	Password         string            `json:"password"`
	UUID             string            `json:"uuid"`
	Cipher           string            `json:"cipher"`
	Network          string            `json:"network"`
	Path             string            `json:"path"`
	XHTTPHost        string            `json:"xhttp_host"`
	Mode             string            `json:"mode"`
	ServiceName      string            `json:"service_name"`
	Headers          map[string]string `json:"headers"`
	Extra            map[string]any    `json:"extra"`
	XMux             map[string]any    `json:"xmux"`
	DownloadSettings map[string]any    `json:"download_settings"`
	TLS              bool              `json:"tls"`
	SNI              string            `json:"sni"`
	ALPN             string            `json:"alpn"`
	Fingerprint      string            `json:"fingerprint"`
	Insecure         bool              `json:"insecure"`
	Flow             string            `json:"flow"`
	Settings         map[string]any    `json:"settings"`
}

func parseCustomSubscriptionNodes(content string) ([]protocol.Node, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return []protocol.Node{}, nil
	}
	var customNodes []customSubscriptionNode
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &customNodes); err != nil {
			return nil, err
		}
	} else {
		var wrapper struct {
			Nodes []customSubscriptionNode `json:"nodes"`
		}
		if err := json.Unmarshal([]byte(trimmed), &wrapper); err == nil && len(wrapper.Nodes) > 0 {
			customNodes = wrapper.Nodes
		} else {
			var single customSubscriptionNode
			if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
				return parseImportedSubscriptionNodes(trimmed)
			}
			customNodes = []customSubscriptionNode{single}
		}
	}
	nodes := make([]protocol.Node, 0, len(customNodes))
	for _, item := range customNodes {
		if node, ok := customSubscriptionNodeToProtocolNode(item); ok {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func subscriptionClientConfigsToNodes(configs []agentsubscribe.ClientConfig) []protocol.Node {
	nodes := make([]protocol.Node, 0, len(configs))
	for _, cfg := range configs {
		if node, ok := subscriptionClientConfigToNode(cfg); ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func subscriptionClientConfigToNode(cfg agentsubscribe.ClientConfig) (protocol.Node, bool) {
	nodeType := normalizeSubscriptionProtocolType(cfg.Protocol)
	if nodeType == "" || strings.TrimSpace(cfg.Server) == "" || cfg.Port <= 0 {
		return protocol.Node{}, false
	}
	settings := settingsFromSubscriptionClientConfig(cfg)
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = fmt.Sprintf("%s-%s-%d", nodeType, cfg.Server, cfg.Port)
	}
	node := protocol.Node{
		Name:     name,
		Type:     nodeType,
		Host:     strings.TrimSpace(cfg.Server),
		Port:     cfg.Port,
		Settings: settings,
		Password: subscriptionClientConfigSecret(cfg),
	}
	if node.Password == "" {
		return protocol.Node{}, false
	}
	return node, true
}

func customSubscriptionNodeToProtocolNode(item customSubscriptionNode) (protocol.Node, bool) {
	nodeType := normalizeSubscriptionProtocolType(firstNonEmpty(item.Type, item.Protocol))
	host := firstNonEmpty(item.CustomHost, item.RelayHost, item.Host, item.Server)
	if nodeType == "" || host == "" || item.Port <= 0 {
		return protocol.Node{}, false
	}
	settings := cloneSettingsMap(item.Settings)
	mergeClientConfigLikeSettings(settings, nodeType, item.Cipher, item.Network, item.Path, item.ServiceName, item.TLS, item.SNI, item.ALPN, item.Fingerprint, item.Insecure, item.Flow, 0, 0, "")
	mergeXHTTPSubscriptionSettings(settings, item.Network, item.XHTTPHost, item.Mode, item.Headers, item.Extra, item.XMux, item.DownloadSettings)
	password := firstNonEmpty(item.Password, item.UUID)
	if nodeType == "vless" || nodeType == "vmess" || nodeType == "tuic" {
		password = firstNonEmpty(item.UUID, item.Password)
	}
	if password == "" {
		return protocol.Node{}, false
	}
	name := strings.TrimSpace(item.Name)
	if name == "" {
		name = fmt.Sprintf("%s-%s-%d", nodeType, host, item.Port)
	}
	return protocol.Node{
		ID:         item.ID,
		Name:       name,
		Type:       nodeType,
		Host:       host,
		Port:       item.Port,
		ServerPort: item.ServerPort,
		Rate:       strings.TrimSpace(item.Rate),
		Tags:       normalizeStringSlice(item.Tags),
		Ports:      strings.TrimSpace(item.Ports),
		Settings:   settings,
		Password:   password,
	}, true
}

func settingsFromSubscriptionClientConfig(cfg agentsubscribe.ClientConfig) map[string]any {
	nodeType := normalizeSubscriptionProtocolType(cfg.Protocol)
	settings := map[string]any{}
	mergeClientConfigLikeSettings(settings, nodeType, cfg.Cipher, cfg.Network, cfg.Path, cfg.ServiceName, cfg.TLS, cfg.SNI, cfg.ALPN, cfg.Fingerprint, cfg.Insecure, cfg.Flow, cfg.UpMbps, cfg.DownMbps, cfg.HopPorts)
	mergeXHTTPSubscriptionSettings(settings, cfg.Network, cfg.Host, cfg.Mode, cfg.Headers, cfg.Extra, cfg.XMux, cfg.DownloadSettings)
	if cfg.CongestionControl != "" {
		settings["congestion_control"] = cfg.CongestionControl
	}
	if cfg.HopInterval > 0 {
		settings["hop_interval"] = cfg.HopInterval
	}
	if cfg.Password != "" && cfg.UUID != "" && nodeType == "tuic" {
		settings["password"] = cfg.Password
	}
	return settings
}

func mergeClientConfigLikeSettings(settings map[string]any, nodeType, cipher, network, path, serviceName string, tls bool, sni, alpn, fingerprint string, insecure bool, flow string, upMbps, downMbps int, hopPorts string) {
	if settings == nil {
		return
	}
	if cipher != "" {
		settings["cipher"] = strings.TrimSpace(cipher)
	}
	if network != "" {
		settings["network"] = normalizeSubscriptionNetwork(network)
	}
	if path != "" || serviceName != "" {
		networkSettings, _ := settings["network_settings"].(map[string]any)
		if networkSettings == nil {
			networkSettings = map[string]any{}
		}
		if path != "" {
			networkSettings["path"] = strings.TrimSpace(path)
		}
		if serviceName != "" {
			networkSettings["serviceName"] = strings.TrimSpace(serviceName)
		}
		settings["network_settings"] = networkSettings
	}
	if flow != "" {
		settings["flow"] = strings.TrimSpace(flow)
	}
	if tls || sni != "" || fingerprint != "" || insecure {
		switch nodeType {
		case "vless", "vmess":
			settings["tls"] = tls || sni != "" || fingerprint != "" || insecure
			settings["tls_settings"] = compactSettingsMap(map[string]any{
				"server_name":    strings.TrimSpace(sni),
				"fingerprint":    strings.TrimSpace(fingerprint),
				"allow_insecure": insecure,
			})
		case "hysteria", "hysteria2":
			settings["version"] = "2"
			settings["tls"] = compactSettingsMap(map[string]any{
				"server_name":    strings.TrimSpace(sni),
				"allow_insecure": insecure,
			})
		default:
			settings["server_name"] = strings.TrimSpace(sni)
			settings["allow_insecure"] = insecure
		}
	}
	if alpn != "" {
		settings["alpn"] = strings.TrimSpace(alpn)
	}
	if upMbps > 0 || downMbps > 0 {
		settings["bandwidth"] = compactSettingsMap(map[string]any{
			"up":   strconv.Itoa(upMbps),
			"down": strconv.Itoa(downMbps),
		})
	}
	if hopPorts != "" {
		settings["ports"] = strings.TrimSpace(hopPorts)
	}
}

func mergeXHTTPSubscriptionSettings(settings map[string]any, network, host, mode string, headers map[string]string, extra, xmux, downloadSettings map[string]any) {
	if settings == nil || normalizeSubscriptionNetwork(network) != "xhttp" {
		return
	}
	networkSettings, _ := settings["network_settings"].(map[string]any)
	if networkSettings == nil {
		networkSettings = map[string]any{}
	}
	if host = strings.TrimSpace(host); host != "" {
		networkSettings["host"] = host
	}
	if mode = strings.ToLower(strings.TrimSpace(mode)); mode != "" {
		networkSettings["mode"] = mode
	}
	if mappedHeaders := stringMapToAnyMap(headers); len(mappedHeaders) > 0 {
		networkSettings["headers"] = mappedHeaders
	}
	if extra := cloneNonEmptyAnyMap(extra); len(extra) > 0 {
		networkSettings["extra"] = extra
	}
	if xmux := cloneNonEmptyAnyMap(xmux); len(xmux) > 0 {
		networkSettings["xmux"] = xmux
	}
	if downloadSettings := cloneNonEmptyAnyMap(downloadSettings); len(downloadSettings) > 0 {
		networkSettings["downloadSettings"] = downloadSettings
	}
	if len(networkSettings) > 0 {
		settings["network_settings"] = networkSettings
	}
}

func normalizeSubscriptionNetwork(network string) string {
	normalized := strings.ToLower(strings.TrimSpace(network))
	if normalized == "splithttp" {
		return "xhttp"
	}
	return normalized
}

func stringMapToAnyMap(values map[string]string) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key != "" {
			result[key] = value
		}
	}
	return result
}

func cloneNonEmptyAnyMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func subscriptionClientConfigSecret(cfg agentsubscribe.ClientConfig) string {
	nodeType := normalizeSubscriptionProtocolType(cfg.Protocol)
	switch nodeType {
	case "vless", "vmess", "tuic":
		return strings.TrimSpace(firstNonEmpty(cfg.UUID, cfg.Password))
	default:
		return strings.TrimSpace(firstNonEmpty(cfg.Password, cfg.UUID))
	}
}

func normalizeSubscriptionSourceType(sourceType string) string {
	switch strings.ToLower(strings.TrimSpace(sourceType)) {
	case "self", "self_hosted", "self-hosted":
		return SubscriptionSourceTypeSelfHosted
	case "imported", "imported_subscription", "imported-subscription":
		return SubscriptionSourceTypeImported
	case "custom", "custom_node", "custom-node":
		return SubscriptionSourceTypeCustom
	default:
		return ""
	}
}

func validSubscriptionSourceType(sourceType string) bool {
	switch sourceType {
	case SubscriptionSourceTypeSelfHosted, SubscriptionSourceTypeImported, SubscriptionSourceTypeCustom:
		return true
	default:
		return false
	}
}

func normalizeSubscriptionProtocolType(protocolType string) string {
	switch strings.ToLower(strings.TrimSpace(protocolType)) {
	case "ss":
		return "shadowsocks"
	case "hy2":
		return "hysteria2"
	case "hysteria2", "hysteria", "vless", "vmess", "shadowsocks", "trojan", "tuic", "socks", "http", "anytls", "naive", "mieru":
		return strings.ToLower(strings.TrimSpace(protocolType))
	default:
		return ""
	}
}

func validateSubscriptionSourceURLSyntax(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return ErrBadRequest
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ErrBadRequest
	}
	return nil
}

type httpSubscriptionSourceFetcher struct {
	client *http.Client
}

func (f *httpSubscriptionSourceFetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	if err := validateSubscriptionSourceFetchURL(ctx, rawURL); err != nil {
		return nil, err
	}
	baseClient := f.client
	if baseClient == nil {
		baseClient = &http.Client{Timeout: 15 * time.Second}
	}
	client := *baseClient
	previousCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := validateSubscriptionSourceFetchURL(req.Context(), req.URL.String()); err != nil {
			return err
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("subscription source fetch failed with status %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, maxImportedSubscriptionBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(body) > maxImportedSubscriptionBytes {
		return nil, fmt.Errorf("subscription source content exceeds %d bytes", maxImportedSubscriptionBytes)
	}
	return body, nil
}

func validateSubscriptionSourceFetchURL(ctx context.Context, rawURL string) error {
	if err := validateSubscriptionSourceURLSyntax(rawURL); err != nil {
		return err
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ErrBadRequest
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" || isBlockedSubscriptionHostname(host) {
		return ErrBadRequest
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedSubscriptionIP(ip) {
			return ErrBadRequest
		}
		return nil
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return ErrBadRequest
	}
	for _, addr := range ips {
		if isBlockedSubscriptionIP(addr.IP) {
			return ErrBadRequest
		}
	}
	return nil
}

func isBlockedSubscriptionHostname(host string) bool {
	lower := strings.ToLower(strings.TrimSuffix(host, "."))
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") || strings.HasSuffix(lower, ".local") {
		return true
	}
	return !strings.Contains(lower, ".")
}

func isBlockedSubscriptionIP(ip net.IP) bool {
	return ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()
}

func sanitizeSubscriptionSourceSyncError(err error, rawURL string) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if raw := strings.TrimSpace(rawURL); raw != "" {
		msg = strings.ReplaceAll(msg, raw, "[redacted-url]")
		if parsed, parseErr := url.Parse(raw); parseErr == nil {
			if requestURI := parsed.RequestURI(); requestURI != "" {
				msg = strings.ReplaceAll(msg, requestURI, "[redacted-url]")
			}
			if parsed.User != nil {
				msg = strings.ReplaceAll(msg, parsed.User.String(), "[redacted]")
			}
			for _, values := range parsed.Query() {
				for _, value := range values {
					if strings.TrimSpace(value) != "" {
						msg = strings.ReplaceAll(msg, value, "[redacted]")
					}
				}
			}
		}
	}
	if len([]rune(msg)) > 512 {
		runes := []rune(msg)
		msg = string(runes[:512])
	}
	return msg
}

func mapSubscriptionSourceRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func filterSubscriptionNodes(nodes []protocol.Node, allowedTypes map[string]struct{}, keywords []string, tags []string) []protocol.Node {
	if len(nodes) == 0 {
		return []protocol.Node{}
	}
	filtered := make([]protocol.Node, 0, len(nodes))
	for _, node := range nodes {
		if !typeAllowed(node.Type, allowedTypes) {
			continue
		}
		if len(keywords) > 0 && !matchesNodeKeywords(node, keywords) {
			continue
		}
		if len(tags) > 0 && !matchesAnyTag(node.Tags, tags) {
			continue
		}
		filtered = append(filtered, node)
	}
	return filtered
}

func matchesNodeKeywords(node protocol.Node, keywords []string) bool {
	name := strings.ToLower(node.Name)
	for _, keyword := range keywords {
		if keyword == "" {
			continue
		}
		if strings.Contains(name, keyword) {
			return true
		}
		for _, tag := range node.Tags {
			if strings.Contains(strings.ToLower(tag), keyword) {
				return true
			}
		}
	}
	return false
}

func cloneSettingsMap(settings map[string]any) map[string]any {
	if len(settings) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(settings))
	for key, value := range settings {
		cloned[key] = value
	}
	return cloned
}

func compactSettingsMap(settings map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range settings {
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				result[key] = v
			}
		case bool:
			if v {
				result[key] = v
			}
		default:
			if value != nil {
				result[key] = value
			}
		}
	}
	return result
}

func normalizeStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

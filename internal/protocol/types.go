// 文件路径: internal/protocol/types.go
// 模块说明: 这是 internal 模块里的 types 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package protocol

import (
	"context"
	"encoding/json"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// CDNConfig carries CDN domain replacement parameters for xhttp subscription export.
type CDNConfig struct {
	Domain string // CDN 域名，替换 node.Host
	Path   string // CDN 回源路径，替换 network_settings.path
	Host   string // CDN Host 头
}

// Node represents a normalized server entry usable by protocol builders.
type Node struct {
	ID          int64
	Name        string
	Type        string
	Host        string
	Port        int
	ServerPort  int
	Rate        string
	Tags        []string
	Ports       string
	Settings    map[string]any
	RawSettings json.RawMessage
	Password    string
}

// BuildRequest carries all contextual data for generating subscription payloads.
type BuildRequest struct {
	Context       context.Context
	User          *repository.User
	Nodes         []Node
	Flag          string
	UserAgent     string
	ClientName    string
	ClientVersion string
	Host          string
	AppName       string
	AppURL        string
	SubscribeURL  string
	Templates     map[string]string
	UserTraffic   *UserTrafficInfo // 用户流量配额和使用信息
	Lang          string
	I18n          *i18n.Manager
	CDN           *CDNConfig // CDN 域名替换配置，仅对 xhttp VLESS 节点生效
}

// UserTrafficInfo contains user traffic quota and usage for subscription headers.
type UserTrafficInfo struct {
	Upload    int64 // 已上传流量 (bytes)
	Download  int64 // 已下载流量 (bytes)
	Total     int64 // 总流量配额 (bytes)
	ExpiredAt int64 // 过期时间 (Unix timestamp)
}

// Result captures the serialized payload emitted by a protocol builder.
type Result struct {
	Payload     []byte
	ContentType string
	Headers     map[string]string
}

// Builder defines the contract implemented by each protocol renderer.
type Builder interface {
	Flags() []string
	Build(req BuildRequest) (*Result, error)
}

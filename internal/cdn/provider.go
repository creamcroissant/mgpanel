// Package cdn 定义 CDN Provider 抽象层，支持多厂商 CDN 接入。
package cdn

import (
	"context"
	"errors"
)

// 通用错误变量
var (
	// ErrNotSupported 表示当前 provider 不支持该操作。
	ErrNotSupported = errors.New("cdn: operation not supported")
)

// ErrProviderNotImplemented 表示请求的 provider 未实现。
type ErrProviderNotImplemented struct {
	Name string
}

func (e *ErrProviderNotImplemented) Error() string {
	return "cdn: provider not implemented: " + e.Name
}

// ErrDistributionNotFound 表示找不到指定的 distribution。
type ErrDistributionNotFound struct {
	ID string
}

func (e *ErrDistributionNotFound) Error() string {
	return "cdn: distribution not found: " + e.ID
}

// ErrProviderConfig 表示 provider 配置错误。
type ErrProviderConfig struct {
	Provider string
	Message  string
}

func (e *ErrProviderConfig) Error() string {
	return "cdn: provider " + e.Provider + " config error: " + e.Message
}

// DistributionResult 表示同步 distribution 后的结果。
type DistributionResult struct {
	ID         string
	DomainName string
	Status     string
}

// ProviderStatus 表示 distribution 的状态。
type ProviderStatus struct {
	Status     string
	DomainName string
	Enabled    bool
}

// TODO: Migrate to generics (Go 1.18+) for type-safe Site/Edge parameters.
// Current interface{} pattern requires runtime type assertions.
//
// Provider is the generic CDN vendor interface. Each provider implements all methods.
//
// Note on type safety: methods accept interface{} parameters because the
// concrete types for site configs, edge lists, distribution results, and
// DNS results differ per provider implementation. Each implementation MUST
// type-assert these parameters to the expected concrete types before use.
// Provider 是 CDN 厂商的通用接口，每种 provider 需实现全部方法。
type Provider interface {
	// Name 返回 provider 名称标识。
	Name() string

	// SyncDistribution 创建或同步 distribution。
	// site   — 站点配置，类型由具体 provider 定义。
	// edges  — 边缘节点列表，类型由具体 provider 定义。
	SyncDistribution(ctx context.Context, site interface{}, edges interface{}) (interface{}, error)

	// DeleteDistribution 删除指定 distribution。
	DeleteDistribution(ctx context.Context, id string) error

	// GetDistributionStatus 查询 distribution 状态。
	GetDistributionStatus(ctx context.Context, id string) (interface{}, error)

	// SyncDNS 同步 DNS 记录。
	SyncDNS(ctx context.Context, domain string, targets []string, proxied bool) (interface{}, error)

	// SyncCacheRules 更新 Cache 规则。
	SyncCacheRules(ctx context.Context, distID string, rules interface{}) error

	// Invalidate 刷新 CDN 缓存。
	Invalidate(ctx context.Context, distID string, paths []string) error
}

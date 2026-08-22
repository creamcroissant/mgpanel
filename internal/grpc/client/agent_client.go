package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// Config 保存 Panel -> Agent 调用的 gRPC 客户端配置。
type Config struct {
	Address   string
	Token     string
	TLS       *TLSConfig
	Keepalive *KeepaliveConfig
	Timeout   TimeoutConfig
}

// TimeoutConfig 保存 gRPC 调用的超时设置。
type TimeoutConfig struct {
	Default time.Duration
	Connect time.Duration
	Install time.Duration
}

// TLSConfig 保存 TLS 设置。
type TLSConfig struct {
	Enabled            bool
	CertFile           string
	KeyFile            string
	CAFile             string
	InsecureSkipVerify bool
}

// KeepaliveConfig 保存 keepalive 设置。
type KeepaliveConfig struct {
	Time    time.Duration
	Timeout time.Duration
}

// AgentClient 封装与 Agent 的 gRPC 连接。
// 该 legacy client 已退役，保留该类型仅为了让过渡期测试和注入点继续编译。
type AgentClient struct {
	conn   *grpc.ClientConn
	client agentv1.AgentServiceClient
	config Config
}

// NewAgentClient 创建新的 Agent gRPC 客户端。
func NewAgentClient(ctx context.Context, cfg Config) (*AgentClient, error) {
	if cfg.Keepalive == nil {
		cfg.Keepalive = &KeepaliveConfig{Time: 30 * time.Second, Timeout: 10 * time.Second}
	}
	if cfg.Timeout.Default == 0 {
		cfg.Timeout.Default = 10 * time.Second
	}
	if cfg.Timeout.Connect == 0 {
		cfg.Timeout.Connect = 5 * time.Second
	}
	if cfg.Timeout.Install == 0 {
		cfg.Timeout.Install = 5 * time.Minute
	}

	opts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                cfg.Keepalive.Time,
			Timeout:             cfg.Keepalive.Timeout,
			PermitWithoutStream: true,
		}),
	}
	if cfg.TLS != nil && cfg.TLS.Enabled {
		tlsCfg, err := buildTLSConfig(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("build TLS config: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("create agent gRPC client: %w", err)
	}
	return &AgentClient{conn: conn, client: agentv1.NewAgentServiceClient(conn), config: cfg}, nil
}

func buildTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}
	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("CA certificate parse failed / CA 证书解析失败")
		}
		tlsCfg.RootCAs = caCertPool
	}
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}

func (c *AgentClient) withAuth(ctx context.Context) context.Context {
	if c == nil || c.config.Token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.config.Token)
}

// Close 关闭 gRPC 连接。
func (c *AgentClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *AgentClient) call(ctx context.Context, fn func(context.Context) error) error {
	return c.callWithTimeout(ctx, c.config.Timeout.Default, fn)
}

func (c *AgentClient) callWithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	callCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	callCtx = c.withAuth(callCtx)
	return fn(callCtx)
}

func callUnary[T any](ctx context.Context, c *AgentClient, fn func(context.Context) (*T, error)) (*T, error) {
	return callUnaryWithTimeout(ctx, c, c.config.Timeout.Default, fn)
}

func callUnaryWithTimeout[T any](ctx context.Context, c *AgentClient, timeout time.Duration, fn func(context.Context) (*T, error)) (*T, error) {
	var resp *T
	err := c.callWithTimeout(ctx, timeout, func(inner context.Context) error {
		var err error
		resp, err = fn(inner)
		return err
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Legacy synchronous RPCs are retired. These methods intentionally return explicit errors
// if still invoked so remaining callers can be migrated without silent fallback behavior.
func (c *AgentClient) GetCores(ctx context.Context) (*agentv1.GetCoresResponse, error) {
	return nil, fmt.Errorf("legacy panel->agent GetCores RPC is retired")
}

func (c *AgentClient) SwitchCore(ctx context.Context, req *agentv1.SwitchCoreRequest) (*agentv1.SwitchCoreResponse, error) {
	return nil, fmt.Errorf("legacy panel->agent SwitchCore RPC is retired")
}

func (c *AgentClient) InstallCore(ctx context.Context, req *agentv1.InstallCoreRequest) (*agentv1.InstallCoreResponse, error) {
	return nil, fmt.Errorf("legacy panel->agent InstallCore RPC is retired")
}

func (c *AgentClient) ReportAccessLogs(ctx context.Context, report *agentv1.AccessLogReport) (*agentv1.AccessLogResponse, error) {
	return callUnary(ctx, c, func(ctx context.Context) (*agentv1.AccessLogResponse, error) {
		return c.client.ReportAccessLogs(ctx, report)
	})
}

func (c *AgentClient) Client() agentv1.AgentServiceClient {
	return c.client
}

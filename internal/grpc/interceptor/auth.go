package interceptor

import (
	"context"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ContextKey 是 context key 的类型。
type ContextKey string

const (
	// AgentHostKey 是已认证 Agent 的上下文 key。
	AgentHostKey ContextKey = "agent_host"
)

// AuthInterceptor 提供 gRPC 鉴权能力。
type AuthInterceptor struct {
	agentHostService service.AgentHostService
}

// NewAuthInterceptor 创建鉴权拦截器。
func NewAuthInterceptor(agentHostService service.AgentHostService) *AuthInterceptor {
	return &AuthInterceptor{
		agentHostService: agentHostService,
	}
}

// Unary 返回用于鉴权的 unary 拦截器。
func (i *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		newCtx, err := i.authenticate(ctx)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// Stream 返回用于鉴权的 stream 拦截器。
func (i *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, err := i.authenticate(ss.Context())
		if err != nil {
			return err
		}
		return handler(srv, &authenticatedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		})
	}
}

// authenticate 从 metadata 解析 token 并完成校验。
func (i *AuthInterceptor) authenticate(ctx context.Context) (context.Context, error) {
	token, err := extractToken(ctx)
	if err != nil {
		return nil, err
	}

	// 通过 AgentHostService 校验 token
	agentHost, err := i.agentHostService.GetByToken(ctx, token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	// 将 Agent 信息写入上下文
	newCtx := context.WithValue(ctx, AgentHostKey, agentHost)
	return newCtx, nil
}

// extractToken 从 gRPC metadata 读取 bearer token。
func extractToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// 解析 "Bearer <token>"
	auth := values[0]
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", status.Error(codes.Unauthenticated, "invalid authorization format")
	}

	return strings.TrimPrefix(auth, "Bearer "), nil
}

// GetAgentHostFromContext 从上下文获取已认证的 Agent。
func GetAgentHostFromContext(ctx context.Context) (*repository.AgentHost, bool) {
	agentHost, ok := ctx.Value(AgentHostKey).(*repository.AgentHost)
	return agentHost, ok
}

// authenticatedServerStream 用认证后的上下文包装 ServerStream。
type authenticatedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedServerStream) Context() context.Context {
	return s.ctx
}

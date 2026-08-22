package grpc

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthInterceptor provides token authentication for Agent gRPC server.
type AuthInterceptor struct {
	token string
}

// NewAuthInterceptor creates a new auth interceptor.
func NewAuthInterceptor(token string) *AuthInterceptor {
	return &AuthInterceptor{token: token}
}

// Unary returns a unary server interceptor for authentication.
func (i *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := i.authenticate(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// Stream returns a stream server interceptor for authentication.
func (i *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := i.authenticate(ss.Context()); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

func (i *AuthInterceptor) authenticate(ctx context.Context) error {
	if i == nil {
		return status.Error(codes.Unauthenticated, "missing auth interceptor")
	}
	if i.token == "" {
		return status.Error(codes.Unauthenticated, "missing auth token")
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization header")
	}
	auth := values[0]
	if !strings.HasPrefix(auth, "Bearer ") {
		return status.Error(codes.Unauthenticated, "invalid authorization format")
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, "Bearer ")), []byte(i.token)) != 1 {
		return status.Error(codes.Unauthenticated, "invalid token")
	}
	return nil
}

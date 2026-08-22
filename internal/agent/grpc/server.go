package grpc

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

// Server wraps a gRPC server on the Agent side.
type Server struct {
	server  *grpc.Server
	logger  *slog.Logger
	address string
}

// Config holds gRPC server configuration.
type Config struct {
	Address string
	TLS     *TLSConfig
}

// TLSConfig holds TLS settings for the server.
type TLSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
}

// NewServer creates a new gRPC server.
func NewServer(cfg Config, handler *Handler, authInterceptor *AuthInterceptor, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if handler == nil {
		return nil, fmt.Errorf("handler is required")
	}
	if authInterceptor == nil {
		return nil, fmt.Errorf("auth interceptor is required")
	}

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(authInterceptor.Unary()),
		grpc.ChainStreamInterceptor(authInterceptor.Stream()),
	}

	if cfg.TLS != nil && cfg.TLS.Enabled {
		cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load TLS certificate: %w", err)
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}

	server := grpc.NewServer(opts...)
	agentv1.RegisterAgentServiceServer(server, handler)

	return &Server{
		server:  server,
		logger:  logger,
		address: cfg.Address,
	}, nil
}

// Start starts the gRPC server.
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.address, err)
	}

	s.logger.Info("Agent gRPC server starting", "address", s.address)
	return s.server.Serve(lis)
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop() {
	s.logger.Info("Agent gRPC server stopping")
	s.server.GracefulStop()
}

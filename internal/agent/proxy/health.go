package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const (
	defaultHealthTimeout  = 10 * time.Second
	defaultHealthInterval = 500 * time.Millisecond
)

// HealthChecker performs TCP health checks on ports.
type HealthChecker struct {
	timeout  time.Duration
	interval time.Duration
}

// NewHealthChecker creates a HealthChecker with optional custom timeouts.
func NewHealthChecker(timeout, interval time.Duration) *HealthChecker {
	if timeout <= 0 {
		timeout = defaultHealthTimeout
	}
	if interval <= 0 {
		interval = defaultHealthInterval
	}
	return &HealthChecker{timeout: timeout, interval: interval}
}

// CheckPorts waits until all ports are reachable or timeout/cancel occurs.
func (h *HealthChecker) CheckPorts(ctx context.Context, ports []int) error {
	if len(ports) == 0 {
		return fmt.Errorf("no ports to check")
	}
	deadline := time.Now().Add(h.timeout)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("health check timeout after %s", h.timeout)
		}

		if err := h.checkAllOnce(ctx, ports); err == nil {
			return nil
		}
		timer := time.NewTimer(h.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (h *HealthChecker) checkAllOnce(ctx context.Context, ports []int) error {
	for _, port := range ports {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if port <= 0 {
			return fmt.Errorf("invalid port: %d", port)
		}
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err != nil {
			if isAddrNotAvailable(err) {
				addr6 := fmt.Sprintf("[::1]:%d", port)
				conn6, err6 := net.DialTimeout("tcp", addr6, 500*time.Millisecond)
				if err6 != nil {
					return err6
				}
				_ = conn6.Close()
				continue
			}
			return err
		}
		_ = conn.Close()
	}
	return nil
}

func isAddrNotAvailable(err error) bool {
	if err == nil {
		return false
	}
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		return strings.Contains(syscallErr.Err.Error(), "cannot assign requested address") ||
			strings.Contains(syscallErr.Err.Error(), "address not available")
	}
	return strings.Contains(strings.ToLower(err.Error()), "connect: cannot assign requested address") ||
		strings.Contains(strings.ToLower(err.Error()), "address not available")
}

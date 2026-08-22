package proxy

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// ConntrackFlusher handles conntrack cleanup for dual-stack ports.
type ConntrackFlusher struct {
	conntrackBin string
	logger       *slog.Logger
}

// NewConntrackFlusher creates a ConntrackFlusher instance.
func NewConntrackFlusher(bin string, logger *slog.Logger) *ConntrackFlusher {
	if strings.TrimSpace(bin) == "" {
		bin = "conntrack"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ConntrackFlusher{conntrackBin: bin, logger: logger}
}

// FlushDualStack clears conntrack entries for the given protocol and port.
func (f *ConntrackFlusher) FlushDualStack(ctx context.Context, port int, proto string) error {
	if port <= 0 {
		return fmt.Errorf("invalid port: %d", port)
	}

	protocol := strings.ToLower(strings.TrimSpace(proto))
	if protocol == "" {
		protocol = "tcp"
	}
	if protocol != "tcp" && protocol != "udp" {
		return fmt.Errorf("unsupported protocol: %s", proto)
	}

	families := []string{"ipv4", "ipv6"}
	for _, family := range families {
		if err := f.runConntrack(ctx, family, protocol, port); err != nil {
			if isNoConntrackEntries(err) {
				continue
			}
			return err
		}
	}
	return nil
}

// FlushAllProtocols clears conntrack entries for both TCP and UDP.
func (f *ConntrackFlusher) FlushAllProtocols(ctx context.Context, port int) error {
	if err := f.FlushDualStack(ctx, port, "tcp"); err != nil {
		return err
	}
	if err := f.FlushDualStack(ctx, port, "udp"); err != nil {
		return err
	}
	return nil
}

func (f *ConntrackFlusher) runConntrack(ctx context.Context, family, protocol string, port int) error {
	args := []string{"-f", family, "-D", "-p", protocol, "--dport", fmt.Sprintf("%d", port)}
	cmd := exec.CommandContext(ctx, f.conntrackBin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return f.wrapConntrackError(err, output, stderr.String())
	}
	// conntrack -D 成功时 stdout 可能有已删除条目的文本，这不算是错误
	return nil
}

func (f *ConntrackFlusher) wrapConntrackError(err error, stdout []byte, stderr string) error {
	message := strings.TrimSpace(stderr)
	if message == "" {
		message = strings.TrimSpace(string(stdout))
	}
	if message == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, message)
}

func isNoConntrackEntries(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "0 flow entries") || strings.Contains(msg, "no entries")
}

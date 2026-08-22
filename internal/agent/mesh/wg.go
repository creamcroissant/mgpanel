package mesh

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// WGExecutor wraps the wg utility for configuring WireGuard interfaces.
type WGExecutor struct {
	wgBin string
	iface string
}

// NewWGExecutor creates a new WGExecutor with the given wg binary path and interface name.
func NewWGExecutor(wgBin, iface string) *WGExecutor {
	return &WGExecutor{wgBin: wgBin, iface: iface}
}

// SetConfig applies a full WireGuard configuration to the interface via wg setconf.
func (w *WGExecutor) SetConfig(ctx context.Context, configContent string) error {
	cmd := exec.CommandContext(ctx, w.wgBin, "setconf", w.iface, "/dev/stdin")
	cmd.Stdin = bytes.NewReader([]byte(configContent))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg setconf: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// AddPeer adds a peer to the WireGuard interface.
func (w *WGExecutor) AddPeer(ctx context.Context, pubkey, endpoint string, allowedIPs []string, keepalive int) error {
	args := []string{"set", w.iface, "peer", pubkey}
	if endpoint != "" {
		args = append(args, "endpoint", endpoint)
	}
	if len(allowedIPs) > 0 {
		args = append(args, "allowed-ips", strings.Join(allowedIPs, ","))
	}
	if keepalive > 0 {
		args = append(args, "persistent-keepalive", fmt.Sprintf("%d", keepalive))
	}
	cmd := exec.CommandContext(ctx, w.wgBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg set peer: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemovePeer removes a peer from the WireGuard interface.
func (w *WGExecutor) RemovePeer(ctx context.Context, pubkey string) error {
	cmd := exec.CommandContext(ctx, w.wgBin, "set", w.iface, "peer", pubkey, "remove")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg set peer remove: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

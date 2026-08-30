package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

// PortMapping represents the mapping between external and internal ports.
type PortMapping struct {
	ExternalPort int
	InternalPort int
	Protocol     string
	Family       string
}

// StateRebuilder reconstructs port mapping state from nftables rules.
type StateRebuilder struct {
	nftBin    string
	tableName string
	logger    *slog.Logger
}

// NewStateRebuilder creates a StateRebuilder instance.
func NewStateRebuilder(nftBin, tableName string, logger *slog.Logger) *StateRebuilder {
	if strings.TrimSpace(nftBin) == "" {
		nftBin = "/usr/sbin/nft"
	}
	if strings.TrimSpace(tableName) == "" {
		tableName = "xboard_proxy"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &StateRebuilder{nftBin: nftBin, tableName: tableName, logger: logger}
}

// RebuildState parses nftables JSON output to recover port mappings.
func (r *StateRebuilder) RebuildState(ctx context.Context) ([]PortMapping, error) {
	output, err := r.runNft(ctx, []string{"-j", "list", "table", "inet", r.tableName})
	if err != nil {
		if isNoSuchTable(err) {
			return nil, nil
		}
		return nil, err
	}

	var root nftJSON
	if err := json.Unmarshal(output, &root); err != nil {
		return nil, fmt.Errorf("parse nft json: %w", err)
	}

	mappings := make([]PortMapping, 0)
	for _, entry := range root.Nftables {
		if entry.Rule == nil {
			continue
		}
		rule := entry.Rule
		if rule.Table != r.tableName || rule.Chain != "prerouting" {
			continue
		}
		mapping, ok := parseDNATRule(rule)
		if !ok {
			continue
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}

func (r *StateRebuilder) runNft(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, r.nftBin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, r.wrapNftError(err, stderr.String())
	}
	return output, nil
}

func (r *StateRebuilder) wrapNftError(err error, stderr string) error {
	if strings.TrimSpace(stderr) == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr))
}

func isNoSuchTable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist") {
		return true
	}
	return strings.Contains(msg, "no such file or directory") && strings.Contains(msg, "process rule")
}

// GetOccupiedInternalPorts returns a set of internal ports currently in use.
func (r *StateRebuilder) GetOccupiedInternalPorts(ctx context.Context) (map[int]bool, error) {
	mappings, err := r.RebuildState(ctx)
	if err != nil {
		return nil, err
	}
	occupied := make(map[int]bool, len(mappings))
	for _, mapping := range mappings {
		if mapping.InternalPort > 0 {
			occupied[mapping.InternalPort] = true
		}
	}
	return occupied, nil
}

// nftables JSON structs for the parts we need.
type nftJSON struct {
	Nftables []nftEntry `json:"nftables"`
}

type nftEntry struct {
	Rule *nftRule `json:"rule,omitempty"`
}

type nftRule struct {
	Family string      `json:"family"`
	Table  string      `json:"table"`
	Chain  string      `json:"chain"`
	Expr   []nftExpr   `json:"expr"`
	Handle interface{} `json:"handle,omitempty"`
}

type nftExpr map[string]any

func parseDNATRule(rule *nftRule) (PortMapping, bool) {
	var mapping PortMapping
	mapping.Family = rule.Family

	var dport int
	var proto string
	var dnatPort int

	for _, expr := range rule.Expr {
		match, ok := expr["match"].(map[string]any)
		if ok {
			left, ok := match["left"].(map[string]any)
			if !ok {
				continue
			}
			if payload, ok := left["payload"].(map[string]any); ok {
				if field, ok := payload["field"].(string); ok {
					if field == "dport" {
						if right := match["right"]; right != nil {
							if port, ok := asInt(right); ok {
								dport = port
							}
						}
					}
				}
				if protoValue, ok := payload["protocol"].(string); ok {
					proto = protoValue
				}
			}
			if meta, ok := left["meta"].(map[string]any); ok {
				if key, ok := meta["key"].(string); ok {
					right := match["right"]
					switch key {
					case "l4proto":
						if protocol, ok := right.(string); ok {
							proto = protocol
						}
					case "nfproto":
						if family, ok := right.(string); ok {
							mapping.Family = family
						}
					}
				}
			}
		}

		nat, ok := expr["dnat"].(map[string]any)
		if !ok {
			continue
		}
		if port := nat["port"]; port != nil {
			if p, ok := asInt(port); ok {
				dnatPort = p
			}
		}
	}

	if dport == 0 || dnatPort == 0 {
		return PortMapping{}, false
	}
	if proto == "" {
		proto = "tcp"
	}
	mapping.ExternalPort = dport
	mapping.InternalPort = dnatPort
	mapping.Protocol = proto
	return mapping, true
}

func asInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
		if f, err := v.Float64(); err == nil {
			return int(f), true
		}
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}
	return 0, false
}

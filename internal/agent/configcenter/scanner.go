package configcenter

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/config"
	"github.com/creamcroissant/mgpanel/internal/agent/protocol/parser"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

const (
	inventorySourceLegacy  = "legacy"
	inventorySourceManaged = "managed"
	inventorySourceMerged  = "merged"

	inventoryParseStatusOK         = "ok"
	inventoryParseStatusParseError = "parse_error"
)

// AgentInventoryScanner scans local config files and builds inventory/index payloads for status report.
type AgentInventoryScanner struct {
	legacyDir       string
	managedDir      string
	mergeOutputFile string
	registry        *parser.Registry
	now             func() int64
}

// NewAgentInventoryScanner creates scanner with normalized protocol paths.
func NewAgentInventoryScanner(cfg config.ProtocolConfig, registry *parser.Registry) (*AgentInventoryScanner, error) {
	legacyDirAbs, managedDirAbs, mergeOutputFile, err := resolveProtocolPaths(cfg)
	if err != nil {
		return nil, err
	}

	if registry == nil {
		registry = parser.NewRegistry()
	}

	return &AgentInventoryScanner{
		legacyDir:       legacyDirAbs,
		managedDir:      managedDirAbs,
		mergeOutputFile: mergeOutputFile,
		registry:        registry,
		now:             func() int64 { return time.Now().Unix() },
	}, nil
}

// Scan scans all sources and returns deterministic inventory and inbound index entries.
func (s *AgentInventoryScanner) Scan() ([]*agentv1.ConfigInventoryEntry, []*agentv1.InboundIndexEntry, error) {
	reportedAt := int64(0)
	if s.now != nil {
		reportedAt = s.now()
	}
	if reportedAt <= 0 {
		reportedAt = time.Now().Unix()
	}

	legacyInv, legacyIdx, err := s.scanSourceDir(inventorySourceLegacy, s.legacyDir, reportedAt, false)
	if err != nil {
		return nil, nil, fmt.Errorf("scan legacy source: %w", err)
	}
	managedInv, managedIdx, err := s.scanSourceDir(inventorySourceManaged, s.managedDir, reportedAt, true)
	if err != nil {
		return nil, nil, fmt.Errorf("scan managed source: %w", err)
	}
	mergedInv, mergedIdx, err := s.scanMergedFile(reportedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("scan merged source: %w", err)
	}

	inventories := make([]*agentv1.ConfigInventoryEntry, 0, len(legacyInv)+len(managedInv)+len(mergedInv))
	inventories = append(inventories, legacyInv...)
	inventories = append(inventories, managedInv...)
	inventories = append(inventories, mergedInv...)

	indexes := make([]*agentv1.InboundIndexEntry, 0, len(legacyIdx)+len(managedIdx)+len(mergedIdx))
	indexes = append(indexes, legacyIdx...)
	indexes = append(indexes, managedIdx...)
	indexes = append(indexes, mergedIdx...)

	sortInventoryEntries(inventories)
	sortInboundIndexEntries(indexes)
	return inventories, indexes, nil
}

func (s *AgentInventoryScanner) scanSourceDir(source, dir string, reportedAt int64, excludeMergeOutput bool) ([]*agentv1.ConfigInventoryEntry, []*agentv1.InboundIndexEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	filenames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry == nil || entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" || !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		if excludeMergeOutput && name == s.mergeOutputFile {
			continue
		}
		filenames = append(filenames, name)
	}
	sort.Strings(filenames)

	inventories := make([]*agentv1.ConfigInventoryEntry, 0, len(filenames))
	indexes := make([]*agentv1.InboundIndexEntry, 0)
	for _, filename := range filenames {
		path := filepath.Join(dir, filename)
		inventoryEntry, indexEntries, scanErr := s.scanFile(source, filename, path, reportedAt)
		if scanErr != nil {
			if inventoryEntry != nil {
				inventories = append(inventories, inventoryEntry)
			}
			continue
		}
		if inventoryEntry != nil {
			inventories = append(inventories, inventoryEntry)
		}
		if len(indexEntries) > 0 {
			indexes = append(indexes, indexEntries...)
		}
	}

	return inventories, indexes, nil
}

func (s *AgentInventoryScanner) scanMergedFile(reportedAt int64) ([]*agentv1.ConfigInventoryEntry, []*agentv1.InboundIndexEntry, error) {
	path := filepath.Join(s.managedDir, s.mergeOutputFile)
	inventoryEntry, indexEntries, scanErr := s.scanFile(inventorySourceMerged, s.mergeOutputFile, path, reportedAt)
	if scanErr != nil {
		if os.IsNotExist(scanErr) {
			return nil, nil, nil
		}
		return nil, nil, scanErr
	}
	if inventoryEntry == nil {
		return nil, nil, nil
	}
	return []*agentv1.ConfigInventoryEntry{inventoryEntry}, indexEntries, nil
}

func (s *AgentInventoryScanner) scanFile(source, filename, path string, reportedAt int64) (*agentv1.ConfigInventoryEntry, []*agentv1.InboundIndexEntry, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	hashSum := md5.Sum(content)
	contentHash := hex.EncodeToString(hashSum[:])

	allDetails, parseErr := s.registry.Parse(filename, content)

	parseStatus := inventoryParseStatusOK
	parseError := ""
	if parseErr != nil {
		allDetails = nil
		parseStatus = inventoryParseStatusParseError
		parseError = strings.TrimSpace(parseErr.Error())
		if parseError == "" {
			parseError = "parse config failed"
		}
	}

	coreType := inferCoreType(allDetails, content, source, filename)
	inventoryEntry := &agentv1.ConfigInventoryEntry{
		Source:      source,
		Filename:    filename,
		CoreType:    coreType,
		ContentHash: contentHash,
		ParseStatus: parseStatus,
		ParseError:  parseError,
		LastSeenAt:  reportedAt,
	}

	if parseErr != nil {
		return inventoryEntry, nil, nil
	}

	indexEntries := buildInboundIndexEntries(source, filename, allDetails, reportedAt)
	return inventoryEntry, dedupeInboundIndexEntries(indexEntries), nil
}

func inferCoreType(details []parser.ProtocolDetails, content []byte, source, filename string) string {
	scores := map[string]int{}
	for _, detail := range details {
		coreType := normalizeCoreType(detail.CoreType)
		if coreType == "" {
			continue
		}
		scores[coreType] += 1
		if strings.TrimSpace(detail.Tag) != "" {
			scores[coreType] += 1
		}
		if strings.TrimSpace(detail.Protocol) != "" {
			scores[coreType] += 1
		}
		if detail.Port > 0 && detail.Port <= 65535 {
			scores[coreType] += 1
		}
	}
	if len(scores) > 0 {
		winner := ""
		bestScore := -1
		for coreType, score := range scores {
			if score > bestScore {
				winner = coreType
				bestScore = score
				continue
			}
			if score == bestScore && winner != "" {
				if source == inventorySourceMerged {
					if coreType == "xray" {
						winner = coreType
					}
					continue
				}
				if coreType < winner {
					winner = coreType
				}
			}
		}
		if winner != "" {
			return winner
		}
	}

	var doc map[string]any
	if err := json.Unmarshal(content, &doc); err == nil {
		if raw, ok := doc["inbounds"]; ok {
			if inbounds, ok := raw.([]any); ok {
				for _, inbound := range inbounds {
					obj, ok := inbound.(map[string]any)
					if !ok {
						continue
					}
					if _, hasType := obj["type"]; hasType {
						return "sing-box"
					}
					if _, hasProtocol := obj["protocol"]; hasProtocol {
						return "xray"
					}
				}
			}
		}
	}

	if source == inventorySourceMerged {
		return "xray"
	}

	name := strings.ToLower(strings.TrimSpace(filename))
	if strings.Contains(name, "xray") {
		return "xray"
	}
	if strings.Contains(name, "sing") {
		return "sing-box"
	}
	return "sing-box"
}

func buildInboundIndexEntries(source, filename string, details []parser.ProtocolDetails, reportedAt int64) []*agentv1.InboundIndexEntry {
	result := make([]*agentv1.InboundIndexEntry, 0, len(details))
	for _, detail := range details {
		tag := strings.TrimSpace(detail.Tag)
		if tag == "" {
			continue
		}
		protocol := strings.ToLower(strings.TrimSpace(detail.Protocol))
		if protocol == "" {
			continue
		}
		listen := normalizeListen(detail.Listen)
		if listen == "" {
			continue
		}
		if detail.Port <= 0 || detail.Port > 65535 {
			continue
		}
		coreType := normalizeCoreType(detail.CoreType)
		if coreType == "" {
			continue
		}
		result = append(result, &agentv1.InboundIndexEntry{
			Source:     source,
			Filename:   filename,
			CoreType:   coreType,
			Tag:        tag,
			Protocol:   protocol,
			Listen:     listen,
			Port:       int32(detail.Port),
			Tls:        normalizeStructJSON(detail.TLS),
			Transport:  normalizeStructJSON(detail.Transport),
			Multiplex:  normalizeStructJSON(detail.Multiplex),
			LastSeenAt: reportedAt,
		})
	}
	return result
}

func normalizeStructJSON(value any) string {
	if value == nil {
		return "{}"
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	var obj map[string]any
	if err := json.Unmarshal(encoded, &obj); err != nil {
		return "{}"
	}
	if obj == nil {
		obj = map[string]any{}
	}
	normalized, err := json.Marshal(obj)
	if err != nil {
		return "{}"
	}
	return string(normalized)
}

func normalizeListen(listen string) string {
	trimmed := strings.TrimSpace(listen)
	if trimmed == "" {
		return ""
	}
	if ip := net.ParseIP(trimmed); ip != nil {
		return ip.String()
	}
	return strings.ToLower(trimmed)
}

func dedupeInboundIndexEntries(entries []*agentv1.InboundIndexEntry) []*agentv1.InboundIndexEntry {
	if len(entries) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(entries))
	result := make([]*agentv1.InboundIndexEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		key := strings.Join([]string{
			strings.TrimSpace(entry.GetSource()),
			strings.TrimSpace(entry.GetFilename()),
			strings.TrimSpace(entry.GetCoreType()),
			strings.TrimSpace(entry.GetTag()),
		}, "|")
		if key == "|||" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, entry)
	}
	return result
}

func sortInventoryEntries(entries []*agentv1.ConfigInventoryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		if left.GetSource() == right.GetSource() {
			if left.GetFilename() == right.GetFilename() {
				return left.GetCoreType() < right.GetCoreType()
			}
			return left.GetFilename() < right.GetFilename()
		}
		return left.GetSource() < right.GetSource()
	})
}

func sortInboundIndexEntries(entries []*agentv1.InboundIndexEntry) {
	sort.Slice(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		if left.GetSource() == right.GetSource() {
			if left.GetFilename() == right.GetFilename() {
				if left.GetTag() == right.GetTag() {
					return left.GetCoreType() < right.GetCoreType()
				}
				return left.GetTag() < right.GetTag()
			}
			return left.GetFilename() < right.GetFilename()
		}
		return left.GetSource() < right.GetSource()
	})
}

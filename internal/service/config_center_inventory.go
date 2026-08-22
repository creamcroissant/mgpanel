package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

var (
	ErrInventoryIngestInvalidRequest = errors.New("service: invalid inventory ingest request / inventory 入库请求无效")
	ErrInventoryIngestNotConfigured  = errors.New("service: inventory ingest service not configured / inventory 入库服务未配置")
)

const (
	inventorySourceLegacy  = "legacy"
	inventorySourceManaged = "managed"
	inventorySourceMerged  = "merged"

	inventoryParseStatusOK         = "ok"
	inventoryParseStatusParseError = "parse_error"
)

// InventoryIngestService ingests agent-reported applied inventory and inbound index into repositories.
type InventoryIngestService interface {
	IngestReport(ctx context.Context, req IngestInventoryReportRequest) error
}

// IngestInventoryReportRequest carries one status report's inventory/index payload.
type IngestInventoryReportRequest struct {
	AgentHostID  int64
	ReportedAt   int64
	Inventory    []InventoryReportEntry
	InboundIndex []InboundIndexReportEntry
}

// InventoryReportEntry is one file-level applied inventory item.
type InventoryReportEntry struct {
	Source      string
	Filename    string
	CoreType    string
	ContentHash string
	ParseStatus string
	ParseError  string
	LastSeenAt  int64
}

// InboundIndexReportEntry is one semantic inbound index item.
type InboundIndexReportEntry struct {
	Source     string
	Filename   string
	CoreType   string
	Tag        string
	Protocol   string
	Listen     string
	Port       int
	TLS        json.RawMessage
	Transport  json.RawMessage
	Multiplex  json.RawMessage
	LastSeenAt int64
}

type inventoryIngestService struct {
	inventories    repository.AgentConfigInventoryRepository
	inboundIndexes repository.InboundIndexRepository
}

// NewInventoryIngestService creates an InventoryIngestService.
func NewInventoryIngestService(
	inventories repository.AgentConfigInventoryRepository,
	inboundIndexes repository.InboundIndexRepository,
) InventoryIngestService {
	return &inventoryIngestService{
		inventories:    inventories,
		inboundIndexes: inboundIndexes,
	}
}

func (s *inventoryIngestService) IngestReport(ctx context.Context, req IngestInventoryReportRequest) error {
	if s == nil || s.inventories == nil || s.inboundIndexes == nil {
		return ErrInventoryIngestNotConfigured
	}
	if req.AgentHostID <= 0 {
		return fmt.Errorf("%w (agent_host_id is required / 不能为空)", ErrInventoryIngestInvalidRequest)
	}
	if len(req.Inventory) == 0 && len(req.InboundIndex) == 0 {
		return nil
	}

	reportedAt := req.ReportedAt
	if reportedAt <= 0 {
		reportedAt = time.Now().Unix()
	}

	inventoryRows, inventoryCores := buildInventoryRows(req.AgentHostID, reportedAt, req.Inventory)
	indexRows, indexCores := buildInboundIndexRows(req.AgentHostID, reportedAt, req.InboundIndex)

	if len(inventoryRows) > 0 {
		if err := s.inventories.UpsertBatch(ctx, inventoryRows); err != nil {
			return err
		}
		for coreType := range inventoryCores {
			if err := s.inventories.DeleteStaleByHostCoreBefore(ctx, req.AgentHostID, coreType, reportedAt); err != nil {
				return err
			}
		}
	}

	if len(indexRows) > 0 {
		if err := s.inboundIndexes.UpsertBatch(ctx, indexRows); err != nil {
			return err
		}
		for coreType := range indexCores {
			if err := s.inboundIndexes.DeleteStaleByHostCoreBefore(ctx, req.AgentHostID, coreType, reportedAt); err != nil {
				return err
			}
		}
	}

	return nil
}

func buildInventoryRows(agentHostID int64, reportedAt int64, entries []InventoryReportEntry) ([]*repository.AgentConfigInventory, map[string]struct{}) {
	rows := make([]*repository.AgentConfigInventory, 0, len(entries))
	coreSet := make(map[string]struct{})
	for _, entry := range entries {
		source := normalizeInventorySource(entry.Source)
		if source == "" {
			continue
		}
		coreType := normalizeInventoryCoreType(entry.CoreType)
		if coreType == "" {
			continue
		}
		filename := strings.TrimSpace(entry.Filename)
		if filename == "" {
			continue
		}

		parseStatus := normalizeInventoryParseStatus(entry.ParseStatus)
		parseError := strings.TrimSpace(entry.ParseError)
		if parseStatus == inventoryParseStatusOK {
			parseError = ""
		}

		rows = append(rows, &repository.AgentConfigInventory{
			AgentHostID: agentHostID,
			CoreType:    coreType,
			Source:      source,
			Filename:    filename,
			HashApplied: strings.TrimSpace(entry.ContentHash),
			ParseStatus: parseStatus,
			ParseError:  parseError,
			LastSeenAt:  reportedAt,
		})
		coreSet[coreType] = struct{}{}
	}
	return rows, coreSet
}

func buildInboundIndexRows(agentHostID int64, reportedAt int64, entries []InboundIndexReportEntry) ([]*repository.InboundIndex, map[string]struct{}) {
	rows := make([]*repository.InboundIndex, 0, len(entries))
	coreSet := make(map[string]struct{})
	for _, entry := range entries {
		source := normalizeInventorySource(entry.Source)
		if source == "" {
			continue
		}
		coreType := normalizeInventoryCoreType(entry.CoreType)
		if coreType == "" {
			continue
		}
		filename := strings.TrimSpace(entry.Filename)
		if filename == "" {
			continue
		}
		tag := normalizeInventoryTag(entry.Tag)
		if tag == "" {
			continue
		}
		protocol := strings.ToLower(strings.TrimSpace(entry.Protocol))
		if protocol == "" {
			continue
		}
		listen := normalizeInventoryListen(entry.Listen)
		if listen == "" {
			continue
		}
		if entry.Port <= 0 || entry.Port > 65535 {
			continue
		}

		rows = append(rows, &repository.InboundIndex{
			AgentHostID: agentHostID,
			CoreType:    coreType,
			Source:      source,
			Filename:    filename,
			Tag:         tag,
			Protocol:    protocol,
			Listen:      listen,
			Port:        entry.Port,
			TLS:         normalizeJSONRaw(entry.TLS),
			Transport:   normalizeJSONRaw(entry.Transport),
			Multiplex:   normalizeJSONRaw(entry.Multiplex),
			LastSeenAt:  reportedAt,
		})
		coreSet[coreType] = struct{}{}
	}
	return rows, coreSet
}

func normalizeInventorySource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case inventorySourceLegacy:
		return inventorySourceLegacy
	case inventorySourceManaged:
		return inventorySourceManaged
	case inventorySourceMerged:
		return inventorySourceMerged
	default:
		return ""
	}
}

func normalizeInventoryParseStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case inventoryParseStatusOK:
		return inventoryParseStatusOK
	case inventoryParseStatusParseError:
		return inventoryParseStatusParseError
	default:
		return inventoryParseStatusParseError
	}
}

func normalizeJSONRaw(raw json.RawMessage) json.RawMessage {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return json.RawMessage("{}")
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return json.RawMessage("{}")
	}
	if obj == nil {
		obj = map[string]any{}
	}
	normalized, err := json.Marshal(obj)
	if err != nil {
		return json.RawMessage("{}")
	}
	return normalized
}

func normalizeInventoryCoreType(coreType string) string {
	return normalizeCoreType(coreType)
}

func normalizeInventoryTag(tag string) string {
	return normalizeTag(tag)
}

func normalizeInventoryListen(listen string) string {
	return normalizeListen(listen)
}

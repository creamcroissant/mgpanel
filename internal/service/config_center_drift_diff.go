package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	protocolparser "github.com/creamcroissant/xboard/internal/agent/protocol/parser"
	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/pmezard/go-difflib/difflib"
)

var (
	ErrDriftAndDiffInvalidRequest = errors.New("service: invalid drift/diff request / 漂移与对比请求无效")
	ErrDriftAndDiffNotConfigured  = errors.New("service: drift/diff service not configured / 漂移与对比服务未配置")
	ErrDriftAndDiffDesiredMissing = errors.New("service: desired artifacts not found / 未找到目标期望配置")
	ErrDriftAndDiffTagMissing     = errors.New("service: desired artifact tag not found / 未找到指定目标 tag")
)

const (
	driftTypeHashMismatch = "hash_mismatch"
	driftTypeMissingTag   = "missing_tag"
	driftTypeTagConflict  = "tag_conflict"
	driftTypeParseError   = "parse_error"

	driftStatusDrift     = "drift"
	driftStatusRecovered = "recovered"
)

// DriftAndDiffService calculates Desired/Applied drift and generates text/semantic diffs.
type DriftAndDiffService interface {
	EvaluateDrift(ctx context.Context, req EvaluateDriftRequest) (*EvaluateDriftResult, error)
	ListAppliedSnapshot(ctx context.Context, req ListAppliedSnapshotRequest) (*ListAppliedSnapshotResult, error)
	ListDriftStates(ctx context.Context, req ListDriftStatesRequest) (*ListDriftStatesResult, error)
	ListArtifacts(ctx context.Context, req ListDesiredArtifactsRequest) (*ListDesiredArtifactsResult, error)
	GetTextDiff(ctx context.Context, req GetTextDiffRequest) (*TextDiffResult, error)
	GetSemanticDiff(ctx context.Context, req GetSemanticDiffRequest) (*SemanticDiffResult, error)
}

// EvaluateDriftRequest defines drift evaluation scope.
type EvaluateDriftRequest struct {
	AgentHostID     int64
	CoreType        string
	DesiredRevision int64
}

// DriftItem describes one drift record computed by service.
type DriftItem struct {
	Filename        string          `json:"filename"`
	Tag             string          `json:"tag"`
	DesiredRevision int64           `json:"desired_revision"`
	DesiredHash     string          `json:"desired_hash"`
	AppliedHash     string          `json:"applied_hash"`
	DriftType       string          `json:"drift_type"`
	Detail          json.RawMessage `json:"detail"`
}

// EvaluateDriftResult is the drift evaluation output.
type EvaluateDriftResult struct {
	DesiredRevision int64       `json:"desired_revision"`
	Items           []DriftItem `json:"items"`
}

// ListAppliedSnapshotRequest defines one applied snapshot listing query.
type ListAppliedSnapshotRequest struct {
	AgentHostID int64
	CoreType    string
	Source      string
	Filename    string
	Tag         string
	Protocol    string
	ParseStatus string
	Limit       int
	Offset      int
}

// ListAppliedSnapshotResult returns paged applied snapshot rows.
type ListAppliedSnapshotResult struct {
	Inventories    []*repository.AgentConfigInventory `json:"inventories"`
	InboundIndexes []*repository.InboundIndex         `json:"inbound_indexes"`
}

// ListDriftStatesRequest defines one drift state listing query.
type ListDriftStatesRequest struct {
	AgentHostID int64
	CoreType    string
	Status      string
	DriftType   string
	Tag         string
	Filename    string
	Limit       int
	Offset      int
}

// ListDriftStatesResult is one drift state page.
type ListDriftStatesResult struct {
	Items []*repository.DriftState `json:"items"`
	Total int64                    `json:"total"`
}

// ListDesiredArtifactsRequest defines one desired artifact listing query.
type ListDesiredArtifactsRequest struct {
	AgentHostID     int64
	CoreType        string
	DesiredRevision int64
	Tag             string
	Filename        string
	IncludeContent  bool
	Limit           int
	Offset          int
}

// ListDesiredArtifactsResult is the desired artifact listing output.
type ListDesiredArtifactsResult struct {
	DesiredRevision int64                         `json:"desired_revision"`
	Items           []*repository.DesiredArtifact `json:"items"`
	Total           int64                         `json:"total"`
}

// GetTextDiffRequest defines one artifact text diff query.
type GetTextDiffRequest struct {
	AgentHostID     int64
	CoreType        string
	DesiredRevision int64
	Filename        string
	Tag             string
}

// TextDiffResult returns desired/applied normalized text and unified diff.
type TextDiffResult struct {
	DesiredRevision int64  `json:"desired_revision"`
	Filename        string `json:"filename"`
	Tag             string `json:"tag"`
	DesiredText     string `json:"desired_text"`
	AppliedText     string `json:"applied_text"`
	UnifiedDiff     string `json:"unified_diff"`
	Different       bool   `json:"different"`
}

// GetSemanticDiffRequest defines semantic diff scope.
type GetSemanticDiffRequest struct {
	AgentHostID     int64
	CoreType        string
	DesiredRevision int64
	Tag             string
}

// SemanticFieldDiff describes one semantic field mismatch.
type SemanticFieldDiff struct {
	Field   string `json:"field"`
	Desired string `json:"desired"`
	Applied string `json:"applied"`
}

// SemanticDiffItem describes one semantic mismatch item.
type SemanticDiffItem struct {
	Tag             string              `json:"tag"`
	DesiredFilename string              `json:"desired_filename,omitempty"`
	AppliedFilename string              `json:"applied_filename,omitempty"`
	DriftType       string              `json:"drift_type"`
	FieldDiffs      []SemanticFieldDiff `json:"field_diffs,omitempty"`
}

// SemanticDiffResult is semantic diff output.
type SemanticDiffResult struct {
	DesiredRevision int64              `json:"desired_revision"`
	Items           []SemanticDiffItem `json:"items"`
}

type driftAndDiffService struct {
	artifacts   repository.DesiredArtifactRepository
	inventories repository.AgentConfigInventoryRepository
	indexes     repository.InboundIndexRepository
	drifts      repository.DriftStateRepository
	parser      *protocolparser.Registry
}

// NewDriftAndDiffService creates DriftAndDiffService.
func NewDriftAndDiffService(
	artifacts repository.DesiredArtifactRepository,
	inventories repository.AgentConfigInventoryRepository,
	indexes repository.InboundIndexRepository,
	drifts repository.DriftStateRepository,
) DriftAndDiffService {
	return &driftAndDiffService{
		artifacts:   artifacts,
		inventories: inventories,
		indexes:     indexes,
		drifts:      drifts,
		parser:      protocolparser.NewRegistry(),
	}
}

func (s *driftAndDiffService) EvaluateDrift(ctx context.Context, req EvaluateDriftRequest) (*EvaluateDriftResult, error) {
	coreType, err := s.validateBaseRequest(req.AgentHostID, req.CoreType)
	if err != nil {
		return nil, err
	}
	desiredRevision, desiredArtifacts, err := s.resolveAndListDesiredArtifacts(ctx, req.AgentHostID, coreType, req.DesiredRevision, false)
	if err != nil {
		return nil, err
	}

	inventories, err := s.listAllInventories(ctx, req.AgentHostID, coreType, nil)
	if err != nil {
		return nil, err
	}
	indexes, err := s.listAllIndexes(ctx, req.AgentHostID, coreType, nil, nil)
	if err != nil {
		return nil, err
	}

	desiredByFilename := make(map[string]*repository.DesiredArtifact, len(desiredArtifacts))
	desiredByTag := make(map[string]*repository.DesiredArtifact, len(desiredArtifacts))
	driftByKey := make(map[driftKey]DriftItem)

	for _, artifact := range desiredArtifacts {
		if artifact == nil {
			continue
		}
		filename := strings.TrimSpace(artifact.Filename)
		if filename == "" {
			continue
		}
		desiredByFilename[filename] = artifact
		tag := normalizeTag(artifact.SourceTag)
		if tag != "" {
			if _, exists := desiredByTag[tag]; !exists {
				desiredByTag[tag] = artifact
			}
		}
	}

	preferredInventories := buildPreferredInventoryByFilename(inventories)
	appliedTagSelections := buildAppliedTagSelections(indexes)

	for _, artifact := range desiredArtifacts {
		if artifact == nil {
			continue
		}
		filename := strings.TrimSpace(artifact.Filename)
		if filename == "" {
			continue
		}
		tag := normalizeTag(artifact.SourceTag)
		preferredInventory := preferredInventories[filename]
		appliedHash := ""
		if preferredInventory != nil {
			appliedHash = strings.TrimSpace(preferredInventory.HashApplied)
		}
		if preferredInventory == nil || appliedHash != strings.TrimSpace(artifact.ContentHash) {
			detail := map[string]any{
				"filename":     filename,
				"desired_hash": strings.TrimSpace(artifact.ContentHash),
				"applied_hash": appliedHash,
			}
			if preferredInventory == nil {
				detail["reason"] = "applied file not found / 未发现已应用文件"
			} else {
				detail["source"] = preferredInventory.Source
				detail["parse_status"] = normalizeInventoryParseStatus(preferredInventory.ParseStatus)
			}
			addDriftItem(driftByKey, DriftItem{
				Filename:        filename,
				Tag:             tag,
				DesiredRevision: desiredRevision,
				DesiredHash:     strings.TrimSpace(artifact.ContentHash),
				AppliedHash:     appliedHash,
				DriftType:       driftTypeHashMismatch,
				Detail:          marshalJSONMap(detail),
			})
		}

		if tag != "" {
			if _, exists := appliedTagSelections[tag]; !exists {
				addDriftItem(driftByKey, DriftItem{
					Filename:        filename,
					Tag:             tag,
					DesiredRevision: desiredRevision,
					DesiredHash:     strings.TrimSpace(artifact.ContentHash),
					AppliedHash:     "",
					DriftType:       driftTypeMissingTag,
					Detail: marshalJSONMap(map[string]any{
						"reason": "desired tag missing in applied index / 已应用索引中缺失期望 tag",
						"tag":    tag,
					}),
				})
			}
		}
	}

	for filename, inventory := range preferredInventories {
		if inventory == nil || normalizeInventoryParseStatus(inventory.ParseStatus) != inventoryParseStatusParseError {
			continue
		}
		desiredTag := ""
		desiredHash := ""
		if artifact := desiredByFilename[filename]; artifact != nil {
			desiredTag = normalizeTag(artifact.SourceTag)
			desiredHash = strings.TrimSpace(artifact.ContentHash)
		}
		addDriftItem(driftByKey, DriftItem{
			Filename:        filename,
			Tag:             desiredTag,
			DesiredRevision: desiredRevision,
			DesiredHash:     desiredHash,
			AppliedHash:     strings.TrimSpace(inventory.HashApplied),
			DriftType:       driftTypeParseError,
			Detail: marshalJSONMap(map[string]any{
				"source":       inventory.Source,
				"parse_error":  strings.TrimSpace(inventory.ParseError),
				"parse_status": normalizeInventoryParseStatus(inventory.ParseStatus),
			}),
		})
	}

	for tag, selection := range appliedTagSelections {
		if len(selection.Conflicts) <= 1 {
			continue
		}
		filenames := make([]string, 0, len(selection.Conflicts))
		for _, entry := range selection.Conflicts {
			if entry == nil {
				continue
			}
			filenames = append(filenames, strings.TrimSpace(entry.Filename))
		}
		sort.Strings(filenames)
		desiredFilename := ""
		desiredHash := ""
		if artifact := desiredByTag[tag]; artifact != nil {
			desiredFilename = strings.TrimSpace(artifact.Filename)
			desiredHash = strings.TrimSpace(artifact.ContentHash)
		}
		if desiredFilename == "" && len(filenames) > 0 {
			desiredFilename = filenames[0]
		}
		source := ""
		if selection.Preferred != nil {
			source = selection.Preferred.Source
		}
		addDriftItem(driftByKey, DriftItem{
			Filename:        desiredFilename,
			Tag:             tag,
			DesiredRevision: desiredRevision,
			DesiredHash:     desiredHash,
			AppliedHash:     "",
			DriftType:       driftTypeTagConflict,
			Detail: marshalJSONMap(map[string]any{
				"source":    source,
				"filenames": filenames,
				"reason":    "multiple applied entries share same effective tag / 生效来源存在重复 tag",
			}),
		})
	}

	driftItems := driftItemsFromMap(driftByKey)
	existingStates, err := s.listAllDriftStates(ctx, req.AgentHostID, coreType)
	if err != nil {
		return nil, err
	}
	if err := s.persistDriftStates(ctx, req.AgentHostID, coreType, desiredRevision, driftItems, existingStates); err != nil {
		return nil, err
	}

	return &EvaluateDriftResult{DesiredRevision: desiredRevision, Items: driftItems}, nil
}

func (s *driftAndDiffService) ListAppliedSnapshot(ctx context.Context, req ListAppliedSnapshotRequest) (*ListAppliedSnapshotResult, error) {
	coreType, err := s.validateBaseRequest(req.AgentHostID, req.CoreType)
	if err != nil {
		return nil, err
	}

	filterSource := strings.ToLower(strings.TrimSpace(req.Source))
	if filterSource != "" {
		switch filterSource {
		case inventorySourceLegacy, inventorySourceManaged, inventorySourceMerged:
		default:
			return nil, fmt.Errorf("%w (source must be legacy/managed/merged)", ErrDriftAndDiffInvalidRequest)
		}
	}
	filterParseStatus := strings.ToLower(strings.TrimSpace(req.ParseStatus))
	if filterParseStatus != "" {
		switch filterParseStatus {
		case inventoryParseStatusOK, inventoryParseStatusParseError:
		default:
			return nil, fmt.Errorf("%w (parse_status must be ok/parse_error)", ErrDriftAndDiffInvalidRequest)
		}
	}

	hostID := req.AgentHostID
	inventoryFilter := repository.AgentConfigInventoryFilter{
		AgentHostID: &hostID,
		CoreType:    &coreType,
		Limit:       req.Limit,
		Offset:      req.Offset,
	}
	indexFilter := repository.InboundIndexFilter{
		AgentHostID: &hostID,
		CoreType:    &coreType,
		Limit:       req.Limit,
		Offset:      req.Offset,
	}

	if filterSource != "" {
		source := filterSource
		inventoryFilter.Source = &source
		indexFilter.Source = &source
	}
	if filename := strings.TrimSpace(req.Filename); filename != "" {
		inventoryFilter.Filename = &filename
		indexFilter.Filename = &filename
	}
	if tag := normalizeTag(req.Tag); tag != "" {
		indexFilter.Tag = &tag
	}
	if protocol := strings.ToLower(strings.TrimSpace(req.Protocol)); protocol != "" {
		indexFilter.Protocol = &protocol
	}
	if filterParseStatus != "" {
		parseStatus := filterParseStatus
		inventoryFilter.ParseStatus = &parseStatus
	}

	inventories, err := s.inventories.List(ctx, inventoryFilter)
	if err != nil {
		return nil, err
	}
	indexes, err := s.indexes.List(ctx, indexFilter)
	if err != nil {
		return nil, err
	}
	return &ListAppliedSnapshotResult{Inventories: inventories, InboundIndexes: indexes}, nil
}

func (s *driftAndDiffService) ListDriftStates(ctx context.Context, req ListDriftStatesRequest) (*ListDriftStatesResult, error) {
	coreType, err := s.validateBaseRequest(req.AgentHostID, req.CoreType)
	if err != nil {
		return nil, err
	}

	filterStatus := strings.ToLower(strings.TrimSpace(req.Status))
	if filterStatus != "" && filterStatus != driftStatusDrift && filterStatus != driftStatusRecovered {
		return nil, fmt.Errorf("%w (status must be drift/recovered)", ErrDriftAndDiffInvalidRequest)
	}
	filterDriftType := strings.ToLower(strings.TrimSpace(req.DriftType))
	if filterDriftType != "" {
		switch filterDriftType {
		case driftTypeHashMismatch, driftTypeMissingTag, driftTypeTagConflict, driftTypeParseError:
		default:
			return nil, fmt.Errorf("%w (drift_type must be hash_mismatch/missing_tag/tag_conflict/parse_error)", ErrDriftAndDiffInvalidRequest)
		}
	}

	hostID := req.AgentHostID
	filter := repository.DriftStateFilter{
		AgentHostID: &hostID,
		CoreType:    &coreType,
		Limit:       req.Limit,
		Offset:      req.Offset,
	}
	if filterStatus != "" {
		status := filterStatus
		filter.Status = &status
	}
	if filterDriftType != "" {
		driftType := filterDriftType
		filter.DriftType = &driftType
	}
	if tag := normalizeTag(req.Tag); tag != "" {
		filter.Tag = &tag
	}
	if filename := strings.TrimSpace(req.Filename); filename != "" {
		filter.Filename = &filename
	}

	total, err := s.drifts.Count(ctx, filter)
	if err != nil {
		return nil, err
	}
	items, err := s.drifts.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &ListDriftStatesResult{Items: items, Total: total}, nil
}

func (s *driftAndDiffService) ListArtifacts(ctx context.Context, req ListDesiredArtifactsRequest) (*ListDesiredArtifactsResult, error) {
	coreType, err := s.validateBaseRequest(req.AgentHostID, req.CoreType)
	if err != nil {
		return nil, err
	}

	desiredRevision, err := s.resolveDesiredRevision(ctx, req.AgentHostID, coreType, req.DesiredRevision)
	if err != nil {
		return nil, err
	}

	targetTag := normalizeTag(req.Tag)
	targetFilename := strings.TrimSpace(req.Filename)
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	core := coreType
	revision := desiredRevision
	filter := repository.DesiredArtifactFilter{
		AgentHostID:     req.AgentHostID,
		CoreType:        &core,
		DesiredRevision: &revision,
		ExcludeContent:  !req.IncludeContent,
		Limit:           limit,
		Offset:          offset,
	}
	if targetTag != "" {
		tag := targetTag
		filter.SourceTag = &tag
	}
	if targetFilename != "" {
		name := targetFilename
		filter.Filename = &name
	}

	items, err := s.artifacts.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	total, err := s.artifacts.Count(ctx, repository.DesiredArtifactFilter{
		AgentHostID:     req.AgentHostID,
		CoreType:        &core,
		DesiredRevision: &revision,
		SourceTag:       filter.SourceTag,
		Filename:        filter.Filename,
	})
	if err != nil {
		return nil, err
	}

	return &ListDesiredArtifactsResult{
		DesiredRevision: desiredRevision,
		Items:           items,
		Total:           total,
	}, nil
}

func (s *driftAndDiffService) GetTextDiff(ctx context.Context, req GetTextDiffRequest) (*TextDiffResult, error) {
	coreType, err := s.validateBaseRequest(req.AgentHostID, req.CoreType)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Filename) == "" && normalizeTag(req.Tag) == "" {
		return nil, fmt.Errorf("%w (filename or tag is required / filename 或 tag 至少需要一个)", ErrDriftAndDiffInvalidRequest)
	}
	desiredRevision, err := s.resolveDesiredRevision(ctx, req.AgentHostID, coreType, req.DesiredRevision)
	if err != nil {
		return nil, err
	}

	artifact, err := s.findDesiredArtifact(ctx, req.AgentHostID, coreType, desiredRevision, req.Filename, req.Tag)
	if err != nil {
		return nil, err
	}
	desiredText, err := canonicalizeJSONText(artifact.Content)
	if err != nil {
		return nil, fmt.Errorf("canonicalize desired artifact text: %w", err)
	}
	appliedText, err := s.buildAppliedTextByFilename(ctx, req.AgentHostID, coreType, artifact.Filename)
	if err != nil {
		return nil, err
	}

	different := desiredText != appliedText
	unifiedDiff := ""
	if different {
		unifiedDiff, err = difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
			A:        difflib.SplitLines(desiredText),
			B:        difflib.SplitLines(appliedText),
			FromFile: "desired/" + artifact.Filename,
			ToFile:   "applied/" + artifact.Filename,
			Context:  3,
		})
		if err != nil {
			return nil, err
		}
	}

	return &TextDiffResult{
		DesiredRevision: desiredRevision,
		Filename:        artifact.Filename,
		Tag:             normalizeTag(artifact.SourceTag),
		DesiredText:     desiredText,
		AppliedText:     appliedText,
		UnifiedDiff:     unifiedDiff,
		Different:       different,
	}, nil
}

func (s *driftAndDiffService) GetSemanticDiff(ctx context.Context, req GetSemanticDiffRequest) (*SemanticDiffResult, error) {
	coreType, err := s.validateBaseRequest(req.AgentHostID, req.CoreType)
	if err != nil {
		return nil, err
	}
	targetTag := normalizeTag(req.Tag)
	desiredRevision, desiredArtifacts, err := s.resolveAndListDesiredArtifacts(ctx, req.AgentHostID, coreType, req.DesiredRevision, true)
	if err != nil {
		return nil, err
	}
	desiredByTag, err := s.buildDesiredSemanticByTag(desiredArtifacts)
	if err != nil {
		return nil, err
	}
	if targetTag != "" {
		if _, exists := desiredByTag[targetTag]; !exists {
			return nil, fmt.Errorf("%w (tag=%s)", ErrDriftAndDiffTagMissing, targetTag)
		}
	}

	var filterTag *string
	if targetTag != "" {
		tag := targetTag
		filterTag = &tag
	}
	indexes, err := s.listAllIndexes(ctx, req.AgentHostID, coreType, nil, filterTag)
	if err != nil {
		return nil, err
	}
	appliedSelections := buildAppliedTagSelections(indexes)

	tags := make([]string, 0, len(desiredByTag))
	for tag := range desiredByTag {
		if targetTag != "" && tag != targetTag {
			continue
		}
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	items := make([]SemanticDiffItem, 0)
	for _, tag := range tags {
		desiredList := desiredByTag[tag]
		if len(desiredList) == 0 {
			continue
		}
		sort.Slice(desiredList, func(i, j int) bool {
			if desiredList[i].Filename == desiredList[j].Filename {
				return desiredList[i].Tag < desiredList[j].Tag
			}
			return desiredList[i].Filename < desiredList[j].Filename
		})
		desiredPrimary := desiredList[0]
		if len(desiredList) > 1 {
			items = append(items, SemanticDiffItem{
				Tag:             tag,
				DesiredFilename: desiredPrimary.Filename,
				DriftType:       driftTypeTagConflict,
			})
			continue
		}

		selection, exists := appliedSelections[tag]
		if !exists {
			items = append(items, SemanticDiffItem{
				Tag:             tag,
				DesiredFilename: desiredPrimary.Filename,
				DriftType:       driftTypeMissingTag,
			})
			continue
		}
		if len(selection.Conflicts) > 1 {
			appliedFilename := ""
			if selection.Preferred != nil {
				appliedFilename = strings.TrimSpace(selection.Preferred.Filename)
			}
			items = append(items, SemanticDiffItem{
				Tag:             tag,
				DesiredFilename: desiredPrimary.Filename,
				AppliedFilename: appliedFilename,
				DriftType:       driftTypeTagConflict,
			})
			continue
		}

		applied := semanticInboundFromIndex(selection.Preferred)
		fieldDiffs := compareSemanticInbound(desiredPrimary, applied)
		if len(fieldDiffs) == 0 {
			continue
		}
		items = append(items, SemanticDiffItem{
			Tag:             tag,
			DesiredFilename: desiredPrimary.Filename,
			AppliedFilename: applied.Filename,
			DriftType:       driftTypeHashMismatch,
			FieldDiffs:      fieldDiffs,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Tag == items[j].Tag {
			return items[i].DriftType < items[j].DriftType
		}
		return items[i].Tag < items[j].Tag
	})

	return &SemanticDiffResult{DesiredRevision: desiredRevision, Items: items}, nil
}

func (s *driftAndDiffService) validateBaseRequest(agentHostID int64, coreType string) (string, error) {
	if s == nil || s.artifacts == nil || s.inventories == nil || s.indexes == nil || s.drifts == nil {
		return "", ErrDriftAndDiffNotConfigured
	}
	if agentHostID <= 0 {
		return "", fmt.Errorf("%w (agent_host_id is required / 不能为空)", ErrDriftAndDiffInvalidRequest)
	}
	normalizedCoreType := normalizeCoreType(coreType)
	if normalizedCoreType == "" {
		return "", fmt.Errorf("%w (core_type must be sing-box or xray / 必须是 sing-box 或 xray)", ErrDriftAndDiffInvalidRequest)
	}
	return normalizedCoreType, nil
}

func (s *driftAndDiffService) resolveAndListDesiredArtifacts(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, includeContent bool) (int64, []*repository.DesiredArtifact, error) {
	revision, err := s.resolveDesiredRevision(ctx, agentHostID, coreType, desiredRevision)
	if err != nil {
		return 0, nil, err
	}

	artifacts, err := s.listDesiredArtifactsByRevision(ctx, agentHostID, coreType, revision, "", "", includeContent)
	if err != nil {
		return 0, nil, err
	}
	if len(artifacts) == 0 {
		return 0, nil, fmt.Errorf("%w (desired_revision=%d)", ErrDriftAndDiffDesiredMissing, revision)
	}
	return revision, artifacts, nil
}

func (s *driftAndDiffService) resolveDesiredRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64) (int64, error) {
	revision := desiredRevision
	if revision <= 0 {
		latest, err := s.artifacts.GetLatestRevision(ctx, agentHostID, coreType)
		if err != nil {
			return 0, err
		}
		if latest <= 0 {
			return 0, ErrDriftAndDiffDesiredMissing
		}
		revision = latest
	}
	return revision, nil
}

func (s *driftAndDiffService) listDesiredArtifactsByRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, sourceTag string, filename string, includeContent bool) ([]*repository.DesiredArtifact, error) {
	limit := 500
	offset := 0
	result := make([]*repository.DesiredArtifact, 0)
	for {
		core := coreType
		revision := desiredRevision
		filter := repository.DesiredArtifactFilter{
			AgentHostID:     agentHostID,
			CoreType:        &core,
			DesiredRevision: &revision,
			ExcludeContent:  !includeContent,
			Limit:           limit,
			Offset:          offset,
		}
		if sourceTag != "" {
			tag := sourceTag
			filter.SourceTag = &tag
		}
		if filename != "" {
			name := filename
			filter.Filename = &name
		}
		items, err := s.artifacts.List(ctx, filter)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			if item != nil {
				result = append(result, item)
			}
		}
		if len(items) < limit {
			break
		}
		offset += limit
	}
	return result, nil
}

func (s *driftAndDiffService) listAllInventories(ctx context.Context, agentHostID int64, coreType string, filename *string) ([]*repository.AgentConfigInventory, error) {
	limit := 500
	offset := 0
	result := make([]*repository.AgentConfigInventory, 0)
	for {
		hostID := agentHostID
		core := coreType
		filter := repository.AgentConfigInventoryFilter{
			AgentHostID: &hostID,
			CoreType:    &core,
			Limit:       limit,
			Offset:      offset,
		}
		if filename != nil {
			name := strings.TrimSpace(*filename)
			filter.Filename = &name
		}
		items, err := s.inventories.List(ctx, filter)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			if item != nil {
				result = append(result, item)
			}
		}
		if len(items) < limit {
			break
		}
		offset += limit
	}
	return result, nil
}

func (s *driftAndDiffService) listAllIndexes(ctx context.Context, agentHostID int64, coreType string, filename *string, tag *string) ([]*repository.InboundIndex, error) {
	limit := 500
	offset := 0
	result := make([]*repository.InboundIndex, 0)
	for {
		hostID := agentHostID
		core := coreType
		filter := repository.InboundIndexFilter{
			AgentHostID: &hostID,
			CoreType:    &core,
			Limit:       limit,
			Offset:      offset,
		}
		if filename != nil {
			name := strings.TrimSpace(*filename)
			filter.Filename = &name
		}
		if tag != nil {
			normalizedTag := normalizeTag(*tag)
			filter.Tag = &normalizedTag
		}
		items, err := s.indexes.List(ctx, filter)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			if item != nil {
				result = append(result, item)
			}
		}
		if len(items) < limit {
			break
		}
		offset += limit
	}
	return result, nil
}

func (s *driftAndDiffService) listAllDriftStates(ctx context.Context, agentHostID int64, coreType string) ([]*repository.DriftState, error) {
	limit := 500
	offset := 0
	result := make([]*repository.DriftState, 0)
	for {
		hostID := agentHostID
		core := coreType
		items, err := s.drifts.List(ctx, repository.DriftStateFilter{
			AgentHostID: &hostID,
			CoreType:    &core,
			Limit:       limit,
			Offset:      offset,
		})
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			if item != nil {
				result = append(result, item)
			}
		}
		if len(items) < limit {
			break
		}
		offset += limit
	}
	return result, nil
}

func (s *driftAndDiffService) persistDriftStates(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, current []DriftItem, existing []*repository.DriftState) error {
	now := time.Now().Unix()
	currentMap := make(map[driftKey]DriftItem, len(current))
	for _, item := range current {
		currentMap[toDriftKey(item.Filename, item.Tag, item.DriftType)] = item
	}
	existingMap := make(map[driftKey]*repository.DriftState, len(existing))
	staleOpenExists := false
	for _, item := range existing {
		if item == nil {
			continue
		}
		key := toDriftKey(item.Filename, item.Tag, item.DriftType)
		existingMap[key] = item
		if item.Status == driftStatusDrift {
			if _, ok := currentMap[key]; !ok {
				staleOpenExists = true
			}
		}
	}

	if staleOpenExists {
		if err := s.drifts.MarkRecoveredByHostCore(ctx, agentHostID, coreType, now); err != nil {
			return err
		}
	}

	for _, item := range current {
		key := toDriftKey(item.Filename, item.Tag, item.DriftType)
		existingItem := existingMap[key]
		if !shouldUpsertDrift(existingItem, item, staleOpenExists) {
			continue
		}
		if err := s.drifts.Upsert(ctx, &repository.DriftState{
			AgentHostID:     agentHostID,
			CoreType:        coreType,
			Filename:        item.Filename,
			Tag:             item.Tag,
			DesiredRevision: desiredRevision,
			DesiredHash:     item.DesiredHash,
			AppliedHash:     item.AppliedHash,
			DriftType:       item.DriftType,
			Status:          driftStatusDrift,
			Detail:          normalizeJSONDocumentRaw(item.Detail),
			FirstDetectedAt: now,
			LastChangedAt:   now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func shouldUpsertDrift(existing *repository.DriftState, current DriftItem, staleRecovered bool) bool {
	if existing == nil {
		return true
	}
	if staleRecovered {
		return true
	}
	if existing.Status != driftStatusDrift {
		return true
	}
	if existing.DesiredRevision != current.DesiredRevision {
		return true
	}
	if strings.TrimSpace(existing.DesiredHash) != strings.TrimSpace(current.DesiredHash) {
		return true
	}
	if strings.TrimSpace(existing.AppliedHash) != strings.TrimSpace(current.AppliedHash) {
		return true
	}
	return !jsonRawEqual(existing.Detail, current.Detail)
}

func (s *driftAndDiffService) buildAppliedTextByFilename(ctx context.Context, agentHostID int64, coreType, filename string) (string, error) {
	trimmedFilename := strings.TrimSpace(filename)
	if trimmedFilename == "" {
		return "", nil
	}
	indexes, err := s.listAllIndexes(ctx, agentHostID, coreType, &trimmedFilename, nil)
	if err != nil {
		return "", err
	}
	if len(indexes) == 0 {
		return "", nil
	}

	highestPriority := 0
	for _, entry := range indexes {
		priority := sourcePriority(entry.Source)
		if priority > highestPriority {
			highestPriority = priority
		}
	}
	selected := make([]*repository.InboundIndex, 0, len(indexes))
	for _, entry := range indexes {
		if sourcePriority(entry.Source) == highestPriority {
			selected = append(selected, entry)
		}
	}
	if len(selected) == 0 {
		return "", nil
	}

	sort.Slice(selected, func(i, j int) bool {
		leftTag := normalizeTag(selected[i].Tag)
		rightTag := normalizeTag(selected[j].Tag)
		if leftTag == rightTag {
			if selected[i].Port == selected[j].Port {
				return selected[i].Filename < selected[j].Filename
			}
			return selected[i].Port < selected[j].Port
		}
		return leftTag < rightTag
	})

	inbounds := make([]map[string]any, 0, len(selected))
	for _, entry := range selected {
		if entry == nil {
			continue
		}
		inbounds = append(inbounds, map[string]any{
			"tag":       normalizeTag(entry.Tag),
			"protocol":  strings.ToLower(strings.TrimSpace(entry.Protocol)),
			"listen":    normalizeListen(entry.Listen),
			"port":      entry.Port,
			"tls":       jsonValueFromRaw(entry.TLS),
			"transport": jsonValueFromRaw(entry.Transport),
			"multiplex": jsonValueFromRaw(entry.Multiplex),
		})
	}

	return canonicalizeJSONValueText(map[string]any{"inbounds": inbounds})
}

func (s *driftAndDiffService) buildDesiredSemanticByTag(artifacts []*repository.DesiredArtifact) (map[string][]semanticInbound, error) {
	result := make(map[string][]semanticInbound)
	for _, artifact := range artifacts {
		if artifact == nil {
			continue
		}
		details, err := s.parser.Parse(strings.TrimSpace(artifact.Filename), artifact.Content)
		if err != nil {
			return nil, fmt.Errorf("parse desired artifact (filename=%s): %w", artifact.Filename, err)
		}
		if len(details) == 0 {
			tag := normalizeTag(artifact.SourceTag)
			if tag == "" {
				continue
			}
			result[tag] = append(result[tag], semanticInbound{
				Tag:       tag,
				Filename:  strings.TrimSpace(artifact.Filename),
				TLS:       json.RawMessage("{}"),
				Transport: json.RawMessage("{}"),
				Multiplex: json.RawMessage("{}"),
			})
			continue
		}

		for _, detail := range details {
			tag := normalizeTag(firstNonEmpty(detail.Tag, artifact.SourceTag))
			if tag == "" {
				continue
			}
			result[tag] = append(result[tag], semanticInbound{
				Tag:       tag,
				Filename:  strings.TrimSpace(artifact.Filename),
				Protocol:  strings.ToLower(strings.TrimSpace(detail.Protocol)),
				Listen:    normalizeListen(detail.Listen),
				Port:      detail.Port,
				TLS:       normalizeJSONDocumentFromValue(detail.TLS),
				Transport: normalizeJSONDocumentFromValue(detail.Transport),
				Multiplex: normalizeJSONDocumentFromValue(detail.Multiplex),
			})
		}
	}
	return result, nil
}

func compareSemanticInbound(desired, applied semanticInbound) []SemanticFieldDiff {
	result := make([]SemanticFieldDiff, 0)
	if strings.ToLower(strings.TrimSpace(desired.Protocol)) != strings.ToLower(strings.TrimSpace(applied.Protocol)) {
		result = append(result, SemanticFieldDiff{Field: "protocol", Desired: desired.Protocol, Applied: applied.Protocol})
	}
	if normalizeListen(desired.Listen) != normalizeListen(applied.Listen) {
		result = append(result, SemanticFieldDiff{Field: "listen", Desired: desired.Listen, Applied: applied.Listen})
	}
	if desired.Port != applied.Port {
		result = append(result, SemanticFieldDiff{Field: "port", Desired: strconv.Itoa(desired.Port), Applied: strconv.Itoa(applied.Port)})
	}
	desiredTLS := normalizedJSONText(desired.TLS)
	appliedTLS := normalizedJSONText(applied.TLS)
	if desiredTLS != appliedTLS {
		result = append(result, SemanticFieldDiff{Field: "tls", Desired: desiredTLS, Applied: appliedTLS})
	}
	desiredTransport := normalizedJSONText(desired.Transport)
	appliedTransport := normalizedJSONText(applied.Transport)
	if desiredTransport != appliedTransport {
		result = append(result, SemanticFieldDiff{Field: "transport", Desired: desiredTransport, Applied: appliedTransport})
	}
	desiredMultiplex := normalizedJSONText(desired.Multiplex)
	appliedMultiplex := normalizedJSONText(applied.Multiplex)
	if desiredMultiplex != appliedMultiplex {
		result = append(result, SemanticFieldDiff{Field: "multiplex", Desired: desiredMultiplex, Applied: appliedMultiplex})
	}
	return result
}

func (s *driftAndDiffService) findDesiredArtifact(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, filename, tag string) (*repository.DesiredArtifact, error) {
	trimmedFilename := strings.TrimSpace(filename)
	normalizedTag := normalizeTag(tag)
	if trimmedFilename != "" {
		artifact, err := s.artifacts.FindByHostCoreRevisionFilename(ctx, agentHostID, coreType, desiredRevision, trimmedFilename)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, ErrDriftAndDiffDesiredMissing
			}
			return nil, err
		}
		if artifact == nil {
			return nil, ErrDriftAndDiffDesiredMissing
		}
		if normalizedTag != "" && normalizeTag(artifact.SourceTag) != normalizedTag {
			return nil, fmt.Errorf("%w (filename/tag mismatch)", ErrDriftAndDiffInvalidRequest)
		}
		return artifact, nil
	}

	if normalizedTag == "" {
		return nil, fmt.Errorf("%w (filename or tag is required / filename 或 tag 至少需要一个)", ErrDriftAndDiffInvalidRequest)
	}
	candidates, err := s.listDesiredArtifactsByRevision(ctx, agentHostID, coreType, desiredRevision, normalizedTag, "", true)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, ErrDriftAndDiffDesiredMissing
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Filename == candidates[j].Filename {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Filename < candidates[j].Filename
	})
	if len(candidates) > 1 {
		return nil, fmt.Errorf("%w (artifact selector is ambiguous / 目标 artifact 不唯一)", ErrDriftAndDiffInvalidRequest)
	}
	return candidates[0], nil
}

type driftKey struct {
	Filename  string
	Tag       string
	DriftType string
}

func toDriftKey(filename, tag, driftType string) driftKey {
	return driftKey{
		Filename:  strings.TrimSpace(filename),
		Tag:       normalizeTag(tag),
		DriftType: strings.TrimSpace(driftType),
	}
}

func addDriftItem(target map[driftKey]DriftItem, item DriftItem) {
	key := toDriftKey(item.Filename, item.Tag, item.DriftType)
	if _, exists := target[key]; exists {
		return
	}
	item.Filename = key.Filename
	item.Tag = key.Tag
	item.DriftType = key.DriftType
	item.Detail = normalizeJSONDocumentRaw(item.Detail)
	target[key] = item
}

func driftItemsFromMap(source map[driftKey]DriftItem) []DriftItem {
	items := make([]DriftItem, 0, len(source))
	for _, item := range source {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Filename == items[j].Filename {
			if items[i].Tag == items[j].Tag {
				return items[i].DriftType < items[j].DriftType
			}
			return items[i].Tag < items[j].Tag
		}
		return items[i].Filename < items[j].Filename
	})
	return items
}

func sourcePriority(source string) int {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case inventorySourceManaged:
		return 3
	case inventorySourceMerged:
		return 2
	case inventorySourceLegacy:
		return 1
	default:
		return 0
	}
}

func buildPreferredInventoryByFilename(items []*repository.AgentConfigInventory) map[string]*repository.AgentConfigInventory {
	grouped := make(map[string][]*repository.AgentConfigInventory)
	for _, item := range items {
		if item == nil {
			continue
		}
		filename := strings.TrimSpace(item.Filename)
		if filename == "" {
			continue
		}
		grouped[filename] = append(grouped[filename], item)
	}
	result := make(map[string]*repository.AgentConfigInventory, len(grouped))
	for filename, rows := range grouped {
		sort.Slice(rows, func(i, j int) bool {
			leftPriority := sourcePriority(rows[i].Source)
			rightPriority := sourcePriority(rows[j].Source)
			if leftPriority == rightPriority {
				if rows[i].LastSeenAt == rows[j].LastSeenAt {
					return rows[i].ID > rows[j].ID
				}
				return rows[i].LastSeenAt > rows[j].LastSeenAt
			}
			return leftPriority > rightPriority
		})
		result[filename] = rows[0]
	}
	return result
}

type appliedTagSelection struct {
	Preferred *repository.InboundIndex
	Conflicts []*repository.InboundIndex
}

func buildAppliedTagSelections(items []*repository.InboundIndex) map[string]appliedTagSelection {
	grouped := make(map[string][]*repository.InboundIndex)
	for _, item := range items {
		if item == nil {
			continue
		}
		tag := normalizeTag(item.Tag)
		if tag == "" {
			continue
		}
		grouped[tag] = append(grouped[tag], item)
	}

	result := make(map[string]appliedTagSelection, len(grouped))
	for tag, rows := range grouped {
		sort.Slice(rows, func(i, j int) bool {
			leftPriority := sourcePriority(rows[i].Source)
			rightPriority := sourcePriority(rows[j].Source)
			if leftPriority == rightPriority {
				if rows[i].LastSeenAt == rows[j].LastSeenAt {
					if rows[i].Filename == rows[j].Filename {
						return rows[i].ID > rows[j].ID
					}
					return rows[i].Filename < rows[j].Filename
				}
				return rows[i].LastSeenAt > rows[j].LastSeenAt
			}
			return leftPriority > rightPriority
		})
		bestPriority := sourcePriority(rows[0].Source)
		conflicts := make([]*repository.InboundIndex, 0, len(rows))
		for _, row := range rows {
			if sourcePriority(row.Source) != bestPriority {
				break
			}
			conflicts = append(conflicts, row)
		}
		result[tag] = appliedTagSelection{
			Preferred: rows[0],
			Conflicts: conflicts,
		}
	}
	return result
}

type semanticInbound struct {
	Tag       string
	Filename  string
	Protocol  string
	Listen    string
	Port      int
	TLS       json.RawMessage
	Transport json.RawMessage
	Multiplex json.RawMessage
}

func semanticInboundFromIndex(item *repository.InboundIndex) semanticInbound {
	if item == nil {
		return semanticInbound{TLS: json.RawMessage("{}"), Transport: json.RawMessage("{}"), Multiplex: json.RawMessage("{}")}
	}
	return semanticInbound{
		Tag:       normalizeTag(item.Tag),
		Filename:  strings.TrimSpace(item.Filename),
		Protocol:  strings.ToLower(strings.TrimSpace(item.Protocol)),
		Listen:    normalizeListen(item.Listen),
		Port:      item.Port,
		TLS:       normalizeJSONDocumentRaw(item.TLS),
		Transport: normalizeJSONDocumentRaw(item.Transport),
		Multiplex: normalizeJSONDocumentRaw(item.Multiplex),
	}
}

func marshalJSONMap(value map[string]any) json.RawMessage {
	if value == nil {
		return json.RawMessage("{}")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return normalizeJSONDocumentRaw(encoded)
}

func normalizeJSONDocumentFromValue(value any) json.RawMessage {
	if value == nil {
		return json.RawMessage("{}")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return normalizeJSONDocumentRaw(encoded)
}

func normalizeJSONDocumentRaw(raw []byte) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage("{}")
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return json.RawMessage("{}")
	}
	if value == nil {
		return json.RawMessage("{}")
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("{}")
	}
	return normalized
}

func normalizedJSONText(raw json.RawMessage) string {
	return string(normalizeJSONDocumentRaw(raw))
}

func jsonValueFromRaw(raw json.RawMessage) any {
	normalized := normalizeJSONDocumentRaw(raw)
	var value any
	if err := json.Unmarshal(normalized, &value); err != nil || value == nil {
		return map[string]any{}
	}
	return value
}

func jsonRawEqual(left, right json.RawMessage) bool {
	return normalizedJSONText(left) == normalizedJSONText(right)
}

func canonicalizeJSONText(raw []byte) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "", nil
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return "", err
	}
	if value == nil {
		value = map[string]any{}
	}
	return canonicalizeJSONValueText(value)
}

func canonicalizeJSONValueText(value any) (string, error) {
	normalized, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	if err := json.Indent(&buffer, normalized, "", "  "); err != nil {
		return "", err
	}
	buffer.WriteByte('\n')
	return buffer.String(), nil
}

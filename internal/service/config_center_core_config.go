package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/creamcroissant/xboard/internal/repository"
)

// CoreConfigItemService manages non-inbound core config items (outbound/routing/dns/core_settings).
type CoreConfigItemService interface {
	Upsert(ctx context.Context, req UpsertCoreConfigItemRequest) (id int64, revision int64, err error)
	List(ctx context.Context, filter ListCoreConfigItemFilter) ([]*repository.CoreConfigItem, int64, error)
	Delete(ctx context.Context, id int64) error
}

// UpsertCoreConfigItemRequest carries create/update fields for one core config item.
type UpsertCoreConfigItemRequest struct {
	ID          int64           // 0 = create
	AgentHostID *int64          // nil = 模板项
	CoreType    string
	ConfigType  string          // outbound | routing | dns | core_settings
	Tag         string
	Enabled     *bool
	ConfigData  json.RawMessage
	OperatorID  int64
	ChangeNote  string
}

// ListCoreConfigItemFilter constrains core config item list query.
type ListCoreConfigItemFilter struct {
	AgentHostID *int64
	CoreType    *string
	ConfigType  *string
	Tag         *string
	Enabled     *bool
	IsTemplate  *bool
	Limit       int
	Offset      int
}

type coreConfigItemService struct {
	items    repository.CoreConfigItemRepository
	compiler ArtifactCompilerService
}

// NewCoreConfigItemService creates CoreConfigItemService.
func NewCoreConfigItemService(items repository.CoreConfigItemRepository, compilers ...ArtifactCompilerService) CoreConfigItemService {
	var compiler ArtifactCompilerService
	if len(compilers) > 0 {
		compiler = compilers[0]
	}
	return &coreConfigItemService{items: items, compiler: compiler}
}

func (s *coreConfigItemService) Upsert(ctx context.Context, req UpsertCoreConfigItemRequest) (int64, int64, error) {
	if s == nil || s.items == nil {
		return 0, 0, fmt.Errorf("core config item service not configured")
	}
	if req.ID <= 0 {
		return s.create(ctx, req)
	}
	return s.update(ctx, req)
}

func (s *coreConfigItemService) create(ctx context.Context, req UpsertCoreConfigItemRequest) (int64, int64, error) {
	isTemplate := req.AgentHostID == nil

	if !isTemplate && *req.AgentHostID <= 0 {
		return 0, 0, &InboundSpecValidationError{}
	}

	configType := strings.TrimSpace(req.ConfigType)
	if configType == "" {
		validationErr := &InboundSpecValidationError{}
		validationErr.add("config_type", "is required / 不能为空")
		return 0, 0, validationErr
	}
	tag := normalizeTag(req.Tag)
	if tag == "" {
		validationErr := &InboundSpecValidationError{}
		validationErr.add("tag", "is required / 不能为空")
		return 0, 0, validationErr
	}
	coreType := normalizeCoreType(req.CoreType)
	if coreType == "" {
		validationErr := &InboundSpecValidationError{}
		validationErr.add("core_type", "must be sing-box or xray / 必须是 sing-box 或 xray")
		return 0, 0, validationErr
	}

	configData := req.ConfigData
	if len(strings.TrimSpace(string(configData))) == 0 {
		configData = json.RawMessage("{}")
	}
	// Validate JSON
	var dummy any
	if err := json.Unmarshal(configData, &dummy); err != nil {
		validationErr := &InboundSpecValidationError{}
		validationErr.add("config_data", "must be valid JSON / 必须是合法 JSON")
		return 0, 0, validationErr
	}

	if isTemplate {
		if existing, findErr := s.items.FindByCoreTypeTag(ctx, coreType, configType, tag); findErr == nil && existing != nil {
			return 0, 0, &InboundSpecConflictError{Kind: "template_tag", Field: "tag", Value: tag, ExistingSpecID: existing.ID}
		}
	} else {
		if existing, findErr := s.items.FindByHostCoreTypeTag(ctx, *req.AgentHostID, coreType, configType, tag); findErr == nil && existing != nil {
			return 0, 0, &InboundSpecConflictError{Kind: "tag", Field: "tag", Value: tag, ExistingSpecID: existing.ID}
		}
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	var desiredRevision int64 = 1
	if !isTemplate && *req.AgentHostID > 0 {
		if latest, err := s.compiler.GetLatestRevision(ctx, *req.AgentHostID, coreType); err == nil && latest > 0 {
			desiredRevision = latest + 1
		}
	}

	item := &repository.CoreConfigItem{
		AgentHostID:     req.AgentHostID,
		CoreType:        coreType,
		ConfigType:      configType,
		Tag:             tag,
		Enabled:         enabled,
		ConfigData:      configData,
		DesiredRevision: desiredRevision,
		CreatedBy:       req.OperatorID,
		UpdatedBy:       req.OperatorID,
	}
	if err := s.items.Create(ctx, item); err != nil {
		return 0, 0, err
	}

	if isTemplate {
		slog.Debug("template core config item created", "id", item.ID, "core_type", coreType, "config_type", configType, "tag", tag)
		return item.ID, 0, nil
	}

	slog.Debug("core config item created", "id", item.ID, "agent_host_id", *req.AgentHostID, "core_type", coreType, "config_type", configType, "tag", tag)
	if err := s.renderCoreConfigArtifacts(ctx, *req.AgentHostID, coreType); err != nil {
		return item.ID, item.DesiredRevision, err
	}
	return item.ID, item.DesiredRevision, nil
}

func (s *coreConfigItemService) update(ctx context.Context, req UpsertCoreConfigItemRequest) (int64, int64, error) {
	existing, err := s.items.FindByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, 0, ErrNotFound
		}
		return 0, 0, err
	}

	agentHostID := existing.AgentHostID
	if req.AgentHostID != nil && *req.AgentHostID > 0 {
		agentHostID = req.AgentHostID
	}

	coreType := existing.CoreType
	if strings.TrimSpace(req.CoreType) != "" {
		coreType = req.CoreType
	}

	tag := existing.Tag
	if strings.TrimSpace(req.Tag) != "" {
		tag = req.Tag
	}

	configType := existing.ConfigType
	if strings.TrimSpace(req.ConfigType) != "" {
		configType = req.ConfigType
	}

	configData := existing.ConfigData
	if len(strings.TrimSpace(string(req.ConfigData))) > 0 {
		configData = req.ConfigData
	}

	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	coreType = normalizeCoreType(coreType)
	tag = normalizeTag(tag)
	configType = strings.TrimSpace(configType)

	// Validate JSON
	var dummy any
	if err := json.Unmarshal(configData, &dummy); err != nil {
		validationErr := &InboundSpecValidationError{}
		validationErr.add("config_data", "must be valid JSON / 必须是合法 JSON")
		return 0, 0, validationErr
	}

	if agentHostID == nil {
		if found, tagErr := s.items.FindByCoreTypeTag(ctx, coreType, configType, tag); tagErr == nil && found != nil && found.ID != existing.ID {
			return 0, 0, &InboundSpecConflictError{Kind: "template_tag", Field: "tag", Value: tag, ExistingSpecID: found.ID}
		}
	} else {
		if found, tagErr := s.items.FindByHostCoreTypeTag(ctx, *agentHostID, coreType, configType, tag); tagErr == nil && found != nil && found.ID != existing.ID {
			return 0, 0, &InboundSpecConflictError{Kind: "tag", Field: "tag", Value: tag, ExistingSpecID: found.ID}
		}
	}

	existing.AgentHostID = agentHostID
	existing.CoreType = coreType
	existing.ConfigType = configType
	existing.Tag = tag
	existing.Enabled = enabled
	existing.ConfigData = configData
	existing.UpdatedBy = req.OperatorID

	if err := s.items.Update(ctx, existing); err != nil {
		return 0, 0, err
	}

	if agentHostID == nil {
		slog.Debug("template core config item updated", "id", existing.ID, "core_type", coreType, "config_type", configType, "tag", tag)
		return existing.ID, 0, nil
	}

	if err := s.renderCoreConfigArtifacts(ctx, *agentHostID, coreType); err != nil {
		return existing.ID, existing.DesiredRevision, err
	}
	return existing.ID, existing.DesiredRevision, nil
}


func (s *coreConfigItemService) renderCoreConfigArtifacts(ctx context.Context, agentHostID int64, coreType string) error {
	if s == nil || s.compiler == nil {
		return nil
	}
	// Items are already saved; compute global revision from artifacts table
	// to keep inbound + core config artifacts at the same revision
	revision := int64(1)
	if latest, err := s.compiler.GetLatestRevision(ctx, agentHostID, coreType); err == nil && latest > 0 {
		revision = latest + 1
	}
	_, err := s.compiler.RenderCoreConfigs(ctx, RenderArtifactsRequest{
		AgentHostID:     agentHostID,
		CoreType:        coreType,
		DesiredRevision: revision,
	})
	if err != nil {
		return fmt.Errorf("render core config artifacts: %w", err)
	}
	return nil
}

func (s *coreConfigItemService) List(ctx context.Context, filter ListCoreConfigItemFilter) ([]*repository.CoreConfigItem, int64, error) {
	if s == nil || s.items == nil {
		return nil, 0, fmt.Errorf("core config item service not configured")
	}

	repoFilter := repository.CoreConfigItemFilter{
		AgentHostID: filter.AgentHostID,
		ConfigType:  filter.ConfigType,
		Tag:         filter.Tag,
		Enabled:     filter.Enabled,
		IsTemplate:  filter.IsTemplate,
		Limit:       filter.Limit,
		Offset:      filter.Offset,
	}
	if filter.CoreType != nil {
		coreType := normalizeCoreType(*filter.CoreType)
		repoFilter.CoreType = &coreType
	}

	items, err := s.items.List(ctx, repoFilter)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.items.Count(ctx, repoFilter)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *coreConfigItemService) Delete(ctx context.Context, id int64) error {
	if s == nil || s.items == nil {
		return fmt.Errorf("core config item service not configured")
	}
	item, err := s.items.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	// Delete first, then render to ensure deletion is reflected in the rendered output.
	if item.AgentHostID != nil {
		if err := s.items.Delete(ctx, id); err != nil {
			return err
		}
		if err := s.renderCoreConfigArtifacts(ctx, safeDeref(item.AgentHostID), item.CoreType); err != nil {
			return err
		}
		return nil
	}
	return s.items.Delete(ctx, id)
}

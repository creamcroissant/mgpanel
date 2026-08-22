package service

import (
	"context"
	"fmt"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/template"
)

// ConfigTemplateService manages configuration templates.
type ConfigTemplateService interface {
	// CRUD operations
	Create(ctx context.Context, req CreateConfigTemplateRequest) (*repository.ConfigTemplate, error)
	Update(ctx context.Context, id int64, req UpdateConfigTemplateRequest) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*repository.ConfigTemplate, error)
	ListAll(ctx context.Context) ([]*repository.ConfigTemplate, error)

	// Validation and preview
	ValidateTemplate(ctx context.Context, content, templateType string) (*template.ValidationResult, error)
	PreviewRender(ctx context.Context, templateID int64) ([]byte, error)
}

// CreateConfigTemplateRequest contains data for creating a new config template.
type CreateConfigTemplateRequest struct {
	Name         string
	Type         string   // sing-box, xray
	Content      string   // Template content
	Description  string
	MinVersion   string   // Minimum core version required
	Capabilities []string // Required capabilities
}

// UpdateConfigTemplateRequest contains data for updating a config template.
type UpdateConfigTemplateRequest struct {
	Name         *string
	Type         *string
	Content      *string
	Description  *string
	MinVersion   *string
	Capabilities []string // nil means no change, empty slice clears
}

type configTemplateService struct {
	configTemplates repository.ConfigTemplateRepository
	engine          *template.Engine
	validator       *template.Validator
}

// NewConfigTemplateService creates a new config template service.
func NewConfigTemplateService(
	configTemplates repository.ConfigTemplateRepository,
) ConfigTemplateService {
	return &configTemplateService{
		configTemplates: configTemplates,
		engine:          template.NewEngine(),
		validator:       template.NewValidator(),
	}
}

func (s *configTemplateService) Create(ctx context.Context, req CreateConfigTemplateRequest) (*repository.ConfigTemplate, error) {
	// Validate template before creating
	validationResult := s.validator.ValidateTemplate(req.Content, req.Type)

	tpl := &repository.ConfigTemplate{
		Name:            req.Name,
		Type:            req.Type,
		Content:         req.Content,
		Description:     req.Description,
		MinVersion:      req.MinVersion,
		Capabilities:    req.Capabilities,
		SchemaVersion:   1,
		IsValid:         validationResult.Valid,
		ValidationError: "",
	}

	// Store validation error if any
	if !validationResult.Valid && len(validationResult.Errors) > 0 {
		tpl.ValidationError = validationResult.Errors[0]
	}

	// Ensure capabilities is not nil
	if tpl.Capabilities == nil {
		tpl.Capabilities = []string{}
	}

	if err := s.configTemplates.Create(ctx, tpl); err != nil {
		return nil, fmt.Errorf("create config template: %v / 创建配置模板失败: %w", err, err)
	}

	return tpl, nil
}

func (s *configTemplateService) Update(ctx context.Context, id int64, req UpdateConfigTemplateRequest) error {
	tpl, err := s.configTemplates.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Apply updates
	if req.Name != nil {
		tpl.Name = *req.Name
	}
	if req.Type != nil {
		tpl.Type = *req.Type
	}
	if req.Content != nil {
		tpl.Content = *req.Content
	}
	if req.Description != nil {
		tpl.Description = *req.Description
	}
	if req.MinVersion != nil {
		tpl.MinVersion = *req.MinVersion
	}
	if req.Capabilities != nil {
		tpl.Capabilities = req.Capabilities
	}

	// Re-validate if content or type changed
	if req.Content != nil || req.Type != nil {
		validationResult := s.validator.ValidateTemplate(tpl.Content, tpl.Type)
		tpl.IsValid = validationResult.Valid
		tpl.ValidationError = ""
		if !validationResult.Valid && len(validationResult.Errors) > 0 {
			tpl.ValidationError = validationResult.Errors[0]
		}
	}

	// Ensure capabilities is not nil
	if tpl.Capabilities == nil {
		tpl.Capabilities = []string{}
	}

	return s.configTemplates.Update(ctx, tpl)
}

func (s *configTemplateService) Delete(ctx context.Context, id int64) error {
	return s.configTemplates.Delete(ctx, id)
}

func (s *configTemplateService) FindByID(ctx context.Context, id int64) (*repository.ConfigTemplate, error) {
	return s.configTemplates.FindByID(ctx, id)
}

func (s *configTemplateService) ListAll(ctx context.Context) ([]*repository.ConfigTemplate, error) {
	return s.configTemplates.ListAll(ctx)
}

func (s *configTemplateService) ValidateTemplate(ctx context.Context, content, templateType string) (*template.ValidationResult, error) {
	result := s.validator.ValidateTemplate(content, templateType)
	return result, nil
}

func (s *configTemplateService) PreviewRender(ctx context.Context, templateID int64) ([]byte, error) {
	tpl, err := s.configTemplates.FindByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("find template: %v / 获取模板失败: %w", err, err)
	}

	// Use preview render with sample data
	output, err := s.engine.PreviewRender(tpl.Content)
	if err != nil {
		return nil, fmt.Errorf("preview render: %v / 模板预览渲染失败: %w", err, err)
	}

	return output, nil
}

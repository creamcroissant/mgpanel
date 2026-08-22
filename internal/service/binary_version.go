package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

const (
	BinaryVersionComponentAgent   = "agent"
	BinaryVersionComponentSingBox = "sing-box"
	BinaryVersionComponentXray    = "xray"

	BinaryVersionStatusInstalled = "installed"
	BinaryVersionStatusMissing   = "missing"
	BinaryVersionStatusOutdated  = "outdated"
	BinaryVersionStatusUpToDate  = "up_to_date"
	BinaryVersionStatusUnknown   = "unknown"
)

var binaryVersionComponents = []string{BinaryVersionComponentAgent, BinaryVersionComponentSingBox, BinaryVersionComponentXray}

var errBinaryVersionRemoteUnavailable = errors.New("binary version remote provider unavailable / 远端版本检查源不可用")

type BinaryVersionService interface {
	UpdateLocalVersions(ctx context.Context, req UpdateLocalVersionsRequest) error
	ListVersionStates(ctx context.Context, req ListVersionStatesRequest) ([]BinaryVersionStateView, error)
	RefreshRemoteVersion(ctx context.Context, req RefreshRemoteVersionRequest) (*BinaryVersionStateView, error)
}

type BinaryVersionRemoteProvider interface {
	LatestVersion(ctx context.Context, component string) (string, error)
}

type UpdateLocalVersionsRequest struct {
	AgentHostID     int64
	AgentVersion    string
	CurrentCoreType string
	CoreVersion     string
	Capabilities    []string
	BuildTags       []string
	CoreStates      []CoreVersionReport
}

type CoreVersionReport struct {
	Component    string
	Version      string
	Installed    *bool
	Capabilities []string
	BuildTags    []string
}

type ListVersionStatesRequest struct {
	AgentHostID int64
}

type RefreshRemoteVersionRequest struct {
	AgentHostID int64
	Component   string
}

type BinaryVersionStateView struct {
	ID             int64    `json:"id,omitempty"`
	AgentHostID    int64    `json:"agent_host_id"`
	Component      string   `json:"component"`
	LocalVersion   string   `json:"local_version"`
	RemoteVersion  string   `json:"remote_version,omitempty"`
	Status         string   `json:"status"`
	Capabilities   []string `json:"capabilities,omitempty"`
	BuildTags      []string `json:"build_tags,omitempty"`
	LastCheckedAt  int64    `json:"last_checked_at,omitempty"`
	LastCheckError string   `json:"last_check_error,omitempty"`
	UpdatedAt      int64    `json:"updated_at,omitempty"`
}

type BinaryVersionServiceOptions struct {
	Now            func() time.Time
	CoreOperations repository.CoreOperationRepository
}

type binaryVersionService struct {
	versions       repository.BinaryVersionStateRepository
	hosts          repository.AgentHostRepository
	provider       BinaryVersionRemoteProvider
	coreOperations repository.CoreOperationRepository
	now            func() time.Time
}

// staticBinaryVersionProvider holds a read-only version map populated at startup.
type staticBinaryVersionProvider struct {
	versions map[string]string
}

func NewBinaryVersionService(versions repository.BinaryVersionStateRepository, hosts repository.AgentHostRepository, provider BinaryVersionRemoteProvider) BinaryVersionService {
	return NewBinaryVersionServiceWithOptions(versions, hosts, provider, BinaryVersionServiceOptions{})
}

func NewBinaryVersionServiceWithOptions(versions repository.BinaryVersionStateRepository, hosts repository.AgentHostRepository, provider BinaryVersionRemoteProvider, opts BinaryVersionServiceOptions) BinaryVersionService {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &binaryVersionService{versions: versions, hosts: hosts, provider: provider, coreOperations: opts.CoreOperations, now: now}
}

func NewStaticBinaryVersionProvider(versions map[string]string) BinaryVersionRemoteProvider {
	copied := make(map[string]string, len(versions))
	for component, version := range versions {
		normalized, err := NormalizeBinaryVersionComponent(component)
		if err != nil {
			continue
		}
		if version = strings.TrimSpace(version); version != "" {
			copied[normalized] = version
		}
	}
	return &staticBinaryVersionProvider{versions: copied}
}

func (p *staticBinaryVersionProvider) LatestVersion(ctx context.Context, component string) (string, error) {
	normalized, err := NormalizeBinaryVersionComponent(component)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(p.versions[normalized])
	if version == "" {
		return "", errBinaryVersionRemoteUnavailable
	}
	return version, nil
}

func (s *binaryVersionService) UpdateLocalVersions(ctx context.Context, req UpdateLocalVersionsRequest) error {
	if s == nil || s.versions == nil || s.hosts == nil {
		return ErrNotImplemented
	}
	if req.AgentHostID <= 0 {
		return ErrBadRequest
	}
	if _, err := s.findHost(ctx, req.AgentHostID); err != nil {
		return err
	}

	if agentVersion := strings.TrimSpace(req.AgentVersion); agentVersion != "" {
		if err := s.upsertLocalState(ctx, CoreVersionReport{Component: BinaryVersionComponentAgent, Version: agentVersion}, req.AgentHostID); err != nil {
			return err
		}
	}

	coreReports := append([]CoreVersionReport(nil), req.CoreStates...)
	if strings.TrimSpace(req.CurrentCoreType) != "" && strings.TrimSpace(req.CoreVersion) != "" {
		coreReports = append(coreReports, CoreVersionReport{Component: req.CurrentCoreType, Version: req.CoreVersion, Capabilities: req.Capabilities, BuildTags: req.BuildTags})
	}

	seen := make(map[string]struct{}, len(coreReports))
	for _, report := range coreReports {
		report.Version = strings.TrimSpace(report.Version)
		explicitlyMissing := report.Installed != nil && !*report.Installed
		if report.Version == "" && !explicitlyMissing {
			continue
		}
		component, err := NormalizeBinaryVersionComponent(report.Component)
		if err != nil {
			return err
		}
		if component == BinaryVersionComponentAgent {
			continue
		}
		key := component + "\x00" + report.Version
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		report.Component = component
		if err := s.upsertLocalState(ctx, report, req.AgentHostID); err != nil {
			return err
		}
	}
	return nil
}

func (s *binaryVersionService) ListVersionStates(ctx context.Context, req ListVersionStatesRequest) ([]BinaryVersionStateView, error) {
	if s == nil || s.versions == nil || s.hosts == nil {
		return nil, ErrNotImplemented
	}
	if req.AgentHostID <= 0 {
		return nil, ErrBadRequest
	}
	host, err := s.findHost(ctx, req.AgentHostID)
	if err != nil {
		return nil, err
	}
	states, err := s.versions.List(ctx, repository.BinaryVersionFilter{AgentHostID: &req.AgentHostID, Limit: len(binaryVersionComponents)})
	if err != nil {
		return nil, err
	}
	byComponent := make(map[string]*repository.BinaryVersionState, len(states))
	for _, state := range states {
		if state == nil {
			continue
		}
		component, err := NormalizeBinaryVersionComponent(state.Component)
		if err != nil {
			continue
		}
		byComponent[component] = state
	}
	s.applyCompletedCoreOperationFallbacks(ctx, req.AgentHostID, byComponent)

	result := make([]BinaryVersionStateView, 0, len(binaryVersionComponents))
	for _, component := range binaryVersionComponents {
		if state, ok := byComponent[component]; ok {
			result = append(result, binaryVersionStateViewFromRepo(state))
			continue
		}
		result = append(result, binaryVersionStateViewFromRepo(localStateFromHost(host, component, s.now().Unix())))
	}
	return result, nil
}

func (s *binaryVersionService) applyCompletedCoreOperationFallbacks(ctx context.Context, agentHostID int64, byComponent map[string]*repository.BinaryVersionState) {
	if s == nil || s.coreOperations == nil {
		return
	}
	operationType := "install"
	status := "completed"
	operations, err := s.coreOperations.List(ctx, repository.CoreOperationFilter{
		AgentHostID:   &agentHostID,
		OperationType: &operationType,
		Status:        &status,
		Limit:         50,
	})
	if err != nil {
		return
	}
	seen := make(map[string]struct{}, len(binaryVersionComponents))
	for _, operation := range operations {
		report, ok := coreOperationVersionReport(operation)
		if !ok {
			continue
		}
		component, err := NormalizeBinaryVersionComponent(report.Component)
		if err != nil || component == BinaryVersionComponentAgent {
			continue
		}
		if _, ok := seen[component]; ok {
			continue
		}
		report.Component = component
		state := byComponent[component]
		if !shouldApplyCoreOperationFallback(state, operation, report) {
			seen[component] = struct{}{}
			continue
		}
		if err := s.upsertLocalState(ctx, report, agentHostID); err != nil {
			seen[component] = struct{}{}
			continue
		}
		if updated, err := s.versions.FindByHostComponent(ctx, agentHostID, component); err == nil {
			byComponent[component] = updated
		}
		seen[component] = struct{}{}
	}
}

func coreOperationVersionReport(operation *repository.CoreOperation) (CoreVersionReport, bool) {
	if operation == nil || strings.TrimSpace(operation.OperationType) != "install" || strings.TrimSpace(operation.Status) != "completed" {
		return CoreVersionReport{}, false
	}
	var payload agentv1.InstallCorePayload
	if len(operation.RequestPayload) > 0 {
		if err := json.Unmarshal(operation.RequestPayload, &payload); err != nil {
			slog.Debug("failed to parse install core request payload", "error", err)
		}
	}
	var result agentv1.InstallCoreResponse
	if len(operation.ResultPayload) > 0 {
		if err := json.Unmarshal(operation.ResultPayload, &result); err != nil {
				slog.Warn("binary_version: parse ResultPayload failed", "error", err)
			}
	}
	component := strings.TrimSpace(result.GetCoreType())
	if component == "" {
		component = strings.TrimSpace(operation.CoreType)
	}
	if component == "" {
		return CoreVersionReport{}, false
	}
	action := strings.ToLower(strings.TrimSpace(payload.GetAction()))
	if action == "" {
		action = "install"
	}
	installed := action != "uninstall"
	version := strings.TrimSpace(result.GetVersion())
	if !installed {
		version = ""
	} else if version == "" {
		return CoreVersionReport{}, false
	}
	return CoreVersionReport{Component: component, Version: version, Installed: &installed}, true
}

func shouldApplyCoreOperationFallback(state *repository.BinaryVersionState, operation *repository.CoreOperation, report CoreVersionReport) bool {
	if state == nil {
		return true
	}
	installed := report.Installed == nil || *report.Installed
	localVersion := strings.TrimSpace(state.LocalVersion)
	if installed && localVersion == "" {
		return true
	}
	operationTime := operation.UpdatedAt
	if operation.FinishedAt != nil {
		operationTime = *operation.FinishedAt
	}
	if operationTime > 0 && state.UpdatedAt > operationTime {
		return false
	}
	if !installed {
		return localVersion != ""
	}
	return localVersion != strings.TrimSpace(report.Version)
}

func (s *binaryVersionService) RefreshRemoteVersion(ctx context.Context, req RefreshRemoteVersionRequest) (*BinaryVersionStateView, error) {
	if s == nil || s.versions == nil || s.hosts == nil {
		return nil, ErrNotImplemented
	}
	if req.AgentHostID <= 0 {
		return nil, ErrBadRequest
	}
	component, err := NormalizeBinaryVersionComponent(req.Component)
	if err != nil {
		return nil, err
	}
	host, err := s.findHost(ctx, req.AgentHostID)
	if err != nil {
		return nil, err
	}
	state, err := s.findOrCreateState(ctx, host, component)
	if err != nil {
		return nil, err
	}
	byComponent := map[string]*repository.BinaryVersionState{component: state}
	s.applyCompletedCoreOperationFallbacks(ctx, req.AgentHostID, byComponent)
	if updated := byComponent[component]; updated != nil {
		state = updated
	}

	checkedAt := s.now().Unix()
	if s.provider == nil {
		return s.recordRefreshFailure(ctx, state, errBinaryVersionRemoteUnavailable.Error(), checkedAt)
	}
	remoteVersion, err := s.provider.LatestVersion(ctx, component)
	remoteVersion = strings.TrimSpace(remoteVersion)
	if err != nil {
		return s.recordRefreshFailure(ctx, state, err.Error(), checkedAt)
	}
	if remoteVersion == "" {
		return s.recordRefreshFailure(ctx, state, errBinaryVersionRemoteUnavailable.Error(), checkedAt)
	}

	status := calculateBinaryVersionStatus(component, state.LocalVersion, remoteVersion)
	if err := s.versions.UpdateCheckResult(ctx, state.AgentHostID, component, remoteVersion, status, "", checkedAt); err != nil {
		return nil, mapBinaryVersionRepoError(err)
	}
	updated, err := s.versions.FindByHostComponent(ctx, state.AgentHostID, component)
	if err != nil {
		return nil, mapBinaryVersionRepoError(err)
	}
	view := binaryVersionStateViewFromRepo(updated)
	return &view, nil
}

func (s *binaryVersionService) findHost(ctx context.Context, agentHostID int64) (*repository.AgentHost, error) {
	host, err := s.hosts.FindByID(ctx, agentHostID)
	if err != nil {
		return nil, mapBinaryVersionRepoError(err)
	}
	return host, nil
}

func (s *binaryVersionService) upsertLocalState(ctx context.Context, report CoreVersionReport, agentHostID int64) error {
	component, err := NormalizeBinaryVersionComponent(report.Component)
	if err != nil {
		return err
	}
	existing, err := s.versions.FindByHostComponent(ctx, agentHostID, component)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return mapBinaryVersionRepoError(err)
	}
	remoteVersion := ""
	if existing != nil {
		remoteVersion = existing.RemoteVersion
	}
	_, err = s.versions.Upsert(ctx, &repository.BinaryVersionState{
		AgentHostID:      agentHostID,
		Component:        component,
		LocalVersion:     strings.TrimSpace(report.Version),
		Status:           calculateBinaryVersionStatus(component, report.Version, remoteVersion),
		CapabilitiesJSON: marshalStringSlice(report.Capabilities),
		BuildTagsJSON:    marshalStringSlice(report.BuildTags),
		UpdatedAt:        s.now().Unix(),
	})
	if err != nil {
		return mapBinaryVersionRepoError(err)
	}
	return nil
}

func (s *binaryVersionService) findOrCreateState(ctx context.Context, host *repository.AgentHost, component string) (*repository.BinaryVersionState, error) {
	state, err := s.versions.FindByHostComponent(ctx, host.ID, component)
	if err == nil {
		return state, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, mapBinaryVersionRepoError(err)
	}
	state = localStateFromHost(host, component, s.now().Unix())
	created, err := s.versions.Upsert(ctx, state)
	if err != nil {
		return nil, mapBinaryVersionRepoError(err)
	}
	return created, nil
}

func (s *binaryVersionService) recordRefreshFailure(ctx context.Context, state *repository.BinaryVersionState, message string, checkedAt int64) (*BinaryVersionStateView, error) {
	if strings.TrimSpace(message) == "" {
		message = errBinaryVersionRemoteUnavailable.Error()
	}
	if err := s.versions.UpdateCheckResult(ctx, state.AgentHostID, state.Component, "", "", message, checkedAt); err != nil {
		return nil, mapBinaryVersionRepoError(err)
	}
	updated, err := s.versions.FindByHostComponent(ctx, state.AgentHostID, state.Component)
	if err != nil {
		return nil, mapBinaryVersionRepoError(err)
	}
	view := binaryVersionStateViewFromRepo(updated)
	return &view, nil
}

func NormalizeBinaryVersionComponent(component string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(component))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	switch normalized {
	case BinaryVersionComponentAgent:
		return BinaryVersionComponentAgent, nil
	case "singbox", BinaryVersionComponentSingBox:
		return BinaryVersionComponentSingBox, nil
	case BinaryVersionComponentXray:
		return BinaryVersionComponentXray, nil
	default:
		return "", ErrBadRequest
	}
}

func localStateFromHost(host *repository.AgentHost, component string, now int64) *repository.BinaryVersionState {
	state := &repository.BinaryVersionState{AgentHostID: host.ID, Component: component, Status: BinaryVersionStatusMissing, CapabilitiesJSON: "[]", BuildTagsJSON: "[]", UpdatedAt: now}
	switch component {
	case BinaryVersionComponentAgent:
		state.LocalVersion = strings.TrimSpace(host.AgentVersion)
	case BinaryVersionComponentSingBox, BinaryVersionComponentXray:
		currentCoreType, err := NormalizeBinaryVersionComponent(host.CurrentCoreType)
		if err == nil && currentCoreType == component {
			state.LocalVersion = strings.TrimSpace(host.CoreVersion)
			state.CapabilitiesJSON = marshalStringSlice(host.Capabilities)
			state.BuildTagsJSON = marshalStringSlice(host.BuildTags)
		}
	}
	state.Status = calculateBinaryVersionStatus(component, state.LocalVersion, state.RemoteVersion)
	return state
}

func binaryVersionStateViewFromRepo(state *repository.BinaryVersionState) BinaryVersionStateView {
	if state == nil {
		return BinaryVersionStateView{Status: BinaryVersionStatusUnknown}
	}
	return BinaryVersionStateView{
		ID:             state.ID,
		AgentHostID:    state.AgentHostID,
		Component:      state.Component,
		LocalVersion:   state.LocalVersion,
		RemoteVersion:  state.RemoteVersion,
		Status:         state.Status,
		Capabilities:   unmarshalStringSlice(state.CapabilitiesJSON),
		BuildTags:      unmarshalStringSlice(state.BuildTagsJSON),
		LastCheckedAt:  state.LastCheckedAt,
		LastCheckError: state.LastCheckError,
		UpdatedAt:      state.UpdatedAt,
	}
}

func calculateBinaryVersionStatus(component, localVersion, remoteVersion string) string {
	localVersion = strings.TrimSpace(localVersion)
	remoteVersion = strings.TrimSpace(remoteVersion)
	if localVersion == "" {
		if component == BinaryVersionComponentAgent {
			return BinaryVersionStatusUnknown
		}
		return BinaryVersionStatusMissing
	}
	if remoteVersion == "" {
		return BinaryVersionStatusInstalled
	}
	if comparison, ok := compareBinaryVersions(localVersion, remoteVersion); ok {
		if comparison < 0 {
			return BinaryVersionStatusOutdated
		}
		return BinaryVersionStatusUpToDate
	}
	return BinaryVersionStatusUnknown
}

func compareBinaryVersions(left, right string) (int, bool) {
	leftParts, ok := parseBinaryVersion(left)
	if !ok {
		return 0, false
	}
	rightParts, ok := parseBinaryVersion(right)
	if !ok {
		return 0, false
	}
	for i := 0; i < len(leftParts); i++ {
		if leftParts[i] < rightParts[i] {
			return -1, true
		}
		if leftParts[i] > rightParts[i] {
			return 1, true
		}
	}
	return 0, true
}

func parseBinaryVersion(version string) ([3]int, bool) {
	var parts [3]int
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	start := -1
	for i, r := range version {
		if r >= '0' && r <= '9' {
			start = i
			break
		}
	}
	if start < 0 {
		return parts, false
	}
	segments := strings.Split(version[start:], ".")
	for i := 0; i < len(segments) && i < len(parts); i++ {
		segment := leadingDigits(segments[i])
		if segment == "" {
			if i == 0 {
				return parts, false
			}
			break
		}
		value, err := strconv.Atoi(segment)
		if err != nil {
			return parts, false
		}
		parts[i] = value
	}
	return parts, true
}

func leadingDigits(value string) string {
	for i, r := range value {
		if r < '0' || r > '9' {
			return value[:i]
		}
	}
	return value
}

func marshalStringSlice(values []string) string {
	cleaned := cleanStringSlice(values)
	if len(cleaned) == 0 {
		return "[]"
	}
	data, err := json.Marshal(cleaned)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func unmarshalStringSlice(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return cleanStringSlice(values)
}

func cleanStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func mapBinaryVersionRepoError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("binary version state: %w", err)
}

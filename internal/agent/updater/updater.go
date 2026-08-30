package updater

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	DefaultReleaseBaseURL = "https://github.com"
	DefaultReleaseRepo    = "creamcroissant/xboard2p"
	DefaultReleaseTag     = "latest"

	StatusIdle          = "idle"
	StatusChecked       = "checked"
	StatusInProgress    = "in_progress"
	StatusPendingHealth = "pending_health"
	StatusHealthy       = "healthy"
	StatusFailed        = "failed"
	StatusRolledBack    = "rolled_back"

	PhaseChecking        = "checking"
	PhaseCompatible      = "compatible"
	PhaseUpToDate        = "up_to_date"
	PhaseLocked          = "locked"
	PhaseJitter          = "jitter"
	PhaseDownloading     = "downloading"
	PhaseBackingUp       = "backing_up"
	PhaseReplacing       = "replacing"
	PhaseHealthPending   = "health_pending"
	PhaseHealthConfirmed = "health_confirmed"
	PhaseRollback        = "rollback"
)

const (
	defaultHealthTimeout    = 2 * time.Minute
	defaultMaxCrashCount    = 3
	defaultJitterMax        = 30 * time.Second
	defaultMaxDownloadBytes = 200 * 1024 * 1024
)

var (
	ErrInvalidConfig       = errors.New("updater: invalid config")
	ErrInvalidRequest      = errors.New("updater: invalid request")
	ErrChecksumMismatch    = errors.New("updater: checksum mismatch")
	ErrLockedBadVersion    = errors.New("updater: target version is locked")
	ErrRollbackUnavailable = errors.New("updater: rollback unavailable")
)

type Config struct {
	AutoEnabled      bool
	CurrentVersion   string
	BinaryPath       string
	StatePath        string
	BackupDir        string
	ReleaseBaseURL   string
	ReleaseRepo      string
	ReleaseTag       string
	OS               string
	Arch             string
	HealthTimeout    time.Duration
	MaxCrashCount    int
	JitterMin        time.Duration
	JitterMax        time.Duration
	MaxDownloadBytes int64
	HTTPClient       *http.Client
	Now              func() time.Time
}

type CheckRequest struct {
	TargetVersion  string `json:"target_version"`
	ReleaseTag     string `json:"release_tag"`
	ReleaseRepo    string `json:"release_repo"`
	ReleaseBaseURL string `json:"release_base_url"`
	AssetName      string `json:"asset_name"`
	AssetURL       string `json:"asset_url"`
	ChecksumURL    string `json:"checksum_url"`
	SHA256         string `json:"sha256"`
}

type UpdateRequest struct {
	CheckRequest
	JitterMinSeconds int64 `json:"jitter_min_seconds"`
	JitterMaxSeconds int64 `json:"jitter_max_seconds"`
}

type CheckResult struct {
	CurrentVersion string `json:"current_version"`
	TargetVersion  string `json:"target_version"`
	ReleaseTag     string `json:"release_tag"`
	AssetName      string `json:"asset_name"`
	Checksum       string `json:"checksum"`
	Compatible     bool   `json:"compatible"`
	UpToDate       bool   `json:"up_to_date"`
	Locked         bool   `json:"locked"`
}

type UpdateResult struct {
	CurrentVersion    string `json:"current_version"`
	TargetVersion     string `json:"target_version"`
	PreviousVersion   string `json:"previous_version"`
	BackupPath        string `json:"backup_path,omitempty"`
	HealthDeadlineAt  int64  `json:"health_deadline_at"`
	RollbackAvailable bool   `json:"rollback_available"`
}

type Event struct {
	Status  string
	Phase   string
	Level   string
	Message string
	Payload []byte
}

type ProgressFunc func(ctx context.Context, event Event) error

type Status struct {
	CurrentVersion    string `json:"current_version"`
	TargetVersion     string `json:"target_version"`
	Status            string `json:"status"`
	Phase             string `json:"phase"`
	PreviousVersion   string `json:"previous_version"`
	ErrorMessage      string `json:"error_message"`
	StartedAt         int64  `json:"started_at"`
	FinishedAt        int64  `json:"finished_at"`
	LastCheckedAt     int64  `json:"last_checked_at"`
	LastCheckError    string `json:"last_check_error"`
	RollbackAvailable bool   `json:"rollback_available"`
	RolledBack        bool   `json:"rolled_back"`
	LockedBadVersion  string `json:"locked_bad_version"`
	CrashCount        int32  `json:"crash_count"`
	HealthDeadlineAt  int64  `json:"health_deadline_at"`
}

type state struct {
	Status
	BackupPath string `json:"backup_path,omitempty"`
	BinaryPath string `json:"binary_path,omitempty"`
}

type Updater struct {
	cfg    Config
	client *http.Client
	now    func() time.Time

	mu    sync.Mutex
	state state
}

type releaseSource struct {
	TargetVersion string
	ReleaseTag    string
	AssetName     string
	AssetURL      string
	ChecksumURL   string
	SHA256        string
}

func New(cfg Config) (*Updater, error) {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	u := &Updater{cfg: normalized, client: normalized.HTTPClient, now: normalized.Now}
	if u.client == nil {
		u.client = &http.Client{Timeout: 5 * time.Minute}
	}
	if u.now == nil {
		u.now = time.Now
	}
	if err := u.loadState(); err != nil {
		return nil, err
	}
	return u, nil
}

func (u *Updater) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.statusLocked()
}

func (u *Updater) Check(ctx context.Context, req CheckRequest) (*CheckResult, error) {
	source, err := u.resolveSource(req)
	if err != nil {
		u.recordCheckFailure(err)
		return nil, err
	}
	if source.SHA256 == "" {
		source.SHA256, err = u.fetchExpectedChecksum(ctx, source)
		if err != nil {
			u.recordCheckFailure(err)
			return nil, err
		}
	}

	u.mu.Lock()
	locked := sameVersion(source.TargetVersion, u.state.LockedBadVersion)
	current := firstNonEmpty(u.state.CurrentVersion, u.cfg.CurrentVersion)
	upToDate := source.TargetVersion != "" && sameVersion(source.TargetVersion, current)
	result := &CheckResult{
		CurrentVersion: current,
		TargetVersion:  source.TargetVersion,
		ReleaseTag:     source.ReleaseTag,
		AssetName:      source.AssetName,
		Checksum:       source.SHA256,
		Compatible:     !locked && !upToDate,
		UpToDate:       upToDate,
		Locked:         locked,
	}
	now := u.now().Unix()
	u.state.CurrentVersion = current
	u.state.TargetVersion = source.TargetVersion
	u.state.Status.Status = StatusChecked
	u.state.LastCheckedAt = now
	u.state.LastCheckError = ""
	switch {
	case locked:
		u.state.Phase = PhaseLocked
	case upToDate:
		u.state.Phase = PhaseUpToDate
	default:
		u.state.Phase = PhaseCompatible
	}
	err = u.saveStateLocked()
	u.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (u *Updater) Update(ctx context.Context, req UpdateRequest, progress ProgressFunc) (*UpdateResult, error) {
	source, err := u.resolveSource(req.CheckRequest)
	if err != nil {
		u.fail(err, "resolve")
		return nil, err
	}
	if err := u.ensureTargetAllowed(source.TargetVersion); err != nil {
		u.fail(err, PhaseLocked)
		return nil, err
	}
	if err := emit(ctx, progress, Event{Status: StatusInProgress, Phase: PhaseChecking, Level: "info", Message: "agent update check started"}); err != nil {
		return nil, err
	}
	if source.SHA256 == "" {
		source.SHA256, err = u.fetchExpectedChecksum(ctx, source)
		if err != nil {
			u.fail(err, PhaseChecking)
			return nil, err
		}
	}
	if delay := u.updateJitter(req); delay > 0 {
		payload, _ := json.Marshal(map[string]int64{"delay_seconds": int64(delay.Seconds())})
		if err := emit(ctx, progress, Event{Status: StatusInProgress, Phase: PhaseJitter, Level: "info", Message: "agent update jitter delay", Payload: payload}); err != nil {
			return nil, err
		}
		if err := waitContext(ctx, delay); err != nil {
			u.fail(err, PhaseJitter)
			return nil, err
		}
	}

	u.beginUpdate(source)
	if err := emit(ctx, progress, Event{Status: StatusInProgress, Phase: PhaseDownloading, Level: "info", Message: "agent update download started"}); err != nil {
		return nil, err
	}
	tmpPath, err := u.downloadToTemp(ctx, source)
	if err != nil {
		u.fail(err, PhaseDownloading)
		return nil, err
	}
	defer os.Remove(tmpPath)

	if err := verifyFileChecksum(tmpPath, source.SHA256); err != nil {
		u.fail(err, PhaseDownloading)
		return nil, err
	}
	if err := emit(ctx, progress, Event{Status: StatusInProgress, Phase: PhaseBackingUp, Level: "info", Message: "agent binary backup started"}); err != nil {
		return nil, err
	}
	backupPath, err := u.backupCurrentBinary()
	if err != nil {
		u.fail(err, PhaseBackingUp)
		return nil, err
	}
	if err := emit(ctx, progress, Event{Status: StatusInProgress, Phase: PhaseReplacing, Level: "info", Message: "agent binary replacement started"}); err != nil {
		return nil, err
	}
	if err := replaceBinary(tmpPath, u.cfg.BinaryPath); err != nil {
		u.fail(err, PhaseReplacing)
		return nil, err
	}

	result, err := u.markPendingHealth(source, backupPath)
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(result)
	if err := emit(ctx, progress, Event{Status: StatusPendingHealth, Phase: PhaseHealthPending, Level: "info", Message: "agent update waiting for health confirmation", Payload: payload}); err != nil {
		return nil, err
	}
	return result, nil
}

func (u *Updater) MarkHealthy() error {
	u.mu.Lock()
	if u.state.Status.Status != StatusPendingHealth {
		u.mu.Unlock()
		return nil
	}
	backupPath := u.state.BackupPath
	u.state.CurrentVersion = firstNonEmpty(u.state.TargetVersion, u.state.CurrentVersion, u.cfg.CurrentVersion)
	u.state.TargetVersion = ""
	u.state.PreviousVersion = ""
	u.state.Status.Status = StatusHealthy
	u.state.Phase = PhaseHealthConfirmed
	u.state.ErrorMessage = ""
	u.state.FinishedAt = u.now().Unix()
	u.state.RollbackAvailable = false
	u.state.RolledBack = false
	u.state.CrashCount = 0
	u.state.HealthDeadlineAt = 0
	u.state.BackupPath = ""
	err := u.saveStateLocked()
	u.mu.Unlock()
	if err != nil {
		return err
	}
	if backupPath != "" {
		_ = os.Remove(backupPath)
	}
	return nil
}

func (u *Updater) RecordStartup() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.state.Status.Status != StatusPendingHealth {
		return nil
	}
	u.state.CrashCount++
	if int(u.state.CrashCount) >= u.cfg.MaxCrashCount {
		return u.rollbackLocked(true, "startup crash threshold reached")
	}
	return u.saveStateLocked()
}

func (u *Updater) RollbackIfHealthExpired() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.state.Status.Status != StatusPendingHealth || u.state.HealthDeadlineAt <= 0 || u.now().Unix() <= u.state.HealthDeadlineAt {
		return nil
	}
	return u.rollbackLocked(false, "health confirmation timed out")
}

func (u *Updater) Rollback() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.rollbackLocked(false, "manual rollback")
}

func (u *Updater) JitterDelay() time.Duration {
	return randomDuration(u.cfg.JitterMin, u.cfg.JitterMax)
}

func (u *Updater) loadState() error {
	data, err := os.ReadFile(u.cfg.StatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			u.state = state{Status: Status{CurrentVersion: u.cfg.CurrentVersion, Status: StatusIdle}, BinaryPath: u.cfg.BinaryPath}
			return nil
		}
		return fmt.Errorf("read updater state: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		u.state = state{Status: Status{CurrentVersion: u.cfg.CurrentVersion, Status: StatusIdle}, BinaryPath: u.cfg.BinaryPath}
		return nil
	}
	var st state
	if err := json.Unmarshal(data, &st); err != nil {
		return fmt.Errorf("parse updater state: %w", err)
	}
	if st.CurrentVersion == "" {
		st.CurrentVersion = u.cfg.CurrentVersion
	}
	if st.Status.Status == "" {
		st.Status.Status = StatusIdle
	}
	if st.BinaryPath == "" {
		st.BinaryPath = u.cfg.BinaryPath
	}
	u.state = st
	return nil
}

func (u *Updater) statusLocked() Status {
	status := u.state.Status
	if status.CurrentVersion == "" {
		status.CurrentVersion = u.cfg.CurrentVersion
	}
	if status.Status == "" {
		status.Status = StatusIdle
	}
	return status
}

func (u *Updater) saveStateLocked() error {
	if err := os.MkdirAll(filepath.Dir(u.cfg.StatePath), 0o700); err != nil {
		return fmt.Errorf("create updater state dir: %w", err)
	}
	data, err := json.MarshalIndent(u.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal updater state: %w", err)
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(u.cfg.StatePath), ".agent-update-state-*")
	if err != nil {
		return fmt.Errorf("create updater state temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set updater state permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write updater state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close updater state: %w", err)
	}
	if err := os.Rename(tmpPath, u.cfg.StatePath); err != nil {
		return fmt.Errorf("replace updater state: %w", err)
	}
	return nil
}

func (u *Updater) recordCheckFailure(err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.state.LastCheckedAt = u.now().Unix()
	u.state.LastCheckError = err.Error()
	u.state.Status.Status = StatusFailed
	u.state.Phase = PhaseChecking
	u.state.ErrorMessage = err.Error()
	_ = u.saveStateLocked()
}

func (u *Updater) ensureTargetAllowed(target string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if sameVersion(target, u.state.LockedBadVersion) {
		return ErrLockedBadVersion
	}
	return nil
}

func (u *Updater) beginUpdate(source releaseSource) {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := u.now().Unix()
	u.state.CurrentVersion = firstNonEmpty(u.state.CurrentVersion, u.cfg.CurrentVersion)
	u.state.TargetVersion = source.TargetVersion
	u.state.Status.Status = StatusInProgress
	u.state.Phase = PhaseDownloading
	u.state.ErrorMessage = ""
	u.state.StartedAt = now
	u.state.FinishedAt = 0
	u.state.RolledBack = false
	u.state.BinaryPath = u.cfg.BinaryPath
	_ = u.saveStateLocked()
}

func (u *Updater) fail(updateErr error, phase string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.state.Status.Status = StatusFailed
	u.state.Phase = phase
	u.state.ErrorMessage = updateErr.Error()
	u.state.FinishedAt = u.now().Unix()
	_ = u.saveStateLocked()
}

func (u *Updater) markPendingHealth(source releaseSource, backupPath string) (*UpdateResult, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := u.now()
	previous := firstNonEmpty(u.state.CurrentVersion, u.cfg.CurrentVersion)
	deadline := now.Add(u.cfg.HealthTimeout).Unix()
	u.state.PreviousVersion = previous
	u.state.TargetVersion = source.TargetVersion
	u.state.Status.Status = StatusPendingHealth
	u.state.Phase = PhaseHealthPending
	u.state.ErrorMessage = ""
	u.state.BackupPath = backupPath
	u.state.BinaryPath = u.cfg.BinaryPath
	u.state.RollbackAvailable = true
	u.state.RolledBack = false
	u.state.CrashCount = 0
	u.state.HealthDeadlineAt = deadline
	if err := u.saveStateLocked(); err != nil {
		return nil, err
	}
	return &UpdateResult{CurrentVersion: previous, TargetVersion: source.TargetVersion, PreviousVersion: previous, BackupPath: backupPath, HealthDeadlineAt: deadline, RollbackAvailable: true}, nil
}

func (u *Updater) rollbackLocked(lockBad bool, message string) error {
	if u.state.BackupPath == "" {
		return ErrRollbackUnavailable
	}
	if _, err := os.Stat(u.state.BackupPath); err != nil {
		return fmt.Errorf("rollback backup missing: %w", err)
	}
	binaryPath := firstNonEmpty(u.state.BinaryPath, u.cfg.BinaryPath)
	if err := copyToPathAtomically(u.state.BackupPath, binaryPath); err != nil {
		u.state.Status.Status = StatusFailed
		u.state.Phase = PhaseRollback
		u.state.ErrorMessage = err.Error()
		u.state.FinishedAt = u.now().Unix()
		_ = u.saveStateLocked()
		return err
	}
	if lockBad && u.state.TargetVersion != "" {
		u.state.LockedBadVersion = u.state.TargetVersion
	}
	u.state.CurrentVersion = firstNonEmpty(u.state.PreviousVersion, u.cfg.CurrentVersion)
	u.state.TargetVersion = ""
	u.state.Status.Status = StatusRolledBack
	u.state.Phase = PhaseRollback
	u.state.ErrorMessage = message
	u.state.FinishedAt = u.now().Unix()
	u.state.RollbackAvailable = false
	u.state.RolledBack = true
	u.state.CrashCount = 0
	u.state.HealthDeadlineAt = 0
	return u.saveStateLocked()
}

func (u *Updater) resolveSource(req CheckRequest) (releaseSource, error) {
	base := firstNonEmpty(req.ReleaseBaseURL, u.cfg.ReleaseBaseURL)
	repo := strings.Trim(firstNonEmpty(req.ReleaseRepo, u.cfg.ReleaseRepo), "/")
	tag := firstNonEmpty(req.ReleaseTag, u.cfg.ReleaseTag)
	assetName := firstNonEmpty(req.AssetName, buildAssetName(u.cfg.OS, u.cfg.Arch))
	if base == "" || repo == "" || tag == "" || assetName == "" {
		return releaseSource{}, fmt.Errorf("%w: release source is incomplete", ErrInvalidRequest)
	}
	assetURL := strings.TrimSpace(req.AssetURL)
	checksumURL := strings.TrimSpace(req.ChecksumURL)
	if assetURL == "" || checksumURL == "" {
		assetURL, checksumURL = buildReleaseURLs(base, repo, tag, assetName)
	}
	if err := validateTrustedURL(assetURL, base); err != nil {
		return releaseSource{}, err
	}
	if err := validateTrustedURL(checksumURL, base); err != nil {
		return releaseSource{}, err
	}
	target := firstNonEmpty(req.TargetVersion, tag)
	return releaseSource{TargetVersion: target, ReleaseTag: tag, AssetName: assetName, AssetURL: assetURL, ChecksumURL: checksumURL, SHA256: normalizeChecksum(req.SHA256)}, nil
}

func (u *Updater) fetchExpectedChecksum(ctx context.Context, source releaseSource) (string, error) {
	data, err := u.fetchBytes(ctx, source.ChecksumURL, 2*1024*1024)
	if err != nil {
		return "", err
	}
	checksum := lookupExpectedChecksum(source.AssetName, string(data))
	if checksum == "" {
		return "", fmt.Errorf("%w: checksum entry not found for %s", ErrInvalidRequest, source.AssetName)
	}
	return checksum, nil
}

func (u *Updater) downloadToTemp(ctx context.Context, source releaseSource) (string, error) {
	if err := os.MkdirAll(filepath.Dir(u.cfg.BinaryPath), 0o755); err != nil {
		return "", fmt.Errorf("create binary dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(u.cfg.BinaryPath), ".agent-update-*")
	if err != nil {
		return "", fmt.Errorf("create update temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
	}()

	resp, err := u.request(ctx, source.AssetURL)
	if err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		os.Remove(tmpPath)
		return "", fmt.Errorf("download release asset returned status %d", resp.StatusCode)
	}
	reader := io.LimitReader(resp.Body, u.cfg.MaxDownloadBytes+1)
	n, err := io.Copy(tmp, reader)
	if err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("write release asset: %w", err)
	}
	if n == 0 {
		os.Remove(tmpPath)
		return "", fmt.Errorf("%w: downloaded release asset is empty", ErrInvalidRequest)
	}
	if n > u.cfg.MaxDownloadBytes {
		os.Remove(tmpPath)
		return "", fmt.Errorf("%w: release asset exceeds maximum size", ErrInvalidRequest)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close release asset temp: %w", err)
	}
	return tmpPath, nil
}

func (u *Updater) fetchBytes(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	resp, err := u.request(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read download: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: download exceeds maximum size", ErrInvalidRequest)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("%w: download is empty", ErrInvalidRequest)
	}
	return data, nil
}

func (u *Updater) request(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build release request: %w", err)
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download release asset: %w", err)
	}
	return resp, nil
}

func (u *Updater) backupCurrentBinary() (string, error) {
	info, err := os.Stat(u.cfg.BinaryPath)
	if err != nil {
		return "", fmt.Errorf("stat current agent binary: %w", err)
	}
	if err := os.MkdirAll(u.cfg.BackupDir, 0o700); err != nil {
		return "", fmt.Errorf("create updater backup dir: %w", err)
	}
	name := fmt.Sprintf("agent-%s-%d.bak", sanitizeVersion(firstNonEmpty(u.state.CurrentVersion, u.cfg.CurrentVersion)), u.now().UnixNano())
	backupPath := filepath.Join(u.cfg.BackupDir, name)
	if err := copyFile(u.cfg.BinaryPath, backupPath, info.Mode().Perm()); err != nil {
		return "", err
	}
	return backupPath, nil
}

func (u *Updater) updateJitter(req UpdateRequest) time.Duration {
	min := u.cfg.JitterMin
	max := u.cfg.JitterMax
	if req.JitterMinSeconds > 0 {
		min = time.Duration(req.JitterMinSeconds) * time.Second
	}
	if req.JitterMaxSeconds > 0 {
		max = time.Duration(req.JitterMaxSeconds) * time.Second
	}
	return randomDuration(min, max)
}

func normalizeConfig(cfg Config) (Config, error) {
	if strings.TrimSpace(cfg.CurrentVersion) == "" {
		cfg.CurrentVersion = "unknown"
	}
	if strings.TrimSpace(cfg.BinaryPath) == "" {
		exe, err := os.Executable()
		if err != nil {
			return Config{}, fmt.Errorf("%w: resolve current executable: %w", ErrInvalidConfig, err)
		}
		cfg.BinaryPath = exe
	}
	cfg.BinaryPath = filepath.Clean(cfg.BinaryPath)
	if strings.TrimSpace(cfg.StatePath) == "" {
		cfg.StatePath = filepath.Join(filepath.Dir(cfg.BinaryPath), "agent-update-state.json")
	}
	if strings.TrimSpace(cfg.BackupDir) == "" {
		cfg.BackupDir = filepath.Join(filepath.Dir(cfg.BinaryPath), "backups")
	}
	cfg.StatePath = filepath.Clean(cfg.StatePath)
	cfg.BackupDir = filepath.Clean(cfg.BackupDir)
	cfg.ReleaseBaseURL = firstNonEmpty(cfg.ReleaseBaseURL, DefaultReleaseBaseURL)
	cfg.ReleaseRepo = firstNonEmpty(cfg.ReleaseRepo, DefaultReleaseRepo)
	cfg.ReleaseTag = firstNonEmpty(cfg.ReleaseTag, DefaultReleaseTag)
	cfg.OS = firstNonEmpty(cfg.OS, runtime.GOOS)
	cfg.Arch = firstNonEmpty(cfg.Arch, runtime.GOARCH)
	if cfg.HealthTimeout <= 0 {
		cfg.HealthTimeout = defaultHealthTimeout
	}
	if cfg.MaxCrashCount <= 0 {
		cfg.MaxCrashCount = defaultMaxCrashCount
	}
	if cfg.JitterMax == 0 {
		cfg.JitterMax = defaultJitterMax
	}
	if cfg.JitterMax < cfg.JitterMin {
		return Config{}, fmt.Errorf("%w: jitter max is smaller than min", ErrInvalidConfig)
	}
	if cfg.MaxDownloadBytes <= 0 {
		cfg.MaxDownloadBytes = defaultMaxDownloadBytes
	}
	if _, err := ParseBaseURL(cfg.ReleaseBaseURL); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func validateTrustedURL(rawURL, rawBase string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%w: invalid release URL", ErrInvalidRequest)
	}
	base, err := ParseBaseURL(rawBase)
	if err != nil {
		return err
	}
	if u.Scheme != base.Scheme || !strings.EqualFold(u.Host, base.Host) {
		return fmt.Errorf("%w: release URL is outside configured base", ErrInvalidRequest)
	}
	if u.Scheme != "https" && base.Scheme != "http" {
		return fmt.Errorf("%w: release URL must use HTTPS", ErrInvalidRequest)
	}
	return nil
}

func ParseBaseURL(rawBase string) (*url.URL, error) {
	base, err := url.Parse(strings.TrimRight(strings.TrimSpace(rawBase), "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("%w: invalid release base URL", ErrInvalidConfig)
	}
	if base.Scheme != "https" && base.Scheme != "http" {
		return nil, fmt.Errorf("%w: unsupported release base URL scheme", ErrInvalidConfig)
	}
	return base, nil
}

func buildReleaseURLs(base, repo, tag, assetName string) (string, string) {
	base = strings.TrimRight(base, "/")
	if tag == "latest" {
		prefix := fmt.Sprintf("%s/%s/releases/latest/download", base, repo)
		return prefix + "/" + assetName, prefix + "/SHA256SUMS.txt"
	}
	prefix := fmt.Sprintf("%s/%s/releases/download/%s", base, repo, tag)
	return prefix + "/" + assetName, prefix + "/SHA256SUMS.txt"
}

func buildAssetName(goos, goarch string) string {
	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("agent-%s-%s%s", goos, goarch, ext)
}

func lookupExpectedChecksum(assetName, manifest string) string {
	for _, line := range strings.Split(manifest, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		checksum := normalizeChecksum(fields[0])
		listed := strings.TrimPrefix(strings.TrimSpace(fields[1]), "*")
		listed = strings.TrimPrefix(listed, "./")
		if listed == assetName || listed == "deploy/"+assetName || listed == "dist/release/"+assetName || strings.HasSuffix(listed, "/"+assetName) {
			return checksum
		}
	}
	return ""
}

func verifyFileChecksum(path, expected string) error {
	actual, err := hashFileSHA256(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, normalizeChecksum(expected)) {
		return fmt.Errorf("%w: expected %s actual %s", ErrChecksumMismatch, normalizeChecksum(expected), actual)
	}
	return nil
}

func hashFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for checksum: %w", err)
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func replaceBinary(tmpPath, binaryPath string) error {
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("mark release asset executable: %w", err)
	}
	if err := os.Rename(tmpPath, binaryPath); err != nil {
		return fmt.Errorf("replace agent binary: %w", err)
	}
	return nil
}

func copyToPathAtomically(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".agent-rollback-*")
	if err != nil {
		return fmt.Errorf("create rollback temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("copy file: %w", err)
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set destination permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close destination file: %w", err)
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("replace agent binary during rollback: %w", err)
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("copy file: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close destination file: %w", closeErr)
	}
	if err := os.Chmod(dst, mode); err != nil {
		return fmt.Errorf("set destination permissions: %w", err)
	}
	return nil
}

func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func randomDuration(min, max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	if min < 0 {
		min = 0
	}
	if max <= min {
		return min
	}
	delta := int64(max - min)
	n, err := rand.Int(rand.Reader, big.NewInt(delta+1))
	if err != nil {
		return min
	}
	return min + time.Duration(n.Int64())
}

func emit(ctx context.Context, progress ProgressFunc, event Event) error {
	if progress == nil {
		return nil
	}
	return progress(ctx, event)
}

func normalizeChecksum(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	input = strings.TrimPrefix(input, "sha256:")
	return input
}

func sameVersion(a, b string) bool {
	a = normalizeVersion(a)
	b = normalizeVersion(b)
	return a != "" && b != "" && a == b
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(strings.ToLower(version))
	version = strings.TrimPrefix(version, "v")
	return version
}

func sanitizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range version {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

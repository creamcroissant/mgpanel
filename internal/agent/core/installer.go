package core

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/xboard/internal/agent/capability"
	"github.com/creamcroissant/xboard/internal/agent/initsys"
	"github.com/creamcroissant/xboard/internal/agent/protocol"
	agentv1 "github.com/creamcroissant/xboard/pkg/pb/agent/v1"
)

const (
	installCommandTimeout = 15 * time.Minute
	downloadTimeout       = 10 * time.Minute
	apiTimeout            = 30 * time.Second
	downloadRetries       = 3
	githubAPICacheTTL     = 5 * time.Minute
	githubAPIBaseURL      = "https://api.github.com"

	FlavorOfficial     = "official"
	FlavorWithV2rayAPI = "with-v2ray-api"
)

type GitHubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest,omitempty"`
}

type GitHubReleaseResponse struct {
	TagName string               `json:"tag_name"`
	Assets  []GitHubReleaseAsset `json:"assets"`
}

type InstallerConfig struct {
	ScriptPath         string
	SingBoxBinaryPath  string
	XrayBinaryPath     string
	ServiceName        string
	SingBoxReleaseRepo string
	XrayReleaseRepo    string
	ReleaseBaseURL     string
	CoreInstallDir     string
}

type Installer struct {
	cfg            InstallerConfig
	initSys        initsys.InitSystem
	detector       *capability.Detector
	logger         *slog.Logger
	client         *http.Client
	downloadClient *http.Client
	mu             sync.Mutex
	locks          map[CoreType]*sync.Mutex
	releaseCache   map[string]*cachedRelease
	cacheMu        sync.RWMutex
}

type cachedRelease struct {
	resp    *GitHubReleaseResponse
	expires time.Time
}

func NewInstaller(cfg InstallerConfig, initSys initsys.InitSystem, logger *slog.Logger) *Installer {
	if initSys == nil {
		initSys = initsys.Detect()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Installer{
		cfg:            cfg,
		initSys:        initSys,
		detector:       capability.NewDetector(strings.TrimSpace(cfg.SingBoxBinaryPath), strings.TrimSpace(cfg.XrayBinaryPath)),
		logger:         logger,
		client:         &http.Client{Timeout: apiTimeout},
		downloadClient: &http.Client{Timeout: downloadTimeout},
		locks:          make(map[CoreType]*sync.Mutex),
		releaseCache:   make(map[string]*cachedRelease),
	}
}

func (i *Installer) InstallCore(ctx context.Context, req *agentv1.InstallCoreRequest) (*agentv1.InstallCoreResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("empty install request")
	}
	coreType, err := normalizeInstallCoreType(req.CoreType)
	if err != nil {
		return nil, err
	}
	action, err := normalizeInstallAction(req.Action)
	if err != nil {
		return nil, err
	}
	version := strings.TrimSpace(req.Version)
	channel := strings.TrimSpace(req.Channel)
	flavor := strings.TrimSpace(req.Flavor)
	if flavor == "" {
		flavor = FlavorOfficial
	}

	lock := i.lockForCore(coreType)
	lock.Lock()
	defer lock.Unlock()

	previousVersion, previousErr := i.detectVersion(ctx, coreType)
	_, _, stablePath, _ := i.resolveCoreAsset(coreType, flavor)

	if action == "uninstall" {
		i.logger.Info("uninstalling core", "type", coreType)
		if err := os.Remove(stablePath); err != nil && !os.IsNotExist(err) {
			i.logger.Warn("remove stable symlink", "path", stablePath, "error", err)
		}
		coreDir := filepath.Join(i.cfg.CoreInstallDir, string(coreType))
		if err := os.RemoveAll(coreDir); err != nil {
			i.logger.Warn("remove core dir", "path", coreDir, "error", err)
			return nil, fmt.Errorf("uninstall remove core dir: %w", err)
		}
		return &agentv1.InstallCoreResponse{
			Success: true, Changed: true, CoreType: string(coreType),
			Version: "", PreviousVersion: previousVersion,
			Message: fmt.Sprintf("Uninstalled %s", string(coreType)),
		}, nil
	}

	repo, binaryName, stablePath, err := i.resolveCoreAsset(coreType, flavor)
	if err != nil {
		return nil, err
	}
	resolvedTag, downloadURL, sha256Hex, assetName, err := i.resolveRelease(ctx, repo, version, channel, coreType, flavor, binaryName)
	if err != nil {
		return nil, err
	}

	i.logger.Info("installing core", "type", coreType, "repo", repo, "tag", resolvedTag, "asset", assetName)

	versionDir := filepath.Join(i.cfg.CoreInstallDir, string(coreType), resolvedTag)
	targetPath := filepath.Join(versionDir, binaryName)

	if action == "ensure" {
		if _, err := os.Stat(targetPath); err == nil {
			if err := i.ensureSymlink(targetPath, stablePath); err != nil {
				i.logger.Warn("failed to update symlink during ensure", "error", err)
			}
			cv, detectErr := i.detectVersion(ctx, coreType)
			if detectErr != nil {
				i.logger.Warn("failed to detect version after ensure", "error", detectErr)
			}
			return &agentv1.InstallCoreResponse{
				Success: true, Changed: false, CoreType: string(coreType),
				Version: cv, PreviousVersion: previousVersion,
				Message: fmt.Sprintf("Core %s already installed at %s", string(coreType), resolvedTag),
			}, nil
		}
	}

	workdir, err := os.MkdirTemp("", "xboard-core-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(workdir)

	archivePath := filepath.Join(workdir, assetName)
	if err := i.downloadFile(ctx, downloadURL, archivePath); err != nil {
		return nil, fmt.Errorf("download %s: %w", assetName, err)
	}
	if sha256Hex != "" {
		if err := i.verifyChecksum(archivePath, sha256Hex); err != nil {
			return nil, fmt.Errorf("checksum: %w", err)
		}
	}

	binaryPath, err := i.extractBinary(archivePath, workdir, binaryName)
	if err != nil {
		return nil, fmt.Errorf("extract: %w", err)
	}
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return nil, fmt.Errorf("create version dir: %w", err)
	}
	if err := copyFile(binaryPath, targetPath, 0o755); err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}
	if err := i.ensureSymlink(targetPath, stablePath); err != nil {
		return nil, fmt.Errorf("create symlink: %w", err)
	}

	cv, currentErr := i.detectVersion(ctx, coreType)
	if currentErr != nil {
		return nil, fmt.Errorf("detect version: %w", currentErr)
	}
	resp := &agentv1.InstallCoreResponse{
		Success:  true,
		Changed:  installChanged(previousVersion, cv, previousErr, nil, action, version),
		Message:  fmt.Sprintf("Installed %s %s", string(coreType), resolvedTag),
		CoreType: string(coreType), Version: cv, PreviousVersion: previousVersion,
	}
	if req.Activate {
		ok, aErr := i.activateCore(ctx, coreType)
		resp.Activated = ok
		if aErr != nil {
			return i.handleActivationFailure(ctx, resp, coreType, previousVersion, previousErr, cv, flavor, aErr), nil
		}
	}
	return resp, nil
}

func (i *Installer) resolveCoreAsset(coreType CoreType, flavor string) (repo, binaryName, stablePath string, err error) {
	switch coreType {
	case CoreTypeSingBox:
		repo = i.cfg.SingBoxReleaseRepo
		if repo == "" {
			return "", "", "", fmt.Errorf("sing-box release repo not configured")
		}
		binaryName = "sing-box"
		stablePath = strings.TrimSpace(i.cfg.SingBoxBinaryPath)
		if stablePath == "" {
			stablePath = "/opt/xboard/agent/bin/sing-box"
		}
	case CoreTypeXray:
		repo = i.cfg.XrayReleaseRepo
		if repo == "" {
			return "", "", "", fmt.Errorf("xray release repo not configured")
		}
		binaryName = "xray"
		stablePath = strings.TrimSpace(i.cfg.XrayBinaryPath)
		if stablePath == "" {
			stablePath = "/opt/xboard/agent/bin/xray"
		}
	default:
		return "", "", "", fmt.Errorf("unsupported core type")
	}
	return
}

func (i *Installer) buildAssetName(coreType CoreType, flavor, tag, binaryName string) string {
	norm := strings.TrimPrefix(tag, "v")
	switch coreType {
	case CoreTypeSingBox:
		if flavor == FlavorWithV2rayAPI {
			return fmt.Sprintf("sing-box-linux-%s", runtime.GOARCH)
		}
		return fmt.Sprintf("sing-box-%s-linux-%s.tar.gz", norm, runtime.GOARCH)
	case CoreTypeXray:
		return fmt.Sprintf("Xray-linux-%s.zip", archToXrayToken(runtime.GOARCH))
	}
	return binaryName
}

func (i *Installer) resolveRelease(ctx context.Context, repo, version, channel string, coreType CoreType, flavor string, binaryName string) (resolvedTag, downloadURL, sha256Hex, assetName string, err error) {
	apiURL := i.buildReleaseAPIURL(repo, version, channel)
	i.cacheMu.RLock()
	var cachedResp *GitHubReleaseResponse
	if cached, ok := i.releaseCache[apiURL]; ok && time.Now().Before(cached.expires) {
		cachedResp = cached.resp
	}
	i.cacheMu.RUnlock()

	if cachedResp != nil {
		a := i.buildAssetName(coreType, flavor, cachedResp.TagName, binaryName)
		dl, sh := findAssetURL(cachedResp.Assets, a)
		return cachedResp.TagName, dl, sh, a, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", "", "", fmt.Errorf("create release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "xboard-agent/1.0")
	resp, netErr := i.client.Do(req)
	if netErr != nil {
		return "", "", "", "", fmt.Errorf("fetch: %w", netErr)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return "", "", "", "", fmt.Errorf("GitHub API rate limited (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", "", "", fmt.Errorf("read release response: %w", err)
	}
	var release GitHubReleaseResponse
	if err := json.Unmarshal(body, &release); err != nil {
		return "", "", "", "", fmt.Errorf("failed to parse release info: %w", err)
	}
	if release.TagName == "" {
		return "", "", "", "", fmt.Errorf("empty tag_name")
	}

	resolvedTag = release.TagName
	assetName = i.buildAssetName(coreType, flavor, resolvedTag, binaryName)
	downloadURL, sha256Hex = findAssetURL(release.Assets, assetName)
	if downloadURL == "" {
		return "", "", "", "", fmt.Errorf("asset %q not found in %s (%s)", assetName, resolvedTag, repo)
	}

	i.cacheMu.Lock()
	i.releaseCache[apiURL] = &cachedRelease{resp: &release, expires: time.Now().Add(githubAPICacheTTL)}
	i.cacheMu.Unlock()
	return
}

func (i *Installer) buildReleaseAPIURL(repo, version, channel string) string {
	if version != "" {
		normalized := version
		if !strings.HasPrefix(normalized, "v") {
			normalized = "v" + normalized
		}
		return fmt.Sprintf("%s/repos/%s/releases/tags/%s", githubAPIBaseURL, repo, normalized)
	}
	return fmt.Sprintf("%s/repos/%s/releases/latest", githubAPIBaseURL, repo)
}

func findAssetURL(assets []GitHubReleaseAsset, assetName string) (string, string) {
	for _, a := range assets {
		if a.Name == assetName {
			return a.BrowserDownloadURL, a.Digest
		}
	}
	return "", ""
}

func (i *Installer) verifyChecksum(path, expectedHex string) error {
	expectedHex = strings.TrimPrefix(strings.TrimSpace(expectedHex), "sha256:")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("checksum read: %w", err)
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expectedHex {
		return fmt.Errorf("expected %s, got %s", expectedHex, actual)
	}
	return nil
}

func (i *Installer) downloadFile(ctx context.Context, url, path string) error {
	// Detach from caller context so gRPC deadline doesn't kill the download;
	// the downloadClient's own 10-minute timeout provides the safety net.
	dlCtx := context.WithoutCancel(ctx)

	var lastErr error
	for attempt := 0; attempt < downloadRetries; attempt++ {
		if attempt > 0 {
			i.logger.Info("retrying download", "url", url, "attempt", attempt+1, "max", downloadRetries)
			select {
			case <-time.After(time.Duration(attempt) * 5 * time.Second):
			case <-dlCtx.Done():
				return dlCtx.Err()
			}
		}

		lastErr = i.downloadOnce(dlCtx, url, path)
		if lastErr == nil {
			return nil
		}
		i.logger.Warn("download failed, will retry", "url", url, "attempt", attempt+1, "error", lastErr)
		// Clean up partial file on failure
		os.Remove(path)
	}
	return fmt.Errorf("download failed after %d retries: %w", downloadRetries, lastErr)
}

func (i *Installer) downloadOnce(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", "xboard-agent/1.0")
	resp, err := i.downloadClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create download file: %w", err)
	}
	defer out.Close()
	written, err := io.Copy(out, io.LimitReader(resp.Body, 200<<20))
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	if written == 0 {
		return fmt.Errorf("empty download")
	}
	return nil
}

func (i *Installer) extractBinary(archivePath, workdir, binaryName string) (string, error) {
	switch {
	case strings.HasSuffix(archivePath, ".tar.gz"), strings.HasSuffix(archivePath, ".tgz"):
		ed := filepath.Join(workdir, "extracted")
		os.MkdirAll(ed, 0o755)
		if err := untarGz(archivePath, ed); err != nil {
			return "", err
		}
		return walkBinary(ed, binaryName)
	case strings.HasSuffix(archivePath, ".zip"):
		ed := filepath.Join(workdir, "extracted")
		os.MkdirAll(ed, 0o755)
		if err := unzip(archivePath, ed); err != nil {
			return "", err
		}
		return walkBinary(ed, binaryName)
	default:
		dest := filepath.Join(workdir, binaryName)
		return dest, copyFile(archivePath, dest, 0o755)
	}
}

func (i *Installer) ensureSymlink(target, stable string) error {
	os.MkdirAll(filepath.Dir(stable), 0o755)
	os.Remove(stable)
	if err := os.Symlink(target, stable); err != nil {
		return copyFile(target, stable, 0o755)
	}
	return nil
}

func untarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		tgt := filepath.Join(dest, filepath.Clean(h.Name))
		switch h.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(tgt, os.FileMode(h.Mode))
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(tgt), 0o755)
			out, err := os.OpenFile(tgt, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode))
			if err != nil {
				return fmt.Errorf("create extracted file %s: %w", h.Name, err)
			}
			_, err = io.Copy(out, tr)
			closeErr := out.Close()
			if err != nil {
				return fmt.Errorf("write extracted file %s: %w", h.Name, err)
			}
			if closeErr != nil {
				return fmt.Errorf("close extracted file %s: %w", h.Name, closeErr)
			}
		}
	}
	return nil
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		tgt := filepath.Join(dest, filepath.Clean(f.Name))
		if f.FileInfo().IsDir() {
			os.MkdirAll(tgt, 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(tgt), 0o755)
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}

		out, err := os.OpenFile(tgt, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return fmt.Errorf("create extracted file %s: %w", f.Name, err)
		}

		_, err = io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if err != nil {
			return fmt.Errorf("write extracted file %s: %w", f.Name, err)
		}
		if closeErr != nil {
			return fmt.Errorf("close extracted file %s: %w", f.Name, closeErr)
		}
	}
	return nil
}

func walkBinary(root, name string) (string, error) {
	var found string
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == name {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("binary %q not found", name)
	}
	return found, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	in.Close() // no defer: ensure close before returning
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func archToXrayToken(arch string) string {
	switch arch {
	case "amd64":
		return "64"
	case "arm64":
		return "arm64-v8a"
	}
	return arch
}

func (i *Installer) handleActivationFailure(ctx context.Context, resp *agentv1.InstallCoreResponse, coreType CoreType, previousVersion string, previousErr error, currentVersion, flavor string, activateErr error) *agentv1.InstallCoreResponse {
	resp.Success = false
	resp.Error = activateErr.Error()
	resp.Message = "core install completed but activation failed"
	resp.Activated = false
	if previousErr != nil || previousVersion == "" || currentVersion == "" || previousVersion == currentVersion {
		return resp
	}
	if err := i.rollbackCore(ctx, coreType, previousVersion, flavor); err != nil {
		resp.Error = fmt.Sprintf("%s; rollback: %v", activateErr.Error(), err)
		return resp
	}
	if _, err := i.activateCore(ctx, coreType); err != nil {
		resp.Error = fmt.Sprintf("%s; rollback activate: %v", activateErr.Error(), err)
		return resp
	}
	if rv, err := i.detectVersion(ctx, coreType); err == nil {
		resp.RolledBack = true
		resp.Version = rv
		resp.PreviousVersion = currentVersion
		resp.Message = "core install activation failed and previous version restored"
	}
	return resp
}

func (i *Installer) lockForCore(coreType CoreType) *sync.Mutex {
	i.mu.Lock()
	defer i.mu.Unlock()
	if lock, ok := i.locks[coreType]; ok {
		return lock
	}
	lock := &sync.Mutex{}
	i.locks[coreType] = lock
	return lock
}

func normalizeInstallCoreType(raw string) (CoreType, error) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(CoreTypeSingBox), "singbox":
		return CoreTypeSingBox, nil
	case string(CoreTypeXray):
		return CoreTypeXray, nil
	}
	return "", fmt.Errorf("unsupported core_type")
}

func normalizeInstallAction(raw string) (string, error) {
	act := strings.TrimSpace(strings.ToLower(raw))
	switch act {
	case "install", "upgrade", "ensure", "uninstall":
		return act, nil
	}
	return "", fmt.Errorf("unsupported action")
}

func (i *Installer) detectVersion(ctx context.Context, coreType CoreType) (string, error) {
	switch coreType {
	case CoreTypeSingBox:
		caps, err := i.detector.DetectSingBox(ctx)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(caps.CoreVersion), nil
	case CoreTypeXray:
		caps, err := i.detector.DetectXray(ctx)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(caps.CoreVersion), nil
	}
	return "", fmt.Errorf("unsupported core type")
}

func installChanged(previousVersion, currentVersion string, previousErr, currentErr error, action, requestedVersion string) bool {
	if currentErr != nil {
		return false
	}
	if previousErr != nil || currentVersion != previousVersion {
		return true
	}
	if requestedVersion != "" && requestedVersion != currentVersion {
		return true
	}
	return action == "install"
}

func (i *Installer) rollbackCore(ctx context.Context, coreType CoreType, version, flavor string) error {
	_, binaryName, stablePath, err := i.resolveCoreAsset(coreType, flavor)
	if err != nil {
		return err
	}
	versionDir := filepath.Join(i.cfg.CoreInstallDir, string(coreType), version)
	targetPath := filepath.Join(versionDir, binaryName)
	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("rollback target %s not found: %w", targetPath, err)
	}
	if err := i.ensureSymlink(targetPath, stablePath); err != nil {
		return fmt.Errorf("rollback symlink: %w", err)
	}
	i.logger.Info("core rollback completed", "type", coreType, "version", version)
	return nil
}

func (i *Installer) activateCore(ctx context.Context, coreType CoreType) (bool, error) {
	svc := strings.TrimSpace(i.cfg.ServiceName)
	if svc == "" {
		switch coreType {
		case CoreTypeSingBox:
			svc = "sing-box"
		case CoreTypeXray:
			svc = "xray"
		}
	}
	if svc == "" {
		return false, fmt.Errorf("service name empty")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return true, i.initSys.Restart(ctx, svc)
}

func sanitizeCommandOutput(b []byte) string {
	return protocol.SanitizeCommandOutput(b)
}

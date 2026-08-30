package cdn

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/updater"
)

const (
	// DefaultLinuxAmd64URL is the default download URL for Caddy Linux amd64.
	DefaultLinuxAmd64URL = "https://github.com/caddyserver/caddy/releases/latest/download/caddy_linux_amd64.tar.gz"
	maxDownloadBytes     = 100 * 1024 * 1024 // 100 MB
	versionTimeout       = 10 * time.Second
)

// CaddyInstaller manages the download, extraction, and verification of the Caddy binary.
type CaddyInstaller struct {
	binDir      string
	binPath     string
	downloadURL string
	client      *http.Client
}

// NewCaddyInstaller creates a new CaddyInstaller with default paths and download URL.
func NewCaddyInstaller() *CaddyInstaller {
	return &CaddyInstaller{
		binDir:      "/opt/work/mgpanel/caddy/",
		binPath:     "/opt/work/mgpanel/caddy/caddy",
		downloadURL: DefaultLinuxAmd64URL,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// SetDownloadURL overrides the default download URL.
func (ci *CaddyInstaller) SetDownloadURL(url string) {
	if trimmed := strings.TrimSpace(url); trimmed != "" {
		ci.downloadURL = trimmed
	}
}

// IsInstalled returns true if the caddy binary exists at binPath.
func (ci *CaddyInstaller) IsInstalled() bool {
	_, err := os.Stat(ci.binPath)
	return err == nil
}

// Version returns the installed caddy version string by running "caddy version".
func (ci *CaddyInstaller) Version(ctx context.Context) (string, error) {
	if !ci.IsInstalled() {
		return "", fmt.Errorf("caddy not installed")
	}
	versionCtx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()
	cmd := exec.CommandContext(versionCtx, ci.binPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get caddy version: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Install downloads, extracts, and verifies the Caddy binary.
// It skips the download if a valid installation already exists.
func (ci *CaddyInstaller) Install(ctx context.Context) error {
	// Skip if already installed with a valid version.
	if ci.IsInstalled() {
		ver, verErr := ci.Version(ctx)
		if verErr == nil && ver != "" {
			return nil
		}
	}

	// Validate download URL using updater.ParseBaseURL.
	if _, err := updater.ParseBaseURL(ci.downloadURL); err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}

	// Ensure bin directory exists.
	if err := os.MkdirAll(ci.binDir, 0o755); err != nil {
		return fmt.Errorf("create bin directory: %w", err)
	}

	// Download the tar.gz archive.
	data, err := ci.downloadBytes(ctx, ci.downloadURL)
	if err != nil {
		return fmt.Errorf("download caddy: %w", err)
	}

	// Extract the caddy binary from the archive.
	binData, err := extractCaddyFromTarGz(data)
	if err != nil {
		return fmt.Errorf("extract caddy binary: %w", err)
	}

	// Write the binary with executable permissions.
	if err := os.WriteFile(ci.binPath, binData, 0o755); err != nil {
		return fmt.Errorf("write caddy binary: %w", err)
	}

	// Verify the installed binary.
	ver, err := ci.Version(ctx)
	if err != nil {
		return fmt.Errorf("verify caddy installation: %w", err)
	}
	if ver == "" {
		return fmt.Errorf("caddy version is empty after installation")
	}

	return nil
}

// downloadBytes fetches the full content of the given URL with a size limit.
func (ci *CaddyInstaller) downloadBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build download request: %w", err)
	}
	resp, err := ci.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download caddy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if int64(len(data)) > maxDownloadBytes {
		return nil, fmt.Errorf("downloaded file exceeds maximum size")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("downloaded file is empty")
	}
	return data, nil
}

// extractCaddyFromTarGz finds and extracts the "caddy" binary from a tar.gz archive.
func extractCaddyFromTarGz(data []byte) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decompress gzip: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(header.Name) != "caddy" {
			continue
		}
		binData, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, fmt.Errorf("read caddy binary from archive: %w", err)
		}
		return binData, nil
	}
	return nil, fmt.Errorf("caddy binary not found in archive")
}

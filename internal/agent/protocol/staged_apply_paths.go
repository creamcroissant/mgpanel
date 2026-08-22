package protocol

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// StagedApplyPaths contains normalized directories shared by staged apply transactions.
type StagedApplyPaths struct {
	CurrentDir      string
	LegacyDir       string
	ManagedDir      string
	MergeOutputFile string
}

var ErrStagedApplyConfigDirRequired = errors.New("at least one protocol config dir is required")

// ResolveStagedApplyPaths normalizes protocol directories for staged apply transactions.
func ResolveStagedApplyPaths(cfg Config) (StagedApplyPaths, error) {
	currentDir := strings.TrimSpace(cfg.ConfigDir)
	managedDir := strings.TrimSpace(cfg.ManagedConfigDir)
	legacyDir := strings.TrimSpace(cfg.LegacyConfigDir)
	if currentDir == "" {
		switch {
		case managedDir != "":
			currentDir = managedDir
		case legacyDir != "":
			currentDir = legacyDir
		default:
			currentDir = "/etc/sing-box/conf"
		}
	}
	if managedDir == "" {
		managedDir = currentDir
	}
	if legacyDir == "" {
		legacyDir = currentDir
	}

	currentAbs, err := resolveApplyDir(currentDir, "current")
	if err != nil {
		return StagedApplyPaths{}, err
	}
	managedAbs, err := resolveApplyDir(managedDir, "managed")
	if err != nil {
		return StagedApplyPaths{}, err
	}
	legacyAbs, err := resolveApplyDir(legacyDir, "legacy")
	if err != nil {
		return StagedApplyPaths{}, err
	}

	mergeOutputFile := strings.TrimSpace(cfg.MergeOutputFile)
	if mergeOutputFile == "" {
		mergeOutputFile = "config.json"
	}
	mergeOutputFile, err = sanitizeFilename(mergeOutputFile)
	if err != nil {
		return StagedApplyPaths{}, fmt.Errorf("invalid merge output file: %w", err)
	}

	return StagedApplyPaths{
		CurrentDir:      currentAbs,
		LegacyDir:       legacyAbs,
		ManagedDir:      managedAbs,
		MergeOutputFile: mergeOutputFile,
	}, nil
}

func resolveApplyDir(raw, field string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%s config dir is required", field)
	}
	abs, err := filepath.Abs(filepath.Clean(trimmed))
	if err != nil {
		return "", fmt.Errorf("resolve %s config dir: %w", field, err)
	}
	if abs == string(filepath.Separator) {
		return "", fmt.Errorf("%s config dir must not be root path", field)
	}
	return abs, nil
}

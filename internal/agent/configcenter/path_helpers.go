package configcenter

import (
	"github.com/creamcroissant/mgpanel/internal/agent/config"
	"github.com/creamcroissant/mgpanel/internal/agent/protocol"
)

func resolveProtocolPaths(cfg config.ProtocolConfig) (legacyDirAbs, managedDirAbs, mergeOutputFile string, err error) {
	if cfg.ConfigDir == "" && cfg.LegacyConfigDir == "" && cfg.ManagedConfigDir == "" {
		return "", "", "", protocol.ErrStagedApplyConfigDirRequired
	}
	paths, err := protocol.ResolveStagedApplyPaths(protocol.Config{
		ConfigDir:        cfg.ConfigDir,
		LegacyConfigDir:  cfg.LegacyConfigDir,
		ManagedConfigDir: cfg.ManagedConfigDir,
		MergeOutputFile:  cfg.MergeOutputFile,
	})
	if err != nil {
		return "", "", "", err
	}
	return paths.LegacyDir, paths.ManagedDir, paths.MergeOutputFile, nil
}

func sanitizeFilename(filename string) (string, error) {
	return protocol.SanitizeFilename(filename)
}

func normalizeCoreType(coreType string) string {
	return protocol.NormalizeCoreType(coreType)
}

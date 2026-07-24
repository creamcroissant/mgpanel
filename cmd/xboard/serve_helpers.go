package main

import (
	"os"
	"strings"

	"github.com/creamcroissant/xboard/internal/bootstrap"
	"github.com/creamcroissant/xboard/internal/config"
	"github.com/creamcroissant/xboard/internal/service"
)

func resolveRuntimeVersion() string {
	runtimeVersion := strings.TrimSpace(Version)
	if runtimeVersion == "" || runtimeVersion == "unknown" {
		return "go-dev"
	}
	return runtimeVersion
}

func buildBootstrapConfig(cfg *config.Config, runtimeVersion string, signingKey string) *bootstrap.Config {
	return &bootstrap.Config{
		HTTP: bootstrap.HTTPConfig{
			Addr:            cfg.HTTP.Addr,
			ShutdownTimeout: cfg.HTTP.ShutdownTimeout,
		},
		Log: bootstrap.LogConfig{
			Level:       cfg.Log.SlogLevel(),
			Format:      cfg.Log.Format,
			AddSource:   cfg.Log.AddSource,
			Environment: cfg.Log.Environment,
		},
		DB: bootstrap.DBConfig{
			SQLitePath: cfg.DB.Path,
		},
		Auth: bootstrap.AuthConfig{
			SigningKey: signingKey,
			TokenTTL:   cfg.Auth.TokenTTL,
			Issuer:     cfg.Auth.Issuer,
			Audience:   cfg.Auth.Audience,
			Leeway:     cfg.Auth.Leeway,
			BcryptCost: cfg.Auth.BcryptCost,
		},
		UI: bootstrap.UIConfig{
			Admin: bootstrap.AdminUIConfig{
				Enabled:       cfg.UI.Admin.Enabled,
				Dir:           cfg.UI.Admin.Dir,
				Title:         cfg.UI.Admin.Title,
				Version:       runtimeVersion,
				BaseURL:       cfg.UI.Admin.BaseURL,
				HiddenModules: cfg.UI.Admin.HiddenModules,
			},
			Install: bootstrap.InstallUIConfig{
				Enabled: cfg.UI.Install.Enabled,
				Dir:     cfg.UI.Install.Dir,
			},
		},
	}
}

func buildBinaryVersionProvider() service.BinaryVersionRemoteProvider {
	repos := map[string]string{
		service.BinaryVersionComponentAgent:   envOrDefault("XBOARD_VERSION_REPO_AGENT", "creamcroissant/xboard2p"),
		service.BinaryVersionComponentSingBox: envOrDefault("XBOARD_VERSION_REPO_SING_BOX", "SagerNet/sing-box"),
		service.BinaryVersionComponentXray:    envOrDefault("XBOARD_VERSION_REPO_XRAY", "XTLS/Xray-core"),
	}
	return service.NewGitHubVersionProviderWithRepos(repos)
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
package access

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/core"
)

type SingboxAccessCollector struct {
	manager *core.Manager
	logger  *slog.Logger
	client  *http.Client
	// configDir 用于读取注入的 experimental.json（含 clash_api 配置）
	configDir string
}

func NewSingboxAccessCollector(manager *core.Manager, configDir string, logger *slog.Logger) *SingboxAccessCollector {
	if configDir == "" {
		configDir = "/etc/sing-box/conf"
	}
	return &SingboxAccessCollector{
		manager:   manager,
		logger:    logger,
		client:    &http.Client{Timeout: 5 * time.Second},
		configDir: configDir,
	}
}

func (c *SingboxAccessCollector) Type() string {
	return "sing-box"
}

// portToInt 将 Clash API 返回的端口（可能是字符串或数字）转为 int。
func portToInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		var p int
		if _, err := fmt.Sscanf(n, "%d", &p); err == nil {
			return p
		}
	}
	return 0
}

func (c *SingboxAccessCollector) Collect(ctx context.Context) ([]AccessLogEntry, error) {
	instances := c.manager.ListInstances()
	var entries []AccessLogEntry

	for _, inst := range instances {
		if inst.CoreType != core.CoreTypeSingBox || inst.Status != core.StatusRunning {
			continue
		}

		newEntries, err := c.collectFromInstance(ctx, inst)
		if err != nil {
			c.logger.Debug("failed to collect access log from sing-box instance",
				"instance_id", inst.ID,
				"error", err,
			)
			continue
		}
		entries = append(entries, newEntries...)
	}

	// 即使没有注册的实例，也尝试从默认 experimental.json 直接连接 clash API
	if len(entries) == 0 && c.configDir != "" {
		apiAddr, secret, err := c.readClashAPIConfigFromFile(filepath.Join(c.configDir, "experimental.json"))
		if err == nil {
			newEntries, err := c.collectFromAPI(ctx, apiAddr, secret)
			if err == nil {
				entries = append(entries, newEntries...)
				c.logger.Info("access log collected from default clash api", "count", len(newEntries))
			} else {
				c.logger.Warn("collect from default clash api", "error", err)
			}
		}
	}

	return entries, nil
}

func (c *SingboxAccessCollector) collectFromInstance(ctx context.Context, inst *core.CoreInstance) ([]AccessLogEntry, error) {
	apiAddr, secret, err := c.getClashAPIConfig(inst.ConfigPath)
	if err != nil {
		return nil, nil // API not configured
	}
	return c.collectFromAPI(ctx, apiAddr, secret)
}

func (c *SingboxAccessCollector) collectFromAPI(ctx context.Context, apiAddr, secret string) ([]AccessLogEntry, error) {
	url := fmt.Sprintf("http://%s/connections", apiAddr)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data struct {
		Connections []struct {
			ID          string `json:"id"`
			Metadata    struct {
				Network         string `json:"network"`
				Type            string `json:"type"`
				SourceIP        string `json:"sourceIP"`
				DestinationIP   string `json:"destinationIP"`
				SourcePort      any    `json:"sourcePort"`
				DestinationPort any    `json:"destinationPort"`
				Host            string `json:"host"`
			} `json:"metadata"`
			Upload   int64  `json:"upload"`
			Download int64  `json:"download"`
			Start    string `json:"start"`
			Chains   []string `json:"chains"`
		} `json:"connections"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var entries []AccessLogEntry
	for _, conn := range data.Connections {
		t, err := time.Parse(time.RFC3339, conn.Start)
		if err != nil {
			t = time.Now()
		}

		entries = append(entries, AccessLogEntry{
			// UserEmail: unknown for now via this API
			SourceIP:        conn.Metadata.SourceIP,
			TargetDomain:    conn.Metadata.Host,
			TargetIP:        conn.Metadata.DestinationIP,
			TargetPort:      portToInt(conn.Metadata.DestinationPort),
			Protocol:        conn.Metadata.Network,
			Upload:          conn.Upload,
			Download:        conn.Download,
			ConnectionStart: t,
		})
	}

	return entries, nil
}

func (c *SingboxAccessCollector) getClashAPIConfig(configPath string) (string, string, error) {
	// 优先从实例配置读取 experimental.clash_api
	if configPath != "" {
		apiAddr, secret, err := c.readClashAPIConfigFromFile(configPath)
		if err == nil {
			return apiAddr, secret, nil
		}
	}
	// 回退到注入的 experimental.json（由 Manager.ensureClashAPIConfig 生成）
	if c.configDir != "" {
		expPath := filepath.Join(c.configDir, "experimental.json")
		apiAddr, secret, err := c.readClashAPIConfigFromFile(expPath)
		if err == nil {
			return apiAddr, secret, nil
		}
	}
	return "", "", fmt.Errorf("clash_api not configured")
}

func (c *SingboxAccessCollector) readClashAPIConfigFromFile(path string) (string, string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}

	var config struct {
		Experimental struct {
			ClashAPI struct {
				ExternalController string `json:"external_controller"`
				Secret             string `json:"secret"`
			} `json:"clash_api"`
		} `json:"experimental"`
	}

	if err := json.Unmarshal(content, &config); err != nil {
		return "", "", err
	}

	if config.Experimental.ClashAPI.ExternalController == "" {
		return "", "", fmt.Errorf("clash_api not configured")
	}

	return config.Experimental.ClashAPI.ExternalController, config.Experimental.ClashAPI.Secret, nil
}

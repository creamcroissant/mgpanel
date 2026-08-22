package access

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/core"
)

type XrayAccessCollector struct {
	manager *core.Manager
	offsets map[string]int64 // instanceID -> offset
	logger  *slog.Logger
}

func NewXrayAccessCollector(manager *core.Manager, logger *slog.Logger) *XrayAccessCollector {
	return &XrayAccessCollector{
		manager: manager,
		offsets: make(map[string]int64),
		logger:  logger,
	}
}

func (c *XrayAccessCollector) Type() string {
	return "xray"
}

func (c *XrayAccessCollector) Collect(ctx context.Context) ([]AccessLogEntry, error) {
	// Iterate over Xray instances
	instances := c.manager.ListInstances()
	var entries []AccessLogEntry

	for _, inst := range instances {
		if inst.CoreType != core.CoreTypeXray || inst.Status != core.StatusRunning {
			continue
		}

		newEntries, err := c.collectFromInstance(ctx, inst)
		if err != nil {
			c.logger.Error("failed to collect access log from instance",
				"instance_id", inst.ID,
				"error", err,
			)
			continue
		}
		entries = append(entries, newEntries...)
	}

	return entries, nil
}

func (c *XrayAccessCollector) collectFromInstance(ctx context.Context, inst *core.CoreInstance) ([]AccessLogEntry, error) {
	logPath, err := c.getAccessLogPath(inst.ConfigPath)
	if err != nil {
		// Log path might not be configured, which is fine
		return nil, nil
	}

	if logPath == "" || logPath == "none" {
		return nil, nil
	}

	file, err := os.Open(logPath)
	if err != nil {
		// File might not exist yet
		return nil, nil
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	offset := c.offsets[inst.ID]

	// Handle log rotation or truncation
	if stat.Size() < offset {
		offset = 0
	}

	if _, err := file.Seek(offset, 0); err != nil {
		return nil, err
	}

	var entries []AccessLogEntry
	scanner := bufio.NewScanner(file)

	newOffset := offset
	for scanner.Scan() {
		line := scanner.Text()
		newOffset += int64(len(line) + 1) // +1 for newline

		entry, ok := c.parseLine(line)
		if ok {
			entries = append(entries, entry)
		}
	}

	if err := scanner.Err(); err != nil {
		c.logger.Warn("xray access log scan error", "instance", inst.ID, "error", err)
	}

	c.offsets[inst.ID] = newOffset
	return entries, nil
}

func (c *XrayAccessCollector) getAccessLogPath(configPath string) (string, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	var config struct {
		Log struct {
			Access string `json:"access"`
		} `json:"log"`
	}

	if err := json.Unmarshal(content, &config); err != nil {
		return "", err
	}

	return config.Log.Access, nil
}

// Example: 2023/10/27 10:00:00 127.0.0.1:53504 accepted tcp:google.com:443 [vless_tcp_reality] email: test@example.com
func (c *XrayAccessCollector) parseLine(line string) (AccessLogEntry, bool) {
	// Basic parsing logic for Xray access log format
	parts := strings.Fields(line)
	if len(parts) < 6 {
		return AccessLogEntry{}, false
	}

	// Determine if this is an access log line
	if parts[3] != "accepted" {
		return AccessLogEntry{}, false
	}

	timestampStr := parts[0] + " " + parts[1]
	t, err := time.Parse("2006/01/02 15:04:05", timestampStr)
	if err != nil {
		return AccessLogEntry{}, false
	}

	src := parts[2]
	destInfo := parts[4] // protocol:dest:port

	destParts := strings.Split(destInfo, ":")
	if len(destParts) < 3 {
		return AccessLogEntry{}, false
	}

	protocol := destParts[0]
	destDomain := destParts[1]
	destPortStr := destParts[2]

	var email string
	for i, part := range parts {
		if strings.HasPrefix(part, "email:") {
			email = strings.TrimPrefix(part, "email:")
			if i+1 < len(parts) && email == "" {
				// Handle case where email is separated by space? unlikely in standard log
			}
			break
		}
	}

	// Skip if no email (unidentified user)
	if email == "" {
		return AccessLogEntry{}, false
	}

	srcIP := src
	if host, _, err := net.SplitHostPort(src); err == nil {
		srcIP = host
	}

	destPort, _ := strconv.Atoi(destPortStr)

	return AccessLogEntry{
		UserEmail:       email,
		SourceIP:        srcIP,
		TargetDomain:    destDomain,
		TargetPort:      destPort,
		Protocol:        protocol,
		ConnectionStart: t,
	}, true
}

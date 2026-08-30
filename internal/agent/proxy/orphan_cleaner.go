package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	defaultPIDDir      = "/var/run/xboard/cores"
	pidFilePermissions = 0600
)

// PIDFile describes the process metadata persisted to disk.
type PIDFile struct {
	PID        int       `json:"pid"`
	InstanceID string    `json:"instance_id"`
	CoreType   string    `json:"core_type"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	Ports      []int     `json:"ports"`
}

// OrphanCleaner manages pid files and cleanup for orphaned core processes.
type OrphanCleaner struct {
	pidDir string
	logger *slog.Logger
}

// NewOrphanCleaner creates an OrphanCleaner.
func NewOrphanCleaner(pidDir string, logger *slog.Logger) *OrphanCleaner {
	if pidDir == "" {
		pidDir = defaultPIDDir
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &OrphanCleaner{pidDir: pidDir, logger: logger}
}

// WritePIDFile stores pid metadata for a running instance.
func (c *OrphanCleaner) WritePIDFile(instanceID string, pid int, coreType string, ports []int) error {
	if instanceID == "" || pid <= 0 {
		return fmt.Errorf("invalid pid metadata")
	}
	if err := os.MkdirAll(c.pidDir, 0755); err != nil {
		return fmt.Errorf("create pid dir: %w", err)
	}

	payload := PIDFile{
		PID:        pid,
		InstanceID: instanceID,
		CoreType:   coreType,
		Status:     "active",
		CreatedAt:  time.Now(),
		Ports:      append([]int(nil), ports...),
	}

	path := c.pidPath(instanceID)
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal pid file: %w", err)
	}
	if err := os.WriteFile(path, data, pidFilePermissions); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	return nil
}

// MarkDraining marks an instance pid file as draining.
func (c *OrphanCleaner) MarkDraining(instanceID string) error {
	payload, path, err := c.loadPID(instanceID)
	if err != nil {
		return err
	}
	payload.Status = "draining"
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal pid file: %w", err)
	}
	if err := os.WriteFile(path, data, pidFilePermissions); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	return nil
}

// RemovePIDFile deletes the pid file for the instance.
func (c *OrphanCleaner) RemovePIDFile(instanceID string) error {
	if instanceID == "" {
		return nil
	}
	if err := os.Remove(c.pidPath(instanceID)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove pid file: %w", err)
	}
	return nil
}

// CleanupOrphans scans pid files and terminates lingering processes.
func (c *OrphanCleaner) CleanupOrphans(ctx context.Context) error {
	entries, err := os.ReadDir(c.pidDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read pid dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		instanceID := strings.TrimSuffix(entry.Name(), ".json")
		payload, path, err := c.loadPID(instanceID)
		if err != nil {
			c.logger.Warn("load pid file failed", "file", entry.Name(), "error", err)
			continue
		}

		if payload.PID <= 0 || !isProcessRunning(payload.PID) {
			_ = os.Remove(path)
			continue
		}

		if err := terminateProcess(ctx, payload.PID); err != nil {
			c.logger.Error("terminate orphan process failed", "pid", payload.PID, "error", err)
			continue
		}
		_ = os.Remove(path)
	}

	return nil
}

func (c *OrphanCleaner) pidPath(instanceID string) string {
	return filepath.Join(c.pidDir, instanceID+".json")
}

func (c *OrphanCleaner) loadPID(instanceID string) (PIDFile, string, error) {
	path := c.pidPath(instanceID)
	data, err := os.ReadFile(path)
	if err != nil {
		return PIDFile{}, path, fmt.Errorf("read pid file: %w", err)
	}
	var payload PIDFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return PIDFile{}, path, fmt.Errorf("unmarshal pid file: %w", err)
	}
	return payload, path, nil
}

func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func terminateProcess(ctx context.Context, pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		proc.Signal(syscall.SIGKILL)
		return ctx.Err()
	case <-timer.C:
	}

	return proc.Signal(syscall.SIGKILL)
}

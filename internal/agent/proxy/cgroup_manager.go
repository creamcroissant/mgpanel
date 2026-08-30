package proxy

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultCgroupBase = "/sys/fs/cgroup/xboard"

var errCgroupUnsupported = errors.New("cgroup v2 not supported")

// CgroupManager manages cgroup v2 process groups for proxy cores.
type CgroupManager struct {
	basePath string
	logger   *slog.Logger
}

// NewCgroupManager creates a cgroup manager with a base path.
func NewCgroupManager(basePath string, logger *slog.Logger) *CgroupManager {
	if strings.TrimSpace(basePath) == "" {
		basePath = defaultCgroupBase
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &CgroupManager{basePath: basePath, logger: logger}
}

// IsSupported reports whether cgroup v2 is available on this host.
func (m *CgroupManager) IsSupported() bool {
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return false
	}
	return true
}

// CreateGroup creates a cgroup under the manager base path.
func (m *CgroupManager) CreateGroup(name string) (string, error) {
	if !m.IsSupported() {
		return "", errCgroupUnsupported
	}
	group, err := sanitizeCgroupName(name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(m.basePath, 0755); err != nil {
		return "", fmt.Errorf("create cgroup base: %w", err)
	}
	path := filepath.Join(m.basePath, group)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", fmt.Errorf("create cgroup: %w", err)
	}
	return path, nil
}

// AddProcess moves a process into the named cgroup.
func (m *CgroupManager) AddProcess(group string, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	path, err := m.CreateGroup(group)
	if err != nil {
		return err
	}
	procsPath := filepath.Join(path, "cgroup.procs")
	if err := os.WriteFile(procsPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("write cgroup procs: %w", err)
	}
	return nil
}

// KillGroup terminates all processes in the named cgroup.
func (m *CgroupManager) KillGroup(group string) error {
	if !m.IsSupported() {
		return errCgroupUnsupported
	}
	path, err := m.groupPath(group)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat cgroup: %w", err)
	}

	killPath := filepath.Join(path, "cgroup.kill")
	if _, err := os.Stat(killPath); err == nil {
		if err := os.WriteFile(killPath, []byte("1"), 0644); err != nil {
			return fmt.Errorf("write cgroup kill: %w", err)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat cgroup kill: %w", err)
	}

	pids, err := m.readGroupPids(path)
	if err != nil {
		return err
	}
	if err := killPIDs(pids); err != nil {
		return fmt.Errorf("kill cgroup processes: %w", err)
	}
	return nil
}

func (m *CgroupManager) groupPath(name string) (string, error) {
	if !m.IsSupported() {
		return "", errCgroupUnsupported
	}
	group, err := sanitizeCgroupName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(m.basePath, group), nil
}

func (m *CgroupManager) readGroupPids(groupPath string) ([]int, error) {
	data, err := os.ReadFile(filepath.Join(groupPath, "cgroup.procs"))
	if err != nil {
		return nil, fmt.Errorf("read cgroup procs: %w", err)
	}
	fields := strings.Fields(string(data))
	pids := make([]int, 0, len(fields))
	for _, item := range fields {
		pid, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil || pid <= 0 {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func sanitizeCgroupName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed == "." || trimmed == ".." {
		return "", fmt.Errorf("invalid cgroup name: %s", name)
	}
	if strings.Contains(trimmed, string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid cgroup name: %s", name)
	}
	return trimmed, nil
}

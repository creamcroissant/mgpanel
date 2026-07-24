package proxy

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

// GroupLockManager manages locks per port group to allow parallel switching on disjoint groups.
type GroupLockManager struct {
	locks map[string]*sync.Mutex
	mu    sync.Mutex
}

// NewGroupLockManager creates a GroupLockManager.
func NewGroupLockManager() *GroupLockManager {
	return &GroupLockManager{
		locks: make(map[string]*sync.Mutex),
	}
}

// Lock acquires the mutex for the specified group.
func (m *GroupLockManager) Lock(groupID string) {
	m.getLock(groupID).Lock()
}

// Unlock releases the mutex for the specified group.
func (m *GroupLockManager) Unlock(groupID string) {
	m.getLock(groupID).Unlock()
}

// TryLock attempts to acquire the mutex for the specified group.
func (m *GroupLockManager) TryLock(groupID string) bool {
	lock := m.getLock(groupID)
	return lock.TryLock()
}

// RemoveGroupLock removes the mutex for the specified group from the map.
func (m *GroupLockManager) RemoveGroupLock(groupID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.locks, groupID)
}

func (m *GroupLockManager) getLock(groupID string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[groupID]
	if !ok {
		lock = &sync.Mutex{}
		m.locks[groupID] = lock
	}
	return lock
}

// ComputeGroupID returns a deterministic group ID by sorting and joining ports.
func ComputeGroupID(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	copied := append([]int(nil), ports...)
	sort.Ints(copied)
	parts := make([]string, 0, len(copied))
	for _, port := range copied {
		parts = append(parts, strconv.Itoa(port))
	}
	return strings.Join(parts, "-")
}

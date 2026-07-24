package proxy

import (
	"sort"
)

// GetGroup returns a snapshot of the group by id.
func (s *Switcher) GetGroup(groupID string) *InstanceGroup {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	group := s.groups[groupID]
	if group == nil {
		return nil
	}
	return cloneGroup(group)
}

// ListGroups returns all groups tracked by Switcher.
func (s *Switcher) ListGroups() []*InstanceGroup {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()

	groups := make([]*InstanceGroup, 0, len(s.groups))
	for _, group := range s.groups {
		groups = append(groups, cloneGroup(group))
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ID < groups[j].ID
	})
	return groups
}

// addSlotToGroup appends a slot to an existing group or creates the group.
func (s *Switcher) addSlotToGroup(groupID string, slot SlotInfo, externalPorts []int) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()

	group, ok := s.groups[groupID]
	if !ok {
		s.groups[groupID] = &InstanceGroup{
			ID:            groupID,
			ExternalPorts: externalPorts,
			Slots:         []SlotInfo{slot},
		}
		return
	}
	// Update external ports on existing group if not already set
	if len(externalPorts) > 0 && len(group.ExternalPorts) == 0 {
		group.ExternalPorts = externalPorts
	}
	group.Slots = append(group.Slots, slot)
}

// removeSlotFromGroup removes a slot by instance ID from the group.
func (s *Switcher) removeSlotFromGroup(groupID, instanceID string) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()

	group, ok := s.groups[groupID]
	if !ok {
		return
	}
	filtered := make([]SlotInfo, 0, len(group.Slots))
	for _, slot := range group.Slots {
		if slot.InstanceID != instanceID {
			filtered = append(filtered, slot)
		}
	}
	if len(filtered) == 0 {
		delete(s.groups, groupID)
		s.groupLocks.RemoveGroupLock(groupID)
		return
	}
	group.Slots = filtered
}

func (s *Switcher) updateSlotStatus(groupID, instanceID, status string) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	group, ok := s.groups[groupID]
	if !ok {
		return
	}
	for i := range group.Slots {
		if group.Slots[i].InstanceID == instanceID {
			group.Slots[i].Status = status
			return
		}
	}
}

// findInstanceSlot locates an instance by ID across all groups.
// Returns the groupID, slot copy, and external ports.
func (s *Switcher) findInstanceSlot(instanceID string) (string, *SlotInfo, []int) {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()

	for _, group := range s.groups {
		for _, slot := range group.Slots {
			if slot.InstanceID == instanceID {
				slotCopy := slot
				extPorts := clonePorts(group.ExternalPorts)
				return group.ID, &slotCopy, extPorts
			}
		}
	}
	return "", nil, nil
}

// GetInstance returns the slot info for a specific instance, or nil.
func (s *Switcher) GetInstance(instanceID string) *SlotInfo {
	_, slot, _ := s.findInstanceSlot(instanceID)
	return slot
}

func (s *Switcher) setGroup(group *InstanceGroup) {
	if group == nil {
		return
	}
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	s.groups[group.ID] = cloneGroup(group)
}

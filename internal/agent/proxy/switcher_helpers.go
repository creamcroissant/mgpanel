package proxy

import (
	"strings"
)

// makeDNATRules builds a set of DNAT rules from external/internal port mappings.
func makeDNATRules(externalPorts []int, portMappings map[int]int) []*DNATRule {
	rules := make([]*DNATRule, 0, len(externalPorts))
	for _, extPort := range externalPorts {
		intPort := portMappings[extPort]
		if extPort > 0 && intPort > 0 {
			rules = append(rules, &DNATRule{
				ExternalPort: extPort,
				InternalPort: intPort,
				Protocol:     "both",
			})
		}
	}
	return rules
}

// portMappingFromSlot returns the stored port mappings from a slot.
func portMappingFromSlot(slot *SlotInfo) map[int]int {
	if slot == nil {
		return nil
	}
	return slot.PortMappings
}

func normalizeSwitcherConfig(cfg SwitcherConfig) SwitcherConfig {
	if cfg.PortRangeStart <= 0 {
		cfg.PortRangeStart = 30000
	}
	if cfg.PortRangeEnd <= 0 {
		cfg.PortRangeEnd = 40000
	}
	if cfg.PortRangeEnd < cfg.PortRangeStart {
		cfg.PortRangeEnd = cfg.PortRangeStart
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 10
	}
	if cfg.HealthTimeout <= 0 {
		cfg.HealthTimeout = defaultHealthTimeout
	}
	if cfg.HealthInterval <= 0 {
		cfg.HealthInterval = defaultHealthInterval
	}
	if cfg.DrainTimeout <= 0 {
		cfg.DrainTimeout = defaultDrainTimeout
	}
	if strings.TrimSpace(cfg.NftBin) == "" {
		cfg.NftBin = "/usr/sbin/nft"
	}
	if strings.TrimSpace(cfg.ConntrackBin) == "" {
		cfg.ConntrackBin = "conntrack"
	}
	if strings.TrimSpace(cfg.NftTableName) == "" {
		cfg.NftTableName = "mgpanel_proxy"
	}
	if strings.TrimSpace(cfg.PIDDir) == "" {
		cfg.PIDDir = defaultPIDDir
	}
	if strings.TrimSpace(cfg.CgroupBasePath) == "" {
		cfg.CgroupBasePath = defaultCgroupBase
	}
	return cfg
}

func internalPortsFromMappings(externalPorts []int, mappings map[int]int) []int {
	ports := make([]int, 0, len(externalPorts))
	for _, external := range externalPorts {
		internal := mappings[external]
		if internal > 0 {
			ports = append(ports, internal)
		}
	}
	return ports
}

func hasMappingConflict(occupied map[int]bool, ports []int) bool {
	if len(ports) == 0 {
		return true
	}
	seen := make(map[int]bool, len(ports))
	for _, port := range ports {
		if port <= 0 {
			return true
		}
		if seen[port] {
			return true
		}
		seen[port] = true
		if occupied != nil && occupied[port] {
			return true
		}
	}
	return false
}

func clonePortMap(m map[int]int) map[int]int {
	if m == nil {
		return nil
	}
	out := make(map[int]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func clonePorts(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	out := make([]int, len(values))
	copy(out, values)
	return out
}

func cloneGroup(group *InstanceGroup) *InstanceGroup {
	if group == nil {
		return nil
	}
	slots := make([]SlotInfo, len(group.Slots))
	for i, s := range group.Slots {
		pm := make(map[int]int, len(s.PortMappings))
		for k, v := range s.PortMappings {
			pm[k] = v
		}
		slots[i] = SlotInfo{
			InstanceID:    s.InstanceID,
			CoreType:      s.CoreType,
			InternalPorts: clonePorts(s.InternalPorts),
			PortMappings:  pm,
			Status:        s.Status,
			CreatedAt:     s.CreatedAt,
		}
	}
	return &InstanceGroup{
		ID:            group.ID,
		ExternalPorts: clonePorts(group.ExternalPorts),
		Slots:         slots,
	}
}

func sanitizeToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, value)
}

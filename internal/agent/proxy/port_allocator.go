package proxy

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// PortAllocator provides randomized port allocation within a configured range.
type PortAllocator struct {
	rangeStart int
	rangeEnd   int
	rng        *rand.Rand
	mu         sync.Mutex
}

// NewPortAllocator creates a port allocator with a randomized source.
func NewPortAllocator(start, end int) *PortAllocator {
	if start <= 0 {
		start = 30000
	}
	if end <= 0 || end < start {
		end = start
	}
	return &PortAllocator{
		rangeStart: start,
		rangeEnd:   end,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// AllocateRandom returns a random port within the configured range.
func (a *PortAllocator) AllocateRandom() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.rangeEnd <= a.rangeStart {
		return a.rangeStart
	}
	span := a.rangeEnd - a.rangeStart + 1
	return a.rangeStart + a.rng.Intn(span)
}

// AllocateWithRetry allocates port mappings and retries on address conflicts.
func (a *PortAllocator) AllocateWithRetry(
	ctx context.Context,
	externalPorts []int,
	startFn func(portMappings map[int]int) error,
	maxRetries int,
) (map[int]int, error) {
	if len(externalPorts) == 0 {
		return nil, fmt.Errorf("no external ports provided")
	}
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		portMappings := make(map[int]int, len(externalPorts))
		for _, externalPort := range externalPorts {
			portMappings[externalPort] = a.AllocateRandom()
		}

		if err := startFn(portMappings); err != nil {
			if isAddressInUse(err) {
				continue
			}
			return nil, err
		}
		return portMappings, nil
	}

	return nil, fmt.Errorf("port allocation retries exceeded")
}

func isAddressInUse(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "address already in use")
}

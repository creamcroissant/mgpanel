//go:build !windows

package proxy

import (
	"errors"
	"fmt"
	"syscall"
)

func killPIDs(pids []int) error {
	var firstErr error
	for _, pid := range pids {
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr != nil {
		return fmt.Errorf("kill pids: %w", firstErr)
	}
	return nil
}

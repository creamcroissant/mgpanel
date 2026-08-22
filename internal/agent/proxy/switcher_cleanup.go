package proxy

import (
	"context"
	"fmt"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/core"
)

// Shutdown performs best-effort cleanup.
func (s *Switcher) Shutdown(ctx context.Context) error {
	if s.orphanCleaner != nil {
		if err := s.orphanCleaner.CleanupOrphans(ctx); err != nil {
			s.logger.Warn("cleanup orphans failed", "error", err)
		}
	}
	return nil
}

func (s *Switcher) asyncCleanup(ctx context.Context, instanceID string, groupID string) {
	if instanceID == "" {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("async cleanup panic recovered", "error", r)
			}
		}()
		if s.orphanCleaner != nil {
			if err := s.orphanCleaner.MarkDraining(instanceID); err != nil {
				s.logger.Warn("mark draining failed", "instance_id", instanceID, "error", err)
			}
		}
		s.updateSlotStatus(groupID, instanceID, "draining")

		select {
		case <-ctx.Done():
			s.logger.WarnContext(ctx, "switcher: cleanup cancelled during drain", "instance_id", instanceID, "group_id", groupID)
			return
		case <-time.After(s.config.DrainTimeout):
		}

		stopCtx, stopCancel := context.WithTimeout(ctx, 5*time.Second)
		defer stopCancel()

		stopErr := s.coreMgr.StopInstance(stopCtx, instanceID)
		if stopErr != nil {
			s.logger.Warn("stop old instance failed", "instance_id", instanceID, "error", stopErr)
			if s.cgroupMgr != nil && s.cgroupMgr.IsSupported() {
				if err := s.cgroupMgr.KillGroup(instanceID); err != nil {
					s.logger.Warn("kill cgroup failed", "instance_id", instanceID, "error", err)
				}
			}
		}

		if s.orphanCleaner != nil {
			if err := s.orphanCleaner.RemovePIDFile(instanceID); err != nil {
				s.logger.Warn("remove pid file failed", "instance_id", instanceID, "error", err)
			}
		}

		s.removeSlotFromGroup(groupID, instanceID)
	}()
}

func (s *Switcher) ensurePIDTracking(ctx context.Context, coreType string, instanceID string, ports []int) error {
	if s.orphanCleaner == nil {
		return nil
	}
	coreImpl, ok := s.coreMgr.GetCore(core.CoreType(coreType))
	if !ok {
		return fmt.Errorf("core not registered: %s", coreType)
	}

	inst, err := coreImpl.Status(ctx, instanceID)
	if err != nil {
		return err
	}

	if inst == nil {
		s.logger.Warn("instance status is nil", "instance_id", instanceID)
		return nil
	}

	if inst.PID <= 0 {
		s.logger.Warn("instance pid not available, skip pid tracking", "instance_id", instanceID)
		return nil
	}

	if s.cgroupMgr != nil && s.cgroupMgr.IsSupported() {
		if err := s.cgroupMgr.AddProcess(instanceID, inst.PID); err != nil {
			s.logger.Warn("add process to cgroup failed", "instance_id", instanceID, "error", err)
		}
	}
	return s.orphanCleaner.WritePIDFile(instanceID, inst.PID, string(coreType), ports)
}

func (s *Switcher) cleanupNewInstance(instanceID string, pidFileWritten bool) {
	if instanceID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.coreMgr.StopInstance(ctx, instanceID); err != nil {
		s.logger.Warn("stop instance failed", "instance_id", instanceID, "error", err)
		if s.cgroupMgr != nil && s.cgroupMgr.IsSupported() {
			if err := s.cgroupMgr.KillGroup(instanceID); err != nil {
				s.logger.Warn("kill cgroup failed", "instance_id", instanceID, "error", err)
			}
		}
	}
	if pidFileWritten && s.orphanCleaner != nil {
		if err := s.orphanCleaner.RemovePIDFile(instanceID); err != nil {
			s.logger.Warn("remove pid file failed", "instance_id", instanceID, "error", err)
		}
	}
}

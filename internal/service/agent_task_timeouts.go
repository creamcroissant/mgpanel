package service

import "time"

// Package-level timeout values for agent task execution.
// Set via SetAgentTaskTimeouts during bootstrap (serve.go).
// Zero values mean "use default".
var (
	coreOperationClaimTimeout      = 2 * time.Minute
	agentLifecycleOperationClaimTimeout = 2 * time.Minute
	applyRunClaimTimeout            = 10 * time.Minute
)

// SetAgentTaskTimeouts configures agent task timeout values from config.
// Called during bootstrap. Zero values keep existing defaults.
func SetAgentTaskTimeouts(coreOp, lifecycleOp, applyRun time.Duration) {
	if coreOp > 0 {
		coreOperationClaimTimeout = coreOp
	}
	if lifecycleOp > 0 {
		agentLifecycleOperationClaimTimeout = lifecycleOp
	}
	if applyRun > 0 {
		applyRunClaimTimeout = applyRun
	}
}

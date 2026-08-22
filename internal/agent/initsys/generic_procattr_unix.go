//go:build !windows

package initsys

import (
	"os/exec"
	"syscall"
)

func configureGenericCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}

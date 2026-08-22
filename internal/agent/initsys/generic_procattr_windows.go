//go:build windows

package initsys

import "os/exec"

func configureGenericCommand(cmd *exec.Cmd) {
	_ = cmd
}

//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
}
func platformVersion() string { return "10.0" } // Supported Windows 10/11 both report NT 10.0.

//go:build !windows

package core

import (
	"os/exec"
	"strings"
)

func configureProcess(cmd *exec.Cmd) {}
func platformVersion() string {
	b, _ := exec.Command("uname", "-r").Output()
	return strings.TrimSpace(string(b))
}

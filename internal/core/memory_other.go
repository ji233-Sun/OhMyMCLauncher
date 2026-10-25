//go:build !windows

package core

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func SystemMemory() (MemoryStatus, error) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MemoryStatus{}, errors.New("此平台暂时无法读取系统物理内存，请使用自定义方案")
	}
	var total, available uint64
	for _, line := range strings.Split(string(b), "\n") {
		var key string
		var value uint64
		if _, err := fmt.Sscanf(line, "%s %d kB", &key, &value); err == nil {
			switch key {
			case "MemTotal:":
				total = value / 1024
			case "MemAvailable:":
				available = value / 1024
			}
		}
	}
	if total == 0 {
		return MemoryStatus{}, errors.New("系统未返回物理内存容量，请使用自定义方案")
	}
	return memoryStatus(total, available), nil
}

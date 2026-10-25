//go:build windows

package core

import (
	"fmt"
	"syscall"
	"unsafe"
)

var globalMemoryStatusEx = syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

func SystemMemory() (MemoryStatus, error) {
	var status struct {
		Length, Load                       uint32
		TotalPhys, AvailPhys               uint64
		TotalPage, AvailPage, TotalVirtual uint64
		AvailVirtual, AvailExtendedVirtual uint64
	}
	status.Length = uint32(unsafe.Sizeof(status))
	ok, _, err := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if ok == 0 {
		return MemoryStatus{}, fmt.Errorf("读取系统物理内存失败: %w", err)
	}
	return memoryStatus(status.TotalPhys>>20, status.AvailPhys>>20), nil
}

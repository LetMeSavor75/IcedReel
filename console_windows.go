//go:build windows

package main

import (
	"os"
	"syscall"
)

const enableVirtualTerminalProcessing = 0x0004

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleOutputCP = kernel32.NewProc("GetConsoleOutputCP")
	procSetConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
	procSetConsoleMode     = kernel32.NewProc("SetConsoleMode")
)

func enableUTF8Console() func() {
	info, err := os.Stdout.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return func() {}
	}

	oldCP, _, _ := procGetConsoleOutputCP.Call()
	cpChanged := false
	if oldCP != 0 && oldCP != 65001 {
		if r, _, _ := procSetConsoleOutputCP.Call(65001); r != 0 {
			cpChanged = true
		}
	}

	out, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	var oldMode uint32
	vtChanged := false
	if err == nil && syscall.GetConsoleMode(out, &oldMode) == nil && oldMode&enableVirtualTerminalProcessing == 0 {
		if r, _, _ := procSetConsoleMode.Call(uintptr(out), uintptr(oldMode|enableVirtualTerminalProcessing)); r != 0 {
			vtChanged = true
		}
	}

	return func() {
		if cpChanged {
			procSetConsoleOutputCP.Call(oldCP)
		}
		if vtChanged {
			procSetConsoleMode.Call(uintptr(out), uintptr(oldMode))
		}
	}
}

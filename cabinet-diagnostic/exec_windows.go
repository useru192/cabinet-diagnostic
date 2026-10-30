//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procSetConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
	procSetConsoleCP       = kernel32.NewProc("SetConsoleCP")
)

// initConsole 把控制台输入输出代码页切到 UTF-8。
// 设备端输出可能包含中文，否则会显示为乱码。
func initConsole() {
	procSetConsoleOutputCP.Call(65001)
	procSetConsoleCP.Call(65001)
}

// hideWindow 避免每次执行 adb 时闪出黑色窗口。
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

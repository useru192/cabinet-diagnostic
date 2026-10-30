//go:build !windows

package adb

import "os/exec"

// InitConsole 在非 Windows 平台上无需处理控制台代码页。
func InitConsole() {}

func hideWindow(cmd *exec.Cmd) {}

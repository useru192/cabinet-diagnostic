//go:build !windows

package main

import "os/exec"

func initConsole() {}

func hideWindow(cmd *exec.Cmd) {}

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

var stdin = bufio.NewReader(os.Stdin)

func main() {
	initConsole()
	printBanner()

	runner, err := NewRunner()
	if err != nil {
		fmt.Printf("[ERROR] ADB not available: %v\n", err)
		fmt.Println("        Set the ADB_PATH environment variable to a valid adb.exe.")
		fmt.Println()
		pressEnter()
		return
	}
	fmt.Printf("[INFO]   ADB: %s\n", runner.AdbPath())

	ctx := context.Background()

	// 连接检测：失败时允许重试（未授权场景需要在设备屏幕上点允许）
	for {
		conn := runner.DetectConnection(ctx)
		if conn.Status == ConnReady {
			fmt.Printf("[OK]     Device connected: %s\n", conn.Serial)
			break
		}

		printConnProblem(conn)
		fmt.Println()
		fmt.Println("  [Enter] Retry    [0] Exit")
		fmt.Print("  > ")
		line, ok := readLine()
		if !ok || line == "0" {
			return
		}
		fmt.Println()
	}

	// 设备基础信息，同时验证设备是否真正响应命令
	info, err := runner.DeviceInfo(ctx)
	if err != nil {
		if errors.Is(err, ErrTimeout) {
			fmt.Println("[TIMEOUT] Device accepted the connection but does not respond to commands.")
		} else {
			fmt.Printf("[ERROR]   Failed to read device info: %v\n", err)
		}
		fmt.Println()
		pressEnter()
		return
	}
	printDeviceInfo(info)

	menuLoop()
}

// menuLoop 主菜单循环。非法输入不会导致程序退出。
func menuLoop() {
	for {
		printMenu()

		line, ok := readLine()
		if !ok {
			fmt.Println()
			return
		}

		switch line {
		case "1":
			notImplemented("Check System Status")
		case "2":
			notImplemented("Check Network Connectivity")
		case "3":
			notImplemented("Run Full Diagnostics")
		case "0":
			fmt.Println("Bye.")
			return
		default:
			fmt.Println("Invalid option, please try again.")
			fmt.Println()
		}
	}
}

func printBanner() {
	fmt.Println("========================================")
	fmt.Println("   " + AppName)
	fmt.Println("========================================")
	fmt.Println()
}

func printMenu() {
	fmt.Println()
	fmt.Println("----------------------------------------")
	fmt.Println("1. Check System Status")
	fmt.Println("2. Check Network Connectivity")
	fmt.Println("3. Run Full Diagnostics")
	fmt.Println("0. Exit")
	fmt.Println("----------------------------------------")
	fmt.Print("Please select an option: ")
}

func printDeviceInfo(i DeviceInfo) {
	root := "NO"
	if i.Root {
		root = "YES"
	}
	fmt.Println()
	fmt.Println("--------------------------------------------------------------")
	fmt.Printf("%-16s: %s\n", "Serial", orUnknown(i.Serial))
	fmt.Printf("%-16s: %s\n", "Model", orUnknown(i.Model))
	fmt.Printf("%-16s: %s\n", "Android version", orUnknown(i.AndroidVersion))
	fmt.Printf("%-16s: %s\n", "Kernel", orUnknown(i.Kernel))
	fmt.Printf("%-16s: %s\n", "Uptime", orUnknown(i.Uptime))
	fmt.Printf("%-16s: %s\n", "Root", root)
	fmt.Println("--------------------------------------------------------------")
}

func printConnProblem(c DeviceConn) {
	switch c.Status {
	case ConnADBNotFound:
		fmt.Println("[ERROR] adb not found.")
		fmt.Println("        Check ADB installation, or set ADB_PATH.")
	case ConnStartFailed:
		fmt.Printf("[ERROR] Failed to start adb server: %s\n", c.Detail)
		fmt.Println("        Check whether another process is blocking port 5037.")
	case ConnNoDevice:
		fmt.Println("[ERROR] No device detected.")
		fmt.Println("        Check the USB cable and the USB port.")
	case ConnUnauthorized:
		fmt.Printf("[ERROR] Device %s is unauthorized.\n", orUnknown(c.Serial))
		fmt.Println("        Please check the device screen and tap \"Allow\"")
		fmt.Println("        on the USB debugging prompt, then press Enter to retry.")
	case ConnOffline:
		fmt.Printf("[ERROR] Device %s is offline.\n", orUnknown(c.Serial))
		fmt.Println("        Try reconnecting the USB cable.")
	case ConnTimeout:
		fmt.Printf("[TIMEOUT] %s\n", c.Detail)
		fmt.Println("          The device may be hung or the USB connection is unstable.")
	default:
		fmt.Printf("[UNKNOWN] Cannot determine device state (%s).\n", c.Detail)
	}
}

// notImplemented 第一阶段只做骨架，检测功能在后续阶段接入。
func notImplemented(name string) {
	fmt.Printf("[INFO] \"%s\" is not implemented yet.\n", name)
	fmt.Println("       It will be added in the next phase.")
}

func pressEnter() {
	fmt.Print("Press Enter to exit...")
	readLine()
}

// readLine 读取一行输入。返回 false 表示输入流已结束。
func readLine() (string, bool) {
	s, err := stdin.ReadString('\n')
	s = strings.TrimSpace(s)
	if err != nil && s == "" {
		return "", false
	}
	return s, true
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "[UNKNOWN]"
	}
	return s
}

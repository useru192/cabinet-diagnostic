// 换电柜主控诊断工具：通过 adb 检测设备状态，并生成检测日志供研发分析。
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"cabinet-diagnostic/internal/action"
	"cabinet-diagnostic/internal/adb"
	"cabinet-diagnostic/internal/check"
	"cabinet-diagnostic/internal/config"
)

var stdin = bufio.NewReader(os.Stdin)

func main() {
	adb.InitConsole()
	printBanner()

	runner, err := adb.NewRunner()
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
	var conn adb.DeviceConn
	for {
		conn = runner.DetectConnection(ctx)
		if conn.Status == adb.ConnReady {
			fmt.Println("[OK]     Device connected.")
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
		// 信息读取失败时仍要把设备标识打出来，否则用户不知道连的是哪台
		if errors.Is(err, adb.ErrTimeout) {
			fmt.Printf("[TIMEOUT] Device %s accepted the connection but does not respond to commands.\n",
				orUnknown(conn.Serial))
		} else {
			fmt.Printf("[ERROR]   Device %s: failed to read device info: %v\n",
				orUnknown(conn.Serial), err)
		}
		fmt.Println()
		pressEnter()
		return
	}
	printDeviceInfo(info)

	// 一次启动只产生一份报告 + 一份审计日志，菜单复测追加到同一份。
	sess := check.NewSession(ctx, runner, info)
	defer sess.Close()

	auditor := action.NewAuditor(info)
	defer auditor.Close()

	// 启动体检：系统 + 网络，只输出一个 SUMMARY
	sess.Startup(ctx)

	menuLoop(ctx, runner, sess, auditor)
}

// menuLoop 主菜单循环。非法输入不会导致程序退出。
func menuLoop(ctx context.Context, runner *adb.Runner, sess *check.Session, auditor *action.Auditor) {
	for {
		printMenu()

		line, ok := readLine()
		if !ok {
			fmt.Println()
			return
		}

		switch line {
		case "1":
			sess.System(ctx)
		case "2":
			sess.Network(ctx)
		case "3":
			// 删除流程自带交互：返回 false 表示用户输入空行返回菜单。
			for {
				if !action.DeleteFlow(ctx, runner, auditor, readLine) {
					break
				}
			}
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
	fmt.Println("   " + config.AppName)
	fmt.Println("========================================")
	fmt.Println("Version: " + config.AppVersion)
}

func printMenu() {
	fmt.Println()
	fmt.Println("----------------------------------------")
	fmt.Println("1. Check System Status")
	fmt.Println("2. Check Network Connectivity")
	fmt.Println("3. Delete Device File")
	fmt.Println("0. Exit")
	fmt.Println("----------------------------------------")
	fmt.Print("Please select an option: ")
}

func printDeviceInfo(i adb.DeviceInfo) {
	root := "NO"
	if i.Root {
		root = "YES"
	}

	fmt.Println()
	fmt.Println("--------------------------------------------------------------")
	fmt.Printf("%-16s: %s\n", "Serial", orUnknown(i.Serial))
	fmt.Printf("%-16s: %s\n", "Device ID", orUnknown(i.DeviceID))
	fmt.Printf("%-16s: %s\n", "Uptime", orUnknown(i.Uptime))
	fmt.Printf("%-16s: %s\n", "Root", root)
	fmt.Println("--------------------------------------------------------------")
}

func printConnProblem(c adb.DeviceConn) {
	switch c.Status {
	case adb.ConnADBNotFound:
		fmt.Println("[ERROR] adb not found.")
		fmt.Println("        Check ADB installation, or set ADB_PATH.")
	case adb.ConnStartFailed:
		fmt.Printf("[ERROR] Failed to start adb server: %s\n", c.Detail)
		fmt.Println("        Check whether another process is blocking port 5037.")
	case adb.ConnNoDevice:
		fmt.Println("[ERROR] No device detected.")
		fmt.Println("        Check the USB cable and the USB port.")
	case adb.ConnUnauthorized:
		fmt.Printf("[ERROR] Device %s is unauthorized.\n", orUnknown(c.Serial))
		fmt.Println("        Please check the device screen and tap \"Allow\"")
		fmt.Println("        on the USB debugging prompt, then press Enter to retry.")
	case adb.ConnOffline:
		fmt.Printf("[ERROR] Device %s is offline.\n", orUnknown(c.Serial))
		fmt.Println("        Try reconnecting the USB cable.")
	case adb.ConnTimeout:
		fmt.Printf("[TIMEOUT] %s\n", c.Detail)
		fmt.Println("          The device may be hung or the USB connection is unstable.")
	default:
		fmt.Printf("[UNKNOWN] Cannot determine device state (%s).\n", c.Detail)
	}
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

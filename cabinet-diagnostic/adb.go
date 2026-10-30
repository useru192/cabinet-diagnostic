package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// adb.exe 及其在 Windows 上必须同目录存在的两个依赖 DLL。
// 内嵌的目的是让交付物只有一个 exe，避免 "AdbWinApi.dll was not found"。
//
//go:embed assets
var assetsFS embed.FS

var embeddedAdbFiles = []string{
	"assets/adb.exe",
	"assets/AdbWinApi.dll",
	"assets/AdbWinUsbApi.dll",
}

// ErrTimeout 表示命令在超时时间内没有返回。
var ErrTimeout = errors.New("command timed out")

// Output 是一次 adb 命令的执行结果。
// 注意：ExitCode 非 0 不视为执行错误，由调用方根据业务语义判断。
type Output struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}

// Combined 返回适合写入检测日志的完整输出。
func (o Output) Combined() string {
	var b strings.Builder
	b.WriteString(o.Stdout)
	if s := strings.TrimRight(o.Stderr, "\r\n"); s != "" {
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteString("\n")
		}
		b.WriteString(s)
	}
	return b.String()
}

// Runner 封装 adb 命令执行。
type Runner struct {
	adbPath string
	serial  string
}

// NewRunner 解析 adb 路径并返回执行器。
// 优先级：环境变量 ADB_PATH > 内嵌 adb（解压到 %TEMP%）。
func NewRunner() (*Runner, error) {
	p, err := resolveAdb()
	if err != nil {
		return nil, err
	}
	return &Runner{adbPath: p}, nil
}

// AdbPath 返回实际使用的 adb.exe 路径。
func (r *Runner) AdbPath() string { return r.adbPath }

// Serial 返回当前绑定的设备序列号。
func (r *Runner) Serial() string { return r.serial }

// SetSerial 绑定设备序列号，之后所有命令自动带 -s。
func (r *Runner) SetSerial(s string) { r.serial = s }

// Run 执行一条 adb 命令。
func (r *Runner) Run(ctx context.Context, timeout time.Duration, args ...string) (Output, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	argv := args
	if r.serial != "" {
		argv = append([]string{"-s", r.serial}, args...)
	}

	cmd := exec.CommandContext(cctx, r.adbPath, argv...)
	hideWindow(cmd)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	out := Output{Stdout: stdout.String(), Stderr: stderr.String()}

	if errors.Is(cctx.Err(), context.DeadlineExceeded) {
		out.TimedOut = true
		return out, ErrTimeout
	}

	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			out.ExitCode = exitErr.ExitCode()
			return out, nil
		}
		return out, runErr
	}

	return out, nil
}

// Shell 在设备端执行一条 shell 命令。
func (r *Runner) Shell(ctx context.Context, timeout time.Duration, script string) (Output, error) {
	return r.Run(ctx, timeout, "shell", script)
}

// ---------------------------------------------------------------
// 连接状态
// ---------------------------------------------------------------

// ConnStatus 表示 ADB 连接状态。
type ConnStatus int

const (
	ConnReady ConnStatus = iota
	ConnADBNotFound
	ConnStartFailed
	ConnNoDevice
	ConnUnauthorized
	ConnOffline
	ConnTimeout
	ConnUnknown
)

// DeviceConn 是一次连接检测的结果。
type DeviceConn struct {
	Status ConnStatus
	Serial string
	Detail string
}

// DetectConnection 依次检查 adb 可用性、设备数量与授权状态。
func (r *Runner) DetectConnection(ctx context.Context) DeviceConn {
	if _, err := r.Run(ctx, TimeoutServerStart, "start-server"); err != nil {
		if errors.Is(err, ErrTimeout) {
			return DeviceConn{Status: ConnTimeout, Detail: "adb start-server timed out"}
		}
		return DeviceConn{Status: ConnStartFailed, Detail: err.Error()}
	}

	out, err := r.Run(ctx, TimeoutCommon, "devices")
	if err != nil {
		if errors.Is(err, ErrTimeout) {
			return DeviceConn{Status: ConnTimeout, Detail: "adb devices timed out"}
		}
		return DeviceConn{Status: ConnStartFailed, Detail: err.Error()}
	}

	devices := parseDevices(out.Stdout)

	// 现场只会连接一台主控，不处理多设备场景。
	if len(devices) == 0 {
		return DeviceConn{Status: ConnNoDevice, Detail: "no device attached"}
	}

	d := devices[0]
	switch d.state {
	case "device":
		r.serial = d.serial
		return DeviceConn{Status: ConnReady, Serial: d.serial}
	case "unauthorized":
		return DeviceConn{Status: ConnUnauthorized, Serial: d.serial}
	case "offline":
		return DeviceConn{Status: ConnOffline, Serial: d.serial}
	default:
		return DeviceConn{Status: ConnUnknown, Serial: d.serial, Detail: d.state}
	}
}

type adbDevice struct {
	serial string
	state  string
}

// parseDevices 解析 adb devices 的输出。
//
//	List of devices attached
//	8925c67aefcd0126	device
func parseDevices(stdout string) []adbDevice {
	var devices []adbDevice
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "List of devices") || strings.HasPrefix(line, "*") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		devices = append(devices, adbDevice{serial: fields[0], state: fields[1]})
	}
	return devices
}

// ---------------------------------------------------------------
// 设备基础信息
// ---------------------------------------------------------------

// DeviceInfo 是设备的基础信息快照。
type DeviceInfo struct {
	Serial         string
	Model          string
	AndroidVersion string
	Kernel         string
	Uptime         string
	Root           bool
}

// DeviceInfo 读取设备基础信息。任一命令超时都会返回错误，
// 调用方据此判断设备是否"无响应"。
func (r *Runner) DeviceInfo(ctx context.Context) (DeviceInfo, error) {
	info := DeviceInfo{Serial: r.serial}

	idOut, err := r.Shell(ctx, TimeoutCommon, "id")
	if err != nil {
		return info, err
	}
	info.Root = strings.Contains(idOut.Stdout, "uid=0")

	info.Model = r.prop(ctx, "ro.product.model")
	info.AndroidVersion = r.prop(ctx, "ro.build.version.release")

	if o, err := r.Shell(ctx, TimeoutCommon, "uname -a"); err == nil {
		info.Kernel = strings.TrimSpace(o.Stdout)
	}
	if o, err := r.Shell(ctx, TimeoutCommon, "uptime"); err == nil {
		info.Uptime = strings.TrimSpace(o.Stdout)
	}

	return info, nil
}

func (r *Runner) prop(ctx context.Context, key string) string {
	o, err := r.Shell(ctx, TimeoutCommon, "getprop "+key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(o.Stdout)
}

// ---------------------------------------------------------------
// adb 路径解析
// ---------------------------------------------------------------

func resolveAdb() (string, error) {
	if p := strings.TrimSpace(os.Getenv("ADB_PATH")); p != "" {
		if !fileExists(p) {
			return "", fmt.Errorf("ADB_PATH points to a missing file: %s", p)
		}
		return p, nil
	}
	return extractEmbeddedAdb()
}

// extractEmbeddedAdb 把内嵌的 adb.exe 与依赖 DLL 释放到临时目录。
// 目录名带内容哈希，内容不变则复用，避免每次启动重复写盘。
func extractEmbeddedAdb() (string, error) {
	hash := sha256.New()
	blobs := make(map[string][]byte, len(embeddedAdbFiles))
	for _, name := range embeddedAdbFiles {
		b, err := assetsFS.ReadFile(name)
		if err != nil {
			return "", fmt.Errorf("embedded %s unreadable: %w", name, err)
		}
		blobs[name] = b
		hash.Write(b)
	}

	dir := filepath.Join(os.TempDir(), "cabinet-diagnostic",
		hex.EncodeToString(hash.Sum(nil))[:16])
	exePath := filepath.Join(dir, "adb.exe")
	dllPath := filepath.Join(dir, "AdbWinApi.dll")

	if fileExists(exePath) && fileExists(dllPath) {
		return exePath, nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create temp dir failed: %w", err)
	}

	for _, name := range embeddedAdbFiles {
		dst := filepath.Join(dir, filepath.Base(name))
		if err := os.WriteFile(dst, blobs[name], 0o755); err != nil {
			return "", fmt.Errorf("write %s failed: %w", dst, err)
		}
	}

	return exePath, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

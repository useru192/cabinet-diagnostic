package action

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cabinet-diagnostic/internal/adb"
	"cabinet-diagnostic/internal/config"
)

// Auditor 是本次运行的删除审计日志。
//
// 与检测报告分开存放（deletion_<设备标识>_<时间>.txt）：
// 检测报告描述设备状态，审计日志记录人为写操作，两者用途不同，不混在一起。
//
// 一次启动只有一个审计文件，多次删除追加写入。
// 文件在第一次记录时才创建：整次运行没删过东西，就不产生空文件。
type Auditor struct {
	info  adb.DeviceInfo
	f     *os.File
	path  string
	tried bool
}

// NewAuditor 创建审计写入器，此时不产生任何文件。
func NewAuditor(info adb.DeviceInfo) *Auditor {
	return &Auditor{info: info}
}

// log 追加一行审计记录，首次调用时创建文件并写头部。
func (a *Auditor) log(format string, args ...any) {
	if a == nil {
		return
	}
	a.ensure()
	if a.f == nil {
		return
	}
	fmt.Fprintf(a.f, format+"\n", args...)
}

// ensure 惰性创建审计文件。
func (a *Auditor) ensure() {
	if a.tried {
		return
	}
	a.tried = true

	name := fmt.Sprintf("deletion_%s_%s.txt", auditID(a.info), time.Now().Format("20060102_150405"))
	f, path, err := createAuditFile(name)
	if err != nil {
		fmt.Printf("[WARNING] Cannot create audit log: %v\n", err)
		return
	}
	a.f, a.path = f, path

	// UTF-8 BOM，与检测报告保持一致，保证记事本无乱码。
	fmt.Fprint(f, "\ufeff")
	fmt.Fprintln(f, "==============================================================")
	fmt.Fprintln(f, " File Deletion Audit Log")
	fmt.Fprintln(f, "==============================================================")
	fmt.Fprintf(f, "Generated at   : %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(f, "Tool version   : %s\n", config.AppVersion)
	host, _ := os.Hostname()
	fmt.Fprintf(f, "Host           : %s / %s\n", host, os.Getenv("USERNAME"))
	fmt.Fprintf(f, "Device serial  : %s\n", auditOrUnknown(a.info.Serial))
	fmt.Fprintf(f, "Device ID      : %s\n", auditOrUnknown(a.info.DeviceID))
	fmt.Fprintln(f, "==============================================================")
}

// Close 关闭审计文件。整次运行没有删除操作时不输出任何内容。
func (a *Auditor) Close() {
	if a == nil || a.f == nil {
		return
	}
	a.f.Close()
	fmt.Printf("Audit log saved: %s\n", a.path)
}

func createAuditFile(name string) (*os.File, string, error) {
	exeDir := "."
	if p, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(p)
	}

	if dir := filepath.Join(exeDir, "reports"); os.MkdirAll(dir, 0o755) == nil {
		path := filepath.Join(dir, name)
		if f, err := os.Create(path); err == nil {
			return f, path, nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", err
	}
	dir := filepath.Join(home, "Documents", "CabinetDiagnostic", "reports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}

	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return nil, "", err
	}
	return f, path, nil
}

// auditID 优先业务设备号，回退 adb 序列号（与 check 包的报告命名规则一致）。
func auditID(info adb.DeviceInfo) string {
	id := info.DeviceID
	if id == "" {
		id = info.Serial
	}

	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func auditOrUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "[UNKNOWN]"
	}
	return s
}

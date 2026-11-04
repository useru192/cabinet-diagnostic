package check

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cabinet-diagnostic/internal/adb"
	"cabinet-diagnostic/internal/config"
)

const reportSep = "--------------------------------------------------------------"

// reporter 负责把检测过程和结论写入报告文件。
//
// 每检测完一项立即落盘，不做缓冲：主控卡死时程序可能被强杀，
// 集中到最后一次写会导致前面的数据全部丢失，而现场往往只复现一次。
type reporter struct {
	f     *os.File
	path  string
	seq   int // 检测项序号，整个会话内递增
	round int // 轮次：启动时一次，之后每次菜单检测再加一轮
}

// newReporter 创建报告文件并写入头部。创建失败时返回 nil，
// 调用方自动降级为仅终端输出，不影响检测本身。
func newReporter(ctx context.Context, r *adb.Runner, info adb.DeviceInfo) *reporter {
	name := fmt.Sprintf("report_%s_%s.txt", reportID(info), time.Now().Format("20060102_150405"))
	f, path, err := createReportFile(name)
	if err != nil {
		fmt.Printf("[WARNING] Cannot create report file: %v\n", err)
		return nil
	}

	rp := &reporter{f: f, path: path}
	rp.writeHeader(ctx, r, info)
	return rp
}

func (rp *reporter) writeHeader(ctx context.Context, r *adb.Runner, info adb.DeviceInfo) {
	root := "NO"
	if info.Root {
		root = "YES"
	}
	host, _ := os.Hostname()

	// UTF-8 BOM：保证记事本正确识别编码，避免中文显示为乱码。
	fmt.Fprint(rp.f, "\ufeff")

	fmt.Fprintln(rp.f, "==============================================================")
	fmt.Fprintln(rp.f, " Cabinet Diagnostic Report")
	fmt.Fprintln(rp.f, "==============================================================")
	fmt.Fprintf(rp.f, "Generated at   : %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(rp.f, "Tool version   : %s\n", config.AppVersion)
	fmt.Fprintf(rp.f, "ADB version    : %s\n", orUnknown(r.Version(ctx)))
	fmt.Fprintf(rp.f, "Host           : %s / %s\n", host, os.Getenv("USERNAME"))
	fmt.Fprintln(rp.f, reportSep)
	fmt.Fprintf(rp.f, "Device serial  : %s\n", orUnknown(info.Serial))
	fmt.Fprintf(rp.f, "Device ID      : %s\n", orUnknown(info.DeviceID))
	fmt.Fprintf(rp.f, "Uptime         : %s\n", orUnknown(info.Uptime))
	fmt.Fprintf(rp.f, "Root           : %s\n", root)
	fmt.Fprintln(rp.f, "==============================================================")
}

// record 输出到终端并追加到报告文件，返回原结果便于串联。
// rp 为 nil 时只输出终端。
func (rp *reporter) record(res Result) Result {
	printResult(res)
	if rp == nil {
		return res
	}
	rp.writeResult(res)
	return res
}

// beginRound 写入一轮检测的分隔标题。
// 同一份报告里可能有多轮（启动体检 + 菜单复测），分隔开便于研发对照。
func (rp *reporter) beginRound(title string) {
	if rp == nil || rp.f == nil {
		return
	}
	rp.round++
	fmt.Fprintln(rp.f)
	fmt.Fprintln(rp.f, "==============================================================")
	fmt.Fprintf(rp.f, " Round %d : %s  (%s)\n", rp.round, title, time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintln(rp.f, "==============================================================")
}

func (rp *reporter) writeResult(res Result) {
	rp.seq++
	fmt.Fprintf(rp.f, "\n[%d] %s\n", rp.seq, res.Name)
	fmt.Fprintln(rp.f, reportSep)

	if res.Skipped {
		fmt.Fprintf(rp.f, "Skipped: %s\n", res.SkipReason)
		return
	}
	if res.Cmd != "" {
		fmt.Fprintf(rp.f, "$ %s\n", res.Cmd)
	}
	if raw := strings.TrimRight(res.Raw, "\r\n"); raw != "" {
		fmt.Fprintln(rp.f, raw)
	}

	fmt.Fprintln(rp.f, "Result:")
	for _, it := range res.Items {
		fmt.Fprintf(rp.f, "  %-13s %s\n", "["+string(it.Level)+"]", it.Message)
	}
}

func (rp *reporter) writeSummary(results []Result) {
	rp.seq++
	fmt.Fprintf(rp.f, "\n[%d] SUMMARY\n", rp.seq)
	fmt.Fprintln(rp.f, reportSep)
	for _, res := range results {
		fmt.Fprintf(rp.f, "%-22s: [%s]\n", res.Name, res.WorstLevel())
	}
}

// endReport 写入报告结尾。会话结束时调用一次。
func (rp *reporter) endReport() {
	if rp == nil || rp.f == nil {
		return
	}
	fmt.Fprintln(rp.f)
	fmt.Fprintln(rp.f, "==============================================================")
	fmt.Fprintln(rp.f, " End of report")
	fmt.Fprintln(rp.f, "==============================================================")
}

func (rp *reporter) close() {
	if rp == nil || rp.f == nil {
		return
	}
	rp.f.Close()
	fmt.Printf("\nReport saved: %s\n", rp.path)
}

// createReportFile 把报告写到 exe 同目录的 reports 子目录；
// 无写权限时回退到用户文档目录下的 CabinetDiagnostic。
func createReportFile(name string) (*os.File, string, error) {
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

// reportID 优先使用业务设备号，读不到时回退 adb 序列号。
func reportID(info adb.DeviceInfo) string {
	id := info.DeviceID
	if id == "" {
		id = info.Serial
	}
	return sanitizeID(id)
}

func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_':
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

// ---------------------------------------------------------------
// 终端输出
// ---------------------------------------------------------------

// printResult 把单个检测项输出到终端。
// 原始输出不在此打印，只进报告文件，避免现场人员看到一屏看不懂的内容。
func printResult(res Result) {
	fmt.Println()
	fmt.Printf("========== %s ==========\n", res.Name)

	if res.Skipped {
		fmt.Printf("Skipped: %s\n", res.SkipReason)
		return
	}

	for _, it := range res.Items {
		if it.Detail {
			continue // 明细行只写入报告，不在终端显示
		}
		fmt.Printf("%-13s %s\n", "["+string(it.Level)+"]", it.Message)
	}
}

// printSummary 输出所有检测项的总览。
func printSummary(results []Result) {
	fmt.Println()
	fmt.Println("========== SUMMARY ==========")
	for _, res := range results {
		fmt.Printf("%-22s: [%s]\n", res.Name, res.WorstLevel())
	}
}

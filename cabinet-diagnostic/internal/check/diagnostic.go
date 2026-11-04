package check

import (
	"context"
	"fmt"

	"cabinet-diagnostic/internal/adb"
)

// Session 是整次程序运行的检测会话。
//
// 一次启动只创建一份报告文件：启动体检与后续菜单复测都追加到同一份，
// 用轮次分隔，研发能看清"测了几次、结果有没有变化"。
type Session struct {
	rp *reporter
	r  *adb.Runner
}

// NewSession 创建会话并生成报告文件（写头部）。
func NewSession(ctx context.Context, r *adb.Runner, info adb.DeviceInfo) *Session {
	return &Session{rp: newReporter(ctx, r, info), r: r}
}

// Startup 执行启动体检：系统检测 + 网络检测，只输出一个 SUMMARY。
func (s *Session) Startup(ctx context.Context) []Result {
	s.rp.beginRound("STARTUP CHECK")

	var results []Result
	results = append(results, s.system(ctx)...)

	fmt.Println()
	fmt.Println("[INFO]   Checking network connectivity...")
	results = append(results, s.network(ctx)...)

	s.rp.writeSummary(results)
	printSummary(results)

	return results
}

// System 执行系统类检测（菜单 1），单独输出一个 SUMMARY。
func (s *Session) System(ctx context.Context) []Result {
	s.rp.beginRound("SYSTEM STATUS")

	results := s.system(ctx)
	s.rp.writeSummary(results)
	printSummary(results)

	return results
}

// Network 执行网络连通性检测（菜单 2），单独输出一个 SUMMARY。
func (s *Session) Network(ctx context.Context) []Result {
	s.rp.beginRound("NETWORK CONNECTIVITY")

	results := s.network(ctx)
	s.rp.writeSummary(results)
	printSummary(results)

	return results
}

// system 执行系统类检测项，不打印汇总——由调用方决定汇总粒度。
//
// 顺序：系统响应 → 磁盘 → 内存 → 大文件 → 关键文件 → 关键进程。
// 磁盘出现告警时才追加大文件分析，避免无谓的目录遍历。
func (s *Session) system(ctx context.Context) []Result {
	var results []Result

	results = append(results, s.rp.record(checkSystemResponse(ctx, s.r)))
	disk := s.rp.record(checkDisk(ctx, s.r))
	results = append(results, disk)
	results = append(results, s.rp.record(checkMemory(ctx, s.r)))
	results = append(results, s.rp.record(checkLargeFiles(ctx, s.r, disk.HasLevel(LevelWarning))))
	results = append(results, s.rp.record(checkFiles(ctx, s.r)))
	results = append(results, s.rp.record(checkProcesses(ctx, s.r)))

	return results
}

// network 执行网络检测，不打印汇总。
func (s *Session) network(ctx context.Context) []Result {
	return []Result{s.rp.record(checkNetwork(ctx, s.r))}
}

// Close 写入报告结尾并关闭文件。程序退出前调用。
func (s *Session) Close() {
	s.rp.endReport()
	s.rp.close()
}

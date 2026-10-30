package check

import (
	"context"

	"cabinet-diagnostic/internal/adb"
)

// RunSystemChecks 执行系统类检测项：系统响应、磁盘、内存。
// 磁盘出现告警时追加一次大文件占用分析。
// 每完成一项立即写入报告文件，避免中途退出丢失数据。
func RunSystemChecks(ctx context.Context, r *adb.Runner, info adb.DeviceInfo) []Result {
	rp := newReporter(ctx, r, info)
	defer rp.close()

	var results []Result

	results = append(results, rp.record(checkSystemResponse(ctx, r)))

	disk := rp.record(checkDisk(ctx, r))
	results = append(results, disk)

	results = append(results, rp.record(checkMemory(ctx, r)))
	results = append(results, rp.record(checkLargeFiles(ctx, r, disk.HasLevel(LevelWarning))))

	if rp != nil {
		rp.writeSummary(results)
	}
	printSummary(results)

	return results
}

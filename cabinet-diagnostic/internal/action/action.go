// Package action 实现对设备的写操作。
//
// 第一阶段检测工具是全程只读的，本包是唯一例外（任务9：主控文件删除）。
// 设计原则（DESIGN 4.10 预留接口的落地）：
//   - 任何写操作必须先 Describe（预览影响），用户显式确认后才 Apply；
//   - 全部操作写入审计日志，"谁在什么时候删了什么"必须可追溯；
//   - 与 check 包完全隔离：检测功能不 import 本包，删除出问题不影响体检。
package action

import (
	"context"

	"cabinet-diagnostic/internal/adb"
)

// Preview 是一次写操作的影响预览，展示给用户做删除决定。
type Preview struct {
	Path      string
	IsDir     bool
	IsSymlink bool
	Size      int64
	Owner     string
	Modified  string // ls 显示的日期 时间
}

// Action 是写操作的统一接口。后续如需"清理日志"等批量操作，照此接入。
type Action interface {
	// Name 返回操作名称，用于终端与审计日志。
	Name() string
	// Describe 只读地预览操作影响，不产生任何写动作。
	Describe(ctx context.Context, r *adb.Runner) (Preview, error)
	// Apply 执行操作。confirm 为 false 时必须拒绝执行。
	Apply(ctx context.Context, r *adb.Runner, confirm bool) (Result, error)
}

// Result 是一次写操作的执行结果。
// Done 只有在操作完成且回读验证通过时才为 true。
type Result struct {
	Done   bool
	Detail string
}

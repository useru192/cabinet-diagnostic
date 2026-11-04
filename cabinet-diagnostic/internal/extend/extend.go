// Package extend 只存放后续阶段的扩展接口定义，不包含任何实现。
//
// 对应 DESIGN 4.10 与 prompt 十三：
//   - 后续：删除文件 / 清理日志 —— 仅定义 Action 接口；
//   - 后续：日志回传 —— 仅定义 Reporter 接口。
//
// 第一阶段约束（prompt 十三、十五）：本包不被任何菜单注册，
// 不 import 任何写操作实现，除类型定义外不含任何可执行逻辑，
// 因此不会产生删除、重启、上传等任何写动作。
package extend

import (
	"context"

	"cabinet-diagnostic/internal/adb"
	"cabinet-diagnostic/internal/check"
)

// Runner 是设备命令执行者的抽象。第一阶段即 adb.Runner。
type Runner = *adb.Runner

// Report 是待回传的检测报告，内容为各组检测结果。
type Report = []check.Result

// Preview 是一次写操作的影响预览，供人工确认前查看。
// 第一阶段没有任何实现产生它。
type Preview struct {
	Path      string
	IsDir     bool
	IsSymlink bool
	Size      int64
	Owner     string
	Modified  string
}

// Result 是一次写操作的执行结果。Done 仅在操作完成并验证通过时为 true。
type Result struct {
	Done   bool
	Detail string
}

// Action 是写操作的统一接口（DESIGN 4.10）。
//
// 第一阶段只定义，不实现、不注册菜单。后续接入"清理日志"等功能时，
// 必须遵守：先 Describe 预览影响，用户显式确认后才 Apply。
type Action interface {
	// Name 返回操作名称，用于终端提示与审计记录。
	Name() string
	// Describe 只读地预览操作影响，不得产生任何写动作。
	Describe(ctx context.Context, r Runner) (Preview, error)
	// Apply 执行操作。confirm 为 false 时必须拒绝执行。
	Apply(ctx context.Context, r Runner, confirm bool) (Result, error)
}

// Reporter 是检测报告回传接口（DESIGN 4.10）。
//
// 第一阶段只定义，不实现、不上传任何数据。
type Reporter interface {
	Upload(ctx context.Context, report Report) error
}

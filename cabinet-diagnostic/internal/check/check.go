// Package check 实现各检测项、流程编排与报告生成。
package check

import (
	"fmt"
	"time"
)

// Level 是检测结论的等级，与设计文档定义的状态集合一致。
type Level string

const (
	LevelOK         Level = "OK"
	LevelWarning    Level = "WARNING"
	LevelError      Level = "ERROR"
	LevelMissing    Level = "MISSING"
	LevelRunning    Level = "RUNNING"
	LevelNotRunning Level = "NOT RUNNING"
	LevelTimeout    Level = "TIMEOUT"
	LevelUnknown    Level = "UNKNOWN"
	LevelInfo       Level = "INFO"
)

// Item 是一条判定结论。
type Item struct {
	Level   Level
	Message string
}

// Result 是一个检测项的完整结果。
//
// Cmd 记录执行的原始命令，Raw 原样保留命令输出。
// 报告生成时直接引用 Raw，任何检测项都不得对其二次加工。
type Result struct {
	Name       string // 检测项名称，如 "DISK CHECK"
	Cmd        string // 执行的原始命令，写入报告
	Raw        string // 原始输出，写入报告
	Items      []Item
	Skipped    bool
	SkipReason string
	Duration   time.Duration
}

// add 追加一条判定结论。
func (r *Result) add(level Level, format string, args ...any) {
	r.Items = append(r.Items, Item{Level: level, Message: fmt.Sprintf(format, args...)})
}

// HasLevel 判断结果中是否包含某个等级。
func (r Result) HasLevel(level Level) bool {
	for _, it := range r.Items {
		if it.Level == level {
			return true
		}
	}
	return false
}

// levelOrder 是汇总时使用的严重程度顺序，越靠前越严重。
//
// INFO 排在最后：它代表"补充说明"而非判定结论。
// 例如磁盘检测里既有 [OK] 分区又有 [INFO] 统计说明，
// 整体应汇总为 OK，而不是被 INFO 拉低。
// 只有全部条目都是 INFO 的检测项（如未设阈值的内存检测）才汇总为 INFO。
var levelOrder = []Level{
	LevelError,
	LevelMissing,
	LevelNotRunning,
	LevelTimeout,
	LevelWarning,
	LevelUnknown,
	LevelOK,
	LevelRunning,
	LevelInfo,
}

// WorstLevel 返回结果中最严重的等级，用于汇总。
// RUNNING 等同于 OK，汇总时不作为独立等级出现。
func (r Result) WorstLevel() Level {
	if r.Skipped {
		return LevelInfo
	}
	for _, l := range levelOrder {
		if r.HasLevel(l) {
			if l == LevelRunning {
				return LevelOK
			}
			return l
		}
	}
	return LevelOK
}



// Package parsers 把设备端命令的原始输出转换成结构化数据。
//
// 本包只负责"拆字段"，不做任何判定（哪个分区该告警由上层决定），
// 不执行任何命令，所有函数都是纯函数，便于用真机输出做单元测试。
package parsers

import (
	"strconv"
	"strings"
)

// Partition 是 df 输出中的一个挂载点。
type Partition struct {
	Filesystem string // 设备名，如 /dev/block/dm-6、overlay、tmpfs
	Size       string // 原始可读容量，如 "10G"
	Used       string // 原始已用容量
	Avail      string // 原始剩余容量
	UsePercent int    // 使用率百分比（0-100）
	MountedOn  string // 挂载点，如 /data

	// SizeBytes / UsedBytes 由可读容量换算而来，
	// 用于判断两个挂载点是否为同一块盘（真机上 /data 与 /storage/emulated 数值完全相同）。
	SizeBytes int64
	UsedBytes int64
}

// ParseDf 解析 df -h 的输出。
//
// 兼容 toybox 的实现差异：长设备名可能导致行首出现空行或字段折行，
// 因此按"从右往左"定位字段，而不是固定列位置：
//
//	最后一列   = 挂载点
//	倒数第二列 = 使用率
//	倒数第三列 = 剩余
//	倒数第四列 = 已用
//	倒数第五列 = 容量
//	其余       = 设备名
//
// 返回无法解析的行（不含表头与空行），供上层标记 [UNKNOWN]。
func ParseDf(stdout string) (partitions []Partition, unparsed []string) {
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}

		f := strings.Fields(line)
		if isDfHeader(f) {
			continue
		}
		if len(f) < 6 {
			unparsed = append(unparsed, strings.TrimSpace(line))
			continue
		}

		n := len(f)
		pct, ok := parsePercent(f[n-2])
		if !ok {
			unparsed = append(unparsed, strings.TrimSpace(line))
			continue
		}

		sizeBytes, ok1 := parseHumanSize(f[n-5])
		usedBytes, ok2 := parseHumanSize(f[n-4])
		if !ok1 || !ok2 {
			unparsed = append(unparsed, strings.TrimSpace(line))
			continue
		}

		partitions = append(partitions, Partition{
			Filesystem: strings.Join(f[:n-5], " "),
			Size:       f[n-5],
			Used:       f[n-4],
			Avail:      f[n-3],
			UsePercent: pct,
			MountedOn:  f[n-1],
			SizeBytes:  sizeBytes,
			UsedBytes:  usedBytes,
		})
	}
	return partitions, unparsed
}

func isDfHeader(f []string) bool {
	return len(f) >= 2 && f[0] == "Filesystem" && f[len(f)-1] == "on"
}

// parsePercent 解析 "4%" / "100%" 这类字段。
func parsePercent(s string) (int, bool) {
	s = strings.TrimSuffix(strings.TrimSpace(s), "%")
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 || v > 100 {
		return 0, false
	}
	return v, true
}

// parseHumanSize 把 "979M" / "1.2G" / "0" / "824K" 换算为字节数。
func parseHumanSize(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}

	mult := int64(1)
	num := s
	switch s[len(s)-1] {
	case 'K', 'k':
		mult, num = 1<<10, s[:len(s)-1]
	case 'M', 'm':
		mult, num = 1<<20, s[:len(s)-1]
	case 'G', 'g':
		mult, num = 1<<30, s[:len(s)-1]
	}

	v, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, false
	}
	return int64(v * float64(mult)), true
}

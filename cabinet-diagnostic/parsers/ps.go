package parsers

import (
	"strconv"
	"strings"
)

// Process 是 ps -ef 输出的一行。
//
// 表头固定为 8 列：
//
//	UID PID PPID C STIME TTY TIME CMD
//
// 第 8 列起是完整命令行（含参数），因此 CMD 列可能包含空格。
type Process struct {
	UID   string
	PID   int
	PPID  int
	C     string // 表头中的 C 列（CPU 占用）
	STIME string // 启动时间
	TTY   string
	Time  string // 累计 CPU 时间
	Cmd   string // 完整命令行
	Args  []string
}

// ParsePs 解析 ps -ef 的输出。
//
// 不使用 grep 过滤，由调用方在本机做匹配，避免 sh -c / grep 自身污染进程列表。
// 返回无法解析的行（不含表头与空行）。
func ParsePs(stdout string) (procs []Process, unparsed []string) {
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}

		f := strings.Fields(line)
		if isPsHeader(f) {
			continue
		}
		if len(f) < 8 {
			unparsed = append(unparsed, strings.TrimSpace(line))
			continue
		}

		pid, err1 := strconv.Atoi(f[1])
		ppid, err2 := strconv.Atoi(f[2])
		if err1 != nil || err2 != nil {
			unparsed = append(unparsed, strings.TrimSpace(line))
			continue
		}

		args := make([]string, len(f)-7)
		copy(args, f[7:])

		procs = append(procs, Process{
			UID:   f[0],
			PID:   pid,
			PPID:  ppid,
			C:     f[3],
			STIME: f[4],
			TTY:   f[5],
			Time:  f[6],
			Cmd:   strings.Join(args, " "),
			Args:  args,
		})
	}
	return procs, unparsed
}

func isPsHeader(f []string) bool {
	return len(f) >= 8 && f[0] == "UID" && f[len(f)-1] == "CMD"
}

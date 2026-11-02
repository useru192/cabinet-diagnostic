package check

import (
	"context"
	"errors"
	"strings"
	"time"

	"cabinet-diagnostic/internal/adb"
	"cabinet-diagnostic/internal/config"
	"cabinet-diagnostic/parsers"
)

// checkProcesses 检查 11 项关键进程与守护脚本是否运行。
//
// 设备端不使用 grep：ps -ef 全量返回后在本机过滤，
// 避免 sh -c / grep 自身污染进程列表。
func checkProcesses(ctx context.Context, r *adb.Runner) (res Result) {
	res.Name = "PROCESS CHECK"
	start := time.Now()
	defer func() { res.Duration = time.Since(start) }()

	res.Cmd = "adb shell ps -ef"
	out, err := r.Shell(ctx, config.TimeoutCommon, "ps -ef")
	res.Raw = out.Combined()

	if err != nil {
		if errors.Is(err, adb.ErrTimeout) {
			res.add(LevelTimeout, "Device did not respond to 'ps -ef' within %s", config.TimeoutCommon)
		} else {
			res.add(LevelError, "Failed to run 'ps -ef': %v", err)
		}
		return
	}

	procs, unparsed := parsers.ParsePs(out.Stdout)
	if len(procs) == 0 {
		res.add(LevelUnknown, "No process could be parsed from ps output")
		return
	}

	running := runningSet(procs)

	var missing int
	for _, name := range config.RequiredProcesses {
		if running[name] {
			res.add(LevelRunning, "%-42s", name)
			continue
		}
		missing++
		res.add(LevelNotRunning, "%s", name)
	}

	res.add(LevelInfo, "%d of %d required processes running",
		len(config.RequiredProcesses)-missing, len(config.RequiredProcesses))

	for _, line := range unparsed {
		res.add(LevelUnknown, "Unparsed ps line: %s", line)
	}
	return
}

// runningSet 把进程列表归一化为可比对的名字集合。
func runningSet(procs []parsers.Process) map[string]bool {
	set := make(map[string]bool, len(procs))
	for _, p := range procs {
		if key, ok := processKey(p.Args); ok {
			set[key] = true
		}
	}
	return set
}

// processKey 把一条命令行归一化为用于比对的名字。
//
//	sh ./restart.sh cabinetcontrolgo  ->  "sh ./restart.sh cabinetcontrolgo"
//	sh ./startup.sh                   ->  "sh ./startup.sh"
//	cabinetcontrolgo                  ->  "cabinetcontrolgo"
//	cabinetapp.wasion:jse 57 61 ...   ->  "cabinetapp.wasion:jse"（不会误判为主进程）
//	sh -c ps -ef | grep ...           ->  "sh"（不会误判为脚本进程）
//
// 首字段整串精确比较，因此 :jse 这类子进程和 sh -c / grep 自身都会被自然排除。
func processKey(args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	// shell 脚本：归一化为 "sh <脚本> [参数]"，同时覆盖
	// "sh ./restart.sh cabinetcontrolgo" 与 "sh ./startup.sh" 两种形态。
	if args[0] == "sh" && len(args) >= 2 && strings.HasSuffix(args[1], ".sh") {
		key := "sh " + args[1]
		if len(args) >= 3 {
			key += " " + args[2]
		}
		return key, true
	}
	return args[0], true
}

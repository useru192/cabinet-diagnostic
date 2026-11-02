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

// checkFiles 检查业务目录下关键文件与目录是否齐全。
//
// 只检查存在性，不检查文件内容。单项缺失不会中断其余项的检查。
func checkFiles(ctx context.Context, r *adb.Runner) (res Result) {
	res.Name = "FILE CHECK"
	start := time.Now()
	defer func() { res.Duration = time.Since(start) }()

	res.Cmd = "adb shell ls -la " + config.CabinetDir
	out, err := r.Shell(ctx, config.TimeoutCommon, "ls -la "+config.CabinetDir)
	res.Raw = out.Combined()

	if err != nil {
		if errors.Is(err, adb.ErrTimeout) {
			res.add(LevelTimeout, "Device did not respond to 'ls -la %s' within %s",
				config.CabinetDir, config.TimeoutCommon)
		} else {
			res.add(LevelError, "Failed to run 'ls -la %s': %v", config.CabinetDir, err)
		}
		return
	}

	if out.ExitCode != 0 {
		if strings.Contains(strings.ToLower(out.Stderr), "no such file") {
			res.add(LevelError, "Directory not found: %s", config.CabinetDir)
		} else {
			res.add(LevelError, "Cannot list %s (exit code %d)", config.CabinetDir, out.ExitCode)
		}
		return
	}

	entries, unparsed := parsers.ParseLsLong(out.Stdout)
	if len(entries) == 0 {
		res.add(LevelUnknown, "No entry parsed from 'ls -la %s'", config.CabinetDir)
		return
	}

	found, missing := diffRequired(entries, config.RequiredFiles)

	for _, name := range config.RequiredFiles {
		e, ok := found[name]
		if !ok {
			continue // 缺失项在下面按清单顺序单独输出
		}
		res.add(LevelOK, "%-42s (%s)", config.CabinetDir+"/"+name, e.Type)
	}
	for _, name := range missing {
		res.add(LevelMissing, "%s/%s", config.CabinetDir, name)
	}

	res.add(LevelInfo, "%d of %d required items present",
		len(config.RequiredFiles)-len(missing), len(config.RequiredFiles))

	for _, line := range unparsed {
		res.add(LevelUnknown, "Unparsed ls line: %s", line)
	}
	return
}

// diffRequired 比对目录项与必需清单。
//
// 名称按大小写精确匹配：真机上 CabinetControlgo（目录）与
// cabinetcontrolgo（文件）是两个不同条目，不能互相替代。
func diffRequired(entries []parsers.FileEntry, required []string) (found map[string]parsers.FileEntry, missing []string) {
	found = make(map[string]parsers.FileEntry, len(entries))
	for _, e := range entries {
		found[e.Name] = e
	}
	for _, name := range required {
		if _, ok := found[name]; !ok {
			missing = append(missing, name)
		}
	}
	return found, missing
}

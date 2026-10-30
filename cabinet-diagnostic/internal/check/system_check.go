package check

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"cabinet-diagnostic/internal/adb"
	"cabinet-diagnostic/internal/config"
	"cabinet-diagnostic/parsers"
)

// ---------------------------------------------------------------
// 系统响应
// ---------------------------------------------------------------

// checkSystemResponse 确认设备能正常执行命令，并给出 root 状态。
func checkSystemResponse(ctx context.Context, r *adb.Runner) (res Result) {
	res.Name = "SYSTEM RESPONSE"
	start := time.Now()
	defer func() { res.Duration = time.Since(start) }()

	res.Cmd = "adb shell id"
	out, err := r.Shell(ctx, config.TimeoutCommon, "id")
	res.Raw = out.Combined()

	if err != nil {
		if errors.Is(err, adb.ErrTimeout) {
			res.add(LevelTimeout, "Device did not respond to 'id' within %s", config.TimeoutCommon)
		} else {
			res.add(LevelError, "Failed to run 'id': %v", err)
		}
		return
	}
	if out.ExitCode != 0 {
		res.add(LevelError, "'id' exited with code %d", out.ExitCode)
		return
	}

	mode := "non-root"
	if strings.Contains(out.Stdout, "uid=0") {
		mode = "root"
	}
	res.add(LevelOK, "Device responsive (%s, %s)", mode, firstToken(out.Stdout))
	return
}

// ---------------------------------------------------------------
// 磁盘
// ---------------------------------------------------------------

type mountClass int

const (
	mountWritable mountClass = iota
	mountReadOnly
	mountVirtual
	mountDuplicate
	mountOther
)

// checkDisk 检测各分区使用率。
//
// 只对可写数据分区告警：真机上 "/" 恒为 100%（只读系统分区），
// 若统一按阈值判断会对每次运行产生误报。
func checkDisk(ctx context.Context, r *adb.Runner) (res Result) {
	res.Name = "DISK CHECK"
	start := time.Now()
	defer func() { res.Duration = time.Since(start) }()

	res.Cmd = "adb shell df -h"
	out, err := r.Shell(ctx, config.TimeoutCommon, "df -h")
	res.Raw = out.Combined()

	if err != nil {
		if errors.Is(err, adb.ErrTimeout) {
			res.add(LevelTimeout, "Device did not respond to 'df -h' within %s", config.TimeoutCommon)
		} else {
			res.add(LevelError, "Failed to run 'df -h': %v", err)
		}
		return
	}

	partitions, unparsed := parsers.ParseDf(out.Stdout)
	if len(partitions) == 0 {
		res.add(LevelUnknown, "No partition could be parsed from df output")
		return
	}

	var readOnly, virtual int
	for _, p := range partitions {
		switch classifyMount(p) {
		case mountWritable:
			msg := formatPartition(p)
			if p.UsePercent >= config.DiskWarnPercent {
				res.add(LevelWarning, "%s", msg)
			} else {
				res.add(LevelOK, "%s", msg)
			}
		case mountReadOnly:
			readOnly++
		case mountVirtual:
			virtual++
		case mountDuplicate:
			res.add(LevelInfo, "Duplicate mount skipped: %s (same disk as a parent mount)", p.MountedOn)
		default:
			res.add(LevelUnknown, "Unclassified mount: %s (%d%%)", p.MountedOn, p.UsePercent)
		}
	}

	if readOnly > 0 {
		res.add(LevelInfo, "Read-only system mounts not counted: %d", readOnly)
	}
	if virtual > 0 {
		res.add(LevelInfo, "Virtual memory mounts skipped: %d", virtual)
	}
	for _, line := range unparsed {
		res.add(LevelUnknown, "Unparsed df line: %s", line)
	}
	return
}

func classifyMount(p parsers.Partition) mountClass {
	switch {
	case containsStr(config.WritableMounts, p.MountedOn):
		return mountWritable
	case containsStr(config.ReadOnlyMounts, p.MountedOn):
		return mountReadOnly
	case containsStr(config.DedupMounts, p.MountedOn):
		return mountDuplicate
	case containsStr(config.VirtualMounts, p.MountedOn) || p.Filesystem == "tmpfs":
		return mountVirtual
	default:
		return mountOther
	}
}

func formatPartition(p parsers.Partition) string {
	return fmt.Sprintf("%-14s %3d%%  %6s total / %6s used / %6s avail",
		p.MountedOn, p.UsePercent, p.Size, p.Used, p.Avail)
}

// ---------------------------------------------------------------
// 内存
// ---------------------------------------------------------------

var reMemAvailable = regexp.MustCompile(`MemAvailable:\s+(\d+)`)

// checkMemory 打印内存使用情况。
//
// 第一阶段不做阈值判定：free 的 used/free 受 buffer/cache 影响会严重误导
// （真机上 free 显示 48 MB，实际 MemAvailable 有 847 MB），
// 且不同批次主控内存容量不同，没有基线数据时设阈值必然误报。
// 阈值一旦在 config.MemWarnAvailablePercent 中设定，此函数自动生效。
func checkMemory(ctx context.Context, r *adb.Runner) (res Result) {
	res.Name = "MEMORY CHECK"
	start := time.Now()
	defer func() { res.Duration = time.Since(start) }()

	res.Cmd = "adb shell free -m; head -30 /proc/meminfo"
	freeOut, err := r.Shell(ctx, config.TimeoutCommon, "free -m")
	res.Raw = freeOut.Combined()

	if err != nil {
		if errors.Is(err, adb.ErrTimeout) {
			res.add(LevelTimeout, "Device did not respond to 'free -m' within %s", config.TimeoutCommon)
		} else {
			res.add(LevelError, "Failed to run 'free -m': %v", err)
		}
		return
	}

	if miOut, miErr := r.Shell(ctx, config.TimeoutCommon, "head -30 /proc/meminfo"); miErr == nil {
		if s := strings.TrimSpace(miOut.Stdout); s != "" {
			res.Raw += "\n--- /proc/meminfo ---\n" + s
		}
	}

	st, ok := parseFreeMem(freeOut.Stdout)
	if !ok {
		res.add(LevelUnknown, "Cannot parse 'free -m' output")
		return
	}
	if st.total <= 0 {
		res.add(LevelUnknown, "Invalid total memory value: %d MB", st.total)
		return
	}

	// 优先用 /proc/meminfo 的 MemAvailable：它排除了可回收的 cache，
	// 比 free 的 free 列更能反映真实可用内存。
	availMB, source := st.avail, "free"
	if mb, okMB := parseMemAvailable(res.Raw); okMB {
		availMB, source = mb, "/proc/meminfo"
	}

	pct := availMB * 100 / st.total
	res.add(LevelInfo, "Available memory %d MB (%d%% of total, from %s)", availMB, pct, source)

	if config.MemWarnAvailablePercent != nil && pct < *config.MemWarnAvailablePercent {
		res.add(LevelWarning, "Available memory %d%% is below the configured threshold %d%%",
			pct, *config.MemWarnAvailablePercent)
	}
	return
}

// memStat 是 free -m 的解析结果，单位 MB。
//
// 只保留 total 与 avail：used / free 受 buffer/cache 影响会严重误导，
// 真实可用内存以 avail 为准，并优先取 /proc/meminfo 的 MemAvailable。
type memStat struct {
	total int
	avail int
}

// parseFreeMem 兼容两种 free 输出格式。
//
// 新格式（procps-ng）：
//
//	              total  used  free  shared  buff/cache  available
//	Mem:           1958  1910    47      33         549        700
//
// 旧格式（本机 Android 11 实测）：
//
//	                total  used  free  shared  buffers
//	Mem:             1958  1910    47      33        4
//	-/+ buffers/cache:            1906    51
func parseFreeMem(stdout string) (memStat, bool) {
	var st memStat
	found := false

	for _, line := range strings.Split(stdout, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}

		switch f[0] {
		case "Mem:":
			st.total, _ = strconv.Atoi(f[1])
			if len(f) >= 7 {
				st.avail, _ = strconv.Atoi(f[6])
			}
			found = true
		case "-/+":
			// "-/+ buffers/cache:  <used>  <free>"
			if len(f) >= 4 {
				st.avail, _ = strconv.Atoi(f[3])
			}
		}
	}
	return st, found
}

func parseMemAvailable(meminfo string) (int, bool) {
	m := reMemAvailable.FindStringSubmatch(meminfo)
	if m == nil {
		return 0, false
	}
	kb, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return kb / 1024, true
}

// ---------------------------------------------------------------
// 大文件
// ---------------------------------------------------------------

type fileSize struct {
	path string
	size int64
}

// checkLargeFiles 在存在磁盘告警时统计业务目录下占用最大的文件。
//
// 不使用 du -ak：目录累计值会排在最前，看不到真正的大文件。
func checkLargeFiles(ctx context.Context, r *adb.Runner, triggered bool) (res Result) {
	res.Name = "LARGE FILE ANALYSIS"
	start := time.Now()
	defer func() { res.Duration = time.Since(start) }()

	if !triggered {
		res.Skipped = true
		res.SkipReason = fmt.Sprintf("no writable partition reached %d%%", config.DiskWarnPercent)
		return
	}

	res.Cmd = "adb shell ls -lR " + config.CabinetDir
	out, err := r.Shell(ctx, config.TimeoutScan, "ls -lR "+config.CabinetDir)
	res.Raw = out.Combined()

	if err != nil {
		if errors.Is(err, adb.ErrTimeout) {
			res.add(LevelTimeout, "Scanning %s timed out after %s", config.CabinetDir, config.TimeoutScan)
		} else {
			res.add(LevelError, "Failed to scan %s: %v", config.CabinetDir, err)
		}
		return
	}
	if out.ExitCode != 0 && strings.Contains(strings.ToLower(out.Stderr), "permission denied") {
		res.add(LevelUnknown, "Permission denied while scanning %s", config.CabinetDir)
		return
	}

	files := collectFiles(out.Stdout)
	if len(files) == 0 {
		res.add(LevelUnknown, "No regular file found under %s", config.CabinetDir)
		return
	}

	sort.Slice(files, func(i, j int) bool { return files[i].size > files[j].size })

	n := len(files)
	if n > config.TopFilesLimit {
		n = config.TopFilesLimit
	}
	for i := 0; i < n; i++ {
		res.add(LevelInfo, "%10s  %s", humanSize(files[i].size), files[i].path)
	}
	return
}

// collectFiles 从 ls -lR 的输出中收集所有普通文件及其完整路径。
// ls -lR 每进入一个目录都会输出一行 "/path:" 作为分隔。
func collectFiles(listing string) []fileSize {
	var files []fileSize
	curDir := ""

	for _, raw := range strings.Split(listing, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasSuffix(trimmed, ":") && len(strings.Fields(trimmed)) == 1 {
			curDir = strings.TrimSuffix(trimmed, ":")
			continue
		}

		entries, _ := parsers.ParseLsLong(line)
		for _, e := range entries {
			if e.Type != parsers.EntryFile {
				continue
			}
			if e.Name == "." || e.Name == ".." {
				continue
			}
			files = append(files, fileSize{path: curDir + "/" + e.Name, size: e.Size})
		}
	}
	return files
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	val := float64(n)
	units := []string{"K", "M", "G", "T"}
	for _, u := range units {
		val /= unit
		if val < unit {
			return fmt.Sprintf("%.1f %s", val, u)
		}
	}
	return fmt.Sprintf("%.1f P", val/unit)
}

// ---------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func firstToken(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

func orUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "[UNKNOWN]"
	}
	return s
}

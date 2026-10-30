package check

import (
	"testing"

	"cabinet-diagnostic/parsers"
)

// 真机 Android 11 的 free -m 输出（旧格式，无 buff/cache 与 available 列）。
const freeOldFormat = "\t\ttotal        used        free      shared     buffers\n" +
	"Mem:             1958        1910          47          33           4\n" +
	"-/+ buffers/cache:           1906          51\n" +
	"Swap:             979           0         979\n"

// procps-ng 的新格式。
const freeNewFormat = "               total        used        free      shared  buff/cache   available\n" +
	"Mem:           1958        1910          47          33         549          700\n" +
	"Swap:          1023           0        1023\n"

func TestParseFreeMemOldFormat(t *testing.T) {
	st, ok := parseFreeMem(freeOldFormat)
	if !ok {
		t.Fatal("parseFreeMem returned false for old format")
	}
	if st.total != 1958 {
		t.Errorf("total = %d, want 1958", st.total)
	}
	// 旧格式没有 available 列，应取 "-/+ buffers/cache" 的 free
	if st.avail != 51 {
		t.Errorf("avail = %d, want 51 (from buffers/cache line)", st.avail)
	}
}

func TestParseFreeMemNewFormat(t *testing.T) {
	st, ok := parseFreeMem(freeNewFormat)
	if !ok {
		t.Fatal("parseFreeMem returned false for new format")
	}
	if st.total != 1958 || st.avail != 700 {
		t.Errorf("unexpected values: %+v", st)
	}
}

func TestParseFreeMemGarbage(t *testing.T) {
	if _, ok := parseFreeMem("no memory info here\n"); ok {
		t.Error("garbage input should not be parsed")
	}
}

func TestParseMemAvailable(t *testing.T) {
	meminfo := "MemTotal:        2005364 kB\n" +
		"MemFree:           50112 kB\n" +
		"MemAvailable:     868352 kB\n"

	mb, ok := parseMemAvailable(meminfo)
	if !ok {
		t.Fatal("parseMemAvailable returned false")
	}
	if mb != 848 {
		t.Errorf("mb = %d, want 848", mb)
	}

	if _, ok := parseMemAvailable("MemFree: 100 kB\n"); ok {
		t.Error("missing MemAvailable should return false")
	}
}

func TestWorstLevel(t *testing.T) {
	cases := []struct {
		name string
		res  Result
		want Level
	}{
		{
			name: "all ok with info notes",
			res: Result{Items: []Item{
				{Level: LevelOK, Message: "/data 7%"},
				{Level: LevelInfo, Message: "7 read-only mounts not counted"},
			}},
			want: LevelOK,
		},
		{
			name: "warning beats ok",
			res: Result{Items: []Item{
				{Level: LevelOK, Message: "/cache 1%"},
				{Level: LevelWarning, Message: "/data 92%"},
			}},
			want: LevelWarning,
		},
		{
			name: "info only",
			res:  Result{Items: []Item{{Level: LevelInfo, Message: "available 848 MB"}}},
			want: LevelInfo,
		},
		{
			name: "error beats everything",
			res: Result{Items: []Item{
				{Level: LevelWarning, Message: "warn"},
				{Level: LevelError, Message: "boom"},
			}},
			want: LevelError,
		},
		{
			name: "skipped",
			res:  Result{Skipped: true},
			want: LevelInfo,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.res.WorstLevel(); got != c.want {
				t.Errorf("WorstLevel() = %s, want %s", got, c.want)
			}
		})
	}
}

func TestClassifyMount(t *testing.T) {
	cases := []struct {
		mountedOn  string
		filesystem string
		want       mountClass
	}{
		{"/data", "/dev/block/dm-6", mountWritable},
		{"/cache", "/dev/block/mmcblk0p10", mountWritable},
		{"/mnt/scratch", "/dev/block/dm-5", mountWritable},
		{"/", "/dev/block/dm-0", mountReadOnly},
		{"/system", "overlay", mountReadOnly},
		{"/metadata", "/dev/block/mmcblk0p11", mountReadOnly},
		{"/storage/emulated", "/dev/fuse", mountDuplicate},
		{"/apex", "tmpfs", mountVirtual},
		{"/dev", "tmpfs", mountVirtual},
		{"/vendor/lib", "/dev/block/dm-9", mountOther},
	}

	for _, c := range cases {
		p := parsers.Partition{MountedOn: c.mountedOn, Filesystem: c.filesystem}
		if got := classifyMount(p); got != c.want {
			t.Errorf("classifyMount(%s, %s) = %v, want %v", c.mountedOn, c.filesystem, got, c.want)
		}
	}
}

func TestCollectFiles(t *testing.T) {
	listing := "/data/cabinet:\n" +
		"total 89777\n" +
		"drwxrwxrwx 17 shell shell 3452 2024-04-19 12:35 CabinetControlgo\n" +
		"-rwxrwxrwx  1 root  root  20110464 2024-05-17 11:37 cabinet.apk\n" +
		"\n" +
		"/data/cabinet/upgrade:\n" +
		"total 8\n" +
		"-rwxrwxrwx  1 root  root    102400 2024-07-16 09:27 ota.bin\n"

	files := collectFiles(listing)
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %+v", len(files), files)
	}

	want := map[string]int64{
		"/data/cabinet/cabinet.apk":     20110464,
		"/data/cabinet/upgrade/ota.bin": 102400,
	}
	for _, f := range files {
		size, ok := want[f.path]
		if !ok {
			t.Errorf("unexpected path %q", f.path)
			continue
		}
		if f.size != size {
			t.Errorf("%s size = %d, want %d", f.path, f.size, size)
		}
	}
}

func TestHumanSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{2048, "2.0 K"},
		{20110464, "19.2 M"},
		{10 << 30, "10.0 G"},
	}

	for _, c := range cases {
		if got := humanSize(c.in); got != c.want {
			t.Errorf("humanSize(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

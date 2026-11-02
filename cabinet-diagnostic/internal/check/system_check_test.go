package check

import (
	"testing"

	"cabinet-diagnostic/parsers"
)

// 真机 Android 11 的 /proc/meminfo 节选。
const meminfoSample = "MemTotal:        2005076 kB\n" +
	"MemFree:           47944 kB\n" +
	"Buffers:            4232 kB\n" +
	"Cached:           934964 kB\n" +
	"MemAvailable:     875404 kB\n" +
	"SwapTotal:       1002532 kB\n" +
	"SwapFree:        1002532 kB\n"

func TestMeminfoMB(t *testing.T) {
	total, ok := meminfoMB(reMemTotal, meminfoSample)
	if !ok {
		t.Fatal("MemTotal not parsed")
	}
	if total != 1958 {
		t.Errorf("total = %d, want 1958", total)
	}

	avail, ok := meminfoMB(reMemAvailable, meminfoSample)
	if !ok {
		t.Fatal("MemAvailable not parsed")
	}
	if avail != 854 {
		t.Errorf("avail = %d, want 854", avail)
	}
}

func TestMeminfoMBMissing(t *testing.T) {
	if _, ok := meminfoMB(reMemTotal, "MemFree: 100 kB\n"); ok {
		t.Error("missing MemTotal should return false")
	}
	if _, ok := meminfoMB(reMemAvailable, "MemFree: 100 kB\n"); ok {
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

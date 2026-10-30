package parsers

import (
	"os"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func findPartition(parts []Partition, mount string) *Partition {
	for i := range parts {
		if parts[i].MountedOn == mount {
			return &parts[i]
		}
	}
	return nil
}

func TestParseDfRealDevice(t *testing.T) {
	parts, unparsed := ParseDf(readFixture(t, "df_h.txt"))

	if len(unparsed) != 0 {
		t.Errorf("unexpected unparsed lines: %v", unparsed)
	}
	if len(parts) != 16 {
		t.Fatalf("got %d partitions, want 16", len(parts))
	}

	cases := []struct {
		mount string
		pct   int
	}{
		{"/", 100},
		{"/data", 4},
		{"/cache", 1},
		{"/mnt/scratch", 1},
		{"/metadata", 2},
	}
	for _, c := range cases {
		p := findPartition(parts, c.mount)
		if p == nil {
			t.Fatalf("mount %s not found", c.mount)
		}
		if p.UsePercent != c.pct {
			t.Errorf("%s use%% = %d, want %d", c.mount, p.UsePercent, c.pct)
		}
	}
}

// TestParseDfLongDeviceName 覆盖 toybox 长设备名导致的换行。
// fixture 中 /dev/block/mmcblk0p11 前面有一个空行。
func TestParseDfLongDeviceName(t *testing.T) {
	parts, _ := ParseDf(readFixture(t, "df_h.txt"))

	p := findPartition(parts, "/metadata")
	if p == nil {
		t.Fatal("/metadata not found, long device name line was probably dropped")
	}
	if p.Filesystem != "/dev/block/mmcblk0p11" {
		t.Errorf("filesystem = %q, want /dev/block/mmcblk0p11", p.Filesystem)
	}
	if p.Size != "11M" {
		t.Errorf("size = %q, want 11M", p.Size)
	}
}

// TestParseDfDuplicateDisk 验证同一块盘的两个挂载点能被识别出来。
func TestParseDfDuplicateDisk(t *testing.T) {
	parts, _ := ParseDf(readFixture(t, "df_h.txt"))

	data := findPartition(parts, "/data")
	emulated := findPartition(parts, "/storage/emulated")
	if data == nil || emulated == nil {
		t.Fatal("/data or /storage/emulated not found")
	}

	if data.SizeBytes != 10<<30 {
		t.Errorf("/data SizeBytes = %d, want %d", data.SizeBytes, int64(10<<30))
	}
	if data.SizeBytes != emulated.SizeBytes || data.UsedBytes != emulated.UsedBytes {
		t.Errorf("/data and /storage/emulated should be the same disk: %+v vs %+v", data, emulated)
	}
}

func TestParseDfUnparsableLine(t *testing.T) {
	in := "Filesystem Size Used Avail Use% Mounted on\n" +
		"garbage line here\n" +
		"/dev/block/x 1G 1M 1G 1% /ok\n"

	parts, unparsed := ParseDf(in)

	if len(parts) != 1 || parts[0].MountedOn != "/ok" {
		t.Fatalf("parts = %+v, want exactly one /ok partition", parts)
	}
	if len(unparsed) != 1 || unparsed[0] != "garbage line here" {
		t.Errorf("unparsed = %v, want the garbage line", unparsed)
	}
}

func TestParseDfEmpty(t *testing.T) {
	parts, unparsed := ParseDf("")
	if len(parts) != 0 || len(unparsed) != 0 {
		t.Errorf("empty input should produce nothing, got %+v / %v", parts, unparsed)
	}
}

func TestParseHumanSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"0", 0, true},
		{"824K", 824 << 10, true},
		{"979M", 979 << 20, true},
		{"1.2M", 1258291, true},
		{"10G", 10 << 30, true},
		{"2T", 2 << 40, true},
		{"", 0, false},
		{"abc", 0, false},
	}

	for _, c := range cases {
		got, ok := parseHumanSize(c.in)
		if ok != c.ok {
			t.Errorf("parseHumanSize(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseHumanSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParsePercent(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"4%", 4, true},
		{"100%", 100, true},
		{"0%", 0, true},
		{"abc%", 0, false},
		{"101%", 0, false},
	}

	for _, c := range cases {
		got, ok := parsePercent(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parsePercent(%q) = %d,%v want %d,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

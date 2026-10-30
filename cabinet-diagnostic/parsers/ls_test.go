package parsers

import (
	"strings"
	"testing"
)

func findEntry(entries []FileEntry, name string) *FileEntry {
	for i := range entries {
		if entries[i].Name == name {
			return &entries[i]
		}
	}
	return nil
}

func TestParseLsRealDevice(t *testing.T) {
	fixture := readFixture(t, "ls_la.txt")
	entries, unparsed := ParseLsLong(fixture)

	if len(unparsed) != 0 {
		t.Errorf("unexpected unparsed lines: %v", unparsed)
	}

	// 期望行数 = 非空且非 total 行的数量，确保没有丢行
	want := 0
	for _, ln := range strings.Split(fixture, "\n") {
		s := strings.TrimSpace(ln)
		if s == "" || strings.HasPrefix(s, "total ") {
			continue
		}
		want++
	}
	if len(entries) != want {
		t.Fatalf("got %d entries, want %d", len(entries), want)
	}
}

// TestParseLsCaseSensitive 覆盖真机上同时存在
// CabinetControlgo（目录）与 cabinetcontrolgo（文件）的情况。
func TestParseLsCaseSensitive(t *testing.T) {
	entries, _ := ParseLsLong(readFixture(t, "ls_la.txt"))

	upper := findEntry(entries, "CabinetControlgo")
	lower := findEntry(entries, "cabinetcontrolgo")

	if upper == nil {
		t.Fatal("CabinetControlgo not found")
	}
	if lower == nil {
		t.Fatal("cabinetcontrolgo not found")
	}
	if upper.Type != EntryDir {
		t.Errorf("CabinetControlgo type = %v, want directory", upper.Type)
	}
	if lower.Type != EntryFile {
		t.Errorf("cabinetcontrolgo type = %v, want file", lower.Type)
	}
}

// TestParseLsDirectoryNotMissing 覆盖 upgrade 这类目录不能被误判为缺失。
func TestParseLsDirectoryNotMissing(t *testing.T) {
	entries, _ := ParseLsLong(readFixture(t, "ls_la.txt"))

	up := findEntry(entries, "upgrade")
	if up == nil {
		t.Fatal("upgrade not found")
	}
	if up.Type != EntryDir {
		t.Errorf("upgrade type = %v, want directory", up.Type)
	}
}

func TestParseLsSizeAndMeta(t *testing.T) {
	entries, _ := ParseLsLong(readFixture(t, "ls_la.txt"))

	cases := []struct {
		name  string
		size  int64
		owner string
	}{
		{"cabinet.apk", 20110464, "root"},
		{"sqlite.db", 2093056, "root"},
		{"deviceid", 11, "root"},
		{"cabinetapp", 5923808, "shell"},
	}

	for _, c := range cases {
		e := findEntry(entries, c.name)
		if e == nil {
			t.Errorf("%s not found", c.name)
			continue
		}
		if e.Size != c.size {
			t.Errorf("%s size = %d, want %d", c.name, e.Size, c.size)
		}
		if e.Owner != c.owner {
			t.Errorf("%s owner = %q, want %q", c.name, e.Owner, c.owner)
		}
	}
}

func TestParseLsSymlink(t *testing.T) {
	in := "total 8\n" +
		"lrwxrwxrwx 1 root root 21 2024-04-19 12:03 libfoo.so -> libfoo.so.1\n"

	entries, unparsed := ParseLsLong(in)

	if len(unparsed) != 0 {
		t.Fatalf("unparsed = %v", unparsed)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Type != EntrySymlink {
		t.Errorf("type = %v, want symlink", entries[0].Type)
	}
	if entries[0].Name != "libfoo.so" {
		t.Errorf("name = %q, want libfoo.so", entries[0].Name)
	}
}

func TestParseLsEmpty(t *testing.T) {
	entries, unparsed := ParseLsLong("total 0\n")
	if len(entries) != 0 || len(unparsed) != 0 {
		t.Errorf("total only should produce nothing, got %+v / %v", entries, unparsed)
	}
}

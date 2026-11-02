package check

import (
	"testing"

	"cabinet-diagnostic/internal/config"
	"cabinet-diagnostic/parsers"
)

func TestDiffRequired(t *testing.T) {
	listing := "total 8\n" +
		"-rwxrwxrwx 1 root  root 5828344 2026-10-27 17:11 cabinetcontrolgo\n" +
		"drwxrwxrwx 17 shell shell    3452 2024-04-19 12:35 CabinetControlgo\n" +
		"drwxrwxrwx  2 root  root     3452 2024-07-16 09:27 upgrade\n"

	entries, unparsed := parsers.ParseLsLong(listing)
	if len(unparsed) != 0 {
		t.Fatalf("unparsed lines: %v", unparsed)
	}

	required := []string{"cabinetcontrolgo", "CabinetControlgo", "upgrade", "cabinetmqtt.yaml"}
	found, missing := diffRequired(entries, required)

	if len(missing) != 1 || missing[0] != "cabinetmqtt.yaml" {
		t.Errorf("missing = %v, want [cabinetmqtt.yaml]", missing)
	}
	if found["upgrade"].Type != parsers.EntryDir {
		t.Errorf("upgrade type = %v, want directory", found["upgrade"].Type)
	}
	if found["cabinetcontrolgo"].Type != parsers.EntryFile {
		t.Errorf("cabinetcontrolgo type = %v, want file", found["cabinetcontrolgo"].Type)
	}
}

// 大小写必须精确匹配，不能互相替代。
func TestDiffRequiredCaseSensitive(t *testing.T) {
	listing := "total 8\n" +
		"drwxrwxrwx 17 shell shell 3452 2024-04-19 12:35 CabinetControlgo\n"

	entries, _ := parsers.ParseLsLong(listing)
	_, missing := diffRequired(entries, []string{"CabinetControlgo", "cabinetcontrolgo"})

	if len(missing) != 1 || missing[0] != "cabinetcontrolgo" {
		t.Errorf("missing = %v, want [cabinetcontrolgo]", missing)
	}
}

func TestDiffRequiredMissingWhenNoEntries(t *testing.T) {
	_, missing := diffRequired(nil, []string{"a", "b"})
	if len(missing) != 2 {
		t.Errorf("missing = %v, want 2 items", missing)
	}
}

// 清单属于业务数据，会随设备批次调整，因此不锁死条目数量，
// 只校验结构性问题（空条目、重复）和几个必须存在的关键项。
func TestRequiredFileList(t *testing.T) {
	if len(config.RequiredFiles) == 0 {
		t.Fatal("RequiredFiles is empty")
	}

	seen := make(map[string]bool, len(config.RequiredFiles))
	for _, name := range config.RequiredFiles {
		if name == "" {
			t.Error("RequiredFiles contains an empty entry")
			continue
		}
		if seen[name] {
			t.Errorf("RequiredFiles contains duplicate entry %q", name)
		}
		seen[name] = true
	}

	for _, want := range []string{"cabinetcontrolgo", "cabinetmqtt", "deviceid", "sqlite.db", "upgrade"} {
		if !seen[want] {
			t.Errorf("RequiredFiles missing %q", want)
		}
	}
}

func TestRequiredProcessList(t *testing.T) {
	if len(config.RequiredProcesses) == 0 {
		t.Fatal("RequiredProcesses is empty")
	}

	seen := make(map[string]bool, len(config.RequiredProcesses))
	for _, name := range config.RequiredProcesses {
		if name == "" {
			t.Error("RequiredProcesses contains an empty entry")
			continue
		}
		if seen[name] {
			t.Errorf("RequiredProcesses contains duplicate entry %q", name)
		}
		seen[name] = true
	}

	for _, want := range []string{
		"sh ./restart.sh cabinetcontrolgo",
		"cabinetcontrolgo",
	} {
		if !seen[want] {
			t.Errorf("RequiredProcesses missing %q", want)
		}
	}
}

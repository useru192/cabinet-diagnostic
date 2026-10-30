package adb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDevices(t *testing.T) {
	cases := []struct {
		name   string
		stdout string
		want   []adbDevice
	}{
		{"empty output", "", nil},
		{"header only", "List of devices attached\n\n", nil},
		{
			"single ready device",
			"List of devices attached\n8925c67aefcd0126\tdevice\n\n",
			[]adbDevice{{serial: "8925c67aefcd0126", state: "device"}},
		},
		{
			"unauthorized device",
			"List of devices attached\n8925c67aefcd0126\tunauthorized\n",
			[]adbDevice{{serial: "8925c67aefcd0126", state: "unauthorized"}},
		},
		{
			"offline device",
			"List of devices attached\nabc123\toffline\n",
			[]adbDevice{{serial: "abc123", state: "offline"}},
		},
		{
			"daemon started message is ignored",
			"* daemon not running; starting now at tcp:5037\nList of devices attached\nabc\tdevice\n",
			[]adbDevice{{serial: "abc", state: "device"}},
		},
		{"line without state is ignored", "List of devices attached\nbroken-line\n", nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseDevices(c.stdout)
			if len(got) != len(c.want) {
				t.Fatalf("got %d devices, want %d: %+v", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("device[%d] = %+v, want %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestOutputCombined(t *testing.T) {
	cases := []struct {
		name string
		out  Output
		want string
	}{
		{"stdout only", Output{Stdout: "hello\n"}, "hello\n"},
		{"stdout and stderr", Output{Stdout: "hello", Stderr: "oops\n"}, "hello\noops"},
		{"stderr only", Output{Stderr: "oops\n"}, "oops"},
		{"both empty", Output{}, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.out.Combined(); got != c.want {
				t.Errorf("Combined() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestExtractEmbeddedAdb(t *testing.T) {
	p, err := extractEmbeddedAdb()
	if err != nil {
		t.Fatalf("extractEmbeddedAdb() error: %v", err)
	}
	if !fileExists(p) {
		t.Fatalf("extracted adb.exe not found at %s", p)
	}
	// adb.exe 依赖同目录的 DLL，缺一个就会在启动时报错
	dir := filepath.Dir(p)
	for _, name := range []string{"AdbWinApi.dll", "AdbWinUsbApi.dll"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing dependency %s next to adb.exe", name)
		}
	}
}

package main

import "testing"

func TestParseDevices(t *testing.T) {
	cases := []struct {
		name   string
		stdout string
		want   []adbDevice
	}{
		{
			name:   "empty output",
			stdout: "",
			want:   nil,
		},
		{
			name:   "header only",
			stdout: "List of devices attached\n\n",
			want:   nil,
		},
		{
			name:   "single ready device",
			stdout: "List of devices attached\n8925c67aefcd0126\tdevice\n\n",
			want:   []adbDevice{{serial: "8925c67aefcd0126", state: "device"}},
		},
		{
			name:   "unauthorized device",
			stdout: "List of devices attached\n8925c67aefcd0126\tunauthorized\n",
			want:   []adbDevice{{serial: "8925c67aefcd0126", state: "unauthorized"}},
		},
		{
			name:   "offline device",
			stdout: "List of devices attached\nabc123\toffline\n",
			want:   []adbDevice{{serial: "abc123", state: "offline"}},
		},
		{
			name:   "multiple devices",
			stdout: "List of devices attached\naaa\tdevice\nbbb\tdevice\n",
			want: []adbDevice{
				{serial: "aaa", state: "device"},
				{serial: "bbb", state: "device"},
			},
		},
		{
			name:   "daemon started message is ignored",
			stdout: "* daemon not running; starting now at tcp:5037\nList of devices attached\nabc\tdevice\n",
			want:   []adbDevice{{serial: "abc", state: "device"}},
		},
		{
			name:   "line without state is ignored",
			stdout: "List of devices attached\nbroken-line\n",
			want:   nil,
		},
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
		{
			name: "stdout only",
			out:  Output{Stdout: "hello\n"},
			want: "hello\n",
		},
		{
			name: "stdout and stderr",
			out:  Output{Stdout: "hello", Stderr: "oops\n"},
			want: "hello\noops",
		},
		{
			name: "stderr only",
			out:  Output{Stderr: "oops\n"},
			want: "oops",
		},
		{
			name: "both empty",
			out:  Output{},
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.out.Combined(); got != c.want {
				t.Errorf("Combined() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestEmbeddedAdbExtract(t *testing.T) {
	p, err := extractEmbeddedAdb()
	if err != nil {
		t.Fatalf("extractEmbeddedAdb() error: %v", err)
	}
	if !fileExists(p) {
		t.Fatalf("extracted adb.exe not found at %s", p)
	}
	// adb.exe 依赖同目录的 DLL，缺一个就会在启动时报错
	for _, name := range []string{"AdbWinApi.dll", "AdbWinUsbApi.dll"} {
		if !fileExists(dirOf(p) + "\\" + name) {
			t.Errorf("missing dependency %s next to adb.exe", name)
		}
	}
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '\\' || p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}

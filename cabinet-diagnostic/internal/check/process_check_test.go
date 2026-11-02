package check

import (
	"testing"

	"cabinet-diagnostic/parsers"
)

// 真机 ps -ef 节选，包含守护脚本、业务进程、子进程和 grep 自身。
const psSample = "UID             PID   PPID C STIME TTY          TIME CMD\n" +
	"root              1      0 0 12:03:05 ?     00:00:02 init second_stage\n" +
	"root           1773   1198 0 12:03:31 ?     00:00:01 sh ./restart.sh cabinetcontrolgo\n" +
	"root           1774   1198 0 12:03:31 ?     00:00:01 sh ./restart.sh cabinettcpclient\n" +
	"root           1792   1773 2 12:03:31 ?     00:01:24 cabinetcontrolgo\n" +
	"root           1794   1774 0 12:03:31 ?     00:00:03 cabinettcpclient\n" +
	"u0_a125        2023    260 8 12:03:51 ?     00:04:57 cabinetapp.wasion\n" +
	"u0_a125        2076   2023 1 12:03:52 ?     00:00:25 cabinetapp.wasion:jse 57 61 1 /data/user/0/cabinetapp.wasion/app_crash/crash_dump.log\n" +
	"shell         21497    336 0 13:03:47 ?     00:00:00 sh -c ps -ef | grep -E 'restart.sh|cabinet'\n" +
	"shell         21500  21497 1 13:03:47 ?     00:00:00 grep -E restart.sh|cabinet\n"

func TestProcessKey(t *testing.T) {
	cases := []struct {
		args []string
		want string
		ok   bool
	}{
		{[]string{"sh", "./restart.sh", "cabinetcontrolgo"}, "sh ./restart.sh cabinetcontrolgo", true},
		{[]string{"sh", "./startup.sh"}, "sh ./startup.sh", true},
		{[]string{"sh", "/data/cabinet/restart.sh", "cabinetmqtt"}, "sh /data/cabinet/restart.sh cabinetmqtt", true},
		{[]string{"cabinetcontrolgo"}, "cabinetcontrolgo", true},
		{[]string{"cabinetapp.wasion"}, "cabinetapp.wasion", true},
		{[]string{"cabinetapp.wasion:jse", "57", "61", "1"}, "cabinetapp.wasion:jse", true},
		{[]string{"sh", "-c", "ps -ef | grep cabinet"}, "sh", true},
		{[]string{"grep", "-E", "restart.sh|cabinet"}, "grep", true},
		{nil, "", false},
	}

	for _, c := range cases {
		got, ok := processKey(c.args)
		if ok != c.ok || got != c.want {
			t.Errorf("processKey(%v) = %q,%v want %q,%v", c.args, got, ok, c.want, c.ok)
		}
	}
}

func TestRunningSet(t *testing.T) {
	procs, unparsed := parsers.ParsePs(psSample)
	if len(unparsed) != 0 {
		t.Fatalf("unparsed lines: %v", unparsed)
	}

	running := runningSet(procs)

	present := []string{
		"sh ./restart.sh cabinetcontrolgo",
		"sh ./restart.sh cabinettcpclient",
		"cabinetcontrolgo",
		"cabinettcpclient",
		"cabinetapp.wasion",
	}
	for _, name := range present {
		if !running[name] {
			t.Errorf("%q should be detected as running", name)
		}
	}

	absent := []string{
		"cabinetmqtt",
		"sh ./restart.sh cabinetmqtt",
		"cabinetlocalupgrade",
	}
	for _, name := range absent {
		if running[name] {
			t.Errorf("%q should not be detected as running", name)
		}
	}
}

// 子进程不能被当成主进程，否则主进程崩了也会判为正常运行。
func TestSubProcessNotCountedAsMain(t *testing.T) {
	procs, _ := parsers.ParsePs(psSample)
	running := runningSet(procs)

	if !running["cabinetapp.wasion"] {
		t.Fatal("main cabinetapp.wasion should be running in the fixture")
	}

	// 只保留 :jse 子进程时，主进程应判定为未运行
	var onlySub []parsers.Process
	for _, p := range procs {
		if len(p.Args) > 0 && p.Args[0] == "cabinetapp.wasion:jse" {
			onlySub = append(onlySub, p)
		}
	}
	if runningSet(onlySub)["cabinetapp.wasion"] {
		t.Error("cabinetapp.wasion should not be reported running when only the :jse child exists")
	}
}

// 守护脚本在、业务进程不在，两个条目应给出不同结论。
func TestGuardRunningButBusinessMissing(t *testing.T) {
	procs, _ := parsers.ParsePs(psSample)
	running := runningSet(procs)

	if running["sh ./restart.sh cabinetmqtt"] {
		t.Error("fixture has no cabinetmqtt guard process")
	}
	if running["cabinetmqtt"] {
		t.Error("fixture has no cabinetmqtt process")
	}

	// 反例：守护存在时结论应独立
	if !running["sh ./restart.sh cabinetcontrolgo"] || !running["cabinetcontrolgo"] {
		t.Error("cabinetcontrolgo guard and business process should both be running")
	}
}

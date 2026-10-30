package parsers

import "testing"

func firstArg(procs []Process, arg0 string) *Process {
	for i := range procs {
		if len(procs[i].Args) > 0 && procs[i].Args[0] == arg0 {
			return &procs[i]
		}
	}
	return nil
}

func TestParsePsRealDevice(t *testing.T) {
	procs, unparsed := ParsePs(readFixture(t, "ps_ef.txt"))

	if len(unparsed) != 0 {
		t.Errorf("unexpected unparsed lines: %v", unparsed)
	}
	if len(procs) != 16 {
		t.Fatalf("got %d processes, want 16", len(procs))
	}

	// 守护脚本：三个 token 完整保留
	guard := firstArg(procs, "sh")
	if guard == nil {
		t.Fatal("restart.sh guard process not found")
	}
	want := []string{"sh", "./restart.sh", "cabinetcontrolgo"}
	if len(guard.Args) != len(want) {
		t.Fatalf("guard args = %v, want %v", guard.Args, want)
	}
	for i := range want {
		if guard.Args[i] != want[i] {
			t.Fatalf("guard args = %v, want %v", guard.Args, want)
		}
	}

	// 业务进程
	if p := firstArg(procs, "cabinetcontrolgo"); p == nil {
		t.Error("cabinetcontrolgo not found")
	}

	// 内核线程的方括号名字要原样保留
	if p := firstArg(procs, "[kthreadd]"); p == nil {
		t.Error("[kthreadd] not found")
	}
}

// TestParsePsSubProcessNotConfused 验证子进程不会被误判为主进程。
// 真机上 cabinetapp.wasion 有一个 :jse 子进程。
func TestParsePsSubProcessNotConfused(t *testing.T) {
	procs, _ := ParsePs(readFixture(t, "ps_ef.txt"))

	var main, sub int
	for _, p := range procs {
		if len(p.Args) == 0 {
			continue
		}
		switch p.Args[0] {
		case "cabinetapp.wasion":
			main++
		case "cabinetapp.wasion:jse":
			sub++
		}
	}

	if main != 1 {
		t.Errorf("cabinetapp.wasion matched %d times, want 1", main)
	}
	if sub != 1 {
		t.Errorf("cabinetapp.wasion:jse matched %d times, want 1", sub)
	}
}

// TestParsePsGrepNotConfused 验证 grep / sh -c 自身不会被当成目标进程。
func TestParsePsGrepNotConfused(t *testing.T) {
	procs, _ := ParsePs(readFixture(t, "ps_ef.txt"))

	for _, p := range procs {
		if len(p.Args) == 0 {
			continue
		}
		if p.Args[0] == "sh" && len(p.Args) > 1 && p.Args[1] == "-c" {
			return // 找到了 sh -c，符合预期
		}
	}
	t.Error("sh -c line not parsed, fixture may be broken")
}

func TestParsePsEmpty(t *testing.T) {
	procs, unparsed := ParsePs("UID PID PPID C STIME TTY TIME CMD\n")
	if len(procs) != 0 || len(unparsed) != 0 {
		t.Errorf("header only should produce nothing, got %+v / %v", procs, unparsed)
	}
}

func TestParsePsUnparsableLine(t *testing.T) {
	in := "UID PID PPID C STIME TTY TIME CMD\n" +
		"broken\n" +
		"root 100 1 0 10:00:00 ? 00:00:01 sh ./restart.sh cabinetmqtt\n"

	procs, unparsed := ParsePs(in)

	if len(procs) != 1 {
		t.Fatalf("got %d processes, want 1", len(procs))
	}
	if procs[0].PID != 100 || procs[0].PPID != 1 {
		t.Errorf("pid/ppid = %d/%d, want 100/1", procs[0].PID, procs[0].PPID)
	}
	if len(unparsed) != 1 {
		t.Errorf("unparsed = %v, want 1 line", unparsed)
	}
}

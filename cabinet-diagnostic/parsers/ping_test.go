package parsers

import "testing"

func TestParsePingDNSFailed(t *testing.T) {
	// 真机当前状态：ping: unknown host www.baidu.com，exit 2
	res := ParsePing(readFixture(t, "ping_fail.txt"), "", 2)

	if !res.DNSFailed {
		t.Error("DNSFailed = false, want true")
	}
	if res.Resolved {
		t.Error("Resolved = true, want false")
	}
	if res.Success {
		t.Error("Success = true, want false")
	}
	if res.Raw == "" {
		t.Error("Raw should keep the original output for the report")
	}
}

// dns 错误可能出现在 stderr 而不是 stdout。
func TestParsePingDNSFailedOnStderr(t *testing.T) {
	res := ParsePing("", "ping: unknown host www.baidu.com\n", 2)

	if !res.DNSFailed || res.Success {
		t.Errorf("DNSFailed = %v, Success = %v; want true, false", res.DNSFailed, res.Success)
	}
}

func TestParsePingSuccess(t *testing.T) {
	res := ParsePing(readFixture(t, "ping_ok.txt"), "", 0)

	if res.DNSFailed {
		t.Error("DNSFailed = true, want false")
	}
	if !res.Resolved {
		t.Error("Resolved = false, want true")
	}
	if !res.Success {
		t.Error("Success = false, want true")
	}
	if !res.StatsFound {
		t.Error("StatsFound = false, want true")
	}
	if res.Transmitted != 4 || res.Received != 4 {
		t.Errorf("transmitted/received = %d/%d, want 4/4", res.Transmitted, res.Received)
	}
	if res.LossPercent != 0 {
		t.Errorf("loss = %d, want 0", res.LossPercent)
	}
}

func TestParsePingAllLost(t *testing.T) {
	res := ParsePing(readFixture(t, "ping_loss.txt"), "", 1)

	if res.DNSFailed {
		t.Error("DNSFailed = true, want false")
	}
	if !res.Resolved {
		t.Error("Resolved = false, want true (domain was resolved)")
	}
	if res.Success {
		t.Error("Success = true, want false")
	}
	if res.Received != 0 || res.LossPercent != 100 {
		t.Errorf("received/loss = %d/%d, want 0/100", res.Received, res.LossPercent)
	}
}

// 没有统计行时不能乐观判定成功。
func TestParsePingNoStatsNotSuccess(t *testing.T) {
	res := ParsePing("", "", 1)

	if res.Success {
		t.Error("Success = true, want false when nothing was received")
	}
	if res.DNSFailed {
		t.Error("DNSFailed = true, want false")
	}
	if res.StatsFound {
		t.Error("StatsFound = true, want false")
	}
}

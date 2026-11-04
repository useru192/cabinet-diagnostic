package check

import (
	"strings"
	"testing"

	"cabinet-diagnostic/parsers"
)

// 真机 fixture 的内联副本（与 parsers/testdata 保持一致），
// judgeNetwork 只依赖解析结果，直接用 ParsePing 构造输入。
const (
	pingOKOutput = "PING www.baidu.com (110.242.68.3) 56(84) bytes of data.\n" +
		"64 bytes from 110.242.68.3: icmp_seq=1 ttl=52 time=32.1 ms\n" +
		"64 bytes from 110.242.68.3: icmp_seq=2 ttl=52 time=30.4 ms\n" +
		"64 bytes from 110.242.68.3: icmp_seq=3 ttl=52 time=31.2 ms\n" +
		"64 bytes from 110.242.68.3: icmp_seq=4 ttl=52 time=29.8 ms\n" +
		"\n" +
		"--- www.baidu.com ping statistics ---\n" +
		"4 packets transmitted, 4 received, 0% packet loss\n"

	pingLossOutput = "PING www.baidu.com (110.242.68.3) 56(84) bytes of data.\n" +
		"\n" +
		"--- www.baidu.com ping statistics ---\n" +
		"4 packets transmitted, 0 received, 100% packet loss\n"

	pingFailOutput = "ping: unknown host www.baidu.com\n"

	pingPartialLossOutput = "PING www.baidu.com (110.242.68.3) 56(84) bytes of data.\n" +
		"64 bytes from 110.242.68.3: icmp_seq=1 ttl=52 time=32.1 ms\n" +
		"\n" +
		"--- www.baidu.com ping statistics ---\n" +
		"4 packets transmitted, 1 received, 75% packet loss\n"
)

func networkItems(t *testing.T, stdout, stderr string, exitCode int) []Item {
	t.Helper()
	pr := parsers.ParsePing(stdout, stderr, exitCode)
	return judgeNetwork(pr)
}

func hasLevelWithText(items []Item, level Level, substr string) bool {
	for _, it := range items {
		if it.Level == level && strings.Contains(it.Message, substr) {
			return true
		}
	}
	return false
}

func TestJudgeNetworkOK(t *testing.T) {
	items := networkItems(t, pingOKOutput, "", 0)

	if !hasLevelWithText(items, LevelOK, "DNS resolution successful") {
		t.Errorf("missing [OK] DNS: %+v", items)
	}
	if !hasLevelWithText(items, LevelOK, "Ping successful") {
		t.Errorf("missing [OK] Ping: %+v", items)
	}
	if !hasLevelWithText(items, LevelOK, "Network: CONNECTED") {
		t.Errorf("missing [OK] CONNECTED: %+v", items)
	}
	for _, it := range items {
		if it.Level == LevelError || it.Level == LevelWarning {
			t.Errorf("unexpected non-OK item: %+v", it)
		}
	}
}

func TestJudgeNetworkDNSFailed(t *testing.T) {
	// 真机当前状态：unknown host，exit 2
	items := networkItems(t, pingFailOutput, "", 2)

	if !hasLevelWithText(items, LevelError, "DNS resolution failed") {
		t.Errorf("missing [ERROR] DNS: %+v", items)
	}
	if !hasLevelWithText(items, LevelError, "NOT CONNECTED") {
		t.Errorf("missing [ERROR] NOT CONNECTED: %+v", items)
	}
	if hasLevelWithText(items, LevelOK, "Ping successful") {
		t.Errorf("DNS failure must not report ping success: %+v", items)
	}
}

func TestJudgeNetworkAllLost(t *testing.T) {
	// 域名解析成功但 100% 丢包
	items := networkItems(t, pingLossOutput, "", 1)

	if !hasLevelWithText(items, LevelOK, "DNS resolution successful") {
		t.Errorf("domain resolved, should have [OK] DNS: %+v", items)
	}
	if !hasLevelWithText(items, LevelWarning, "Ping failed") {
		t.Errorf("missing [WARNING] ping failed: %+v", items)
	}
	if !hasLevelWithText(items, LevelError, "NOT CONNECTED") {
		t.Errorf("missing [ERROR] NOT CONNECTED: %+v", items)
	}
}

func TestJudgeNetworkPartialLoss(t *testing.T) {
	items := networkItems(t, pingPartialLossOutput, "", 0)

	if !hasLevelWithText(items, LevelOK, "Ping successful") {
		t.Errorf("received>0 means ping succeeded: %+v", items)
	}
	if !hasLevelWithText(items, LevelWarning, "75%") {
		t.Errorf("missing [WARNING] loss percent: %+v", items)
	}
	if !hasLevelWithText(items, LevelWarning, "CONNECTED (with packet loss)") {
		t.Errorf("missing WARNING connected-with-loss: %+v", items)
	}
}

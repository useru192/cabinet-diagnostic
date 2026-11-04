package check

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cabinet-diagnostic/internal/adb"
	"cabinet-diagnostic/internal/config"
	"cabinet-diagnostic/parsers"
)

// ---------------------------------------------------------------
// 网络连通性（任务6）
// ---------------------------------------------------------------

// checkNetwork 在设备端 ping 百度并给出连通性结论。
//
// 判定表（与设计文档 4.8 一致）：
//   - DNS 解析失败            -> [ERROR]   Network: NOT CONNECTED
//   - 收到 ICMP 响应          -> [OK]      Network: CONNECTED
//   - 部分丢包（有响应但有丢失） -> [WARNING] 补充丢包率
//   - 全部丢包 / 目标不可达    -> [WARNING] Network: NOT CONNECTED
//   - 命令超时                -> [TIMEOUT] Network: NOT CONNECTED
//   - 设备无 ping 命令        -> [UNKNOWN]
//
// 明确不做 4G/WiFi 归因：ping 不通不代表模块损坏（见设计文档 D7）。
func checkNetwork(ctx context.Context, r *adb.Runner) (res Result) {
	res.Name = "NETWORK CHECK"

	script := fmt.Sprintf("ping -c %d -W %d %s", config.PingCount, config.PingWaitSec, config.PingTarget)
	res.Cmd = "adb shell " + script

	out, err := r.Shell(ctx, config.TimeoutPing, script)
	res.Raw = out.Combined()

	if err != nil {
		if errors.Is(err, adb.ErrTimeout) {
			res.add(LevelTimeout, "ping did not finish within %s", config.TimeoutPing)
			res.add(LevelError, "Network: NOT CONNECTED")
		} else {
			res.add(LevelError, "Failed to run ping: %v", err)
		}
		return
	}

	// 设备端无 ping 命令：shell 报 "ping: not found"，退出码 127。
	lower := strings.ToLower(res.Raw)
	if out.ExitCode == 127 || strings.Contains(lower, "not found") {
		res.add(LevelUnknown, "ping command not available on device")
		return
	}

	pr := parsers.ParsePing(out.Stdout, out.Stderr, out.ExitCode)
	for _, it := range judgeNetwork(pr) {
		res.add(it.Level, "%s", it.Message)
	}
	return
}

// judgeNetwork 把 ping 解析结果翻译为判定条目。
// 独立成纯函数：单测用 fixture 覆盖，不需要真机。
//
// 终端只显示结论：成功时一行 CONNECTED，失败时展开原因。
// 目标 IP、DNS、丢包等细节标为 Detail，只写入报告。
func judgeNetwork(pr parsers.PingResult) []Item {
	target := "Target: " + config.PingTarget
	if pr.TargetIP != "" {
		target = fmt.Sprintf("Target: %s (%s)", config.PingTarget, pr.TargetIP)
	}
	items := []Item{{Level: LevelInfo, Message: target, Detail: true}}

	if pr.DNSFailed {
		return append(items,
			Item{Level: LevelError, Message: "DNS resolution failed (unknown host)"},
			Item{Level: LevelError, Message: "Network: NOT CONNECTED"})
	}

	switch {
	case pr.Success && pr.LossPercent == 0:
		items = append(items,
			Item{Level: LevelOK, Message: "DNS resolution successful", Detail: true},
			Item{Level: LevelOK, Message: "Ping successful, packet loss 0%", Detail: true},
			Item{Level: LevelOK, Message: fmt.Sprintf("Network: CONNECTED (%s)", config.PingTarget)})

	case pr.Success:
		// 有丢包属于"能用但有隐患"，终端展开让现场看到。
		items = append(items,
			Item{Level: LevelOK, Message: "DNS resolution successful", Detail: true},
			Item{Level: LevelOK, Message: "Ping successful"},
			Item{Level: LevelWarning, Message: fmt.Sprintf("Packet loss %d%% (%d of %d lost)",
				pr.LossPercent, pr.Transmitted-pr.Received, pr.Transmitted)},
			Item{Level: LevelWarning, Message: "Network: CONNECTED (with packet loss)"})

	default:
		// 域名能解析但一个响应都没收到：目标不可达，也可能只是禁 ICMP。
		items = append(items, Item{Level: LevelOK, Message: "DNS resolution successful", Detail: true})
		if pr.StatsFound {
			items = append(items, Item{Level: LevelWarning,
				Message: fmt.Sprintf("Ping failed, %d packets transmitted, 0 received", pr.Transmitted)})
		} else {
			items = append(items, Item{Level: LevelWarning,
				Message: "Ping failed or target is unreachable (no statistics in output)"})
		}
		items = append(items, Item{Level: LevelError, Message: "Network: NOT CONNECTED"})
	}
	return items
}

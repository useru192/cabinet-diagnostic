package parsers

import (
	"regexp"
	"strconv"
	"strings"
)

// dnsErrorMarkers 是 ping 在域名解析阶段失败时的输出特征。
// 出现任意一个即判定为 DNS 解析失败，而不是"目标不可达"。
var dnsErrorMarkers = []string{
	"unknown host",
	"bad address",
	"name or service not known",
	"no address associated",
	"temporary failure in name resolution",
}

var (
	reTxRx   = regexp.MustCompile(`(\d+) packets transmitted,\s*(\d+) (?:packets )?received`)
	reLoss   = regexp.MustCompile(`(\d+)% packet loss`)
	reTTL    = regexp.MustCompile(`ttl=\d+`)
	rePingIP = regexp.MustCompile(`PING \S+ \(([0-9.]+)\)`)
)

// PingResult 是一次 ping 检测的解析结果。
type PingResult struct {
	DNSFailed   bool // 域名解析失败
	Resolved    bool // 域名解析成功
	Success     bool // 收到响应
	StatsFound  bool // 是否存在统计行
	Transmitted int
	Received    int
	LossPercent int
	TargetIP    string // 解析出的目标 IP，如 PING www.baidu.com (110.242.68.3)
	Raw         string // 原始输出（stdout + stderr），用于写入检测日志
}

// ParsePing 解析 ping 命令的输出。
// exitCode 为命令退出码，仅在缺少统计行时作为兜底判据。
func ParsePing(stdout, stderr string, exitCode int) PingResult {
	raw := stdout
	if s := strings.TrimSpace(stderr); s != "" {
		if strings.TrimSpace(raw) != "" {
			raw += "\n"
		}
		raw += stderr
	}

	res := PingResult{Raw: raw}
	lower := strings.ToLower(raw)

	if m := rePingIP.FindStringSubmatch(raw); m != nil {
		res.TargetIP = m[1]
	}

	for _, marker := range dnsErrorMarkers {
		if strings.Contains(lower, marker) {
			res.DNSFailed = true
			res.Resolved = false
			res.Success = false
			return res
		}
	}
	res.Resolved = true

	if m := reTxRx.FindStringSubmatch(raw); m != nil {
		res.StatsFound = true
		res.Transmitted, _ = strconv.Atoi(m[1])
		res.Received, _ = strconv.Atoi(m[2])
	}
	if m := reLoss.FindStringSubmatch(raw); m != nil {
		res.LossPercent, _ = strconv.Atoi(m[1])
	}

	if res.StatsFound {
		res.Success = res.Received > 0
		return res
	}

	// 没有统计行时退化为看退出码与响应行。
	// 判定方向偏保守：无法确认收到响应就不算成功。
	res.Success = exitCode == 0 && reTTL.MatchString(lower)
	return res
}

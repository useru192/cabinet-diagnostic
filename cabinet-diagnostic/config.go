package main

import "time"

// ---------------------------------------------------------------
// 程序信息
// ---------------------------------------------------------------

const (
	AppName    = "Battery Swap Cabinet Diagnostic Tool"
	AppVersion = "0.1.0"
)

// ---------------------------------------------------------------
// 磁盘
// ---------------------------------------------------------------

// DiskWarnPercent 使用率达到或超过该值即告警。
const DiskWarnPercent = 80

// WritableMounts 可写数据分区：逐条输出并参与告警。
// /data 承载业务数据，满了会直接导致主控卡死；
// /cache 为 OTA 升级包与 recovery 日志暂存区；
// /mnt/scratch 为 APK 安装中转区。
var WritableMounts = []string{"/data", "/cache", "/mnt/scratch"}

// ReadOnlyMounts 只读系统分区：仅展示，不参与告警。
// 真机上 "/" 恒为 100%，若参与告警会对每次运行产生误报。
var ReadOnlyMounts = []string{
	"/", "/system", "/vendor", "/odm", "/product", "/system_ext", "/metadata",
}

// VirtualMounts 虚拟/内存盘：跳过不统计。
var VirtualMounts = []string{
	"/dev", "/apex", "/linkerconfig", "/data_mirror", "/mnt",
}

// DedupMounts 与其它分区同盘的挂载点，去重后不再重复统计。
// 真机上 /storage/emulated 与 /data 的 Size/Used 完全一致。
var DedupMounts = []string{"/storage/emulated"}

// ---------------------------------------------------------------
// 内存
// ---------------------------------------------------------------

// MemWarnAvailablePercent 内存告警阈值，含义为
// MemAvailable / MemTotal 低于该百分比时告警。
//
// nil 表示第一阶段不做告警判定，仅输出数据。
// 需要用真实设备基线测量后再设值，届时只改此变量即可。
var MemWarnAvailablePercent *int = nil

// ---------------------------------------------------------------
// 网络
// ---------------------------------------------------------------

const (
	PingTarget  = "www.baidu.com"
	PingCount   = 4
	PingWaitSec = 2
)

// ---------------------------------------------------------------
// 目录与统计
// ---------------------------------------------------------------

const (
	// CabinetDir 关键文件所在的业务目录。
	CabinetDir = "/data/cabinet"

	// TopFilesLimit 大文件分析最多输出的文件数量。
	TopFilesLimit = 40
)

// ---------------------------------------------------------------
// 超时
// ---------------------------------------------------------------

var (
	TimeoutServerStart = 5 * time.Second  // adb start-server
	TimeoutCommon      = 5 * time.Second  // getprop / ps / df / ls / id / uptime
	TimeoutPing        = 15 * time.Second // ping
	TimeoutScan        = 10 * time.Second // 目录遍历类命令
)

// ---------------------------------------------------------------
// 关键文件清单（与需求文档逐字一致，大小写敏感，不得增删）
// ---------------------------------------------------------------

var RequiredFiles = []string{
	"CabinetControlgo",
	"cabinet.apk",
	"cabinet.toml",
	"cabinet.toml.bak",
	"cabinettcpclient.pid",
	"grpcurl",
	"cabinetapp",
	"cabinetapp.pid",
	"cabinetlocalupgrade",
	"cabinetlocalupgrade.pid",
	"cabinetbluetooth.apk",
	"cabinetcontrolgo",
	"cabinetcontrolgo.pid",
	"cabinetcontrolgo.toml",
	"cabinetcontrolgo.toml.bak",
	"cabinetmqtt.pid",
	"cabinetmqtt.yaml",
	"cabinettcpclient",
	"restart.sh",
	"restart_app.sh",
	"restart_general_app.sh",
	"restart_xiaoyaapp.sh",
	"cabinetapp.yaml",
	"cabinetlocalupgrade.yaml",
	"deviceid",
	"sqlite.db",
	"startup.sh",
	"init.sh",
	"ota_upgrade_app.sh",
	"ota_upgrade_main.sh",
	"ota_upgrade_mcu.sh",
	"cabinetmqtt",
	"upgrade",
}

// ---------------------------------------------------------------
// 关键进程清单（与需求文档逐字一致，不得增删）
// ---------------------------------------------------------------

var RequiredProcesses = []string{
	"sh ./restart.sh cabinetcontrolgo",
	"sh ./restart.sh cabinettcpclient",
	"sh ./restart.sh cabinetmqtt",
	"sh ./restart.sh cabinetapp",
	"sh ./restart.sh cabinetlocalupgrade",
	"cabinetcontrolgo",
	"cabinettcpclient",
	"cabinetapp",
	"cabinetmqtt",
	"cabinetlocalupgrade",
	"cabinetapp.wasion",
}

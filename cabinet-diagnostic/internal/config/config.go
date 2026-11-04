// Package config 集中存放阈值、清单与超时，调整参数只需改这里。
package config

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
const DiskWarnPercent = 85

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
var DedupMounts = []string{"/storage/emulated"}

// ---------------------------------------------------------------
// 内存
// ---------------------------------------------------------------

// MemWarnUsedPercent 内存使用率达到或超过该值即告警。
// 设为 0 表示不告警。
var MemWarnUsedPercent = 85

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

	// DeviceIDFile 业务设备号文件，内容形如 HDZKA100337。
	DeviceIDFile = CabinetDir + "/deviceid"

	// TopFilesLimit 大文件分析最多输出的文件数量。
	TopFilesLimit = 20
)

// ---------------------------------------------------------------
// 超时
// ---------------------------------------------------------------

var (
	TimeoutServerStart = 5 * time.Second  // adb start-server
	TimeoutCommon      = 5 * time.Second  // ps / df / ls / id / uptime
	TimeoutPing        = 15 * time.Second // ping
	TimeoutScan        = 10 * time.Second // 目录遍历类命令
	TimeoutDelete      = 10 * time.Second // rm + 回读验证（菜单 3：删除设备文件）
)

// ---------------------------------------------------------------
// 删除约束（菜单 3「Delete Device File」）
//
// 删除是本工具唯一的设备写操作，由 internal/action 实施。
// 约束集中在此处维护，action 包只读不写：改保护清单只需在此增删一行。
// ---------------------------------------------------------------

// DeleteRoot 是唯一允许删除操作的目录前缀。
// 白名单之外的任何路径一律拒绝，防止误删系统或业务关键文件。
const DeleteRoot = CabinetDir

// DeleteProtectedFiles 禁止删除的业务关键文件（按基名精确匹配，大小写敏感）。
//
// 原则：可执行程序、配置、业务数据（数据库/设备号）、开机与守护脚本全部禁删——
// 删掉任何一个都可能导致柜子无法启动或业务中断，且现场没有恢复能力。
// 日志类文件不在保护之列，这才是本功能的目标场景。
var DeleteProtectedFiles = []string{
	// 业务数据
	"deviceid",
	"sqlite.db",
	// 主配置
	"cabinet.toml",
	"cabinet.toml.bak",
	"cabinetcontrolgo.toml",
	"cabinetcontrolgo.toml.bak",
	"cabinetmqtt.yaml",
	"cabinetapp.yaml",
	"cabinetlocalupgrade.yaml",
	// 可执行程序与 APK
	"cabinetcontrolgo",
	"cabinettcpclient",
	"cabinetmqtt",
	"cabinetapp",
	"cabinetlocalupgrade",
	"cabinet.apk",
	"cabinetbluetooth.apk",
	"grpcurl",
	// 脚本（开机、守护、升级）
	"startup.sh",
	"init.sh",
	"restart.sh",
	"restart_app.sh",
	"restart_general_app.sh",
	"ota_upgrade_app.sh",
	"ota_upgrade_main.sh",
	"ota_upgrade_mcu.sh",
}

// ---------------------------------------------------------------
// 关键文件清单（大小写敏感）
// ---------------------------------------------------------------

var RequiredFiles = []string{
	//"cabinetControlgo",
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
// 关键进程清单
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
}

# 换电柜主控诊断工具 — 设计方案

> 版本：v0.1（待确认）
> 目标设备：RK3568 / Android 11
> 运行环境：Windows + Go

---

## 一、项目现状

| 项 | 状态 |
|---|---|
| Go 环境 | `go1.26.3 windows/amd64`，已就绪 |
| 已有 Go 代码 | 无，全新项目 |
| 已有可复用资产 | `getsysteminfo.bat`（采集命令与超时策略）、`platform-tools/`（adb 31.0.3 + 2 个 DLL） |
| 联调设备 | `8925c67aefcd0126`，`rk3568_r`，Android 11，kernel 4.19.193，**非 root（uid=2000 shell）** |
| 真机验证状态 | 4 类命令输出样本已采集完毕（见附录 A） |

新项目建在独立目录 `cabinet-diagnostic/`，**不修改、不删除**现有 `getsysteminfo.bat`，它继续作为手动 fallback 保留。

---

## 二、范围

**做：**
- ADB 连接与设备状态判定
- 系统响应、磁盘、内存检测
- 大文件占用分析
- `/data/cabinet` 目录与 33 项关键文件检测
- 11 项关键进程检测
- 百度网络连通性检测（仅判定通/不通）
- 检测结果汇总输出
- **生成设备检测日志** `report_<设备序列号>_<时间>.txt`：包含全部检测项目、执行的原始命令、命令原始输出与判定结论，供研发人员离线全面分析

**不做（第一阶段）：**
- 删除任何设备文件
- 日志回传
- 服务重启、升级
- 4G / WiFi 模块归因

**约束：**
- 全程只读
- 所有 ADB 命令必须带超时
- 不得伪造结果、不得把检测失败当正常

---

## 三、总体设计

### 3.1 分层

```
┌─────────────────────────────────────────┐
│  main.go        菜单 / 交互循环          │
├─────────────────────────────────────────┤
│  diagnostic.go  流程编排 / 结果汇总      │
├─────────────────────────────────────────┤
│  system_check  file_check  process_check │
│  network_check        （检测项，只做判定）│
├─────────────────────────────────────────┤
│  parsers        df / ps / ls / ping 解析  │
│                （纯函数，可单测）         │
├─────────────────────────────────────────┤
│  adb.go         命令执行 / 超时 / 状态    │
├─────────────────────────────────────────┤
│  config.go      阈值 / 清单 / 目标        │
└─────────────────────────────────────────┘
```

**核心原则：解析层与执行层分离。**
`adb.go` 只负责"拿到字符串"，parsers 只负责"字符串 → 结构体"，检测项只负责"结构体 → 判定"。这样 15 个测试场景全部可以用 fixture 字符串覆盖，不需要真机。

### 3.2 关键设计决策

| # | 决策 | 依据（真机实测） |
|---|---|---|
| D1 | 磁盘告警**只针对可写数据分区**，只读系统分区不参与 | `/`（`/dev/block/dm-0`）恒定 **100%**，如果按 80% 阈值会对每次运行都报假警 |
| D2 | 只读系统分区折叠显示，虚拟盘跳过 | 实测 5 个 overlay 挂载点共用 674M、5 个 tmpfs 共用 979M |
| D3 | `/storage/emulated`(fuse) 与 `/data`(dm-6) 去重 | 实测两者 `10G / 349M` 完全相同，是同一块盘 |
| D4 | 进程判定用**名称精确匹配**，不使用 PID/PPID | PID 每次开机变化，不可作为判据 |
| D5 | 进程匹配要求 **CMD 第一个 token 完全相等** | 自动排除 `cabinetapp.wasion:jse`（子进程）和 `grep` 自身 |
| D6 | 设备端**不使用 grep**，`ps -ef` 全量回传后在本机过滤 | 避免 `sh -c ...`、`grep ...` 污染进程列表 |
| D7 | 网络检测只输出**通/不通**，不做 4G/WiFi 归因 | 按需求确认 |
| D8 | 所有检查在**非 root** 下必须可完成 | 实测 `uid=2000(shell)` 下 `/data/cabinet` 权限 777，完全可读 |
| D9 | `df -h` 解析从右往左定位字段 | toybox 长设备名会换行（实测 `/dev/block/mmcblk0p11` 那行） |
| D10 | 无法判定的结果统一 `[UNKNOWN]`，**不降级为正常** | 安全原则 |
| D11 | 检测日志必须记录**原始命令输出**，不能只记结论 | 研发需要离线复核；结论无法脱离原始数据验证 |
| D12 | 被跳过的检测项也要写入日志并说明原因 | 否则无法区分"没问题"与"没检测" |

---

## 四、模块设计

### 4.1 `main.go` — 程序入口与菜单

**输入**：stdin 用户按键
**输出**：菜单文本、检测结果

```
========================================
   Battery Swap Cabinet Diagnostic Tool
========================================

Device: RK3568 / Android 11

1. Check System Status
2. Check Network Connectivity
3. Run Full Diagnostics
0. Exit

Please select an option:
```

**异常处理**
- 非法输入（非 1/2/3/0、空行、超长）：提示 `Invalid option, please try again.`，回到菜单
- stdin 读取 EOF：按 0 处理，正常退出
- 任何检测项 panic：recover 后输出 `[ERROR] Internal error in <module>`，不退出程序

**验收标准**
- [ ] 1/2/3/0 均能正确进入对应功能并返回菜单
- [ ] 输入 `abc`、空格、回车、`0x01` 不崩溃且提示重试
- [ ] 0 正常退出，退出码 0
- [ ] 菜单为纯英文，无乱码

---

### 4.2 `adb.go` — ADB 执行与连接判定

**输入**：adb 路径、命令参数、超时时间
**输出**：`stdout string`、`exitCode int`、`error`

**接口**

```go
type Runner interface {
    Run(ctx context.Context, args ...string) (Output, error)
    Shell(ctx context.Context, cmd string) (Output, error) // 等价 adb shell <cmd>
}

type Output struct {
    Stdout   string
    Stderr   string
    ExitCode int
    TimedOut bool
}
```

**连接状态判定（`adb devices` 输出解析）**

| 输出特征 | 判定 |
|---|---|
| `adb` 可执行文件不存在 / 启动失败 | `ADB_NOT_FOUND` |
| 无任何设备行 | `NO_DEVICE` |
| 状态为 `unauthorized` | `UNAUTHORIZED` |
| 状态为 `offline` | `OFFLINE` |
| 状态为 `device` | `READY` |
| `adb devices` 本身超时 | `ADB_TIMEOUT` |

**超时**
- 启动时 `adb start-server`：15s
- 常规查询（`ps`/`df`/`ls`/`id`/`uptime`）：5s
- `ping`：15s
- `du` 类遍历：120s
- 超时后**不重试**，直接标记 `[TIMEOUT]`，并停止依赖设备的后续检测

**异常处理**
- 找不到 adb：提示检查 ADB 安装与环境变量，**不退出程序**，菜单仍可用（网络检测也依赖 adb，会一并提示）
- **多设备**：现场只会连接一台主控，**不实现多设备选择逻辑**。若出现多台，直接使用检测到的第一台
- **未授权（unauthorized）**：提示用户在**设备屏幕**上点击 USB 调试弹窗的 `Allow`（建议勾选 `Always allow from this computer`），然后按 Enter 重试。提示文案：
  ```text
  [ERROR] Device is unauthorized.
          Please check the device screen and tap "Allow" on the
          USB debugging prompt, then press Enter to retry.
  ```

**验收标准**
- [ ] 拔掉设备 → `NO_DEVICE`，提示准确
- [ ] 设备未授权 → `UNAUTHORIZED`，并提示在设备屏幕点击 Allow
- [ ] 不实现多设备选择逻辑（现场只有一台）
- [ ] 人为设置 1ms 超时 → `[TIMEOUT]`，程序不卡死
- [ ] 设备未连接时选择 3（全量检测），后续项全部跳过并说明原因，不产生大量报错

---

### 4.3 `config.go` — 配置

**内容**

```go
const DiskWarnPercent = 80

// 参与告警的可写数据分区白名单
var WritableMounts = []string{"/data", "/cache", "/mnt/scratch"}

// 只读/虚拟挂载点（展示但不告警）
var ReadOnlyMounts = []string{"/", "/system", "/vendor", "/odm",
    "/product", "/system_ext", "/metadata"}

var RequiredFiles = []string{ /* 33 项，见 prompt 8.1，一字不改 */ }

var RequiredProcesses = []string{ /* 11 项，见 prompt 9.1，一字不改 */ }

const PingTarget = "www.baidu.com"
const PingCount = 4
const PingTimeoutSec = 2

// 待确认：内存告警阈值，暂不设默认告警，仅输出数据
var MemWarnUsedPercent = 85 // 内存使用率阈值，0 表示不告警
```

**验收标准**
- [ ] 33 个文件名与 prompt 8.1 逐字一致（含大小写）
- [ ] 11 个进程名与 prompt 9.1 逐字一致
- [ ] 阈值修改后无需改其他代码
- [ ] 有单元测试断言清单长度与关键条目

---

### 4.4 `diagnostic.go` — 流程编排

**输入**：`Runner`、用户选择项
**输出**：`Report` 结构体（分组的结果列表 + 总览）

**全量检测顺序**（严格按 prompt 十一）
1. ADB 连接检查
2. 系统响应检查
3. 磁盘空间检查
4. 大文件占用分析
5. 内存检查
6. `/data/cabinet` 目录及关键文件检查
7. 关键进程检查
8. Ping 百度
9. 汇总

**控制逻辑**
- 步骤 1 或 2 失败 → 打印原因，**终止**后续设备端检测
- 步骤 3-8 中某项失败 → 记录 `[UNKNOWN]`，**继续**下一项

**验收标准**
- [ ] 无设备时选 3，只输出连接失败原因，不输出 5-8 项的无效报错
- [ ] 单项失败不影响后续项
- [ ] 汇总包含：异常分区、缺失文件、未运行进程、网络结果

---

### 4.5 `system_check.go` — 系统响应 / 磁盘 / 内存

**输入**：`adb shell` 输出字符串
**输出**：`SystemResult`

#### 磁盘

解析规则（D9）：每行 `strings.Fields` 后从右往左取——`f[n-1]`=挂载点、`f[n-2]`=Use%、`f[n-3]`=Avail、`f[n-4]`=Used、`f[n-5]`=Size。
长设备名换行产生的空行/续行直接跳过。

分区分类（D1/D2/D3）：

| 挂载点 | 归属 | 处理 |
|---|---|---|
| `/data`、`/cache`、`/mnt/scratch` | 可写数据 | 逐条输出，≥80% 告警 |
| `/`、`/system`、`/vendor`、`/odm`、`/product`、`/system_ext`、`/metadata` | 只读系统 | 折叠为一行，标 `[RO]`，不告警 |
| `tmpfs` 类型、`/dev`、`/apex`、`/linkerconfig`、`/data_mirror` | 虚拟 | 折叠为一行，标 `[SKIP]` |
| `/storage/emulated` | 与 `/data` 同盘 | 去重，不重复输出 |
| 无法解析 | — | `[UNKNOWN]`，不算正常 |

> 三个可写分区的性质不同：
> - `/data`：业务数据（日志、sqlite.db 等），**满了会直接导致主控卡死，是重点**
> - `/cache`：OTA 升级包与 recovery 日志的暂存区，满了主要影响系统升级
> - `/mnt/scratch`：APK 安装过程的中转区，满了主要影响应用安装/更新
>
> 后两者满了不会造成卡死，但同样按 80% 告警——判断成本为零，且能提前暴露升级失败的原因。报告中会标明是哪个分区，便于研发区分。

**输出示例（基于真机数据）**

```text
========== Disk Check ==========
[OK]      /data         4%    10G total / 349M used / 10G avail
[OK]      /cache        1%    356M total / 112K used / 344M avail
[OK]      /mnt/scratch  1%    674M total / 1.2M used / 673M avail
[RO]      /, /system, /vendor, /odm, /product, /system_ext, /metadata (read-only, not counted)
[SKIP]    5 virtual mounts (tmpfs)
```

#### 大文件

**仅当**存在 ≥80% 的可写分区时触发。

命令：`ls -lR /data/cabinet` → 解析出所有**普通文件**的大小 → 按大小降序取前 40。

> 注意：不能用 `du -ak | sort -rn`，因为目录累计值会排在最前，看不到真正的大文件。

**异常**：非 root 且权限不足 → `[UNKNOWN] Permission denied`；目录不存在 → `[ERROR] Directory not found`；超时 → `[TIMEOUT]`

#### 内存

命令：`cat /proc/meminfo`

不使用 `free -m`：它的 `used`/`free` 把 buffer/cache 也算进去了，会严重误导
（真机上 `free` 显示 48 MB，实际 `MemAvailable` 有 847 MB）。
统一取 `/proc/meminfo` 的 `MemTotal` 与 `MemAvailable`。

**判定**：使用率 = `100 - MemAvailable/MemTotal`，阈值 `MemWarnUsedPercent`（当前 85%）。

| 条件 | 标签 | 输出 |
|---|---|---|
| 使用率 < 85% | `[OK]` | `Memory usage 53% (Total 1958 MB, Used 1037 MB)` |
| 使用率 ≥ 85% | `[WARNING]` | 同上格式 |
| 阈值设为 0 | `[INFO]` | 只输出数据，不作判断 |

阈值与磁盘保持同一语义（使用率），便于现场理解。不同批次主控内存容量不同，
因此只比百分比，不设固定 MB 值。

**验收标准**
- [ ] 真机 `df -h` 输出能正确解析出 4 个可写分区，`/` 不触发告警
- [ ] 构造 92% 的 `/data` 输出 → 触发 `[WARNING]`
- [ ] 构造无法解析的行 → `[UNKNOWN]`
- [ ] `/data/cabinet` 不存在 → 精确输出 `[ERROR] Directory not found: /data/cabinet`
- [ ] 内存命令无输出 → `[UNKNOWN]`，不崩溃

---

### 4.6 `file_check.go` — 目录与关键文件

**输入**：`ls -la /data/cabinet` 原始输出
**输出**：33 项逐项结果

**解析规则**
- 每行 `strings.Fields`，跳过 `total N` 和 `d .` / `d ..`
- 类型：`f[0][0]` — `d`=目录、`-`=普通文件、`l`=符号链接
- 名称：`strings.Join(f[7:], " ")`（兼容文件名含空格）
- 大小写**精确匹配**（`CabinetControlgo` 与 `cabinetcontrolgo` 是两个不同条目，真机确认两者都存在）

**输出**

```text
========== File Check ==========
[OK]      /data/cabinet/CabinetControlgo          (directory)
[OK]      /data/cabinet/cabinetcontrolgo          (file)
[MISSING] /data/cabinet/cabinetmqtt.yaml
```

**异常处理**
- 目录不存在 → `[ERROR] Directory not found: /data/cabinet`，33 项全部跳过
- 单个文件缺失 → 输出 `[MISSING]`，**继续检查其余项**
- 无法解析的 `ls` 行 → 忽略该行，若有必需文件最终未匹配到，按 `[MISSING]` 处理

**验收标准**
- [ ] 用真机 `ls -la` 输出做 fixture，33 项全部判 `[OK]`
- [ ] 删除任意一项后该行变 `[MISSING]`，其余不受影响
- [ ] `upgrade`（目录）判为已存在，不误报
- [ ] `CabinetControlgo` 与 `cabinetcontrolgo` 分别匹配，不混淆

---

### 4.7 `process_check.go` — 关键进程

**输入**：`ps -ef` 原始输出（全量，不 grep）
**输出**：11 项逐项结果

**解析规则（D4/D5/D6）**
- 表头 `UID PID PPID C STIME TTY TIME CMD`，共 7 个固定列，CMD = `f[7:]`
- 业务进程：`f[7] == 目标名`（整串精确相等）
- 守护脚本：`f[7]=="sh" && f[8]=="./restart.sh" && f[9]==目标名`
- **不使用 PID / PPID 做任何判断**（每次开机都变）

**已验证的匹配结果（真机数据）**

| ps CMD | 目标 | 结果 |
|---|---|---|
| `sh ./restart.sh cabinetcontrolgo` | 同左 | RUNNING |
| `cabinetcontrolgo` | `cabinetcontrolgo` | RUNNING |
| `cabinetapp.wasion` | `cabinetapp.wasion` | RUNNING |
| `cabinetapp.wasion:jse 57 61 1 /data/...` | `cabinetapp.wasion` | 正确排除（token0 不等） |
| `sh -c ps -ef \| grep ...` | 任意 | 正确排除 |
| `grep -E restart.sh\|cabinet` | 任意 | 正确排除 |

**输出**

```text
========== Process Check ==========
[RUNNING]      cabinetcontrolgo
[RUNNING]      sh ./restart.sh cabinetcontrolgo
[NOT RUNNING]  cabinetmqtt
```

**「守护在、业务不在」**：守护脚本与业务进程是两条独立条目，分别判定即可，无需 PPID。

**异常处理**
- `ps -ef` 不支持或输出为空 → 11 项全部 `[UNKNOWN]`，不得判为正常
- 权限不足导致 CMD 列被截断 → 目标名仍能精确匹配；无法匹配时 `[UNKNOWN]`

**验收标准**
- [ ] 真机 `ps -ef` fixture → 11 项结果与手工核对一致
- [ ] 构造 `cabinetapp.wasion:jse` 行 → 不误判主进程存活
- [ ] 构造只有守护脚本、无业务进程的输出 → 守护 RUNNING、业务 NOT RUNNING
- [ ] 构造 `grep` 行 → 不误判
- [ ] `ps` 输出为空 → 全部 `[UNKNOWN]`

---

### 4.8 `network_check.go` — 网络连通性

**输入**：`adb shell ping -c 4 -W 2 www.baidu.com` 的 stdout/stderr/exitCode
**输出**：连通 / 不连通 + 一行依据

**判定表**

| 条件 | 输出 |
|---|---|
| exitCode=0 且出现 `packets transmitted` 且有接收 | `[OK] DNS resolution successful` + `[OK] Ping successful` + `--> Network: CONNECTED` |
| stderr 含 `unknown host` / `bad address` / `No address` | `[ERROR] DNS resolution failed` + `--> Network: NOT CONNECTED` |
| 命令超时 | `[TIMEOUT]` + `--> Network: NOT CONNECTED` |
| exitCode≠0 且非 DNS 错误 | `[WARNING] Ping failed or target unreachable` + `--> Network: NOT CONNECTED` |
| `ping` 命令不存在 | `[UNKNOWN] ping not available` |

**明确不做**：不判断 4G/WiFi、不读路由表、不读 `ttyUSB`、不分析原因。

**验收标准**
- [ ] 真机当前状态（`unknown host`，exit 2）→ `NOT CONNECTED`，且不输出额外分析
- [ ] 连通时 → `CONNECTED`
- [ ] 超时 → `[TIMEOUT]`
- [ ] 设备无 `ping` → `[UNKNOWN]`

---

### 4.9 `report.go` — 检测日志与结果汇总

产出两个东西：**终端显示**（给现场人员看）和 **检测日志文件**（给研发分析）。两者内容不同，不能互相代替。

#### 4.9.1 检测日志文件

**文件名**：`report_<设备标识>_<yyyyMMdd_HHmmss>.txt`

- **设备标识优先用业务设备号**（`/data/cabinet/deviceid`，形如 `HDZKA100337`），读不到时回退 adb 序列号
- 例：`report_HDZKA100337_20261030_161957.txt`

**生成时机**

- 每执行一次检测（菜单 1 / 2 / 3）生成一个独立文件
- **连接阶段失败时不生成报告**（没有任何检测数据，空文件只会干扰判断）
- **边检测边写**：每完成一项立即落盘、不缓冲。主控卡死时程序可能被强杀，集中到最后写会导致前面的数据全部丢失，而现场往往只复现一次

**保存位置**：优先 exe 所在目录；无写权限时回退到 `%USERPROFILE%\Documents\CabinetDiagnostic\`，并在终端提示实际路径。

**编码**：UTF-8 with BOM（保证 Windows 记事本正确识别中文，避免出现之前那种乱码问题）。

**文件结构（三段）**

1. **环境快照** — 生成时间、工具版本、adb 版本、主机名/用户、设备序列号、业务设备号、uptime、root 状态
2. **检测明细** — 按检测顺序，每一项都必须包含三要素：
   - 检测项名称与序号
   - 执行的原始命令
   - **命令原始输出（完整保留，不裁剪）**
   - 该项判定结论与判定依据
3. **汇总** — 与终端 Summary 一致

**核心原则：日志必须包含判定结论所依据的全部原始数据。** 研发拿到这一个文件就能独立复核，不需要再向现场索要任何东西。

**被跳过的检测项也要记录**，并写明原因（如 `Skipped: no writable partition >= 80%`），否则研发无法判断是"没问题"还是"没检测"。

**样例**

```text
==============================================================
 Cabinet Diagnostic Report
==============================================================
Generated at   : 2026-10-29 14:45:12
Tool version   : 0.1.0
ADB version    : Android Debug Bridge version 1.0.41 (31.0.3-7562133)
Host           : DESKTOP-XXXX / Administrator
--------------------------------------------------------------
Device serial  : f4b9a37a1aabf412
Device ID      : HDZKA100337
Uptime         : 12:58:28 up 55 min, 0 users, load average: 1.63, 1.06, 0.63
Root           : NO (uid=2000 shell)
==============================================================

[1] SYSTEM RESPONSE
--------------------------------------------------------------
$ adb shell id
uid=2000(shell) gid=2000(shell) groups=2000(shell),1004(input),...

Result: [OK] Device responsive


[2] DISK CHECK
--------------------------------------------------------------
$ adb shell df -h
Filesystem            Size  Used Avail Use% Mounted on
/dev/block/dm-0       936M  933M     0 100% /
/dev/block/dm-6        10G  349M   10G   4% /data
/dev/fuse              10G  349M   10G   4% /storage/emulated
...

Result: [OK] 3 writable partitions normal
        [RO] /, /system, /vendor, /odm, /product, /system_ext, /metadata (read-only, not counted)
        [SKIP] 5 virtual mounts (tmpfs)


[3] LARGE FILE ANALYSIS
--------------------------------------------------------------
Skipped: no writable partition >= 80%


[9] SUMMARY
--------------------------------------------------------------
Disk        : [OK]
Files       : [MISSING] 1 of 33 missing
Processes   : [NOT RUNNING] 1 of 11 not running
Network     : [ERROR] NOT CONNECTED

Issues found:
  - [MISSING] /data/cabinet/cabinetmqtt.yaml
  - [NOT RUNNING] cabinetmqtt
  - [ERROR] Network unreachable (DNS resolution failed)

==============================================================
 End of report
==============================================================
```

#### 4.9.2 终端显示

只输出各检测项的判定行 + Summary + 日志文件路径，**不刷原始输出**（避免现场人员看一屏看不懂的内容）。

```text
========== Summary ==========
Disk        : [OK]            3 writable partitions normal
Files       : [MISSING]       1 of 33 missing
Processes   : [NOT RUNNING]   1 of 11 not running
Network     : [ERROR]         NOT CONNECTED

Issues found:
  - [MISSING] /data/cabinet/cabinetmqtt.yaml
  - [NOT RUNNING] cabinetmqtt
  - [ERROR] Network unreachable (DNS resolution failed)

Report saved: D:\tools\report_8925c67aefcd0126_20261029_144512.txt
```

**状态汇总优先级**：`ERROR` > `NOT RUNNING` / `MISSING` > `TIMEOUT` > `WARNING` > `UNKNOWN` > `OK`

**验收标准**
- [ ] 文件名严格为 `report_<serial>_<yyyyMMdd_HHmmss>.txt`
- [ ] 每个检测项在报告中都具备「命令 + 原始输出 + 结论」三要素
- [ ] 原始输出不裁剪、不截断
- [ ] 被跳过的检测项有明确的 `Skipped: <原因>` 记录
- [ ] 报告含设备序列号、业务设备号、uptime、root 状态
- [ ] 记事本打开无乱码
- [ ] 目录只读时回退到 Documents 并提示，流程不中断
- [ ] 报告写入失败不影响终端显示，也不影响检测结论

---

### 4.10 预留扩展接口（第一阶段只定义，不实现）

```go
// 后续：删除文件 / 清理日志
type Action interface {
    Name() string
    Describe(ctx context.Context, r Runner) (Preview, error) // 先预览影响
    Apply(ctx context.Context, r Runner, confirm bool) (Result, error)
}

// 后续：日志回传
type Reporter interface {
    Upload(ctx context.Context, report Report) error
}
```

第一阶段：两个接口定义在 `extend.go`，**不注册到菜单、不产生任何写操作**。

---

## 五、文件结构

```text
cabinet-diagnostic/
├── main.go                  // 入口、菜单与主循环
├── go.mod
├── build.bat                // go vet + go test + go build
├── parsers/                 // 纯解析函数（可单测）
│   ├── df.go
│   ├── ps.go
│   ├── ls.go
│   ├── ping.go
│   └── testdata/            // 真机输出 fixture
│       ├── df_h.txt
│       ├── ps_ef.txt
│       ├── ls_la.txt
│       ├── ping_ok.txt
│       ├── ping_loss.txt
│       └── ping_fail.txt
├── internal/
│   ├── config/
│   │   └── config.go        // 阈值、文件清单、进程清单、超时
│   ├── adb/
│   │   ├── adb.go           // 命令执行、超时、连接状态、设备信息
│   │   ├── adb_test.go
│   │   ├── exec_windows.go  // 控制台 UTF-8、隐藏 adb 黑窗
│   │   ├── exec_other.go
│   │   └── assets/          // 内嵌 adb.exe + 2 个依赖 DLL
│   └── check/
│       ├── check.go         // Result / Item / Level 契约
│       ├── system_check.go  // 系统响应 / 磁盘 / 内存 / 大文件
│       ├── file_check.go    // 待实现
│       ├── process_check.go // 待实现
│       ├── network_check.go // 待实现（由他人负责）
│       ├── diagnostic.go    // 流程编排
│       ├── report.go        // 报告文件 + 终端输出
│       └── system_check_test.go
├── reports/                 // 生成的检测日志，运行时自动创建
└── README.md
```

> 根目录只保留 `main.go`，其余按包归置。注意 Go 要求 `_test.go` 与被测代码**同包同目录**，无法单独放到 `test/` 目录，因此测试文件跟随各自所属的包。

> 相对 prompt 第十二节，增加了 `parsers/` 和 `testdata/`。理由：解析是唯一需要精细单测的部分，独立成包更清晰。

---

## 六、测试计划（对应 prompt 十四，15 项）

| # | 场景 | 类型 | 验证点 |
|---|---|---|---|
| 1 | ADB 不存在 | 模拟 | `ADB_NOT_FOUND`，不崩溃 |
| 2 | 设备未连接 | 模拟 | `NO_DEVICE` |
| 3 | 设备未授权 | fixture | `UNAUTHORIZED` |
| 4 | 设备无响应 | 模拟超时 | `[TIMEOUT]`，后续检测停止 |
| 5 | 磁盘 <80% | fixture | 全部 `[OK]` |
| 6 | 磁盘 ≥80% | fixture | `[WARNING]` + 触发大文件分析 |
| 7 | 大文件统计成功/失败 | fixture | 成功排序正确；失败 `[UNKNOWN]` |
| 8 | 内存获取失败 | fixture | `[UNKNOWN]` |
| 9 | `/data/cabinet` 不存在 | fixture | `[ERROR] Directory not found` |
| 10 | 关键文件缺失 | fixture | `[MISSING]` + 其余不受影响 |
| 11 | 进程运行/未运行 | fixture | `[RUNNING]` / `[NOT RUNNING]` |
| 12 | 守护在、业务不在 | fixture | 两条结果不一致 |
| 13 | 百度解析成功/失败 | fixture | `[OK]` / `[ERROR]` |
| 14 | Ping 成功/失败/超时 | fixture | 三种状态可区分 |
| 15 | 菜单非法输入 | 手工 | 不崩溃、提示重试 |

**额外补充测试（本次实测发现的问题）**

| # | 场景 | 验证点 |
|---|---|---|
| 16 | `/` 分区 100% | **不产生** `[WARNING]` |
| 17 | `/data` 与 `/storage/emulated` 同盘 | 只输出一次 |
| 18 | `cabinetapp.wasion:jse` 子进程 | 不误判主进程存活 |
| 19 | toybox `df -h` 长设备名换行 | 解析不串行 |
| 20 | 检测日志生成 | 文件名格式正确；每项含命令+原始输出+结论；跳过项有原因 |
| 21 | 报告目录只读 | 回退到 Documents 并提示，流程不中断 |

**测试方式**：`parsers` 包全部用 `testdata/` 中的真机输出做表驱动测试，`go test ./...` 不需真机。

---

## 七、待你确认的事项

| # | 问题 | 我的建议 |
|---|---|---|
| ~~Q1~~ | ~~报告是否需要落盘？~~ | **已确认**：必须落盘，`report_<设备序列号>_<时间>.txt`，内容为检测项目 + 原始输出，供研发全面分析 |
| ~~Q2~~ | ~~`adb.exe` + DLL 是否内嵌？~~ | **已确认**：内嵌 `go:embed`，单文件交付 |
| ~~Q3~~ | ~~内存告警阈值？~~ | **已确认**：使用率 85% 告警，与磁盘阈值同语义 |
| ~~Q4~~ | ~~80% 阈值是否管 `/cache`、`/mnt/scratch`？~~ | **已确认**：管，但 `/data` 是重点 |
| ~~Q5~~ | ~~大文件分析是否每次跑？~~ | **已确认**：仅在存在 ≥80% 可写分区时触发 |
| ~~Q6~~ | ~~多台设备如何处理？~~ | **已确认**：现场只有一台，不实现多设备逻辑 |
| ~~Q7~~ | ~~unauthorized 是否提示？~~ | **已确认**：提示在设备屏幕点击 Allow 后按 Enter 重试 |

> 全部问题已确认，可进入阶段 1 编码。

---

## 附录 A：真机实测数据（解析器设计依据）

**设备**：`8925c67aefcd0126` / `rk3568_r` / Android 11 / kernel 4.19.193 / 非 root

### A.1 `df -h`

```text
Filesystem            Size  Used Avail Use% Mounted on
tmpfs                 979M  824K  978M   1% /dev
tmpfs                 979M     0  979M   0% /mnt
/dev/block/mmcblk0p11  11M  144K   11M   2% /metadata
/dev/block/dm-0       936M  933M     0 100% /
/dev/block/dm-5       674M  1.2M  673M   1% /mnt/scratch
overlay               674M  1.2M  673M   1% /system
overlay               674M  1.2M  673M   1% /vendor
overlay               674M  1.2M  673M   1% /odm
overlay               674M  1.2M  673M   1% /product
overlay               674M  1.2M  673M   1% /system_ext
tmpfs                 979M     0  979M   0% /apex
tmpfs                 979M  264K  979M   1% /linkerconfig
/dev/block/mmcblk0p10 356M  112K  344M   1% /cache
/dev/block/dm-6        10G  349M   10G   4% /data
tmpfs                 979M     0  979M   0% /data_mirror
/dev/fuse              10G  349M   10G   4% /storage/emulated
```

### A.2 `ps -ef`（节选）

```text
UID PID PPID C STIME TTY TIME CMD
root  1773 1198 0 12:03:31 ? 00:00:01 sh ./restart.sh cabinetcontrolgo
root  1792 1773 2 12:03:31 ? 00:01:24 cabinetcontrolgo
u0_a125 2023 260 8 12:03:51 ? 00:04:57 cabinetapp.wasion
u0_a125 2076 2023 1 12:03:52 ? 00:00:25 cabinetapp.wasion:jse 57 61 1 /data/user/0/cabinetapp.wasion/app_crash/crash_dump.log
```

### A.3 `ls -la /data/cabinet`（节选）

```text
total 89777
drwxrwxrwx  9 root   root       3452 2024-04-19 12:03 .
drwxrwx--x 48 system system     4096 2024-04-19 12:03 ..
drwxrwxrwx 17 shell  shell      3452 2024-04-19 12:35 CabinetControlgo
-rwxrwxrwx  1 root   root   20110464 2024-05-17 11:37 cabinet.apk
-rwxrwxrwx  1 root   root   5828344 2026-10-27 17:11 cabinetcontrolgo
drwxrwxrwx  2 root   root       3452 2024-07-16 09:27 upgrade
```

> 33 项清单在真机上**全部存在**。

### A.4 `ping`

```text
$ ping -c 2 -W 2 www.baidu.com
ping: unknown host www.baidu.com      (exit 2)
```

网络环境实际状态：仅 `eth0`（`192.168.0.1/24`，`linkdown`），无默认路由，`net.dns1` 为空。

---

*文档结束。确认第七节 Q1–Q7 后即可进入阶段 1 编码。*

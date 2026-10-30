@echo off
chcp 936 >nul
setlocal EnableExtensions EnableDelayedExpansion
title 主控卡死诊断 - 信息采集
color 0A

REM =====================================================================
REM  主控卡死诊断采集脚本（内存 / 存储 / 进程）
REM  用法：双击运行，全程约 1-2 分钟
REM  产出：脚本所在目录 mem_report_设备序列号_时间.txt
REM =====================================================================

REM ------------------------- 配置区 -------------------------
REM 留空 = 自动查找（会找同目录 platform-tools\adb.exe、PATH、常见安装位置）
set "ADB="
REM 留空 = 自动识别设备；多台时会提示选择
set "SERIAL="
REM 采集完是否自动打开报告：1=打开，0=不打开
set "OPEN_REPORT=1"
REM ----------------------------------------------------------

echo ==================================================
echo    主控卡死诊断 - 信息采集
echo ==================================================
echo.

REM ---------- 第 0 步：查找 adb ----------
call :find_adb
if not defined ADB (
    echo [错误] 未找到可用的 adb
    echo.
    echo   已按以下顺序查找，均未找到完整可用的版本：
    echo     1. 脚本目录\platform-tools\adb.exe
    echo     2. 脚本目录\adb.exe
    echo     3. 环境变量 PATH 中的 adb
    echo     4. 常见安装位置
    echo.
    echo   注意：adb.exe 必须和 AdbWinApi.dll、AdbWinUsbApi.dll 在同一目录，
    echo         只有 adb.exe 的会被判为不可用（否则会报 AdbWinApi.dll 缺失）。
    echo.
    echo   解决办法（二选一）：
    echo     1. 把完整的 platform-tools 文件夹放到本脚本同一目录
    echo        （目录下必须含 adb.exe + AdbWinApi.dll + AdbWinUsbApi.dll）
    echo     2. 用记事本打开本脚本，把开头的 ADB= 改成 adb.exe 完整路径
    echo.
    pause
    exit /b 1
)
echo [1/4] adb: %ADB%

REM ---------- 第 1 步：定位设备 ----------
REM 先落到临时文件再解析：避免 adb 路径含 () 时 for /f 内联执行被截断
set "DEVLIST=%TEMP%\adb_devices_%RANDOM%.txt"
"%ADB%" devices > "%DEVLIST%" 2>nul
set "N=0"
for /f "usebackq skip=1 tokens=1,2" %%a in ("%DEVLIST%") do (
    if "%%b"=="device" (
        set /a N+=1
        set "DEV!N!=%%a"
    )
)
del "%DEVLIST%" >nul 2>&1
if !N!==0 (
    echo [错误] 没有检测到已连接的设备
    echo        请检查 USB 连接 / 驱动，或在 cmd 里执行 adb devices 确认
    echo.
    "%ADB%" devices
    pause
    exit /b 1
)
if !N!==1 (
    set "SERIAL=!DEV1!"
) else (
    echo [提示] 检测到 !N! 台设备：
    for /l %%i in (1,1,!N!) do echo        %%i. !DEV%%i!
    echo.
    set /p "SERIAL=请输入要分析设备的序列号: "
)
if "!SERIAL!"=="" (
    echo [错误] 未选择设备
    pause
    exit /b 1
)
set "SEROPT=-s !SERIAL!"
echo [2/4] 目标设备: !SERIAL!

REM ---------- 第 2 步：root ----------
"%ADB%" %SEROPT% root >nul 2>&1
ping -n 3 127.0.0.1 >nul
"%ADB%" %SEROPT% shell id 2>nul | findstr /C:"uid=0" >nul
if errorlevel 1 (
    echo [警告] 未取得 root 权限，部分目录可能读不到
) else (
    echo [3/4] 已取得 root 权限
)

REM ---------- 第 3 步：逐项采集 ----------
REM 本次运行使用唯一的设备端文件名，避免历史残留会话交叉写入
for /f %%i in ('powershell -NoProfile -Command "Get-Date -Format yyyyMMdd_HHmmss"') do set "STAMP=%%i"
if not defined STAMP set "STAMP=%RANDOM%"
set "RPT=/data/local/tmp/mem_report_%STAMP%.txt"
set "OUT=%~dp0mem_report_%SERIAL%_%STAMP%.txt"

echo [4/4] 开始采集，请保持设备连接，勿关闭窗口...
"%ADB%" %SEROPT% shell "rm -f /data/local/tmp/mem_report*.txt" 2>nul

echo    - [1/11] 系统信息 / 运行时间
"%ADB%" %SEROPT% shell "echo '===== [1] TIME / DEVICE =====' >> %RPT%; date >> %RPT%; getprop ro.product.model >> %RPT%; getprop ro.serialno >> %RPT%; getprop ro.build.display.id >> %RPT%; getprop ro.build.version.release >> %RPT%; uname -a >> %RPT%; uptime >> %RPT%"

echo    - [2/11] 磁盘空间 df -h
"%ADB%" %SEROPT% shell "echo '' >> %RPT%; echo '===== [2] DISK SPACE (df -h) =====' >> %RPT%; df -h >> %RPT% 2>&1"

echo    - [3/11] inode 使用 df -i
"%ADB%" %SEROPT% shell "echo '' >> %RPT%; echo '===== [3] INODES (df -i) =====' >> %RPT%; df -i >> %RPT% 2>&1"

echo    - [4/11] 内存 free -m / meminfo
"%ADB%" %SEROPT% shell "echo '' >> %RPT%; echo '===== [4] RAM (free -m) =====' >> %RPT%; free -m >> %RPT% 2>&1; echo '--- /proc/meminfo ---' >> %RPT%; head -30 /proc/meminfo >> %RPT% 2>&1"

echo    - [5/11] 进程内存 TOP 20 / 负载
"%ADB%" %SEROPT% shell "echo '' >> %RPT%; echo '===== [5] TOP 20 PROCESSES BY RSS =====' >> %RPT%; timeout 60 top -b -n 1 -m 20 >> %RPT% 2>&1; echo '--- loadavg ---' >> %RPT%; cat /proc/loadavg >> %RPT% 2>&1"

echo    - [6/11] /data 各目录大小
"%ADB%" %SEROPT% shell "echo '' >> %RPT%; echo '===== [6] /data CHILDREN SIZE =====' >> %RPT%; timeout 120 du -sh /data/* 2>/dev/null >> %RPT%"

echo    - [7/11] /data/cabinet 大小明细
"%ADB%" %SEROPT% shell "echo '' >> %RPT%; echo '===== [7] /data/cabinet SIZE =====' >> %RPT%; timeout 120 du -sh /data/cabinet >> %RPT% 2>&1; echo '--- children ---' >> %RPT%; timeout 120 du -sh /data/cabinet/* 2>/dev/null >> %RPT%"

echo    - [8/11] /data/cabinet 最大 40 项
"%ADB%" %SEROPT% shell "echo '' >> %RPT%; echo '===== [8] /data/cabinet TOP 40 (KB) =====' >> %RPT%; timeout 120 du -ak /data/cabinet 2>/dev/null | sort -rn | head -40 >> %RPT%"

echo    - [9/11] 大于 10MB 的文件
"%ADB%" %SEROPT% shell "echo '' >> %RPT%; echo '===== [9] FILES LARGER THAN 10MB =====' >> %RPT%; timeout 120 find /data/cabinet -type f -size +10M -exec ls -lh {} \; 2>/dev/null >> %RPT%"

echo    - [10/11] /data/cabinet 完整文件列表
"%ADB%" %SEROPT% shell "echo '' >> %RPT%; echo '===== [10] RECURSIVE FILE LIST (ls -lRh) =====' >> %RPT%; timeout 180 ls -lRh /data/cabinet >> %RPT% 2>&1"

echo    - [11/11] 系统内存统计 dumpsys meminfo
"%ADB%" %SEROPT% shell "echo '' >> %RPT%; echo '===== [11] DUMPSYS MEMINFO (head 120) =====' >> %RPT%; timeout 60 dumpsys meminfo 2>/dev/null | head -120 >> %RPT%; echo '' >> %RPT%; echo '===== END OF REPORT =====' >> %RPT%"

REM ---------- 第 4 步：拉取报告（校验结束标记，失败自动重试一次） ----------
"%ADB%" %SEROPT% pull "%RPT%" "%OUT%" >nul 2>&1
findstr /C:"END OF REPORT" "%OUT%" >nul 2>&1
if errorlevel 1 (
    ping -n 3 127.0.0.1 >nul
    "%ADB%" %SEROPT% pull "%RPT%" "%OUT%" >nul 2>&1
)

set "RSIZE="
if exist "%OUT%" for %%A in ("%OUT%") do set "RSIZE=%%~zA"
if not defined RSIZE goto STREAM
if "%RSIZE%"=="0" goto STREAM
findstr /C:"END OF REPORT" "%OUT%" >nul 2>&1
if errorlevel 1 goto STREAM

echo.
if %RSIZE% LSS 500 (
    echo [警告] 报告只有 %RSIZE% 字节，设备可能无响应，内容仅供参考
)
echo ==================================================
echo   采集完成
echo   报告文件: %OUT%
echo ==================================================
"%ADB%" %SEROPT% shell "rm -f %RPT%" 2>nul
if "%OPEN_REPORT%"=="1" start "" "%OUT%"
echo.
pause
exit /b 0

REM ---------- 备用方案：设备端写不了文件时，直接输出到电脑 ----------
:STREAM
echo [警告] 设备端报告不完整，改为直接输出到电脑...
set "OUT=%~dp0mem_report_%SERIAL%_%STAMP%.txt"
(
    echo ===== [1] TIME / DEVICE =====
    "%ADB%" %SEROPT% shell "date; getprop ro.product.model; getprop ro.serialno; uname -a; uptime"
    echo ===== [2] DISK SPACE df -h =====
    "%ADB%" %SEROPT% shell "df -h; df -i"
    echo ===== [3] RAM free -m =====
    "%ADB%" %SEROPT% shell "free -m; head -30 /proc/meminfo"
    echo ===== [4] TOP 20 BY RSS =====
    "%ADB%" %SEROPT% shell "timeout 60 top -b -n 1 -m 20; cat /proc/loadavg"
    echo ===== [5] /data CHILDREN =====
    "%ADB%" %SEROPT% shell "timeout 120 du -sh /data/* 2>/dev/null"
    echo ===== [6] /data/cabinet CHILDREN =====
    "%ADB%" %SEROPT% shell "timeout 120 du -sh /data/cabinet/* 2>/dev/null"
    echo ===== [7] /data/cabinet TOP 40 (KB) =====
    "%ADB%" %SEROPT% shell "timeout 120 du -ak /data/cabinet 2>/dev/null | sort -rn | head -40"
    echo ===== [8] FILES >10MB =====
    "%ADB%" %SEROPT% shell "timeout 120 find /data/cabinet -type f -size +10M -exec ls -lh {} \; 2>/dev/null"
    echo ===== END OF REPORT =====
) > "%OUT%" 2>&1
echo [完成] 报告已生成: %OUT%
if "%OPEN_REPORT%"=="1" if exist "%OUT%" start "" "%OUT%"
echo.
pause
exit /b 0

REM ---------- adb 自动查找 ----------
:find_adb
if defined ADB exit /b
if exist "%~dp0platform-tools\adb.exe" if exist "%~dp0platform-tools\AdbWinApi.dll" (
    set "ADB=%~dp0platform-tools\adb.exe"
    exit /b
)
if exist "%~dp0adb.exe" if exist "%~dp0AdbWinApi.dll" (
    set "ADB=%~dp0adb.exe"
    exit /b
)
for /f "delims=" %%p in ('where adb 2^>nul') do (
    if not defined ADB if exist "%%~dppAdbWinApi.dll" set "ADB=%%p"
)
if defined ADB exit /b
for %%p in (
    "%LOCALAPPDATA%\Android\Sdk\platform-tools\adb.exe"
    "%USERPROFILE%\platform-tools\adb.exe"
) do (
    if not defined ADB if exist %%p if exist "%%~dppAdbWinApi.dll" set "ADB=%%~p"
)
exit /b

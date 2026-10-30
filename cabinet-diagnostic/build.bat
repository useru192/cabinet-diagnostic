@echo off
setlocal
cd /d "%~dp0."

REM The machine's GOOS/GOARCH may be pinned to linux/arm, so set the target here.
set "GOOS=windows"
set "GOARCH=amd64"

echo [1/3] go vet
go vet ./... || goto fail

echo [2/3] go test
go test ./... || goto fail

echo [3/3] build
go build -trimpath -ldflags "-s -w" -o cabinet-diag.exe . || goto fail

echo.
echo Build OK: cabinet-diag.exe
exit /b 0

:fail
echo.
echo Build FAILED
exit /b 1

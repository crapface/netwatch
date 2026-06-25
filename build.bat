@echo off
REM ============================================================
REM  NetWatch build script (cmd.exe)
REM  Produces a portable, self-contained Windows exe in .\dist
REM  Requirements: Go 1.22+ (https://go.dev/dl/). Nothing else.
REM ============================================================
setlocal
cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
    echo Go toolchain not found. Install Go 1.22+ from https://go.dev/dl/ and re-run.
    exit /b 1
)
for /f "delims=" %%v in ('go version') do echo Go: %%v

set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
set GOTOOLCHAIN=local

if not exist dist mkdir dist

echo Compiling (release, optimized)...
go build -trimpath -ldflags "-H windowsgui -s -w" -o dist\NetWatch.exe .
if errorlevel 1 (
    echo Build failed.
    exit /b 1
)

if exist README.md     copy /Y README.md     dist\ >nul
if exist README.es.md  copy /Y README.es.md  dist\ >nul
if exist TESTPLAN.txt  copy /Y TESTPLAN.txt   dist\ >nul

echo.
echo OK  -^>  dist\NetWatch.exe
echo Portable: copy the 'dist' folder anywhere and double-click NetWatch.exe.
echo No installer, no runtime, no admin rights required.
endlocal

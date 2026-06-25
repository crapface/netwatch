<#
  NetWatch build script (PowerShell)
  ----------------------------------
  Produces a portable, self-contained Windows executable in .\dist.

  Usage:   right-click -> "Run with PowerShell"
           or:  powershell -ExecutionPolicy Bypass -File build.ps1

  Requirements: Go 1.22+ (https://go.dev/dl/). Nothing else.
  The Windows manifest + icon are pre-baked into rsrc_windows_amd64.syso,
  so a plain `go build` yields a themed, DPI-aware, non-elevated exe.
#>

$ErrorActionPreference = "Stop"
Set-Location -Path $PSScriptRoot

Write-Host "=== NetWatch build ===" -ForegroundColor Cyan

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go toolchain not found. Install Go 1.22+ from https://go.dev/dl/ and re-run."
    exit 1
}
Write-Host ("Go: " + (go version))

$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"          # pure Go: no C compiler needed
$env:GOTOOLCHAIN = "local"

# Optional: regenerate the manifest/icon resource if go-winres is installed.
# Not required — rsrc_windows_amd64.syso is committed and picked up automatically.
if ((Get-Command go-winres -ErrorAction SilentlyContinue) -and (Test-Path "winres\winres.json")) {
    Write-Host "Regenerating Windows resource (manifest + icon)..."
    go-winres make --in "winres\winres.json" --arch amd64 --product-version 1.0.0 --file-version 1.0.0 --out rsrc
} else {
    Write-Host "Using committed rsrc_windows_amd64.syso (manifest + icon)."
}

New-Item -ItemType Directory -Force -Path "dist" | Out-Null

Write-Host "Compiling (release, optimized)..."
go build -trimpath -ldflags "-H windowsgui -s -w" -o "dist\NetWatch.exe" .
if ($LASTEXITCODE -ne 0) { Write-Error "Build failed."; exit 1 }

foreach ($f in @("README.md", "README.es.md", "TESTPLAN.txt")) {
    if (Test-Path $f) { Copy-Item $f "dist\" -Force }
}

$sizeMB = [math]::Round((Get-Item "dist\NetWatch.exe").Length / 1MB, 1)
Write-Host ""
Write-Host ("OK  ->  dist\NetWatch.exe  ($sizeMB MB)") -ForegroundColor Green
Write-Host "Portable: copy the 'dist' folder anywhere and double-click NetWatch.exe."
Write-Host "No installer, no runtime, no admin rights required."

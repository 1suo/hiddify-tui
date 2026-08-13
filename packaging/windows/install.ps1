#Requires -RunAsAdministrator
# Install hiddify-tui on Windows. Installs the client, and, if a standalone
# hiddify-core.exe is provided, registers it as a LocalSystem headless service.
param(
    [string]$BuildDir = ".",
    [string]$InstallDir = "$env:ProgramData\Hiddify",
    [string]$BinDir = "$env:ProgramFiles\hiddify-tui",
    [string]$ConfigPath = "$env:ProgramData\Hiddify\active-config.json"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path "$BuildDir\hiddify-tui.exe")) {
    throw "hiddify-tui.exe not found in $BuildDir (build with 'make build')"
}

New-Item -ItemType Directory -Force -Path $BinDir, $InstallDir | Out-Null
Copy-Item "$BuildDir\hiddify-tui.exe" "$BinDir\hiddify-tui.exe" -Force
if (Test-Path "$BuildDir\hiddify-migrate.exe") {
    Copy-Item "$BuildDir\hiddify-migrate.exe" "$BinDir\hiddify-migrate.exe" -Force
}

# Put the client on PATH.
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$BinDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$BinDir", "User")
}

# The core is bundled with the Hiddify GUI (serves Core gRPC at 127.0.0.1:17078).
# No standalone Windows core is published; only install the service when one is
# provided.
if (Test-Path "$BuildDir\hiddify-core.exe") {
    Copy-Item "$BuildDir\hiddify-core.exe" "$InstallDir\hiddify-core.exe" -Force
    if (Get-Service -Name "hiddify-core" -ErrorAction SilentlyContinue) {
        throw "hiddify-core service already exists; run uninstall.ps1 first."
    }
    $serviceArgs = @("run", "-c", $ConfigPath)
    New-Service -Name "hiddify-core" `
        -DisplayName "Hiddify Core" `
        -BinaryPathName "`"$InstallDir\hiddify-core.exe`" $serviceArgs" `
        -StartupType Automatic `
        -Description "Hiddify Core headless service" | Out-Null
    Write-Host "Installed hiddify-core service. Start it with: Start-Service hiddify-core"
} else {
    Write-Host "note: no standalone core for Windows; hiddify-tui connects to the Hiddify GUI's core on 127.0.0.1:17078"
}

Write-Host "Installed hiddify-tui to $BinDir"

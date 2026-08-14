#Requires -RunAsAdministrator
# Install hiddify-tui on Windows. If a standalone core is provided, a SYSTEM
# startup task runs it through the headless lifecycle wrapper.
param(
    [string]$BuildDir = ".",
    [string]$InstallDir = "$env:ProgramData\hiddify-tui",
    [string]$BinDir = "$env:ProgramFiles\hiddify-tui"
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
$pathEntries = @($userPath -split ";" | Where-Object { $_ })
if ($pathEntries -notcontains $BinDir) {
    $newPath = (@($pathEntries) + $BinDir) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
}

# The core is bundled with the Hiddify GUI (serves Core gRPC at 127.0.0.1:17078).
# No standalone Windows core is published; only install the service when one is
# provided.
if (Test-Path "$BuildDir\hiddify-core.exe") {
    if (-not (Test-Path "$BuildDir\hiddify-core-daemon.exe")) {
        throw "hiddify-core-daemon.exe is required with a standalone core"
    }
    Copy-Item "$BuildDir\hiddify-core.exe" "$InstallDir\hiddify-core.exe" -Force
    Copy-Item "$BuildDir\hiddify-core-daemon.exe" "$InstallDir\hiddify-core-daemon.exe" -Force
    if (Get-ScheduledTask -TaskName "hiddify-core" -ErrorAction SilentlyContinue) {
        throw "hiddify-core startup task already exists; run uninstall.ps1 first."
    }
    $action = New-ScheduledTaskAction `
        -Execute "$InstallDir\hiddify-core-daemon.exe" `
        -Argument "--core-binary `"$InstallDir\hiddify-core.exe`" --state-dir `"$InstallDir\state`""
    $trigger = New-ScheduledTaskTrigger -AtStartup
    $principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
    Register-ScheduledTask -TaskName "hiddify-core" -Action $action -Trigger $trigger -Principal $principal `
        -Description "Persistent headless Hiddify Core" | Out-Null
    $listener = Get-NetTCPConnection -LocalPort 17078 -State Listen -ErrorAction SilentlyContinue
    if ($listener) {
        Write-Host "Installed the startup task but did not start it: port 17078 is already in use."
        Write-Host "The existing VPN/core process was not interrupted."
    } else {
        Start-ScheduledTask -TaskName "hiddify-core"
        Write-Host "Installed and started the standalone headless core."
    }
} else {
    Write-Host "note: no standalone core for Windows; hiddify-tui connects to the Hiddify GUI's core on 127.0.0.1:17078"
}

Write-Host "Installed hiddify-tui to $BinDir"

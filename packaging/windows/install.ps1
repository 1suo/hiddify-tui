#Requires -RunAsAdministrator
# Install hiddify-tui and the official Windows core runtime. A SYSTEM startup
# task runs the DLL host through the headless lifecycle wrapper.
param(
    [string]$BuildDir = ".",
    [string]$InstallDir = "$env:ProgramData\hiddify-tui",
    [string]$BinDir = "$env:ProgramFiles\hiddify-tui"
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path "$BuildDir\hiddify-tui.exe")) {
    throw "hiddify-tui.exe not found in $BuildDir (build with 'make build')"
}
foreach ($file in @("hiddify-core-daemon.exe", "hiddify-core-host.exe", "hiddify-core.dll", "libcronet.dll")) {
    if (-not (Test-Path "$BuildDir\$file")) {
        throw "$file not found in $BuildDir"
    }
}
if (Get-ScheduledTask -TaskName "hiddify-core" -ErrorAction SilentlyContinue) {
    throw "hiddify-core startup task already exists; run uninstall.ps1 first."
}

New-Item -ItemType Directory -Force -Path $BinDir, $InstallDir | Out-Null
Copy-Item "$BuildDir\hiddify-tui.exe" "$BinDir\hiddify-tui.exe" -Force
if (Test-Path "$BuildDir\hiddify-migrate.exe") {
    Copy-Item "$BuildDir\hiddify-migrate.exe" "$BinDir\hiddify-migrate.exe" -Force
}
if (Test-Path "$BuildDir\packaging\windows\uninstall.ps1") {
    Copy-Item "$BuildDir\packaging\windows\uninstall.ps1" "$InstallDir\uninstall.ps1" -Force
}

# Put the client on PATH.
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$pathEntries = @($userPath -split ";" | Where-Object { $_ })
if ($pathEntries -notcontains $BinDir) {
    $newPath = (@($pathEntries) + $BinDir) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
}

foreach ($file in @("hiddify-core-daemon.exe", "hiddify-core-host.exe", "hiddify-core.dll", "libcronet.dll")) {
    Copy-Item "$BuildDir\$file" "$InstallDir\$file" -Force
}
$action = New-ScheduledTaskAction `
    -Execute "$InstallDir\hiddify-core-host.exe" `
    -Argument "serve -D `"$InstallDir\state`""
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

Write-Host "Installed hiddify-tui to $BinDir"

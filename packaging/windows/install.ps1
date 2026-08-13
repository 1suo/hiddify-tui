#Requires -RunAsAdministrator
# Install the Hiddify core daemon as a LocalSystem Windows service and the
# system-proxy agent as a per-user scheduled task.
param(
    [string]$InstallRoot = "$env:ProgramData\Hiddify",
    [string]$CoreBinary = "$InstallRoot\hiddify-core.exe",
    [string]$AgentBinary = "$InstallRoot\hiddify-agent.exe",
    [string]$StateDir = "$env:ProgramData\Hiddify\state",
    [string]$PipeName = "\\.\pipe\hiddify-control",
    [string]$DesignatedUser = ""
)

$ErrorActionPreference = "Stop"

if ($DesignatedUser -eq "") {
    throw "DesignatedUser is required; it is the single desktop user allowed to control the daemon."
}

New-Item -ItemType Directory -Force -Path $InstallRoot, $StateDir | Out-Null
New-Item -ItemType Directory -Force -Path "$env:ProgramData\Hiddify\runtime" | Out-Null

if (-not (Test-Path $CoreBinary)) {
    throw "Core binary not found at $CoreBinary"
}

# LocalSystem automatic service for the always-running daemon.
$serviceArgs = @("daemon", "run",
    "--state-dir=$StateDir",
    "--pipe=$PipeName",
    "--designated-user=$DesignatedUser")
if (Get-Service -Name "hiddify-core" -ErrorAction SilentlyContinue) {
    throw "hiddify-core service already exists; run uninstall.ps1 first."
}
$service = New-Service -Name "hiddify-core" `
    -DisplayName "Hiddify Core daemon" `
    -BinaryPathName "`"$CoreBinary`" $serviceArgs" `
    -StartupType Automatic `
    -Description "Hiddify Core local control daemon"
$service | Set-Service -StartupType Automatic

# Per-user scheduled startup task for the system-proxy agent.
$taskArgs = "--socket=$PipeName"
$action = New-ScheduledTaskAction -Execute $AgentBinary -Argument $taskArgs
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $DesignatedUser
$settings = New-ScheduledTaskSettingsSet -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
Register-ScheduledTask -TaskName "hiddify-agent" `
    -Action $action -Trigger $trigger -Settings $settings `
    -User $DesignatedUser -RunLevel Limited | Out-Null

Write-Host "Installed hiddify-core service and hiddify-agent task for $DesignatedUser."
Write-Host "Start the service with: Start-Service hiddify-core"

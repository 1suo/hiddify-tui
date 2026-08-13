#Requires -RunAsAdministrator
# Remove the Hiddify core daemon service and the system-proxy agent task.
# Profiles are preserved unless -Purge is supplied.
param(
    [switch]$Purge,
    [string]$StateDir = "$env:ProgramData\Hiddify\state"
)

$ErrorActionPreference = "Stop"

# Restore the user's proxy state through the agent before removing anything.
$agent = "$env:ProgramData\Hiddify\hiddify-agent.exe"
if (Test-Path $agent) {
    & $agent --restore 2>$null
}

if (Get-Service -Name "hiddify-core" -ErrorAction SilentlyContinue) {
    Stop-Service -Name "hiddify-core" -Force -ErrorAction SilentlyContinue
    sc.exe delete "hiddify-core" | Out-Null
}

if (Get-ScheduledTask -TaskName "hiddify-agent" -ErrorAction SilentlyContinue) {
    Unregister-ScheduledTask -TaskName "hiddify-agent" -Confirm:$false
}

if ($Purge) {
    Write-Host "Purging state directory: $StateDir"
    Remove-Item -Recurse -Force -Path $StateDir -ErrorAction SilentlyContinue
} else {
    Write-Host "Profiles preserved. Use -Purge to remove $StateDir."
}

Write-Host "Uninstall complete."

#Requires -RunAsAdministrator
# Remove the Hiddify core headless startup task (and legacy service).
param(
    [switch]$Purge,
    [string]$InstallDir = "$env:ProgramData\hiddify-tui",
    [string]$BinDir = "$env:ProgramFiles\hiddify-tui"
)

$ErrorActionPreference = "Stop"

if (Get-ScheduledTask -TaskName "hiddify-core" -ErrorAction SilentlyContinue) {
    Stop-ScheduledTask -TaskName "hiddify-core" -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName "hiddify-core" -Confirm:$false
}

# Remove services created by older installer versions.
if (Get-Service -Name "hiddify-core" -ErrorAction SilentlyContinue) {
    Stop-Service -Name "hiddify-core" -Force -ErrorAction SilentlyContinue
    sc.exe delete "hiddify-core" | Out-Null
}

Remove-Item -Force -Path "$BinDir\hiddify-tui.exe", "$BinDir\hiddify-migrate.exe" `
    -ErrorAction SilentlyContinue
Remove-Item -Force -Path "$InstallDir\hiddify-core.exe", "$InstallDir\hiddify-core-daemon.exe" `
    -ErrorAction SilentlyContinue
if (Test-Path $BinDir) {
    Remove-Item -Path $BinDir -ErrorAction SilentlyContinue
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$pathEntries = @($userPath -split ";" | Where-Object { $_ -and $_ -ne $BinDir })
[Environment]::SetEnvironmentVariable("Path", ($pathEntries -join ";"), "User")

if ($Purge) {
    Write-Host "Purging state directory: $InstallDir"
    Remove-Item -Recurse -Force -Path $InstallDir -ErrorAction SilentlyContinue
}

Write-Host "Uninstall complete."

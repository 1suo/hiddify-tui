#Requires -RunAsAdministrator
# Remove the Hiddify core headless service.
param(
    [switch]$Purge,
    [string]$StateDir = "$env:ProgramData\Hiddify"
)

$ErrorActionPreference = "Stop"

if (Get-Service -Name "hiddify-core" -ErrorAction SilentlyContinue) {
    Stop-Service -Name "hiddify-core" -Force -ErrorAction SilentlyContinue
    sc.exe delete "hiddify-core" | Out-Null
}

if ($Purge) {
    Write-Host "Purging state directory: $StateDir"
    Remove-Item -Recurse -Force -Path $StateDir -ErrorAction SilentlyContinue
}

Write-Host "Uninstall complete."

#Requires -RunAsAdministrator
# Install the Hiddify core as a LocalSystem headless service.
param(
    [string]$InstallRoot = "$env:ProgramData\Hiddify",
    [string]$CoreBinary = "$InstallRoot\hiddify-core.exe",
    [string]$ConfigPath = "$env:ProgramData\Hiddify\active-config.json"
)

$ErrorActionPreference = "Stop"

New-Item -ItemType Directory -Force -Path $InstallRoot | Out-Null

if (-not (Test-Path $CoreBinary)) {
    throw "Core binary not found at $CoreBinary"
}

if (Get-Service -Name "hiddify-core" -ErrorAction SilentlyContinue) {
    throw "hiddify-core service already exists; run uninstall.ps1 first."
}

$serviceArgs = @("run", "-c", $ConfigPath)
$service = New-Service -Name "hiddify-core" `
    -DisplayName "Hiddify Core" `
    -BinaryPathName "`"$CoreBinary`" $serviceArgs" `
    -StartupType Automatic `
    -Description "Hiddify Core headless service"
$service | Set-Service -StartupType Automatic

Write-Host "Installed hiddify-core service. Start it with: Start-Service hiddify-core"

# Install the latest hiddify-tui release and official Windows core runtime.
# Pin a release with $env:HIDDIFY_TUI_VERSION = "vX.Y.Z".

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "install: run PowerShell as Administrator"
}
if ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne [Runtime.InteropServices.Architecture]::X64) {
    throw "install: the official Windows core currently supports x64 only"
}
if (-not (Get-Command tar.exe -ErrorAction SilentlyContinue)) {
    throw "install: required command not found: tar.exe"
}

$tag = $env:HIDDIFY_TUI_VERSION
if (-not $tag) {
    $latest = Invoke-RestMethod -Uri "https://api.github.com/repos/1suo/hiddify-tui/releases/latest"
    $tag = $latest.tag_name
}
if (-not $tag.StartsWith("v")) {
    $tag = "v$tag"
}
if ($tag -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$') {
    throw "install: invalid release version: $tag"
}

$version = $tag.Substring(1)
$archive = "hiddify-tui_${version}_windows_amd64.zip"
$hostArchive = "hiddify-core-host_${version}_windows_amd64.zip"
$releaseUrl = "https://github.com/1suo/hiddify-tui/releases/download/$tag"
$coreVersion = "v4.1.0"
$coreArchive = "hiddify-lib-windows-amd64.tar.gz"
$coreUrl = "https://github.com/hiddify/hiddify-core/releases/download/$coreVersion/$coreArchive"
$coreSHA256 = "fc610e67d9fdf23da7cc10633f38806d7081a1c120157d1fbe5ac8cc41b315b4"
$tempDir = Join-Path ([IO.Path]::GetTempPath()) "hiddify-tui-install-$([guid]::NewGuid())"

New-Item -ItemType Directory -Path $tempDir | Out-Null
try {
    Write-Host "Downloading hiddify-tui $tag and hiddify-core $coreVersion (x64)"
    Invoke-WebRequest -Uri "$releaseUrl/$archive" -OutFile "$tempDir\$archive"
    Invoke-WebRequest -Uri "$releaseUrl/$hostArchive" -OutFile "$tempDir\$hostArchive"
    Invoke-WebRequest -Uri "$releaseUrl/checksums.txt" -OutFile "$tempDir\checksums.txt"
    Invoke-WebRequest -Uri $coreUrl -OutFile "$tempDir\$coreArchive"

    foreach ($releaseArchive in @($archive, $hostArchive)) {
        $checksumLine = Get-Content "$tempDir\checksums.txt" | Where-Object { $_ -match "\s$([regex]::Escape($releaseArchive))$" } | Select-Object -First 1
        if (-not $checksumLine) {
            throw "install: $releaseArchive is missing from checksums.txt"
        }
        $expected = ($checksumLine -split '\s+')[0].ToLowerInvariant()
        $actual = (Get-FileHash "$tempDir\$releaseArchive" -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -ne $expected) {
            throw "install: release checksum verification failed for $releaseArchive"
        }
    }
    $actualCore = (Get-FileHash "$tempDir\$coreArchive" -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualCore -ne $coreSHA256) {
        throw "install: core checksum verification failed"
    }

    $buildDir = "$tempDir\build"
    New-Item -ItemType Directory -Path $buildDir | Out-Null
    Expand-Archive -Path "$tempDir\$archive" -DestinationPath $buildDir
    Expand-Archive -Path "$tempDir\$hostArchive" -DestinationPath $buildDir
    & tar.exe -xzf "$tempDir\$coreArchive" -C $buildDir
    if ($LASTEXITCODE -ne 0) {
        throw "install: failed to extract the core runtime"
    }
    $innerInstaller = "$buildDir\packaging\windows\install.ps1"
    if (-not (Test-Path $innerInstaller)) {
        throw "install: Windows installer is missing from the release archive"
    }
    Unblock-File $innerInstaller
    & $innerInstaller -BuildDir $buildDir
} finally {
    Remove-Item -Recurse -Force -Path $tempDir -ErrorAction SilentlyContinue
}

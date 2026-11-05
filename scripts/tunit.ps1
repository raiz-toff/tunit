#Requires -Version 5.0
<#
.SYNOPSIS
  Install tunit — universal system diagnostic, stress, and load testing tool.
.DESCRIPTION
  Downloads the tunit binary for Windows and installs it to C:\Program Files\tunit.
  Usage: iex (iwr https://github.com/raiz-toff/tunit/releases/latest/download/tunit.ps1).Content
#>

$ErrorActionPreference = 'Stop'

$repo    = 'raiz-toff/tunit'
$asset   = 'tunit-windows-amd64.exe'
$version = if ($env:TUNIT_VERSION) { $env:TUNIT_VERSION } else { 'latest' }
$installDir = Join-Path $env:ProgramFiles 'tunit'

if ($version -eq 'latest') {
    $url = "https://github.com/$repo/releases/latest/download/$asset"
} else {
    $url = "https://github.com/$repo/releases/download/$version/$asset"
}

Write-Host "Downloading tunit $version for Windows/amd64..." -ForegroundColor Cyan
$tmp = Join-Path $env:TEMP "tunit-install-$([guid]::NewGuid().ToString('N').Substring(0,8)).exe"

try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing
} catch {
    Write-Host "Download failed: $_" -ForegroundColor Red
    Write-Host "URL: $url" -ForegroundColor Yellow
    exit 1
}

# Create install directory and move binary
if (-not (Test-Path $installDir)) {
    try {
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    } catch {
        Write-Host "Cannot create $installDir — try running as Administrator." -ForegroundColor Red
        exit 1
    }
}

$dest = Join-Path $installDir 'tunit.exe'
Move-Item -Path $tmp -Destination $dest -Force

# Add to PATH if not already there
$machinePath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
if ($machinePath -notlike "*$installDir*") {
    try {
        [Environment]::SetEnvironmentVariable('Path', "$machinePath;$installDir", 'Machine')
        $env:Path = "$env:Path;$installDir"
        Write-Host "Added $installDir to system PATH." -ForegroundColor Green
    } catch {
        Write-Host "Could not add to PATH — try running as Administrator, or add manually:" -ForegroundColor Yellow
        Write-Host "  $installDir" -ForegroundColor Yellow
    }
}

Write-Host "Installed: $dest" -ForegroundColor Green
Write-Host "Run: tunit" -ForegroundColor Cyan

#Requires -Version 5.0
<#
.SYNOPSIS
  tunit — Universal system diagnostic, stress, and load testing tool
.DESCRIPTION
  Downloads the tunit binary for Windows and runs it.
  Usage: irm https://github.com/raiz-toff/tunit/releases/latest/download/tunit.ps1 | iex
#>

$ErrorActionPreference = 'Stop'
$repo    = 'raiz-toff/tunit'
$asset   = 'tunit-windows-amd64.exe'
$version = if ($env:TUNIT_VERSION) { $env:TUNIT_VERSION } else { 'latest' }
$dest    = Join-Path $env:TEMP 'tunit.exe'

if ($version -eq 'latest') {
    $url = "https://github.com/$repo/releases/latest/download/$asset"
} else {
    $url = "https://github.com/$repo/releases/download/$version/$asset"
}

Write-Host "Downloading tunit $version for Windows/amd64..." -ForegroundColor Cyan
Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing

# Self-elevate to Administrator if needed
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "Requesting administrator privileges..." -ForegroundColor Yellow
    Start-Process powershell.exe -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$dest`"" -Verb RunAs
    exit
}

& $dest @args

#Requires -Version 5.1
<#
.SYNOPSIS
  Install yunxiao CLI from GitHub Releases onto Windows.

.DESCRIPTION
  Downloads the latest (or -Version) windows-amd64 zip from
  xiaoxiaolin0918/yunxiao-cli releases, extracts yunxiao.exe (and
  skills/ + profiles/ when present) into %USERPROFILE%\.local\bin,
  and prepends that directory to the user PATH when missing.

.PARAMETER Version
  Release version without leading v (e.g. 0.16.39). Default: latest.

.PARAMETER Dir
  Install directory. Default: $env:USERPROFILE\.local\bin

.PARAMETER SkipPath
  Do not modify user PATH.
#>
[CmdletBinding()]
param(
  [string]$Version = "",
  [string]$Dir = "",
  [switch]$SkipPath
)

$ErrorActionPreference = "Stop"
$Repo = "xiaoxiaolin0918/yunxiao-cli"

if (-not $Dir) {
  $Dir = Join-Path $env:USERPROFILE ".local\bin"
}
New-Item -ItemType Directory -Force -Path $Dir | Out-Null

function Get-LatestTag {
  $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{
    "Accept" = "application/vnd.github+json"
    "User-Agent" = "yunxiao-install-ps1"
  }
  return $rel.tag_name
}

if (-not $Version) {
  $tag = Get-LatestTag
  $Version = $tag.TrimStart("v")
  Write-Host "Latest release: $tag"
} else {
  $Version = $Version.TrimStart("v")
  Write-Host "Installing version: $Version"
}

$asset = "yunxiao-cli-$Version-windows-amd64.zip"
$urls = @(
  "https://github.com/$Repo/releases/download/v$Version/$asset",
  "https://ghproxy.net/https://github.com/$Repo/releases/download/v$Version/$asset"
)

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("yunxiao-install-" + [guid]::NewGuid().ToString("n"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$zipPath = Join-Path $tmp $asset

try {
  $ok = $false
  foreach ($u in $urls) {
    try {
      Write-Host "Downloading $u"
      Invoke-WebRequest -Uri $u -OutFile $zipPath -UseBasicParsing
      if ((Get-Item $zipPath).Length -gt 0) { $ok = $true; break }
    } catch {
      Write-Warning "download failed: $u ($_)"
    }
  }
  if (-not $ok) { throw "failed to download $asset" }

  $extract = Join-Path $tmp "extract"
  Expand-Archive -Path $zipPath -DestinationPath $extract -Force

  $exe = Get-ChildItem -Path $extract -Recurse -Filter "yunxiao.exe" | Select-Object -First 1
  if (-not $exe) { throw "yunxiao.exe not found in archive" }

  $stageRoot = $exe.Directory.FullName
  Copy-Item -Force $exe.FullName (Join-Path $Dir "yunxiao.exe")

  foreach ($side in @("skills", "profiles")) {
    $src = Join-Path $stageRoot $side
    if (Test-Path $src) {
      $dst = Join-Path $Dir $side
      if (Test-Path $dst) { Remove-Item -Recurse -Force $dst }
      Copy-Item -Recurse -Force $src $dst
      Write-Host "Installed $side/ -> $dst"
    }
  }

  if (-not $SkipPath) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $parts = @()
    if ($userPath) { $parts = $userPath -split ";" | Where-Object { $_ -ne "" } }
    $normDir = [System.IO.Path]::GetFullPath($Dir).TrimEnd("\")
    $has = $false
    foreach ($p in $parts) {
      try {
        if ([System.IO.Path]::GetFullPath($p).TrimEnd("\") -ieq $normDir) { $has = $true; break }
      } catch {}
    }
    if (-not $has) {
      $newPath = if ($parts.Count -gt 0) { ($normDir + ";" + ($parts -join ";")) } else { $normDir }
      [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
      Write-Host "Prepended to user PATH: $normDir (open a new shell to pick it up)"
    } else {
      Write-Host "User PATH already contains $normDir"
    }
  }

  $installed = Join-Path $Dir "yunxiao.exe"
  & $installed --version
  Write-Host "OK: $installed"
  Write-Host "Tip: later upgrades -> yunxiao update --yes"
}
finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
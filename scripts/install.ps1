<#
.SYNOPSIS
Installs the built docker-htop plugin for the current Windows user and for a WSL distribution.

.DESCRIPTION
Copies dist/windows-amd64/docker-htop.exe to %USERPROFILE%\.docker\cli-plugins and runs
scripts/install.sh inside the WSL distribution, which copies dist/linux-amd64/docker-htop to
~/.docker/cli-plugins there. Run scripts/build.ps1 first. Close a running `docker htop` before
installing over it.

.PARAMETER Distro
WSL distribution to install into.

.PARAMETER SkipWsl
Install for Windows only.
#>
[CmdletBinding()]
param(
    [string]$Distro = 'Ubuntu',
    [switch]$SkipWsl
)

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$source = Join-Path $root 'dist\windows-amd64\docker-htop.exe'
if (-not (Test-Path $source)) {
    throw "$source is missing; run scripts\build.ps1 first"
}

$pluginDir = Join-Path $env:USERPROFILE '.docker\cli-plugins'
New-Item -ItemType Directory -Force $pluginDir | Out-Null
Copy-Item $source (Join-Path $pluginDir 'docker-htop.exe') -Force
Write-Host "installed $(Join-Path $pluginDir 'docker-htop.exe')"

if (-not $SkipWsl) {
    & wsl.exe -d $Distro --cd $root -- sh scripts/install.sh
    if ($LASTEXITCODE -ne 0) {
        throw "install in WSL distribution '$Distro' failed"
    }
}

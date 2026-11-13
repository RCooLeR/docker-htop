<#
.SYNOPSIS
Builds the docker-htop plugin for Linux (amd64, arm64) and Windows (amd64).

.DESCRIPTION
For each architecture, compiles the sampler into internal/samplerbin/bin, where the plugin
embeds it, then builds the plugin itself into dist/linux-arm64, dist/linux-amd64 and
dist/windows-amd64. Needs Go 1.27 or newer on PATH; nothing is installed.

.PARAMETER Version
Version string stamped into the binaries.
#>
[CmdletBinding()]
param(
    [string]$Version = 'dev'
)

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$ldflags = "-s -w -X github.com/RCooLeR/docker-htop/internal/buildinfo.Version=$Version"
$saved = @{ GOOS = $env:GOOS; GOARCH = $env:GOARCH; CGO_ENABLED = $env:CGO_ENABLED }

function Invoke-GoBuild {
    param([string]$Os, [string]$Arch, [string]$Output, [string]$Package)

    $env:GOOS = $Os
    $env:GOARCH = $Arch
    & go build -trimpath -ldflags $ldflags -o $Output $Package
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed for $Package ($Os/$Arch)"
    }
    Write-Host "built $Output"
}

Push-Location $root
try {
    $env:CGO_ENABLED = '0'
    $sampler = 'internal/samplerbin/bin/docker-htop-sampler'
    # A plugin embeds whatever sampler is in internal/samplerbin/bin, so each group builds the
    # sampler for its architecture first. The amd64 group comes last and leaves its sampler in
    # place for local builds and tests.
    Invoke-GoBuild -Os linux -Arch arm64 -Output $sampler -Package './cmd/docker-htop-sampler'
    Invoke-GoBuild -Os linux -Arch arm64 -Output 'dist/linux-arm64/docker-htop' -Package './cmd/docker-htop'

    Invoke-GoBuild -Os linux -Arch amd64 -Output $sampler -Package './cmd/docker-htop-sampler'
    Invoke-GoBuild -Os linux -Arch amd64 -Output 'dist/linux-amd64/docker-htop' -Package './cmd/docker-htop'
    Invoke-GoBuild -Os windows -Arch amd64 -Output 'dist/windows-amd64/docker-htop.exe' -Package './cmd/docker-htop'
}
finally {
    foreach ($name in $saved.Keys) {
        if ($null -eq $saved[$name]) {
            Remove-Item "Env:$name" -ErrorAction SilentlyContinue
        }
        else {
            Set-Item "Env:$name" $saved[$name]
        }
    }
    Pop-Location
}

#!/bin/sh
# Builds the docker-htop plugin for Linux (amd64, arm64) and Windows (amd64) on a machine with
# Go 1.27 or newer. The counterpart of scripts/build.ps1 for Linux and macOS hosts.
#
# Usage: scripts/build.sh [version]
set -eu

version=${1:-dev}
root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

ldflags="-s -w -X github.com/RCooLeR/docker-htop/internal/buildinfo.Version=$version"
sampler=internal/samplerbin/bin/docker-htop-sampler

# build <os> <arch> <output> <package>
build() {
    CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -trimpath -ldflags "$ldflags" -o "$3" "$4"
    echo "built $3"
}

# A plugin embeds whatever sampler is in internal/samplerbin/bin, so each group builds the
# sampler for its architecture first. The amd64 group comes last and leaves its sampler in
# place for local builds and tests.
build linux arm64 "$sampler" ./cmd/docker-htop-sampler
build linux arm64 dist/linux-arm64/docker-htop ./cmd/docker-htop

build linux amd64 "$sampler" ./cmd/docker-htop-sampler
build linux amd64 dist/linux-amd64/docker-htop ./cmd/docker-htop
build windows amd64 dist/windows-amd64/docker-htop.exe ./cmd/docker-htop

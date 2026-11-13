#!/bin/sh
# Installs the Linux build of the docker-htop plugin for the current user.
# Build it first with scripts/build.sh, or with scripts/build.ps1 on Windows (Go 1.27 or newer).
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)

machine=$(uname -m)
case "$machine" in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *)
        echo "no build for machine type $machine" >&2
        exit 1
        ;;
esac

source="$root/dist/linux-$arch/docker-htop"
target="${DOCKER_CONFIG:-$HOME/.docker}/cli-plugins"

if [ ! -f "$source" ]; then
    echo "$source is missing; run scripts/build.sh (or scripts/build.ps1 on Windows) first" >&2
    exit 1
fi

mkdir -p "$target"
install -m 0755 "$source" "$target/docker-htop"
echo "installed $target/docker-htop"

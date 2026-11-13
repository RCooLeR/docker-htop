# Installation

The plugin is one executable named `docker-htop` (`docker-htop.exe` on Windows) in a Docker CLI
plugin directory. There is no service and no configuration file.

## Requirements

- The Docker CLI with plugin support and an engine with Linux containers. The plugin uses Engine
  API features from version 1.41, so Docker 20.10 or newer.
- Linux on x86-64 or ARM64, or Windows on x86-64.
- Access to the engine: `docker ps` must work for your user without extra steps.

## Get the binary

### Download a release

Each [release](https://github.com/RCooLeR/docker-htop/releases) carries:

| File | For |
|---|---|
| `docker-htop-linux-amd64` | Linux and WSL on x86-64 (`uname -m` says `x86_64`) |
| `docker-htop-linux-arm64` | Linux on ARM64 (`uname -m` says `aarch64`) |
| `docker-htop-windows-amd64.exe` | Windows on x86-64 |
| `checksums.txt` | SHA-256 of the files above |

On Linux, download the file for your machine and check it:

```sh
curl -fsSLO https://github.com/RCooLeR/docker-htop/releases/latest/download/docker-htop-linux-amd64
curl -fsSLO https://github.com/RCooLeR/docker-htop/releases/latest/download/checksums.txt
sha256sum --check --ignore-missing checksums.txt
```

### Build from source

Needs Go 1.27 or newer; nothing else is installed or downloaded apart from Go modules. One build
produces every target, so build on whichever machine has Go and copy the result where you need
it. Run from the repository root.

On Linux or macOS:

```sh
sh scripts/build.sh            # optional version: sh scripts/build.sh 1.2.3
```

On Windows:

```powershell
.\scripts\build.ps1            # optional version: .\scripts\build.ps1 -Version 1.2.3
```

Both write `dist/linux-amd64/docker-htop`, `dist/linux-arm64/docker-htop` and
`dist/windows-amd64/docker-htop.exe`.

The build compiles the sampler for each architecture first and embeds it in the plugin. A plain
`go build ./cmd/docker-htop` or `go install github.com/RCooLeR/docker-htop/cmd/docker-htop@latest`
skips that step: the result works wherever the plugin can read `/proc` itself or falls back to
the Engine API, but it cannot start helper containers (see [Collected data](collected-data.md#modes)).

## Linux

The commands below use the downloaded file name; with a source build use
`dist/linux-amd64/docker-htop` or `dist/linux-arm64/docker-htop` instead.

For your own user:

```sh
install -D -m 0755 docker-htop-linux-amd64 ~/.docker/cli-plugins/docker-htop
```

For all users, or when you run Docker through `sudo`:

```sh
sudo install -D -m 0755 docker-htop-linux-amd64 /usr/local/lib/docker/cli-plugins/docker-htop
```

From a checkout that holds a built `dist/`, `sh scripts/install.sh` installs the right
architecture for your own user.

The file is statically linked and needs nothing else, so a machine without internet access or
without the repository only needs that one file copied over:

```sh
scp docker-htop-linux-amd64 server:/tmp/
ssh server 'sudo install -D -m 0755 /tmp/docker-htop-linux-amd64 /usr/local/lib/docker/cli-plugins/docker-htop'
```

With Docker Engine on the same machine the plugin reads `/proc` directly and starts no helper
container; the header then shows `Source: local /proc` and the CPU, memory and load of that host
itself.

## WSL

This section is for Docker Desktop with WSL integration switched on for your distribution
(Docker Desktop, Settings, Resources, WSL integration). If you installed Docker Engine inside the
distribution instead, follow [Linux](#linux).

The plugin has to be installed inside the distribution: the `docker` command there does not see
plugins installed for Windows. In a shell inside the distribution, download the Linux file as
above and install it for your user:

```sh
install -D -m 0755 docker-htop-linux-amd64 ~/.docker/cli-plugins/docker-htop
```

From a source build on Windows, with the repository on a Windows drive:

```powershell
.\scripts\build.ps1
.\scripts\install.ps1                  # Windows and the distribution "Ubuntu"
.\scripts\install.ps1 -Distro Debian   # another distribution
```

Notes:

- Keep the plugin in `~/.docker/cli-plugins`. Docker Desktop fills
  `/usr/local/lib/docker/cli-plugins` with its own plugins and manages that directory.
- The CPU, memory and load in the header describe the WSL 2 virtual machine that Docker Desktop
  shares with your distributions, not Windows as a whole.
- Running `docker htop` in Windows and in WSL at the same time is fine; each instance starts its
  own helper container.

## Windows

Download the plugin into your Docker plugin folder:

```powershell
New-Item -ItemType Directory -Force "$env:USERPROFILE\.docker\cli-plugins" | Out-Null
Invoke-WebRequest https://github.com/RCooLeR/docker-htop/releases/latest/download/docker-htop-windows-amd64.exe -OutFile "$env:USERPROFILE\.docker\cli-plugins\docker-htop.exe"
```

From a source build, `.\scripts\install.ps1 -SkipWsl` copies `dist\windows-amd64\docker-htop.exe`
to the same place.

The plugin connects through the current Docker context, which is Docker Desktop's named pipe by
default. Use Windows Terminal; the Docker Desktop engine must be in Linux containers mode. Close
a running `docker htop` before installing over it.

## Check

```sh
docker htop --version
docker htop --no-stream
```

`docker info` lists the plugin under the client's plugins. A file that is in a plugin directory
but does not work is listed there under invalid plugins, with the reason.

## Update

Replace the file with the new version. The plugin replaces its helper image by itself when the
embedded sampler changed.

## Uninstall

Delete the plugin file, then the helper image if one was created:

```sh
rm ~/.docker/cli-plugins/docker-htop
docker images docker-htop-sampler      # shows the tag to remove, if any
docker image rm docker-htop-sampler:<tag>
```

On Windows the file is `%USERPROFILE%\.docker\cli-plugins\docker-htop.exe`. The plugin keeps no
other state.

## Troubleshooting

| What you see | Likely cause |
|---|---|
| `docker: 'htop' is not a docker command` | The file is not in a plugin directory of the user running `docker`, is not executable, or is not named `docker-htop`. With `sudo docker`, install for all users. |
| `exec format error`, or the plugin is listed as invalid | The file is for another architecture; compare `uname -m` with the file name. |
| `permission denied` on the Docker socket | Your user may not use the engine; the plugin needs the same access as `docker ps`. |
| The header says `Source: Engine API` with a reason | The plugin could not read `/proc` itself and could not use the helper container. The reason is shown: started with `--no-sampler`, built without the sampler, an engine of another architecture than the plugin, or an engine that refuses containers with the host PID namespace (for example Docker Desktop's Enhanced Container Isolation). |
| Function keys do nothing | The terminal keeps them for itself. Every function key has a letter alternative, see [Usage](usage.md#keys). |
| On Windows `docker htop`, `docker --help` or `docker info` hangs right after installing | Security software can hold a new executable when it starts, and the Docker CLI starts every plugin to list it. Remove `docker-htop.exe` from `cli-plugins` to get the CLI back, then allow the file in your security software. |
| `TLS connections to the engine are not supported` | The engine is reached over TLS or `ssh://`, see [Configuration](configuration.md#choosing-the-engine). |

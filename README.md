# docker htop

A Docker CLI plugin that shows containers and their processes in one live terminal view: the
per-container numbers of `docker stats` (CPU, memory against the limit, network) together with
the per-process rows and per-core bars of htop. There are builds for Linux (x86-64 and ARM64,
including WSL) and Windows, for engines with Linux containers; see
[what has been tested](docs/index.md#tested-with).

```
 0[||||||          38.2%]  4[||||            22.7%]  8[|||             18.9%]
 1[||              12.5%]  5[|                9.3%]  9[|                6.2%]
Mem[||||||||||||||||||||||||            9.8G/15.6G]   Swp[|                    0.3G/4.0G]
Containers: 21 running, 1 paused, 1 exited   Tasks: 154, 412 thr   Load: 1.84 1.32 0.97
Name / PID            User       CPU%         Mem   Net rx/tx  Tasks  Command / status
▾ homelab  compose, 4 containers
  ▾ postgres          postgres   61.6   470M/2.0G    96K/1.4M      7  Up 3 days (healthy)
      ├─ 2231         postgres    0.3        142M                  1  postgres -D /var/lib/postgresql/data
      └─ 2304         postgres   61.2        310M                  1  postgres: autovacuum worker app
  ▸ mosquitto         1883        0.4    12M/256M   310K/290K      1  Up 3 days
```

## Install

Needs Docker 20.10 or newer. Download the file for your machine from the
[releases](https://github.com/RCooLeR/docker-htop/releases) and put it into a Docker CLI plugin
folder under the name `docker-htop`.

Linux and WSL (x86-64; use `docker-htop-linux-arm64` on ARM64):

```sh
curl -fsSLO https://github.com/RCooLeR/docker-htop/releases/latest/download/docker-htop-linux-amd64
install -D -m 0755 docker-htop-linux-amd64 ~/.docker/cli-plugins/docker-htop
```

Windows:

```powershell
New-Item -ItemType Directory -Force "$env:USERPROFILE\.docker\cli-plugins" | Out-Null
Invoke-WebRequest https://github.com/RCooLeR/docker-htop/releases/latest/download/docker-htop-windows-amd64.exe -OutFile "$env:USERPROFILE\.docker\cli-plugins\docker-htop.exe"
```

Checksums, installing for all users, building from source, updating and removing:
[docs/installation.md](docs/installation.md).

## Use

```sh
docker htop              # q quits, F1 or ? shows the keys
docker htop --no-stream  # one reading as text
```

Arrows move, Enter folds a container, F6 changes the sort order, F4 filters, F7 follows logs,
F8 opens a shell, F9 sends a signal after asking.

## Documentation

- [Installation](docs/installation.md): Linux, WSL, Windows, troubleshooting
- [Usage](docs/usage.md): the screen and the keys
- [Configuration](docs/configuration.md): flags, environment variables, choosing the engine
- [Collected data](docs/collected-data.md): where the numbers come from and what they mean
- [Security](docs/security.md): the helper container and what the plugin can change
- [Development](docs/development.md): layout, tests, design choices

# docker htop documentation

| Page | What it covers |
|---|---|
| [Installation](installation.md) | Building, then installing on Linux, in WSL and on Windows; updating, removing, troubleshooting |
| [Usage](usage.md) | The screen, the keys, logs, shell and signals, one-shot text output |
| [Configuration](configuration.md) | Flags, environment variables, choosing the engine |
| [Collected data](collected-data.md) | Where each number comes from and what it means; the two data sources |
| [Security](security.md) | The helper containers, their privileges, and what the plugin can change |
| [Development](development.md) | Code layout, checks, tests, design choices |

## Tested with

- Docker Desktop 29 on Windows 11, used from WSL 2 (Ubuntu) with the Linux build of the plugin
  (sampler mode and Engine API mode).
- Local mode, by running the Linux build inside a container that shares the PID namespace of the
  Docker Desktop virtual machine.

- The automated tests run on Linux and on Windows for every change.

Expected to work but not verified on a real machine yet: the Windows build, the ARM64 build,
Docker Engine installed directly on Linux, rootless Docker, Podman's Docker-compatible socket, and
hosts with SELinux enforcing.

Each plugin binary carries the helper for its own architecture, so helper containers need an
engine of the same architecture as the plugin (x86-64 or ARM64).

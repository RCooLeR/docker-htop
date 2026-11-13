# Changelog

## [0.1.0] - 2026-11-13

First release.

### Added

- `docker htop`, a Docker CLI plugin with a live terminal view of containers and their processes:
  per-core CPU bars, host memory, swap and load on top, then Compose projects, containers and
  processes with CPU, memory against the limit, network rates and task counts.
- Three data sources, chosen at start: the plugin's own `/proc` where it runs on the Docker host,
  a hardened helper container that shares the host's PID namespace where it does not (Docker
  Desktop), and the Engine API alone as a fallback or with `--no-sampler`.
- Sorting, filtering, search, folding, logs (`docker logs --follow`), a shell (`docker exec`) and
  signals to a container or to a single process, each only after a key press.
- `--no-stream` for one reading as plain text, `--interval`, `--all`, and the docker global flags
  `--context`, `--host` and `--config`.
- Binaries for Linux (amd64, arm64) and Windows (amd64). Each embeds the helper binary for its own
  architecture and creates the helper image locally; nothing is pulled from a registry.

### Known limits

- Tested in WSL 2 against Docker Desktop. Not yet verified on a real machine: the Windows build
  (it passes the automated tests) and the direct `/proc` mode on a Linux host with its own engine.
- Engines reached over TLS or `ssh://` are not supported.
- Signalling a single process needs an engine of the same architecture as the plugin.

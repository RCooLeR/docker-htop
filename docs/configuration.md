# Configuration

The plugin has no configuration file. Everything is a flag, and some flags have an environment
variable.

## Flags

```
docker htop [options]
```

| Flag | Environment variable | Default | Meaning |
|---|---|---|---|
| `--interval`, `-d` | `DOCKER_HTOP_INTERVAL` | `1s` | Time between two readings, from `200ms` to `1m` |
| `--all`, `-a` | `DOCKER_HTOP_ALL` | off | Show stopped containers too (the `a` key toggles it while running) |
| `--no-stream` | | off | Print one reading as text and exit |
| `--no-sampler` | `DOCKER_HTOP_NO_SAMPLER` | off | Use only the Engine API: read no `/proc`, start no helper container |
| `--version` | | | Print the version and exit |
| `--help`, `-h` | | | Show the options |

Durations use Go's notation: `500ms`, `2s`, `1m`.

`NO_COLOR` switches colours off in `--no-stream` output on a terminal.

## Choosing the engine

The plugin connects to the engine the `docker` command itself would use. The docker global flags
go before `htop`, as with any docker command:

```sh
docker --context desktop-linux htop
docker --host unix:///run/user/1000/docker.sock htop
```

The engine is picked in this order:

1. `--context`
2. `--host`
3. `DOCKER_HOST`
4. `DOCKER_CONTEXT`
5. the current context from the Docker configuration (`docker context use ...`)
6. the default: `unix:///var/run/docker.sock`, on Windows `npipe:////./pipe/docker_engine`

`--config` (or `DOCKER_CONFIG`) selects the Docker configuration directory, `~/.docker` by default.

Supported endpoints:

| Endpoint | Supported |
|---|---|
| `unix://` socket | yes |
| `npipe://` named pipe (Windows) | yes |
| `tcp://` without TLS | yes |
| `tcp://` with TLS, contexts that carry TLS material | no |
| `ssh://` | no |

For an engine on another machine, run the plugin on that machine, for example over SSH; the
Linux binary is a single file, see [Installation](installation.md#linux).

# Development

## Layout

| Path | Owns |
|---|---|
| `cmd/docker-htop` | The plugin entry: flags, the plugin metadata answer, exit code |
| `cmd/docker-htop-sampler` | The helper binary that runs inside the sampler image |
| `internal/app` | Wiring: endpoint, client, sampler, monitor, UI; the only package that knows all others |
| `internal/config` | Flags and the validated settings |
| `internal/docker` | Engine API client over `net/http`: unix socket, Windows named pipe, plain TCP |
| `internal/sampler` | The procfs reader and the checked signal sender |
| `internal/monitor` | Polls the engine, merges sampler readings, emits one snapshot per interval |
| `internal/ui` | The terminal UI and the plain-text renderer |
| `internal/model` | Types shared by the packages above, including the sampler wire format |
| `internal/samplerbin` | The embedded sampler binary |
| `scripts` | Build and install scripts |

The docker CLI runs the plugin as `docker-htop [docker global flags] htop [flags]`, and as
`docker-htop docker-cli-plugin-metadata` to list it.

## Build and checks

Go 1.27 or newer. From the repository root:

```sh
sh scripts/build.sh      # Windows: .\scripts\build.ps1
gofmt -s -l .
go vet ./...
GOOS=linux go vet ./...  # when working on Windows
go test ./...
```

`internal/samplerbin/bin/docker-htop-sampler` is a build output and is not tracked. Without it the
module still builds and tests; the plugin then runs in Engine API mode.

## Tests

Unit tests use no engine: `internal/docker` talks to a fake engine over `httptest`,
`internal/sampler` reads a procfs fixture under `testdata/proc`, `internal/monitor` uses a fake
engine and explicit timestamps, `internal/ui` drives the model with fabricated snapshots and
compares plain output with golden files (`-update` rewrites them).

`internal/docker` also has a test against the local engine. It runs with
`DOCKER_HTOP_INTEGRATION=1`; its sampler round trip additionally needs
`DOCKER_HTOP_BUILD_FAKE_SAMPLER=1` (compile the stand-in sampler on the fly) or
`DOCKER_HTOP_FAKE_SAMPLER=<path to a prebuilt linux/amd64 one>`. It creates and removes only
`docker-htop-*` helpers.

To run tests on Linux from a Windows checkout when WSL has an older Go, cross-compile the test
binary and execute it in the package directory:

```powershell
$env:GOOS = 'linux'; go test -c -o $env:TEMP\ui.test .\internal\ui\; Remove-Item Env:GOOS
wsl.exe -d Ubuntu --cd "$PWD\internal\ui" -- /mnt/c/Users/<you>/AppData/Local/Temp/ui.test '-test.count=1'
```

## Design choices

- **No Docker SDK.** The plugin needs about a dozen stable Engine API endpoints; the client is
  hand-written over `net/http`, which keeps the dependency tree small. The Windows named pipe
  needs `github.com/Microsoft/go-winio`.
- **The sampler is its own tiny binary** with only the standard library, embedded in the plugin
  and loaded as a `scratch` image, so no registry or image build is involved. The image tag is a
  hash of the binary: a plugin never talks to a sampler of another version.
- **`internal/model/sample.go` is the wire format** between plugin and sampler, one JSON line per
  reading with cumulative counters. The monitor derives every rate.
- **Three sources, tried in order** (`internal/app/source.go`): the plugin's own `/proc` when it
  shows the engine's containers, the sampler container, then the Engine API alone. The first two
  feed the monitor the same `model.Sample` values, so everything after the source is shared. The
  helper image is loaded only when a helper container is actually needed.
- **Missing is not zero.** Optional readings are nil pointers through all layers and are drawn
  as `—`.
- **The sampler has no write path.** Signals to a process go through a separate one-shot helper
  that gets the one capability it needs, see [Security](security.md).
- **Terminal UI:** Bubble Tea and Lip Gloss, limited to the 16 ANSI colours so the view follows
  the terminal's palette.

## Platform limits

- Each plugin embeds the sampler for its own architecture (linux/amd64 or linux/arm64). The image
  is declared for the architecture read from the sampler's ELF header, and an engine of another
  architecture gets no helper containers.
- Releases: pushing a `vMAJOR.MINOR.PATCH` tag on the tip of `main` runs
  `.github/workflows/release.yml`, which tests on Linux and Windows, builds every target with
  `scripts/build.sh` and publishes a GitHub release whose text is that version's section of
  `CHANGELOG.md`.
- TLS and `ssh://` endpoints are not implemented.
- Line endings: shell scripts and test fixtures are LF, PowerShell scripts CRLF (`.gitattributes`).

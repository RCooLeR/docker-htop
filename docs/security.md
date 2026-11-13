# Security

The plugin talks to the Docker socket, which is root-equivalent on the machine that runs the
engine. It needs the same access as `docker ps` and gets no more than your user already has. This
page says what it does with that access.

## What it reads

- Through the Engine API: the container list, container counters, the engine's `top` listing, and
  the file `/etc/passwd` of each running container (to show user names).
- From `/proc` of the Docker host, directly or through the sampler container (see
  [Collected data](collected-data.md#sources)): for every process its PID, parent, UID, state,
  thread count, CPU time, memory, command line and cgroup path. Only processes that belong to a
  container are kept.

Command lines can contain secrets that were passed as arguments. The plugin shows them on your
screen, as `ps` and `docker top` do, and stores or sends them nowhere. Before drawing, it replaces
control characters and escape sequences in every string that comes from the engine or a process,
so a crafted command line cannot manipulate the terminal.

## What it creates

| Object | Name | Lifetime |
|---|---|---|
| Image | `docker-htop-sampler:<hash>` | Created when a helper container is first needed, from the binary embedded in the plugin; stays until you remove it. Older tags are removed when a new one is created. |
| Sampler container | `docker-htop-sampler-<id>` | While `docker htop` runs, and only where the plugin cannot read `/proc` itself |
| Signal helper container | `docker-htop-signal-<id>` | A moment, once per signal sent to a single process |

All carry the label `io.github.rcooler.docker-htop.role`, and the plugin hides them from its own
list.

Where the plugin reads `/proc` directly (Docker Engine on the same Linux machine) it creates
nothing at all until you send a signal to a single process.

## The sampler container

| Setting | Value |
|---|---|
| PID namespace | the host's, so it can read `/proc` of every process |
| Network | none |
| Root filesystem | read-only; the image holds one static binary and no shell |
| Capabilities | all dropped |
| Privilege escalation | `no-new-privileges` |
| Limits | 64 MiB of memory, 16 processes |
| Logging | none |

It can read what any unprivileged process on the host can read from `/proc`, and it cannot change
anything. The plugin holds the container's standard input open; when the plugin exits, is killed
or loses its connection, the engine closes that input and the sampler stops. The container is
created with automatic removal.

## What it can change

Nothing happens without a key press on the selected row.

- **F9 on a container** sends the chosen signal to the container's main process through the
  Engine API, like `docker kill --signal`.
- **F9 on a process** starts the one-shot signal helper: the same image and settings as the
  sampler plus the single capability `KILL`. The helper refuses unless the PID belongs to the
  selected container, sends the signal, and is removed. It is given the PID shown on the screen;
  it checks the container, not that the PID still names the same process as a moment ago.
- **F7 and F8** run `docker logs --follow` and `docker exec` for the selected container with
  your own `docker` command.

Signals offered: `SIGTERM`, `SIGKILL`, `SIGINT`, `SIGHUP`, `SIGQUIT`, `SIGUSR1`, `SIGUSR2`,
`SIGSTOP`, `SIGCONT`.

## Running without helpers

`--no-sampler` (or `DOCKER_HTOP_NO_SAMPLER=1`) keeps the plugin on the Engine API: it reads no
`/proc`, creates no image and no helper container, and cannot signal single processes. An engine
that forbids containers with the host PID namespace stops the sampler in the same way, and the
header says so.

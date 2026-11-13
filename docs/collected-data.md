# Collected data

The plugin reads from two places: the Engine API for containers, and `/proc` of the machine that
runs the containers for processes and cores.

## Sources

**Engine API.** The container list, each container's state and status text, and its counters:
CPU time, memory, network and block I/O bytes, number of tasks. These are the counters
`docker stats` reads. The plugin asks once per interval and turns two readings into rates.

**`/proc` of the Docker host.** Process and per-core numbers are only there. The plugin gets at
it in one of two ways:

- **Directly.** When the plugin runs on the Docker host itself, as with Docker Engine installed on
  a Linux machine, it reads `/proc` on its own. Nothing is created in Docker.
- **Through the sampler.** With Docker Desktop the host is a virtual machine the plugin cannot
  look into, from Windows or from WSL. It then starts a small helper container that shares the
  host's PID namespace and reads `/proc` there once per interval:
  - The sampler binary is embedded in the plugin. On first use the plugin loads a local image
    `docker-htop-sampler:<hash>` made of that one file; nothing is pulled from a registry.
  - While `docker htop` runs, one container `docker-htop-sampler-<id>` runs from that image.
  - It stops and removes itself when the plugin goes away, whatever the reason.

  The container's privileges are listed in [Security](security.md).

The plugin chooses at start. It reads its own `/proc`; if a process of a running container shows
up there, it can see the containers and reads directly. While no container runs, it compares the
engine's host name with the machine's. In every other case it uses the sampler.

## Modes

The header names the source in use.

| | Local mode | Sampler mode | Engine API mode |
|---|---|---|---|
| Header shows | `Source: local /proc` | `Source: sampler` | `Source: Engine API` and the reason |
| Helper container while running | none | one | none |
| Container numbers | Engine API | Engine API | Engine API |
| Processes | `/proc`, exact | `/proc`, exact | the engine's `top` listing |
| Process CPU | exact, per interval | exact, per interval | averaged over at least 3 seconds (the counter has whole seconds) |
| Per-core bars, host memory, swap, load, task count | yes | yes | no: one CPU bar and one memory bar summed over the containers |
| Signal to a single process | yes, through a one-shot helper | yes, through a one-shot helper | not with `--no-sampler` or without the helper image |

The plugin uses Engine API mode when it is started with `--no-sampler`, and when it cannot read
`/proc` directly and the sampler is not available: built without it, an engine of another
architecture than the plugin, or a helper container that cannot start. It also switches to it if
the readings stop while the view is open.

## What the numbers mean

| Value | Meaning |
|---|---|
| Core bars | Share of the interval each core was not idle; time waiting for I/O counts as idle |
| Mem bar | Used memory (total minus free, buffers and cache), then buffers and cache, of the Docker host |
| Swp bar | Swap in use on the Docker host |
| Tasks, thr | Processes and threads on the Docker host, in containers or not |
| Load | 1, 5 and 15 minute load average of the Docker host |
| Container CPU% | CPU time used during the interval, on the `docker stats` scale: 100 is one fully used core |
| Container Mem | Usage without inactive file cache, as `docker stats` shows it, then the limit; `—` when the container has no limit |
| Net rx/tx | Bytes received and sent per second, summed over the container's interfaces; `—` when the container has none of its own (host or no networking) |
| Container Tasks | Processes and threads the engine counts in the container |
| Process PID | The PID on the Docker host, not the one inside the container |
| Process User | The name from the container's own `/etc/passwd`; the numeric UID when the image has no entry for it |
| Process CPU% | Same scale as the container's |
| Process Mem | Resident set size |
| Process Tasks | Threads |

"Docker host" is the machine whose kernel runs the containers: the Linux host itself, or with
Docker Desktop its virtual machine. In WSL 2 that virtual machine is shared with your
distributions, so its CPU and memory include what they use.

A rate needs two readings, so CPU and network values are `—` for the first second, and again for
one interval after a container restarts. Unknown values are always `—`, never zero.

The bytes units are binary: `K`, `M` and `G` are powers of 1024.

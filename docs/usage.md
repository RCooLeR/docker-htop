# Usage

```sh
docker htop              # full-screen view; q quits
docker htop --no-stream  # one reading as text, then exit
```

Flags are described in [Configuration](configuration.md).

## The screen

```
 0[||||||          38.2%]  4[||||            22.7%]  8[|||             18.9%]
 1[||              12.5%]  5[|                9.3%]  9[|                6.2%]
Mem[||||||||||||||||||||||||            9.8G/15.6G]   Swp[|                    0.3G/4.0G]
Containers: 21 running, 1 paused, 1 exited   Tasks: 154, 412 thr   Load: 1.84 1.32 0.97
Engine: docker-desktop 29.9.2   Source: sampler   Refresh: 1s   Sort: CPU%
Name / PID            User       CPU%         Mem   Net rx/tx  Tasks  Command / status
▾ homelab  compose, 4 containers
  ▾ postgres          postgres   61.6   470M/2.0G    96K/1.4M      7  Up 3 days (healthy)
      ├─ 2231         postgres    0.3        142M                  1  postgres -D /var/lib/postgresql/data
      └─ 2304         postgres   61.2        310M                  1  postgres: autovacuum worker app
  ▸ mosquitto         1883        0.4    12M/256M   310K/290K      1  Up 3 days
 F1Help  F3Search  F4Filter  F5Tree  F6Sort  F7Logs  F8Shell  F9Signal  F10Quit
```

- **Header:** one bar per CPU core, memory (used, then cache) and swap of the machine that runs
  the containers, then container counts, tasks and load. The second summary line names the engine,
  the data source, the refresh interval and the sort key.
- **Table:** Compose projects, their containers, and each container's processes. Containers
  outside Compose are grouped under `standalone`.
- **Container row:** name, user of its main process, CPU, memory used and limit, network receive
  and transmit rate per second, number of tasks, and the engine's status text.
- **Process row:** PID on the Docker host, user, CPU, resident memory, threads, command line.
- A paused container is shown in the warning colour, a stopped one muted, and a CPU value of 50
  or more in the danger colour. Colours come from your terminal's own 16-colour palette.
- On a narrow terminal the network column is dropped first, then the user column.

What the numbers mean exactly is in [Collected data](collected-data.md). A value the plugin does
not know is shown as `—`, never as zero.

## Keys

Every function key has a letter alternative, because some terminals keep function keys for
themselves.

| Key | Action |
|---|---|
| Up/Down, `k`/`j`, PgUp/PgDn, Home/End | Move the cursor |
| Enter, Space | Fold or unfold the project or container under the cursor |
| Left, Right | Fold (or jump to the parent row), unfold |
| F5, `t` | Fold or unfold all containers |
| F6 | Cycle the sort key: CPU, memory, name, tasks |
| `P`, `M`, `N` | Sort by CPU, memory, name |
| `a` | Show or hide stopped containers |
| F4, `\` | Filter; Enter keeps it, Esc clears it |
| F3, `/` | Search; Enter jumps to the next match, `n` repeats |
| F7, `l` | Follow the logs of the selected container |
| F8, `s` | Open a shell in the selected container |
| F9, `K` | Send a signal to the selected container or process |
| F1, `?`, `h` | Help |
| F10, `q`, Ctrl+C | Quit |

Sorting applies to containers inside a project and to processes inside a container. The cursor
stays on the same container or process when rows move.

The filter matches container names and images, and process commands and users, ignoring case. A
matching container keeps all its processes; a matching process keeps its container.

## Logs, shell and signals

These act on the row under the cursor. On a process row, logs and shell use its container.

- **Logs (F7)** runs `docker logs --follow --tail 200` in the terminal. Ctrl+C ends the logs and
  returns to the view.
- **Shell (F8)** runs `docker exec` with `bash` when the image has it, otherwise `sh`. Leaving
  the shell returns to the view. It is not available for stopped containers.
- **Signal (F9)** opens a dialog that names the target and lists the signals, `SIGTERM` first.
  Enter sends the selected one, Esc cancels. Nothing is sent before Enter. On a container row
  the signal goes to the container's main process; on a process row to that process only, through
  a one-shot helper container (not available with `--no-sampler`).

How signals are delivered and what else the plugin may change is in [Security](security.md).

## One reading as text

`docker htop --no-stream` prints the header and the whole tree once and exits, like
`docker stats --no-stream`. It waits about three intervals first, because rates need two readings.

- When the output is not a terminal (a pipe or a file) lines are 120 columns wide and carry no
  colour codes.
- On a terminal the width is the terminal's; set `NO_COLOR` to drop the colours.
- Add `--all` to include stopped containers.

# `prt` — list and kill listening ports

[Türkçe](README.md)

`prt` is a small CLI that shows which processes are listening on TCP/UDP ports and lets you stop them safely. Built for the "I left Redis/Postgres running again" problem.

- **Fast:** reads `/proc` directly on Linux (no `lsof` needed); on macOS it batches `lsof`/`ps`/`launchctl` into a single call each.
- **Safe:** shows what will be killed, asks for confirmation, refuses PID 1, itself and system daemons (`sshd`, `systemd`, `launchd`…), and verifies the process really exited.
- **Easy:** interactive picker, port ranges, filters, `--json`, shell completion.
- **Security-minded:** listeners reachable from the network (`0.0.0.0`/LAN) are highlighted; `--exposed` lists only those.

> The CLI messages are currently in Turkish.

## Install

```bash
go install github.com/mustafacavusoglu/prt@latest
```

Prebuilt binaries for Linux, macOS and Windows are attached to each [release](https://github.com/mustafacavusoglu/prt/releases).

## Usage

```bash
prt list [port|range ...]   # --name redis  --exposed  --all  --udp  --wide  --json
prt kill <port|range> ...   # --yes  --dry-run  -f/--force  -s HUP  --timeout 3s  --no-brew  --unsafe
prt                         # interactive picker (when run in a terminal)
prt completion zsh|bash|fish|powershell
```

Exit codes: `0` ok · `1` error · `2` nothing listening · `3` permission denied (use sudo) · `4` cancelled · `5` process survived the signal.

## Development

```bash
make test   # go test -race ./...
make lint   # vet for linux/darwin/windows + gofmt
```

License: [MIT](LICENSE)

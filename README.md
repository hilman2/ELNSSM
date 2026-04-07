# ELNSSM

**Even Less Non-Sucking Service Manager** — a modern Windows service manager
written in Go. Wraps any executable as a Windows service with health checks,
restart policies, notifications, dependency management and a web GUI.

[![CI](https://github.com/hilman2/ELNSSM/actions/workflows/ci.yml/badge.svg)](https://github.com/hilman2/ELNSSM/actions/workflows/ci.yml)
[![Release](https://github.com/hilman2/ELNSSM/actions/workflows/release.yml/badge.svg)](https://github.com/hilman2/ELNSSM/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/hilman2/ELNSSM)](https://goreportcard.com/report/github.com/hilman2/ELNSSM)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/hilman2/ELNSSM.svg)](https://pkg.go.dev/github.com/hilman2/ELNSSM)

> A spiritual successor to NSSM, built for the second half of the 2020s:
> single binary, embedded web UI, REST + WebSocket API, modern process
> supervision and notifications out of the box.

---

## Features

- **Single binary** — Guardian service, CLI and web GUI in one `.exe` (no CGo).
- **Process supervision** — restart policies, exponential backoff, crash-loop
  detection, Windows Job Objects for guaranteed process-tree termination.
- **Health checks** — HTTP, TCP and script probes with history.
- **Service dependencies** — start/stop in topological order.
- **Notifications** — SMTP, generic webhook, Slack/Discord, Telegram, ntfy.
- **Embedded web GUI** — pure HTML/CSS/JS, no Node toolchain required.
- **REST + WebSocket API** — IP-allowlisted, optional token/basic auth.
- **Live config reload** — YAML configs watched via `fsnotify`.
- **Two-tier persistence** — YAML for user config, embedded bbolt for runtime
  state and event history.
- **Cross-arch** — released for `windows/amd64` and `windows/arm64`.

## Installation

### Pre-built release (recommended)

Download the latest archive for your architecture from the
[Releases page](https://github.com/hilman2/ELNSSM/releases) and extract
`elnssm.exe` to a directory on your `PATH` (e.g. `C:\Program Files\ELNSSM`).

Then install the Guardian as a Windows service (run from an elevated shell):

```powershell
elnssm install
```

### From source

Requires Go 1.25 or newer.

```powershell
git clone https://github.com/hilman2/ELNSSM.git
cd ELNSSM
go build -o elnssm.exe .
```

Or via `make`:

```sh
make build
```

## Quick start

```powershell
# Install the Guardian as a Windows service
elnssm install

# Add a service definition
elnssm add my-app --exec "C:\apps\my-app\my-app.exe" --args "--port 8080"

# Start it
elnssm start my-app

# Inspect status
elnssm status my-app

# Open the web GUI (default: http://127.0.0.1:9100)
elnssm gui
```

## Configuration

ELNSSM uses two layers of YAML files, both stored under
`%ProgramData%\ELNSSM\config\` by default:

| File | Purpose |
| --- | --- |
| `elnssm.yaml` | Global Guardian / API / notification settings |
| `services/<name>.yaml` | Per-service definitions (exec, env, restart policy, health checks, …) |

See [`configs/elnssm.example.yaml`](configs/elnssm.example.yaml) and
[`configs/service.example.yaml`](configs/service.example.yaml) for fully
annotated examples.

## Web GUI

The Guardian binds to `127.0.0.1:9100` by default and is IP-allowlisted to
loopback. Open `http://127.0.0.1:9100` after installation. To expose the GUI
on the network, edit `elnssm.yaml`:

```yaml
api:
  listen: "0.0.0.0:9100"
  ip_whitelist:
    - "10.0.0.0/8"
  auth:
    enabled: true
    type: "token"
```

Generate or rotate the API token with `elnssm reset-token`.

## Architecture

```
                    +--------------------+
   Windows SCM ---->|  Guardian Service  |<---- elnssm CLI / REST / WS
                    +--------------------+
                              |
              +---------------+----------------+
              |               |                |
        Job Objects     Health Runner    Notifier
              |               |                |
       +---------+      +---------+      +-----+-----+
       | child 1 | ...  | http/tcp|      | smtp/web/ |
       | child 2 |      | script  |      | ntfy/...  |
       +---------+      +---------+      +-----------+
```

See [`TESTPLAN.md`](TESTPLAN.md) and the source under `internal/` for
detailed design and test notes.

## Development

```sh
# Vet & build
make vet
make build

# Run tests
make test
```

We lint with [golangci-lint](https://golangci-lint.run/) using
`.golangci.yml`. CI runs vet, lint, tests and a release-style build on every
push and PR.

## Releasing

Tagged releases are produced automatically by the
[`release.yml`](.github/workflows/release.yml) workflow via
[GoReleaser](https://goreleaser.com/). To cut a new release:

```sh
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

The workflow builds `windows/amd64` and `windows/arm64` archives, generates a
checksum file and publishes them to the GitHub release page.

## Contributing

Contributions are very welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md)
and the [Code of Conduct](CODE_OF_CONDUCT.md) before opening an issue or PR.

## Security

Found a vulnerability? Please **do not** open a public issue. See
[SECURITY.md](SECURITY.md) for the responsible-disclosure policy.

## License

ELNSSM is released under the [MIT License](LICENSE).

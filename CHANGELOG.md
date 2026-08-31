# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-08-31

### Security
- Authentication could be skipped entirely by any caller from `127.0.0.1`,
  ahead of the SSPI check for `BUILTIN\Administrators` and with no way to turn
  it off. It is now controlled by `api.auth.allow_local_bypass`.
- The cluster heartbeat took the slave's address from the `X-ELNSSM-Listen` and
  `X-Forwarded-For` headers. Both are chosen by the caller and the value reached
  the URL the master requests, carrying the slave token with it.
- Health check and lifecycle hook scripts were written to predictable paths in
  a directory unprivileged users can write to, then executed with the
  Guardian's privileges. They are now created exclusively under unpredictable
  names.

### Added
- `api.auth.allow_local_bypass` decides whether loopback callers skip
  authentication. Defaults to `true`, which is the previous behaviour.
- `interpreter` on script health checks selects `cmd` or `powershell`
  explicitly, instead of relying on the interpreter being guessed.
- Test coverage for the `manager` and `cluster` packages, which had none.

### Changed
- The cluster heartbeat carries a `listen_port` field, and the master forms a
  slave's address from the peer IP plus that port. A slave older than 0.2.0
  reports no port; it is still listed as a node, but the master refuses to
  proxy to it.

### Fixed
- The cluster proxy never worked: the recorded address held the heartbeat
  connection's ephemeral source port rather than the slave's API port.
- The Guardian could terminate on a nil pointer dereference when a service was
  detached during a restart while its monitor was running.
- A resource monitor goroutine leaked on every start and restart, each one
  polling a PID that Windows had since reassigned.
- A monitor goroutine could survive a detach and restart a process the new
  Guardian had already adopted, leaving two copies running.
- Service state was read by the API while the monitor wrote it, without
  synchronisation.
- A delayed auto-start slept through shutdown and then started its process,
  leaving it unsupervised.
- Script health checks using cmdlets such as `Stop-Service` ran under `cmd.exe`
  and failed permanently, which with `restart_on_health_fail` meant an endless
  restart loop.
- The README showed an `ip_whitelist` example using CIDR notation, which is
  rejected at startup.

## [0.1.0] - 2026-04-07

### Added
- Initial public release scaffolding: README, LICENSE (MIT), CONTRIBUTING,
  CODE_OF_CONDUCT, SECURITY policy, CHANGELOG.
- GitHub Actions: CI (vet + lint + test + build), GoReleaser-based release
  workflow, CodeQL security scanning, Dependabot for Go modules and Actions.
- GoReleaser configuration for `windows/amd64` and `windows/arm64` archives.
- `.golangci.yml` linter configuration.
- Issue templates (bug report, feature request) and pull request template.

### Changed
- Module path renamed to `github.com/hilman2/ELNSSM`.

### Removed
- Tracked binary artifacts (`elnssm.exe`, `elnssm.exe~`, `testapp.exe`) — now
  produced by CI/GoReleaser only.

[Unreleased]: https://github.com/hilman2/ELNSSM/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/hilman2/ELNSSM/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/hilman2/ELNSSM/releases/tag/v0.1.0

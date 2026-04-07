# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/hilman2/ELNSSM/compare/HEAD...HEAD

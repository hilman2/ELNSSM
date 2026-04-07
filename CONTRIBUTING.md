# Contributing to ELNSSM

Thanks for taking the time to contribute! ELNSSM is an open-source project and
welcomes pull requests, bug reports, ideas and discussion.

## Ground rules

- Be kind. We follow the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md).
- Project language is **English**: code, comments, commit messages, issues, PRs.
- ELNSSM targets **Windows only**. Pure Go, no CGo.
- Keep changes focused. Small, reviewable PRs land faster than mega-PRs.

## Reporting bugs

Open a [bug report](https://github.com/hilman2/ELNSSM/issues/new?template=bug_report.yml)
and include:

- ELNSSM version (`elnssm version`)
- Windows version
- Minimal `service.yaml` that reproduces the problem
- Steps to reproduce, expected vs. actual behavior
- Relevant excerpt from `%ProgramData%\ELNSSM\logs\guardian.log`

## Suggesting features

Open a [feature request](https://github.com/hilman2/ELNSSM/issues/new?template=feature_request.yml).
Describe the use case first, then the proposed solution. "Why" matters more
than "how".

## Development setup

Requirements:

- Go 1.25 or newer
- Windows 10/11 or Windows Server (some integration tests require the SCM)
- `make` (optional, but used by the Makefile)
- [`golangci-lint`](https://golangci-lint.run/usage/install/) for local linting

```sh
git clone https://github.com/hilman2/ELNSSM.git
cd ELNSSM
go mod download
make build
make test
```

## Pull request checklist

Before opening a PR, please make sure:

- [ ] `go vet ./...` is clean
- [ ] `golangci-lint run` is clean (or you've explained the new diagnostic)
- [ ] `go test ./...` passes
- [ ] New behavior has tests where reasonable
- [ ] User-visible changes are noted in [`CHANGELOG.md`](CHANGELOG.md) under
      the `## [Unreleased]` heading
- [ ] Commit messages follow the pattern below

## Commit messages

We loosely follow [Conventional Commits](https://www.conventionalcommits.org/).
Common prefixes:

- `feat:` — new user-visible feature
- `fix:` — bug fix
- `refactor:` — internal restructuring, no behavior change
- `docs:` — documentation only
- `test:` — tests only
- `build:` / `ci:` — build system or CI changes
- `chore:` — anything else

Example:

```
feat(health): add gRPC health probe

Adds a new health-check type that calls the gRPC health protocol on the
configured target. Closes #42.
```

## Branching & PRs

- Branch off `main`.
- Rebase, don't merge, when keeping your branch up to date.
- Squash trivial commits before merging if reviewers ask.

## License

By contributing, you agree that your contributions will be licensed under the
[MIT License](LICENSE).

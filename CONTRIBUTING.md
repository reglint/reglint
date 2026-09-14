# Contributing to RegLint

Thanks for your interest in contributing. This guide covers setup and the conventions that keep the project healthy.

## Getting Started

The easiest way to work on RegLint is the devcontainer: it ships Go 1.25, installs the quality toolchain (`.devcontainer/install-go-tools.sh`), and activates the git hooks (`lefthook install`) on container creation.

Manual setup requires:

- Go 1.25
- [lefthook](https://github.com/evilmartians/lefthook) — run `lefthook install` after cloning
- The toolchain used by the Makefile: `golangci-lint`, `govulncheck`, `gosec`, `go-arch-lint`, `gremlins` (see `.devcontainer/install-go-tools.sh` for exact versions)

Verify your setup:

```bash
make build
make test
```

## Git Hooks

Pre-commit hooks run automatically on every commit that touches `*.go`:

- `make format` (fixed files are re-staged)
- `make test-coverage` (gate: 90%)
- Mutation testing in diff mode (`gremlins --diff HEAD`)
- `make lint`, `make security`, `make arch`

## Workflow

1. **Specs first.** Read [`specs/README.md`](specs/README.md) before any feature work. Specs describe intent, not implementation; implement to spec, and propose spec changes in the same PR when reality demands them.
2. **Test-driven.** Write the failing test first, then implement. Follow the patterns in [`specs/testing-and-validations.md`](specs/testing-and-validations.md).
3. **Branch + PR.** Branch from `main` and open a pull request. CI runs the `quality` (lint, tests, race, flaky, coverage, mutation, security, arch) and `security` workflows on every PR.
4. **Conventional commits.** Use prefixes such as `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `ci:`.
5. **Mutation testing late.** `make mutation` is a final-stage tool, not part of your inner development loop.

## Make Targets

| Target | Purpose |
| --- | --- |
| `make quality` | Full local gate: tests, lint, race, flaky, coverage, mutation, security, arch |
| `make test` / `make test-race` | Unit tests, optionally with the race detector |
| `make coverage` | Coverage gate (minimum 90%) |
| `make lint` / `make security` / `make arch` | Static analysis, vulnerability scan, architecture rules |
| `make build` | Build the CLI binary to `bin/reglint` |
| `make run ARGS='...'` | Run the CLI from source |
| `make analyze-example` | Analyze test fixtures with the example config |
| `make test-e2e-smoke` / `make test-e2e` | Compiled-binary e2e smoke / full matrix |

## Releasing

Releases are automated: a changelog PR, a `vX.Y.Z` tag on `main`, and GoReleaser publishing binaries, checksums, and the Homebrew formula. The full procedure lives in [`specs/release-process.md`](specs/release-process.md).

Maintainers should be aware of the supporting repositories — [`iyaki/homebrew-tap`](https://github.com/iyaki/homebrew-tap) (Homebrew formula, updated by every release) and [`iyaki/reglint-action`](https://github.com/iyaki/reglint-action) (GitHub Action, versioned independently) — and of the `TAP_GITHUB_TOKEN` secret the release workflow needs to update the tap.

## Reporting Issues

Open a GitHub issue and include:

- The output of `reglint version` and the exact command you ran
- The rule configuration involved
- The command output

RegLint findings quote matched source text, which may be sensitive: redact match excerpts before pasting output.

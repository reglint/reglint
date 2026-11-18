# Contributing to RegLint

Thanks for your interest in contributing. This guide covers setup and the conventions that keep the project healthy.

## Getting Started

RegLint develops on [devenv](https://devenv.sh) (Nix). One-time host setup: install Nix and [devenv](https://devenv.sh/getting-started/) 2.4, and add the devenv binary cache to `/etc/nix/nix.conf` (or `~/.config/nix/nix.conf`):

```
extra-substituters = https://devenv.cachix.org https://cache.nixos.org
extra-trusted-public-keys = devenv.cachix.org-1:w1cLUi8dv3hnoSPGAuibQv+f9TZLr6cv/Hm9XgU50cw= cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY=
```

Then, from the repo root:

```bash
devenv shell
```

The first shell builds the environment and runs the `reglint:setup` task, which installs the pinned quality toolchain into `.cache/go-tools` (see `scripts/install-go-tools.sh`) and activates the git hooks. Later shells are instant; the task only re-runs when the tool script changes. CI runs the same make targets inside this environment (`.github/actions/setup-devenv`), so toolchain versions are identical locally and in CI.

Verify your setup:

```bash
make build
make test
```

Manual (non-nix) setup still works: Go 1.25, [lefthook](https://github.com/evilmartians/lefthook) (`lefthook install` after cloning), and the toolchain from `GOBIN=… bash scripts/install-go-tools.sh` on your `PATH`.

Working on the docs site (optional): `python3 -m venv .venv && . .venv/bin/activate && pip install -r requirements.txt`, then `make docs-site`.

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

Maintainers should be aware of the supporting repositories — [`reglint/homebrew-tap`](https://github.com/reglint/homebrew-tap) (Homebrew formula, updated by every release) and [`reglint/reglint-action`](https://github.com/reglint/reglint-action) (GitHub Action, versioned independently) — and of the `TAP_GITHUB_TOKEN` secret the release workflow needs to update the tap.

## Reporting Issues

Open a GitHub issue and include:

- The output of `reglint version` and the exact command you ran
- The rule configuration involved
- The command output

RegLint findings quote matched source text, which may be sensitive: redact match excerpts before pasting output.

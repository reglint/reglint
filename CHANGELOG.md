# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). The state of `main` between releases is described under [Unreleased].

## [Unreleased]

## [0.1.0] - 2026-09-12

### Added

- `analyze` command: regex-based scanning with YAML-defined rules, include/exclude globs, per-rule path filters, severities, and a `--fail-on` threshold that drives exit codes (`0` success, `1` error, `2` threshold met).
- `init` command to generate a starter `reglint-rules.yaml`.
- Console, JSON, and SARIF output formatters; multiple formats per run via `--out-json` / `--out-sarif`; ANSI colors with `NO_COLOR` support.
- Baseline workflow: `--baseline` compare mode keyed by `(filePath, message)` with count-based tolerance, and `--write-baseline` to regenerate.
- Git integration (off by default): staged-file mode (`--git-mode staged`) and diff-target mode (`--git-mode diff --git-diff <target>`), with `--git-added-lines-only` reporting.
- Ignore-file processing (on by default): `.gitignore` → `.ignore` → `.reglintignore` evaluation order, with `--no-gitignore` and `--no-ignore-files` opt-outs.
- File-safety guards: binary and oversized files (default 5 MB) are skipped and counted in scan stats.
- Quality infrastructure: coverage gate (90%), race and flaky detection, mutation testing with gremlins, `golangci-lint` / `govulncheck` / `gosec` / `go-arch-lint`, and a compiled-binary end-to-end suite.
- `version` command reporting the build-time version (ldflags-injected, `dev` by default).
- Tag-triggered GoReleaser release pipeline producing cross-platform binaries and checksums.

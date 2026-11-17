# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). The state of `main` between releases is described under [Unreleased].

## [Unreleased]

## [1.1.1] - 2026-11-17

### Fixed

- `analyze` no longer aborts when a directory cannot be read (e.g. `chmod 000` or root-owned leftovers): unreadable directories are skipped with a `reglint: warning: skipping unreadable path` diagnostic on stderr, and the exit code reflects findings only. Unreadable ignore files still fail the run.
- `exclude` globs (config or `--exclude`) now prune matching directories before their contents are read, so excluded-but-unreadable directories emit no warning and are never opened.
- `--git-mode staged` and `--git-mode diff` scan only the git candidate set instead of walking the whole tree, so non-candidate (possibly unreadable) directories are never touched.
- Config/CLI `exclude` globs now apply to files passed as explicit path arguments (e.g. `reglint analyze sub/c.php` with `exclude: ["sub/**"]` in the config), matching paths relative to the working directory like the default walk does.

## [1.1.0] - 2026-10-12

### Added

- `validate` command to check config and rule files for errors without scanning.
- Rule files: load additional rules via `ruleFiles` config and repeatable `--rules` flags, layered after the main config.

### Changed

- Release binaries are stripped (`-s -w`); unstripped debug builds available via `make build-debug`.

## [1.0.0] - 2026-09-29

First stable release — no functional changes since 0.2.1. As of this version, RegLint commits to backward compatibility of its public surface: the `reglint-rules.yaml` rule schema, the `analyze`/`init`/`version` commands with their flags and exit codes (`0` success, `1` error, `2` `--fail-on` threshold met), the `console`, `json`, `sarif`, and `github` output formats, the baseline file format, and the ignore-file precedence. Breaking changes ship only in a new major version.

### Changed

- Specs aligned with reality: PR annotations ship natively via `--format github` and are consumed by the `reglint/reglint-action` marketplace action; the once-planned reviewdog wrapper was never built and is no longer planned.

## [0.2.1] - 2026-09-26

### Fixed

- `--git-added-lines-only` no longer fails with `git mode diff failed to resolve added lines` when the diff contains a single line longer than 64 KiB (e.g. a committed minified JSON file). Parse errors now include the underlying cause instead of discarding it.

### Added

- Community health files: `SECURITY.md`, `CODE_OF_CONDUCT.md`, GitHub issue templates, and a pull request template.

### Changed

- README restructured for discoverability: release badge, expanded overview with example output, table of contents, comparison table, and FAQ.

## [0.2.0] - 2026-09-15

### Added

- GitHub Actions output format: `--format github` emits workflow commands (`::error` / `::warning` / `::notice` annotations) with `file` / `line` / `col` / rule `title` metadata, sensitive match text escaped, capped at 10 annotations per severity with a summary line pointing to `--format json` or `sarif` for the full list.

## [0.1.1] - 2026-09-14

### Added

- `reglint version` falls back to the module version from build info for `go install`-built binaries instead of reporting `dev`.

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
- One-line `curl | sh` installer (`install.sh`) with SHA-256 checksum verification, installing to `~/.local/bin`.
- Homebrew formula published to `reglint/homebrew-tap` on every release (`brew install reglint/tap/reglint`).
- GitHub Action (`reglint/reglint-action@v1`) to run scans in workflows, installing the checksum-verified release binary.

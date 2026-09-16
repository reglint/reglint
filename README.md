# RegLint

[![Release](https://img.shields.io/github/v/release/reglint/reglint)](https://github.com/reglint/reglint/releases)
[![quality](https://github.com/reglint/reglint/actions/workflows/quality.yml/badge.svg)](https://github.com/reglint/reglint/actions/workflows/quality.yml)
[![security](https://github.com/reglint/reglint/actions/workflows/security.yml/badge.svg)](https://github.com/reglint/reglint/actions/workflows/security.yml)
[![e2e-full](https://github.com/reglint/reglint/actions/workflows/e2e-full.yml/badge.svg)](https://github.com/reglint/reglint/actions/workflows/e2e-full.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Your own linter, defined in YAML regex.** RegLint is a regex-based static analysis tool for source repositories: scan any codebase with rules you write and report findings as console output, JSON, SARIF, or native GitHub Actions PR annotations.

- **Plain YAML rules** — a message, an RE2 regex, a severity: that's a rule.
- **CI-native** — `github` output format emits PR annotations; SARIF uploads to GitHub code scanning.
- **Fast and dependency-free** — a single static Go binary with parallel scans, `.gitignore` support, baseline mode, and staged/diff scoping.

## Table of Contents

- [Install](#install)
- [Quickstart](#quickstart)
- [Why RegLint?](#why-reglint)
- [Use in GitHub Actions](#use-in-github-actions)
- [CLI Overview](#cli-overview)
- [Exit Codes](#exit-codes)
- [Configuration](#configuration)
- [Output Formats](#output-formats)
- [Baseline Workflow](#baseline-workflow)
- [Git-Scoped Scans](#git-scoped-scans)
- [Development](#development)
- [Documentation](#documentation)
- [CI Recipe (GitHub Actions)](#ci-recipe-github-actions)
- [CI Recipe: PR Annotations (GitHub Actions)](#ci-recipe-pr-annotations-github-actions)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)
- [Contributing](#contributing)
- [Changelog](#changelog)
- [License](#license)

## Install

### Homebrew

```bash
brew install reglint/tap/reglint
```

The formula lives in [`reglint/homebrew-tap`](https://github.com/reglint/homebrew-tap) and is updated automatically on every release.

### One-line install

```bash
curl -fsSL https://raw.githubusercontent.com/reglint/reglint/main/install.sh | sh
```

Detects your platform, verifies the SHA-256 checksum, and installs to `~/.local/bin` (override with `REGLINT_INSTALL_DIR` or pin with `REGLINT_VERSION=v0.1.0`).

### Go install

Requires Go 1.25 or newer:

```bash
go install github.com/reglint/reglint/cmd/reglint@latest
```

### Prebuilt binaries

Download an archive and `checksums.txt` from [GitHub Releases](https://github.com/reglint/reglint/releases), verify the checksum, then extract and run the binary:

```bash
sha256sum -c checksums.txt --ignore-missing
tar -xzf reglint_<version>_linux_amd64.tar.gz
```

Or build from source:

```bash
git clone https://github.com/reglint/reglint
cd reglint
make build
./bin/reglint --help
```

## Quickstart

Create a starter rules file, then run a scan:

```bash
reglint init
reglint analyze --config reglint-rules.yaml
```

A rule is a message, a regex, and a severity:

```yaml
rules:
  - message: "Avoid hardcoded token: $1"
    regex: "token\\s*[:=]\\s*([A-Za-z0-9_-]+)"
    severity: "error"
    paths:
      - "src/**"
```

Findings print per file with `line:column` positions, and a summary line closes the run:

```text
src/client.go
- ERROR 42:12 Avoid hardcoded token: sk_live_example
  src/client.go:42

Summary: files=31 skipped=0 matches=1 durationMs=18
```

The exit code is `2` when a finding meets the `--fail-on` threshold — see [Exit Codes](#exit-codes). Use `reglint --help` or `reglint analyze --help` for the full command reference.

RegLint validates its own codebase in CI with the same rule schema — the repository policy lives in [`reglint-rules.yaml`](reglint-rules.yaml) and runs as the `reglint` job in [quality](.github/workflows/quality.yml) (`make scan` locally).

## Why RegLint?

RegLint sits between one-off `grep` invocations and heavyweight language-aware analyzers. It is for team-specific rules — banned APIs, TODO policies, license headers, hardcoded tokens, naming conventions — that apply across languages and that you want enforced in CI today.

| | `grep` + shell scripts | Secret scanners (gitleaks, trufflehog) | Language-aware analyzers (semgrep) | RegLint |
|---|---|---|---|---|
| Custom rules | Ad hoc pipelines | TOML rulesets | Per-language DSL | One YAML schema for every file type |
| Language coverage | Any text | Any (secret patterns) | One language per ruleset | Any text |
| Severity levels + fail threshold | DIY exit codes | Varies | Yes | Built in (`--fail-on`) |
| Baseline of known findings | No | Varies | Varies | Built in |
| GitHub PR annotations + SARIF | No | Varies | Via extra tooling | Native output formats |
| Deployment | Your scripts | Single binary | Binary or server | Single static binary |

When to pick something else: you need AST-accurate matches inside one language (semgrep), or a curated provider-specific secrets database (gitleaks). When you need your own rules everywhere, RegLint gets out of the way.

## Use in GitHub Actions

```yaml
name: scan
on: [push]
jobs:
  reglint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: reglint/reglint-action@v1.1.0
        with:
          fail-on: error
```

The action lives in [reglint/reglint-action](https://github.com/reglint/reglint-action) and is versioned independently of RegLint releases, publishing full `vX.Y.Z` tags only (no moving major tag — pin exact versions). It installs the checksum-verified binary and fails the step on the `fail-on` threshold; pin the tool with `tool-version: v0.1.0` (defaults to the latest release).

## CLI Overview

```bash
reglint <command> [flags]

Commands:
  analyze (alias: analyse)
  init
  version
```

Common usage patterns:

```bash
# Analyze current directory with default config path
reglint analyze

# Analyze specific roots
reglint analyze services/api services/web

# Generate config at a custom location
reglint init --out configs/reglint-rules.yaml
reglint analyze --config configs/reglint-rules.yaml
```

## Exit Codes

- `0`: command succeeded and no `--fail-on` threshold was triggered.
- `1`: command/config/runtime error (for example invalid flags or invalid baseline file).
- `2`: command succeeded, but at least one finding met `--fail-on` severity.

## Configuration

Default config path is `reglint-rules.yaml`.

Top-level fields:

- `rules` (required): list of regex rules.
- `include` / `exclude`: repository-level glob controls.
- `failOn`: one of `error`, `warning`, `notice`, `info`.
- `concurrency`: worker count override.
- `baseline`: default baseline file path.
- `git`: default Git scan settings.
- `consoleColorsEnabled`: enable or disable ANSI color in console output.
- `ignoreFilesEnabled`: enable or disable ignore file processing.
- `ignoreFiles`: custom ignore file list.

Binary files and files larger than `--max-file-size` (default 5 MB) are skipped; the console summary line and the `stats` section of the JSON output report skipped file counts.

Minimal example:

```yaml
include:
  - "**/*"
exclude:
  - "**/.git/**"
  - "**/node_modules/**"
failOn: "error"
consoleColorsEnabled: true
rules:
  - message: "Avoid hardcoded token: $1"
    regex: "token\\s*[:=]\\s*([A-Za-z0-9_-]+)"
    severity: "error"
    paths:
      - "src/**"
```

Console output uses ANSI severity colors by default. You can disable colors in config or for a single run with `NO_COLOR`:

```bash
NO_COLOR=1 reglint analyze --config reglint-rules.yaml --format console
```

## Output Formats

- `console` writes to stdout.
- `json` writes to stdout only when it is the single selected format.
- `sarif` writes to stdout only when it is the single selected format.
- `github` writes GitHub Actions annotations to stdout — always stdout, even alongside other formats; annotations are only recognized in the step log, so there is no `--out-github` flag.
- When combining multiple formats, use `--out-json` and/or `--out-sarif` as needed.

```bash
reglint analyze --format console,json --out-json /tmp/scan.json
reglint analyze --format sarif --out-sarif /tmp/scan.sarif
reglint analyze --format github,sarif --out-sarif /tmp/scan.sarif
```

## Baseline Workflow

Baseline compare mode suppresses known findings using `(filePath, message)` with count-based tolerance.

Compare against an existing baseline:

```bash
reglint analyze --config testdata/rules/fail.yaml --baseline testdata/baseline/valid-equal.json testdata/fixtures
```

Use baseline from config (without passing `--baseline`):

```bash
reglint analyze --config testdata/rules/baseline.yaml testdata/fixtures
```

Generate or refresh a baseline from current findings:

```bash
reglint analyze --config testdata/rules/fail.yaml --baseline testdata/baseline/generated.json --write-baseline testdata/fixtures
```

`--write-baseline` exits `0` on successful write, even if findings exist.

## Git-Scoped Scans

Git integration is optional and defaults to `off`.

```bash
reglint analyze --config reglint-rules.yaml
```

Scan staged files only:

```bash
reglint analyze --config reglint-rules.yaml --git-mode staged
```

Scan files selected by a diff target:

```bash
reglint analyze --config reglint-rules.yaml --git-mode diff --git-diff HEAD~1..HEAD
```

`--git-diff` implies `--git-mode diff` if `--git-mode` is not provided.

Restrict reporting to matches on added lines:

```bash
reglint analyze --config reglint-rules.yaml --git-mode diff --git-diff HEAD~1..HEAD --git-added-lines-only
```

Ignore-file behavior:

- Ignore files are enabled by default in all modes (`off`, `staged`, `diff`).
- Default evaluation order is `.gitignore`, then `.ignore`, then `.reglintignore`.
- Use `--no-gitignore` to disable only `.gitignore`.
- Use `--no-ignore-files` to disable all ignore-file processing.

## Development

Run quick end-to-end smoke coverage:

```bash
make test-e2e-smoke
```

Run full end-to-end matrix:

```bash
make test-e2e
```

Run full local quality checks:

```bash
make quality
```

## Documentation

Technical specifications and design docs live in [`specs/`](specs/README.md): core architecture, data model, configuration, regex rules, ignore files, git integration, CLI contracts, formatter output contracts, and the release process.

## CI Recipe (GitHub Actions)

Example workflow that runs RegLint on pull requests and uploads SARIF results:

```yaml
name: reglint

on:
  pull_request:
  push:
    branches: [main]

jobs:
  analyze:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build reglint
        run: make build

      - name: Run reglint and emit SARIF
        run: |
          mkdir -p artifacts
          ./bin/reglint analyze \
            --config reglint-rules.yaml \
            --format console,sarif \
            --out-sarif artifacts/reglint.sarif

      - name: Upload SARIF
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: artifacts/reglint.sarif
```

Notes:

- Keep `--format console,sarif` so logs stay visible in job output while SARIF is archived.
- If you use `--fail-on`, findings at that threshold fail the job with exit code `2`.
- `if: always()` on SARIF upload keeps diagnostics available even when analyze fails.

## CI Recipe: PR Annotations (GitHub Actions)

`--format github` writes GitHub Actions workflow commands, so the runner surfaces findings as annotations in the run summary and inline in the PR "Files changed" view — no token, upload step, or extra permissions:

```yaml
name: reglint-annotations

on:
  pull_request:

jobs:
  annotate:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build reglint
        run: make build

      - name: Annotate added lines
        run: |
          ./bin/reglint analyze \
            --config reglint-rules.yaml \
            --format github \
            --git-mode diff \
            --git-diff "origin/${{ github.base_ref }}" \
            --git-added-lines-only
```

Notes:

- GitHub caps annotations at 10 per severity per step; pair with `--format json,sarif` and out flags for the full list.
- `--git-added-lines-only` restricts annotations to lines the PR adds, so pre-existing findings stay out of review.
- With `--fail-on`, findings at that threshold fail the job with exit code `2` while annotations still render.

## Troubleshooting

- `config file not found: reglint-rules.yaml`
  - Run `reglint init` in the repository root or pass `--config <path>`.
- `effective --git-mode=diff requires --git-diff`
  - Add `--git-diff <target>` when using `--git-mode diff`.
- `--out-json is required` or `--out-sarif is required`
  - When selecting multiple formats, provide an output file path for each non-console formatter.
- Command exits `2` in CI
  - This is expected when `--fail-on` threshold is met; tune `failOn` in config or CLI if needed.
- No findings in staged/diff mode when you expected matches
  - Verify file selection (`--git-mode`, `--git-diff`) and ignore settings (`--no-gitignore`, `--no-ignore-files`).

## FAQ

### What is RegLint?

A regex-based linter for source repositories. You define rules in a YAML file (`reglint-rules.yaml`), point RegLint at a directory, and it reports every match with file, line, and column plus a severity. It ships as a single static binary written in Go.

### How is RegLint different from Semgrep?

Semgrep matches code with per-language AST patterns; RegLint matches text with RE2 regexes. That makes RegLint language-agnostic — the same rule scans Go, TypeScript, Terraform, and plain config files — at the cost of AST precision. Many teams run both: Semgrep for language-deep rules, RegLint for cross-cutting conventions.

### Can RegLint find hardcoded secrets?

Yes — a secret is just a regex match. The starter from `reglint init` and the [Quickstart](#quickstart) example show a hardcoded-token rule; combine `paths`, severity, and `--fail-on error` to block merges on leaked credentials. For curated provider-specific detection, a dedicated scanner is a good complement.

### Does RegLint support GitHub code scanning and SARIF?

Yes. `--format sarif` emits a SARIF report you can upload with `github/codeql-action/upload-sarif`, and `--format github` writes native workflow commands that render findings as PR annotations — no upload step, no extra token.

### How do I ignore vendored or generated files?

Ignore files are processed by default in this order: `.gitignore`, `.ignore`, `.reglintignore`. Add paths there, control them globally with `include` / `exclude` globs in the config, per rule with `paths` / `exclude`, or disable with `--no-gitignore` / `--no-ignore-files`.

### How do I adopt RegLint on a legacy codebase?

Generate a baseline once with `--write-baseline`, commit it, and run with `--baseline` in CI so only new findings fail the build. See [Baseline Workflow](#baseline-workflow).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for devcontainer setup, git hooks, and the spec-first workflow.

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## License

[MIT](LICENSE)

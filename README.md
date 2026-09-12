# RegLint
[![quality](https://github.com/iyaki/reglint/actions/workflows/quality.yml/badge.svg)](https://github.com/iyaki/reglint/actions/workflows/quality.yml)
[![security](https://github.com/iyaki/reglint/actions/workflows/security.yml/badge.svg)](https://github.com/iyaki/reglint/actions/workflows/security.yml)
[![e2e-full](https://github.com/iyaki/reglint/actions/workflows/e2e-full.yml/badge.svg)](https://github.com/iyaki/reglint/actions/workflows/e2e-full.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

RegLint is a regex-based linter for source repositories. It scans files using YAML-defined rules and emits `console`, `json`, or `sarif` output for local development and CI pipelines.

## Install

### One-line install

```bash
curl -fsSL https://raw.githubusercontent.com/iyaki/reglint/main/install.sh | sh
```

Detects your platform, verifies the SHA-256 checksum, and installs to `~/.local/bin` (override with `REGLINT_INSTALL_DIR` or pin with `REGLINT_VERSION=v0.1.0`).

### Go install

Requires Go 1.25 or newer:

```bash
go install github.com/iyaki/reglint/cmd/reglint@latest
```

### Prebuilt binaries

Download an archive and `checksums.txt` from [GitHub Releases](https://github.com/iyaki/reglint/releases), verify the checksum, then extract and run the binary:

```bash
sha256sum -c checksums.txt --ignore-missing
tar -xzf reglint_<version>_linux_amd64.tar.gz
```

Or build from source:

```bash
git clone https://github.com/iyaki/reglint
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

Use `reglint --help` or `reglint analyze --help` for the full command reference.

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
- When combining multiple formats, use `--out-json` and/or `--out-sarif` as needed.

```bash
reglint analyze --format console,json --out-json /tmp/scan.json
reglint analyze --format sarif --out-sarif /tmp/scan.sarif
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

Technical specifications and design docs live in [`specs/`](specs/README.md): core architecture, data model, configuration, regex rules, ignore files, git integration, CLI contracts, and formatter output contracts.

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

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for devcontainer setup, git hooks, and the spec-first workflow.

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## License

[MIT](LICENSE)

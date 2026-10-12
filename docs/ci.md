---
description: GitHub Actions recipes for RegLint — SARIF upload to code scanning, native PR annotations, the reglint-action, and pre-commit staged scans.
---

# CI Recipes

## SARIF recipe (GitHub Actions)

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

## PR annotations recipe (GitHub Actions)

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

## Using the reglint-action

Both recipes can replace the build step with the [`reglint/reglint-action`](https://github.com/reglint/reglint-action) — it installs the checksum-verified binary and fails the step on the `fail-on` threshold:

```yaml
      - uses: reglint/reglint-action@v1.1.0
        with:
          fail-on: error
          # tool-version: v1.1.0  # pin the tool version; defaults to latest
```

The action publishes full `vX.Y.Z` tags only (no moving major tag — pin exact versions).

## Running in pre-commit

Scan staged files only so committed code is what gets checked:

```bash
reglint analyze --config reglint-rules.yaml --git-mode staged
```

Pair with a [baseline](baseline.md) on legacy codebases, and see [Git-Scoped Scans](git-scans.md) for file selection details.

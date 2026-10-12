---
description: RegLint output formats — console, JSON, SARIF, and GitHub Actions annotations. Combine formats and route files with --format, --out-json, and --out-sarif.
---

# Output Formats

Select formats with `--format` as a comma-separated list (`console`, `json`, `sarif`, `github`).

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

Ordering is deterministic everywhere: matches sort by file path (byte-wise), then by line, then by column.

## Format notes

### console

Human-readable report to stdout: findings grouped per file with `line:column` positions and severity colors, closed by a summary line reporting file, skip, match, and duration counts.

### json

Machine-readable report on stdout (single-format runs) or written by `--out-json`. Includes the match list plus a `stats` section with skipped file counts.

### sarif

Static Analysis Results Interchange Format for GitHub code scanning and other SARIF consumers. Write to a file with `--out-sarif` and upload it — see the [SARIF CI recipe](ci.md#sarif-recipe-github-actions).

### github

GitHub Actions workflow commands rendered as annotations in the run summary and inline in the PR "Files changed" view. No token, upload step, or extra permissions. GitHub caps annotations at 10 per severity per step — pair with `--format json,sarif` and out flags for the full list. See the [PR annotations recipe](ci.md#pr-annotations-recipe-github-actions).

!!! warning "Sensitive content"
    Match text is treated as sensitive; formatter specs document exactly what each format emits. Console output includes the matched line text by design — avoid dumping full scan logs of secret-heavy repos.

Authoritative detail: [`specs/formatter.md`](https://github.com/reglint/reglint/blob/main/specs/formatter.md) and the per-formatter specs (`formatter-console`, `formatter-json`, `formatter-sarif`, `formatter-github`).

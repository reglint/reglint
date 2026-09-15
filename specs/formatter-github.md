# GitHub Formatter

Status: Draft

## Overview

### Purpose

- Provide GitHub Actions annotations for scan results using workflow commands.
- Enable PR and run-summary annotations in CI without external tools, tokens, or extra workflow permissions.
- Follow shared formatter guidelines in `specs/formatter.md`.

### Goals

- Emit one workflow command per match on stdout, mapped to GitHub annotation levels.
- Deterministic output: ordering, truncation, and escaping are stable for identical inputs.
- Zero-dependency integration that works with any RegLint install channel (action, brew, `go install`, binary) on GitHub-hosted and self-hosted runners.
- Respect GitHub per-step annotation limits by truncating client-side, deterministically.

### Non-Goals

- Posting PR review comments or calling the GitHub API (tokens, check runs, uploads). See `specs/release-process.md` for the marketplace action; reviewdog can consume existing SARIF output (`reviewdog -f=sarif`) for review comments.
- Auto-detecting CI environments (no `GITHUB_ACTIONS` sniffing; the format is opt-in and explicit to keep scans deterministic).
- End-line or end-column annotation ranges.
- Annotation output for other CI providers (GitLab, Jenkins, etc.).

### Scope

- Render a `ScanResult` as GitHub workflow command lines on stdout.
- Use only data already present in `ScanResult` plus the compiled ruleset for rule ids.

## Architecture

### Module/package layout (tree format)

```
internal/
  output/
    github.go
```

### Component diagram (ASCII)

```
[ScanResult] -> [GitHub Formatter] -> stdout (workflow commands) -> [GitHub runner] -> annotations
```

### Data flow summary

1. Receive `ScanResult` from the scan service (already post-suppression: baseline and git filters apply upstream).
2. Sort matches deterministically (see Ordering).
3. Convert each match to one workflow command line, capped per level (see Truncation).
4. Append a truncation summary notice when caps discarded matches.
5. Write lines to stdout, each terminated with `\n`.

## Data model

### Core Entities

GitHubAnnotation

- Definition: A single workflow command line rendered for one match.
- Fields:
  - `level` (string, required): `error|warning|notice` (see Severity mapping).
  - `title` (string, required): Rule id (see Rule id mapping). Example: `RC0001`.
  - `file` (string, required): Match file path with path separators normalized to `/` (same normalization as SARIF `artifactLocation.uri`).
  - `line` (int, required): 1-based line number.
  - `col` (int, required): 1-based rune column index.
  - `message` (string, required): Interpolated match message with workflow-command escaping applied.

GitHubTruncationSummary

- Definition: A single `notice` workflow command emitted when per-level caps discard matches.
- Fields:
  - `shown` (int, required): Match lines rendered.
  - `total` (int, required): Total matches in the result.

Command syntax (one line per annotation):

```
::{level} file={file},line={line},col={col},title={title}::{message}
```

- Property order is fixed: `file`, `line`, `col`, `title`.
- The summary line uses only `title`:

```
::notice title=RegLint::{shown} of {total} matches shown; GitHub limits annotations to 10 per severity per step. Use --format json or sarif for the full list.
```

### Relationships

- `GitHubAnnotation` derives from `Match` in `specs/data-model.md`.
- Rule ids derive from the compiled ruleset, same mapping as `specs/formatter-sarif.md`.

### Persistence Notes

- No persistence. Output is written to stdout only.

## Workflows

### Render annotations (happy path)

1. Sort matches (Ordering).
2. For each match in order, map severity to a level and build one command line with escaped values.
3. Enforce per-level caps (see Truncation); matches beyond a cap are dropped in canonical order.
4. Write one line per rendered match.
5. If any match was dropped in any level, append exactly one truncation summary line.

### No matches

- Write nothing. Zero matches produce no output lines.

### Error cases

- Write errors abort the run with exit code 1 (shared formatter contract).
- Output is never interleaved with other formats' content on a single line; each command occupies a full line.
- Exit codes are governed by `--fail-on` exactly as with other formats; this formatter never changes exit behavior.

## APIs

- Internal writer interface only. No network APIs.
- Suggested signature: `WriteGitHub(result ScanResult, ruleSet []rules.Rule, out io.Writer) error`.

## Client SDK Design

- No client SDK. Formatter is internal only.

## Configuration

- CLI flag: `--format github`.
- Output destination: stdout only. No `--out-github` flag: GitHub recognizes workflow commands only in the step's log stream, so file output has no consumer.
- May be combined with other formats (e.g. `--format github,sarif --out-sarif reglint.sarif`); annotations render to stdout while file-based formats use their out flags.

## Permissions

- No permissions or authentication. Workflow commands are parsed from the step log by the GitHub runner; no token and no extra workflow `permissions` block is required.

## Security Considerations

- Annotation messages contain interpolated match messages that may include sensitive data (same caveat as console and SARIF output).
- Do not emit raw `matchText` beyond what rule message interpolation itself includes.
- Paths are repo-relative; no absolute paths are emitted (unlike console output).

## Dependencies

- Standard library only.

## Verifications

- Output is deterministic across runs with identical inputs.
- Every rendered line matches the command syntax exactly; properties appear in fixed order `file`, `line`, `col`, `title`.
- Message escaping applies `%25`, `%0A`, `%0D`; property values additionally apply `%3A`, `%2C`.
- Per-level output never exceeds 10 `error`, 10 `warning`, and 10 `notice` match lines.
- Exactly one truncation summary notice is emitted if and only if matches were dropped.
- Zero matches produce empty output.
- Output contains no ANSI escape sequences and no raw `matchText`.
- Ordering matches the shared formatter ordering (see Appendices).
- Manual QA in a `pull_request` workflow: annotations appear in the run summary and inline in the PR Files changed view.

## Appendices

### Severity mapping

| Rule severity | Annotation level |
| ------------- | ---------------- |
| `error`       | `error`          |
| `warning`     | `warning`        |
| `notice`      | `notice`         |
| `info`        | `notice`         |

Note: mirrors the SARIF collapse of `notice`/`info` into one level (`note`).

### Rule id mapping

- Same as SARIF: `RC` + zero-padded 4-digit 1-based index of the rule in the rules file. Example: `RC0001`.

### Truncation

- GitHub limits workflow-command annotations to 10 error and 10 warning annotations per step (see [Checks API docs](https://docs.github.com/rest/checks/runs)). Notice limits are undocumented; a cap is applied for determinism.
- Cap: at most 10 match lines per level (`error`, `warning`, `notice`), keeping the first matches in canonical ordering.
- The summary notice is one additional `notice` line beyond the cap; if a runner drops it, the capped `error`/`warning` annotations (the levels that matter for review) are unaffected.

### Escaping

| Input | In message | In property values |
| ----- | ---------- | ------------------ |
| `%`   | `%25`      | `%25`              |
| `\n`  | `%0A`      | `%0A`              |
| `\r`  | `%0D`      | `%0D`              |
| `:`   | unchanged  | `%3A`              |
| `,`   | unchanged  | `%2C`              |

### Ordering

- Shared ordering from `specs/formatter.md` appendices: `filePath` (ascending, byte-wise), then `line`, then `column`, then severity rank `error > warning > notice > info`, then `message` (ascending, byte-wise).

### Output example

```
::error file=src/auth/token.go,line=12,col=5,title=RC0001::Avoid hardcoded token%3A ab
::warning file=src/server.go,line=42,col=1,title=RC0003::Unexpected debug flag
::notice file=docs/readme.md,line=3,col=9,title=RC0004::Use of TODO comment
::notice title=RegLint::14 of 34 matches shown; GitHub limits annotations to 10 per severity per step. Use --format json or sarif for the full list.
```

Notes:

- On `pull_request` workflows, the runner surfaces these annotations inline in the PR Files changed view; on all workflows they appear in the run summary Annotations section.
- Pair with `--git-mode=diff --git-added-lines-only` to restrict annotations to lines touched by the PR.

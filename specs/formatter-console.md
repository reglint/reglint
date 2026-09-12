# Console Formatter

Status: Implemented

## Overview

### Purpose

- Provide human-readable scan results on stdout for local usage.
- Provide optional ANSI color highlighting for severity labels in console output.
- Keep output stable and grep-friendly across platforms when colors are disabled.
- Follow shared formatter guidelines in `specs/formatter.md`.

### Goals

- Deterministic ordering of files and matches.
- Clear severity labeling and location formatting.
- Single-line summary with scan stats.
- Configurable ANSI color support with deterministic color mapping.

### Non-Goals

- Interactive or TUI-style output.
- Machine-optimized formats (see JSON/SARIF formatter specs).
- Streaming output per file while scanning (render after scan completes).

### Scope

- Render a `ScanResult` to stdout when `--format console` is selected.
- Use only data already present in `ScanResult`.

## Architecture

### Module/package layout (tree format)

```
internal/
  output/
    console.go
```

### Component diagram (ASCII)

```
[ScanResult] -> [Console Formatter] -> stdout
```

### Data flow summary

1. Receive `ScanResult` from the scan service.
2. Resolve effective console color settings.
3. Sort matches deterministically (see Ordering).
4. Group matches by relative `filePath`.
5. Render each match as a two-line block.
6. Render the summary line.

## Data model

### Core Entities

ConsoleMatchLine

- Definition: A rendered line for a single match.
- Fields:
  - `filePath` (string): Relative file path used for ordering.
  - `severityLabel` (string): One of `ERROR|WARN|NOTICE|INFO`.
  - `line` (int): 1-based line number.
  - `column` (int): 1-based column number (rune index).
  - `message` (string): Interpolated match message.
  - `absolutePath` (string): Absolute file path printed on the line below the finding as `<absolutePath>:<line>`.

ConsoleSummary

- Definition: Summary of the scan.
- Fields:
  - `filesScanned` (int)
  - `filesSkipped` (int)
  - `matches` (int)
  - `durationMs` (int64)

ConsoleColorSettings

- Definition: Effective controls for ANSI color emission in console output.
- Fields:
  - `enabled` (bool): Whether ANSI colors are emitted.
  - `source` (string): One of `default|config|env` indicating where the final value came from.

### Relationships

- `ConsoleMatchLine` derives from `Match` in `specs/data-model.md`.
- `ConsoleSummary` derives from `ScanStats` in `specs/data-model.md`.
- `ConsoleColorSettings` is resolved from RuleSet configuration in `specs/configuration.md` and environment-variable overrides in `specs/cli-analyze.md`.

### Persistence Notes

- No persistence. Output is written to stdout.

## Workflows

### Render matches (happy path)

1. Resolve effective color settings.
2. Sort matches (Ordering).
3. For each `filePath`, print the file header line.
4. For each match, print a bullet-prefixed finding line.
5. Print the absolute path line with `:<line>` on the next line.
6. Print a blank line between findings.
7. After all findings, print the summary line.

### Resolve color settings

1. Start with default `enabled=true`.
2. If RuleSet `consoleColorsEnabled` is set, use that value.
3. If `NO_COLOR` is set and non-empty, force `enabled=false`.
4. If `` is set and non-empty, force `enabled=false`.
5. Use fixed severity-to-color mapping when `enabled=true`.

Notes:

- Color settings apply only to `console` output.
- `json` and `sarif` outputs never include ANSI control sequences.

### No matches

1. Print `No matches found.`
2. Print the summary line.

## APIs

- Internal writer interface only. No network APIs.
- Suggested signature: `WriteConsole(result ScanResult, out io.Writer) error`.

## Client SDK Design

- No client SDK. Formatter is internal only.

## Configuration

- CLI flag: `--format console` (default).
- Output destination: stdout only.
- RuleSet field: `consoleColorsEnabled` (default `true`).
- Environment variable: `NO_COLOR` (non-empty) disables ANSI colors.

## Permissions

- No permissions or authentication.

## Security Considerations

- Output contains interpolated messages that may include sensitive data.
- Do not emit raw `matchText` in console output.
- Output includes absolute file paths, which may reveal local filesystem layout.

## Dependencies

- Standard library only.

## Verifications

- Console output is deterministic across runs with identical inputs.
- `No matches found.` appears when `matches == 0`.
- Summary line includes files scanned, skipped, total matches, and duration.
- During tests/QA, validate that every `absolutePath` refers to an existing file; the CLI does not validate at runtime.
- With colors enabled, severity labels are wrapped in ANSI SGR codes and reset with `\x1b[0m`.
- With `consoleColorsEnabled: false`, console output contains no ANSI escape sequences.
- With `NO_COLOR` set, console output contains no ANSI escape sequences.

## Appendices

### Ordering

- Sort by `filePath` (ascending, byte-wise).
- Within a file, sort by `line` then `column`.
- If ties remain, sort by severity order `error > warning > notice > info`.
- If ties remain, sort by `message` (ascending, byte-wise).

### Severity color mapping

- `ERROR`: `\x1b[31m` (red)
- `WARN`: `\x1b[33m` (yellow)
- `NOTICE`: `\x1b[36m` (cyan)
- `INFO`: `\x1b[34m` (blue)
- Reset after each colored segment with `\x1b[0m`.

### Output format

```
- \x1b[31mERROR\x1b[0m 12:5 Avoid hardcoded token: abc123
  /abs/path/to/file.ext:12

- \x1b[33mWARN\x1b[0m  42:1 Unexpected debug flag
  /abs/path/to/file.ext:42

- \x1b[36mNOTICE\x1b[0m 3:9 Use of TODO comment
  /abs/another/file.go:3

Summary: files=10 skipped=1 matches=3 durationMs=120
```

When colors are disabled, the same output is emitted without ANSI SGR sequences.

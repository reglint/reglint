# Analyze Command

Status: Implemented

## Overview

### Purpose

- Run regex scans using a YAML rules config and selected output formats.
- Parse flags, load rules, build a `ScanRequest`, and render outputs.
- Provide deterministic exit codes for CI usage.

### Goals

- Stable, well-documented flags and exit codes.
- Clear validation errors and deterministic behavior.
- Support console, JSON, and SARIF output selection.
- Allow ANSI colors in console output to be disabled via config or environment variables.
- Provide the `analyse` alias with identical behavior.
- Support optional baseline suppression using file/message count comparison.
- Support generating/regenerating baseline files from current findings.
- Support optional Git-scoped selection (`staged`, `diff`, `added lines`).

### Non-Goals

- Interactive UI or TUI.
- Editing or generating rules files (see `specs/cli-init.md`).
- Remote or daemonized scanning.

### Scope

- Command syntax, flags, defaults, and validation rules.
- Mapping flags to `ScanRequest` and output writers.
- Baseline-aware filtering behavior before output rendering and fail-on evaluation.
- Baseline generation/regeneration behavior for capturing all current findings.
- Git mode and diff-scoped file/line selection behavior.

## Architecture

### Module/package layout (tree format)

```
cmd/
  reglint/
    main.go
internal/
  cli/
    analyze.go
  baseline/
  config/
  rules/
  scan/
  output/
```

### Component diagram (ASCII)

```
[Analyze Command] -> [Config Loader] -> [Scan Service] -> [Baseline Comparator/Writer] -> [Output Writers]
```

### Data flow summary

1. Parse flags and positional paths.
2. Load and validate the rules config.
3. Resolve include/exclude lists and defaults.
4. Resolve effective console color settings from config and environment variables.
5. Build `ScanRequest`.
6. Run scan.
7. If `--write-baseline` is set, generate baseline from full findings and overwrite target file.
8. Else if baseline is active, filter matches using `specs/cli-analyze-baseline.md`.
9. Render requested outputs.
10. Exit with a deterministic code.

## Data model

### Core Entities

CLIConfig

- Definition: Parsed CLI inputs and resolved defaults for the analyze command.
- Fields:
  - `configPath` (string, required)
  - `roots` (list of string, required)
  - `formats` (list of string, required): `console|json|sarif|github`
  - `outJSON` (string, optional)
  - `outSARIF` (string, optional)
  - `include` (list of string, optional)
  - `exclude` (list of string, optional)
  - `concurrency` (int, required)
  - `maxFileSizeBytes` (int64, required)
  - `failOnSeverity` (string, optional)
  - `baselinePath` (string, optional)
  - `rulesetBaselinePath` (string, optional)
  - `consoleColorsEnabled` (bool, required)
  - `gitMode` (string, required): `off|staged|diff`
  - `gitDiffTarget` (string, optional)
  - `gitAddedLinesOnly` (bool, required)
  - `gitignoreEnabled` (bool, required)
  - `noIgnoreFiles` (bool, required): disable all ignore file processing for this run

Reference struct (Go):

```go
type CLIConfig struct {
    ConfigPath       string
    Roots            []string
    Formats          []string
    OutJSON          string
    OutSARIF         string
    Include          []string
    Exclude          []string
    Concurrency      int
    MaxFileSizeBytes int64
    FailOnSeverity   string
    BaselinePath     string
    RuleSetBaselinePath string
    ConsoleColorsEnabled bool
    GitMode             string
    GitDiffTarget       string
    GitAddedLinesOnly   bool
    GitignoreEnabled    bool
    NoIgnoreFiles      bool
}
```

### Relationships

- `CLIConfig` maps directly to `ScanRequest` in `specs/data-model.md`.
- Severity values align with `specs/regex-rules.md`.

## Workflows

### Parse flags and args (happy path)

1. Parse flags.
2. Collect positional arguments as `roots`.
3. If no roots provided, set `roots = ["."]`.
4. Resolve the config path (default `reglint-rules.yaml`).
5. Validate flags and formats (see Validation).
6. Load and compile rules from the config file.
7. Resolve effective baseline path from `--baseline` and RuleSet `baseline`.
8. Apply CLI overrides and build `ScanRequest`.
9. Resolve effective Git settings from defaults, RuleSet, and CLI.
10. Assemble scan hooks for this run (Git hooks active only when Git mode is enabled).
11. In `gitMode=staged|diff`, Git hooks validate requirements and resolve candidate files (and optional added-line filters).
12. Run scan.
13. If `--write-baseline` is set, generate baseline from full scan findings and overwrite the target file.
14. Else if baseline is active, filter matches using `specs/cli-analyze-baseline.md`.
15. Render outputs.

### Validation and errors

- `--config` (`-c`) defaults to `reglint-rules.yaml` in the current directory. If the file is missing or unreadable, print an error and exit with code 1.
- `--format` (`-f`) must include only `console`, `json`, `sarif`, or `github`.
- `--concurrency` must be a positive integer.
- `--max-file-size` must be a positive integer.
- `--fail-on` must be one of `error|warning|notice|info` when set.
- `--baseline` must point to a readable baseline JSON file when set.
- `--write-baseline` requires an effective baseline path from `--baseline` or RuleSet `baseline`.
- When `--write-baseline` is set, existing baseline content is ignored and overwritten.
- RuleSet `baseline` must be a valid path string when set in config.
- If RuleSet `baseline` is set and `--baseline` is unset, use RuleSet baseline path.
- If multiple formats are requested:
  - `json` requires `--out-json`.
  - `sarif` requires `--out-sarif`.
- In comparison mode (no `--write-baseline`), baseline schema/content validation follows `specs/cli-analyze-baseline.md`.
- `--git-mode` must be one of `off|staged|diff`.
- `--git-diff` implies `--git-mode=diff` for effective runtime behavior.
- If both `--git-mode` and `--git-diff` are provided, `--git-diff` takes precedence for mode resolution and effective mode is `diff`.
- In effective `git-mode=diff`, `--git-diff` is required.
- `--git-added-lines-only` is valid only when `--git-mode=staged|diff`.
- In `--git-mode=off`, Git binary and repository context must not be required.
- In `--git-mode=staged|diff`, missing Git executable, non-repository context, or unresolved diff target must exit with code 1.
- In Git-enabled runs, hook execution failures must return a single error and exit with code 1.
- Any validation failure prints a single error message and exits with code 1.

### Exit codes

- `0`: scan completed and no match at or above `--fail-on` (or `--fail-on` unset).
- `2`: scan completed and has matches at or above `--fail-on`.
- `1`: configuration or runtime error.
- In `--write-baseline` mode, a successful baseline write always exits `0` (even with matches).

## Configuration

### Command syntax

```
reglint analyze [flags] [path ...]
reglint analyse [flags] [path ...]
```

### Flags

| Flag                     | Type   | Required | Default              | Purpose                                       |
| ------------------------ | ------ | -------- | -------------------- | --------------------------------------------- |
| `--config,-c`            | string | no       | `reglint-rules.yaml` | Path to YAML rules config file.               |
| `--format,-f`            | string | no       | `console`            | Comma-separated list of `console,json,sarif,github`. |
| `--out-json`             | string | no       | none                 | Output path for JSON results.                 |
| `--out-sarif`            | string | no       | none                 | Output path for SARIF results.                |
| `--include`              | string | no       | none                 | Repeatable include glob for all rules.        |
| `--exclude`              | string | no       | none                 | Repeatable exclude glob for all rules.        |
| `--concurrency`          | int    | no       | `GOMAXPROCS`         | Worker count.                                 |
| `--max-file-size`        | int    | no       | `5242880`            | Skip files larger than N bytes.               |
| `--fail-on`              | string | no       | none                 | Fail if matches at or above severity.         |
| `--baseline`             | string | no       | none                 | Baseline JSON path for suppression.           |
| `--write-baseline`       | bool   | no       | `false`              | Generate/regenerate baseline from findings.   |
| `--git-mode`             | string | no       | `off`                | Select Git mode: `off,staged,diff`.           |
| `--git-diff`             | string | no       | none                 | Diff target/range for `--git-mode=diff`.      |
| `--git-added-lines-only` | bool   | no       | `false`              | Restrict matches to added lines in Git mode.  |
| `--no-gitignore`         | bool   | no       | `false`              | Disable `.gitignore` filtering for this run.  |
| `--no-ignore-files`      | bool   | no       | `false`              | Disable ignore file loading and matching.     |

### Precedence

- If `--include` is provided, it replaces the RuleSet `include` list from `specs/configuration.md`.
- If `--exclude` is provided, it replaces the RuleSet `exclude` list from `specs/configuration.md`.
- If `--include` is not provided, use RuleSet `include` or default to `**/*`.
- If `--exclude` is not provided, use RuleSet `exclude` or default to `**/.git/**`, `**/node_modules/**`, `**/vendor/**`.
- If `--fail-on` is provided, it overrides RuleSet `failOn`.
- If `--fail-on` is not provided, use RuleSet `failOn` when set; otherwise unset.
- If `--baseline` is provided, it overrides RuleSet `baseline`.
- If `--baseline` is not provided, use RuleSet `baseline` when set; otherwise baseline is disabled.
- If `--git-mode` is provided, it overrides RuleSet `git.mode`.
- If `--git-diff` is provided, it overrides RuleSet `git.diff`.
- If `--git-diff` is provided, effective Git mode is forced to `diff`.
- If `--git-added-lines-only` is set, it overrides RuleSet `git.addedLinesOnly`.
- If `--no-gitignore` is set, `.gitignore` filtering is disabled for this run.
- If `--no-ignore-files` is set, all ignore file processing is disabled for this run (overrides `--no-gitignore`).
- If `--baseline` is provided, baseline filtering is applied before formatter rendering.
- If `--baseline` is provided, `--fail-on` is evaluated against regression matches only.
- If `--write-baseline` is set, suppression is disabled and baseline is generated from full (unsuppressed) matches.
- If `--write-baseline` is set, `--fail-on` does not affect the final exit code.
- If `--write-baseline` is set and baseline file already exists, existing baseline is ignored and overwritten.
- Git selection/filtering order is:
  - candidate files by Git mode (`off|staged|diff`)
  - include globs
  - exclude globs
  - `.gitignore` (when enabled)
  - `.ignore` (higher priority than `.gitignore`)
  - `.reglintignore` (higher priority than `.ignore` and `.gitignore`)
  - per-rule `paths` / `exclude`
  - added-lines-only filtering
- Console color behavior for `--format console`:
  - Start from RuleSet `consoleColorsEnabled` from `specs/configuration.md` (default `true`).
  - If `NO_COLOR` is set and non-empty, force colors disabled.
  - Environment variables have higher precedence than RuleSet.

### Output behavior

- `console` always writes to stdout.
- `console` may include ANSI colors for severity labels when colors are enabled.
- If colors are disabled by RuleSet or environment variables, `console` output is plain text.
- If `json` is the only format and `--out-json` is unset, write JSON to stdout.
- If `sarif` is the only format and `--out-sarif` is unset, write SARIF to stdout.
- If multiple formats are requested, JSON and SARIF must have explicit output paths.
- Environment-variable color controls apply only to `console`; `json` and `sarif` never emit ANSI escape sequences.
- With `--baseline`, formatters receive regression-only matches; `stats.matches` reflects regression count.
- With `--write-baseline`, formatters receive full matches and baseline file is written from full findings.
- In Git mode, output schemas are unchanged; only file/line eligibility changes.

## Verifications

- `reglint analyze --config reglint-rules.yaml` scans current directory and exits with code 0/2.
- `reglint analyze -c reglint-rules.yaml -f json` writes JSON to stdout.
- `reglint analyze -c reglint-rules.yaml -f console,json --out-json /tmp/scan.json` writes console to stdout and JSON to file.
- Invalid `--fail-on` value exits with code 1 and prints an error.
- With `consoleColorsEnabled: false` in config, `-f console` output has no ANSI escape sequences.
- With `NO_COLOR=1`, `-f console` output has no ANSI escape sequences even if config enables colors.
- `reglint analyze -c reglint-rules.yaml --baseline testdata/baseline.json` reports only matches above baseline counts.
- With `baseline` set in rules config and no `--baseline`, analyze uses the config baseline path.
- With both config baseline and `--baseline`, analyze uses the CLI baseline path.
- `reglint analyze -c reglint-rules.yaml --baseline testdata/baseline.json --write-baseline` regenerates baseline from all current findings.
- With `--write-baseline`, existing baseline file contents are ignored.
- With `--write-baseline`, successful baseline write exits with code 0 even when matches are present.
- With `--write-baseline` and no effective baseline path, analyze exits with code 1.
- Invalid baseline file exits with code 1 and prints one error message.
- `reglint analyze -c reglint-rules.yaml --git-mode staged` scans only staged files.
- `reglint analyze -c reglint-rules.yaml --git-mode diff --git-diff HEAD~1..HEAD` scans only files in that diff.
- `reglint analyze -c reglint-rules.yaml --git-diff HEAD~1..HEAD` implies `--git-mode diff` and scans only files in that diff.
- `reglint analyze -c reglint-rules.yaml --git-mode diff --git-diff HEAD~1..HEAD --git-added-lines-only` reports only matches on added lines.
- `reglint analyze -c reglint-rules.yaml --git-mode staged` exits with code 1 when Git is unavailable.
- `reglint analyze -c reglint-rules.yaml --git-mode off` does not require Git.

### E2E scenario coverage map

- Happy path and fail thresholds: `E2E-SMOKE-001`, `E2E-SMOKE-003`, `E2E-SMOKE-004`
- Validation errors and single-error behavior: `E2E-SMOKE-002`, `E2E-FULL-011`
- Color controls and env precedence: `E2E-SMOKE-005`
- Path handling: `E2E-SMOKE-006`
- Baseline compare/generation/precedence: `E2E-FULL-001`, `E2E-FULL-002`, `E2E-FULL-003`
- Formatter output-path and stdout behavior: `E2E-FULL-004`, `E2E-FULL-005`, `E2E-FULL-006`
- Git mode behavior: `E2E-FULL-007`, `E2E-FULL-008`, `E2E-FULL-009`, `E2E-FULL-010`, `E2E-FULL-011`
- File handling and deterministic ordering: `E2E-FULL-012`, `E2E-FULL-013`, `E2E-FULL-014`, `E2E-FULL-015`

See `specs/e2e-test-suite.md` for canonical e2e scenario definitions and tiering.

## Appendices

### Examples

```
reglint analyze --config configs/example.rules.yaml
reglint analyse -c configs/example.rules.yaml -f json --out-json /tmp/scan.json
reglint analyze -c configs/example.rules.yaml -f sarif --out-sarif /tmp/scan.sarif
reglint analyze -c configs/example.rules.yaml --baseline testdata/baseline.json --write-baseline
reglint analyze -c configs/example.rules.yaml --git-mode staged
reglint analyze -c configs/example.rules.yaml --git-mode diff --git-diff HEAD~1..HEAD --git-added-lines-only
```

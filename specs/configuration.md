# Configuration

Status: Implemented

## Overview

### Purpose

- Define the YAML schema for RegLint rules.
- Capture defaults and validation rules in one place.
- Document global scan controls that apply across rules.

### Goals

- Keep the rules config easy to read and stable across versions.
- Ensure deterministic defaults for scanning behavior.

### Non-Goals

- Define CLI flags or output formats (see other specs).

## Schema

RuleSet

- Definition: Top-level YAML configuration file.
- Fields:
  - `rules` (list of Rule, required): Rule schema, defaults, and validation are defined in `specs/regex-rules.md`.
  - `include` (list of string, optional): Global include globs. Overridden by per-rule `paths`.
  - `exclude` (list of string, optional): Global exclude globs. Overridden by per-rule `exclude`.
  - `failOn` (string, optional): One of `error|warning|notice|info`. Causes non-zero exit status at or above this severity.
  - `concurrency` (int, optional): Worker count for scanning.
  - `baseline` (string, optional): Default baseline JSON path used by `analyze` when `--baseline` is not set.
  - `consoleColorsEnabled` (bool, optional): Enable ANSI colors in `console` formatter output.
  - `ignoreFilesEnabled` (bool, optional): Enable ignore file processing. Runtime behavior is defined in `specs/ignore-files.md`.
  - `ignoreFiles` (list of string, optional): Ordered ignore file names to evaluate. Runtime behavior is defined in `specs/ignore-files.md`.
  - `git` (GitSettings, optional): Optional Git integration settings for scoped scans.

GitSettings

- Definition: Optional RuleSet-level controls for Git-backed scan scope.
- Fields:
  - `mode` (string, optional): `off|staged|diff`
  - `diff` (string, optional): Diff target/range used by `mode: diff`.
  - `addedLinesOnly` (bool, optional): Report only matches on added lines.
  - `gitignoreEnabled` (bool, optional): Enable `.gitignore` filtering.

## YAML example (with globals)

```yaml
include:
  - "src/**"
  - "lib/**"
exclude:
  - "**/generated/**"
failOn: "error"
concurrency: 8
baseline: "testdata/baseline.json"
consoleColorsEnabled: true
ignoreFilesEnabled: true
ignoreFiles:
  - ".ignore"
  - ".reglintignore"
git:
  mode: "diff"
  diff: "HEAD~1..HEAD"
  addedLinesOnly: false
  gitignoreEnabled: true
rules:
  - message: "This is an error message"
    regex: "regex1"
    severity: "error"
    paths:
      - "src/**/*.js"
      - "lib/**/*.js"
  - message: "This is a warning message"
    regex: "regex2"
    severity: "warning"
    exclude:
      - "lib/vendor/**"
```

## Defaults

- RuleSet `include`: `**/*` if missing.
- RuleSet `exclude`: `**/.git/**`, `**/node_modules/**`, `**/vendor/**` if missing.
- `failOn`: unset (no failure threshold) if missing.
- `concurrency`: `GOMAXPROCS` if missing.
- `baseline`: unset (baseline disabled unless CLI flag sets it) if missing.
- `consoleColorsEnabled`: `true` if missing.
- `ignoreFilesEnabled`: `true` if missing.
- `ignoreFiles`: `['.ignore', '.reglintignore']` if missing.
- `git.mode`: `off` if missing.
- `git.diff`: unset if missing.
- `git.addedLinesOnly`: `false` if missing.
- `git.gitignoreEnabled`: `true` if missing.

## Validation

- YAML must parse successfully.
- `rules` is required.
- `failOn` must be one of the allowed values when set.
- `concurrency` must be a positive integer when set.
- `baseline` must be a non-empty string when set.
- `consoleColorsEnabled` must be a boolean when set.
- `ignoreFilesEnabled` must be a boolean when set.
- `ignoreFiles` values must be non-empty file names without path separators and without duplicates when set.
- `git.mode` must be one of `off|staged|diff` when set.
- `git.diff` must be a non-empty string when set.
- `git.diff` is valid only when `git.mode=diff`.
- `git.mode=diff` requires `git.diff`.
- `git.addedLinesOnly=true` is valid only when `git.mode=staged|diff`.
- `git.gitignoreEnabled` must be a boolean when set.
- Rules are validated per `specs/regex-rules.md`.

## Notes

- Rule schema, defaults, and path override behavior are defined in `specs/regex-rules.md`.
- Runtime environment-variable precedence for console colors is defined in `specs/cli-analyze.md`.
- Baseline generation/regeneration behavior using `--write-baseline` is defined in `specs/cli-analyze-baseline.md`.
- Git runtime behavior and error semantics are defined in `specs/git-integration.md` and `specs/cli-analyze.md`.

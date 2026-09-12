# Analyze Baseline Support

Status: Implemented

## Overview

### Purpose

- Add baseline-aware analyze runs so teams can adopt RegLint incrementally without failing on pre-existing findings.
- Compare current findings against a committed baseline using a file path + message key with count-based tolerance.
- Keep baseline behavior deterministic and consistent with existing `analyze` output and exit-code semantics.

### Goals

- Support a baseline file that stores counts by `(filePath, message)`.
- Report only regressions beyond baseline counts.
- Support generating/regenerating baseline files from current findings.
- Keep formatter schemas unchanged (console/JSON/SARIF still consume `ScanResult`).
- Preserve deterministic output and exit-code behavior for CI.

### Non-Goals

- Per-line or per-column suppressions.
- Baseline keys based on `matchText` or absolute paths.
- Automatic background baseline updates during normal analyze runs.
- Changing formatter output schemas to include baseline metadata.

### Scope

- New `analyze` CLI flag to load a baseline file.
- New `analyze` CLI flag to generate/regenerate a baseline file from current findings.
- RuleSet configuration field to define a default baseline file path.
- Baseline document schema and validation rules.
- Comparison algorithm and how filtered matches flow into output + `--fail-on` evaluation.

## Architecture

### Module/package layout (tree format)

```
internal/
  cli/
    analyze.go
  baseline/
    model.go
    loader.go
    compare.go
    writer.go
```

### Component diagram (ASCII)

```
[Analyze Command]
       |
       v
[Scan Service] ------> [Baseline Comparator] ------> [Output Formatters]
       |                       ^
       |                       |
       +--> [Baseline Builder + Writer]      [Baseline Loader]
```

### Data flow summary

1. Parse analyze flags and load rules config.
2. Resolve effective baseline path from CLI/config precedence.
3. Run scan and produce full `ScanResult`.
4. If `--write-baseline` is set, build and write baseline from full matches, ignoring any existing baseline file contents.
5. Else if an effective baseline path is set, load and validate baseline document.
6. Compare full matches with baseline counts using `(filePath, message)` keys.
7. Produce an effective `ScanResult` containing only regression matches.
8. Render selected formatter outputs from the effective result.
9. Determine exit code:
   - With `--write-baseline`, return `0` when baseline write succeeds.
   - Otherwise apply normal `--fail-on` behavior to effective matches.

## Data model

### Core Entities

BaselineEntry

- Definition: Baseline count for one file/message key.
- Fields:
  - `filePath` (string, required): Relative path using `/` separators.
  - `message` (string, required): Interpolated match message.
  - `count` (int, required): Expected count for this key, must be `> 0`.

BaselineDocument

- Definition: On-disk baseline file consumed by `analyze`.
- Fields:
  - `schemaVersion` (int, required): Current value `1`.
  - `entries` (list of BaselineEntry, required): May be empty.

BaselineComparison

- Definition: Deterministic comparison result between current matches and baseline counts.
- Fields:
  - `regressions` (list of Match, required): Matches beyond baseline counts.
  - `suppressedCount` (int, required): Current matches covered by baseline.
  - `improvementsCount` (int, required): Baseline counts not observed in current scan.

BaselineGeneration

- Definition: Deterministic baseline document generated from current scan findings.
- Fields:
  - `document` (BaselineDocument, required): Generated baseline payload.
  - `entryCount` (int, required): Number of unique `(filePath, message)` keys.

Reference structs (Go):

```go
type BaselineEntry struct {
    FilePath string `json:"filePath"`
    Message  string `json:"message"`
    Count    int    `json:"count"`
}

type BaselineDocument struct {
    SchemaVersion int             `json:"schemaVersion"`
    Entries       []BaselineEntry `json:"entries"`
}

type BaselineComparison struct {
    Regressions       []scan.Match
    SuppressedCount   int
    ImprovementsCount int
}

type BaselineGeneration struct {
    Document   BaselineDocument
    EntryCount int
}
```

### Relationships

- Baseline matching uses `Match.filePath` + `Match.message` from `specs/data-model.md`.
- Baseline path defaults may come from RuleSet configuration in `specs/configuration.md`.
- Baseline processing occurs after scan aggregation and before formatter rendering.
- Baseline generation uses full current matches before any suppression.
- Existing formatter contracts in `specs/formatter.md` remain unchanged.

### Persistence Notes

- Baseline is stored as UTF-8 JSON.
- Duplicate `(filePath, message)` keys are invalid.
- `entries` are canonicalized in lexical order (`filePath`, then `message`) for deterministic diffs.
- Baseline generation overwrites the target file when it already exists.

## Workflows

### Analyze with baseline (happy path)

Precondition: `--write-baseline` is not set.

1. Parse standard analyze flags.
2. Load RuleSet and resolve effective baseline path:
   - If `--baseline` is set, use it.
   - Else if RuleSet `baseline` is set, use it.
   - Else baseline is disabled.
3. Run scan to obtain full `ScanResult`.
4. Load baseline JSON and validate schema/content.
5. Sort current matches deterministically (shared ordering: `filePath`, `line`, `column`, severity order, `message`).
6. Build a mutable `remainingCount[(filePath, message)]` map from baseline entries.
7. Iterate sorted current matches:
   - If `remainingCount[key] > 0`, decrement and suppress this match.
   - Else, include this match in `regressions`.
8. Compute `improvementsCount` as the sum of remaining map values.
9. Build effective `ScanResult` with `matches = regressions` and `stats.matches = len(regressions)`.
10. Render outputs and evaluate exit code using effective matches.

### Generate/regenerate baseline (happy path)

1. Parse standard analyze flags including `--write-baseline`.
2. Resolve effective baseline path using normal precedence (`--baseline` > RuleSet `baseline`).
3. Validate that an effective baseline path is present.
4. Run scan to obtain full `ScanResult`.
5. Ignore any existing baseline file content (do not parse or compare it).
6. Aggregate full matches by `(filePath, message)` into positive counts.
7. Build `BaselineDocument` with `schemaVersion=1` and lexically sorted entries.
8. Write JSON baseline to the effective baseline path, overwriting existing files.
9. Render outputs from full (unsuppressed) matches.
10. Return exit code `0` when baseline write succeeds, regardless of matches or `--fail-on`.

### Baseline count outcomes

- If current count equals baseline count for a key: no regressions for that key.
- If current count is higher: only the excess count is reported as regressions.
- If current count is lower: no regressions; contributes to `improvementsCount`.

### Analyze without baseline

- If neither `--baseline` nor RuleSet `baseline` is set and `--write-baseline` is not set, behavior is unchanged from `specs/cli-analyze.md`.

### Baseline path resolution

- RuleSet `baseline` relative paths are resolved from the directory containing the loaded rules config file.
- `--baseline` relative paths are resolved from the current working directory.
- CLI flag `--baseline` has higher precedence than RuleSet `baseline`.

### Baseline generation behavior

- `--write-baseline` requires an effective baseline path (`--baseline` or RuleSet `baseline`).
- When `--write-baseline` is set, existing baseline files are ignored as input and treated only as overwrite targets.
- Baseline suppression is disabled when `--write-baseline` is set.
- When `--write-baseline` is set and baseline write succeeds, exit status is always `0`.

### Validation and errors

- In comparison mode (no `--write-baseline`), effective baseline path (from CLI or RuleSet) must point to a readable regular file.
- In `--write-baseline` mode, effective baseline path must be writable (or creatable in a writable parent directory).
- In `--write-baseline` mode, existing baseline file contents are never parsed as input.
- In comparison mode, baseline file must parse as JSON.
- `schemaVersion` must be `1`.
- `entries` must be present (can be empty).
- Every entry must satisfy:
  - `filePath` is non-empty, relative, normalized, and contains no `..` traversal segments.
  - `message` is non-empty.
  - `count` is a positive integer.
- Duplicate `(filePath, message)` entries are rejected.
- Any baseline validation failure exits with code `1` and a single error message.
- Baseline write failures exit with code `1` and a single error message.

## APIs

- CLI only. No network APIs.

## Client SDK Design

- No SDK changes. Feature is internal to `analyze` behavior.

## Configuration

### Analyze flag additions

| Flag               | Type   | Required | Default | Purpose                                                  |
| ------------------ | ------ | -------- | ------- | -------------------------------------------------------- |
| `--baseline`       | string | no       | none    | Path to baseline JSON used for suppression.              |
| `--write-baseline` | bool   | no       | `false` | Generate/regenerate baseline from full current findings. |

### RuleSet additions

| Field      | Type   | Required | Default | Purpose                                             |
| ---------- | ------ | -------- | ------- | --------------------------------------------------- |
| `baseline` | string | no       | none    | Default baseline JSON path when `--baseline` unset. |

### Precedence and interaction

- Baseline path source precedence is `--baseline` > RuleSet `baseline` > unset.
- Baseline filtering runs before `--fail-on` threshold evaluation.
- `--fail-on` applies to regression matches only when baseline is active.
- Existing include/exclude and ignore-file filtering still run before baseline comparison.
- When `--write-baseline` is set, baseline suppression is skipped and full matches are used for output and baseline file generation.
- When `--write-baseline` is set, `--fail-on` does not affect exit status.

### Baseline file example

```json
{
	"schemaVersion": 1,
	"entries": [
		{
			"filePath": "src/auth/token.go",
			"message": "Avoid hardcoded token: abc123",
			"count": 2
		},
		{
			"filePath": "src/http/server.go",
			"message": "Debug endpoint enabled",
			"count": 1
		}
	]
}
```

## Permissions

- No authentication or roles.

## Security Considerations

- Baseline `message` values may include interpolated sensitive data; repositories should treat baseline files as sensitive artifacts.
- Baseline files must never include `matchText`.
- Error output must not print full baseline file contents.
- Baseline generation overwrites files; users should review diffs before committing regenerated baselines.

## Dependencies

- Standard library `encoding/json` for baseline parsing.

## Open Questions / Risks

- Message interpolation can make baseline keys volatile if rule messages include changing values.
- Future work may introduce optional stable keys (for example rule IDs) without breaking this baseline schema.

## Verifications

- With `--baseline` and equal counts, outputs contain zero matches and exit code follows `--fail-on` semantics for zero regressions.
- With `--baseline` and count increase on one key, only excess matches are output.
- With `--baseline` and count decrease, no regressions are output and command does not fail due to that key.
- With RuleSet `baseline` set and no `--baseline` flag, baseline suppression is applied.
- With both RuleSet `baseline` and `--baseline`, CLI flag path is used.
- With `--write-baseline`, command writes a canonical baseline containing all current findings by `(filePath, message)` counts.
- With `--write-baseline` and an existing baseline file, existing baseline content is ignored and overwritten.
- With `--write-baseline`, successful baseline write exits with code `0` even when matches exist and `--fail-on` is set.
- With `--write-baseline` and no effective baseline path, command exits `1` with one error message.
- Invalid baseline JSON or duplicate keys returns exit code `1` with one error message.
- Repeated runs with identical inputs produce identical regression outputs.

## Appendices

### Comparison pseudocode

```text
remaining := map[(filePath, message)]count from baseline
regressions := []

for match in sorted(currentMatches):
  key := (match.filePath, match.message)
  if remaining[key] > 0:
    remaining[key]--
    continue
  regressions.append(match)

improvementsCount := sum(remaining values)
```

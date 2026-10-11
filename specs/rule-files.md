# Rule Files

Status: Draft

## Overview

### Purpose

- Allow splitting regex rules out of the base config into one or more dedicated rule files.
- Enable shared, curated rule packs (e.g., language-specific rulesets) referenced from the base config or passed on the command line.
- Keep merge semantics simple, deterministic, and append-only.

### Goals

- Support a `ruleFiles` list in the base RuleSet config.
- Support a repeatable `--rules` CLI flag on `analyze`.
- Treat a rule file as a YAML document whose only allowed populated field is `rules`.
- Define deterministic merge order across base config, `ruleFiles`, and `--rules`.
- Reject duplicate rules across the merged set with a clear validation error.

### Non-Goals

- Glob patterns in `ruleFiles` or `--rules`.
- Nested rule files (a rule file referencing other rule files).
- Merging or overriding any setting (include/exclude, failOn, git, baseline, etc.) from rule files.
- Remote rule files (URLs, network fetch).
- Rule overrides or precedence mechanisms between files.

### Scope

- Rule file document schema and validation.
- `ruleFiles` config field: resolution, loading order, and errors.
- `--rules` CLI flag: repeatable values, resolution, and errors.
- Merge algorithm and duplicate detection.

## Rule File Format

A rule file is a YAML document parsed as a RuleSet (see `specs/configuration.md`) with additional constraints:

- `rules` is required and must be non-empty. Rules follow `specs/regex-rules.md`.
- Every other RuleSet field must be absent. Presence of any of `include`, `exclude`, `failOn`, `concurrency`, `baseline`, `git`, `consoleColorsEnabled`, `ignoreFilesEnabled`, `ignoreFiles`, or `ruleFiles` is a validation error naming the offending field.
- Unknown top-level keys already fail per existing RuleSet parsing behavior.

Example rule file (`rules-go.yaml`):

```yaml
rules:
  - message: "TODO comment left in code"
    regex: "TODO|FIXME"
    severity: "notice"
    paths:
      - "**/*.go"
  - message: "panic outside main/test"
    regex: "\\bpanic\\("
    severity: "warning"
    paths:
      - "**/*.go"
    exclude:
      - "**/*_test.go"
```

## Data Model

### RuleSet field addition

- `ruleFiles` (list of string, optional): Ordered list of rule file paths loaded after the base config's own `rules`. Paths are resolved relative to the directory of the config file that declares them. Defined in `specs/configuration.md`.

### CLI flag addition

- `--rules` (repeatable string, optional): Ordered list of rule file paths loaded last, after base config and `ruleFiles`. Paths are resolved relative to the process working directory. Defined in `specs/cli-analyze.md`.

### Reference struct (Go)

```go
// internal/config
type RuleSet struct {
    // ... existing fields ...
    RuleFiles StringList `yaml:"ruleFiles,omitempty"`
}

// internal/cli
type CLIConfig struct {
    // ... existing fields ...
    RuleFilePaths []string
}
```

## Merge Semantics

- Effective rule list is the concatenation, in order:
  1. `rules` from the base config file.
  2. `rules` from each `ruleFiles` entry, in declared order.
  3. `rules` from each `--rules` flag value, in flag order.
- Appended rules keep their parsed severity/paths/exclude defaults unchanged (see `specs/regex-rules.md`).
- All other effective settings (include, exclude, failOn, git, baseline, ignore files, colors) come from the base config and CLI flags only. Rule files contribute rules and nothing else.
- Scanning iterates the merged list in this order; existing deterministic ordering guarantees are unchanged.

### Duplicate detection

- After merging, two rules with the same `(message, regex)` pair are a validation error.
- The error identifies the duplicated rule and both source files.
- Duplicates within a single file are the same error.

## Loading Workflow

1. Load and validate the base config (existing behavior, plus the new `ruleFiles` field validation).
2. For each `ruleFiles` entry, in order:
   - Resolve the path relative to the base config file's directory.
   - Read, parse, and validate as a rule file (format constraints above).
3. For each `--rules` value, in flag order:
   - Resolve the path relative to the process working directory.
   - Read, parse, and validate as a rule file.
4. Concatenate rules per Merge Semantics and run duplicate detection.
5. Any failure aborts with a single error message and exit code 1 before scanning starts.

## Validation

- `ruleFiles` values must be non-empty strings without duplicates when set.
- Duplicate paths across `ruleFiles` and `--rules` (after resolution) are a validation error; loading the same file twice is never intentional.
- A rule file that is missing, unreadable, or unparseable exits with code 1 and a single error message naming the file.
- A rule file without a non-empty `rules` list exits with code 1.
- A rule file containing any non-`rules` RuleSet field exits with code 1, naming the field.
- A rule file declaring `ruleFiles` exits with code 1 (nested rule files are not supported).
- Duplicate `(message, regex)` in the merged set exits with code 1.
- Rules inside rule files are validated per `specs/regex-rules.md`.

## Notes

- `init` (see `specs/cli-init.md`) is unchanged; it still writes a single starter config.
- Formatter output schemas are unchanged; rules from rule files are indistinguishable from base-config rules at output time.
- Baseline, Git mode, and ignore-file behaviors apply to the merged rule list exactly as they apply today.

## Verifications

- Base config with `ruleFiles: [rules-go.yaml]` scans with base rules followed by rule-file rules.
- `reglint analyze -c reglint.yaml --rules rules-go.yaml` produces the same rule list as declaring the file in `ruleFiles` (order aside).
- Multiple `--rules` flags load in flag order after `ruleFiles`.
- A rule file containing `failOn: error` exits with code 1 naming `failOn`.
- A duplicate `(message, regex)` across base config and a rule file exits with code 1 naming both sources.
- A missing rule file path exits with code 1 with a single error message.
- `ruleFiles` path is resolved relative to the config directory: running `reglint analyze -c conf/reglint.yaml` from the repo root resolves `conf/rules-go.yaml` for `ruleFiles: [rules-go.yaml]`.

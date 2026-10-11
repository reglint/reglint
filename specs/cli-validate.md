# Validate Command

Status: Implemented

## Overview

### Purpose

- Check a reglint config and its merged rules for correctness without scanning any files.
- Give a fast, deterministic pass/fail signal for config authoring and CI pre-flight.

### Goals

- Validate the same inputs `analyze` accepts: base config, its `ruleFiles`, and repeatable `--rules` packs.
- Report the merged rule count on success.
- Exit non-zero with a single actionable error on any config or rule error.

### Non-Goals

- Scanning files or rendering report output (use `analyze`).
- Interactive fixing or rewriting of configs.
- Standalone rule-pack validation: a base config is always required; `--rules` only attaches packs to it.

### Scope

- Command syntax, flags, defaults, and validation behavior for validate.
- Exit codes and error surface.

## Architecture

### Module/package layout (tree format)

```
cmd/
  reglint/
    main.go
internal/
  cli/
    validate.go
  config/
```

### Component diagram (ASCII)

```
[Validate Command] -> [Config Loader] -> [RuleSet Validation]
```

### Data flow summary

1. Parse flags.
2. Check the config path exists (same check as `analyze`).
3. Load and validate the config plus merged rule files via `config.LoadRuleSetWithFiles`.
4. Print a success message with the merged rule count, or a single error and exit 1.

## Data model

### Core Entities

ValidateConfig

- Definition: Parsed CLI inputs and resolved defaults for validate.
- Fields:
  - `ConfigPath` (string, required)
  - `RuleFilePaths` (list of string, optional)

Reference struct (Go):

```go
type ValidateConfig struct {
    ConfigPath    string
    RuleFilePaths []string
}
```

## Workflows

### Validate config (happy path)

1. Parse flags.
2. Check the config path exists, is a file, and is readable.
3. Call `config.LoadRuleSetWithFiles(cfg.ConfigPath, cfg.RuleFilePaths)`.
4. Print: `Config OK: <path> (<N> rules)` where N is the merged rule count.
5. Exit 0.

### Validation and errors

- The error surface is exactly what `LoadRuleSetWithFiles` returns (fail-fast single message); no error aggregation.
- Any parse, load, path, rule, or rule-file error exits with code 1 and a single error line on stdout.

### Exit codes

- `0`: config and merged rules are valid.
- `1`: configuration or load error.

No exit-2 semantics: validate never scans, so fail-on thresholds do not apply.

## Configuration

### Command syntax

```
reglint validate [flags]
```

### Flags

| Flag      | Type            | Required | Default              | Purpose                                                            |
| --------- | --------------- | -------- | -------------------- | ------------------------------------------------------------------ |
| `-c`      | string          | no       | `reglint-rules.yaml` | Path to the base config file.                                      |
| `--config`| string          | no       | `reglint-rules.yaml` | Long form of `-c`.                                                 |
| `--rules` | list of string  | no       | none                 | Repeatable extra rule file, resolved relative to the working directory. |

Rule files given via `--rules` load after the config's own `ruleFiles`, identical to `analyze`.

## Verifications

- `reglint validate -c <valid-config>` prints `Config OK: <path> (<N> rules)` and exits 0.
- `reglint validate -c <config-with-invalid-regex>` exits 1 and reports the offending rule.
- `reglint validate -c <missing-file>` exits 1 with `config file not found`.
- `reglint validate --rules <pack-with-settings>` exits 1 and names the invalid key.

## Appendices

### Examples

```
reglint validate
reglint validate -c configs/reglint-rules.yaml
reglint validate -c configs/reglint-rules.yaml --rules packs/go.yaml
```

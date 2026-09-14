# RegLint CLI

Status: Implemented

## Overview

### Purpose

- Provide the single CLI entrypoint for RegLint.
- Define shared CLI behavior and subcommand structure.

### Goals

- Stable, well-documented flags and exit codes.
- Clear validation errors and deterministic behavior.
- Support console, JSON, and SARIF output selection.
- Support optional baseline suppression for incremental adoption.
- Support baseline generation/regeneration from current findings.

### Non-Goals

- Interactive UI or TUI.
- Editing or generating rules files.
- Remote or daemonized scanning.

### Scope

- Command structure and shared behaviors (exit codes, global help).
- Delegated subcommand specs for details.

## Architecture

### Module/package layout (tree format)

```
cmd/
  reglint/
    main.go
internal/
  config/
  rules/
  scan/
  output/
```

### Component diagram (ASCII)

```
[CLI] -> [Config Loader] -> [Scan Service] -> [Output Writers]
```

### Data flow summary

1. Parse CLI args and resolve the subcommand.
2. Delegate to the subcommand handler.
3. Subcommand performs validation and work.
4. Exit with a deterministic code.

## Data model

### Core Entities

CLICommand

- Definition: A subcommand entry in the CLI.
- Fields:
  - `name` (string, required)
  - `aliases` (list of string, optional)
  - `handler` (function, required)

### Relationships

- Subcommand handlers implement the behavior defined in `specs/cli-analyze.md` and `specs/cli-init.md`.

### Persistence Notes

- No persistence. CLI is a single-shot execution.

## Workflows

### Resolve command (happy path)

1. Parse args and select a subcommand.
2. If the command is `analyze` or `analyse`, run analyze handler.
3. If the command is `init`, run init handler.
4. If no command is provided, print help and exit 1.

### Validation and errors

- Unknown command prints a single error message and exits with code 1.
- Subcommand-specific validation is defined in the subcommand specs.
- Analyze baseline behavior is defined in `specs/cli-analyze-baseline.md`.
- Analyze baseline regeneration behavior is defined in `specs/cli-analyze-baseline.md`.

### Exit codes

- `0`: success.
- `1`: configuration or runtime error.
- `2`: analyze command found matches at or above `--fail-on`.
- Exception: `analyze --write-baseline` returns `0` on successful baseline write.

## APIs

- CLI only. No network APIs.

## Client SDK Design

- No SDK. Use the CLI interface.

## Configuration

### Command syntax

```
reglint <command> [flags]

Commands:
  analyze (alias: analyse)
  init
  version
```

### Flags

- See `specs/cli-analyze.md` and `specs/cli-init.md`.

### Precedence

- Command-specific precedence is defined in the subcommand specs.

### Output behavior

- Command-specific output behavior is defined in the subcommand specs.

## Permissions

- No authentication or roles.

## Security Considerations

- Output may contain sensitive data. See formatter specs for redaction rules.

## Dependencies

- Standard library `flag` for parsing.
- Config and scan packages described in `specs/data-model.md`.

## Open Questions / Risks

- None.

## Verifications

- `reglint analyze --config reglint-rules.yaml` scans current directory and exits with code 0/2.
- `reglint init` writes `reglint-rules.yaml` in the current directory.

## Appendices

### Examples

```
reglint analyze --config configs/example.rules.yaml
reglint analyse -c configs/example.rules.yaml -f json --out-json /tmp/scan.json
reglint init --out configs/reglint-rules.yaml
```

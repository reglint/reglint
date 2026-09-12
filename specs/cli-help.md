# CLI Help Flag

Status: Implemented

## Overview

### Purpose

- Provide deterministic, discoverable `--help`/`-h` output for the RegLint CLI.
- Allow users to inspect commands and flags without triggering validation or scans.

### Goals

- `--help` and `-h` work for the root command and each subcommand.
- Help output is stable, machine-testable, and independent of filesystem state.
- Help requests exit with code `0` and do not perform any side effects.

### Non-Goals

- Man pages or Markdown documentation generation.
- Interactive prompts or TUI help.
- Localization or formatting themes.

### Scope

- Root help output for `reglint`.
- Subcommand help output for `analyze` (alias `analyse`), `init`, and `version`.
- Argument precedence, short-circuit behavior, and exit codes.

## Architecture

### Module/package layout (tree format)

```
cmd/
  reglint/
    main.go
internal/
  cli/
    cli.go
    analyze.go
    init.go
    help.go
```

### Component diagram (ASCII)

```
[CLI Args] -> [Help Detector] -> [Help Writer] -> [stdout]
           -> [Command Router] -> [Command Handler]
```

### Data flow summary

1. Inspect args to detect help intent and its context (root vs command).
2. If help is requested, render the corresponding help topic and exit 0.
3. Otherwise, route to the selected command handler.

## Data model

### Core Entities

HelpTopic

- Definition: A structured description of a help topic for a CLI command.
- Fields:
  - `name` (string, required): `root|analyze|init|version`.
  - `usage` (list of string, required): usage lines to print.
  - `aliases` (list of string, optional): command aliases.
  - `flags` (list of HelpFlag, required).

HelpFlag

- Definition: A help-visible CLI flag description.
- Fields:
  - `long` (string, required): e.g., `--config`.
  - `short` (string, optional): e.g., `-h`.
  - `type` (string, required): `string|bool|int`.
  - `default` (string, required): stringified default or `none`.
  - `description` (string, required).

Reference structs (Go):

```go
type HelpTopic struct {
    Name    string
    Usage   []string
    Aliases []string
    Flags   []HelpFlag
}

type HelpFlag struct {
    Long        string
    Short       string
    Type        string
    Default     string
    Description string
}
```

### Relationships

- `HelpTopic.flags` must include the flags defined in `specs/cli-analyze.md` and `specs/cli-init.md`, plus `-h/--help`.
- The `analyse` alias shares the `analyze` help topic.

### Persistence Notes

- No persistence. Help output is computed per invocation.

## Workflows

### Root help (happy path)

1. If the first argument is `-h` or `--help`, select the `root` help topic.
2. Render root help output.
3. Exit with code `0`.

### Command help (happy path)

1. If the first argument is `analyze`, `analyse`, `init`, or `version`, inspect remaining args for `-h` or `--help`.
2. If present, select the matching help topic.
3. Render command help output.
4. Exit with code `0`.

### Validation and errors

- Help requests short-circuit command parsing, config loading, and output validation.
- Unknown commands still print a single error message and exit `1`, even if `--help` is present.
- Help output is written to the CLI output writer (stdout by default).

### Exit codes

- `0`: help displayed.
- `1`: unknown command or invalid args (when help is not requested).
- `2`: analyze command fail-on threshold (unchanged by help).

## APIs

- CLI only. No network APIs.

## Client SDK Design

- No SDK. Use the CLI interface.

## Configuration

### Command syntax

```
reglint --help
reglint -h
reglint analyze --help
reglint analyse -h
reglint init --help
reglint version --help
```

### Help flag

| Flag        | Type | Required | Default | Purpose              |
| ----------- | ---- | -------- | ------- | -------------------- |
| `--help,-h` | bool | no       | `false` | Print help and exit. |

### Output behavior

Root help output must include, in order:

1. `Usage:` line.
2. Usage line `reglint <command> [flags]`.
3. `Commands:` section with `analyze (alias: analyse)`, `init`, and `version`.
4. `Flags:` section containing `-h, --help`.

Command help output must include, in order:

1. `Usage:` line.
2. Usage lines for the command (including alias for analyze).
3. `Flags:` section listing all flags defined in the command spec plus `-h, --help`.

Formatting rules:

- Each flag must be rendered on a single line in the form:
  `  <short>, <long> <type> (default <value>)  <description>`
- If a short form does not exist, omit it and keep the long form aligned.
- Use `none` as the default value when no default is defined.

## Permissions

- No authentication or roles.

## Security Considerations

- Help output must not access or reveal filesystem contents.
- Help output must not include match text or scan results.

## Dependencies

- Standard library `flag` may be used for parsing, but help output must follow this spec.

## Open Questions / Risks

- Help output must stay in sync with command specs when flags change.

## Verifications

- `reglint --help` exits `0` and prints the root usage and commands list.
- `reglint analyze --help` exits `0` and lists analyze flags including `--config/-c` and `--format/-f`.
- `reglint analyze --help` exits `0` and lists baseline-related `--baseline` flag.
- `reglint analyze --help` exits `0` and lists baseline generation `--write-baseline` flag.
- `reglint analyse -h` exits `0` and prints the analyze usage lines.
- `reglint init --help` exits `0` and lists `--out` and `--force`.
- `reglint version --help` exits `0` and prints the version usage line.
- `reglint bogus --help` exits `1` and prints `Unknown command: bogus` only.

## Appendices

### Examples

Root help:

```
Usage:
  reglint <command> [flags]

Commands:
  analyze (alias: analyse)
  init
  version

Flags:
  -h, --help bool (default false)  Print help and exit.
```

Analyze help:

```
Usage:
  reglint analyze [flags] [path ...]
  reglint analyse [flags] [path ...]

Flags:
  -h, --help bool (default false)  Print help and exit.
  -c, --config string (default reglint-rules.yaml)  Path to YAML rules config file.
  -f, --format string (default console)  Comma-separated list of formats.
  --out-json string (default none)  Output path for JSON results.
  --out-sarif string (default none)  Output path for SARIF results.
  --include string (default none)  Repeatable include glob.
  --exclude string (default none)  Repeatable exclude glob.
  --concurrency int (default GOMAXPROCS)  Worker count.
  --max-file-size int (default 5242880)  Maximum file size in bytes.
  --fail-on string (default none)  Fail if matches at or above severity.
  --baseline string (default none)  Baseline JSON path for suppression.
  --write-baseline bool (default false)  Generate/regenerate baseline from findings.
```

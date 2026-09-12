# Ignore Files Support

Status: Implemented

## Overview

### Purpose

- Add first-class support for repository ignore files so scans skip non-relevant paths without duplicating globs in RegLint config.
- Keep ignore behavior deterministic and reproducible across runs and platforms.

### Goals

- Support `.ignore`-style path filtering during `analyze` file discovery.
- Load and apply ignore rules from `.ignore`, and `.reglintignore` by default.
- Enable `.gitignore` filtering by default for all scan modes.
- Allow CLI and config overrides to disable ignore processing or customize ignore file names.
- Preserve existing include/exclude and per-rule filtering semantics.

### Non-Goals

- Reading Git global excludes (for example `core.excludesFile`) or `.git/info/exclude`.
- Full emulation of all Git internals beyond documented ignore syntax support in this spec.
- Changes to output formatter schemas and severity behavior.

### Scope

- Analyze command flags and RuleSet configuration for ignore behavior.
- Ignore file discovery, parsing, and matching during scan entry collection.
- Deterministic precedence between include/exclude filters and ignore rules.

## Architecture

### Module/package layout (tree format)

```
internal/
  cli/
    analyze.go
  config/
    model.go
    rules.go
  ignore/
    loader.go
    parser.go
    matcher.go
  scan/
    model.go
    engine.go
```

### Component diagram (ASCII)

```
[Analyze Flags + RuleSet]
           |
           v
[Ignore Settings Resolver]
           |
           v
[Ignore Loader/Parser] ---> [Ignore Matcher]
           |                      |
           +-----------> [File Walker] -> [Rule Matching]
```

### Data flow summary

1. Parse analyze flags and load RuleSet YAML.
2. Resolve effective ignore settings (defaults -> RuleSet -> CLI overrides).
3. Discover configured ignore files while collecting scan entries.
4. Parse ignore files into ordered rules with source metadata.
5. For each candidate path, apply include/exclude filters first, then ignore matcher.
6. Scan only selected files; rendering and exit code logic stay unchanged.

## Data model

### Core Entities

IgnoreSettings

- Definition: Effective controls for ignore support on a single analyze run.
- Fields:
  - `enabled` (bool, required, default `true`)
  - `files` (list of string, required, ordered): ignore file names, default `['.ignore', '.reglintignore']`

IgnoreRule

- Definition: One parsed ignore rule with origin metadata.
- Fields:
  - `baseDir` (string, required): root-relative directory that owns the ignore file.
  - `source` (string, required): root-relative ignore file path.
  - `line` (int, required): 1-based source line number.
  - `pattern` (string, required): normalized pattern text.
  - `negated` (bool, required): true for `!pattern` rules.
  - `directoryOnly` (bool, required): true for trailing `/` rules.

IgnoreMatcher

- Definition: Ordered, immutable matcher for a scan root.
- Behavior:
  - Evaluates all matching ignore rules in order.
  - Last matching rule wins.
  - Final state is ignored unless the winning rule is negated.

Reference structs (Go):

```go
type IgnoreSettings struct {
    Enabled bool
    Files   []string
}

type IgnoreRule struct {
    BaseDir       string
    Source        string
    Line          int
    Pattern       string
    Negated       bool
    DirectoryOnly bool
}
```

### Relationships

- `IgnoreSettings` is resolved by the CLI from RuleSet defaults and CLI flags.
- `ScanRequest` (see `specs/data-model.md`) is extended with effective ignore settings.
- Ignore filtering happens before per-rule path/exclude checks in rule execution.
- `Match` and formatter outputs are unchanged by this feature.
- `.gitignore` support is enabled by default and may be disabled with CLI/config controls.

### Persistence Notes

- No persistence. Ignore rules are loaded and compiled per command invocation.

## Workflows

### Resolve effective ignore settings

1. Start with defaults: `enabled=true`, `files=['.ignore', '.reglintignore']`.
2. Apply RuleSet overrides (`ignoreFilesEnabled`, `ignoreFiles`) when provided.
3. Apply CLI overrides:
   - `--no-ignore-files` forces `enabled=false`.
4. Validate final settings before scanning starts.

### Discover and parse ignore files

1. For each scan root directory, walk directories in deterministic lexical order.
2. In each directory, evaluate ignore file names in effective `files` order.
3. For each existing ignore file:
   - Read UTF-8 text and normalize line endings (`\r\n` -> `\n`).
   - Parse line-by-line into `IgnoreRule` values.
4. Keep global rule order deterministic:
   - parent directory before child directory,
   - within a directory, by `files` list order,
   - within a file, by line number.

### Ignore syntax support

- Blank lines are ignored.
- `#` starts a comment unless escaped.
- `\#` and `\!` represent literal leading `#` and `!`.
- Leading `!` negates a prior ignore decision (un-ignore).
- Leading `/` anchors the pattern to the ignore file directory.
- Trailing `/` matches directories only.
- Wildcards support `*`, `?`, and `**` path glob semantics.

### Path selection precedence

For each candidate file path (root-relative, slash-separated):

1. It must match at least one effective include glob.
2. If it matches any effective exclude glob, it is excluded.
3. If ignore support is enabled, evaluate ignore matcher (last match wins).
4. If ignored, do not scan file contents.
5. If selected, proceed with existing size/binary/readability checks.

Notes:

- Ignore negation cannot re-include a path already removed by include/exclude filters.
- Rule-level `paths` and `exclude` still apply during match evaluation.
- When `.gitignore` support is enabled, `.gitignore` is evaluated before `.ignore/.reglintignore`.
- If `.gitignore`, `.ignore` or `.reglintignore` produce conflicting decisions for the same path, `.ignore` and `.reglintignore` has the highest priority, followed by `.ignore` and then `.gitignore`.
- `.gitignore` cannot re-include paths excluded by earlier include/exclude decisions.
- `.gitignore` support does not imply reading Git global excludes (`core.excludesFile`) or `.git/info/exclude`.

### Validation and errors

- `ignoreFiles` values must be non-empty file names.
- Ignore file names must not contain path separators.
- Duplicate ignore file names are rejected.
- If a discovered ignore file cannot be read, analyze exits with code `1`.
- If an ignore pattern is invalid, analyze exits with code `1` and includes `<source>:<line>` in the error message.

## APIs

- CLI only. No network APIs.

## Client SDK Design

- No SDK changes. Feature is internal to CLI and scan service behavior.

## Configuration

### RuleSet additions

| Field                | Type           | Required | Default                         | Purpose                                                  |
| -------------------- | -------------- | -------- | ------------------------------- | -------------------------------------------------------- |
| `ignoreFilesEnabled` | bool           | no       | `true`                          | Enable/disable ignore file support globally.             |
| `ignoreFiles`        | list of string | no       | `['.ignore', '.reglintignore']` | Ordered ignore file names to evaluate in each directory. |

### Analyze flag additions

| Flag                | Type | Required | Default | Purpose                                                |
| ------------------- | ---- | -------- | ------- | ------------------------------------------------------ |
| `--no-ignore-files` | bool | no       | `false` | Disable ignore file loading and matching for this run. |
| `--no-gitignore`    | bool | no       | `false` | Disable `.gitignore` matching for this run.            |

### Precedence

- `--no-ignore-files` has highest precedence and disables ignore processing.
- Else, use RuleSet `ignoreFiles` when set.
- Else, use the default ignore file list.
- `.gitignore` processing is enabled by default and can be disabled with RuleSet `git.gitignoreEnabled: false` or `--no-gitignore`.

### YAML example

```yaml
ignoreFilesEnabled: true
ignoreFiles:
  - ".ignore"
  - ".reglintignore"

rules:
  - message: "Avoid hardcoded token: $1"
    regex: "token\\s*[:=]\\s*([A-Za-z0-9_-]+)"
```

## Permissions

- No authentication or roles.

## Security Considerations

- Ignore file contents may reveal repository structure; error messages must not dump full file contents.
- Path matching must use normalized, root-relative paths to avoid traversal ambiguities.
- Ignore parsing must be deterministic and free of OS-dependent path separator behavior.

## Dependencies

- `github.com/bmatcuk/doublestar/v4` for wildcard matching consistency.
- Standard library `path/filepath` and `strings` for normalization and parsing.

## Open Questions / Risks

- Very large repositories may need directory-pruning optimization after correctness is proven.

## Verifications

- A root `.ignore` file excludes matching files from scan results.
- A nested `.ignore` with `!` negation re-includes a file under its directory when include/exclude filters allow it.
- `reglint analyze --no-ignore-files ...` scans files that would otherwise be ignored.
- Invalid ignore pattern reports `<ignore-file>:<line>` and exits with code `1`.
- Runs with identical inputs produce identical file selection and output ordering.
- `.gitignore` filtering applies by default and is skipped with `--no-gitignore`.

## Appendices

### Example ignore files

```text
# root .ignore
dist/
generated/**

# keep one fixture
!generated/keep.txt
```

```text
# nested .reglintignore in src/
vendor/
```

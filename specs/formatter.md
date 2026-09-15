# Formatter Core

Status: Implemented

## Overview

### Purpose

- Define the shared formatter contract and decouple CLI output from format specifics.
- Provide a stable extension point for new output formats.

### Goals

- Consistent formatter interface and registry.
- Deterministic ordering for match lists.
- Clear separation of CLI output handling and formatter rendering.
- Explicit guidance for adding future formats.

### Non-Goals

- Define concrete output schemas (see per-formatter specs).
- Perform scan-time or runtime filesystem validation.
- Streaming output while scanning.

### Scope

- Formatter interface and registry responsibilities.
- Format identifiers used by the CLI.
- Shared rendering guidelines (ordering, sensitive output).
- Development/QA validation expectations.

## Architecture

### Module/package layout (tree format)

```
internal/
  output/
    formatter.go
    registry.go
    console.go
    json.go
    sarif.go
```

### Component diagram (ASCII)

```
[Analyze Command] -> [Formatter Registry] -> [Formatter] -> io.Writer
```

### Data flow summary

1. Parse the `--format` list in the CLI.
2. Resolve formatters from the registry.
3. CLI prepares output writers (stdout or files).
4. Call formatter `Write` with `ScanResult` and writer.
5. Propagate formatter errors to the CLI.

Notes:

- The `ScanResult` passed to formatters may be post-processed by CLI workflows (for example baseline suppression); formatter schemas and contracts remain unchanged.

## Data model

### Core Entities

Formatter

- Definition: Renderer for a single output format.
- Methods:
  - `Name() string`: Format identifier (lowercase).
  - `Write(result ScanResult, out io.Writer) error`.

FormatterRegistry

- Definition: Mapping of format identifiers to formatter implementations.
- Fields:
  - `formats` (map[string]Formatter)

FormatID

- Definition: CLI-visible identifier for a formatter.
- Values: `console`, `json`, `sarif`, `github` (extensible).

### Relationships

- `Formatter.Write` consumes `ScanResult` from `specs/data-model.md`.
- CLI selects formatters via `FormatID` values (see `specs/cli-analyze.md`).
- Per-formatter specs define output schemas and field details.

### Persistence Notes

- Formatters are stateless; they only write to an `io.Writer`.

## Workflows

### Resolve formatters (happy path)

1. Split `--format` by comma.
2. Trim whitespace and normalize to lowercase.
3. Look up each format in the registry.
4. If any format is unknown, return a single error.

### Render outputs

1. CLI opens output writers (stdout or files).
2. For each formatter in the requested order, call `Write`.
3. If any formatter returns an error, stop and return the error.

### No matches

- Formatters define how to render zero matches (see per-formatter specs).

## APIs

- Internal interface only. No public SDK.
- Suggested interface:

```go
type Formatter interface {
    Name() string
    Write(result ScanResult, out io.Writer) error
}
```

## Configuration

- CLI flag: `--format` uses `FormatID` values.
- Output paths are CLI concerns (see `specs/cli-analyze.md`).

## Permissions

- No authentication or roles.

## Security Considerations

- Treat `matchText` as sensitive; only emit it when a formatter spec explicitly requires it.
- Do not emit file contents or binary data unless a formatter spec explicitly requires it.

## Dependencies

- Standard library only for the core interface and registry.

## Verifications

- Format identifiers are stable and lowercase.
- Formatters do not perform filesystem validation during normal CLI runs.
- Tests/QA validate that any emitted absolute paths refer to existing files where applicable.

## Appendices

### Ordering (for match lists)

- Sort by `filePath` (ascending, byte-wise).
- Within a file, sort by `line` then `column`.

### Adding a new formatter

1. Create a new spec `specs/formatter-<name>.md`.
2. Implement a formatter in `internal/output/<name>.go`.
3. Register it in the formatter registry.
4. Update CLI docs and `specs/README.md`.

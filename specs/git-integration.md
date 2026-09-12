# Git Integration

Status: Implemented

## Overview

### Purpose

- Add optional Git-backed scan scoping to `analyze` without changing default non-Git behavior.
- Reuse repository metadata when requested to reduce scan scope and noise.
- Keep selection deterministic and reproducible across runs.

### Goals

- Keep `.gitignore` filtering enabled by default, with optional CLI/config opt-out.
- Support scanning only files in the staging area.
- Support scanning only files from a Git diff target.
- Support reporting matches only on lines added by the selected diff.
- Keep behavior unchanged when Git mode is not requested.
- Integrate through hook points to keep core scan logic minimally invasive.

### Non-Goals

- Require Git for regular scans.
- Read Git global excludes (for example `core.excludesFile`) or `.git/info/exclude`.
- Change formatter output schemas.
- Introduce network calls or remote Git providers.

### Scope

- Analyze command flags and RuleSet settings for Git mode.
- Git-backed candidate file and optional line-scope selection.
- Precedence with include/exclude, ignore files, and per-rule path filters.
- Runtime validation and deterministic error handling.

## Architecture

### Module/package layout (tree format)

```
internal/
  cli/
    analyze.go
  hooks/
    scan_hooks.go
  config/
    model.go
    loader.go
  git/
    adapter.go
    diff.go
    ignore.go
    model.go
  scan/
    model.go
    service.go
    engine.go
```

### Component diagram (ASCII)

```
[Analyze Flags + RuleSet]
           |
           v
[Git Settings Resolver]
           |
           v
[Scan Hooks Registry] ---> [Git Hook Provider] -----> [Changed File Set + Added Line Map]
           |
           v
[Scan Service] -> [Scan Engine] -> [Output Writers]
```

### Data flow summary

1. Parse `analyze` flags and load RuleSet.
2. Resolve effective Git settings (defaults -> RuleSet -> CLI overrides).
3. Register scan hooks for this run (Git hooks enabled only when Git mode is active).
4. If `git.mode=off`, Git hooks are no-op and filesystem discovery remains unchanged.
5. If `git.mode=staged|diff`, Git hooks validate capability and resolve changed files.
6. If `addedLinesOnly=true`, Git post-match hook resolves and enforces added-line sets.
7. Continue with standard scan pipeline and output rendering.

### Hook contracts

- `OnCapabilitiesCheck(ctx, request) -> error`
  - Runs before Git-dependent selection.
  - In `mode=off`: must be no-op.
  - In `mode=staged|diff`: validates Git executable and repository context.
- `BeforeCollectCandidates(ctx, request) -> CandidateScope`
  - Optionally narrows file candidates to staged/diff files.
  - No-op when Git mode is off.
- `BeforeIgnoreEvaluation(ctx, request) -> IgnoreAugmentation`
  - Optionally contributes `.gitignore` rules when enabled.
  - Must preserve existing `.ignore/.reglintignore` behavior and priority.
- `AfterMatch(ctx, match, request) -> keep|drop`
  - Optionally filters matches to added lines only.
  - No-op when `addedLinesOnly=false`.

Hook model requirements:

- Hooks are optional and composable; absence of Git hooks must preserve current behavior.
- Hook execution order must be deterministic.
- Hook failures are fatal only when the corresponding Git mode is active.

## Data model

### Core entities

GitSettings

- Definition: Effective controls for Git integration in one analyze run.
- Fields:
  - `mode` (string, required): `off|staged|diff`
  - `diffTarget` (string, optional): revision, commit, or range used by `diff` mode
  - `addedLinesOnly` (bool, required, default `false`)
  - `gitignoreEnabled` (bool, required, default `true`)

GitSelection

- Definition: Deterministic selection payload returned by Git adapter.
- Fields:
  - `files` (list of string, required): root-relative slash-separated files
  - `addedLinesByFile` (map string -> set of int, optional): 1-based added lines

### Relationships

- `GitSettings` is resolved by CLI from RuleSet + flags.
- `ScanRequest` receives optional `git` constraints (see `specs/data-model.md`).
- `Match` and `ScanResult` schemas remain unchanged.

## Workflows

### Resolve effective Git settings

1. Start with defaults:
   - `mode=off`
   - `diffTarget` unset
   - `addedLinesOnly=false`
   - `gitignoreEnabled=true`
2. Apply RuleSet `git` overrides when configured.
3. Apply CLI overrides (highest precedence).
4. Validate cross-field constraints before scanning starts.

### Candidate file selection

For each candidate file path (root-relative, slash-separated), selection order is:

1. If `mode=off`, use existing filesystem walk candidates.
2. If `mode=staged`, candidate set is files present in staging area.
3. If `mode=diff`, candidate set is files present in the selected diff target.
4. Normalize and lexicographically sort candidate file paths.
5. Apply global include globs.
6. Apply global exclude globs.
7. Apply ignore processing:
   - `.gitignore` when `gitignoreEnabled=true`.
   - `.ignore` / `.reglintignore` by existing ignore settings.
8. Apply per-rule `paths` and per-rule `exclude`.
9. Run size/binary/readability checks.

Notes:

- Ignore negation cannot re-include a path excluded by earlier include/exclude filtering.
- File selection must be deterministic for the same repository state and inputs.
- When `.gitignore`, `.ignore` or `.reglintignore` produce conflicting decisions for the same path, `.ignore` and `.reglintignore` has the highest priority, followed by `.ignore` and then `.gitignore`.

### Added-lines-only filtering

1. `addedLinesOnly=true` is valid only when `mode=staged|diff`.
2. Build `addedLinesByFile` from selected Git diff/staging context.
3. Report only matches whose `line` is present in `addedLinesByFile[filePath]`.
4. Deletions and context lines never qualify.
5. Files without added lines produce zero reported matches in this mode.

### Validation and errors

- In `mode=off`, Git availability and repository context must not be required.
- In `mode=staged|diff`, the following are fatal runtime errors (exit code `1`):
  - Git executable unavailable.
  - Current path is not in a Git repository.
  - Requested diff target is invalid or cannot be resolved.
  - Git metadata required for selection cannot be computed.
- Empty changed-file sets are valid and return normal scan success behavior.
- Any validation failure prints a single error message and exits with code `1`.
- Hook failures must surface as a single error message with exit code `1` in Git-enabled modes.

## Configuration

### RuleSet additions

| Field                  | Type   | Required | Default | Purpose                                                      |
| ---------------------- | ------ | -------- | ------- | ------------------------------------------------------------ |
| `git.mode`             | string | no       | `off`   | Select Git mode: `off`, `staged`, or `diff`.                 |
| `git.diff`             | string | no       | none    | Diff target/range for `git.mode=diff`.                       |
| `git.addedLinesOnly`   | bool   | no       | `false` | Restrict reported matches to added lines in Git diff output. |
| `git.gitignoreEnabled` | bool   | no       | `true`  | Enable `.gitignore` filtering across scans.                  |

### Analyze flag additions

| Flag                     | Type   | Required | Default | Purpose                                                  |
| ------------------------ | ------ | -------- | ------- | -------------------------------------------------------- |
| `--git-mode`             | string | no       | `off`   | Select Git mode: `off`, `staged`, or `diff`.             |
| `--git-diff`             | string | no       | none    | Diff target/range when `--git-mode=diff`.                |
| `--git-added-lines-only` | bool   | no       | `false` | Restrict reported matches to added lines in Git context. |
| `--no-gitignore`         | bool   | no       | `false` | Disable `.gitignore` filtering for this run.             |

### Precedence

- CLI Git flags override RuleSet `git.*` values.
- `--git-diff` implies effective Git mode `diff`.
- Existing non-Git precedence remains unchanged.
- Selection/filtering precedence is:
  - Git mode candidate selection -> include -> exclude -> `.gitignore` -> `.ignore` -> `.reglintignore` -> per-rule filters -> added-lines-only.

## Security Considerations

- Git integration must not log raw match text while resolving file or line selections.
- Errors should expose only actionable context (mode/target/path), not sensitive file contents.

## Dependencies

- Git CLI executable when `mode=staged|diff`.
- Existing glob and ignore matching dependencies remain unchanged.

## Verifications

- `git.mode=off` with no Git installed runs normal scans without Git-related failures.
- `git.mode=staged` with no Git installed exits `1` with a single Git-mode error.
- `git.mode=staged` outside a Git repo exits `1`.
- `git.mode=staged` scans only staged files.
- `git.mode=diff` scans only files from the requested diff target.
- Invalid `git.diff` target exits `1` with a single error.
- `git.addedLinesOnly=true` with `git.mode=diff` reports matches only on added lines.
- `git.addedLinesOnly=true` with `git.mode=off` is rejected at validation time.
- `.gitignore` filtering applies when enabled and is skipped with `--no-gitignore`.
- Include/exclude and per-rule filters still apply after Git candidate selection.
- Repeated runs with identical repo state and inputs produce identical file and match ordering.

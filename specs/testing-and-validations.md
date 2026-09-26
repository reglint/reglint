# Testing and Validations

## Overview

### Purpose

- Define validation rules for configuration, rules, and CLI inputs.
- Establish the automated test strategy for correctness and regression safety.
- Keep outputs consistent across console, JSON, and SARIF formats.

### Goals

- Fail fast on invalid configuration or rule definitions with clear errors.
- Ensure validation happens before any scan starts.
- Provide deterministic outputs so tests and CI are stable.
- Cover core workflows and edge cases with unit and integration tests.

### Non-Goals

- Performance benchmarking or load testing.
- Formal verification or proof of correctness.
- Defining CI pipelines beyond required test commands.

### Scope

- CLI-only execution (no services or APIs).
- Validation and testing of local scans, outputs, and exit codes.
- Local quality tooling (linting, security, architecture guardrails) and git hooks.

## Quality Tooling

### Tooling set

- `golangci-lint` (lint aggregator)
- `govulncheck` (Go vulnerability scanner)
- `go-arch-lint` (architecture guardrails)
- `gofmt` (formatting)
- `gremlins` (mutation testing)
- `lefthook` (git hooks runner)

### Local usage

Full suite:

```bash
make quality
```

Install git hooks:

```bash
lefthook install
```

Targeted runs:

```bash
make lint
make test
make test-race
make test-flaky
make coverage
make mutation
make security
make arch
```

### Configuration files

- `/.golangci.yml` - lint configuration
- `/.go-arch-lint.yml` - architecture rules
- `/lefthook.yml` - git hook configuration

### CI coverage

- `/.github/workflows/quality.yml` runs lint, and architecture checks.
- `/.github/workflows/quality.yml` runs mutation testing via `make mutation`.
- E2E policy requires a PR smoke gate and a nightly/manual full matrix (see `specs/e2e-test-suite.md`).

## Validation Rules

### RuleSet (configuration)

- YAML must parse successfully.
- `rules` is required and must be a non-empty list.
- `include` and `exclude` must be lists of strings when set.
- `failOn` must be one of `error|warning|notice|info` when set.
- `concurrency` must be a positive integer when set.
- `git.mode` must be one of `off|staged|diff` when set.
- `git.diff` is valid only when `git.mode=diff`.
- `git.mode=diff` requires `git.diff`.
- `git.addedLinesOnly=true` is valid only when `git.mode=staged|diff`.

### Rule (per entry)

- `message` is required and must be non-empty.
- `regex` is required and must compile with RE2.
- `severity` must be one of `error|warning|notice|info` when set.
- `paths` and `exclude` must be lists of strings when set.

### CLI (analyze)

- Unknown commands exit with code `1` and show a single error message.
- `--config` (`-c`) must point to a readable file.
- `--format` (`-f`) must be one of `console|json|sarif` (per formatter specs).
- Output paths must be writable when `--out-*` flags are set.
- `--baseline` must point to a readable JSON file when set.
- RuleSet `baseline` must be a non-empty string path when set.
- `--write-baseline` requires an effective baseline path from `--baseline` or RuleSet `baseline`.
- `--git-mode` must be one of `off|staged|diff`.
- `--git-diff` implies effective `--git-mode=diff`.
- Effective `--git-mode=diff` requires `--git-diff`.
- `--git-added-lines-only` is valid only with `--git-mode=staged|diff`.

### Baseline file

- JSON must parse successfully.
- `schemaVersion` is required and must be `1`.
- `entries` is required and must be a list.
- Each entry requires non-empty `filePath` and `message`.
- Each entry `count` must be a positive integer.
- Duplicate `(filePath, message)` keys are rejected.

### Baseline path precedence

- Effective baseline path precedence is `--baseline` > RuleSet `baseline` > unset.
- RuleSet `baseline` relative paths are resolved from the rules config directory.
- `--baseline` relative paths are resolved from the current working directory.

### Baseline generation

- `--write-baseline` generates baseline entries from full current findings (no suppression).
- Existing baseline file contents are ignored when `--write-baseline` is set.
- Baseline output is canonical JSON (`schemaVersion=1`, sorted `entries`).
- Baseline write errors fail with exit code `1`.
- In `--write-baseline` mode, baseline JSON parsing/validation is skipped for existing baseline files.
- In `--write-baseline` mode, successful baseline write returns exit code `0` regardless of matches or `--fail-on`.

### Runtime scan

- Invalid YAML or regex patterns exit with code `1`.
- File read errors are recorded and scanning continues.
- Matches at or above `failOn` return exit code `2`.
- Output writer errors exit with code `1`.
- In `git-mode=off`, missing Git executable must not fail the run.
- In `git-mode=staged|diff`, missing Git executable, non-repo context, or invalid diff target exit with code `1`.
- In Git-enabled runs, hook execution failures return a single error and exit with code `1`.

## Test Strategy

### Unit tests

- Config loader: YAML parsing, defaults, and validation errors.
- Rule compiler: regex compilation, severity normalization, message templates.
- Path filtering: include/exclude behavior with doublestar globs.
- Scan engine: line/column mapping and match aggregation.

### Integration tests

- CLI analyze happy path with fixture rules and sample files.
- Exit code behavior for invalid config, invalid regex, and `failOn` threshold.
- Exit code and output behavior with baseline suppression (equal/increase/decrease counts).
- Config-defined baseline path behavior and CLI override precedence.
- Baseline generation/regeneration behavior including overwrite and ignore-existing-baseline semantics.
- Baseline generation exit behavior (`0` on successful write even when matches exist).
- Output writers produce valid JSON and SARIF.
- Git mode `staged` scans only staged files.
- Git mode `diff` scans only files in the requested diff target.
- `--git-added-lines-only` reports only matches on added lines.
- `.gitignore` filtering can be enabled/disabled and behaves deterministically.
- In Git-enabled scans, conflicting ignore decisions resolve with `.ignore/.reglintignore` priority over `.gitignore`.
- With Git mode off, enabling hook infrastructure does not change scan outputs.

### End-to-end tests

- E2E tests validate process-level CLI behavior using a compiled `reglint` binary.
- E2E scenarios are fixture-driven and assert exit code, stdout/stderr contracts, and output artifacts.
- Scenario catalog, tiers, and execution contracts are defined in `specs/e2e-test-suite.md`.
- Tier policy:
  - Smoke tier (`PR` required): fast, deterministic, non-flaky core flow checks.
  - Full tier (`nightly/manual`): broader edge-case matrix, still deterministic.
- E2E failures must report scenario ID, fixture reference, and replay command.

### Golden tests

- Snapshot console/JSON/SARIF outputs for fixture directories.
- Keep outputs deterministic by sorting matches by file path, then line, then rule index.

### Regression tests

- Binary and large file skipping behavior.
- Regex capture group interpolation (`$0`, `$1`, `$$`).
- Mixed include/exclude rules and per-rule overrides.
- Baseline key stability for `(filePath, message)` with deterministic suppression ordering.
- Precedence stability across Git selection, include/exclude, ignore handling, per-rule filters, and added-lines filtering.
- Added-lines resolution parses diffs containing arbitrarily long lines (e.g. minified one-line files); underlying parse causes surface in the error message.
- Regression case for `.ignore/.reglintignore` priority over `.gitignore` on conflicting paths.

### Mutation testing

- Enforced via `gremlins` on local quality runs, pre-commit hooks, and CI.
- Minimum mutation score default: `0.8` (override with `MUTATION_SCORE_MIN`).
- Optional minimum mutant coverage default: `0` (override with `MUTATION_COVERAGE_MIN`).
- Optional diff target can be passed with `make mutation ARGS='--diff <target>'`.

### Coverage

- Minimum line coverage default: `90%` (override with `COVERAGE_MIN`).

## Test Data

- Use a top-level `testdata/` directory with:
  - `rules/` for sample RuleSet files.
  - `fixtures/` for code samples and edge cases.
  - `golden/` for expected outputs.

## Test Execution

- `make test` runs unit and integration tests.
- `make lint` is the canonical lint entrypoint and uses `/.golangci.yml`.
- `make test-flaky` reruns the full test suite with shuffle to catch flaky behavior (`FLAKY_COUNT` default `20`).
- Optional: `UPDATE_GOLDEN=1 make test` to refresh golden files.
- E2E command targets for implementation are:
  - `make test-e2e-smoke` for PR-required smoke coverage.
  - `make test-e2e` for nightly/manual full matrix coverage.
- Unit tests must enforce line coverage > 90% (exclude integration and testdata packages).
- Mutation testing must meet the minimum mutation score.

## Verifications

- `make test` passes.
- `reglint analyze --config testdata/rules/example.yaml ./testdata/fixtures` exits with `0` when below `failOn`.
- `reglint analyze --config testdata/rules/fail.yaml ./testdata/fixtures` exits with `2` when above `failOn`.
- `reglint analyze --config testdata/rules/example.yaml --git-mode off ./testdata/fixtures` does not require Git.
- `reglint analyze --config testdata/rules/example.yaml --git-mode staged` exits with `1` when Git is unavailable.
- E2E smoke scenarios pass in PR validation; full e2e matrix runs on nightly/manual workflow.

# End-to-End Test Suite

Status: Implemented

## Overview

### Purpose

- Define a comprehensive, black-box e2e test suite for `reglint` CLI behavior.
- Validate user-visible contracts at process boundaries: exit codes, stdout/stderr, and output artifacts.
- Establish deterministic test tiers for fast PR validation and broader nightly coverage.

### Goals

- Execute scenarios against a compiled binary (not in-process function calls).
- Cover critical `analyze` and `init` flows with fixture-driven test cases.
- Enforce deterministic assertions suitable for CI.
- Provide clear scenario IDs, replay commands, and failure diagnostics.

### Non-Goals

- Replace unit, integration, golden, or mutation tests.
- Validate internal package implementation details.
- Define formatter schemas beyond existing formatter specs.

### Scope

- Binary execution and process-level behavior validation.
- Fixture setup and scenario orchestration requirements.
- Tiering model: PR smoke suite and nightly/manual full matrix.
- CI-facing e2e verification requirements.

## Architecture

### Module/package layout (tree format)

```
cmd/
  reglint/
testdata/
  fixtures/
  rules/
  baseline/
  golden/
```

### Component diagram (ASCII)

```
[Build reglint binary] -> [Scenario Harness] -> [Fixture Workspace] -> [reglint process]
                                                     |
                                                     v
                              [Assert exit code + stdout/stderr + output files]
```

### Data flow summary

1. Build binary once for the test run.
2. Materialize fixture workspace for each scenario.
3. Run command with scenario args/env.
4. Capture process result (exit code, stdout, stderr).
5. Assert required contracts for that scenario.
6. Record scenario ID and replay command on failure.

## Data model

### Core Entities

E2EScenario

- Definition: Single executable contract test at CLI process boundary.
- Fields:
  - `id` (string, required): stable identifier (`E2E-SMOKE-*` or `E2E-FULL-*`).
  - `tier` (string, required): `smoke|full`.
  - `name` (string, required): human-readable scenario label.
  - `fixture` (string, required): fixture workspace reference.
  - `command` (string list, required): executable and args.
  - `env` (map[string]string, optional): per-scenario environment overrides.
  - `expectedExit` (int, required): expected process exit code.
  - `assertions` (list, required): output/file/structure assertions.

E2EAssertion

- Definition: Deterministic check attached to a scenario.
- Types:
  - `stdoutContains` / `stderrContains`
  - `stdoutNotContains` / `stderrNotContains`
  - `stdoutRegex` / `stderrRegex`
  - `fileExists` / `fileNotExists`
  - `jsonFieldEquals` / `sarifFieldEquals`

### Relationships

- Scenario command/flag semantics are defined by `specs/cli-analyze.md` and `specs/cli-init.md`.
- Formatter output schema details are defined by `specs/formatter-json.md` and `specs/formatter-sarif.md`.
- Test policy and quality gates are defined by `specs/testing-and-validations.md`.

## Workflows

### Local smoke workflow

1. Build the CLI binary.
2. Run smoke scenarios only.
3. Treat any failure as blocking for local merge readiness.

### Local full workflow

1. Build the CLI binary.
2. Run full matrix (smoke + full scenarios).
3. Collect scenario diagnostics for any failure.

### CI PR workflow

1. Run smoke tier on pull requests.
2. Keep runtime budget tight and deterministic.
3. Fail the PR gate on any smoke scenario failure.

### CI nightly/manual workflow

1. Run full matrix on schedule and manual dispatch.
2. Include broader edge cases and slower scenarios.
3. Emit scenario-level diagnostics for triage.

## Configuration

### Determinism controls

- Assertions must avoid clock-dependent values.
- Ordering-sensitive outputs must be validated against deterministic sort contracts.
- Scenario fixtures must be self-contained and network-independent.

### Environment controls

- `NO_COLOR` handling must be explicitly tested for console output.
- Git availability and repository context must be scenario-controlled.
- Relative path behavior must be tested with controlled working directories.

### Runtime policy

- Smoke tier must remain fast and stable for PR usage.
- Full tier may be slower but must remain deterministic and reproducible.
- Flaky behavior is not permitted in smoke tier.

## Scenario Matrix

### Smoke Tier (PR blocking)

- `E2E-SMOKE-001`: analyze happy path; valid config + fixtures -> exit `0` and deterministic summary.
- `E2E-SMOKE-002`: invalid config path/content -> exit `1` and single actionable error message.
- `E2E-SMOKE-003`: fail-on threshold exceeded -> exit `2`.
- `E2E-SMOKE-004`: no findings -> exit `0`.
- `E2E-SMOKE-005`: `NO_COLOR=1` disables ANSI in console output.
- `E2E-SMOKE-006`: path containing spaces scans successfully with correct path reporting.

### Full Tier (nightly/manual)

- `E2E-FULL-001`: baseline compare mode (`--baseline`) suppresses non-regressions.
- `E2E-FULL-002`: baseline generation mode (`--write-baseline`) overwrites target and exits `0`.
- `E2E-FULL-003`: baseline path precedence (`--baseline` over rules `baseline`).
- `E2E-FULL-004`: JSON-only format writes to stdout when `--out-json` is unset.
- `E2E-FULL-005`: SARIF-only format writes to stdout when `--out-sarif` is unset.
- `E2E-FULL-006`: multi-format runs require explicit `--out-json`/`--out-sarif` paths.
- `E2E-FULL-007`: `--git-mode off` works when Git executable is unavailable.
- `E2E-FULL-008`: `--git-mode staged` scans only staged files.
- `E2E-FULL-009`: `--git-mode diff --git-diff <target>` scans diff-selected files.
- `E2E-FULL-010`: `--git-added-lines-only` reports only matches on added lines.
- `E2E-FULL-011`: Git-enabled run outside repo exits `1` with single error.
- `E2E-FULL-012`: binary/oversized files are skipped with deterministic stats.
- `E2E-FULL-013`: unreadable files record errors while scan continues.
- `E2E-FULL-014`: ignore precedence is deterministic: `.reglintignore` > `.ignore` > `.gitignore`.
- `E2E-FULL-015`: repeated runs over same fixture produce stable ordering.

## Security Considerations

- E2E assertions must treat `matchText` as sensitive.
- Failure output and diagnostics must not require exposing raw secret-like match content.

## Test Execution

- Required commands for implementation phase:

```bash
make build
make test-e2e-smoke
make test-e2e
```

Notes:

- `make test-e2e-smoke` is the PR-required tier.
- `make test-e2e` is the comprehensive full tier for nightly/manual execution.

## Verifications

- Every e2e scenario has a unique ID and deterministic assertions.
- Smoke scenarios are sufficient to detect regressions in core user flows.
- Full scenarios cover baseline, formatter, git-mode, ignore, and file-handling edges.
- CI policy is explicit: PR smoke gate + nightly/manual full matrix.
- Failures report scenario ID, fixture path, and a replayable command.

## Appendices

### Scenario ID naming

- Smoke: `E2E-SMOKE-###`
- Full: `E2E-FULL-###`

### Cross-reference map

- Analyze semantics: `specs/cli-analyze.md`
- Init semantics: `specs/cli-init.md`
- Testing policy: `specs/testing-and-validations.md`

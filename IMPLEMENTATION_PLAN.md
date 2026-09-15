# Implementation Plan (formatter-github)

**Status:** Not started (0/26 checklist items) — spec `specs/formatter-github.md` committed at `30252e9` (Draft); formatter, registry/CLI wiring, tests, and docs all pending.
**Last Updated:** 2026-09-15
**Primary Specs:** `specs/formatter-github.md` (related: `specs/formatter.md`, `specs/formatter-sarif.md`, `specs/cli-analyze.md`, `specs/testing-and-validations.md`)

## Quick Reference

| System / Subsystem | Specs | Modules / Packages | Artifacts | Status |
| --- | --- | --- | --- | --- |
| GitHub formatter (workflow commands, caps, escaping) | `specs/formatter-github.md` | `internal/output/github.go` (new) | `WriteGitHub`, `GitHubFormatter{Rules}` | ⬜ Missing |
| Formatter registry + CLI format resolution | `specs/formatter-github.md`, `specs/formatter.md` | `internal/cli/analyze.go` (`parseFormats`, `defaultOutputRegistry`, `renderFormat`) | `github` accepted by `--format` | ⬜ Missing |
| Output routing (stdout-only, no `--out-github`) | `specs/formatter-github.md`, `specs/cli-analyze.md` | `internal/cli/analyze.go` (`validateOutputPaths`) | No new flag; multi-format rule unchanged | ✅ Verified no change needed |
| Shared mappings reuse (rule id, path normalization, ordering) | `specs/formatter-github.md`, `specs/formatter-sarif.md` | `internal/output/sarif.go` (`ruleIDForIndex`, `normalizePath`), `internal/output/console.go` (`severityRank`) | Reused as-is | ✅ Implemented (sources exist) |
| Unit + golden tests | `specs/formatter-github.md`, `specs/testing-and-validations.md` | `internal/output/github_test.go`, `internal/output/golden_test.go`, `testdata/golden/github.txt` (new) | Golden + targeted cases | ⬜ Missing |
| CLI + process tests | `specs/testing-and-validations.md` | `internal/cli/analyze_output_test.go`, `internal/cli/cli_test.go`, `cmd/reglint/main_test.go` | `--format github` contracts | ⬜ Missing |
| Docs (README, CI example) | `specs/formatter-github.md` | `README.md` | Formats list + PR annotations example | ⬜ Missing |
| Spec deltas for `github` FormatID | `specs/cli-analyze.md`, `specs/cli.md`, `specs/formatter.md` | spec files only | Formats enum updates | ⬜ Blocked pending user approval (AGENTS.md: update specs only when asked) |

## Phase 1: Formatter core in `internal/output`

**Goal:** Implement the GitHub workflow-command formatter per spec, reusing existing mappings.
**Status:** Not started
**Paths:** `internal/output/github.go`, `internal/output/sarif.go`, `internal/output/console.go`
**Reference pattern:** `internal/output/sarif.go` (`WriteSARIF`/`SARIFFormatter` shape), `internal/output/console.go:228` (`severityRank`)

### 1.1 Implementation checklist (TDD: write failing tests first per AGENTS.md)

- [ ] Add `WriteGitHub(result scan.Result, ruleSet []rules.Rule, out io.Writer) error` and `GitHubFormatter{Rules []rules.Rule}` with `Name() == "github"` (mirror `SARIFFormatter`, sarif.go:88-100).
- [ ] Escape `%`→`%25`, `\n`→`%0A`, `\r`→`%0D` everywhere; additionally `:`→`%3A`, `,`→`%2C` in property values only.
- [ ] Reuse `ruleIDForIndex` (sarif.go:108) for `title=RC%04d` and `normalizePath` (sarif.go:102) for `file=`.
- [ ] Severity map: `error→error`, `warning→warning`, `notice→notice`, `info→notice`.
- [ ] Emit `::{level} file={file},line={line},col={col},title={title}::{message}` with fixed property order; each line `\n`-terminated.
- [ ] Escape `%`→`%25`, `\n`→`%0A`, `\r`→%0D` everywhere; additionally `:`→`%3A`, `,`→`%2C` in property values only.
- [ ] Enforce caps: first 10 rendered lines per level (`error`/`warning`/`notice`) in canonical order.
- [ ] Emit exactly one `:::notice title=RegLint::{shown} of {total} matches shown; ...` line iff any match was dropped.
- [ ] Zero matches → zero output lines. No ANSI sequences, no raw `matchText`.

**Definition of Done**

- `go test ./internal/output` green; all spec Verifications bullets (formatter-github.md:149-159) covered by tests.
- No new dependencies (stdlib only, spec Dependencies section).

**Risks/Dependencies**

- Sort comparator would become a 4th inline copy (console/json/sarif/github); follow existing per-formatter convention for now, note consolidation candidate (`ponytail:` comment allowed) without broad refactor.

## Phase 2: Registry and CLI wiring

**Goal:** Accept `--format github`, render to stdout, keep exit-code behavior untouched.
**Status:** Not started
**Paths:** `internal/cli/analyze.go`, `internal/cli/help.go`
**Reference pattern:** `internal/cli/analyze.go:757-766` (`renderFormat` switch), `:742-748` (`defaultOutputRegistry`)

### 2.1 Wiring checklist

- [ ] Register `output.GitHubFormatter{Rules: ruleset}` in `defaultOutputRegistry` (analyze.go:742-748).
- [ ] Register `output.GitHubFormatter{}` in `parseFormats` validation registry (analyze.go:196-200).
- [ ] Add `case "github":` to `renderFormat` → `formatter.Write(result, out)` (stdout buffer always; no file branch, unlike json/sarif).
- [ ] Confirm `validateOutputPaths` (analyze.go:273-292) needs no change: `github` has no out-flag requirement; `--format github,sarif --out-sarif x.sarif` and `--format github,json --out-json x.json` validate; `--out-github` remains an unknown-flag error by design (spec Configuration section).
- [ ] Help text: `--format` description (help.go:134-140) says "Comma-separated list of formats." with no enum — verify no update required; add none unless drift found.

**Definition of Done**

- `reglint analyze --format github` writes annotations to stdout; `--fail-on` exit codes identical to other formats (spec Workflows/Error cases).

**Risks/Dependencies**

- None beyond Phase 1.

## Phase 3: Tests

**Goal:** Lock spec contracts at unit, CLI, and process level.
**Status:** Not started
**Paths:** `internal/output/github_test.go`, `internal/output/golden_test.go`, `internal/output/ansi_assertions_test.go`, `internal/cli/analyze_output_test.go`, `internal/cli/cli_test.go`, `cmd/reglint/main_test.go`, `testdata/golden/`
**Reference pattern:** `internal/output/golden_test.go` (`assertGoldenBytes`, `testdata/golden/`), `internal/cli/analyze_output_test.go:52-73` (stdout/multi-format contracts)

### 3.1 Unit/output tests

- [ ] Golden: `TestGoldenGitHubOutput` with `testdata/golden/github.txt` (extend `golden_test.go`; reuse `goldenSampleResult` or a github-specific fixture).
- [ ] Escaping: `%`, newline, CR in message and in property values (`:`/`,` percent-encoded only in properties).
- [ ] Severity map incl. `info→notice`; rule-id mapping from ruleset index.
- [ ] Caps: 11+ matches per level → exactly 10 rendered; summary line emitted once iff dropped; no summary at exactly 10/10/10.
- [ ] Zero matches → empty output; ordering matches shared ordering; no ANSI; formatter error propagates as `error`.

### 3.2 CLI + process tests

- [ ] `parseFormats` accepts `github`; unknown formats still rejected (extend existing parse tests in `internal/cli/cli_test.go`).
- [ ] `renderFormat` github-to-stdout and `github,sarif` combination (pattern: `TestWriteSARIFOutputToStdout`, `analyze_output_test.go:62`).
- [ ] Process-level run asserting stdout annotation lines end-to-end (pattern: `cmd/reglint/main_test.go` SARIF output tests, e.g. `:784`).

**Definition of Done**

- Targeted: `go test ./internal/output ./internal/cli`; suite: `make test`; quality gates per AGENTS.md (`make quality` before close; `make mutation` only at final stage).

**Risks/Dependencies**

- Golden file updates are additive; existing goldens untouched.

## Phase 4: Docs and spec-index alignment

**Goal:** Users can discover the format; spec references stay coherent.
**Status:** Not started
**Paths:** `README.md`, `specs/README.md` (already indexed), `specs/cli-analyze.md`, `specs/cli.md`, `specs/formatter.md`

### 4.1 Docs checklist

- [ ] README.md:7 formats list; README Output section (~:150-164) add `github` stdout rule (stdout even alongside other formats — no out flag); add PR-annotations CI example near the SARIF workflow example (~:250-298), ideally with `--git-mode=diff --git-added-lines-only` per spec notes.
- [ ] Spec deltas (formats enums) in `specs/cli-analyze.md:85,155,199`, `specs/cli.md:16`, `specs/formatter.md:84` — **[ ] pending user approval**: AGENTS.md says update specs only when asked.

**Definition of Done**

- README examples run as printed against the built binary (`make build`).

**Risks/Dependencies**

- Blocked item: spec enum deltas (4.1 second bullet).

## Phase 5: Final verification

**Goal:** Prove end-to-end behavior and close out.
**Status:** Not started
**Paths:** repo root

### 5.1 Verification checklist

- [ ] `make build` + `make run ARGS='analyze --config configs/example.rules.yaml --format github'` — inspect annotation lines.
- [ ] `make analyze-example` / `make analyze-fail` unchanged exit semantics with `--format github`.
- [ ] `make quality` (lint, coverage ≥90%, security, arch).
- [ ] `make mutation` (final stage only, per AGENTS.md).

**Definition of Done**

- Verification Log entries recorded below with real command results.

**Risks/Dependencies**

- None.

## Verification Log

- 2026-09-15: `git log --oneline -n 5 -- specs/formatter-github.md` - latest (and only) spec commit is `30252e9` "Add GitHub Formatter specification", current HEAD; tests run: none (planning mode); bug fixes discovered: none; files touched: none.
- 2026-09-15: read `specs/formatter-github.md` - confirmed locked design: FormatID `github`, stdout-only (no `--out-github`), syntax `::{level} file=,line=,col=,title=::{message}`, severity map error/error warning/warning notice/notice info/notice, RC%04d rule ids, 10-per-level caps + single summary notice, shared ordering, escaping table; tests run: none; bug fixes discovered: none; files touched: none.
- 2026-09-15: glob `internal/**/*.go` + grep `github|workflow command|::(error|warning|notice)` across `internal`, `cmd`, `README.md` - confirmed zero implementation: no `internal/output/github.go`, no workflow-command emitters, no `github` format references; tests run: none; bug fixes discovered: none; files touched: none.
- 2026-09-15: read `internal/cli/analyze.go:176-209,273-307,714-819`, `internal/output/{formatter,registry,sarif,console}.go` - mapped all wiring points: `parseFormats` registry (analyze.go:196-200), `defaultOutputRegistry` (analyze.go:742-748), `renderFormat` switch (analyze.go:757-766), `validateOutputPaths` multi-format rule (analyze.go:273-292, no change needed for github); confirmed reusable `ruleIDForIndex` (sarif.go:108), `normalizePath` (sarif.go:102), `severityRank` (console.go:228); tests run: none; bug fixes discovered: none; files touched: none.
- 2026-09-15: read test conventions `internal/output/golden_test.go`, `internal/output/ansi_assertions_test.go`, `internal/cli/analyze_output_test.go`, grep `cmd/reglint/main_test.go` - identified golden/stdout/multi-format/process test patterns to extend; tests run: none; bug fixes discovered: none; files touched: none.
- 2026-09-15: `git status --short` - clean tree before plan rewrite; tests run: none; bug fixes discovered: none; files touched: none.
- 2026-09-15: regenerated `IMPLEMENTATION_PLAN.md` - replaced stale gitignore-scope plan (dated 2026-03-12) with formatter-github plan; tests run: none (plan-only update); bug fixes discovered: none; files touched: `IMPLEMENTATION_PLAN.md`.

## Summary

| Phase | Status |
| --- | --- |
| Phase 1: Formatter core in `internal/output` | Not started |
| Phase 2: Registry and CLI wiring | Not started |
| Phase 3: Tests | Not started |
| Phase 4: Docs and spec-index alignment | Not started (spec deltas blocked on user approval) |
| Phase 5: Final verification | Not started |

**Remaining effort:** Phases 1-3 are the core (one new file `internal/output/github.go` + three wiring points in `internal/cli/analyze.go` + tests); Phase 4 is small README work with one blocked spec-delta item; Phase 5 is gate runs.

## Known Existing Work

- `specs/formatter-github.md` is complete (Draft) and indexed in `specs/README.md:39`; design decisions are locked in the spec — implement to it, do not re-litigate.
- Formatter interface (`internal/output/formatter.go`) and `Registry` (`internal/output/registry.go`) already support adding a formatter with zero registry changes.
- `internal/output/sarif.go` already provides `ruleIDForIndex` (RC%04d) and `normalizePath` — spec mandates identical mappings; reuse, do not duplicate.
- `internal/output/console.go` already provides `severityRank` for the shared ordering.
- Golden-test infrastructure (`assertGoldenBytes`, `testdata/golden/`) and ANSI-free assertion helpers already exist for formatter testing.
- Multi-format stdout/file routing rules for json/sarif already exist in `validateOutputPaths`/`write*Output` — `github` intentionally has no file branch.

## Manual Deployment Tasks

- Manual QA in a real GitHub repository: run a `pull_request` workflow using `--format github` and confirm annotations appear in the run summary and inline in the PR Files changed view (spec Verifications, formatter-github.md:159). Cannot be verified locally.
- Optional follow-up (not deployment-blocking): decide whether reglint-action marketplace workflow docs should mention `--format github` vs reviewdog/SARIF (spec Non-Goals defers this to `specs/release-process.md`).

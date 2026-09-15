# Implementation Plan (formatter-github)

**Status:** In progress (22/28 checklist items) — Phases 1–2 formatter core, registry/CLI wiring, and all unit/CLI/process tests landed (`1930576`, `e8598e2`); docs and spec deltas pending.
**Last Updated:** 2026-09-15
**Primary Specs:** `specs/formatter-github.md` (related: `specs/formatter.md`, `specs/formatter-sarif.md`, `specs/cli-analyze.md`, `specs/testing-and-validations.md`)

## Quick Reference

| System / Subsystem | Specs | Modules / Packages | Artifacts | Status |
| --- | --- | --- | --- | --- |
| GitHub formatter (workflow commands, caps, escaping) | `specs/formatter-github.md` | `internal/output/github.go` (new) | `WriteGitHub`, `GitHubFormatter{Rules}` | ✅ Done (`1930576`) |
| Formatter registry + CLI format resolution | `specs/formatter-github.md`, `specs/formatter.md` | `internal/cli/analyze.go` (`parseFormats`, `defaultOutputRegistry`, `renderFormat`) | `github` accepted by `--format` | ✅ Done (`e8598e2`) |
| Output routing (stdout-only, no `--out-github`) | `specs/formatter-github.md`, `specs/cli-analyze.md` | `internal/cli/analyze.go` (`validateOutputPaths`) | No new flag; multi-format rule unchanged | ✅ Verified no change needed |
| Shared mappings reuse (rule id, path normalization, ordering) | `specs/formatter-github.md`, `specs/formatter-sarif.md` | `internal/output/sarif.go` (`ruleIDForIndex`, `normalizePath`), `internal/output/console.go` (`severityRank`) | Reused as-is | ✅ Implemented (sources exist) |
| Unit + golden tests | `specs/formatter-github.md`, `specs/testing-and-validations.md` | `internal/output/github_test.go`, `testdata/golden/github.txt` (new) | Golden + targeted cases | ✅ Done (`1930576`, in `github_test.go`; `golden_test.go` untouched) |
| CLI + process tests | `specs/testing-and-validations.md` | `internal/cli/analyze_output_test.go`, `internal/cli/cli_test.go`, `cmd/reglint/main_test.go` | `--format github` contracts | ✅ Done (`e8598e2`, in `analyze_output_test.go`/`analyze_handle_test.go`/`main_test.go`; `cli_test.go` untouched) |
| Docs (README, CI example) | `specs/formatter-github.md` | `README.md` | Formats list + PR annotations example | ⬜ Missing |
| Spec deltas for `github` FormatID | `specs/cli-analyze.md`, `specs/cli.md`, `specs/formatter.md` | spec files only | Formats enum updates | ⬜ Blocked pending user approval (AGENTS.md: update specs only when asked) |

## Phase 1: Formatter core in `internal/output`

**Goal:** Implement the GitHub workflow-command formatter per spec, reusing existing mappings.
**Status:** Done (2026-09-15)
**Paths:** `internal/output/github.go`, `internal/output/sarif.go`, `internal/output/console.go`
**Reference pattern:** `internal/output/sarif.go` (`WriteSARIF`/`SARIFFormatter` shape), `internal/output/console.go:228` (`severityRank`)

### 1.1 Implementation checklist (TDD: write failing tests first per AGENTS.md)

- [x] Add `WriteGitHub(result scan.Result, ruleSet []rules.Rule, out io.Writer) error` and `GitHubFormatter{Rules []rules.Rule}` with `Name() == "github"` (mirror `SARIFFormatter`, sarif.go:88-100). `ruleSet` kept per spec signature but unused (rule ids derive from `match.RuleIndex`), so param is `_`.
- [x] Escape `%`→`%25`, `\n`→`%0A`, `\r`→`%0D` everywhere; additionally `:`→`%3A`, `,`→`%2C` in property values only.
- [x] Reuse `ruleIDForIndex` (sarif.go:108) for `title=RC%04d` and `normalizePath` (sarif.go:102) for `file=`.
- [x] Severity map: `error→error`, `warning→warning`, `notice→notice`, `info→notice`.
- [x] Emit `::{level} file={file},line={line},col={col},title={title}::{message}` with fixed property order; each line `\n`-terminated.
- [x] (duplicate of escaping item above)
- [x] Enforce caps: first 10 rendered lines per level (`error`/`warning`/`notice`) in canonical order.
- [x] Emit exactly one `:::notice title=RegLint::{shown} of {total} matches shown; ...` line iff any match was dropped.
- [x] Zero matches → zero output lines. No ANSI sequences, no raw `matchText`.

**Definition of Done**

- `go test ./internal/output` green; all spec Verifications bullets (formatter-github.md:149-159) covered by tests.
- No new dependencies (stdlib only, spec Dependencies section).

**Risks/Dependencies**

- Sort comparator would become a 4th inline copy (console/json/sarif/github); follow existing per-formatter convention for now, note consolidation candidate (`ponytail:` comment allowed) without broad refactor.

## Phase 2: Registry and CLI wiring

**Goal:** Accept `--format github`, render to stdout, keep exit-code behavior untouched.
**Status:** Done (2026-09-15)
**Paths:** `internal/cli/analyze.go`, `internal/cli/help.go`
**Reference pattern:** `internal/cli/analyze.go:757-766` (`renderFormat` switch), `:742-748` (`defaultOutputRegistry`)

### 2.1 Wiring checklist

- [x] Register `output.GitHubFormatter{Rules: ruleset}` in `defaultOutputRegistry` (analyze.go:742-748).
- [x] Register `output.GitHubFormatter{}` in `parseFormats` validation registry (analyze.go:196-200).
- [x] Add `case "github":` to `renderFormat` → `formatter.Write(result, out)` (stdout buffer always; no file branch, unlike json/sarif).
- [x] Confirm `validateOutputPaths` (analyze.go:273-292) needs no change: `github` has no out-flag requirement; `--format github,sarif --out-sarif x.sarif` and `--format github,json --out-json x.json` validate; `--out-github` remains an unknown-flag error by design (spec Configuration section).
- [x] Help text: `--format` description (help.go:134-140) says "Comma-separated list of formats." with no enum — verified no drift; no change made.

**Definition of Done**

- `reglint analyze --format github` writes annotations to stdout; `--fail-on` exit codes identical to other formats (spec Workflows/Error cases).

**Risks/Dependencies**

- None beyond Phase 1.

## Phase 3: Tests

**Goal:** Lock spec contracts at unit, CLI, and process level.
**Status:** Done — 3.1 unit/output tests done (`1930576`); 3.2 CLI/process tests done (`e8598e2`)
**Paths:** `internal/output/github_test.go`, `internal/output/golden_test.go`, `internal/output/ansi_assertions_test.go`, `internal/cli/analyze_output_test.go`, `internal/cli/cli_test.go`, `cmd/reglint/main_test.go`, `testdata/golden/`
**Reference pattern:** `internal/output/golden_test.go` (`assertGoldenBytes`, `testdata/golden/`), `internal/cli/analyze_output_test.go:52-73` (stdout/multi-format contracts)

### 3.1 Unit/output tests

- [x] Golden: `TestGoldenGitHubOutput` with `testdata/golden/github.txt` (lives in `github_test.go`; reuses `goldenSarifResult`/`goldenSarifRules` fixtures since they set `RuleIndex`/`MatchText`).
- [x] Escaping: `%`, newline, CR in message and in property values (`:`/`,` percent-encoded only in properties).
- [x] Severity map incl. `info→notice`; rule-id mapping from ruleset index.
- [x] Caps: 11+ matches per level → exactly 10 rendered; summary line emitted once iff dropped; no summary at exactly 10/10/10.
- [x] Zero matches → empty output; ordering matches shared ordering; no ANSI; formatter error propagates as `error`.

### 3.2 CLI + process tests

- [x] `parseFormats` accepts `github`; unknown formats still rejected (acceptance via `TestHandleAnalyzeWritesGitHubAnnotations`, analyze_handle_test.go:386; rejection via existing `TestHandleAnalyzeReturnsErrorWhenFormatsInvalid`).
- [x] `renderFormat` github-to-stdout and `github,sarif` combination (pattern: `TestWriteSARIFOutputToStdout`, `analyze_output_test.go:62`).
- [x] Process-level run asserting stdout annotation lines end-to-end (pattern: `cmd/reglint/main_test.go` SARIF output tests, e.g. `:784`).

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
- 2026-09-15: `go test ./internal/output/ -run 'TestGitHub|TestGoldenGitHub'` (RED) - initial run failed to compile (`WriteGitHub`/`GitHubFormatter` undefined), confirming tests exercise missing functionality; bug fixes discovered: none; files touched: `internal/output/github_test.go`.
- 2026-09-15: `go test ./internal/output/ -run 'TestGitHub|TestGoldenGitHub' -v` (GREEN) - 7/7 pass: golden, escaping (`%25`/`%0A`/`%0D` + `%3A`/`%2C` in properties), severity map incl. `info→notice`, caps (31 lines, exact summary text), no-summary-at-exact-cap, zero matches, writer-error propagation; bug fixes discovered: composite literal needed parens in `if` condition (test fix, not product); files touched: `internal/output/github.go`, `internal/output/github_test.go`, `testdata/golden/github.txt`.
- 2026-09-15: `go test ./internal/output/ -cover` - 93.9% statements (above 90% gate); `gofmt -l` and `go vet` clean; bug fixes discovered: none; files touched: none.
- 2026-09-15: `make test` (`go test ./...`) - all 10 packages ok; bug fixes discovered: none; files touched: none.
- 2026-09-15: pre-commit `golangci-lint` - initial commit attempt blocked (2x `lll` line length, 1x `revive` unused parameter); fixed by splitting summary-string literals and renaming `ruleSet` param to `_`; re-run clean; committed as `1930576` (3 files, 295 insertions); bug fixes discovered: none; files touched: `internal/output/github.go`, `internal/output/github_test.go`, `testdata/golden/github.txt`.
- 2026-09-15: `go test ./internal/cli/ -run 'GitHub'`, `go test ./cmd/reglint/ -run 'GitHubFormat'` (RED) - initial run failed: `invalid format: github` at render level, exit code 1 at process level, confirming tests exercise missing wiring; bug fixes discovered: none; files touched: `internal/cli/analyze_output_test.go`, `internal/cli/analyze_handle_test.go`, `cmd/reglint/main_test.go`.
- 2026-09-15: same targeted runs (GREEN) - 4/4 pass: render github-to-stdout exact annotation line, github+sarif out-file combination, HandleAnalyze end-to-end `--format github` exit 0 with exact stdout, process-level run with ANSI-free exact output; bug fixes discovered: two edit hunks initially swallowed closing `)`/`if err` lines (syntax breaks, restored); files touched: `internal/cli/analyze.go`, `internal/cli/analyze_output_test.go`, `internal/cli/analyze_handle_test.go`, `cmd/reglint/main_test.go`.
- 2026-09-15: `make test` - all packages ok; `make build` + `./bin/reglint analyze --config testdata/rules/example.yaml --format github testdata/fixtures` - prints `::error file=sample.txt,line=1,col=1,title=RC0001::Found token token=abc`, exit 0; `--format github,sarif --out-sarif` accepted; `--out-github` rejected (`flag provided but not defined`); help.go format description re-checked, no drift; bug fixes discovered: none; files touched: none.
- 2026-09-15: `make quality` (test, lint, test-race, test-flaky, coverage ≥90% gate, mutation, gosec 0 issues, go-arch-lint OK) - all gates green; committed as `e8598e2` (4 files, 113 insertions); bug fixes discovered: none; files touched: `internal/cli/analyze.go`, `internal/cli/analyze_output_test.go`, `internal/cli/analyze_handle_test.go`, `cmd/reglint/main_test.go`.

## Summary

| Phase | Status |
| --- | --- |
| Phase 1: Formatter core in `internal/output` | Done (`1930576`) |
| Phase 2: Registry and CLI wiring | Done (`e8598e2`) |
| Phase 3: Tests | Done (`1930576`, `e8598e2`) |
| Phase 4: Docs and spec-index alignment | Not started (spec deltas blocked on user approval) |
| Phase 5: Final verification | Not started |

**Remaining effort:** Phases 1–3 done (`1930576`, `e8598e2`); Phase 4 is small README work with one blocked spec-delta item; Phase 5 is gate runs (quality already green at Phase 2 close-out).

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

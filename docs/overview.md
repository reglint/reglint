---
description: RegLint is a regex-based static analysis tool — plain YAML rules, single static binary, console/JSON/SARIF/GitHub output. Start with the docs overview.
---

# Overview

**Your own linter, defined in YAML regex.** RegLint is a regex-based static analysis tool for source repositories: scan any codebase with rules you write and report findings as console output, JSON, SARIF, or native GitHub Actions PR annotations.

- **Plain YAML rules** — a message, an RE2 regex, a severity: that's a rule.
- **CI-native** — `github` output format emits PR annotations; SARIF uploads to GitHub code scanning.
- **Fast and dependency-free** — a single static Go binary with parallel scans, `.gitignore` support, baseline mode, and staged/diff scoping.

[:material-download: Install](install.md){ .md-button } [:material-flag: Quickstart](quickstart.md){ .md-button } [:material-robot: For AI agents & CI/CD](agents.md){ .md-button }

## Why RegLint?

RegLint sits between one-off `grep` invocations and heavyweight language-aware analyzers. It is for team-specific rules — banned APIs, TODO policies, license headers, hardcoded tokens, naming conventions — that apply across languages and that you want enforced in CI today.

| | `grep` + shell scripts | Secret scanners (gitleaks, trufflehog) | Language-aware analyzers (semgrep) | RegLint |
|---|---|---|---|---|
| Custom rules | Ad hoc pipelines | TOML rulesets | Per-language DSL | One YAML schema for every file type |
| Language coverage | Any text | Any (secret patterns) | One language per ruleset | Any text |
| Severity levels + fail threshold | DIY exit codes | Varies | Yes | Built in (`--fail-on`) |
| Baseline of known findings | No | Varies | Varies | Built in |
| GitHub PR annotations + SARIF | No | Varies | Via extra tooling | Native output formats |
| Deployment | Your scripts | Single binary | Binary or server | Single static binary |

When to pick something else: you need AST-accurate matches inside one language (semgrep), or a curated provider-specific secrets database (gitleaks). When you need your own rules everywhere, RegLint gets out of the way.

RegLint validates its own codebase in CI with the same rule schema — the repository policy lives in [`reglint-rules.yaml`](https://github.com/reglint/reglint/blob/main/reglint-rules.yaml) and runs as the `reglint` job in [quality](https://github.com/reglint/reglint/blob/main/.github/workflows/quality.yml).

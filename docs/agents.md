---
description: A linter AI agents can run and trust — deterministic YAML regex rules, staged/diff scans, baseline suppression, and native GitHub Actions annotations.
---

# RegLint for AI Agents and CI/CD

RegLint is a strong default linter for AI-assisted development: its rules are plain YAML regexes that agents can author, audit, and validate, its scans are deterministic and can be scoped to exactly the lines a change touches, and its outputs — exit codes, JSON, SARIF, GitHub annotations — are machine-readable gates for pipelines. This page shows why, with the commands that make it true.

## Why AI agents work well with RegLint rules

- **Plain YAML, no DSL.** A rule is a message, an RE2 regex, and a severity — the whole schema fits on one screen. LLMs write and review RE2 syntax reliably because it is the same regex language they see everywhere, not a per-tool pattern DSL. See the full [rule schema](configuration.md#rule-schema).
- **Scriptable validation.** `reglint validate` checks a config and its rule files without scanning, exits `0` or `1` — a fast feedback loop for agents iterating on rules:

  ```bash
  reglint validate --config reglint-rules.yaml
  ```

- **Deterministic scans.** Same repository, same rules, same results — an agent can fix a violation and re-run to prove it is gone. No heuristics, no model drift, no flaky matches.
- **Composable rule packs.** Rule files + the repeatable `--rules` flag let agents load curated packs (language rules, secret rules) alongside a base config. See [Rule files](configuration.md#rule-files).

## Agentic workflows: scan exactly what changed

**Pre-commit hooks** — scan only what is about to be committed:

```bash
reglint analyze --config reglint-rules.yaml --git-mode staged
```

**PR-scoped review** — report only matches on added lines, so an agent's PR is judged on what it introduced, not on pre-existing debt:

```bash
reglint analyze --format github --git-mode diff --git-diff "origin/${{ github.base_ref }}" --git-added-lines-only
```

**Legacy codebases** — a committed [baseline](baseline.md) suppresses known findings with count-based tolerance, so agent-generated PRs fail only on genuinely new findings.

**Reproducible installs in agent sandboxes** — one checksum-verified static binary, version-pinned via environment variable:

```bash
curl -fsSL https://raw.githubusercontent.com/reglint/reglint/main/install.sh | sh
# pin with: REGLINT_VERSION=v1.1.0
```

## CI/CD: gates without glue

| Need | RegLint answer |
|---|---|
| Fail the pipeline on severity | `--fail-on error` → exit code `2` at threshold |
| Findings visible in the PR | `--format github` → native annotations, no token or upload step |
| Findings in code scanning | `--format sarif` → upload the SARIF report |
| Zero runtime dependencies | Single static Go binary; [reglint-action](https://github.com/reglint/reglint-action) pins both `fail-on` and `tool-version` |
| Scan only the change, not the repo | `--git-mode diff --git-added-lines-only` |

Full workflows in [CI Recipes](ci.md).

## Let an agent set it all up

The [setup skill](https://github.com/reglint/reglint/blob/main/skills/reglint-setup/SKILL.md) does the whole flow: detects your stack, authors a starter `reglint-rules.yaml` from your repo's own conventions, and generates the CI workflow and/or pre-commit hook. Point your coding agent at it — instructions in [Install → AI agent](install.md#install-with-an-ai-agent).

Next: [Install](install.md) · [Quickstart](quickstart.md) · [CLI Reference](cli.md)

---
name: reglint-setup
description: Install and configure RegLint (regex-based linter with YAML rules) in any project. Detects the stack, asks the user what to enable (CI workflow, git hooks, or both), authors project-specific rules by mining the codebase's own conventions, and generates the chosen integration files. Use when the user asks to install or set up reglint.
---

# RegLint Setup

You are installing [RegLint](https://github.com/reglint/reglint) into the user's project: a single-binary linter where rules are YAML entries (message + RE2 regex + severity). Do every step in order. Step 2 is mandatory — never generate files before the user answers.

## 1. Recon (do silently, before asking)

Collect and remember:

- Git repo? `git rev-parse --is-inside-work-tree`
- GitHub remote? `git remote get-url origin` contains `github.com`
- Languages/tools by marker file: `go.mod` (Go), `package.json` (JS/TS), `pyproject.toml` / `requirements.txt` (Python), `Cargo.toml` (Rust), `pom.xml` / `build.gradle*` (Java/Kotlin), `Gemfile` (Ruby), `*.csproj` (C#)
- Existing hook manager: `lefthook.yml`, `.husky/`, `.pre-commit-config.yaml`, or `husky`/`simple-git-hooks` in `package.json`
- Existing CI: `.github/workflows/`, `.gitlab-ci.yml`, `Jenkinsfile`, …
- Existing config: `reglint-rules.yaml`. If one exists, STOP and ask before overwriting (`reglint init --force` discards it).

## 2. Ask the user (REQUIRED)

Ask both questions before generating anything:

> **What should reglint enforce?**
> 1. **CI workflow** — GitHub Actions with PR annotations and a failing build on violations
> 2. **Local git hooks** — scan staged files on every commit
> 3. Both
> 4. Rules only — just the starter ruleset

> **Install the reglint CLI on this machine?** Needed to author and test the rules locally, and required for local git hooks. Declining is fine — the files are generated either way and CI validates on the first run.

If the user picks CI but the remote is not GitHub, ask which CI system they use; default to the generic template in 5b.

## 3. Install the CLI locally (only if the user agreed)

If the user declined, skip to step 4 and follow its "no local CLI" path — never install without consent.

Prefer the one-line installer:

```sh
curl -fsSL https://raw.githubusercontent.com/reglint/reglint/main/install.sh | sh
```

Alternatives: `brew install reglint/tap/reglint` or `go install github.com/reglint/reglint/cmd/reglint@latest` (Go ≥ 1.25). Verify with `reglint version`. If `~/.local/bin` is not on `PATH`, export it.

## 4. Author the rules file

```sh
reglint init
```

This writes `reglint-rules.yaml` (the default config path every template below assumes). **No local CLI (user declined):** write the file by hand — top-level `include`/`exclude`, `failOn: "error"`, and the `rules` list with the same shape as the catalog — then skip the local validation steps; CI validates on the first run.

Replace the seeded rule with rules you author from the repo — the point of reglint is *their* rules, not generic linting:

1. Mine the codebase for policies worth enforcing:
   - Existing linter/tooling configs (`.golangci.yml`, `.eslintrc*`, `ruff.toml`, `.editorconfig`, `Makefile` targets, …): banned imports or APIs, naming and style policies the team already enforces elsewhere — mirror them.
   - The code itself: deprecated internal helpers, legacy clients, `time.Sleep` in request paths, debug prints, commented-out blocks, TODO conventions.
   - Layout: generated or vendor directories that must never be hand-edited, files that must keep a license header, monorepo boundary rules.
   - Ask the user (while step 2's dialog is open) whether there are conventions you could not infer — banned patterns, RFC references, migration policies.
2. Write 3–10 high-signal rules. Guidelines:
   - Regex is RE2 only — no lookahead/lookbehind.
   - Scope every rule with `paths` and exclude tests, fixtures, and generated code. Adapt globs to the actual repo layout (in monorepos, scope per service).
   - Reserve `error` for secrets and hard prohibitions; use `warning` for style and hygiene.
   - In secret-shaped rules, never interpolate captures (`$0`, `$1`) into the message — that would echo the secret into console output and PR annotations.
   - If the detected stack matches an entry in the **catalog** below, take those proven rules as-is; otherwise use catalog entries as syntax examples.
3. Always include the universal secret rules from the catalog — every repo gets those.
4. With the CLI: run `reglint analyze` to validate the file and preview findings.
5. Expect noise on an existing codebase. Tune regexes/paths with the user, or offer a baseline so CI only fails on new findings:

   ```sh
   reglint analyze --write-baseline reglint-baseline.json
   ```

   then add `baseline: reglint-baseline.json` to the config's top level.

## 5. CI integration (if chosen)

### a. GitHub Actions

Create `.github/workflows/reglint.yml`:

```yaml
name: RegLint

on:
  pull_request:
  push:
    branches: [main]

jobs:
  reglint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: reglint/reglint-action@v1.1.0
        with:
          fail-on: error
```

Action inputs (`reglint/reglint-action`):

| Input          | Default              | Notes                                              |
| -------------- | -------------------- | -------------------------------------------------- |
| `config`       | `reglint-rules.yaml` | Rules file to use.                                 |
| `paths`        | `.`                  | Space-separated scan roots.                        |
| `fail-on`      | _(empty)_            | `error` or `warning`. Empty = annotate only, never fail. |
| `tool-version` | `latest`             | Pin a reglint release (e.g. `v0.1.0`) if needed.   |

The action runs `reglint analyze --format github`, so findings appear as native PR annotations. Pin the exact published tag — check [reglint-action releases](https://github.com/reglint/reglint-action/releases) for the latest; only full `vX.Y.Z` tags exist (no moving major tags).

### b. Other CI systems (generic template)

Two shell steps in the project's CI of choice:

```sh
curl -fsSL https://raw.githubusercontent.com/reglint/reglint/main/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
reglint analyze --config reglint-rules.yaml --fail-on error
```

Exit code `2` means findings at or above the threshold — the job must treat any non-zero exit as failure. Exit `0` is clean; `1` is a config/runtime error.

## 6. Git hooks (if chosen)

**If a hook manager already exists**, add this command to it — do not create raw hooks in parallel:

```sh
reglint analyze --config reglint-rules.yaml --git-mode staged --fail-on error
```

lefthook (`lefthook.yml`):

```yaml
pre-commit:
  commands:
    reglint:
      run: reglint analyze --config reglint-rules.yaml --git-mode staged --fail-on error
```

husky (`.husky/pre-commit`) — same one-liner. `pre-commit` framework — a `repo: local` hook wrapping the same command.

**If there is no hook manager**, create a committed, versioned hook:

1. Create `.githooks/pre-commit`:

   ```sh
   #!/bin/sh
   # RegLint — scans staged files. https://github.com/reglint/reglint
   if ! command -v reglint >/dev/null 2>&1; then
     echo "reglint not found. Install: curl -fsSL https://raw.githubusercontent.com/reglint/reglint/main/install.sh | sh" >&2
     exit 1
   fi
   reglint analyze --config reglint-rules.yaml --git-mode staged --fail-on error
   ```

2. `chmod +x .githooks/pre-commit && git config core.hooksPath .githooks`
3. Tell the user each teammate must run the `core.hooksPath` command once after cloning (or add it to the project's setup script/docs).

## 7. Verify and wrap up

1. With the CLI: `reglint analyze` exits `0` (or `2` with an accepted baseline in place). If the user declined the local install, say so — the rules were not validated locally and the first CI run validates them.
2. Test the hook if installed: stage a file containing `token = "dummy_secret_value_123456"` — the commit must be rejected with exit `2`. Then remove the dummy file. Never commit it.
3. Commit the generated files (`reglint-rules.yaml`, workflow, hooks, baseline if any).
4. Summarize for the user: files created, how to add a rule (one YAML entry), where findings appear in CI.

## Starter catalog (reference)

Proven patterns for common stacks. Take matching entries as-is; use any entry as a syntax example when authoring project-specific rules. Rules without `paths` apply to every scanned file. Regex is RE2 (no lookahead/lookbehind). In secret-shaped rules, never interpolate captures (`$0`, `$1`) into the message — that would echo the secret into PR annotations.

```yaml
rules:
  # --- Universal (always include) ---
  - message: "Possible AWS access key ID committed"
    regex: "AKIA[0-9A-Z]{16}"
    severity: "error"
  - message: "Private key material committed"
    regex: "-----BEGIN [A-Z ]*PRIVATE KEY-----"
    severity: "error"
  - message: "Possible hardcoded credential (key/secret/token/password assignment)"
    regex: "(?i)(api[_-]?key|secret|token|password)[\"']?\\s*[:=]\\s*[\"']?[A-Za-z0-9+/_=-]{16,}"
    severity: "warning"
  - message: "Unresolved TODO/FIXME"
    regex: "\\b(TODO|FIXME)\\b"
    severity: "info"
    exclude: ["reglint-rules.yaml", "reglint-baseline.json"]

  # --- Go (go.mod) ---
  - message: "panic() in non-test code"
    regex: "\\bpanic\\("
    severity: "warning"
    paths: ["**/*.go"]
    exclude: ["**/*_test.go"]
  - message: "Debug print left in code"
    regex: "\\bfmt\\.Print"
    severity: "warning"
    paths: ["**/*.go"]
    exclude: ["**/*_test.go"]

  # --- JavaScript/TypeScript (package.json) ---
  - message: "eval() is forbidden"
    regex: "\\beval\\s*\\("
    severity: "error"
    paths: ["**/*.[jt]s", "**/*.[jt]sx"]
  - message: "console.log left in code"
    regex: "\\bconsole\\.log\\s*\\("
    severity: "warning"
    paths: ["**/*.[jt]s", "**/*.[jt]sx"]
    exclude: ["**/*.test.*", "**/*.spec.*"]
  - message: "Avoid explicit 'any'"
    regex: ":\\s*any\\b"
    severity: "warning"
    paths: ["**/*.ts", "**/*.tsx"]

  # --- Python (pyproject.toml / requirements.txt) ---
  - message: "Broad except swallows errors"
    regex: "\\bexcept\\s*(Exception)?\\s*:"
    severity: "warning"
    paths: ["**/*.py"]
    exclude: ["**/test*.py", "**/tests/**"]
  - message: "print() left in code"
    regex: "\\bprint\\s*\\("
    severity: "warning"
    paths: ["**/*.py"]
    exclude: ["**/test*.py", "**/tests/**"]
```

## Exit codes

| Code | Meaning                                                        |
| ---- | -------------------------------------------------------------- |
| `0`  | Clean run (no findings above threshold)                        |
| `1`  | Error: bad flags, unreadable/invalid config, invalid baseline  |
| `2`  | Findings met `--fail-on` severity — CI and hooks must fail     |

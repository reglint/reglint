---
description: RegLint CLI reference — analyze, init, validate, and version commands with every flag, default, and exit code for the regex-based linter.
---

# CLI Reference

```bash
reglint <command> [flags]
```

| Command | Alias | Purpose |
|---|---|---|
| `analyze` | `analyse` | Scan paths with a rules config and report findings. |
| `init` | — | Write a starter `reglint-rules.yaml`. |
| `validate` | — | Validate a config and its rule files without scanning. |
| `version` | — | Print version information. |

No command prints help and exits `1`.

## Exit codes

- `0`: command succeeded and no `--fail-on` threshold was triggered.
- `1`: command/config/runtime error (for example invalid flags or invalid baseline file).
- `2`: command succeeded, but at least one finding met `--fail-on` severity.

Exception: `analyze --write-baseline` returns `0` on successful baseline write, even if findings exist.

## analyze

```bash
reglint analyze [flags] [path ...]
reglint analyse [flags] [path ...]
```

Scans the current directory by default, or the given paths.

| Flag | Type | Default | Purpose |
|---|---|---|---|
| `-c`, `--config` | string | `reglint-rules.yaml` | Path to YAML rules config file. |
| `-f`, `--format` | string | `console` | Comma-separated list of formats. |
| `--out-json` | string | none | Output path for JSON results. |
| `--out-sarif` | string | none | Output path for SARIF results. |
| `--include` | string | none | Repeatable include glob. |
| `--exclude` | string | none | Repeatable exclude glob. |
| `--concurrency` | int | `GOMAXPROCS` | Worker count. |
| `--max-file-size` | int | `5242880` | Maximum file size in bytes. |
| `--fail-on` | string | none | Fail if matches at or above severity. |
| `--baseline` | string | none | Baseline JSON path for suppression. |
| `--write-baseline` | bool | `false` | Generate/regenerate baseline from findings. |
| `--git-mode` | string | `off` | Select Git mode: `off`, `staged`, or `diff`. |
| `--git-diff` | string | none | Diff target/range for `--git-mode=diff`. Implies `--git-mode diff`. |
| `--git-added-lines-only` | bool | `false` | Restrict matches to added lines in Git mode. |
| `--no-gitignore` | bool | `false` | Disable `.gitignore` filtering for this run. |
| `--no-ignore-files` | bool | `false` | Disable ignore file loading and matching. |

Common usage patterns:

```bash
# Analyze current directory with default config path
reglint analyze

# Analyze specific roots
reglint analyze services/api services/web

# Generate config at a custom location
reglint init --out configs/reglint-rules.yaml
reglint analyze --config configs/reglint-rules.yaml
```

## init

```bash
reglint init [flags]
```

| Flag | Type | Default | Purpose |
|---|---|---|---|
| `--out` | string | `reglint-rules.yaml` | Output path for the config file. |
| `--force` | bool | `false` | Overwrite existing config file. |

## validate

```bash
reglint validate [flags]
```

Loads and validates the config and every referenced rule file (merge order and duplicate detection included) without scanning. Exits `0` when valid, `1` with a single error message otherwise.

| Flag | Type | Default | Purpose |
|---|---|---|---|
| `-c`, `--config` | string | `reglint-rules.yaml` | Path to YAML rules config file. |
| `--rules` | string | none | Repeatable rule file path loaded after config ruleFiles. |

## version

```bash
reglint version
```

Prints version information. Takes no flags besides `--help`.

Authoritative detail: [`specs/cli.md`](https://github.com/reglint/reglint/blob/main/specs/cli.md), [`specs/cli-analyze.md`](https://github.com/reglint/reglint/blob/main/specs/cli-analyze.md), [`specs/cli-init.md`](https://github.com/reglint/reglint/blob/main/specs/cli-init.md), [`specs/cli-validate.md`](https://github.com/reglint/reglint/blob/main/specs/cli-validate.md).

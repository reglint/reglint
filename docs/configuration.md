---
description: RegLint configuration reference — every reglint-rules.yaml field, defaults, rule schema, rule files, ignore files, git settings, and validation rules.
---

# Configuration

Default config path is `reglint-rules.yaml`. Generate a starter with `reglint init`.

## Top-level fields

Field names are verified against the shipped binary (Go struct in `internal/config`).

- `rules` (required unless `ruleFiles` is non-empty): list of regex rules.
- `include`: repository-level include globs (default `**/*`). Overridden by per-rule `paths`.
- `exclude`: repository-level exclude globs (default `**/.git/**`, `**/node_modules/**`, `**/vendor/**`). Overridden by per-rule `exclude`.
- `failOn`: one of `error`, `warning`, `notice`, `info`. Unset by default (no failure threshold).
- `concurrency`: worker count override (default `GOMAXPROCS`).
- `baseline`: default baseline file path used by `analyze` when `--baseline` is not set.
- `ruleFiles`: ordered rule file paths loaded after this file's own `rules`. Paths resolve relative to this config file's directory — see [Rule files](#rule-files).
- `git`: default Git scan settings — see [Git settings](#git-settings).
- `consoleColorsEnabled`: enable or disable ANSI color in console output (default `true`).
- `ignoreFilesEnabled`: enable or disable ignore file processing (default `true`).
- `ignoreFiles`: custom ignore file list (default `.ignore`, `.reglintignore`) — see [Ignore files](#ignore-files).

Binary files and files larger than `--max-file-size` (default 5 MB) are skipped; the console summary line and the `stats` section of the JSON output report skipped file counts.

### Git settings

`git.mode` must be one of `off|staged|diff` (default `off`); `mode: diff` requires `git.diff`; `git.addedLinesOnly` is valid only for `staged|diff`.

- `git.mode`: `off`, `staged`, or `diff`.
- `git.diff`: diff target/range used by `mode: diff`.
- `git.addedLinesOnly`: report only matches on added lines (default `false`).
- `git.gitignoreEnabled`: enable `.gitignore` filtering (default `true`).

## Rule schema

Each rule in `rules`:

- `message` (required): text reported for matches. Capture groups may be referenced as `$1`, `$2`, …
- `regex` (required): an RE2 regular expression.
- `severity`: one of `error`, `warning`, `notice`, `info`.
- `paths`: per-rule include globs; a match only counts for paths matching at least one.
- `exclude`: per-rule exclude globs; matching paths are skipped for this rule.

## Full example

```yaml
include:
  - "src/**"
  - "lib/**"
exclude:
  - "**/generated/**"
failOn: "error"
concurrency: 8
baseline: "testdata/baseline.json"
consoleColorsEnabled: true
ignoreFilesEnabled: true
ignoreFiles:
  - ".ignore"
  - ".reglintignore"
git:
  mode: "diff"
  diff: "HEAD~1..HEAD"
  addedLinesOnly: false
  gitignoreEnabled: true
ruleFiles:
  - "rules-go.yaml"
  - "rules-security.yaml"
rules:
  - message: "Avoid hardcoded token: $1"
    regex: "token\\s*[:=]\\s*([A-Za-z0-9_-]+)"
    severity: "error"
    paths:
      - "src/**"
  - message: "This is a warning message"
    regex: "regex2"
    severity: "warning"
    exclude:
      - "lib/vendor/**"
```

## Rule files

Split rules out of the base config into dedicated files — useful for shared, curated rule packs referenced from the base config or passed on the command line.

- A rule file is a YAML document whose only allowed populated field is `rules` (non-empty). Any other setting — including `ruleFiles` itself — is a validation error naming the offending field; unknown top-level keys are rejected too.
- Merge order is append-only: base `rules` → `ruleFiles` entries in declared order → `--rules` flag values in flag order.
- Two rules with the same `(message, regex)` pair anywhere in the merged set are a validation error (also within a single file).
- `reglint analyze` accepts a repeatable `--rules <path>` flag; these paths resolve relative to the current working directory, unlike `ruleFiles` (config directory). Duplicate paths across `ruleFiles` and `--rules` are rejected.
- Rule files contribute rules and nothing else: include/exclude, `failOn`, git, baseline, ignore, and color settings come from the base config and CLI flags only.

Example rule file (`rules-go.yaml`):

```yaml
rules:
  - message: "TODO comment left in code"
    regex: "TODO|FIXME"
    severity: "notice"
    paths:
      - "**/*.go"
  - message: "panic outside main/test"
    regex: "\\bpanic\\("
    severity: "warning"
    paths:
      - "**/*.go"
    exclude:
      - "**/*_test.go"
```

## Ignore files

Ignore files are processed by default in this order: `.gitignore`, then `.ignore`, then `.reglintignore`. Supported syntax: blank lines, `#` comments, `!` negation, leading `/` anchoring, trailing `/` directory-only rules, and `*`/`?`/`**` wildcards. The last matching rule wins.

Precedence per candidate path:

1. Must match at least one effective include glob.
2. Any exclude glob match removes it.
3. Ignore matcher (last match wins) can still ignore it; `.ignore`/`.reglintignore` take priority over `.gitignore` on conflicts.
4. `.gitignore` cannot re-include a path removed by include/exclude filters.

Disable per run with `--no-gitignore` (only `.gitignore`) or `--no-ignore-files` (all ignore files), or globally with `git.gitignoreEnabled: false` / `ignoreFilesEnabled: false`. Git global excludes (`core.excludesFile`, `.git/info/exclude`) are not read.

An invalid ignore pattern exits with code `1` and reports `<source>:<line>`.

## Validation

- `rules` is required unless `ruleFiles` is non-empty.
- `failOn` must be one of the allowed values when set.
- `concurrency` must be a positive integer when set.
- `baseline` must be a non-empty string when set.
- `ignoreFiles` values must be non-empty file names without path separators or duplicates.
- `git.mode` must be `off|staged|diff`; `mode: diff` requires `git.diff`; `addedLinesOnly: true` requires `staged|diff`.
- Any YAML parse failure or invalid rule exits with code `1` before scanning starts.

## Colors

Console output uses ANSI severity colors by default. Disable in config or for a single run with `NO_COLOR`:

```bash
NO_COLOR=1 reglint analyze --config reglint-rules.yaml --format console
```

Authoritative detail: [`specs/configuration.md`](https://github.com/reglint/reglint/blob/main/specs/configuration.md), [`specs/rule-files.md`](https://github.com/reglint/reglint/blob/main/specs/rule-files.md), [`specs/ignore-files.md`](https://github.com/reglint/reglint/blob/main/specs/ignore-files.md).

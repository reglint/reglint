---
description: Scan staged files or diffs with RegLint git-scoped modes — --git-mode staged/diff, --git-diff targets, added-lines-only reporting, and ignore-file behavior.
---

# Git-Scoped Scans

Git integration is optional and defaults to `off`.

Scan the full repository (default):

```bash
reglint analyze --config reglint-rules.yaml
```

Scan staged files only:

```bash
reglint analyze --config reglint-rules.yaml --git-mode staged
```

Scan files selected by a diff target:

```bash
reglint analyze --config reglint-rules.yaml --git-mode diff --git-diff HEAD~1..HEAD
```

`--git-diff` implies `--git-mode diff` if `--git-mode` is not provided.

Restrict reporting to matches on added lines:

```bash
reglint analyze --config reglint-rules.yaml --git-mode diff --git-diff HEAD~1..HEAD --git-added-lines-only
```

The same settings are available in config under `git`:

```yaml
git:
  mode: "diff"
  diff: "HEAD~1..HEAD"
  addedLinesOnly: false
  gitignoreEnabled: true
```

## Ignore-file behavior

- Ignore files are enabled by default in all modes (`off`, `staged`, `diff`).
- Default evaluation order is `.gitignore`, then `.ignore`, then `.reglintignore`.
- Use `--no-gitignore` to disable only `.gitignore`.
- Use `--no-ignore-files` to disable all ignore-file processing.

Full semantics in [Configuration → Ignore files](configuration.md#ignore-files).

## Common errors

- `effective --git-mode=diff requires --git-diff` — add `--git-diff <target>` when using `--git-mode diff`.

Authoritative detail: [`specs/git-integration.md`](https://github.com/reglint/reglint/blob/main/specs/git-integration.md).

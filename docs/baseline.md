---
description: Suppress known findings with the RegLint baseline workflow — generate a baseline with --write-baseline, commit it, and fail CI only on new findings.
---

# Baseline Workflow

Baseline compare mode suppresses known findings using `(filePath, message)` with count-based tolerance — adopt RegLint on a legacy codebase without a flood of pre-existing findings.

Compare against an existing baseline:

```bash
reglint analyze --config reglint-rules.yaml --baseline baseline.json
```

Use baseline from config (without passing `--baseline`):

```yaml
baseline: "baseline.json"
```

```bash
reglint analyze --config reglint-rules.yaml
```

Generate or refresh a baseline from current findings:

```bash
reglint analyze --config reglint-rules.yaml --baseline baseline.json --write-baseline
```

`--write-baseline` exits `0` on successful write, even if findings exist.

## Typical adoption on a legacy codebase

1. Generate a baseline once with `--write-baseline` and commit it.
2. Run with `--baseline` in CI so only new findings fail the build.
3. Gradually fix suppressed findings and refresh the baseline with `--write-baseline`.

`--fail-on` still applies after suppression: only findings that survive baseline filtering count toward the threshold. If a configured baseline file is invalid, analyze exits `1`.

Authoritative detail: [`specs/cli-analyze-baseline.md`](https://github.com/reglint/reglint/blob/main/specs/cli-analyze-baseline.md).

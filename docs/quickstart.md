---
description: Create your first RegLint rules file and run a scan in two commands. Define rules as message, RE2 regex, and severity in plain YAML.
---

# Quickstart

Create a starter rules file, then run a scan:

```bash
reglint init
reglint analyze --config reglint-rules.yaml
```

A rule is a message, a regex, and a severity:

```yaml
rules:
  - message: "Avoid hardcoded token: $1"
    regex: "token\\s*[:=]\\s*([A-Za-z0-9_-]+)"
    severity: "error"
    paths:
      - "src/**"
```

Findings print per file with `line:column` positions, and a summary line closes the run:

```text
src/client.go
- ERROR 42:12 Avoid hardcoded token: sk_live_example
  src/client.go:42

Summary: files=31 skipped=0 matches=1 durationMs=18
```

The exit code is `2` when a finding meets the `--fail-on` threshold — see [Exit Codes](cli.md#exit-codes). Use `reglint --help` or `reglint analyze --help` for the full command reference, or browse the [CLI Reference](cli.md).

Next steps:

- [Configuration](configuration.md) — every config field, defaults, and validation.
- [Output Formats](output-formats.md) — console, JSON, SARIF, GitHub annotations.
- [Baseline Workflow](baseline.md) — adopt RegLint on a codebase with existing findings.

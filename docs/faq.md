---
description: RegLint FAQ and troubleshooting — config errors, exit codes, secrets detection, SARIF support, ignoring vendored files, and adopting on legacy codebases.
---

# FAQ

## Troubleshooting

- `config file not found: reglint-rules.yaml`
  - Run `reglint init` in the repository root or pass `--config <path>`.
- `effective --git-mode=diff requires --git-diff`
  - Add `--git-diff <target>` when using `--git-mode diff`.
- `--out-json is required` or `--out-sarif is required`
  - When selecting multiple formats, provide an output file path for each non-console formatter.
- Command exits `2` in CI
  - This is expected when `--fail-on` threshold is met; tune `failOn` in config or CLI if needed.
- No findings in staged/diff mode when you expected matches
  - Verify file selection (`--git-mode`, `--git-diff`) and ignore settings (`--no-gitignore`, `--no-ignore-files`).

## What is RegLint?

A regex-based linter for source repositories. You define rules in a YAML file (`reglint-rules.yaml`), point RegLint at a directory, and it reports every match with file, line, and column plus a severity. It ships as a single static binary written in Go.

## How is RegLint different from Semgrep?

Semgrep matches code with per-language AST patterns; RegLint matches text with RE2 regexes. That makes RegLint language-agnostic — the same rule scans Go, TypeScript, Terraform, and plain config files — at the cost of AST precision. Many teams run both: Semgrep for language-deep rules, RegLint for cross-cutting conventions.

## Can RegLint find hardcoded secrets?

Yes — a secret is just a regex match. The starter from `reglint init` and the [Quickstart](quickstart.md) show a hardcoded-token rule; combine `paths`, severity, and `--fail-on error` to block merges on leaked credentials. For curated provider-specific detection, a dedicated scanner is a good complement.

## Does RegLint support GitHub code scanning and SARIF?

Yes. `--format sarif` emits a SARIF report you can upload with `github/codeql-action/upload-sarif`, and `--format github` writes native workflow commands that render findings as PR annotations — no upload step, no extra token. See [CI Recipes](ci.md).

## How do I ignore vendored or generated files?

Ignore files are processed by default in this order: `.gitignore`, `.ignore`, `.reglintignore`. Add paths there, control them globally with `include` / `exclude` globs in the config, per rule with `paths` / `exclude`, or disable with `--no-gitignore` / `--no-ignore-files`.

## How do I adopt RegLint on a legacy codebase?

Generate a baseline once with `--write-baseline`, commit it, and run with `--baseline` in CI so only new findings fail the build. See [Baseline Workflow](baseline.md).

<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "FAQPage",
  "mainEntity": [
    {
      "@type": "Question",
      "name": "What is RegLint?",
      "acceptedAnswer": { "@type": "Answer", "text": "A regex-based linter for source repositories. You define rules in a YAML file (reglint-rules.yaml), point RegLint at a directory, and it reports every match with file, line, and column plus a severity. It ships as a single static binary written in Go." }
    },
    {
      "@type": "Question",
      "name": "How is RegLint different from Semgrep?",
      "acceptedAnswer": { "@type": "Answer", "text": "Semgrep matches code with per-language AST patterns; RegLint matches text with RE2 regexes. That makes RegLint language-agnostic — the same rule scans Go, TypeScript, Terraform, and plain config files — at the cost of AST precision. Many teams run both: Semgrep for language-deep rules, RegLint for cross-cutting conventions." }
    },
    {
      "@type": "Question",
      "name": "Can RegLint find hardcoded secrets?",
      "acceptedAnswer": { "@type": "Answer", "text": "Yes — a secret is just a regex match. The starter from reglint init and the Quickstart show a hardcoded-token rule; combine paths, severity, and --fail-on error to block merges on leaked credentials. For curated provider-specific detection, a dedicated scanner is a good complement." }
    },
    {
      "@type": "Question",
      "name": "Does RegLint support GitHub code scanning and SARIF?",
      "acceptedAnswer": { "@type": "Answer", "text": "Yes. --format sarif emits a SARIF report you can upload with github/codeql-action/upload-sarif, and --format github writes native workflow commands that render findings as PR annotations — no upload step, no extra token." }
    },
    {
      "@type": "Question",
      "name": "How do I ignore vendored or generated files?",
      "acceptedAnswer": { "@type": "Answer", "text": "Ignore files are processed by default in this order: .gitignore, .ignore, .reglintignore. Add paths there, control them globally with include / exclude globs in the config, per rule with paths / exclude, or disable with --no-gitignore / --no-ignore-files." }
    },
    {
      "@type": "Question",
      "name": "How do I adopt RegLint on a legacy codebase?",
      "acceptedAnswer": { "@type": "Answer", "text": "Generate a baseline once with --write-baseline, commit it, and run with --baseline in CI so only new findings fail the build." }
    }
  ]
}
</script>

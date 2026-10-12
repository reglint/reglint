---
name: reglint-learn
description: Turn a just-fixed bug, incident, or review finding into a RegLint rule so the same mistake cannot recur silently. Use after fixing any bug whose root cause is a textual pattern (banned API, debug leftover, secret, outdated idiom), when the user says "never again", "make sure this doesn't happen again", or "learn from this", after a postmortem, or when a bad pattern keeps slipping back into the codebase. Complements reglint-setup, which installs RegLint in the first place.
---

# RegLint Learn

You just fixed (or are fixing) a problem. The fix repairs today's instance; a rule prevents the next one. RegLint rules are plain YAML entries (message + RE2 regex + severity) in `reglint-rules.yaml` — cheap to add, deterministic to enforce, visible to every future commit and CI run. Your job: distill the *class* of the mistake into a rule, prove the rule catches it, and commit it alongside the fix.

No RegLint in this project yet? Install it first with the [setup skill](https://github.com/reglint/reglint/blob/main/skills/reglint-setup/SKILL.md), then come back.

## 1. Decide whether a rule is the right tool

A rule works only if the mistake is expressible as a text pattern. Ask:

- **Does the root cause leave a textual fingerprint?** A banned function call, a debug print, a hardcoded credential, a legacy import, a magic value, an outdated API — yes. A wrong algorithm, a race condition, a broken permission check, a logic inversion — usually no.
- **Can RE2 express it without absurd false positives?** RE2 only: no lookahead/lookbehind. If matching the pattern requires context a regex cannot see, a rule will either miss the bug or drown the team in noise.
- **Is it likely to recur?** One-off typos don't need rules.

If the answer is no, say so plainly — "this class of bug isn't regex-detectable, so I can't encode it as a reglint rule; it needs a real test" — and stop. Forcing a rule is worse than none: it trains people to ignore findings.

If the mistake is *behavioral* (not just textual), also write a regression test. The rule is a seatbelt, not a substitute.

## 2. Check for an existing rule

Read `reglint-rules.yaml` (and any rule files listed under `ruleFiles:`). If a rule already covers this pattern — even loosely — tighten or extend *that* rule instead of adding a duplicate. Two overlapping rules means the next reader won't know which one owns the pattern.

## 3. Write the rule

Append to `reglint-rules.yaml`:

```yaml
  # 2026-10-12: rate limiter broke in prod — someone swapped the client back
  # to the deprecated NewLegacyClient. Banned until the migration deletes it.
  - message: "Deprecated NewLegacyClient: use ratelimit.New (see #142)"
    regex: "ratelimit\\.NewLegacyClient\\("
    severity: "error"
    paths: ["**/*.go"]
    exclude: ["**/*_test.go", "internal/legacy/**"]
```

Guidelines:

- **`message` states the why and the fix**, not just the what. "Deprecated X: use Y" beats "found X". A future reader meets this message at 2am with no memory of the incident.
- **Keep the YAML comment** above the rule: date, one line on what happened, link to the issue/PR if any. The comment is the institutional memory; the regex is just the enforcement.
- **Severity**: `error` for things that broke prod, secrets, hard prohibitions. `warning` for hygiene and risk. Don't inflate — if everything is an error, nothing is.
- **Scope it**: `paths` to the files where the pattern is a bug, `exclude` tests/fixtures/generated code where it may be legitimate. An unscoped rule over a monorepo is noise.
- **Regex is RE2**: no lookahead/lookbehind. To match an escape sequence *as text* (e.g. the characters `[ \t]`), escape the backslash in YAML (`'[ \\t]'`) — a single backslash matches a real tab and silently matches nothing.
- **Never interpolate captures** (`$0`, `$1`) into messages of secret-shaped rules — that would echo the secret into console output and PR annotations.
- Prefer matching the **stable signature** of the mistake (the API name, the import path) over incidental whitespace or formatting, or the rule breaks on the next formatting change.

If the rule fits better in a shared pack than this repo's config (e.g. it applies to every microservice), ask the user; the repeatable `--rules` flag loads extra rule files alongside the base config.

## 4. Prove the rule works

Never ship an unproven rule. Three checks, in order:

1. **It parses** (fast, no scan):
   ```sh
   reglint validate --config reglint-rules.yaml
   ```
2. **It catches the original mistake.** Drop the real offending snippet (the code that caused today's incident, before your fix) into a scratch file inside the repo, e.g. `reglint-repro.tmp.go`, then:
   ```sh
   reglint analyze --config reglint-rules.yaml --fail-on <severity>
   ```
   The scratch file must be reported (exit code `2` at the rule's severity). If it isn't, the regex is wrong — fix it before continuing.
3. **It doesn't catch neighbors.** In the same scratch file, add a benign lookalike (the correct API, a commented-out mention, a string literal that merely resembles the pattern) and confirm the rule reports only the real one. Then delete the scratch file — never commit it.

4. **Count the noise.** Run a full `reglint analyze` on the repo. If the new rule flags historical instances that nobody is fixing today, either fix them (best) or ask the user about a baseline:

   ```sh
   reglint analyze --write-baseline reglint-baseline.json
   ```

   then add `baseline: reglint-baseline.json` at the config's top level. Prefer fixing: a baseline that grows forever is debt with a JSON extension. Baselines are ideal combined with diff-scoped CI (`--git-mode diff --git-added-lines-only`), where legacy debt is tolerated but new occurrences fail.

## 5. Commit with the fix

The rule belongs in the **same commit/PR as the fix** when one exists — the fix repairs the past, the rule guards the future, and reviewers see both together. For postmortems where the incident predates the repo state, a standalone "learnings" PR is fine.

Summarize for the user: the rule added, what it catches, the repro that proved it, and where it will surface (hook, CI annotations).

## Exit codes

| Code | Meaning                                                     |
| ---- | ----------------------------------------------------------- |
| `0`  | Clean run (no findings above threshold)                      |
| `1`  | Error: bad flags, unreadable/invalid config, invalid baseline |
| `2`  | Findings met `--fail-on` severity — hooks and CI must fail    |

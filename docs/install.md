---
description: Install RegLint via Homebrew, curl script, go install, prebuilt binaries, or the GitHub Actions reglint-action. Checksum-verified, single static Go binary.
---

# Install

## Homebrew

```bash
brew install reglint/tap/reglint
```

The formula lives in [`reglint/homebrew-tap`](https://github.com/reglint/homebrew-tap) and is updated automatically on every release.

## One-line install

```bash
curl -fsSL https://raw.githubusercontent.com/reglint/reglint/main/install.sh | sh
```

Detects your platform, verifies the SHA-256 checksum, and installs to `~/.local/bin` (override with `REGLINT_INSTALL_DIR` or pin with `REGLINT_VERSION=v0.1.0`).

## Go install

Requires Go 1.25 or newer:

```bash
go install github.com/reglint/reglint/cmd/reglint@latest
```

## Prebuilt binaries

Download an archive and `checksums.txt` from [GitHub Releases](https://github.com/reglint/reglint/releases), verify the checksum, then extract and run the binary:

```bash
sha256sum -c checksums.txt --ignore-missing
tar -xzf reglint_<version>_linux_amd64.tar.gz
```

Or build from source:

```bash
git clone https://github.com/reglint/reglint
cd reglint
make build
./bin/reglint --help
```

## Install with an AI agent

Point your coding agent (Claude Code, OpenCode, Cursor, Copilot, …) at the setup skill and it does the rest: detects your stack, asks whether you want CI, git hooks, or both, then authors a starter `reglint-rules.yaml` from your repo's own conventions (existing linter configs, banned APIs, code policies) plus universal secret rules — and generates the integration files: a GitHub Actions workflow (via [`reglint/reglint-action`](https://github.com/reglint/reglint-action)) and/or a committed `.githooks/pre-commit` wired into `core.hooksPath`.

Give your agent this instruction:

> Instala reglint siguiendo las instrucciones de https://raw.githubusercontent.com/reglint/reglint/main/skills/reglint-setup/SKILL.md

The skill is plain markdown — read or review it at [`skills/reglint-setup/SKILL.md`](https://github.com/reglint/reglint/blob/main/skills/reglint-setup/SKILL.md).

Or install with Vercel's [skills CLI](https://github.com/vercel-labs/skills), which copies the skills into `.agents/skills/`, wires your agent's skill directory, and pins them in a `skills-lock.json`:

```bash
npx skills add https://github.com/reglint/reglint -y                            # both skills
npx skills add https://github.com/reglint/reglint --skill reglint-learn -y      # just one
```

`reglint-setup` installs and configures reglint; `reglint-learn` makes the agent turn every bug it fixes into a new rule, proven against the original offending snippet.

## Use in GitHub Actions

```yaml
name: scan
on: [push]
jobs:
  reglint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: reglint/reglint-action@v1.1.0
        with:
          fail-on: error
```

The action lives in [reglint/reglint-action](https://github.com/reglint/reglint-action) and is versioned independently of RegLint releases, publishing full `vX.Y.Z` tags only (no moving major tag — pin exact versions). It installs the checksum-verified binary and fails the step on the `fail-on` threshold; pin the tool with `tool-version: v0.1.0` (defaults to the latest release).

See [CI Recipes](ci.md) for full workflow examples, including SARIF upload and PR annotations.

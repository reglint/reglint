# Release Process

Status: Implemented

## Overview

### Purpose

- Publish reproducible, automated releases of the RegLint CLI from `main`.
- Make every release traceable to a SemVer tag and a changelog entry.

### Goals

- Tag-triggered releases with cross-platform binaries and checksums.
- Injected build version, so `reglint version` reports the release version.
- No manual build or upload steps.

### Non-Goals

- Homebrew taps, Docker images, or other distribution channels.
- Artifact signing (SHA-256 checksums only).
- Releases from feature branches.

### Scope

- Versioning policy, tagging procedure, and the `release` GitHub Actions workflow.
- Version injection via `internal/cli.version` (ldflags).

## Workflows

### Versioning

- Releases follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
- Tags are `vX.Y.Z` and are cut on `main` merge commits only — never on feature branches.
- `CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/): changes accumulate under `[Unreleased]` and are promoted at release time.

### Cutting a release

1. Open a `chore(release): vX.Y.Z` PR that moves the `CHANGELOG.md` `[Unreleased]` entries into a new `## [X.Y.Z] - YYYY-MM-DD` section and re-opens an empty `[Unreleased]`.
2. Merge the PR into `main`.
3. Tag the merge commit and push the tag:

   ```
   git tag vX.Y.Z <merge-commit-sha>
   git push origin vX.Y.Z
   ```

4. The `release` workflow builds all artifacts and publishes the GitHub Release with binaries, `checksums.txt`, and commit-derived notes.
5. Verify the assets and module resolution:

   ```
   go install github.com/iyaki/reglint/cmd/reglint@vX.Y.Z
   ```

## Automation

### Release workflow

`.github/workflows/release.yml` runs on:

- Push of a `v*` tag: full release (publishes the GitHub Release).
- `workflow_dispatch`: snapshot dry-run (`goreleaser release --snapshot --clean`) that builds all artifacts without publishing.

GoReleaser is pinned to the same version in the workflow and `.devcontainer/install-go-tools.sh`; upgrade both together. A published release is immutable history: never delete or re-tag it — ship a patch release instead.

### Version injection

- `internal/cli.version` defaults to `dev` for local builds.
- GoReleaser injects the release version with `-ldflags "-X github.com/iyaki/reglint/internal/cli.version={{ .Version }}"`.
- `reglint version` prints `reglint version <value>`.

### Artifacts

| OS      | Arch          | Format |
| ------- | ------------- | ------ |
| linux   | amd64, arm64  | tar.gz |
| darwin  | amd64, arm64  | tar.gz |
| windows | amd64, arm64  | zip    |

Each release also ships a `checksums.txt` with SHA-256 sums for all archives.

### Install script

`install.sh` at the repository root is a POSIX `sh` installer for machines without a package manager:

- Detects OS (`linux`/`darwin`) and architecture (`amd64`/`arm64`).
- Resolves the version from `REGLINT_VERSION`, defaulting to the latest release via the `releases/latest` redirect (no API call).
- Downloads the release archive plus `checksums.txt` and verifies the SHA-256 sum before extracting.
- Installs to `REGLINT_INSTALL_DIR`, default `$HOME/.local/bin`, without root.

The `release` workflow smoke-tests it on every tag push: it installs from the tag's own `install.sh` and asserts `reglint version` matches the tag.

### Homebrew tap

Every tag-push release updates the `iyaki/homebrew-tap` formula via GoReleaser, so `brew install iyaki/tap/reglint` tracks the newest release. Pushing to the tap repository requires the `TAP_GITHUB_TOKEN` secret (a token with write access to `iyaki/homebrew-tap`) because the workflow `GITHUB_TOKEN` is scoped to this repository only.

## Verifications

- Pushing a `vX.Y.Z` tag produces a published (non-draft) GitHub Release with 6 archives plus `checksums.txt`.
- A downloaded linux/amd64 binary reports `reglint version X.Y.Z`.
- `go install github.com/iyaki/reglint/cmd/reglint@latest` resolves the newest tag instead of a pseudo-version.
- `workflow_dispatch` runs complete green without creating a release.
- The installer smoke step passes on tag pushes: `install.sh` from the tag installs the binary and `reglint version` matches the tag.

## Appendices

### Examples

Checklist for `v0.2.0`:

```
1. Open chore(release): v0.2.0 PR updating CHANGELOG.md.
2. Merge the PR.
3. git tag v0.2.0 <merge-commit-sha> && git push origin v0.2.0
4. Watch the release workflow; verify assets and go install ...@v0.2.0.
```

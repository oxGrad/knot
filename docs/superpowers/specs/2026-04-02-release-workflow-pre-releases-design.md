# Design Spec: Enhanced Release Workflow for Nightly Builds

This document outlines the changes to `knot`'s release process to support nightly builds without marking them as the "Latest" release on GitHub or Homebrew.

## Goals

- Support tags with suffixes (e.g., `v1.0.0-nightly.20240115`).
- Ensure nightly builds are not marked as "Latest" on GitHub.
- Maintain a stable `knot` Homebrew formula (`brew install knot`).
- Provide a `knot-nightly` Homebrew formula (`brew install knot-nightly`) that specifically tracks nightly builds.

## Proposed Changes

### 1. GitHub Actions Trigger (`.github/workflows/release.yml`)
The workflow trigger needs to be expanded to catch tags that include a hyphen and a suffix. We will also add a condition to ensure the workflow only runs when a tag is pushed to the `main` branch.

```yaml
on:
  push:
    tags:
      - "v[0-9]*.[0-9]*.[0-9]*"   # Matches stable: v1.0.0
      - "v[0-9]*.[0-9]*.[0-9]*-*" # Matches nightly: v1.0.0-nightly.20240115

jobs:
  release:
    name: Release
    runs-on: ubuntu-latest
    if: github.event.base_ref == 'refs/heads/main' # Only run if tag is on main
    steps:
      # ...
```

Note: `github.event.base_ref` is populated when a tag is pushed, indicating the branch it was created from.


### 2. GoReleaser Configuration (`.goreleaser.yaml`)
We will transition from the deprecated `brews` section to `homebrew_casks` and split the configuration into two entries using conditional logic based on whether the current tag is a pre-release (`.Prerelease`).

#### Stable Cask (`knot`)
- **Name:** `knot`
- **Skip Upload:** `{{ .Prerelease }}` (True if the tag has a suffix)

#### Nightly Cask (`knot-nightly`)
- **Name:** `knot-nightly`
- **Skip Upload:** `{{ not .Prerelease }}` (True if the tag is stable)
- **Repo/Token:** Same as the stable cask.

#### Migration to `homebrew_casks`:
- Rename `brews:` to `homebrew_casks:`.
- Ensure the `repository` points to your existing tap.
- Use the `url` field or let GoReleaser handle the binary download link (default behavior for casks).
- Note: Casks are now the preferred way to distribute binaries in GoReleaser v2, replacing Formulas.


### 3. GitHub Release Behavior

GoReleaser automatically detects semantic versioning suffixes (like `-nightly.20240115`) and sets the `prerelease` flag to `true` on the GitHub Release. This prevents it from being marked as "Latest."

## Testing Strategy

- **Manual Verification:** Review the generated GoReleaser configuration for syntax errors.
- **Dry-run (Optional):** If possible, run `goreleaser release --snapshot --clean` locally to ensure the configuration is valid (though this won't test the conditional Homebrew upload).
- **Validation:** Ensure `v[0-9]*.[0-9]*.[0-9]*-*` correctly matches the requested `v1.0.0-nightly.20240115` format.

## Rollout Plan

1. Update `.github/workflows/release.yml`.
2. Update `.goreleaser.yaml`.
3. Commit and push changes to `main`.

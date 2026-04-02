# Design Spec: Enhanced Release Workflow for Pre-releases

This document outlines the changes to `knot`'s release process to support release candidates (RC), beta versions, and nightly builds without marking them as the "Latest" release on GitHub or Homebrew.

## Goals
- Support tags with suffixes (e.g., `v1.0.0-rc.1`, `v1.0.0-nightly.20240115`).
- Ensure pre-releases are not marked as "Latest" on GitHub.
- Maintain a stable `knot` Homebrew formula (`brew install knot`).
- Provide a `knot-nightly` Homebrew formula (`brew install knot-nightly`) that specifically tracks pre-releases.

## Proposed Changes

### 1. GitHub Actions Trigger (`.github/workflows/release.yml`)
The workflow trigger needs to be expanded to catch tags that include a hyphen and a suffix.

```yaml
on:
  push:
    tags:
      - "v[0-9]*.[0-9]*.[0-9]*"   # Matches stable: v1.0.0
      - "v[0-9]*.[0-9]*.[0-9]*-*" # Matches pre-release: v1.0.0-nightly.20240115
```

### 2. GoReleaser Configuration (`.goreleaser.yaml`)
We will split the `brews` configuration into two entries using conditional logic based on whether the current tag is a pre-release (`.Prerelease`).

#### Stable Formula (`knot.rb`)
- **Name:** `knot`
- **Skip Upload:** `{{ .Prerelease }}` (True if the tag has a suffix)

#### Nightly Formula (`knot-nightly.rb`)
- **Name:** `knot-nightly`
- **Skip Upload:** `{{ not .Prerelease }}` (True if the tag is stable)
- **Repo/Token:** Same as the stable formula.

### 3. GitHub Release Behavior
GoReleaser automatically detects semantic versioning suffixes (like `-rc.1` or `-nightly.20240115`) and sets the `prerelease` flag to `true` on the GitHub Release. This prevents it from being marked as "Latest."

## Testing Strategy
- **Manual Verification:** Review the generated GoReleaser configuration for syntax errors.
- **Dry-run (Optional):** If possible, run `goreleaser release --snapshot --clean` locally to ensure the configuration is valid (though this won't test the conditional Homebrew upload).
- **Validation:** Ensure `v[0-9]*.[0-9]*.[0-9]*-*` correctly matches the requested `v1.0.0-nightly.20240115` format.

## Rollout Plan
1. Update `.github/workflows/release.yml`.
2. Update `.goreleaser.yaml`.
3. Commit and push changes to `main`.

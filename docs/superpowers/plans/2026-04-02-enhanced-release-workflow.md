# Enhanced Release Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Support nightly builds without marking them as "Latest" on GitHub or Homebrew, and ensure releases only trigger from the `main` branch.

**Architecture:** Update GitHub Actions trigger with branch and tag filters, and transition GoReleaser from deprecated `brews` to `homebrew_casks` with conditional logic for stable vs. nightly releases.

**Tech Stack:** GitHub Actions, GoReleaser v2.

---

### Task 1: Update GitHub Actions Workflow

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Expand tag triggers and add branch restriction**

```yaml
on:
  push:
    tags:
      - "v[0-9]*.[0-9]*.[0-9]*"   # Stable: v1.0.0
      - "v[0-9]*.[0-9]*.[0-9]*-*" # Nightly: v1.0.0-nightly.20240115

permissions:
  contents: write

jobs:
  release:
    name: Release
    runs-on: ubuntu-latest
    if: github.event.base_ref == 'refs/heads/main' # Only run if tag is on main
    steps:
      # ... (rest of the file remains same)
```

- [ ] **Step 2: Verify YAML syntax**

Run: `yamllint .github/workflows/release.yml` (if available) or manually check for indentation.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: update release trigger with nightly tags and branch restriction"
```

### Task 2: Migrate GoReleaser to homebrew_casks (Stable)

**Files:**
- Modify: `.goreleaser.yaml`

- [ ] **Step 1: Replace brews with homebrew_casks for stable releases**

```yaml
homebrew_casks:
  # Stable cask (brew install knot)
  - name: knot
    skip_upload: "{{ .Prerelease }}"
    repository:
      owner: oxGrad
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    homepage: "https://github.com/oxGrad/knot"
    description: "A lightweight, configurable dotfiles manager"
    license: MIT
    # Note: Casks handle binary installation differently; usually no 'install' block needed if binary is at root
    # GoReleaser will generate a Cask that downloads the archive and links the binary.
```

- [ ] **Step 2: Commit**

```bash
git add .goreleaser.yaml
git commit -m "chore: migrate stable release to homebrew_casks"
```

### Task 3: Add Nightly Cask Configuration

**Files:**
- Modify: `.goreleaser.yaml`

- [ ] **Step 1: Add homebrew_casks entry for nightly builds**

```yaml
homebrew_casks:
  # Stable cask (brew install knot)
  - name: knot
    skip_upload: "{{ .Prerelease }}"
    repository:
      owner: oxGrad
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    homepage: "https://github.com/oxGrad/knot"
    description: "A lightweight, configurable dotfiles manager"
    license: MIT

  # Nightly cask (brew install knot-nightly)
  - name: knot-nightly
    skip_upload: "{{ not .Prerelease }}"
    repository:
      owner: oxGrad
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    homepage: "https://github.com/oxGrad/knot"
    description: "A lightweight, configurable dotfiles manager"
    license: MIT
```

- [ ] **Step 2: Check GoReleaser config validity**

Run: `goreleaser check`

- [ ] **Step 3: Commit**

```bash
git add .goreleaser.yaml
git commit -m "chore: add knot-nightly homebrew_cask"
```

### Task 4: Final Verification and Cleanup

- [ ] **Step 1: Verify design spec compliance**

Ensure `.Prerelease` logic correctly matches the spec.
- Stable (`v1.0.0`): `.Prerelease` is false -> `knot` uploaded, `knot-nightly` skipped.
- Nightly (`v1.0.0-nightly...`): `.Prerelease` is true -> `knot` skipped, `knot-nightly` uploaded.

- [ ] **Step 2: Remove temporary design spec (optional, follow project norms)**

- [ ] **Step 3: Final commit**

```bash
git commit --allow-empty -m "ci: complete enhanced release workflow implementation"
```

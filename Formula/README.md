# Homebrew Formula

This directory contains the Homebrew formula template for cloudcoop.

## Overview

The `cloudcoop.rb` file is a **template** showing the expected formula structure.
GoReleaser automatically generates the actual formula with correct versions and
checksums during releases and publishes it to a separate Homebrew tap repository.

## Installation (for users)

Once releases are set up, users can install cloudcoop via:

```bash
brew tap cloud-coop/tap
brew install cloudcoop
```

Or in a single command:

```bash
brew install cloud-coop/tap/cloudcoop
```

## How It Works

1. **Release Trigger**: When a new version tag (e.g., `v1.0.0`) is pushed to GitHub
2. **GoReleaser Builds**: The release workflow runs GoReleaser which:
   - Builds binaries for all platforms (darwin/linux, amd64/arm64)
   - Creates release archives with checksums
   - Generates the Homebrew formula with correct URLs and SHA256 hashes
   - Publishes the formula to the `cloud-coop/homebrew-tap` repository
3. **Homebrew Updates**: Users running `brew upgrade` get the new version

## GoReleaser Configuration

The Homebrew tap is configured in `.goreleaser.yml`:

```yaml
brews:
  - name: cloudcoop
    repository:
      owner: cloud-coop
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/cloud-coop/cloudcoop"
    description: "Terminal UI for managing sandboxed AI coding agents on cloud VMs"
    license: "Apache-2.0"
    install: |
      bin.install "cloudcoop"
    test: |
      assert_match "cloudcoop version", shell_output("#{bin}/cloudcoop version 2>&1")
```

## Prerequisites for Releases

1. **Homebrew Tap Repository**: Create `cloud-coop/homebrew-tap` repository on GitHub
2. **GitHub Token**: Add `HOMEBREW_TAP_GITHUB_TOKEN` secret to the main repository
   with write access to the tap repository
3. **GoReleaser Config**: Ensure `.goreleaser.yml` has the `brews` section configured

## Local Development

The formula in this directory is for reference only. To test formula changes:

1. Copy the formula to your local tap: `cp Formula/cloudcoop.rb $(brew --repo)/Library/Taps/cloud-coop/homebrew-tap/Formula/`
2. Update version and checksums manually
3. Test with: `brew install --build-from-source cloudcoop`

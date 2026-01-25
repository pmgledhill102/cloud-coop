# typed: false
# frozen_string_literal: true

# Homebrew formula for cloudcoop
#
# IMPORTANT: This file is a TEMPLATE showing the expected formula structure.
# GoReleaser automatically generates the actual formula and publishes it to the
# homebrew-tap repository during releases.
#
# To install cloudcoop via Homebrew (once the tap is set up):
#   brew tap cloud-coop/tap
#   brew install cloudcoop
#
# GoReleaser configuration (in .goreleaser.yml) handles:
# - Automatic version substitution
# - SHA256 checksum calculation
# - Multi-platform URL generation
# - Publishing to the homebrew-tap repository
#
# See .goreleaser.yml brews section for the actual configuration.
#
class Cloudcoop < Formula
  desc "Terminal UI for managing sandboxed AI coding agents on cloud VMs"
  homepage "https://github.com/cloud-coop/cloudcoop"
  license "Apache-2.0"
  version "0.0.0"

  on_macos do
    on_intel do
      url "https://github.com/cloud-coop/cloudcoop/releases/download/v0.0.0/cloudcoop_darwin_amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end

    on_arm do
      url "https://github.com/cloud-coop/cloudcoop/releases/download/v0.0.0/cloudcoop_darwin_arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/cloud-coop/cloudcoop/releases/download/v0.0.0/cloudcoop_linux_amd64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end

    on_arm do
      url "https://github.com/cloud-coop/cloudcoop/releases/download/v0.0.0/cloudcoop_linux_arm64.tar.gz"
      sha256 "0000000000000000000000000000000000000000000000000000000000000000"
    end
  end

  def install
    bin.install "cloudcoop"
  end

  test do
    assert_match "cloudcoop version", shell_output("#{bin}/cloudcoop version 2>&1")
  end
end

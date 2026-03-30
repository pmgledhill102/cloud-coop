#!/bin/bash
# setup.sh - Local development environment setup for cloudcoop
#
# This script prepares your local environment for development by:
# - Checking Go version compatibility
# - Installing pre-commit hooks
# - Installing golangci-lint for Go linting
# - Downloading Go module dependencies

set -euo pipefail

# Configuration
REQUIRED_GO_MAJOR=1
REQUIRED_GO_MINOR=21  # Minimum Go version for this project
PROJECT_GO_VERSION="1.25.8"  # Version specified in go.mod

# Colors for output (disabled if not a terminal)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

# Helper functions
info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

die() {
    error "$1"
    exit 1
}

# Check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check Go version
check_go_version() {
    info "Checking Go installation..."

    if ! command_exists go; then
        die "Go is not installed. Please install Go ${PROJECT_GO_VERSION} or later.
    Visit: https://go.dev/doc/install"
    fi

    local go_version
    go_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+(\.[0-9]+)?' | head -1)

    if [ -z "$go_version" ]; then
        die "Could not determine Go version. Please ensure Go is properly installed."
    fi

    # Extract major and minor version numbers
    local major minor
    major=$(echo "$go_version" | sed 's/go//' | cut -d. -f1)
    minor=$(echo "$go_version" | sed 's/go//' | cut -d. -f2)

    if [ "$major" -lt "$REQUIRED_GO_MAJOR" ] || \
       { [ "$major" -eq "$REQUIRED_GO_MAJOR" ] && [ "$minor" -lt "$REQUIRED_GO_MINOR" ]; }; then
        die "Go version ${go_version} is too old. Please install Go ${REQUIRED_GO_MAJOR}.${REQUIRED_GO_MINOR} or later.
    Your version: ${go_version}
    Required: go${REQUIRED_GO_MAJOR}.${REQUIRED_GO_MINOR}+
    Project uses: go${PROJECT_GO_VERSION}
    Visit: https://go.dev/doc/install"
    fi

    success "Go ${go_version} installed (project uses go${PROJECT_GO_VERSION})"
}

# Install pre-commit
install_precommit() {
    info "Checking pre-commit installation..."

    if command_exists pre-commit; then
        local version
        version=$(pre-commit --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "unknown")
        success "pre-commit ${version} is already installed"
    else
        info "Installing pre-commit..."

        if command_exists pip3; then
            pip3 install pre-commit
        elif command_exists pip; then
            pip install pre-commit
        elif command_exists brew; then
            brew install pre-commit
        else
            die "Could not install pre-commit. Please install it manually:
    pip install pre-commit
    or
    brew install pre-commit"
        fi

        success "pre-commit installed successfully"
    fi
}

# Run pre-commit install
setup_precommit_hooks() {
    info "Setting up pre-commit hooks..."

    if [ ! -f ".pre-commit-config.yaml" ]; then
        die "No .pre-commit-config.yaml found. Are you in the project root directory?"
    fi

    pre-commit install

    success "Pre-commit hooks installed"
}

# Install golangci-lint
install_golangci_lint() {
    info "Checking golangci-lint installation..."

    if command_exists golangci-lint; then
        local version
        version=$(golangci-lint --version 2>/dev/null | grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
        success "golangci-lint ${version} is already installed"
    else
        info "Installing golangci-lint..."

        if command_exists brew; then
            brew install golangci-lint
        elif command_exists go; then
            # Install using go install
            go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
        else
            die "Could not install golangci-lint. Please install it manually:
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    or
    brew install golangci-lint
    See: https://golangci-lint.run/usage/install/"
        fi

        success "golangci-lint installed successfully"
    fi
}

# Download Go module dependencies
download_go_modules() {
    info "Downloading Go module dependencies..."

    if [ ! -f "go.mod" ]; then
        die "No go.mod found. Are you in the project root directory?"
    fi

    go mod download

    success "Go modules downloaded"
}

# Verify the setup
verify_setup() {
    info "Verifying setup..."

    local errors=0

    if ! command_exists go; then
        error "Go is not available"
        ((errors++))
    fi

    if ! command_exists pre-commit; then
        error "pre-commit is not available"
        ((errors++))
    fi

    if ! command_exists golangci-lint; then
        error "golangci-lint is not available"
        ((errors++))
    fi

    if [ "$errors" -gt 0 ]; then
        die "Setup verification failed with ${errors} error(s)"
    fi

    success "All tools verified"
}

# Main setup function
main() {
    echo ""
    echo "========================================"
    echo "  cloudcoop Development Setup"
    echo "========================================"
    echo ""

    # Change to project root if we're in scripts/
    if [ "$(basename "$PWD")" = "scripts" ]; then
        cd ..
        info "Changed to project root: $PWD"
    fi

    # Ensure we're in the right directory
    if [ ! -f "go.mod" ] || [ ! -f ".pre-commit-config.yaml" ]; then
        die "Please run this script from the project root directory."
    fi

    check_go_version
    install_precommit
    setup_precommit_hooks
    install_golangci_lint
    download_go_modules
    verify_setup

    echo ""
    echo "========================================"
    echo -e "  ${GREEN}Setup complete!${NC}"
    echo "========================================"
    echo ""
    echo "You can now:"
    echo "  - Run 'make build' to compile the project"
    echo "  - Run 'make test' to run tests"
    echo "  - Run 'make lint' to run the linter"
    echo "  - Run 'pre-commit run --all-files' to check all files"
    echo ""
}

# Run main function
main "$@"

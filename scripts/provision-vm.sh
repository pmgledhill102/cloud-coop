#!/bin/bash
# VM Provisioning Script - Installs all development tooling for Claude Code agents
# Run as root via startup-script metadata or manually after VM creation
#
# This script implements the cloudcoop provisioning contract (ADR-0021):
# - Status reporting to /var/run/cloudcoop/provision-status
# - Progress reporting to /var/run/cloudcoop/provision-progress
# - Idempotent design (safe to re-run for retry/refresh/resume)
#
# Package management strategy:
# - Homebrew: languages, dev tools, CLIs, database clients (always current)
# - apt: system packages, Docker, Google Cloud CLI (Linux-specific)
#
# Status values: pending | running | completed | failed

set -e

# ============================================
# Cloudcoop Contract: Status & Progress Reporting
# ============================================
STATUS_DIR="/var/run/cloudcoop"
STATUS_FILE="$STATUS_DIR/provision-status"
PROGRESS_FILE="$STATUS_DIR/provision-progress"
LOG_DIR="/var/log/cloudcoop"
LOG_FILE="$LOG_DIR/provision.log"

TOTAL_STEPS=19
CURRENT_STEP=0

mkdir -p "$STATUS_DIR" "$LOG_DIR"
exec > >(tee -a "$LOG_FILE") 2>&1
trap 'echo -e "failed\nStep $CURRENT_STEP/$TOTAL_STEPS: $CURRENT_TASK failed with exit code $?" > "$STATUS_FILE"; exit 1' ERR

report_progress() {
    CURRENT_STEP=$((CURRENT_STEP + 1))
    CURRENT_TASK="$1"
    echo "$CURRENT_STEP/$TOTAL_STEPS $CURRENT_TASK" > "$PROGRESS_FILE"
    echo ""
    echo "=== [$CURRENT_STEP/$TOTAL_STEPS] $CURRENT_TASK ==="
}

echo "running" > "$STATUS_FILE"
echo "0/$TOTAL_STEPS Initializing" > "$PROGRESS_FILE"

echo "=== Cloudcoop VM Provisioning ==="
echo "Started at: $(date)"
echo "Hostname: $(hostname)"

export DEBIAN_FRONTEND=noninteractive
export HOME=/root

wait_for_apt() {
    local max_wait=300
    local waited=0
    while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 || \
          fuser /var/lib/apt/lists/lock >/dev/null 2>&1; do
        if [ $waited -ge $max_wait ]; then
            echo "Timeout waiting for apt locks"
            pkill -9 unattended-upgr 2>/dev/null || true
            sleep 2
            break
        fi
        echo "Waiting for apt lock (${waited}s)..."
        sleep 5
        waited=$((waited + 5))
    done
}

# ============================================
# Wait for cloud-init
# ============================================
report_progress "Waiting for cloud-init"
cloud-init status --wait || true
sleep 2  # Extra delay to ensure cloud-init has finished writing files

# ============================================
# Configure GCE APT Mirror
# ============================================
report_progress "Configuring GCE apt mirror"

GCP_ZONE=$(curl -sH "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/zone 2>/dev/null | cut -d/ -f4)
GCP_REGION=$(echo "$GCP_ZONE" | sed 's/-[a-z]$//')

if [ -n "$GCP_REGION" ]; then
    echo "Detected GCP region: $GCP_REGION"
    GCE_MIRROR="${GCP_REGION}.gce.ports.ubuntu.com"
    if curl -sI "http://${GCE_MIRROR}/" >/dev/null 2>&1; then
        echo "Using GCE mirror: $GCE_MIRROR"
        # Ubuntu 24.04+ uses deb822 format in /etc/apt/sources.list.d/ubuntu.sources
        SOURCES_FILE="/etc/apt/sources.list.d/ubuntu.sources"
        if [ -f "$SOURCES_FILE" ] && ! grep -q "gce.ports.ubuntu.com" "$SOURCES_FILE"; then
            sed -i "s|ports.ubuntu.com|${GCE_MIRROR}|g" "$SOURCES_FILE"
            # Verify the change was applied
            if grep -q "gce.ports.ubuntu.com" "$SOURCES_FILE"; then
                echo "GCE mirror configured successfully in $SOURCES_FILE"
            else
                echo "WARNING: Failed to configure GCE mirror in $SOURCES_FILE"
            fi
        fi
        # Fallback for older Ubuntu with traditional sources.list
        if [ -s /etc/apt/sources.list ] && ! grep -q "gce.ports.ubuntu.com" /etc/apt/sources.list 2>/dev/null; then
            sed -i "s|ports.ubuntu.com|${GCE_MIRROR}|g" /etc/apt/sources.list 2>/dev/null || true
        fi
    fi
fi

# Force IPv4 for APT (IPv6 can be slow/broken in cloud environments)
echo 'Acquire::ForceIPv4 "true";' > /etc/apt/apt.conf.d/99force-ipv4

# ============================================
# System Updates & Core Packages
# ============================================
report_progress "Updating system packages"
wait_for_apt
apt-get update
apt-get upgrade -y

report_progress "Installing core system packages"
apt-get install -y \
    build-essential \
    curl \
    wget \
    git \
    jq \
    unzip \
    zip \
    tmux \
    zsh \
    vim \
    nano \
    htop \
    ncdu \
    procps \
    sudo \
    ca-certificates \
    gnupg \
    lsb-release \
    iptables \
    ipset \
    iproute2 \
    dnsutils \
    sqlite3 \
    gdb \
    cmake \
    clang-format

# ============================================
# Create sandbox user (needed for Homebrew)
# ============================================
report_progress "Creating sandbox user"
if ! id "sandbox" &>/dev/null; then
    useradd -m -s /bin/zsh sandbox
    echo "sandbox ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/sandbox
fi

# ============================================
# Homebrew (package manager for dev tools)
# ============================================
report_progress "Installing Homebrew"
if [ ! -d /home/linuxbrew/.linuxbrew ]; then
    # Homebrew must be installed as non-root user
    sudo -u sandbox bash -c 'NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"'
fi

# Set up brew path for this script
eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
export PATH="/home/linuxbrew/.linuxbrew/bin:$PATH"

# Make Homebrew tools available to ALL users and ALL session types
# (interactive, non-interactive SSH, login, non-login) by adding to /etc/environment.
# PAM reads this for every session regardless of shell initialization.
if ! grep -q "linuxbrew" /etc/environment 2>/dev/null; then
    sed -i 's|^PATH="\(.*\)"|PATH="/home/linuxbrew/.linuxbrew/bin:/home/linuxbrew/.linuxbrew/sbin:\1"|' /etc/environment
fi

# Full brew shellenv for interactive sessions (sets HOMEBREW_PREFIX, MANPATH, etc.)
if ! grep -q "linuxbrew" /home/sandbox/.bashrc 2>/dev/null; then
    sed -i '/^# for examples$/a\
\
# Homebrew (must be before interactive guard for non-interactive SSH)\
eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"\
' /home/sandbox/.bashrc
    chown sandbox:sandbox /home/sandbox/.bashrc
fi

# ============================================
# Dev Tools via Homebrew (comprehensive)
# ============================================
report_progress "Installing dev tools via Homebrew"

# Change to a directory accessible by sandbox user (brew requires readable cwd)
cd /tmp

# Add HashiCorp tap for Terraform
sudo -u sandbox /home/linuxbrew/.linuxbrew/bin/brew tap hashicorp/tap

# Install all languages, tools, and CLIs via Homebrew
sudo -u sandbox /home/linuxbrew/.linuxbrew/bin/brew install \
    go \
    node \
    ruby \
    python@3 \
    openjdk@21 \
    php \
    rust \
    dotnet \
    maven \
    gradle \
    sbt \
    composer \
    awscli \
    azure-cli \
    kubernetes-cli \
    helm \
    k9s \
    hashicorp/tap/terraform \
    trivy \
    dive \
    crane \
    skopeo \
    postgresql@17 \
    mysql-client \
    redis \
    mongosh \
    dolt \
    beads \
    golangci-lint \
    actionlint \
    hadolint \
    shellcheck \
    tflint \
    git-delta \
    ripgrep \
    fd \
    fzf \
    yq \
    gh \
    btop \
    just \
    gastown

# ============================================
# Docker (using Ubuntu system packages)
# ============================================
report_progress "Installing Docker"
apt-get install -y docker.io docker-compose-v2 docker-buildx

systemctl enable docker
systemctl start docker
cat > /etc/docker/daemon.json <<EOF
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
EOF
systemctl restart docker

# ============================================
# Node.js global packages (Claude Code)
# ============================================
report_progress "Installing Node.js packages"
# Run npm as sandbox user with brew in PATH (npm's shebang uses /usr/bin/env node)
sudo -u sandbox bash -c 'eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)" && npm config set cafile /etc/ssl/certs/ca-certificates.crt && npm install -g @anthropic-ai/claude-code yarn pnpm typescript ts-node eslint prettier cspell markdownlint-cli2 playwright'

# ============================================
# Playwright browsers & system deps (Ubuntu 24.04)
# ============================================
report_progress "Installing Playwright browsers and system deps"
# System deps for Chromium, Firefox, WebKit on Ubuntu 24.04 (libasound2 works cleanly here, unlike 25.04+)
apt-get install -y \
    libglib2.0-0 libnss3 libnspr4 libatk1.0-0 \
    libatk-bridge2.0-0 libcups2 libdrm2 libdbus-1-3 \
    libxkbcommon0 libatspi2.0-0 libxcomposite1 \
    libxdamage1 libxrandr2 libgbm1 libpango-1.0-0 \
    libcairo2 libasound2 libx11-6 libxext6 libxfixes3 \
    xvfb

# Install Playwright browsers (as sandbox user with brew node)
sudo -u sandbox bash -c 'eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)" && npx playwright install --with-deps'

# ============================================
# Python tools via uv
# ============================================
report_progress "Installing Python tools"
curl -LsSf https://astral.sh/uv/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"

uv tool install pipx
uv tool install poetry
uv tool install black
uv tool install ruff
uv tool install mypy
uv tool install pytest
uv tool install pre-commit
uv tool install httpie
uv tool install yamllint
uv tool install checkov

# ============================================
# Ruby gems
# ============================================
report_progress "Installing Ruby gems"
# Run gem as sandbox user with brew in PATH (gem may invoke ruby via env)
sudo -u sandbox bash -c 'eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)" && gem install bundler rubocop'

# ============================================
# Testing tools
# ============================================
report_progress "Installing testing tools"

# CLI testing
apt-get install -y bats shunit2 expect

# TUI testing
apt-get install -y asciinema

# E2E browser testing system deps (Cypress)
apt-get install -y \
    libgtk2.0-0 libgtk-3-0 libnotify-dev \
    libgconf-2-4 libxss1 libxtst6 xauth

# npm testing tools: E2E, unit testing, a11y
sudo -u sandbox bash -c 'eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)" && npm install -g cypress puppeteer jest vitest tsx @axe-core/cli pa11y'

# Python testing tools
uv tool install pexpect
uv tool install hypothesis
uv tool install pytest-cov
uv tool install pytest-xdist
uv tool install pytest-asyncio
uv tool install textual-dev

# Go testing tools
sudo -u sandbox bash -c 'eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)" && go install gotest.tools/gotestsum@latest && go install github.com/charmbracelet/vhs@latest'

# Rust testing tools
sudo -u sandbox bash -c 'eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)" && cargo install cargo-nextest cargo-tarpaulin'

# API testing: k6
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg \
    --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" \
    | tee /etc/apt/sources.list.d/k6.list
apt-get update && apt-get install -y k6

# API testing: hurl
curl -LO https://github.com/Orange-OpenSource/hurl/releases/latest/download/hurl_amd64.deb
dpkg -i hurl_amd64.deb || apt-get install -f -y
rm -f hurl_amd64.deb

# CI testing: act (run GitHub Actions locally)
curl --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/nektos/act/master/install.sh | bash

# ============================================
# Google Cloud CLI (apt required for Linux)
# ============================================
report_progress "Installing Google Cloud CLI"
echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee /etc/apt/sources.list.d/google-cloud-sdk.list
curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --batch --yes --dearmor -o /usr/share/keyrings/cloud.google.gpg
apt-get update
apt-get install -y google-cloud-cli google-cloud-cli-gke-gcloud-auth-plugin

# ============================================
# ZSH + Oh My Zsh
# ============================================
report_progress "Configuring ZSH"
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended || true

# ============================================
# Configure sandbox user
# ============================================
report_progress "Configuring sandbox user"
usermod -aG docker sandbox

# Copy Oh My Zsh to sandbox user
cp -r /root/.oh-my-zsh /home/sandbox/ 2>/dev/null || true
chown -R sandbox:sandbox /home/sandbox/

# ============================================
# Workspace directories
# ============================================
report_progress "Creating workspace directories"
mkdir -p /workspaces
for i in $(seq 1 16); do
    mkdir -p /workspaces/agent-$i
done
chown -R sandbox:sandbox /workspaces

# ============================================
# Environment setup
# ============================================
cat > /home/sandbox/.zshrc <<'EOF'
export ZSH="$HOME/.oh-my-zsh"
ZSH_THEME="robbyrussell"
plugins=(git docker kubectl)
source $ZSH/oh-my-zsh.sh

# Homebrew (includes all languages and tools)
eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"

# Keg-only formula paths (not linked to main bin)
export PATH="/home/linuxbrew/.linuxbrew/opt/postgresql@17/bin:$PATH"
export PATH="/home/linuxbrew/.linuxbrew/opt/mysql-client/bin:$PATH"

# Aliases
alias ll='ls -la'
alias k='kubectl'
alias d='docker'
alias g='git'

# Load API key from metadata if available
if command -v curl &> /dev/null; then
    METADATA_KEY=$(curl -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/attributes/anthropic-api-key" 2>/dev/null)
    if [ -n "$METADATA_KEY" ] && [ "$METADATA_KEY" != "" ]; then
        export ANTHROPIC_API_KEY="$METADATA_KEY"
    fi
fi
EOF
chown sandbox:sandbox /home/sandbox/.zshrc

# tmux config
cat > /home/sandbox/.tmux.conf <<'EOF'
# Enable mouse support (for clicking URLs, scrolling, selecting panes)
set -g mouse on

# Allow passthrough of OSC sequences (enables clickable hyperlinks in Ghostty, etc.)
set -g allow-passthrough on

# Proper terminal settings for modern features
set -g default-terminal "tmux-256color"
set -as terminal-features ",xterm-256color:RGB"
set -as terminal-features ",*:hyperlinks"

# Enable focus events (helps with some terminal features)
set -g focus-events on

# Scrollback buffer
set -g history-limit 50000

# Window/pane numbering
set -g base-index 1
setw -g pane-base-index 1
EOF
chown sandbox:sandbox /home/sandbox/.tmux.conf

# git config
cat > /home/sandbox/.gitconfig <<'EOF'
[core]
    pager = delta
[interactive]
    diffFilter = delta --color-only
[delta]
    navigate = true
    light = false
    line-numbers = true
[init]
    defaultBranch = main
[pull]
    rebase = true
EOF
chown sandbox:sandbox /home/sandbox/.gitconfig

# Agent scripts
cat > /usr/local/bin/start-agents.sh <<'SCRIPT'
#!/bin/bash
AGENT_COUNT=${1:-12}
SESSION_NAME="claude-agents"
tmux kill-session -t $SESSION_NAME 2>/dev/null || true
tmux new-session -d -s $SESSION_NAME -n "agent-1"
for i in $(seq 2 $AGENT_COUNT); do
    tmux new-window -t $SESSION_NAME -n "agent-$i"
done
for i in $(seq 1 $AGENT_COUNT); do
    tmux send-keys -t $SESSION_NAME:agent-$i "cd /workspaces/agent-$i && claude --dangerously-skip-permissions" Enter
done
echo "Started $AGENT_COUNT agents. Attach with: tmux attach -t $SESSION_NAME"
SCRIPT
chmod +x /usr/local/bin/start-agents.sh

# ============================================
# Cleanup
# ============================================
report_progress "Cleaning up"
apt-get autoremove -y
apt-get clean

# ============================================
# Dotfiles (optional, from VM metadata)
# ============================================
DOTFILES_URL=$(curl -s -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/attributes/cloudcoop-dotfiles-url" 2>/dev/null || true)

if [ -n "$DOTFILES_URL" ]; then
    report_progress "Applying dotfiles"
    echo "Dotfiles install script: $DOTFILES_URL"

    # Pre-seed chezmoi config for non-interactive init (headless VM = minimal)
    CHEZMOI_CONFIG_DIR="/home/sandbox/.config/chezmoi"
    mkdir -p "$CHEZMOI_CONFIG_DIR"
    cat > "$CHEZMOI_CONFIG_DIR/chezmoi.toml" <<'CHEZMOI_EOF'
[data]
    machine_type = "minimal"
CHEZMOI_EOF
    chown -R sandbox:sandbox "/home/sandbox/.config"

    # Run the dotfiles install script as the sandbox user
    su - sandbox -c "sh -c \"\$(curl -fsSL '$DOTFILES_URL')\"" || {
        echo "WARNING: Dotfiles install failed (non-fatal), continuing..."
    }
else
    report_progress "Dotfiles (skipped, no URL configured)"
fi

# ============================================
# Done
# ============================================
echo "completed" > "$STATUS_FILE"
echo "$TOTAL_STEPS/$TOTAL_STEPS Provisioning complete" > "$PROGRESS_FILE"

echo ""
echo "=== Provisioning Complete ==="
echo "Finished at: $(date)"
echo ""
echo "Installed via Homebrew:"
brew list --formula | tr '\n' ' '
echo ""
echo ""
echo "Key versions:"
echo "  - Node.js    : $(node --version 2>/dev/null || echo 'N/A')"
echo "  - Go         : $(go version 2>/dev/null | awk '{print $3}' || echo 'N/A')"
echo "  - Python     : $(python3 --version 2>/dev/null || echo 'N/A')"
echo "  - Ruby       : $(ruby --version 2>/dev/null | awk '{print $2}' || echo 'N/A')"
echo "  - Rust       : $(rustc --version 2>/dev/null | awk '{print $2}' || echo 'N/A')"
echo "  - Java       : $(java --version 2>/dev/null | head -1 || echo 'N/A')"
echo "  - .NET       : $(dotnet --version 2>/dev/null || echo 'N/A')"
echo "  - Docker     : $(docker --version 2>/dev/null | awk '{print $3}' | tr -d ',' || echo 'N/A')"
echo "  - Terraform  : $(terraform --version 2>/dev/null | head -1 | awk '{print $2}' || echo 'N/A')"
echo "  - kubectl    : $(kubectl version --client -o json 2>/dev/null | jq -r '.clientVersion.gitVersion' || echo 'N/A')"
echo "  - AWS CLI    : $(aws --version 2>/dev/null | awk '{print $1}' || echo 'N/A')"
echo "  - Azure CLI  : $(az --version 2>/dev/null | head -1 | awk '{print $2}' || echo 'N/A')"
echo "  - Dolt       : $(dolt version 2>/dev/null | head -1 | awk '{print $3}' || echo 'N/A')"
echo "  - Beads      : $(bd --version 2>/dev/null | awk '{print $3}' || echo 'N/A')"
echo "  - Claude Code: $(claude --version 2>/dev/null || echo 'N/A')"
echo "  - Gemini CLI : $(gemini --version 2>/dev/null || echo 'N/A')"
echo "  - Gastown    : $(gastown --version 2>/dev/null || echo 'N/A')"
echo "  - Playwright : $(npx playwright --version 2>/dev/null || echo 'N/A')"
echo "  - k6         : $(k6 version 2>/dev/null || echo 'N/A')"
echo ""
echo "Quick start:"
echo "  1. SSH: gcloud compute ssh $(hostname) --zone=ZONE --tunnel-through-iap"
echo "  2. Switch user: sudo su - sandbox"
echo "  3. Start agents: start-agents.sh 12"

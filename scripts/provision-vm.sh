#!/bin/bash
# VM Provisioning Script - Installs all development tooling for Claude Code agents
# Run as root via startup-script metadata or manually after VM creation
#
# This script implements the cloudcoop provisioning contract (ADR-0021):
# - Status reporting to /var/run/cloudcoop/provision-status
# - Progress reporting to /var/run/cloudcoop/provision-progress
# - Idempotent design (safe to re-run for retry/refresh/resume)
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

# Total number of provisioning steps (for progress reporting)
TOTAL_STEPS=36
CURRENT_STEP=0

# Create required directories
mkdir -p "$STATUS_DIR" "$LOG_DIR"

# Redirect output to log file
exec > >(tee -a "$LOG_FILE") 2>&1

# Error trap: capture failures and write to status file
trap 'echo -e "failed\nStep $CURRENT_STEP/$TOTAL_STEPS: $CURRENT_TASK failed with exit code $?" > "$STATUS_FILE"; exit 1' ERR

# Progress reporting function
report_progress() {
    CURRENT_STEP=$((CURRENT_STEP + 1))
    CURRENT_TASK="$1"
    echo "$CURRENT_STEP/$TOTAL_STEPS $CURRENT_TASK" > "$PROGRESS_FILE"
    echo ""
    echo "=== [$CURRENT_STEP/$TOTAL_STEPS] $CURRENT_TASK ==="
}

# Mark provisioning as running
echo "running" > "$STATUS_FILE"
echo "0/$TOTAL_STEPS Initializing" > "$PROGRESS_FILE"

echo "=== Cloudcoop VM Provisioning ==="
echo "Started at: $(date)"
echo "Hostname: $(hostname)"
echo "Status file: $STATUS_FILE"
echo "Progress file: $PROGRESS_FILE"
echo "Log file: $LOG_FILE"

# ============================================
# Load versions from config file (if available)
# ============================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSIONS_FILE="${SCRIPT_DIR}/../config/versions.env"

if [ -f "$VERSIONS_FILE" ]; then
    echo "Loading versions from $VERSIONS_FILE"
    # shellcheck source=/dev/null
    source "$VERSIONS_FILE"
else
    echo "No versions.env found, using defaults"
fi

# Default versions (overridden by versions.env if present)
# These match the discord-bot-test-suite CI requirements
NODE_VERSION="${NODE_VERSION:-24}"
GO_VERSION="${GO_VERSION:-1.25.3}"
PYTHON_VERSION="${PYTHON_VERSION:-3.14}"
RUST_VERSION="${RUST_VERSION:-1.93}"
JAVA_VERSION="${JAVA_VERSION:-21}"
RUBY_VERSION="${RUBY_VERSION:-3.4}"
PHP_VERSION="${PHP_VERSION:-8.5}"
DOTNET_VERSION="${DOTNET_VERSION:-10.0}"
GRADLE_VERSION="${GRADLE_VERSION:-8.12}"
SBT_VERSION="${SBT_VERSION:-1.10.6}"
DELTA_VERSION="${DELTA_VERSION:-0.18.2}"
GOLANGCI_LINT_VERSION="${GOLANGCI_LINT_VERSION:-v2.8.0}"
ACTIONLINT_VERSION="${ACTIONLINT_VERSION:-1.7.7}"
HADOLINT_VERSION="${HADOLINT_VERSION:-2.12.0}"
KUBECTL_VERSION="${KUBECTL_VERSION:-1.30}"

export DEBIAN_FRONTEND=noninteractive

# Wait for cloud-init to complete
cloud-init status --wait || true

# ============================================
# Configure GCE APT Mirror (3x faster downloads)
# ============================================
report_progress "Configuring GCE apt mirror"

# Detect GCP region from metadata
GCP_ZONE=$(curl -sH "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/zone 2>/dev/null | cut -d/ -f4)
GCP_REGION=$(echo "$GCP_ZONE" | sed 's/-[a-z]$//')

if [ -n "$GCP_REGION" ]; then
    echo "Detected GCP region: $GCP_REGION"
    # Use GCE-internal Ubuntu mirror for faster ARM64 downloads
    GCE_MIRROR="${GCP_REGION}.gce.ports.ubuntu.com"

    # Test if GCE mirror is reachable
    if curl -sI "http://${GCE_MIRROR}/" >/dev/null 2>&1; then
        echo "Using GCE mirror: $GCE_MIRROR"
        # Replace ports.ubuntu.com with GCE mirror in sources (idempotent)
        # Only replace if not already using a GCE mirror
        # Handle both legacy sources.list and new deb822 .sources format
        if ! grep -q "gce.ports.ubuntu.com" /etc/apt/sources.list 2>/dev/null; then
            sed -i "s|ports.ubuntu.com|${GCE_MIRROR}|g" /etc/apt/sources.list 2>/dev/null || true
        fi
        for f in /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources; do
            if [ -f "$f" ] && ! grep -q "gce.ports.ubuntu.com" "$f" 2>/dev/null; then
                sed -i "s|ports.ubuntu.com|${GCE_MIRROR}|g" "$f" 2>/dev/null || true
            fi
        done
    else
        echo "GCE mirror not reachable, using default ports.ubuntu.com"
    fi
else
    echo "Not running on GCP, using default mirrors"
fi

# ============================================
# System Updates
# ============================================
report_progress "Updating system packages"
apt-get update
apt-get upgrade -y

# ============================================
# Core System Packages (from Anthropic devcontainer + CI requirements)
# ============================================
report_progress "Installing core system packages"
apt-get install -y \
    apt-transport-https \
    ca-certificates \
    curl \
    wget \
    gnupg \
    gnupg2 \
    lsb-release \
    software-properties-common \
    build-essential \
    cmake \
    git \
    less \
    procps \
    sudo \
    fzf \
    zsh \
    man-db \
    unzip \
    zip \
    iptables \
    ipset \
    iproute2 \
    dnsutils \
    jq \
    nano \
    vim \
    tmux \
    screen \
    htop \
    ncdu \
    clang-format \
    sqlite3 \
    gdb \
    skopeo

# ============================================
# git-delta (from Anthropic devcontainer)
# ============================================
report_progress "Installing git-delta ${DELTA_VERSION}"
ARCH=$(dpkg --print-architecture)
wget -q "https://github.com/dandavison/delta/releases/download/${DELTA_VERSION}/git-delta_${DELTA_VERSION}_${ARCH}.deb" -O /tmp/git-delta.deb
dpkg -i /tmp/git-delta.deb || apt-get install -f -y
rm /tmp/git-delta.deb

# ============================================
# ripgrep and fd-find
# ============================================
report_progress "Installing ripgrep and fd-find"
apt-get install -y ripgrep fd-find

# fd is installed as fdfind on Ubuntu - create symlink
ln -sf "$(which fdfind)" /usr/local/bin/fd

# ============================================
# GitHub CLI
# ============================================
report_progress "Installing GitHub CLI"
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | tee /etc/apt/sources.list.d/github-cli.list > /dev/null
apt-get update
apt-get install -y gh

# ============================================
# Docker
# ============================================
report_progress "Installing Docker"
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

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
# Node.js (Claude Code runtime + JS/TS services)
# ============================================
report_progress "Installing Node.js ${NODE_VERSION}"
curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash -
apt-get install -y nodejs

# Install global npm packages (including linting tools from pre-commit)
npm install -g \
    @anthropic-ai/claude-code \
    yarn \
    pnpm \
    typescript \
    ts-node \
    eslint \
    prettier \
    cspell \
    markdownlint-cli2

# ============================================
# Python
# ============================================
report_progress "Installing Python"

# Try deadsnakes PPA for specific Python version, fall back to system Python
# deadsnakes may not support newest Ubuntu versions
SYSTEM_PYTHON_VERSION=$(python3 --version 2>/dev/null | grep -oP '\d+\.\d+' || echo "")

if [ -n "$SYSTEM_PYTHON_VERSION" ]; then
    echo "System Python version: $SYSTEM_PYTHON_VERSION"
fi

# Check if deadsnakes supports this Ubuntu release
UBUNTU_CODENAME=$(lsb_release -cs)
if add-apt-repository -y ppa:deadsnakes/ppa 2>&1 | grep -q "does not have a Release file"; then
    echo "deadsnakes PPA does not support Ubuntu $UBUNTU_CODENAME, using system Python $SYSTEM_PYTHON_VERSION"
    PYTHON_VERSION="$SYSTEM_PYTHON_VERSION"
    # Ensure venv and dev packages are installed for system Python
    apt-get install -y python3-venv python3-dev || true
else
    apt-get update
    # Try to install requested Python version
    if apt-get install -y python${PYTHON_VERSION} python${PYTHON_VERSION}-venv python${PYTHON_VERSION}-dev 2>/dev/null; then
        echo "Installed Python ${PYTHON_VERSION} from deadsnakes"
        update-alternatives --install /usr/bin/python3 python3 /usr/bin/python${PYTHON_VERSION} 1
        update-alternatives --install /usr/bin/python python /usr/bin/python${PYTHON_VERSION} 1
    else
        echo "Python ${PYTHON_VERSION} not available, using system Python $SYSTEM_PYTHON_VERSION"
        PYTHON_VERSION="$SYSTEM_PYTHON_VERSION"
        apt-get install -y python3-venv python3-dev || true
    fi
fi

echo "Using Python version: $(python3 --version)"

# Install uv (fast Python package manager)
report_progress "Installing uv package manager"
curl -LsSf https://astral.sh/uv/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"

# Install Python tools using uv (including pre-commit hooks: ruff, yamllint)
uv tool install pipx
uv tool install poetry
uv tool install black
uv tool install ruff
uv tool install mypy
uv tool install pytest
uv tool install pre-commit
uv tool install httpie
uv tool install yamllint

# ============================================
# Go
# ============================================
report_progress "Installing Go ${GO_VERSION}"
wget -q "https://go.dev/dl/go${GO_VERSION}.linux-$(dpkg --print-architecture).tar.gz" -O /tmp/go.tar.gz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tar.gz
rm /tmp/go.tar.gz

cat > /etc/profile.d/go.sh <<'EOF'
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
EOF

export PATH=$PATH:/usr/local/go/bin
export GOPATH=/root/go
export PATH=$PATH:$GOPATH/bin

# Install Go tools (golangci-lint v2 from pre-commit)
report_progress "Installing golangci-lint ${GOLANGCI_LINT_VERSION}"
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b /usr/local/bin ${GOLANGCI_LINT_VERSION}

go install github.com/go-delve/delve/cmd/dlv@latest

# ============================================
# actionlint (GitHub Actions linting - from pre-commit)
# ============================================
report_progress "Installing actionlint ${ACTIONLINT_VERSION}"
wget -q "https://github.com/rhysd/actionlint/releases/download/v${ACTIONLINT_VERSION}/actionlint_${ACTIONLINT_VERSION}_linux_$(dpkg --print-architecture).tar.gz" -O /tmp/actionlint.tar.gz
tar -xzf /tmp/actionlint.tar.gz -C /usr/local/bin actionlint
chmod +x /usr/local/bin/actionlint
rm /tmp/actionlint.tar.gz

# ============================================
# hadolint (Dockerfile linting - from pre-commit)
# ============================================
report_progress "Installing hadolint ${HADOLINT_VERSION}"
HADOLINT_ARCH=$(dpkg --print-architecture)
if [ "$HADOLINT_ARCH" = "amd64" ]; then HADOLINT_ARCH="x86_64"; fi
wget -q "https://github.com/hadolint/hadolint/releases/download/v${HADOLINT_VERSION}/hadolint-Linux-${HADOLINT_ARCH}" -O /usr/local/bin/hadolint
chmod +x /usr/local/bin/hadolint

# ============================================
# ShellCheck (Shell linting - from pre-commit)
# ============================================
report_progress "Installing ShellCheck"
apt-get install -y shellcheck

# ============================================
# Rust
# ============================================
report_progress "Installing Rust ${RUST_VERSION}"
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain ${RUST_VERSION}
source "$HOME/.cargo/env"
rustup component add clippy rustfmt

# ============================================
# Java (Temurin)
# ============================================
report_progress "Installing Java ${JAVA_VERSION}"
wget -qO - https://packages.adoptium.net/artifactory/api/gpg/key/public | gpg --dearmor | tee /etc/apt/keyrings/adoptium.gpg > /dev/null
echo "deb [signed-by=/etc/apt/keyrings/adoptium.gpg] https://packages.adoptium.net/artifactory/deb $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/adoptium.list
apt-get update
apt-get install -y temurin-${JAVA_VERSION}-jdk

# Maven
apt-get install -y maven

# Gradle
report_progress "Installing Gradle ${GRADLE_VERSION}"
wget -q "https://services.gradle.org/distributions/gradle-${GRADLE_VERSION}-bin.zip" -O /tmp/gradle.zip
unzip -q /tmp/gradle.zip -d /opt
ln -sf /opt/gradle-${GRADLE_VERSION}/bin/gradle /usr/local/bin/gradle
rm /tmp/gradle.zip

# ============================================
# Scala / sbt
# ============================================
report_progress "Installing sbt ${SBT_VERSION}"
wget -q "https://github.com/sbt/sbt/releases/download/v${SBT_VERSION}/sbt-${SBT_VERSION}.tgz" -O /tmp/sbt.tgz
tar -xzf /tmp/sbt.tgz -C /opt
ln -sf /opt/sbt/bin/sbt /usr/local/bin/sbt
rm /tmp/sbt.tgz

# ============================================
# Ruby
# ============================================
report_progress "Installing Ruby ${RUBY_VERSION}"
# Use rbenv for version management
apt-get install -y \
    autoconf \
    bison \
    libssl-dev \
    libyaml-dev \
    libreadline-dev \
    zlib1g-dev \
    libncurses-dev \
    libffi-dev \
    libgdbm-dev

git clone https://github.com/rbenv/rbenv.git /opt/rbenv
git clone https://github.com/rbenv/ruby-build.git /opt/rbenv/plugins/ruby-build

export PATH="/opt/rbenv/bin:/opt/rbenv/shims:$PATH"
eval "$(rbenv init -)"

# Install Ruby (this takes a while)
rbenv install ${RUBY_VERSION}.0 || rbenv install ${RUBY_VERSION}.1 || apt-get install -y ruby-full
rbenv global ${RUBY_VERSION}.0 2>/dev/null || rbenv global ${RUBY_VERSION}.1 2>/dev/null || true

# Install gems (rubocop from pre-commit)
gem install bundler rubocop

# ============================================
# PHP
# ============================================
report_progress "Installing PHP ${PHP_VERSION}"
add-apt-repository -y ppa:ondrej/php
apt-get update
apt-get install -y \
    php${PHP_VERSION} \
    php${PHP_VERSION}-cli \
    php${PHP_VERSION}-common \
    php${PHP_VERSION}-curl \
    php${PHP_VERSION}-mbstring \
    php${PHP_VERSION}-xml \
    php${PHP_VERSION}-zip \
    php${PHP_VERSION}-sodium

# Composer
curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer

# ============================================
# .NET
# ============================================
report_progress "Installing .NET ${DOTNET_VERSION}"
wget "https://packages.microsoft.com/config/ubuntu/$(lsb_release -rs)/packages-microsoft-prod.deb" -O /tmp/packages-microsoft-prod.deb
dpkg -i /tmp/packages-microsoft-prod.deb
rm /tmp/packages-microsoft-prod.deb
apt-get update
apt-get install -y dotnet-sdk-${DOTNET_VERSION} || apt-get install -y dotnet-sdk-8.0

# ============================================
# Google Cloud CLI
# ============================================
report_progress "Installing Google Cloud CLI"
echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee /etc/apt/sources.list.d/google-cloud-sdk.list
curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg
apt-get update
apt-get install -y google-cloud-cli google-cloud-cli-gke-gcloud-auth-plugin

# ============================================
# AWS CLI
# ============================================
report_progress "Installing AWS CLI"
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o /tmp/awscliv2.zip
unzip -q /tmp/awscliv2.zip -d /tmp
/tmp/aws/install
rm -rf /tmp/aws /tmp/awscliv2.zip

# ============================================
# Azure CLI
# ============================================
report_progress "Installing Azure CLI"
curl -sL https://aka.ms/InstallAzureCLIDeb | bash

# ============================================
# Terraform
# ============================================
report_progress "Installing Terraform"
wget -O- https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/hashicorp.list
apt-get update
apt-get install -y terraform

# ============================================
# Kubernetes Tools
# ============================================
report_progress "Installing Kubernetes tools"
curl -fsSL https://pkgs.k8s.io/core:/stable:/v${KUBECTL_VERSION}/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v${KUBECTL_VERSION}/deb/ /" | tee /etc/apt/sources.list.d/kubernetes.list
apt-get update
apt-get install -y kubectl

# Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# k9s
wget -q "https://github.com/derailed/k9s/releases/latest/download/k9s_Linux_$(dpkg --print-architecture).tar.gz" -O /tmp/k9s.tar.gz
tar -xzf /tmp/k9s.tar.gz -C /usr/local/bin k9s
rm /tmp/k9s.tar.gz

# ============================================
# Database Clients
# ============================================
report_progress "Installing database clients"
apt-get install -y \
    postgresql-client \
    default-mysql-client \
    redis-tools

wget -qO - https://www.mongodb.org/static/pgp/server-7.0.asc | gpg --dearmor -o /usr/share/keyrings/mongodb-server-7.0.gpg
echo "deb [signed-by=/usr/share/keyrings/mongodb-server-7.0.gpg] https://repo.mongodb.org/apt/ubuntu $(lsb_release -cs)/mongodb-org/7.0 multiverse" | tee /etc/apt/sources.list.d/mongodb-org-7.0.list
apt-get update
apt-get install -y mongodb-mongosh || true

# ============================================
# Additional CLI Tools
# ============================================
report_progress "Installing additional CLI tools"

# yq (YAML processor - used in CI)
wget -q "https://github.com/mikefarah/yq/releases/latest/download/yq_linux_$(dpkg --print-architecture)" -O /usr/local/bin/yq
chmod +x /usr/local/bin/yq

# btop
apt-get install -y btop || true

# micro editor
curl https://getmic.ro | bash
mv micro /usr/local/bin/

# grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
cp /root/go/bin/grpcurl /usr/local/bin/ 2>/dev/null || true

# Beads (bd) - Issue tracking
echo "Installing Beads (bd)"
curl -fsSL https://raw.githubusercontent.com/steveyegge/beads/main/scripts/install.sh | bash || true

# ============================================
# Container Tools
# ============================================
report_progress "Installing container tools"

# crane (container registry tool)
go install github.com/google/go-containerregistry/cmd/crane@latest
cp /root/go/bin/crane /usr/local/bin/ 2>/dev/null || true

# dive (container layer analyzer)
DIVE_VERSION=$(curl -s https://api.github.com/repos/wagoodman/dive/releases/latest | grep tag_name | cut -d '"' -f 4)
wget -q "https://github.com/wagoodman/dive/releases/download/${DIVE_VERSION}/dive_${DIVE_VERSION#v}_linux_amd64.deb" -O /tmp/dive.deb
dpkg -i /tmp/dive.deb || apt-get install -f -y
rm /tmp/dive.deb

# ============================================
# Security Scanning Tools
# ============================================
report_progress "Installing security tools"

# Trivy
wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | gpg --dearmor -o /usr/share/keyrings/trivy.gpg
echo "deb [signed-by=/usr/share/keyrings/trivy.gpg] https://aquasecurity.github.io/trivy-repo/deb generic main" | tee /etc/apt/sources.list.d/trivy.list
apt-get update
apt-get install -y trivy

# Semgrep
uv tool install semgrep

# ============================================
# ZSH Configuration (from Anthropic devcontainer)
# ============================================
report_progress "Configuring ZSH"
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended || true

# ============================================
# Create sandbox user
# ============================================
report_progress "Creating sandbox user"
if ! id "sandbox" &>/dev/null; then
    useradd -m -s /bin/zsh sandbox
    usermod -aG docker sandbox
    echo "sandbox ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/sandbox
fi

# Copy tools to sandbox user
cp -r /root/.cargo /home/sandbox/ 2>/dev/null || true
cp -r /root/go /home/sandbox/ 2>/dev/null || true
cp -r /root/.oh-my-zsh /home/sandbox/ 2>/dev/null || true
chown -R sandbox:sandbox /home/sandbox/

# ============================================
# Create workspace directories
# ============================================
report_progress "Creating workspace directories"
mkdir -p /workspaces
for i in $(seq 1 16); do
    mkdir -p /workspaces/agent-$i
done
chown -R sandbox:sandbox /workspaces

# ============================================
# Environment setup for sandbox user
# ============================================
cat > /home/sandbox/.zshrc <<'EOF'
export ZSH="$HOME/.oh-my-zsh"
ZSH_THEME="robbyrussell"
plugins=(git docker kubectl)
source $ZSH/oh-my-zsh.sh

# Path additions
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin:$HOME/.cargo/bin:/opt/rbenv/bin:/opt/rbenv/shims
export GOPATH=$HOME/go

# rbenv init
if command -v rbenv &> /dev/null; then
    eval "$(rbenv init -)"
fi

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

# ============================================
# tmux configuration
# ============================================
cat > /home/sandbox/.tmux.conf <<'EOF'
set -g mouse on
set -g history-limit 50000
set -g base-index 1
setw -g pane-base-index 1
set -g default-terminal "screen-256color"
set -g status-bg colour235
set -g status-fg white
set -g status-left '[#S] '
set -g status-right '#H | %H:%M'
bind -n M-Left select-pane -L
bind -n M-Right select-pane -R
bind -n M-Up select-pane -U
bind -n M-Down select-pane -D
EOF
chown sandbox:sandbox /home/sandbox/.tmux.conf

# ============================================
# Agent management scripts
# ============================================
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

echo "Started $AGENT_COUNT agent sessions in tmux session '$SESSION_NAME'"
echo "Attach with: tmux attach -t $SESSION_NAME"
echo "Or use: attach-agent.sh <number>"
SCRIPT

cat > /usr/local/bin/stop-agents.sh <<'SCRIPT'
#!/bin/bash
tmux kill-session -t claude-agents 2>/dev/null && echo "Stopped all agents" || echo "No agents running"
SCRIPT

cat > /usr/local/bin/attach-agent.sh <<'SCRIPT'
#!/bin/bash
AGENT_NUM=${1:-1}
tmux select-window -t claude-agents:agent-$AGENT_NUM 2>/dev/null
tmux attach -t claude-agents
SCRIPT

cat > /usr/local/bin/list-agents.sh <<'SCRIPT'
#!/bin/bash
tmux list-windows -t claude-agents 2>/dev/null || echo "No agents running"
SCRIPT

chmod +x /usr/local/bin/*.sh

# ============================================
# Git configuration
# ============================================
cat > /home/sandbox/.gitconfig <<'EOF'
[core]
    pager = delta
[interactive]
    diffFilter = delta --color-only
[delta]
    navigate = true
    light = false
    line-numbers = true
[merge]
    conflictstyle = diff3
[diff]
    colorMoved = default
[init]
    defaultBranch = main
[pull]
    rebase = true
EOF
chown sandbox:sandbox /home/sandbox/.gitconfig

# ============================================
# Cleanup
# ============================================
report_progress "Cleaning up"
apt-get autoremove -y
apt-get clean
rm -rf /var/lib/apt/lists/*

# ============================================
# Summary
# ============================================

# Mark provisioning as completed
echo "completed" > "$STATUS_FILE"
echo "$TOTAL_STEPS/$TOTAL_STEPS Provisioning complete" > "$PROGRESS_FILE"

echo ""
echo "=== Provisioning Complete ==="
echo "Finished at: $(date)"
echo "Status: completed (written to $STATUS_FILE)"
echo ""
echo "Installed versions:"
echo "  - Node.js    : $(node --version 2>/dev/null || echo 'N/A')"
echo "  - Python     : $(python3 --version 2>/dev/null || echo 'N/A')"
echo "  - Go         : $(/usr/local/go/bin/go version 2>/dev/null | awk '{print $3}' || echo 'N/A')"
echo "  - Rust       : $(rustc --version 2>/dev/null | awk '{print $2}' || echo 'N/A')"
echo "  - Java       : $(java --version 2>&1 | head -1 || echo 'N/A')"
echo "  - Ruby       : $(ruby --version 2>/dev/null | awk '{print $2}' || echo 'N/A')"
echo "  - PHP        : $(php --version 2>/dev/null | head -1 | awk '{print $2}' || echo 'N/A')"
echo "  - .NET       : $(dotnet --version 2>/dev/null || echo 'N/A')"
echo "  - Gradle     : $(gradle --version 2>/dev/null | grep Gradle | awk '{print $2}' || echo 'N/A')"
echo "  - sbt        : $(sbt --version 2>/dev/null | grep 'sbt script' | awk '{print $4}' || echo 'N/A')"
echo "  - Docker     : $(docker --version 2>/dev/null | awk '{print $3}' | tr -d ',' || echo 'N/A')"
echo "  - Terraform  : $(terraform --version 2>/dev/null | head -1 | awk '{print $2}' || echo 'N/A')"
echo "  - AWS CLI    : $(aws --version 2>/dev/null | awk '{print $1}' | cut -d/ -f2 || echo 'N/A')"
echo "  - Azure CLI  : $(az --version 2>/dev/null | head -1 | awk '{print $2}' || echo 'N/A')"
echo "  - Claude Code: $(claude --version 2>/dev/null || echo 'N/A')"
echo ""
echo "Linting tools:"
echo "  - golangci-lint : $(golangci-lint --version 2>/dev/null | awk '{print $4}' || echo 'N/A')"
echo "  - ruff          : $(ruff --version 2>/dev/null | awk '{print $2}' || echo 'N/A')"
echo "  - eslint        : $(eslint --version 2>/dev/null || echo 'N/A')"
echo "  - prettier      : $(prettier --version 2>/dev/null || echo 'N/A')"
echo "  - rubocop       : $(rubocop --version 2>/dev/null || echo 'N/A')"
echo "  - actionlint    : $(actionlint --version 2>/dev/null || echo 'N/A')"
echo "  - hadolint      : $(hadolint --version 2>/dev/null || echo 'N/A')"
echo "  - shellcheck    : $(shellcheck --version 2>/dev/null | grep version: | awk '{print $2}' || echo 'N/A')"
echo "  - clang-format  : $(clang-format --version 2>/dev/null | awk '{print $3}' || echo 'N/A')"
echo ""
echo "Container tools:"
echo "  - crane         : $(crane version 2>/dev/null || echo 'N/A')"
echo "  - dive          : $(dive --version 2>/dev/null | head -1 || echo 'N/A')"
echo "  - trivy         : $(trivy --version 2>/dev/null | head -1 | awk '{print $2}' || echo 'N/A')"
echo "  - skopeo        : $(skopeo --version 2>/dev/null | awk '{print $3}' || echo 'N/A')"
echo ""
echo "Quick start:"
echo "  1. SSH in: gcloud compute ssh $(hostname) --zone=ZONE --tunnel-through-iap"
echo "  2. Switch user: sudo su - sandbox"
echo "  3. Start agents: start-agents.sh 12"
echo "  4. Attach: tmux attach -t claude-agents"

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
# - Homebrew: dev tools, language runtimes, CLI utilities (always current)
# - apt: system packages, Docker, cloud CLIs
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

TOTAL_STEPS=28
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

# ============================================
# Versions (for tools not managed by brew)
# ============================================
RUST_VERSION="${RUST_VERSION:-1.93}"
JAVA_VERSION="${JAVA_VERSION:-21}"
DOTNET_VERSION="${DOTNET_VERSION:-10.0}"
KUBECTL_VERSION="${KUBECTL_VERSION:-1.30}"

export DEBIAN_FRONTEND=noninteractive
export HOME=/root

# Wait for cloud-init and apt locks
cloud-init status --wait || true

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
        if ! grep -q "gce.ports.ubuntu.com" /etc/apt/sources.list 2>/dev/null; then
            sed -i "s|ports.ubuntu.com|${GCE_MIRROR}|g" /etc/apt/sources.list 2>/dev/null || true
        fi
    fi
fi

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
# Homebrew (package manager for dev tools)
# ============================================
report_progress "Installing Homebrew"
if [ ! -d /home/linuxbrew/.linuxbrew ]; then
    NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
fi

# Set up brew for this script
eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
export PATH="/home/linuxbrew/.linuxbrew/bin:$PATH"

# ============================================
# Dev Tools via Homebrew
# ============================================
report_progress "Installing dev tools via Homebrew"
brew install \
    go \
    node \
    ruby \
    python@3 \
    golangci-lint \
    actionlint \
    hadolint \
    shellcheck \
    git-delta \
    ripgrep \
    fd \
    fzf \
    yq \
    gh \
    k9s \
    helm \
    btop

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
eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
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
uv tool install semgrep

# ============================================
# Ruby gems
# ============================================
report_progress "Installing Ruby gems"
eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
gem install bundler rubocop

# ============================================
# Rust
# ============================================
report_progress "Installing Rust"
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain ${RUST_VERSION}
source "$HOME/.cargo/env"
rustup component add clippy rustfmt

# ============================================
# Java (system OpenJDK) + Maven
# ============================================
report_progress "Installing Java"
apt-get install -y openjdk-${JAVA_VERSION}-jdk maven

# ============================================
# PHP (system packages)
# ============================================
report_progress "Installing PHP"
apt-get install -y php php-cli php-common php-curl php-mbstring php-xml php-zip
curl -sS https://getcomposer.org/installer | php -- --install-dir=/usr/local/bin --filename=composer

# ============================================
# .NET
# ============================================
report_progress "Installing .NET"
wget "https://packages.microsoft.com/config/ubuntu/$(lsb_release -rs)/packages-microsoft-prod.deb" -O /tmp/packages-microsoft-prod.deb
dpkg -i /tmp/packages-microsoft-prod.deb
rm /tmp/packages-microsoft-prod.deb
apt-get update
apt-get install -y dotnet-sdk-${DOTNET_VERSION} || apt-get install -y dotnet-sdk-8.0

# ============================================
# Cloud CLIs
# ============================================
report_progress "Installing Google Cloud CLI"
echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee /etc/apt/sources.list.d/google-cloud-sdk.list
curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --batch --yes --dearmor -o /usr/share/keyrings/cloud.google.gpg
apt-get update
apt-get install -y google-cloud-cli google-cloud-cli-gke-gcloud-auth-plugin

report_progress "Installing AWS CLI"
AWS_ARCH="x86_64"
if [ "$(uname -m)" = "aarch64" ]; then
    AWS_ARCH="aarch64"
fi
curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-${AWS_ARCH}.zip" -o /tmp/awscliv2.zip
unzip -qo /tmp/awscliv2.zip -d /tmp
if [ -d /usr/local/aws-cli ]; then
    /tmp/aws/install --update
else
    /tmp/aws/install
fi
rm -rf /tmp/aws /tmp/awscliv2.zip

report_progress "Installing Azure CLI"
curl -sL https://aka.ms/InstallAzureCLIDeb | bash

# ============================================
# Kubernetes tools
# ============================================
report_progress "Installing Kubernetes tools"
curl -fsSL https://pkgs.k8s.io/core:/stable:/v${KUBECTL_VERSION}/deb/Release.key | gpg --batch --yes --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v${KUBECTL_VERSION}/deb/ /" | tee /etc/apt/sources.list.d/kubernetes.list
apt-get update
apt-get install -y kubectl

# ============================================
# Terraform
# ============================================
report_progress "Installing Terraform"
wget -O- https://apt.releases.hashicorp.com/gpg | gpg --batch --yes --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/hashicorp.list
apt-get update
apt-get install -y terraform

# ============================================
# Container tools
# ============================================
report_progress "Installing container tools"
apt-get install -y skopeo

# Trivy
wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | gpg --batch --yes --dearmor -o /usr/share/keyrings/trivy.gpg
echo "deb [signed-by=/usr/share/keyrings/trivy.gpg] https://aquasecurity.github.io/trivy-repo/deb generic main" | tee /etc/apt/sources.list.d/trivy.list
apt-get update
apt-get install -y trivy

# ============================================
# Database clients
# ============================================
report_progress "Installing database clients"
apt-get install -y postgresql-client default-mysql-client redis-tools

# ============================================
# ZSH + Oh My Zsh
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

# Homebrew
eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"

# Rust
export PATH=$PATH:$HOME/.cargo/bin

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
set -g mouse on
set -g history-limit 50000
set -g base-index 1
setw -g pane-base-index 1
set -g default-terminal "screen-256color"
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
echo "  - Docker     : $(docker --version 2>/dev/null | awk '{print $3}' | tr -d ',' || echo 'N/A')"
echo "  - Claude Code: $(claude --version 2>/dev/null || echo 'N/A')"
echo ""
echo "Quick start:"
echo "  1. SSH: gcloud compute ssh $(hostname) --zone=ZONE --tunnel-through-iap"
echo "  2. Switch user: sudo su - sandbox"
echo "  3. Start agents: start-agents.sh 12"

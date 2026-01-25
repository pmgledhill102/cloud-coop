# Complete Tooling Reference

This document lists all tools pre-installed on the Claude Sandbox VM by the provisioning script.

## Sources

The tooling list is based on:
1. **Anthropic's official devcontainer** - [github.com/anthropics/claude-code/.devcontainer](https://github.com/anthropics/claude-code/tree/main/.devcontainer)
2. **discord-bot-test-suite CI** - Pre-commit hooks, GitHub workflows, and Dockerfiles
3. **Docker's Claude Code sandbox template** - [docker/sandbox-templates:claude-code](https://docs.docker.com/ai/sandboxes/claude-code/)

---

## Language Runtimes

Versions match the discord-bot-test-suite Dockerfiles and CI:

| Runtime | Version | Package Managers | Linters/Formatters |
|---------|---------|------------------|-------------------|
| **Node.js** | 24 | npm, yarn, pnpm | eslint, prettier, cspell, markdownlint-cli2 |
| **Python** | 3.11 | pip, pipx, poetry, uv | black, ruff, mypy, yamllint |
| **Go** | 1.25 | go modules | golangci-lint v2.8.0 |
| **Rust** | 1.93 | cargo, rustup | clippy, rustfmt |
| **Java** | 21 (Temurin) | Maven, Gradle 8.12 | Spotless (via Maven) |
| **Kotlin** | (JVM 21) | Gradle 8.12 | ktlint (via Gradle) |
| **Scala** | 3.3 | sbt 1.10.6 | scalafmt (via sbt) |
| **Ruby** | 3.4 | bundler, gem | rubocop |
| **PHP** | 8.5 | composer | Laravel Pint (via Composer) |
| **.NET** | 10.0 | dotnet CLI | dotnet format |
| **C++** | (system) | cmake | clang-format |

---

## Pre-commit Hook Tools

These tools are used by `.pre-commit-config.yaml`:

| Tool | Version | Language | Purpose |
|------|---------|----------|---------|
| pre-commit-hooks | v4.5.0 | Python | General file checks |
| prettier | v3.1.0 | Node.js | JSON/YAML/Markdown formatting |
| yamllint | v1.35.1 | Python | YAML linting |
| actionlint | v1.7.7 | Go | GitHub Actions linting |
| cspell | latest | Node.js | Spelling check for Markdown |
| markdownlint-cli2 | latest | Node.js | Markdown linting |
| golangci-lint | v2.8.0 | Go | Go linting |
| ruff | v0.9.6 | Python | Python linting/formatting |
| rubocop | latest | Ruby | Ruby linting |
| eslint | latest | Node.js | JavaScript/TypeScript linting |
| clang-format | system | C++ | C++ formatting |
| rustfmt | bundled | Rust | Rust formatting |
| clippy | bundled | Rust | Rust linting |
| shellcheck | v0.9.0 | Haskell | Shell script linting |
| hadolint | v2.12.0 | Haskell | Dockerfile linting |

---

## Core Tools (from Anthropic Devcontainer)

| Tool | Version | Purpose |
|------|---------|---------|
| Node.js | 24 | Claude Code runtime |
| npm | bundled | Node package manager |
| git | latest | Version control |
| gh | latest | GitHub CLI |
| jq | latest | JSON processor |
| fzf | latest | Fuzzy finder |
| ripgrep (rg) | latest | Fast code search |
| git-delta | 0.18.2 | Syntax-highlighted git diffs |
| zsh | latest | Shell |
| oh-my-zsh | latest | ZSH framework |

---

## Node.js Global Packages

```
@anthropic-ai/claude-code
yarn
pnpm
typescript
ts-node
eslint
prettier
cspell
markdownlint-cli2
```

---

## Python Packages (pip)

```
pipx
poetry
uv
black
ruff
mypy
pytest
pre-commit
httpie
yamllint
semgrep
```

---

## Go Tools

```
golangci-lint (v2.8.0)
delve (dlv)
grpcurl
```

---

## Container & Cloud Tools

| Tool | Purpose |
|------|---------|
| docker | Container runtime |
| docker-compose | Multi-container orchestration |
| docker-buildx | Extended build capabilities |
| gcloud | Google Cloud CLI |
| gsutil | Cloud Storage CLI |
| bq | BigQuery CLI |
| kubectl | Kubernetes CLI |
| helm | Kubernetes package manager |
| k9s | Kubernetes TUI |
| terraform | Infrastructure as code |

---

## Database Clients

| Client | Database |
|--------|----------|
| psql | PostgreSQL |
| mysql | MySQL/MariaDB |
| redis-cli | Redis |
| mongosh | MongoDB |

---

## Configuration File

All versions are defined in `config/versions.env`:

```bash
# Language Runtimes (match Dockerfiles)
NODE_VERSION="24"
PYTHON_VERSION="3.11"
GO_VERSION="1.25.3"
RUST_VERSION="1.93"
JAVA_VERSION="21"
RUBY_VERSION="3.4"
PHP_VERSION="8.5"
DOTNET_VERSION="10.0"

# Build Tools
GRADLE_VERSION="8.12"
SBT_VERSION="1.10.6"

# Linting Tools (match pre-commit)
GOLANGCI_LINT_VERSION="v2.8.0"
DELTA_VERSION="0.18.2"
ACTIONLINT_VERSION="1.7.7"
HADOLINT_VERSION="2.12.0"
```

To update versions:
1. Edit `config/versions.env`
2. Re-run `provision-vm.sh` or rebuild the VM

---

## Verification Commands

After provisioning, verify installations:

```bash
# Languages
node --version          # v24.x
python3 --version       # 3.11.x
go version              # go1.25.x
rustc --version         # 1.93.x
java --version          # 21.x
ruby --version          # 3.4.x
php --version           # 8.5.x
dotnet --version        # 10.0.x

# Build tools
gradle --version        # 8.12
sbt --version          # 1.10.6

# Linting
golangci-lint --version # 2.8.0
ruff --version          # 0.9.x
actionlint --version    # 1.7.7
hadolint --version      # 2.12.0

# Claude Code
claude --version
```

---

## Adding Custom Tools

To add tools for your specific projects:

1. **Modify `config/versions.env`** - Add new version variables
2. **Modify `scripts/provision-vm.sh`** - Add installation steps
3. **Use Packer** - Bake a custom VM image with additional tools
4. **Install at runtime** - Via the sandbox user:

```bash
sudo su - sandbox
npm install -g some-tool
pip install some-package
go install github.com/some/tool@latest
```

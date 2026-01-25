# Repository Maturity Patterns - Extraction Guide

This document captures all the mature patterns from `discord-bot-test-suite` for replication in new repositories.

---

## 1. Git Configuration

### `.gitignore`

```gitignore
# Secrets & Environment
.env*
*.pem
*.key
credentials.json
secrets.yaml

# IDE
.idea/
.vscode/

# OS
.DS_Store
Thumbs.db

# Logs & temp
*.log
*.tmp

# Build outputs (customize per language)
node_modules/
__pycache__/
target/
bin/
obj/
```

### `.gitattributes`

```gitattributes
* text=auto eol=lf
*.png binary
*.jpg binary
*.gif binary
*.ico binary
*.woff binary
*.woff2 binary
*.ttf binary
*.eot binary
*.zip binary
*.tar.gz binary

# Linguist overrides
docs/* linguist-documentation
```

---

## 2. Editor Configuration

### `.editorconfig`

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 2

[*.go]
indent_style = tab
indent_size = 4

[*.{py,java,cs,cpp,rs,php}]
indent_size = 4

[Makefile]
indent_style = tab
```

---

## 3. Pre-commit Configuration

### `.pre-commit-config.yaml` (Key Structure)

```yaml
default_install_hook_types: [pre-commit, commit-msg]

repos:
  # === ALWAYS RUN (fast, safe) ===
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
      - id: check-json
      - id: check-added-large-files
        args: ['--maxkb=1000']
      - id: check-merge-conflict
      - id: detect-private-key
      - id: no-commit-to-branch
        args: ['--branch', 'main']

  # Prettier (JSON/YAML/Markdown)
  - repo: https://github.com/pre-commit/mirrors-prettier
    rev: v3.1.0
    hooks:
      - id: prettier
        types_or: [json, yaml, markdown]

  # YAML lint
  - repo: https://github.com/adrienverge/yamllint
    rev: v1.35.1
    hooks:
      - id: yamllint
        args: ['-c', '.yamllint.yaml']

  # GitHub Actions lint
  - repo: https://github.com/rhysd/actionlint
    rev: v1.7.7
    hooks:
      - id: actionlint

  # Shell scripts
  - repo: https://github.com/shellcheck-py/shellcheck-py
    rev: v0.9.0.6
    hooks:
      - id: shellcheck

  # Dockerfiles
  - repo: https://github.com/hadolint/hadolint
    rev: v2.12.0
    hooks:
      - id: hadolint

  # === AGENT-ONLY (slower, skip with HUMAN_HERE=1) ===
  - repo: local
    hooks:
      - id: golangci-lint
        name: golangci-lint
        entry: bash -c 'if [ -z "$HUMAN_HERE" ]; then golangci-lint run; fi'
        language: system
        pass_filenames: false
        # ... repeat pattern for other slow linters
```

**Key Pattern**: Use `HUMAN_HERE=1` env var to skip slow linters for human developers while agents run full checks.

---

## 4. Linter Configurations

### `.prettierrc`

```json
{
  "semi": true,
  "singleQuote": true,
  "tabWidth": 2,
  "printWidth": 100,
  "trailingComma": "es5",
  "endOfLine": "lf",
  "proseWrap": "preserve"
}
```

### `.prettierignore`

```gitignore
node_modules/
build/
dist/
.git/
*.lock
```

### `.yamllint.yaml`

```yaml
extends: default
rules:
  line-length:
    max: 160
  truthy:
    check-keys: false
  comments:
    min-spaces-from-content: 1
```

### `.markdownlint.json`

```json
{
  "line-length": { "line_length": 120, "tables": false, "code_blocks": false, "headings": false },
  "no-inline-html": { "allowed_elements": ["details", "summary", "br"] }
}
```

### `.hadolint.yaml`

```yaml
failure-threshold: error
ignored:
  - DL3008
```

### `.cspell.json`

```json
{
  "$schema": "https://raw.githubusercontent.com/streetsidesoftware/cspell/main/packages/cspell-types/cspell.schema.json",
  "version": "0.2",
  "ignorePaths": ["node_modules", ".git", "vendor", "build", "dist"],
  "dictionaries": ["en_US", "en-gb", "typescript", "node", "go", "rust"],
  "words": ["your", "custom", "terms"]
}
```

---

## 5. GitHub Configuration

### `.github/CODEOWNERS`

```text
* @your-username

/docs/ @your-username
*.md @your-username
/.github/ @your-username
/tests/ @your-username
```

### `.github/pull_request_template.md`

```markdown
## Summary

<!-- 1-3 sentences describing the change -->

## Changes

-

## Test Plan

- [ ] Tests pass locally
- [ ] Linters pass
- [ ] Manual testing completed

## Related Issues

<!-- Link to issues: closes #123 -->

## Checklist

- [ ] Code follows project standards
- [ ] No sensitive data committed
- [ ] Documentation updated (if needed)
- [ ] CI checks pass
```

### `.github/actionlint.yaml`

```yaml
self-hosted-runner:
  labels: []
config-variables:
  - GCP_PROJECT_ID
  - GCP_REGION
```

---

## 6. GitHub Actions Workflows

### Key Patterns

**Reusable Composite Actions** (`.github/actions/`)

- `docker-build/action.yml` - Standardized Docker builds with GHA cache
- `run-tests/action.yml` - Standardized test execution
- `deploy/action.yml` - Standardized deployment

**Gatekeeper Jobs** for branch protection:

```yaml
lint-complete:
  if: always()
  needs: [lint-yaml, lint-markdown, lint-actions]
  runs-on: ubuntu-latest
  steps:
    - run: |
        if [[ "${{ contains(needs.*.result, 'failure') }}" == "true" ]]; then
          exit 1
        fi
```

**Concurrency Control**:

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true # For PR workflows
  # cancel-in-progress: false  # For deployment workflows
```

**Path Filtering**:

```yaml
on:
  push:
    paths:
      - 'services/my-service/**'
      - '.github/workflows/my-service.yml'
```

**Repository Check** (prevent forks from deploying):

```yaml
if: github.repository == 'owner/repo' && github.ref == 'refs/heads/main'
```

---

## 7. Dependabot Configuration

### `.github/dependabot.yml`

```yaml
version: 2
updates:
  - package-ecosystem: 'npm'
    directory: '/'
    schedule:
      interval: 'weekly'
      day: 'monday'
    commit-message:
      prefix: 'deps(npm)'
    groups:
      npm-deps:
        patterns:
          - '*'

  - package-ecosystem: 'github-actions'
    directory: '/'
    schedule:
      interval: 'weekly'
    commit-message:
      prefix: 'ci'
```

### Auto-merge Workflow (`.github/workflows/dependabot-auto-merge.yml`)

```yaml
name: Dependabot Auto-merge
on: pull_request

permissions:
  contents: write
  pull-requests: write

jobs:
  auto-merge:
    if: github.actor == 'dependabot[bot]'
    runs-on: ubuntu-latest
    steps:
      - uses: dependabot/fetch-metadata@v2
        id: metadata
      - if: steps.metadata.outputs.update-type != 'version-update:semver-major'
        run: gh pr merge --auto --squash "$PR_URL"
        env:
          PR_URL: ${{ github.event.pull_request.html_url }}
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## 8. AI Assistant Configuration

### `CLAUDE.md`

```markdown
# CLAUDE.md

## Project Overview

Brief description of what the project does.

## Architecture

Key architectural decisions and patterns.

## Commands

\`\`\`bash

# Build

npm run build

# Test

npm test

# Lint

npm run lint
\`\`\`

## Key Constraints

- Security requirements
- Performance requirements
- Coding standards
```

### `AGENTS.md`

Guidelines for multi-agent workflows if using agent swarms.

---

## 9. Scripts Directory

### Essential Scripts Pattern

```text
scripts/
├── setup.sh          # Local dev environment setup
├── test.sh           # Run tests with proper env
├── lint.sh           # Run all linters
└── build.sh          # Build artifacts
```

**Script Template**:

```bash
#!/bin/bash
set -euo pipefail

# Description and usage at top
# Cleanup traps for resources
# Health checks before operations
# Clear error messages
```

---

## 10. Documentation Structure

```text
docs/
├── architecture/     # System design docs
├── guides/          # How-to guides
├── CONTRIBUTING.md  # Contribution guidelines
├── LINTING.md       # Linting setup details
└── TESTING.md       # Testing strategy
```

---

## Files to Copy (Checklist)

### Root Level

- [ ] `.gitignore`
- [ ] `.gitattributes`
- [ ] `.editorconfig`
- [ ] `.pre-commit-config.yaml`
- [ ] `.prettierrc`
- [ ] `.prettierignore`
- [ ] `.yamllint.yaml` (if using yamllint)
- [ ] `.markdownlint.json`
- [ ] `.hadolint.yaml` (if using Docker)
- [ ] `.cspell.json`
- [ ] `CLAUDE.md`
- [ ] `README.md`
- [ ] `CONTRIBUTING.md`

### GitHub Directory

- [ ] `.github/CODEOWNERS`
- [ ] `.github/pull_request_template.md`
- [ ] `.github/actionlint.yaml`
- [ ] `.github/dependabot.yml`
- [ ] `.github/workflows/lint.yml`
- [ ] `.github/workflows/dependabot-auto-merge.yml`
- [ ] `.github/actions/` (composite actions)

### Scripts

- [ ] `scripts/` directory with utility scripts

---

## Summary Statistics (Original Repo)

| Category                 | Count |
| ------------------------ | ----- |
| GitHub Actions workflows | 26    |
| Pre-commit hooks         | 35+   |
| Linter configurations    | 15+   |
| Composite actions        | 3     |
| Documentation files      | 10+   |
| Utility scripts          | 3     |

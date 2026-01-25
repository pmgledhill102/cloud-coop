# Beads Restructure Plan

## Problem
1. **Prefix**: Issues use `cloud-coop-*` but config specifies `cc`
2. **Hierarchy**: Using flat dependencies instead of parent-child structure

## Target Structure

```
cc-1 [EPIC] Repository Configuration & CI/CD Setup
├── cc-1.1 [feature] Git Configuration
│   ├── cc-1.1.1 [task] Create .gitignore for Go project
│   └── cc-1.1.2 [task] Create .gitattributes with Go and binary settings
├── cc-1.2 [feature] Editor Configuration
│   └── cc-1.2.1 [task] Create .editorconfig
├── cc-1.3 [feature] Pre-commit Hooks Setup
│   ├── cc-1.3.1 [task] Create linter configuration files
│   └── cc-1.3.2 [task] Create .pre-commit-config.yaml
├── cc-1.4 [feature] Go Tooling Configuration
│   ├── cc-1.4.1 [task] Initialize go.mod
│   ├── cc-1.4.2 [task] Create Makefile with standard targets
│   └── cc-1.4.3 [task] Create .golangci.yml configuration
├── cc-1.5 [feature] GitHub Repository Configuration
│   ├── cc-1.5.1 [task] Create .github/dependabot.yml
│   ├── cc-1.5.2 [task] Create .github/pull_request_template.md
│   ├── cc-1.5.3 [task] Create Dependabot auto-merge workflow
│   ├── cc-1.5.4 [task] Create GitHub issue templates
│   └── cc-1.5.5 [task] Create .github/CODEOWNERS
├── cc-1.6 [feature] CI/CD Workflows
│   ├── cc-1.6.1 [task] Create CI lint workflow
│   ├── cc-1.6.2 [task] Create CI build workflow
│   └── cc-1.6.3 [task] Create CI test workflow
├── cc-1.7 [feature] Testing Infrastructure
│   ├── cc-1.7.1 [task] Create test directory structure
│   ├── cc-1.7.2 [task] Configure test coverage reporting
│   └── cc-1.7.3 [task] Create test helper utilities
├── cc-1.8 [feature] Release Automation
│   ├── cc-1.8.1 [task] Create .goreleaser.yml configuration
│   ├── cc-1.8.2 [task] Create release workflow
│   └── cc-1.8.3 [task] Create Homebrew formula template
└── cc-1.9 [feature] Documentation Structure
    ├── cc-1.9.1 [task] Create CLAUDE.md
    ├── cc-1.9.2 [task] Create scripts/setup.sh
    ├── cc-1.9.3 [task] Create CONTRIBUTING.md
    └── cc-1.9.4 [task] Document VM provisioning prerequisites

cc-2 [EPIC] Tracer Bullet Implementation (MVP)
├── cc-2.1 [feature] Iteration 1: CLI/TUI Skeleton
│   ├── cc-2.1.1 [task] Establish logging patterns (zerolog/slog)
│   └── cc-2.1.2 [task] Establish error handling patterns
├── cc-2.2 [task] Gate 1: Foundation Review
├── cc-2.3 [feature] Iteration 2: GCP Authentication & Read
├── cc-2.4 [task] Gate 2: Cloud Integration Review
├── cc-2.5 [feature] Iteration 3: VM Lifecycle Control
├── cc-2.6 [task] Gate 3: Lifecycle Control Review
├── cc-2.7 [feature] Iteration 4: SSH Connectivity
├── cc-2.8 [task] Gate 4: SSH Integration Review
├── cc-2.9 [feature] Iteration 5: Agent Sessions (Read)
├── cc-2.10 [task] Gate 5: Agent Read Review
├── cc-2.11 [feature] Iteration 6: Agent Sessions (Write)
├── cc-2.12 [task] Gate 6: Agent Write Review
├── cc-2.13 [feature] Iteration 7: Interactive Connect
├── cc-2.14 [task] Gate 7: Interactive Connect Review
├── cc-2.15 [feature] Iteration 8: Configuration & Polish
├── cc-2.16 [task] Gate 8: Polish Review
└── cc-2.17 [task] Final Review: MVP Readiness

cc-3 [EPIC] Post-MVP Enhancements
└── cc-3.1 [feature] Terminal config generator (Ghostty/iTerm2/Kitty)
```

## Execution Steps

### Step 1: Delete All Existing Issues
```bash
# Get all issue IDs and delete them
bd list --status=all --limit=0 | grep -oE 'cloud-coop-[a-z0-9]+' | xargs -I {} bd delete {} --force
```

### Step 2: Create Epic 1 - Repository Configuration
```bash
# Create epic
bd create "Repository Configuration & CI/CD Setup" --type=epic --priority=1

# Note the ID (e.g., cc-abc), then create children with --parent
bd create "Git Configuration" --type=feature --priority=2 --parent=cc-XXX
bd create "Create .gitignore for Go project" --type=task --priority=1 --parent=cc-XXX.1
bd create "Create .gitattributes with Go and binary settings" --type=task --priority=1 --parent=cc-XXX.1
# ... continue for all children
```

### Step 3: Create Epic 2 - Tracer Bullet (MVP)
```bash
bd create "Tracer Bullet Implementation (MVP)" --type=epic --priority=1
# Create iterations and gates as children
```

### Step 4: Create Epic 3 - Post-MVP
```bash
bd create "Post-MVP Enhancements" --type=epic --priority=4
# Create future features as children
```

### Step 5: Add Sequential Dependencies
After parent-child structure is in place, add execution-order dependencies:
```bash
# Iteration 1 must complete before Gate 1
bd dep add cc-2.2 cc-2.1

# Gate 1 must complete before Iteration 2
bd dep add cc-2.3 cc-2.2

# etc.
```

### Step 6: Sync
```bash
bd sync --flush-only
git add .beads/
git commit -m "Restructure beads with parent-child hierarchy"
```

## Notes

- Parent-child relationships are structural (what belongs to what)
- Dependencies are execution-order (what must complete before what)
- Both can coexist - a task can be a child of a feature AND depend on another task
- The web UI should show proper epic → feature → task nesting with parent-child

## Resume Command
```bash
# To resume this work:
claude "Continue the beads restructure from BEADS-RESTRUCTURE-PLAN.md"
```

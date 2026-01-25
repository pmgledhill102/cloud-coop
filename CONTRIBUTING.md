# Contributing to cloudcoop

Thank you for your interest in contributing to cloudcoop! This document provides
guidelines and instructions for contributing to the project.

## Table of Contents

- [Development Setup](#development-setup)
- [Code Style Guidelines](#code-style-guidelines)
- [Testing Requirements](#testing-requirements)
- [Running Linters](#running-linters)
- [Commit Message Format](#commit-message-format)
- [Pull Request Process](#pull-request-process)

## Development Setup

### Prerequisites

- **Go 1.25+**: Install from [go.dev](https://go.dev/dl/)
- **golangci-lint**: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- **goimports**: `go install golang.org/x/tools/cmd/goimports@latest`
- **pre-commit** (optional): `pip install pre-commit`

### Clone and Build

```bash
# Clone the repository
git clone https://github.com/cloud-coop/cloudcoop.git
cd cloudcoop

# Download dependencies
go mod download

# Build the binary
make build

# Run tests
make test
```

### Install Pre-commit Hooks (Recommended)

Pre-commit hooks catch issues before they reach CI:

```bash
# Install pre-commit
pip install pre-commit

# Install hooks
pre-commit install

# Run manually on all files
pre-commit run --all-files
```

### IDE Setup

The project includes an `.editorconfig` file. Ensure your editor respects it:

- **Go files**: Tabs for indentation
- **YAML/JSON/Markdown**: 2 spaces for indentation
- **All files**: UTF-8 encoding, LF line endings, trailing newline

For VS Code, the `.vscode/` directory contains recommended settings.

## Code Style Guidelines

### Go Conventions

We follow standard Go conventions with some project-specific additions:

1. **Formatting**: Use `gofmt` and `goimports`

   ```bash
   make fmt
   ```

2. **Naming**:
   - Use `mixedCase` for unexported identifiers
   - Use `MixedCase` for exported identifiers
   - Avoid stuttering (e.g., `config.Config` is fine, `config.ConfigConfig` is not)
   - Acronyms should be all caps: `HTTPServer`, `userID`

3. **Error Handling**:
   - Always check errors (enforced by `errcheck` linter)
   - Use `errors.Is()` and `errors.As()` for error comparison
   - Wrap errors with context: `fmt.Errorf("failed to create VM: %w", err)`

4. **Comments**:
   - Exported functions require doc comments
   - Comments should be complete sentences
   - Start with the function/type name: `// CreateVM provisions a new virtual machine.`

5. **Imports**:
   - Group imports: standard library, third-party, local packages
   - Use `goimports` to manage ordering automatically

### Project Structure

```text
cloudcoop/
├── cmd/cloudcoop/     # Main entry point
├── internal/          # Private application code
│   ├── cloud/         # Cloud provider implementations
│   ├── config/        # Configuration handling
│   ├── tui/           # Terminal UI components
│   └── vm/            # VM management logic
├── pkg/               # Public libraries (if any)
├── scripts/           # Development and operational scripts
├── docs/              # Documentation
└── decisions/         # Architecture Decision Records
```

### Linter Configuration

The project uses golangci-lint with the following enabled linters:

| Linter         | Purpose                    |
| -------------- | -------------------------- |
| `errcheck`     | Detect unchecked errors    |
| `gosimple`     | Simplification suggestions |
| `govet`        | Suspicious constructs      |
| `ineffassign`  | Ineffectual assignments    |
| `staticcheck`  | Advanced static analysis   |
| `unused`       | Unused code detection      |
| `misspell`     | Spelling mistakes          |
| `revive`       | Go style enforcement       |

See `.golangci.yml` for the complete configuration.

## Testing Requirements

### Test Levels

1. **Unit Tests** (required for all PRs)
   - Fast, no external dependencies
   - Mock cloud SDK clients
   - Run with `make test` or `go test ./...`

2. **Integration Tests** (for cloud SDK changes)
   - Real API calls against dev project
   - Use build tag: `//go:build integration`
   - Run with `go test -tags=integration ./...`

3. **E2E Tests** (for major features)
   - Full workflow testing
   - Use build tag: `//go:build e2e`
   - Run with `go test -tags=e2e -timeout=15m ./e2e/...`

### Writing Tests

```go
// Good: Table-driven tests
func TestParseConfig(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *Config
        wantErr bool
    }{
        {
            name:  "valid config",
            input: `cloud: gcp`,
            want:  &Config{Cloud: "gcp"},
        },
        {
            name:    "empty input",
            input:   "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseConfig(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("ParseConfig() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Coverage

Generate coverage reports:

```bash
make test-coverage
# Open coverage/coverage.html in browser
```

## Running Linters

### Quick Check

```bash
# Run all linters
make lint

# Or directly
golangci-lint run ./...
```

### Pre-commit Hooks

If pre-commit is installed, hooks run automatically on `git commit`.

To skip hooks temporarily (use sparingly):

```bash
# Skip specific hook
SKIP=golangci-lint git commit -m "message"

# Skip all hooks (requires human approval)
git commit --no-verify -m "message"
```

### Available Linters

| Tool          | Command                          | Purpose                   |
| ------------- | -------------------------------- | ------------------------- |
| golangci-lint | `make lint`                      | Go code analysis          |
| yamllint      | `yamllint .`                     | YAML validation           |
| shellcheck    | `shellcheck scripts/*.sh`        | Shell script analysis     |
| markdownlint  | `markdownlint-cli2 "**/*.md"`    | Markdown formatting       |
| actionlint    | `actionlint`                     | GitHub Actions validation |

## Commit Message Format

### Structure

```text
<type>: <description>

[optional body]

[optional footer]
```

### Types

| Type       | Description                                              |
| ---------- | -------------------------------------------------------- |
| `feat`     | New feature                                              |
| `fix`      | Bug fix                                                  |
| `docs`     | Documentation only                                       |
| `style`    | Formatting, no code change                               |
| `refactor` | Code change that neither fixes a bug nor adds a feature  |
| `test`     | Adding or updating tests                                 |
| `chore`    | Maintenance tasks                                        |

### Examples

```bash
# Good
feat: add VM resize command
fix: handle SSH connection timeout gracefully
docs: update development setup instructions
refactor: extract cloud provider interface

# Bad
fixed bug          # No type, vague description
WIP               # Not descriptive
updates           # Not descriptive
```

### Guidelines

- Use imperative mood: "add feature" not "added feature"
- Keep first line under 72 characters
- Reference issues in footer: `Fixes #123` or `Closes cc-1.5`
- Separate subject from body with blank line

## Pull Request Process

### Before Submitting

1. **Create a branch**

   ```bash
   git checkout -b feat/my-feature
   ```

2. **Make changes and test**

   ```bash
   make fmt         # Format code
   make lint        # Run linters
   make test        # Run tests
   make build       # Verify build
   ```

3. **Commit with meaningful messages**

   ```bash
   git commit -m "feat: add VM status monitoring"
   ```

### Submitting

1. **Push your branch**

   ```bash
   git push -u origin feat/my-feature
   ```

2. **Create Pull Request**
   - Use the PR template
   - Provide clear description of changes
   - Link related issues
   - Add test plan

### PR Checklist

Before requesting review, ensure:

- [ ] Code follows project style guidelines
- [ ] All tests pass (`make test`)
- [ ] Linters pass (`make lint`)
- [ ] Documentation updated if needed
- [ ] No sensitive data (secrets, credentials) included
- [ ] Commit messages follow format
- [ ] PR description is complete

### Review Process

1. **CI checks** must pass
2. **Code review** by maintainer
3. **Address feedback** with new commits (avoid force-push during review)
4. **Squash and merge** after approval

### After Merge

- Delete your feature branch
- Pull latest main: `git checkout main && git pull`

## Getting Help

- Check existing [documentation](docs/)
- Review [Architecture Decision Records](decisions/)
- Open an issue for bugs or feature requests
- Use the issue tracker for questions

## License

By contributing, you agree that your contributions will be licensed under the
Apache 2.0 License.

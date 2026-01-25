# ADR-0014: Configuration File Format

## Status

Accepted

## Context

cloudcoop needs a configuration file format for storing user preferences, VM configurations, agent settings, and project defaults. The format must be:

- Human-readable and hand-editable
- Support comments for documentation
- Work well with Go's type system
- Be familiar to the target audience (developers using cloud infrastructure)

## Decision

Use **TOML** (Tom's Obvious Minimal Language) as the configuration file format.

Configuration files will be named `cloudcoop.toml` and located at:
- `~/.config/cloudcoop/cloudcoop.toml` (user defaults)
- `./cloudcoop.toml` (project-specific overrides)

## Options Considered

### Option 1: YAML

YAML is widely used in cloud/DevOps tooling (Kubernetes, Docker Compose, GitHub Actions).

**Pros:**
- Very familiar to cloud/DevOps users
- Human-readable with good support for complex structures
- Supports comments
- Multi-document support

**Cons:**
- Whitespace-sensitive syntax leads to subtle, hard-to-debug errors
- Complex specification with many edge cases
- "Norway problem" and other type coercion surprises (`yes` becomes boolean)
- Requires external dependency (gopkg.in/yaml.v3)

### Option 2: JSON

JSON is universal and has stdlib support in Go.

**Pros:**
- No external dependencies (encoding/json in stdlib)
- Universal format, excellent tooling
- Unambiguous parsing
- Good for machine-generated config

**Cons:**
- No comments - critical limitation for configuration files
- Verbose syntax (quotes on all keys, trailing comma restrictions)
- Poor experience for hand-editing
- No multi-line strings

### Option 3: TOML (Selected)

TOML is designed specifically for configuration files, used by Rust (Cargo.toml), Hugo, GoReleaser, and many Go tools.

**Pros:**
- Designed for configuration files - clear, unambiguous syntax
- Supports comments (essential for documenting config)
- No whitespace sensitivity
- Clean section-based structure fits VM/agent configs well
- Strong Go ecosystem support (github.com/BurntSushi/toml)
- Familiar to Go and Rust developers
- Maps cleanly to Go structs

**Cons:**
- Less familiar to some developers than YAML
- Deeply nested structures can become verbose
- Smaller ecosystem than YAML/JSON

## Example Configuration

```toml
# cloudcoop configuration

[defaults]
project = "my-gcp-project"
region = "europe-north1"
zone = "europe-north1-a"

[vm.dev]
name = "dev-agent"
machine_type = "c4a-highcpu-4"
spot = true
disk_size_gb = 50

[vm.prod]
name = "prod-agent"
machine_type = "c4a-highcpu-8"
spot = false
disk_size_gb = 100

[agent.default]
shell = "/bin/bash"
tmux_session = "agent"

[logging]
level = "info"
format = "text"
```

## Consequences

### Positive

- Clear, unambiguous configuration syntax
- Comments enable self-documenting config files
- Aligns with Go ecosystem conventions (Hugo, GoReleaser, etc.)
- Type-safe parsing with good error messages
- Section-based structure organizes VM and agent configs naturally

### Negative

- Adds external dependency (github.com/BurntSushi/toml or github.com/pelletier/go-toml)
- Some users may need to learn TOML syntax
- Complex nested structures require careful design

### Neutral

- Config file location follows XDG conventions (`~/.config/cloudcoop/`)
- Project-local config (`./cloudcoop.toml`) overrides user defaults

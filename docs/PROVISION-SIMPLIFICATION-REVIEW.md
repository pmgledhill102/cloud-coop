# Provisioning Script Simplification Review

Review of `scripts/provision-vm.sh` to identify opportunities for simplification
by using Ubuntu 25.04 (Plucky Puffin) system defaults instead of external repositories.

## Current State

The script installs 15+ version-pinned components, many from external PPAs or direct downloads. This creates:

1. **Complexity**: PPA availability checks, fallback logic, architecture detection
2. **Fragility**: External repos may not support new Ubuntu releases
3. **Maintenance burden**: Version updates require script changes

## Component Analysis

### Components That Could Use Ubuntu Defaults

| Component | Current | Ubuntu 25.04 Default | Recommendation |
|-----------|---------|---------------------|----------------|
| **Python** | 3.14 (deadsnakes) | 3.13 | **Use system** - 3.13 is sufficient, deadsnakes often unavailable |
| **PHP** | 8.5 (ondrej/php) | 8.4 | **Use system** - 8.4 is current stable, ondrej often unavailable |
| **Java** | 21 (Adoptium) | 21 (OpenJDK) | **Use system** - OpenJDK 21 is equivalent |
| **Go** | 1.25.3 (direct) | 1.24 | **Consider system** - one minor version behind |
| **Rust** | 1.93 (rustup) | 1.84 | **Keep rustup** - too far behind, rustup is standard |
| **Ruby** | 3.4 (rbenv) | 3.3 | **Consider system** - one minor version behind |
| **ShellCheck** | apt | apt | Already using system |
| **Docker** | docker.com | 27.5 | **Consider system** - Ubuntu's Docker 27.5 is current |

### Components That Need External Sources

| Component | Reason |
|-----------|--------|
| **Node.js 24** | Ubuntu ships older LTS; NodeSource needed for v24 |
| **Claude Code** | npm package, not in Ubuntu repos |
| **golangci-lint** | Not in Ubuntu repos |
| **actionlint** | Not in Ubuntu repos |
| **hadolint** | Not in Ubuntu repos |
| **Terraform** | HashiCorp repo required |
| **kubectl/Helm** | Kubernetes repos required |
| **AWS/Azure/GCloud CLI** | Vendor repos required |
| **git-delta** | Not in Ubuntu repos (or too old) |

### Components to Consider Removing

| Component | Rationale |
|-----------|-----------|
| **sbt** | Scala-specific, low usage likelihood |
| **Gradle** | Can use Maven for most Java projects |
| **MongoDB mongosh** | Often unavailable, limited use case |
| **dive** | Nice-to-have, not essential |
| **crane** | Nice-to-have, not essential |
| **Trivy** | Nice-to-have for security scanning |
| **Semgrep** | Nice-to-have for security scanning |

## Recommended Changes

### High Impact (Significant Simplification)

1. **Python**: Remove deadsnakes PPA, use system Python 3.13
   - Removes: 25 lines of PPA detection/fallback code
   - Risk: None - 3.13 is production-ready

2. **PHP**: Remove ondrej/php PPA, use system PHP 8.4
   - Removes: 20 lines of PPA detection/fallback code
   - Risk: None - 8.4 is current stable

3. **Java**: Remove Adoptium, use system OpenJDK
   - Removes: 20 lines of repo detection/fallback code
   - Risk: None - OpenJDK 21 is equivalent to Temurin

4. **Docker**: Consider using Ubuntu's Docker 27.5
   - Removes: Docker GPG/repo setup
   - Risk: Minor version differences, but Ubuntu tracks closely

### Medium Impact

1. **Go**: Consider using system Go 1.24
   - Current script downloads 1.25.3 directly
   - Tradeoff: Lose one minor version, gain simplicity
   - Decision: Keep external if Go 1.25 features needed

2. **Ruby**: Consider using system Ruby + gems
   - rbenv build takes significant time
   - Tradeoff: Lose version flexibility
   - Decision: System Ruby may be sufficient

### Low Impact (Keep As-Is)

- **Node.js**: Keep NodeSource - Ubuntu's version too old
- **Rust**: Keep rustup - standard installer, version control
- **Cloud CLIs**: Keep vendor repos - authoritative source
- **Kubernetes tools**: Keep k8s repos - need current versions
- **Linting tools**: Keep direct downloads - not in Ubuntu

## Proposed Simplified Script Structure

```text
EXTERNAL REPOS NEEDED:
- NodeSource (Node.js 24)
- HashiCorp (Terraform)
- Kubernetes (kubectl)
- Google/AWS/Azure (cloud CLIs)

DIRECT DOWNLOADS:
- golangci-lint, actionlint, hadolint (linting)
- git-delta, k9s, yq (CLI tools)
- Rust via rustup

USE UBUNTU DEFAULTS:
- Python 3.13
- PHP 8.4
- Java 21 (OpenJDK)
- Docker 27.5
- Ruby 3.3
- Go 1.24 (optional)
```

## Estimated Reduction

| Metric | Before | After | Reduction |
|--------|--------|-------|-----------|
| Lines of code | ~840 | ~650 | ~23% |
| External PPAs | 4 | 0 | 100% |
| Fallback logic blocks | 5 | 0 | 100% |
| Install time (est.) | ~8 min | ~5 min | ~37% |

## Decision Matrix

For each component, score based on:

- **Need for latest**: Does Claude Code or typical dev work need bleeding edge?
- **Stability**: How often does external source break on new Ubuntu?
- **Complexity cost**: How much code does external source add?

| Component | Need Latest | Stability | Complexity | Verdict |
|-----------|-------------|-----------|------------|---------|
| Python | Low | Poor | High | **Use system** |
| PHP | Low | Poor | High | **Use system** |
| Java | Low | Poor | High | **Use system** |
| Go | Medium | Good | Medium | Review |
| Rust | High | Good | Low | Keep rustup |
| Ruby | Low | Poor | High | Consider system |
| Node | High | Good | Low | Keep NodeSource |
| Docker | Low | Good | Medium | Consider system |

## Next Steps

1. Create branch `feat/provision-simplify`
2. Remove deadsnakes, ondrej/php, Adoptium PPAs
3. Test on fresh Ubuntu 25.04 VM
4. Measure install time improvement
5. Verify all tools functional with system versions

## References

- [Ubuntu 25.04 Release Notes](https://linuxiac.com/ubuntu-25-04-plucky-puffin-released/)
- [Ubuntu Python Availability](https://documentation.ubuntu.com/ubuntu-for-developers/reference/availability/python/)

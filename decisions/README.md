# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for cloudcoop.

## Index

| ID | Title | Status | Date |
|----|-------|--------|------|
| [ADR-0001](0001-agent-execution-model.md) | Agent Execution Model | Accepted | 2026-01-25 |
| [ADR-0002](0002-storage-strategy.md) | Storage Strategy | Accepted | 2026-01-25 |
| [ADR-0003](0003-instance-provisioning-model.md) | Instance Provisioning Model | Accepted | 2026-01-25 |
| [ADR-0004](0004-region-selection.md) | Region Selection | Accepted | 2026-01-25 |
| [ADR-0005](0005-machine-type.md) | Machine Type | Accepted | 2026-01-25 |
| [ADR-0006](0006-cloud-vs-local-execution.md) | Cloud vs Local Execution | Accepted | 2026-01-25 |
| [ADR-0007](0007-infrastructure-management-approach.md) | Infrastructure Management (gcloud vs Terraform) | Accepted | 2026-01-25 |
| [ADR-0008](0008-agent-agnostic-design.md) | Agent-Agnostic Design | Accepted | 2026-01-25 |
| [ADR-0009](0009-api-key-management.md) | API Key Management | Accepted | 2026-01-25 |
| [ADR-0010](0010-cloud-agnostic-design.md) | Cloud-Agnostic Design | Accepted | 2026-01-25 |
| [ADR-0011](0011-tui-implementation-approach.md) | TUI Implementation Approach | Accepted | 2026-01-25 |
| [ADR-0012](0012-dynamic-ip-firewall.md) | Dynamic IP-Based Firewall | Accepted | 2026-01-25 |
| [ADR-0013](0013-ssh-remote-execution.md) | SSH and Remote Execution | Accepted | 2026-01-25 |
| [ADR-0014](0014-configuration-file-format.md) | Configuration File Format | Accepted | 2026-01-25 |
| [ADR-0015](0015-ssh-testing-in-sandboxed-environments.md) | SSH Testing in Sandboxed Environments | Accepted | 2026-01-25 |
| [ADR-0016](0016-error-handling-pattern.md) | Error Handling Pattern | Accepted | 2026-01-25 |
| [ADR-0017](0017-logging-strategy.md) | Logging Strategy | Accepted | 2026-01-25 |
| [ADR-0018](0018-tui-state-machine.md) | TUI State Machine | Accepted | 2026-01-25 |
| [ADR-0019](0019-ssh-testing-infrastructure.md) | SSH Testing Infrastructure | Accepted | 2026-01-25 |
| [ADR-0020](0020-vm-service-account-requirement.md) | VM Service Account Requirement | Accepted | 2026-01-26 |

## About ADRs

Architecture Decision Records capture important architectural decisions made during the design and
implementation of a system. Each ADR describes:

- **Context**: The situation and forces at play
- **Decision**: What was decided
- **Options Considered**: Alternatives evaluated with pros/cons
- **Consequences**: The resulting impact

### Template

New ADRs should follow the [MADR template](0000-template.md).

### Statuses

- **Proposed**: Under discussion
- **Accepted**: Approved and in effect
- **Deprecated**: No longer valid
- **Superseded**: Replaced by another ADR

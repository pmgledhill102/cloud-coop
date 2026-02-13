# ADR-0029: VM Git Identity

## Status

Accepted

## Context

When agents commit code on the VM, git requires `user.name` and `user.email` to be configured.
The provisioning script creates a `.gitconfig` for the sandbox user but does not set identity
fields. Without them, `git commit` fails or falls back to a generated identity based on the
system hostname, which produces confusing commit metadata.

Users already have `user.name` and `user.email` configured on their local workstation. Asking
them to configure it again in `cloudcoop.toml` or on the VM is redundant and error-prone.

## Decision

During `cloudcoop agents sync`, read the local `git config user.name` and `git config user.email`
and set them on the VM via `git config --global` over SSH.

**Behaviour:**

1. Read `git config user.name` and `git config user.email` on the local machine.
2. If both are set, run `git config --global user.name "..."` and
   `git config --global user.email "..."` on the VM over SSH.
3. If either is missing locally, skip with a warning:
   `Warning: local git user.name/user.email not configured, skipping VM git identity setup`
4. This runs once per sync, before the clone step. It is idempotent — repeated syncs
   overwrite with the current local values.

**Why global config (not per-repo):**

The sandbox user runs all repos. Global config means every repo on the VM gets the correct
identity without per-worktree configuration. If a user needs per-repo overrides, they can
use `pre_commands` in `cloudcoop.toml` ([ADR-0027](0027-agent-startup-hooks.md)).

## Options Considered

### Option 1: Copy from Local Machine (Chosen)

Read `git config user.name` and `git config user.email` locally and set them on the VM
during `agents sync`.

**Pros:**

- Zero configuration — uses values already set on the workstation
- Always in sync with the user's local identity
- Idempotent — safe to run repeatedly
- No new config fields to document or maintain

**Cons:**

- Requires local git identity to be configured (warns if missing)
- Overwrites any manually set VM identity on each sync

### Option 2: Configuration in cloudcoop.toml

Add `[git]` section with `name` and `email` fields.

**Pros:**

- Explicit — user sees the config and controls it
- Can differ from local identity if desired

**Cons:**

- Redundant — duplicates what's already in local git config
- Easy to forget — users set up git identity once and expect it to work everywhere
- More config fields to maintain and validate

### Option 3: Set During Provisioning

Prompt for name/email during `cloudcoop setup` and bake into the provisioning script.

**Pros:**

- Set once at VM creation time

**Cons:**

- Provisioning is VM-scoped, not user-scoped — doesn't work for shared VMs
- Cannot change without re-provisioning
- `cloudcoop setup` runs infrequently; users would forget what they entered

### Option 4: Pre-Commands

Leave it to users via `agents.pre_commands`.

**Pros:**

- No code changes needed

**Cons:**

- Easy to forget — git commits fail with a confusing error
- Boilerplate that every user would need to add
- Git identity is a baseline expectation, not an advanced configuration

## Consequences

### Positive

- Git commits on the VM have correct author metadata from the first sync
- No additional configuration required — just works
- Keeps VM identity in sync with local workstation across syncs

### Negative

- Users who want a different identity on the VM must override via `pre_commands`
- Adds two SSH commands to each `agents sync` invocation (negligible overhead)

### Neutral

- Runs after SSH connection is established but before clone/worktree setup
- Complements [ADR-0026](0026-vm-git-authentication.md) (deploy keys for auth)
  and [ADR-0027](0027-agent-startup-hooks.md) (pre-commands for overrides)

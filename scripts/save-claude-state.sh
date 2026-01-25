#!/bin/bash
# save-claude-state.sh
# Called by systemd on shutdown/preemption to save Claude Code state
#
# GCP gives 30 seconds notice before spot preemption.
# This script must complete within ~25 seconds.

set -e

PERSIST_MOUNT="/mnt/persist"
STATE_DIR="$PERSIST_MOUNT/session-state"
SANDBOX_USER="${SANDBOX_USER:-sandbox}"
SANDBOX_HOME="/home/$SANDBOX_USER"
LOG_FILE="/var/log/claude-state-save.log"

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') $1" | tee -a "$LOG_FILE"
}

log "=== Starting Claude state save ==="

# Check if this is a preemption
IS_PREEMPTED=$(curl -s -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/preempted" 2>/dev/null || echo "unknown")
log "Preemption status: $IS_PREEMPTED"

# Create state directory with timestamp
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
SAVE_DIR="$STATE_DIR/$TIMESTAMP"
mkdir -p "$SAVE_DIR"

# 1. Save tmux session list
log "Saving tmux session info..."
if su - "$SANDBOX_USER" -c "tmux list-sessions" > "$SAVE_DIR/tmux-sessions.txt" 2>/dev/null; then
    log "  Saved tmux sessions list"
else
    log "  No tmux sessions running"
fi

# 2. Save which windows/agents were active
if su - "$SANDBOX_USER" -c "tmux list-windows -t claude-agents" > "$SAVE_DIR/tmux-windows.txt" 2>/dev/null; then
    log "  Saved tmux windows list"
fi

# 3. Capture current working directories for each agent
log "Capturing agent working directories..."
for i in $(seq 1 16); do
    PANE_PWD=$(su - "$SANDBOX_USER" -c "tmux display-message -p -t claude-agents:agent-$i '#{pane_current_path}'" 2>/dev/null || echo "")
    if [ -n "$PANE_PWD" ]; then
        echo "$i:$PANE_PWD" >> "$SAVE_DIR/agent-directories.txt"
    fi
done

# 4. Save any uncommitted git changes in workspaces
log "Saving uncommitted git changes..."
for workspace in "$PERSIST_MOUNT/workspaces"/agent-*; do
    if [ -d "$workspace/.git" ]; then
        AGENT_NAME=$(basename "$workspace")
        cd "$workspace"

        # Check for uncommitted changes
        if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null; then
            log "  $AGENT_NAME: has uncommitted changes, creating stash..."
            # Stash changes with a descriptive message
            su - "$SANDBOX_USER" -c "cd $workspace && git stash push -m 'auto-stash-preemption-$TIMESTAMP'" 2>/dev/null || true
            echo "$AGENT_NAME:stashed" >> "$SAVE_DIR/git-state.txt"
        else
            echo "$AGENT_NAME:clean" >> "$SAVE_DIR/git-state.txt"
        fi
    fi
done

# 5. Save Claude session IDs for each agent
log "Saving Claude session info..."
# Claude stores sessions in SQLite at ~/.claude/
# We can query the most recent session per directory
if [ -f "$SANDBOX_HOME/.claude/claude.db" ] || [ -f "$PERSIST_MOUNT/.claude/claude.db" ]; then
    CLAUDE_DB="$PERSIST_MOUNT/.claude/claude.db"
    if [ -f "$CLAUDE_DB" ]; then
        # Get recent sessions (last 24 hours)
        sqlite3 "$CLAUDE_DB" "SELECT session_id, working_directory, created_at FROM sessions WHERE created_at > datetime('now', '-1 day') ORDER BY created_at DESC;" > "$SAVE_DIR/claude-sessions.txt" 2>/dev/null || true
        log "  Saved Claude session list"
    fi
fi

# 6. Record instance metadata for restart
log "Saving instance metadata..."
INSTANCE_NAME=$(curl -s -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/name" 2>/dev/null || echo "unknown")
ZONE=$(curl -s -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/zone" 2>/dev/null | awk -F/ '{print $NF}')
MACHINE_TYPE=$(curl -s -H "Metadata-Flavor: Google" \
    "http://metadata.google.internal/computeMetadata/v1/instance/machine-type" 2>/dev/null | awk -F/ '{print $NF}')

cat > "$SAVE_DIR/instance-info.txt" << EOF
timestamp=$TIMESTAMP
instance_name=$INSTANCE_NAME
zone=$ZONE
machine_type=$MACHINE_TYPE
preempted=$IS_PREEMPTED
EOF

# 7. Create a "latest" symlink
ln -sfn "$SAVE_DIR" "$STATE_DIR/latest"

# 8. Sync to ensure data is written
log "Syncing filesystem..."
sync

# 9. Clean up old state saves (keep last 10)
log "Cleaning up old state saves..."
ls -dt "$STATE_DIR"/20* 2>/dev/null | tail -n +11 | xargs rm -rf 2>/dev/null || true

log "=== State save complete ==="
log "Saved to: $SAVE_DIR"

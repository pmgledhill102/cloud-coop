#!/bin/bash
# restore-claude-state.sh
# Called on startup to restore Claude Code state after spot preemption
#
# This script:
# 1. Checks for saved state from previous run
# 2. Restores git stashes
# 3. Prepares session info for manual resumption

set -e

PERSIST_MOUNT="/mnt/persist"
STATE_DIR="$PERSIST_MOUNT/session-state"
SANDBOX_USER="${SANDBOX_USER:-sandbox}"
LOG_FILE="/var/log/claude-state-restore.log"

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') $1" | tee -a "$LOG_FILE"
}

log "=== Starting Claude state restore ==="

# Check for latest saved state
LATEST_STATE="$STATE_DIR/latest"
if [ ! -L "$LATEST_STATE" ] || [ ! -d "$LATEST_STATE" ]; then
    log "No saved state found. Fresh start."
    exit 0
fi

SAVE_DIR=$(readlink -f "$LATEST_STATE")
log "Found saved state: $SAVE_DIR"

# Read instance info
if [ -f "$SAVE_DIR/instance-info.txt" ]; then
    source "$SAVE_DIR/instance-info.txt"
    log "Previous instance: $instance_name (preempted: $preempted)"
fi

# 1. Restore git stashes
log "Checking for git stashes to restore..."
if [ -f "$SAVE_DIR/git-state.txt" ]; then
    while IFS=: read -r agent_name status; do
        if [ "$status" = "stashed" ]; then
            WORKSPACE="$PERSIST_MOUNT/workspaces/$agent_name"
            if [ -d "$WORKSPACE/.git" ]; then
                log "  $agent_name: has stashed changes"
                # List stashes for this workspace
                STASH_LIST=$(cd "$WORKSPACE" && git stash list 2>/dev/null | head -3)
                if [ -n "$STASH_LIST" ]; then
                    log "    Stashes available:"
                    echo "$STASH_LIST" | while read -r line; do
                        log "      $line"
                    done
                fi
            fi
        fi
    done < "$SAVE_DIR/git-state.txt"
fi

# 2. Show Claude sessions available for resumption
log "Claude sessions available for resumption:"
if [ -f "$SAVE_DIR/claude-sessions.txt" ]; then
    while IFS='|' read -r session_id working_dir created_at; do
        log "  Session: $session_id"
        log "    Directory: $working_dir"
        log "    Created: $created_at"
    done < "$SAVE_DIR/claude-sessions.txt"
fi

# 3. Create a helper script for the user
RESUME_SCRIPT="/home/$SANDBOX_USER/resume-agents.sh"
cat > "$RESUME_SCRIPT" << 'SCRIPT'
#!/bin/bash
# Auto-generated script to resume Claude agents after preemption
#
# Usage:
#   ./resume-agents.sh           # Resume all agents with --continue
#   ./resume-agents.sh --list    # List available sessions
#   ./resume-agents.sh --pick    # Interactive session picker

PERSIST_MOUNT="/mnt/persist"
STATE_DIR="$PERSIST_MOUNT/session-state/latest"

case "$1" in
    --list)
        echo "Available Claude sessions:"
        echo ""
        claude --resume 2>/dev/null || echo "Run 'claude --resume' for interactive picker"
        ;;
    --pick)
        # Start tmux and open session picker in each window
        AGENT_COUNT=${2:-12}
        SESSION_NAME="claude-agents"

        tmux kill-session -t $SESSION_NAME 2>/dev/null || true
        tmux new-session -d -s $SESSION_NAME -n "agent-1"

        for i in $(seq 2 $AGENT_COUNT); do
            tmux new-window -t $SESSION_NAME -n "agent-$i"
        done

        # Each agent gets interactive session picker
        for i in $(seq 1 $AGENT_COUNT); do
            tmux send-keys -t $SESSION_NAME:agent-$i "cd /workspaces/agent-$i && claude --resume" Enter
        done

        echo "Started $AGENT_COUNT agents with session picker"
        echo "Attach with: tmux attach -t $SESSION_NAME"
        ;;
    *)
        # Default: auto-continue most recent session in each workspace
        AGENT_COUNT=${1:-12}
        SESSION_NAME="claude-agents"

        tmux kill-session -t $SESSION_NAME 2>/dev/null || true
        tmux new-session -d -s $SESSION_NAME -n "agent-1"

        for i in $(seq 2 $AGENT_COUNT); do
            tmux new-window -t $SESSION_NAME -n "agent-$i"
        done

        # Each agent continues its most recent session
        for i in $(seq 1 $AGENT_COUNT); do
            tmux send-keys -t $SESSION_NAME:agent-$i "cd /workspaces/agent-$i && claude --continue --dangerously-skip-permissions" Enter
        done

        echo "Started $AGENT_COUNT agents with --continue (resuming previous sessions)"
        echo "Attach with: tmux attach -t $SESSION_NAME"
        ;;
esac
SCRIPT

chown "$SANDBOX_USER:$SANDBOX_USER" "$RESUME_SCRIPT"
chmod +x "$RESUME_SCRIPT"

# 4. Show summary
log ""
log "=== State restore complete ==="
log ""
log "To resume agents:"
log "  1. SSH into the instance"
log "  2. sudo su - sandbox"
log "  3. ./resume-agents.sh         # Auto-continue all"
log "     ./resume-agents.sh --pick  # Interactive picker"
log ""
log "To restore git stashes manually:"
log "  cd /workspaces/agent-N"
log "  git stash pop"
log ""

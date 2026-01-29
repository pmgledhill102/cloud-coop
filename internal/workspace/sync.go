package workspace

import (
	"fmt"
	"strings"

	"github.com/cloud-coop/cloudcoop/internal/agent"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// SyncOptions configures the sync operation.
type SyncOptions struct {
	AgentCommand string   // command for tmux windows (empty → "bash")
	PreCommands  []string // pre-commands to run before agent
	RepoOwner    string   // from deploykey.ParseRepoRef
	RepoName     string
}

// SyncResult describes the outcome of a sync operation.
type SyncResult struct {
	Slug             string
	Cloned           bool
	Fetched          bool
	WorktreesCreated []string
	WorktreesSkipped []string
	WindowsCreated   []string
	WindowsSkipped   []string
	StaleWorktrees   []string
}

// WorktreeName derives a worktree name from a local worktree.
// Returns ("", false) for bare worktrees or worktrees with no branch/commit.
func WorktreeName(wt Worktree) (string, bool) {
	if wt.Bare {
		return "", false
	}
	if wt.Branch != "" {
		return strings.ReplaceAll(wt.Branch, "/", "-"), true
	}
	if wt.Commit != "" {
		sha := wt.Commit
		if len(sha) > 8 {
			sha = sha[:8]
		}
		return "detached-" + sha, true
	}
	return "", false
}

// ParseRemoteWorktrees parses `git worktree list --porcelain` output from the
// VM and returns a map of name→path for worktrees under /workspaces/<slug>/.
func ParseRemoteWorktrees(output, slug string) map[string]string {
	worktrees := parseWorktrees(output)
	prefix := "/workspaces/" + slug + "/"
	result := make(map[string]string)
	for _, wt := range worktrees {
		if wt.Bare {
			continue
		}
		if !strings.HasPrefix(wt.Path, prefix) {
			continue
		}
		name := strings.TrimPrefix(wt.Path, prefix)
		if name != "" && !strings.Contains(name, "/") {
			result[name] = wt.Path
		}
	}
	return result
}

// Sync synchronizes local worktrees to the remote VM. It clones the bare repo
// if missing, fetches latest, creates worktrees, and starts tmux windows.
func Sync(runner ssh.Runner, info *Info, opts SyncOptions) (*SyncResult, error) {
	slug := info.Slug
	result := &SyncResult{Slug: slug}

	bareRepo := "/repos/" + slug + ".git"
	wsDir := "/workspaces/" + slug

	// 1. Ensure directories exist (sudo for root-owned paths, chown to current user).
	mkdirCmd := fmt.Sprintf(
		`sudo mkdir -p /repos %s && sudo chown "$(id -un):$(id -gn)" /repos %s`,
		shellEscape(wsDir), shellEscape(wsDir),
	)
	_, err := runner.Run(mkdirCmd)
	if err != nil {
		return nil, fmt.Errorf("create directories: %w", err)
	}

	// 2. Check if bare repo exists.
	_, err = runner.Run("test -d " + shellEscape(bareRepo))
	if err != nil {
		// 3. Clone bare repo.
		cloneURL := fmt.Sprintf("git@github-%s:%s/%s.git", slug, opts.RepoOwner, opts.RepoName)
		_, err = runner.Run("git clone --bare " + shellEscape(cloneURL) + " " + shellEscape(bareRepo))
		if err != nil {
			return nil, fmt.Errorf("clone bare repo: %w", err)
		}
		result.Cloned = true
	}

	// Ensure fetch refspec is configured (git clone --bare doesn't set one).
	// Use refs/remotes/origin/* to avoid conflicts with checked-out worktrees.
	_, _ = runner.Run("git -C " + shellEscape(bareRepo) +
		" config remote.origin.fetch '+refs/heads/*:refs/remotes/origin/*'")

	// 4. Fetch all.
	_, err = runner.Run("git -C " + shellEscape(bareRepo) + " fetch --all --prune")
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	result.Fetched = true

	// 5. List remote worktrees.
	wtOutput, err := runner.Run("git -C " + shellEscape(bareRepo) + " worktree list --porcelain")
	if err != nil {
		return nil, fmt.Errorf("list remote worktrees: %w", err)
	}
	remoteWorktrees := ParseRemoteWorktrees(wtOutput, slug)

	// 6. Create missing worktrees.
	localNames := make(map[string]bool)
	for _, wt := range info.Worktrees {
		name, ok := WorktreeName(wt)
		if !ok {
			continue
		}
		localNames[name] = true

		if _, exists := remoteWorktrees[name]; exists {
			result.WorktreesSkipped = append(result.WorktreesSkipped, name)
			continue
		}

		wtPath := wsDir + "/" + name
		var addCmd string
		if wt.Branch != "" {
			// Create local branch tracking the remote ref.
			addCmd = "git -C " + shellEscape(bareRepo) + " worktree add -b " +
				shellEscape(wt.Branch) + " " + shellEscape(wtPath) + " " +
				shellEscape("origin/"+wt.Branch)
		} else {
			addCmd = "git -C " + shellEscape(bareRepo) + " worktree add --detach " +
				shellEscape(wtPath) + " " + shellEscape(wt.Commit)
		}
		_, err = runner.Run(addCmd)
		if err != nil {
			return nil, fmt.Errorf("create worktree %s: %w", name, err)
		}
		result.WorktreesCreated = append(result.WorktreesCreated, name)
	}

	// 7. List existing tmux windows.
	listResult, err := agent.ListSessions(runner, slug)
	if err != nil {
		return nil, fmt.Errorf("list tmux sessions: %w", err)
	}
	existingWindows := make(map[string]bool)
	if !listResult.NoSession {
		for _, s := range listResult.Sessions {
			existingWindows[s.Name] = true
		}
	}

	// 8. Create tmux windows for each worktree.
	agentCmd := opts.AgentCommand
	if agentCmd == "" {
		agentCmd = "bash"
	}
	for _, wt := range info.Worktrees {
		name, ok := WorktreeName(wt)
		if !ok {
			continue
		}
		if existingWindows[name] {
			result.WindowsSkipped = append(result.WindowsSkipped, name)
			continue
		}

		wtPath := wsDir + "/" + name
		command := BuildCommand(wtPath, opts.PreCommands, agentCmd)

		_, err = agent.CreateSession(runner, slug, agent.CreateSessionOptions{
			Name:    name,
			Command: command,
		})
		if err != nil {
			return nil, fmt.Errorf("create tmux window %s: %w", name, err)
		}
		result.WindowsCreated = append(result.WindowsCreated, name)
	}

	// 9. Detect stale worktrees (remote but not local).
	for name := range remoteWorktrees {
		if !localNames[name] {
			result.StaleWorktrees = append(result.StaleWorktrees, name)
		}
	}

	return result, nil
}

// BuildCommand constructs the full shell command chain for a tmux window.
// It joins cd, pre-commands, and the agent command with " && ".
func BuildCommand(worktreePath string, preCommands []string, agentCommand string) string {
	parts := []string{"cd " + shellEscape(worktreePath)}
	parts = append(parts, preCommands...)
	parts = append(parts, agentCommand)
	return strings.Join(parts, " && ")
}

// shellEscape escapes a string for safe use in shell commands.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

package workspace

import (
	"errors"
	"strings"
	"testing"
)

func TestWorktreeName(t *testing.T) {
	tests := []struct {
		name   string
		wt     Worktree
		want   string
		wantOK bool
	}{
		{
			name:   "branch main",
			wt:     Worktree{Branch: "main"},
			want:   "main",
			wantOK: true,
		},
		{
			name:   "branch with slash",
			wt:     Worktree{Branch: "feature/auth"},
			want:   "feature-auth",
			wantOK: true,
		},
		{
			name:   "branch with multiple slashes",
			wt:     Worktree{Branch: "user/paul/fix"},
			want:   "user-paul-fix",
			wantOK: true,
		},
		{
			name:   "detached HEAD",
			wt:     Worktree{Commit: "abc123de90ef1234"},
			want:   "detached-abc123de",
			wantOK: true,
		},
		{
			name:   "bare worktree",
			wt:     Worktree{Bare: true},
			want:   "",
			wantOK: false,
		},
		{
			name:   "empty branch and commit",
			wt:     Worktree{},
			want:   "",
			wantOK: false,
		},
		{
			name:   "short commit",
			wt:     Worktree{Commit: "short"},
			want:   "detached-short",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := WorktreeName(tt.wt)
			if ok != tt.wantOK {
				t.Errorf("WorktreeName() ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("WorktreeName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRemoteWorktrees(t *testing.T) {
	tests := []struct {
		name   string
		output string
		slug   string
		want   map[string]string
	}{
		{
			name: "two worktrees",
			output: "worktree /repos/backend.git\nHEAD abc123\nbare\n\n" +
				"worktree /workspaces/backend/main\nHEAD def456\nbranch refs/heads/main\n\n" +
				"worktree /workspaces/backend/feature-auth\nHEAD 789abc\nbranch refs/heads/feature/auth\n",
			slug: "backend",
			want: map[string]string{
				"main":         "/workspaces/backend/main",
				"feature-auth": "/workspaces/backend/feature-auth",
			},
		},
		{
			name:   "bare repo entry excluded",
			output: "worktree /repos/backend.git\nHEAD abc123\nbare\n",
			slug:   "backend",
			want:   map[string]string{},
		},
		{
			name:   "empty output",
			output: "",
			slug:   "backend",
			want:   map[string]string{},
		},
		{
			name: "worktree under different slug excluded",
			output: "worktree /repos/backend.git\nHEAD abc123\nbare\n\n" +
				"worktree /workspaces/frontend/main\nHEAD def456\nbranch refs/heads/main\n",
			slug: "backend",
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRemoteWorktrees(tt.output, tt.slug)
			if len(got) != len(tt.want) {
				t.Errorf("ParseRemoteWorktrees() returned %d entries, want %d\ngot: %v", len(got), len(tt.want), got)
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ParseRemoteWorktrees()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

// syncMockRunner is a mock ssh.Runner that returns sequential results and records commands.
type syncMockRunner struct {
	calls    []syncMockCall
	callIdx  int
	commands []string
}

type syncMockCall struct {
	output string
	err    error
}

func (m *syncMockRunner) Run(cmd string) (string, error) {
	m.commands = append(m.commands, cmd)
	if m.callIdx >= len(m.calls) {
		return "", errors.New("unexpected call: " + cmd)
	}
	call := m.calls[m.callIdx]
	m.callIdx++
	return call.output, call.err
}

func (m *syncMockRunner) Close() error {
	return nil
}

func TestSync_FreshSync(t *testing.T) {
	// agent.CreateSession calls ListSessions internally, so each CreateSession
	// call generates a list-windows command before the create command.
	runner := &syncMockRunner{
		calls: []syncMockCall{
			// 1. mkdir
			{output: "", err: nil},
			// 2. test -d (not found)
			{output: "", err: errors.New("exit status 1")},
			// 3. git clone --bare
			{output: "", err: nil},
			// 4. git config fetch refspec
			{output: "", err: nil},
			// 5. git fetch
			{output: "", err: nil},
			// 6. git worktree list --porcelain (bare repo only)
			{output: "worktree /repos/backend.git\nHEAD abc\nbare\n"},
			// 7. git worktree add /workspaces/backend/main main
			{output: "", err: nil},
			// 8. git worktree add /workspaces/backend/feature-auth feature/auth
			{output: "", err: nil},
			// 9. agent.ListSessions → tmux list-windows (no session)
			{output: "can't find session: backend", err: errors.New("exit status 1")},
			// 10. agent.CreateSession("main") → ListSessions internally
			{output: "can't find session: backend", err: errors.New("exit status 1")},
			// 11. tmux new-session
			{output: "", err: nil},
			// 12. agent.CreateSession("feature-auth") → ListSessions internally
			{output: "0|main|cd '/workspaces/backend/main' && bash\n", err: nil},
			// 13. tmux new-window
			{output: "", err: nil},
		},
	}

	info := &Info{
		RemoteURL: "git@github.com:acme/backend.git",
		Slug:      "backend",
		Worktrees: []Worktree{
			{Path: "/home/user/backend", Branch: "main", Commit: "abc123"},
			{Path: "/home/user/backend-feature", Branch: "feature/auth", Commit: "def456"},
		},
	}

	result, err := Sync(runner, info, SyncOptions{
		RepoOwner: "acme",
		RepoName:  "backend",
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if !result.Cloned {
		t.Error("expected Cloned = true")
	}
	if !result.Fetched {
		t.Error("expected Fetched = true")
	}
	if len(result.WorktreesCreated) != 2 {
		t.Errorf("expected 2 worktrees created, got %d: %v", len(result.WorktreesCreated), result.WorktreesCreated)
	}
	if len(result.WindowsCreated) != 2 {
		t.Errorf("expected 2 windows created, got %d: %v", len(result.WindowsCreated), result.WindowsCreated)
	}
	if result.Slug != "backend" {
		t.Errorf("expected slug 'backend', got %q", result.Slug)
	}

	// Verify clone URL uses deploy key alias
	found := false
	for _, cmd := range runner.commands {
		if strings.Contains(cmd, "git clone --bare") && strings.Contains(cmd, "git@github-backend:acme/backend.git") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected clone command with deploy key alias")
	}
}

func TestSync_IdempotentReSync(t *testing.T) {
	runner := &syncMockRunner{
		calls: []syncMockCall{
			// 1. mkdir
			{output: "", err: nil},
			// 2. test -d (exists)
			{output: "", err: nil},
			// 3. git config fetch refspec
			{output: "", err: nil},
			// 4. git fetch
			{output: "", err: nil},
			// 4. git worktree list --porcelain (main already exists)
			{output: "worktree /repos/backend.git\nHEAD abc\nbare\n\n" +
				"worktree /workspaces/backend/main\nHEAD def\nbranch refs/heads/main\n"},
			// 5. agent.ListSessions (main window exists)
			{output: "0|main|bash\n", err: nil},
		},
	}

	info := &Info{
		RemoteURL: "git@github.com:acme/backend.git",
		Slug:      "backend",
		Worktrees: []Worktree{
			{Path: "/home/user/backend", Branch: "main", Commit: "abc123"},
		},
	}

	result, err := Sync(runner, info, SyncOptions{
		RepoOwner: "acme",
		RepoName:  "backend",
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if result.Cloned {
		t.Error("expected Cloned = false")
	}
	if !result.Fetched {
		t.Error("expected Fetched = true")
	}
	if len(result.WorktreesCreated) != 0 {
		t.Errorf("expected 0 worktrees created, got %d", len(result.WorktreesCreated))
	}
	if len(result.WorktreesSkipped) != 1 {
		t.Errorf("expected 1 worktree skipped, got %d", len(result.WorktreesSkipped))
	}
	if len(result.WindowsCreated) != 0 {
		t.Errorf("expected 0 windows created, got %d", len(result.WindowsCreated))
	}
	if len(result.WindowsSkipped) != 1 {
		t.Errorf("expected 1 window skipped, got %d", len(result.WindowsSkipped))
	}
}

func TestSync_Incremental(t *testing.T) {
	runner := &syncMockRunner{
		calls: []syncMockCall{
			// 1. mkdir
			{output: "", err: nil},
			// 2. test -d (exists)
			{output: "", err: nil},
			// 3. git config fetch refspec
			{output: "", err: nil},
			// 4. git fetch
			{output: "", err: nil},
			// 4. git worktree list (main exists remotely)
			{output: "worktree /repos/backend.git\nHEAD abc\nbare\n\n" +
				"worktree /workspaces/backend/main\nHEAD def\nbranch refs/heads/main\n"},
			// 5. git worktree add feature-auth
			{output: "", err: nil},
			// 6. agent.ListSessions (main window exists)
			{output: "0|main|bash\n", err: nil},
			// 7. agent.CreateSession("feature-auth") → ListSessions internally
			{output: "0|main|bash\n", err: nil},
			// 8. tmux new-window
			{output: "", err: nil},
		},
	}

	info := &Info{
		RemoteURL: "git@github.com:acme/backend.git",
		Slug:      "backend",
		Worktrees: []Worktree{
			{Path: "/home/user/backend", Branch: "main", Commit: "abc123"},
			{Path: "/home/user/backend-feature", Branch: "feature/auth", Commit: "def456"},
		},
	}

	result, err := Sync(runner, info, SyncOptions{
		RepoOwner: "acme",
		RepoName:  "backend",
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if len(result.WorktreesCreated) != 1 {
		t.Errorf("expected 1 worktree created, got %d", len(result.WorktreesCreated))
	}
	if result.WorktreesCreated[0] != "feature-auth" {
		t.Errorf("expected worktree 'feature-auth', got %q", result.WorktreesCreated[0])
	}
	if len(result.WorktreesSkipped) != 1 {
		t.Errorf("expected 1 worktree skipped, got %d", len(result.WorktreesSkipped))
	}
	if len(result.WindowsCreated) != 1 {
		t.Errorf("expected 1 window created, got %d", len(result.WindowsCreated))
	}
	if len(result.WindowsSkipped) != 1 {
		t.Errorf("expected 1 window skipped, got %d", len(result.WindowsSkipped))
	}
}

func TestSync_StaleDetection(t *testing.T) {
	runner := &syncMockRunner{
		calls: []syncMockCall{
			// 1. mkdir
			{output: "", err: nil},
			// 2. test -d (exists)
			{output: "", err: nil},
			// 3. git config fetch refspec
			{output: "", err: nil},
			// 4. git fetch
			{output: "", err: nil},
			// 4. git worktree list (main + old-experiment on remote)
			{output: "worktree /repos/backend.git\nHEAD abc\nbare\n\n" +
				"worktree /workspaces/backend/main\nHEAD def\nbranch refs/heads/main\n\n" +
				"worktree /workspaces/backend/old-experiment\nHEAD 999\nbranch refs/heads/old-experiment\n"},
			// 5. agent.ListSessions (main exists)
			{output: "0|main|bash\n", err: nil},
		},
	}

	info := &Info{
		RemoteURL: "git@github.com:acme/backend.git",
		Slug:      "backend",
		Worktrees: []Worktree{
			{Path: "/home/user/backend", Branch: "main", Commit: "abc123"},
		},
	}

	result, err := Sync(runner, info, SyncOptions{
		RepoOwner: "acme",
		RepoName:  "backend",
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if len(result.StaleWorktrees) != 1 {
		t.Errorf("expected 1 stale worktree, got %d", len(result.StaleWorktrees))
	}
	if len(result.StaleWorktrees) > 0 && result.StaleWorktrees[0] != "old-experiment" {
		t.Errorf("expected stale worktree 'old-experiment', got %q", result.StaleWorktrees[0])
	}
}

func TestSync_CloneFailure(t *testing.T) {
	runner := &syncMockRunner{
		calls: []syncMockCall{
			// 1. mkdir
			{output: "", err: nil},
			// 2. test -d (not found)
			{output: "", err: errors.New("exit status 1")},
			// 3. git clone --bare (fails)
			{output: "Permission denied", err: errors.New("exit status 128")},
		},
	}

	info := &Info{
		RemoteURL: "git@github.com:acme/backend.git",
		Slug:      "backend",
		Worktrees: []Worktree{
			{Path: "/home/user/backend", Branch: "main", Commit: "abc123"},
		},
	}

	_, err := Sync(runner, info, SyncOptions{
		RepoOwner: "acme",
		RepoName:  "backend",
	})
	if err == nil {
		t.Fatal("expected error from clone failure")
	}
	if !strings.Contains(err.Error(), "clone bare repo") {
		t.Errorf("expected clone error, got: %v", err)
	}
}

func TestSync_MixedTypes(t *testing.T) {
	runner := &syncMockRunner{
		calls: []syncMockCall{
			// 1. mkdir
			{output: "", err: nil},
			// 2. test -d (exists)
			{output: "", err: nil},
			// 3. git config fetch refspec
			{output: "", err: nil},
			// 4. git fetch
			{output: "", err: nil},
			// 4. git worktree list (empty besides bare)
			{output: "worktree /repos/backend.git\nHEAD abc\nbare\n"},
			// 5. git worktree add main
			{output: "", err: nil},
			// 6. git worktree add detached-abc123de
			{output: "", err: nil},
			// 7. agent.ListSessions (no session)
			{output: "can't find session: backend", err: errors.New("exit status 1")},
			// 8. agent.CreateSession("main") → ListSessions
			{output: "can't find session: backend", err: errors.New("exit status 1")},
			// 9. tmux new-session
			{output: "", err: nil},
			// 10. agent.CreateSession("detached-abc123de") → ListSessions
			{output: "0|main|bash\n", err: nil},
			// 11. tmux new-window
			{output: "", err: nil},
		},
	}

	info := &Info{
		RemoteURL: "git@github.com:acme/backend.git",
		Slug:      "backend",
		Worktrees: []Worktree{
			{Path: "/home/user/backend.git", Bare: true, Commit: "abc123"},
			{Path: "/home/user/backend", Branch: "main", Commit: "abc123"},
			{Path: "/home/user/backend-detached", Commit: "abc123de90ef1234"},
		},
	}

	result, err := Sync(runner, info, SyncOptions{
		RepoOwner: "acme",
		RepoName:  "backend",
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Bare should be skipped, so 2 worktrees + 2 windows
	if len(result.WorktreesCreated) != 2 {
		t.Errorf("expected 2 worktrees created, got %d: %v", len(result.WorktreesCreated), result.WorktreesCreated)
	}
	if len(result.WindowsCreated) != 2 {
		t.Errorf("expected 2 windows created, got %d: %v", len(result.WindowsCreated), result.WindowsCreated)
	}
}

func TestSync_CustomCommand(t *testing.T) {
	runner := &syncMockRunner{
		calls: []syncMockCall{
			// 1. mkdir
			{output: "", err: nil},
			// 2. test -d (exists)
			{output: "", err: nil},
			// 3. git config fetch refspec
			{output: "", err: nil},
			// 4. git fetch
			{output: "", err: nil},
			// 4. git worktree list (main exists)
			{output: "worktree /repos/backend.git\nHEAD abc\nbare\n\n" +
				"worktree /workspaces/backend/main\nHEAD def\nbranch refs/heads/main\n"},
			// 5. agent.ListSessions (no session)
			{output: "can't find session: backend", err: errors.New("exit status 1")},
			// 6. agent.CreateSession → ListSessions
			{output: "can't find session: backend", err: errors.New("exit status 1")},
			// 7. tmux new-session
			{output: "", err: nil},
		},
	}

	info := &Info{
		RemoteURL: "git@github.com:acme/backend.git",
		Slug:      "backend",
		Worktrees: []Worktree{
			{Path: "/home/user/backend", Branch: "main", Commit: "abc123"},
		},
	}

	result, err := Sync(runner, info, SyncOptions{
		AgentCommand: "claude",
		RepoOwner:    "acme",
		RepoName:     "backend",
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if len(result.WindowsCreated) != 1 {
		t.Fatalf("expected 1 window created, got %d", len(result.WindowsCreated))
	}

	// Verify the tmux command includes the cd + agent command.
	// The entire command string is shell-escaped by agent.CreateSession,
	// so look for the key parts within the escaped form.
	found := false
	for _, cmd := range runner.commands {
		if strings.Contains(cmd, "/workspaces/backend/main") && strings.Contains(cmd, "claude") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tmux command with 'cd ... && claude'")
		for i, cmd := range runner.commands {
			t.Logf("  command[%d]: %s", i, cmd)
		}
	}
}

func TestSync_DefaultCommand(t *testing.T) {
	runner := &syncMockRunner{
		calls: []syncMockCall{
			// 1. mkdir
			{output: "", err: nil},
			// 2. test -d (exists)
			{output: "", err: nil},
			// 3. git config fetch refspec
			{output: "", err: nil},
			// 4. git fetch
			{output: "", err: nil},
			// 4. git worktree list (main exists remotely)
			{output: "worktree /repos/backend.git\nHEAD abc\nbare\n\n" +
				"worktree /workspaces/backend/main\nHEAD def\nbranch refs/heads/main\n"},
			// 5. agent.ListSessions (no session)
			{output: "can't find session: backend", err: errors.New("exit status 1")},
			// 6. agent.CreateSession → ListSessions
			{output: "can't find session: backend", err: errors.New("exit status 1")},
			// 7. tmux new-session
			{output: "", err: nil},
		},
	}

	info := &Info{
		RemoteURL: "git@github.com:acme/backend.git",
		Slug:      "backend",
		Worktrees: []Worktree{
			{Path: "/home/user/backend", Branch: "main", Commit: "abc123"},
		},
	}

	_, err := Sync(runner, info, SyncOptions{
		AgentCommand: "", // empty → should default to "bash"
		RepoOwner:    "acme",
		RepoName:     "backend",
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Verify the tmux command uses "bash".
	// The entire command string is shell-escaped by agent.CreateSession.
	found := false
	for _, cmd := range runner.commands {
		if strings.Contains(cmd, "/workspaces/backend/main") && strings.Contains(cmd, "bash") && strings.Contains(cmd, "new-session") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tmux command with default 'bash' command")
		for i, cmd := range runner.commands {
			t.Logf("  command[%d]: %s", i, cmd)
		}
	}
}

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		preCommands []string
		agentCmd    string
		want        string
	}{
		{
			name:        "no pre-commands",
			path:        "/workspaces/backend/main",
			preCommands: nil,
			agentCmd:    "claude",
			want:        "cd '/workspaces/backend/main' && claude",
		},
		{
			name:        "with pre-commands",
			path:        "/workspaces/backend/main",
			preCommands: []string{"export A=1", "nvm use 18"},
			agentCmd:    "claude",
			want:        "cd '/workspaces/backend/main' && export A=1 && nvm use 18 && claude",
		},
		{
			name:        "empty pre-commands slice",
			path:        "/workspaces/backend/main",
			preCommands: []string{},
			agentCmd:    "bash",
			want:        "cd '/workspaces/backend/main' && bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCommand(tt.path, tt.preCommands, tt.agentCmd)
			if got != tt.want {
				t.Errorf("BuildCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSync_PreCommands(t *testing.T) {
	runner := &syncMockRunner{
		calls: []syncMockCall{
			// 1. mkdir
			{output: "", err: nil},
			// 2. test -d (exists)
			{output: "", err: nil},
			// 3. git config fetch refspec
			{output: "", err: nil},
			// 4. git fetch
			{output: "", err: nil},
			// 4. git worktree list (main exists remotely)
			{output: "worktree /repos/backend.git\nHEAD abc\nbare\n\n" +
				"worktree /workspaces/backend/main\nHEAD def\nbranch refs/heads/main\n"},
			// 5. agent.ListSessions (no session)
			{output: "can't find session: backend", err: errors.New("exit status 1")},
			// 6. agent.CreateSession → ListSessions
			{output: "can't find session: backend", err: errors.New("exit status 1")},
			// 7. tmux new-session
			{output: "", err: nil},
		},
	}

	info := &Info{
		RemoteURL: "git@github.com:acme/backend.git",
		Slug:      "backend",
		Worktrees: []Worktree{
			{Path: "/home/user/backend", Branch: "main", Commit: "abc123"},
		},
	}

	_, err := Sync(runner, info, SyncOptions{
		AgentCommand: "claude",
		PreCommands:  []string{"export BEADS_NO_DAEMON=1", "nvm use 18"},
		RepoOwner:    "acme",
		RepoName:     "backend",
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Verify the tmux command includes pre-commands in the chain.
	found := false
	for _, cmd := range runner.commands {
		if strings.Contains(cmd, "export BEADS_NO_DAEMON=1") &&
			strings.Contains(cmd, "nvm use 18") &&
			strings.Contains(cmd, "claude") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected tmux command with pre-commands in chain")
		for i, cmd := range runner.commands {
			t.Logf("  command[%d]: %s", i, cmd)
		}
	}
}

package workspace

import (
	"errors"
	"strings"
	"testing"
)

// mockResult holds the return values for a mocked git command.
type mockResult struct {
	output string
	err    error
}

// mockGitRunner implements GitRunner for tests.
type mockGitRunner struct {
	results map[string]mockResult // key = joined args
}

func (m *mockGitRunner) Run(args ...string) (string, error) {
	key := strings.Join(args, " ")
	if r, ok := m.results[key]; ok {
		return r.output, r.err
	}
	return "", errors.New("unexpected git command: " + key)
}

func TestSlugFromURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    string
		wantErr error
	}{
		{
			name: "ssh scp style",
			url:  "git@github.com:acme/acme-backend.git",
			want: "acme-backend",
		},
		{
			name: "https with .git",
			url:  "https://github.com/acme/frontend.git",
			want: "frontend",
		},
		{
			name: "https without .git",
			url:  "https://github.com/acme/frontend",
			want: "frontend",
		},
		{
			name: "ssh:// scheme",
			url:  "ssh://git@github.com/acme/my-repo.git",
			want: "my-repo",
		},
		{
			name: "http scheme",
			url:  "http://gitlab.example.com/group/subgroup/project.git",
			want: "project",
		},
		{
			name: "gitlab nested path",
			url:  "git@gitlab.com:group/subgroup/project.git",
			want: "project",
		},
		{
			name: "mixed case",
			url:  "git@github.com:Acme/My-Repo.git",
			want: "my-repo",
		},
		{
			name: "trailing slash",
			url:  "https://github.com/acme/repo/",
			want: "repo",
		},
		{
			name: "trailing slash with .git",
			url:  "https://github.com/acme/repo.git/",
			want: "repo",
		},
		{
			name: "no .git suffix",
			url:  "git@github.com:org/repo",
			want: "repo",
		},
		{
			name:    "empty string",
			url:     "",
			wantErr: ErrInvalidRemoteURL,
		},
		{
			name:    "whitespace only",
			url:     "   ",
			wantErr: ErrInvalidRemoteURL,
		},
		{
			name:    "invalid scheme",
			url:     "ftp://example.com/repo.git",
			wantErr: ErrInvalidRemoteURL,
		},
		{
			name:    "no path",
			url:     "git@github.com:",
			wantErr: ErrInvalidRemoteURL,
		},
		{
			name: "whitespace around url",
			url:  "  git@github.com:acme/repo.git  ",
			want: "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SlugFromURL(tt.url)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("SlugFromURL(%q) error = %v, want %v", tt.url, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SlugFromURL(%q) unexpected error: %v", tt.url, err)
			}
			if got != tt.want {
				t.Errorf("SlugFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestRemoteURL(t *testing.T) {
	tests := []struct {
		name    string
		mock    mockGitRunner
		want    string
		wantErr error
	}{
		{
			name: "happy path",
			mock: mockGitRunner{results: map[string]mockResult{
				"remote get-url origin": {output: "git@github.com:acme/backend.git"},
			}},
			want: "git@github.com:acme/backend.git",
		},
		{
			name: "not a git repo",
			mock: mockGitRunner{results: map[string]mockResult{
				"remote get-url origin": {err: errors.New("fatal: not a git repository")},
			}},
			wantErr: ErrNotGitRepo,
		},
		{
			name: "no remote",
			mock: mockGitRunner{results: map[string]mockResult{
				"remote get-url origin": {err: errors.New("error: No such remote 'origin'")},
			}},
			wantErr: ErrNoRemote,
		},
		{
			name: "empty output",
			mock: mockGitRunner{results: map[string]mockResult{
				"remote get-url origin": {output: ""},
			}},
			wantErr: ErrNoRemote,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RemoteURL(&tt.mock)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("RemoteURL() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("RemoteURL() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("RemoteURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListWorktrees(t *testing.T) {
	tests := []struct {
		name    string
		mock    mockGitRunner
		want    []Worktree
		wantErr error
	}{
		{
			name: "single worktree",
			mock: mockGitRunner{results: map[string]mockResult{
				"worktree list --porcelain": {output: "worktree /home/user/project\nHEAD abc123def456\nbranch refs/heads/main\n"},
			}},
			want: []Worktree{
				{Path: "/home/user/project", Branch: "main", Commit: "abc123def456"},
			},
		},
		{
			name: "multiple worktrees",
			mock: mockGitRunner{results: map[string]mockResult{
				"worktree list --porcelain": {output: "worktree /home/user/project\nHEAD abc123\nbranch refs/heads/main\n\nworktree /home/user/project-feature\nHEAD def456\nbranch refs/heads/feature-auth\n"},
			}},
			want: []Worktree{
				{Path: "/home/user/project", Branch: "main", Commit: "abc123"},
				{Path: "/home/user/project-feature", Branch: "feature-auth", Commit: "def456"},
			},
		},
		{
			name: "bare worktree",
			mock: mockGitRunner{results: map[string]mockResult{
				"worktree list --porcelain": {output: "worktree /home/user/project.git\nHEAD abc123\nbare\n\nworktree /home/user/project-main\nHEAD def456\nbranch refs/heads/main\n"},
			}},
			want: []Worktree{
				{Path: "/home/user/project.git", Commit: "abc123", Bare: true},
				{Path: "/home/user/project-main", Branch: "main", Commit: "def456"},
			},
		},
		{
			name: "detached HEAD",
			mock: mockGitRunner{results: map[string]mockResult{
				"worktree list --porcelain": {output: "worktree /home/user/project\nHEAD abc123\ndetached\n"},
			}},
			want: []Worktree{
				{Path: "/home/user/project", Commit: "abc123"},
			},
		},
		{
			name: "empty output",
			mock: mockGitRunner{results: map[string]mockResult{
				"worktree list --porcelain": {output: ""},
			}},
			want: nil,
		},
		{
			name: "not a git repo",
			mock: mockGitRunner{results: map[string]mockResult{
				"worktree list --porcelain": {err: errors.New("fatal: not a git repository")},
			}},
			wantErr: ErrNotGitRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ListWorktrees(&tt.mock)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ListWorktrees() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ListWorktrees() unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ListWorktrees() returned %d worktrees, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ListWorktrees()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name    string
		mock    mockGitRunner
		want    *Info
		wantErr string
	}{
		{
			name: "happy path",
			mock: mockGitRunner{results: map[string]mockResult{
				"remote get-url origin":     {output: "git@github.com:acme/backend.git"},
				"worktree list --porcelain": {output: "worktree /home/user/backend\nHEAD abc123\nbranch refs/heads/main\n"},
			}},
			want: &Info{
				RemoteURL: "git@github.com:acme/backend.git",
				Slug:      "backend",
				Worktrees: []Worktree{
					{Path: "/home/user/backend", Branch: "main", Commit: "abc123"},
				},
			},
		},
		{
			name: "remote URL error propagates",
			mock: mockGitRunner{results: map[string]mockResult{
				"remote get-url origin": {err: errors.New("fatal: not a git repository")},
			}},
			wantErr: "detecting remote URL",
		},
		{
			name: "worktree error propagates",
			mock: mockGitRunner{results: map[string]mockResult{
				"remote get-url origin":     {output: "git@github.com:acme/backend.git"},
				"worktree list --porcelain": {err: errors.New("some worktree error")},
			}},
			wantErr: "listing worktrees",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Detect(&tt.mock)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("Detect() expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Detect() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Detect() unexpected error: %v", err)
			}
			if got.RemoteURL != tt.want.RemoteURL {
				t.Errorf("Detect().RemoteURL = %q, want %q", got.RemoteURL, tt.want.RemoteURL)
			}
			if got.Slug != tt.want.Slug {
				t.Errorf("Detect().Slug = %q, want %q", got.Slug, tt.want.Slug)
			}
			if len(got.Worktrees) != len(tt.want.Worktrees) {
				t.Fatalf("Detect().Worktrees has %d entries, want %d", len(got.Worktrees), len(tt.want.Worktrees))
			}
			for i := range got.Worktrees {
				if got.Worktrees[i] != tt.want.Worktrees[i] {
					t.Errorf("Detect().Worktrees[%d] = %+v, want %+v", i, got.Worktrees[i], tt.want.Worktrees[i])
				}
			}
		})
	}
}

func TestParseWorktrees(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Worktree
	}{
		{
			name:  "standard single block",
			input: "worktree /path/to/repo\nHEAD aabbccdd\nbranch refs/heads/main\n",
			want: []Worktree{
				{Path: "/path/to/repo", Branch: "main", Commit: "aabbccdd"},
			},
		},
		{
			name:  "multiple trailing newlines",
			input: "worktree /path/to/repo\nHEAD aabbccdd\nbranch refs/heads/main\n\n\n",
			want: []Worktree{
				{Path: "/path/to/repo", Branch: "main", Commit: "aabbccdd"},
			},
		},
		{
			name:  "no trailing newline",
			input: "worktree /path/to/repo\nHEAD aabbccdd\nbranch refs/heads/main",
			want: []Worktree{
				{Path: "/path/to/repo", Branch: "main", Commit: "aabbccdd"},
			},
		},
		{
			name:  "bare then linked",
			input: "worktree /bare.git\nHEAD 111111\nbare\n\nworktree /linked\nHEAD 222222\nbranch refs/heads/dev\n",
			want: []Worktree{
				{Path: "/bare.git", Commit: "111111", Bare: true},
				{Path: "/linked", Branch: "dev", Commit: "222222"},
			},
		},
		{
			name:  "detached head",
			input: "worktree /path\nHEAD abcdef\ndetached\n",
			want: []Worktree{
				{Path: "/path", Commit: "abcdef"},
			},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace only",
			input: "   \n  \n  ",
			want:  nil,
		},
		{
			name:  "block without worktree line is skipped",
			input: "HEAD aabbccdd\nbranch refs/heads/main\n",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWorktrees(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseWorktrees() returned %d worktrees, want %d\ngot: %+v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseWorktrees()[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

package deploykey

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// --- Mocks ---

type mockCommandRunner struct {
	calls []mockCmdCall
	idx   int
}

type mockCmdCall struct {
	output string
	err    error
}

func (m *mockCommandRunner) Run(name string, args ...string) (string, error) {
	if m.idx >= len(m.calls) {
		return "", errors.New("unexpected command call")
	}
	call := m.calls[m.idx]
	m.idx++
	return call.output, call.err
}

type mockFileSystem struct {
	homeDir string
	homeErr error
	files   map[string][]byte
	statErr map[string]error
}

func (m *mockFileSystem) Stat(path string) (os.FileInfo, error) {
	if err, ok := m.statErr[path]; ok {
		return nil, err
	}
	if _, ok := m.files[path]; ok {
		return nil, nil // file exists
	}
	return nil, os.ErrNotExist
}

func (m *mockFileSystem) ReadFile(path string) ([]byte, error) {
	if data, ok := m.files[path]; ok {
		return data, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockFileSystem) UserHomeDir() (string, error) {
	return m.homeDir, m.homeErr
}

type mockSSHRunner struct {
	calls    []mockSSHCall
	idx      int
	commands []string
}

type mockSSHCall struct {
	output string
	err    error
}

func (m *mockSSHRunner) Run(cmd string) (string, error) {
	m.commands = append(m.commands, cmd)
	if m.idx >= len(m.calls) {
		return "", errors.New("unexpected SSH call")
	}
	call := m.calls[m.idx]
	m.idx++
	return call.output, call.err
}

func (m *mockSSHRunner) Close() error {
	return nil
}

// --- Tests ---

func TestParseRepoRef(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    RepoRef
		wantErr bool
	}{
		{
			name: "SCP-style",
			url:  "git@github.com:cloud-coop/cloudcoop.git",
			want: RepoRef{Owner: "cloud-coop", Name: "cloudcoop"},
		},
		{
			name: "HTTPS",
			url:  "https://github.com/cloud-coop/cloudcoop.git",
			want: RepoRef{Owner: "cloud-coop", Name: "cloudcoop"},
		},
		{
			name: "HTTPS no .git",
			url:  "https://github.com/cloud-coop/cloudcoop",
			want: RepoRef{Owner: "cloud-coop", Name: "cloudcoop"},
		},
		{
			name: "SSH protocol",
			url:  "ssh://git@github.com/cloud-coop/cloudcoop.git",
			want: RepoRef{Owner: "cloud-coop", Name: "cloudcoop"},
		},
		{
			name: "nested GitLab path",
			url:  "https://gitlab.com/group/subgroup/myrepo.git",
			want: RepoRef{Owner: "subgroup", Name: "myrepo"},
		},
		{
			name: "trailing slash",
			url:  "https://github.com/cloud-coop/cloudcoop/",
			want: RepoRef{Owner: "cloud-coop", Name: "cloudcoop"},
		},
		{
			name:    "empty string",
			url:     "",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			url:     "not-a-url",
			wantErr: true,
		},
		{
			name:    "only repo name",
			url:     "https://github.com/cloudcoop.git",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRepoRef(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRepoRef(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
				return
			}
			if !tt.wantErr && (got.Owner != tt.want.Owner || got.Name != tt.want.Name) {
				t.Errorf("ParseRepoRef(%q) = %+v, want %+v", tt.url, got, tt.want)
			}
		})
	}
}

func TestKeyPath(t *testing.T) {
	fs := &mockFileSystem{homeDir: "/home/user"}
	kp, err := KeyPath(fs, "my-repo")
	if err != nil {
		t.Fatalf("KeyPath() error = %v", err)
	}
	if kp.PrivatePath != "/home/user/.ssh/cloudcoop-deploy-my-repo" {
		t.Errorf("PrivatePath = %q, want %q", kp.PrivatePath, "/home/user/.ssh/cloudcoop-deploy-my-repo")
	}
	if kp.PublicPath != "/home/user/.ssh/cloudcoop-deploy-my-repo.pub" {
		t.Errorf("PublicPath = %q, want %q", kp.PublicPath, "/home/user/.ssh/cloudcoop-deploy-my-repo.pub")
	}
}

func TestKeyPath_HomeDirError(t *testing.T) {
	fs := &mockFileSystem{homeErr: errors.New("no home")}
	_, err := KeyPath(fs, "slug")
	if err == nil {
		t.Fatal("KeyPath() expected error")
	}
}

func TestEnsureKey_NewKey(t *testing.T) {
	fs := &mockFileSystem{
		homeDir: "/home/user",
		files: map[string][]byte{
			// Public key gets created after keygen
			"/home/user/.ssh/cloudcoop-deploy-myrepo.pub": []byte("ssh-ed25519 AAAA... comment"),
		},
	}
	cmd := &mockCommandRunner{
		calls: []mockCmdCall{
			{output: "", err: nil}, // ssh-keygen succeeds
			{output: "", err: nil}, // gh auth status succeeds
			{output: "", err: nil}, // gh api register succeeds
		},
	}

	result, err := EnsureKey(fs, cmd, Options{
		Slug: "myrepo",
		Repo: RepoRef{Owner: "owner", Name: "myrepo"},
	})
	if err != nil {
		t.Fatalf("EnsureKey() error = %v", err)
	}
	if !result.Generated {
		t.Error("Expected Generated = true")
	}
	if !result.Registered {
		t.Error("Expected Registered = true")
	}
	if result.ManualNeeded {
		t.Error("Expected ManualNeeded = false")
	}
}

func TestEnsureKey_ExistingKey(t *testing.T) {
	fs := &mockFileSystem{
		homeDir: "/home/user",
		files: map[string][]byte{
			"/home/user/.ssh/cloudcoop-deploy-myrepo":     []byte("PRIVATE KEY"),
			"/home/user/.ssh/cloudcoop-deploy-myrepo.pub": []byte("ssh-ed25519 AAAA... comment"),
		},
	}
	cmd := &mockCommandRunner{
		calls: []mockCmdCall{
			{output: "", err: nil}, // gh auth status
			{output: "", err: nil}, // gh api register
		},
	}

	result, err := EnsureKey(fs, cmd, Options{
		Slug: "myrepo",
		Repo: RepoRef{Owner: "owner", Name: "myrepo"},
	})
	if err != nil {
		t.Fatalf("EnsureKey() error = %v", err)
	}
	if result.Generated {
		t.Error("Expected Generated = false for existing key")
	}
	if !result.Registered {
		t.Error("Expected Registered = true")
	}
}

func TestEnsureKey_NoGH(t *testing.T) {
	fs := &mockFileSystem{
		homeDir: "/home/user",
		files: map[string][]byte{
			"/home/user/.ssh/cloudcoop-deploy-myrepo":     []byte("PRIVATE KEY"),
			"/home/user/.ssh/cloudcoop-deploy-myrepo.pub": []byte("ssh-ed25519 AAAA... comment"),
		},
	}
	cmd := &mockCommandRunner{
		calls: []mockCmdCall{
			{output: "", err: errors.New("executable file not found")}, // gh not available
		},
	}

	result, err := EnsureKey(fs, cmd, Options{
		Slug: "myrepo",
		Repo: RepoRef{Owner: "owner", Name: "myrepo"},
	})
	if err != nil {
		t.Fatalf("EnsureKey() error = %v", err)
	}
	if !result.ManualNeeded {
		t.Error("Expected ManualNeeded = true")
	}
	if result.ManualMessage == "" {
		t.Error("Expected ManualMessage to be non-empty")
	}
	if !strings.Contains(result.ManualMessage, "owner/myrepo") {
		t.Errorf("ManualMessage should contain repo ref, got: %s", result.ManualMessage)
	}
}

func TestEnsureKey_GHRegisterFails(t *testing.T) {
	fs := &mockFileSystem{
		homeDir: "/home/user",
		files: map[string][]byte{
			"/home/user/.ssh/cloudcoop-deploy-myrepo":     []byte("PRIVATE KEY"),
			"/home/user/.ssh/cloudcoop-deploy-myrepo.pub": []byte("ssh-ed25519 AAAA... comment"),
		},
	}
	cmd := &mockCommandRunner{
		calls: []mockCmdCall{
			{output: "", err: nil},                         // gh auth status succeeds
			{output: "", err: errors.New("403 forbidden")}, // gh api fails
		},
	}

	_, err := EnsureKey(fs, cmd, Options{
		Slug: "myrepo",
		Repo: RepoRef{Owner: "owner", Name: "myrepo"},
	})
	if err == nil {
		t.Fatal("Expected error from EnsureKey")
	}
	if !errors.Is(err, ErrGHRegisterFailed) {
		t.Errorf("Expected ErrGHRegisterFailed, got: %v", err)
	}
}

func TestEnsureKey_ReRegistersOnConflict(t *testing.T) {
	fs := &mockFileSystem{
		homeDir: "/home/user",
		files: map[string][]byte{
			"/home/user/.ssh/cloudcoop-deploy-myrepo":     []byte("PRIVATE KEY"),
			"/home/user/.ssh/cloudcoop-deploy-myrepo.pub": []byte("ssh-ed25519 AAAA... comment"),
		},
	}
	cmd := &mockCommandRunner{
		calls: []mockCmdCall{
			{output: "", err: nil},                                 // gh auth status
			{output: "", err: errors.New("key is already in use")}, // gh api POST (conflict)
			{output: "12345", err: nil},                            // gh api GET (list keys, jq extracts ID)
			{output: "", err: nil},                                 // gh api DELETE
			{output: "", err: nil},                                 // gh api POST (re-register)
		},
	}

	result, err := EnsureKey(fs, cmd, Options{
		Slug: "myrepo",
		Repo: RepoRef{Owner: "owner", Name: "myrepo"},
	})
	if err != nil {
		t.Fatalf("EnsureKey() error = %v", err)
	}
	if !result.Registered {
		t.Error("Expected Registered = true after re-register")
	}
	if cmd.idx != 5 {
		t.Errorf("Expected 5 command calls, got %d", cmd.idx)
	}
}

func TestSetupVM_FullFlow(t *testing.T) {
	fs := &mockFileSystem{
		homeDir: "/home/user",
		files: map[string][]byte{
			"/home/user/.ssh/cloudcoop-deploy-myrepo": []byte("PRIVATE KEY DATA"),
		},
	}
	runner := &mockSSHRunner{
		calls: []mockSSHCall{
			{output: "", err: nil},                     // write key
			{output: "", err: errors.New("not found")}, // grep config (not found)
			{output: "", err: nil},                     // append config
			{output: "HEAD ref", err: nil},             // git ls-remote
		},
	}

	result, err := SetupVM(runner, fs, KeyPair{
		PrivatePath: "/home/user/.ssh/cloudcoop-deploy-myrepo",
		PublicPath:  "/home/user/.ssh/cloudcoop-deploy-myrepo.pub",
	}, Options{
		Slug: "myrepo",
		Repo: RepoRef{Owner: "owner", Name: "myrepo"},
	})
	if err != nil {
		t.Fatalf("SetupVM() error = %v", err)
	}
	if !result.KeyCopied {
		t.Error("Expected KeyCopied = true")
	}
	if !result.ConfigWritten {
		t.Error("Expected ConfigWritten = true")
	}
	if !result.Verified {
		t.Error("Expected Verified = true")
	}

	// Check that key write command contains base64
	if len(runner.commands) < 1 {
		t.Fatal("Expected at least 1 command")
	}
	if !strings.Contains(runner.commands[0], "base64") {
		t.Errorf("Expected key write command to use base64, got: %s", runner.commands[0])
	}
}

func TestSetupVM_PreflightFails(t *testing.T) {
	fs := &mockFileSystem{
		homeDir: "/home/user",
		files: map[string][]byte{
			"/home/user/.ssh/cloudcoop-deploy-myrepo": []byte("PRIVATE KEY DATA"),
		},
	}
	runner := &mockSSHRunner{
		calls: []mockSSHCall{
			{output: "", err: nil},                         // write key
			{output: "", err: errors.New("not found")},     // grep config (not found)
			{output: "", err: nil},                         // append config
			{output: "", err: errors.New("access denied")}, // git ls-remote fails
		},
	}

	result, err := SetupVM(runner, fs, KeyPair{
		PrivatePath: "/home/user/.ssh/cloudcoop-deploy-myrepo",
		PublicPath:  "/home/user/.ssh/cloudcoop-deploy-myrepo.pub",
	}, Options{
		Slug: "myrepo",
		Repo: RepoRef{Owner: "owner", Name: "myrepo"},
	})
	if err == nil {
		t.Fatal("Expected error from SetupVM")
	}
	if !errors.Is(err, ErrPreflightFailed) {
		t.Errorf("Expected ErrPreflightFailed, got: %v", err)
	}
	if result.VerifyError == "" {
		t.Error("Expected VerifyError to be set")
	}
}

func TestSetupVM_IdempotentConfig(t *testing.T) {
	fs := &mockFileSystem{
		homeDir: "/home/user",
		files: map[string][]byte{
			"/home/user/.ssh/cloudcoop-deploy-myrepo": []byte("PRIVATE KEY DATA"),
		},
	}
	runner := &mockSSHRunner{
		calls: []mockSSHCall{
			{output: "", err: nil},         // write key
			{output: "", err: nil},         // grep config (FOUND - already exists)
			{output: "HEAD ref", err: nil}, // git ls-remote
		},
	}

	result, err := SetupVM(runner, fs, KeyPair{
		PrivatePath: "/home/user/.ssh/cloudcoop-deploy-myrepo",
		PublicPath:  "/home/user/.ssh/cloudcoop-deploy-myrepo.pub",
	}, Options{
		Slug: "myrepo",
		Repo: RepoRef{Owner: "owner", Name: "myrepo"},
	})
	if err != nil {
		t.Fatalf("SetupVM() error = %v", err)
	}
	if !result.ConfigWritten {
		t.Error("Expected ConfigWritten = true (idempotent)")
	}
	// Should only have 3 commands (write key, grep config, verify) - no append
	if len(runner.commands) != 3 {
		t.Errorf("Expected 3 commands for idempotent case, got %d: %v", len(runner.commands), runner.commands)
	}
}

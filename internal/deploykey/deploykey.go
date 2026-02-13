// Package deploykey manages SSH deploy keys for GitHub repository access.
package deploykey

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/cloud-coop/cloudcoop/internal/shell"
	"github.com/cloud-coop/cloudcoop/internal/ssh"
)

// Sentinel errors.
var (
	ErrInvalidRemoteURL = errors.New("invalid remote URL")
	ErrKeyGenFailed     = errors.New("ssh-keygen failed")
	ErrGHNotAvailable   = errors.New("gh CLI not available")
	ErrGHNotAuthed      = errors.New("gh CLI not authenticated")
	ErrGHRegisterFailed = errors.New("failed to register deploy key")
	ErrSCPFailed        = errors.New("failed to copy key to VM")
	ErrPreflightFailed  = errors.New("deploy key verification failed")
)

// RepoRef identifies a GitHub repository by owner and name.
type RepoRef struct {
	Owner string
	Name  string
}

// KeyPair holds paths to the private and public key files.
type KeyPair struct {
	PrivatePath string
	PublicPath  string
}

// SetupResult describes the outcome of EnsureKey.
type SetupResult struct {
	KeyPair       KeyPair
	Generated     bool   // true if key was newly generated
	Registered    bool   // true if registered on GitHub this run
	ManualNeeded  bool   // true if gh unavailable
	ManualMessage string // fallback instructions
}

// VMSetupResult describes the outcome of SetupVM.
type VMSetupResult struct {
	KeyCopied     bool
	ConfigWritten bool
	Verified      bool
	VerifyError   string
}

// Options configures deploy key operations.
type Options struct {
	Slug      string
	RemoteURL string
	Repo      RepoRef
}

// CommandRunner abstracts command execution for testability.
type CommandRunner interface {
	Run(name string, args ...string) (string, error)
}

// FileSystem abstracts filesystem operations for testability.
type FileSystem interface {
	Stat(path string) (os.FileInfo, error)
	ReadFile(path string) ([]byte, error)
	UserHomeDir() (string, error)
}

// execCommandRunner implements CommandRunner using os/exec.
type execCommandRunner struct{}

func (r *execCommandRunner) Run(name string, args ...string) (string, error) {
	return runExecCommand(name, args...)
}

// NewCommandRunner returns a production CommandRunner.
func NewCommandRunner() CommandRunner {
	return &execCommandRunner{}
}

// osFileSystem implements FileSystem using the real OS.
type osFileSystem struct{}

func (f *osFileSystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (f *osFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (f *osFileSystem) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

// NewFileSystem returns a production FileSystem.
func NewFileSystem() FileSystem {
	return &osFileSystem{}
}

// ParseRepoRef extracts owner and repository name from a git remote URL.
// Supports SCP-style (git@github.com:owner/repo.git), HTTPS, and SSH URLs.
func ParseRepoRef(remoteURL string) (RepoRef, error) {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return RepoRef{}, ErrInvalidRemoteURL
	}

	var path string
	switch {
	case strings.HasPrefix(remoteURL, "ssh://"),
		strings.HasPrefix(remoteURL, "https://"),
		strings.HasPrefix(remoteURL, "http://"):
		u, err := url.Parse(remoteURL)
		if err != nil {
			return RepoRef{}, fmt.Errorf("%w: %s", ErrInvalidRemoteURL, err)
		}
		path = u.Path

	case strings.Contains(remoteURL, ":") && !strings.Contains(remoteURL, "://"):
		// SCP-style: git@host:path
		idx := strings.Index(remoteURL, ":")
		path = remoteURL[idx+1:]

	default:
		return RepoRef{}, fmt.Errorf("%w: %s", ErrInvalidRemoteURL, remoteURL)
	}

	// Strip trailing slashes and .git suffix
	path = strings.TrimRight(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimRight(path, "/")
	path = strings.TrimLeft(path, "/")

	// Extract last two path segments (owner/repo)
	segments := strings.Split(path, "/")
	if len(segments) < 2 {
		return RepoRef{}, fmt.Errorf("%w: need owner/repo in %s", ErrInvalidRemoteURL, remoteURL)
	}

	// Take the last two segments (supports nested GitLab paths like group/subgroup/repo)
	owner := segments[len(segments)-2]
	name := segments[len(segments)-1]

	if owner == "" || name == "" {
		return RepoRef{}, fmt.Errorf("%w: empty owner or repo in %s", ErrInvalidRemoteURL, remoteURL)
	}

	return RepoRef{Owner: owner, Name: name}, nil
}

// KeyPath returns the expected key pair paths for a given slug.
func KeyPath(fs FileSystem, slug string) (KeyPair, error) {
	home, err := fs.UserHomeDir()
	if err != nil {
		return KeyPair{}, fmt.Errorf("get home dir: %w", err)
	}

	privPath := home + "/.ssh/cloudcoop-deploy-" + slug
	return KeyPair{
		PrivatePath: privPath,
		PublicPath:  privPath + ".pub",
	}, nil
}

// EnsureKey checks for an existing deploy key or generates a new one,
// then attempts to register it on GitHub via the gh CLI.
func EnsureKey(fs FileSystem, cmd CommandRunner, opts Options) (*SetupResult, error) {
	kp, err := KeyPath(fs, opts.Slug)
	if err != nil {
		return nil, err
	}

	result := &SetupResult{KeyPair: kp}

	// Check if key already exists
	_, err = fs.Stat(kp.PrivatePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat key: %w", err)
		}

		// Generate new key
		_, genErr := cmd.Run("ssh-keygen",
			"-t", "ed25519",
			"-f", kp.PrivatePath,
			"-N", "",
			"-C", "cloudcoop-deploy-"+opts.Slug,
		)
		if genErr != nil {
			return nil, fmt.Errorf("%w: %s", ErrKeyGenFailed, genErr)
		}
		result.Generated = true
	}

	// Try to register via gh CLI
	regErr := registerKey(fs, cmd, kp, opts)
	if regErr != nil {
		if errors.Is(regErr, ErrGHNotAvailable) || errors.Is(regErr, ErrGHNotAuthed) {
			result.ManualNeeded = true
			pubKey, readErr := fs.ReadFile(kp.PublicPath)
			if readErr != nil {
				return nil, fmt.Errorf("read public key: %w", readErr)
			}
			result.ManualMessage = fmt.Sprintf(
				"Add this deploy key to %s/%s:\n\n"+
					"  1. Go to https://github.com/%s/%s/settings/keys\n"+
					"  2. Click 'Add deploy key'\n"+
					"  3. Title: cloudcoop-deploy-%s\n"+
					"  4. Key:\n%s",
				opts.Repo.Owner, opts.Repo.Name,
				opts.Repo.Owner, opts.Repo.Name,
				opts.Slug,
				strings.TrimSpace(string(pubKey)),
			)
			return result, nil
		}
		return nil, regErr
	}
	result.Registered = true

	return result, nil
}

// registerKey attempts to register the public key on GitHub via gh CLI.
func registerKey(fs FileSystem, cmd CommandRunner, kp KeyPair, opts Options) error {
	// Check if gh is available
	_, err := cmd.Run("gh", "auth", "status")
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "not found") || strings.Contains(errStr, "executable file not found") {
			return ErrGHNotAvailable
		}
		return ErrGHNotAuthed
	}

	// Read public key
	pubKey, err := fs.ReadFile(kp.PublicPath)
	if err != nil {
		return fmt.Errorf("read public key: %w", err)
	}

	// Register via gh api using field flags (avoids stdin and unsupported --body flag)
	endpoint := fmt.Sprintf("repos/%s/%s/keys", opts.Repo.Owner, opts.Repo.Name)
	title := fmt.Sprintf("cloudcoop-deploy-%s", opts.Slug)
	key := strings.TrimSpace(string(pubKey))
	_, err = cmd.Run("gh", "api", endpoint, "--method", "POST",
		"-f", "title="+title,
		"-f", "key="+key,
		"-F", "read_only=false")
	if err != nil {
		// "key is already in use" means the deploy key was registered in a
		// previous run — treat as success for idempotent re-sync.
		if strings.Contains(err.Error(), "key is already in use") {
			return nil
		}
		return fmt.Errorf("%w: %s", ErrGHRegisterFailed, err)
	}

	return nil
}

// SetupVM copies the deploy key to the VM and configures SSH for the repo.
func SetupVM(runner ssh.Runner, fs FileSystem, keyPair KeyPair, opts Options) (*VMSetupResult, error) {
	result := &VMSetupResult{}

	// Read private key
	privKey, err := fs.ReadFile(keyPair.PrivatePath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	// Transfer key via base64 encoding (avoids SCP dependency).
	// Use $HOME instead of ~ because ~ doesn't expand inside quotes.
	b64Key := base64.StdEncoding.EncodeToString(privKey)
	remoteKeyPath := "$HOME/.ssh/cloudcoop-deploy-" + opts.Slug

	// Create .ssh dir, write key, set permissions
	writeCmd := fmt.Sprintf(
		`mkdir -p "$HOME/.ssh" && echo %s | base64 -d > "%s" && chmod 600 "%s"`,
		shellEscape(b64Key),
		remoteKeyPath,
		remoteKeyPath,
	)
	_, err = runner.Run(writeCmd)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSCPFailed, err)
	}
	result.KeyCopied = true

	// Configure SSH host alias for this repo
	hostAlias := "github-" + opts.Slug
	configBlock := fmt.Sprintf("\nHost %s\n  HostName github.com\n  User git\n  IdentityFile ~/.ssh/cloudcoop-deploy-%s\n  IdentitiesOnly yes\n",
		hostAlias, opts.Slug)

	// Check if config already contains this host alias (idempotent)
	checkCmd := fmt.Sprintf("grep -q %s ~/.ssh/config 2>/dev/null", shellEscape("Host "+hostAlias))
	_, err = runner.Run(checkCmd)
	if err != nil {
		// Not found — append config
		appendCmd := fmt.Sprintf("echo %s >> ~/.ssh/config && chmod 600 ~/.ssh/config",
			shellEscape(configBlock))
		_, err = runner.Run(appendCmd)
		if err != nil {
			return result, fmt.Errorf("write SSH config: %w", err)
		}
	}
	result.ConfigWritten = true

	// Preflight: verify git access via the host alias
	verifyCmd := fmt.Sprintf("git ls-remote %s:%s/%s.git HEAD",
		shellEscape(hostAlias),
		shellEscape(opts.Repo.Owner),
		shellEscape(opts.Repo.Name))
	_, err = runner.Run(verifyCmd)
	if err != nil {
		result.VerifyError = err.Error()
		return result, fmt.Errorf("%w: %s", ErrPreflightFailed, err)
	}
	result.Verified = true

	return result, nil
}

// shellEscape is a convenience alias for shell.Escape.
var shellEscape = shell.Escape

func runExecCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cloud-coop/cloudcoop/internal/deploykey"
	"github.com/cloud-coop/cloudcoop/internal/log"
	"github.com/cloud-coop/cloudcoop/internal/workspace"
)

var agentsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local worktrees to the VM",
	Long: `Sync local git worktrees to the cloud VM.

Reads the local repository's worktrees, clones a bare repo on the VM if needed,
creates matching worktrees, and starts tmux windows for each.

Run from within a git repository with an origin remote configured.

Examples:
  cloudcoop agents sync                    # sync current repo
  cloudcoop agents sync --command claude   # override agent command`,
	RunE: runAgentsSync,
}

var syncCommand string

func init() {
	agentsSyncCmd.Flags().StringVar(&syncCommand, "command", "", "command to run in tmux windows")
}

func runAgentsSync(cmd *cobra.Command, args []string) error {
	// 1. Detect workspace.
	info, err := workspace.Detect(workspace.NewGitRunner("."))
	if err != nil {
		if errors.Is(err, workspace.ErrNotGitRepo) {
			fmt.Fprintln(os.Stderr, "Must run from a git repository.")
			return nil
		}
		if errors.Is(err, workspace.ErrNoRemote) {
			fmt.Fprintln(os.Stderr, "No origin remote configured.")
			return nil
		}
		return fmt.Errorf("detect workspace: %w", err)
	}

	log.Debug("detected workspace", "slug", info.Slug, "worktrees", len(info.Worktrees))

	// 2. Standard VM connection.
	conn, err := connectToVM(cmd)
	if err != nil {
		return err
	}
	if conn == nil {
		return nil
	}
	defer conn.Close()

	// 3. Git identity: copy local user.name/user.email to VM.
	gitID, ok := workspace.LocalGitIdentity(workspace.NewGitRunner("."))
	if ok {
		if err := workspace.SetupVMGitIdentity(conn.Client, gitID); err != nil {
			return fmt.Errorf("git identity setup: %w", err)
		}
		log.Debug("set VM git identity", "name", gitID.Name, "email", gitID.Email)
	} else {
		fmt.Fprintln(os.Stderr, "Warning: local git user.name/user.email not configured, skipping VM git identity setup")
	}

	// 4. Deploy key setup.
	repo, err := deploykey.ParseRepoRef(info.RemoteURL)
	if err != nil {
		return fmt.Errorf("parse remote URL: %w", err)
	}

	fs := deploykey.NewFileSystem()
	cmdRunner := deploykey.NewCommandRunner()
	dkOpts := deploykey.Options{
		Slug:      info.Slug,
		RemoteURL: info.RemoteURL,
		Repo:      repo,
	}

	setupResult, err := deploykey.EnsureKey(fs, cmdRunner, dkOpts)
	if err != nil {
		return fmt.Errorf("deploy key setup: %w", err)
	}

	if setupResult.ManualNeeded {
		fmt.Fprintln(os.Stderr, setupResult.ManualMessage)
		return nil
	}

	vmSetup, err := deploykey.SetupVM(conn.Client, fs, setupResult.KeyPair, dkOpts)
	if err != nil {
		if errors.Is(err, deploykey.ErrPreflightFailed) {
			fmt.Fprintf(os.Stderr, "Deploy key verification failed: %s\n", vmSetup.VerifyError)
			return nil
		}
		return fmt.Errorf("deploy key VM setup: %w", err)
	}

	// 5. Resolve agent command: flag > repo-specific > default > "" (sync defaults to "bash").
	agentCommand := syncCommand
	if agentCommand == "" {
		agentCommand = conn.Config.Agents.ResolveCommand(info.Slug)
	}

	// 6. Resolve pre-commands: global + repo-specific.
	preCommands := conn.Config.Agents.ResolvePreCommands(info.Slug)

	// 7. Sync.
	syncResult, err := workspace.Sync(conn.Client, info, workspace.SyncOptions{
		AgentCommand: agentCommand,
		PreCommands:  preCommands,
		RepoOwner:    repo.Owner,
		RepoName:     repo.Name,
	})
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	// 8. Print results.
	printSyncResult(syncResult)
	return nil
}

func printSyncResult(r *workspace.SyncResult) {
	fmt.Printf("Repository: %s\n", r.Slug)
	fmt.Println()

	if r.Cloned {
		fmt.Println("Bare clone: created")
	} else {
		fmt.Println("Bare clone: exists")
	}
	if r.Fetched {
		fmt.Println("Fetched latest")
	}
	fmt.Println()

	fmt.Println("Worktrees:")
	for _, name := range r.WorktreesCreated {
		fmt.Printf("  + %-20s (created)\n", name)
	}
	for _, name := range r.WorktreesSkipped {
		fmt.Printf("  = %-20s (exists)\n", name)
	}
	fmt.Println()

	fmt.Println("Agent sessions:")
	for _, name := range r.WindowsCreated {
		fmt.Printf("  + %-20s (started)\n", name)
	}
	for _, name := range r.WindowsSkipped {
		fmt.Printf("  = %-20s (exists)\n", name)
	}

	if len(r.StaleWorktrees) > 0 {
		fmt.Println()
		fmt.Println("Stale (remote only):")
		for _, name := range r.StaleWorktrees {
			fmt.Printf("  ! %s\n", name)
		}
	}
}

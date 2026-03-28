package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for cloudcoop.

To load completions:

Bash:
  $ source <(cloudcoop completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ cloudcoop completion bash > /etc/bash_completion.d/cloudcoop
  # macOS:
  $ cloudcoop completion bash > $(brew --prefix)/etc/bash_completion.d/cloudcoop

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. Execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  # To load completions for each session, execute once:
  $ cloudcoop completion zsh > "${fpath[1]}/_cloudcoop"
  # You will need to start a new shell for this to take effect.

Fish:
  $ cloudcoop completion fish | source
  # To load completions for each session, execute once:
  $ cloudcoop completion fish > ~/.config/fish/completions/cloudcoop.fish

PowerShell:
  PS> cloudcoop completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, add the output to your profile.`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("specify a shell: bash, zsh, fish, or powershell")
		}
		return cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs)(cmd, args)
	},
	Example: `  cloudcoop completion bash
  cloudcoop completion zsh
  source <(cloudcoop completion zsh)`,
	Annotations: map[string]string{"skip-config": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "instrumentation-score",
	Short: "Evaluate Prometheus metrics quality with automated scoring",
	Long: `Instrumentation Score Service - A spec-compliant tool for measuring Prometheus metrics quality.

Implements the Instrumentation Score specification (https://github.com/instrumentation-score/spec)
adapted for Prometheus metrics.

Commands:
  analyze     - Collect metrics from Prometheus where grouped by job
  evaluate    - Evaluate job metrics with scoring and cost analysis
  completion  - Generate shell completion scripts

Workflow:
  1. Collect: instrumentation-score analyze --output-dir ./reports
  2. Evaluate: instrumentation-score evaluate --job-dir ./reports/job_metrics_*/`,
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for instrumentation-score.

To load completions:

Bash:
  $ source <(instrumentation-score completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ instrumentation-score completion bash > /etc/bash_completion.d/instrumentation-score
  # macOS:
  $ instrumentation-score completion bash > $(brew --prefix)/etc/bash_completion.d/instrumentation-score

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ instrumentation-score completion zsh > "${fpath[1]}/_instrumentation-score"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ instrumentation-score completion fish | source

  # To load completions for each session, execute once:
  $ instrumentation-score completion fish > ~/.config/fish/completions/instrumentation-score.fish

PowerShell:
  PS> instrumentation-score completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> instrumentation-score completion powershell > instrumentation-score.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		switch args[0] {
		case "bash":
			err = cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			err = cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			err = cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			err = cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating completion: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(evaluateCmd)
	rootCmd.AddCommand(completionCmd)
}

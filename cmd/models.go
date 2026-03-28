package cmd

import (
	"fmt"

	"github.com/fingerprint/notetools/internal/llama"
	"github.com/spf13/cobra"
)

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage GGUF models",
}

var modelsPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Download all required models from HuggingFace",
	RunE: func(cmd *cobra.Command, args []string) error {
		for _, m := range llama.AllModels {
			fmt.Fprintf(cmd.ErrOrStderr(), "Pulling %s...\n", m.Name)
			if err := m.Pull(); err != nil {
				return fmt.Errorf("failed to pull %s: %w", m.Name, err)
			}
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "All models ready.")
		return nil
	},
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List models and their download status",
	Run: func(cmd *cobra.Command, args []string) {
		for _, m := range llama.AllModels {
			status := "missing"
			if m.IsReady() {
				status = "ready"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %s\n", m.Name, status)
		}
	},
}

func init() {
	modelsCmd.AddCommand(modelsPullCmd)
	modelsCmd.AddCommand(modelsListCmd)
	rootCmd.AddCommand(modelsCmd)
}

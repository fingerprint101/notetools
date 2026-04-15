package cmd

import (
	"fmt"

	"github.com/fingerprint/notetools/internal/claude"
	"github.com/fingerprint/notetools/internal/codex"
	"github.com/fingerprint/notetools/internal/config"
	"github.com/fingerprint/notetools/internal/llm"
	"github.com/fingerprint/notetools/internal/opencode"
	"github.com/spf13/cobra"
)

var (
	noOverwrite bool
	appConfig   config.Config
)

var rootCmd = &cobra.Command{
	Use:   "nt",
	Short: "AI CLI for document explanation, transcript cleaning, and note merging",
	Long:  "nt (notetools) uses LLM providers (via opencode) to explain PDFs, clean transcripts, and merge notes.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		appConfig, err = config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&noOverwrite, "no-overwrite", false, "skip if output file already exists")
}

func Execute() error {
	return rootCmd.Execute()
}

func providerFor(cmdName string) (llm.Provider, string) {
	cc := config.GetCommandConfig(appConfig, cmdName)
	switch cc.Provider {
	case "claude":
		return claude.New(), cc.Model
	case "codex":
		return codex.New(), cc.Model
	case "opencode":
		return opencode.New(), cc.Model
	default:
		return opencode.New(), cc.Model
	}
}

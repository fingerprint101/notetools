package cmd

import (
	"fmt"

	"github.com/fingerprint/notetools/internal/app"
	"github.com/fingerprint/notetools/internal/llm"
	"github.com/spf13/cobra"
)

var (
	noOverwrite bool
	appConfig   app.Config
)

var rootCmd = &cobra.Command{
	Use:   "nt",
	Short: "AI CLI for document explanation, transcript cleaning, note merging, and coverage checks",
	Long:  "nt (notetools) uses configurable LLM CLI providers to explain PDFs, clean transcripts, merge notes, and check coverage.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		appConfig, err = app.Load()
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
	return app.ProviderFor(appConfig, cmdName)
}

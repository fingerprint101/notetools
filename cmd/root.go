package cmd

import (
	"fmt"

	"github.com/fingerprint/notetools/internal/config"
	"github.com/fingerprint/notetools/internal/llm"
	"github.com/fingerprint/notetools/internal/ollama"
	opencodepkg "github.com/fingerprint/notetools/internal/opencode"
	"github.com/spf13/cobra"
)

var (
	noOverwrite bool
	ollamaHost  string
	appConfig   config.Config
)

var rootCmd = &cobra.Command{
	Use:   "nt",
	Short: "Local AI CLI for OCR, document review, and note merging",
	Long:  "nt (notetools) uses local or remote models for PDF-to-Markdown OCR and document review and note merging.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if ollamaHost != "" {
			ollama.SetHost(ollamaHost)
		}
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
	rootCmd.PersistentFlags().StringVar(&ollamaHost, "ollama-host", "", "ollama base URL (overrides OLLAMA_HOST env var)")
}

func Execute() error {
	return rootCmd.Execute()
}

func providerFor(cmdName string) (llm.Provider, string) {
	cc := config.GetCommandConfig(appConfig, cmdName)
	switch cc.Provider {
	case "opencode":
		return opencodepkg.New(), cc.Model
	default:
		return ollama.New(), cc.Model
	}
}

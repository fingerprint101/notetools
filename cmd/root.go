package cmd

import (
	"github.com/fingerprint/notetools/internal/ollama"
	"github.com/spf13/cobra"
)

var (
	noOverwrite bool
	ollamaHost  string
)

var rootCmd = &cobra.Command{
	Use:   "nt",
	Short: "Local AI CLI for OCR, document review, and note merging",
	Long:  "nt (notetools) uses ollama for PDF-to-Markdown OCR (glm-ocr) and document review and note merging (gemma4:e4b).",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if ollamaHost != "" {
			ollama.SetHost(ollamaHost)
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

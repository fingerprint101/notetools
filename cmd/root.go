package cmd

import (
	"github.com/spf13/cobra"
)

var (
	verbose     bool
	noOverwrite bool
	gpuLayers   int
)

var rootCmd = &cobra.Command{
	Use:   "nt",
	Short: "Local AI CLI for OCR, document review, and note merging",
	Long:  "nt (notetools) uses llama.cpp for PDF-to-Markdown OCR and Claude Code for document review and note merging.",
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "show llama.cpp output")
	rootCmd.PersistentFlags().BoolVar(&noOverwrite, "no-overwrite", false, "skip if output file already exists")
	rootCmd.PersistentFlags().IntVar(&gpuLayers, "gpu-layers", -1, "number of layers to offload to GPU (-1 = all)")
}

func Execute() error {
	return rootCmd.Execute()
}

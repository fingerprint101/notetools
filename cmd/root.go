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
	Use:   "notetools",
	Short: "Local AI CLI for OCR, transcription, and document review",
	Long:  "notetools uses llama.cpp to run GLM-OCR and Voxtral locally for PDF-to-Markdown conversion, audio transcription, and document review via Claude.",
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "show llama.cpp output")
	rootCmd.PersistentFlags().BoolVar(&noOverwrite, "no-overwrite", false, "skip if output file already exists")
	rootCmd.PersistentFlags().IntVar(&gpuLayers, "gpu-layers", -1, "number of layers to offload to GPU (-1 = all)")
}

func Execute() error {
	return rootCmd.Execute()
}

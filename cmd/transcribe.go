package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fingerprint/notetools/internal/llama"
	"github.com/spf13/cobra"
)

var supportedAudioFormats = map[string]bool{
	".wav": true,
	".mp3": true,
	".m4a": true,
}

var transcribeCmd = &cobra.Command{
	Use:   "transcribe <audio>",
	Short: "Transcribe an audio file to Markdown using Voxtral",
	Args:  cobra.ExactArgs(1),
	RunE:  runTranscribe,
}

func init() {
	rootCmd.AddCommand(transcribeCmd)
}

func runTranscribe(cmd *cobra.Command, args []string) error {
	audioPath := args[0]
	if _, err := os.Stat(audioPath); err != nil {
		return fmt.Errorf("file not found: %s", audioPath)
	}

	ext := strings.ToLower(filepath.Ext(audioPath))
	if !supportedAudioFormats[ext] {
		return fmt.Errorf("unsupported audio format '%s'; supported: .wav, .mp3, .m4a", ext)
	}

	stem := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	outputPath := filepath.Join(filepath.Dir(audioPath), stem+"_transcript.md")
	if noOverwrite {
		if _, err := os.Stat(outputPath); err == nil {
			fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outputPath)
			return nil
		}
	}

	if !llama.Voxtral.IsReady() {
		return fmt.Errorf("Voxtral model not found; run 'notetools models pull' first")
	}

	fmt.Fprintf(os.Stderr, "Transcribing %s with Voxtral...\n", filepath.Base(audioPath))
	out, err := llama.Run(llama.RunOpts{
		ModelPath:  llama.Voxtral.Model.LocalPath(),
		MMProjPath: llama.Voxtral.MMProjLocalPath(),
		Prompt:     "Transcribe this audio.",
		AudioPath:  audioPath,
		GPULayers:  gpuLayers,
		Verbose:    verbose,
	})
	if err != nil {
		return fmt.Errorf("transcription failed: %w", err)
	}

	md := fmt.Sprintf("# Transcript: %s\n\n%s", filepath.Base(audioPath), strings.TrimSpace(out))
	if err := os.WriteFile(outputPath, []byte(md), 0o644); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Written: %s\n", outputPath)
	return nil
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fingerprint/notetools/internal/notes"
	"github.com/spf13/cobra"
)

var cleanOutput string

var cleanCmd = &cobra.Command{
	Use:     "clean <transcript>",
	Aliases: []string{"c"},
	Short:   "Clean a raw transcript: section it and rewrite each section",
	Long: `Takes a raw transcript file (plain text or Markdown), splits it into
thematic sections, then cleans each section individually
to produce a coherent, readable Markdown document.`,
	Args: cobra.ExactArgs(1),
	RunE: runClean,
}

func init() {
	cleanCmd.Flags().StringVarP(&cleanOutput, "output", "o", "", "output file path (default: {stem}_cleaned.md)")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	inputPath := args[0]
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("file not found: %s", inputPath)
	}

	transcript := strings.TrimSpace(string(content))
	if transcript == "" {
		return fmt.Errorf("input transcript is empty: %s", inputPath)
	}

	stem := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := cleanOutput
	if outputPath == "" {
		outputPath = filepath.Join(filepath.Dir(inputPath), stem+"_cleaned.md")
	}

	if noOverwrite {
		if _, err := os.Stat(outputPath); err == nil {
			fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outputPath)
			return nil
		}
	}

	p, model := providerFor("clean")

	fmt.Fprintf(os.Stderr, "Splitting transcript into sections...\n")
	sections, err := notes.SectionTranscript(cmd.Context(), p, model, transcript)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Got %d sections.\n", len(sections))

	cleaned := make([]notes.TranscriptSection, 0, len(sections))
	for i, s := range sections {
		fmt.Fprintf(os.Stderr, "Cleaning section %d/%d: %s\n", i+1, len(sections), s.Title)
		text, err := notes.CleanSection(cmd.Context(), p, model, s.Title, s.Content)
		if err != nil {
			return err
		}
		cleaned = append(cleaned, notes.TranscriptSection{Title: s.Title, Content: text})
	}

	docTitle := stem + " - cleaned transcript per section"
	result := notes.RenderCleanMarkdown(docTitle, cleaned)

	if err := os.WriteFile(outputPath, []byte(result), 0o644); err != nil {
		return err
	}

	fmt.Print(result)
	fmt.Fprintf(os.Stderr, "Written: %s\n", outputPath)
	return nil
}

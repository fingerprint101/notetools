package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fingerprint/notetools/internal/notes"
	"github.com/spf13/cobra"
)

var summarizeOutput string

var summarizeCmd = &cobra.Command{
	Use:     "summarize <note>[:<start>-<end>]",
	Aliases: []string{"s"},
	Short:   "Summarize notes in a schematic Markdown format",
	Long: `Summarize a note, or a selected line range from a note, into a
schematic Markdown summary that touches every topic discussed without becoming
discursive prose.`,
	Args: cobra.ExactArgs(1),
	RunE: runSummarize,
}

func init() {
	summarizeCmd.Flags().StringVarP(&summarizeOutput, "output", "o", "", "output file path (default: {stem}_summary.md)")
	rootCmd.AddCommand(summarizeCmd)
}

func runSummarize(cmd *cobra.Command, args []string) error {
	inputPath, start, end, err := parseFileArg(args[0])
	if err != nil {
		return err
	}

	content, err := readLines(inputPath, start, end)
	if err != nil {
		return err
	}

	note := strings.TrimSpace(content)
	if note == "" {
		return fmt.Errorf("input note is empty: %s", rangeDesc(inputPath, start, end))
	}

	stem := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := summarizeOutput
	if outputPath == "" {
		outputPath = filepath.Join(filepath.Dir(inputPath), stem+"_summary.md")
	}

	if noOverwrite {
		if _, err := os.Stat(outputPath); err == nil {
			fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outputPath)
			return nil
		}
	}

	p, model := providerFor("summarize")
	fmt.Fprintf(os.Stderr, "Summarizing with %s (%s)...\n", p.Name(), model)
	result, err := notes.Summarize(cmd.Context(), p, model, note)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, []byte(result), 0o644); err != nil {
		return err
	}

	fmt.Print(result)
	fmt.Fprintf(os.Stderr, "Written: %s\n", outputPath)
	return nil
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fingerprint/notetools/internal/merge"
	"github.com/spf13/cobra"
)

var (
	mergeOutput       string
	mergeInstructions string
)

var mergeCmd = &cobra.Command{
	Use:     "merge <file1>[:<start>-<end>] <file2>[:<start>-<end>]",
	Aliases: []string{"m"},
	Short:   "Merge two note snippets using Claude",
	Long:    "Merge portions of two Markdown notes into a single document. All details from both snippets are preserved. Use line ranges (e.g. file.md:10-50) to select specific sections.",
	Args:    cobra.ExactArgs(2),
	RunE:    runMerge,
}

func init() {
	mergeCmd.Flags().StringVarP(&mergeOutput, "output", "o", "", "output file path (default: {stem1}_{stem2}_merged.md)")
	mergeCmd.Flags().StringVarP(&mergeInstructions, "instructions", "i", "", "additional instructions for the merge")
	rootCmd.AddCommand(mergeCmd)
}

func runMerge(cmd *cobra.Command, args []string) error {
	path1, start1, end1, err := parseFileArg(args[0])
	if err != nil {
		return err
	}
	path2, start2, end2, err := parseFileArg(args[1])
	if err != nil {
		return err
	}

	snippet1, err := readLines(path1, start1, end1)
	if err != nil {
		return err
	}
	snippet2, err := readLines(path2, start2, end2)
	if err != nil {
		return err
	}

	outputPath := mergeOutput
	if outputPath == "" {
		stem1 := strings.TrimSuffix(filepath.Base(path1), filepath.Ext(path1))
		stem2 := strings.TrimSuffix(filepath.Base(path2), filepath.Ext(path2))
		outputPath = filepath.Join(filepath.Dir(path1), stem1+"_"+stem2+"_merged.md")
	}

	if noOverwrite {
		if _, err := os.Stat(outputPath); err == nil {
			fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outputPath)
			return nil
		}
	}

	fmt.Fprintf(os.Stderr, "Merging with Claude...\n")
	result, err := merge.RunClaude(snippet1, snippet2, mergeInstructions)
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

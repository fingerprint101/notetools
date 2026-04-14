package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fingerprint/notetools/internal/merge"
	"github.com/spf13/cobra"
)

const (
	ansiRed   = "\033[31m"
	ansiGreen = "\033[32m"
	ansiReset = "\033[0m"
)

var (
	mergeOutput       string
	mergeInstructions string
)

var mergeCmd = &cobra.Command{
	Use:     "merge <file1>[:<start>-<end>] <file2>[:<start>-<end>]",
	Aliases: []string{"m"},
	Short:   "Merge two note snippets",
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

	p, model := providerFor("merge")

	inPlace := !cmd.Flags().Changed("output")

	if !inPlace {
		outputPath := mergeOutput
		if noOverwrite {
			if _, err := os.Stat(outputPath); err == nil {
				fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outputPath)
				return nil
			}
		}

		fmt.Fprintf(os.Stderr, "Merging with %s (%s)...\n", p, model)
		result, err := merge.Run(cmd.Context(), p, model, snippet1, snippet2, mergeInstructions)
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

	fmt.Fprintf(os.Stderr, "Merging with %s (%s)...\n", p, model)
	result, err := merge.Run(cmd.Context(), p, model, snippet1, snippet2, mergeInstructions)
	if err != nil {
		return err
	}

	rangeLabel := rangeDesc(path2, start2, end2)
	fmt.Printf("--- %s\n+++ merged\n", rangeLabel)
	for _, line := range strings.Split(snippet2, "\n") {
		fmt.Printf("%s- %s%s\n", ansiRed, line, ansiReset)
	}
	for _, line := range strings.Split(result, "\n") {
		fmt.Printf("%s+ %s%s\n", ansiGreen, line, ansiReset)
	}

	fmt.Print("\nAccept changes? [y/N]: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(scanner.Text())
	if answer != "y" && answer != "Y" {
		fmt.Fprintln(os.Stderr, "Aborted.")
		return nil
	}

	if err := replaceLines(path2, start2, end2, result); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Updated: %s\n", rangeLabel)
	return nil
}

func rangeDesc(path string, start, end int) string {
	if start == 0 && end == 0 {
		return path
	}
	if start == end {
		return fmt.Sprintf("%s (line %d)", path, start)
	}
	return fmt.Sprintf("%s (lines %d-%d)", path, start, end)
}

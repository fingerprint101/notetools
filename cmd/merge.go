package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fingerprint/notetools/internal/notes"
	"github.com/spf13/cobra"
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

	if !inPlace && noOverwrite {
		if _, err := os.Stat(mergeOutput); err == nil {
			fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", mergeOutput)
			return nil
		}
	}

	fmt.Fprintf(os.Stderr, "Merging with %s (%s)...\n", p.Name(), model)
	result, err := notes.Merge(cmd.Context(), p, model, snippet1, snippet2, mergeInstructions)
	if err != nil {
		return err
	}

	if !inPlace {
		if err := os.WriteFile(mergeOutput, []byte(result), 0o644); err != nil {
			return err
		}
		fmt.Print(result)
		fmt.Fprintf(os.Stderr, "Written: %s\n", mergeOutput)
		return nil
	}

	rangeLabel := rangeDesc(path2, start2, end2)
	printMarkdownDiff(rangeLabel, "merged", snippet2, result)

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

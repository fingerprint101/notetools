package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fingerprint/notetools/internal/notes"
	"github.com/spf13/cobra"
)

var (
	mergeOutput       string
	mergeInstructions string
	mergeTokenBudget  int
)

var mergeCmd = &cobra.Command{
	Use:     "merge <file1>[:<start>-<end>] <file2>[:<start>-<end>]",
	Aliases: []string{"m"},
	Short:   "Merge two Markdown notes or snippets",
	Long: `Merge two Markdown notes.

When either argument includes a line range, merge the selected snippets interactively.
When neither argument includes a range, plan and execute a full-document merge from
SOURCE into TARGET, updating TARGET in place unless --output is provided.`,
	Args: cobra.ExactArgs(2),
	RunE: runMerge,
}

func init() {
	mergeCmd.Flags().StringVarP(&mergeOutput, "output", "o", "", "write merged output to this path instead of updating the second file/range in place")
	mergeCmd.Flags().StringVarP(&mergeInstructions, "instructions", "i", "", "additional instructions for the merge")
	mergeCmd.Flags().IntVar(&mergeTokenBudget, "token-budget", notes.DefaultPlanTokenBudget, "estimated prompt token budget for full-document planning requests")
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

	if start1 == 0 && end1 == 0 && start2 == 0 && end2 == 0 {
		return runFullMerge(cmd, path1, path2)
	}
	return runSnippetMerge(cmd, path1, start1, end1, path2, start2, end2)
}

func runFullMerge(cmd *cobra.Command, sourcePath, targetPath string) error {
	if !strings.HasSuffix(sourcePath, ".md") || !strings.HasSuffix(targetPath, ".md") {
		return fmt.Errorf("both input files must be Markdown (.md) files")
	}

	sourceContent, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", sourcePath, err)
	}
	targetContent, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", targetPath, err)
	}

	outputPath := targetPath
	if cmd.Flags().Changed("output") {
		outputPath = mergeOutput
		if noOverwrite {
			if _, err := os.Stat(outputPath); err == nil {
				fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outputPath)
				return nil
			}
		}
		if err := os.WriteFile(outputPath, targetContent, 0o644); err != nil {
			return fmt.Errorf("write output target copy: %w", err)
		}
	}

	planProvider, planModel := providerFor("plan")
	fmt.Fprintf(os.Stderr, "Planning merge with %s (%s)...\n", planProvider.Name(), planModel)
	mappings, err := notes.PlanWithTokenBudget(cmd.Context(), planProvider, planModel, string(sourceContent), string(targetContent), mergeTokenBudget)
	if err != nil {
		return err
	}

	absSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}
	absTargetPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	doc := notes.NewPlanDocument(absSourcePath, absTargetPath, mappings)

	executeProvider, executeModel := providerFor("execute")
	fmt.Fprintf(os.Stderr, "Executing merge with %s (%s)...\n", executeProvider.Name(), executeModel)
	beforeTarget := string(targetContent)
	err = notes.ExecutePlan(cmd.Context(), executeProvider, executeModel, doc, mergeInstructions, func(progress notes.ExecuteProgress) {
		if progress.PresentInDst {
			fmt.Fprintf(
				os.Stderr,
				"[%d/%d] Merging %q: source %d-%d into target %d-%d\n",
				progress.Step,
				progress.Total,
				progress.Title,
				progress.SourceStart,
				progress.SourceEnd,
				progress.TargetStart,
				progress.TargetEnd,
			)
			return
		}

		location := "end of file"
		if progress.InsertAfter > 0 {
			location = fmt.Sprintf("after line %d", progress.InsertAfter)
		}
		fmt.Fprintf(
			os.Stderr,
			"[%d/%d] Inserting %q: source %d-%d %s\n",
			progress.Step,
			progress.Total,
			progress.Title,
			progress.SourceStart,
			progress.SourceEnd,
			location,
		)
	})
	if err != nil {
		return err
	}

	afterTarget, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("read target after merge: %w", err)
	}
	printMarkdownDiff(outputPath+" (before)", outputPath+" (after)", beforeTarget, string(afterTarget))
	fmt.Fprintf(os.Stderr, "Updated: %s\n", outputPath)
	return nil
}

func runSnippetMerge(cmd *cobra.Command, path1 string, start1, end1 int, path2 string, start2, end2 int) error {
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

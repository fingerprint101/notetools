package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fingerprint/notetools/internal/plan"
	"github.com/spf13/cobra"
)

var planOutput string

var planCmd = &cobra.Command{
	Use:   "plan <source.md> <target.md>",
	Short: "Plan the merging of two Markdown notes",
	Long: `Plan how to merge SOURCE into TARGET.

For each section in SOURCE, the plan identifies:
- The line range in TARGET that covers the same content, or
- The line in TARGET after which the missing content should be inserted.

The plan is written to a Markdown file (default: plan-<source>-<target>.md).`,
	Args: cobra.ExactArgs(2),
	RunE: runPlan,
}

func init() {
	planCmd.Flags().StringVarP(&planOutput, "output", "o", "", "output file path (default: plan-<source>-<target>.md)")
	rootCmd.AddCommand(planCmd)
}

func runPlan(cmd *cobra.Command, args []string) error {
	path1 := args[0]
	path2 := args[1]

	if !strings.HasSuffix(path1, ".md") || !strings.HasSuffix(path2, ".md") {
		return fmt.Errorf("both input files must be Markdown (.md) files")
	}

	content1, err := os.ReadFile(path1)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", path1, err)
	}
	content2, err := os.ReadFile(path2)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", path2, err)
	}

	outPath := planOutput
	if outPath == "" {
		stem1 := strings.TrimSuffix(filepath.Base(path1), ".md")
		stem2 := strings.TrimSuffix(filepath.Base(path2), ".md")
		outPath = fmt.Sprintf("plan-%s-%s.md", stem1, stem2)
	}

	if noOverwrite {
		if _, err := os.Stat(outPath); err == nil {
			fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outPath)
			return nil
		}
	}

	p, model := providerFor("plan")
	fmt.Fprintf(os.Stderr, "Planning with %s (%s)...\n", p, model)

	mappings, err := plan.Run(cmd.Context(), p, model, string(content1), string(content2))
	if err != nil {
		return err
	}

	file2Lines := strings.Split(string(content2), "\n")
	file1Name := filepath.Base(path1)
	file2Name := filepath.Base(path2)

	md := plan.RenderMarkdown(file1Name, file2Name, mappings, file2Lines)

	if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Written: %s\n", outPath)
	fmt.Print(md)
	return nil
}

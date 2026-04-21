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

The output is a machine-readable JSON plan for the execute command.
For each section in SOURCE, it records either:
- The line range in TARGET that covers the same content, or
- The line in TARGET after which the missing content should be inserted.

The plan is written to a JSON file (default: plan-<source>-<target>.json).`,
	Args: cobra.ExactArgs(2),
	RunE: runPlan,
}

func init() {
	planCmd.Flags().StringVarP(&planOutput, "output", "o", "", "output file path (default: plan-<source>-<target>.json)")
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
		outPath = fmt.Sprintf("plan-%s-%s.json", stem1, stem2)
	}

	if noOverwrite {
		if _, err := os.Stat(outPath); err == nil {
			fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outPath)
			return nil
		}
	}

	p, model := providerFor("plan")
	fmt.Fprintf(os.Stderr, "Planning with %s (%s)...\n", p.Name(), model)

	mappings, err := plan.Run(cmd.Context(), p, model, string(content1), string(content2))
	if err != nil {
		return err
	}

	absPath1, err := filepath.Abs(path1)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}
	absPath2, err := filepath.Abs(path2)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}

	doc := plan.NewDocument(absPath1, absPath2, mappings)
	output, err := plan.Render(doc)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outPath, []byte(output), 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Written: %s\n", outPath)
	fmt.Print(output)
	return nil
}

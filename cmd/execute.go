package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fingerprint/notetools/internal/execute"
	"github.com/fingerprint/notetools/internal/plan"
	"github.com/spf13/cobra"
)

var executeInstructions string

var executeCmd = &cobra.Command{
	Use:   "execute <plan.json>",
	Short: "Execute each merge described by a plan file",
	Long: `Execute the JSON merge plan produced by "nt plan".

The command reads the machine-readable plan file, then
applies each merge to the target note in sequence while reporting progress.`,
	Args: cobra.ExactArgs(1),
	RunE: runExecute,
}

func init() {
	executeCmd.Flags().StringVarP(&executeInstructions, "instructions", "i", "", "additional instructions for each merge")
	rootCmd.AddCommand(executeCmd)
}

func runExecute(cmd *cobra.Command, args []string) error {
	planPath := args[0]
	content, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", planPath, err)
	}

	doc, err := plan.Parse(string(content))
	if err != nil {
		return fmt.Errorf("parse plan file: %w", err)
	}

	p, model := providerFor("merge")
	fmt.Fprintf(os.Stderr, "Executing plan %s with %s (%s)...\n", filepath.Base(planPath), p.Name(), model)

	err = execute.Run(cmd.Context(), p, model, doc, executeInstructions, func(progress execute.Progress) {
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

	fmt.Fprintf(os.Stderr, "Updated: %s\n", doc.TargetPath)
	return nil
}

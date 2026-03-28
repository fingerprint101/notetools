package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fingerprint/notetools/internal/review"
	"github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
	Use:     "review <markdown>",
	Aliases: []string{"r"},
	Short:   "Review a Markdown document for issues using Claude",
	Args:    cobra.ExactArgs(1),
	RunE:    runReview,
}

func init() {
	rootCmd.AddCommand(reviewCmd)
}

func runReview(cmd *cobra.Command, args []string) error {
	mdPath := args[0]
	content, err := os.ReadFile(mdPath)
	if err != nil {
		return fmt.Errorf("file not found: %s", mdPath)
	}

	stem := strings.TrimSuffix(filepath.Base(mdPath), filepath.Ext(mdPath))
	outputPath := filepath.Join(filepath.Dir(mdPath), stem+"_review.md")
	if noOverwrite {
		if _, err := os.Stat(outputPath); err == nil {
			fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outputPath)
			return nil
		}
	}

	fmt.Fprintf(os.Stderr, "Reviewing with Claude...\n")
	result, err := review.RunClaude(string(content))
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

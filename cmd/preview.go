package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var previewCmd = &cobra.Command{
	Use:     "preview <file>[:<start>-<end>]",
	Aliases: []string{"p"},
	Short:   "Preview a file with line numbers",
	Long:    "Display file contents with line numbers. Use a range (e.g. file.md:10-30) to show specific lines. Useful for finding line ranges before merging.",
	Args:    cobra.ExactArgs(1),
	RunE:    runPreview,
}

func init() {
	rootCmd.AddCommand(previewCmd)
}

func runPreview(cmd *cobra.Command, args []string) error {
	path, start, end, err := parseFileArg(args[0])
	if err != nil {
		return err
	}

	content, err := readLines(path, start, end)
	if err != nil {
		return err
	}

	lines := strings.Split(content, "\n")
	lineOffset := 1
	if start > 0 {
		lineOffset = start
	}

	for i, line := range lines {
		fmt.Printf("%6d\t%s\n", lineOffset+i, line)
	}

	return nil
}

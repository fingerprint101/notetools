package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/fingerprint/notetools/internal/notes"
	"github.com/spf13/cobra"
)

var checkTokenBudget int

var checkCmd = &cobra.Command{
	Use:     "check <source>... <target>",
	Aliases: []string{"ck"},
	Short:   "Check whether a target contains all information from source files",
	Long: `Check whether the last argument contains all substantive information from
the preceding source files. Arguments may be whole files or file ranges such as
notes.md:10-80.`,
	Args: cobra.MinimumNArgs(2),
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().IntVar(&checkTokenBudget, "token-budget", notes.DefaultCheckTokenBudget, "estimated prompt token budget for coverage-check requests")
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	sourceArgs := args[:len(args)-1]
	targetArg := args[len(args)-1]

	sources := make([]notes.CheckSource, 0, len(sourceArgs))
	for _, arg := range sourceArgs {
		path, start, end, err := parseFileArg(arg)
		if err != nil {
			return err
		}
		content, err := readLines(path, start, end)
		if err != nil {
			return err
		}
		sources = append(sources, notes.CheckSource{
			Path:       path,
			Content:    content,
			LineOffset: checkLineOffset(start),
		})
	}

	targetPath, targetStart, targetEnd, err := parseFileArg(targetArg)
	if err != nil {
		return err
	}
	targetContent, err := readLines(targetPath, targetStart, targetEnd)
	if err != nil {
		return err
	}

	p, model := providerFor("check")
	fmt.Fprintf(os.Stderr, "Checking coverage with %s (%s)...\n", p.Name(), model)
	missing, err := notes.CheckCoverage(cmd.Context(), p, model, sources, targetContent, checkTokenBudget, func(progress notes.CheckProgress) {
		label := progress.SourcePath
		if label == "" {
			label = "sources"
		}
		fmt.Fprintf(os.Stderr, "[%d/%d] Checking %d section(s) from %s\n", progress.Step, progress.Total, progress.Sections, label)
	})
	if err != nil {
		return err
	}

	fmt.Print(renderCheckReport(missing))
	return nil
}

func checkLineOffset(start int) int {
	if start <= 0 {
		return 0
	}
	return start - 1
}

func renderCheckReport(missing []notes.MissingInfo) string {
	if len(missing) == 0 {
		return "nothing to report\n"
	}

	var b strings.Builder
	b.WriteString("# Missing information report\n\n")
	for _, item := range missing {
		title := item.Title
		if title == "" {
			title = "Untitled section"
		}
		fmt.Fprintf(&b, "## %s\n", title)
		fmt.Fprintf(&b, "- Source: %s:%d-%d\n", item.SourcePath, item.StartLine, item.EndLine)
		if item.Summary != "" {
			fmt.Fprintf(&b, "- Missing details: %s\n", item.Summary)
		}
		b.WriteString("\n")
	}
	return b.String()
}

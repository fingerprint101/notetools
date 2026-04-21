package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fingerprint/notetools/internal/docs"
	"github.com/spf13/cobra"
)

var explainOutput string

var explainCmd = &cobra.Command{
	Use:     "explain <pdf>",
	Aliases: []string{"e"},
	Short:   "Explain a PDF: identify sections and explain each page",
	Long: `Takes a PDF document, identifies coherent thematic sections by page range,
then explains each page individually. Outputs a single Markdown file with
all explanations chained together.`,
	Args: cobra.ExactArgs(1),
	RunE: runExplain,
}

func init() {
	explainCmd.Flags().StringVarP(&explainOutput, "output", "o", "", "output file path (default: {stem}_explained.md)")
	rootCmd.AddCommand(explainCmd)
}

func runExplain(cmd *cobra.Command, args []string) error {
	pdfPath := args[0]
	if _, err := os.Stat(pdfPath); err != nil {
		return fmt.Errorf("file not found: %s", pdfPath)
	}

	stem := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
	outputPath := explainOutput
	if outputPath == "" {
		outputPath = filepath.Join(filepath.Dir(pdfPath), stem+"_explained.md")
	}

	if noOverwrite {
		if _, err := os.Stat(outputPath); err == nil {
			fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outputPath)
			return nil
		}
	}

	p, model := providerFor("explain")

	fmt.Fprintf(os.Stderr, "Identifying sections in %s...\n", pdfPath)
	sections, err := docs.IdentifySections(cmd.Context(), p, model, pdfPath)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Found %d sections.\n", len(sections))

	fmt.Fprintf(os.Stderr, "Rendering PDF pages at 150 DPI...\n")
	pages, err := docs.RenderPages(pdfPath, 150)
	if err != nil {
		return err
	}
	defer func() {
		if len(pages) > 0 {
			os.RemoveAll(filepath.Dir(pages[0]))
		}
	}()

	explained := make([]docs.SectionWithExplanation, 0, len(sections))
	for i, s := range sections {
		fmt.Fprintf(os.Stderr, "  Section %d/%d: %s (pages %d-%d)\n", i+1, len(sections), s.Title, s.StartPage, s.EndPage)

		var sectionPages []string
		for pg := s.StartPage; pg <= s.EndPage; pg++ {
			if pg < 1 || pg > len(pages) {
				fmt.Fprintf(os.Stderr, "    Warning: page %d out of range, skipping\n", pg)
				continue
			}
			sectionPages = append(sectionPages, pages[pg-1])
		}

		fmt.Fprintf(os.Stderr, "    Explaining %d page(s)...\n", len(sectionPages))
		exp, err := docs.ExplainSection(cmd.Context(), p, model, sectionPages, s.Title, s.StartPage, s.EndPage)
		if err != nil {
			return err
		}

		explained = append(explained, docs.SectionWithExplanation{
			Section:     s,
			Explanation: exp,
		})
	}

	docTitle := stem + " - explained"
	result := docs.RenderExplainMarkdown(docTitle, explained)

	if err := os.WriteFile(outputPath, []byte(result), 0o644); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Written: %s\n", outputPath)
	return nil
}

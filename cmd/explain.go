package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fingerprint/notetools/internal/docs"
	"github.com/spf13/cobra"
)

var (
	explainOutput   string
	explainLanguage string
)

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
	explainCmd.Flags().StringVarP(&explainOutput, "output", "o", "", "output file path (default: {stem}.md)")
	explainCmd.Flags().StringVarP(&explainLanguage, "language", "l", "", "target language for the generated explanation (default: same as document)")
	rootCmd.AddCommand(explainCmd)
}

func runExplain(cmd *cobra.Command, args []string) error {
	pdfPath := args[0]
	if _, err := os.Stat(pdfPath); err != nil {
		return fmt.Errorf("file not found: %s", pdfPath)
	}
	targetLanguage := strings.TrimSpace(explainLanguage)

	stem := strings.TrimSuffix(filepath.Base(pdfPath), filepath.Ext(pdfPath))
	outputPath := explainOutput
	if outputPath == "" {
		outputPath = filepath.Join(filepath.Dir(pdfPath), stem+".md")
	}

	if noOverwrite {
		if _, err := os.Stat(outputPath); err == nil {
			fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outputPath)
			return nil
		}
	}

	p, model := providerFor("explain")
	if targetLanguage != "" {
		fmt.Fprintf(os.Stderr, "Target language: %s\n", targetLanguage)
	}

	fmt.Fprintf(os.Stderr, "Rendering PDF pages at 120 DPI...\n")
	pages, err := docs.RenderPages(pdfPath, 120)
	if err != nil {
		return err
	}
	defer func() {
		if len(pages) > 0 {
			os.RemoveAll(filepath.Dir(pages[0]))
		}
	}()

	fmt.Fprintf(os.Stderr, "Identifying sections in %s...\n", pdfPath)
	sections, err := docs.IdentifySections(cmd.Context(), p, model, pdfPath, len(pages))
	if err != nil {
		return err
	}
	if err := docs.ValidateSections(sections, len(pages)); err != nil {
		return fmt.Errorf("invalid section plan: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Found %d sections.\n", len(sections))

	explained := make([]docs.SectionWithExplanation, 0, len(sections))
	imageDir := filepath.Join(filepath.Dir(outputPath), "Images")
	imageRelDir := "Images"
	for i, s := range sections {
		fmt.Fprintf(os.Stderr, "  Section %d/%d: %s (pages %d-%d)\n", i+1, len(sections), s.Title, s.StartPage, s.EndPage)

		var sectionPages []string
		for pg := s.StartPage; pg <= s.EndPage; pg++ {
			sectionPages = append(sectionPages, pages[pg-1])
		}

		fmt.Fprintf(os.Stderr, "    Explaining %d page(s)...\n", len(sectionPages))
		priorContext := docs.BuildPriorSectionContext(explained, 1500)
		exp, err := docs.ExplainSection(cmd.Context(), p, model, sectionPages, s.Title, s.StartPage, s.EndPage, i+1, targetLanguage, priorContext)
		if err != nil {
			return err
		}

		markdown := exp.Markdown
		var warnings []string
		markdown, warnings = docs.MaterializeCrops(cmd.Context(), markdown, exp.Crops, sectionPages, imageDir, imageRelDir)
		for _, warning := range warnings {
			fmt.Fprintf(os.Stderr, "    Warning: %s\n", warning)
		}

		explained = append(explained, docs.SectionWithExplanation{
			Section:     s,
			Explanation: markdown,
			Memory:      exp.Memory,
		})
	}

	result := docs.RenderExplainMarkdown(stem, explained)

	if err := os.WriteFile(outputPath, []byte(result), 0o644); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Written: %s\n", outputPath)
	return nil
}

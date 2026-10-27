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
	explainNoImg    bool
	explainDetail   int
)

var explainCmd = &cobra.Command{
	Use:     "explain <pdf>",
	Aliases: []string{"e"},
	Short:   "Turn a PDF into study notes with adjustable detail",
	Long: `Takes a PDF document, identifies coherent thematic sections by page range,
then explains the concepts at the selected level of detail. Outputs a single
Markdown file organized by section.

Detail levels (--detail / -d):
  1  Concise study notes with core concepts and brief supporting details (default).
  2  The same material explained in greater depth.
  3  The same material explained with the fullest depth and reasoning.`,
	Args: cobra.ExactArgs(1),
	RunE: runExplain,
}

func init() {
	explainCmd.Flags().StringVarP(&explainOutput, "output", "o", "", "output file path (default: {stem}.md)")
	explainCmd.Flags().StringVarP(&explainLanguage, "language", "l", "auto", "output language for all notes and titles (e.g. Italian, English; auto detects once per document)")
	explainCmd.Flags().BoolVar(&explainNoImg, "noimg", false, "skip cropped images in the output Markdown")
	explainCmd.Flags().IntVarP(&explainDetail, "detail", "d", 1, "explanation detail: 1 concise, 2 detailed, 3 thorough")
	rootCmd.AddCommand(explainCmd)
}

func runExplain(cmd *cobra.Command, args []string) error {
	if err := docs.ValidateExplainDetail(explainDetail); err != nil {
		return err
	}
	pdfPath := args[0]
	if _, err := os.Stat(pdfPath); err != nil {
		return fmt.Errorf("file not found: %s", pdfPath)
	}
	targetLanguage := strings.TrimSpace(explainLanguage)
	if targetLanguage == "" {
		return fmt.Errorf("language must not be empty; use --language auto to detect it from the document")
	}

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

	fmt.Fprintf(os.Stderr, "Identifying sections in %s...\n", pdfPath)
	plan, err := docs.IdentifySections(cmd.Context(), p, model, pdfPath, len(pages), targetLanguage)
	if err != nil {
		return err
	}
	sections := plan.Sections
	targetLanguage = plan.Language
	fmt.Fprintf(os.Stderr, "Output language: %s\n", targetLanguage)
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
		priorContext := docs.BuildPriorSectionContext(explained, 3500)
		exp, err := docs.ExplainSection(cmd.Context(), p, model, sectionPages, s.Title, s.StartPage, s.EndPage, i+1, explainDetail, !explainNoImg, targetLanguage, priorContext)
		if err != nil {
			return err
		}

		markdown := exp.Markdown
		if explainNoImg {
			markdown = docs.RemoveImagePlaceholders(markdown)
		} else {
			var warnings []string
			markdown, warnings = docs.MaterializeCrops(cmd.Context(), markdown, exp.Crops, sectionPages, imageDir, imageRelDir)
			for _, warning := range warnings {
				fmt.Fprintf(os.Stderr, "    Warning: %s\n", warning)
			}
		}

		explained = append(explained, docs.SectionWithExplanation{
			Section:     s,
			Explanation: markdown,
		})
	}

	result := docs.RenderExplainMarkdown(plan.DocumentTitle, explained)

	if err := os.WriteFile(outputPath, []byte(result), 0o644); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Written: %s\n", outputPath)
	return nil
}

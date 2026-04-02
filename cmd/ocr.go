package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fingerprint/notetools/internal/ollama"
	"github.com/fingerprint/notetools/internal/pdf"
	"github.com/spf13/cobra"
)

const ocrModel = "glm-ocr"

const ocrPrompt = "Convert this document page to Markdown. " +
	"Preserve all headings, lists, tables, and code blocks. " +
	"Output only Markdown, no commentary."

var ocrCmd = &cobra.Command{
	Use:     "ocr <pdf>",
	Aliases: []string{"o"},
	Short:   "Convert a PDF to Markdown using GLM-OCR via ollama",
	Args:    cobra.ExactArgs(1),
	RunE:    runOCR,
}

func init() {
	rootCmd.AddCommand(ocrCmd)
}

func runOCR(cmd *cobra.Command, args []string) error {
	pdfPath := args[0]
	if _, err := os.Stat(pdfPath); err != nil {
		return fmt.Errorf("file not found: %s", pdfPath)
	}

	ext := filepath.Ext(pdfPath)
	outputPath := strings.TrimSuffix(pdfPath, ext) + ".md"
	if noOverwrite {
		if _, err := os.Stat(outputPath); err == nil {
			fmt.Fprintf(os.Stderr, "Skipping: output already exists: %s\n", outputPath)
			return nil
		}
	}

	fmt.Fprintf(os.Stderr, "Rendering PDF pages...\n")
	pages, err := pdf.RenderPages(pdfPath, 150)
	if err != nil {
		return err
	}
	defer func() {
		if len(pages) > 0 {
			os.RemoveAll(filepath.Dir(pages[0]))
		}
	}()

	fmt.Fprintf(os.Stderr, "Processing %d page(s) with GLM-OCR via ollama...\n", len(pages))
	results := make([]string, 0, len(pages))

	for i, pagePath := range pages {
		fmt.Fprintf(os.Stderr, "  Page %d/%d...\n", i+1, len(pages))
		out, err := ollama.GenerateWithImage(cmd.Context(), ocrModel, ocrPrompt, pagePath)
		if err != nil {
			return fmt.Errorf("OCR page %d: %w", i+1, err)
		}
		results = append(results, strings.TrimSpace(out))
	}

	output := strings.Join(results, "\n\n---\n\n")
	if err := os.WriteFile(outputPath, []byte(output), 0o644); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Written: %s\n", outputPath)
	return nil
}

package explain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fingerprint/notetools/internal/llm"
)

type Section struct {
	Title     string `json:"title"`
	StartPage int    `json:"start_page"`
	EndPage   int    `json:"end_page"`
}

type SectionWithExplanation struct {
	Section
	Explanation string
}

type sectionsResponse struct {
	Sections []Section `json:"sections"`
}

var sectionsSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]any{
		"sections": map[string]any{
			"type":     "array",
			"minItems": 1,
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"title":      map[string]any{"type": "string", "minLength": 1},
					"start_page": map[string]any{"type": "integer", "minimum": 1},
					"end_page":   map[string]any{"type": "integer", "minimum": 1},
				},
				"required": []string{"title", "start_page", "end_page"},
			},
		},
	},
	"required": []string{"sections"},
}

func IdentifySections(ctx context.Context, p llm.Provider, model, pdfPath string) ([]Section, error) {
	prompt := fmt.Sprintf(`You are analyzing a PDF document. Here is the document: %s

Task:
- Identify all coherent thematic sections in the document.
- For each section, provide its title, the starting page number, and the ending page number.
- Pages are 1-indexed.
- Sections must be contiguous, non-overlapping, and cover the entire document.
- Prefer specific, well-defined sections over broad ones.
- Give each section a concise, descriptive title.
- Reply only with valid JSON matching the required schema.`, pdfPath)

	raw, err := p.GenerateJSON(ctx, model, prompt, sectionsSchema)
	if err != nil {
		return nil, fmt.Errorf("section identification failed: %w", err)
	}

	var resp sectionsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse sections JSON: %w", err)
	}

	if len(resp.Sections) == 0 {
		return nil, fmt.Errorf("model returned an empty section list")
	}

	for i := range resp.Sections {
		resp.Sections[i].Title = strings.TrimSpace(resp.Sections[i].Title)
	}

	return resp.Sections, nil
}

func ExplainSection(ctx context.Context, p llm.Provider, model string, pagePaths []string, title string, startPage, endPage int) (string, error) {
	prompt := fmt.Sprintf(`You are preparing study notes from a section titled "%s" (pages %d-%d) of a document.
The section spans %d page(s), provided as images in order.

Your goal is to produce an EXHAUSTIVE breakdown that can be used as raw material for
hand-written notes. Err on the side of including too much rather than too little — the
reader will trim later.

For every distinct topic, concept, definition, example, formula, diagram, or claim in
the section:
- Name it with a descriptive sub-heading (use '###').
- Define terms precisely. Include formal definitions verbatim when the document provides them.
- State all claims, properties, and results the document makes about it.
- Reproduce every formula, equation, or notation exactly, using LaTeX ($...$ or $$...$$).
- Describe diagrams, tables, and figures in words, preserving labels, axes, and values.
- Work through examples step by step, preserving numbers and intermediate steps.
- Note edge cases, exceptions, assumptions, and caveats the document mentions.
- Preserve cross-references ("see section X", "as shown in figure Y") when they appear.
- Capture the author's reasoning and motivation, not just the conclusions.

Constraints:
- Do not summarize or compress — unfold each idea fully.
- Do not invent content not present on the pages. If something is unclear, say so.
- Do not add introductions, conclusions, or meta-commentary about the section.
- Use Markdown: sub-headings, bullet lists, numbered lists for procedures, fenced code for code.
- Write in the same language as the document.
- Output only the breakdown itself.`, title, startPage, endPage, len(pagePaths))

	out, err := p.GenerateWithImages(ctx, model, prompt, pagePaths)
	if err != nil {
		return "", fmt.Errorf("explain section %q: %w", title, err)
	}

	return strings.TrimSpace(out), nil
}

func RenderMarkdown(docTitle string, sections []SectionWithExplanation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", docTitle)
	for _, s := range sections {
		if s.StartPage == s.EndPage {
			fmt.Fprintf(&b, "## %s (page %d)\n\n", s.Title, s.StartPage)
		} else {
			fmt.Fprintf(&b, "## %s (pages %d-%d)\n\n", s.Title, s.StartPage, s.EndPage)
		}
		fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(s.Explanation))
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

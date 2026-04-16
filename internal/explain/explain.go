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
- Identify the major thematic sections in the document.
- For each section, provide its title, the starting page number, and the ending page number.
- Pages are 1-indexed.
- Sections must be contiguous, non-overlapping, and cover the entire document.
- Aim for broad, high-level sections: each section should cover multiple related pages. A 20-30 page document should have roughly 4-6 sections, not one per topic or per slide.
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

Your goal is to produce dense, exhaustive notes written in flowing prose. The notes should
read like a well-written textbook explanation — not like a structured outline or a list of
bullet points. A reader should be able to understand the material deeply from your notes alone.

Writing style rules (follow these strictly):
- Write in connected paragraphs. Avoid bullet lists except for short enumerations of truly
  parallel items (e.g. a list of algorithm steps, a list of named properties). Never use
  bullets as a substitute for a sentence.
- Use '###' sub-headings only to mark a genuine new sub-topic within the section. Do not
  create a sub-heading for every concept. Several related concepts can live in one paragraph
  under the same heading.
- Bold text ($**term**$) only when introducing a technical term for the first time, inline
  in a sentence (e.g. "this is called **entropy**"). Do not use bold as a label prefix
  like "**Definition:**" or "**Key point:**".
- Reproduce every formula exactly using LaTeX ($...$ inline or $$...$$ display). Introduce
  each formula in a sentence that explains what its symbols mean.
- When a diagram or figure is important, describe what it shows in plain prose and integrate
  that description into the surrounding explanation. Do not create a separate "Diagram:" section.
- Work through examples step by step in prose, including all numbers and intermediate steps.
- Capture motivation and reasoning — not just "what" but "why" and "when".

Content rules:
- Do not summarize or compress — cover every concept, claim, definition, and example fully.
- Do not invent content not present on the pages.
- Do not add introductions, conclusions, or meta-commentary.
- Write in the same language as the document.
- Output only the notes themselves.`, title, startPage, endPage, len(pagePaths))

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

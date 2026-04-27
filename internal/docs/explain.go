package docs

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

type Crop struct {
	ID         string `json:"id"`
	ImageIndex int    `json:"image_index"`
	X1         int    `json:"x1"`
	Y1         int    `json:"y1"`
	X2         int    `json:"x2"`
	Y2         int    `json:"y2"`
	AltText    string `json:"alt_text"`
}

type SectionExplanation struct {
	Markdown string `json:"explanation_markdown"`
	Crops    []Crop `json:"crops"`
}

type sectionsResponse struct {
	Sections []Section `json:"sections"`
}

var sectionExplanationSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]any{
		"explanation_markdown": map[string]any{"type": "string", "minLength": 1},
		"crops": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"id":          map[string]any{"type": "string", "pattern": "^[A-Za-z0-9_-]+$"},
					"image_index": map[string]any{"type": "integer", "minimum": 1},
					"x1":          map[string]any{"type": "integer", "minimum": 0, "maximum": 1000},
					"y1":          map[string]any{"type": "integer", "minimum": 0, "maximum": 1000},
					"x2":          map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
					"y2":          map[string]any{"type": "integer", "minimum": 1, "maximum": 1000},
					"alt_text":    map[string]any{"type": "string"},
				},
				"required": []string{"id", "image_index", "x1", "y1", "x2", "y2", "alt_text"},
			},
		},
	},
	"required": []string{"explanation_markdown", "crops"},
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

func ExplainSection(ctx context.Context, p llm.Provider, model string, pagePaths []string, title string, startPage, endPage, sectionNumber int, includeImages bool) (SectionExplanation, error) {
	prompt := buildExplainPrompt(title, startPage, endPage, len(pagePaths), sectionNumber, includeImages)

	raw, err := p.GenerateJSONWithImages(ctx, model, prompt, pagePaths, sectionExplanationSchema)
	if err != nil {
		return SectionExplanation{}, fmt.Errorf("explain section %q: %w", title, err)
	}

	var exp SectionExplanation
	if err := json.Unmarshal([]byte(raw), &exp); err != nil {
		return SectionExplanation{}, fmt.Errorf("failed to parse explanation JSON for section %q: %w", title, err)
	}

	exp.Markdown = strings.TrimSpace(exp.Markdown)
	for i := range exp.Crops {
		exp.Crops[i].ID = strings.TrimSpace(exp.Crops[i].ID)
		exp.Crops[i].AltText = strings.TrimSpace(exp.Crops[i].AltText)
	}
	if exp.Markdown == "" {
		return SectionExplanation{}, fmt.Errorf("model returned an empty explanation for section %q", title)
	}

	return exp, nil
}

func buildExplainPrompt(title string, startPage, endPage, pageCount, sectionNumber int, includeImages bool) string {
	var imageIndexGuide strings.Builder
	for i := 1; i <= pageCount; i++ {
		if i > 1 {
			imageIndexGuide.WriteString("\n")
		}
		fmt.Fprintf(&imageIndexGuide, "  image_index %d = document page %d", i, startPage+i-1)
	}

	imageRules := `Image crop rules:
- Return JSON with "explanation_markdown" and "crops".
- "explanation_markdown" contains the notes themselves and may include image placeholders.
- Include crops only when a visual region materially improves understanding, such as diagrams,
  dense formulas, architecture sketches, plots, or tables.
- Do not crop decorative, redundant, or low-value content. In particular, do not crop
  text-only bullet slides unless the exact visual layout is necessary for understanding.
- When including a crop, place an inline placeholder exactly where the image best supports the
  prose: [[image:section-%02d-001]], [[image:section-%02d-002]], and so on.
- Each crop id must exactly match one placeholder in "explanation_markdown".
- Use image_index as a 1-based index into the provided section images, not the document page
  number and not a zero-based index:
%s
- Coordinates are normalized integers from 0 to 1000 relative to the selected image:
  x1,y1 is the top-left corner and x2,y2 is the bottom-right corner. For example,
  x1=0,y1=0,x2=1000,y2=500 means the top half of the selected image. Do not use
  pixel coordinates from your resized image view.
- The crop coordinates and alt_text must describe the same visible region. Never create a crop
  for a diagram, figure, or table that appears only on an adjacent or following page outside the
  provided image_index. If the right visual is not present in the provided section images, omit
  the placeholder and explain the concept in prose.
- Write alt_text as one concise plain-text sentence. Do not use Markdown, LaTeX syntax, dollar
  delimiters, backslash commands, or raw formulas. Describe formulas in words instead.`
	if !includeImages {
		imageRules = `Image crop rules:
- Return JSON with "explanation_markdown" and "crops".
- Do not include image placeholders in "explanation_markdown".
- Return an empty array for "crops".
- Still use the provided page images as source material for the written explanation.`
	} else {
		imageRules = fmt.Sprintf(imageRules, sectionNumber, sectionNumber, imageIndexGuide.String())
	}

	return fmt.Sprintf(`You are preparing study notes from a section titled "%s" (pages %d-%d) of a document.
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
- Write in a direct explanatory voice, as if the notes themselves are teaching the material.
- Do not refer to slides, pages, figures, or the document as an external source unless that
  reference is genuinely required to understand the content.
- Rewrite source-reporting phrasing into direct exposition. Avoid wording such as "the slide says",
  "the slides show", "this page introduces", "the figure illustrates", or "the document states".

Content rules:
- Do not summarize or compress — cover every concept, claim, definition, and example fully.
- Do not invent content not present on the pages.
- Do not add introductions, conclusions, or meta-commentary.
- Write in the same language as the document.

%s
- Output only valid JSON matching the schema.`, title, startPage, endPage, pageCount, imageRules)
}

func RenderExplainMarkdown(docTitle string, sections []SectionWithExplanation) string {
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

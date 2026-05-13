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
	Memory      SectionMemory
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
	Markdown string        `json:"explanation_markdown"`
	Crops    []Crop        `json:"crops"`
	Memory   SectionMemory `json:"memory"`
}

type SectionMemory struct {
	IntroducedTerms     []string `json:"introduced_terms"`
	Formulas            []string `json:"formulas"`
	RecurringPrinciples []string `json:"recurring_principles"`
	OpenThreads         []string `json:"open_threads"`
}

type sectionsResponse struct {
	Sections []Section `json:"sections"`
}

func memoryArraySchema() map[string]any {
	return map[string]any{
		"type":     "array",
		"maxItems": 8,
		"items": map[string]any{
			"type":      "string",
			"minLength": 1,
			"maxLength": 180,
		},
	}
}

var sectionExplanationSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]any{
		"explanation_markdown": map[string]any{"type": "string", "minLength": 1},
		"memory": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"introduced_terms":     memoryArraySchema(),
				"formulas":             memoryArraySchema(),
				"recurring_principles": memoryArraySchema(),
				"open_threads":         memoryArraySchema(),
			},
			"required": []string{"introduced_terms", "formulas", "recurring_principles", "open_threads"},
		},
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
	"required": []string{"explanation_markdown", "memory", "crops"},
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

func IdentifySections(ctx context.Context, p llm.Provider, model, pdfPath string, pageCount int) ([]Section, error) {
	prompt := fmt.Sprintf(`You are analyzing a PDF document. Here is the document: %s
The document has %d page(s).

Task:
- Identify the natural thematic sections in the document at lecture-unit granularity.
- For each section, provide its title, the starting page number, and the ending page number.
- Pages are 1-indexed.
- Sections must be contiguous, non-overlapping, and cover the entire document.
- Do not collapse every document into only a handful of broad chapters, but also do not split
  merely because a new example, table, benchmark, formula, or worked case appears. Split only
  when the document moves to a substantially new lecture unit or sustained topic.
- Do not create one section per slide or page. A section should usually cover a coherent run of
  related pages.
- For slide decks and technical lecture notes, prefer section ranges of about 7-15 pages when
  the topic flow allows it. Shorter sections are fine for brief transitions, but avoid creating
  many 2-4 page sections unless the document genuinely changes topic that often.
- As a rough target, a 50-70 page slide deck usually needs about 5-8 sections, a 70-100 page
  document usually needs about 7-10 sections, and a 100+ page document usually needs about 10-14
  sections. Adjust to the document's actual topic boundaries.
- Give each section a concise, descriptive title.
- Reply only with valid JSON matching the required schema.`, pdfPath, pageCount)

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

func ValidateSections(sections []Section, pageCount int) error {
	if len(sections) == 0 {
		return fmt.Errorf("no sections found")
	}
	if pageCount <= 0 {
		return fmt.Errorf("invalid page count %d", pageCount)
	}

	nextStart := 1
	for i, section := range sections {
		if strings.TrimSpace(section.Title) == "" {
			return fmt.Errorf("section %d has an empty title", i+1)
		}
		if section.StartPage != nextStart {
			return fmt.Errorf("section %q starts at page %d, expected %d", section.Title, section.StartPage, nextStart)
		}
		if section.EndPage < section.StartPage {
			return fmt.Errorf("section %q has invalid page range %d-%d", section.Title, section.StartPage, section.EndPage)
		}
		if section.EndPage > pageCount {
			return fmt.Errorf("section %q ends at page %d, but PDF has %d page(s)", section.Title, section.EndPage, pageCount)
		}
		nextStart = section.EndPage + 1
	}

	if nextStart != pageCount+1 {
		return fmt.Errorf("section ranges end at page %d, but PDF has %d page(s)", nextStart-1, pageCount)
	}
	return nil
}

func ExplainSection(ctx context.Context, p llm.Provider, model string, pagePaths []string, title string, startPage, endPage, sectionNumber int, targetLanguage, priorContext string) (SectionExplanation, error) {
	prompt := buildExplainPrompt(title, startPage, endPage, len(pagePaths), sectionNumber, targetLanguage, priorContext)

	raw, err := p.GenerateJSONWithImages(ctx, model, prompt, pagePaths, sectionExplanationSchema)
	if err != nil {
		return SectionExplanation{}, fmt.Errorf("explain section %q: %w", title, err)
	}

	var exp SectionExplanation
	if err := json.Unmarshal([]byte(raw), &exp); err != nil {
		return SectionExplanation{}, fmt.Errorf("failed to parse explanation JSON for section %q: %w", title, err)
	}

	exp.Markdown = strings.TrimSpace(exp.Markdown)
	exp.Memory = cleanSectionMemory(exp.Memory)
	for i := range exp.Crops {
		exp.Crops[i].ID = strings.TrimSpace(exp.Crops[i].ID)
		exp.Crops[i].AltText = strings.TrimSpace(exp.Crops[i].AltText)
	}
	if exp.Markdown == "" {
		return SectionExplanation{}, fmt.Errorf("model returned an empty explanation for section %q", title)
	}

	return exp, nil
}

func cleanSectionMemory(memory SectionMemory) SectionMemory {
	return SectionMemory{
		IntroducedTerms:     cleanMemoryItems(memory.IntroducedTerms, 8),
		Formulas:            cleanMemoryItems(memory.Formulas, 8),
		RecurringPrinciples: cleanMemoryItems(memory.RecurringPrinciples, 8),
		OpenThreads:         cleanMemoryItems(memory.OpenThreads, 8),
	}
}

func cleanMemoryItems(items []string, limit int) []string {
	var out []string
	seen := make(map[string]bool)
	for _, item := range items {
		item = strings.Join(strings.Fields(item), " ")
		key := strings.ToLower(item)
		if item == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func buildExplainPrompt(title string, startPage, endPage, pageCount, sectionNumber int, targetLanguage, priorContext string) string {
	var imageIndexGuide strings.Builder
	for i := 1; i <= pageCount; i++ {
		if i > 1 {
			imageIndexGuide.WriteString("\n")
		}
		fmt.Fprintf(&imageIndexGuide, "  image_index %d = document page %d", i, startPage+i-1)
	}

	imageRules := fmt.Sprintf(`Image crop rules:
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
  delimiters, backslash commands, or raw formulas. Describe formulas in words instead.`, sectionNumber, sectionNumber, imageIndexGuide.String())

	languageRule := "- Write in the same language as the document."
	if strings.TrimSpace(targetLanguage) != "" {
		languageRule = fmt.Sprintf("- Write the explanation in %s, even if the source document is in another language.", strings.TrimSpace(targetLanguage))
	}

	priorContext = strings.TrimSpace(priorContext)
	priorContextRule := ""
	if priorContext != "" {
		priorContextRule = fmt.Sprintf(`
Earlier-section context:
%s

Use this to avoid re-teaching definitions, formulas, or principles that were already established.
Still explain any new examples, caveats, numbers, assumptions, contrasts, or extensions in the
current pages.`, priorContext)
	}

	return fmt.Sprintf(`Produce complete study notes for the section "%s" (pages %d-%d).
The section spans %d page(s), provided as images in order. The notes must teach the material
directly and be understandable without the source document.

Coverage contract:
- Cover each distinct source concept, claim, definition, formula, example, caveat, comparison,
  and reasoning step exactly once.
- Preserve numbers, formulas, worked-example steps, named methods, conditions, source examples,
  concrete contrasts, and intuition that explains why a distinction matters.
- Compress only true duplication: repeated wording, repeated table rows, broad recaps, or examples
  that add no new mechanism, number, contrast, caveat, or interpretation.
- Do not invent content, add unsupported importance claims, or add broad real-world commentary.

Writing contract:
- Write connected paragraphs in a direct explanatory voice, like a careful textbook explanation.
  Use bullets only for true parallel lists or algorithm steps.
- Use '###' headings only for genuine subtopics. Bold a technical term only at first introduction,
  inline in a sentence.
- Rewrite source-reporting phrasing into direct exposition. Avoid "the slide says", "this page
  shows", "the figure illustrates", and similar wording.
- Every sentence must define, explain, connect, calculate, compare, qualify, or interpret something
  concrete. Include short connective explanations when they help a reader understand why two ideas
  differ, how an example demonstrates a concept, or when a caveat matters.
- Do not collapse a source explanation into a terse definition if the source gives a motivating
  scenario, contrast, or example. Preserve that teaching move in concise prose.
- Stop when the concept is clear; do not add recap or conclusion paragraphs.
- If an overview list is followed by a dedicated treatment, name the item briefly in the overview
  and save the explanation for the dedicated treatment.

Formulas, examples, and visuals:
- Reproduce formulas exactly in LaTeX ($...$ inline or $$...$$ display) and explain the symbols.
- Work through examples in prose with all numbers and intermediate steps present in the source.
- Explain important diagrams, tables, plots, and figures in prose integrated with the surrounding
  explanation.

Memory contract:
- Return compact memory for later sections. Include only established items that would prevent
  unnecessary re-explanation later.
- introduced_terms: terms fully defined here.
- formulas: formulas established here, with short meanings.
- recurring_principles: reusable principles or assumptions.
- open_threads: concepts introduced here but not fully developed yet.
%s

%s
%s
- Output only valid JSON matching the schema.`, title, startPage, endPage, pageCount, languageRule, priorContextRule, imageRules)
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

func BuildPriorSectionContext(sections []SectionWithExplanation, maxChars int) string {
	if len(sections) == 0 || maxChars <= 0 {
		return ""
	}

	var lines []string
	for _, section := range sections {
		lines = append(lines, priorSectionContextLines(section)...)
	}
	if len(lines) == 0 {
		return ""
	}

	var selected []string
	used := len("Already established:\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if used+len(lines[i])+1 > maxChars {
			break
		}
		used += len(lines[i]) + 1
		selected = append(selected, lines[i])
	}

	var b strings.Builder
	b.WriteString("Already established:\n")
	for i := len(selected) - 1; i >= 0; i-- {
		b.WriteString(selected[i])
		b.WriteByte('\n')
	}

	return strings.TrimSpace(b.String())
}

func priorSectionContextLines(section SectionWithExplanation) []string {
	var lines []string
	prefix := fmt.Sprintf("%s:", section.Title)
	appendItems := func(label string, items []string) {
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("- %s %s: %s", prefix, label, item))
		}
	}

	appendItems("term", section.Memory.IntroducedTerms)
	appendItems("formula", section.Memory.Formulas)
	appendItems("principle", section.Memory.RecurringPrinciples)
	appendItems("open thread", section.Memory.OpenThreads)

	return lines
}

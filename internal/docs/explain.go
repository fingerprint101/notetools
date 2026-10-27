package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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

type SectionPlan struct {
	Language      string    `json:"language"`
	DocumentTitle string    `json:"document_title"`
	Sections      []Section `json:"sections"`
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
		"language":       map[string]any{"type": "string", "minLength": 1},
		"document_title": map[string]any{"type": "string", "minLength": 1},
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
	"required": []string{"language", "document_title", "sections"},
}

func IdentifySections(ctx context.Context, p llm.Provider, model, pdfPath string, pageCount int, targetLanguage string) (SectionPlan, error) {
	targetLanguage = strings.TrimSpace(targetLanguage)
	languageSelection := `Determine the predominant language of the document as a whole and return its name in
"language". If the document mixes languages equally or has no identifiable language, use English.
Use this single language for "document_title" and every section title.`
	if targetLanguage != "" && !strings.EqualFold(targetLanguage, "auto") {
		languageSelection = fmt.Sprintf(`The requested output language is %q. Return this exact value in "language".
Write "document_title" and every section title in this language, translating source titles
where necessary. The source language and the language of these instructions do not override it.`, targetLanguage)
	}
	prompt := fmt.Sprintf(`You are analyzing a PDF document. Here is the document: %s
The document has %d page(s).

Output language:
%s

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
- Give the document and each section a concise, descriptive title in the selected output language.
- Reply only with valid JSON matching the required schema.`, pdfPath, pageCount, languageSelection)

	raw, err := p.GenerateJSON(ctx, model, prompt, sectionsSchema)
	if err != nil {
		return SectionPlan{}, fmt.Errorf("section identification failed: %w", err)
	}

	var resp SectionPlan
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return SectionPlan{}, fmt.Errorf("failed to parse sections JSON: %w", err)
	}

	if len(resp.Sections) == 0 {
		return SectionPlan{}, fmt.Errorf("model returned an empty section list")
	}

	resp.Language = strings.TrimSpace(resp.Language)
	if targetLanguage != "" && !strings.EqualFold(targetLanguage, "auto") {
		// Explicit user choice remains authoritative even if the model echoes a different name.
		resp.Language = targetLanguage
	}
	if resp.Language == "" || strings.EqualFold(resp.Language, "auto") {
		return SectionPlan{}, fmt.Errorf("model did not resolve an output language")
	}
	resp.DocumentTitle = strings.TrimSpace(resp.DocumentTitle)
	if resp.DocumentTitle == "" {
		return SectionPlan{}, fmt.Errorf("model returned an empty document title")
	}

	for i := range resp.Sections {
		resp.Sections[i].Title = strings.TrimSpace(resp.Sections[i].Title)
	}

	return resp, nil
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

func ExplainSection(ctx context.Context, p llm.Provider, model string, pagePaths []string, title string, startPage, endPage, sectionNumber, detailLevel int, includeImages bool, targetLanguage, priorContext string) (SectionExplanation, error) {
	if err := ValidateExplainDetail(detailLevel); err != nil {
		return SectionExplanation{}, fmt.Errorf("explain section %q: %w", title, err)
	}
	targetLanguage = strings.TrimSpace(targetLanguage)
	if targetLanguage == "" || strings.EqualFold(targetLanguage, "auto") {
		return SectionExplanation{}, fmt.Errorf("explain section %q: output language must be resolved first", title)
	}
	prompt := buildExplainPrompt(title, startPage, endPage, len(pagePaths), sectionNumber, detailLevel, includeImages, targetLanguage, priorContext)

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

// ValidateExplainDetail checks the supported explanation detail levels.
func ValidateExplainDetail(level int) error {
	if level < 1 || level > 3 {
		return fmt.Errorf("detail level must be 1, 2, or 3 (got %d)", level)
	}
	return nil
}

func explainDetailRules(level int) (string, string) {
	if level == 1 {
		return `Your goal is to produce concise study notes that make the important concepts immediately
recognizable. Explain the central ideas well enough to study from, while giving secondary
material proportionately less space. Do not turn the section into a textbook chapter or a
page-by-page transcription.`, `- Aim for roughly 300-600 words for a typical 7-15 page section. This is a guide, not a quota:
  write much less for sparse or repetitive pages, and exceed it only when distinct core concepts
  or a necessary derivation require it. Do not pad short sections or cut essential reasoning.`
	}

	goal := `Your goal is to produce study notes that explain the selected concepts and their reasoning
in depth. Select content using the prioritization, formula, example, and visual rules below.
Keep the focus on central ideas, with proportionately less space for secondary material.
Unpack the meaning and reasoning of the selected ideas in clear prose. Include topics,
supporting facts, examples, derivations, and edge cases only when warranted by the selection
rules; do not add material merely to make the explanation longer.
Do not transcribe page by page, repeat yourself, or invent unsupported explanations.`
	depth := `Explanation depth:
- Describe each selected concept in developed paragraphs: clarify what it means, how it
  works, and why it works. Explain the reasoning behind the main steps.
- Make the logical connections between the selected ideas explicit. For formulas, examples,
  and qualifications already warranted by the selection rules, explain their meaning and
  reasoning clearly.`
	wordRange := "600-1000"
	if level == 3 {
		depth = `Explanation depth:
- Explain each selected concept thoroughly through connected paragraphs. Carefully unpack
  its meaning and explain how and why it works, spelling out the intermediate reasoning
  step by step and explaining why each step follows from the preceding one.
- Walk the reader through the connections between the selected ideas so each logical step
  is understandable. For formulas, examples, and qualifications already warranted by the
  selection rules, thoroughly explain their meaning and reasoning.`
		wordRange = "1000-1600"
	}
	lengthRule := fmt.Sprintf(`- Aim for roughly %s words for a typical 7-15 page section. This is a guide, not a quota:
  write much less for sparse or repetitive pages, and exceed it only when explaining the
  selected material requires it. Never broaden coverage, repeat ideas, or pad to meet a length.`, wordRange)
	return goal + "\n\n" + depth, lengthRule
}

func buildExplainPrompt(title string, startPage, endPage, pageCount, sectionNumber, detailLevel int, includeImages bool, targetLanguage, priorContext string) string {
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

	languageRule := fmt.Sprintf(`Output language (mandatory for the entire section): %q.
- Write all prose, headings, list items, table labels, captions, and crop alt_text in this language.
- Translate source text rather than copying sentences in another language. Do not switch language
  because the source pages, section title, earlier context, or these instructions use another one.
- Keep proper names, code, mathematical notation, and established technical terms when translation
  would be misleading. Explain such terms in the output language; do not add bilingual prose.
- Keep JSON keys and image placeholder IDs unchanged.
- Before returning the JSON, check every human-readable field and rewrite any unintended language
  switches into the chosen output language.`, targetLanguage)

	priorContext = strings.TrimSpace(priorContext)
	priorContextRule := ""
	if priorContext != "" {
		priorContextRule = fmt.Sprintf(`
Earlier-section context:
%s

Use this only to avoid re-teaching material that has already been explained. If the current pages
repeat an earlier definition, formula, benchmark category, or general principle, mention it only
briefly and focus on the new detail, contrast, number, example, or exception added by this section.
Preserve new core concepts and essential qualifications even when they use a previously
introduced term; apply the same prioritization rules to new supporting details.`, priorContext)
	}

	goal, lengthRule := explainDetailRules(detailLevel)
	return fmt.Sprintf(`You are preparing study notes from a section titled "%s" (pages %d-%d) of a document.
The section spans %d page(s), provided as images in order.

%s

%s

Prioritization and length:
- Before writing, distinguish core concepts from supporting details using the section's learning
  objectives, emphasis, and dependencies: a concept is core when it explains the main topic or is
  needed to understand or apply other central ideas. Do not infer importance from page count alone.
- Spend most of the explanation on those core concepts: what they mean, how they work, and the
  essential why or when. Keep conditions, assumptions, and caveats that affect correctness next
  to the concept they qualify; never demote them just to shorten the notes.
- Compress secondary facts, historical context, implementation variants, and illustrative numbers.
  Omit incidental details and repetitive examples that do not change understanding or application.
  Do not give every named term its own explanation or equal visual weight.
%s
- Explain each idea once. If an overview is followed by a dedicated treatment, reserve the
  explanation for that treatment. Do not add a recap that repeats the body.

Structure and emphasis:
- Start directly with the main concept or relationship, not a generic introduction or a separate
  summary that will be repeated below.
- Use short, connected paragraphs for explanations. Use '###' headings for the main subtopics
  only; do not create a heading for every term or example.
- Bold the few central terms or short essential relationships that a reader should notice on a
  quick scan. Usually highlight each once. Do not bold every new technical term, secondary fact,
  whole paragraph, or label prefix. Use ordinary Markdown **term**, without math delimiters.
- Put useful secondary material that would interrupt the main explanation under a single optional
  '#### Supporting details' heading at the end of the section, using a few short bullets. Omit
  this subsection when unnecessary; it is not a place to dump every omitted source detail.
  Translate this heading into the output language. Do not repeat the core explanation there.
- Use short lists or compact tables when they make parallel properties, comparisons, or algorithm
  steps easier to scan. Use prose to explain relationships, rather than making everything a list.

Formulas, examples, and visuals:
- Include formulas needed to understand or apply the core concepts. Reproduce retained formulas
  accurately in LaTeX ($...$ inline or $$...$$ display), define their symbols on first use, and
  state relevant assumptions. Omit duplicate rearrangements and incidental formulas.
- Include a worked example only when it clarifies a core idea or teaches a necessary procedure.
  Prefer one representative example per method, retaining the key steps and result. Show all
  intermediate steps only when those steps are themselves the point of the lesson; omit routine
  arithmetic and repeated worked cases. Keep distinct cases when they reveal a meaningful limit
  or contrast, but describe the difference briefly.
- For tables or score reports, explain the meaningful comparison or pattern and use only the
  representative values needed to support it. Do not narrate every row or column.
- Integrate important diagrams into the relevant explanation. Describe the relationship the
  reader needs to understand, without duplicating every visible label or adding a separate recap.

Accuracy and voice:
- Preserve all core concepts and the reasoning needed to connect them. Shorten by selecting and
  compressing supporting material, not by making the main explanation vague or cryptic.
- Do not invent content, unsupported claims of importance, or likely exam questions.
- Write in a direct explanatory voice. Do not refer to slides, pages, figures, or the document
  as an external source unless required to understand the content. Avoid "the slide says",
  "the figure illustrates", and similar source-reporting phrasing.
- Every sentence must define, explain, connect, or qualify a specific idea. Once it is clear,
  stop; avoid generic transitions, broad relevance claims, conclusions, and meta-commentary.
- Do not re-teach material listed in earlier-section context. Briefly connect to it only when
  needed, then focus on the new concept, mechanism, contrast, or exception. A new incidental
  fact does not need inclusion merely because it was absent from earlier sections.

%s
%s
- Output only valid JSON matching the schema.`, title, startPage, endPage, pageCount, languageRule, goal, lengthRule, priorContextRule, imageRules)
}

func RenderExplainMarkdown(docTitle string, sections []SectionWithExplanation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", docTitle)
	for _, s := range sections {
		if s.StartPage == s.EndPage {
			fmt.Fprintf(&b, "## %s (%d)\n\n", s.Title, s.StartPage)
		} else {
			fmt.Fprintf(&b, "## %s (%d–%d)\n\n", s.Title, s.StartPage, s.EndPage)
		}
		fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(s.Explanation))
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func BuildPriorSectionContext(sections []SectionWithExplanation, maxChars int) string {
	if len(sections) == 0 || maxChars <= 0 {
		return ""
	}

	var b strings.Builder
	var lines []string
	for _, section := range sections {
		line := priorSectionContextLine(section)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}

	var selected []string
	used := len("Already covered:\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if used+len(lines[i])+1 > maxChars {
			break
		}
		used += len(lines[i]) + 1
		selected = append(selected, lines[i])
	}

	b.WriteString("Already covered:\n")
	for i := len(selected) - 1; i >= 0; i-- {
		b.WriteString(selected[i])
		b.WriteByte('\n')
	}

	return strings.TrimSpace(b.String())
}

func priorSectionContextLine(section SectionWithExplanation) string {
	var parts []string
	if headings := markdownHeadings(section.Explanation); len(headings) > 0 {
		parts = append(parts, "subtopics: "+strings.Join(headings, ", "))
	}
	if terms := boldTerms(section.Explanation); len(terms) > 0 {
		parts = append(parts, "emphasized concepts: "+strings.Join(terms, ", "))
	}

	pageLabel := fmt.Sprintf("pages %d-%d", section.StartPage, section.EndPage)
	if section.StartPage == section.EndPage {
		pageLabel = fmt.Sprintf("page %d", section.StartPage)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("- %s (%s)", section.Title, pageLabel)
	}
	return fmt.Sprintf("- %s (%s): %s", section.Title, pageLabel, strings.Join(parts, "; "))
}

func markdownHeadings(markdown string) []string {
	return uniqueMatches(markdown, regexp.MustCompile(`(?m)^###\s+(.+?)\s*$`), 8)
}

func boldTerms(markdown string) []string {
	return uniqueMatches(markdown, regexp.MustCompile(`\*\*([^*\n]{2,80})\*\*`), 12)
}

func uniqueMatches(text string, re *regexp.Regexp, limit int) []string {
	var values []string
	seen := make(map[string]bool)
	for _, match := range re.FindAllStringSubmatch(text, -1) {
		value := strings.TrimSpace(match[1])
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		values = append(values, value)
		if len(values) >= limit {
			break
		}
	}
	return values
}

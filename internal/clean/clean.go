package clean

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fingerprint/notetools/internal/llm"
)

type Section struct {
	Title     string
	StartLine int
	EndLine   int
	Content   string
}

type sectionRange struct {
	Title     string `json:"title"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type sectionsResponse struct {
	Sections []sectionRange `json:"sections"`
}

type cleanedResponse struct {
	CleanedContent string `json:"cleaned_content"`
}

var sectionSchema = map[string]any{
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
					"start_line": map[string]any{"type": "integer", "minimum": 1},
					"end_line":   map[string]any{"type": "integer", "minimum": 1},
				},
				"required": []string{"title", "start_line", "end_line"},
			},
		},
	},
	"required": []string{"sections"},
}

var cleanSectionSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]any{
		"cleaned_content": map[string]any{"type": "string", "minLength": 1},
	},
	"required": []string{"cleaned_content"},
}

func SectionTranscript(ctx context.Context, p llm.Provider, model, transcript string) ([]Section, error) {
	numberedTranscript, lineCount := withLineNumbers(transcript)

	prompt := fmt.Sprintf(`You are organizing an automatic transcript of an Italian university lecture.

Task:
- Split the transcript into coherent thematic sections.
- Return only section titles plus inclusive line ranges.

Mandatory constraints:
- Do not summarize.
- Do not correct the text.
- Do not remove anything.
- Do not add anything.
- Preserve the original order of the lecture.
- Each section must contain a contiguous block of the original transcript lines.
- The combined line ranges must cover the full transcript from line 1 to line %d.
- Use the numbered lines below as the source of truth for boundaries.
- Line ranges must be contiguous and non-overlapping.
- Give each section a short title in Italian, using correct Italian spelling, including accents when needed.
- Prefer a fine-grained structure rather than a small number of broad sections.
- Create specific sections whenever the speaker changes subtopic, example, system category, comparison, or teaching focus.
- Do not merge clearly different subtopics into the same section.
- Prefer more sections over fewer sections when in doubt.
- For long transcripts, prefer at least 8 to 15 sections unless the transcript is genuinely very uniform.
- Avoid sections that are overly broad or that cover multiple major ideas.
- Reply only with valid JSON matching the required schema.

Transcript:
<<<TRANSCRIPT
%s
TRANSCRIPT>>>
`, lineCount, numberedTranscript)

	raw, err := p.GenerateJSON(ctx, model, prompt, sectionSchema)
	if err != nil {
		return nil, fmt.Errorf("sectioning failed: %w", err)
	}

	var resp sectionsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse sections JSON: %w", err)
	}

	if len(resp.Sections) == 0 {
		return nil, fmt.Errorf("model returned an empty section list")
	}

	lines := strings.Split(transcript, "\n")
	sections := make([]Section, 0, len(resp.Sections))
	nextStart := 1

	for i := range resp.Sections {
		sec := resp.Sections[i]
		sec.Title = strings.TrimSpace(sec.Title)
		if sec.Title == "" {
			return nil, fmt.Errorf("model returned an empty title for section %d", i+1)
		}
		if sec.StartLine != nextStart {
			return nil, fmt.Errorf("section %q starts at line %d, expected %d", sec.Title, sec.StartLine, nextStart)
		}
		if sec.EndLine < sec.StartLine || sec.EndLine > len(lines) {
			return nil, fmt.Errorf("section %q has invalid line range %d-%d", sec.Title, sec.StartLine, sec.EndLine)
		}

		content := strings.TrimSpace(strings.Join(lines[sec.StartLine-1:sec.EndLine], "\n"))
		sections = append(sections, Section{
			Title:     sec.Title,
			StartLine: sec.StartLine,
			EndLine:   sec.EndLine,
			Content:   content,
		})
		nextStart = sec.EndLine + 1
	}

	if nextStart != len(lines)+1 {
		return nil, fmt.Errorf("section ranges do not cover the full transcript")
	}

	return sections, nil
}

func CleanSection(ctx context.Context, p llm.Provider, model, title, content string) (string, error) {
	prompt := fmt.Sprintf(`You are cleaning a single section from an automatic transcript of a university lecture.

Task:
- Work only on the provided section.
- Make the discourse coherent and readable.
- Remove obvious transcription noise, spurious repetitions, and fragments that are clearly meaningless.
- Correct phrases or terms that were clearly misheard by the transcriber when the technical context makes the intended meaning evident.
- Fix punctuation and sentence boundaries.

Mandatory constraints:
- Do not change the topic.
- Do not add new concepts that are not supported by the text.
- Preserve all substantial information present in the section.
- Return the cleaned text in correct Italian, using proper accents and apostrophes where needed.
- Do not include the title, notes, comments, or markdown.
- Reply only with valid JSON matching the required schema.

Section title:
%s

Section text:
<<<SECTION
%s
SECTION>>>
`, title, content)

	raw, err := p.GenerateJSON(ctx, model, prompt, cleanSectionSchema)
	if err != nil {
		return "", fmt.Errorf("cleaning section %q failed: %w", title, err)
	}

	var resp cleanedResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", fmt.Errorf("failed to parse cleaned section JSON: %w", err)
	}

	cleaned := strings.TrimSpace(resp.CleanedContent)
	if cleaned == "" {
		return "", fmt.Errorf("model returned empty cleaned content for section %q", title)
	}

	return cleaned, nil
}

func RenderMarkdown(docTitle string, sections []Section) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", docTitle)
	for _, s := range sections {
		fmt.Fprintf(&b, "## %s\n%s\n\n", s.Title, strings.TrimSpace(s.Content))
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func withLineNumbers(text string) (string, int) {
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%d\t%s\n", i+1, line)
	}
	return strings.TrimRight(b.String(), "\n"), len(lines)
}

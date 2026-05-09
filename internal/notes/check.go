package notes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fingerprint/notetools/internal/llm"
)

const DefaultCheckTokenBudget = 60000
const checkSectionCharBudget = 6000

type CheckSource struct {
	Path       string
	Content    string
	LineOffset int
}

type MissingInfo struct {
	SourcePath string
	Title      string
	StartLine  int
	EndLine    int
	Summary    string
}

type CheckProgress struct {
	Step       int
	Total      int
	SourcePath string
	Sections   int
}

type coverageSection struct {
	SourceIndex int
	Index       int
	Title       string
	StartLine   int
	EndLine     int
	Content     string
}

type missingInfoJSON struct {
	SourceIndex  int    `json:"source_index"`
	SectionIndex int    `json:"section_index"`
	Title        string `json:"title"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	Summary      string `json:"summary"`
}

type checkResponse struct {
	Missing []missingInfoJSON `json:"missing"`
}

var checkSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]any{
		"missing": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"source_index":  map[string]any{"type": "integer", "minimum": 0},
					"section_index": map[string]any{"type": "integer", "minimum": 0},
					"title":         map[string]any{"type": "string"},
					"start_line":    map[string]any{"type": "integer", "minimum": 1},
					"end_line":      map[string]any{"type": "integer", "minimum": 1},
					"summary":       map[string]any{"type": "string"},
				},
				"required": []string{"source_index", "section_index", "title", "start_line", "end_line", "summary"},
			},
		},
	},
	"required": []string{"missing"},
}

func CheckCoverage(ctx context.Context, p llm.Provider, model string, sources []CheckSource, targetContent string, tokenBudget int, notify func(CheckProgress)) ([]MissingInfo, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("at least one source is required")
	}
	if strings.TrimSpace(targetContent) == "" {
		return nil, fmt.Errorf("target content is empty")
	}
	if tokenBudget <= 0 {
		tokenBudget = DefaultCheckTokenBudget
	}

	targetContext := formatNumberedLines(targetContent)
	basePrompt := buildCheckPrompt(targetContext, "")
	sourceBudget := tokenBudget - estimateTokens(basePrompt) - 1000
	if sourceBudget < 1 {
		sourceBudget = 1
	}

	var sections []coverageSection
	for sourceIndex, source := range sources {
		if strings.TrimSpace(source.Content) == "" {
			return nil, fmt.Errorf("source content is empty: %s", source.Path)
		}
		sourceSections := splitCoverageSections(source.Content, source.LineOffset)
		for i := range sourceSections {
			sourceSections[i].SourceIndex = sourceIndex
			sourceSections[i].Index = len(sections)
			sections = append(sections, sourceSections[i])
		}
	}
	if len(sections) == 0 {
		return nil, nil
	}
	sectionsByIndex := map[int]coverageSection{}
	for _, section := range sections {
		sectionsByIndex[section.Index] = section
	}

	batches := checkBatches(sections, sourceBudget)
	var missing []MissingInfo
	seenMissing := map[string]bool{}
	for i, batch := range batches {
		if notify != nil {
			notify(CheckProgress{
				Step:       i + 1,
				Total:      len(batches),
				SourcePath: batchSourceLabel(batch, sources),
				Sections:   len(batch),
			})
		}
		raw, err := p.GenerateJSON(ctx, model, buildCheckPrompt(targetContext, formatCheckBatch(batch, sources)), checkSchema)
		if err != nil {
			return nil, fmt.Errorf("coverage check failed: %w", err)
		}

		var resp checkResponse
		if err := json.Unmarshal([]byte(raw), &resp); err != nil {
			return nil, fmt.Errorf("failed to parse coverage JSON: %w", err)
		}
		for _, item := range resp.Missing {
			if item.SourceIndex < 0 || item.SourceIndex >= len(sources) {
				continue
			}
			section, ok := sectionsByIndex[item.SectionIndex]
			if !ok || section.SourceIndex != item.SourceIndex {
				continue
			}
			key := fmt.Sprintf("%d:%d:%d", section.SourceIndex, section.StartLine, section.EndLine)
			if seenMissing[key] {
				continue
			}
			seenMissing[key] = true
			missing = append(missing, MissingInfo{
				SourcePath: sources[item.SourceIndex].Path,
				Title:      section.Title,
				StartLine:  section.StartLine,
				EndLine:    section.EndLine,
				Summary:    strings.TrimSpace(item.Summary),
			})
		}
	}

	return missing, nil
}

func buildCheckPrompt(numberedTarget, sourceBatch string) string {
	return fmt.Sprintf(`You are an information coverage auditor. Your job is to decide whether the TARGET document contains all substantive information from the SOURCE sections.

For each SOURCE SECTION:
- Treat the target as covering information even when it is paraphrased, translated, reorganized, or split across multiple places.
- Do not mark a section missing just because wording, heading names, order, or language differ.
- Mark a section missing only when it contains concrete substantive details that are absent from the target.
- Ignore filler, repetitions, transcription artifacts, false starts, and purely stylistic differences.
- Preserve the source_index, section_index, title, start_line, and end_line from the section header when reporting missing information.
- In summary, briefly name the missing details.

Return valid JSON only. If nothing is missing, return {"missing":[]}.

TARGET:
%s

SOURCE SECTIONS:
%s`, numberedTarget, sourceBatch)
}

func splitCoverageSections(content string, lineOffset int) []coverageSection {
	lines := strings.Split(content, "\n")
	headings := markdownHeadings(lines)
	if len(headings) > 0 {
		return coverageSectionsFromHeadings(lines, headings, lineOffset)
	}
	return coverageSectionsFromParagraphs(lines, lineOffset)
}

type coverageHeading struct {
	line  int
	title string
}

func markdownHeadings(lines []string) []coverageHeading {
	var headings []coverageHeading
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		level := 0
		for level < len(trimmed) && trimmed[level] == '#' {
			level++
		}
		if level < 1 || level > 6 || level >= len(trimmed) || trimmed[level] != ' ' {
			continue
		}
		title := strings.TrimSpace(trimmed[level:])
		if title == "" {
			continue
		}
		headings = append(headings, coverageHeading{line: i + 1, title: title})
	}
	return headings
}

func coverageSectionsFromHeadings(lines []string, headings []coverageHeading, lineOffset int) []coverageSection {
	sections := make([]coverageSection, 0, len(headings))
	for i, heading := range headings {
		end := len(lines)
		if i+1 < len(headings) {
			end = headings[i+1].line - 1
		}
		content := strings.TrimSpace(strings.Join(lines[heading.line-1:end], "\n"))
		if content == "" {
			continue
		}
		sections = append(sections, coverageSection{
			Title:     heading.title,
			StartLine: heading.line + lineOffset,
			EndLine:   end + lineOffset,
			Content:   trimSectionForCheck(content),
		})
	}
	return sections
}

func coverageSectionsFromParagraphs(lines []string, lineOffset int) []coverageSection {
	var sections []coverageSection
	start := 0
	var b strings.Builder

	flush := func(end int) {
		text := strings.TrimSpace(b.String())
		if text == "" {
			b.Reset()
			start = 0
			return
		}
		sections = append(sections, coverageSection{
			Title:     fallbackSectionTitle(text, len(sections)+1),
			StartLine: start + lineOffset,
			EndLine:   end + lineOffset,
			Content:   trimSectionForCheck(text),
		})
		b.Reset()
		start = 0
	}

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush(i)
			continue
		}
		if start == 0 {
			start = i + 1
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
		if b.Len() >= checkSectionCharBudget {
			flush(i + 1)
		}
	}
	flush(len(lines))
	return sections
}

func fallbackSectionTitle(text string, index int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return fmt.Sprintf("Section %d", index)
	}
	if len(words) > 12 {
		words = words[:12]
	}
	return truncateString(strings.Join(words, " "), 80)
}

func trimSectionForCheck(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= checkSectionCharBudget {
		return text
	}
	return strings.TrimSpace(truncateString(text, checkSectionCharBudget)) + "\n\n[Section truncated for checking; report this line range if any visible details are missing.]"
}

func truncateString(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func checkBatches(sections []coverageSection, sourceBudget int) [][]coverageSection {
	var batches [][]coverageSection
	var current []coverageSection
	currentTokens := 0

	for _, section := range sections {
		sectionTokens := estimateTokens(formatCheckBatch([]coverageSection{section}, nil))
		if len(current) > 0 && currentTokens+sectionTokens > sourceBudget {
			batches = append(batches, current)
			current = nil
			currentTokens = 0
		}
		current = append(current, section)
		currentTokens += sectionTokens
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func formatCheckBatch(sections []coverageSection, sources []CheckSource) string {
	var b strings.Builder
	for _, section := range sections {
		sourcePath := ""
		if section.SourceIndex >= 0 && section.SourceIndex < len(sources) {
			sourcePath = sources[section.SourceIndex].Path
		}
		fmt.Fprintf(&b, "SOURCE SECTION source_index=%d, section_index=%d, lines %d-%d, title=%q, path=%q\n",
			section.SourceIndex,
			section.Index,
			section.StartLine,
			section.EndLine,
			section.Title,
			sourcePath,
		)
		b.WriteString(section.Content)
		b.WriteString("\n\n")
	}
	return b.String()
}

func batchSourceLabel(batch []coverageSection, sources []CheckSource) string {
	if len(batch) == 0 {
		return ""
	}
	sourceIndex := batch[0].SourceIndex
	if sourceIndex < 0 || sourceIndex >= len(sources) {
		return ""
	}
	for _, section := range batch[1:] {
		if section.SourceIndex != sourceIndex {
			return "multiple sources"
		}
	}
	return sources[sourceIndex].Path
}

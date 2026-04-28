package notes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fingerprint/notetools/internal/llm"
)

type Mapping struct {
	Title           string `json:"title"`
	File1Start      int    `json:"file1_start"`
	File1End        int    `json:"file1_end"`
	PresentInFile2  bool   `json:"present_in_file2"`
	File2Start      int    `json:"file2_start"`
	File2End        int    `json:"file2_end"`
	InsertAfterLine int    `json:"insert_after_line"`
}

type PlanDocument struct {
	Version    int       `json:"version"`
	SourcePath string    `json:"source_path"`
	TargetPath string    `json:"target_path"`
	Mappings   []Mapping `json:"mappings"`
}

const planDocumentVersion = 1
const DefaultPlanTokenBudget = 60000

type noteSection struct {
	Title     string
	Level     int
	StartLine int
	EndLine   int
}

type sectionMatch struct {
	File1Index      int  `json:"file1_index"`
	Present         bool `json:"present"`
	File2Start      int  `json:"file2_start"`
	File2End        int  `json:"file2_end"`
	InsertAfterLine int  `json:"insert_after_line"`
}

type matchResponse struct {
	Matches []sectionMatch `json:"matches"`
}

var matchSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]any{
		"matches": map[string]any{
			"type":     "array",
			"minItems": 1,
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"file1_index":       map[string]any{"type": "integer", "minimum": 0},
					"present":           map[string]any{"type": "boolean"},
					"file2_start":       map[string]any{"type": "integer", "minimum": 0},
					"file2_end":         map[string]any{"type": "integer", "minimum": 0},
					"insert_after_line": map[string]any{"type": "integer", "minimum": 0},
				},
				"required": []string{"file1_index", "present", "file2_start", "file2_end", "insert_after_line"},
			},
		},
	},
	"required": []string{"matches"},
}

func parseNoteSections(content string) []noteSection {
	lines := strings.Split(content, "\n")
	type head struct {
		level int
		line  int
		title string
	}
	var heads []head
	for i, line := range lines {
		trim := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trim, "## ") && !strings.HasPrefix(trim, "### ") {
			continue
		}
		level := 0
		for level < len(trim) && trim[level] == '#' {
			level++
		}
		if level != 2 && level != 3 {
			continue
		}
		title := strings.TrimSpace(trim[level:])
		heads = append(heads, head{level: level, line: i + 1, title: title})
	}

	out := make([]noteSection, 0, len(heads))
	for i, h := range heads {
		end := len(lines)
		for j := i + 1; j < len(heads); j++ {
			if heads[j].level <= h.level {
				end = heads[j].line - 1
				break
			}
		}
		out = append(out, noteSection{
			Title:     h.title,
			Level:     h.level,
			StartLine: h.line,
			EndLine:   end,
		})
	}
	return out
}

func sectionText(lines []string, sec noteSection) string {
	if sec.StartLine <= 0 || sec.StartLine > len(lines) || sec.EndLine < sec.StartLine {
		return ""
	}
	end := sec.EndLine
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[sec.StartLine-1:end], "\n")
}

func formatNumberedLines(content string) string {
	lines := strings.Split(content, "\n")
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%04d %s\n", i+1, line)
	}
	return b.String()
}

func formatSourceBatch(secs []indexedSection, lines []string) string {
	var b strings.Builder
	for _, s := range secs {
		fmt.Fprintf(&b, "SOURCE SECTION file1_index=%d, H%d, lines %d-%d, title=%q\n", s.Index, s.Level, s.StartLine, s.EndLine, s.Title)
		b.WriteString(sectionText(lines, s.noteSection))
		b.WriteString("\n\n")
	}
	return b.String()
}

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

type indexedSection struct {
	noteSection
	Index int
}

func Plan(ctx context.Context, p llm.Provider, model, file1Content, file2Content string) ([]Mapping, error) {
	return PlanWithTokenBudget(ctx, p, model, file1Content, file2Content, DefaultPlanTokenBudget)
}

func PlanWithTokenBudget(ctx context.Context, p llm.Provider, model, file1Content, file2Content string, tokenBudget int) ([]Mapping, error) {
	if tokenBudget <= 0 {
		tokenBudget = DefaultPlanTokenBudget
	}

	s1 := parseNoteSections(file1Content)
	if len(s1) == 0 {
		return nil, fmt.Errorf("no ## or ### sections found in source file")
	}

	lines1 := strings.Split(file1Content, "\n")
	lines2 := strings.Split(file2Content, "\n")
	insertAnchors := safeInsertAnchors(lines2)
	numberedTarget := formatNumberedLines(file2Content)
	basePrompt := buildPlanPrompt(numberedTarget, formatInsertAnchors(lines2, insertAnchors), "")
	sourceBudget := tokenBudget - estimateTokens(basePrompt) - 1000
	if sourceBudget < 1 {
		sourceBudget = 1
	}

	var allMatches []sectionMatch
	for _, batch := range planBatches(s1, lines1, sourceBudget) {
		prompt := buildPlanPrompt(numberedTarget, formatInsertAnchors(lines2, insertAnchors), formatSourceBatch(batch, lines1))
		raw, err := p.GenerateJSON(ctx, model, prompt, matchSchema)
		if err != nil {
			return nil, fmt.Errorf("plan generation failed: %w", err)
		}

		var resp matchResponse
		if err := json.Unmarshal([]byte(raw), &resp); err != nil {
			return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
		}
		allMatches = append(allMatches, resp.Matches...)
	}

	mappings, err := toMappings(matchResponse{Matches: allMatches}, s1, len(lines2), insertAnchors)
	if err != nil {
		return nil, err
	}
	return mappings, nil
}

func buildPlanPrompt(numberedTarget, insertAnchors, sourceBatch string) string {
	return fmt.Sprintf(`You are a note-planning assistant. You are given the full destination Markdown note with authoritative line numbers, plus one or more full source sections. For EACH source section, decide whether the destination note already covers the same topic.

Output one match record per source section:
- file1_index: the source section index shown in the section header.
- present: true if the destination covers the same topic, even if worded or organised differently.
- when present is true: file2_start and file2_end are the best destination line range to merge into. Set insert_after_line to 0.
- when present is false: insert_after_line is the destination line after which the missing section fits best logically. Choose insert_after_line only from SAFE INSERT_AFTER_LINE VALUES below. Never insert in the middle of a paragraph, formula, list item, or explanation. Use the final destination line to append at the end. Use 0 only when no specific destination line fits, which will also append at the end. Set file2_start and file2_end to 0.

Match by semantic topic, not by heading wording. A source section may map to any destination line range, including a range inside a broad or poorly named destination section. Destination line numbers are authoritative. Output every source section exactly once, in order. Reply with valid JSON only.

DESTINATION NOTE:
%s

SAFE INSERT_AFTER_LINE VALUES:
%s

SOURCE SECTIONS:
%s`, numberedTarget, insertAnchors, sourceBatch)
}

func planBatches(secs []noteSection, lines []string, sourceBudget int) [][]indexedSection {
	var batches [][]indexedSection
	var current []indexedSection
	currentTokens := 0

	for i, sec := range secs {
		indexed := indexedSection{noteSection: sec, Index: i}
		sectionTokens := estimateTokens(formatSourceBatch([]indexedSection{indexed}, lines))
		if len(current) > 0 && currentTokens+sectionTokens > sourceBudget {
			batches = append(batches, current)
			current = nil
			currentTokens = 0
		}
		current = append(current, indexed)
		currentTokens += sectionTokens
	}

	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func NewPlanDocument(sourcePath, targetPath string, mappings []Mapping) PlanDocument {
	return PlanDocument{
		Version:    planDocumentVersion,
		SourcePath: sourcePath,
		TargetPath: targetPath,
		Mappings:   mappings,
	}
}

func RenderPlan(doc PlanDocument) (string, error) {
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal plan document: %w", err)
	}
	return string(raw) + "\n", nil
}

func ParsePlan(content string) (PlanDocument, error) {
	var doc PlanDocument
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		return PlanDocument{}, fmt.Errorf("parse plan document: %w", err)
	}
	if doc.Version != planDocumentVersion {
		return PlanDocument{}, fmt.Errorf("unsupported plan version %d", doc.Version)
	}
	if doc.SourcePath == "" || doc.TargetPath == "" {
		return PlanDocument{}, fmt.Errorf("plan is missing source or target paths")
	}
	return doc, nil
}

func toMappings(resp matchResponse, s1 []noteSection, targetLineCount int, insertAnchors []int) ([]Mapping, error) {
	matches := make(map[int]sectionMatch, len(s1))
	for _, m := range resp.Matches {
		if m.File1Index < 0 || m.File1Index >= len(s1) {
			continue
		}
		if _, exists := matches[m.File1Index]; exists {
			continue
		}
		matches[m.File1Index] = m
	}

	out := make([]Mapping, 0, len(s1))
	for i, sec := range s1 {
		m, ok := matches[i]
		if ok {
			mp, err := mappingFor(m, s1, targetLineCount, insertAnchors)
			if err != nil {
				return nil, err
			}
			out = append(out, mp)
			continue
		}
		out = append(out, Mapping{
			Title:           sec.Title,
			File1Start:      sec.StartLine,
			File1End:        sec.EndLine,
			PresentInFile2:  false,
			InsertAfterLine: 0,
		})
	}

	return out, nil
}

func mappingFor(m sectionMatch, s1 []noteSection, targetLineCount int, insertAnchors []int) (Mapping, error) {
	sec := s1[m.File1Index]
	mp := Mapping{
		Title:          sec.Title,
		File1Start:     sec.StartLine,
		File1End:       sec.EndLine,
		PresentInFile2: m.Present,
	}
	if m.Present {
		if m.File2Start <= 0 || m.File2End < m.File2Start {
			return Mapping{}, fmt.Errorf("section %q has invalid target line range %d-%d", sec.Title, m.File2Start, m.File2End)
		}
		if targetLineCount > 0 && m.File2End > targetLineCount {
			return Mapping{}, fmt.Errorf("section %q target line range %d-%d exceeds target length (%d lines)", sec.Title, m.File2Start, m.File2End, targetLineCount)
		}
		mp.File2Start = m.File2Start
		mp.File2End = m.File2End
		return mp, nil
	}
	if m.InsertAfterLine > 0 {
		mp.InsertAfterLine = nearestSafeInsertAnchor(clampLine(m.InsertAfterLine, targetLineCount), insertAnchors)
	}
	return mp, nil
}

func safeInsertAnchors(lines []string) []int {
	anchors := map[int]bool{0: true, len(lines): true}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			anchors[i+1] = true
		}
	}

	for _, sec := range parseNoteSections(strings.Join(lines, "\n")) {
		anchors[sec.EndLine] = true
		if sec.StartLine > 1 {
			anchors[sec.StartLine-1] = true
		}
	}

	out := make([]int, 0, len(anchors))
	for line := range anchors {
		out = append(out, line)
	}
	sort.Ints(out)
	return out
}

func formatInsertAnchors(lines []string, anchors []int) string {
	var b strings.Builder
	for _, anchor := range anchors {
		if anchor == 0 {
			b.WriteString("- 0: append at end when no specific destination line fits\n")
			continue
		}
		context := ""
		if anchor <= len(lines) {
			context = strings.TrimSpace(lines[anchor-1])
		}
		if context == "" && anchor < len(lines) {
			context = "before " + strings.TrimSpace(lines[anchor])
		}
		if context == "" {
			context = "end of file"
		}
		fmt.Fprintf(&b, "- %d: %s\n", anchor, context)
	}
	return strings.TrimRight(b.String(), "\n")
}

func nearestSafeInsertAnchor(line int, anchors []int) int {
	if line <= 0 || len(anchors) == 0 {
		return 0
	}
	best := anchors[0]
	bestDistance := abs(line - best)
	for _, anchor := range anchors[1:] {
		distance := abs(line - anchor)
		if distance < bestDistance || (distance == bestDistance && anchor < best) {
			best = anchor
			bestDistance = distance
		}
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func clampLine(line, maxLine int) int {
	if line < 1 {
		return 1
	}
	if maxLine > 0 && line > maxLine {
		return maxLine
	}
	return line
}

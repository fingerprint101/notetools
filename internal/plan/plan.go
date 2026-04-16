package plan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fingerprint/notetools/internal/llm"
)

// Mapping describes how one section from file1 relates to file2.
type Mapping struct {
	Title          string `json:"title"`
	File1Start     int    `json:"file1_start"`
	File1End       int    `json:"file1_end"`
	PresentInFile2 bool   `json:"present_in_file2"`
	// Set when PresentInFile2 is true.
	File2Start int `json:"file2_start"`
	File2End   int `json:"file2_end"`
	// Set when PresentInFile2 is false. 0 means "append at end".
	InsertAfterLine int `json:"insert_after_line"`
}

type section struct {
	Title     string
	Level     int
	StartLine int // 1-indexed line of heading
	EndLine   int // 1-indexed last content line included in this section
}

type sectionMatch struct {
	File1Index            int  `json:"file1_index"`
	Present               bool `json:"present"`
	File2Index            int  `json:"file2_index"`
	InsertAfterFile2Index int  `json:"insert_after_file2_index"`
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
					"file1_index":              map[string]any{"type": "integer", "minimum": 0},
					"present":                  map[string]any{"type": "boolean"},
					"file2_index":              map[string]any{"type": "integer", "minimum": 0},
					"insert_after_file2_index": map[string]any{"type": "integer", "minimum": 0},
				},
				"required": []string{"file1_index", "present", "file2_index", "insert_after_file2_index"},
			},
		},
	},
	"required": []string{"matches"},
}

// parseSections extracts every ## or ### heading and the line range it covers
// (until the next heading of equal or higher precedence).
func parseSections(content string) []section {
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

	out := make([]section, 0, len(heads))
	for i, h := range heads {
		end := len(lines)
		for j := i + 1; j < len(heads); j++ {
			if heads[j].level <= h.level {
				end = heads[j].line - 1
				break
			}
		}
		out = append(out, section{
			Title:     h.title,
			Level:     h.level,
			StartLine: h.line,
			EndLine:   end,
		})
	}
	return out
}

// snippet returns up to maxChars of body content, skipping headings and blank
// lines, starting just after the section's heading line.
func snippet(lines []string, sec section, maxChars int) string {
	var b strings.Builder
	for li := sec.StartLine + 1; li <= sec.EndLine && li <= len(lines); li++ {
		line := strings.TrimSpace(lines[li-1])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(line)
		if b.Len() >= maxChars {
			s := b.String()
			return s[:maxChars] + "..."
		}
	}
	return b.String()
}

func formatSections(label string, secs []section, lines []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s SECTIONS (%d total):\n", label, len(secs))
	for i, s := range secs {
		fmt.Fprintf(&b, "[%d] (H%d, lines %d-%d) %s\n", i, s.Level, s.StartLine, s.EndLine, s.Title)
		snip := snippet(lines, s, 220)
		if snip != "" {
			fmt.Fprintf(&b, "    > %s\n", snip)
		}
	}
	return b.String()
}

// Run produces a merge plan: for each section in file1Content, it identifies
// the corresponding section in file2Content (or where to insert if absent).
//
// Sections are extracted deterministically from the markdown headings; the LLM
// is asked only the semantic question of which file 1 section matches which
// file 2 section. This keeps the prompt small and avoids forcing the model to
// count line numbers across two long documents.
func Run(ctx context.Context, p llm.Provider, model, file1Content, file2Content string) ([]Mapping, error) {
	s1 := parseSections(file1Content)
	if len(s1) == 0 {
		return nil, fmt.Errorf("no ## or ### sections found in source file")
	}
	s2 := parseSections(file2Content)

	lines1 := strings.Split(file1Content, "\n")
	lines2 := strings.Split(file2Content, "\n")

	maxF1 := len(s1) - 1
	maxF2 := len(s2) - 1
	if maxF2 < 0 {
		maxF2 = 0
	}

	prompt := fmt.Sprintf(`You are a note-planning assistant. You are given the section outlines of two Markdown notes. For EACH section in FILE 1, decide whether FILE 2 covers the same topic.

Output one match record per FILE 1 section:
- file1_index: index of the FILE 1 section (0..%d)
- present: true if FILE 2 covers the same topic (even if worded or organised differently); false if absent
- when present is true: file2_index = matching FILE 2 section index (0..%d). Set insert_after_file2_index to 0 (it is ignored).
- when present is false: insert_after_file2_index = the FILE 2 section index AFTER which the missing content fits best logically. Use the index of the LAST FILE 2 section to mean "append at end". Set file2_index to 0 (it is ignored).

Match by topic, not by heading wording. A H2 in FILE 1 may correspond to a H2 in FILE 2 even if their titles differ. Output every FILE 1 section exactly once, in order. Reply with valid JSON only.

%s
%s`, maxF1, maxF2, formatSections("FILE 1", s1, lines1), formatSections("FILE 2", s2, lines2))

	raw, err := p.GenerateJSON(ctx, model, prompt, matchSchema)
	if err != nil {
		return nil, fmt.Errorf("plan generation failed: %w", err)
	}

	var resp matchResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	return toMappings(resp, s1, s2), nil
}

func toMappings(resp matchResponse, s1, s2 []section) []Mapping {
	seen := make(map[int]bool, len(s1))
	out := make([]Mapping, 0, len(s1))

	for _, m := range resp.Matches {
		if m.File1Index < 0 || m.File1Index >= len(s1) || seen[m.File1Index] {
			continue
		}
		seen[m.File1Index] = true
		out = append(out, mappingFor(m, s1, s2))
	}

	for i, sec := range s1 {
		if seen[i] {
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

	return out
}

func mappingFor(m sectionMatch, s1, s2 []section) Mapping {
	sec := s1[m.File1Index]
	mp := Mapping{
		Title:          sec.Title,
		File1Start:     sec.StartLine,
		File1End:       sec.EndLine,
		PresentInFile2: m.Present,
	}
	if m.Present {
		if m.File2Index >= 0 && m.File2Index < len(s2) {
			mp.File2Start = s2[m.File2Index].StartLine
			mp.File2End = s2[m.File2Index].EndLine
		}
		return mp
	}
	switch {
	case len(s2) == 0:
		mp.InsertAfterLine = 0
	case m.InsertAfterFile2Index >= len(s2)-1:
		mp.InsertAfterLine = 0
	default:
		mp.InsertAfterLine = s2[m.InsertAfterFile2Index].EndLine
	}
	return mp
}

// RenderMarkdown produces the plan as a human-readable Markdown file.
func RenderMarkdown(file1Name, file2Name string, mappings []Mapping, file2Lines []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Merge Plan: `%s` → `%s`\n\n", file1Name, file2Name)
	fmt.Fprintf(&b, "For each section in `%s`, this plan shows where the content maps to in `%s`.\n\n", file1Name, file2Name)
	fmt.Fprintf(&b, "---\n\n")

	for _, m := range mappings {
		if m.PresentInFile2 {
			fmt.Fprintf(&b, "## [PRESENT] %s\n\n", m.Title)
			fmt.Fprintf(&b, "- **Source** (`%s`): lines %d–%d\n", file1Name, m.File1Start, m.File1End)
			fmt.Fprintf(&b, "- **Target** (`%s`): lines %d–%d\n", file2Name, m.File2Start, m.File2End)
		} else {
			if m.InsertAfterLine == 0 {
				fmt.Fprintf(&b, "## [INSERT at end] %s\n\n", m.Title)
				fmt.Fprintf(&b, "- **Source** (`%s`): lines %d–%d\n", file1Name, m.File1Start, m.File1End)
				fmt.Fprintf(&b, "- **Target** (`%s`): append at end\n", file2Name)
			} else {
				fmt.Fprintf(&b, "## [INSERT after line %d] %s\n\n", m.InsertAfterLine, m.Title)
				fmt.Fprintf(&b, "- **Source** (`%s`): lines %d–%d\n", file1Name, m.File1Start, m.File1End)
				fmt.Fprintf(&b, "- **Target** (`%s`): insert after line %d", file2Name, m.InsertAfterLine)
				if m.InsertAfterLine > 0 && m.InsertAfterLine <= len(file2Lines) {
					context := strings.TrimSpace(file2Lines[m.InsertAfterLine-1])
					if context != "" {
						fmt.Fprintf(&b, " — `%s`", truncate(context, 80))
					}
				}
				fmt.Fprintf(&b, "\n")
			}
		}
		fmt.Fprintf(&b, "\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

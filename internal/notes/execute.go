package notes

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fingerprint/notetools/internal/llm"
)

const targetContextCharBudget = 5000

type ExecuteProgress struct {
	Step         int
	Total        int
	Title        string
	SourceStart  int
	SourceEnd    int
	TargetStart  int
	TargetEnd    int
	InsertAfter  int
	PresentInDst bool
}

func ExecutePlan(ctx context.Context, p llm.Provider, model string, doc PlanDocument, instructions string, notify func(ExecuteProgress)) error {
	sourceData, err := os.ReadFile(doc.SourcePath)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}
	targetData, err := os.ReadFile(doc.TargetPath)
	if err != nil {
		return fmt.Errorf("read target file: %w", err)
	}

	sourceLines := strings.Split(string(sourceData), "\n")
	targetLines := strings.Split(string(targetData), "\n")
	targetContext := buildTargetContext(string(targetData), targetContextCharBudget)
	mappings := append([]Mapping(nil), doc.Mappings...)

	for i, mapping := range mappings {
		sourceSnippet, err := sliceLines(sourceLines, mapping.File1Start, mapping.File1End)
		if err != nil {
			return fmt.Errorf("source section %q: %w", mapping.Title, err)
		}

		progress := ExecuteProgress{
			Step:         i + 1,
			Total:        len(mappings),
			Title:        mapping.Title,
			SourceStart:  mapping.File1Start,
			SourceEnd:    mapping.File1End,
			InsertAfter:  mapping.InsertAfterLine,
			PresentInDst: mapping.PresentInFile2,
		}

		if mapping.PresentInFile2 {
			start := mapping.File2Start
			end := mapping.File2End
			progress.TargetStart = start
			progress.TargetEnd = end
			if notify != nil {
				notify(progress)
			}

			targetSnippet, err := sliceLines(targetLines, start, end)
			if err != nil {
				return fmt.Errorf("target section %q: %w", mapping.Title, err)
			}

			merged, err := MergeWithOptions(ctx, p, model, sourceSnippet, targetSnippet, MergeOptions{
				Instructions:  instructions,
				TargetContext: targetContext,
			})
			if err != nil {
				return fmt.Errorf("merge %q failed: %w", mapping.Title, err)
			}

			replLines := strings.Split(merged, "\n")
			replacementEnd := start + len(replLines) - 1
			delta := len(replLines) - (end - start + 1)
			targetLines = replaceRange(targetLines, start, end, replLines)
			recalculateAfterReplace(mappings[i+1:], start, end, replacementEnd, delta)
		} else {
			insertAfter := len(targetLines)
			if mapping.InsertAfterLine > 0 {
				insertAfter = mapping.InsertAfterLine
			}
			progress.InsertAfter = insertAfter
			if notify != nil {
				notify(progress)
			}

			insertLines := strings.Split(sourceSnippet, "\n")
			targetLines = insertRange(targetLines, insertAfter, insertLines)
			recalculateAfterInsert(mappings[i+1:], insertAfter, len(insertLines))
		}

		if err := os.WriteFile(doc.TargetPath, []byte(strings.Join(targetLines, "\n")), 0o644); err != nil {
			return fmt.Errorf("write target file: %w", err)
		}
	}

	return nil
}

func buildTargetContext(content string, charBudget int) string {
	if charBudget <= 0 {
		return ""
	}

	lines := strings.Split(content, "\n")
	var headings []string
	var prose []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			headings = append(headings, trimmed)
			continue
		}
		if isContextProseLine(trimmed) {
			prose = append(prose, trimmed)
		}
	}

	var b strings.Builder
	if len(headings) > 0 {
		b.WriteString("Headings:\n")
		for _, heading := range headings {
			if !appendContextLine(&b, "- "+heading, charBudget) {
				return strings.TrimSpace(b.String())
			}
		}
	}

	if len(prose) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("Representative prose:\n")
		for _, line := range prose {
			if !appendContextLine(&b, line, charBudget) {
				break
			}
		}
	}

	return strings.TrimSpace(b.String())
}

func isContextProseLine(line string) bool {
	if strings.HasPrefix(line, "![") ||
		strings.HasPrefix(line, "|") ||
		strings.HasPrefix(line, "```") ||
		strings.HasPrefix(line, "$$") {
		return false
	}
	return true
}

func appendContextLine(b *strings.Builder, line string, charBudget int) bool {
	if b.Len()+len(line)+1 > charBudget {
		return false
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(line)
	return true
}

func sliceLines(lines []string, start, end int) (string, error) {
	if start <= 0 || end <= 0 {
		return "", fmt.Errorf("invalid line range %d-%d", start, end)
	}
	if start > len(lines) {
		return "", fmt.Errorf("start line %d exceeds file length (%d lines)", start, len(lines))
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return "", fmt.Errorf("invalid line range %d-%d", start, end)
	}
	return strings.Join(lines[start-1:end], "\n"), nil
}

func recalculateAfterReplace(mappings []Mapping, start, end, replacementEnd, delta int) {
	for i := range mappings {
		mapping := &mappings[i]
		if mapping.PresentInFile2 {
			mapping.File2Start = recalculateRangeStartAfterReplace(mapping.File2Start, start, end, delta)
			mapping.File2End = recalculateRangeEndAfterReplace(mapping.File2End, start, end, replacementEnd, delta)
			if mapping.File2End < mapping.File2Start {
				mapping.File2End = mapping.File2Start
			}
			continue
		}

		if mapping.InsertAfterLine == 0 {
			continue
		}
		mapping.InsertAfterLine = recalculatePointAfterReplace(mapping.InsertAfterLine, start, end, replacementEnd, delta)
	}
}

func recalculateRangeStartAfterReplace(line, start, end, delta int) int {
	switch {
	case line < start:
		return line
	case line <= end:
		return start
	default:
		return line + delta
	}
}

func recalculateRangeEndAfterReplace(line, start, end, replacementEnd, delta int) int {
	switch {
	case line < start:
		return line
	case line <= end:
		return replacementEnd
	default:
		return line + delta
	}
}

func recalculatePointAfterReplace(line, start, end, replacementEnd, delta int) int {
	switch {
	case line < start:
		return line
	case line <= end:
		return replacementEnd
	default:
		return line + delta
	}
}

func recalculateAfterInsert(mappings []Mapping, after, insertedLines int) {
	if insertedLines == 0 {
		return
	}

	for i := range mappings {
		mapping := &mappings[i]
		if mapping.PresentInFile2 {
			switch {
			case mapping.File2End <= after:
				continue
			case mapping.File2Start > after:
				mapping.File2Start += insertedLines
				mapping.File2End += insertedLines
			default:
				mapping.File2End += insertedLines
			}
			continue
		}

		if mapping.InsertAfterLine == 0 {
			continue
		}
		if mapping.InsertAfterLine >= after {
			mapping.InsertAfterLine += insertedLines
		}
	}
}

func replaceRange(lines []string, start, end int, replacement []string) []string {
	out := make([]string, 0, len(lines)-(end-start+1)+len(replacement))
	out = append(out, lines[:start-1]...)
	out = append(out, replacement...)
	out = append(out, lines[end:]...)
	return out
}

func insertRange(lines []string, after int, insertion []string) []string {
	if after <= 0 {
		out := make([]string, 0, len(insertion)+len(lines))
		out = append(out, insertion...)
		out = append(out, lines...)
		return out
	}
	if after >= len(lines) {
		out := make([]string, 0, len(lines)+len(insertion))
		out = append(out, lines...)
		out = append(out, insertion...)
		return out
	}

	out := make([]string, 0, len(lines)+len(insertion))
	out = append(out, lines[:after]...)
	out = append(out, insertion...)
	out = append(out, lines[after:]...)
	return out
}

package execute

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fingerprint/notetools/internal/llm"
	"github.com/fingerprint/notetools/internal/merge"
	"github.com/fingerprint/notetools/internal/plan"
)

type Progress struct {
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

type lineEdit struct {
	kind          string
	origStart     int
	origEnd       int
	currentStart  int
	currentEnd    int
	insertAfter   int
	insertedLines int
}

func Run(ctx context.Context, p llm.Provider, model string, doc plan.Document, instructions string, notify func(Progress)) error {
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
	edits := make([]lineEdit, 0, len(doc.Mappings))

	for i, mapping := range doc.Mappings {
		sourceSnippet, err := sliceLines(sourceLines, mapping.File1Start, mapping.File1End)
		if err != nil {
			return fmt.Errorf("source section %q: %w", mapping.Title, err)
		}

		progress := Progress{
			Step:         i + 1,
			Total:        len(doc.Mappings),
			Title:        mapping.Title,
			SourceStart:  mapping.File1Start,
			SourceEnd:    mapping.File1End,
			InsertAfter:  mapping.InsertAfterLine,
			PresentInDst: mapping.PresentInFile2,
		}

		if mapping.PresentInFile2 {
			start := resolveLine(edits, mapping.File2Start, false)
			end := resolveLine(edits, mapping.File2End, true)
			progress.TargetStart = start
			progress.TargetEnd = end
			if notify != nil {
				notify(progress)
			}

			targetSnippet, err := sliceLines(targetLines, start, end)
			if err != nil {
				return fmt.Errorf("target section %q: %w", mapping.Title, err)
			}

			merged, err := merge.Run(ctx, p, model, sourceSnippet, targetSnippet, instructions)
			if err != nil {
				return fmt.Errorf("merge %q failed: %w", mapping.Title, err)
			}

			replLines := strings.Split(merged, "\n")
			shiftTrackedReplacementsAfterReplace(edits, start, end, len(replLines))
			targetLines = replaceRange(targetLines, start, end, replLines)
			upsertReplaceEdit(&edits, mapping.File2Start, mapping.File2End, start, start+len(replLines)-1)
		} else {
			insertAfter := len(targetLines)
			if mapping.InsertAfterLine > 0 {
				insertAfter = resolveInsertAfter(edits, mapping.InsertAfterLine)
			}
			progress.InsertAfter = insertAfter
			if notify != nil {
				notify(progress)
			}

			merged, err := merge.Run(ctx, p, model, sourceSnippet, "", buildInsertInstructions(instructions))
			if err != nil {
				return fmt.Errorf("insert %q failed: %w", mapping.Title, err)
			}

			insertLines := strings.Split(merged, "\n")
			shiftTrackedReplacementsAfterInsert(edits, insertAfter, len(insertLines))
			targetLines = insertRange(targetLines, insertAfter, insertLines)
			edits = append(edits, lineEdit{
				kind:          "insert",
				insertAfter:   mapping.InsertAfterLine,
				insertedLines: len(insertLines),
			})
		}

		if err := os.WriteFile(doc.TargetPath, []byte(strings.Join(targetLines, "\n")), 0o644); err != nil {
			return fmt.Errorf("write target file: %w", err)
		}
	}

	return nil
}

func buildInsertInstructions(extra string) string {
	base := "SNIPPET 2 is intentionally empty because the target note does not yet cover this section. Preserve the structure and detail from SNIPPET 1 as a standalone section that can be inserted directly into the target note. Only adapt heading level or phrasing when needed to fit the target note's style."
	if extra == "" {
		return base
	}
	return base + " " + extra
}

func resolveLine(edits []lineEdit, original int, endOfRange bool) int {
	line := original
	for _, edit := range edits {
		switch edit.kind {
		case "replace":
			if original < edit.origStart {
				continue
			}
			if original > edit.origEnd {
				line += (edit.currentEnd - edit.currentStart + 1) - (edit.origEnd - edit.origStart + 1)
				continue
			}
			if endOfRange {
				return edit.currentEnd
			}
			return edit.currentStart
		case "insert":
			if original > edit.insertAfter {
				line += edit.insertedLines
			}
		}
	}
	return line
}

func resolveInsertAfter(edits []lineEdit, original int) int {
	line := original
	for _, edit := range edits {
		switch edit.kind {
		case "replace":
			if original < edit.origStart {
				continue
			}
			if original > edit.origEnd {
				line += (edit.currentEnd - edit.currentStart + 1) - (edit.origEnd - edit.origStart + 1)
				continue
			}
			return edit.currentEnd
		case "insert":
			if original >= edit.insertAfter {
				line += edit.insertedLines
			}
		}
	}
	return line
}

func upsertReplaceEdit(edits *[]lineEdit, origStart, origEnd, currentStart, currentEnd int) {
	for i := range *edits {
		edit := &(*edits)[i]
		if edit.kind == "replace" && edit.origStart == origStart && edit.origEnd == origEnd {
			edit.currentStart = currentStart
			edit.currentEnd = currentEnd
			return
		}
	}

	*edits = append(*edits, lineEdit{
		kind:         "replace",
		origStart:    origStart,
		origEnd:      origEnd,
		currentStart: currentStart,
		currentEnd:   currentEnd,
	})
}

func shiftTrackedReplacementsAfterReplace(edits []lineEdit, start, end, replacementLines int) {
	delta := replacementLines - (end - start + 1)
	if delta == 0 {
		return
	}

	for i := range edits {
		edit := &edits[i]
		if edit.kind != "replace" {
			continue
		}
		if edit.currentStart > end {
			edit.currentStart += delta
			edit.currentEnd += delta
		}
	}
}

func shiftTrackedReplacementsAfterInsert(edits []lineEdit, after, insertedLines int) {
	if insertedLines == 0 {
		return
	}

	for i := range edits {
		edit := &edits[i]
		if edit.kind != "replace" {
			continue
		}
		if edit.currentStart > after {
			edit.currentStart += insertedLines
			edit.currentEnd += insertedLines
			continue
		}
		if edit.currentEnd > after {
			edit.currentEnd += insertedLines
		}
	}
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

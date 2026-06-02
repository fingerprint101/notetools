package notes

import (
	"context"
	"fmt"
	"strings"

	"github.com/fingerprint/notetools/internal/llm"
)

func Summarize(ctx context.Context, p llm.Provider, model, note string) (string, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		return "", fmt.Errorf("note content is empty")
	}

	prompt := buildSummarizePrompt(note)

	result, err := p.Generate(ctx, model, prompt)
	if err != nil {
		return "", fmt.Errorf("summarization failed: %w", err)
	}

	result = strings.TrimSpace(result)
	if result == "" {
		return "", fmt.Errorf("model returned an empty summary")
	}

	return result + "\n", nil
}

func buildSummarizePrompt(note string) string {
	return fmt.Sprintf(`You are creating a lite version of university notes.

Task:
- Keep the same main section structure as the original note.
- For each section, write a TL;DR version of that section's content.
- Go straight to the point: keep the core concepts, definitions, formulas, mechanisms, comparisons, and conclusions.
- Keep enough detail that the result can be used to review the lecture without reopening the original note.
- Use Markdown headings that correspond to the original sections.
- Preserve the distinction between the slide/main-note explanation and recording-derived material.

Mandatory constraints:
- Do not reorganize the note into a different structure.
- Do not create extra sections that are not present in the original note.
- Do not omit a topic that appears in the original note's section; compress it instead.
- Preserve every Markdown image exactly as it appears in the source, including alt text and path.
- Keep each image near the summarized text it illustrates. Put an image on its own line if that keeps the summary readable.
- Preserve every Obsidian recording-reference callout exactly, including the [!NOTE] marker and Source line.
- When a recording-reference callout is followed by a recording transcript section, write a concise summary of that recording under the callout and its heading.
- Do not prefix recording summaries with labels like "TL;DR:" or "Summary:"; start directly with the recording content.
- Do not merge recording transcript details into the main slide/content section without the recording-reference callout; readers must be able to tell what came from recordings.
- Preserve important numeric values, named concepts, formulas, definitions, contrasts, and cause/effect explanations.
- Format math formulas for Obsidian Markdown: use $...$ for inline math and $$...$$ for display math.
- Do not add concepts that are not supported by the note.
- Return only the summary Markdown, with no preface or commentary.

Note:
<<<NOTE
%s
NOTE>>>`, note)
}

func markdownImages(line string) []string {
	var images []string
	for {
		start := strings.Index(line, "![")
		if start == -1 {
			return images
		}

		altEnd := strings.Index(line[start:], "](")
		if altEnd == -1 {
			return images
		}
		pathStart := start + altEnd + 2
		pathEnd := strings.Index(line[pathStart:], ")")
		if pathEnd == -1 {
			return images
		}

		end := pathStart + pathEnd + 1
		images = append(images, line[start:end])
		line = line[end:]
	}
}

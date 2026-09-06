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
	return fmt.Sprintf(`You are creating a study-focused version of university notes.

Task:
- Keep the original note structure recognizable, but make the output more useful for studying than a simple short summary.
- For each original section, write a concise but complete study summary of that section's content.
- Go straight to the point while preserving core concepts, definitions, formulas, mechanisms, comparisons, examples, and conclusions.
- Add learning aids where they genuinely help: explicit definitions, compact comparison tables, concrete examples, common-confusion notes, cheat-sheet bullets, and recap questions.
- Keep enough detail that the result can be used to review the lecture without reopening the original note.
- Use Markdown headings that correspond to the original sections, and add small local study-aid subsections only when useful.
- Preserve the distinction between the slide/main-note explanation and recording-derived material.

Mandatory constraints:
- Do not reorganize the note into a different structure.
- Do not invent unrelated top-level sections. Added study-aid subsections must support the nearby original material.
- Do not omit a topic that appears in the original note's section; compress it instead.
- Preserve every Markdown image exactly as it appears in the source, including alt text and path.
- Keep each image near the summarized text it illustrates. Put an image on its own line if that keeps the summary readable.
- Preserve every Obsidian recording-reference callout exactly, including the [!NOTE] marker and Source line.
- When a recording-reference callout is followed by a recording transcript section, write a concise summary of that recording under the callout and its heading.
- Do not prefix recording summaries with labels like "TL;DR:" or "Summary:"; start directly with the recording content.
- Do not merge recording transcript details into the main slide/content section without the recording-reference callout; readers must be able to tell what came from recordings.
- Preserve important numeric values, named concepts, formulas, definitions, contrasts, and cause/effect explanations.
- Preserve Italian spelling, apostrophes, and accents exactly when rewriting Italian terms or recording titles.
- Do not strip or replace Italian diacritics. Examples: write "perché", not "perche"; "più", not "piu"; "velocità", not "velocita"; "associatività", not "associativita"; "c'è", not "c'e"; "l'indirizzo", not "l indirizzo".
- Format math formulas for Obsidian Markdown: use $...$ for inline math and $$...$$ for display math.
- Do not add concepts that are not supported by the note.
- Return only the summary Markdown, with no preface or commentary.

Study-aid guidance:
- Definitions: define central concepts explicitly in simple language. Include what the concept is, why it matters, and what it is often confused with when relevant.
- Comparison tables: use compact Markdown tables when concepts differ along clear dimensions, such as latency, cost, locality, correctness guarantee, mapping rule, or when to use.
- Examples: include tiny worked examples for abstract mechanisms, formulas, address splits, protocols, or before/after optimizations.
- Common confusions: call out likely misunderstandings directly when the source material suggests them.
- Recap questions: add a short "Recap questions" subsection for long or exam-relevant sections. Questions should test understanding, comparison, calculations, or mechanism tracing. Include short answers when useful.
- Cheat-sheet bullets: for dense technical sections, add compact facts, formulas, rules, or "remember this" bullets.
- Do not force every study-aid type into every section; choose the smallest set that improves review value.

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

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

	prompt := fmt.Sprintf(`You are creating a lite version of university notes.

Task:
- Keep the same main section structure as the original note.
- For each section, write a TL;DR version of that section's content.
- Go straight to the point: keep the core concepts, definitions, formulas, mechanisms, comparisons, and conclusions.
- Keep enough detail that the result can be used to review the lecture without reopening the original note.
- Use Markdown headings that correspond to the original sections.

Mandatory constraints:
- Do not reorganize the note into a different structure.
- Do not create extra sections that are not present in the original note.
- Do not expand any section beyond the original section.
- Do not include too many examples; keep examples only when they are essential to understand the concept.
- Do not include image captions, slide references, or decorative source artifacts unless their content is essential.
- Do not omit a topic that appears in the original note's section; compress it instead.
- Preserve important numeric values, named concepts, formulas, definitions, contrasts, and cause/effect explanations.
- Format math formulas for Obsidian Markdown: use $...$ for inline math and $$...$$ for display math.
- Do not add concepts that are not supported by the note.
- Return only the summary Markdown, with no preface or commentary.

Note:
<<<NOTE
%s
NOTE>>>`, note)

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

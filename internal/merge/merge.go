package merge

import (
	"context"
	"fmt"

	"github.com/fingerprint/notetools/internal/llm"
)

const promptTemplate = `You are a note-merging assistant. You are given two snippets from different
people's notes on the same topic. Merge them into a single unified Markdown document.
Treat SNIPPET 1 as source material to integrate into SNIPPET 2, which is the target note whose
style, tone, and formatting conventions should be preserved by default.

CRITICAL RULES:
- Preserve ALL details from BOTH snippets. Do not summarize or condense.
- If both snippets cover the same point, keep the most detailed version and
  supplement it with any unique details from the other.
- If the snippets cover different subtopics, include both in a logical order.
- Preserve original formatting: headings, lists, tables, code blocks.
- Use consistent heading levels (adjust if the two snippets use different levels).
- Match the writing style of SNIPPET 2 unless an additional instruction says otherwise.
- Write in a unified single-author voice, as if the merged note directly explains the subject.
- Do not describe the content as coming from slides, notes, or another source unless that
  provenance is itself essential.
- Rewrite source-reporting phrasing into direct exposition. For example, avoid wording such as
  "the slide says", "the slides show", "the notes say", or "the author says".
- Do not add commentary, introductions, or conclusions that were not in the originals.
- If there are contradictions between the two sources, keep both versions and
  mark the conflict with a comment like: <!-- CONFLICT: source 1 says X, source 2 says Y -->
%s
--- SNIPPET 1 ---
%s

--- SNIPPET 2 ---
%s`

func buildPrompt(snippet1, snippet2, instructions string) string {
	extra := ""
	if instructions != "" {
		extra = fmt.Sprintf("\nAdditional instructions: %s\n", instructions)
	}

	return fmt.Sprintf(promptTemplate, extra, snippet1, snippet2)
}

func Run(ctx context.Context, p llm.Provider, model, snippet1, snippet2, instructions string) (string, error) {
	prompt := buildPrompt(snippet1, snippet2, instructions)
	return p.Generate(ctx, model, prompt)
}

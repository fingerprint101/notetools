package merge

import (
	"context"
	"fmt"

	"github.com/fingerprint/notetools/internal/llm"
)

const promptTemplate = `You are a note-merging assistant. You are given two snippets from different
people's notes on the same topic. Merge them into a single unified Markdown document.

CRITICAL RULES:
- Preserve ALL details from BOTH snippets. Do not summarize or condense.
- If both snippets cover the same point, keep the most detailed version and
  supplement it with any unique details from the other.
- If the snippets cover different subtopics, include both in a logical order.
- Preserve original formatting: headings, lists, tables, code blocks.
- Use consistent heading levels (adjust if the two snippets use different levels).
- Do not add commentary, introductions, or conclusions that were not in the originals.
- If there are contradictions between the two sources, keep both versions and
  mark the conflict with a comment like: <!-- CONFLICT: source 1 says X, source 2 says Y -->
%s
--- SNIPPET 1 ---
%s

--- SNIPPET 2 ---
%s`

func Run(ctx context.Context, p llm.Provider, model, snippet1, snippet2, instructions string) (string, error) {
	extra := ""
	if instructions != "" {
		extra = fmt.Sprintf("\nAdditional instructions: %s\n", instructions)
	}

	prompt := fmt.Sprintf(promptTemplate, extra, snippet1, snippet2)
	return p.Generate(ctx, model, prompt)
}

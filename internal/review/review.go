package review

import (
	"context"
	"fmt"

	"github.com/fingerprint/notetools/internal/ollama"
)

const textModel = "gemma4:e4b"

const promptTemplate = `You are a study assistant reviewing AI-generated notes or transcripts.
Review the following Markdown document for:
- Factual inconsistencies or contradictions within the text
- Formatting issues (broken tables, malformed lists, garbled text suggesting OCR errors)
- Missing or cut-off sections
- Repeated content or duplicate passages
- In transcripts: speaker confusion, missing punctuation, run-on sentences

Output a Markdown report with:
1. A brief summary of overall quality (1-2 sentences)
2. A bullet list of specific issues found, each with a short quote of the problematic text and a suggested fix
3. If no issues are found, say so explicitly

Document to review:
---
%s`

// Run sends a document to the local text model for review and returns the output.
func Run(ctx context.Context, content string) (string, error) {
	prompt := fmt.Sprintf(promptTemplate, content)
	return ollama.Generate(ctx, textModel, prompt)
}

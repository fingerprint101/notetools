package review

import (
	"fmt"
	"os/exec"
)

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

// RunClaude sends a document to the claude CLI for review and returns the output.
func RunClaude(content string) (string, error) {
	if _, err := exec.LookPath("claude"); err != nil {
		return "", fmt.Errorf("'claude' not found in PATH; install: npm install -g @anthropic-ai/claude-code")
	}

	prompt := fmt.Sprintf(promptTemplate, content)
	cmd := exec.Command("claude", "-p", prompt, "--output-format", "text")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude failed: %w", err)
	}

	return string(out), nil
}

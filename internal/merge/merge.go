package merge

import (
	"fmt"
	"os/exec"
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

// RunClaude sends two snippets to the claude CLI for merging and returns the merged output.
func RunClaude(snippet1, snippet2, instructions string) (string, error) {
	if _, err := exec.LookPath("claude"); err != nil {
		return "", fmt.Errorf("'claude' not found in PATH; install: npm install -g @anthropic-ai/claude-code")
	}

	extra := ""
	if instructions != "" {
		extra = fmt.Sprintf("\nAdditional instructions: %s\n", instructions)
	}

	prompt := fmt.Sprintf(promptTemplate, extra, snippet1, snippet2)
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

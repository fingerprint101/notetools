package clean

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Section struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type sectionsResponse struct {
	Sections []Section `json:"sections"`
}

type cleanedResponse struct {
	CleanedContent string `json:"cleaned_content"`
}

var sectionSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]any{
		"sections": map[string]any{
			"type":     "array",
			"minItems": 1,
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"title":   map[string]any{"type": "string", "minLength": 1},
					"content": map[string]any{"type": "string", "minLength": 1},
				},
				"required": []string{"title", "content"},
			},
		},
	},
	"required": []string{"sections"},
}

var cleanSectionSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"properties": map[string]any{
		"cleaned_content": map[string]any{"type": "string", "minLength": 1},
	},
	"required": []string{"cleaned_content"},
}

func runClaude(prompt string, schema map[string]any) (string, error) {
	if _, err := exec.LookPath("claude"); err != nil {
		return "", fmt.Errorf("'claude' not found in PATH; install: npm install -g @anthropic-ai/claude-code")
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("failed to marshal schema: %w", err)
	}

	cmd := exec.Command("claude", "-p", prompt, "--bare", "--json-schema", string(schemaJSON))
	cmd.Env = filterEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude exited with code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude failed: %w", err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return "", fmt.Errorf("claude returned empty output")
	}
	return output, nil
}

func filterEnv(environ []string) []string {
	filtered := make([]string, 0, len(environ))
	for _, e := range environ {
		key, _, _ := strings.Cut(e, "=")
		switch key {
		case "CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "VIRTUAL_ENV":
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

// extractJSON attempts to find valid JSON in the response, stripping markdown fences if present.
func extractJSON(text string) (string, error) {
	stripped := strings.TrimSpace(text)

	// Strip markdown code fences.
	if lines := strings.Split(stripped, "\n"); len(lines) >= 3 &&
		strings.HasPrefix(lines[0], "```") && strings.HasPrefix(lines[len(lines)-1], "```") {
		stripped = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
	}

	if json.Valid([]byte(stripped)) {
		return stripped, nil
	}

	// Try extracting the outermost braces.
	start := strings.Index(stripped, "{")
	end := strings.LastIndex(stripped, "}")
	if start != -1 && end > start {
		candidate := stripped[start : end+1]
		if json.Valid([]byte(candidate)) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("claude did not return valid JSON")
}

// SectionTranscript splits a transcript into thematic sections via Claude.
func SectionTranscript(transcript string) ([]Section, error) {
	prompt := fmt.Sprintf(`You are organizing an automatic transcript of an Italian university lecture.

Task:
- Split the transcript into coherent thematic sections.

Mandatory constraints:
- Do not summarize.
- Do not correct the text.
- Do not remove anything.
- Do not add anything.
- Preserve the original order of the lecture.
- Each section must contain a contiguous block of the original text.
- The combined content of all sections must cover the full transcript.
- Give each section a short title in Italian, using correct Italian spelling, including accents when needed.
- Prefer a fine-grained structure rather than a small number of broad sections.
- Create specific sections whenever the speaker changes subtopic, example, system category, comparison, or teaching focus.
- Do not merge clearly different subtopics into the same section.
- Prefer more sections over fewer sections when in doubt.
- For long transcripts, prefer at least 8 to 15 sections unless the transcript is genuinely very uniform.
- Avoid sections that are overly broad or that cover multiple major ideas.
- Reply only with valid JSON matching the required schema.

Transcript:
<<<TRANSCRIPT
%s
TRANSCRIPT>>>
`, transcript)

	raw, err := runClaude(prompt, sectionSchema)
	if err != nil {
		return nil, fmt.Errorf("sectioning failed: %w", err)
	}

	jsonText, err := extractJSON(raw)
	if err != nil {
		return nil, err
	}

	var resp sectionsResponse
	if err := json.Unmarshal([]byte(jsonText), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse sections JSON: %w", err)
	}

	if len(resp.Sections) == 0 {
		return nil, fmt.Errorf("claude returned an empty section list")
	}

	for i := range resp.Sections {
		resp.Sections[i].Title = strings.TrimSpace(resp.Sections[i].Title)
		resp.Sections[i].Content = strings.TrimSpace(resp.Sections[i].Content)
	}

	return resp.Sections, nil
}

// CleanSection sends a single section to Claude for cleanup and returns the cleaned text.
func CleanSection(title, content string) (string, error) {
	prompt := fmt.Sprintf(`You are cleaning a single section from an automatic transcript of an Italian university lecture.

Task:
- Work only on the provided section.
- Make the discourse coherent and readable.
- Remove obvious transcription noise, spurious repetitions, and fragments that are clearly meaningless.
- Correct phrases or terms that were clearly misheard by the transcriber when the technical context makes the intended meaning evident.
- Fix punctuation and sentence boundaries.

Mandatory constraints:
- Do not change the topic.
- Do not add new concepts that are not supported by the text.
- Preserve all substantial information present in the section.
- Return the cleaned text in correct Italian, using proper accents and apostrophes where needed.
- Do not include the title, notes, comments, or markdown.
- Reply only with valid JSON matching the required schema.

Section title:
%s

Section text:
<<<SECTION
%s
SECTION>>>
`, title, content)

	raw, err := runClaude(prompt, cleanSectionSchema)
	if err != nil {
		return "", fmt.Errorf("cleaning section %q failed: %w", title, err)
	}

	jsonText, err := extractJSON(raw)
	if err != nil {
		return "", err
	}

	var resp cleanedResponse
	if err := json.Unmarshal([]byte(jsonText), &resp); err != nil {
		return "", fmt.Errorf("failed to parse cleaned section JSON: %w", err)
	}

	cleaned := strings.TrimSpace(resp.CleanedContent)
	if cleaned == "" {
		return "", fmt.Errorf("claude returned empty cleaned content for section %q", title)
	}

	return cleaned, nil
}

// RenderMarkdown assembles cleaned sections into a final Markdown document.
func RenderMarkdown(docTitle string, sections []Section) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", docTitle)
	for _, s := range sections {
		fmt.Fprintf(&b, "## %s\n%s\n\n", s.Title, strings.TrimSpace(s.Content))
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

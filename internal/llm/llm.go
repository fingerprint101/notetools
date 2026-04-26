package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Provider interface {
	Name() string
	Generate(ctx context.Context, model, prompt string) (string, error)
	GenerateWithImage(ctx context.Context, model, prompt, imagePath string) (string, error)
	GenerateWithImages(ctx context.Context, model, prompt string, imagePaths []string) (string, error)
	GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (string, error)
	GenerateJSONWithImages(ctx context.Context, model, prompt string, imagePaths []string, schema map[string]any) (string, error)
}

// JSONPromptSuffix is the boilerplate appended to JSON prompts for CLI-based
// providers that don't support schema-constrained generation natively.
func JSONPromptSuffix(schema map[string]any) string {
	b, _ := json.Marshal(schema)
	return fmt.Sprintf("\n\nYou MUST respond with valid JSON matching this schema. Output ONLY valid JSON, no markdown fences, no commentary.\nSchema: %s", string(b))
}

// ExtractJSON tries to pull a valid JSON object out of a model's raw text
// response. It strips markdown fences and falls back to the outermost braces.
func ExtractJSON(text string) string {
	s := strings.TrimSpace(text)

	if lines := strings.Split(s, "\n"); len(lines) >= 2 && strings.HasPrefix(lines[0], "```") {
		end := len(lines)
		if strings.HasPrefix(lines[end-1], "```") {
			end--
		}
		s = strings.TrimSpace(strings.Join(lines[1:end], "\n"))
	}

	if json.Valid([]byte(s)) {
		return s
	}

	start := strings.Index(s, "{")
	last := strings.LastIndex(s, "}")
	if start != -1 && last > start {
		candidate := s[start : last+1]
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}
	return s
}

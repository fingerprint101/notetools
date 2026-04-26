package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/fingerprint/notetools/internal/llm"
)

type GeminiClient struct{}

type geminiHeadlessResponse struct {
	Response string `json:"response"`
	Error    any    `json:"error"`
}

func NewGemini() *GeminiClient {
	return &GeminiClient{}
}

func (c *GeminiClient) Name() string {
	return "gemini"
}

func runGemini(ctx context.Context, model, prompt string) (string, error) {
	args := []string{"--output-format", "json"}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", model)
	}

	cmd := exec.CommandContext(ctx, "gemini", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader(prompt)

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("gemini run: %w\n%s", err, msg)
		}
		return "", fmt.Errorf("gemini run: %w", err)
	}

	var resp geminiHeadlessResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err == nil {
		if resp.Error != nil && strings.TrimSpace(resp.Response) == "" {
			return "", fmt.Errorf("gemini returned an error response: %v", resp.Error)
		}
		return strings.TrimSpace(resp.Response), nil
	}

	return strings.TrimSpace(stdout.String()), nil
}

func withDirectFileContext(prompt string, paths []string) string {
	if len(paths) == 0 {
		return prompt
	}

	var b strings.Builder
	b.WriteString("Read these local files in order as part of the prompt context:\n")
	for _, path := range paths {
		fmt.Fprintf(&b, "@%s\n", path)
	}
	b.WriteString("\n")
	b.WriteString(prompt)
	return b.String()
}

func (c *GeminiClient) Generate(ctx context.Context, model, prompt string) (string, error) {
	return runGemini(ctx, model, prompt)
}

func (c *GeminiClient) GenerateWithImage(ctx context.Context, model, prompt, imagePath string) (string, error) {
	return c.GenerateWithImages(ctx, model, prompt, []string{imagePath})
}

func (c *GeminiClient) GenerateWithImages(ctx context.Context, model, prompt string, imagePaths []string) (string, error) {
	return runGemini(ctx, model, withDirectFileContext(prompt, imagePaths))
}

func (c *GeminiClient) GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (string, error) {
	raw, err := runGemini(ctx, model, prompt+llm.JSONPromptSuffix(schema))
	if err != nil {
		return "", err
	}
	return llm.ExtractJSON(raw), nil
}

func (c *GeminiClient) GenerateJSONWithImages(ctx context.Context, model, prompt string, imagePaths []string, schema map[string]any) (string, error) {
	raw, err := runGemini(ctx, model, withDirectFileContext(prompt+llm.JSONPromptSuffix(schema), imagePaths))
	if err != nil {
		return "", err
	}
	return llm.ExtractJSON(raw), nil
}

var _ llm.Provider = (*GeminiClient)(nil)

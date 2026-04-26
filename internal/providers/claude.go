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

type ClaudeClient struct{}

func NewClaude() *ClaudeClient {
	return &ClaudeClient{}
}

func (c *ClaudeClient) Name() string {
	return "claude"
}

func runClaude(ctx context.Context, model, prompt string, imagePaths []string, schema map[string]any) (string, error) {
	args := []string{"-p"}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", model)
	}
	if len(imagePaths) > 0 {
		args = append(args, "--allowedTools", "Read")
	}
	for _, imagePath := range imagePaths {
		args = append(args, "--add-dir", imagePath)
	}
	if schema != nil {
		schemaJSON, err := json.Marshal(schema)
		if err != nil {
			return "", fmt.Errorf("marshal schema: %w", err)
		}
		args = append(args, "--output-format", "json", "--json-schema", string(schemaJSON))
	}

	cmd := exec.CommandContext(ctx, "claude", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader(prompt)

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("claude run: %w\n%s", err, msg)
		}
		return "", fmt.Errorf("claude run: %w", err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func promptWithImages(prompt string, imagePaths []string) string {
	if len(imagePaths) == 0 {
		return prompt
	}
	var b strings.Builder
	b.WriteString(prompt)
	b.WriteString("\n\nImages to analyze (read them in order):\n")
	for _, p := range imagePaths {
		fmt.Fprintf(&b, "- %s\n", p)
	}
	return b.String()
}

func (c *ClaudeClient) Generate(ctx context.Context, model, prompt string) (string, error) {
	return runClaude(ctx, model, prompt, nil, nil)
}

func (c *ClaudeClient) GenerateWithImage(ctx context.Context, model, prompt, imagePath string) (string, error) {
	return c.GenerateWithImages(ctx, model, prompt, []string{imagePath})
}

func (c *ClaudeClient) GenerateWithImages(ctx context.Context, model, prompt string, imagePaths []string) (string, error) {
	return runClaude(ctx, model, promptWithImages(prompt, imagePaths), imagePaths, nil)
}

func (c *ClaudeClient) GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (string, error) {
	raw, err := runClaude(ctx, model, prompt, nil, schema)
	if err != nil {
		return "", err
	}
	return extractClaudeJSON(raw, schema)
}

func (c *ClaudeClient) GenerateJSONWithImages(ctx context.Context, model, prompt string, imagePaths []string, schema map[string]any) (string, error) {
	raw, err := runClaude(ctx, model, promptWithImages(prompt, imagePaths), imagePaths, schema)
	if err != nil {
		return "", err
	}
	return extractClaudeJSON(raw, schema)
}

func extractClaudeJSON(raw string, schema map[string]any) (string, error) {
	if schema == nil {
		return llm.ExtractJSON(raw), nil
	}
	type claudeJSONResponse struct {
		Result string `json:"result"`
	}
	var resp claudeJSONResponse
	if err := json.Unmarshal([]byte(raw), &resp); err == nil && strings.TrimSpace(resp.Result) != "" {
		return llm.ExtractJSON(resp.Result), nil
	}
	return llm.ExtractJSON(raw), nil
}

var _ llm.Provider = (*ClaudeClient)(nil)

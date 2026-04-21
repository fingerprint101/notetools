package providers

import (
	"bytes"
	"context"
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

func runClaude(ctx context.Context, model, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt, "--model", model)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

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

func runClaudeWithImages(ctx context.Context, model, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt, "--model", model, "--allowedTools", "Read")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("claude run: %w\n%s", err, msg)
		}
		return "", fmt.Errorf("claude run: %w", err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c *ClaudeClient) Generate(ctx context.Context, model, prompt string) (string, error) {
	return runClaude(ctx, model, prompt)
}

func (c *ClaudeClient) GenerateWithImage(ctx context.Context, model, prompt, imagePath string) (string, error) {
	return c.GenerateWithImages(ctx, model, prompt, []string{imagePath})
}

func (c *ClaudeClient) GenerateWithImages(ctx context.Context, model, prompt string, imagePaths []string) (string, error) {
	return runClaudeWithImages(ctx, model, promptWithImages(prompt, imagePaths))
}

func (c *ClaudeClient) GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (string, error) {
	raw, err := runClaude(ctx, model, prompt+llm.JSONPromptSuffix(schema))
	if err != nil {
		return "", err
	}
	return llm.ExtractJSON(raw), nil
}

var _ llm.Provider = (*ClaudeClient)(nil)

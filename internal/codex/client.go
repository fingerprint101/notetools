package codex

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/fingerprint/notetools/internal/llm"
)

type Client struct{}

func New() *Client {
	return &Client{}
}

func (c *Client) Name() string {
	return "codex"
}

func run(ctx context.Context, model, prompt string) (string, error) {
	cmd := exec.CommandContext(
		ctx,
		"codex",
		"exec",
		"--ephemeral",
		"--skip-git-repo-check",
		"--model",
		model,
		prompt,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("codex run: %w\n%s", err, msg)
		}
		return "", fmt.Errorf("codex run: %w", err)
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

func (c *Client) Generate(ctx context.Context, model, prompt string) (string, error) {
	return run(ctx, model, prompt)
}

func (c *Client) GenerateWithImage(ctx context.Context, model, prompt, imagePath string) (string, error) {
	return c.GenerateWithImages(ctx, model, prompt, []string{imagePath})
}

func (c *Client) GenerateWithImages(ctx context.Context, model, prompt string, imagePaths []string) (string, error) {
	return run(ctx, model, promptWithImages(prompt, imagePaths))
}

func (c *Client) GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (string, error) {
	raw, err := run(ctx, model, prompt+llm.JSONPromptSuffix(schema))
	if err != nil {
		return "", err
	}
	return llm.ExtractJSON(raw), nil
}

var _ llm.Provider = (*Client)(nil)

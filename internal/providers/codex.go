package providers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/fingerprint/notetools/internal/llm"
)

type CodexClient struct{}

func NewCodex() *CodexClient {
	return &CodexClient{}
}

func (c *CodexClient) Name() string {
	return "codex"
}

func runCodex(ctx context.Context, model, prompt string) (string, error) {
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

func (c *CodexClient) Generate(ctx context.Context, model, prompt string) (string, error) {
	return runCodex(ctx, model, prompt)
}

func (c *CodexClient) GenerateWithImage(ctx context.Context, model, prompt, imagePath string) (string, error) {
	return c.GenerateWithImages(ctx, model, prompt, []string{imagePath})
}

func (c *CodexClient) GenerateWithImages(ctx context.Context, model, prompt string, imagePaths []string) (string, error) {
	return runCodex(ctx, model, promptWithImages(prompt, imagePaths))
}

func (c *CodexClient) GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (string, error) {
	raw, err := runCodex(ctx, model, prompt+llm.JSONPromptSuffix(schema))
	if err != nil {
		return "", err
	}
	return llm.ExtractJSON(raw), nil
}

var _ llm.Provider = (*CodexClient)(nil)

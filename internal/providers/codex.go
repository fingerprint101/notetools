package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
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

func runCodex(ctx context.Context, model, prompt string, imagePaths []string, schema map[string]any) (string, error) {
	args := []string{
		"exec",
		"--ephemeral",
		"--skip-git-repo-check",
	}
	if strings.TrimSpace(model) != "" {
		args = append(args, "--model", model)
	}
	for _, imagePath := range imagePaths {
		args = append(args, "--image", imagePath)
	}

	var schemaPath string
	if schema != nil {
		schemaJSON, err := json.Marshal(schema)
		if err != nil {
			return "", fmt.Errorf("marshal schema: %w", err)
		}

		f, err := os.CreateTemp("", "nt-codex-schema-*.json")
		if err != nil {
			return "", fmt.Errorf("create codex schema file: %w", err)
		}
		if _, err := f.Write(schemaJSON); err != nil {
			f.Close()
			os.Remove(f.Name())
			return "", fmt.Errorf("write codex schema file: %w", err)
		}
		if err := f.Close(); err != nil {
			os.Remove(f.Name())
			return "", fmt.Errorf("close codex schema file: %w", err)
		}
		schemaPath = f.Name()
		defer os.Remove(schemaPath)
		args = append(args, "--output-schema", schemaPath)
	}

	args = append(args, "-")

	cmd := exec.CommandContext(ctx, "codex", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader(prompt)

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("codex run: %w\n%s", err, msg)
		}
		return "", fmt.Errorf("codex run: %w", err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c *CodexClient) Generate(ctx context.Context, model, prompt string) (string, error) {
	return runCodex(ctx, model, prompt, nil, nil)
}

func (c *CodexClient) GenerateWithImage(ctx context.Context, model, prompt, imagePath string) (string, error) {
	return c.GenerateWithImages(ctx, model, prompt, []string{imagePath})
}

func (c *CodexClient) GenerateWithImages(ctx context.Context, model, prompt string, imagePaths []string) (string, error) {
	return runCodex(ctx, model, prompt, imagePaths, nil)
}

func (c *CodexClient) GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (string, error) {
	raw, err := runCodex(ctx, model, prompt, nil, schema)
	if err != nil {
		return "", err
	}
	return llm.ExtractJSON(raw), nil
}

var _ llm.Provider = (*CodexClient)(nil)

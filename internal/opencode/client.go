package opencode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type Client struct{}

func New() *Client {
	return &Client{}
}

func (c *Client) Name() string {
	return "opencode"
}

type event struct {
	Type string `json:"type"`
	Part struct {
		Text string `json:"text"`
	} `json:"part"`
}

func runOpenCode(ctx context.Context, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, "opencode", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("opencode run: %w\n%s", err, msg)
		}
		return "", fmt.Errorf("opencode run: %w", err)
	}

	var result strings.Builder
	decoder := json.NewDecoder(&stdout)
	for {
		var ev event
		if err := decoder.Decode(&ev); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		if ev.Type == "text" && ev.Part.Text != "" {
			result.WriteString(ev.Part.Text)
		}
	}

	return result.String(), nil
}

func (c *Client) Generate(ctx context.Context, model, prompt string) (string, error) {
	return runOpenCode(ctx, []string{
		"run", "-m", model, "--format", "json", prompt,
	})
}

func (c *Client) GenerateWithImage(ctx context.Context, model, prompt, imagePath string) (string, error) {
	return c.GenerateWithImages(ctx, model, prompt, []string{imagePath})
}

func (c *Client) GenerateWithImages(ctx context.Context, model, prompt string, imagePaths []string) (string, error) {
	args := []string{
		"run", "-m", model, "--format", "json", prompt,
	}
	for _, p := range imagePaths {
		args = append(args, "-f", p)
	}
	return runOpenCode(ctx, args)
}

func (c *Client) GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (string, error) {
	args := []string{
		"run", "-m", model, "--format", "json",
	}

	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("marshal schema: %w", err)
	}

	jsonPrompt := fmt.Sprintf(
		"%s\n\nYou MUST respond with valid JSON matching this schema. Output ONLY valid JSON, no markdown fences, no commentary.\nSchema: %s",
		prompt, string(schemaJSON),
	)

	args = append(args, jsonPrompt)

	raw, err := runOpenCode(ctx, args)
	if err != nil {
		return "", err
	}

	return extractJSON(raw), nil
}

func extractJSON(text string) string {
	stripped := strings.TrimSpace(text)

	scanner := bufio.NewScanner(strings.NewReader(stripped))
	inFence := false
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "```") {
			if inFence {
				inFence = false
				continue
			}
			inFence = true
			continue
		}
		if inFence {
			lines = append(lines, line)
		}
	}
	if len(lines) > 0 {
		stripped = strings.Join(lines, "\n")
	}

	if json.Valid([]byte(stripped)) {
		return stripped
	}

	start := strings.Index(stripped, "{")
	end := strings.LastIndex(stripped, "}")
	if start != -1 && end > start {
		candidate := stripped[start : end+1]
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}

	return stripped
}

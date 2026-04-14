package ollama

import (
	"context"

	"github.com/fingerprint/notetools/internal/llm"
)

type Client struct{}

func New() *Client {
	return &Client{}
}

func (c *Client) Generate(ctx context.Context, model, prompt string) (string, error) {
	return Generate(ctx, model, prompt)
}

func (c *Client) GenerateWithImage(ctx context.Context, model, prompt, imagePath string) (string, error) {
	return GenerateWithImage(ctx, model, prompt, imagePath)
}

func (c *Client) GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (string, error) {
	return GenerateJSON(ctx, model, prompt, schema)
}

var _ llm.Provider = (*Client)(nil)

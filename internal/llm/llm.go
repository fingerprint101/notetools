package llm

import "context"

type Provider interface {
	Generate(ctx context.Context, model, prompt string) (string, error)
	GenerateWithImage(ctx context.Context, model, prompt, imagePath string) (string, error)
	GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (string, error)
}

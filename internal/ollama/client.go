package ollama

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

const defaultHost = "http://localhost:11434"

var hostOverride string

// ErrNotRunning is returned when the ollama daemon is unreachable.
var ErrNotRunning = errors.New("ollama is not running — start with: ollama serve")

// SetHost overrides the ollama base URL for all subsequent calls.
// An empty string resets to the default (OLLAMA_HOST env var or localhost:11434).
func SetHost(h string) {
	hostOverride = h
}

func baseURL() string {
	if hostOverride != "" {
		return strings.TrimRight(hostOverride, "/")
	}
	if h := os.Getenv("OLLAMA_HOST"); h != "" {
		return strings.TrimRight(h, "/")
	}
	return defaultHost
}

type generateRequest struct {
	Model  string         `json:"model"`
	Prompt string         `json:"prompt"`
	Stream bool           `json:"stream"`
	Images []string       `json:"images,omitempty"`
	Format map[string]any `json:"format,omitempty"`
}

type generateResponse struct {
	Response string `json:"response"`
}

func post(ctx context.Context, body generateRequest) (string, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL()+"/api/generate", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if isConnRefused(err) {
			return "", ErrNotRunning
		}
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var gr generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return gr.Response, nil
}

func isConnRefused(err error) bool {
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return strings.Contains(netErr.Error(), "connection refused")
	}
	return false
}

// Generate sends a plain-text prompt to the model and returns the response.
func Generate(ctx context.Context, model, prompt string) (string, error) {
	return post(ctx, generateRequest{
		Model:  model,
		Prompt: prompt,
	})
}

// GenerateWithImage sends a prompt with an image file to the model.
// imagePath must be a readable PNG file; it is base64-encoded before sending.
func GenerateWithImage(ctx context.Context, model, prompt, imagePath string) (string, error) {
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image %s: %w", imagePath, err)
	}
	encoded := base64.StdEncoding.EncodeToString(imgData)
	return post(ctx, generateRequest{
		Model:  model,
		Prompt: prompt,
		Images: []string{encoded},
	})
}

// GenerateJSON sends a prompt with a JSON schema constraint and returns the raw JSON string.
func GenerateJSON(ctx context.Context, model, prompt string, schema map[string]any) (string, error) {
	return post(ctx, generateRequest{
		Model:  model,
		Prompt: prompt,
		Format: schema,
	})
}

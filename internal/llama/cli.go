package llama

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// RunOpts configures a single llama-mtmd-cli invocation.
type RunOpts struct {
	ModelPath  string
	MMProjPath string
	Prompt     string
	ImagePath  string // for vision models
	AudioPath  string // for audio models
	GPULayers  int    // -1 = all
	CtxSize    int    // 0 = default
	MaxTokens  int    // 0 = default
	Verbose    bool
}

// findBinary locates the llama-mtmd-cli executable.
func findBinary() (string, error) {
	if p := os.Getenv("NOTETOOLS_LLAMA_BIN"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	p, err := exec.LookPath("llama-mtmd-cli")
	if err != nil {
		return "", fmt.Errorf("llama-mtmd-cli not found in PATH; install llama.cpp or set NOTETOOLS_LLAMA_BIN")
	}
	return p, nil
}

// Run invokes llama-mtmd-cli and returns the model's text output.
func Run(opts RunOpts) (string, error) {
	bin, err := findBinary()
	if err != nil {
		return "", err
	}

	args := []string{
		"-m", opts.ModelPath,
		"-p", opts.Prompt,
	}

	if opts.MMProjPath != "" {
		args = append(args, "--mmproj", opts.MMProjPath)
	}

	if opts.ImagePath != "" {
		args = append(args, "--image", opts.ImagePath)
	}
	if opts.AudioPath != "" {
		args = append(args, "--audio", opts.AudioPath)
	}

	ngl := opts.GPULayers
	if ngl < 0 {
		ngl = 99999 // offload all layers
	}
	args = append(args, "-ngl", strconv.Itoa(ngl))

	if opts.CtxSize > 0 {
		args = append(args, "-c", strconv.Itoa(opts.CtxSize))
	}
	if opts.MaxTokens > 0 {
		args = append(args, "-n", strconv.Itoa(opts.MaxTokens))
	}

	cmd := exec.Command(bin, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout

	if opts.Verbose {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = &stderr
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("llama-mtmd-cli failed: %w\n%s", err, stderr.String())
	}

	return stdout.String(), nil
}

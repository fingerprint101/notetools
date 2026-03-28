package llama

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

const hfBaseURL = "https://huggingface.co"

// ModelFile describes a single file to download from HuggingFace.
type ModelFile struct {
	Repo     string // e.g. "ggml-org/GLM-OCR-GGUF"
	Filename string // e.g. "GLM-OCR-Q8_0.gguf"
}

// Model describes a complete model (main weights + optional multimodal projector).
type Model struct {
	Name   string
	Model  ModelFile
	MMProj *ModelFile // nil if projector is embedded in the model
}

var GLMOcr = Model{
	Name: "glm-ocr",
	Model: ModelFile{
		Repo:     "ggml-org/GLM-OCR-GGUF",
		Filename: "GLM-OCR-Q8_0.gguf",
	},
	MMProj: &ModelFile{
		Repo:     "ggml-org/GLM-OCR-GGUF",
		Filename: "mmproj-GLM-OCR-Q8_0.gguf",
	},
}

var Voxtral = Model{
	Name: "voxtral",
	Model: ModelFile{
		Repo:     "andrijdavid/Voxtral-Mini-4B-Realtime-2602-GGUF",
		Filename: "Q8_0.gguf",
	},
	MMProj: nil, // audio encoder is embedded in the model
}

var AllModels = []Model{GLMOcr, Voxtral}

// ModelsDir returns the directory where models are stored.
func ModelsDir() string {
	if d := os.Getenv("NOTETOOLS_DATA_DIR"); d != "" {
		return filepath.Join(d, "models")
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "notetools", "models")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "notetools", "models")
}

// ModelPath returns the local path for a model file.
func (m ModelFile) LocalPath() string {
	return filepath.Join(ModelsDir(), m.Filename)
}

// Exists checks if the model file is downloaded.
func (m ModelFile) Exists() bool {
	_, err := os.Stat(m.LocalPath())
	return err == nil
}

// IsReady returns true if all required files are downloaded.
func (m Model) IsReady() bool {
	if !m.Model.Exists() {
		return false
	}
	if m.MMProj != nil && !m.MMProj.Exists() {
		return false
	}
	return true
}

// downloadURL returns the HuggingFace download URL for a file.
func (m ModelFile) downloadURL() string {
	return fmt.Sprintf("%s/%s/resolve/main/%s", hfBaseURL, m.Repo, m.Filename)
}

// Download fetches a model file from HuggingFace with progress reporting.
func (m ModelFile) Download() error {
	dest := m.LocalPath()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	resp, err := http.Get(m.downloadURL())
	if err != nil {
		return fmt.Errorf("download %s: %w", m.Filename, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", m.Filename, resp.StatusCode)
	}

	totalSize, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(tmp) // clean up on failure
	}()

	var written int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			written += int64(n)
			if totalSize > 0 {
				pct := float64(written) / float64(totalSize) * 100
				fmt.Fprintf(os.Stderr, "\r  %s: %.1f%% (%d / %d MB)",
					m.Filename, pct, written/(1024*1024), totalSize/(1024*1024))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	fmt.Fprintln(os.Stderr)

	f.Close()
	return os.Rename(tmp, dest)
}

// Pull downloads model and mmproj files if not already present.
func (m Model) Pull() error {
	files := []ModelFile{m.Model}
	if m.MMProj != nil {
		files = append(files, *m.MMProj)
	}
	for _, mf := range files {
		if mf.Exists() {
			fmt.Fprintf(os.Stderr, "  %s: already downloaded\n", mf.Filename)
			continue
		}
		fmt.Fprintf(os.Stderr, "  Downloading %s...\n", mf.Filename)
		if err := mf.Download(); err != nil {
			return err
		}
	}
	return nil
}

// MMProjLocalPath returns the mmproj local path, or empty string if not needed.
func (m Model) MMProjLocalPath() string {
	if m.MMProj == nil {
		return ""
	}
	return m.MMProj.LocalPath()
}

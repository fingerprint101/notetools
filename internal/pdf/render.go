package pdf

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/gen2brain/go-fitz"
)

// RenderPages renders each page of a PDF to a PNG file in a temp directory.
// Returns the paths to the PNG files. Caller is responsible for cleanup.
func RenderPages(pdfPath string, dpi int) ([]string, error) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("open PDF: %w", err)
	}
	defer doc.Close()

	pageCount := doc.NumPage()
	if pageCount == 0 {
		return nil, fmt.Errorf("PDF has no extractable pages")
	}

	tmpDir, err := os.MkdirTemp("", "notetools-ocr-*")
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, pageCount)
	for i := 0; i < pageCount; i++ {
		img, err := doc.ImageDPI(i, float64(dpi))
		if err != nil {
			return paths, fmt.Errorf("render page %d: %w", i+1, err)
		}

		p := filepath.Join(tmpDir, fmt.Sprintf("page_%04d.png", i+1))
		f, err := os.Create(p)
		if err != nil {
			return paths, err
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			return paths, fmt.Errorf("encode page %d: %w", i+1, err)
		}
		f.Close()
		paths = append(paths, p)
	}

	return paths, nil
}

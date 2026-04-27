package docs

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	imagePlaceholderRE = regexp.MustCompile(`\[\[image:([A-Za-z0-9_-]+)\]\]`)
	cropIDRE           = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

const (
	defaultCropPaddingPx = 40
	cropCoordinateScale  = 1000
)

func MaterializeCrops(ctx context.Context, markdown string, crops []Crop, pagePaths []string, imageDir, imageRelDir string) (string, []string) {
	placeholders := placeholderIDs(markdown)
	replacements := make(map[string]string)
	var warnings []string

	createdDir := false
	seen := make(map[string]bool)
	for _, crop := range crops {
		if seen[crop.ID] {
			warnings = append(warnings, fmt.Sprintf("duplicate crop id %q; skipped", crop.ID))
			continue
		}
		seen[crop.ID] = true

		if !cropIDRE.MatchString(crop.ID) {
			warnings = append(warnings, fmt.Sprintf("invalid crop id %q; skipped", crop.ID))
			continue
		}
		if !placeholders[crop.ID] {
			warnings = append(warnings, fmt.Sprintf("crop id %q has no matching placeholder; skipped", crop.ID))
			continue
		}
		if crop.ImageIndex < 1 || crop.ImageIndex > len(pagePaths) {
			warnings = append(warnings, fmt.Sprintf("crop %q references image %d, but section has %d image(s); skipped", crop.ID, crop.ImageIndex, len(pagePaths)))
			continue
		}

		src := pagePaths[crop.ImageIndex-1]
		width, height, err := imageSize(src)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("could not read image size for crop %q: %v; skipped", crop.ID, err))
			continue
		}

		x, y, w, h, ok := normalizedCrop(crop, width, height)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("crop %q has invalid coordinates; skipped", crop.ID))
			continue
		}

		if !createdDir {
			if err := os.MkdirAll(imageDir, 0o755); err != nil {
				warnings = append(warnings, fmt.Sprintf("could not create image folder %s: %v; skipped remaining crops", imageDir, err))
				break
			}
			createdDir = true
		}

		outPath := filepath.Join(imageDir, crop.ID+".png")
		if err := cropImage(ctx, src, outPath, x, y, w, h); err != nil {
			warnings = append(warnings, fmt.Sprintf("could not crop %q: %v; skipped", crop.ID, err))
			continue
		}

		alt := crop.AltText
		if alt == "" {
			alt = crop.ID
		}
		replacements[crop.ID] = fmt.Sprintf("![%s](%s)", escapeMarkdownAlt(alt), markdownImagePath(imageRelDir, crop.ID))
	}

	for id := range placeholders {
		if replacements[id] == "" {
			warnings = append(warnings, fmt.Sprintf("placeholder %q has no generated crop; removed", id))
		}
	}

	return replaceImagePlaceholders(markdown, replacements), warnings
}

func RemoveImagePlaceholders(markdown string) string {
	return replaceImagePlaceholders(markdown, nil)
}

func placeholderIDs(markdown string) map[string]bool {
	ids := make(map[string]bool)
	for _, match := range imagePlaceholderRE.FindAllStringSubmatch(markdown, -1) {
		ids[match[1]] = true
	}
	return ids
}

func replaceImagePlaceholders(markdown string, replacements map[string]string) string {
	return imagePlaceholderRE.ReplaceAllStringFunc(markdown, func(token string) string {
		matches := imagePlaceholderRE.FindStringSubmatch(token)
		if len(matches) != 2 {
			return ""
		}
		return replacements[matches[1]]
	})
}

func imageSize(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func normalizedCrop(crop Crop, width, height int) (int, int, int, int, bool) {
	if crop.X1 < 0 || crop.X1 > cropCoordinateScale ||
		crop.Y1 < 0 || crop.Y1 > cropCoordinateScale ||
		crop.X2 < 0 || crop.X2 > cropCoordinateScale ||
		crop.Y2 < 0 || crop.Y2 > cropCoordinateScale ||
		crop.X2 <= crop.X1 || crop.Y2 <= crop.Y1 {
		return 0, 0, 0, 0, false
	}

	x1 := clamp(scaleCropCoord(crop.X1, width)-defaultCropPaddingPx, 0, width)
	y1 := clamp(scaleCropCoord(crop.Y1, height)-defaultCropPaddingPx, 0, height)
	x2 := clamp(scaleCropCoord(crop.X2, width)+defaultCropPaddingPx, 0, width)
	y2 := clamp(scaleCropCoord(crop.Y2, height)+defaultCropPaddingPx, 0, height)

	if x2 <= x1 || y2 <= y1 {
		return 0, 0, 0, 0, false
	}
	return x1, y1, x2 - x1, y2 - y1, true
}

func scaleCropCoord(v, size int) int {
	return (v*size + cropCoordinateScale/2) / cropCoordinateScale
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func escapeMarkdownAlt(text string) string {
	text = strings.ReplaceAll(text, "]", "\\]")
	text = strings.ReplaceAll(text, "\n", " ")
	return text
}

func markdownImagePath(imageRelDir, cropID string) string {
	relPath := filepath.ToSlash(filepath.Join(imageRelDir, cropID+".png"))
	parts := strings.Split(relPath, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

type subImager interface {
	SubImage(r image.Rectangle) image.Image
}

func cropImage(ctx context.Context, src, dst string, x, y, width, height int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	rect := image.Rect(x, y, x+width, y+height)
	var cropped image.Image
	if si, ok := img.(subImager); ok {
		cropped = si.SubImage(rect)
	} else {
		return fmt.Errorf("decoded image does not support cropping")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	return png.Encode(out, cropped)
}

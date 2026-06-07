package image

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/davidbyttow/govips/v2/vips"

	"github.com/devailab/assettune-mcp/internal/config"
)

type Converter struct {
	quality      int
	maxDimension int
}

func NewConverter(quality, maxDimension int) *Converter {
	return &Converter{
		quality:      clampQuality(quality),
		maxDimension: maxDimension,
	}
}

func (c *Converter) Convert(input string) (outputPath string, originalSize, optimizedSize int64, err error) {
	info, err := os.Stat(input)
	if err != nil {
		return "", 0, 0, fmt.Errorf("stat: %w", err)
	}
	originalSize = info.Size()

	image, err := vips.NewImageFromFile(input)
	if err != nil {
		return "", 0, 0, fmt.Errorf("load: %w", err)
	}
	defer image.Close()

	if err := image.AutoRotate(); err != nil {
		return "", 0, 0, fmt.Errorf("auto-rotate: %w", err)
	}

	if c.maxDimension > 0 {
		if w, h := image.Width(), image.Height(); w > c.maxDimension || h > c.maxDimension {
			if err := image.Resize(c.scaleFactor(w, h), vips.KernelLanczos3); err != nil {
				return "", 0, 0, fmt.Errorf("resize: %w", err)
			}
		}
	}

	params := &vips.WebpExportParams{
		Quality:      c.quality,
		StripMetadata: true,
	}
	buf, _, err := image.ExportWebp(params)
	if err != nil {
		return "", 0, 0, fmt.Errorf("export: %w", err)
	}

	outputPath = c.outputPath(input)
	if err := os.WriteFile(outputPath, buf, 0644); err != nil {
		return "", 0, 0, fmt.Errorf("write: %w", err)
	}

	optimizedSize = int64(len(buf))
	return outputPath, originalSize, optimizedSize, nil
}

func (c *Converter) scaleFactor(w, h int) float64 {
	largest := w
	if h > largest {
		largest = h
	}
	return float64(c.maxDimension) / float64(largest)
}

func (c *Converter) outputPath(input string) string {
	dir := filepath.Dir(input)
	base := filepath.Base(input)
	ext := filepath.Ext(base)
	stem := base[:len(base)-len(ext)]
	return filepath.Join(dir, stem+config.OutputSuffix+".webp")
}

func ResolveConcurrency(requested int) int {
	if requested <= 0 {
		return runtime.NumCPU()
	}
	if requested > runtime.NumCPU()*4 {
		return runtime.NumCPU() * 4
	}
	return requested
}

var ErrNoImages = errors.New("no supported images found")

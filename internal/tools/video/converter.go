package video

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	ffmpeg "github.com/u2takey/ffmpeg-go"

	"github.com/devailab/assettune-mcp/internal/config"
)

var ErrNoVideos = errors.New("no supported videos found")

// CheckFFmpeg returns an error with OS-specific install instructions
// if the ffmpeg binary is not available in PATH.
func CheckFFmpeg() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf(
			"ffmpeg binary not found in PATH\n\n%s",
			installInstructions(),
		)
	}
	return nil
}

// Converter handles the ffmpeg transcoding to web-optimised MP4.
type Converter struct{}

func NewConverter() *Converter { return &Converter{} }

// Convert transcodes input to a web-optimised MP4 and returns the output path
// plus before/after byte sizes. For .mp4 → .mp4 it writes via a temp file
// to avoid corrupting the source if the process is interrupted.
func (c *Converter) Convert(ctx context.Context, input string) (outputPath string, originalSize, optimizedSize int64, err error) {
	info, err := os.Stat(input)
	if err != nil {
		return "", 0, 0, fmt.Errorf("stat: %w", err)
	}
	originalSize = info.Size()

	outputPath = c.outputPath(input)

	if err = c.runFFmpeg(ctx, input, outputPath); err != nil {
		return "", 0, 0, err
	}

	info, err = os.Stat(outputPath)
	if err != nil {
		return "", 0, 0, fmt.Errorf("stat output: %w", err)
	}
	optimizedSize = info.Size()
	return outputPath, originalSize, optimizedSize, nil
}

func (c *Converter) runFFmpeg(ctx context.Context, input, output string) error {
	args := ffmpeg.Input(input).
		Output(output, ffmpeg.KwArgs{
			"c:v":      "libx264",
			"c:a":      "aac",
			"movflags": "+faststart",
			"pix_fmt":  "yuv420p",
			"crf":      "23",
			"preset":   "medium",
		}).
		OverWriteOutput().
		GetArgs()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		out := stderr.String()
		if len(out) > 1500 {
			out = "…" + out[len(out)-1500:]
		}
		return fmt.Errorf("ffmpeg: %w\n%s", err, out)
	}
	return nil
}

// outputPath returns the destination path: always {stem}_web.mp4.
// The _web suffix keeps the original file untouched regardless of input format.
func (c *Converter) outputPath(input string) string {
	dir := filepath.Dir(input)
	base := filepath.Base(input)
	ext := filepath.Ext(base)
	stem := base[:len(base)-len(ext)]
	return filepath.Join(dir, stem+config.OutputSuffix+".mp4")
}

// ResolveConcurrency caps parallel workers at NumCPU.
// Defaults to 1 because each ffmpeg process already uses multiple threads.
func ResolveConcurrency(requested int) int {
	if requested <= 0 {
		return 1
	}
	if requested > runtime.NumCPU() {
		return runtime.NumCPU()
	}
	return requested
}

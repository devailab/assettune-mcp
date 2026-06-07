package video

import (
	"path/filepath"
	"runtime"
	"strings"
)

var supportedVideoExtensions = map[string]struct{}{
	".mp4":  {},
	".webm": {},
	".mov":  {},
	".mkv":  {},
}

// VideoArgs are the MCP tool input parameters.
type VideoArgs struct {
	Folder      string `json:"folder" jsonschema:"folder path relative to the current workspace, must stay inside it"`
	Concurrency int    `json:"concurrency,omitempty" jsonschema:"parallel workers, defaults to 1 (video transcoding is CPU-intensive)"`
	Overwrite   *bool  `json:"overwrite,omitempty" jsonschema:"re-encode even if output already exists, defaults to true; set to false to skip already-converted files"`
}

// FileResult holds per-file conversion data returned to the caller.
type FileResult struct {
	Original    string  `json:"original"`
	Optimized   string  `json:"optimized"`
	OriginalKB  int64   `json:"original_kb"`
	OptimizedKB int64   `json:"optimized_kb"`
	SavingsPct  float64 `json:"savings_pct"`
	Error       string  `json:"error,omitempty"`
	Skipped     bool    `json:"skipped"`
}

// VideoResult is the aggregate outcome returned by Run and the MCP tool.
type VideoResult struct {
	Folder       string       `json:"folder"`
	TotalFound   int          `json:"total_found"`
	Converted    int          `json:"converted"`
	Skipped      int          `json:"skipped"`
	Failed       int          `json:"failed"`
	OriginalKB   int64        `json:"total_original_kb"`
	OptimizedKB  int64        `json:"total_optimized_kb"`
	TotalSavings float64      `json:"total_savings_pct"`
	Files        []FileResult `json:"files"`
}

func isSupportedVideo(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := supportedVideoExtensions[ext]
	return ok
}

// installInstructions returns OS-specific ffmpeg install guidance.
func installInstructions() string {
	switch runtime.GOOS {
	case "darwin":
		return "Install ffmpeg on macOS:\n" +
			"  • Homebrew (recommended): brew install ffmpeg\n" +
			"  • MacPorts:               sudo port install ffmpeg\n" +
			"  • Official download:      https://ffmpeg.org/download.html"
	case "linux":
		return "Install ffmpeg on Linux:\n" +
			"  • Ubuntu/Debian: sudo apt update && sudo apt install ffmpeg\n" +
			"  • Fedora/RHEL:   sudo dnf install ffmpeg\n" +
			"                   (enable RPM Fusion first: https://rpmfusion.org)\n" +
			"  • Arch Linux:    sudo pacman -S ffmpeg\n" +
			"  • Alpine:        apk add ffmpeg\n" +
			"  • Snap:          sudo snap install ffmpeg\n" +
			"  • Official:      https://ffmpeg.org/download.html"
	case "windows":
		return "Install ffmpeg on Windows:\n" +
			"  • winget:      winget install -e --id Gyan.FFmpeg\n" +
			"  • Chocolatey:  choco install ffmpeg\n" +
			"  • Scoop:       scoop install ffmpeg\n" +
			"  • Official:    https://ffmpeg.org/download.html#build-windows"
	default:
		return "Install ffmpeg: https://ffmpeg.org/download.html"
	}
}

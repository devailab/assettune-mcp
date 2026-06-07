package video

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/devailab/assettune-mcp/internal/config"
)

// Scanner finds video files to process within a directory tree.
type Scanner struct {
	skipExisting bool
}

func NewScanner(skipExisting bool) *Scanner {
	return &Scanner{skipExisting: skipExisting}
}

// Scan walks root once and returns videos that need conversion.
//
// Skip logic when skipExisting=true:
//   - Non-mp4 sources (mov, webm, mkv): skipped if their .mp4 output already exists.
//   - mp4 sources: always included — we cannot detect whether they are already
//     web-optimised without reading codec metadata.
func (s *Scanner) Scan(root string) ([]string, error) {
	var candidates []string
	// existing maps "stem.mp4" → present, used for skip checks of non-mp4 sources.
	existing := make(map[string]struct{})

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if s.skipExisting && ext == ".mp4" && strings.HasSuffix(path, config.OutputSuffix+".mp4") {
			existing[path] = struct{}{}
		}
		if isSupportedVideo(path) && !strings.HasSuffix(path, config.OutputSuffix+".mp4") {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !s.skipExisting {
		return candidates, nil
	}

	videos := candidates[:0]
	for _, p := range candidates {
		stem := p[:len(p)-len(filepath.Ext(p))]
		webPath := stem + config.OutputSuffix+".mp4"
		if _, exists := existing[webPath]; !exists {
			videos = append(videos, p)
		}
	}
	return videos, nil
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

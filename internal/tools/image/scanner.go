package image

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/devailab/assettune-mcp/internal/config"
)

type Scanner struct {
	skipExistingWebP bool
}

func NewScanner(skipExistingWebP bool) *Scanner {
	return &Scanner{skipExistingWebP: skipExistingWebP}
}

// Scan walks root once, collecting supported images while tracking existing
// optimized outputs in the same pass to avoid a redundant second walk.
func (s *Scanner) Scan(root string) ([]string, error) {
	var candidates []string
	existing := make(map[string]struct{})
	outputSuffix := config.OutputSuffix + ".webp"

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if s.skipExistingWebP && strings.HasSuffix(path, outputSuffix) {
			existing[path] = struct{}{}
			return nil
		}
		if isSupportedImage(path) {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if !s.skipExistingWebP {
		return candidates, nil
	}

	images := candidates[:0]
	for _, p := range candidates {
		stem := p[:len(p)-len(filepath.Ext(p))]
		if _, exists := existing[stem+outputSuffix]; !exists {
			images = append(images, p)
		}
	}
	return images, nil
}

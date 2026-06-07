package image

import (
	"path/filepath"
	"strings"
)

const (
	defaultQuality = 80
	minQuality     = 1
	maxQuality     = 100
)

var supportedExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".gif":  {},
	".bmp":  {},
	".tif":  {},
	".tiff": {},
	".heic": {},
	".heif": {},
	".avif": {},
}

type OptimizeArgs struct {
	Folder       string `json:"folder" jsonschema:"folder path relative to the current workspace, must stay inside it"`
	Quality      int    `json:"quality,omitempty" jsonschema:"webp quality from 1 to 100, defaults to 80"`
	Concurrency  int    `json:"concurrency,omitempty" jsonschema:"number of parallel workers, defaults to runtime.NumCPU"`
	MaxDimension int    `json:"max_dimension,omitempty" jsonschema:"maximum width or height in pixels, 0 keeps original size"`
	Overwrite    *bool  `json:"overwrite,omitempty" jsonschema:"overwrite existing webp files, defaults to true; set to false to skip images that already have a .webp counterpart"`
}

type FileResult struct {
	Original    string  `json:"original"`
	Optimized   string  `json:"optimized"`
	OriginalKB  int64   `json:"original_kb"`
	OptimizedKB int64   `json:"optimized_kb"`
	SavingsPct  float64 `json:"savings_pct"`
	Error       string  `json:"error,omitempty"`
	Skipped     bool    `json:"skipped"`
}

type OptimizeResult struct {
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

func isSupportedImage(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := supportedExtensions[ext]
	return ok
}

func clampQuality(q int) int {
	if q <= 0 {
		return defaultQuality
	}
	if q < minQuality {
		return minQuality
	}
	if q > maxQuality {
		return maxQuality
	}
	return q
}

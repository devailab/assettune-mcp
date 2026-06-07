package image

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const toolName = "optimize_images"

// vipsOnce ensures vips.Startup is called exactly once across all tool invocations.
var vipsOnce sync.Once

type tool struct{}

func Register(server *mcp.Server) {
	t := &tool{}
	mcp.AddTool(server, &mcp.Tool{
		Name: toolName,
		Description: "Recursively scans a folder inside the current workspace for images " +
			"(jpg, jpeg, png, gif, bmp, tif, tiff, heic, heif, avif) and converts them to " +
			"optimized WebP format. " +
			"SECURITY: only relative paths within the current workspace are accepted; " +
			"absolute paths, home-relative paths (~), and any path that would escape the " +
			"workspace via '..' are rejected. " +
			"The tool never reads or writes files outside the workspace root.",
	}, t.handle)
}

func (t *tool) handle(ctx context.Context, _ *mcp.CallToolRequest, args OptimizeArgs) (*mcp.CallToolResult, OptimizeResult, error) {
	workspace, err := os.Getwd()
	if err != nil {
		return errorResult(fmt.Sprintf("cannot determine workspace: %v", err))
	}

	folder, err := resolveWorkspacePath(workspace, args.Folder)
	if err != nil {
		return errorResult(err.Error())
	}

	info, err := os.Stat(folder)
	if err != nil {
		return errorResult(fmt.Sprintf("cannot access folder %q: %v", args.Folder, err))
	}
	if !info.IsDir() {
		return errorResult(fmt.Sprintf("%q is not a directory", args.Folder))
	}

	vipsOnce.Do(func() { vips.Startup(nil) })

	overwrite := args.Overwrite == nil || *args.Overwrite
	result, err := Run(ctx, folder, args.Quality, args.MaxDimension, args.Concurrency, overwrite)
	if err != nil {
		return errorResult(err.Error())
	}

	// Use relative paths so server filesystem layout is never exposed to the client.
	makeRelative(workspace, &result)

	var status string
	if result.Failed > 0 {
		status = fmt.Sprintf("warning: %d file(s) failed", result.Failed)
	} else {
		status = "ok"
	}
	summary := fmt.Sprintf(
		"[%s] Converted %d/%d images — saved %.1f%% (%.0f KB → %.0f KB). Skipped: %d. Failed: %d.",
		status, result.Converted, result.TotalFound, result.TotalSavings,
		float64(result.OriginalKB), float64(result.OptimizedKB),
		result.Skipped, result.Failed,
	)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: summary}},
	}, result, nil
}

// makeRelative rewrites all absolute paths in result to be relative to workspace.
func makeRelative(workspace string, r *OptimizeResult) {
	r.Folder, _ = filepath.Rel(workspace, r.Folder)
	for i := range r.Files {
		if p, err := filepath.Rel(workspace, r.Files[i].Original); err == nil {
			r.Files[i].Original = p
		}
		if p, err := filepath.Rel(workspace, r.Files[i].Optimized); err == nil {
			r.Files[i].Optimized = p
		}
	}
}

// Run executes image optimization on dir and returns the result.
// Callers are responsible for calling vips.Startup before this and vips.Shutdown after.
func Run(ctx context.Context, dir string, quality, maxDimension, concurrency int, overwrite bool) (OptimizeResult, error) {
	converter := NewConverter(quality, maxDimension)
	scanner := NewScanner(!overwrite)

	images, err := scanner.Scan(dir)
	if err != nil {
		return OptimizeResult{}, fmt.Errorf("scan: %w", err)
	}
	if len(images) == 0 {
		return OptimizeResult{}, ErrNoImages
	}

	result := runWorkers(ctx, converter, images, concurrency, overwrite)
	result.Folder = dir
	return result, nil
}

func resolveWorkspacePath(workspace, requested string) (string, error) {
	if strings.TrimSpace(requested) == "" {
		return "", errors.New("folder is required")
	}
	if strings.HasPrefix(requested, "~") {
		return "", errors.New("absolute paths and home-relative paths are not allowed")
	}
	cleaned := filepath.Clean(requested)
	if filepath.IsAbs(cleaned) {
		return "", errors.New("absolute paths are not allowed, provide a path relative to the workspace")
	}

	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("invalid workspace: %w", err)
	}
	resolved := filepath.Join(absWorkspace, cleaned)

	rel, err := filepath.Rel(absWorkspace, resolved)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", errors.New("path must stay within the workspace")
	}
	return resolved, nil
}

func runWorkers(ctx context.Context, converter *Converter, images []string, concurrency int, overwrite bool) OptimizeResult {
	workers := ResolveConcurrency(concurrency)
	jobs := make(chan string, workers)
	results := make(chan FileResult, len(images))

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					results <- process(converter, path, overwrite)
				}
			}
		}()
	}

sendLoop:
	for _, img := range images {
		select {
		case <-ctx.Done():
			break sendLoop
		case jobs <- img:
		}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	summary := OptimizeResult{
		TotalFound: len(images),
		Files:      make([]FileResult, 0, len(images)),
	}
	for r := range results {
		summary.Files = append(summary.Files, r)
		summary.OriginalKB += r.OriginalKB
		summary.OptimizedKB += r.OptimizedKB
		switch {
		case r.Skipped:
			summary.Skipped++
		case r.Error != "":
			summary.Failed++
		default:
			summary.Converted++
		}
	}
	if summary.OriginalKB > 0 {
		saved := summary.OriginalKB - summary.OptimizedKB
		summary.TotalSavings = float64(saved) / float64(summary.OriginalKB) * 100
	}
	return summary
}

func process(converter *Converter, path string, overwrite bool) FileResult {
	if !overwrite {
		webp := converter.outputPath(path)
		if _, err := os.Stat(webp); err == nil {
			return summarySkipped(path, webp)
		}
	}

	output, origSize, optSize, err := converter.Convert(path)
	if err != nil {
		return FileResult{Original: path, Error: err.Error()}
	}
	fr := FileResult{
		Original:    path,
		Optimized:   output,
		OriginalKB:  origSize / 1024,
		OptimizedKB: optSize / 1024,
	}
	if origSize > 0 {
		fr.SavingsPct = float64(origSize-optSize) / float64(origSize) * 100
	}
	return fr
}

func summarySkipped(input, output string) FileResult {
	origSize, _ := fileSize(input)
	optSize, _ := fileSize(output)
	fr := FileResult{
		Original:    input,
		Optimized:   output,
		OriginalKB:  origSize / 1024,
		OptimizedKB: optSize / 1024,
		Skipped:     true,
	}
	if origSize > 0 {
		fr.SavingsPct = float64(origSize-optSize) / float64(origSize) * 100
	}
	return fr
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func errorResult(msg string) (*mcp.CallToolResult, OptimizeResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}, OptimizeResult{}, errors.New(msg)
}

package video

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const toolName = "optimize_videos"

type tool struct{}

func Register(server *mcp.Server) {
	t := &tool{}
	mcp.AddTool(server, &mcp.Tool{
		Name: toolName,
		Description: "Recursively scans a folder inside the current workspace for videos " +
			"(mp4, webm, mov, mkv) and converts them to web-optimized MP4 " +
			"(H.264 / libx264, AAC audio, FastStart enabled, yuv420p pixel format). " +
			"Requires ffmpeg to be installed on the host — the tool returns installation " +
			"instructions if it is missing. " +
			"SECURITY: only relative paths within the current workspace are accepted; " +
			"absolute paths, home-relative (~) paths, and any path escaping the workspace " +
			"via '..' are rejected.",
	}, t.handle)
}

func (t *tool) handle(ctx context.Context, _ *mcp.CallToolRequest, args VideoArgs) (*mcp.CallToolResult, VideoResult, error) {
	// Fail fast with install instructions before any path work.
	if err := CheckFFmpeg(); err != nil {
		return errorResult(err.Error())
	}

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

	overwrite := args.Overwrite == nil || *args.Overwrite
	result, err := Run(ctx, folder, args.Concurrency, overwrite)
	if err != nil {
		return errorResult(err.Error())
	}

	makeRelative(workspace, &result)

	status := "ok"
	if result.Failed > 0 {
		status = fmt.Sprintf("warning: %d file(s) failed", result.Failed)
	}
	summary := fmt.Sprintf(
		"[%s] Converted %d/%d videos — saved %.1f%% (%.0f KB → %.0f KB). Skipped: %d. Failed: %d.",
		status, result.Converted, result.TotalFound, result.TotalSavings,
		float64(result.OriginalKB), float64(result.OptimizedKB),
		result.Skipped, result.Failed,
	)
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: summary}},
	}, result, nil
}

// Run executes video optimisation on dir.
// Callers must ensure ffmpeg is available (call CheckFFmpeg first).
func Run(ctx context.Context, dir string, concurrency int, overwrite bool) (VideoResult, error) {
	scanner := NewScanner(!overwrite)
	videos, err := scanner.Scan(dir)
	if err != nil {
		return VideoResult{}, fmt.Errorf("scan: %w", err)
	}
	if len(videos) == 0 {
		return VideoResult{}, ErrNoVideos
	}

	result := runWorkers(ctx, NewConverter(), videos, concurrency)
	result.Folder = dir
	return result, nil
}

func resolveWorkspacePath(workspace, requested string) (string, error) {
	if strings.TrimSpace(requested) == "" {
		return "", errors.New("folder is required")
	}
	if strings.HasPrefix(requested, "~") {
		return "", errors.New("home-relative paths are not allowed")
	}
	cleaned := filepath.Clean(requested)
	if filepath.IsAbs(cleaned) {
		return "", errors.New("absolute paths are not allowed")
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

func runWorkers(ctx context.Context, converter *Converter, videos []string, concurrency int) VideoResult {
	workers := ResolveConcurrency(concurrency)
	jobs := make(chan string, workers)
	results := make(chan FileResult, len(videos))

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
					results <- process(ctx, converter, path)
				}
			}
		}()
	}

sendLoop:
	for _, v := range videos {
		select {
		case <-ctx.Done():
			break sendLoop
		case jobs <- v:
		}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	summary := VideoResult{
		TotalFound: len(videos),
		Files:      make([]FileResult, 0, len(videos)),
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

func process(ctx context.Context, converter *Converter, path string) FileResult {
	output, origSize, optSize, err := converter.Convert(ctx, path)
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


func makeRelative(workspace string, r *VideoResult) {
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

func errorResult(msg string) (*mcp.CallToolResult, VideoResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		IsError: true,
	}, VideoResult{}, errors.New(msg)
}

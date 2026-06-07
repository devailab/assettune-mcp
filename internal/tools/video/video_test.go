package video

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Unit tests — no ffmpeg required
// ---------------------------------------------------------------------------

func TestIsSupportedVideo(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"video.mp4", true},
		{"video.webm", true},
		{"video.mov", true},
		{"video.mkv", true},
		{"video.MP4", true},
		{"video.MOV", true},
		{"video.MKV", true},
		{"video.WEBM", true},
		{"video.avi", false},
		{"video.flv", false},
		{"video.png", false},
		{"video", false},
	}
	for _, tc := range tests {
		got := isSupportedVideo(tc.path)
		if got != tc.want {
			t.Errorf("isSupportedVideo(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestResolveConcurrency(t *testing.T) {
	cpus := runtime.NumCPU()
	tests := []struct {
		in   int
		want int
	}{
		{0, 1},
		{-1, 1},
		{1, 1},
		{cpus, cpus},
		{cpus + 1, cpus},
		{9999, cpus},
	}
	for _, tc := range tests {
		got := ResolveConcurrency(tc.in)
		if got != tc.want {
			t.Errorf("ResolveConcurrency(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestConverter_outputPath(t *testing.T) {
	c := NewConverter()
	tests := []struct {
		input string
		want  string
	}{
		{"/dir/video.mov", "/dir/video_optimized.mp4"},
		{"/dir/video.webm", "/dir/video_optimized.mp4"},
		{"/dir/video.mkv", "/dir/video_optimized.mp4"},
		{"/dir/video.mp4", "/dir/video_optimized.mp4"},
		{"/dir/video.MOV", "/dir/video_optimized.mp4"},
	}
	for _, tc := range tests {
		got := c.outputPath(tc.input)
		if got != tc.want {
			t.Errorf("outputPath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestScanner_Scan(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		got, err := NewScanner(false).Scan(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("want 0, got %d", len(got))
		}
	})

	t.Run("finds supported formats", func(t *testing.T) {
		dir := t.TempDir()
		touchFile(t, dir, "a.mp4")
		touchFile(t, dir, "b.mov")
		touchFile(t, dir, "c.mkv")
		touchFile(t, dir, "d.webm")
		touchFile(t, dir, "skip.avi")
		touchFile(t, dir, "skip.txt")

		got, err := NewScanner(false).Scan(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 4 {
			t.Errorf("want 4, got %d: %v", len(got), got)
		}
	})

	t.Run("uppercase extensions found", func(t *testing.T) {
		dir := t.TempDir()
		touchFile(t, dir, "clip.MOV")
		touchFile(t, dir, "clip.MKV")

		got, err := NewScanner(false).Scan(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Errorf("want 2, got %d", len(got))
		}
	})

	t.Run("skips source when _optimized.mp4 output exists", func(t *testing.T) {
		dir := t.TempDir()
		touchFile(t, dir, "clip.mov")         // clip_optimized.mp4 exists → skip
		touchFile(t, dir, "clip_optimized.mp4")     // existing output
		touchFile(t, dir, "other.webm")       // no other_optimized.mp4 → include
		touchFile(t, dir, "video.mp4")        // video_optimized.mp4 exists → skip
		touchFile(t, dir, "video_optimized.mp4")    // existing output (excluded from candidates)

		got, err := NewScanner(true).Scan(dir)
		if err != nil {
			t.Fatal(err)
		}
		// clip.mov skipped, video.mp4 skipped, other.webm included
		if len(got) != 1 {
			t.Errorf("want 1 (other.webm), got %d: %v", len(got), got)
		}
		for _, p := range got {
			if strings.HasSuffix(p, "clip.mov") || strings.HasSuffix(p, "video.mp4") {
				t.Errorf("already-converted source should be skipped: %s", p)
			}
		}
	})

	t.Run("mp4 source included when no _optimized.mp4 exists", func(t *testing.T) {
		dir := t.TempDir()
		touchFile(t, dir, "video.mp4")

		got, err := NewScanner(true).Scan(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Errorf("want 1, got %d", len(got))
		}
	})

	t.Run("_optimized.mp4 files are never treated as source", func(t *testing.T) {
		dir := t.TempDir()
		touchFile(t, dir, "clip_optimized.mp4") // output file, not a source

		got, err := NewScanner(false).Scan(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("_optimized.mp4 should not appear as candidate, got %d: %v", len(got), got)
		}
	})

	t.Run("walks subdirectories", func(t *testing.T) {
		dir := t.TempDir()
		sub := filepath.Join(dir, "sub")
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatal(err)
		}
		touchFile(t, dir, "root.mp4")
		touchFile(t, sub, "nested.mov")

		got, err := NewScanner(false).Scan(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Errorf("want 2, got %d", len(got))
		}
	})
}

func TestInstallInstructions(t *testing.T) {
	msg := installInstructions()
	if msg == "" {
		t.Error("installInstructions should not be empty")
	}
	if !strings.Contains(msg, "ffmpeg") {
		t.Error("installInstructions should mention ffmpeg")
	}
}

func TestResolveWorkspacePath(t *testing.T) {
	ws := t.TempDir()
	tests := []struct {
		name    string
		req     string
		wantErr bool
	}{
		{"empty", "", true},
		{"spaces", "   ", true},
		{"absolute", "/tmp/videos", true},
		{"home-relative", "~/videos", true},
		{"escape", "../escape", true},
		{"deep escape", "a/../../escape", true},
		{"valid", "media/videos", false},
		{"dot", ".", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveWorkspacePath(ws, tc.req)
			if tc.wantErr && err == nil {
				t.Errorf("want error, got %q", got)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("want no error, got %v", err)
			}
		})
	}
}

func TestMakeRelative(t *testing.T) {
	ws := "/srv/project"
	r := &VideoResult{
		Folder: "/srv/project/media/videos",
		Files: []FileResult{
			{Original: "/srv/project/media/videos/clip.mov", Optimized: "/srv/project/media/videos/clip_optimized.mp4"},
		},
	}
	makeRelative(ws, r)
	if r.Folder != "media/videos" {
		t.Errorf("Folder = %q, want media/videos", r.Folder)
	}
	if r.Files[0].Original != "media/videos/clip.mov" {
		t.Errorf("Original = %q, want media/videos/clip.mov", r.Files[0].Original)
	}
	if r.Files[0].Optimized != "media/videos/clip_optimized.mp4" {
		t.Errorf("Optimized = %q, want media/videos/clip_optimized.mp4", r.Files[0].Optimized)
	}
}

// ---------------------------------------------------------------------------
// Integration tests — require ffmpeg
// ---------------------------------------------------------------------------

func skipIfNoFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed — skipping integration test")
	}
}

func skipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping slow integration test (-short)")
	}
}

func TestCheckFFmpeg(t *testing.T) {
	err := CheckFFmpeg()
	if _, lookErr := exec.LookPath("ffmpeg"); lookErr != nil {
		if err == nil {
			t.Error("CheckFFmpeg should return error when ffmpeg is missing")
		}
		if !strings.Contains(err.Error(), "ffmpeg") {
			t.Error("error should mention ffmpeg")
		}
	} else {
		if err != nil {
			t.Errorf("CheckFFmpeg returned error but ffmpeg exists: %v", err)
		}
	}
}

func TestRun_convertsVideos(t *testing.T) {
	skipIfNoFFmpeg(t)
	skipIfShort(t)

	src := fixtureVideo(t)
	dir := t.TempDir()
	data, _ := os.ReadFile(src)
	_ = os.WriteFile(filepath.Join(dir, filepath.Base(src)), data, 0644)

	result, err := Run(context.Background(), dir, 1, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Converted != 1 {
		t.Errorf("Converted = %d, want 1", result.Converted)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d: %v", result.Failed, result.Files)
	}
	if len(result.Files) != 1 {
		t.Fatalf("Files = %d, want 1", len(result.Files))
	}
	if result.TotalSavings <= 30 {
		t.Errorf("TotalSavings = %.2f%%, want > 30%%", result.TotalSavings)
	}
	if result.Files[0].SavingsPct <= 30 {
		t.Errorf("Files[0].SavingsPct = %.2f%%, want > 30%%", result.Files[0].SavingsPct)
	}
}

func TestRun_noVideosReturnsError(t *testing.T) {
	skipIfNoFFmpeg(t)

	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0644)

	_, err := Run(context.Background(), dir, 1, true)
	if err != ErrNoVideos {
		t.Errorf("want ErrNoVideos, got: %v", err)
	}
}

// fixtureVideo returns the canonical real video fixture used by integration tests.
func fixtureVideo(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "media", "videos", "video-1.mp4")
	if _, err := os.Stat(path); err != nil {
		t.Skip("missing fixture media/videos/video-1.mp4")
	}
	return path
}

func touchFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
}

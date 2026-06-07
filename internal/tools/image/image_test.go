package image

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidbyttow/govips/v2/vips"
)

func TestMain(m *testing.M) {
	vips.Startup(nil)
	code := m.Run()
	vips.Shutdown()
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// resolveWorkspacePath
// ---------------------------------------------------------------------------

func TestResolveWorkspacePath(t *testing.T) {
	ws := t.TempDir()

	tests := []struct {
		name      string
		requested string
		wantErr   bool
	}{
		{"absolute path", "/tmp/images", true},
		{"path escape", "../escape", true},
		{"valid relative", "images", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveWorkspacePath(ws, tc.requested)
			if tc.wantErr {
				if err == nil {
					t.Errorf("want error, got path %q", got)
				}
				return
			}
			if err != nil {
				t.Errorf("want no error, got %v", err)
				return
			}
			// resolved path must start with workspace
			rel, _ := filepath.Rel(ws, got)
			if len(rel) > 2 && rel[:2] == ".." {
				t.Errorf("resolved path %q escapes workspace %q", got, ws)
			}
		})
	}
}

func TestRun_convertsImages(t *testing.T) {
	src := fixtureImage(t)
	dir := t.TempDir()

	// copy one real image into the temp dir
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dst := filepath.Join(dir, filepath.Base(src))
	if err := os.WriteFile(dst, data, 0644); err != nil {
		t.Fatal(err)
	}

	result, err := Run(context.Background(), dir, 80, 0, 1, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.TotalFound != 1 {
		t.Errorf("TotalFound = %d, want 1", result.TotalFound)
	}
	if result.Converted != 1 {
		t.Errorf("Converted = %d, want 1", result.Converted)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0 (errors: %v)", result.Failed, result.Files)
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
	if filepath.Base(result.Files[0].Optimized) != filepath.Base(dst[:len(dst)-len(filepath.Ext(dst))])+"_optimized.webp" {
		t.Errorf("Optimized path = %q, want *_optimized.webp", result.Files[0].Optimized)
	}
}

func TestRun_skipsExistingWebP(t *testing.T) {
	src := fixtureImage(t)
	dir := t.TempDir()

	data, _ := os.ReadFile(src)
	base := filepath.Base(src)
	_ = os.WriteFile(filepath.Join(dir, base), data, 0644)

	// pre-create the webp so the scanner skips the source
	stem := base[:len(base)-len(filepath.Ext(base))]
	_ = os.WriteFile(filepath.Join(dir, stem+"_optimized.webp"), []byte("dummy"), 0644)

	result, err := Run(context.Background(), dir, 80, 0, 1, false)
	if !errors.Is(err, ErrNoImages) {
		t.Fatalf("want ErrNoImages, got result=%+v err=%v", result, err)
	}
}

func TestMakeRelative(t *testing.T) {
	workspace := "/srv/project"
	result := &OptimizeResult{
		Folder: "/srv/project/images",
		Files: []FileResult{
			{Original: "/srv/project/images/photo.png", Optimized: "/srv/project/images/photo.webp"},
			{Original: "/srv/project/assets/bg.jpg", Optimized: "/srv/project/assets/bg.webp"},
		},
	}
	makeRelative(workspace, result)

	if result.Folder != "images" {
		t.Errorf("Folder = %q, want %q", result.Folder, "images")
	}
	if result.Files[0].Original != "images/photo.png" {
		t.Errorf("Files[0].Original = %q, want %q", result.Files[0].Original, "images/photo.png")
	}
	if result.Files[0].Optimized != "images/photo.webp" {
		t.Errorf("Files[0].Optimized = %q, want %q", result.Files[0].Optimized, "images/photo.webp")
	}
	if result.Files[1].Original != "assets/bg.jpg" {
		t.Errorf("Files[1].Original = %q, want %q", result.Files[1].Original, "assets/bg.jpg")
	}
}

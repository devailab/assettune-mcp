package image

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestScanner_Scan(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "nested")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	touch(t, dir, "new.png")
	touch(t, dir, "done.jpg")
	touch(t, dir, "done_optimized.webp")
	touch(t, sub, "photo.JPG")
	touch(t, dir, "skip.txt")

	got, err := NewScanner(true).Scan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rel := make([]string, 0, len(got))
	for _, path := range got {
		relativePath, err := filepath.Rel(dir, path)
		if err != nil {
			t.Fatalf("filepath.Rel(%q, %q): %v", dir, path, err)
		}
		rel = append(rel, relativePath)
	}
	slices.Sort(rel)

	want := []string{"nested/photo.JPG", "new.png"}
	slices.Sort(want)

	if !slices.Equal(rel, want) {
		t.Errorf("Scan() = %v, want %v", rel, want)
	}
}

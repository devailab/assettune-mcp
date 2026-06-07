package image

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fixtureImage returns the canonical real image fixture used by integration tests.
func fixtureImage(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "media", "images", "image-1.jpg")
	if _, err := os.Stat(path); err != nil {
		t.Skip("missing fixture media/images/image-1.jpg")
	}
	return path
}

// touch creates an empty file in dir for scanner tests.
func touch(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
		t.Fatalf("touch %s: %v", name, err)
	}
	return path
}

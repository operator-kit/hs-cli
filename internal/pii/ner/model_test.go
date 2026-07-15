package ner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheDirReturnsNonEmpty(t *testing.T) {
	dir, err := CacheDir()
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	if dir == "" || !strings.Contains(dir, "pii-model") {
		t.Fatalf("unexpected cache directory %q", dir)
	}
}

func TestIsModelReadyFalseByDefault(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if IsModelReady() {
		t.Fatal("empty cache reported ready")
	}
}

func TestIsModelReadyFalseWhenOnlyLegacyVersionMatches(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	t.Setenv("LOCALAPPDATA", tmpDir)
	dir, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".version"), []byte(ModelVersion), 0o600); err != nil {
		t.Fatal(err)
	}
	if IsModelReady() {
		t.Fatal("legacy version marker reported ready")
	}
}

func TestModelPathsErrorsWhenNotReady(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	t.Setenv("LOCALAPPDATA", tmpDir)
	if _, err := ModelPaths(); err == nil {
		t.Fatal("ModelPaths returned paths for an unready cache")
	}
}

func TestRemoveModelRemovesExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	t.Setenv("LOCALAPPDATA", tmpDir)
	dir, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fixture"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveModel(); err != nil {
		t.Fatalf("RemoveModel: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("cache still exists after removal: %v", err)
	}
}

func TestRemoveModelNoErrorWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	t.Setenv("LOCALAPPDATA", tmpDir)
	if err := RemoveModel(); err != nil {
		t.Fatalf("RemoveModel missing cache: %v", err)
	}
}

func TestRuntimeLibNameForPlatform(t *testing.T) {
	tests := []struct {
		platform Platform
		want     string
	}{
		{Platform{OS: "linux", Arch: "amd64"}, "libonnxruntime.so"},
		{Platform{OS: "darwin", Arch: "arm64"}, "libonnxruntime.dylib"},
		{Platform{OS: "windows", Arch: "amd64"}, "onnxruntime.dll"},
	}
	for _, test := range tests {
		if got := runtimeLibNameFor(test.platform); got != test.want {
			t.Fatalf("%s runtime name = %q, want %q", test.platform.Key(), got, test.want)
		}
	}
}

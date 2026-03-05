package ner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheDir_ReturnsNonEmpty(t *testing.T) {
	dir, err := CacheDir()
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	if dir == "" {
		t.Fatal("CacheDir should return non-empty path")
	}
	if !strings.Contains(dir, "ner-model") {
		t.Fatalf("CacheDir should contain 'ner-model': %s", dir)
	}
}

func TestIsModelReady_FalseByDefault(t *testing.T) {
	// Point to a temp dir that doesn't have .version
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if IsModelReady() {
		t.Fatal("IsModelReady should be false when no .version file exists")
	}
}

func TestIsModelReady_TrueWhenVersionMatches(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	t.Setenv("LOCALAPPDATA", tmpDir)

	dir, err := CacheDir()
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".version"), []byte(ModelVersion), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsModelReady() {
		t.Fatal("IsModelReady should be true when .version matches")
	}
}

func TestIsModelReady_FalseWhenVersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	t.Setenv("LOCALAPPDATA", tmpDir)

	dir, err := CacheDir()
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".version"), []byte("0.0.0"), 0o644); err != nil {
		t.Fatal(err)
	}
	if IsModelReady() {
		t.Fatal("IsModelReady should be false when version mismatches")
	}
}

func TestModelPaths_ErrorWhenNotInstalled(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	t.Setenv("LOCALAPPDATA", tmpDir)

	_, err := ModelPaths()
	if err == nil {
		t.Fatal("ModelPaths should error when model is not installed")
	}
}

func TestRemoveModel_NoErrorOnMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	t.Setenv("LOCALAPPDATA", tmpDir)

	if err := RemoveModel(); err != nil {
		t.Fatalf("RemoveModel should not error when dir doesn't exist: %v", err)
	}
}

func TestRemoveModel_RemovesExistingDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	t.Setenv("LOCALAPPDATA", tmpDir)

	dir, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".version"), []byte(ModelVersion), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveModel(); err != nil {
		t.Fatalf("RemoveModel: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("RemoveModel should delete cache dir")
	}
}

func TestRuntimeLibName(t *testing.T) {
	name := runtimeLibName()
	if name == "" {
		t.Fatal("runtimeLibName should return non-empty")
	}
	// Should be one of the known lib names
	valid := map[string]bool{
		"libonnxruntime.so":    true,
		"libonnxruntime.dylib": true,
		"onnxruntime.dll":      true,
	}
	if !valid[name] {
		t.Fatalf("unexpected runtime lib name: %q", name)
	}
}

func TestBundleURL(t *testing.T) {
	url := bundleURL()
	if !strings.Contains(url, ModelVersion) {
		t.Fatalf("bundle URL should contain version: %s", url)
	}
	if !strings.Contains(url, "github.com") {
		t.Fatalf("bundle URL should be GitHub: %s", url)
	}
}

// makeTarGz creates a minimal tar.gz archive with the given files.
func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestDownloadAndExtract(t *testing.T) {
	archive := makeTarGz(t, map[string]string{
		"model_quantized.onnx": "fake-model-data",
		"tokenizer.json":       `{"model": "test"}`,
		"config.json":          `{"id2label": {"0": "O"}}`,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.WriteHeader(http.StatusOK)
		w.Write(archive)
	}))
	defer srv.Close()

	dir := t.TempDir()
	var progressCalled bool
	err := downloadAndExtract(srv.URL, dir, func(read, total int64) {
		progressCalled = true
	})
	if err != nil {
		t.Fatalf("downloadAndExtract: %v", err)
	}

	// Check extracted files
	for _, name := range []string{"model_quantized.onnx", "tokenizer.json", "config.json"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing extracted file %s: %v", name, err)
		}
		// Check sidecar hash
		hashPath := path + ".sha256"
		if _, err := os.Stat(hashPath); err != nil {
			t.Fatalf("missing hash file %s: %v", hashPath, err)
		}
	}

	if !progressCalled {
		t.Fatal("progress callback should have been called")
	}
}

func TestDownloadAndExtract_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := downloadAndExtract(srv.URL, t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error should mention status code: %v", err)
	}
}

func TestDownloadAndExtract_PathTraversal(t *testing.T) {
	// Create an archive with a path-traversal attempt
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{
		Name: "../../../etc/passwd",
		Mode: 0o644,
		Size: 4,
	}
	tw.WriteHeader(hdr)
	tw.Write([]byte("evil"))
	// Also add a normal file
	hdr2 := &tar.Header{
		Name: "safe.txt",
		Mode: 0o644,
		Size: 4,
	}
	tw.WriteHeader(hdr2)
	tw.Write([]byte("good"))
	tw.Close()
	gw.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(buf.Bytes())
	}))
	defer srv.Close()

	dir := t.TempDir()
	err := downloadAndExtract(srv.URL, dir, nil)
	if err != nil {
		t.Fatalf("downloadAndExtract: %v", err)
	}

	// The path traversal file should NOT exist outside the target dir
	// The safe file should exist
	if _, err := os.Stat(filepath.Join(dir, "safe.txt")); err != nil {
		t.Fatal("safe.txt should be extracted")
	}
}

func TestProgressReader(t *testing.T) {
	data := bytes.NewReader([]byte("hello world"))
	var lastRead, lastTotal int64
	pr := &progressReader{
		r:     data,
		total: 11,
		fn: func(read, total int64) {
			lastRead = read
			lastTotal = total
		},
	}
	out, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(out) != "hello world" {
		t.Fatalf("unexpected content: %q", out)
	}
	if lastRead != 11 {
		t.Fatalf("expected read=11, got %d", lastRead)
	}
	if lastTotal != 11 {
		t.Fatalf("expected total=11, got %d", lastTotal)
	}
}

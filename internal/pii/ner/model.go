package ner

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Paths holds resolved file paths for the model bundle.
type Paths struct {
	RuntimeLib    string // platform-specific ONNX Runtime shared lib
	ModelONNX     string // model_quantized.onnx
	TokenizerJSON string // tokenizer.json
	ConfigJSON    string // config.json
}

// baseURL is the GitHub release download URL template.
const baseURL = "https://github.com/operator-kit/hs-cli/releases/download/pii-model-v%s/pii-model-%s-%s-%s.tar.gz"

// ProgressFunc reports download progress (bytesRead, totalBytes).
// totalBytes may be -1 if unknown.
type ProgressFunc func(bytesRead, totalBytes int64)

// CacheDir returns the OS-specific cache directory for the PII model.
func CacheDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Caches", "hs", "pii-model"), nil
	case "windows":
		dir := os.Getenv("LOCALAPPDATA")
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(dir, "hs", "pii-model"), nil
	default: // linux, freebsd, etc
		dir := os.Getenv("XDG_CACHE_HOME")
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			dir = filepath.Join(home, ".cache")
		}
		return filepath.Join(dir, "hs", "pii-model"), nil
	}
}

// IsModelReady checks whether the model bundle is present and matches
// the expected version without downloading anything.
func IsModelReady() bool {
	dir, err := CacheDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(dir, ".version"))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == ModelVersion
}

// ModelPaths returns resolved paths if the model is installed, or an error.
func ModelPaths() (*Paths, error) {
	dir, err := CacheDir()
	if err != nil {
		return nil, err
	}
	if !IsModelReady() {
		return nil, fmt.Errorf("model not installed (run \"hs pii-model install\")")
	}

	libName := runtimeLibName()
	p := &Paths{
		RuntimeLib:    filepath.Join(dir, libName),
		ModelONNX:     filepath.Join(dir, "model_quantized.onnx"),
		TokenizerJSON: filepath.Join(dir, "tokenizer.json"),
		ConfigJSON:    filepath.Join(dir, "config.json"),
	}

	// Verify all files exist
	for _, f := range []string{p.RuntimeLib, p.ModelONNX, p.TokenizerJSON, p.ConfigJSON} {
		if _, err := os.Stat(f); err != nil {
			return nil, fmt.Errorf("missing file %s: %w", filepath.Base(f), err)
		}
	}
	return p, nil
}

// EnsureModel downloads and extracts the model bundle if not present.
func EnsureModel(progress ProgressFunc) (*Paths, error) {
	if IsModelReady() {
		return ModelPaths()
	}

	dir, err := CacheDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}

	url := bundleURL()
	if err := downloadAndExtract(url, dir, progress); err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}

	// Write version marker
	if err := os.WriteFile(filepath.Join(dir, ".version"), []byte(ModelVersion), 0o644); err != nil {
		return nil, err
	}

	return ModelPaths()
}

// RemoveModel deletes the cached model bundle.
func RemoveModel() error {
	dir, err := CacheDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func bundleURL() string {
	return fmt.Sprintf(baseURL, ModelVersion, ModelVersion, runtime.GOOS, runtime.GOARCH)
}

func runtimeLibName() string {
	switch runtime.GOOS {
	case "darwin":
		return "libonnxruntime.dylib"
	case "windows":
		return "onnxruntime.dll"
	default:
		return "libonnxruntime.so"
	}
}

func downloadAndExtract(url, dir string, progress ProgressFunc) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	var reader io.Reader = resp.Body
	if progress != nil {
		reader = &progressReader{r: resp.Body, total: resp.ContentLength, fn: progress}
	}

	// Stream through gzip → tar → extract
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		// Security: prevent path traversal
		name := filepath.Base(hdr.Name)
		if name == "." || name == ".." || strings.Contains(hdr.Name, "..") {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeReg:
			dst := filepath.Join(dir, name)
			if err := extractFile(dst, tr, hdr.Mode); err != nil {
				return err
			}
		}
	}
	return nil
}

func extractFile(dst string, r io.Reader, mode int64) error {
	// Compute SHA-256 while writing
	h := sha256.New()
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode)|0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(io.MultiWriter(f, h), r); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	// Write sidecar hash file
	hashHex := hex.EncodeToString(h.Sum(nil))
	return os.WriteFile(dst+".sha256", []byte(hashHex), 0o644)
}

type progressReader struct {
	r     io.Reader
	read  int64
	total int64
	fn    ProgressFunc
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	p.read += int64(n)
	p.fn(p.read, p.total)
	return n, err
}

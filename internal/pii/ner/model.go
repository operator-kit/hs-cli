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

// ModelState describes whether the current platform can run the model and
// whether a locally cached bundle is complete enough to use.
type ModelState string

const (
	ModelUnsupported         ModelState = "unsupported"
	ModelAbsent              ModelState = "absent"
	ModelInstalledUnverified ModelState = "installed-unverified"
	ModelCorrupt             ModelState = "corrupt"
	ModelReady               ModelState = "ready"
)

// ModelStatus is the read-only result of inspecting the local model cache.
type ModelStatus struct {
	State    ModelState
	Platform Platform
	Dir      string
	Present  bool
	Reason   string
}

// Usable reports whether the current runtime may load this installation.
// Legacy bundles remain usable during the migration to verified installs.
func (s ModelStatus) Usable() bool {
	return s.State == ModelInstalledUnverified || s.State == ModelReady
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

// Status inspects the local model cache without downloading anything.
func Status() ModelStatus {
	platform := CurrentPlatform()
	dir, err := CacheDir()
	if err != nil {
		return ModelStatus{State: ModelCorrupt, Platform: platform, Reason: err.Error()}
	}
	return statusAt(dir, platform)
}

func statusAt(dir string, platform Platform) ModelStatus {
	status := ModelStatus{Platform: platform, Dir: dir}
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		status.Present = true
	}
	capability := RuntimeCapabilityFor(platform)
	if !capability.Supported {
		status.State = ModelUnsupported
		status.Reason = capability.Reason
		return status
	}

	data, err := os.ReadFile(filepath.Join(dir, ".version"))
	if os.IsNotExist(err) {
		status.State = ModelAbsent
		return status
	}
	if err != nil {
		status.State = ModelCorrupt
		status.Reason = fmt.Sprintf("read version marker: %v", err)
		return status
	}
	if strings.TrimSpace(string(data)) != ModelVersion {
		status.State = ModelCorrupt
		status.Reason = "installed model version does not match this hs version"
		return status
	}

	paths := pathsAt(dir, platform)
	for _, path := range []string{paths.RuntimeLib, paths.ModelONNX, paths.TokenizerJSON, paths.ConfigJSON} {
		info, statErr := os.Stat(path)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			status.State = ModelCorrupt
			status.Reason = fmt.Sprintf("missing or invalid model file %s", filepath.Base(path))
			return status
		}
	}

	status.State = ModelInstalledUnverified
	status.Reason = "legacy installation has not been verified against a trusted manifest"
	return status
}

// IsModelReady reports whether the model is usable on the current platform.
func IsModelReady() bool {
	return Status().Usable()
}

// ModelPaths returns resolved paths if the model is installed, or an error.
func ModelPaths() (*Paths, error) {
	dir, err := CacheDir()
	if err != nil {
		return nil, err
	}
	status := statusAt(dir, CurrentPlatform())
	if !status.Usable() {
		if status.Reason != "" {
			return nil, fmt.Errorf("PII model %s: %s", status.State, status.Reason)
		}
		return nil, fmt.Errorf("PII model %s (run \"hs pii-model install\")", status.State)
	}
	return pathsAt(dir, CurrentPlatform()), nil
}

func pathsAt(dir string, platform Platform) *Paths {
	return &Paths{
		RuntimeLib:    filepath.Join(dir, runtimeLibNameFor(platform)),
		ModelONNX:     filepath.Join(dir, "model_quantized.onnx"),
		TokenizerJSON: filepath.Join(dir, "tokenizer.json"),
		ConfigJSON:    filepath.Join(dir, "config.json"),
	}
}

// EnsureModel downloads and extracts the model bundle if not present.
func EnsureModel(progress ProgressFunc) (*Paths, error) {
	dir, err := CacheDir()
	if err != nil {
		return nil, err
	}
	return ensureModelAt(CurrentPlatform(), dir, progress, downloadAndExtract)
}

type modelDownloader func(url, dir string, progress ProgressFunc) error

func ensureModelAt(platform Platform, dir string, progress ProgressFunc, download modelDownloader) (*Paths, error) {
	capability := RuntimeCapabilityFor(platform)
	if !capability.Supported {
		return nil, fmt.Errorf("PII model unsupported: %s", capability.Reason)
	}
	if status := statusAt(dir, platform); status.Usable() {
		return pathsAt(dir, platform), nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}

	url := bundleURLFor(platform)
	if err := download(url, dir, progress); err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}

	// Write version marker
	if err := os.WriteFile(filepath.Join(dir, ".version"), []byte(ModelVersion), 0o644); err != nil {
		return nil, err
	}

	status := statusAt(dir, platform)
	if !status.Usable() {
		return nil, fmt.Errorf("downloaded PII model is %s: %s", status.State, status.Reason)
	}
	return pathsAt(dir, platform), nil
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
	return bundleURLFor(CurrentPlatform())
}

func bundleURLFor(platform Platform) string {
	return fmt.Sprintf(baseURL, ModelVersion, ModelVersion, platform.OS, platform.Arch)
}

func runtimeLibName() string {
	return runtimeLibNameFor(CurrentPlatform())
}

func runtimeLibNameFor(platform Platform) string {
	switch platform.OS {
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

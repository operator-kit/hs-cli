package ner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	readyMarkerName    = ".ready.json"
	readyMarkerSchema  = 1
	maxReadyMarkerSize = 8 * 1024
)

// Paths holds resolved file paths for the model bundle.
type Paths struct {
	RuntimeLib    string
	ModelONNX     string
	TokenizerJSON string
	ConfigJSON    string
}

type ModelState string

const (
	ModelUnsupported         ModelState = "unsupported"
	ModelAbsent              ModelState = "absent"
	ModelInstalledUnverified ModelState = "installed-unverified"
	ModelCorrupt             ModelState = "corrupt"
	ModelReady               ModelState = "ready"
)

type ModelStatus struct {
	State    ModelState
	Platform Platform
	Dir      string
	Present  bool
	Reason   string
}

// Usable is deliberately strict: only a manifest-verified, smoke-tested bundle
// may be attached to the redaction engine.
func (s ModelStatus) Usable() bool {
	return s.State == ModelReady
}

// ProgressFunc reports verified download progress. totalBytes is always the
// exact trusted archive size.
type ProgressFunc func(bytesRead, totalBytes int64)

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
	default:
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

func Status() ModelStatus {
	platform := CurrentPlatform()
	dir, err := CacheDir()
	if err != nil {
		return ModelStatus{State: ModelCorrupt, Platform: platform, Reason: err.Error()}
	}
	return statusAt(dir, platform)
}

func statusAt(dir string, platform Platform) ModelStatus {
	manifest, err := LoadTrustedManifest()
	if err != nil {
		return ModelStatus{
			State:    ModelCorrupt,
			Platform: platform,
			Dir:      dir,
			Reason:   err.Error(),
		}
	}
	return statusAtWithManifest(dir, platform, manifest)
}

func statusAtWithManifest(cacheRoot string, platform Platform, manifest TrustedManifest) ModelStatus {
	status := ModelStatus{Platform: platform, Dir: cacheRoot}
	if info, err := os.Stat(cacheRoot); err == nil && info.IsDir() {
		status.Present = true
	}
	capability := RuntimeCapabilityFor(platform)
	if !capability.Supported {
		status.State = ModelUnsupported
		status.Reason = capability.Reason
		return status
	}
	bundle, err := manifest.BundleFor(platform)
	if err != nil {
		status.State = ModelCorrupt
		status.Reason = err.Error()
		return status
	}

	trustedDir := trustedInstallDir(cacheRoot, platform, bundle)
	if info, err := os.Lstat(trustedDir); err == nil {
		status.Dir = trustedDir
		status.Present = true
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			status.State = ModelCorrupt
			status.Reason = "trusted install path is not a real directory"
			return status
		}
		if reason := validateTrustedInstall(trustedDir, platform, bundle, manifest); reason != "" {
			status.State = ModelCorrupt
			status.Reason = reason
			return status
		}
		status.State = ModelReady
		return status
	} else if !os.IsNotExist(err) {
		status.State = ModelCorrupt
		status.Reason = fmt.Sprintf("inspect trusted install: %v", err)
		return status
	}

	version, err := os.ReadFile(filepath.Join(cacheRoot, ".version"))
	if os.IsNotExist(err) {
		status.State = ModelAbsent
		return status
	}
	if err != nil {
		status.State = ModelCorrupt
		status.Reason = fmt.Sprintf("read legacy version marker: %v", err)
		return status
	}
	if strings.TrimSpace(string(version)) != ModelVersion {
		status.State = ModelCorrupt
		status.Reason = "legacy model version does not match this hs version"
		return status
	}

	legacyPaths := pathsAt(cacheRoot, platform)
	for _, file := range []string{
		legacyPaths.RuntimeLib,
		legacyPaths.ModelONNX,
		legacyPaths.TokenizerJSON,
		legacyPaths.ConfigJSON,
	} {
		info, statErr := os.Lstat(file)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			status.State = ModelCorrupt
			status.Reason = fmt.Sprintf("missing or invalid legacy model file %s", filepath.Base(file))
			return status
		}
	}
	status.State = ModelInstalledUnverified
	status.Reason = "legacy installation is not verified against the trusted manifest; reinstall it before use"
	return status
}

func validateTrustedInstall(
	dir string,
	platform Platform,
	bundle BundleManifest,
	manifest TrustedManifest,
) string {
	marker, err := readReadyMarker(filepath.Join(dir, readyMarkerName))
	if err != nil {
		return fmt.Sprintf("invalid ready marker: %v", err)
	}
	if marker.SchemaVersion != readyMarkerSchema ||
		marker.ModelVersion != manifest.ModelVersion ||
		marker.Platform != platform.Key() ||
		marker.ArchiveSHA256 != bundle.Archive.SHA256 ||
		marker.ManifestSHA256 != manifest.Fingerprint() {
		return "ready marker does not match the trusted manifest"
	}
	for _, expected := range bundle.Files {
		info, statErr := os.Lstat(filepath.Join(dir, expected.Name))
		if statErr != nil || !info.Mode().IsRegular() || info.Size() != expected.Size {
			return fmt.Sprintf("missing or invalid trusted model file %s", expected.Name)
		}
	}
	return ""
}

func IsModelReady() bool {
	return Status().State == ModelReady
}

func ModelPaths() (*Paths, error) {
	status := Status()
	if status.State != ModelReady {
		if status.Reason != "" {
			return nil, fmt.Errorf("PII model %s: %s", status.State, status.Reason)
		}
		return nil, fmt.Errorf("PII model %s (run \"hs pii-model install\")", status.State)
	}
	return pathsAt(status.Dir, status.Platform), nil
}

func pathsAt(dir string, platform Platform) *Paths {
	return &Paths{
		RuntimeLib:    filepath.Join(dir, runtimeLibNameFor(platform)),
		ModelONNX:     filepath.Join(dir, "model_quantized.onnx"),
		TokenizerJSON: filepath.Join(dir, "tokenizer.json"),
		ConfigJSON:    filepath.Join(dir, "config.json"),
	}
}

func EnsureModel(progress ProgressFunc) (*Paths, error) {
	return EnsureModelContext(context.Background(), progress)
}

func EnsureModelContext(ctx context.Context, progress ProgressFunc) (*Paths, error) {
	cacheRoot, err := CacheDir()
	if err != nil {
		return nil, err
	}
	manifest, err := LoadTrustedManifest()
	if err != nil {
		return nil, err
	}
	installer := NewBundleInstaller(cacheRoot, CurrentPlatform(), manifest)
	return installer.Install(ctx, progress)
}

func RemoveModel() error {
	dir, err := CacheDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func trustedInstallDir(cacheRoot string, platform Platform, bundle BundleManifest) string {
	return filepath.Join(
		cacheRoot,
		"versions",
		ModelVersion,
		platform.Key(),
		bundle.Archive.SHA256,
	)
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

type readyMarker struct {
	SchemaVersion  int    `json:"schema_version"`
	ModelVersion   string `json:"model_version"`
	Platform       string `json:"platform"`
	ArchiveSHA256  string `json:"archive_sha256"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

func writeReadyMarker(
	dir string,
	platform Platform,
	bundle BundleManifest,
	manifest TrustedManifest,
) error {
	marker := readyMarker{
		SchemaVersion:  readyMarkerSchema,
		ModelVersion:   manifest.ModelVersion,
		Platform:       platform.Key(),
		ArchiveSHA256:  bundle.Archive.SHA256,
		ManifestSHA256: manifest.Fingerprint(),
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	file, err := os.OpenFile(
		filepath.Join(dir, readyMarkerName),
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		return err
	}
	if written, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	} else if written != len(data) {
		_ = file.Close()
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func readReadyMarker(markerPath string) (readyMarker, error) {
	info, err := os.Lstat(markerPath)
	if err != nil {
		return readyMarker{}, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxReadyMarkerSize {
		return readyMarker{}, fmt.Errorf("marker is not a bounded regular file")
	}
	data, err := os.ReadFile(markerPath)
	if err != nil {
		return readyMarker{}, err
	}
	var marker readyMarker
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&marker); err != nil {
		return readyMarker{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return readyMarker{}, fmt.Errorf("marker contains trailing JSON")
		}
		return readyMarker{}, err
	}
	return marker, nil
}

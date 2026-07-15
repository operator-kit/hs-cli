package ner

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"sync"
)

const (
	trustedManifestSchema    = 1
	maximumArchiveSizeLimit  = 512 << 20
	maximumExpandedSizeLimit = 1 << 30
	maximumRuntimeSourceSize = 512 << 20
)

//go:embed trusted_manifest.json
var embeddedTrustedManifest []byte

type TrustedManifest struct {
	SchemaVersion      int              `json:"schema_version"`
	ModelVersion       string           `json:"model_version"`
	ModelRepository    string           `json:"model_repository"`
	ModelRevision      string           `json:"model_revision"`
	ONNXRuntimeVersion string           `json:"onnx_runtime_version"`
	Bundles            []BundleManifest `json:"bundles"`
	fingerprint        string
}

type BundleManifest struct {
	GOOS            string          `json:"goos"`
	GOARCH          string          `json:"goarch"`
	Archive         ArchiveManifest `json:"archive"`
	RuntimeSource   SourceArchive   `json:"runtime_source"`
	RuntimeLibrary  string          `json:"runtime_library"`
	MaxExpandedSize int64           `json:"max_expanded_size"`
	Files           []FileManifest  `json:"files"`
}

type ArchiveManifest struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	MaxSize  int64  `json:"max_size"`
}

type SourceArchive struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type FileManifest struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func (b BundleManifest) Platform() Platform {
	return Platform{OS: b.GOOS, Arch: b.GOARCH}
}

func (m TrustedManifest) BundleFor(platform Platform) (BundleManifest, error) {
	for _, bundle := range m.Bundles {
		if bundle.Platform() == platform {
			return bundle, nil
		}
	}
	return BundleManifest{}, fmt.Errorf("trusted manifest has no bundle for %s", platform.Key())
}

func (m TrustedManifest) Fingerprint() string {
	if m.fingerprint != "" {
		return m.fingerprint
	}
	copy := m
	copy.fingerprint = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

var (
	manifestOnce sync.Once
	manifest     TrustedManifest
	manifestErr  error
)

func LoadTrustedManifest() (TrustedManifest, error) {
	manifestOnce.Do(func() {
		manifest, manifestErr = decodeTrustedManifest(embeddedTrustedManifest)
	})
	return manifest, manifestErr
}

func decodeTrustedManifest(data []byte) (TrustedManifest, error) {
	var parsed TrustedManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return TrustedManifest{}, fmt.Errorf("decode trusted PII model manifest: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return TrustedManifest{}, err
	}
	sum := sha256.Sum256(data)
	parsed.fingerprint = hex.EncodeToString(sum[:])
	if err := parsed.Validate(); err != nil {
		return TrustedManifest{}, fmt.Errorf("validate trusted PII model manifest: %w", err)
	}
	return parsed, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trusted PII model manifest contains trailing JSON")
		}
		return fmt.Errorf("decode trusted PII model manifest trailer: %w", err)
	}
	return nil
}

func (m TrustedManifest) Validate() error {
	if m.SchemaVersion != trustedManifestSchema {
		return fmt.Errorf("schema version %d is unsupported", m.SchemaVersion)
	}
	if m.ModelVersion != ModelVersion {
		return fmt.Errorf("model version %q does not match compiled version %q", m.ModelVersion, ModelVersion)
	}
	if m.ModelRepository == "" {
		return fmt.Errorf("model repository is empty")
	}
	if !validHexDigest(m.ModelRevision, 40) {
		return fmt.Errorf("model revision must be a 40-character hexadecimal commit")
	}
	if m.ONNXRuntimeVersion == "" {
		return fmt.Errorf("ONNX Runtime version is empty")
	}

	requiredPlatforms := SupportedPlatforms()
	if len(m.Bundles) != len(requiredPlatforms) {
		return fmt.Errorf("bundle count %d does not match supported platform count %d", len(m.Bundles), len(requiredPlatforms))
	}
	seenPlatforms := make(map[Platform]struct{}, len(m.Bundles))
	for _, bundle := range m.Bundles {
		platform := bundle.Platform()
		if _, duplicate := seenPlatforms[platform]; duplicate {
			return fmt.Errorf("duplicate bundle for %s", platform.Key())
		}
		seenPlatforms[platform] = struct{}{}
		if !RuntimeCapabilityFor(platform).Supported {
			return fmt.Errorf("bundle advertises unsupported platform %s", platform.Key())
		}
		if err := validateBundle(m, bundle); err != nil {
			return fmt.Errorf("%s: %w", platform.Key(), err)
		}
	}
	for _, platform := range requiredPlatforms {
		if _, ok := seenPlatforms[platform]; !ok {
			return fmt.Errorf("missing bundle for supported platform %s", platform.Key())
		}
	}
	return nil
}

func validateBundle(manifest TrustedManifest, bundle BundleManifest) error {
	if bundle.RuntimeLibrary != runtimeLibNameFor(bundle.Platform()) {
		return fmt.Errorf("runtime library %q does not match platform", bundle.RuntimeLibrary)
	}
	expectedFilename := fmt.Sprintf("pii-model-%s-%s.tar.gz", manifest.ModelVersion, bundle.Platform().Key())
	if bundle.Archive.Filename != expectedFilename {
		return fmt.Errorf("archive filename %q, want %q", bundle.Archive.Filename, expectedFilename)
	}
	if err := validateHTTPSURL(bundle.Archive.URL); err != nil {
		return fmt.Errorf("archive URL: %w", err)
	}
	if path.Base(mustURLPath(bundle.Archive.URL)) != bundle.Archive.Filename {
		return fmt.Errorf("archive URL does not end with %q", bundle.Archive.Filename)
	}
	if !validHexDigest(bundle.Archive.SHA256, 64) {
		return fmt.Errorf("archive SHA-256 is invalid")
	}
	if bundle.Archive.Size <= 0 || bundle.Archive.MaxSize < bundle.Archive.Size ||
		bundle.Archive.MaxSize > maximumArchiveSizeLimit {
		return fmt.Errorf("archive size bounds are invalid")
	}
	if err := validateHTTPSURL(bundle.RuntimeSource.URL); err != nil {
		return fmt.Errorf("runtime source URL: %w", err)
	}
	if !validHexDigest(bundle.RuntimeSource.SHA256, 64) || bundle.RuntimeSource.Size <= 0 ||
		bundle.RuntimeSource.Size > maximumRuntimeSourceSize {
		return fmt.Errorf("runtime source identity is invalid")
	}
	if bundle.MaxExpandedSize <= 0 || bundle.MaxExpandedSize > maximumExpandedSizeLimit {
		return fmt.Errorf("expanded size limit is invalid")
	}

	requiredNames := []string{"config.json", "model_quantized.onnx", "tokenizer.json", bundle.RuntimeLibrary}
	if len(bundle.Files) != len(requiredNames) {
		return fmt.Errorf("file count %d, want %d", len(bundle.Files), len(requiredNames))
	}
	seenFiles := make(map[string]struct{}, len(bundle.Files))
	var expanded int64
	for _, file := range bundle.Files {
		if file.Name == "" || path.Base(file.Name) != file.Name || strings.Contains(file.Name, "\\") {
			return fmt.Errorf("file name %q is not a safe archive basename", file.Name)
		}
		if _, duplicate := seenFiles[file.Name]; duplicate {
			return fmt.Errorf("duplicate file %q", file.Name)
		}
		seenFiles[file.Name] = struct{}{}
		if !validHexDigest(file.SHA256, 64) || file.Size <= 0 {
			return fmt.Errorf("file identity for %q is invalid", file.Name)
		}
		if file.Size > bundle.MaxExpandedSize-expanded {
			return fmt.Errorf("declared files exceed expanded size limit")
		}
		expanded += file.Size
	}
	for _, required := range requiredNames {
		if _, ok := seenFiles[required]; !ok {
			return fmt.Errorf("required file %q is missing", required)
		}
	}
	return nil
}

func validHexDigest(value string, length int) bool {
	if len(value) != length || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateHTTPSURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("must be an absolute HTTPS URL without credentials or fragment")
	}
	return nil
}

func mustURLPath(raw string) string {
	parsed, _ := url.Parse(raw)
	return parsed.Path
}

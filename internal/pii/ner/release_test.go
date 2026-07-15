package ner

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type releaseSourceLock struct {
	SchemaVersion int    `json:"schema_version"`
	BundleVersion string `json:"bundle_version"`
	Model         struct {
		Repository string `json:"repository"`
		Revision   string `json:"revision"`
		Files      []struct {
			SourcePath string `json:"source_path"`
			Name       string `json:"name"`
			SHA256     string `json:"sha256"`
			Size       int64  `json:"size"`
		} `json:"files"`
	} `json:"model"`
	ONNXRuntime struct {
		Version   string `json:"version"`
		Platforms []struct {
			GOOS        string `json:"goos"`
			GOARCH      string `json:"goarch"`
			URL         string `json:"url"`
			SHA256      string `json:"sha256"`
			Size        int64  `json:"size"`
			LibraryGlob string `json:"library_glob"`
			LibraryName string `json:"library_name"`
		} `json:"platforms"`
	} `json:"onnx_runtime"`
	Limits struct {
		MaxArchiveSize  int64 `json:"max_archive_size"`
		MaxExpandedSize int64 `json:"max_expanded_size"`
	} `json:"limits"`
}

func TestPIIModelReleaseWorkflowParsesAndSmokesEverySupportedPlatform(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "pii-model.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow yaml.Node
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatalf("workflow YAML: %v", err)
	}
	text := string(data)
	for _, platform := range SupportedPlatforms() {
		if !strings.Contains(text, "platform: "+platform.Key()) {
			t.Fatalf("release workflow has no smoke job for %s", platform.Key())
		}
	}
	if strings.Contains(text, "platform: windows-") {
		t.Fatal("release workflow advertises unsupported Windows runtime")
	}
}

func TestReleaseSourceLockMatchesTrustedManifest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "pii-model-sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lock releaseSourceLock
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&lock); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadTrustedManifest()
	if err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != 1 || lock.BundleVersion != manifest.ModelVersion ||
		lock.Model.Repository != manifest.ModelRepository ||
		lock.Model.Revision != manifest.ModelRevision ||
		lock.ONNXRuntime.Version != manifest.ONNXRuntimeVersion {
		t.Fatal("release source identity does not match the embedded manifest")
	}

	gotPlatforms := make([]Platform, 0, len(lock.ONNXRuntime.Platforms))
	for _, source := range lock.ONNXRuntime.Platforms {
		platform := Platform{OS: source.GOOS, Arch: source.GOARCH}
		gotPlatforms = append(gotPlatforms, platform)
		bundle, err := manifest.BundleFor(platform)
		if err != nil {
			t.Fatal(err)
		}
		if source.URL != bundle.RuntimeSource.URL || source.SHA256 != bundle.RuntimeSource.SHA256 ||
			source.Size != bundle.RuntimeSource.Size || source.LibraryName != bundle.RuntimeLibrary {
			t.Fatalf("%s runtime source does not match trusted bundle", platform.Key())
		}
		if lock.Limits.MaxArchiveSize != bundle.Archive.MaxSize ||
			lock.Limits.MaxExpandedSize != bundle.MaxExpandedSize {
			t.Fatalf("%s limits do not match trusted bundle", platform.Key())
		}
	}
	if got, want := platformKeys(gotPlatforms), platformKeys(SupportedPlatforms()); !slices.Equal(got, want) {
		t.Fatalf("source-lock platforms = %v, want %v", got, want)
	}

	for _, sourceFile := range lock.Model.Files {
		for _, bundle := range manifest.Bundles {
			found := false
			for _, trustedFile := range bundle.Files {
				if trustedFile.Name == sourceFile.Name {
					found = true
					if trustedFile.SHA256 != sourceFile.SHA256 || trustedFile.Size != sourceFile.Size {
						t.Fatalf("%s differs in %s", sourceFile.Name, bundle.Platform().Key())
					}
				}
			}
			if !found {
				t.Fatalf("%s missing from %s", sourceFile.Name, bundle.Platform().Key())
			}
		}
	}
}

package ner

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLegacyBundle(t *testing.T, dir string, platform Platform, includeFiles bool) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".version"), []byte(ModelVersion), 0o600); err != nil {
		t.Fatal(err)
	}
	if !includeFiles {
		return
	}
	for _, name := range []string{runtimeLibNameFor(platform), "model_quantized.onnx", "tokenizer.json", "config.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestModelStatusUnsupportedEvenWithInstalledFiles(t *testing.T) {
	dir := t.TempDir()
	platform := Platform{OS: "windows", Arch: "amd64"}
	writeLegacyBundle(t, dir, platform, true)

	status := statusAt(dir, platform)
	if status.State != ModelUnsupported {
		t.Fatalf("status = %s, want %s", status.State, ModelUnsupported)
	}
	if status.Usable() {
		t.Fatal("unsupported runtime reported a usable model")
	}
}

func TestModelStatusVersionMarkerAloneIsCorrupt(t *testing.T) {
	dir := t.TempDir()
	platform := Platform{OS: "linux", Arch: "amd64"}
	writeLegacyBundle(t, dir, platform, false)

	status := statusAt(dir, platform)
	if status.State != ModelCorrupt {
		t.Fatalf("status = %s, want %s", status.State, ModelCorrupt)
	}
	if status.Usable() {
		t.Fatal("version marker alone reported a usable model")
	}
}

func TestModelStatusDistinguishesAbsentAndInstalledUnverified(t *testing.T) {
	platform := Platform{OS: "linux", Arch: "amd64"}
	dir := t.TempDir()
	if status := statusAt(dir, platform); status.State != ModelAbsent {
		t.Fatalf("empty status = %s, want %s", status.State, ModelAbsent)
	}

	writeLegacyBundle(t, dir, platform, true)
	status := statusAt(dir, platform)
	if status.State != ModelInstalledUnverified {
		t.Fatalf("installed status = %s, want %s", status.State, ModelInstalledUnverified)
	}
	if !status.Usable() {
		t.Fatal("supported legacy bundle should remain usable until trusted migration is installed")
	}
}

func TestEnsureModelUnsupportedPlatformDoesNotDownloadOrMutateCache(t *testing.T) {
	cacheParent := t.TempDir()
	cacheDir := filepath.Join(cacheParent, "pii-model")
	downloadCalls := 0
	_, err := ensureModelAt(
		Platform{OS: "windows", Arch: "amd64"},
		cacheDir,
		nil,
		func(string, string, ProgressFunc) error {
			downloadCalls++
			return nil
		},
	)
	if err == nil {
		t.Fatal("unsupported install returned no error")
	}
	if downloadCalls != 0 {
		t.Fatalf("download calls = %d, want 0", downloadCalls)
	}
	if _, statErr := os.Stat(cacheDir); !os.IsNotExist(statErr) {
		t.Fatalf("unsupported install mutated cache: %v", statErr)
	}
}

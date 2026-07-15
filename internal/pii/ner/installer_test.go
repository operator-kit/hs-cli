package ner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var fixturePlatform = Platform{OS: "linux", Arch: "amd64"}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type tarFixtureEntry struct {
	Name     string
	Body     []byte
	Typeflag byte
	Linkname string
}

func fixtureContents(platform Platform) map[string][]byte {
	return map[string][]byte{
		"config.json":               []byte("{\"id2label\":{\"0\":\"O\",\"1\":\"B-PER\"}}"),
		"model_quantized.onnx":      []byte("tiny-model-fixture"),
		"tokenizer.json":            []byte("{\"model\":{\"type\":\"WordPiece\"}}"),
		runtimeLibNameFor(platform): []byte("tiny-runtime-fixture"),
	}
}

func regularFixtureEntries(platform Platform, contents map[string][]byte) []tarFixtureEntry {
	names := []string{"config.json", "model_quantized.onnx", "tokenizer.json", runtimeLibNameFor(platform)}
	entries := []tarFixtureEntry{{Name: "./", Typeflag: tar.TypeDir}}
	for _, name := range names {
		entries = append(entries, tarFixtureEntry{Name: "./" + name, Body: contents[name], Typeflag: tar.TypeReg})
	}
	return entries
}

func makeFixtureArchive(t *testing.T, entries []tarFixtureEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		typeflag := entry.Typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		size := int64(0)
		if typeflag == tar.TypeReg || typeflag == tar.TypeRegA {
			size = int64(len(entry.Body))
		}
		header := &tar.Header{
			Name:     entry.Name,
			Mode:     0o777,
			Size:     size,
			Typeflag: typeflag,
			Linkname: entry.Linkname,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if size > 0 {
			if _, err := tarWriter.Write(entry.Body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func fixtureManifestForArchive(
	t *testing.T,
	platform Platform,
	archive []byte,
	contents map[string][]byte,
	mutate func(*BundleManifest),
) TrustedManifest {
	t.Helper()
	manifest, err := LoadTrustedManifest()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Bundles = append([]BundleManifest(nil), manifest.Bundles...)
	index := -1
	for candidate := range manifest.Bundles {
		if manifest.Bundles[candidate].Platform() == platform {
			index = candidate
			break
		}
	}
	if index < 0 {
		t.Fatalf("fixture platform %s missing from manifest", platform.Key())
	}
	bundle := manifest.Bundles[index]
	bundle.Files = nil
	var expanded int64
	for _, name := range []string{"config.json", "model_quantized.onnx", "tokenizer.json", runtimeLibNameFor(platform)} {
		data := contents[name]
		sum := sha256.Sum256(data)
		bundle.Files = append(bundle.Files, FileManifest{
			Name:   name,
			SHA256: hex.EncodeToString(sum[:]),
			Size:   int64(len(data)),
		})
		expanded += int64(len(data))
	}
	archiveSum := sha256.Sum256(archive)
	bundle.Archive.SHA256 = hex.EncodeToString(archiveSum[:])
	bundle.Archive.Size = int64(len(archive))
	bundle.Archive.MaxSize = int64(len(archive)) + 1024
	bundle.MaxExpandedSize = expanded + 1024
	if mutate != nil {
		mutate(&bundle)
	}
	manifest.Bundles[index] = bundle
	manifest.fingerprint = ""
	if err := manifest.Validate(); err != nil {
		t.Fatalf("fixture manifest is invalid: %v", err)
	}
	return manifest
}

type servedInstaller struct {
	installer *BundleInstaller
	requests  *atomic.Int32
}

func newServedInstaller(
	t *testing.T,
	manifest TrustedManifest,
	archive []byte,
	status int,
	validator RuntimeValidator,
) servedInstaller {
	t.Helper()
	requests := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if status != http.StatusOK {
			writer.WriteHeader(status)
			return
		}
		writer.Header().Set("Content-Type", "application/gzip")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(archive)
	}))
	t.Cleanup(server.Close)
	installer := NewBundleInstaller(t.TempDir(), fixturePlatform, manifest)
	installer.Client = server.Client()
	installer.ResolveURL = func(BundleManifest) string { return server.URL }
	installer.ValidateRuntime = validator
	return servedInstaller{installer: installer, requests: requests}
}

func TestInstallerTrustedBundlePromotesAtomically(t *testing.T) {
	contents := fixtureContents(fixturePlatform)
	archive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, contents))
	manifest := fixtureManifestForArchive(t, fixturePlatform, archive, contents, nil)
	validatorCalls := 0
	served := newServedInstaller(t, manifest, archive, http.StatusOK, func(ctx context.Context, paths *Paths) error {
		validatorCalls++
		for _, path := range []string{paths.RuntimeLib, paths.ModelONNX, paths.TokenizerJSON, paths.ConfigJSON} {
			if _, err := os.Stat(path); err != nil {
				return err
			}
		}
		return ctx.Err()
	})

	var progress [][2]int64
	paths, err := served.installer.Install(context.Background(), func(read, total int64) {
		progress = append(progress, [2]int64{read, total})
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if validatorCalls != 1 {
		t.Fatalf("validator calls = %d, want 1", validatorCalls)
	}
	if served.requests.Load() != 1 {
		t.Fatalf("requests = %d, want 1", served.requests.Load())
	}
	if len(progress) == 0 {
		t.Fatal("progress callback was not called")
	}
	var previous int64
	for _, event := range progress {
		if event[0] < previous || event[0] > event[1] || event[1] != int64(len(archive)) {
			t.Fatalf("invalid progress sequence: %v", progress)
		}
		previous = event[0]
	}
	if previous != int64(len(archive)) {
		t.Fatalf("final progress = %d, want %d", previous, len(archive))
	}

	status := statusAtWithManifest(served.installer.CacheRoot, fixturePlatform, manifest)
	if status.State != ModelReady || !status.Usable() {
		t.Fatalf("status = %+v, want ready", status)
	}
	if filepath.Dir(paths.ModelONNX) != status.Dir {
		t.Fatalf("paths resolve to %q, status dir is %q", filepath.Dir(paths.ModelONNX), status.Dir)
	}
	bundle, err := manifest.BundleFor(fixturePlatform)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(status.Dir, bundle.Archive.Filename)); !os.IsNotExist(err) {
		t.Fatalf("download archive was promoted with content: %v", err)
	}
	for name, want := range contents {
		got, err := os.ReadFile(filepath.Join(status.Dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s content changed", name)
		}
	}

	again, err := served.installer.Install(context.Background(), nil)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if served.requests.Load() != 1 || validatorCalls != 1 {
		t.Fatalf("idempotent install repeated work: requests=%d validators=%d", served.requests.Load(), validatorCalls)
	}
	if again.ModelONNX != paths.ModelONNX {
		t.Fatalf("idempotent paths changed: %q != %q", again.ModelONNX, paths.ModelONNX)
	}
}

func TestInstallerRejectsMutatedArchiveBeforeExtraction(t *testing.T) {
	contents := fixtureContents(fixturePlatform)
	archive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, contents))
	manifest := fixtureManifestForArchive(t, fixturePlatform, archive, contents, nil)
	mutated := append([]byte(nil), archive...)
	mutated[len(mutated)/2] ^= 0xff
	served := newServedInstaller(t, manifest, mutated, http.StatusOK, func(context.Context, *Paths) error {
		t.Fatal("runtime validator ran for a mutated archive")
		return nil
	})

	_, err := served.installer.Install(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("mutated archive error = %v", err)
	}
	assertNoFinalInstall(t, served.installer, manifest)
}

func TestInstallerRejectsInnerFileHashMismatch(t *testing.T) {
	contents := fixtureContents(fixturePlatform)
	archive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, contents))
	manifest := fixtureManifestForArchive(t, fixturePlatform, archive, contents, func(bundle *BundleManifest) {
		bundle.Files[0].SHA256 = strings.Repeat("0", 64)
	})
	served := newServedInstaller(t, manifest, archive, http.StatusOK, func(context.Context, *Paths) error {
		t.Fatal("runtime validator ran for an inner hash mismatch")
		return nil
	})
	if _, err := served.installer.Install(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "file") {
		t.Fatalf("inner hash error = %v", err)
	}
	assertNoFinalInstall(t, served.installer, manifest)
}

func TestInstallerRejectsUnsafeOrIncompleteArchive(t *testing.T) {
	contents := fixtureContents(fixturePlatform)
	regular := regularFixtureEntries(fixturePlatform, contents)
	wrongSize := append([]tarFixtureEntry(nil), regular...)
	wrongSize[1].Body = append(append([]byte(nil), wrongSize[1].Body...), 'x')
	tests := []struct {
		name    string
		entries []tarFixtureEntry
	}{
		{"missing", regular[:len(regular)-1]},
		{"unexpected", append([]tarFixtureEntry{{Name: "unexpected.txt", Body: []byte("x"), Typeflag: tar.TypeReg}}, regular...)},
		{"duplicate", append([]tarFixtureEntry{{Name: "config.json", Body: contents["config.json"], Typeflag: tar.TypeReg}}, regular...)},
		{"traversal", append([]tarFixtureEntry{{Name: "../outside", Body: []byte("x"), Typeflag: tar.TypeReg}}, regular...)},
		{"absolute", append([]tarFixtureEntry{{Name: "/outside", Body: []byte("x"), Typeflag: tar.TypeReg}}, regular...)},
		{"windows-separator", append([]tarFixtureEntry{{Name: "..\\outside", Body: []byte("x"), Typeflag: tar.TypeReg}}, regular...)},
		{"symlink", append([]tarFixtureEntry{{Name: "config.json", Typeflag: tar.TypeSymlink, Linkname: "outside"}}, regular[2:]...)},
		{"hardlink", append([]tarFixtureEntry{{Name: "config.json", Typeflag: tar.TypeLink, Linkname: "outside"}}, regular[2:]...)},
		{"fifo", append([]tarFixtureEntry{{Name: "config.json", Typeflag: tar.TypeFifo}}, regular[2:]...)},
		{"device", append([]tarFixtureEntry{{Name: "config.json", Typeflag: tar.TypeChar}}, regular[2:]...)},
		{"nested-directory", append([]tarFixtureEntry{{Name: "nested/", Typeflag: tar.TypeDir}}, regular...)},
		{"wrong-file-size", wrongSize},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archive := makeFixtureArchive(t, test.entries)
			manifest := fixtureManifestForArchive(t, fixturePlatform, archive, contents, nil)
			served := newServedInstaller(t, manifest, archive, http.StatusOK, func(context.Context, *Paths) error {
				t.Fatal("runtime validator ran for an unsafe archive")
				return nil
			})
			if _, err := served.installer.Install(context.Background(), nil); err == nil {
				t.Fatal("unsafe archive was accepted")
			}
			assertNoFinalInstall(t, served.installer, manifest)
		})
	}
}

func TestInstallerRejectsOversizedAndTruncatedResponses(t *testing.T) {
	contents := fixtureContents(fixturePlatform)
	archive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, contents))
	manifest := fixtureManifestForArchive(t, fixturePlatform, archive, contents, nil)

	t.Run("oversized", func(t *testing.T) {
		oversized := append(append([]byte(nil), archive...), 0)
		served := newServedInstaller(t, manifest, oversized, http.StatusOK, func(context.Context, *Paths) error {
			return nil
		})
		var maxProgress int64
		_, err := served.installer.Install(context.Background(), func(read, _ int64) {
			if read > maxProgress {
				maxProgress = read
			}
		})
		if err == nil {
			t.Fatal("oversized response was accepted")
		}
		if maxProgress > int64(len(archive)) {
			t.Fatalf("progress exceeded trusted size: %d > %d", maxProgress, len(archive))
		}
	})

	t.Run("truncated-gzip", func(t *testing.T) {
		truncated := archive[:len(archive)-8]
		truncatedManifest := fixtureManifestForArchive(t, fixturePlatform, truncated, contents, nil)
		served := newServedInstaller(t, truncatedManifest, truncated, http.StatusOK, func(context.Context, *Paths) error {
			return nil
		})
		if _, err := served.installer.Install(context.Background(), nil); err == nil {
			t.Fatal("truncated gzip stream was accepted")
		}
	})
}

func TestInstallerFailureLeavesPriorTrustedInstallUntouched(t *testing.T) {
	firstContents := fixtureContents(fixturePlatform)
	firstArchive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, firstContents))
	firstManifest := fixtureManifestForArchive(t, fixturePlatform, firstArchive, firstContents, nil)
	first := newServedInstaller(t, firstManifest, firstArchive, http.StatusOK, func(context.Context, *Paths) error {
		return nil
	})
	paths, err := first.installer.Install(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(paths.ConfigJSON)
	if err != nil {
		t.Fatal(err)
	}

	secondContents := fixtureContents(fixturePlatform)
	secondContents["config.json"] = []byte("{\"replacement\":true}")
	secondArchive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, secondContents))
	secondManifest := fixtureManifestForArchive(t, fixturePlatform, secondArchive, secondContents, nil)
	second := newServedInstaller(t, secondManifest, secondArchive, http.StatusOK, func(context.Context, *Paths) error {
		return errors.New("smoke inference failed")
	})
	second.installer.CacheRoot = first.installer.CacheRoot
	if _, err := second.installer.Install(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "smoke inference failed") {
		t.Fatalf("replacement error = %v", err)
	}

	after, err := os.ReadFile(paths.ConfigJSON)
	if err != nil {
		t.Fatalf("previous installation disappeared: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("failed replacement changed the previous installation")
	}
	if status := statusAtWithManifest(first.installer.CacheRoot, fixturePlatform, firstManifest); status.State != ModelReady {
		t.Fatalf("previous manifest status = %+v", status)
	}
	assertNoFinalInstall(t, second.installer, secondManifest)
}

func TestInstallerConcurrentInstallsConverge(t *testing.T) {
	contents := fixtureContents(fixturePlatform)
	archive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, contents))
	manifest := fixtureManifestForArchive(t, fixturePlatform, archive, contents, nil)
	validationBarrier := make(chan struct{})
	var validations atomic.Int32
	var release sync.Once
	served := newServedInstaller(t, manifest, archive, http.StatusOK, func(context.Context, *Paths) error {
		if validations.Add(1) == 2 {
			release.Do(func() { close(validationBarrier) })
		}
		<-validationBarrier
		return nil
	})

	errorsFound := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := served.installer.Install(context.Background(), nil)
			errorsFound <- err
		}()
	}
	for range 2 {
		if err := <-errorsFound; err != nil {
			t.Fatalf("concurrent Install: %v", err)
		}
	}
	if status := statusAtWithManifest(served.installer.CacheRoot, fixturePlatform, manifest); status.State != ModelReady {
		t.Fatalf("concurrent status = %+v", status)
	}
	stagingEntries, err := os.ReadDir(filepath.Join(served.installer.CacheRoot, ".staging"))
	if err != nil {
		t.Fatal(err)
	}
	if len(stagingEntries) != 0 {
		t.Fatalf("concurrent install left staging entries: %v", stagingEntries)
	}
}

func TestInstallerRejectsHTTPFailureAndCancellation(t *testing.T) {
	contents := fixtureContents(fixturePlatform)
	archive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, contents))
	manifest := fixtureManifestForArchive(t, fixturePlatform, archive, contents, nil)

	t.Run("non-200", func(t *testing.T) {
		served := newServedInstaller(t, manifest, nil, http.StatusNotFound, func(context.Context, *Paths) error {
			return nil
		})
		if _, err := served.installer.Install(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "404") {
			t.Fatalf("HTTP failure error = %v", err)
		}
		assertNoFinalInstall(t, served.installer, manifest)
	})

	t.Run("cancelled", func(t *testing.T) {
		requests := new(atomic.Int32)
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			requests.Add(1)
			<-request.Context().Done()
		}))
		defer server.Close()
		installer := NewBundleInstaller(t.TempDir(), fixturePlatform, manifest)
		installer.Client = server.Client()
		installer.ResolveURL = func(BundleManifest) string { return server.URL }
		installer.ValidateRuntime = func(context.Context, *Paths) error { return nil }
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if _, err := installer.Install(ctx, nil); err == nil {
			t.Fatal("cancelled request returned no error")
		}
		if requests.Load() != 1 {
			t.Fatalf("requests = %d, want 1", requests.Load())
		}
		assertNoFinalInstall(t, installer, manifest)
	})
}

func TestInstallerRuntimeValidationHonorsContextCancellation(t *testing.T) {
	contents := fixtureContents(fixturePlatform)
	archive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, contents))
	manifest := fixtureManifestForArchive(t, fixturePlatform, archive, contents, nil)
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	served := newServedInstaller(t, manifest, archive, http.StatusOK, func(context.Context, *Paths) error {
		close(started)
		<-release
		close(finished)
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := served.installer.Install(ctx, nil)
		result <- err
	}()
	<-started
	begin := time.Now()
	cancel()
	var err error
	select {
	case err = <-result:
	case <-time.After(time.Second):
		t.Fatal("cancelled runtime validation did not return")
	}
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("runtime cancellation error = %v", err)
	}
	if elapsed := time.Since(begin); elapsed > time.Second {
		t.Fatalf("runtime cancellation took %v", elapsed)
	}
	close(release)
	<-finished

	bundle, _ := manifest.BundleFor(fixturePlatform)
	if _, err := os.Stat(trustedInstallDir(served.installer.CacheRoot, fixturePlatform, bundle)); !os.IsNotExist(err) {
		t.Fatalf("timed-out runtime produced a final install: %v", err)
	}
	staging := filepath.Join(served.installer.CacheRoot, ".staging")
	entries, err := os.ReadDir(staging)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("timed-out native validation staging entries = %d, want 1", len(entries))
	}
	old := time.Now().Add(-staleStagingAge - time.Hour)
	if err := os.Chtimes(filepath.Join(staging, entries[0].Name()), old, old); err != nil {
		t.Fatal(err)
	}
	if err := cleanupStaleStaging(staging, time.Now()); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(staging); err != nil || len(entries) != 0 {
		t.Fatalf("stale timed-out staging was not cleaned: entries=%v error=%v", entries, err)
	}
}

func TestInstallerRejectsInvalidManifestBeforeRequestOrCacheMutation(t *testing.T) {
	contents := fixtureContents(fixturePlatform)
	archive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, contents))
	manifest := fixtureManifestForArchive(t, fixturePlatform, archive, contents, nil)
	manifest.ModelVersion = "wrong"
	cacheRoot := filepath.Join(t.TempDir(), "pii-model")
	requests := 0
	installer := &BundleInstaller{
		Client: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests++
			return nil, errors.New("unexpected request")
		})},
		CacheRoot:       cacheRoot,
		Platform:        fixturePlatform,
		Manifest:        manifest,
		ValidateRuntime: func(context.Context, *Paths) error { return nil },
	}
	if _, err := installer.Install(context.Background(), nil); err == nil {
		t.Fatal("invalid manifest was accepted")
	}
	if requests != 0 {
		t.Fatalf("invalid manifest issued %d requests", requests)
	}
	if _, err := os.Stat(cacheRoot); !os.IsNotExist(err) {
		t.Fatalf("invalid manifest mutated cache: %v", err)
	}
}

func TestInstallerRejectsSymlinkedStagingRoot(t *testing.T) {
	contents := fixtureContents(fixturePlatform)
	archive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, contents))
	manifest := fixtureManifestForArchive(t, fixturePlatform, archive, contents, nil)
	served := newServedInstaller(t, manifest, archive, http.StatusOK, func(context.Context, *Paths) error {
		return nil
	})
	cacheRoot := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(cacheRoot, ".staging")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	served.installer.CacheRoot = cacheRoot
	if _, err := served.installer.Install(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "real directory") {
		t.Fatalf("symlinked staging error = %v", err)
	}
	if served.requests.Load() != 0 {
		t.Fatalf("symlinked staging issued %d requests", served.requests.Load())
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("symlink target was mutated: %v", entries)
	}
}

func TestCleanupStaleStagingIsBoundedToOldInstallerEntries(t *testing.T) {
	staging := t.TempDir()
	old := filepath.Join(staging, "install-old")
	fresh := filepath.Join(staging, "install-fresh")
	unrelated := filepath.Join(staging, "operator-note")
	for _, path := range []string{old, fresh, unrelated} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	oldTime := now.Add(-staleStagingAge - time.Hour)
	if err := os.Chtimes(old, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := cleanupStaleStaging(staging, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old installer staging survived: %v", err)
	}
	for _, path := range []string{fresh, unrelated} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("cleanup removed %s: %v", filepath.Base(path), err)
		}
	}
}

func TestReadyMarkerIsWrittenOnlyOnceAndStatusDetectsDamage(t *testing.T) {
	contents := fixtureContents(fixturePlatform)
	archive := makeFixtureArchive(t, regularFixtureEntries(fixturePlatform, contents))
	manifest := fixtureManifestForArchive(t, fixturePlatform, archive, contents, nil)
	served := newServedInstaller(t, manifest, archive, http.StatusOK, func(context.Context, *Paths) error {
		return nil
	})
	paths, err := served.installer.Install(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	bundle, _ := manifest.BundleFor(fixturePlatform)
	if err := writeReadyMarker(filepath.Dir(paths.ModelONNX), fixturePlatform, bundle, manifest); err == nil {
		t.Fatal("ready marker was overwritten")
	}
	if err := os.Truncate(paths.ConfigJSON, 1); err != nil {
		t.Fatal(err)
	}
	status := statusAtWithManifest(served.installer.CacheRoot, fixturePlatform, manifest)
	if status.State != ModelCorrupt || status.Usable() {
		t.Fatalf("damaged install status = %+v", status)
	}
}

func assertNoFinalInstall(t *testing.T, installer *BundleInstaller, manifest TrustedManifest) {
	t.Helper()
	bundle, err := manifest.BundleFor(installer.Platform)
	if err != nil {
		t.Fatal(err)
	}
	final := trustedInstallDir(installer.CacheRoot, installer.Platform, bundle)
	if _, err := os.Lstat(final); !os.IsNotExist(err) {
		t.Fatalf("failed install left final directory %q: %v", final, err)
	}
	staging := filepath.Join(installer.CacheRoot, ".staging")
	entries, err := os.ReadDir(staging)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed install left staging entries: %v", entries)
	}
}

func TestCopyExactArchiveRejectsShortWrite(t *testing.T) {
	writer := shortWriter{}
	if _, err := copyExactArchive(writer, bytes.NewReader([]byte("abcd")), 4, nil); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write error = %v", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	return len(data) - 1, nil
}

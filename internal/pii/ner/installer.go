package ner

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultInstallTimeout        = 15 * time.Minute
	defaultResponseHeaderTimeout = 30 * time.Second
	defaultMaxRedirects          = 3
	staleStagingAge              = 24 * time.Hour
)

type InstallLimits struct {
	OperationTimeout time.Duration
}

type RuntimeValidator func(context.Context, *Paths) error

type BundleInstaller struct {
	Client          *http.Client
	CacheRoot       string
	Platform        Platform
	Manifest        TrustedManifest
	Limits          InstallLimits
	ResolveURL      func(BundleManifest) string
	ValidateRuntime RuntimeValidator
}

func NewBundleInstaller(cacheRoot string, platform Platform, manifest TrustedManifest) *BundleInstaller {
	return &BundleInstaller{
		Client:          defaultModelHTTPClient(),
		CacheRoot:       cacheRoot,
		Platform:        platform,
		Manifest:        manifest,
		Limits:          InstallLimits{OperationTimeout: defaultInstallTimeout},
		ValidateRuntime: validateRuntimeBundle,
	}
}

func defaultModelHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= defaultMaxRedirects {
				return fmt.Errorf("stopped after %d redirects", defaultMaxRedirects)
			}
			if req.URL.Scheme != "https" || req.URL.Host == "" || req.URL.User != nil {
				return fmt.Errorf("refusing non-HTTPS model redirect")
			}
			return nil
		},
	}
}

func (i *BundleInstaller) Install(ctx context.Context, progress ProgressFunc) (*Paths, error) {
	if i == nil {
		return nil, fmt.Errorf("PII model installer is nil")
	}
	capability := RuntimeCapabilityFor(i.Platform)
	if !capability.Supported {
		return nil, fmt.Errorf("PII model unsupported: %s", capability.Reason)
	}
	if i.CacheRoot == "" {
		return nil, fmt.Errorf("PII model cache root is empty")
	}
	if err := i.Manifest.Validate(); err != nil {
		return nil, fmt.Errorf("trusted PII model manifest: %w", err)
	}
	bundle, err := i.Manifest.BundleFor(i.Platform)
	if err != nil {
		return nil, err
	}
	if i.ValidateRuntime == nil {
		return nil, fmt.Errorf("PII model runtime validator is required")
	}
	client := i.Client
	if client == nil {
		client = defaultModelHTTPClient()
	}

	if status := statusAtWithManifest(i.CacheRoot, i.Platform, i.Manifest); status.State == ModelReady {
		return pathsAt(status.Dir, i.Platform), nil
	}

	timeout := i.Limits.OperationTimeout
	if timeout <= 0 {
		timeout = defaultInstallTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stagingRoot, err := prepareStagingRoot(i.CacheRoot)
	if err != nil {
		return nil, fmt.Errorf("create PII model staging root: %w", err)
	}
	stage, err := os.MkdirTemp(stagingRoot, "install-")
	if err != nil {
		return nil, fmt.Errorf("create PII model staging directory: %w", err)
	}
	if err := os.Chmod(stage, 0o700); err != nil {
		_ = os.RemoveAll(stage)
		return nil, fmt.Errorf("secure PII model staging directory: %w", err)
	}
	cleanupStage := true
	defer func() {
		if cleanupStage {
			_ = os.RemoveAll(stage)
		}
	}()

	archivePath := filepath.Join(stage, bundle.Archive.Filename)
	rawURL := bundle.Archive.URL
	if i.ResolveURL != nil {
		rawURL = i.ResolveURL(bundle)
	}
	if err := downloadTrustedArchive(ctx, client, rawURL, archivePath, bundle.Archive, progress); err != nil {
		return nil, fmt.Errorf("download trusted PII model archive: %w", err)
	}

	contentDir := filepath.Join(stage, "content")
	if err := os.Mkdir(contentDir, 0o700); err != nil {
		return nil, fmt.Errorf("create staged PII model content: %w", err)
	}
	if err := extractTrustedArchive(archivePath, contentDir, bundle); err != nil {
		return nil, fmt.Errorf("extract trusted PII model archive: %w", err)
	}

	stagedPaths := pathsAt(contentDir, i.Platform)
	validationDone := make(chan error, 1)
	go func() {
		validationDone <- i.ValidateRuntime(ctx, stagedPaths)
	}()
	select {
	case err := <-validationDone:
		if err != nil {
			return nil, fmt.Errorf("validate staged PII model runtime: %w", err)
		}
	case <-ctx.Done():
		// The native runtime call cannot be forcibly interrupted safely. Leave
		// its private staging directory in place while this process exits; the
		// bounded stale-staging cleanup removes it on a later invocation.
		cleanupStage = false
		return nil, fmt.Errorf("validate staged PII model runtime: %w", ctx.Err())
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("validate staged PII model runtime: %w", err)
	}
	if err := writeReadyMarker(contentDir, i.Platform, bundle, i.Manifest); err != nil {
		return nil, fmt.Errorf("write PII model ready marker: %w", err)
	}

	finalDir := trustedInstallDir(i.CacheRoot, i.Platform, bundle)
	if err := promoteTrustedInstall(contentDir, finalDir, i.CacheRoot, i.Platform, i.Manifest); err != nil {
		return nil, fmt.Errorf("promote trusted PII model: %w", err)
	}

	status := statusAtWithManifest(i.CacheRoot, i.Platform, i.Manifest)
	if status.State != ModelReady {
		return nil, fmt.Errorf("promoted PII model is %s: %s", status.State, status.Reason)
	}
	return pathsAt(status.Dir, i.Platform), nil
}

func downloadTrustedArchive(
	ctx context.Context,
	client *http.Client,
	rawURL string,
	destination string,
	expected ArchiveManifest,
	progress ProgressFunc,
) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("request archive: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	if response.ContentLength >= 0 && response.ContentLength != expected.Size {
		return fmt.Errorf("declared content length %d, want %d", response.ContentLength, expected.Size)
	}
	if expected.Size > expected.MaxSize {
		return fmt.Errorf("trusted archive size %d exceeds limit %d", expected.Size, expected.MaxSize)
	}

	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create staged archive: %w", err)
	}
	hash := sha256.New()
	written, copyErr := copyExactArchive(io.MultiWriter(file, hash), response.Body, expected.Size, progress)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return fmt.Errorf("close staged archive: %w", closeErr)
	}
	if written != expected.Size {
		return fmt.Errorf("archive size %d, want %d", written, expected.Size)
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != expected.SHA256 {
		return fmt.Errorf("archive SHA-256 %s does not match trusted manifest", actualHash)
	}
	return nil
}

func copyExactArchive(destination io.Writer, source io.Reader, expected int64, progress ProgressFunc) (int64, error) {
	if expected <= 0 {
		return 0, fmt.Errorf("expected archive size must be positive")
	}
	buffer := make([]byte, 32*1024)
	var written int64
	emptyReads := 0
	for {
		remaining := expected - written
		readSize := int64(len(buffer))
		if remaining < readSize {
			readSize = remaining + 1
		}
		if readSize <= 0 {
			readSize = 1
		}
		n, readErr := source.Read(buffer[:readSize])
		if n == 0 && readErr == nil {
			emptyReads++
			if emptyReads >= 100 {
				return written, io.ErrNoProgress
			}
			continue
		}
		emptyReads = 0
		if n > 0 {
			if int64(n) > remaining {
				return written, fmt.Errorf("archive exceeds trusted size %d", expected)
			}
			count, writeErr := destination.Write(buffer[:n])
			written += int64(count)
			if writeErr != nil {
				return written, fmt.Errorf("write staged archive: %w", writeErr)
			}
			if count != n {
				return written, io.ErrShortWrite
			}
			if progress != nil {
				progress(written, expected)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return written, fmt.Errorf("read archive response: %w", readErr)
		}
	}
	if written != expected {
		return written, fmt.Errorf("archive ended at %d bytes, want %d", written, expected)
	}
	return written, nil
}

func extractTrustedArchive(archivePath, destination string, bundle BundleManifest) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("gzip header: %w", err)
	}
	defer gzipReader.Close()

	expected := make(map[string]FileManifest, len(bundle.Files))
	for _, file := range bundle.Files {
		expected[file.Name] = file
	}
	seen := make(map[string]struct{}, len(expected))
	tarReader := tar.NewReader(gzipReader)
	var expanded int64
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return fmt.Errorf("tar stream: %w", nextErr)
		}

		name, root, nameErr := strictArchiveName(header.Name)
		if nameErr != nil {
			return nameErr
		}
		if root {
			if header.Typeflag != tar.TypeDir {
				return fmt.Errorf("archive root entry must be a directory")
			}
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return fmt.Errorf("archive entry %q has forbidden type %d", header.Name, header.Typeflag)
		}
		spec, ok := expected[name]
		if !ok {
			return fmt.Errorf("archive contains unexpected file %q", name)
		}
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("archive contains duplicate file %q", name)
		}
		if header.Size != spec.Size {
			return fmt.Errorf("archive file %q has size %d, want %d", name, header.Size, spec.Size)
		}
		if header.Size > bundle.MaxExpandedSize-expanded {
			return fmt.Errorf("archive exceeds expanded size limit %d", bundle.MaxExpandedSize)
		}
		expanded += header.Size
		if err := extractTrustedFile(tarReader, filepath.Join(destination, name), spec); err != nil {
			return err
		}
		seen[name] = struct{}{}
	}
	// tar stops at its end markers; drain gzip so its checksum and trailer are
	// verified before the staged content can be promoted.
	if _, err := io.Copy(io.Discard, gzipReader); err != nil {
		return fmt.Errorf("gzip trailer: %w", err)
	}
	if len(seen) != len(expected) {
		missing := make([]string, 0, len(expected)-len(seen))
		for name := range expected {
			if _, ok := seen[name]; !ok {
				missing = append(missing, name)
			}
		}
		return fmt.Errorf("archive is missing required files: %s", strings.Join(missing, ", "))
	}
	return nil
}

func strictArchiveName(raw string) (name string, root bool, err error) {
	if raw == "." || raw == "./" {
		return "", true, nil
	}
	if raw == "" || strings.ContainsRune(raw, '\x00') || strings.Contains(raw, "\\") || path.IsAbs(raw) {
		return "", false, fmt.Errorf("archive entry has unsafe path %q", raw)
	}
	name = strings.TrimPrefix(raw, "./")
	if name == "" || name == "." || name == ".." || path.Clean(name) != name ||
		strings.HasPrefix(name, "../") || strings.Contains(name, "/") {
		return "", false, fmt.Errorf("archive entry has unsafe path %q", raw)
	}
	return name, false, nil
}

func extractTrustedFile(reader io.Reader, destination string, expected FileManifest) error {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create staged file %q: %w", expected.Name, err)
	}
	hash := sha256.New()
	written, copyErr := io.CopyN(io.MultiWriter(file, hash), reader, expected.Size)
	if copyErr != nil {
		_ = file.Close()
		return fmt.Errorf("extract file %q: %w", expected.Name, copyErr)
	}
	if written != expected.Size {
		_ = file.Close()
		return fmt.Errorf("file %q size %d, want %d", expected.Name, written, expected.Size)
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != expected.SHA256 {
		_ = file.Close()
		return fmt.Errorf("file %q SHA-256 %s does not match trusted manifest", expected.Name, actualHash)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync staged file %q: %w", expected.Name, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close staged file %q: %w", expected.Name, err)
	}
	return nil
}

func promoteTrustedInstall(contentDir, finalDir, cacheRoot string, platform Platform, manifest TrustedManifest) error {
	if status := statusAtWithManifest(cacheRoot, platform, manifest); status.State == ModelReady {
		return nil
	}
	parent := filepath.Dir(finalDir)
	if err := ensureInstallParent(cacheRoot, parent); err != nil {
		return err
	}

	var replaced string
	if _, err := os.Lstat(finalDir); err == nil {
		placeholder, tempErr := os.MkdirTemp(parent, ".replaced-")
		if tempErr != nil {
			return tempErr
		}
		if removeErr := os.Remove(placeholder); removeErr != nil {
			return removeErr
		}
		replaced = placeholder
		if renameErr := os.Rename(finalDir, replaced); renameErr != nil {
			return renameErr
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(contentDir, finalDir); err != nil {
		if replaced != "" {
			_ = os.Rename(replaced, finalDir)
		}
		if status := statusAtWithManifest(cacheRoot, platform, manifest); status.State == ModelReady {
			if replaced != "" {
				_ = os.RemoveAll(replaced)
			}
			return nil
		}
		return err
	}
	if replaced != "" {
		_ = os.RemoveAll(replaced)
	}
	return nil
}

func prepareStagingRoot(cacheRoot string) (string, error) {
	if err := ensurePrivateDirectory(cacheRoot); err != nil {
		return "", err
	}
	stagingRoot := filepath.Join(cacheRoot, ".staging")
	if err := ensurePrivateDirectory(stagingRoot); err != nil {
		return "", err
	}
	if err := cleanupStaleStaging(stagingRoot, time.Now()); err != nil {
		return "", err
	}
	return stagingRoot, nil
}

func cleanupStaleStaging(stagingRoot string, now time.Time) error {
	entries, err := os.ReadDir(stagingRoot)
	if err != nil {
		return err
	}
	cutoff := now.Add(-staleStagingAge)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "install-") {
			continue
		}
		target := filepath.Join(stagingRoot, entry.Name())
		info, err := os.Lstat(target)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("remove stale staging entry %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func ensureInstallParent(cacheRoot, parent string) error {
	root, err := filepath.Abs(cacheRoot)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(parent)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("install path escapes cache root")
	}
	if err := ensurePrivateDirectory(root); err != nil {
		return err
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." || component == ".." {
			return fmt.Errorf("install path contains unsafe component %q", component)
		}
		current = filepath.Join(current, component)
		if err := ensurePrivateDirectory(current); err != nil {
			return err
		}
	}
	return nil
}

func ensurePrivateDirectory(dir string) error {
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a real directory", dir)
	}
	return os.Chmod(dir, 0o700)
}

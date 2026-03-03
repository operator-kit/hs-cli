package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
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
	"time"
)

// InstallDirOverride allows tests to override the install directory detection.
var InstallDirOverride string

func archiveName(version string) string {
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("hs_%s_%s_%s.%s", version, runtime.GOOS, runtime.GOARCH, ext)
}

func findAsset(release *ReleaseResponse, name string) *Asset {
	for i := range release.Assets {
		if release.Assets[i].Name == name {
			return &release.Assets[i]
		}
	}
	return nil
}

func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func verifyChecksum(data []byte, expectedHash string) error {
	h := sha256.Sum256(data)
	actual := hex.EncodeToString(h[:])
	if actual != expectedHash {
		return fmt.Errorf("checksum mismatch: got %s, want %s", actual, expectedHash)
	}
	return nil
}

func findChecksum(checksums []byte, archiveName string) (string, error) {
	for _, line := range strings.Split(string(checksums), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == archiveName {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("checksum not found for %s", archiveName)
}

// Update downloads and installs the release, replacing the current binary.
func Update(release *ReleaseResponse, out io.Writer) error {
	version := strings.TrimPrefix(release.TagName, "v")
	archive := archiveName(version)

	archiveAsset := findAsset(release, archive)
	if archiveAsset == nil {
		return fmt.Errorf("no asset found for %s", archive)
	}
	checksumAsset := findAsset(release, "checksums.txt")
	if checksumAsset == nil {
		return fmt.Errorf("no checksums.txt in release")
	}

	fmt.Fprintf(out, "Downloading %s...\n", archive)

	checksumData, err := download(checksumAsset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	expectedHash, err := findChecksum(checksumData, archive)
	if err != nil {
		return err
	}

	archiveData, err := download(archiveAsset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("download archive: %w", err)
	}

	if err := verifyChecksum(archiveData, expectedHash); err != nil {
		return err
	}
	fmt.Fprintln(out, "Checksum verified.")

	// Extract to temp dir
	tmpDir, err := os.MkdirTemp("", "hs-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if runtime.GOOS == "windows" {
		err = extractZip(archiveData, tmpDir)
	} else {
		err = extractTarGz(archiveData, tmpDir)
	}
	if err != nil {
		return fmt.Errorf("extract archive: %w", err)
	}

	// Determine install directory
	installDir := InstallDirOverride
	if installDir == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("find current binary: %w", err)
		}
		exe, err = filepath.EvalSymlinks(exe)
		if err != nil {
			return fmt.Errorf("resolve symlinks: %w", err)
		}
		installDir = filepath.Dir(exe)
	}

	// Replace hs binary
	binName := "hs"
	if runtime.GOOS == "windows" {
		binName = "hs.exe"
	}
	src := filepath.Join(tmpDir, binName)
	dst := filepath.Join(installDir, binName)
	fmt.Fprintf(out, "Replacing %s...\n", dst)
	if err := replaceBinary(src, dst); err != nil {
		return fmt.Errorf("replace %s: %w", binName, err)
	}

	return nil
}

func extractTarGz(data []byte, destDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Base(hdr.Name)
		if name == "." || name == ".." {
			continue
		}

		dst := filepath.Join(destDir, name)
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(f, tr)
		f.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func extractZip(data []byte, destDir string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		if name == "." || name == ".." {
			continue
		}

		src, err := f.Open()
		if err != nil {
			return err
		}
		dst := filepath.Join(destDir, name)
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(out, src)
		src.Close()
		out.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func replaceBinary(src, dst string) error {
	if runtime.GOOS == "windows" {
		return replaceBinaryWindows(src, dst)
	}
	return replaceBinaryUnix(src, dst)
}

func replaceBinaryUnix(src, dst string) error {
	newPath := dst + ".new"
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		os.Remove(newPath)
		return err
	}
	dstFile.Close()

	if err := os.Rename(newPath, dst); err != nil {
		os.Remove(newPath)
		return err
	}
	return nil
}

func replaceBinaryWindows(src, dst string) error {
	oldPath := dst + ".old"
	_ = os.Remove(oldPath) // cleanup leftover from previous update

	if err := os.Rename(dst, oldPath); err != nil {
		return fmt.Errorf("rename current binary: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		_ = os.Rename(oldPath, dst) // rollback
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		_ = os.Rename(oldPath, dst)
		return err
	}
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		_ = os.Remove(dst)
		_ = os.Rename(oldPath, dst)
		return err
	}
	dstFile.Close()

	_ = os.Remove(oldPath) // best-effort cleanup
	return nil
}

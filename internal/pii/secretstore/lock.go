package secretstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type FileLock struct {
	path string
}

func NewFileLock(path string) *FileLock {
	return &FileLock{path: path}
}

func (l *FileLock) WithLock(ctx context.Context, fn func() error) error {
	if l == nil || l.path == "" {
		return fmt.Errorf("PII secret initialization lock path is empty")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return fmt.Errorf("creating PII secret lock directory: %w", err)
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("opening PII secret initialization lock: %w", err)
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("securing PII secret initialization lock: %w", err)
	}
	if err := acquireFileLock(ctx, file); err != nil {
		return fmt.Errorf("acquiring PII secret initialization lock: %w", err)
	}

	runErr := fn()
	unlockErr := releaseFileLock(file)
	if runErr != nil {
		return runErr
	}
	if unlockErr != nil {
		return fmt.Errorf("releasing PII secret initialization lock: %w", unlockErr)
	}
	return nil
}

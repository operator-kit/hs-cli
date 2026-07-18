//go:build windows

package secretstore

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func acquireFileLock(ctx context.Context, file *os.File) error {
	handle := windows.Handle(file.Fd())
	for {
		overlapped := new(windows.Overlapped)
		err := windows.LockFileEx(
			handle,
			windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
			0,
			1,
			0,
			overlapped,
		)
		if err == nil {
			return nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func releaseFileLock(file *os.File) error {
	overlapped := new(windows.Overlapped)
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlapped)
}

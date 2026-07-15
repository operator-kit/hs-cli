//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package secretstore

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func acquireFileLock(ctx context.Context, file *os.File) error {
	for {
		err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
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
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}

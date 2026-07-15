//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd || windows)

package secretstore

import (
	"context"
	"fmt"
	"os"
)

func acquireFileLock(context.Context, *os.File) error {
	return fmt.Errorf("cross-process file locking is unsupported on this platform")
}

func releaseFileLock(*os.File) error {
	return nil
}

package secretstore

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestFileLockSerializesCallers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pii-secret.lock")
	first := NewFileLock(path)
	second := NewFileLock(path)
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondEntered := make(chan struct{})
	errs := make(chan error, 2)

	go func() {
		errs <- first.WithLock(context.Background(), func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstEntered

	go func() {
		errs <- second.WithLock(context.Background(), func() error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("second caller entered while first held the lock")
	case <-time.After(75 * time.Millisecond):
	}
	close(releaseFirst)

	select {
	case <-secondEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("second caller did not enter after lock release")
	}
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("WithLock returned error: %v", err)
		}
	}
}

func TestFileLockHonorsCancelledContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pii-secret.lock")
	first := NewFileLock(path)
	second := NewFileLock(path)
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)

	go func() {
		firstDone <- first.WithLock(context.Background(), func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstEntered

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := second.WithLock(ctx, func() error {
		t.Fatal("cancelled waiter acquired the lock")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WithLock error = %v, want context.Canceled", err)
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first WithLock returned error: %v", err)
	}
}

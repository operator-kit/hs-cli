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
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)

	go func() {
		firstDone <- first.WithLock(context.Background(), func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	select {
	case <-firstEntered:
	case err := <-firstDone:
		t.Fatalf("first caller failed to acquire lock: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("first caller did not acquire lock")
	}
	releasedFirst := false
	defer func() {
		if !releasedFirst {
			close(releaseFirst)
		}
	}()

	go func() {
		secondDone <- second.WithLock(context.Background(), func() error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("second caller entered while first held the lock")
	case err := <-secondDone:
		t.Fatalf("second caller returned while first held the lock: %v", err)
	case <-time.After(75 * time.Millisecond):
	}
	close(releaseFirst)
	releasedFirst = true

	select {
	case <-secondEntered:
	case err := <-secondDone:
		secondDone = nil
		if err != nil {
			t.Fatalf("second caller failed while waiting for lock: %v", err)
		}
		select {
		case <-secondEntered:
		default:
			t.Fatal("second caller returned without entering the lock")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second caller did not enter after lock release")
	}
	if err := <-firstDone; err != nil {
		t.Fatalf("first WithLock returned error: %v", err)
	}
	if secondDone != nil {
		if err := <-secondDone; err != nil {
			t.Fatalf("second WithLock returned error: %v", err)
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
	select {
	case <-firstEntered:
	case err := <-firstDone:
		t.Fatalf("first caller failed to acquire lock: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("first caller did not acquire lock")
	}
	releasedFirst := false
	defer func() {
		if !releasedFirst {
			close(releaseFirst)
		}
	}()

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
	releasedFirst = true
	if err := <-firstDone; err != nil {
		t.Fatalf("first WithLock returned error: %v", err)
	}
}

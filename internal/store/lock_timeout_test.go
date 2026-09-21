package store

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"IntegTERM/internal/model"
)

// Keep a real transaction open until release. Cleanup releases and joins it
// even if an assertion fails, so no file handles survive TempDir cleanup.
func holdStoreTransaction(t *testing.T, instance *Store) func() {
	t.Helper()
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- instance.WithTransaction(func(tx *Transaction) error {
			close(entered)
			<-release
			return tx.SaveConfig(model.Config{WindowX: 17})
		})
	}()
	var once sync.Once
	unlock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() {
		unlock()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("holder transaction: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("holder transaction did not stop")
		}
	})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("holder did not acquire the file lock")
	}
	return unlock
}

func TestStoreLockTimeoutAndRetryAfterRelease(t *testing.T) {
	dir := t.TempDir()
	holder := New(dir)
	unlock := holdStoreTransaction(t, holder)
	const timeout = 50 * time.Millisecond
	contender := NewWithLockTimeout(dir, timeout)
	var called atomic.Bool
	started := time.Now()
	err := contender.WithTransaction(func(*Transaction) error { called.Store(true); return nil })
	if !errors.Is(err, ErrLockTimeout) {
		t.Fatalf("contending transaction = %v, want ErrLockTimeout", err)
	}
	if elapsed := time.Since(started); elapsed < timeout || elapsed > 2*time.Second {
		t.Fatalf("lock timeout returned after %s, want about %s", elapsed, timeout)
	}
	if called.Load() {
		t.Fatal("timed-out transaction callback ran")
	}
	unlock()
	// The same instance must be reusable; no canceled waiter may later steal
	// the lock or keep a background OS lock request alive.
	cfg, err := contender.LoadConfig()
	if err != nil || cfg.WindowX != 17 {
		t.Fatalf("retry after release = %+v, %v", cfg, err)
	}
}

func TestStoreLockTimeoutDoesNotCancelActiveTransaction(t *testing.T) {
	dir := t.TempDir()
	instance := NewWithLockTimeout(dir, 20*time.Millisecond)
	err := instance.WithTransaction(func(tx *Transaction) error {
		time.Sleep(60 * time.Millisecond)
		// Its acquisition deadline expired, but it still owns the lock.
		other := NewWithLockTimeout(dir, 15*time.Millisecond)
		if _, err := other.LoadConfig(); !errors.Is(err, ErrLockTimeout) {
			t.Errorf("active transaction lost its lock: %v", err)
		}
		return tx.SaveConfig(model.Config{WindowX: 29})
	})
	if err != nil {
		t.Fatalf("active transaction canceled by lock deadline: %v", err)
	}
	cfg, err := instance.LoadConfig()
	if err != nil || cfg.WindowX != 29 {
		t.Fatalf("transaction did not complete: %+v, %v", cfg, err)
	}
}

func TestExistingStoreConstructorStillWaitsForLock(t *testing.T) {
	dir := t.TempDir()
	unlock := holdStoreTransaction(t, New(dir))
	contender := New(dir)
	done := make(chan error, 1)
	go func() { _, err := contender.LoadConfig(); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("default constructor did not wait for existing lock: %v", err)
	case <-time.After(60 * time.Millisecond):
	}
	unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("default constructor failed after release: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("default constructor did not acquire released lock")
	}
}

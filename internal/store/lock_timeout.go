package store

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// ErrLockTimeout means another transaction still owns the storage lock. It is
// safe to retry: the timed-out caller has not started its transaction callback.
var ErrLockTimeout = errors.New("storage is busy: timed out waiting for the file lock")

func lockFileWithTimeout(file *os.File, timeout time.Duration) error {
	if timeout <= 0 {
		return lockFile(file)
	}
	deadline := time.Now().Add(timeout)
	for {
		locked, err := tryLockFile(file)
		if err != nil || locked {
			return err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("%w after %s", ErrLockTimeout, timeout)
		}
		time.Sleep(min(10*time.Millisecond, remaining))
		if !time.Now().Before(deadline) {
			return fmt.Errorf("%w after %s", ErrLockTimeout, timeout)
		}
	}
}

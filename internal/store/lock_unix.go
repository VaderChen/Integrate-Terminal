//go:build !windows

package store

import (
	"os"
	"syscall"
)

func lockFile(file *os.File) error {
	for {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
		if err != syscall.EINTR {
			return err
		}
	}
}
func unlockFile(file *os.File) { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }

// A separate open file description competes even within the same process.
func tryLockFile(file *os.File) (bool, error) {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN || err == syscall.EINTR {
		return false, nil
	}
	return err == nil, err
}

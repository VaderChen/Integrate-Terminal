//go:build !windows

package app

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func acquireBackgroundServiceLock(path string) (*BackgroundServiceLock, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrBackgroundServiceAlreadyRunning
		}
		return nil, fmt.Errorf("lock background service: %w", err)
	}
	return &BackgroundServiceLock{file: file}, nil
}

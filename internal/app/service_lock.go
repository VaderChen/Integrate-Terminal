package app

import (
	"errors"
	"os"
)

var ErrBackgroundServiceAlreadyRunning = errors.New("background service is already running")

type BackgroundServiceLock struct {
	file *os.File
}

func (lock *BackgroundServiceLock) Release() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	file := lock.file
	lock.file = nil
	return file.Close()
}

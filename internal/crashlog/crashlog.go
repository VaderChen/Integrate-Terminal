package crashlog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

var (
	initOnce sync.Once
	logPath  string
)

func Init() string {
	initOnce.Do(func() {
		baseDir := resolveAppDataDir()
		_ = os.MkdirAll(baseDir, 0o755)
		logPath = filepath.Join(baseDir, "service-crash.log")
	})
	return logPath
}

func Path() string {
	return Init()
}

func Recover(scope string) {
	if recovered := recover(); recovered != nil {
		Write(scope, recovered)
	}
}

func Write(scope string, recovered interface{}) {
	path := Init()
	payload := fmt.Sprintf("[%s] panic in %s: %v\n%s\n",
		time.Now().Format(time.RFC3339),
		scope,
		recovered,
		debug.Stack(),
	)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("write crash log failed (%s): %v; original panic: %v", path, err, recovered)
		return
	}
	defer file.Close()

	if _, err := file.WriteString(payload); err != nil {
		log.Printf("append crash log failed (%s): %v; original panic: %v", path, err, recovered)
		return
	}

	log.Printf("panic recovered in %s; stack trace written to %s", scope, path)
}

func resolveAppDataDir() string {
	baseDir, err := os.UserConfigDir()
	if err == nil && strings.TrimSpace(baseDir) != "" {
		return filepath.Join(baseDir, "IntegTERM")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".integterm")
	}
	return "data"
}

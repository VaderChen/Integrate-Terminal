package crashlog

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"IntegTERM/internal/boundedlog"
)

// 崩潰與 REST 伺服器錯誤共用同一個檔案及儲存上限。
const maxLogBytes = 5 * 1024 * 1024

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

func Writer() io.Writer { return logWriter{path: Init()} }

type logWriter struct{ path string }

func (writer logWriter) Write(data []byte) (int, error) {
	n, err := boundedlog.Append(writer.path, data, maxLogBytes)
	if err != nil {
		return os.Stderr.Write(data)
	}
	return n, nil
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

	if _, err := boundedlog.Append(path, []byte(payload), maxLogBytes); err != nil {
		log.Printf("write crash log failed (%s): %v; original panic: %v", path, err, recovered)
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

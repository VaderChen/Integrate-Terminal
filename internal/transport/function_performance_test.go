package transport

import (
	"os"
	"strings"
	"testing"
)

func BenchmarkRemotePathInspection(b *testing.B) {
	target := "/root/" + strings.Repeat("directory/", 20) + "file"
	lstat := func(value string) (os.FileMode, error) {
		if value == target {
			return 0, nil
		}
		return os.ModeDir, nil
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, exists, err := inspectRemoteRootPath("/root", target, false, lstat); err != nil || !exists {
			b.Fatalf("路徑驗證失敗：%v", err)
		}
	}
}

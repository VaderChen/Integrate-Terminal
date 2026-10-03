package session

import (
	"bytes"
	"fmt"
	"testing"

	"IntegTERM/internal/model"
)

// 使用固定的終端區塊與歷史筆數，比較高頻路徑的時間和配置量。
func BenchmarkTerminalOutput(b *testing.B) {
	for _, name := range []string{"plain", "ansi", "cwd"} {
		b.Run(name, func(b *testing.B) {
			line := []byte("terminal output line for throughput measurement\r\n")
			if name == "ansi" {
				line = []byte("\x1b[32mterminal output line\x1b[0m\r\n")
			} else if name == "cwd" {
				line = []byte("terminal output\r\n\x1b]9;cwd=/srv/project\a")
			}
			chunk := bytes.Repeat(line, 4096/len(line))
			buffer := bytes.Repeat([]byte("x\n"), sshOutputBufferLimit/2)
			b.SetBytes(int64(len(chunk)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				complete, _ := splitUTF8SafeChunk(nil, chunk)
				visible, _, _ := stripTerminalSignals(nil, complete)
				buffer = appendTerminalOutput(buffer, visible)
			}
			if len(buffer) > sshOutputBufferLimit {
				b.Fatal("終端緩衝超過上限")
			}
		})
	}
}

func BenchmarkAppendLogs(b *testing.B) {
	for _, count := range []int{1000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				manager := &Manager{}
				for j := 0; j < count; j++ {
					manager.AppendLog("檔案傳輸完成", "done")
				}
			}
		})
	}
}

func BenchmarkAppendTransfers(b *testing.B) {
	for _, count := range []int{1000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				manager := &Manager{transferParents: make(map[string]string)}
				for j := 0; j < count; j++ {
					manager.addTransfer("檔案", "download")
				}
				if got := len(manager.SampleTransfers()); got != count {
					b.Fatalf("傳輸筆數 = %d，預期 %d", got, count)
				}
			}
		})
	}
}

func BenchmarkPausedTransferUpdate(b *testing.B) {
	manager := &Manager{
		pausedTransfers: map[string]bool{"paused": true},
	}
	manager.insertTransferLocked(model.TransferItem{ID: "paused", Progress: 40, Status: "paused"})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		manager.updateTransfer("paused", 40, 0, "paused")
	}
}

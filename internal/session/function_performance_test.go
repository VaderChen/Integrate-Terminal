package session

import (
	"bytes"
	"testing"
)

func BenchmarkTelnetPlainOutput(b *testing.B) {
	session := &telnetTerminalSession{}
	data := bytes.Repeat([]byte("terminal output\r\n"), 256)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(session.negotiate(data)) != len(data) {
			b.Fatal("一般輸出內容不完整")
		}
	}
}

func BenchmarkTelnetInput(b *testing.B) {
	data := string(bytes.Repeat([]byte("echo test\r\n"), 1000))
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(normalizeTelnetInput(data)) != len(data) {
			b.Fatal("CRLF 不應重複展開")
		}
	}
}

func BenchmarkTransferProgressLookup(b *testing.B) {
	manager := &Manager{transferParents: make(map[string]string)}
	first := manager.addTransfer("first", "download")
	for i := 1; i < 10000; i++ {
		manager.addTransfer("next", "download")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.updateTransfer(first, i%100, 100, "running")
		if manager.transferProgress(first) != i%100 {
			b.Fatal("進度讀寫不一致")
		}
	}
}

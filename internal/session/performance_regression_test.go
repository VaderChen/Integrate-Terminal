package session

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"IntegTERM/internal/model"
)

type limitedTerminalReader struct {
	io.Reader
	limit int
}

func (r limitedTerminalReader) Read(buffer []byte) (int, error) {
	return r.Reader.Read(buffer[:min(len(buffer), r.limit)])
}

func TestTerminalStreamPreservesSplitUTF8AndControlSequences(t *testing.T) {
	input := "開始🌿\r\n\x1b[32mgreen\x1b[0m\x1b]9;cwd=/srv/專案\a尾端\x1b]10;color\x1b\\!\x1b]incomplete"
	want := "開始🌿\r\n\x1b[32mgreen\x1b[0m尾端!"
	for _, size := range []int{1, 2, 3, 4, 7, 31, 4096} {
		manager := &Manager{}
		session := &sshTerminalSession{id: "chunked"}
		manager.streamSSHOutput(nil, session, limitedTerminalReader{strings.NewReader(input), size})
		if got := string(session.outputBuffer); got != want {
			t.Fatalf("區塊大小 %d：輸出 = %q，預期 %q", size, got, want)
		}
	}
}

func TestTerminalSignalsRetainPendingBytesWhenReaderReusesBuffer(t *testing.T) {
	chunk := []byte("visible\x1b]9;cwd=/srv/")
	visible, pending, _ := stripTerminalSignals(nil, chunk)
	if string(visible) != "visible" {
		t.Fatalf("可見輸出 = %q", visible)
	}
	clear(chunk)
	visible, pending, cwds := stripTerminalSignals(pending, []byte("project\a尾端"))
	if string(visible) != "尾端" || len(pending) != 0 || len(cwds) != 1 || cwds[0] != "/srv/project" {
		t.Fatalf("跨區塊控制序列遺失：%q %q %v", visible, pending, cwds)
	}
	utf8Chunk := []byte{'a', 0xe4, 0xb8}
	complete, rest := splitUTF8SafeChunk(nil, utf8Chunk)
	if string(complete) != "a" {
		t.Fatalf("完整 UTF-8 區段 = %q", complete)
	}
	clear(utf8Chunk)
	complete, rest = splitUTF8SafeChunk(rest, []byte{0xad})
	if string(complete) != "中" || len(rest) != 0 {
		t.Fatalf("跨區塊 UTF-8 遺失：%q %q", complete, rest)
	}
}

func TestTerminalBufferReusePreservesLimitUTF8AndSnapshots(t *testing.T) {
	session := &sshTerminalSession{id: "buffer"}
	manager := &Manager{sshSessions: map[string]*sshTerminalSession{session.id: session}}
	session.outputBuffer = appendTerminalOutput(nil, []byte("原有內容\n"))
	snapshot := manager.GetTerminalOutputSnapshot(session.id)
	chunk := []byte(strings.Repeat("界", 1000))
	for range 100 {
		session.outputBuffer = appendTerminalOutput(session.outputBuffer, chunk)
	}
	if !utf8.Valid(session.outputBuffer) || len(session.outputBuffer) > sshOutputBufferLimit {
		t.Fatal("緩衝截斷破壞 UTF-8 或超過上限")
	}
	if got, want := string(session.outputBuffer), strings.Repeat("界", sshOutputBufferLimit/3); got != want {
		t.Fatalf("緩衝尾端內容錯誤：取得 %d bytes，預期 %d bytes", len(got), len(want))
	}
	if snapshot.Output != "原有內容\n" {
		t.Fatal("重用緩衝改變既有快照")
	}
}

func TestLogSnapshotsKeepNewestFirstAndIndependent(t *testing.T) {
	manager := &Manager{}
	manager.AppendLog("第一筆", "done")
	manager.AppendLog("第二筆", "failed")
	snapshot := manager.SampleLogs()
	manager.AppendLog("第三筆", "running")
	latest := manager.SampleLogs()
	if len(latest) != 3 || latest[0].Message != "第三筆" || latest[1].Message != "第二筆" || latest[2].Message != "第一筆" {
		t.Fatalf("日誌順序錯誤：%v", latest)
	}
	latest[1].Message = "呼叫端修改"
	if snapshot[0].Message != "第二筆" || manager.SampleLogs()[1].Message != "第二筆" {
		t.Fatal("日誌快照共用可變資料")
	}
	if logs := manager.ClearLogs(); logs == nil || len(logs) != 0 {
		t.Fatalf("清空日誌應回傳空陣列：%v", logs)
	}
	if len(snapshot) != 2 || snapshot[1].Message != "第一筆" {
		t.Fatal("清空日誌改變既有快照")
	}
}

func TestLogLimitRetainsLatestEntriesAcrossWrapsAndClear(t *testing.T) {
	manager := &Manager{}
	const total = maxLogEntries*3 + 17
	for i := 0; i < total; i++ {
		manager.AppendLog(fmt.Sprint(i), "done")
	}
	snapshot := manager.SampleLogs()
	if len(snapshot) != maxLogEntries {
		t.Fatalf("日誌筆數 = %d，預期上限 %d", len(snapshot), maxLogEntries)
	}
	for i, item := range snapshot {
		if want := fmt.Sprint(total - 1 - i); item.Message != want {
			t.Fatalf("第 %d 筆 = %q，預期 %q", i, item.Message, want)
		}
	}
	manager.AppendLog("覆寫最舊紀錄", "failed")
	if snapshot[len(snapshot)-1].Message != fmt.Sprint(total-maxLogEntries) {
		t.Fatal("淘汰最舊紀錄改變已回傳快照")
	}
	manager.ClearLogs()
	for i := 0; i < 4; i++ {
		manager.AppendLog(fmt.Sprint(i), "running")
	}
	for i, item := range manager.SampleLogs() {
		if item.Message != fmt.Sprint(3-i) {
			t.Fatalf("清空後日誌順序錯誤：%v", manager.SampleLogs())
		}
	}
}

func TestTransferQueueKeepsOrderAndSnapshots(t *testing.T) {
	manager := &Manager{transferParents: make(map[string]string)}
	first := manager.addTransfer("第一筆", "upload")
	second := manager.addTransfer("第二筆", "download")
	snapshot := manager.SampleTransfers()
	third := manager.addTransfer("第三筆", "upload")
	manager.updateTransfer(second, 100, 0, "done")
	remaining := manager.ClearCompletedTransfers()
	if len(remaining) != 2 || remaining[0].ID != third || remaining[1].ID != first {
		t.Fatalf("清理後順序錯誤：%v", remaining)
	}
	manager.removeTransfer(third)
	if items := manager.SampleTransfers(); len(items) != 1 || items[0].ID != first {
		t.Fatalf("移除後內容錯誤：%v", items)
	}
	if len(snapshot) != 2 || snapshot[0].ID != second || snapshot[0].Status != "running" || snapshot[1].ID != first {
		t.Fatal("佇列異動改變既有快照")
	}
}

func TestTransferNotificationsRequireSubscriberAndChangedState(t *testing.T) {
	manager := &Manager{
		stateEvents:     make(chan struct{}, 1),
		pausedTransfers: map[string]bool{"paused": true},
	}
	manager.insertTransferLocked(model.TransferItem{ID: "paused", Progress: 40, Status: "paused"})
	manager.AppendLog("背景服務紀錄", "done")
	if len(manager.stateEvents) != 0 || len(manager.SampleLogs()) != 1 {
		t.Fatal("背景服務應保存資料，不排入 GUI 通知")
	}
	manager.SetEventContext(context.Background())
	manager.updateTransfer("paused", 40, 0, "paused")
	if len(manager.stateEvents) != 0 {
		t.Fatal("相同暫停狀態不應重複通知")
	}
	manager.updateTransfer("paused", 41, 0, "running")
	if len(manager.stateEvents) != 1 || manager.SampleTransfers()[0].Status != "paused" {
		t.Fatal("進度變更必須通知，且不能解除既有暫停")
	}
}

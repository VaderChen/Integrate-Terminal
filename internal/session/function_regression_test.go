package session

import (
	"bytes"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTransferIDsRemainUniqueWhenClockMovesBackward(t *testing.T) {
	lastID := time.Now().Add(time.Hour).UnixNano()
	manager := &Manager{lastItemTimestamp: lastID}
	first := manager.addTransfer("first", "download")
	manager.removeTransfer(first)
	second := manager.addTransfer("second", "download")
	if first != "transfer-"+strconv.FormatInt(lastID+1, 10) || second != "transfer-"+strconv.FormatInt(lastID+2, 10) {
		t.Fatalf("時鐘回退重複使用 ID：%s %s", first, second)
	}
}

func TestLogIDsRemainUniqueAcrossClockRollbackAndClear(t *testing.T) {
	lastID := time.Now().Add(time.Hour).UnixNano()
	manager := &Manager{lastItemTimestamp: lastID}
	manager.AppendLog("first", "done")
	first := manager.SampleLogs()[0].ID
	manager.ClearLogs()
	manager.AppendLog("second", "done")
	second := manager.SampleLogs()[0].ID
	if first != "log-"+strconv.FormatInt(lastID+1, 10) || second != "log-"+strconv.FormatInt(lastID+2, 10) {
		t.Fatalf("清空與時鐘回退重複使用日誌 ID：%s %s", first, second)
	}
}

func TestTransferIndexMaintainsOrderThroughArbitraryRemoval(t *testing.T) {
	manager := &Manager{transferParents: make(map[string]string), pausedTransfers: make(map[string]bool), cancelledTransfers: make(map[string]bool)}
	ids := make([]string, 500)
	for i := range ids {
		ids[i] = manager.addTransfer(fmt.Sprint(i), "download")
	}
	original := manager.SampleTransfers()
	removed := make(map[string]bool)
	for _, index := range rand.New(rand.NewSource(12)).Perm(len(ids)) {
		id := ids[index]
		manager.updateTransfer(id, index%100, 12, "running")
		if manager.transferProgress(id) != index%100 {
			t.Fatal("進度索引指向其他傳輸")
		}
		manager.removeTransfer(id)
		removed[id] = true
		remaining := manager.SampleTransfers()
		position := 0
		for i := len(ids) - 1; i >= 0; i-- {
			if !removed[ids[i]] {
				if position >= len(remaining) || remaining[position].ID != ids[i] {
					t.Fatalf("移除 %d 後順序錯誤", index)
				}
				position++
			}
		}
		if len(remaining) != position {
			t.Fatal("索引筆數與快照不符")
		}
	}
	if manager.newestTransfer != nil || manager.transferItems != nil {
		t.Fatal("空佇列仍保留節點或索引容量")
	}
	for _, item := range original {
		if item.Progress != 0 || item.Status != "running" {
			t.Fatal("進度異動改變既有快照")
		}
	}
}

func TestTransferIndexConcurrentReadersAndWorkers(t *testing.T) {
	manager := &Manager{transferParents: make(map[string]string)}
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := 0; i < 100; i++ {
				id := manager.addTransfer("worker", "upload")
				manager.updateTransfer(id, 50, 1024, "running")
				if manager.transferProgress(id) != 50 {
					t.Error("並行更新遺失進度")
				}
				manager.SampleTransfers()
				manager.removeTransfer(id)
			}
		}()
	}
	workers.Wait()
	if len(manager.SampleTransfers()) != 0 {
		t.Fatal("工作結束後仍有殘留傳輸")
	}
}

func TestIndexedTransferControlsPreserveBulkAndIndividualBehavior(t *testing.T) {
	manager := &Manager{transferParents: make(map[string]string), pausedTransfers: make(map[string]bool), cancelledTransfers: make(map[string]bool)}
	first := manager.addTransfer("first", "upload")
	second := manager.addTransfer("second", "download")
	failed := manager.addTransfer("failed", "download")
	manager.updateTransfer(failed, 30, 0, "failed")
	manager.TogglePauseAllTransfers()
	third := manager.addTransfer("third", "upload")
	for _, item := range manager.SampleTransfers() {
		want := "paused"
		if item.ID == failed {
			want = "failed"
		}
		if item.Status != want {
			t.Fatalf("全部暫停狀態錯誤：%+v", item)
		}
	}
	manager.TogglePauseAllTransfers()
	manager.TogglePauseTransfer(first)
	if !manager.isTransferPaused(first) || manager.isTransferPaused(second) || manager.isTransferPaused(third) {
		t.Fatal("個別暫停影響其他傳輸")
	}
	manager.TogglePauseTransfer(first)
	manager.CancelTransfer(second)
	manager.updateTransfer(second, 90, 100, "running")
	remaining := manager.ClearCompletedTransfers()
	if len(remaining) != 3 || remaining[0].ID != third || remaining[1].ID != failed || remaining[2].ID != first {
		t.Fatalf("清除完成項目改變保留順序：%v", remaining)
	}
	if remaining[0].Status != "running" || remaining[1].Status != "failed" || remaining[2].Status != "running" {
		t.Fatal("繼續或清除操作改變失敗項目狀態")
	}
	if !manager.isTransferCancelled(second) {
		t.Fatal("移除取消列後不應重新啟用工作")
	}
	manager.updateTransfer(second, 0, 0, "cancelled")
	if manager.isTransferCancelled(second) {
		t.Fatal("工作完成後未清除取消狀態")
	}
}

func TestTelnetInputPreservesBytesAndNewlineRules(t *testing.T) {
	cases := map[string]string{
		"": "", "plain": "plain", "中文😀": "中文😀", "\x00\xff": "\x00\xff",
		"\r": "\r\n", "\n": "\r\n", "\r\n": "\r\n", "\r\r\n\n": "\r\n\r\n\r\n",
		"a\rb\nc\r\nd": "a\r\nb\r\nc\r\nd",
	}
	for input, expected := range cases {
		if got := string(normalizeTelnetInput(input)); got != expected {
			t.Fatalf("輸入 %q = %q，預期 %q", input, got, expected)
		}
	}
}

func TestTelnetNegotiationDoesNotRetainReadBuffer(t *testing.T) {
	session := &telnetTerminalSession{conn: &telnetTestConn{}}
	readBuffer := []byte{'o', 'k', telnetIAC}
	first := append([]byte(nil), session.negotiate(readBuffer)...)
	clear(readBuffer)
	second := session.negotiate([]byte{telnetIAC, '!'})
	if !bytes.Equal(append(first, second...), []byte{'o', 'k', telnetIAC, '!'}) {
		t.Fatal("跨區塊協商狀態借用了已覆寫的讀取緩衝")
	}
	plain := make([]byte, 4, 16)
	copy(plain, "test")
	output := session.negotiate(plain)
	output = append(output, '!')
	if string(output) != "test!" || plain[:cap(plain)][4] != 0 {
		t.Fatal("追加輸出覆寫了呼叫端緩衝")
	}
	unfinished := append([]byte{telnetIAC, telnetSB}, []byte(strings.Repeat("x", 64*1024))...)
	session.negotiate(unfinished)
	if len(session.negotiationPending) != 0 {
		t.Fatal("未完成的子協商超過原有上限")
	}
}

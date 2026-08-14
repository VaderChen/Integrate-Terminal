package session

import (
	"strings"
	"testing"
	"time"
)

func TestLocalTerminalSessionAcceptsInput(t *testing.T) {
	manager := NewManager()
	sessionID, err := manager.StartLocalSession(nil, "")
	if err != nil {
		t.Fatalf("start local terminal: %v", err)
	}
	defer manager.CloseLocalSession(sessionID)

	if err := manager.WriteLocalInput(sessionID, "printf 'integterm-local-ok\\n'\n"); err != nil {
		t.Fatalf("write local terminal input: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(manager.GetLocalOutputBuffer(sessionID), "integterm-local-ok") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("local terminal did not return expected output: %q", manager.GetLocalOutputBuffer(sessionID))
}

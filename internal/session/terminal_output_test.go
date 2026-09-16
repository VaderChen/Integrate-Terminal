package session

import (
	"bytes"
	"strings"
	"testing"
)

func TestStripTerminalSignals_RemovesOSCAndCapturesCwd(t *testing.T) {
	visible, pending, cwds := stripTerminalSignals(nil, []byte("hello\x1b]9;cwd=/srv/app\a\x1b]10;rgb:eeee/eeee/ecec\a world"))
	if got := string(visible); got != "hello world" {
		t.Fatalf("visible = %q, want %q", got, "hello world")
	}
	if pending != nil {
		t.Fatalf("pending = %q, want nil", string(pending))
	}
	if len(cwds) != 1 || cwds[0] != "/srv/app" {
		t.Fatalf("cwds = %#v, want [/srv/app]", cwds)
	}
}

func TestStripTerminalSignals_HandlesSplitOSCSequence(t *testing.T) {
	visible, pending, cwds := stripTerminalSignals(nil, []byte("a\x1b]10;rgb:eeee"))
	if got := string(visible); got != "a" {
		t.Fatalf("visible = %q, want %q", got, "a")
	}
	if got := string(pending); got != "\x1b]10;rgb:eeee" {
		t.Fatalf("pending = %q, want incomplete OSC", got)
	}
	if len(cwds) != 0 {
		t.Fatalf("cwds = %#v, want empty", cwds)
	}

	visible, pending, cwds = stripTerminalSignals(pending, []byte("/eeee/ecec\a!"))
	if got := string(visible); got != "!" {
		t.Fatalf("visible = %q, want %q", got, "!")
	}
	if pending != nil {
		t.Fatalf("pending = %q, want nil", string(pending))
	}
	if len(cwds) != 0 {
		t.Fatalf("cwds = %#v, want empty", cwds)
	}
}

func TestStripTerminalSignals_HandlesSTTerminator(t *testing.T) {
	visible, pending, cwds := stripTerminalSignals(nil, []byte("x\x1b]10;rgb:eeee/eeee/ecec\x1b\\y"))
	if got := string(visible); got != "xy" {
		t.Fatalf("visible = %q, want %q", got, "xy")
	}
	if pending != nil {
		t.Fatalf("pending = %q, want nil", string(pending))
	}
	if len(cwds) != 0 {
		t.Fatalf("cwds = %#v, want empty", cwds)
	}
}

func TestSSHStreamDiscardsIncompleteOSCAtEOF(t *testing.T) {
	manager := NewManager()
	session := &sshTerminalSession{id: "test-session"}
	manager.streamSSHOutput(nil, session, strings.NewReader("visible\x1b]9;cwd=/should-not-leak"))

	if got := string(session.outputBuffer); got != "visible" {
		t.Fatalf("output buffer = %q, want %q", got, "visible")
	}
}

func TestAppendTerminalOutputTrimsAtLineBoundary(t *testing.T) {
	prefix := bytes.Repeat([]byte("x"), sshOutputBufferLimit+32)
	buffer := append(prefix, '\n')
	buffer = append(buffer, []byte("\x1b[31msafe\x1b[0m")...)

	trimmed := appendTerminalOutput(nil, buffer)
	if got := string(trimmed); got != "\x1b[31msafe\x1b[0m" {
		t.Fatalf("trimmed buffer = %q", got)
	}
}

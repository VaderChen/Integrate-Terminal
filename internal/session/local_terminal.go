package session

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/creack/pty"
	"github.com/google/uuid"
)

type localTerminalSession struct {
	id           string
	cmd          *exec.Cmd
	ptyFile      *os.File
	lock         sync.Mutex
	outputBuffer []byte
	started      bool
	startupBytes []byte
}

func (m *Manager) StartLocalSession(ctx context.Context, cwd string) (string, error) {
	sessionID := fmt.Sprintf("local-%s", uuid.NewString())
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}

	cmd := exec.Command(shell, "-l")
	if cwd != "" {
		cmd.Dir = cwd
	} else if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	cmd.Env = append(os.Environ(),
		"SHELL_SESSIONS_DISABLE=1",
		"TERM=xterm-256color",
	)

	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: 32,
		Cols: 120,
	})
	if err != nil {
		return "", err
	}

	session := &localTerminalSession{
		id:      sessionID,
		cmd:     cmd,
		ptyFile: ptyFile,
	}
	m.mu.Lock()
	m.localSessions[sessionID] = session
	m.mu.Unlock()

	go m.streamLocalOutput(ctx, session)
	go m.watchLocalExit(ctx, session)

	return sessionID, nil
}

func (m *Manager) streamLocalOutput(ctx context.Context, session *localTerminalSession) {
	buffer := make([]byte, 4096)
	var pending []byte
	var pendingControl []byte
	for {
		n, err := session.ptyFile.Read(buffer)
		if n > 0 {
			chunk, rest := splitUTF8SafeChunk(pending, buffer[:n])
			pending = rest
			if len(chunk) > 0 {
				visibleChunk, nextPendingControl, _, clipboardTexts := stripTerminalSignals(pendingControl, chunk)
				pendingControl = nextPendingControl
				for _, clipboardText := range clipboardTexts {
					emitSessionEvent(ctx, fmt.Sprintf("ssh:clipboard:%s", session.id), clipboardText)
				}
				session.lock.Lock()
				if !session.started {
					session.startupBytes = append(session.startupBytes, visibleChunk...)
					visibleChunk, session.started = stripInitialLocalPromptArtifacts(session.startupBytes)
					if session.started {
						session.startupBytes = nil
					} else {
						session.lock.Unlock()
						continue
					}
				}
				session.outputBuffer = appendTerminalOutput(session.outputBuffer, visibleChunk)
				session.lock.Unlock()
				if len(visibleChunk) == 0 {
					continue
				}
				emitSessionEvent(ctx, fmt.Sprintf("ssh:output:%s", session.id), string(visibleChunk))
			}
		}
		if err != nil {
			if len(pendingControl) > 0 {
				pendingControl = nil
			}
			if len(pending) > 0 {
				session.lock.Lock()
				session.outputBuffer = appendTerminalOutput(session.outputBuffer, pending)
				session.lock.Unlock()
				emitSessionEvent(ctx, fmt.Sprintf("ssh:output:%s", session.id), string(pending))
			}
			if err != io.EOF {
				emitSessionEvent(ctx, fmt.Sprintf("ssh:error:%s", session.id), err.Error())
			}
			return
		}
	}
}

func (m *Manager) watchLocalExit(ctx context.Context, session *localTerminalSession) {
	_ = session.cmd.Wait()
	emitSessionEvent(ctx, fmt.Sprintf("ssh:closed:%s", session.id))
	m.removeLocalSession(session.id)
}

func (m *Manager) WriteLocalInput(sessionID string, data string) error {
	m.mu.RLock()
	session, ok := m.localSessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("local session not found")
	}

	session.lock.Lock()
	defer session.lock.Unlock()
	_, err := session.ptyFile.Write([]byte(data))
	return err
}

func (m *Manager) ResizeLocalSession(sessionID string, cols uint16, rows uint16) error {
	m.mu.RLock()
	session, ok := m.localSessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("local session not found")
	}
	return pty.Setsize(session.ptyFile, &pty.Winsize{Rows: rows, Cols: cols})
}

func (m *Manager) CloseLocalSession(sessionID string) error {
	m.mu.RLock()
	session, ok := m.localSessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil
	}

	session.lock.Lock()
	defer session.lock.Unlock()

	if session.cmd.Process != nil {
		_ = session.cmd.Process.Kill()
	}
	_ = session.ptyFile.Close()
	m.removeLocalSession(sessionID)
	return nil
}

func (m *Manager) GetLocalOutputBuffer(sessionID string) string {
	m.mu.RLock()
	session, ok := m.localSessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return ""
	}

	session.lock.Lock()
	defer session.lock.Unlock()
	return string(session.outputBuffer)
}

func resolveLocalTerminalPath(cwd string) string {
	if cwd == "" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return "/"
	}
	if absolute, err := filepath.Abs(cwd); err == nil {
		return absolute
	}
	return cwd
}

func (m *Manager) removeLocalSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.localSessions, sessionID)
}

func stripInitialLocalPromptArtifacts(chunk []byte) ([]byte, bool) {
	if len(chunk) == 0 {
		return nil, false
	}

	// zsh may emit a one-time reverse-video '%' marker before the first prompt.
	if bytes.HasPrefix(chunk, []byte("\x1b[1m\x1b[7m%\x1b[27m\x1b[1m\x1b[0m")) {
		if marker := bytes.Index(chunk, []byte("\x1b[J")); marker >= 0 {
			return chunk[marker+3:], true
		}
		return nil, false
	}

	return chunk, true
}

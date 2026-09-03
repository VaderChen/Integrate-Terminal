package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/sshutil"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

type sshTerminalSession struct {
	id           string
	client       *ssh.Client
	session      *ssh.Session
	stdin        io.WriteCloser
	lock         sync.Mutex
	outputBuffer []byte
}

const sshOutputBufferLimit = 128 * 1024

func (m *Manager) StartSSHSession(ctx context.Context, site model.Site) (string, error) {
	sessionID := fmt.Sprintf("ssh-%s", uuid.NewString())

	authMethods := make([]ssh.AuthMethod, 0, 2)
	if site.Password != "" {
		authMethods = append(authMethods, ssh.Password(site.Password))
	}
	if site.PPKPath != "" {
		signer, err := sshutil.SignerFromPPK(site.PPKPath, site.PPKPassphrase)
		if err != nil {
			return "", fmt.Errorf("load ppk: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if len(authMethods) == 0 {
		return "", fmt.Errorf("missing ssh auth method")
	}

	hostKeyCallback, err := sshutil.KnownHostsCallback()
	if err != nil {
		return "", err
	}

	sshConfig := &ssh.ClientConfig{
		User:            site.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	client, err := sshutil.DialWithRouteRetry("tcp", fmt.Sprintf("%s:%d", site.Host, site.Port), sshConfig)
	if err != nil {
		var trustErr *sshutil.HostTrustRequiredError
		if errors.As(err, &trustErr) {
			return "", trustErr
		}
		return "", err
	}

	sshSession, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return "", err
	}

	stdin, err := sshSession.StdinPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return "", err
	}

	stdout, err := sshSession.StdoutPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return "", err
	}

	stderr, err := sshSession.StderrPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return "", err
	}

	if err := sshSession.RequestPty("xterm-256color", 32, 120, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return "", err
	}

	_ = sshSession.Setenv("PROMPT_COMMAND", `printf '\033]9;cwd=%s\007' "$PWD"${PROMPT_COMMAND:+;$PROMPT_COMMAND}`)

	if err := sshSession.Shell(); err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return "", err
	}

	session := &sshTerminalSession{
		id:      sessionID,
		client:  client,
		session: sshSession,
		stdin:   stdin,
	}
	m.mu.Lock()
	m.sshSessions[sessionID] = session
	m.mu.Unlock()

	go m.streamSSHOutput(ctx, session, stdout)
	go m.streamSSHOutput(ctx, session, stderr)
	go m.watchSSHExit(ctx, session)

	return sessionID, nil
}

func (m *Manager) streamSSHOutput(ctx context.Context, session *sshTerminalSession, reader io.Reader) {
	buffer := make([]byte, 4096)
	var pending []byte
	var pendingControl []byte
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			chunk, rest := splitUTF8SafeChunk(pending, buffer[:n])
			pending = rest
			if len(chunk) > 0 {
				visibleChunk, nextPendingControl, cwdPaths, clipboardTexts := stripTerminalSignals(pendingControl, chunk)
				pendingControl = nextPendingControl
				for _, cwdPath := range cwdPaths {
					emitSessionEvent(ctx, fmt.Sprintf("ssh:cwd:%s", session.id), cwdPath)
				}
				for _, clipboardText := range clipboardTexts {
					emitSessionEvent(ctx, fmt.Sprintf("ssh:clipboard:%s", session.id), clipboardText)
				}
				if len(visibleChunk) == 0 {
					goto afterChunk
				}
				session.lock.Lock()
				session.outputBuffer = appendTerminalOutput(session.outputBuffer, visibleChunk)
				session.lock.Unlock()
				emitSessionEvent(ctx, fmt.Sprintf("ssh:output:%s", session.id), string(visibleChunk))
			}
		}
	afterChunk:
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

func (m *Manager) GetSSHOutputBuffer(sessionID string) string {
	if local := m.GetLocalOutputBuffer(sessionID); local != "" {
		return local
	}
	if telnet := m.GetTelnetOutputBuffer(sessionID); telnet != "" {
		return telnet
	}

	m.mu.RLock()
	session, ok := m.sshSessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return ""
	}

	session.lock.Lock()
	defer session.lock.Unlock()
	return string(session.outputBuffer)
}

func (m *Manager) watchSSHExit(ctx context.Context, session *sshTerminalSession) {
	_ = session.session.Wait()
	emitSessionEvent(ctx, fmt.Sprintf("ssh:closed:%s", session.id))
	m.removeSSHSession(session.id)
}

func (m *Manager) WriteSSHInput(sessionID string, data string) error {
	m.mu.RLock()
	_, hasLocal := m.localSessions[sessionID]
	_, hasTelnet := m.telnetSessions[sessionID]
	session, ok := m.sshSessions[sessionID]
	m.mu.RUnlock()
	if hasLocal {
		return m.WriteLocalInput(sessionID, data)
	}
	if hasTelnet {
		return m.WriteTelnetInput(sessionID, data)
	}
	if !ok {
		return fmt.Errorf("ssh session not found")
	}

	session.lock.Lock()
	defer session.lock.Unlock()
	_, err := session.stdin.Write([]byte(data))
	return err
}

func (m *Manager) ResizeSSHSession(sessionID string, cols uint16, rows uint16) error {
	m.mu.RLock()
	_, hasLocal := m.localSessions[sessionID]
	_, hasTelnet := m.telnetSessions[sessionID]
	session, ok := m.sshSessions[sessionID]
	m.mu.RUnlock()
	if hasLocal {
		return m.ResizeLocalSession(sessionID, cols, rows)
	}
	if hasTelnet {
		return m.ResizeTelnetSession(sessionID, cols, rows)
	}
	if !ok {
		return fmt.Errorf("ssh session not found")
	}
	return session.session.WindowChange(int(rows), int(cols))
}

func (m *Manager) CloseSSHSession(sessionID string) error {
	m.mu.RLock()
	_, hasLocal := m.localSessions[sessionID]
	_, hasTelnet := m.telnetSessions[sessionID]
	session, ok := m.sshSessions[sessionID]
	m.mu.RUnlock()
	if hasLocal {
		return m.CloseLocalSession(sessionID)
	}
	if hasTelnet {
		return m.CloseTelnetSession(sessionID)
	}
	if !ok {
		return nil
	}

	session.lock.Lock()
	defer session.lock.Unlock()

	_ = session.session.Close()
	_ = session.client.Close()
	m.removeSSHSession(sessionID)
	return nil
}

func (m *Manager) removeSSHSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sshSessions, sessionID)
}

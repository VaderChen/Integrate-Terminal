package session

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

"github.com/VaderChen/Integrate-Terminal/internal/model"

	"github.com/google/uuid"
)

type telnetTerminalSession struct {
	id            string
	conn          net.Conn
	lock          sync.Mutex
	outputBuffer  []byte
	username      string
	password      string
	sentUsername  bool
	sentPassword  bool
	passwordTimer *time.Timer
}

const (
	telnetIAC  = 255
	telnetDONT = 254
	telnetDO   = 253
	telnetWONT = 252
	telnetWILL = 251

	telnetOptBinary   = 0
	telnetOptEcho     = 1
	telnetOptSGA      = 3
	telnetOptTermType = 24
	telnetOptNAWS     = 31

	telnetSB = 250
	telnetSE = 240

	telnetTermTypeIS   = 0
	telnetTermTypeSEND = 1
)

func (m *Manager) StartTelnetSession(ctx context.Context, site model.Site) (string, error) {
	sessionID := fmt.Sprintf("telnet-%s", uuid.NewString())

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", site.Host, site.Port), 10*time.Second)
	if err != nil {
		return "", err
	}

	session := &telnetTerminalSession{
		id:       sessionID,
		conn:     conn,
		username: strings.TrimSpace(site.Username),
		password: site.Password,
	}
	m.mu.Lock()
	m.telnetSessions[sessionID] = session
	m.mu.Unlock()

	go m.streamTelnetOutput(ctx, session)
	return sessionID, nil
}

func (m *Manager) streamTelnetOutput(ctx context.Context, session *telnetTerminalSession) {
	buffer := make([]byte, 4096)
	var pending []byte
	var pendingControl []byte
	for {
		n, err := session.conn.Read(buffer)
		if n > 0 {
			payload := session.negotiate(buffer[:n])
			chunk, rest := splitUTF8SafeChunk(pending, payload)
			pending = rest
			if len(chunk) > 0 {
				visibleChunk, nextPendingControl, _, clipboardTexts := stripTerminalSignals(pendingControl, chunk)
				pendingControl = nextPendingControl
				for _, clipboardText := range clipboardTexts {
					emitSessionEvent(ctx, fmt.Sprintf("ssh:clipboard:%s", session.id), clipboardText)
				}
				if len(visibleChunk) == 0 {
					goto afterChunk
				}
				session.lock.Lock()
				session.outputBuffer = appendTerminalOutput(session.outputBuffer, visibleChunk)
				session.maybeAutoLogin()
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
			emitSessionEvent(ctx, fmt.Sprintf("ssh:closed:%s", session.id))
			m.removeTelnetSession(session.id)
			return
		}
	}
}

func (s *telnetTerminalSession) negotiate(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	var plain bytes.Buffer
	for i := 0; i < len(data); i++ {
		if data[i] != telnetIAC {
			plain.WriteByte(data[i])
			continue
		}

		if i+1 >= len(data) {
			break
		}

		cmd := data[i+1]
		if cmd == telnetIAC {
			plain.WriteByte(telnetIAC)
			i++
			continue
		}

		if cmd == telnetSB {
			end := findTelnetSubnegotiationEnd(data, i+2)
			if end == -1 {
				break
			}
			s.handleSubnegotiation(data[i+2 : end])
			i = end + 1
			continue
		}

		if i+2 >= len(data) {
			break
		}

		opt := data[i+2]
		switch cmd {
		case telnetDO:
			if telnetClientOptionAllowed(opt) {
				_, _ = s.conn.Write([]byte{telnetIAC, telnetWILL, opt})
			} else {
				_, _ = s.conn.Write([]byte{telnetIAC, telnetWONT, opt})
			}
		case telnetWILL:
			if telnetServerOptionAllowed(opt) {
				_, _ = s.conn.Write([]byte{telnetIAC, telnetDO, opt})
			} else {
				_, _ = s.conn.Write([]byte{telnetIAC, telnetDONT, opt})
			}
		case telnetDONT:
			_, _ = s.conn.Write([]byte{telnetIAC, telnetWONT, opt})
		case telnetWONT:
			_, _ = s.conn.Write([]byte{telnetIAC, telnetDONT, opt})
		}
		i += 2
	}

	return plain.Bytes()
}

func (m *Manager) GetTelnetOutputBuffer(sessionID string) string {
	m.mu.RLock()
	session, ok := m.telnetSessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return ""
	}

	session.lock.Lock()
	defer session.lock.Unlock()
	return string(session.outputBuffer)
}

func (m *Manager) WriteTelnetInput(sessionID string, data string) error {
	m.mu.RLock()
	session, ok := m.telnetSessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("telnet session not found")
	}

	session.lock.Lock()
	defer session.lock.Unlock()
	normalized := normalizeTelnetInput(data)
	_, err := session.conn.Write(normalized)
	return err
}

func (m *Manager) ResizeTelnetSession(sessionID string, cols uint16, rows uint16) error {
	m.mu.RLock()
	session, ok := m.telnetSessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("telnet session not found")
	}
	session.lock.Lock()
	defer session.lock.Unlock()
	_, err := session.conn.Write([]byte{
		telnetIAC, telnetSB, telnetOptNAWS,
		byte(cols >> 8), byte(cols),
		byte(rows >> 8), byte(rows),
		telnetIAC, telnetSE,
	})
	return err
}

func (m *Manager) CloseTelnetSession(sessionID string) error {
	m.mu.RLock()
	session, ok := m.telnetSessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil
	}

	session.lock.Lock()
	defer session.lock.Unlock()
	session.stopPasswordFallback()
	_ = session.conn.Close()
	m.removeTelnetSession(sessionID)
	return nil
}

func (m *Manager) removeTelnetSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.telnetSessions, sessionID)
}

func (s *telnetTerminalSession) maybeAutoLogin() {
	if len(s.outputBuffer) == 0 {
		return
	}

	// Inspect only the recent tail to avoid matching stale prompts from earlier output.
	start := len(s.outputBuffer) - 256
	if start < 0 {
		start = 0
	}
	tail := strings.ToLower(string(s.outputBuffer[start:]))

	if !s.sentUsername && s.username != "" && containsPrompt(tail, "login:", "username:") {
		normalized := normalizeTelnetLine(s.username)
		_, _ = s.conn.Write(normalized)
		s.sentUsername = true
		s.schedulePasswordFallback()
		return
	}

	if !s.sentPassword && s.password != "" && containsPrompt(tail, "password:") {
		s.stopPasswordFallback()
		normalized := normalizeTelnetLine(s.password)
		_, _ = s.conn.Write(normalized)
		s.sentPassword = true
	}
}

func containsPrompt(value string, prompts ...string) bool {
	trimmed := strings.TrimSpace(value)
	for _, prompt := range prompts {
		if strings.Contains(trimmed, prompt) {
			return true
		}
	}
	return false
}

func (s *telnetTerminalSession) schedulePasswordFallback() {
	if s.password == "" || s.sentPassword {
		return
	}
	s.stopPasswordFallback()
	s.passwordTimer = time.AfterFunc(1200*time.Millisecond, func() {
		s.lock.Lock()
		defer s.lock.Unlock()
		if s.sentPassword || s.password == "" {
			return
		}
		normalized := normalizeTelnetLine(s.password)
		_, _ = s.conn.Write(normalized)
		s.sentPassword = true
	})
}

func (s *telnetTerminalSession) stopPasswordFallback() {
	if s.passwordTimer == nil {
		return
	}
	s.passwordTimer.Stop()
	s.passwordTimer = nil
}

func (s *telnetTerminalSession) handleSubnegotiation(data []byte) {
	if len(data) == 0 {
		return
	}

	switch data[0] {
	case telnetOptTermType:
		if len(data) >= 2 && data[1] == telnetTermTypeSEND {
			_, _ = s.conn.Write([]byte{
				telnetIAC, telnetSB, telnetOptTermType, telnetTermTypeIS,
				'x', 't', 'e', 'r', 'm', '-', '2', '5', '6', 'c', 'o', 'l', 'o', 'r',
				telnetIAC, telnetSE,
			})
		}
	}
}

func findTelnetSubnegotiationEnd(data []byte, start int) int {
	for i := start; i+1 < len(data); i++ {
		if data[i] == telnetIAC && data[i+1] == telnetSE {
			return i
		}
	}
	return -1
}

func normalizeTelnetInput(data string) []byte {
	if data == "" {
		return nil
	}

	var out bytes.Buffer
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				out.WriteString("\r\n")
				i++
			} else {
				out.WriteString("\r\n")
			}
		case '\n':
			out.WriteString("\r\n")
		default:
			out.WriteByte(data[i])
		}
	}

	return out.Bytes()
}

func normalizeTelnetLine(data string) []byte {
	return normalizeTelnetInput(data + "\r")
}

func telnetClientOptionAllowed(opt byte) bool {
	switch opt {
	case telnetOptBinary, telnetOptSGA, telnetOptTermType, telnetOptNAWS:
		return true
	default:
		return false
	}
}

func telnetServerOptionAllowed(opt byte) bool {
	switch opt {
	case telnetOptBinary, telnetOptEcho, telnetOptSGA:
		return true
	default:
		return false
	}
}

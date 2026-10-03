package session

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"IntegTERM/internal/model"

	"github.com/google/uuid"
)

type telnetTerminalSession struct {
	id                 string
	conn               net.Conn
	lock               sync.Mutex
	outputBuffer       []byte
	outputSequence     uint64
	username           string
	password           string
	sentUsername       bool
	sentPassword       bool
	passwordTimer      *time.Timer
	negotiationPending []byte
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

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(site.Host, strconv.Itoa(site.Port)), 10*time.Second)
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
	outputEvent := "ssh:output:" + session.id
	defer func() {
		_ = session.conn.Close()
		session.lock.Lock()
		session.stopPasswordFallback()
		session.lock.Unlock()
	}()
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
				visibleChunk, nextPendingControl, _ := stripTerminalSignals(pendingControl, chunk)
				pendingControl = nextPendingControl
				if len(visibleChunk) == 0 {
					goto afterChunk
				}
				session.lock.Lock()
				session.outputBuffer = appendTerminalOutput(session.outputBuffer, visibleChunk)
				session.outputSequence++
				session.maybeAutoLogin()
				emitSessionEvent(ctx, outputEvent, string(visibleChunk), session.outputSequence)
				session.lock.Unlock()
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
				session.outputSequence++
				emitSessionEvent(ctx, outputEvent, string(pending), session.outputSequence)
				session.lock.Unlock()
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
	if len(s.negotiationPending) > 0 {
		data = append(s.negotiationPending, data...)
	}
	s.negotiationPending = nil
	// 一般輸出可直接交給同步消費者；限制容量，避免 append 改寫讀取緩衝。
	if bytes.IndexByte(data, telnetIAC) < 0 {
		return data[:len(data):len(data)]
	}
	plain := make([]byte, 0, len(data))
	for i := 0; i < len(data); {
		if data[i] != telnetIAC {
			end := bytes.IndexByte(data[i:], telnetIAC)
			if end < 0 {
				plain = append(plain, data[i:]...)
				break
			}
			plain = append(plain, data[i:i+end]...)
			i += end
			continue
		}
		if i+1 >= len(data) {
			s.negotiationPending = append([]byte(nil), data[i:]...)
			break
		}
		cmd := data[i+1]
		if cmd == telnetIAC {
			plain = append(plain, telnetIAC)
			i += 2
			continue
		}
		if cmd == telnetSB {
			end := findTelnetSubnegotiationEnd(data, i+2)
			if end == -1 {
				s.negotiationPending = append([]byte(nil), data[i:]...)
				break
			}
			payload := bytes.ReplaceAll(data[i+2:end], []byte{telnetIAC, telnetIAC}, []byte{telnetIAC})
			s.handleSubnegotiation(payload)
			i = end + 2
			continue
		}
		if cmd != telnetDO && cmd != telnetWILL && cmd != telnetDONT && cmd != telnetWONT {
			i += 2
			continue
		}
		if i+2 >= len(data) {
			s.negotiationPending = append([]byte(nil), data[i:]...)
			break
		}
		opt := data[i+2]
		switch cmd {
		case telnetDO:
			reply := byte(telnetWONT)
			if telnetClientOptionAllowed(opt) {
				reply = telnetWILL
			}
			_, _ = s.conn.Write([]byte{telnetIAC, reply, opt})
		case telnetWILL:
			reply := byte(telnetDONT)
			if telnetServerOptionAllowed(opt) {
				reply = telnetDO
			}
			_, _ = s.conn.Write([]byte{telnetIAC, reply, opt})
		case telnetDONT:
			_, _ = s.conn.Write([]byte{telnetIAC, telnetWONT, opt})
		case telnetWONT:
			_, _ = s.conn.Write([]byte{telnetIAC, telnetDONT, opt})
		}
		i += 3
	}
	// A peer must not grow an unfinished subnegotiation without limit.
	if len(s.negotiationPending) > 64*1024 {
		s.negotiationPending = nil
	}
	return plain
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

	_ = session.conn.Close()
	session.lock.Lock()
	defer session.lock.Unlock()
	session.stopPasswordFallback()
	m.removeTelnetSession(sessionID)
	return nil
}

func (m *Manager) removeTelnetSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.telnetSessions, sessionID)
}

func (s *telnetTerminalSession) maybeAutoLogin() {
	if len(s.outputBuffer) == 0 || ((s.sentUsername || s.username == "") && (s.sentPassword || s.password == "")) {
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
		if data[i] == telnetIAC {
			if data[i+1] == telnetSE {
				return i
			}
			if data[i+1] == telnetIAC {
				i++
			}
		}
	}
	return -1
}

func normalizeTelnetInput(data string) []byte {
	if data == "" {
		return nil
	}

	// 已有的 CRLF 不展開；依實際所需長度一次配置，避免逐字擴充緩衝。
	extra := strings.Count(data, "\r") + strings.Count(data, "\n") - 2*strings.Count(data, "\r\n")
	if extra == 0 {
		return []byte(data)
	}
	out := make([]byte, 0, len(data)+extra)
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				i++
			}
			out = append(out, '\r', '\n')
		case '\n':
			out = append(out, '\r', '\n')
		default:
			out = append(out, data[i])
		}
	}

	return out
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

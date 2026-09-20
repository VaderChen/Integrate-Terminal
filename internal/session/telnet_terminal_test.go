package session

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

type telnetTestConn struct{ bytes.Buffer }

func (*telnetTestConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (*telnetTestConn) Close() error                     { return nil }
func (*telnetTestConn) LocalAddr() net.Addr              { return nil }
func (*telnetTestConn) RemoteAddr() net.Addr             { return nil }
func (*telnetTestConn) SetDeadline(time.Time) error      { return nil }
func (*telnetTestConn) SetReadDeadline(time.Time) error  { return nil }
func (*telnetTestConn) SetWriteDeadline(time.Time) error { return nil }

func TestTelnetNegotiationAtEveryReadBoundary(t *testing.T) {
	input := []byte{'h', telnetIAC, telnetDO, telnetOptTermType, telnetIAC, telnetSB, telnetOptTermType, telnetTermTypeSEND, telnetIAC, telnetSE, telnetIAC, telnetWILL, telnetOptEcho, 'i', telnetIAC, telnetIAC}
	expectedReply := append([]byte{telnetIAC, telnetWILL, telnetOptTermType, telnetIAC, telnetSB, telnetOptTermType, telnetTermTypeIS}, []byte("xterm-256color")...)
	expectedReply = append(expectedReply, telnetIAC, telnetSE, telnetIAC, telnetDO, telnetOptEcho)
	for boundary := 0; boundary <= len(input); boundary++ {
		conn := &telnetTestConn{}
		session := &telnetTerminalSession{conn: conn}
		output := append(session.negotiate(input[:boundary]), session.negotiate(input[boundary:])...)
		if !bytes.Equal(output, []byte{'h', 'i', 255}) || !bytes.Equal(conn.Bytes(), expectedReply) {
			t.Fatalf("split=%d output=%v reply=%v", boundary, output, conn.Bytes())
		}
		if len(session.negotiationPending) != 0 {
			t.Fatalf("pending bytes after complete commands: %v", session.negotiationPending)
		}
	}
	conn := &telnetTestConn{}
	session := &telnetTerminalSession{conn: conn}
	var output []byte
	for _, value := range input {
		output = append(output, session.negotiate([]byte{value})...)
	}
	if !bytes.Equal(output, []byte{'h', 'i', 255}) || !bytes.Equal(conn.Bytes(), expectedReply) {
		t.Fatalf("bytewise output=%v reply=%v", output, conn.Bytes())
	}
}

func TestTelnetTwoByteCommandDoesNotConsumeText(t *testing.T) {
	session := &telnetTerminalSession{conn: &telnetTestConn{}}
	if output := session.negotiate([]byte{telnetIAC, 241, 'x'}); string(output) != "x" {
		t.Fatalf("NOP consumed text: %v", output)
	}
}

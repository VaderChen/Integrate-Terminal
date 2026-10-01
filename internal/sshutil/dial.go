package sshutil

import (
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// Dial 將既有連線逾時套用到 SSH 握手；成功後清除 deadline，避免正常長連線被中斷。
func Dial(network, address string, config *ssh.ClientConfig) (*ssh.Client, error) {
	conn, err := net.DialTimeout(network, address, config.Timeout)
	if err != nil {
		return nil, err
	}
	if config.Timeout > 0 {
		if err = conn.SetDeadline(time.Now().Add(config.Timeout)); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	sshConn, channels, requests, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err = conn.SetDeadline(time.Time{}); err != nil {
		_ = sshConn.Close()
		return nil, err
	}
	return ssh.NewClient(sshConn, channels, requests), nil
}

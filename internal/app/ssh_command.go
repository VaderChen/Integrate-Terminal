package app

import (
	"IntegTERM/internal/model"
	"IntegTERM/internal/sshutil"
	"bytes"
	"context"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxSSHCommandOutput = 4 << 20

var errSSHOutputLimit = errors.New("SSH command output exceeded the 4 MiB limit")

func sshCommandTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return 10 * time.Second
	}
	if seconds > 600 {
		seconds = 600
	}
	return time.Duration(seconds) * time.Second
}

func (a *App) ExecuteSSHCommand(site model.Site, command string, timeoutSeconds int) (map[string]any, error) {
	return a.executeSSHCommand(a.purchaseContext(), site, command, timeoutSeconds)
}

func (a *App) executeSSHCommand(ctx context.Context, site model.Site, command string, timeoutSeconds int) (map[string]any, error) {
	callback, err := sshutil.KnownHostsCallback()
	if err != nil {
		return nil, err
	}
	return runSSHCommand(ctx, site, command, sshCommandTimeout(timeoutSeconds), callback)
}

type commandOutput struct {
	mu       sync.Mutex
	data     bytes.Buffer
	exceeded bool
	cancel   context.CancelFunc
}

func (b *commandOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := maxSSHCommandOutput - b.data.Len()
	if len(p) > remaining {
		_, _ = b.data.Write(p[:remaining])
		b.exceeded = true
		b.cancel()
		return remaining, errSSHOutputLimit
	}
	return b.data.Write(p)
}
func (b *commandOutput) snapshot() (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.String(), b.exceeded
}

func runSSHCommand(parent context.Context, site model.Site, command string, timeout time.Duration, callback ssh.HostKeyCallback) (map[string]any, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if strings.TrimSpace(site.Host) == "" || strings.TrimSpace(site.Username) == "" {
		return nil, fmt.Errorf("host and username are required")
	}
	if site.Port <= 0 || site.Port > 65535 {
		return nil, fmt.Errorf("invalid SSH port")
	}
	auth := []ssh.AuthMethod{}
	if site.Password != "" {
		auth = append(auth, ssh.Password(site.Password))
	}
	if site.PPKPath != "" {
		signer, err := sshutil.SignerFromPPK(site.PPKPath, site.PPKPassphrase)
		if err != nil {
			return nil, fmt.Errorf("load ppk: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("missing ssh auth method")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	addr := net.JoinHostPort(site.Host, strconv.Itoa(site.Port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	config := &ssh.ClientConfig{User: site.Username, Auth: auth, HostKeyCallback: callback}
	sshConn, channels, requests, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	client := ssh.NewClient(sshConn, channels, requests)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	defer session.Close()
	stdout, stderr := &commandOutput{cancel: cancel}, &commandOutput{cancel: cancel}
	session.Stdout = stdout
	session.Stderr = stderr
	runErr := session.Run(command)
	out, outExceeded := stdout.snapshot()
	errOut, errExceeded := stderr.snapshot()
	if outExceeded || errExceeded {
		return nil, errSSHOutputLimit
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return nil, context.DeadlineExceeded
	}
	if netErr, ok := runErr.(net.Error); ok && netErr.Timeout() {
		return nil, context.DeadlineExceeded
	}
	exitCode := 0
	if runErr != nil {
		var exitErr *ssh.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, runErr
		}
	}
	return map[string]any{"stdout": out, "stderr": errOut, "exitCode": exitCode, "ok": exitCode == 0}, nil
}

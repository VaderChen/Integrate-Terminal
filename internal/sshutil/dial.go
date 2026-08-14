package sshutil

import (
	"errors"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

var routeRetryDelays = []time.Duration{
	250 * time.Millisecond,
	750 * time.Millisecond,
	1500 * time.Millisecond,
	3 * time.Second,
}

// DialWithRouteRetry only retries route-transition errors. Authentication,
// host-key, and connection-refused failures are returned immediately.
func DialWithRouteRetry(network string, address string, config *ssh.ClientConfig) (*ssh.Client, error) {
	var lastErr error
	for attempt := 0; attempt <= len(routeRetryDelays); attempt++ {
		client, err := ssh.Dial(network, address, config)
		if err == nil {
			return client, nil
		}
		lastErr = err
		if !isRouteTransitionError(err) || attempt == len(routeRetryDelays) {
			break
		}
		time.Sleep(routeRetryDelays[attempt])
	}
	return nil, lastErr
}

func isRouteTransitionError(err error) bool {
	return errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH)
}

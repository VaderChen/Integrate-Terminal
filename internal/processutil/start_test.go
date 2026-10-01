package processutil

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestStartReapsExitedChild(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := Start(cmd); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(cmd.Process.Signal(syscall.Signal(0)), os.ErrProcessDone) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("exited child was not reaped")
}
func TestStartReportsLaunchFailure(t *testing.T) {
	if err := Start(exec.Command("/no-such-integterm-test-command")); err == nil {
		t.Fatal("missing command accepted")
	}
}

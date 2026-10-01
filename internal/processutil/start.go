package processutil

import "os/exec"

// Start 保留非同步啟動行為，程序結束時呼叫 Wait 回收 OS 資源。
// 呼叫端不應再對同一個 Cmd 呼叫 Wait 或 Process.Release。
func Start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

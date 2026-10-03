package boundedlog

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestAppendBoundsCurrentAndOversizedRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service.log")
	write := func(data string) {
		t.Helper()
		n, err := Append(path, []byte(data), 10)
		if err != nil || n != len(data) {
			t.Fatalf("寫入結果：%d %v", n, err)
		}
	}
	write("first\n")
	write("end\n")
	if data, _ := os.ReadFile(path); string(data) != "first\nend\n" {
		t.Fatalf("未滿載時沒有保留完整內容：%q", data)
	}
	write("new\n")
	if data, _ := os.ReadFile(path); string(data) != "new\n" {
		t.Fatalf("滿載後沒有保留新紀錄：%q", data)
	}
	write("0123456789tail\n")
	if data, _ := os.ReadFile(path); string(data) != "56789tail\n" {
		t.Fatalf("超長紀錄尾端錯誤：%q", data)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	write("latest\n")
	if info, err := os.Stat(path); err != nil || info.Size() != 7 {
		t.Fatalf("既有超大日誌未收斂至上限：%v %v", info, err)
	}
}

func TestConcurrentWritersRespectCapacity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.log")
	var writers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for i := 0; i < 50; i++ {
				if _, err := Append(path, bytes.Repeat([]byte("x"), 64), 128); err != nil {
					t.Error(err)
				}
				if info, err := os.Stat(path); err != nil || info.Size() > 128 {
					t.Errorf("並行寫入超過上限：%v %v", info, err)
				}
			}
		}()
	}
	writers.Wait()
}

func TestBoundedLogWorker(t *testing.T) {
	path := os.Getenv("INTEGTERM_BOUNDED_LOG_TEST_PATH")
	if path == "" {
		t.Skip("子行程測試入口")
	}
	for i := 0; i < 100; i++ {
		if _, err := Append(path, bytes.Repeat([]byte("x"), 64), 128); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(path); err != nil || info.Size() > 128 {
			t.Fatalf("跨行程寫入超過上限：%v %v", info, err)
		}
	}
}

func TestProcessesShareOneLogLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.log")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	commands := make([]*exec.Cmd, 3)
	outputs := make([]bytes.Buffer, len(commands))
	for i := range commands {
		cmd := exec.Command(executable, "-test.run=^TestBoundedLogWorker$", "-test.timeout=15s")
		cmd.Env = append(os.Environ(), "INTEGTERM_BOUNDED_LOG_TEST_PATH="+path)
		cmd.Stdout, cmd.Stderr = &outputs[i], &outputs[i]
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands[i] = cmd
	}
	for i, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Errorf("子行程寫入失敗：%v\n%s", err, &outputs[i])
		}
	}
}

func TestAppendReportsInvalidDestinationAndLimit(t *testing.T) {
	if _, err := Append(t.TempDir(), []byte("entry"), 10); err == nil {
		t.Fatal("目錄不能當成日誌檔案")
	}
	if _, err := Append(filepath.Join(t.TempDir(), "log"), []byte("entry"), 0); err == nil {
		t.Fatal("無效上限不能被接受")
	}
}

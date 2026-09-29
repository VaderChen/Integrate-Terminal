package updater

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadProgressAndVerifiedCache(t *testing.T) {
	payload := strings.Repeat("update", 12000)
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(payload)))
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// 不提供 Content-Length，總大小仍應來自 Release 資產 metadata。
		for i := 0; i < len(payload); i += 12000 {
			_, _ = w.Write([]byte(payload[i : i+12000]))
			w.(http.Flusher).Flush()
			time.Sleep(110 * time.Millisecond)
		}
	}))
	defer server.Close()
	asset := githubAsset{Name: "IntegTERM-test.dmg", Size: int64(len(payload)), Digest: digest, BrowserDownloadURL: server.URL}
	var events []Progress
	report := func(p Progress) { events = append(events, p) }
	directory := t.TempDir()
	path, err := downloadAssetTo(context.Background(), asset, "1.0.0", directory, report)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != payload {
		t.Fatal("download content differs")
	}
	intermediate := false
	var previous int64
	for _, p := range events {
		if p.TotalBytes != asset.Size {
			t.Fatalf("wrong total: %+v", p)
		}
		if p.Stage == "downloading" {
			if p.DownloadedBytes < previous {
				t.Fatal("progress moved backwards")
			}
			if p.DownloadedBytes > 0 && p.DownloadedBytes < asset.Size {
				intermediate = true
			}
			previous = p.DownloadedBytes
		}
	}
	if !intermediate || previous != asset.Size || events[len(events)-1].Stage != "verifying" {
		t.Fatalf("missing streaming progress: %+v", events)
	}
	events = nil
	if _, err = downloadAssetTo(context.Background(), asset, "1.0.0", directory, report); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(events) != 1 || events[0].Stage != "verifying" {
		t.Fatalf("cache should be verified without downloading: calls=%d events=%+v", calls, events)
	}
}

func TestFailedDownloadsLeaveNoFinalOrPartialFile(t *testing.T) {
	for _, mode := range []string{"digest", "size", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			data := "download content"
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(data))
				w.(http.Flusher).Flush()
				if mode == "cancel" {
					cancel()
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			asset := githubAsset{Name: "IntegTERM-test.dmg", Size: int64(len(data)), Digest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(data))), BrowserDownloadURL: server.URL}
			if mode == "digest" {
				asset.Digest = "sha256:" + strings.Repeat("0", 64)
			}
			if mode == "size" {
				asset.Size++
			}
			directory := t.TempDir()
			target, err := downloadAssetTo(ctx, asset, "1.0.0", directory, func(Progress) {})
			if err == nil || target != "" {
				t.Fatalf("invalid download accepted: %q %v", target, err)
			}
			files, _ := filepath.Glob(filepath.Join(directory, "*"))
			if len(files) > 0 {
				t.Fatalf("leftover files: %v", files)
			}
		})
	}
}

package app

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"IntegTERM/internal/model"
)

// 保留原本逐項包含檢查作為對照，驗證最佳化沒有擴大批次刪除範圍。
func referenceDeleteTargets(side string, paths []string) []string {
	cleaned := make([]string, 0, len(paths))
	seen := make(map[string]bool)
	for _, value := range paths {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if side == "remote" {
			value = path.Clean(value)
		} else {
			value = filepath.Clean(value)
		}
		if !seen[value] {
			seen[value] = true
			cleaned = append(cleaned, value)
		}
	}
	sort.Slice(cleaned, func(i, j int) bool { return len(cleaned[i]) < len(cleaned[j]) })
	result := make([]string, 0, len(cleaned))
	for _, value := range cleaned {
		contained := false
		for _, parent := range result {
			if deleteTargetContains(side, parent, value) {
				contained = true
				break
			}
		}
		if !contained {
			result = append(result, value)
		}
	}
	return result
}

func TestCollapseDeleteTargetsMatchesOriginalContainment(t *testing.T) {
	rng := rand.New(rand.NewSource(47))
	cases := [][]string{
		nil, {}, {"", " ", "\t"},
		{"/", "/a", "/ab/file", "/a/b", "a", ".", "..", "../file"},
		{"foo/bar", "foo", "foobar/child", "./foo", "foo/../other", "foo/bar/"},
		{" /folder ", " /folder /child ", "/中文", "/中文/檔案", "/中/文"},
		{`C:\`, `C:\Users`, `C:\Users\file`, `C:\User`, `\\server\share`, `\\server\share\file`},
	}
	for i := 0; i < 100; i++ {
		values := make([]string, 80)
		for j := range values {
			prefix := []string{"/", "", "../", "./", "//"}[rng.Intn(5)]
			values[j] = fmt.Sprintf("%sdir-%d/child-%d", prefix, rng.Intn(12), rng.Intn(10))
			if j%3 == 0 {
				values[j] = path.Dir(values[j])
			}
		}
		cases = append(cases, values)
	}
	for _, side := range []string{"local", "remote"} {
		for _, values := range cases {
			original := append([]string(nil), values...)
			got, want := collapseNestedDeleteTargets(side, values), referenceDeleteTargets(side, values)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s 路徑 %q：結果 %q，原行為 %q", side, values, got, want)
			}
			if !reflect.DeepEqual(append([]string(nil), values...), original) {
				t.Fatal("修改了呼叫端傳入的路徑")
			}
		}
	}
}

func TestSiteSortPreservesStableKeysAndFallback(t *testing.T) {
	sites := []model.Site{
		{ID: "1", Name: " alpha "}, {ID: "2", Name: "ALPHA"},
		{ID: "3", Name: "", Host: " Alpha "}, {ID: "4", Name: " 中文 "},
		{ID: "5", Name: "beta"}, {ID: "6", Name: ""}, {ID: "7", Name: "Ä"},
	}
	want := append([]model.Site(nil), sites...)
	key := func(site model.Site) string {
		name := strings.ToLower(strings.TrimSpace(site.Name))
		if name == "" {
			name = strings.ToLower(strings.TrimSpace(site.Host))
		}
		return name
	}
	sort.SliceStable(want, func(i, j int) bool { return key(want[i]) < key(want[j]) })
	a := &App{sites: sites}
	if _, err := a.sortSitesByNameLocked(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.sites, want) {
		t.Fatalf("排序不符合原順序：%v", a.sites)
	}
}

func TestVFSFinalizeExcludesConcurrentRestartAndPreservesRetry(t *testing.T) {
	layer := newMCPVirtualLayer(&App{})
	vfs := layer.vfs
	const payload = "分塊內容"
	if _, err := layer.writeVirtualChunk(mcpVFSWriteChunkInput{Path: "file", Content: payload}); err != nil {
		t.Fatal(err)
	}
	input := mcpVFSWriteChunkInput{Path: "file", Offset: int64(len(payload)), Final: true, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))}
	// 暫時鎖住目的地，讓完成階段停在同步寫入，驗證借用資料不會被重啟改寫。
	vfs.mu.Lock()
	locked := true
	defer func() {
		if locked {
			vfs.mu.Unlock()
		}
	}()
	complete := make(chan error, 1)
	go func() { _, err := layer.writeVirtualChunk(input); complete <- err }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		vfs.chunkMu.Lock()
		finalizing := vfs.chunkWrites["file"].finalizing
		vfs.chunkMu.Unlock()
		if finalizing {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("未進入完成階段")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := layer.writeVirtualChunk(mcpVFSWriteChunkInput{Path: "file", Content: "替換"}); err == nil {
		t.Fatal("允許重啟仍在完成的寫入")
	}
	vfs.mu.Unlock()
	locked = false
	if err := <-complete; err != nil {
		t.Fatal(err)
	}
	data, _, _, err := vfs.read("file", 0, 64)
	if err != nil || string(data) != payload {
		t.Fatalf("完成的資料改變：%q %v", data, err)
	}
	if _, err := layer.writeVirtualChunk(mcpVFSWriteChunkInput{Path: "file", Content: payload}); err != nil {
		t.Fatal(err)
	}
	if _, err := layer.writeVirtualChunk(input); err == nil {
		t.Fatal("既有檔案不可在未指定覆寫時被替換")
	}
	if vfs.chunkWrites["file"].finalizing || vfs.chunkBytes != int64(len(payload)) {
		t.Fatal("失敗後沒有保留可重試的分塊資料")
	}
	input.Overwrite = true
	if _, err := layer.writeVirtualChunk(input); err != nil {
		t.Fatal(err)
	}
	if vfs.chunkBytes != 0 || len(vfs.chunkWrites) != 0 {
		t.Fatal("完成後仍保留暫存資料")
	}
}

func TestHiddenTabRemovalReleasesTailAndPreservesVisibleCount(t *testing.T) {
	tabs := []model.Tab{{ID: "one"}, {ID: "two", Hidden: true}, {ID: "three", Hidden: true, Password: "temporary"}}
	a := &App{tabs: tabs}
	a.closeMCPRemoteTab("two")
	if countVisibleTabs(a.tabs) != 1 || len(a.tabs) != 2 || a.tabs[1].ID != "three" {
		t.Fatalf("移除隱藏分頁改變其他分頁：%v", a.tabs)
	}
	if !reflect.DeepEqual(tabs[len(tabs)-1], model.Tab{}) {
		t.Fatal("切片尾端仍保留隱藏分頁資料")
	}
}

package app

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"IntegTERM/internal/model"
)

func BenchmarkCollapseDeleteTargets(b *testing.B) {
	paths := make([]string, 1000)
	for i := range paths {
		paths[i] = fmt.Sprintf("/workspace/folder-%04d/file.txt", i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(collapseNestedDeleteTargets("remote", paths)) != len(paths) {
			b.Fatal("獨立路徑遭到移除")
		}
	}
}

func BenchmarkVFSList(b *testing.B) {
	vfs := newMCPVFS()
	for i := 0; i < 2000; i++ {
		vfs.nodes[fmt.Sprintf("file-%04d", i)] = mcpVFSNode{modified: time.Unix(0, 0)}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items, err := vfs.list("")
		if err != nil || len(items) != 2000 {
			b.Fatalf("列目錄失敗：%d %v", len(items), err)
		}
	}
}

func BenchmarkVFSFinalize(b *testing.B) {
	data := make([]byte, 8*1024*1024)
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	layer := &mcpVirtualLayer{vfs: newMCPVFS()}
	input := mcpVFSWriteChunkInput{Path: "bench.bin", Offset: int64(len(data)), Final: true, SHA256: digest, Overwrite: true}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		layer.vfs.chunkWrites[input.Path] = &mcpVFSChunkWrite{data: data}
		layer.vfs.chunkBytes = int64(len(data))
		if output, err := layer.writeVirtualChunk(input); err != nil || !output.Complete {
			b.Fatalf("完成分塊寫入失敗：%v", err)
		}
	}
}

func BenchmarkSiteSort(b *testing.B) {
	sites := make([]model.Site, 1000)
	for i := range sites {
		sites[i] = model.Site{ID: fmt.Sprint(i), Name: fmt.Sprintf(" Host-%04d ", (i*997)%1000)}
	}
	a := &App{sites: make([]model.Site, len(sites))}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(a.sites, sites)
		if _, err := a.sortSitesByNameLocked(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTerminalFontFilter(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if isTerminalFontFamily("Helvetica Neue") {
			b.Fatal("不應納入比例字型")
		}
	}
}

func BenchmarkVFSLocation(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := parseMCPVFSLocation("sites/server/config/project/assets/file.txt"); err != nil {
			b.Fatal(err)
		}
	}
}

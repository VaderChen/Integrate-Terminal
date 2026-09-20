package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"IntegTERM/internal/credentials"
	"IntegTERM/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPVFSRejectsEncodedControlTraversalAndOversizePaths(t *testing.T) {
	for _, p := range []string{
		mcpVFSRootURI + "/sites/id/%00file", mcpVFSRootURI + "/sites/id/%0d%0aDELE%20secret",
		mcpVFSRootURI + "/sites/id/%2e%2e/secret", mcpVFSRootURI + "/sites/id/%5csecret",
		"integterm-vfs://user@workspace/mcp/a", mcpVFSRootURI + "?",
		strings.Repeat("a", mcpVFSMaxPathBytes+1), strings.Repeat("a/", mcpVFSMaxDepth) + "b",
	} {
		if _, err := normalizeMCPVFSPath(p); err == nil {
			t.Errorf("accepted invalid path %q", p)
		}
	}
	for _, name := range []string{"notes/ leading and trailing ", "notes/100% real.txt", "notes/中文.txt"} {
		got, err := normalizeMCPVFSPath(mcpVFSURI(name))
		if err != nil || got != name {
			t.Fatalf("URI roundtrip = %q, %v; want %q", got, err, name)
		}
	}
}

func TestMCPVFSCannotCreateDescendantsOfFiles(t *testing.T) {
	for _, operation := range []string{"write", "mkdir", "rename"} {
		t.Run(operation, func(t *testing.T) {
			vfs := newMCPVFS()
			if _, err := vfs.write("parent", "original", "", false); err != nil {
				t.Fatal(err)
			}
			if _, err := vfs.write("source", "move me", "", false); err != nil {
				t.Fatal(err)
			}
			before := vfs.workspaceInfo()
			var err error
			switch operation {
			case "write":
				_, err = vfs.write("parent/new/deep", "new", "", false)
			case "mkdir":
				_, err = vfs.mkdir("parent/new/deep")
			case "rename":
				_, err = vfs.rename("source", "parent/new/deep")
			}
			if err == nil {
				t.Fatal("accepted a file as ancestor")
			}
			after := vfs.workspaceInfo()
			if after.BytesUsed != before.BytesUsed || after.FileCount != before.FileCount || after.DirectoryCount != before.DirectoryCount {
				t.Fatalf("failed operation changed tree: before=%+v after=%+v", before, after)
			}
			data, _, _, err := vfs.read("parent", 0, 64)
			if err != nil || string(data) != "original" {
				t.Fatalf("original changed: %q, %v", data, err)
			}
		})
	}
}

func TestMCPVFSNodeQuotaAndWorkspaceByteQuota(t *testing.T) {
	vfs := newMCPVFS()
	for i := 0; i < mcpVFSMaxNodes-1; i++ {
		if _, err := vfs.write(fmt.Sprintf("empty-%d", i), "", "", false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := vfs.write("overflow/child", "payload", "", false); err == nil {
		t.Fatal("empty-file node quota bypass")
	}
	if _, err := vfs.stat("overflow"); err == nil {
		t.Fatal("failed quota check created ancestor")
	}
	if _, err := vfs.delete("empty-0", false); err != nil {
		t.Fatal(err)
	}
	if _, err := vfs.write("replacement", "", "", false); err != nil {
		t.Fatalf("deleting did not free node quota: %v", err)
	}
	vfs = newMCPVFS()
	payload := bytes.Repeat([]byte("x"), mcpVFSTotalSize)
	if _, err := vfs.writeBytes("full", payload, false, mcpVFSMaxChunkedFile); err != nil {
		t.Fatal(err)
	}
	if _, err := vfs.write("overflow", "x", "", false); err == nil {
		t.Fatal("workspace byte quota bypass")
	}
	if _, err := vfs.write("full", "small", "", true); err != nil {
		t.Fatal(err)
	}
	if used := vfs.workspaceInfo().BytesUsed; used != 5 {
		t.Fatalf("replacement accounting = %d", used)
	}
}

func TestMCPVFSDecodeChecksLimitBeforeAllocation(t *testing.T) {
	for _, test := range []struct{ content, encoding string }{
		{strings.Repeat("a", mcpVFSMaxWriteChunk+1), "utf-8"},
		{base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("a"), mcpVFSMaxWriteChunk+1)), "base64"},
	} {
		if _, err := decodeMCPVFSContent(test.content, test.encoding, mcpVFSMaxWriteChunk); err == nil {
			t.Fatalf("accepted oversize %s content", test.encoding)
		}
	}
}

func TestMCPVFSChunkIntegrityQuotaAndRetry(t *testing.T) {
	layer := newMCPVirtualLayer(&App{})
	if _, err := layer.writeVirtualChunk(mcpVFSWriteChunkInput{Path: "file", Content: "old", Final: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := layer.vfs.stat("file"); err == nil {
		t.Fatal("unfinished chunks became visible")
	}
	badHash := strings.Repeat("0", 64)
	if _, err := layer.writeVirtualChunk(mcpVFSWriteChunkInput{Path: "file", Offset: 3, Content: "x", Final: true, SHA256: badHash}); err == nil {
		t.Fatal("incorrect digest committed")
	}
	if len(layer.vfs.chunkWrites) != 0 || layer.vfs.chunkBytes != 0 {
		t.Fatal("bad digest leaked staging")
	}
	payload := "good"
	digest := sha256.Sum256([]byte(payload))
	result, err := layer.writeVirtualChunk(mcpVFSWriteChunkInput{Path: "file", Content: payload, Final: true, SHA256: fmt.Sprintf("%x", digest)})
	if err != nil || !result.Complete {
		t.Fatalf("retry = %+v, %v", result, err)
	}
	for i := 0; i < mcpVFSMaxPendingWrites; i++ {
		if _, err := layer.writeVirtualChunk(mcpVFSWriteChunkInput{Path: fmt.Sprintf("pending-%d", i), Content: "a"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := layer.writeVirtualChunk(mcpVFSWriteChunkInput{Path: "extra", Content: "a"}); err == nil {
		t.Fatal("pending write count is unbounded")
	}
	if _, err := layer.writeVirtualChunk(mcpVFSWriteChunkInput{Path: "pending-0", Content: ""}); err == nil {
		t.Fatal("empty nonfinal chunk accepted")
	}
	if layer.vfs.chunkBytes != mcpVFSMaxPendingWrites {
		t.Fatalf("rejected chunk changed accounting: %d", layer.vfs.chunkBytes)
	}
}

func TestMCPVFSConcurrentCreateHasOneWinner(t *testing.T) {
	vfs := newMCPVFS()
	var wg sync.WaitGroup
	successes := make(chan string, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			value := fmt.Sprint(i)
			if _, err := vfs.write("winner", value, "", false); err == nil {
				successes <- value
			}
			_, _ = vfs.list("")
			_ = vfs.workspaceInfo()
		}(i)
	}
	wg.Wait()
	close(successes)
	var winners []string
	for value := range successes {
		winners = append(winners, value)
	}
	if len(winners) != 1 {
		t.Fatalf("create winners = %v", winners)
	}
	data, _, _, err := vfs.read("winner", 0, 32)
	if err != nil || string(data) != winners[0] {
		t.Fatalf("winner corrupted: %q, %v", data, err)
	}
}

func TestMCPMountPreservesConfiguredRootAndRejectsEscapes(t *testing.T) {
	for _, test := range []struct{ configured, cwd, want string }{
		{"/srv/app", "/home/user", "/srv/app"}, {"project", "/home/user", "/home/user/project"}, {".", "/home/user", "/home/user"}, {"", "/home/user", "/home/user"},
	} {
		root, err := mcpMountRoot(test.configured, test.cwd)
		if err != nil || root != test.want {
			t.Errorf("mount root = %q, %v; want %q", root, err, test.want)
		}
	}
	for _, test := range []struct{ root, target string }{
		{".", "/secret"}, {"/", "../secret"}, {"/srv/app", "/srv/app/../secret"}, {"/srv/app", "/srv/app-sibling/secret"}, {"/srv/app", "/srv/app/a\nDELE file"},
	} {
		if _, err := virtualRemotePath(mcpVFSRemoteMount{SiteID: "s", RootPath: test.root}, test.target); err == nil {
			t.Errorf("accepted outside target %+v", test)
		}
	}
	if _, err := mcpMountRoot("relative", ""); err == nil {
		t.Fatal("guessed root without working directory")
	}
	if _, err := mcpMountRoot("/srv/link/../secret", "/"); err == nil {
		t.Fatal("cleaned traversal before validation")
	}
	site := model.Site{Host: "host", RemotePath: "/srv/app", Password: "first"}
	fingerprint := mcpSiteFingerprint(site)
	site.Name = "renamed"
	site.LastUsedAt = "later"
	if mcpSiteFingerprint(site) != fingerprint {
		t.Fatal("display metadata invalidated connection")
	}
	site.Password = "changed"
	if mcpSiteFingerprint(site) == fingerprint {
		t.Fatal("credential edit reused old mount")
	}
}

func TestMCPStorageFailureIsVisibleAndRAMRemainsUsable(t *testing.T) {
	app := &App{storageInitErr: credentials.ErrLocked}
	layer := newMCPVirtualLayer(app)
	if _, err := layer.listRemoteSites(); !errors.Is(err, credentials.ErrLocked) {
		t.Fatalf("sites hid storage failure: %v", err)
	}
	if _, err := layer.mcpRemoteSiteCount(); !errors.Is(err, credentials.ErrLocked) {
		t.Fatalf("count hid storage failure: %v", err)
	}
	if _, err := layer.writeVirtual("recovery-note", "hello", "", false); err != nil {
		t.Fatalf("RAM unavailable: %v", err)
	}
	ctx := context.Background()
	server := layer.newServer(mcpContractLocal)
	ct, st := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "storage-error", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "vfs_workspace_info", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("workspace_info advertised empty sites after credential failure")
	}
	app.stateMu.Lock()
	app.storageInitErr = nil
	app.sites = []model.Site{{ID: "site-1", Name: "Recovered"}}
	app.stateMu.Unlock()
	result, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "vfs_list", Arguments: map[string]any{"path": "sites"}})
	if err != nil || result.IsError {
		t.Fatalf("discovery did not recover: %+v, %v", result, err)
	}
}

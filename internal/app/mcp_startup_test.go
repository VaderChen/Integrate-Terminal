package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"IntegTERM/internal/model"
	"IntegTERM/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPFileStorageAndUnreadableFiles(t *testing.T) {
	for _, broken := range []bool{false, true} {
		name := "file"
		if broken {
			name = "corrupt-file"
		}
		t.Run(name, func(t *testing.T) {
			saved := newAppTestStore(t.TempDir())
			if err := saved.SaveSites([]model.Site{regressionSite("saved-site")}); err != nil {
				t.Fatal(err)
			}
			if broken {
				if err := os.WriteFile(filepath.Join(saved.BaseDir(), "sites.json"), []byte("{broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(filepath.Join(saved.BaseDir(), "sites.json"))
			if err != nil {
				t.Fatal(err)
			}
			a := newMCPWithStore(store.NewWithLockTimeout(saved.BaseDir(), 100*time.Millisecond))
			a.MCPStartup()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			ct, st := mcp.NewInMemoryTransports()
			server, err := newMCPVirtualLayer(a).newServer(mcpContractLocal).Connect(ctx, st, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			client := mcp.NewClient(&mcp.Implementation{Name: "startup", Version: "1"}, nil)
			connection, err := client.Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			listed, err := connection.ListTools(ctx, nil)
			if err != nil || len(listed.Tools) != 10 || a.storageInitErr == nil {
				t.Fatalf("MCP 握手不應載入站台檔案：%v", err)
			}
			info, err := connection.CallTool(ctx, &mcp.CallToolParams{Name: "vfs_workspace_info", Arguments: map[string]any{}})
			if err != nil || info.IsError != broken {
				t.Fatalf("站台檔案存取不符合預期：broken=%v err=%v result=%+v", broken, err, info)
			}
			if !broken {
				sites, err := connection.CallTool(ctx, &mcp.CallToolParams{Name: "vfs_list", Arguments: map[string]any{"path": "sites"}})
				if err != nil || sites.IsError {
					t.Fatalf("檔案站台探索失敗：%v", err)
				}
				encoded, err := json.Marshal(sites)
				if err != nil || !bytes.Contains(encoded, []byte("saved-site")) {
					t.Fatal("MCP 未回傳檔案內的站台")
				}
			}
			for _, call := range []*mcp.CallToolParams{
				{Name: "vfs_list", Arguments: map[string]any{}},
				{Name: "vfs_write", Arguments: map[string]any{"path": "note", "content": "still available"}},
				{Name: "vfs_read", Arguments: map[string]any{"path": "note"}},
			} {
				result, err := connection.CallTool(ctx, call)
				if err != nil || result.IsError {
					t.Fatalf("%s unavailable after file read error: %v", call.Name, err)
				}
			}
			a.ServiceShutdown()
			after, err := os.ReadFile(filepath.Join(saved.BaseDir(), "sites.json"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("MCP 查詢改動了站台檔案")
			}
			if _, err := os.Stat(filepath.Join(saved.BaseDir(), restTokenFilename)); !os.IsNotExist(err) {
				t.Fatal("stdio startup created an HTTP token")
			}
		})
	}
}

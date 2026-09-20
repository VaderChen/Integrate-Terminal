package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"IntegTERM/internal/credentials"
	"IntegTERM/internal/model"
	"IntegTERM/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpDeniedVault struct{ reads atomic.Int32 }

func (v *mcpDeniedVault) Get(string) ([]byte, error) {
	v.reads.Add(1)
	return nil, credentials.ErrLocked
}
func (*mcpDeniedVault) Put(string, []byte) error { return credentials.ErrLocked }
func (*mcpDeniedVault) Delete(string) error      { return credentials.ErrLocked }

func TestMCPStartsWithoutReadingCredentialsAndKeepsRAMAvailable(t *testing.T) {
	saved := newAppTestStore(t.TempDir())
	if err := saved.SaveSites([]model.Site{regressionSite("saved-site")}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(saved.BaseDir(), "sites.json"))
	if err != nil {
		t.Fatal(err)
	}
	vault := &mcpDeniedVault{}
	a := newMCPWithStore(store.NewWithCredentialsAndLockTimeout(saved.BaseDir(), vault, 100*time.Millisecond))
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
	if err != nil || len(listed.Tools) != 10 || vault.reads.Load() != 0 {
		t.Fatalf("MCP handshake depended on credentials: err=%v reads=%d", err, vault.reads.Load())
	}
	failed, err := connection.CallTool(ctx, &mcp.CallToolParams{Name: "vfs_workspace_info", Arguments: map[string]any{}})
	if err != nil || !failed.IsError || vault.reads.Load() == 0 {
		t.Fatalf("credential failure was hidden: err=%v result=%+v", err, failed)
	}
	for _, call := range []*mcp.CallToolParams{
		{Name: "vfs_list", Arguments: map[string]any{}},
		{Name: "vfs_write", Arguments: map[string]any{"path": "note", "content": "still available"}},
		{Name: "vfs_read", Arguments: map[string]any{"path": "note"}},
	} {
		result, err := connection.CallTool(ctx, call)
		if err != nil || result.IsError {
			t.Fatalf("%s unavailable after credential error: %v", call.Name, err)
		}
	}
	a.ServiceShutdown()
	after, err := os.ReadFile(filepath.Join(saved.BaseDir(), "sites.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed credential access changed saved sites")
	}
	if _, err := os.Stat(filepath.Join(saved.BaseDir(), restTokenFilename)); !os.IsNotExist(err) {
		t.Fatal("stdio startup created an HTTP token")
	}
}

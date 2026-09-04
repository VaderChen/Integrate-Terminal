package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	appversion "github.com/VaderChen/Integrate-Terminal/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNormalizeMCPVFSPathUsesMCPWorkspaceRoot(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty path", input: "", want: ""},
		{name: "root path", input: "/", want: ""},
		{name: "root URI", input: mcpVFSRootURI, want: ""},
		{name: "root URI with slash", input: mcpVFSRootURI + "/", want: ""},
		{name: "child URI", input: mcpVFSRootURI + "/notes/readme.txt", want: "notes/readme.txt"},
		{name: "relative child", input: "notes/readme.txt", want: "notes/readme.txt"},
		{name: "legacy root URI", input: "integterm-vfs://workspace/", wantErr: true},
		{name: "foreign URI path", input: "integterm-vfs://workspace/other/file.txt", wantErr: true},
		{name: "parent traversal", input: mcpVFSRootURI + "/../secret", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeMCPVFSPath(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeMCPVFSPath(%q) returned error: %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("normalizeMCPVFSPath(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestMCPVFSWriteChunkExceedsInlineLimit(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), mcpVFSMaxFileSize+1)
	digest := sha256.Sum256(payload)
	layer := newMCPVirtualLayer(&App{mcpVFS: newMCPVFS()})

	var result mcpVFSWriteChunkOutput
	for offset := 0; offset < len(payload); {
		end := offset + mcpVFSMaxWriteChunk
		if end > len(payload) {
			end = len(payload)
		}
		final := end == len(payload)
		expectedSHA256 := ""
		if final {
			expectedSHA256 = fmt.Sprintf("%x", digest)
		}
		var err error
		result, err = layer.writeVirtualChunk(mcpVFSWriteChunkInput{
			Path:     "artifacts/over-inline-limit.bin",
			Offset:   int64(offset),
			Content:  base64.StdEncoding.EncodeToString(payload[offset:end]),
			Encoding: "base64",
			Final:    final,
			SHA256:   expectedSHA256,
		})
		if err != nil {
			t.Fatalf("write chunk at offset %d: %v", offset, err)
		}
		offset = end
	}

	if !result.Complete || result.Item == nil {
		t.Fatalf("chunked write did not complete: %#v", result)
	}
	if got, want := result.Item.Size, int64(len(payload)); got != want {
		t.Fatalf("chunked file size = %d, want %d", got, want)
	}
	lastByte, _, _, err := layer.vfs.read("artifacts/over-inline-limit.bin", int64(len(payload)-1), 1)
	if err != nil {
		t.Fatalf("read final byte from chunked file: %v", err)
	}
	if !bytes.Equal(lastByte, []byte("a")) {
		t.Fatalf("chunked file final byte = %q, want a", lastByte)
	}
}

func TestMCPVFSURIUsesMCPWorkspaceRoot(t *testing.T) {
	if got := mcpVFSURI(""); got != mcpVFSRootURI {
		t.Fatalf("mcpVFSURI(\"\") = %q, want %q", got, mcpVFSRootURI)
	}
	want := mcpVFSRootURI + "/notes/readme.txt"
	if got := mcpVFSURI("notes/readme.txt"); got != want {
		t.Fatalf("mcpVFSURI(child) = %q, want %q", got, want)
	}
}

func TestParseMCPVFSLocation(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		kind       mcpVFSLocationKind
		path       string
		siteID     string
		remotePath string
		wantErr    bool
	}{
		{name: "RAM root", input: mcpVFSRootURI, kind: mcpVFSLocationRAM, path: ""},
		{name: "RAM file", input: "notes/readme.txt", kind: mcpVFSLocationRAM, path: "notes/readme.txt"},
		{name: "sites namespace", input: "sites", kind: mcpVFSLocationSites, path: "sites"},
		{name: "site root", input: mcpVFSRootURI + "/sites/site-1", kind: mcpVFSLocationSiteRoot, path: "sites/site-1", siteID: "site-1"},
		{name: "remote file", input: mcpVFSRootURI + "/sites/site-1/config/app.yml", kind: mcpVFSLocationRemote, path: "sites/site-1/config/app.yml", siteID: "site-1", remotePath: "config/app.yml"},
		{name: "parent traversal", input: mcpVFSRootURI + "/sites/site-1/../secret", wantErr: true},
		{name: "missing site id", input: "sites/", wantErr: false, kind: mcpVFSLocationSites, path: "sites"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseMCPVFSLocation(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMCPVFSLocation(%q) returned error: %v", test.input, err)
			}
			if got.kind != test.kind || got.path != test.path || got.siteID != test.siteID || got.remotePath != test.remotePath {
				t.Fatalf("parseMCPVFSLocation(%q) = %#v, want kind=%q path=%q siteID=%q remotePath=%q", test.input, got, test.kind, test.path, test.siteID, test.remotePath)
			}
		})
	}
}

func TestMCPRemotePathMapping(t *testing.T) {
	mount := mcpVFSRemoteMount{SiteID: "site-1", RootPath: "/srv/app"}
	if got := remotePathForMCPMount(mount, "config/app.yml"); got != "/srv/app/config/app.yml" {
		t.Fatalf("remotePathForMCPMount() = %q, want %q", got, "/srv/app/config/app.yml")
	}
	if got := remotePathForMCPMount(mount, ""); got != "/srv/app" {
		t.Fatalf("remotePathForMCPMount(root) = %q, want %q", got, "/srv/app")
	}

	virtualPath, err := virtualRemotePath(mount, "/srv/app/config/app.yml")
	if err != nil {
		t.Fatalf("virtualRemotePath() returned error: %v", err)
	}
	if virtualPath != "sites/site-1/config/app.yml" {
		t.Fatalf("virtualRemotePath() = %q, want %q", virtualPath, "sites/site-1/config/app.yml")
	}
	if _, err := virtualRemotePath(mount, "/srv/secret.txt"); err == nil {
		t.Fatal("expected a remote path outside the site root to be rejected")
	}
}

func TestMCPVFSContractIsSelfDescribing(t *testing.T) {
	ctx := context.Background()
	application := &App{mcpVFS: newMCPVFS()}
	server := newMCPVirtualLayer(application).newServer(mcpContractLocal)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect MCP server: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "contract-test", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	defer clientSession.Close()

	instructions := clientSession.InitializeResult().Instructions
	for _, required := range []string{mcpVFSRootURI, "vfs_workspace_info", "vfs_list", "sites/{siteID}", "not an HTTP endpoint"} {
		if !strings.Contains(instructions, required) {
			t.Errorf("initialize instructions do not contain %q", required)
		}
	}
	if got, want := clientSession.InitializeResult().ServerInfo.Version, appversion.ProductVersion(); got != want {
		t.Errorf("MCP server version = %q, want application version %q", got, want)
	}

	toolsResult, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list MCP tools: %v", err)
	}
	var listTool *mcp.Tool
	var writeChunkTool *mcp.Tool
	for _, tool := range toolsResult.Tools {
		switch tool.Name {
		case "vfs_list":
			listTool = tool
		case "vfs_write_chunk":
			writeChunkTool = tool
		}
	}
	if listTool == nil {
		t.Fatal("vfs_list tool was not advertised")
	}
	outputSchema, err := json.Marshal(listTool.OutputSchema)
	if err != nil {
		t.Fatalf("marshal vfs_list output schema: %v", err)
	}
	for _, property := range []string{`"entries"`, `"kind"`, `"nextAction"`} {
		if !strings.Contains(string(outputSchema), property) {
			t.Errorf("vfs_list output schema does not describe %s: %s", property, outputSchema)
		}
	}
	if writeChunkTool == nil {
		t.Fatal("vfs_write_chunk tool was not advertised")
	}
	chunkInputSchema, err := json.Marshal(writeChunkTool.InputSchema)
	if err != nil {
		t.Fatalf("marshal vfs_write_chunk input schema: %v", err)
	}
	for _, property := range []string{`"path"`, `"offset"`, `"content"`, `"encoding"`, `"final"`, `"sha256"`} {
		if !strings.Contains(string(chunkInputSchema), property) {
			t.Errorf("vfs_write_chunk input schema does not describe %s: %s", property, chunkInputSchema)
		}
	}

	listResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "vfs_list", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("call vfs_list with empty arguments: %v", err)
	}
	structured, err := json.Marshal(listResult.StructuredContent)
	if err != nil {
		t.Fatalf("marshal vfs_list result: %v", err)
	}
	for _, property := range []string{`"entries"`, `"kind":"ram"`, `"nextAction"`} {
		if !strings.Contains(string(structured), property) {
			t.Errorf("vfs_list result does not contain %s: %s", property, structured)
		}
	}

	resourceResult, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: mcpVFSRootURI})
	if err != nil {
		t.Fatalf("read VFS root resource: %v", err)
	}
	if len(resourceResult.Contents) != 1 {
		t.Fatalf("root resource returned %d contents, want 1", len(resourceResult.Contents))
	}
	for _, required := range []string{`"workflow"`, `"pathModel"`, `"toolGuide"`, `"examples"`, `"limits"`} {
		if !strings.Contains(resourceResult.Contents[0].Text, required) {
			t.Errorf("root resource does not contain %s", required)
		}
	}

	_, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "vfs_write",
		Arguments: map[string]any{
			"path":     "notes/resources/deep.txt",
			"content":  "resource content",
			"encoding": "utf-8",
		},
	})
	if err != nil {
		t.Fatalf("write nested RAM resource: %v", err)
	}
	resourceURI := mcpVFSURI("notes/resources/deep.txt")
	dynamicResource, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: resourceURI})
	if err != nil {
		t.Fatalf("read nested RAM resource through URI template: %v", err)
	}
	if len(dynamicResource.Contents) != 1 || dynamicResource.Contents[0].Text != "resource content" {
		t.Fatalf("nested RAM resource returned unexpected content: %#v", dynamicResource.Contents)
	}

	firstChunk, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "vfs_write_chunk",
		Arguments: map[string]any{
			"path":     "artifacts/example.bin",
			"offset":   0,
			"content":  "hello ",
			"encoding": "utf-8",
			"final":    false,
		},
	})
	if err != nil {
		t.Fatalf("write first VFS chunk: %v", err)
	}
	firstChunkJSON, err := json.Marshal(firstChunk.StructuredContent)
	if err != nil {
		t.Fatalf("marshal first chunk result: %v", err)
	}
	if !strings.Contains(string(firstChunkJSON), `"nextOffset":6`) || !strings.Contains(string(firstChunkJSON), `"complete":false`) {
		t.Fatalf("first chunk returned unexpected result: %s", firstChunkJSON)
	}

	finalChunk, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "vfs_write_chunk",
		Arguments: map[string]any{
			"path":     "artifacts/example.bin",
			"offset":   6,
			"content":  "world",
			"encoding": "utf-8",
			"final":    true,
			"sha256":   "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
	})
	if err != nil {
		t.Fatalf("finalize VFS chunk write: %v", err)
	}
	finalChunkJSON, err := json.Marshal(finalChunk.StructuredContent)
	if err != nil {
		t.Fatalf("marshal final chunk result: %v", err)
	}
	if !strings.Contains(string(finalChunkJSON), `"complete":true`) {
		t.Fatalf("final chunk returned unexpected result: %s", finalChunkJSON)
	}

	chunkedRead, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "vfs_read",
		Arguments: map[string]any{"path": "artifacts/example.bin"},
	})
	if err != nil {
		t.Fatalf("read completed chunked file: %v", err)
	}
	chunkedReadJSON, err := json.Marshal(chunkedRead.StructuredContent)
	if err != nil {
		t.Fatalf("marshal completed chunked file: %v", err)
	}
	if !strings.Contains(string(chunkedReadJSON), `"content":"hello world"`) {
		t.Fatalf("chunked file returned unexpected content: %s", chunkedReadJSON)
	}
}

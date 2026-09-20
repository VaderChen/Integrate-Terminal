package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"IntegTERM/internal/model"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpTestAuthorizationTransport struct{ base http.RoundTripper }

func (transport mcpTestAuthorizationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.Header.Set("Authorization", "Bearer test")
	return transport.base.RoundTrip(copy)
}

func TestMCPHTTPRoundTripDiscoveryAndVirtualFiles(t *testing.T) {
	a := regressionApp(t, nil)
	server := httptest.NewServer(a.restMux())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	connection, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   server.URL + "/mcp",
		HTTPClient: &http.Client{Transport: mcpTestAuthorizationTransport{http.DefaultTransport}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	listed, err := connection.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool)
	for _, tool := range listed.Tools {
		names[tool.Name] = true
	}
	for _, name := range []string{"vfs_workspace_info", "vfs_list", "vfs_write", "vfs_read", "list_sites", "check_server_status", "read_mcp_docs"} {
		if !names[name] {
			t.Fatalf("missing real MCP tool: %s", name)
		}
	}
	call := func(name string, arguments map[string]any) string {
		t.Helper()
		result, err := connection.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
		if err != nil || result.IsError {
			encoded, _ := json.Marshal(result)
			t.Fatalf("%s: error=%v result=%s", name, err, encoded)
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	call("vfs_workspace_info", map[string]any{})
	call("vfs_write", map[string]any{"path": "review/check.txt", "content": "MCP recovery verified"})
	if value := call("vfs_read", map[string]any{"path": "review/check.txt"}); !strings.Contains(value, "MCP recovery verified") {
		t.Fatal("MCP write/read did not preserve content")
	}
	call("check_server_status", map[string]any{})
	// Exercise the actual generated REST adapter with a saved fixture and a
	// local directory; no remote connection or native Keychain is used.
	localPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(localPath, "fixture.txt"), []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	host, port := startMCPFTPFixture(t)
	site := restSiteRequestExample("ftp")
	site["host"], site["port"] = host, port
	site["id"], site["localPath"], site["remotePath"], site["password"] = "mcp-fixture", localPath, "/", ""
	call("save_site", site)
	call("open_file_tab", map[string]any{"site": site})
	tabs, err := a.GetTabs()
	if err != nil || len(tabs) != 1 {
		t.Fatalf("MCP did not create a file tab: %v", err)
	}
	if value := call("list_local_files", map[string]any{"tabId": tabs[0].ID, "path": localPath}); !strings.Contains(value, "fixture.txt") {
		t.Fatal("MCP query arguments did not reach the REST handler")
	}
	operation := a.createRESTOperation("test")
	call("get_operation", map[string]any{"id": operation.ID})
	call("close_tab", map[string]any{"id": tabs[0].ID})
	call("delete_site", map[string]any{"id": "mcp-fixture"})
	partial, partialErr := connection.CallTool(ctx, &mcp.CallToolParams{Name: "update_config", Arguments: map[string]any{"config": map[string]any{"restServerPort": 1}}})
	if partialErr == nil && !partial.IsError {
		t.Fatal("partial config accepted despite full replacement semantics")
	}
	if a.GetConfig().RESTServerPort == 1 {
		t.Fatal("rejected MCP config changed application settings")
	}
	failed, err := connection.CallTool(ctx, &mcp.CallToolParams{Name: "vfs_read", Arguments: map[string]any{"path": "missing"}})
	if err == nil && !failed.IsError {
		t.Fatal("missing file was reported as a successful MCP call")
	}
	resources, err := connection.ListResources(ctx, nil)
	if err != nil || len(resources.Resources) != 1 || resources.Resources[0].URI != mcpVFSRootURI {
		t.Fatalf("missing MCP root resource: %v", err)
	}
}

func TestMCPRESTSchemasUseRealRequestFields(t *testing.T) {
	encoded, _ := json.Marshal(model.Config{})
	var configFields map[string]any
	if err := json.Unmarshal(encoded, &configFields); err != nil {
		t.Fatal(err)
	}
	for field := range configFields {
		if _, exists := restConfigRequestExample()[field]; !exists {
			t.Fatalf("full config example omits %s", field)
		}
	}
	for _, endpoint := range buildRESTEndpointDocs("http://127.0.0.1:18080") {
		schema := mcpInputSchema(endpoint)
		properties := schema["properties"].(map[string]any)
		for _, obsolete := range []string{"note", "query", "pathParameter"} {
			if _, exists := properties[obsolete]; exists {
				t.Fatalf("%s advertises non-request field %s", endpoint.Operation, obsolete)
			}
		}
		for _, name := range []string{"site", "config"} {
			if property, exists := properties[name]; exists && property.(map[string]any)["type"] != "object" {
				t.Fatalf("%s advertises %s as a label instead of an object", endpoint.Operation, name)
			}
		}
	}
}

func TestMCPHTTPUsesAuthorizationAndExactOriginValidation(t *testing.T) {
	a := regressionApp(t, nil)
	handler := a.restMux()
	server := httptest.NewServer(handler)
	defer server.Close()
	for _, test := range []struct {
		name, token, origin, host string
		status                    int
	}{
		{name: "no token", status: http.StatusUnauthorized},
		{name: "wrong token", token: "wrong", status: http.StatusUnauthorized},
		{name: "foreign origin", token: "test", origin: "https://evil.example", status: http.StatusForbidden},
		{name: "userinfo origin", token: "test", origin: "http://localhost:8080@evil.example", status: http.StatusForbidden},
		{name: "rebound host", token: "test", host: "evil.example", status: http.StatusForbidden},
		{name: "valid local origin", token: "test", origin: "http://localhost:8080", status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.host != "" {
				request.Host = test.host
			}
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.status {
				t.Fatalf("status=%d, want %d", response.StatusCode, test.status)
			}
		})
	}
}

func TestMCPDocsNeverContainUserToken(t *testing.T) {
	a := regressionApp(t, nil)
	a.restServerToken = "unique-sensitive-token-never-export"
	doc, err := a.GetRestAPIDocsMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(doc, a.restServerToken) {
		t.Fatal("token leaked into preview/export")
	}
	for _, expected := range []string{"# IntegTERM MCP", "stdio", "/mcp", "<YOUR_API_TOKEN>", "vfs_workspace_info", a.GetMCPStdioExecutable()} {
		if !strings.Contains(doc, expected) {
			t.Fatalf("missing MCP contract detail: %s", expected)
		}
	}
	if strings.Contains(doc, "integterm-rest-skill") {
		t.Fatal("obsolete SKILL contract restored")
	}
}

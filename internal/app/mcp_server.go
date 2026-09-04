package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"

	appversion "github.com/VaderChen/Integrate-Terminal/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpContract string

const (
	mcpContractLocal   mcpContract = "local"
	mcpContractNetwork mcpContract = "network"
)

type mcpVirtualLayer struct {
	app *App
	vfs *mcpVFS
}

func newMCPVirtualLayer(a *App) *mcpVirtualLayer {
	vfs := a.mcpVFS
	if vfs == nil {
		vfs = newMCPVFS()
		a.mcpVFS = vfs
	} else {
		if vfs.nodes == nil {
			vfs.nodes = newMCPVFS().nodes
		}
		if vfs.remoteMounts == nil {
			vfs.remoteMounts = make(map[string]mcpVFSRemoteMount)
		}
		if vfs.chunkWrites == nil {
			vfs.chunkWrites = make(map[string]*mcpVFSChunkWrite)
		}
	}
	return &mcpVirtualLayer{app: a, vfs: vfs}
}

func (layer *mcpVirtualLayer) newServer(contract mcpContract) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "integterm",
		Version: appversion.ProductVersion(),
	}, &mcp.ServerOptions{
		Instructions: mcpServerInstructions(contract),
	})

	switch contract {
	case mcpContractLocal:
		addMCPVFSFeatures(server, layer)
	case mcpContractNetwork:
		layer.addNetworkFeatures(server)
		addMCPVFSFeatures(server, layer)
	}
	return server
}

func mcpServerInstructions(contract mcpContract) string {
	transport := "This local contract uses stdio: start the compiled IntegTERM executable with the single argument `mcp`. This runs a headless MCP server and does not require the source tree or open the desktop UI. Each stdio client owns an independent server process and RAM workspace; it is not a shared file on disk."
	if contract == mcpContractNetwork {
		transport = "This network contract uses Streamable HTTP through the configured `/mcp` endpoint. HTTP is disabled by default and must be enabled in IntegTERM settings before an external client connects or multiple clients need to share one server-side RAM workspace. Clients share RAM data only when they connect to the same running endpoint."
	}

	return strings.Join([]string{
		"IntegTERM provides a virtual filesystem (VFS) that combines a bounded RAM workspace with saved remote-site mounts.",
		transport,
		"`integterm-vfs://workspace/mcp` is the VFS root resource identifier used inside this MCP session. It is not an HTTP endpoint, shell command, or host filesystem path.",
		"Start every VFS task by calling `vfs_workspace_info` with `{}`, then call `vfs_list` with `{}` to list the root.",
		"Path model: an empty path or the root URI selects the RAM workspace root; `notes/file.txt` selects a RAM file; `sites` lists saved remote sites; `sites/{siteID}` selects a remote root; `sites/{siteID}/{relativePath}` selects a remote file or directory.",
		"Use relative VFS paths or full `integterm-vfs://workspace/mcp/...` URIs. Never substitute an absolute local path, a remote absolute path, or a guessed site name for a returned `siteID`.",
		"For RAM work, call `vfs_write`, `vfs_write_chunk`, `vfs_list`, `vfs_stat`, `vfs_read`, `vfs_mkdir`, `vfs_rename`, and `vfs_delete` directly. RAM parent directories are created automatically and RAM data is cleared when the service stops.",
		"For remote work, call `vfs_list` with `path: \"sites\"`, reuse a returned site path or URI, then call `vfs_connect` or let the first remote file operation connect lazily. Remote changes apply directly under that saved site's configured remote root.",
		"`vfs_write` accepts one inline payload up to 4 MiB. For larger content, call `vfs_write_chunk` sequentially with the returned `nextOffset`, set `final: true` on the last call, and provide the complete file SHA-256. Existing files require `overwrite: true` on the final call.",
		"Chunked files are limited to 32 MiB. `vfs_read` returns at most 1 MiB per call and defaults to 256 KiB; when `truncated` is true, continue with `offset + returnedBytes`.",
		"`vfs_delete` requires `recursive: true` for a non-empty directory. `vfs_rename` cannot cross between RAM and a site or between two sites.",
		"Use `resources/read` only for a known file URI and only when the client supports MCP Resources. Prefer VFS tools for discovery, directory traversal, writes, large reads, and explicit error handling.",
	}, "\n")
}

func (layer *mcpVirtualLayer) addNetworkFeatures(server *mcp.Server) {
	endpoints := buildRESTEndpointDocs(fmt.Sprintf("http://127.0.0.1:%d", sanitizeRESTServerPort(layer.app.config.RESTServerPort)))
	operations := buildRESTOperationDocs()
	for _, operation := range operations {
		endpoint, ok := findMCPEndpoint(endpoints, operation)
		if !ok {
			continue
		}
		endpointDoc := endpoint
		tool := &mcp.Tool{
			Name:        operation.LogicalOperation,
			Title:       endpointDoc.Operation,
			Description: endpointDoc.Description + " " + operation.Notes,
			InputSchema: mcpInputSchema(endpointDoc),
		}
		mcp.AddTool(server, tool, func(ctx context.Context, _ *mcp.CallToolRequest, arguments map[string]any) (*mcp.CallToolResult, any, error) {
			output, err := layer.app.callMCPRESTTool(ctx, endpointDoc, arguments)
			return nil, output, err
		})
	}
}

func (a *App) newMCPHTTPHandler() http.Handler {
	return newMCPStreamableHTTPHandler(newMCPVirtualLayer(a).newServer(mcpContractNetwork))
}

// RunMCPStdio serves the local VFS contract over MCP's stdio transport. The
// integterm-vfs URI is a resource identifier carried inside this transport,
// not a network endpoint.
func (a *App) RunMCPStdio(ctx context.Context) error {
	return newMCPVirtualLayer(a).newServer(mcpContractLocal).Run(ctx, &mcp.StdioTransport{})
}

// GetMCPStdioExecutable returns the executable that should be configured as
// the local MCP command. Returning the resolved process path is important for
// packaged desktop apps: a GUI-launched app is usually not on the Agent's
// PATH, so the display name "IntegTERM" is not a reliable command.
func (a *App) GetMCPStdioExecutable() string {
	executablePath, err := os.Executable()
	if err != nil || strings.TrimSpace(executablePath) == "" {
		return "IntegTERM"
	}
	return executablePath
}

func newMCPStreamableHTTPHandler(server *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
	})
}

func findMCPEndpoint(endpoints []endpointDoc, operation operationDoc) (endpointDoc, bool) {
	for _, endpoint := range endpoints {
		if endpoint.Method == operation.Method && endpoint.Path == operation.Path {
			return endpoint, true
		}
	}
	return endpointDoc{}, false
}

func mcpInputSchema(endpoint endpointDoc) map[string]any {
	properties := make(map[string]any)
	for name, example := range endpoint.Request {
		properties[name] = map[string]any{
			"description": fmt.Sprintf("Example: %v", example),
		}
	}
	for _, name := range pathParameterNames(endpoint.Path) {
		properties[name] = map[string]any{"type": "string"}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": true,
	}
}

func (a *App) callMCPRESTTool(ctx context.Context, endpoint endpointDoc, arguments map[string]any) (any, error) {
	requestPath := endpoint.Path
	for _, name := range pathParameterNames(requestPath) {
		value := strings.TrimSpace(fmt.Sprint(arguments[name]))
		if value == "" || value == "<nil>" {
			return nil, fmt.Errorf("%s is required", name)
		}
		requestPath = strings.ReplaceAll(requestPath, "{"+name+"}", url.PathEscape(value))
	}

	payload := cloneMCPArguments(arguments)
	for _, name := range pathParameterNames(endpoint.Path) {
		delete(payload, name)
	}

	var body bytes.Buffer
	if endpoint.Method == http.MethodGet {
		query := make(url.Values)
		for name, value := range payload {
			appendMCPQueryValue(query, name, value)
		}
		if encoded := query.Encode(); encoded != "" {
			requestPath += "?" + encoded
		}
	} else if len(payload) > 0 {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return nil, err
		}
	}

	request := httptest.NewRequest(endpoint.Method, requestPath, &body).WithContext(ctx)
	if body.Len() > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()

	if strings.HasPrefix(requestPath, "/api/sites") {
		a.stateMu.Lock()
		if err := a.reloadSitesFromStoreLocked(); err != nil {
			a.stateMu.Unlock()
			return nil, fmt.Errorf("reload sites: %w", err)
		}
		a.stateMu.Unlock()
	}
	a.restRoutesMux().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var failure map[string]any
		if err := json.NewDecoder(response.Body).Decode(&failure); err == nil {
			return nil, fmt.Errorf("%v", failure["error"])
		}
		return nil, fmt.Errorf("operation failed with HTTP %d", response.StatusCode)
	}

	if strings.HasPrefix(response.Header.Get("Content-Type"), "text/") {
		return recorder.Body.String(), nil
	}
	var result any
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		return recorder.Body.String(), nil
	}
	return result, nil
}

func cloneMCPArguments(arguments map[string]any) map[string]any {
	result := make(map[string]any, len(arguments))
	for name, value := range arguments {
		result[name] = value
	}
	return result
}

func pathParameterNames(requestPath string) []string {
	var names []string
	for {
		start := strings.IndexByte(requestPath, '{')
		if start < 0 {
			return names
		}
		end := strings.IndexByte(requestPath[start+1:], '}')
		if end < 0 {
			return names
		}
		end += start + 1
		names = append(names, requestPath[start+1:end])
		requestPath = requestPath[end+1:]
	}
}

func appendMCPQueryValue(query url.Values, name string, value any) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			query.Add(name, fmt.Sprint(item))
		}
	case []string:
		for _, item := range typed {
			query.Add(name, item)
		}
	default:
		query.Set(name, fmt.Sprint(value))
	}
}

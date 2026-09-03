package app

import (
	"encoding/json"
	"fmt"
	"strings"
)

type operationDoc struct {
	LogicalOperation string
	Method           string
	Path             string
	Notes            string
}

type endpointDoc struct {
	Operation   string
	Method      string
	Path        string
	Category    string
	Description string
	Request     map[string]any
	Response    map[string]any
	Example     string
}

type mcpToolDoc struct {
	Name        string
	Description string
}

func (a *App) GetRestAPIDocsMarkdown() (string, error) {
	return a.GetMCPContractMarkdown(string(mcpContractNetwork))
}

func (a *App) GetMCPContractMarkdown(contract string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(contract)) {
	case string(mcpContractLocal):
		return a.buildMCPLocalContractMarkdown()
	case string(mcpContractNetwork):
		return a.buildMCPNetworkContractMarkdown()
	default:
		return "", fmt.Errorf("unknown MCP contract %q; use local or network", contract)
	}
}

func (a *App) buildMCPNetworkContractMarkdown() (string, error) {
	status := a.GetRESTServerStatus()
	port := status.Port
	if port <= 0 {
		port = defaultRESTServerPort
	}
	mcpURL := status.MCPURL
	if mcpURL == "" || mcpURL == "/mcp" {
		mcpURL = fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	}
	allowlist := status.Allowlist
	operations := buildRESTOperationDocs()
	endpoints := buildRESTEndpointDocs(strings.TrimSuffix(mcpURL, "/mcp"))

	var builder strings.Builder
	builder.WriteString("# IntegTERM MCP Server\n\n")
	builder.WriteString("IntegTERM provides a local VFS MCP contract by default and an optional standard Model Context Protocol server over Streamable HTTP. Network MCP clients discover and call tools through the endpoint below; no API token or custom authentication header is required.\n\n")
	builder.WriteString("## Connection\n\n")
	builder.WriteString("- Transport: `streamable-http`\n")
	builder.WriteString("- MCP URL: `" + mcpURL + "`\n")
	builder.WriteString(fmt.Sprintf("- Enabled: `%t`\n", status.Enabled))
	builder.WriteString("- HTTP default: `disabled` (enable it only when an external client must connect)\n")
	builder.WriteString("- Access control: source IP allowlist\n")
	builder.WriteString("- Allowlist: `" + strings.Join(allowlist, ", ") + "`\n")
	builder.WriteString("- Default allowlist: `127.0.0.1`\n\n")
	builder.WriteString("## MCP Client Configuration\n\n")
	builder.WriteString("```json\n")
	builder.WriteString(mustJSONIndent(map[string]any{
		"mcpServers": map[string]any{
			"integterm": map[string]any{
				"type": "streamable-http",
				"url":  mcpURL,
			},
		},
	}))
	builder.WriteString("\n```\n\n")
	builder.WriteString("## Security\n\n")
	builder.WriteString("1. Requests are accepted only when the TCP source address matches an allowlisted IP address or CIDR range.\n")
	builder.WriteString("2. The default configuration accepts only `127.0.0.1`.\n")
	builder.WriteString("3. Add a LAN address or CIDR only when a remote MCP client must connect.\n")
	builder.WriteString("4. Browser origins are checked against the same allowlist.\n")
	builder.WriteString("5. Do not configure a broad CIDR unless every host in that network is trusted.\n\n")
	builder.WriteString("## Virtual Workspace\n\n")
	builder.WriteString("- Virtual root URI: `" + mcpVFSRootURI + "`\n")
	builder.WriteString("- Saved sites namespace: `" + mcpVFSRootURI + "/sites/{siteID}`\n")
	builder.WriteString("- External clients connect through the MCP URL above; the `integterm-vfs` URI identifies resources inside that MCP connection and is not a transport endpoint.\n")
	builder.WriteString("- Call `vfs_connect` with a saved-site URI before remote operations, or let the first remote `vfs_list`, `vfs_stat`, `vfs_read`, `vfs_write`, `vfs_mkdir`, `vfs_rename`, or `vfs_delete` call connect lazily.\n")
	builder.WriteString("- Paths outside the `sites` namespace remain bounded RAM files and are cleared when the background service stops.\n\n")
	builder.WriteString("## Available Tools\n\n")
	builder.WriteString("| Tool | Description |\n")
	builder.WriteString("| --- | --- |\n")
	for _, operation := range operations {
		description := operation.Notes
		if endpoint, ok := findMCPEndpoint(endpoints, operation); ok && strings.TrimSpace(endpoint.Description) != "" {
			description = endpoint.Description
		}
		builder.WriteString(fmt.Sprintf("| `%s` | %s |\n", operation.LogicalOperation, strings.ReplaceAll(description, "|", "\\|")))
	}
	for _, tool := range buildMCPVFSToolDocs() {
		builder.WriteString(fmt.Sprintf("| `%s` | %s |\n", tool.Name, strings.ReplaceAll(tool.Description, "|", "\\|")))
	}
	builder.WriteString("\n## Usage Rules\n\n")
	builder.WriteString("- Use `tools/list` to discover the current schemas instead of constructing REST requests.\n")
	builder.WriteString("- Use `tools/call` with the exact tool name and arguments returned by the MCP server.\n")
	builder.WriteString("- Reuse returned `site`, `tabId`, `sessionId`, and operation IDs exactly as returned.\n")
	builder.WriteString("- Use absolute host paths for local and remote file operations exposed by the REST-backed tools; use `integterm-vfs` URIs for virtual workspace operations.\n")
	builder.WriteString("- Upload and download tools return an operation ID; poll `get_operation` until it is `done` or `failed`.\n")
	builder.WriteString("- Saved site protocol `sftp` provides SSH and SFTP capabilities; `ftp` provides Telnet and FTP capabilities.\n")

	return builder.String(), nil
}

func (a *App) buildMCPLocalContractMarkdown() (string, error) {
	status := a.GetRESTServerStatus()

	var builder strings.Builder
	builder.WriteString("# IntegTERM Virtual Workspace Contract\n\n")
	builder.WriteString("This contract defines a virtual filesystem spanning bounded RAM paths and saved remote-site mounts. The local VFS MCP contract is available by default; external MCP clients connect through the optional network MCP endpoint. The virtual URI identifies resources and selects the correct backend inside that connection.\n\n")
	builder.WriteString("## Virtual Workspace\n\n")
	builder.WriteString("- Virtual root URI: `" + mcpVFSRootURI + "`\n")
	builder.WriteString("- Local VFS MCP: `enabled by default`\n")
	builder.WriteString(fmt.Sprintf("- HTTP MCP server: `%t`\n", status.Enabled))
	builder.WriteString("- RAM paths: any path outside `sites`; data is cleared when the background service stops\n")
	builder.WriteString("- Saved sites namespace: `" + mcpVFSRootURI + "/sites/{siteID}`\n")
	builder.WriteString("- Remote paths: descendants of a saved-site URI, resolved relative to that site's configured remote root\n\n")

	builder.WriteString("## Available Tools\n\n")
	builder.WriteString("| Tool | Description |\n")
	builder.WriteString("| --- | --- |\n")
	for _, tool := range buildMCPVFSToolDocs() {
		builder.WriteString(fmt.Sprintf("| `%s` | %s |\n", tool.Name, tool.Description))
	}

	builder.WriteString("\n## Resources\n\n")
	builder.WriteString("- Root resource: `" + mcpVFSRootURI + "`\n")
	builder.WriteString("- File resource template: `integterm-vfs://workspace/mcp/{path}`\n")
	builder.WriteString("- Saved site root: `integterm-vfs://workspace/mcp/sites/{siteID}`\n")
	builder.WriteString("- Remote file: `integterm-vfs://workspace/mcp/sites/{siteID}/{relativeRemotePath}`\n")
	builder.WriteString("- Use the virtual URI returned by `vfs_list`, `vfs_stat`, and `vfs_read`; remote URIs never expose credentials.\n\n")

	builder.WriteString("## Usage Rules\n\n")
	builder.WriteString("- Connect the MCP client to the network endpoint before using any virtual URI; the URI is not itself a transport.\n")
	builder.WriteString("- Call `tools/list`, then `vfs_list` on `sites` to discover saved site IDs without exposing passwords.\n")
	builder.WriteString("- Call `vfs_connect` with a saved-site URI, or let the first remote VFS operation connect lazily.\n")
	builder.WriteString("- Use relative virtual paths or `integterm-vfs://workspace/mcp/...` URIs; cross-site rename is rejected.\n")
	builder.WriteString("- VFS content reads and writes are bounded to 4 MiB per file; use the network transfer tools for larger files.\n")
	builder.WriteString("- SSH and Telnet terminal sessions remain explicit network tools because they are streams rather than filesystem resources.\n")

	return builder.String(), nil
}

func buildMCPVFSToolDocs() []mcpToolDoc {
	return []mcpToolDoc{
		{Name: "vfs_workspace_info", Description: "Read the workspace URI, RAM limits, saved-site count, and active mount count."},
		{Name: "vfs_list", Description: "List RAM entries, saved sites, or files in a mounted remote directory."},
		{Name: "vfs_connect", Description: "Connect a saved site by its `integterm-vfs` site URI."},
		{Name: "vfs_stat", Description: "Read metadata for a RAM or mounted remote file or directory."},
		{Name: "vfs_read", Description: "Read bounded UTF-8 or base64 content from a RAM or remote file."},
		{Name: "vfs_write", Description: "Write UTF-8 or base64 content to a RAM or remote file."},
		{Name: "vfs_mkdir", Description: "Create a RAM or remote directory."},
		{Name: "vfs_rename", Description: "Rename an entry within one RAM namespace or saved-site mount."},
		{Name: "vfs_delete", Description: "Delete a RAM or remote file or directory."},
	}
}

func buildRESTOperationDocs() []operationDoc {
	return []operationDoc{
		{LogicalOperation: "read_skill_markdown", Method: "GET", Path: "/api/docs.md", Notes: "Read the canonical Markdown skill contract"},
		{LogicalOperation: "check_server_status", Method: "GET", Path: "/api/status", Notes: "First step before using the server"},
		{LogicalOperation: "list_sites", Method: "GET", Path: "/api/sites", Notes: "Resolve a usable site object"},
		{LogicalOperation: "save_site", Method: "POST", Path: "/api/sites", Notes: "Create or update a saved site"},
		{LogicalOperation: "delete_site", Method: "DELETE", Path: "/api/sites/{id}", Notes: "Delete a saved site by id"},
		{LogicalOperation: "reorder_sites", Method: "POST", Path: "/api/sites/reorder", Notes: "Persist site ordering"},
		{LogicalOperation: "list_tabs", Method: "GET", Path: "/api/tabs", Notes: "Inspect current tabs"},
		{LogicalOperation: "close_tab", Method: "DELETE", Path: "/api/tabs/{id}", Notes: "Close an existing tab"},
		{LogicalOperation: "open_file_tab", Method: "POST", Path: "/api/tabs/file", Notes: "Open SFTP/FTP file tab"},
		{LogicalOperation: "open_ssh_session", Method: "POST", Path: "/api/tabs/ssh", Notes: "Open interactive SSH session"},
		{LogicalOperation: "open_telnet_session", Method: "POST", Path: "/api/tabs/telnet", Notes: "Open interactive Telnet session"},
		{LogicalOperation: "open_local_session", Method: "POST", Path: "/api/tabs/local", Notes: "Open local terminal session"},
		{LogicalOperation: "execute_single_ssh_command", Method: "POST", Path: "/api/ssh/execute", Notes: "One-shot remote command"},
		{LogicalOperation: "read_terminal_output", Method: "GET", Path: "/api/terminal/output", Notes: "Read buffered session output"},
		{LogicalOperation: "write_terminal_input", Method: "POST", Path: "/api/terminal/input", Notes: "Send text into a session"},
		{LogicalOperation: "resize_terminal", Method: "POST", Path: "/api/terminal/resize", Notes: "Resize a session"},
		{LogicalOperation: "close_terminal_session", Method: "POST", Path: "/api/terminal/close", Notes: "Close by session id"},
		{LogicalOperation: "list_remote_files", Method: "GET", Path: "/api/files/remote", Notes: "Requires a valid file tab"},
		{LogicalOperation: "list_local_files", Method: "GET", Path: "/api/files/local", Notes: "Requires a valid tab"},
		{LogicalOperation: "upload_files", Method: "POST", Path: "/api/files/upload", Notes: "Returns HTTP 202 with operation id"},
		{LogicalOperation: "download_files", Method: "POST", Path: "/api/files/download", Notes: "Returns HTTP 202 with operation id"},
		{LogicalOperation: "get_operation", Method: "GET", Path: "/api/operations/{id}", Notes: "Poll asynchronous upload/download status"},
		{LogicalOperation: "stat_remote_path", Method: "GET", Path: "/api/sftp/stat", Notes: "Read remote metadata"},
		{LogicalOperation: "create_remote_directory", Method: "POST", Path: "/api/sftp/mkdir", Notes: "Requires tabId and absolute path"},
		{LogicalOperation: "rename_remote_path", Method: "POST", Path: "/api/sftp/rename", Notes: "Requires tabId, oldPath, newPath"},
		{LogicalOperation: "delete_remote_path", Method: "POST", Path: "/api/sftp/delete", Notes: "Requires tabId and absolute path"},
		{LogicalOperation: "list_transfers", Method: "GET", Path: "/api/transfers", Notes: "Inspect queue state"},
		{LogicalOperation: "list_logs", Method: "GET", Path: "/api/logs", Notes: "Inspect recent logs"},
		{LogicalOperation: "get_config", Method: "GET", Path: "/api/config", Notes: "Read app config"},
		{LogicalOperation: "update_config", Method: "PUT", Path: "/api/config", Notes: "Replace config object"},
	}
}

func mustJSONIndent(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func buildRESTEndpointDocs(baseURL string) []endpointDoc {
	return []endpointDoc{
		{
			Operation:   "Read Skill Markdown",
			Method:      "GET",
			Path:        "/api/docs.md",
			Category:    "docs",
			Description: "Return the MCP connection guide in Markdown format.",
			Response:    map[string]any{"contentType": "text/markdown", "body": "markdown document"},
			Example:     fmt.Sprintf("curl %s/api/docs.md", baseURL),
		},
		{
			Operation:   "Check Server Status",
			Method:      "GET",
			Path:        "/api/status",
			Category:    "docs",
			Description: "Return MCP server runtime status.",
			Response:    map[string]any{"enabled": true, "running": true, "baseURL": baseURL},
			Example:     fmt.Sprintf("curl %s/api/status", baseURL),
		},
		{
			Operation:   "List Sites",
			Method:      "GET",
			Path:        "/api/sites",
			Category:    "support",
			Description: "List all saved sites. Each site keeps protocol as the site family (`sftp` or `ftp`) and also includes explicit terminal/file capability fields so API clients can see SSH or Telnet support directly.",
			Response:    map[string]any{"sites": []map[string]any{{"protocol": "sftp", "protocolLabel": "ssh/sftp", "supportedModes": []string{"ssh", "sftp"}, "primaryTerminalProtocol": "ssh", "primaryFileProtocol": "sftp"}}},
			Example:     fmt.Sprintf("curl %s/api/sites", baseURL),
		},
		{
			Operation:   "Save Site",
			Method:      "POST",
			Path:        "/api/sites",
			Category:    "support",
			Description: "Create or update a saved site. `protocol` is the saved site family, not a single action mode.",
			Request: map[string]any{
				"name":           "Production SFTP",
				"protocol":       "sftp",
				"protocolLabel":  "ssh/sftp",
				"supportedModes": []string{"ssh", "sftp"},
				"note":           "Saved site protocol family. `sftp` means the site supports both SSH terminal and SFTP file transfer. `ftp` means the site supports both Telnet terminal and FTP file transfer.",
				"host":           "example.com",
				"port":           22,
				"username":       "deploy",
				"password":       "optional",
				"localPath":      "/Users/demo/project",
				"remotePath":     "/srv/app",
			},
			Response: map[string]any{"sites": []string{"updated site list"}},
			Example:  fmt.Sprintf("curl -X POST %s/api/sites -H 'Content-Type: application/json' -d '{\"protocol\":\"sftp\",\"host\":\"example.com\",\"port\":22,\"username\":\"deploy\",\"localPath\":\"/Users/demo/project\",\"remotePath\":\"/srv/app\"}'", baseURL),
		},
		{
			Operation:   "Delete Site",
			Method:      "DELETE",
			Path:        "/api/sites/{id}",
			Category:    "support",
			Description: "Delete a saved site.",
			Response:    map[string]any{"sites": []string{"updated site list"}},
			Example:     fmt.Sprintf("curl -X DELETE %s/api/sites/site-123", baseURL),
		},
		{
			Operation:   "Reorder Sites",
			Method:      "POST",
			Path:        "/api/sites/reorder",
			Category:    "support",
			Description: "Persist a custom site order.",
			Request:     map[string]any{"siteIDs": []string{"site-2", "site-1"}},
			Response:    map[string]any{"sites": []string{"reordered site list"}},
			Example:     fmt.Sprintf("curl -X POST %s/api/sites/reorder -H 'Content-Type: application/json' -d '{\"siteIDs\":[\"site-2\",\"site-1\"]}'", baseURL),
		},
		{
			Operation:   "List Tabs",
			Method:      "GET",
			Path:        "/api/tabs",
			Category:    "support",
			Description: "List all current tabs.",
			Response:    map[string]any{"tabs": []string{"tab objects"}},
			Example:     fmt.Sprintf("curl %s/api/tabs", baseURL),
		},
		{
			Operation:   "Open File Tab",
			Method:      "POST",
			Path:        "/api/tabs/file",
			Category:    "sftp",
			Description: "Open an SFTP/FTP file transfer tab from a site payload. Use the site's `primaryFileProtocol` or `supportedModes` if you need explicit capability hints.",
			Request:     map[string]any{"site": "site object"},
			Response:    map[string]any{"tabs": []string{"updated tab list"}},
			Example:     fmt.Sprintf("curl -X POST %s/api/tabs/file -H 'Content-Type: application/json' -d '{\"site\":{\"protocol\":\"sftp\",\"host\":\"example.com\",\"port\":22,\"username\":\"deploy\",\"localPath\":\"/Users/demo/project\",\"remotePath\":\"/srv/app\"}}'", baseURL),
		},
		{
			Operation:   "Open SSH Session",
			Method:      "POST",
			Path:        "/api/tabs/ssh",
			Category:    "terminal",
			Description: "Open an SSH terminal tab for interactive command control. This is valid for saved sites whose protocol family is `sftp`.",
			Request:     map[string]any{"site": "ssh site object"},
			Response:    map[string]any{"tabs": []string{"updated tab list"}, "sessionId": "terminal session id"},
			Example:     fmt.Sprintf("curl -X POST %s/api/tabs/ssh -H 'Content-Type: application/json' -d '{\"site\":{\"protocol\":\"sftp\",\"host\":\"example.com\",\"port\":22,\"username\":\"deploy\",\"localPath\":\"/Users/demo/project\",\"remotePath\":\"/srv/app\"}}'", baseURL),
		},
		{
			Operation:   "Open Telnet Session",
			Method:      "POST",
			Path:        "/api/tabs/telnet",
			Category:    "terminal",
			Description: "Open a Telnet terminal tab. This is valid for saved sites whose protocol family is `ftp`.",
			Request:     map[string]any{"site": "telnet site object"},
			Response:    map[string]any{"tabs": []string{"updated tab list"}, "sessionId": "terminal session id"},
			Example:     fmt.Sprintf("curl -X POST %s/api/tabs/telnet -H 'Content-Type: application/json' -d '{\"site\":{\"protocol\":\"ftp\",\"host\":\"example.com\",\"port\":23,\"username\":\"demo\",\"localPath\":\"/Users/demo\",\"remotePath\":\"/\"}}'", baseURL),
		},
		{
			Operation:   "Open Local Session",
			Method:      "POST",
			Path:        "/api/tabs/local",
			Category:    "terminal",
			Description: "Open a local terminal tab for local shell control.",
			Request:     map[string]any{"cwd": "/Users/demo/project"},
			Response:    map[string]any{"tabs": []string{"updated tab list"}, "sessionId": "local session id"},
			Example:     fmt.Sprintf("curl -X POST %s/api/tabs/local -H 'Content-Type: application/json' -d '{\"cwd\":\"/Users/demo/project\"}'", baseURL),
		},
		{
			Operation:   "Close Tab",
			Method:      "DELETE",
			Path:        "/api/tabs/{id}",
			Category:    "support",
			Description: "Close a tab by id.",
			Response:    map[string]any{"tabs": []string{"updated tab list"}},
			Example:     fmt.Sprintf("curl -X DELETE %s/api/tabs/tab-123", baseURL),
		},
		{
			Operation:   "List Local Files",
			Method:      "GET",
			Path:        "/api/files/local",
			Category:    "support",
			Description: "List local files for a tab and path.",
			Request:     map[string]any{"query": "tabId=tab-123&path=/Users/demo/project"},
			Response:    map[string]any{"entries": []string{"file entries"}},
			Example:     fmt.Sprintf("curl '%s/api/files/local?tabId=tab-123&path=/Users/demo/project'", baseURL),
		},
		{
			Operation:   "List Remote Files",
			Method:      "GET",
			Path:        "/api/files/remote",
			Category:    "sftp",
			Description: "List remote files for an active SFTP/FTP tab and path.",
			Request:     map[string]any{"query": "tabId=tab-123&path=/srv/app"},
			Response:    map[string]any{"entries": []string{"file entries"}},
			Example:     fmt.Sprintf("curl '%s/api/files/remote?tabId=tab-123&path=/srv/app'", baseURL),
		},
		{
			Operation:   "Upload Files",
			Method:      "POST",
			Path:        "/api/files/upload",
			Category:    "sftp",
			Description: "Queue local files or folders for upload. Returns HTTP 202 immediately; poll the returned operation id.",
			Request:     map[string]any{"tabId": "tab-123", "localPaths": []string{"/Users/demo/build.zip"}, "remoteBase": "/srv/releases"},
			Response:    map[string]any{"operation": map[string]any{"id": "operation-id", "kind": "upload", "status": "queued"}},
			Example:     fmt.Sprintf("curl -X POST %s/api/files/upload -H 'Content-Type: application/json' -d '{\"tabId\":\"tab-123\",\"localPaths\":[\"/Users/demo/build.zip\"],\"remoteBase\":\"/srv/releases\"}'", baseURL),
		},
		{
			Operation:   "Download Files",
			Method:      "POST",
			Path:        "/api/files/download",
			Category:    "sftp",
			Description: "Queue remote files or folders for download. Returns HTTP 202 immediately; poll the returned operation id.",
			Request:     map[string]any{"tabId": "tab-123", "remotePaths": []string{"/srv/app/config.yml"}, "localBase": "/Users/demo/downloads"},
			Response:    map[string]any{"operation": map[string]any{"id": "operation-id", "kind": "download", "status": "queued"}},
			Example:     fmt.Sprintf("curl -X POST %s/api/files/download -H 'Content-Type: application/json' -d '{\"tabId\":\"tab-123\",\"remotePaths\":[\"/srv/app/config.yml\"],\"localBase\":\"/Users/demo/downloads\"}'", baseURL),
		},
		{
			Operation:   "Get Asynchronous Operation",
			Method:      "GET",
			Path:        "/api/operations/{id}",
			Category:    "support",
			Description: "Poll an asynchronous upload or download until status becomes done or failed.",
			Request:     map[string]any{"pathParameter": "operation id returned by upload/download"},
			Response:    map[string]any{"operation": map[string]any{"id": "operation-id", "kind": "upload", "status": "running|done|failed", "error": "present when failed"}},
			Example:     fmt.Sprintf("curl %s/api/operations/operation-id", baseURL),
		},
		{
			Operation:   "Stat Remote Path",
			Method:      "GET",
			Path:        "/api/sftp/stat",
			Category:    "sftp",
			Description: "Read a single remote entry metadata by path.",
			Request:     map[string]any{"query": "tabId=tab-123&path=/srv/app/config.yml"},
			Response:    map[string]any{"name": "config.yml", "path": "/srv/app/config.yml", "isDir": false, "size": 128},
			Example:     fmt.Sprintf("curl '%s/api/sftp/stat?tabId=tab-123&path=/srv/app/config.yml'", baseURL),
		},
		{
			Operation:   "Create Remote Directory",
			Method:      "POST",
			Path:        "/api/sftp/mkdir",
			Category:    "sftp",
			Description: "Create a remote directory by absolute path.",
			Request:     map[string]any{"tabId": "tab-123", "path": "/srv/app/releases"},
			Response:    map[string]any{"ok": true},
			Example:     fmt.Sprintf("curl -X POST %s/api/sftp/mkdir -H 'Content-Type: application/json' -d '{\"tabId\":\"tab-123\",\"path\":\"/srv/app/releases\"}'", baseURL),
		},
		{
			Operation:   "Rename Remote Path",
			Method:      "POST",
			Path:        "/api/sftp/rename",
			Category:    "sftp",
			Description: "Rename or move a remote file or directory.",
			Request:     map[string]any{"tabId": "tab-123", "oldPath": "/srv/app/.env.tmp", "newPath": "/srv/app/.env"},
			Response:    map[string]any{"ok": true},
			Example:     fmt.Sprintf("curl -X POST %s/api/sftp/rename -H 'Content-Type: application/json' -d '{\"tabId\":\"tab-123\",\"oldPath\":\"/srv/app/.env.tmp\",\"newPath\":\"/srv/app/.env\"}'", baseURL),
		},
		{
			Operation:   "Delete Remote Path",
			Method:      "POST",
			Path:        "/api/sftp/delete",
			Category:    "sftp",
			Description: "Delete a remote file or directory recursively.",
			Request:     map[string]any{"tabId": "tab-123", "path": "/srv/app/old-release"},
			Response:    map[string]any{"ok": true},
			Example:     fmt.Sprintf("curl -X POST %s/api/sftp/delete -H 'Content-Type: application/json' -d '{\"tabId\":\"tab-123\",\"path\":\"/srv/app/old-release\"}'", baseURL),
		},
		{
			Operation:   "Execute Single SSH Command",
			Method:      "POST",
			Path:        "/api/ssh/execute",
			Category:    "ssh",
			Description: "Execute a single SSH command and return stdout, stderr, and exit code. Use a site whose protocol family is `sftp`, which means SSH terminal + SFTP file transfer support.",
			Request:     map[string]any{"site": "ssh site object", "command": "pwd", "timeoutSeconds": 10},
			Response:    map[string]any{"ok": true, "stdout": "/srv/app\n", "stderr": "", "exitCode": 0},
			Example:     fmt.Sprintf("curl -X POST %s/api/ssh/execute -H 'Content-Type: application/json' -d '{\"site\":{\"host\":\"example.com\",\"port\":22,\"username\":\"deploy\",\"protocol\":\"sftp\",\"localPath\":\"/tmp\",\"remotePath\":\"/srv/app\",\"password\":\"secret\"},\"command\":\"pwd\",\"timeoutSeconds\":10}'", baseURL),
		},
		{
			Operation:   "Read Terminal Output",
			Method:      "GET",
			Path:        "/api/terminal/output",
			Category:    "terminal",
			Description: "Read buffered SSH/local terminal output for a session id.",
			Request:     map[string]any{"query": "sessionId=session-123"},
			Response:    map[string]any{"sessionId": "session-123", "output": "buffered output"},
			Example:     fmt.Sprintf("curl '%s/api/terminal/output?sessionId=session-123'", baseURL),
		},
		{
			Operation:   "Write Terminal Input",
			Method:      "POST",
			Path:        "/api/terminal/input",
			Category:    "terminal",
			Description: "Write raw input into an SSH, Telnet, or local terminal session.",
			Request:     map[string]any{"sessionId": "session-123", "data": "ls -la\n"},
			Response:    map[string]any{"ok": true},
			Example:     fmt.Sprintf("curl -X POST %s/api/terminal/input -H 'Content-Type: application/json' -d '{\"sessionId\":\"session-123\",\"data\":\"ls -la\\n\"}'", baseURL),
		},
		{
			Operation:   "Resize Terminal",
			Method:      "POST",
			Path:        "/api/terminal/resize",
			Category:    "terminal",
			Description: "Resize an SSH, Telnet, or local terminal session.",
			Request:     map[string]any{"sessionId": "session-123", "cols": 120, "rows": 32},
			Response:    map[string]any{"ok": true},
			Example:     fmt.Sprintf("curl -X POST %s/api/terminal/resize -H 'Content-Type: application/json' -d '{\"sessionId\":\"session-123\",\"cols\":120,\"rows\":32}'", baseURL),
		},
		{
			Operation:   "Close Terminal Session",
			Method:      "POST",
			Path:        "/api/terminal/close",
			Category:    "terminal",
			Description: "Close an SSH, Telnet, or local terminal session by session id.",
			Request:     map[string]any{"sessionId": "session-123"},
			Response:    map[string]any{"ok": true},
			Example:     fmt.Sprintf("curl -X POST %s/api/terminal/close -H 'Content-Type: application/json' -d '{\"sessionId\":\"session-123\"}'", baseURL),
		},
		{
			Operation:   "List Transfers",
			Method:      "GET",
			Path:        "/api/transfers",
			Category:    "transfers",
			Description: "List transfer queue items.",
			Response:    map[string]any{"transfers": []string{"transfer items"}},
			Example:     fmt.Sprintf("curl %s/api/transfers", baseURL),
		},
		{
			Operation:   "List Logs",
			Method:      "GET",
			Path:        "/api/logs",
			Category:    "transfers",
			Description: "List operation logs.",
			Response:    map[string]any{"logs": []string{"log items"}},
			Example:     fmt.Sprintf("curl %s/api/logs", baseURL),
		},
		{
			Operation:   "Get Config",
			Method:      "GET",
			Path:        "/api/config",
			Category:    "config",
			Description: "Read current app config.",
			Response:    map[string]any{"config": "config object"},
			Example:     fmt.Sprintf("curl %s/api/config", baseURL),
		},
		{
			Operation:   "Update Config",
			Method:      "PUT",
			Path:        "/api/config",
			Category:    "config",
			Description: "Update app config, including MCP server and allowlist settings.",
			Request:     map[string]any{"config": "full config object"},
			Response:    map[string]any{"config": "updated config object"},
			Example:     fmt.Sprintf("curl -X PUT %s/api/config -H 'Content-Type: application/json' -d '{\"config\":{\"restServerEnabled\":true,\"restServerPort\":18080}}'", baseURL),
		},
	}
}

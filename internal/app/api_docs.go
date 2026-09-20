package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
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

// GetRestAPIDocsMarkdown is retained as a Wails/REST compatibility name.
// The generated contract now describes the real MCP transports and never embeds
// the user's token; explicit token reveal/copy is a separate settings action.
func (a *App) getRestAPIDocsMarkdownLocked() (string, error) {
	status := a.getRESTServerStatusLocked()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", status.Port)
	var builder strings.Builder
	builder.WriteString("# IntegTERM MCP\n\n")
	builder.WriteString("Generated: " + time.Now().Format(time.RFC3339) + "\n\n")
	builder.WriteString("## Local stdio\n\n")
	builder.WriteString(mcpServerInstructions(mcpContractLocal) + "\n\n")
	builder.WriteString("Configure an MCP client with this executable and argument array. The stdio process works without opening the desktop UI or enabling HTTP. No HTTP token is needed for stdio.\n\n```json\n")
	builder.WriteString(mustJSONIndent(map[string]any{"mcpServers": map[string]any{"integterm": map[string]any{"command": a.GetMCPStdioExecutable(), "args": []string{"mcp"}}}}))
	builder.WriteString("\n```\n\n## Streamable HTTP\n\n")
	builder.WriteString(mcpServerInstructions(mcpContractNetwork) + "\n\n")
	builder.WriteString(fmt.Sprintf("Endpoint: `%s/mcp`\n\nHTTP enabled: `%t`\n\n", baseURL, status.Enabled))
	builder.WriteString("HTTP listens only on this Mac's loopback address. Supply your API token from Settings in the Authorization header. The placeholder below is intentional; exported documentation and examples never contain your actual token. Client configuration keys may differ by MCP client.\n\n```json\n")
	builder.WriteString(mustJSONIndent(map[string]any{"mcpServers": map[string]any{"integterm": map[string]any{"url": baseURL + "/mcp", "headers": map[string]string{"Authorization": "Bearer <YOUR_API_TOKEN>"}}}}))
	builder.WriteString("\n```\n\n## VFS tools and resources\n\n")
	builder.WriteString("Both transports expose `vfs_workspace_info`, `vfs_list`, `vfs_stat`, `vfs_read`, `vfs_write`, `vfs_write_chunk`, `vfs_mkdir`, `vfs_delete`, `vfs_rename`, and `vfs_connect`. Start with `vfs_workspace_info` using `{}`, then `vfs_list` using `{}`. Discover exact argument schemas through MCP `tools/list`; call tools through `tools/call`. The root resource is `integterm-vfs://workspace/mcp`.\n\n")
	builder.WriteString("## HTTP automation tools\n\n")
	builder.WriteString("HTTP additionally exposes the following operations through MCP. The REST paths are also available for existing integrations. These are not stdio tools. Keep returned IDs; do not substitute names for IDs or infer successful completion from an accepted asynchronous transfer.\n\n")
	builder.WriteString("| MCP tool | REST method | REST path | Notes |\n| --- | --- | --- | --- |\n")
	for _, operation := range buildRESTOperationDocs() {
		builder.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", operation.LogicalOperation, operation.Method, operation.Path, operation.Notes))
	}
	builder.WriteString("\n## REST compatibility reference\n\n")
	for _, endpoint := range buildRESTEndpointDocs(baseURL) {
		builder.WriteString("### " + endpoint.Operation + "\n\n`" + endpoint.Method + " " + endpoint.Path + "`\n\n" + endpoint.Description + "\n\n")
		if len(endpoint.Request) > 0 {
			builder.WriteString("Request example:\n\n```json\n" + mustJSONIndent(endpoint.Request) + "\n```\n\n")
		}
		if len(endpoint.Response) > 0 {
			builder.WriteString("Response shape:\n\n```json\n" + mustJSONIndent(endpoint.Response) + "\n```\n\n")
		}
		builder.WriteString("```sh\n" + addRESTAuthToCurl(endpoint.Example, "<YOUR_API_TOKEN>") + "\n```\n\n")
	}
	return builder.String(), nil
}

func addRESTAuthToCurl(example string, token string) string {
	if token == "" || !strings.HasPrefix(example, "curl ") {
		return example
	}
	return strings.Replace(example, "curl ", "curl -H 'Authorization: Bearer "+token+"' ", 1)
}

func buildRESTOperationDocs() []operationDoc {
	return []operationDoc{
		{LogicalOperation: "read_mcp_docs", Method: "GET", Path: "/api/docs.md", Notes: "Read the MCP transport and tool contract"},
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
			Operation:   "Read MCP documentation",
			Method:      "GET",
			Path:        "/api/docs.md",
			Category:    "docs",
			Description: "Return the full REST API document in Markdown format.",
			Response:    map[string]any{"contentType": "text/markdown", "body": "markdown document"},
			Example:     fmt.Sprintf("curl %s/api/docs.md", baseURL),
		},
		{
			Operation:   "Check Server Status",
			Method:      "GET",
			Path:        "/api/status",
			Category:    "docs",
			Description: "Return REST API server runtime status.",
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
			Description: "Create or update a saved site. `protocol` is the saved site family. SFTP requires a password or ppkPath; supply id when updating an existing site.",
			Request:     restSiteRequestExample("sftp"),
			Response:    map[string]any{"sites": []string{"updated site list"}},
			Example:     fmt.Sprintf("curl -X POST %s/api/sites -H 'Content-Type: application/json' -d '{\"protocol\":\"sftp\",\"host\":\"example.com\",\"port\":22,\"username\":\"deploy\",\"password\":\"<SITE_PASSWORD>\",\"localPath\":\"/Users/demo/project\",\"remotePath\":\"/srv/app\"}'", baseURL),
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
			Request:     map[string]any{"site": restSiteRequestExample("sftp")},
			Response:    map[string]any{"tabs": []string{"updated tab list"}},
			Example:     fmt.Sprintf("curl -X POST %s/api/tabs/file -H 'Content-Type: application/json' -d '{\"site\":{\"protocol\":\"sftp\",\"host\":\"example.com\",\"port\":22,\"username\":\"deploy\",\"password\":\"<SITE_PASSWORD>\",\"localPath\":\"/Users/demo/project\",\"remotePath\":\"/srv/app\"}}'", baseURL),
		},
		{
			Operation:   "Open SSH Session",
			Method:      "POST",
			Path:        "/api/tabs/ssh",
			Category:    "terminal",
			Description: "Open an SSH terminal tab for interactive command control. This is valid for saved sites whose protocol family is `sftp`.",
			Request:     map[string]any{"site": restSiteRequestExample("sftp")},
			Response:    map[string]any{"tabs": []string{"updated tab list"}, "sessionId": "terminal session id"},
			Example:     fmt.Sprintf("curl -X POST %s/api/tabs/ssh -H 'Content-Type: application/json' -d '{\"site\":{\"protocol\":\"sftp\",\"host\":\"example.com\",\"port\":22,\"username\":\"deploy\",\"password\":\"<SITE_PASSWORD>\",\"localPath\":\"/Users/demo/project\",\"remotePath\":\"/srv/app\"}}'", baseURL),
		},
		{
			Operation:   "Open Telnet Session",
			Method:      "POST",
			Path:        "/api/tabs/telnet",
			Category:    "terminal",
			Description: "Open a Telnet terminal tab. This is valid for saved sites whose protocol family is `ftp`.",
			Request:     map[string]any{"site": restTelnetRequestExample()},
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
			Request:     map[string]any{"tabId": "tab-123", "path": "/Users/demo/project"},
			Response:    map[string]any{"entries": []string{"file entries"}},
			Example:     fmt.Sprintf("curl '%s/api/files/local?tabId=tab-123&path=/Users/demo/project'", baseURL),
		},
		{
			Operation:   "List Remote Files",
			Method:      "GET",
			Path:        "/api/files/remote",
			Category:    "sftp",
			Description: "List remote files for an active SFTP/FTP tab and path.",
			Request:     map[string]any{"tabId": "tab-123", "path": "/srv/app"},
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
			Request:     map[string]any{"id": "operation-123"},
			Response:    map[string]any{"operation": map[string]any{"id": "operation-id", "kind": "upload", "status": "running|done|failed", "error": "present when failed"}},
			Example:     fmt.Sprintf("curl %s/api/operations/operation-id", baseURL),
		},
		{
			Operation:   "Stat Remote Path",
			Method:      "GET",
			Path:        "/api/sftp/stat",
			Category:    "sftp",
			Description: "Read a single remote entry metadata by path.",
			Request:     map[string]any{"tabId": "tab-123", "path": "/srv/app/config.yml"},
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
			Request:     map[string]any{"site": restSiteRequestExample("sftp"), "command": "pwd", "timeoutSeconds": 10},
			Response:    map[string]any{"ok": true, "stdout": "/srv/app\n", "stderr": "", "exitCode": 0},
			Example:     fmt.Sprintf("curl -X POST %s/api/ssh/execute -H 'Content-Type: application/json' -d '{\"site\":{\"host\":\"example.com\",\"port\":22,\"username\":\"deploy\",\"protocol\":\"sftp\",\"localPath\":\"/tmp\",\"remotePath\":\"/srv/app\",\"password\":\"<SITE_PASSWORD>\"},\"command\":\"pwd\",\"timeoutSeconds\":10}'", baseURL),
		},
		{
			Operation:   "Read Terminal Output",
			Method:      "GET",
			Path:        "/api/terminal/output",
			Category:    "terminal",
			Description: "Read buffered SSH/local terminal output for a session id.",
			Request:     map[string]any{"sessionId": "session-123"},
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
			Description: "Replace the full config object. First call get_config, edit the returned config, then send every field back. Omitting fields resets them; this is not a partial update.",
			Request:     map[string]any{"config": restConfigRequestExample()},
			Response:    map[string]any{"config": "updated config object"},
			Example:     fmt.Sprintf("curl -X PUT %s/api/config -H 'Content-Type: application/json' --data-binary @config-envelope.json", baseURL),
		},
	}
}

func restSiteRequestExample(protocol string) map[string]any {
	port := 22
	if protocol == "ftp" {
		port = 21
	}
	return map[string]any{
		"id": "site-123", "name": "Example", "protocol": protocol,
		"host": "example.invalid", "port": port, "username": "demo",
		"password": "<SITE_PASSWORD>", "ppkPath": "", "ppkPassphrase": "",
		"localPath": "/Users/demo", "remotePath": "/srv/app",
	}
}

func restTelnetRequestExample() map[string]any {
	site := restSiteRequestExample("ftp")
	site["port"] = 23
	return site
}

func restConfigRequestExample() map[string]any {
	return map[string]any{
		"windowWidth": 1280, "windowHeight": 800, "windowX": 0, "windowY": 0,
		"proUnlock": false, "lastActiveTab": "", "restoreTabsOnStart": true,
		"closeTerminalTabOnDisconnect": false, "showHiddenFiles": false,
		"showTrayIcon": false, "rememberWindowPosition": true, "telnetLocalEcho": false,
		"restServerEnabled": false, "restServerPort": 18080,
		"fontScale": "medium", "language": "en", "theme": "neutral", "siteFolders": []string{},
	}
}

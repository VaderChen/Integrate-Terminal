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
	builder.WriteString("IntegTERM provides a local VFS MCP contract over stdio and an optional standard Model Context Protocol server over Streamable HTTP. Network MCP clients discover and call tools through the endpoint below; clients connected to the same running endpoint share that service's RAM workspace. No API token or custom authentication header is required.\n\n")
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
	builder.WriteString("- Use this optional HTTP service when multiple agents must share one server-side RAM workspace; every participating agent must connect to this same running endpoint.\n")
	builder.WriteString("- Call `vfs_connect` with a saved-site URI before remote operations, or let the first remote `vfs_list`, `vfs_stat`, `vfs_read`, `vfs_write`, `vfs_write_chunk`, `vfs_mkdir`, `vfs_rename`, or `vfs_delete` call connect lazily.\n")
	builder.WriteString("- Paths outside the `sites` namespace remain bounded RAM files and are cleared when the background service stops.\n\n")
	writeMCPVFSAgentGuide(&builder)
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
	stdioExecutable := a.GetMCPStdioExecutable()
	stdioConfig := map[string]any{
		"mcpServers": map[string]any{
			"integterm-vfs": map[string]any{
				"command": stdioExecutable,
				"args":    []string{"mcp"},
			},
		},
	}
	stdioConfigJSON, err := json.MarshalIndent(stdioConfig, "", "  ")
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("# IntegTERM Virtual Workspace Contract\n\n")
	builder.WriteString("This contract defines a virtual filesystem spanning bounded RAM paths and saved remote-site mounts. Local MCP clients should start the compiled application with the `mcp` argument and communicate over stdio. This runs a headless MCP server; it does not require the source tree or open the desktop UI. Each stdio client owns an independent MCP process and RAM workspace, not a shared file on disk. The `integterm-vfs` URI identifies resources inside that MCP connection; it is not a command or network endpoint.\n\n")
	builder.WriteString("## Local MCP Connection\n\n")
	builder.WriteString("- Transport: `stdio`\n")
	builder.WriteString("- Executable: `" + stdioExecutable + "`\n")
	builder.WriteString("- Argument: `mcp`\n")
	builder.WriteString("- Development command: `go run . mcp`\n\n")
	builder.WriteString("Use this MCP client configuration (the command path is resolved from the running app):\n\n")
	builder.WriteString("```json\n")
	builder.WriteString(string(stdioConfigJSON))
	builder.WriteString("\n```\n\n")
	builder.WriteString("## Virtual Workspace\n\n")
	builder.WriteString("- Virtual root URI: `" + mcpVFSRootURI + "`\n")
	builder.WriteString("- Local VFS MCP: `available through the stdio command above`\n")
	builder.WriteString(fmt.Sprintf("- HTTP MCP server: `%t`\n", status.Enabled))
	builder.WriteString("- RAM paths: any path outside `sites`; data belongs to this stdio MCP process and is cleared when the process stops\n")
	builder.WriteString("- Saved sites namespace: `" + mcpVFSRootURI + "/sites/{siteID}`\n")
	builder.WriteString("- Remote paths: descendants of a saved-site URI, resolved relative to that site's configured remote root\n")
	builder.WriteString("- Shared RAM option: enable Streamable HTTP only when multiple agents must connect to the same server-side workspace\n\n")
	writeMCPVFSAgentGuide(&builder)

	builder.WriteString("## Available Tools\n\n")
	builder.WriteString("| Tool | Description |\n")
	builder.WriteString("| --- | --- |\n")
	for _, tool := range buildMCPVFSToolDocs() {
		builder.WriteString(fmt.Sprintf("| `%s` | %s |\n", tool.Name, tool.Description))
	}

	builder.WriteString("\n## Resources\n\n")
	builder.WriteString("- Root resource: `" + mcpVFSRootURI + "`\n")
	builder.WriteString("- File resource template: `integterm-vfs://workspace/mcp/{+path}` (`+` allows nested paths)\n")
	builder.WriteString("- Saved site root: `integterm-vfs://workspace/mcp/sites/{siteID}`\n")
	builder.WriteString("- Remote file: `integterm-vfs://workspace/mcp/sites/{siteID}/{relativeRemotePath}`\n")
	builder.WriteString("- Use the virtual URI returned by `vfs_list`, `vfs_stat`, and `vfs_read`; remote URIs never expose credentials.\n\n")

	builder.WriteString("## Usage Rules\n\n")
	builder.WriteString("- Connect the MCP client through stdio using the `mcp` command before using any virtual URI; the URI is not itself a transport.\n")
	builder.WriteString("- Do not expect RAM paths from one stdio MCP process to appear in another process; use one shared Streamable HTTP endpoint when cross-agent RAM sharing is required.\n")
	builder.WriteString("- Call `tools/list`, then call `vfs_list` with an empty path or the root URI to inspect the workspace.\n")
	builder.WriteString("- Call `vfs_list` on `sites` to discover saved site IDs without exposing passwords.\n")
	builder.WriteString("- Call `vfs_connect` with a saved-site URI, or let the first remote VFS operation connect lazily.\n")
	builder.WriteString("- Use relative virtual paths or `integterm-vfs://workspace/mcp/...` URIs; cross-site rename is rejected.\n")
	builder.WriteString("- Inline `vfs_write` calls are bounded to 4 MiB; use verified `vfs_write_chunk` calls for files up to 32 MiB and network transfer tools beyond that limit.\n")
	builder.WriteString("- SSH and Telnet terminal sessions remain explicit network tools because they are streams rather than filesystem resources.\n")

	return builder.String(), nil
}

func writeMCPVFSAgentGuide(builder *strings.Builder) {
	builder.WriteString("## VFS Agent Quick Start\n\n")
	builder.WriteString("The MCP `initialize` response contains the canonical VFS instructions, while `tools/list` contains the current input and output schemas. These two protocol responses are sufficient for operation; an Agent does not need to inspect IntegTERM source code.\n\n")
	builder.WriteString("1. Call `vfs_workspace_info` with an empty argument object.\n")
	builder.WriteString("2. Call `vfs_list` with `{}` to list the workspace root.\n")
	builder.WriteString("3. For RAM work, use a normal relative path such as `notes/todo.txt`.\n")
	builder.WriteString("4. For remote work, list `sites`, reuse a returned `siteID` path or URI, and optionally call `vfs_connect`. Remote file tools also connect lazily.\n")
	builder.WriteString("5. Reuse the `path` or `uri` returned by VFS tools instead of constructing host paths or guessing identifiers.\n\n")

	builder.WriteString("### Path Model\n\n")
	builder.WriteString("| Path passed to a VFS tool | Meaning |\n")
	builder.WriteString("| --- | --- |\n")
	builder.WriteString("| omitted, empty, or `" + mcpVFSRootURI + "` | RAM workspace root |\n")
	builder.WriteString("| `notes/file.txt` | RAM file or directory |\n")
	builder.WriteString("| `sites` | Saved remote-site list |\n")
	builder.WriteString("| `sites/{siteID}` | Saved site's configured remote root |\n")
	builder.WriteString("| `sites/{siteID}/{relativePath}` | Remote file or directory below that root |\n\n")
	builder.WriteString("The `integterm-vfs://` URI is carried inside an established MCP connection. It is not an HTTP URL, shell command, absolute local path, or absolute remote path.\n\n")

	builder.WriteString("### `tools/call` Parameter Examples\n\n")
	builder.WriteString("Discover the contract and root:\n\n")
	builder.WriteString("```json\n")
	builder.WriteString("{\"name\":\"vfs_workspace_info\",\"arguments\":{}}\n")
	builder.WriteString("{\"name\":\"vfs_list\",\"arguments\":{}}\n")
	builder.WriteString("```\n\n")
	builder.WriteString("Create and read a RAM file:\n\n")
	builder.WriteString("```json\n")
	builder.WriteString("{\"name\":\"vfs_write\",\"arguments\":{\"path\":\"notes/todo.txt\",\"content\":\"hello\",\"encoding\":\"utf-8\"}}\n")
	builder.WriteString("{\"name\":\"vfs_read\",\"arguments\":{\"path\":\"notes/todo.txt\"}}\n")
	builder.WriteString("```\n\n")
	builder.WriteString("Write a larger file in verified sequential chunks (SHA-256 shown is for `hello world`):\n\n")
	builder.WriteString("```json\n")
	builder.WriteString("{\"name\":\"vfs_write_chunk\",\"arguments\":{\"path\":\"artifacts/example.bin\",\"offset\":0,\"content\":\"hello \",\"encoding\":\"utf-8\",\"final\":false}}\n")
	builder.WriteString("{\"name\":\"vfs_write_chunk\",\"arguments\":{\"path\":\"artifacts/example.bin\",\"offset\":6,\"content\":\"world\",\"encoding\":\"utf-8\",\"final\":true,\"sha256\":\"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9\"}}\n")
	builder.WriteString("```\n\n")
	builder.WriteString("Discover and use a remote site (replace `{siteID}` with the returned ID):\n\n")
	builder.WriteString("```json\n")
	builder.WriteString("{\"name\":\"vfs_list\",\"arguments\":{\"path\":\"sites\"}}\n")
	builder.WriteString("{\"name\":\"vfs_connect\",\"arguments\":{\"path\":\"sites/{siteID}\"}}\n")
	builder.WriteString("{\"name\":\"vfs_list\",\"arguments\":{\"path\":\"sites/{siteID}\"}}\n")
	builder.WriteString("{\"name\":\"vfs_read\",\"arguments\":{\"path\":\"sites/{siteID}/config/app.yml\"}}\n")
	builder.WriteString("```\n\n")
	builder.WriteString("Continue a truncated read using `offset + returnedBytes` from the previous result:\n\n")
	builder.WriteString("```json\n")
	builder.WriteString("{\"name\":\"vfs_read\",\"arguments\":{\"path\":\"notes/large.txt\",\"offset\":262144,\"limit\":262144}}\n")
	builder.WriteString("```\n\n")

	builder.WriteString("### Operation Rules and Troubleshooting\n\n")
	builder.WriteString("- `vfs_write` defaults to UTF-8 and accepts one decoded payload up to 4 MiB. Set `encoding` to `base64` for binary data and `overwrite` to `true` when replacing an existing file.\n")
	builder.WriteString("- `vfs_write_chunk` accepts decoded chunks up to 1 MiB and completed files up to 32 MiB. Start at offset 0, reuse each returned `nextOffset`, and provide the full decoded file SHA-256 with `final: true`. A failed hash discards the staged write.\n")
	builder.WriteString("- `vfs_delete` requires `recursive: true` for a non-empty directory. The workspace root, `sites`, and saved-site roots cannot be deleted.\n")
	builder.WriteString("- `vfs_rename` works only within RAM or within one saved-site mount. It cannot cross RAM and remote storage or cross sites.\n")
	builder.WriteString(fmt.Sprintf("- The RAM workspace is limited to %d bytes, one inline write to %d bytes, one chunked file to %d bytes, and one `vfs_read` or write-chunk payload to %d bytes. The default read size is %d bytes.\n", mcpVFSTotalSize, mcpVFSMaxFileSize, mcpVFSMaxChunkedFile, mcpVFSMaxReadSize, mcpVFSDefaultReadSize))
	builder.WriteString("- If a path is rejected, call `vfs_list` again and reuse a returned relative `path` or full `uri`. Do not substitute a host filesystem path.\n")
	builder.WriteString("- If a remote site is unavailable, list `sites` to verify the saved ID, then call `vfs_connect` to surface the connection error explicitly.\n")
	builder.WriteString("- Use `resources/read` only for a known file URI. Use `vfs_read` for chunking, files over 1 MiB, and explicit encoding metadata.\n\n")
}

func buildMCPVFSToolDocs() []mcpToolDoc {
	return []mcpToolDoc{
		{Name: "vfs_workspace_info", Description: "First VFS call. Returns the root URI, complete path model, next discovery call, RAM limits, saved-site count, and active mount count."},
		{Name: "vfs_list", Description: "List immediate children. Use `{}` for the root, `sites` for saved site IDs, or a returned RAM/remote path. Returns namespace kind and next action."},
		{Name: "vfs_connect", Description: "Explicitly connect a saved-site path returned by `vfs_list`; optional because remote file operations also connect lazily."},
		{Name: "vfs_stat", Description: "Read normalized metadata for one known RAM or remote file or directory."},
		{Name: "vfs_read", Description: "Read UTF-8 or base64 content in chunks; continue at `offset + returnedBytes` while `truncated` is true."},
		{Name: "vfs_write", Description: "Create one UTF-8 or base64 payload up to 4 MiB; an existing destination requires `overwrite: true`."},
		{Name: "vfs_write_chunk", Description: "Stage sequential chunks up to 1 MiB each and commit a file up to 32 MiB after final SHA-256 verification."},
		{Name: "vfs_mkdir", Description: "Create a RAM or remote directory; RAM parent directories are created automatically."},
		{Name: "vfs_rename", Description: "Rename only within RAM or one saved-site mount; cross-namespace and cross-site moves are rejected."},
		{Name: "vfs_delete", Description: "Delete a file or directory; a non-empty directory requires `recursive: true`."},
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

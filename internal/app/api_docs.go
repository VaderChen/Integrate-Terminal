package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type apiDocsEnvelope struct {
	Version      string
	GeneratedAt  string
	Server       serverDoc
	Skill        skillDoc
	CoreRules    []string
	OperationMap []operationDoc
	Endpoints    []endpointDoc
}

type serverDoc struct {
	Enabled bool
	BaseURL string
	Host    string
	Port    int
	Token   string
}

type skillDoc struct {
	Name        string
	Description string
	Input       map[string]any
	Output      map[string]any
	Examples    []map[string]any
}

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

func (a *App) GetRestAPIDocsMarkdown() (string, error) {
	status := a.GetRESTServerStatus()
	port := status.Port
	if port <= 0 {
		port = defaultRESTServerPort
	}
	baseURL := status.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	doc := apiDocsEnvelope{
		Version:     "1.0",
		GeneratedAt: time.Now().Format(time.RFC3339),
		Server: serverDoc{
			Enabled: status.Enabled,
			BaseURL: baseURL,
			Host:    "127.0.0.1",
			Port:    port,
			Token:   a.GetRESTServerToken(),
		},
		Skill: skillDoc{
			Name:        "integterm-rest-skill",
			Description: "Use the local IntegTerm REST API server to automate SSH terminals, Telnet terminals, SFTP/FTP file tabs, local terminal tabs, transfer state, and configuration.",
			Input: map[string]any{
				"transport":   "http",
				"auth":        "bearer",
				"token":       a.GetRESTServerToken(),
				"contentType": "application/json",
				"host":        "127.0.0.1",
				"port":        port,
			},
			Output: map[string]any{
				"format": "markdown",
				"contains": []string{
					"REST API server metadata",
					"core workflow rules",
					"operation-to-endpoint mapping",
					"request schema hints",
					"response examples",
					"curl examples",
				},
			},
			Examples: []map[string]any{
				{
					"name": "execute_single_ssh_command_from_sftp_site_family",
					"steps": []string{
						"treat site protocol `sftp` as SSH + SFTP capability",
						"GET /api/sites",
						"POST /api/ssh/execute",
						"read stdout/stderr/exitCode",
					},
				},
				{
					"name": "manage_remote_files_from_sftp_site_family",
					"steps": []string{
						"treat site protocol `sftp` as SSH + SFTP capability",
						"GET /api/sites",
						"POST /api/tabs/file",
						"GET /api/files/remote?tabId={tabId}&path=/srv/app",
						"POST /api/sftp/mkdir",
						"POST /api/sftp/rename",
						"POST /api/sftp/delete",
					},
				},
				{
					"name": "open_telnet_terminal_from_ftp_site_family",
					"steps": []string{
						"treat site protocol `ftp` as Telnet + FTP capability",
						"GET /api/sites",
						"POST /api/tabs/telnet",
						"POST /api/terminal/input",
						"GET /api/terminal/output?sessionId={sessionId}",
					},
				},
			},
		},
		CoreRules: []string{
			"Always verify the server first with `GET /api/status` before assuming any endpoint is available.",
			"Send the generated API token with every non-status request using `Authorization: Bearer {token}`.",
			"File upload and download requests return HTTP 202 immediately with an operation id; never wait for the transfer on the original HTTP request.",
			"Poll `GET /api/operations/{id}` until status is `done` or `failed`. Use `GET /api/transfers` only for detailed live progress.",
			"Treat operation names in this file as logical workflow names, not shell commands.",
			"When calling an endpoint, always build an explicit HTTP request with method, path, query parameters, and JSON body.",
			"Do not replace a required structured object with a display label or alias unless the API explicitly allows it.",
			"Reuse returned `site`, `tabId`, and `sessionId` values exactly as returned by previous API calls.",
			"Use absolute paths for local and remote file operations.",
			"If the target site is not yet known as a full site object, resolve it from `GET /api/sites` first.",
			"Saved site `protocol` is a site family: `sftp` means SSH terminal + SFTP file transfer, and `ftp` means Telnet terminal + FTP file transfer.",
		},
		OperationMap: buildRESTOperationDocs(),
		Endpoints:    buildRESTEndpointDocs(baseURL),
	}

	var builder strings.Builder
	builder.WriteString("---\n")
	builder.WriteString("name: integterm-rest-skill\n")
	builder.WriteString("description: Use the local IntegTerm REST API server to automate SSH terminals, Telnet terminals, SFTP/FTP file tabs, local terminal tabs, transfer state, and configuration.\n")
	builder.WriteString("generated_at: " + doc.GeneratedAt + "\n")
	builder.WriteString("---\n\n")
	builder.WriteString("# integterm-rest-skill\n\n")
	builder.WriteString("Use this skill to operate IntegTerm through its local REST API server.\n\n")
	builder.WriteString("This document is written as an execution-oriented contract for agents. It is not only a human overview. When generating requests, prefer the canonical request templates in this file over ad hoc examples.\n\n")
	builder.WriteString("## Skill Summary\n\n")
	builder.WriteString(doc.Skill.Description + "\n\n")
	builder.WriteString("### Input\n\n")
	builder.WriteString("```json\n")
	builder.WriteString(mustJSONIndent(doc.Skill.Input))
	builder.WriteString("\n```\n\n")
	builder.WriteString("### Output\n\n")
	builder.WriteString("```json\n")
	builder.WriteString(mustJSONIndent(doc.Skill.Output))
	builder.WriteString("\n```\n\n")

	builder.WriteString("## Installation\n\n")
	builder.WriteString("- Server base URL: `" + doc.Server.BaseURL + "`\n")
	builder.WriteString("- Server host: `" + doc.Server.Host + "`\n")
	builder.WriteString(fmt.Sprintf("- Server port: `%d`\n", doc.Server.Port))
	builder.WriteString(fmt.Sprintf("- Server enabled: `%t`\n", doc.Server.Enabled))
	builder.WriteString("- Authorization: `Bearer " + doc.Server.Token + "`\n\n")

	builder.WriteString("## Core Rules\n\n")
	for index, rule := range doc.CoreRules {
		builder.WriteString(fmt.Sprintf("%d. %s\n", index+1, rule))
	}
	builder.WriteString("\n")

	builder.WriteString("## Data Model Rules\n\n")
	builder.WriteString("### Site Object\n\n")
	builder.WriteString("Many endpoints require a full `site` object, not only a site name or alias.\n\n")
	builder.WriteString("Use this rule unless an endpoint explicitly accepts an `id` path parameter:\n\n")
	builder.WriteString("- Allowed source for `site`:\n")
	builder.WriteString("  - a full site object returned by `GET /api/sites`\n")
	builder.WriteString("  - a full site object already stored from a previous successful step\n")
	builder.WriteString("- Not sufficient by itself:\n")
	builder.WriteString("  - site display name\n")
	builder.WriteString("  - alias\n")
	builder.WriteString("  - label\n")
	builder.WriteString("  - guessed host\n\n")

	builder.WriteString("### Session and Tab Identifiers\n\n")
	builder.WriteString("- `sessionId` is produced by terminal-opening endpoints such as `POST /api/tabs/ssh`, `POST /api/tabs/telnet`, and `POST /api/tabs/local`.\n")
	builder.WriteString("- `tabId` is produced by file-tab or tab-list endpoints such as `POST /api/tabs/file` and `GET /api/tabs`.\n")
	builder.WriteString("- Do not invent `sessionId` or `tabId`.\n\n")

	builder.WriteString("## Operation Map\n\n")
	builder.WriteString("| Logical Operation | Method | Path | Notes |\n")
	builder.WriteString("| --- | --- | --- | --- |\n")
	for _, operation := range doc.OperationMap {
		builder.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", operation.LogicalOperation, operation.Method, operation.Path, operation.Notes))
	}
	builder.WriteString("\n")

	builder.WriteString("## Canonical Request Templates\n\n")
	for _, endpoint := range doc.Endpoints {
		builder.WriteString("### " + endpoint.Operation + "\n\n")
		builder.WriteString("**Method**\n")
		builder.WriteString("`" + endpoint.Method + "`\n\n")
		builder.WriteString("**Path**\n")
		builder.WriteString("`" + endpoint.Path + "`\n\n")
		builder.WriteString("**Description**\n")
		builder.WriteString(endpoint.Description + "\n\n")
		if len(endpoint.Request) > 0 {
			builder.WriteString("**Request**\n")
			builder.WriteString("```json\n")
			builder.WriteString(mustJSONIndent(endpoint.Request))
			builder.WriteString("\n```\n\n")
		}
		if len(endpoint.Response) > 0 {
			builder.WriteString("**Expected response keys / shape**\n")
			builder.WriteString("```json\n")
			builder.WriteString(mustJSONIndent(endpoint.Response))
			builder.WriteString("\n```\n\n")
		}
		builder.WriteString("**Example**\n")
		builder.WriteString("```bash\n")
		builder.WriteString(addRESTAuthToCurl(endpoint.Example, doc.Server.Token))
		builder.WriteString("\n```\n\n")
	}

	builder.WriteString("## Workflow Examples\n\n")
	for _, example := range doc.Skill.Examples {
		name, _ := example["name"].(string)
		builder.WriteString("### " + name + "\n\n")
		if steps, ok := example["steps"].([]string); ok {
			for index, step := range steps {
				builder.WriteString(fmt.Sprintf("%d. %s\n", index+1, step))
			}
			builder.WriteString("\n")
		}
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
			Description: "Update app config, including REST API server enable flag.",
			Request:     map[string]any{"config": "full config object"},
			Response:    map[string]any{"config": "updated config object"},
			Example:     fmt.Sprintf("curl -X PUT %s/api/config -H 'Content-Type: application/json' -d '{\"config\":{\"restServerEnabled\":true,\"restServerPort\":18080}}'", baseURL),
		},
	}
}

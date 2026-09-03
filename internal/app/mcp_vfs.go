package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	pathpkg "path"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpVFSRootURI         = "integterm-vfs://workspace/mcp"
	mcpVFSRootPath        = "/mcp"
	mcpVFSMaxFileSize     = 4 * 1024 * 1024
	mcpVFSTotalSize       = 32 * 1024 * 1024
	mcpVFSMaxReadSize     = 1024 * 1024
	mcpVFSDefaultReadSize = 256 * 1024
)

type mcpVFSNode struct {
	directory bool
	data      []byte
	modified  time.Time
}

type mcpVFS struct {
	mu           sync.RWMutex
	nodes        map[string]mcpVFSNode
	totalBytes   int64
	remoteMu     sync.Mutex
	remoteMounts map[string]mcpVFSRemoteMount
}

type mcpVFSItem struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	URI      string `json:"uri"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"isDir"`
	Modified string `json:"modified"`
}

type mcpVFSWorkspaceInput struct{}

type mcpVFSWorkspaceOutput struct {
	RootURI          string `json:"rootURI"`
	FileCount        int    `json:"fileCount"`
	DirectoryCount   int    `json:"directoryCount"`
	BytesUsed        int64  `json:"bytesUsed"`
	MaxBytes         int64  `json:"maxBytes"`
	MaxFileBytes     int64  `json:"maxFileBytes"`
	RemoteSiteCount  int    `json:"remoteSiteCount"`
	MountedSiteCount int    `json:"mountedSiteCount"`
}

type mcpVFSPathInput struct {
	Path string `json:"path" jsonschema:"relative virtual path or integterm-vfs URI"`
}

type mcpVFSReadInput struct {
	Path   string `json:"path" jsonschema:"relative virtual path or integterm-vfs URI"`
	Offset int64  `json:"offset,omitempty" jsonschema:"zero-based byte offset"`
	Limit  int64  `json:"limit,omitempty" jsonschema:"maximum bytes to return; defaults to 262144"`
}

type mcpVFSReadOutput struct {
	Path          string `json:"path"`
	URI           string `json:"uri"`
	Content       string `json:"content"`
	Encoding      string `json:"encoding"`
	Offset        int64  `json:"offset"`
	Size          int64  `json:"size"`
	ReturnedBytes int64  `json:"returnedBytes"`
	Truncated     bool   `json:"truncated"`
}

type mcpVFSWriteInput struct {
	Path      string `json:"path" jsonschema:"relative virtual path or integterm-vfs URI"`
	Content   string `json:"content" jsonschema:"UTF-8 text or base64 data"`
	Encoding  string `json:"encoding,omitempty" jsonschema:"utf-8 or base64; defaults to utf-8"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"replace an existing file when true"`
}

type mcpVFSMkdirInput struct {
	Path string `json:"path" jsonschema:"relative virtual directory path or integterm-vfs URI"`
}

type mcpVFSDeleteInput struct {
	Path      string `json:"path" jsonschema:"relative virtual path or integterm-vfs URI"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"delete directory contents when true"`
}

type mcpVFSRenameInput struct {
	OldPath string `json:"oldPath" jsonschema:"current relative virtual path or integterm-vfs URI"`
	NewPath string `json:"newPath" jsonschema:"new relative virtual path or integterm-vfs URI"`
}

type mcpVFSConnectOutput struct {
	SiteID     string `json:"siteId"`
	SiteName   string `json:"siteName"`
	Protocol   string `json:"protocol"`
	URI        string `json:"uri"`
	RemoteRoot string `json:"remoteRoot"`
	Connected  bool   `json:"connected"`
}

type mcpVFSDeleteOutput struct {
	Path    string `json:"path"`
	URI     string `json:"uri"`
	Deleted bool   `json:"deleted"`
}

func newMCPVFS() *mcpVFS {
	now := time.Now().UTC()
	return &mcpVFS{
		nodes: map[string]mcpVFSNode{
			"": {directory: true, modified: now},
		},
		remoteMounts: make(map[string]mcpVFSRemoteMount),
	}
}

func (vfs *mcpVFS) workspaceInfo() mcpVFSWorkspaceOutput {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	fileCount := 0
	directoryCount := 0
	for _, node := range vfs.nodes {
		if node.directory {
			directoryCount++
		} else {
			fileCount++
		}
	}
	return mcpVFSWorkspaceOutput{
		RootURI:        mcpVFSRootURI,
		FileCount:      fileCount,
		DirectoryCount: directoryCount,
		BytesUsed:      vfs.totalBytes,
		MaxBytes:       mcpVFSTotalSize,
		MaxFileBytes:   mcpVFSMaxFileSize,
	}
}

func (vfs *mcpVFS) list(path string) ([]mcpVFSItem, error) {
	relativePath, err := normalizeMCPVFSPath(path)
	if err != nil {
		return nil, err
	}

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()
	node, ok := vfs.nodes[relativePath]
	if !ok {
		return nil, fmt.Errorf("virtual path not found: %s", path)
	}
	if !node.directory {
		return nil, fmt.Errorf("virtual path is not a directory: %s", path)
	}

	children := make(map[string]mcpVFSNode)
	for nodePath, child := range vfs.nodes {
		if nodePath == relativePath || mcpVFSParent(nodePath) != relativePath {
			continue
		}
		children[nodePath] = child
	}

	items := make([]mcpVFSItem, 0, len(children))
	for nodePath, child := range children {
		items = append(items, mcpVFSItemFromNode(nodePath, child))
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].IsDir != items[right].IsDir {
			return items[left].IsDir
		}
		return items[left].Name < items[right].Name
	})
	return items, nil
}

func (vfs *mcpVFS) stat(path string) (mcpVFSItem, error) {
	relativePath, err := normalizeMCPVFSPath(path)
	if err != nil {
		return mcpVFSItem{}, err
	}

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()
	node, ok := vfs.nodes[relativePath]
	if !ok {
		return mcpVFSItem{}, fmt.Errorf("virtual path not found: %s", path)
	}
	return mcpVFSItemFromNode(relativePath, node), nil
}

func (vfs *mcpVFS) read(path string, offset, limit int64) ([]byte, mcpVFSItem, bool, error) {
	relativePath, err := normalizeMCPVFSPath(path)
	if err != nil {
		return nil, mcpVFSItem{}, false, err
	}
	if offset < 0 {
		return nil, mcpVFSItem{}, false, fmt.Errorf("offset must not be negative")
	}
	if limit < 0 {
		return nil, mcpVFSItem{}, false, fmt.Errorf("limit must not be negative")
	}
	if limit == 0 {
		limit = mcpVFSDefaultReadSize
	}
	if limit > mcpVFSMaxReadSize {
		return nil, mcpVFSItem{}, false, fmt.Errorf("limit exceeds %d bytes", mcpVFSMaxReadSize)
	}

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()
	node, ok := vfs.nodes[relativePath]
	if !ok {
		return nil, mcpVFSItem{}, false, fmt.Errorf("virtual file not found: %s", path)
	}
	if node.directory {
		return nil, mcpVFSItem{}, false, fmt.Errorf("virtual path is a directory: %s", path)
	}

	item := mcpVFSItemFromNode(relativePath, node)
	if offset >= int64(len(node.data)) {
		return []byte{}, item, false, nil
	}
	end := offset + limit
	if end > int64(len(node.data)) {
		end = int64(len(node.data))
	}
	data := append([]byte(nil), node.data[offset:end]...)
	return data, item, end < int64(len(node.data)), nil
}

func (vfs *mcpVFS) write(path, content, encoding string, overwrite bool) (mcpVFSItem, error) {
	relativePath, err := normalizeMCPVFSPath(path)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if relativePath == "" {
		return mcpVFSItem{}, fmt.Errorf("cannot write the virtual workspace root")
	}

	data, err := decodeMCPVFSContent(content, encoding)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if int64(len(data)) > mcpVFSMaxFileSize {
		return mcpVFSItem{}, fmt.Errorf("file exceeds %d bytes", mcpVFSMaxFileSize)
	}

	vfs.mu.Lock()
	defer vfs.mu.Unlock()
	existingBytes := int64(0)
	if existing, ok := vfs.nodes[relativePath]; ok {
		if existing.directory {
			return mcpVFSItem{}, fmt.Errorf("virtual path is a directory: %s", path)
		}
		if !overwrite {
			return mcpVFSItem{}, fmt.Errorf("virtual file already exists: %s", path)
		}
		existingBytes = int64(len(existing.data))
	}
	if vfs.totalBytes-existingBytes+int64(len(data)) > mcpVFSTotalSize {
		return mcpVFSItem{}, fmt.Errorf("workspace exceeds %d bytes", mcpVFSTotalSize)
	}
	vfs.totalBytes -= existingBytes
	vfs.ensureDirectoryLocked(mcpVFSParent(relativePath))
	now := time.Now().UTC()
	vfs.nodes[relativePath] = mcpVFSNode{data: append([]byte(nil), data...), modified: now}
	vfs.totalBytes += int64(len(data))
	return mcpVFSItemFromNode(relativePath, vfs.nodes[relativePath]), nil
}

func (vfs *mcpVFS) mkdir(path string) (mcpVFSItem, error) {
	relativePath, err := normalizeMCPVFSPath(path)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if relativePath == "" {
		return mcpVFSItemFromNode("", mcpVFSNode{directory: true}), nil
	}

	vfs.mu.Lock()
	defer vfs.mu.Unlock()
	if node, ok := vfs.nodes[relativePath]; ok {
		if !node.directory {
			return mcpVFSItem{}, fmt.Errorf("virtual path is a file: %s", path)
		}
		return mcpVFSItemFromNode(relativePath, node), nil
	}
	vfs.ensureDirectoryLocked(relativePath)
	return mcpVFSItemFromNode(relativePath, vfs.nodes[relativePath]), nil
}

func (vfs *mcpVFS) delete(path string, recursive bool) (mcpVFSDeleteOutput, error) {
	relativePath, err := normalizeMCPVFSPath(path)
	if err != nil {
		return mcpVFSDeleteOutput{}, err
	}
	if relativePath == "" {
		return mcpVFSDeleteOutput{}, fmt.Errorf("cannot delete the virtual workspace root")
	}

	vfs.mu.Lock()
	defer vfs.mu.Unlock()
	_, ok := vfs.nodes[relativePath]
	if !ok {
		return mcpVFSDeleteOutput{}, fmt.Errorf("virtual path not found: %s", path)
	}
	prefix := relativePath + "/"
	for nodePath := range vfs.nodes {
		if nodePath != relativePath && !strings.HasPrefix(nodePath, prefix) {
			continue
		}
		if nodePath != relativePath && !recursive {
			return mcpVFSDeleteOutput{}, fmt.Errorf("directory is not empty: %s", path)
		}
	}
	for nodePath, child := range vfs.nodes {
		if nodePath != relativePath && !strings.HasPrefix(nodePath, prefix) {
			continue
		}
		if !child.directory {
			vfs.totalBytes -= int64(len(child.data))
		}
		delete(vfs.nodes, nodePath)
	}
	return mcpVFSDeleteOutput{Path: relativePath, URI: mcpVFSURI(relativePath), Deleted: true}, nil
}

func (vfs *mcpVFS) rename(oldPath string, newPath string) (mcpVFSItem, error) {
	oldRelativePath, err := normalizeMCPVFSPath(oldPath)
	if err != nil {
		return mcpVFSItem{}, err
	}
	newRelativePath, err := normalizeMCPVFSPath(newPath)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if oldRelativePath == "" || newRelativePath == "" {
		return mcpVFSItem{}, fmt.Errorf("cannot rename the virtual workspace root")
	}
	if oldRelativePath == newRelativePath {
		return vfs.stat(oldRelativePath)
	}

	vfs.mu.Lock()
	defer vfs.mu.Unlock()
	source, ok := vfs.nodes[oldRelativePath]
	if !ok {
		return mcpVFSItem{}, fmt.Errorf("virtual path not found: %s", oldPath)
	}
	if _, exists := vfs.nodes[newRelativePath]; exists {
		return mcpVFSItem{}, fmt.Errorf("virtual destination already exists: %s", newPath)
	}
	if source.directory && strings.HasPrefix(newRelativePath, oldRelativePath+"/") {
		return mcpVFSItem{}, fmt.Errorf("cannot move a virtual directory into itself")
	}

	vfs.ensureDirectoryLocked(mcpVFSParent(newRelativePath))
	moved := make(map[string]mcpVFSNode)
	prefix := oldRelativePath + "/"
	for nodePath, node := range vfs.nodes {
		if nodePath == oldRelativePath {
			moved[newRelativePath] = node
			continue
		}
		if strings.HasPrefix(nodePath, prefix) {
			moved[newRelativePath+strings.TrimPrefix(nodePath, oldRelativePath)] = node
		}
	}
	for nodePath := range vfs.nodes {
		if nodePath == oldRelativePath || strings.HasPrefix(nodePath, prefix) {
			delete(vfs.nodes, nodePath)
		}
	}
	for nodePath, node := range moved {
		vfs.nodes[nodePath] = node
	}
	return mcpVFSItemFromNode(newRelativePath, vfs.nodes[newRelativePath]), nil
}

func (vfs *mcpVFS) ensureDirectoryLocked(path string) {
	if path == "" {
		return
	}
	parts := strings.Split(path, "/")
	current := ""
	for _, part := range parts {
		if current == "" {
			current = part
		} else {
			current += "/" + part
		}
		if _, exists := vfs.nodes[current]; !exists {
			vfs.nodes[current] = mcpVFSNode{directory: true, modified: time.Now().UTC()}
		}
	}
}

func addMCPVFSFeatures(server *mcp.Server, layer *mcpVirtualLayer) {
	vfs := layer.vfs
	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_workspace_info",
		Title:       "Get virtual workspace",
		Description: "Return the virtual workspace URI, RAM usage, remote site count, and limits. RAM paths are not backed by the host filesystem and are cleared when the service stops.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ mcpVFSWorkspaceInput) (*mcp.CallToolResult, mcpVFSWorkspaceOutput, error) {
		info := vfs.workspaceInfo()
		info.MountedSiteCount = vfs.mountedRemoteSiteCount()
		info.RemoteSiteCount = layer.mcpRemoteSiteCount()
		return nil, info, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_list",
		Title:       "List virtual entries",
		Description: "List immediate children of a RAM directory, the saved remote sites namespace, or a mounted remote directory. Use a relative path or an integterm-vfs URI; an empty path lists the workspace root.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSPathInput) (*mcp.CallToolResult, map[string]any, error) {
		entries, err := layer.listVirtual(input.Path)
		if err != nil {
			return nil, nil, err
		}
		location, err := parseMCPVFSLocation(input.Path)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"path": location.path, "uri": mcpVFSURI(location.path), "entries": entries}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_stat",
		Title:       "Inspect virtual entry",
		Description: "Return metadata for one RAM or remote virtual file or directory. Remote paths are resolved through the saved site mount in the URI.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSPathInput) (*mcp.CallToolResult, mcpVFSItem, error) {
		item, err := layer.statVirtual(input.Path)
		return nil, item, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_read",
		Title:       "Read virtual file",
		Description: "Read a bounded byte range from a RAM or remote virtual file. UTF-8 data is returned as text; binary data is returned as base64. Use offset and limit for chunks.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSReadInput) (*mcp.CallToolResult, mcpVFSReadOutput, error) {
		data, item, truncated, err := layer.readVirtual(input.Path, input.Offset, input.Limit)
		if err != nil {
			return nil, mcpVFSReadOutput{}, err
		}
		content, encoding := encodeMCPVFSContent(data)
		return nil, mcpVFSReadOutput{
			Path:          item.Path,
			URI:           item.URI,
			Content:       content,
			Encoding:      encoding,
			Offset:        input.Offset,
			Size:          item.Size,
			ReturnedBytes: int64(len(data)),
			Truncated:     truncated,
		}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_write",
		Title:       "Write virtual file",
		Description: "Write UTF-8 or base64 content to a RAM or remote virtual file. RAM files are kept in memory; remote files are written through the saved site's connection.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSWriteInput) (*mcp.CallToolResult, mcpVFSItem, error) {
		item, err := layer.writeVirtual(input.Path, input.Content, input.Encoding, input.Overwrite)
		return nil, item, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_mkdir",
		Title:       "Create virtual directory",
		Description: "Create a RAM or remote virtual directory. RAM parent directories are created automatically; remote directories use the saved site's connection.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSMkdirInput) (*mcp.CallToolResult, mcpVFSItem, error) {
		item, err := layer.mkdirVirtual(input.Path)
		return nil, item, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_delete",
		Title:       "Delete virtual entry",
		Description: "Delete a RAM or remote virtual file or directory. Set recursive to true to remove a remote directory tree; RAM directories follow the same rule.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSDeleteInput) (*mcp.CallToolResult, mcpVFSDeleteOutput, error) {
		result, err := layer.deleteVirtual(input.Path, input.Recursive)
		return nil, result, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_rename",
		Title:       "Rename virtual entry",
		Description: "Rename a RAM or remote virtual file or directory within the same namespace and saved-site mount.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSRenameInput) (*mcp.CallToolResult, mcpVFSItem, error) {
		item, err := layer.renameVirtual(input.OldPath, input.NewPath)
		return nil, item, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_connect",
		Title:       "Connect virtual remote site",
		Description: "Connect a saved remote site identified by integterm-vfs://workspace/mcp/sites/{siteID}. The connection is reused for subsequent virtual operations.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSPathInput) (*mcp.CallToolResult, mcpVFSConnectOutput, error) {
		output, err := layer.connectVirtualSite(input.Path)
		return nil, output, err
	})

	server.AddResource(&mcp.Resource{
		URI:         mcpVFSRootURI,
		Name:        "IntegTERM virtual workspace",
		Title:       "IntegTERM virtual workspace",
		Description: "The root URI of IntegTERM's RAM and saved remote-site virtual filesystem.",
		MIMEType:    "text/plain",
	}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: mcpVFSRootURI, MIMEType: "text/plain", Text: "IntegTERM virtual workspace. Use vfs_list to inspect RAM files and saved remote sites."}}}, nil
	})

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "integterm-vfs://workspace/mcp/{path}",
		Name:        "Virtual file",
		Title:       "Virtual file",
		Description: "Read a RAM or saved remote-site file by its virtual URI.",
	}, func(_ context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		data, item, _, err := layer.readVirtual(request.Params.URI, 0, mcpVFSMaxReadSize)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, mcp.ResourceNotFoundError(request.Params.URI)
			}
			return nil, err
		}
		if item.Size > mcpVFSMaxReadSize {
			return nil, fmt.Errorf("resource exceeds %d bytes; use vfs_read with chunks", mcpVFSMaxReadSize)
		}
		contents := &mcp.ResourceContents{URI: item.URI}
		if utf8.Valid(data) {
			contents.MIMEType = "text/plain; charset=utf-8"
			contents.Text = string(data)
		} else {
			contents.MIMEType = "application/octet-stream"
			contents.Blob = data
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{contents}}, nil
	})
}

func decodeMCPVFSContent(content, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "utf-8", "utf8":
		if !utf8.ValidString(content) {
			return nil, fmt.Errorf("content is not valid UTF-8")
		}
		return []byte(content), nil
	case "base64":
		data, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 content: %w", err)
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q; use utf-8 or base64", encoding)
	}
}

func encodeMCPVFSContent(data []byte) (string, string) {
	if utf8.Valid(data) {
		return string(data), "utf-8"
	}
	return base64.StdEncoding.EncodeToString(data), "base64"
}

func normalizeMCPVFSPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" || value == mcpVFSRootURI || value == mcpVFSRootURI+"/" {
		return "", nil
	}
	if strings.Contains(value, "\x00") {
		return "", fmt.Errorf("virtual path contains a NUL byte")
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", fmt.Errorf("invalid virtual URI: %w", err)
		}
		if !strings.EqualFold(parsed.Scheme, "integterm-vfs") || !strings.EqualFold(parsed.Host, "workspace") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", fmt.Errorf("unsupported virtual URI: %s", value)
		}
		if parsed.Path != mcpVFSRootPath && !strings.HasPrefix(parsed.Path, mcpVFSRootPath+"/") {
			return "", fmt.Errorf("virtual URI must use the MCP workspace root: %s", value)
		}
		value = strings.TrimPrefix(parsed.Path, mcpVFSRootPath)
		value = strings.TrimPrefix(value, "/")
	}
	if strings.Contains(value, "\\") {
		return "", fmt.Errorf("virtual paths must use forward slashes")
	}
	value = strings.Trim(value, "/")
	if value == "" {
		return "", nil
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return "", fmt.Errorf("virtual path cannot contain ..")
		}
	}
	cleaned := pathpkg.Clean(value)
	if cleaned == "." {
		return "", nil
	}
	if strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("virtual path escapes the workspace")
	}
	return cleaned, nil
}

func mcpVFSParent(path string) string {
	index := strings.LastIndexByte(path, '/')
	if index < 0 {
		return ""
	}
	return path[:index]
}

func mcpVFSURI(path string) string {
	if path == "" {
		return mcpVFSRootURI
	}
	return (&url.URL{Scheme: "integterm-vfs", Host: "workspace", Path: mcpVFSRootPath + "/" + path}).String()
}

func mcpVFSItemFromNode(path string, node mcpVFSNode) mcpVFSItem {
	name := path
	if index := strings.LastIndexByte(path, '/'); index >= 0 {
		name = path[index+1:]
	}
	return mcpVFSItem{
		Name:     name,
		Path:     path,
		URI:      mcpVFSURI(path),
		Size:     int64(len(node.data)),
		IsDir:    node.directory,
		Modified: node.modified.Format(time.RFC3339Nano),
	}
}

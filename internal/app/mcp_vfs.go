package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	mcpVFSMaxChunkedFile  = 32 * 1024 * 1024
	mcpVFSMaxWriteChunk   = 1024 * 1024
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
	chunkMu      sync.Mutex
	chunkWrites  map[string]*mcpVFSChunkWrite
	chunkBytes   int64
	remoteMu     sync.Mutex
	remoteMounts map[string]mcpVFSRemoteMount
}

type mcpVFSChunkWrite struct {
	data       []byte
	finalizing bool
}

type mcpVFSItem struct {
	Name     string `json:"name" jsonschema:"entry base name"`
	Path     string `json:"path" jsonschema:"relative VFS path accepted by VFS tools"`
	URI      string `json:"uri" jsonschema:"full integterm-vfs resource URI for this entry"`
	Size     int64  `json:"size" jsonschema:"file size in bytes; directories report zero"`
	IsDir    bool   `json:"isDir" jsonschema:"true for a directory or saved-site root"`
	Modified string `json:"modified" jsonschema:"RFC 3339 modification time when available"`
}

type mcpVFSWorkspaceInput struct{}

type mcpVFSPathModelItem struct {
	Pattern string `json:"pattern" jsonschema:"accepted relative VFS path pattern"`
	Kind    string `json:"kind" jsonschema:"namespace selected by the pattern"`
	Meaning string `json:"meaning" jsonschema:"how VFS tools interpret the path"`
}

type mcpVFSWorkspaceOutput struct {
	RootURI          string                `json:"rootURI" jsonschema:"canonical VFS root resource URI; this is not a transport URL or shell command"`
	TransportNote    string                `json:"transportNote" jsonschema:"how to interpret integterm-vfs URIs inside the MCP connection"`
	FirstCall        string                `json:"firstCall" jsonschema:"next tool call for discovering the workspace root"`
	SitesPath        string                `json:"sitesPath" jsonschema:"path to list saved remote sites"`
	PathModel        []mcpVFSPathModelItem `json:"pathModel" jsonschema:"complete mapping from path forms to VFS namespaces"`
	FileCount        int                   `json:"fileCount" jsonschema:"number of RAM files currently stored"`
	DirectoryCount   int                   `json:"directoryCount" jsonschema:"number of RAM directories including the root"`
	BytesUsed        int64                 `json:"bytesUsed" jsonschema:"RAM file content bytes currently used"`
	MaxBytes         int64                 `json:"maxBytes" jsonschema:"maximum total RAM workspace bytes"`
	MaxFileBytes     int64                 `json:"maxFileBytes" jsonschema:"maximum decoded bytes accepted by one inline vfs_write call"`
	MaxChunkedBytes  int64                 `json:"maxChunkedBytes" jsonschema:"maximum completed file size accepted by vfs_write_chunk"`
	MaxWriteChunk    int64                 `json:"maxWriteChunkBytes" jsonschema:"maximum decoded content bytes accepted by one vfs_write_chunk call"`
	DefaultReadBytes int64                 `json:"defaultReadBytes" jsonschema:"bytes returned by vfs_read when limit is omitted or zero"`
	MaxReadBytes     int64                 `json:"maxReadBytes" jsonschema:"maximum bytes returned by one vfs_read call"`
	RemoteSiteCount  int                   `json:"remoteSiteCount" jsonschema:"number of saved remote sites available under sites"`
	MountedSiteCount int                   `json:"mountedSiteCount" jsonschema:"number of remote sites connected by VFS in this process"`
}

type mcpVFSPathInput struct {
	Path string `json:"path" jsonschema:"required relative VFS path such as notes/file.txt or sites/{siteID}/config/app.yml, or a full URI under integterm-vfs://workspace/mcp; do not pass a host filesystem path or transport URL"`
}

// mcpVFSListInput intentionally makes path optional. An omitted path is the
// canonical way for an MCP client to list the virtual workspace root; keeping
// it optional in the generated schema prevents clients from inventing a host
// filesystem path just to satisfy a required argument.
type mcpVFSListInput struct {
	Path string `json:"path,omitempty" jsonschema:"optional directory path; omit it or use integterm-vfs://workspace/mcp for the root, use sites to discover saved site IDs, or use sites/{siteID}/{relativePath} for a remote directory"`
}

type mcpVFSListOutput struct {
	Path       string       `json:"path" jsonschema:"normalized relative directory path; empty means the workspace root"`
	URI        string       `json:"uri" jsonschema:"full integterm-vfs URI of the listed directory"`
	Kind       string       `json:"kind" jsonschema:"directory namespace: ram, sites, site-root, or remote"`
	Entries    []mcpVFSItem `json:"entries" jsonschema:"immediate child entries; reuse each returned path or URI in later VFS calls"`
	NextAction string       `json:"nextAction" jsonschema:"context-specific instruction for the next safe discovery or file operation"`
}

type mcpVFSReadInput struct {
	Path   string `json:"path" jsonschema:"required RAM or remote file path, for example notes/todo.txt or sites/{siteID}/config/app.yml; full integterm-vfs URIs are also accepted"`
	Offset int64  `json:"offset,omitempty" jsonschema:"zero-based byte offset; use the prior offset plus returnedBytes when truncated is true"`
	Limit  int64  `json:"limit,omitempty" jsonschema:"maximum bytes to return from 1 through 1048576; zero or omitted defaults to 262144"`
}

type mcpVFSReadOutput struct {
	Path          string `json:"path" jsonschema:"normalized relative VFS file path"`
	URI           string `json:"uri" jsonschema:"full integterm-vfs URI of the file"`
	Content       string `json:"content" jsonschema:"UTF-8 text or base64 data according to encoding"`
	Encoding      string `json:"encoding" jsonschema:"utf-8 for valid text or base64 for binary bytes"`
	Offset        int64  `json:"offset" jsonschema:"zero-based byte offset used for this chunk"`
	Size          int64  `json:"size" jsonschema:"total file size in bytes"`
	ReturnedBytes int64  `json:"returnedBytes" jsonschema:"decoded byte count in content for this chunk"`
	Truncated     bool   `json:"truncated" jsonschema:"true when more bytes remain; continue at offset plus returnedBytes"`
}

type mcpVFSWriteInput struct {
	Path      string `json:"path" jsonschema:"required destination such as notes/todo.txt or sites/{siteID}/config/app.yml; RAM parent directories are created automatically"`
	Content   string `json:"content" jsonschema:"UTF-8 text or standard base64 data as declared by encoding; maximum decoded size is 4194304 bytes"`
	Encoding  string `json:"encoding,omitempty" jsonschema:"utf-8 or base64; omitted defaults to utf-8"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"must be true to replace an existing RAM or remote file; omitted defaults to false"`
}

type mcpVFSWriteChunkInput struct {
	Path      string `json:"path" jsonschema:"required RAM or remote destination such as artifacts/archive.bin or sites/{siteID}/releases/archive.bin"`
	Offset    int64  `json:"offset" jsonschema:"required zero-based byte offset; use zero to start or restart, then use the exact nextOffset returned by the previous call"`
	Content   string `json:"content" jsonschema:"UTF-8 text or standard base64 chunk data; maximum decoded chunk size is 1048576 bytes"`
	Encoding  string `json:"encoding,omitempty" jsonschema:"utf-8 or base64; omitted defaults to utf-8"`
	Final     bool   `json:"final" jsonschema:"required completion flag; false stages more data, true verifies sha256 and commits the complete file"`
	SHA256    string `json:"sha256,omitempty" jsonschema:"required when final is true; hexadecimal SHA-256 of the complete decoded file, optionally prefixed with sha256:"`
	Overwrite bool   `json:"overwrite,omitempty" jsonschema:"set true on the final call to replace an existing destination; omitted defaults to false"`
}

type mcpVFSWriteChunkOutput struct {
	Path          string      `json:"path" jsonschema:"normalized relative VFS destination path"`
	URI           string      `json:"uri" jsonschema:"full integterm-vfs URI of the destination"`
	AcceptedBytes int64       `json:"acceptedBytes" jsonschema:"decoded bytes accepted from this call"`
	NextOffset    int64       `json:"nextOffset" jsonschema:"exact offset required for the next chunk or finalization call"`
	Complete      bool        `json:"complete" jsonschema:"true only after SHA-256 verification and destination commit succeed"`
	SHA256        string      `json:"sha256,omitempty" jsonschema:"lowercase SHA-256 of the complete file after successful finalization"`
	Item          *mcpVFSItem `json:"item,omitempty" jsonschema:"final file metadata; present only when complete is true"`
}

type mcpVFSMkdirInput struct {
	Path string `json:"path" jsonschema:"required directory path such as notes/archive or sites/{siteID}/releases; cannot create or replace the reserved sites namespace"`
}

type mcpVFSDeleteInput struct {
	Path      string `json:"path" jsonschema:"required RAM or remote file or directory path; the workspace root, sites, and saved-site roots cannot be deleted"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"must be true to delete a non-empty directory tree; omitted defaults to false"`
}

type mcpVFSRenameInput struct {
	OldPath string `json:"oldPath" jsonschema:"required existing RAM or remote path"`
	NewPath string `json:"newPath" jsonschema:"required unused destination path in the same namespace; rename cannot cross RAM and remote storage or cross saved sites"`
}

type mcpVFSConnectInput struct {
	Path string `json:"path" jsonschema:"required saved-site root or descendant returned by vfs_list sites, for example sites/{siteID} or integterm-vfs://workspace/mcp/sites/{siteID}"`
}

type mcpVFSConnectOutput struct {
	SiteID     string `json:"siteId" jsonschema:"stable saved-site identifier used in VFS paths"`
	SiteName   string `json:"siteName" jsonschema:"display name of the connected saved site"`
	Protocol   string `json:"protocol" jsonschema:"saved-site protocol family"`
	URI        string `json:"uri" jsonschema:"full integterm-vfs URI of the mounted saved-site root"`
	RemoteRoot string `json:"remoteRoot" jsonschema:"configured remote root represented by the site URI"`
	Connected  bool   `json:"connected" jsonschema:"true when the remote mount is ready"`
}

type mcpVFSDeleteOutput struct {
	Path    string `json:"path" jsonschema:"normalized relative VFS path that was deleted"`
	URI     string `json:"uri" jsonschema:"full integterm-vfs URI that was deleted"`
	Deleted bool   `json:"deleted" jsonschema:"true when deletion completed"`
}

func newMCPVFS() *mcpVFS {
	now := time.Now().UTC()
	return &mcpVFS{
		nodes: map[string]mcpVFSNode{
			"": {directory: true, modified: now},
		},
		chunkWrites:  make(map[string]*mcpVFSChunkWrite),
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
		RootURI:       mcpVFSRootURI,
		TransportNote: "Use this URI only as a resource identifier or VFS tool path inside the established MCP connection; it is not an HTTP URL, shell command, or host filesystem path.",
		FirstCall:     "Call vfs_list with {} to list the workspace root.",
		SitesPath:     mcpVFSSitesPath,
		PathModel: []mcpVFSPathModelItem{
			{Pattern: "<empty> or integterm-vfs://workspace/mcp", Kind: "ram-root", Meaning: "RAM workspace root"},
			{Pattern: "notes/file.txt", Kind: "ram", Meaning: "RAM file or directory"},
			{Pattern: "sites", Kind: "sites", Meaning: "saved remote-site directory"},
			{Pattern: "sites/{siteID}", Kind: "site-root", Meaning: "saved remote site's configured remote root"},
			{Pattern: "sites/{siteID}/{relativePath}", Kind: "remote", Meaning: "file or directory below that remote root"},
		},
		FileCount:        fileCount,
		DirectoryCount:   directoryCount,
		BytesUsed:        vfs.totalBytes,
		MaxBytes:         mcpVFSTotalSize,
		MaxFileBytes:     mcpVFSMaxFileSize,
		MaxChunkedBytes:  mcpVFSMaxChunkedFile,
		MaxWriteChunk:    mcpVFSMaxWriteChunk,
		DefaultReadBytes: mcpVFSDefaultReadSize,
		MaxReadBytes:     mcpVFSMaxReadSize,
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
	data, err := decodeMCPVFSContent(content, encoding)
	if err != nil {
		return mcpVFSItem{}, err
	}
	return vfs.writeBytes(path, data, overwrite, mcpVFSMaxFileSize)
}

func (vfs *mcpVFS) writeBytes(path string, data []byte, overwrite bool, maxFileBytes int64) (mcpVFSItem, error) {
	relativePath, err := normalizeMCPVFSPath(path)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if relativePath == "" {
		return mcpVFSItem{}, fmt.Errorf("cannot write the virtual workspace root")
	}
	if maxFileBytes > 0 && int64(len(data)) > maxFileBytes {
		return mcpVFSItem{}, fmt.Errorf("file exceeds %d bytes", maxFileBytes)
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

func (layer *mcpVirtualLayer) writeVirtualChunk(input mcpVFSWriteChunkInput) (mcpVFSWriteChunkOutput, error) {
	location, err := parseMCPVFSLocation(input.Path)
	if err != nil {
		return mcpVFSWriteChunkOutput{}, err
	}
	if location.path == "" || (location.kind != mcpVFSLocationRAM && location.kind != mcpVFSLocationRemote) {
		return mcpVFSWriteChunkOutput{}, fmt.Errorf("chunk destination must be a RAM or remote file path: %s", input.Path)
	}
	if input.Offset < 0 {
		return mcpVFSWriteChunkOutput{}, fmt.Errorf("offset must not be negative")
	}
	chunk, err := decodeMCPVFSContent(input.Content, input.Encoding)
	if err != nil {
		return mcpVFSWriteChunkOutput{}, err
	}
	if int64(len(chunk)) > mcpVFSMaxWriteChunk {
		return mcpVFSWriteChunkOutput{}, fmt.Errorf("chunk exceeds %d bytes", mcpVFSMaxWriteChunk)
	}

	expectedSHA256 := ""
	if input.Final {
		expectedSHA256, err = normalizeMCPVFSSHA256(input.SHA256)
		if err != nil {
			return mcpVFSWriteChunkOutput{}, err
		}
	}

	vfs := layer.vfs
	key := location.path
	vfs.chunkMu.Lock()
	state, exists := vfs.chunkWrites[key]
	if exists && state.finalizing {
		vfs.chunkMu.Unlock()
		return mcpVFSWriteChunkOutput{}, fmt.Errorf("chunk write is already finalizing: %s", input.Path)
	}
	if input.Offset == 0 {
		if exists {
			vfs.chunkBytes -= int64(len(state.data))
		}
		state = &mcpVFSChunkWrite{}
		vfs.chunkWrites[key] = state
		exists = true
	}
	if !exists {
		vfs.chunkMu.Unlock()
		return mcpVFSWriteChunkOutput{}, fmt.Errorf("no staged chunk write for %s; start with offset 0", input.Path)
	}
	if input.Offset != int64(len(state.data)) {
		nextOffset := len(state.data)
		vfs.chunkMu.Unlock()
		return mcpVFSWriteChunkOutput{}, fmt.Errorf("offset %d does not match nextOffset %d for %s", input.Offset, nextOffset, input.Path)
	}
	nextSize := int64(len(state.data)) + int64(len(chunk))
	if nextSize > mcpVFSMaxChunkedFile {
		vfs.chunkMu.Unlock()
		return mcpVFSWriteChunkOutput{}, fmt.Errorf("chunked file exceeds %d bytes", mcpVFSMaxChunkedFile)
	}
	if vfs.chunkBytes+int64(len(chunk)) > mcpVFSMaxChunkedFile {
		vfs.chunkMu.Unlock()
		return mcpVFSWriteChunkOutput{}, fmt.Errorf("chunk staging exceeds %d bytes; finalize or restart an existing chunk write", mcpVFSMaxChunkedFile)
	}
	state.data = append(state.data, chunk...)
	vfs.chunkBytes += int64(len(chunk))
	nextOffset := int64(len(state.data))
	output := mcpVFSWriteChunkOutput{
		Path:          location.path,
		URI:           mcpVFSURI(location.path),
		AcceptedBytes: int64(len(chunk)),
		NextOffset:    nextOffset,
	}
	if !input.Final {
		vfs.chunkMu.Unlock()
		return output, nil
	}
	state.finalizing = true
	stagedData := append([]byte(nil), state.data...)
	vfs.chunkMu.Unlock()

	digest := sha256.Sum256(stagedData)
	actualSHA256 := hex.EncodeToString(digest[:])
	if actualSHA256 != expectedSHA256 {
		vfs.chunkMu.Lock()
		if current := vfs.chunkWrites[key]; current == state {
			vfs.chunkBytes -= int64(len(state.data))
			delete(vfs.chunkWrites, key)
		}
		vfs.chunkMu.Unlock()
		return mcpVFSWriteChunkOutput{}, fmt.Errorf("SHA-256 mismatch for %s: got %s; restart with offset 0", input.Path, actualSHA256)
	}

	item, err := layer.writeVirtualBytes(location.path, stagedData, input.Overwrite, mcpVFSMaxChunkedFile)
	if err != nil {
		vfs.chunkMu.Lock()
		if current := vfs.chunkWrites[key]; current == state {
			state.finalizing = false
		}
		vfs.chunkMu.Unlock()
		return mcpVFSWriteChunkOutput{}, err
	}

	vfs.chunkMu.Lock()
	if current := vfs.chunkWrites[key]; current == state {
		vfs.chunkBytes -= int64(len(state.data))
		delete(vfs.chunkWrites, key)
	}
	vfs.chunkMu.Unlock()
	output.Complete = true
	output.SHA256 = actualSHA256
	output.Item = &item
	return output, nil
}

func normalizeMCPVFSSHA256(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "sha256:")
	if len(value) != sha256.Size*2 {
		return "", fmt.Errorf("sha256 is required on the final chunk and must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", fmt.Errorf("sha256 must contain 64 hexadecimal characters")
	}
	return value, nil
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

func mcpVFSListNextAction(location mcpVFSLocation) string {
	switch location.kind {
	case mcpVFSLocationRAM:
		if location.path == "" {
			return "For RAM files, use a returned path or create one with vfs_write or vfs_write_chunk. For remote files, call vfs_list with path `sites` and reuse a returned site path or URI."
		}
		return "Reuse an entry path or URI with vfs_list, vfs_stat, vfs_read, vfs_write, vfs_write_chunk, vfs_rename, or vfs_delete."
	case mcpVFSLocationSites:
		return "Choose a returned saved-site path or URI. Call vfs_connect explicitly, or pass it to vfs_list and let the connection open lazily."
	case mcpVFSLocationSiteRoot, mcpVFSLocationRemote:
		return "Reuse a returned entry path or URI for remote VFS operations. Changes apply directly below this saved site's configured remote root."
	default:
		return "Reuse returned paths or URIs with VFS tools."
	}
}

func addMCPVFSFeatures(server *mcp.Server, layer *mcpVirtualLayer) {
	vfs := layer.vfs
	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_workspace_info",
		Title:       "Get virtual workspace",
		Description: "Canonical first call for every IntegTERM VFS task. With no arguments, returns the root URI, complete RAM/sites path model, discovery next step, read/write limits, saved-site count, and active mount count. Follow it with vfs_list using {}. The root URI is an identifier inside MCP, not a transport URL or host path.",
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
		Description: "Primary VFS discovery tool. Omit path (call with {}) to list the workspace root; use path `sites` to obtain saved site IDs; use `sites/{siteID}` or a returned URI to list a remote root; use any other relative path for RAM. Returns an explicit namespace kind, reusable entry paths/URIs, and a context-specific nextAction. Remote paths connect lazily.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSListInput) (*mcp.CallToolResult, mcpVFSListOutput, error) {
		entries, err := layer.listVirtual(input.Path)
		if err != nil {
			return nil, mcpVFSListOutput{}, err
		}
		location, err := parseMCPVFSLocation(input.Path)
		if err != nil {
			return nil, mcpVFSListOutput{}, err
		}
		return nil, mcpVFSListOutput{
			Path:       location.path,
			URI:        mcpVFSURI(location.path),
			Kind:       string(location.kind),
			Entries:    entries,
			NextAction: mcpVFSListNextAction(location),
		}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_stat",
		Title:       "Inspect virtual entry",
		Description: "Inspect one known RAM or remote VFS path and return its normalized path, reusable URI, size, directory flag, and modification time. Discover paths with vfs_list first. This does not accept host filesystem paths; remote descendants connect their saved site lazily.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSPathInput) (*mcp.CallToolResult, mcpVFSItem, error) {
		item, err := layer.statVirtual(input.Path)
		return nil, item, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_read",
		Title:       "Read virtual file",
		Description: "Read a byte range from a known RAM or remote VFS file. Valid UTF-8 is returned with encoding `utf-8`; binary bytes use `base64`. The default chunk is 262144 bytes and one call is limited to 1048576 bytes. If truncated is true, call again with offset equal to the prior offset plus returnedBytes.",
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
		Description: "Create or replace a RAM or remote VFS file with one inline payload up to 4194304 decoded bytes. encoding is `utf-8` by default or `base64` for binary data. Set overwrite=true when the destination already exists. Use vfs_write_chunk for larger files or transport-safe incremental writes.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSWriteInput) (*mcp.CallToolResult, mcpVFSItem, error) {
		item, err := layer.writeVirtual(input.Path, input.Content, input.Encoding, input.Overwrite)
		return nil, item, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_write_chunk",
		Title:       "Write virtual file in chunks",
		Description: "Write a RAM or remote file sequentially in decoded chunks of at most 1048576 bytes, up to 33554432 bytes total. Start or restart with offset=0 and final=false, then use each returned nextOffset. On the last call set final=true and provide the SHA-256 of the complete decoded file; set overwrite=true there when replacing an existing file. Data is committed only after final hash verification succeeds.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSWriteChunkInput) (*mcp.CallToolResult, mcpVFSWriteChunkOutput, error) {
		output, err := layer.writeVirtualChunk(input)
		return nil, output, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_mkdir",
		Title:       "Create virtual directory",
		Description: "Create a RAM or remote VFS directory. RAM ancestors are created automatically. A remote path must be below `sites/{siteID}` and connects lazily. The reserved workspace root, `sites`, and saved-site roots are discovered namespaces rather than directories to create.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSMkdirInput) (*mcp.CallToolResult, mcpVFSItem, error) {
		item, err := layer.mkdirVirtual(input.Path)
		return nil, item, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_delete",
		Title:       "Delete virtual entry",
		Description: "Delete one RAM or remote VFS file or directory. A non-empty directory requires recursive=true. The workspace root, reserved `sites` namespace, and saved-site roots cannot be deleted. Remote deletion acts directly on the saved remote site.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSDeleteInput) (*mcp.CallToolResult, mcpVFSDeleteOutput, error) {
		result, err := layer.deleteVirtual(input.Path, input.Recursive)
		return nil, result, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_rename",
		Title:       "Rename virtual entry",
		Description: "Rename or move a RAM or remote VFS entry to an unused destination. Both paths must remain in the same storage namespace: RAM-to-RAM or within one saved site. Cross-site moves and RAM/remote moves are rejected; copy content explicitly when crossing namespaces.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSRenameInput) (*mcp.CallToolResult, mcpVFSItem, error) {
		item, err := layer.renameVirtual(input.OldPath, input.NewPath)
		return nil, item, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "vfs_connect",
		Title:       "Connect virtual remote site",
		Description: "Explicitly connect a saved remote site. First call vfs_list with path `sites`, then pass a returned `sites/{siteID}` path or full URI. The mount represents the site's configured remote root and is reused. This call is optional because remote vfs_list/stat/read/write/mkdir/rename/delete operations connect lazily.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input mcpVFSConnectInput) (*mcp.CallToolResult, mcpVFSConnectOutput, error) {
		output, err := layer.connectVirtualSite(input.Path)
		return nil, output, err
	})

	server.AddResource(&mcp.Resource{
		URI:         mcpVFSRootURI,
		Name:        "IntegTERM virtual workspace",
		Title:       "IntegTERM virtual workspace",
		Description: "Self-contained VFS usage guide and live root listing for IntegTERM's RAM workspace and saved remote-site mounts. Read this resource for workflow, path forms, tool rules, examples, and limits.",
		MIMEType:    "application/json",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		entries, err := layer.listVirtual(mcpVFSRootURI)
		if err != nil {
			return nil, err
		}
		info := vfs.workspaceInfo()
		info.MountedSiteCount = vfs.mountedRemoteSiteCount()
		info.RemoteSiteCount = layer.mcpRemoteSiteCount()
		payload := map[string]any{
			"uri":           mcpVFSRootURI,
			"transportNote": info.TransportNote,
			"workflow": []string{
				"Call vfs_workspace_info with {}.",
				"Call vfs_list with {} to list this root.",
				"Use a normal relative path for RAM, or list sites and reuse a returned siteID for remote work.",
				"Use vfs_list, vfs_stat, vfs_read, vfs_write, vfs_write_chunk, vfs_mkdir, vfs_rename, and vfs_delete with returned paths or URIs.",
			},
			"pathModel": info.PathModel,
			"toolGuide": map[string]string{
				"vfs_workspace_info": "Read the canonical path model and limits first.",
				"vfs_list":           "Discover the root, saved site IDs, and directory children.",
				"vfs_connect":        "Optionally connect one returned saved-site path; remote file tools also connect lazily.",
				"vfs_stat":           "Inspect one known file or directory.",
				"vfs_read":           "Read text or base64 bytes in bounded chunks.",
				"vfs_write":          "Create an inline file up to 4 MiB; set overwrite=true for an existing file.",
				"vfs_write_chunk":    "Stage sequential chunks, then commit with final=true and the complete file SHA-256.",
				"vfs_mkdir":          "Create a RAM or remote directory.",
				"vfs_rename":         "Move only within RAM or within one saved-site mount.",
				"vfs_delete":         "Delete an entry; non-empty directories require recursive=true.",
			},
			"examples": map[string]any{
				"listRoot":         map[string]any{"name": "vfs_list", "arguments": map[string]any{}},
				"writeRAMText":     map[string]any{"name": "vfs_write", "arguments": map[string]any{"path": "notes/todo.txt", "content": "hello", "encoding": "utf-8"}},
				"listSites":        map[string]any{"name": "vfs_list", "arguments": map[string]any{"path": "sites"}},
				"connectSite":      map[string]any{"name": "vfs_connect", "arguments": map[string]any{"path": "sites/{siteID}"}},
				"listRemoteRoot":   map[string]any{"name": "vfs_list", "arguments": map[string]any{"path": "sites/{siteID}"}},
				"continueFileRead": map[string]any{"name": "vfs_read", "arguments": map[string]any{"path": "notes/large.txt", "offset": 262144, "limit": 262144}},
				"startChunkWrite":  map[string]any{"name": "vfs_write_chunk", "arguments": map[string]any{"path": "artifacts/example.bin", "offset": 0, "content": "hello ", "encoding": "utf-8", "final": false}},
				"finishChunkWrite": map[string]any{"name": "vfs_write_chunk", "arguments": map[string]any{"path": "artifacts/example.bin", "offset": 6, "content": "world", "encoding": "utf-8", "final": true, "sha256": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"}},
			},
			"limits": map[string]any{
				"ramWorkspaceBytes": info.MaxBytes,
				"inlineWriteBytes":  info.MaxFileBytes,
				"chunkedFileBytes":  info.MaxChunkedBytes,
				"writeChunkBytes":   info.MaxWriteChunk,
				"defaultReadBytes":  info.DefaultReadBytes,
				"maxReadBytes":      info.MaxReadBytes,
			},
			"entries": entries,
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: mcpVFSRootURI, MIMEType: "application/json", Text: mustJSONIndent(payload)}}}, nil
	})

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "integterm-vfs://workspace/mcp/{+path}",
		Name:        "Virtual file",
		Title:       "Virtual file",
		Description: "Read one known RAM or saved remote-site file by a URI returned from vfs_list or vfs_stat. Use VFS tools for discovery and vfs_read for files over 1 MiB or chunked reads.",
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

package app

import (
	"crypto/sha256"
	"fmt"
	pathpkg "path"
	"strings"

	"IntegTERM/internal/model"
	"IntegTERM/internal/session"
)

const mcpVFSSitesPath = "sites"

type mcpVFSRemoteMount struct {
	SiteID      string
	TabID       string
	RootPath    string
	fingerprint [32]byte
}

type mcpVFSLocationKind string

const (
	mcpVFSLocationRAM      mcpVFSLocationKind = "ram"
	mcpVFSLocationSites    mcpVFSLocationKind = "sites"
	mcpVFSLocationSiteRoot mcpVFSLocationKind = "site-root"
	mcpVFSLocationRemote   mcpVFSLocationKind = "remote"
)

type mcpVFSLocation struct {
	kind       mcpVFSLocationKind
	path       string
	siteID     string
	remotePath string
}

func parseMCPVFSLocation(value string) (mcpVFSLocation, error) {
	normalized, err := normalizeMCPVFSPath(value)
	if err != nil {
		return mcpVFSLocation{}, err
	}
	location := mcpVFSLocation{kind: mcpVFSLocationRAM, path: normalized}
	if normalized == mcpVFSSitesPath {
		location.kind = mcpVFSLocationSites
		return location, nil
	}
	if !strings.HasPrefix(normalized, mcpVFSSitesPath+"/") {
		return location, nil
	}
	parts := strings.Split(normalized, "/")
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		return mcpVFSLocation{}, fmt.Errorf("remote site id is required in virtual path: %s", value)
	}
	location.siteID = parts[1]
	if len(parts) == 2 {
		location.kind = mcpVFSLocationSiteRoot
		return location, nil
	}
	location.kind = mcpVFSLocationRemote
	location.remotePath = strings.Join(parts[2:], "/")
	return location, nil
}

func (vfs *mcpVFS) remoteMount(siteID string) (mcpVFSRemoteMount, bool) {
	vfs.remoteMu.Lock()
	defer vfs.remoteMu.Unlock()
	mount, ok := vfs.remoteMounts[siteID]
	return mount, ok
}

func (vfs *mcpVFS) mountedRemoteSiteCount() int {
	vfs.remoteMu.Lock()
	defer vfs.remoteMu.Unlock()
	return len(vfs.remoteMounts)
}

func (layer *mcpVirtualLayer) listVirtual(value string) ([]mcpVFSItem, error) {
	location, err := parseMCPVFSLocation(value)
	if err != nil {
		return nil, err
	}
	switch location.kind {
	case mcpVFSLocationRAM:
		entries, err := layer.vfs.list(location.path)
		if err != nil {
			return nil, err
		}
		if location.path != "" {
			return entries, nil
		}
		filtered := entries[:0]
		for _, entry := range entries {
			if entry.Path != mcpVFSSitesPath {
				filtered = append(filtered, entry)
			}
		}
		sitesItem := mcpVFSDirectoryItem(mcpVFSSitesPath)
		return append(filtered, sitesItem), nil
	case mcpVFSLocationSites:
		return layer.listRemoteSites()
	case mcpVFSLocationSiteRoot, mcpVFSLocationRemote:
		layer.vfs.remoteOpsMu.Lock()
		defer layer.vfs.remoteOpsMu.Unlock()
		mount, err := layer.ensureRemoteMount(location.siteID)
		if err != nil {
			return nil, err
		}
		remotePath, err := layer.checkedRemotePath(mount, location.remotePath, false)
		if err != nil {
			return nil, err
		}
		entries, err := layer.app.sessionManager.ListRemote(mount.TabID, remotePath)
		if err != nil {
			return nil, err
		}
		layer.app.markTabActivity(mount.TabID)
		layer.app.stateMu.RLock()
		showHidden := layer.app.config.ShowHiddenFiles
		layer.app.stateMu.RUnlock()
		result := make([]mcpVFSItem, 0, len(entries))
		for _, entry := range filterHiddenEntries(entries, showHidden) {
			virtualPath, err := virtualRemotePath(mount, entry.Path)
			if err != nil {
				return nil, err
			}
			result = append(result, mcpVFSRemoteItem(virtualPath, entry))
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported virtual path: %s", value)
	}
}

func (layer *mcpVirtualLayer) statVirtual(value string) (mcpVFSItem, error) {
	location, err := parseMCPVFSLocation(value)
	if err != nil {
		return mcpVFSItem{}, err
	}
	switch location.kind {
	case mcpVFSLocationRAM:
		return layer.vfs.stat(location.path)
	case mcpVFSLocationSites:
		return mcpVFSDirectoryItem(mcpVFSSitesPath), nil
	case mcpVFSLocationSiteRoot:
		site, err := layer.findRemoteSite(location.siteID)
		if err != nil {
			return mcpVFSItem{}, err
		}
		return mcpVFSRemoteSiteItem(site), nil
	case mcpVFSLocationRemote:
		layer.vfs.remoteOpsMu.Lock()
		defer layer.vfs.remoteOpsMu.Unlock()
		mount, err := layer.ensureRemoteMount(location.siteID)
		if err != nil {
			return mcpVFSItem{}, err
		}
		remotePath, err := layer.checkedRemotePath(mount, location.remotePath, false)
		if err != nil {
			return mcpVFSItem{}, err
		}
		entry, err := layer.app.sessionManager.StatRemote(mount.TabID, remotePath)
		if err != nil {
			return mcpVFSItem{}, err
		}
		layer.app.markTabActivity(mount.TabID)
		return mcpVFSRemoteItem(location.path, entry), nil
	default:
		return mcpVFSItem{}, fmt.Errorf("unsupported virtual path: %s", value)
	}
}

func (layer *mcpVirtualLayer) readVirtual(value string, offset int64, limit int64) ([]byte, mcpVFSItem, bool, error) {
	location, err := parseMCPVFSLocation(value)
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
	if location.kind == mcpVFSLocationRAM {
		return layer.vfs.read(location.path, offset, limit)
	}
	if location.kind != mcpVFSLocationRemote {
		return nil, mcpVFSItem{}, false, fmt.Errorf("virtual path is not a file: %s", value)
	}
	layer.vfs.remoteOpsMu.Lock()
	defer layer.vfs.remoteOpsMu.Unlock()
	mount, err := layer.ensureRemoteMount(location.siteID)
	if err != nil {
		return nil, mcpVFSItem{}, false, err
	}
	remotePath, err := layer.checkedRemotePath(mount, location.remotePath, false)
	if err != nil {
		return nil, mcpVFSItem{}, false, err
	}
	data, entry, err := layer.app.sessionManager.ReadRemoteFile(mount.TabID, remotePath, offset, limit, mcpVFSMaxChunkedFile)
	if err != nil {
		return nil, mcpVFSItem{}, false, err
	}
	layer.app.markTabActivity(mount.TabID)
	item := mcpVFSRemoteItem(location.path, entry)
	return data, item, offset < entry.Size && int64(len(data)) < entry.Size-offset, nil
}

func (layer *mcpVirtualLayer) writeVirtual(value string, content string, encoding string, overwrite bool) (mcpVFSItem, error) {
	data, err := decodeMCPVFSContent(content, encoding, mcpVFSMaxFileSize)
	if err != nil {
		return mcpVFSItem{}, err
	}
	return layer.writeVirtualBytes(value, data, overwrite, mcpVFSMaxFileSize)
}

func (layer *mcpVirtualLayer) writeVirtualBytes(value string, data []byte, overwrite bool, maxFileBytes int64) (mcpVFSItem, error) {
	location, err := parseMCPVFSLocation(value)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if location.kind == mcpVFSLocationRAM {
		return layer.vfs.writeBytes(location.path, data, overwrite, maxFileBytes)
	}
	if location.kind != mcpVFSLocationRemote {
		return mcpVFSItem{}, fmt.Errorf("virtual path is not a file: %s", value)
	}
	layer.vfs.remoteOpsMu.Lock()
	defer layer.vfs.remoteOpsMu.Unlock()
	mount, err := layer.ensureRemoteMount(location.siteID)
	if err != nil {
		return mcpVFSItem{}, err
	}
	remotePath, err := layer.checkedRemotePath(mount, location.remotePath, true)
	if err != nil {
		return mcpVFSItem{}, err
	}
	entry, err := layer.app.sessionManager.WriteRemoteFile(mount.TabID, remotePath, data, overwrite, maxFileBytes)
	if err != nil {
		return mcpVFSItem{}, err
	}
	layer.app.markTabActivity(mount.TabID)
	return mcpVFSRemoteItem(location.path, entry), nil
}

func (layer *mcpVirtualLayer) mkdirVirtual(value string) (mcpVFSItem, error) {
	location, err := parseMCPVFSLocation(value)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if location.kind == mcpVFSLocationRAM {
		return layer.vfs.mkdir(location.path)
	}
	if location.kind != mcpVFSLocationRemote {
		return mcpVFSItem{}, fmt.Errorf("cannot create the virtual site namespace")
	}
	layer.vfs.remoteOpsMu.Lock()
	defer layer.vfs.remoteOpsMu.Unlock()
	mount, err := layer.ensureRemoteMount(location.siteID)
	if err != nil {
		return mcpVFSItem{}, err
	}
	remotePath, err := layer.checkedRemotePath(mount, location.remotePath, true)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if err := layer.app.sessionManager.CreateRemoteDirectory(mount.TabID, remotePath); err != nil {
		return mcpVFSItem{}, err
	}
	layer.app.markTabActivity(mount.TabID)
	entry, err := layer.app.sessionManager.StatRemote(mount.TabID, remotePath)
	if err != nil {
		return mcpVFSItem{}, err
	}
	return mcpVFSRemoteItem(location.path, entry), nil
}

func (layer *mcpVirtualLayer) deleteVirtual(value string, recursive bool) (mcpVFSDeleteOutput, error) {
	location, err := parseMCPVFSLocation(value)
	if err != nil {
		return mcpVFSDeleteOutput{}, err
	}
	if location.kind == mcpVFSLocationRAM {
		return layer.vfs.delete(location.path, recursive)
	}
	if location.kind != mcpVFSLocationRemote {
		return mcpVFSDeleteOutput{}, fmt.Errorf("cannot delete the virtual site namespace")
	}
	layer.vfs.remoteOpsMu.Lock()
	defer layer.vfs.remoteOpsMu.Unlock()
	mount, err := layer.ensureRemoteMount(location.siteID)
	if err != nil {
		return mcpVFSDeleteOutput{}, err
	}
	remotePath, err := layer.checkedRemotePath(mount, location.remotePath, false)
	if err != nil {
		return mcpVFSDeleteOutput{}, err
	}
	if err := layer.app.sessionManager.DeleteRemotePathWithRecursive(mount.TabID, remotePath, recursive); err != nil {
		return mcpVFSDeleteOutput{}, err
	}
	layer.app.markTabActivity(mount.TabID)
	return mcpVFSDeleteOutput{Path: location.path, URI: mcpVFSURI(location.path), Deleted: true}, nil
}

func (layer *mcpVirtualLayer) renameVirtual(oldValue string, newValue string) (mcpVFSItem, error) {
	oldLocation, err := parseMCPVFSLocation(oldValue)
	if err != nil {
		return mcpVFSItem{}, err
	}
	newLocation, err := parseMCPVFSLocation(newValue)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if oldLocation.kind == mcpVFSLocationRAM && newLocation.kind == mcpVFSLocationRAM {
		return layer.vfs.rename(oldLocation.path, newLocation.path)
	}
	if oldLocation.kind != mcpVFSLocationRemote || newLocation.kind != mcpVFSLocationRemote || oldLocation.siteID != newLocation.siteID {
		return mcpVFSItem{}, fmt.Errorf("rename must stay within one virtual namespace")
	}
	layer.vfs.remoteOpsMu.Lock()
	defer layer.vfs.remoteOpsMu.Unlock()
	mount, err := layer.ensureRemoteMount(oldLocation.siteID)
	if err != nil {
		return mcpVFSItem{}, err
	}
	oldPath, err := layer.checkedRemotePath(mount, oldLocation.remotePath, false)
	if err != nil {
		return mcpVFSItem{}, err
	}
	newPath, err := layer.checkedRemotePath(mount, newLocation.remotePath, true)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if err := layer.app.sessionManager.RenameRemotePathNoReplace(mount.TabID, oldPath, newPath); err != nil {
		return mcpVFSItem{}, err
	}
	layer.app.markTabActivity(mount.TabID)
	entry, err := layer.app.sessionManager.StatRemote(mount.TabID, newPath)
	if err != nil {
		return mcpVFSItem{}, err
	}
	return mcpVFSRemoteItem(newLocation.path, entry), nil
}

func (layer *mcpVirtualLayer) connectVirtualSite(value string) (mcpVFSConnectOutput, error) {
	location, err := parseMCPVFSLocation(value)
	if err != nil {
		return mcpVFSConnectOutput{}, err
	}
	if location.kind != mcpVFSLocationSiteRoot && location.kind != mcpVFSLocationRemote {
		return mcpVFSConnectOutput{}, fmt.Errorf("use a saved site URI such as %s/sites/{siteID}", mcpVFSRootURI)
	}
	layer.vfs.remoteOpsMu.Lock()
	defer layer.vfs.remoteOpsMu.Unlock()
	mount, err := layer.ensureRemoteMount(location.siteID)
	if err != nil {
		return mcpVFSConnectOutput{}, err
	}
	site, err := layer.findRemoteSite(location.siteID)
	if err != nil {
		return mcpVFSConnectOutput{}, err
	}
	layer.app.markTabActivity(mount.TabID)
	return mcpVFSConnectOutput{
		SiteID:     site.ID,
		SiteName:   site.Name,
		Protocol:   site.Protocol,
		URI:        mcpVFSURI("sites/" + site.ID),
		RemoteRoot: mount.RootPath,
		Connected:  true,
	}, nil
}

func (layer *mcpVirtualLayer) listRemoteSites() ([]mcpVFSItem, error) {
	sites, err := layer.mcpRemoteSites()
	if err != nil {
		return nil, err
	}
	result := make([]mcpVFSItem, 0, len(sites))
	for _, site := range sites {
		result = append(result, mcpVFSRemoteSiteItem(site))
	}
	return result, nil
}

func (layer *mcpVirtualLayer) findRemoteSite(siteID string) (model.Site, error) {
	sites, err := layer.mcpRemoteSites()
	if err != nil {
		return model.Site{}, err
	}
	for _, site := range sites {
		if site.ID == siteID {
			return site, nil
		}
	}
	return model.Site{}, fmt.Errorf("saved remote site not found: %s", siteID)
}

func (layer *mcpVirtualLayer) mcpRemoteSites() ([]model.Site, error) {
	layer.app.stateMu.Lock()
	defer layer.app.stateMu.Unlock()
	if layer.app.storageInitErr != nil {
		if layer.app.store == nil {
			return nil, layer.app.storageInitErr
		}
		if err := layer.app.retryInitialStorageLocked(); err != nil {
			return nil, err
		}
	}
	if layer.app.store != nil {
		if err := layer.app.reloadSitesFromStoreLocked(); err != nil {
			return nil, fmt.Errorf("reload sites: %w", err)
		}
	}
	for _, site := range layer.app.sites {
		normalized, err := normalizeMCPVFSPath(site.ID)
		if err != nil || normalized != site.ID || normalized == "" || strings.Contains(normalized, "/") {
			return nil, fmt.Errorf("saved remote site has an invalid identifier")
		}
	}
	return append([]model.Site(nil), layer.app.sites...), nil
}

func (layer *mcpVirtualLayer) mcpRemoteSiteCount() (int, error) {
	sites, err := layer.mcpRemoteSites()
	return len(sites), err
}

// remoteOpsMu is held by every caller until its complete remote operation ends.
// This prevents two MCP calls from racing a create/rename check and commit.
func (layer *mcpVirtualLayer) ensureRemoteMount(siteID string) (mcpVFSRemoteMount, error) {
	site, err := layer.findRemoteSite(siteID)
	if err != nil {
		return mcpVFSRemoteMount{}, err
	}
	fingerprint := mcpSiteFingerprint(site)
	layer.vfs.remoteMu.Lock()
	defer layer.vfs.remoteMu.Unlock()
	if mount, ok := layer.vfs.remoteMounts[siteID]; ok {
		if mount.fingerprint == fingerprint && layer.app.mcpTabConnected(mount.TabID) {
			return mount, nil
		}
		layer.app.closeMCPRemoteTab(mount.TabID)
		delete(layer.vfs.remoteMounts, siteID)
	}
	for id, mount := range layer.vfs.remoteMounts {
		if !layer.app.mcpTabConnected(mount.TabID) {
			layer.app.closeMCPRemoteTab(mount.TabID)
			delete(layer.vfs.remoteMounts, id)
		}
	}
	if len(layer.vfs.remoteMounts) >= 64 {
		return mcpVFSRemoteMount{}, fmt.Errorf("too many remote VFS mounts")
	}
	tab, err := layer.app.createMCPRemoteTab(site)
	if err != nil {
		return mcpVFSRemoteMount{}, err
	}
	// An edit during connection must not leave the new mount using stale credentials
	// or a root from a previous version of the saved site.
	latest, err := layer.findRemoteSite(siteID)
	if err != nil || mcpSiteFingerprint(latest) != fingerprint {
		layer.app.closeMCPRemoteTab(tab.ID)
		if err != nil {
			return mcpVFSRemoteMount{}, err
		}
		return mcpVFSRemoteMount{}, fmt.Errorf("saved remote site changed while connecting; retry")
	}
	mount := mcpVFSRemoteMount{SiteID: site.ID, TabID: tab.ID, RootPath: tab.RemotePath, fingerprint: fingerprint}
	layer.vfs.remoteMounts[siteID] = mount
	return mount, nil
}

func mcpSiteFingerprint(site model.Site) [32]byte {
	return sha256.Sum256([]byte(fmt.Sprintf("%q", []string{site.Protocol, site.Host, fmt.Sprint(site.Port), site.Username, site.Password, site.PPKPath, site.PPKPassphrase, site.RemotePath})))
}

func (a *App) createMCPRemoteTab(site model.Site) (model.Tab, error) {
	a.stateMu.RLock()
	storageErr := a.storageInitErr
	a.stateMu.RUnlock()
	if storageErr != nil {
		return model.Tab{}, storageErr
	}
	if a.sessionManager == nil {
		return model.Tab{}, fmt.Errorf("remote session manager unavailable")
	}
	tab := session.MakeTab(site)
	cwd, err := a.sessionManager.Connect(tab)
	if err != nil {
		return model.Tab{}, err
	}
	root, err := mcpMountRoot(site.RemotePath, cwd)
	if err == nil {
		err = a.sessionManager.ValidateRemoteRootPath(tab.ID, root, root, false)
	}
	if err != nil {
		_ = a.sessionManager.Disconnect(tab.ID)
		return model.Tab{}, err
	}
	tab.RemotePath = root
	tab.Connected = true
	tab.Hidden = true
	a.stateMu.Lock()
	if a.storageInitErr != nil {
		err = a.storageInitErr
	} else {
		// Hidden MCP sessions are process-local and never belong in tabs.json.
		// Avoid rewriting another process's persisted tabs or LastActiveTab.
		a.tabs = append(a.tabs, tab)
	}
	a.stateMu.Unlock()
	if err != nil {
		_ = a.sessionManager.Disconnect(tab.ID)
		return model.Tab{}, err
	}
	a.markTabActivity(tab.ID)
	return tab, nil
}

func (a *App) closeMCPRemoteTab(tabID string) {
	if a.sessionManager != nil {
		_ = a.sessionManager.Disconnect(tabID)
	}
	a.stateMu.Lock()
	for i, tab := range a.tabs {
		if tab.ID == tabID && tab.Hidden {
			a.tabs = append(a.tabs[:i], a.tabs[i+1:]...)
			break
		}
	}
	a.stateMu.Unlock()
	a.activityMu.Lock()
	delete(a.lastActivity, tabID)
	a.activityMu.Unlock()
}

func mcpMountRoot(configured, cwd string) (string, error) {
	if configured == "" {
		configured = cwd
	}
	for _, segment := range strings.Split(configured, "/") {
		if segment == ".." {
			return "", fmt.Errorf("remote root cannot contain ..")
		}
	}
	if !pathpkg.IsAbs(configured) {
		if !pathpkg.IsAbs(cwd) {
			return "", fmt.Errorf("cannot resolve the configured remote root without an absolute working directory")
		}
		configured = pathpkg.Join(cwd, configured)
	}
	if configured == "" {
		return "", fmt.Errorf("remote root is required")
	}
	for _, r := range configured {
		if r < 32 || r == 127 || r == '\\' {
			return "", fmt.Errorf("invalid remote root")
		}
	}
	return pathpkg.Clean(configured), nil
}

func (layer *mcpVirtualLayer) checkedRemotePath(mount mcpVFSRemoteMount, relative string, missingLeaf bool) (string, error) {
	target := remotePathForMCPMount(mount, relative)
	if _, err := virtualRemotePath(mount, target); err != nil {
		return "", err
	}
	if err := layer.app.sessionManager.ValidateRemoteRootPath(mount.TabID, mount.RootPath, target, missingLeaf); err != nil {
		return "", err
	}
	return target, nil
}

func (a *App) mcpTabConnected(tabID string) bool {
	if a.sessionManager == nil || !a.sessionManager.IsConnected(tabID) {
		return false
	}
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	for _, tab := range a.tabs {
		if tab.ID == tabID {
			return tab.Connected
		}
	}
	return false
}

func remotePathForMCPMount(mount mcpVFSRemoteMount, relativePath string) string {
	if relativePath == "" {
		return mount.RootPath
	}
	return pathpkg.Join(mount.RootPath, relativePath)
}

func virtualRemotePath(mount mcpVFSRemoteMount, remotePath string) (string, error) {
	root := pathpkg.Clean(mount.RootPath)
	target := pathpkg.Clean(remotePath)
	if !pathpkg.IsAbs(root) || !pathpkg.IsAbs(target) {
		return "", fmt.Errorf("remote paths must be absolute")
	}
	for _, segment := range strings.Split(remotePath, "/") {
		if segment == ".." {
			return "", fmt.Errorf("remote path contains traversal")
		}
	}
	relative := ""
	switch {
	case target == root:
	case root == "/":
		relative = strings.TrimPrefix(target, "/")
	case strings.HasPrefix(target, root+"/"):
		relative = strings.TrimPrefix(target, root+"/")
	default:
		return "", fmt.Errorf("remote path escaped virtual site root: %s", remotePath)
	}
	if _, err := normalizeMCPVFSPath(relative); err != nil {
		return "", err
	}

	path := "sites/" + mount.SiteID
	if relative != "" {
		path += "/" + relative
	}
	return path, nil
}

func mcpVFSDirectoryItem(path string) mcpVFSItem {
	name := path
	if index := strings.LastIndexByte(path, '/'); index >= 0 {
		name = path[index+1:]
	}
	return mcpVFSItem{Name: name, Path: path, URI: mcpVFSURI(path), IsDir: true}
}

func mcpVFSRemoteSiteItem(site model.Site) mcpVFSItem {
	name := site.Name
	if strings.TrimSpace(name) == "" {
		name = site.ID
	}
	return mcpVFSItem{Name: name, Path: "sites/" + site.ID, URI: mcpVFSURI("sites/" + site.ID), IsDir: true, Modified: site.LastUsedAt}
}

func mcpVFSRemoteItem(virtualPath string, entry model.FileEntry) mcpVFSItem {
	return mcpVFSItem{
		Name:     entry.Name,
		Path:     virtualPath,
		URI:      mcpVFSURI(virtualPath),
		Size:     entry.Size,
		IsDir:    entry.IsDir,
		Modified: entry.Modified,
	}
}

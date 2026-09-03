package app

import (
	"fmt"
	pathpkg "path"
	"strings"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
	"github.com/VaderChen/Integrate-Terminal/internal/session"
)

const mcpVFSSitesPath = "sites"

type mcpVFSRemoteMount struct {
	SiteID   string
	TabID    string
	RootPath string
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
		mount, err := layer.ensureRemoteMount(location.siteID)
		if err != nil {
			return nil, err
		}
		remotePath := remotePathForMCPMount(mount, location.remotePath)
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
		mount, err := layer.ensureRemoteMount(location.siteID)
		if err != nil {
			return mcpVFSItem{}, err
		}
		entry, err := layer.app.sessionManager.StatRemote(mount.TabID, remotePathForMCPMount(mount, location.remotePath))
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
	mount, err := layer.ensureRemoteMount(location.siteID)
	if err != nil {
		return nil, mcpVFSItem{}, false, err
	}
	data, entry, err := layer.app.sessionManager.ReadRemoteFile(mount.TabID, remotePathForMCPMount(mount, location.remotePath), offset, limit, mcpVFSMaxFileSize)
	if err != nil {
		return nil, mcpVFSItem{}, false, err
	}
	layer.app.markTabActivity(mount.TabID)
	item := mcpVFSRemoteItem(location.path, entry)
	return data, item, int64(len(data))+offset < entry.Size, nil
}

func (layer *mcpVirtualLayer) writeVirtual(value string, content string, encoding string, overwrite bool) (mcpVFSItem, error) {
	location, err := parseMCPVFSLocation(value)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if location.kind == mcpVFSLocationRAM {
		return layer.vfs.write(location.path, content, encoding, overwrite)
	}
	if location.kind != mcpVFSLocationRemote {
		return mcpVFSItem{}, fmt.Errorf("virtual path is not a file: %s", value)
	}
	data, err := decodeMCPVFSContent(content, encoding)
	if err != nil {
		return mcpVFSItem{}, err
	}
	mount, err := layer.ensureRemoteMount(location.siteID)
	if err != nil {
		return mcpVFSItem{}, err
	}
	entry, err := layer.app.sessionManager.WriteRemoteFile(mount.TabID, remotePathForMCPMount(mount, location.remotePath), data, overwrite, mcpVFSMaxFileSize)
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
	mount, err := layer.ensureRemoteMount(location.siteID)
	if err != nil {
		return mcpVFSItem{}, err
	}
	remotePath := remotePathForMCPMount(mount, location.remotePath)
	if err := layer.app.sessionManager.CreateRemoteDirectory(mount.TabID, remotePath); err != nil {
		return mcpVFSItem{}, err
	}
	layer.app.markTabActivity(mount.TabID)
	entry, err := layer.app.sessionManager.StatRemote(mount.TabID, remotePath)
	if err != nil {
		return mcpVFSItem{Path: location.path, URI: mcpVFSURI(location.path), IsDir: true}, nil
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
	mount, err := layer.ensureRemoteMount(location.siteID)
	if err != nil {
		return mcpVFSDeleteOutput{}, err
	}
	if err := layer.app.sessionManager.DeleteRemotePathWithRecursive(mount.TabID, remotePathForMCPMount(mount, location.remotePath), recursive); err != nil {
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
	mount, err := layer.ensureRemoteMount(oldLocation.siteID)
	if err != nil {
		return mcpVFSItem{}, err
	}
	if err := layer.app.sessionManager.RenameRemotePath(mount.TabID, remotePathForMCPMount(mount, oldLocation.remotePath), remotePathForMCPMount(mount, newLocation.remotePath)); err != nil {
		return mcpVFSItem{}, err
	}
	layer.app.markTabActivity(mount.TabID)
	return layer.statVirtual(newLocation.path)
}

func (layer *mcpVirtualLayer) connectVirtualSite(value string) (mcpVFSConnectOutput, error) {
	location, err := parseMCPVFSLocation(value)
	if err != nil {
		return mcpVFSConnectOutput{}, err
	}
	if location.kind != mcpVFSLocationSiteRoot && location.kind != mcpVFSLocationRemote {
		return mcpVFSConnectOutput{}, fmt.Errorf("use a saved site URI such as %s/sites/{siteID}", mcpVFSRootURI)
	}
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
	if layer.app.store != nil {
		if err := layer.app.reloadSitesFromStoreLocked(); err != nil {
			return nil, fmt.Errorf("reload sites: %w", err)
		}
	}
	return append([]model.Site(nil), layer.app.sites...), nil
}

func (layer *mcpVirtualLayer) mcpRemoteSiteCount() int {
	layer.app.stateMu.RLock()
	defer layer.app.stateMu.RUnlock()
	return len(layer.app.sites)
}

func (layer *mcpVirtualLayer) ensureRemoteMount(siteID string) (mcpVFSRemoteMount, error) {
	site, err := layer.findRemoteSite(siteID)
	if err != nil {
		return mcpVFSRemoteMount{}, err
	}
	if mount, ok := layer.vfs.remoteMount(siteID); ok && layer.app.mcpTabConnected(mount.TabID) {
		return mount, nil
	}

	layer.vfs.remoteMu.Lock()
	defer layer.vfs.remoteMu.Unlock()
	if mount, ok := layer.vfs.remoteMounts[siteID]; ok && layer.app.mcpTabConnected(mount.TabID) {
		return mount, nil
	}
	site, err = layer.findRemoteSite(siteID)
	if err != nil {
		return mcpVFSRemoteMount{}, err
	}
	tab, err := layer.app.createMCPRemoteTab(site)
	if err != nil {
		return mcpVFSRemoteMount{}, err
	}
	rootPath := strings.TrimSpace(tab.RemotePath)
	if rootPath == "" {
		rootPath = strings.TrimSpace(site.RemotePath)
	}
	if rootPath == "" {
		rootPath = "/"
	}
	mount := mcpVFSRemoteMount{SiteID: site.ID, TabID: tab.ID, RootPath: rootPath}
	layer.vfs.remoteMounts[siteID] = mount
	return mount, nil
}

func (a *App) createMCPRemoteTab(site model.Site) (model.Tab, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	tab := session.MakeTab(site)
	remotePath, err := a.sessionManager.Connect(tab)
	if err != nil {
		return model.Tab{}, err
	}
	if strings.TrimSpace(remotePath) != "" {
		tab.RemotePath = remotePath
	}
	tab.Connected = true
	tab.Hidden = true
	previousLastActiveTab := a.config.LastActiveTab
	nextTabs := append(cloneTabs(a.tabs), tab)
	a.config.LastActiveTab = previousLastActiveTab
	if err := a.persistTabsLocked(nextTabs); err != nil {
		_ = a.sessionManager.Disconnect(tab.ID)
		return model.Tab{}, err
	}
	a.tabs = nextTabs
	a.markTabActivity(tab.ID)
	a.touchSiteLastUsedLocked(site.ID)
	return tab, nil
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
	relative := ""
	switch {
	case root == ".":
		if target != "." {
			relative = strings.TrimPrefix(target, "./")
		}
	case root == "/":
		relative = strings.TrimPrefix(target, "/")
	case target == root:
		relative = ""
	case strings.HasPrefix(target, root+"/"):
		relative = strings.TrimPrefix(target, root+"/")
	default:
		return "", fmt.Errorf("remote path escaped virtual site root: %s", remotePath)
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

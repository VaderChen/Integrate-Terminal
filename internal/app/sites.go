package app

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
)

func (a *App) reloadSitesFromStoreLocked() error {
	sites, err := a.store.LoadSites()
	if err != nil {
		return err
	}
	a.sites = normalizeLoadedSites(sites)

	if cfg, loadErr := a.store.LoadConfig(); loadErr == nil {
		a.config = cfg
	}
	a.config.SiteFolders = sanitizeSiteFolders(a.config.SiteFolders, a.sites)
	a.config.RESTServerPort = sanitizeRESTServerPort(a.config.RESTServerPort)
	a.config.RESTServerAllowlist = sanitizeRESTServerAllowlist(a.config.RESTServerAllowlist)
	a.config.TransferRetryCount = sanitizeTransferRetryCount(a.config.TransferRetryCount)
	a.config.TransferConflictStrategy = sanitizeTransferConflictStrategy(a.config.TransferConflictStrategy)
	if a.sessionManager != nil {
		a.sessionManager.ConfigureTransferPolicy(a.config.TransferRetryCount, a.config.TransferConflictStrategy)
	}
	return nil
}

func (a *App) touchSiteLastUsedLocked(siteID string) {
	if strings.TrimSpace(siteID) == "" {
		return
	}
	for index := range a.sites {
		if a.sites[index].ID != siteID {
			continue
		}
		a.sites[index].LastUsedAt = time.Now().Format(time.RFC3339)
		_ = a.store.SaveSites(a.sites)
		return
	}
}

func (a *App) SaveSite(site model.Site) ([]model.Site, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	previousSites := cloneSites(a.sites)
	previousConfig := cloneConfig(a.config)
	site.Folder = normalizeSiteFolder(site.Folder)
	site.Tags = normalizeSiteTags(site.Tags)
	if strings.TrimSpace(site.Host) == "" {
		return cloneSites(a.sites), fmt.Errorf("host is required")
	}
	if site.Port <= 0 {
		return cloneSites(a.sites), fmt.Errorf("port must be greater than 0")
	}
	if strings.TrimSpace(site.LocalPath) == "" {
		return cloneSites(a.sites), fmt.Errorf("local path is required")
	}
	if strings.TrimSpace(site.RemotePath) == "" {
		return cloneSites(a.sites), fmt.Errorf("remote path is required")
	}
	if err := validateSiteByProtocol(site); err != nil {
		return cloneSites(a.sites), err
	}

	if site.ID == "" {
		site.ID = fmt.Sprintf("site-%d", time.Now().UnixNano())
	}
	if site.Name == "" {
		site.Name = site.Host
	}
	for _, existing := range a.sites {
		if existing.ID == site.ID {
			site.LastUsedAt = existing.LastUsedAt
			break
		}
	}
	a.config.SiteFolders = upsertSiteFolder(a.config.SiteFolders, site.Folder)

	replaced := false
	for i := range a.sites {
		if a.sites[i].ID == site.ID {
			a.sites[i] = site
			replaced = true
			break
		}
	}
	if !replaced {
		a.sites = append(a.sites, site)
	}

	if err := a.store.SaveSites(a.sites); err != nil {
		a.sites = previousSites
		a.config = previousConfig
		return enrichSites(a.sites), err
	}
	if err := a.store.SaveConfig(a.config); err != nil {
		a.sites = previousSites
		a.config = previousConfig
		_ = a.store.SaveSites(previousSites)
		return enrichSites(a.sites), err
	}
	return enrichSites(a.sites), nil
}

func (a *App) DeleteSite(id string) ([]model.Site, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	previousSites := cloneSites(a.sites)
	filtered := make([]model.Site, 0, len(a.sites))
	for _, site := range a.sites {
		if site.ID != id {
			filtered = append(filtered, site)
		}
	}
	if len(filtered) == len(a.sites) {
		return enrichSites(a.sites), fmt.Errorf("site not found: %s", id)
	}
	a.sites = filtered
	if err := a.store.SaveSites(a.sites); err != nil {
		a.sites = previousSites
		return enrichSites(a.sites), err
	}
	return enrichSites(a.sites), nil
}

func (a *App) SortSitesByName() ([]model.Site, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	previousSites := cloneSites(a.sites)
	sort.SliceStable(a.sites, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(a.sites[i].Name))
		right := strings.ToLower(strings.TrimSpace(a.sites[j].Name))
		if left == "" {
			left = strings.ToLower(strings.TrimSpace(a.sites[i].Host))
		}
		if right == "" {
			right = strings.ToLower(strings.TrimSpace(a.sites[j].Host))
		}
		return left < right
	})

	if err := a.store.SaveSites(a.sites); err != nil {
		a.sites = previousSites
		return enrichSites(a.sites), err
	}
	return enrichSites(a.sites), nil
}

func (a *App) CreateSiteFolder(name string) (model.Config, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	previousConfig := cloneConfig(a.config)
	folder := normalizeSiteFolder(name)
	if folder == "" {
		return cloneConfig(a.config), fmt.Errorf("folder name is required")
	}
	a.config.SiteFolders = upsertSiteFolder(a.config.SiteFolders, folder)
	if err := a.store.SaveConfig(a.config); err != nil {
		a.config = previousConfig
		return cloneConfig(a.config), err
	}
	return cloneConfig(a.config), nil
}

func (a *App) SortSiteFolders() (model.Config, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	previousConfig := cloneConfig(a.config)
	a.config.SiteFolders = sanitizeSiteFolders(a.config.SiteFolders, a.sites)
	sort.SliceStable(a.config.SiteFolders, func(i, j int) bool {
		return strings.ToLower(a.config.SiteFolders[i]) < strings.ToLower(a.config.SiteFolders[j])
	})
	if err := a.store.SaveConfig(a.config); err != nil {
		a.config = previousConfig
		return cloneConfig(a.config), err
	}
	return cloneConfig(a.config), nil
}

func (a *App) RenameSiteFolder(name string, nextName string) (model.SiteLibraryMutationResult, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	previousSites := cloneSites(a.sites)
	previousConfig := cloneConfig(a.config)
	folder := normalizeSiteFolder(name)
	renamedFolder := normalizeSiteFolder(nextName)
	result := model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: cloneConfig(a.config)}
	if folder == "" || renamedFolder == "" {
		return result, fmt.Errorf("folder name is required")
	}
	if strings.EqualFold(folder, renamedFolder) {
		return result, nil
	}
	for _, existing := range a.config.SiteFolders {
		if strings.EqualFold(normalizeSiteFolder(existing), renamedFolder) {
			return result, fmt.Errorf("folder already exists: %s", renamedFolder)
		}
	}

	replaced := false
	for index, existing := range a.config.SiteFolders {
		if strings.EqualFold(normalizeSiteFolder(existing), folder) {
			a.config.SiteFolders[index] = renamedFolder
			replaced = true
		}
	}
	if !replaced {
		a.config.SiteFolders = append(a.config.SiteFolders, renamedFolder)
	}
	for index := range a.sites {
		if strings.EqualFold(normalizeSiteFolder(a.sites[index].Folder), folder) {
			a.sites[index].Folder = renamedFolder
		}
	}
	a.config.SiteFolders = sanitizeSiteFolders(a.config.SiteFolders, a.sites)

	if err := a.store.SaveSites(a.sites); err != nil {
		a.sites = previousSites
		a.config = previousConfig
		return model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: cloneConfig(a.config)}, err
	}
	if err := a.store.SaveConfig(a.config); err != nil {
		a.sites = previousSites
		a.config = previousConfig
		_ = a.store.SaveSites(previousSites)
		return model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: cloneConfig(a.config)}, err
	}
	return model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: cloneConfig(a.config)}, nil
}

func (a *App) ReorderSiteFolders(folderNames []string) (model.Config, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	previousConfig := cloneConfig(a.config)
	currentFolders := sanitizeSiteFolders(a.config.SiteFolders, a.sites)
	if len(folderNames) != len(currentFolders) {
		return cloneConfig(a.config), fmt.Errorf("site folder reorder length mismatch")
	}

	currentByKey := make(map[string]string, len(currentFolders))
	for _, folder := range currentFolders {
		currentByKey[strings.ToLower(folder)] = folder
	}

	reordered := make([]string, 0, len(currentFolders))
	seen := make(map[string]struct{}, len(currentFolders))
	for _, folder := range folderNames {
		normalized := normalizeSiteFolder(folder)
		key := strings.ToLower(normalized)
		existing, ok := currentByKey[key]
		if !ok {
			return cloneConfig(a.config), fmt.Errorf("site folder not found: %s", folder)
		}
		if _, duplicated := seen[key]; duplicated {
			return cloneConfig(a.config), fmt.Errorf("duplicate site folder: %s", folder)
		}
		seen[key] = struct{}{}
		reordered = append(reordered, existing)
	}

	a.config.SiteFolders = reordered
	if err := a.store.SaveConfig(a.config); err != nil {
		a.config = previousConfig
		return cloneConfig(a.config), err
	}
	return cloneConfig(a.config), nil
}

func (a *App) DeleteSiteFolder(name string) (model.SiteLibraryMutationResult, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	previousSites := cloneSites(a.sites)
	previousConfig := cloneConfig(a.config)
	folder := normalizeSiteFolder(name)
	result := model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: cloneConfig(a.config)}
	if folder == "" {
		return result, fmt.Errorf("folder name is required")
	}

	filteredFolders := make([]string, 0, len(a.config.SiteFolders))
	for _, existing := range a.config.SiteFolders {
		normalized := normalizeSiteFolder(existing)
		if normalized == "" || strings.EqualFold(normalized, folder) {
			continue
		}
		filteredFolders = append(filteredFolders, normalized)
	}
	a.config.SiteFolders = filteredFolders
	for index := range a.sites {
		if strings.EqualFold(normalizeSiteFolder(a.sites[index].Folder), folder) {
			a.sites[index].Folder = ""
		}
	}

	if err := a.store.SaveSites(a.sites); err != nil {
		a.sites = previousSites
		a.config = previousConfig
		return model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: cloneConfig(a.config)}, err
	}
	if err := a.store.SaveConfig(a.config); err != nil {
		a.sites = previousSites
		a.config = previousConfig
		_ = a.store.SaveSites(previousSites)
		return model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: cloneConfig(a.config)}, err
	}
	return model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: cloneConfig(a.config)}, nil
}

func enrichSites(sites []model.Site) []model.Site {
	enriched := make([]model.Site, len(sites))
	for index, site := range sites {
		enriched[index] = enrichSite(site)
	}
	return enriched
}

func enrichSite(site model.Site) model.Site {
	switch site.Protocol {
	case "sftp":
		site.ProtocolLabel = "ssh/sftp"
		site.SupportedModes = []string{"ssh", "sftp"}
		site.PrimaryFileProtocol = "sftp"
		site.PrimaryTerminalProtocol = "ssh"
	case "ftp":
		site.ProtocolLabel = "telnet/ftp"
		site.SupportedModes = []string{"telnet", "ftp"}
		site.PrimaryFileProtocol = "ftp"
		site.PrimaryTerminalProtocol = "telnet"
	default:
		site.ProtocolLabel = site.Protocol
	}
	return site
}

func normalizeSiteFolder(folder string) string {
	return strings.TrimSpace(folder)
}

func normalizeLoadedSites(sites []model.Site) []model.Site {
	normalized := make([]model.Site, len(sites))
	for index, site := range sites {
		site.Folder = normalizeSiteFolder(site.Folder)
		site.Tags = normalizeSiteTags(site.Tags)
		normalized[index] = site
	}
	return normalized
}

func sitesEqualByStoredFields(left []model.Site, right []model.Site) bool {
	return reflect.DeepEqual(left, right)
}

func normalizeSiteTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func upsertSiteFolder(folders []string, folder string) []string {
	folder = normalizeSiteFolder(folder)
	if folder == "" {
		return sanitizeSiteFolders(folders, nil)
	}
	for _, existing := range folders {
		if strings.EqualFold(normalizeSiteFolder(existing), folder) {
			return sanitizeSiteFolders(folders, nil)
		}
	}
	return sanitizeSiteFolders(append(folders, folder), nil)
}

func sanitizeSiteFolders(folders []string, sites []model.Site) []string {
	seen := make(map[string]struct{}, len(folders)+len(sites))
	sanitized := make([]string, 0, len(folders)+len(sites))
	appendFolder := func(name string) {
		trimmed := normalizeSiteFolder(name)
		if trimmed == "" {
			return
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		sanitized = append(sanitized, trimmed)
	}
	for _, folder := range folders {
		appendFolder(folder)
	}
	for _, site := range sites {
		appendFolder(site.Folder)
	}
	return sanitized
}

func (a *App) ReorderSites(siteIDs []string) ([]model.Site, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	previousSites := cloneSites(a.sites)
	if len(siteIDs) != len(a.sites) {
		return cloneSites(a.sites), fmt.Errorf("site reorder length mismatch")
	}

	siteByID := make(map[string]model.Site, len(a.sites))
	for _, site := range a.sites {
		siteByID[site.ID] = site
	}

	reordered := make([]model.Site, 0, len(a.sites))
	seen := make(map[string]struct{}, len(a.sites))
	for _, siteID := range siteIDs {
		site, ok := siteByID[siteID]
		if !ok {
			return cloneSites(a.sites), fmt.Errorf("site not found: %s", siteID)
		}
		if _, duplicated := seen[siteID]; duplicated {
			return cloneSites(a.sites), fmt.Errorf("duplicate site id: %s", siteID)
		}
		seen[siteID] = struct{}{}
		reordered = append(reordered, site)
	}

	a.sites = reordered
	if err := a.store.SaveSites(a.sites); err != nil {
		a.sites = previousSites
		return enrichSites(a.sites), err
	}
	return enrichSites(a.sites), nil
}

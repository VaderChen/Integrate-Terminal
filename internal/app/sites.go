package app

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"IntegTERM/internal/model"
	"IntegTERM/internal/store"
)

func (a *App) reloadSitesFromStoreLocked() error {
	return a.store.WithTransaction(func(tx *store.Transaction) error {
		sites, err := tx.LoadSites()
		if err != nil {
			return err
		}
		cfg, err := tx.LoadConfig()
		if err != nil {
			return err
		}
		a.sites = normalizeLoadedSites(sites)
		a.config.SiteFolders = sanitizeSiteFolders(cfg.SiteFolders, a.sites)
		return nil
	})
}

func (a *App) saveSiteLocked(site model.Site) ([]model.Site, error) {
	site.Folder = normalizeSiteFolder(site.Folder)
	if strings.TrimSpace(site.Host) == "" {
		return a.sites, fmt.Errorf("host is required")
	}
	if site.Port <= 0 {
		return a.sites, fmt.Errorf("port must be greater than 0")
	}
	if strings.TrimSpace(site.LocalPath) == "" {
		return a.sites, fmt.Errorf("local path is required")
	}
	if strings.TrimSpace(site.RemotePath) == "" {
		return a.sites, fmt.Errorf("remote path is required")
	}
	if err := validateSiteByProtocol(site); err != nil {
		return a.sites, err
	}

	if site.ID == "" {
		site.ID = fmt.Sprintf("site-%d", time.Now().UnixNano())
	}
	if site.Name == "" {
		site.Name = site.Host
	}
	site.LastUsedAt = time.Now().Format(time.RFC3339)
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

	return enrichSites(a.sites), nil
}

func (a *App) deleteSiteLocked(id string) ([]model.Site, error) {
	filtered := make([]model.Site, 0, len(a.sites))
	for _, site := range a.sites {
		if site.ID != id {
			filtered = append(filtered, site)
		}
	}
	a.sites = filtered
	return enrichSites(a.sites), nil
}

func (a *App) sortSitesByNameLocked() ([]model.Site, error) {
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

	return enrichSites(a.sites), nil
}

func (a *App) createSiteFolderLocked(name string) (model.Config, error) {
	folder := normalizeSiteFolder(name)
	if folder == "" {
		return a.config, fmt.Errorf("folder name is required")
	}
	a.config.SiteFolders = upsertSiteFolder(a.config.SiteFolders, folder)
	return a.config, nil
}

func (a *App) sortSiteFoldersLocked() (model.Config, error) {
	a.config.SiteFolders = sanitizeSiteFolders(a.config.SiteFolders, a.sites)
	sort.SliceStable(a.config.SiteFolders, func(i, j int) bool {
		return strings.ToLower(a.config.SiteFolders[i]) < strings.ToLower(a.config.SiteFolders[j])
	})
	return a.config, nil
}

func (a *App) renameSiteFolderLocked(name string, nextName string) (model.SiteLibraryMutationResult, error) {
	folder := normalizeSiteFolder(name)
	renamedFolder := normalizeSiteFolder(nextName)
	result := model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: a.config}
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

	return model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: a.config}, nil
}

func (a *App) reorderSiteFoldersLocked(folderNames []string) (model.Config, error) {
	currentFolders := sanitizeSiteFolders(a.config.SiteFolders, a.sites)
	if len(folderNames) != len(currentFolders) {
		return a.config, fmt.Errorf("site folder reorder length mismatch")
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
			return a.config, fmt.Errorf("site folder not found: %s", folder)
		}
		if _, duplicated := seen[key]; duplicated {
			return a.config, fmt.Errorf("duplicate site folder: %s", folder)
		}
		seen[key] = struct{}{}
		reordered = append(reordered, existing)
	}

	a.config.SiteFolders = reordered
	return a.config, nil
}

func (a *App) deleteSiteFolderLocked(name string) (model.SiteLibraryMutationResult, error) {
	folder := normalizeSiteFolder(name)
	result := model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: a.config}
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

	return model.SiteLibraryMutationResult{Sites: enrichSites(a.sites), Config: a.config}, nil
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
		normalized[index] = site
	}
	return normalized
}

func sitesEqualByStoredFields(left []model.Site, right []model.Site) bool {
	return reflect.DeepEqual(left, right)
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

func (a *App) reorderSitesLocked(siteIDs []string) ([]model.Site, error) {
	if len(siteIDs) != len(a.sites) {
		return a.sites, fmt.Errorf("site reorder length mismatch")
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
			return a.sites, fmt.Errorf("site not found: %s", siteID)
		}
		if _, duplicated := seen[siteID]; duplicated {
			return a.sites, fmt.Errorf("duplicate site id: %s", siteID)
		}
		seen[siteID] = struct{}{}
		reordered = append(reordered, site)
	}

	a.sites = reordered
	return a.sites, nil
}

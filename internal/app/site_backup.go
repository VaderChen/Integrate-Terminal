package app

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/VaderChen/Integrate-Terminal/internal/model"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	siteBackupFormat          = "integterm-site-library"
	siteBackupVersion         = 1
	siteBackupManifestName    = "manifest.json"
	siteBackupSitesName       = "sites.json"
	siteBackupFoldersName     = "site-folders.json"
	maxSiteBackupManifestSize = 64 << 10
	maxSiteBackupSitesSize    = 16 << 20
	maxSiteBackupFoldersSize  = 1 << 20
)

type siteBackupManifest struct {
	Format    string `json:"format"`
	Version   int    `json:"version"`
	CreatedAt string `json:"createdAt"`
}

func (a *App) GetSiteDataDirectory() string {
	return filepath.Clean(a.store.BaseDir())
}

func (a *App) OpenSiteDataDirectory() error {
	if err := a.store.Ensure(); err != nil {
		return err
	}
	return a.OpenLocalPath(a.GetSiteDataDirectory())
}

func (a *App) BackupSiteLibrary() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("desktop context is not available")
	}

	sites, err := a.store.LoadSites()
	if err != nil {
		return "", fmt.Errorf("load sites for backup: %w", err)
	}
	config, err := a.store.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("load site folders for backup: %w", err)
	}

	targetPath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "備份站台資料",
		DefaultFilename: fmt.Sprintf("integterm-sites-%s.zip", time.Now().Format("20060102-150405")),
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "ZIP Archive", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(targetPath) == "" {
		return "", nil
	}
	if !strings.HasSuffix(strings.ToLower(targetPath), ".zip") {
		targetPath += ".zip"
	}

	if err := writeSiteBackupArchive(targetPath, sites, sanitizeSiteFolders(config.SiteFolders, sites)); err != nil {
		return "", err
	}
	return targetPath, nil
}

func (a *App) RestoreSiteLibraryBackup() (*model.SiteLibraryMutationResult, error) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if a.ctx == nil {
		return nil, fmt.Errorf("desktop context is not available")
	}

	sourcePath, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "還原站台資料",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "ZIP Archive", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sourcePath) == "" {
		return nil, nil
	}

	restoredSites, restoredFolders, err := readSiteBackupArchive(sourcePath)
	if err != nil {
		return nil, err
	}
	restoredSites = normalizeLoadedSites(restoredSites)

	previousSites, err := a.store.LoadSites()
	if err != nil {
		return nil, fmt.Errorf("load current sites before restore: %w", err)
	}
	currentConfig, err := a.store.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load current configuration before restore: %w", err)
	}
	nextConfig := currentConfig
	nextConfig.SiteFolders = sanitizeSiteFolders(restoredFolders, restoredSites)

	if err := a.store.SaveSites(restoredSites); err != nil {
		return nil, fmt.Errorf("save restored sites: %w", err)
	}
	if err := a.store.SaveConfig(nextConfig); err != nil {
		_ = a.store.SaveSites(previousSites)
		return nil, fmt.Errorf("save restored site folders: %w", err)
	}

	a.sites = restoredSites
	a.config = nextConfig
	return &model.SiteLibraryMutationResult{
		Sites:  enrichSites(a.sites),
		Config: cloneConfig(a.config),
	}, nil
}

func writeSiteBackupArchive(targetPath string, sites []model.Site, folders []string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".integterm-sites-*.zip")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if err := tempFile.Chmod(0o600); err != nil {
		_ = tempFile.Close()
		return err
	}

	archive := zip.NewWriter(tempFile)
	manifest := siteBackupManifest{
		Format:    siteBackupFormat,
		Version:   siteBackupVersion,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeSiteBackupJSON(archive, siteBackupManifestName, manifest); err != nil {
		_ = archive.Close()
		_ = tempFile.Close()
		return err
	}
	if err := writeSiteBackupJSON(archive, siteBackupSitesName, sites); err != nil {
		_ = archive.Close()
		_ = tempFile.Close()
		return err
	}
	if err := writeSiteBackupJSON(archive, siteBackupFoldersName, folders); err != nil {
		_ = archive.Close()
		_ = tempFile.Close()
		return err
	}
	if err := archive.Close(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, targetPath)
}

func writeSiteBackupJSON(archive *zip.Writer, name string, value any) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o600)
	header.SetModTime(time.Now())
	entry, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(entry)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func readSiteBackupArchive(sourcePath string) ([]model.Site, []string, error) {
	archive, err := zip.OpenReader(sourcePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open site backup: %w", err)
	}
	defer archive.Close()

	entries := make(map[string]*zip.File, len(archive.File))
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.Name != siteBackupManifestName && entry.Name != siteBackupSitesName && entry.Name != siteBackupFoldersName {
			continue
		}
		if _, exists := entries[entry.Name]; exists {
			return nil, nil, fmt.Errorf("site backup contains duplicate entry %q", entry.Name)
		}
		entries[entry.Name] = entry
	}

	var manifest siteBackupManifest
	if err := readSiteBackupJSON(entries[siteBackupManifestName], siteBackupManifestName, maxSiteBackupManifestSize, &manifest); err != nil {
		return nil, nil, err
	}
	if manifest.Format != siteBackupFormat {
		return nil, nil, fmt.Errorf("unsupported site backup format %q", manifest.Format)
	}
	if manifest.Version != siteBackupVersion {
		return nil, nil, fmt.Errorf("unsupported site backup version %d", manifest.Version)
	}

	var sites []model.Site
	if err := readSiteBackupJSON(entries[siteBackupSitesName], siteBackupSitesName, maxSiteBackupSitesSize, &sites); err != nil {
		return nil, nil, err
	}
	if sites == nil {
		sites = []model.Site{}
	}

	var folders []string
	if err := readSiteBackupJSON(entries[siteBackupFoldersName], siteBackupFoldersName, maxSiteBackupFoldersSize, &folders); err != nil {
		return nil, nil, err
	}
	if folders == nil {
		folders = []string{}
	}
	return sites, folders, nil
}

func readSiteBackupJSON(entry *zip.File, name string, maxSize int64, target any) error {
	if entry == nil {
		return fmt.Errorf("site backup is missing %s", name)
	}
	if entry.UncompressedSize64 > uint64(maxSize) {
		return fmt.Errorf("site backup entry %s is too large", name)
	}

	reader, err := entry.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxSize+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maxSize {
		return fmt.Errorf("site backup entry %s is too large", name)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	return nil
}

import type { Dispatch, SetStateAction } from 'react';
import type { SiteFolderDialogState } from '../appTypes';
import { buildBlankSite, canSaveSite, extractErrorMessage } from '../appUtils';
import type { Config, Site } from '../types';

type Params = {
  t: { connectionFailed: string; siteCopySuffix: string };
  defaultLocalPath: string;
  sites: Site[];
  draftSite: Site;
  draftSiteBaseline: Site;
  siteFolderName: string;
  siteFolderDialog: SiteFolderDialogState | null;
  setSites: Dispatch<SetStateAction<Site[]>>;
  setConfig: Dispatch<SetStateAction<Config>>;
  setDraftSite: Dispatch<SetStateAction<Site>>;
  setDraftSiteBaseline: Dispatch<SetStateAction<Site>>;
  setSiteEditorOpen: Dispatch<SetStateAction<boolean>>;
  setFormExpanded: Dispatch<SetStateAction<boolean>>;
  setErrorMessage: Dispatch<SetStateAction<string>>;
  setSiteFolderName: Dispatch<SetStateAction<string>>;
  setSiteFolderDialog: Dispatch<SetStateAction<SiteFolderDialogState | null>>;
};

export function useSiteLibraryActions({
  t,
  defaultLocalPath,
  sites,
  draftSite,
  draftSiteBaseline,
  siteFolderName,
  siteFolderDialog,
  setSites,
  setConfig,
  setDraftSite,
  setDraftSiteBaseline,
  setSiteEditorOpen,
  setFormExpanded,
  setErrorMessage,
  setSiteFolderName,
  setSiteFolderDialog,
}: Params) {
  const handleOpenNewSiteDialog = () => {
    const blankSite = buildBlankSite(defaultLocalPath);
    setDraftSite(blankSite);
    setDraftSiteBaseline(blankSite);
    setFormExpanded(true);
    setSiteEditorOpen(true);
  };

  const handleOpenEditSiteDialog = (site: Site) => {
    setDraftSite(site);
    setDraftSiteBaseline(site);
    setFormExpanded(true);
    setSiteEditorOpen(true);
  };

  const handleCloseSiteEditor = () => {
    setDraftSite(draftSiteBaseline);
    setFormExpanded(true);
    setSiteEditorOpen(false);
  };

  const handleSaveSite = async () => {
    if (!canSaveSite(draftSite)) return;
    try {
      const nextSites = await window.go?.app?.App?.SaveSite?.(draftSite);
      if (!nextSites) return;
      setSites(nextSites);
      setConfig((current) => ({ ...current, siteFolders: mergeSiteFolders(current.siteFolders, nextSites) }));
      const blankSite = buildBlankSite(defaultLocalPath);
      setDraftSite(blankSite);
      setDraftSiteBaseline(blankSite);
      setSiteEditorOpen(false);
      setFormExpanded(false);
      setErrorMessage('');
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleDeleteSite = async (siteId: string) => {
    const nextSites = await window.go?.app?.App?.DeleteSite?.(siteId);
    if (nextSites) setSites(nextSites);
  };

  const handleCopySite = async (site: Site) => {
    const cloneName = site.name?.trim() ? `${site.name} ${t.siteCopySuffix}` : `${site.host} ${t.siteCopySuffix}`;
    const nextSites = await window.go?.app?.App?.SaveSite?.({ ...site, id: '', name: cloneName, lastUsedAt: '' });
    if (nextSites) {
      setSites(nextSites);
      setConfig((current) => ({ ...current, siteFolders: mergeSiteFolders(current.siteFolders, nextSites) }));
    }
  };

  const handleToggleFavorite = async (siteId: string) => {
    const site = sites.find((item) => item.id === siteId);
    if (!site) return;
    try {
      const nextSites = await window.go?.app?.App?.SaveSite?.({
        ...site,
        tags: site.tags ?? [],
        favorite: !(site.favorite ?? false),
      });
      if (nextSites) {
        setSites(nextSites);
        setConfig((current) => ({ ...current, siteFolders: mergeSiteFolders(current.siteFolders, nextSites) }));
      }
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleSortSitesByName = async () => {
    const nextSites = await window.go?.app?.App?.SortSitesByName?.();
    if (nextSites) setSites(nextSites);
  };

  const handleOpenCreateSiteFolder = () => {
    setSiteFolderName('');
    setSiteFolderDialog({ mode: 'create' });
  };

  const handlePromptRenameSiteFolder = (folder: string) => {
    setSiteFolderName(folder);
    setSiteFolderDialog({ mode: 'rename', folder });
  };

  const handleCreateSiteFolder = async () => {
    const name = siteFolderName.trim();
    if (!name) return;
    try {
      const nextConfig = await window.go?.app?.App?.CreateSiteFolder?.(name);
      if (nextConfig) {
        setConfig(nextConfig);
        setSiteFolderDialog(null);
        setSiteFolderName('');
        setErrorMessage('');
      }
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleSortSiteFolders = async () => {
    try {
      const nextConfig = await window.go?.app?.App?.SortSiteFolders?.();
      if (nextConfig) {
        setConfig(nextConfig);
        setErrorMessage('');
      }
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handlePromptDeleteSiteFolder = (folder: string, siteCount: number) => {
    setSiteFolderDialog({ mode: 'delete', folder, siteCount });
  };

  const handleConfirmSiteFolderDialog = async () => {
    if (!siteFolderDialog) return;
    if (siteFolderDialog.mode === 'create') {
      await handleCreateSiteFolder();
      return;
    }
    if (siteFolderDialog.mode === 'rename') {
      try {
        const result = await window.go?.app?.App?.RenameSiteFolder?.(siteFolderDialog.folder, siteFolderName.trim());
        if (result) {
          setSites(result.sites);
          setConfig(result.config);
          if (draftSite.folder?.trim() === siteFolderDialog.folder.trim()) {
            const nextDraft = { ...draftSite, folder: siteFolderName.trim() };
            setDraftSite(nextDraft);
            setDraftSiteBaseline(nextDraft);
          }
        }
        setSiteFolderDialog(null);
        setSiteFolderName('');
        setErrorMessage('');
      } catch (error) {
        setErrorMessage(extractErrorMessage(error, t.connectionFailed));
      }
      return;
    }
    try {
      const result = await window.go?.app?.App?.DeleteSiteFolder?.(siteFolderDialog.folder);
      if (result) {
        setSites(result.sites);
        setConfig(result.config);
      }
      setSiteFolderDialog(null);
      setSiteFolderName('');
      setErrorMessage('');
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleReorderSites = async (siteIDs: string[]) => {
    const nextSites = await window.go?.app?.App?.ReorderSites?.(siteIDs);
    if (nextSites) setSites(nextSites);
  };

  const handleMoveSiteToFolder = async (siteId: string, folder: string) => {
    const site = sites.find((item) => item.id === siteId);
    if (!site) return;
    const nextFolder = folder.trim();
    if ((site.folder?.trim() ?? '') === nextFolder) return;
    try {
      const nextSites = await window.go?.app?.App?.SaveSite?.({ ...site, folder: nextFolder });
      if (nextSites) {
        setSites(nextSites);
        setConfig((current) => ({ ...current, siteFolders: mergeSiteFolders(current.siteFolders, nextSites) }));
        if (draftSite.id === siteId) {
          const movedSite = nextSites.find((item) => item.id === siteId);
          if (movedSite) {
            setDraftSite(movedSite);
            setDraftSiteBaseline(movedSite);
          }
        }
      }
      setErrorMessage('');
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleReorderSiteFolders = async (folderNames: string[]) => {
    try {
      const nextConfig = await window.go?.app?.App?.ReorderSiteFolders?.(folderNames);
      if (nextConfig) {
        setConfig(nextConfig);
        setErrorMessage('');
      }
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  return {
    handleOpenNewSiteDialog,
    handleOpenEditSiteDialog,
    handleCloseSiteEditor,
    handleSaveSite,
    handleDeleteSite,
    handleCopySite,
    handleToggleFavorite,
    handleSortSitesByName,
    handleOpenCreateSiteFolder,
    handlePromptRenameSiteFolder,
    handleSortSiteFolders,
    handlePromptDeleteSiteFolder,
    handleConfirmSiteFolderDialog,
    handleReorderSites,
    handleMoveSiteToFolder,
    handleReorderSiteFolders,
  };
}

function mergeSiteFolders(currentFolders: string[], sites: Site[]) {
  const merged = [...currentFolders];
  const seen = new Set(currentFolders.map((folder) => folder.trim().toLowerCase()).filter(Boolean));
  for (const site of sites) {
    const folder = site.folder?.trim() ?? '';
    if (!folder) continue;
    const key = folder.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    merged.push(folder);
  }
  return merged;
}

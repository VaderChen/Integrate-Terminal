import type { Site } from '../types';

export type SiteFolderGroup = {
  key: string;
  label: string;
  folderName: string;
  sites: Site[];
  canDelete: boolean;
};

export const DEFAULT_FOLDER_KEY = '__default__';

export function normalizeFolderKey(folder?: string) {
  const trimmed = folder?.trim() ?? '';
  return trimmed === '' ? DEFAULT_FOLDER_KEY : trimmed;
}

export function groupSitesByFolder(sites: Site[], siteFolders: string[], defaultLabel: string): SiteFolderGroup[] {
  const orderedFolderNames = dedupeFolders([
    ...siteFolders,
    ...sites.map((site) => site.folder ?? ''),
  ]);
  const groups = new Map<string, SiteFolderGroup>();
  groups.set(DEFAULT_FOLDER_KEY, {
    key: DEFAULT_FOLDER_KEY,
    label: defaultLabel,
    folderName: '',
    sites: [],
    canDelete: false,
  });

  for (const folderName of orderedFolderNames) {
    const trimmed = folderName.trim();
    if (trimmed === '') {
      continue;
    }
    groups.set(trimmed, {
      key: trimmed,
      label: trimmed,
      folderName: trimmed,
      sites: [],
      canDelete: true,
    });
  }

  for (const site of sites) {
    const key = normalizeFolderKey(site.folder);
    const group = groups.get(key);
    if (group) {
      group.sites.push(site);
      continue;
    }
    groups.set(key, {
      key,
      label: key === DEFAULT_FOLDER_KEY ? defaultLabel : key,
      folderName: key === DEFAULT_FOLDER_KEY ? '' : key,
      sites: [site],
      canDelete: key !== DEFAULT_FOLDER_KEY,
    });
  }

  const defaultGroup = groups.get(DEFAULT_FOLDER_KEY);
  const folderGroups = Array.from(groups.values()).filter((group) => group.key !== DEFAULT_FOLDER_KEY);
  return defaultGroup ? [defaultGroup, ...folderGroups] : folderGroups;
}

function dedupeFolders(folders: string[]) {
  const seen = new Set<string>();
  const deduped: string[] = [];
  for (const folder of folders) {
    const trimmed = folder.trim();
    if (trimmed === '') {
      continue;
    }
    const key = trimmed.toLowerCase();
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    deduped.push(trimmed);
  }
  return deduped;
}

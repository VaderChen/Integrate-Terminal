import type { FileEntry, Tab } from './types';

export type PanelLocation = Pick<Tab, 'id' | 'localPath' | 'remotePath'>;
export type PanelSnapshot = PanelLocation & { localFiles: FileEntry[]; remoteFiles: FileEntry[]; localReady: boolean; remoteReady: boolean };
export type FileDragPayload = {
  tabId: string;
  side: 'local' | 'remote';
  basePath: string;
  paths: string[];
};

export function samePanelLocation(left: PanelLocation | null, right: PanelLocation | null) {
  return !!left && !!right && left.id === right.id
    && left.localPath === right.localPath && left.remotePath === right.remotePath;
}

export function sameConnection(left: Tab | undefined | null, right: Tab) {
  return !!left && left.id === right.id && left.host === right.host && left.port === right.port
    && left.username === right.username && left.protocol === right.protocol && left.sessionId === right.sessionId;
}

function normalizePath(value: string) {
  const parts: string[] = [];
  const normalized = value.replace(/\\/g, '/');
  const absolute = normalized.startsWith('/');
  for (const part of normalized.split('/')) {
    if (!part || part === '.') continue;
    if (part === '..') {
      if (parts.length && parts[parts.length - 1] !== '..') parts.pop();
      else if (!absolute) parts.push(part);
    } else parts.push(part);
  }
  return (absolute ? '/' : '') + parts.join('/');
}

export function isActionableEntry(entry: FileEntry) {
  return entry.name !== '..' && entry.name !== '.' && entry.path.length > 0;
}

export function isPathInside(path: string, directory: string) {
  const base = normalizePath(directory);
  const target = normalizePath(path);
  if (target === base || target.startsWith('/') !== base.startsWith('/')) return false;
  const parentDepth = (value: string) => value.match(/^(?:\.\.(?:\/|$))*/)?.[0].split('/').filter(Boolean).length ?? 0;
  if (parentDepth(target) !== parentDepth(base)) return false;
  if (!base) return target.length > 0 && target !== '..' && !target.startsWith('../');
  return target.startsWith(base === '/' ? '/' : `${base}/`);
}

export function actionableEntries(entries: FileEntry[], side: 'local' | 'remote', basePath: string) {
  return entries.filter(entry => isActionableEntry(entry) && entry.side === side && isPathInside(entry.path, basePath));
}

export function decodeFileDrag(value: string): FileDragPayload | null {
  try {
    const item = JSON.parse(value) as FileDragPayload;
    if (!item || typeof item.tabId !== 'string' || typeof item.basePath !== 'string'
      || (item.side !== 'local' && item.side !== 'remote') || !Array.isArray(item.paths)) return null;
    if (!item.paths.length || item.paths.some(path => typeof path !== 'string' || !isPathInside(path, item.basePath))) return null;
    return { ...item, paths: [...new Set(item.paths)] };
  } catch {
    return null;
  }
}

export function resolveFileDrag(value: string, location: PanelLocation, side: 'local' | 'remote') {
  const payload = decodeFileDrag(value);
  const basePath = side === 'local' ? location.localPath : location.remotePath;
  return payload?.tabId === location.id && payload.side === side && payload.basePath === basePath ? payload.paths : null;
}

export function createPanelLoader(options: {
  getActiveTab: () => Tab | null;
  readLocal: (tab: Tab) => Promise<FileEntry[]>;
  readRemote: (tab: Tab) => Promise<FileEntry[]>;
  onSnapshot: (snapshot: PanelSnapshot | null) => void;
  onError: (error: unknown) => void;
}) {
  let generation = 0;
  const invalidate = () => {
    generation += 1;
    options.onSnapshot(null);
  };
  const load = async (tab: Tab | null) => {
    if (!samePanelLocation(options.getActiveTab(), tab)) return;
    invalidate();
    if (!tab || tab.mode === 'terminal') return;
    const request = generation;
    let snapshot: PanelSnapshot = {
      id: tab.id, localPath: tab.localPath, remotePath: tab.remotePath,
      localFiles: [], remoteFiles: [], localReady: false, remoteReady: false,
    };
    const current = () => request === generation && samePanelLocation(options.getActiveTab(), tab);
    const readSide = async (side: 'local' | 'remote') => {
      if (side === 'remote' && !tab.connected) return;
      try {
        const files = await (side === 'local' ? options.readLocal(tab) : options.readRemote(tab));
        if (!current()) return;
        snapshot = side === 'local'
          ? { ...snapshot, localFiles: files, localReady: true }
          : { ...snapshot, remoteFiles: files, remoteReady: true };
        options.onSnapshot(snapshot);
      } catch (error) {
        if (current()) options.onError(error);
      }
    };
    await Promise.all([readSide('local'), readSide('remote')]);
  };
  return { load, invalidate };
}

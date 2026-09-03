import type { BootstrapPayload, FileEntry, FileSortState, HostTrustPrompt, Site } from './types';

export const fallbackBootstrap: BootstrapPayload = {
  sites: [],
  tabs: [],
  config: {
    windowWidth: 1440,
    windowHeight: 920,
    windowX: 0,
    windowY: 0,
    lastActiveTab: '',
    restoreTabsOnStart: true,
    closeTerminalTabOnDisconnect: true,
    showHiddenFiles: false,
    showTrayIcon: false,
    rememberWindowPosition: false,
    telnetLocalEcho: true,
    restServerEnabled: false,
    restServerPort: 18080,
    restServerAllowlist: ['127.0.0.1'],
    fontScale: 'medium',
    language: '',
    theme: 'neutral',
    siteFolders: [],
    transferRetryCount: 2,
    transferConflictStrategy: 'overwrite',
  },
  defaultLocalPath: '/',
  localFiles: [],
  remoteFiles: [],
  transfers: [],
  logs: [],
};

export function buildBlankSite(defaultLocalPath: string): Site {
  return {
    id: '',
    name: '',
    folder: '',
    protocol: 'sftp',
    host: '',
    port: 22,
    username: '',
    password: '',
    ppkPath: '',
    ppkPassphrase: '',
    localPath: defaultLocalPath,
    remotePath: '/',
    lastUsedAt: '',
    tags: [],
    favorite: false,
  };
}

export function canSaveSite(site: Site) {
  const hasBaseFields =
    site.host.trim() !== '' &&
    site.port > 0 &&
    site.localPath.trim() !== '' &&
    site.remotePath.trim() !== '';

  if (!hasBaseFields) return false;

  if (site.protocol === 'ftp') {
    return site.username.trim() !== '';
  }

  return site.username.trim() !== '' && (site.password.trim() !== '' || site.ppkPath.trim() !== '');
}

export function extractErrorMessage(error: unknown, fallback = 'Connection failed') {
  if (error instanceof Error && error.message) {
    return error.message;
  }

  if (typeof error === 'string' && error.trim()) {
    return error;
  }

  if (typeof error === 'object' && error !== null) {
    const maybeMessage = Reflect.get(error, 'message');
    if (typeof maybeMessage === 'string' && maybeMessage.trim()) {
      return maybeMessage;
    }

    try {
      const serialized = JSON.stringify(error);
      if (serialized && serialized !== '{}') {
        return serialized;
      }
    } catch {
      return fallback;
    }
  }

  return fallback;
}

export function appendLocalNetworkHint(message: string, hint: string) {
  const normalized = message.toLowerCase();
  if (!normalized.includes('no route to host') && !normalized.includes('network is unreachable')) {
    return message;
  }
  return `${message} ${hint}`;
}

export function extractHostTrustPrompt(error: unknown): HostTrustPrompt | null {
  const value = typeof error === 'string'
    ? error
    : error instanceof Error
      ? error.message
      : typeof error === 'object' && error !== null && typeof Reflect.get(error, 'message') === 'string'
        ? String(Reflect.get(error, 'message'))
        : '';

  if (!value.startsWith('HOST_TRUST_REQUIRED:')) {
    return null;
  }

  try {
    const payload = JSON.parse(value.slice('HOST_TRUST_REQUIRED:'.length)) as HostTrustPrompt;
    if (
      typeof payload.host === 'string' &&
      typeof payload.port === 'number' &&
      typeof payload.hostPattern === 'string' &&
      typeof payload.keyType === 'string' &&
      typeof payload.fingerprintSHA256 === 'string' &&
      typeof payload.authorizedKey === 'string'
    ) {
      return payload;
    }
  } catch {
    return null;
  }

  return null;
}

export function withParentEntry(entries: FileEntry[], currentPath: string, side: 'local' | 'remote') {
  const parentPath = getParentPath(currentPath, side);
  if (!parentPath || parentPath === currentPath) {
    return entries;
  }

  const parentEntry: FileEntry = {
    name: '..',
    path: parentPath,
    size: 0,
    modified: '',
    isDir: true,
    side,
  };

  return [parentEntry, ...entries];
}

export function sortEntries(entries: FileEntry[], sortState: FileSortState) {
  return [...entries].sort((left, right) => {
    if (left.name === '..') return -1;
    if (right.name === '..') return 1;
    if (left.isDir !== right.isDir) {
      return left.isDir ? -1 : 1;
    }

    let comparison = 0;
    if (sortState.key === 'size') {
      comparison = left.size - right.size;
    } else if (sortState.key === 'modified') {
      comparison = left.modified.localeCompare(right.modified);
    } else {
      comparison = left.name.localeCompare(right.name);
    }

    return sortState.direction === 'asc' ? comparison : -comparison;
  });
}

export function basename(path: string) {
  const normalized = path.replace(/\\/g, '/');
  const parts = normalized.split('/');
  return parts[parts.length - 1] || path;
}

function getParentPath(currentPath: string, side: 'local' | 'remote') {
  if (!currentPath) return '';

  if (side === 'remote') {
    if (currentPath === '/' || currentPath === '.') return '';
    const normalized = currentPath.endsWith('/') && currentPath.length > 1 ? currentPath.slice(0, -1) : currentPath;
    const idx = normalized.lastIndexOf('/');
    if (idx <= 0) return '/';
    return normalized.slice(0, idx);
  }

  const normalized = currentPath.endsWith('/') && currentPath.length > 1 ? currentPath.slice(0, -1) : currentPath;
  const idx = normalized.lastIndexOf('/');
  if (idx <= 0) return '';
  return normalized.slice(0, idx);
}

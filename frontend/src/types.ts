export type Site = {
  id: string;
  name: string;
  folder: string;
  protocol: 'ftp' | 'sftp';
  host: string;
  port: number;
  username: string;
  password: string;
  ppkPath: string;
  ppkPassphrase: string;
  localPath: string;
  remotePath: string;
  lastUsedAt: string;
};

export type Tab = {
  id: string;
  siteId: string;
  title: string;
  mode: 'file' | 'terminal';
  protocol: 'ftp' | 'sftp' | 'ssh' | 'telnet' | 'local';
  host: string;
  port: number;
  username: string;
  password: string;
  ppkPath: string;
  ppkPassphrase: string;
  localPath: string;
  remotePath: string;
  sessionId: string;
  connected: boolean;
  hidden: boolean;
};

export type FileEntry = {
  name: string;
  path: string;
  size: number;
  modified: string;
  isDir: boolean;
  side: 'local' | 'remote';
};

export type FileSortKey = 'name' | 'modified' | 'size';

export type FileSortState = {
  key: FileSortKey;
  direction: 'asc' | 'desc';
};

export type TransferItem = {
  id: string;
  direction: 'upload' | 'download';
  name: string;
  progress: number;
  speedBps: number;
  status: 'running' | 'paused' | 'done' | 'failed' | 'cancelled';
};

export type LogItem = {
  id: string;
  message: string;
  status: 'running' | 'done' | 'failed';
  createdAt: string;
};

export type Config = {
  windowWidth: number;
  windowHeight: number;
  windowX: number;
  windowY: number;
  lastActiveTab: string;
  restoreTabsOnStart: boolean;
  closeTerminalTabOnDisconnect: boolean;
  showHiddenFiles: boolean;
  showTrayIcon: boolean;
  rememberWindowPosition: boolean;
  telnetLocalEcho: boolean;
  restServerEnabled: boolean;
  restServerPort: number;
  restServerAllowlist: string[];
  fontScale: 'xsmall' | 'small' | 'medium' | 'large' | 'xlarge';
  language: '' | 'zh-TW' | 'zh-CN' | 'en' | 'ja' | 'ko';
  theme: 'neutral' | 'light' | 'dark' | 'contrast';
  siteFolders: string[];
};

export type RestServerStatus = {
  enabled: boolean;
  running: boolean;
  baseURL: string;
  mcpURL: string;
  port: number;
  attached: boolean;
  allowlist: string[];
};

export type BootstrapPayload = {
  sites: Site[];
  tabs: Tab[];
  config: Config;
  defaultLocalPath: string;
  localFiles: FileEntry[];
  remoteFiles: FileEntry[];
  transfers: TransferItem[];
  logs: LogItem[];
};

export type SiteLibraryMutationResult = {
  sites: Site[];
  config: Config;
};

export type HostTrustPrompt = {
  host: string;
  port: number;
  hostPattern: string;
  keyType: string;
  fingerprintSHA256: string;
  authorizedKey: string;
};

import type { HostTrustPrompt, Site, FileEntry } from './types';
import type { FontFamilyId as TerminalFontFamilyId, FontScale as TerminalFontScale } from './components/SSHConsolePanel';

export type ActionDialogState =
  | { mode: 'mkdir'; side: 'local' | 'remote' }
  | { mode: 'rename'; side: 'local' | 'remote'; entry: FileEntry }
  | { mode: 'delete'; side: 'local' | 'remote'; entry: FileEntry; entries: FileEntry[] };

export type ConnectDialogState = {
  site: Site;
};

export type HostTrustDialogState = {
  site: Site;
  selectedMode: 'ssh' | 'sftp';
  prompt: HostTrustPrompt;
};

export type TerminalPreferences = {
  themeId: string;
  fontScale: TerminalFontScale;
  fontFamilyId: TerminalFontFamilyId;
};

export type PathContextMenuState = {
  x: number;
  y: number;
  side: 'local' | 'remote';
  path: string;
};

export type TerminalUploadConfirmState = {
  paths: string[];
  remotePath: string;
};

export type SiteFolderDialogState =
  | { mode: 'create' }
  | { mode: 'rename'; folder: string }
  | { mode: 'delete'; folder: string; siteCount: number };

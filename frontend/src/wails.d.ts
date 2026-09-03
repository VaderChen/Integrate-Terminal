declare global {
  interface Window {
    runtime?: {
      EventsOnMultiple?: (
        eventName: string,
        callback: (...data: unknown[]) => void,
        maxCallbacks: number,
      ) => () => void;
      OnFileDrop?: (
        callback: (x: number, y: number, paths: string[]) => void,
        useDropTarget: boolean,
      ) => void;
      OnFileDropOff?: () => void;
    };
    go?: {
      app?: {
        App?: {
          ApproveHost: (prompt: import('./types').HostTrustPrompt) => Promise<void>;
          Bootstrap: () => Promise<import('./types').BootstrapPayload>;
          SaveSite: (site: import('./types').Site) => Promise<import('./types').Site[]>;
          DeleteSite: (id: string) => Promise<import('./types').Site[]>;
          SortSitesByName: () => Promise<import('./types').Site[]>;
          ReorderSites: (siteIDs: string[]) => Promise<import('./types').Site[]>;
          CreateSiteFolder: (name: string) => Promise<import('./types').Config>;
          SortSiteFolders: () => Promise<import('./types').Config>;
          RenameSiteFolder: (name: string, nextName: string) => Promise<import('./types').SiteLibraryMutationResult>;
          DeleteSiteFolder: (name: string) => Promise<import('./types').SiteLibraryMutationResult>;
          ReorderSiteFolders: (folderNames: string[]) => Promise<import('./types').Config>;
          CreateTab: (site: import('./types').Site) => Promise<import('./types').Tab[]>;
          CreateSSHTab: (site: import('./types').Site) => Promise<import('./types').Tab[]>;
          CreateTelnetTab: (site: import('./types').Site) => Promise<import('./types').Tab[]>;
          CreateLocalTerminalTab: (cwd: string) => Promise<import('./types').Tab[]>;
          CloseTab: (tabID: string) => Promise<import('./types').Tab[]>;
          ApproveQuit: () => Promise<void>;
          StopBackgroundService: () => Promise<void>;
          Connect: (tabID: string) => Promise<import('./types').Tab[]>;
          Disconnect: (tabID: string) => Promise<import('./types').Tab[]>;
          ReorderTabs: (tabIDs: string[]) => Promise<import('./types').Tab[]>;
          UpdateTabPaths: (tabID: string, localPath: string, remotePath: string) => Promise<import('./types').Tab[]>;
          ListLocal: (tabID: string, path: string) => Promise<import('./types').FileEntry[]>;
          ListRemote: (tabID: string, path: string) => Promise<import('./types').FileEntry[]>;
          GetTransfers: () => Promise<import('./types').TransferItem[]>;
          GetLogs: () => Promise<import('./types').LogItem[]>;
          GetSiteDataDirectory: () => Promise<string>;
          OpenSiteDataDirectory: () => Promise<void>;
          BackupSiteLibrary: () => Promise<string>;
          RestoreSiteLibraryBackup: () => Promise<import('./types').SiteLibraryMutationResult | null>;
          SelectPPKFile: () => Promise<string>;
          SelectDirectory: () => Promise<string>;
          AuthorizeKeyDirectory: (suggestedPath: string) => Promise<string>;
          PendingKeyAuthorizations: () => Promise<string[]>;
          OpenLocalPath: (targetPath: string) => Promise<void>;
          ExecuteLocalPath: (targetPath: string) => Promise<void>;
          SaveConfig: (config: import('./types').Config) => Promise<import('./types').Config>;
          UploadDroppedPaths: (tabID: string, localPaths: string[], remoteBase: string) => Promise<void>;
          UploadDroppedPathsToSite: (site: import('./types').Site, localPaths: string[], remoteBase: string) => Promise<void>;
          DownloadDroppedPaths: (tabID: string, remotePaths: string[], localBase: string) => Promise<void>;
          CompareDirectories: (tabID: string, localPath: string, remotePath: string) => Promise<import('./types').FileComparison[]>;
          SyncDirectories: (tabID: string, localPath: string, remotePath: string, direction: 'upload' | 'download') => Promise<void>;
          MoveEntriesToDirectory: (tabID: string, side: 'local' | 'remote', sourcePaths: string[], targetDirectory: string) => Promise<void>;
          CreateDirectory: (tabID: string, side: 'local' | 'remote', basePath: string, name: string) => Promise<void>;
          RenameEntry: (tabID: string, side: 'local' | 'remote', sourcePath: string, newName: string) => Promise<void>;
          DeleteEntry: (tabID: string, side: 'local' | 'remote', targetPath: string) => Promise<void>;
          DeleteEntries: (tabID: string, side: 'local' | 'remote', targetPaths: string[]) => Promise<void>;
          ClearCompletedTransfers: () => Promise<import('./types').TransferItem[]>;
          ClearAllTransfers: () => Promise<import('./types').TransferItem[]>;
          CancelTransfer: (itemID: string) => Promise<import('./types').TransferItem[]>;
          TogglePauseTransfer: (itemID: string) => Promise<import('./types').TransferItem[]>;
          TogglePauseAllTransfers: () => Promise<import('./types').TransferItem[]>;
          ClearLogs: () => Promise<import('./types').LogItem[]>;
          StartSSHSession: (site: import('./types').Site) => Promise<string>;
          StartTelnetSession: (site: import('./types').Site) => Promise<string>;
          GetSSHOutputBuffer: (sessionID: string) => Promise<string>;
          ListSystemFonts: () => Promise<string[]>;
          WriteSSHInput: (sessionID: string, data: string) => Promise<void>;
          ResizeSSHSession: (sessionID: string, cols: number, rows: number) => Promise<void>;
          CloseSSHSession: (sessionID: string) => Promise<void>;
          ResetWindowToDefaultScale: () => Promise<void>;
          GetRestAPIDocsMarkdown: () => Promise<string>;
          GetMCPContractMarkdown: (contract: 'local' | 'network') => Promise<string>;
          ExportRestAPIDocsMarkdown: () => Promise<string>;
          ExportMCPContractMarkdown: (contract: 'local' | 'network') => Promise<string>;
          GetRESTServerStatus: () => Promise<import('./types').RestServerStatus>;
          CheckForUpdates: () => Promise<import('./types').UpdateCheckResult>;
          StartUpdate: (expectedTag: string) => Promise<import('./types').UpdateActionResult>;
        };
      };
    };
  }
}

export {};

import { useEffect, useRef } from 'react';
import type React from 'react';
import { EventsOn, OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime';
import { basename, extractErrorMessage } from '../appUtils';
import { isPathInside, sameConnection } from '../filePanelState';
import type { LogItem, Tab, TransferItem } from '../types';

type Params = {
  t: { connectionFailed: string };
  activeTabRef: React.MutableRefObject<Tab | null>;
  tabsRef: React.MutableRefObject<Tab[]>;
  canAcceptFileDrop: () => boolean;
  setTransfers: React.Dispatch<React.SetStateAction<TransferItem[]>>;
  setLogs: React.Dispatch<React.SetStateAction<LogItem[]>>;
  setErrorMessage: React.Dispatch<React.SetStateAction<string>>;
  refreshPanelsForPaths: (tab: Tab, nextLocalPath: string, nextRemotePath: string) => Promise<void>;
  onTerminalUploadStateChange?: (active: boolean) => void;
  requestTerminalDropConfirm?: (paths: string[], remotePath: string) => Promise<boolean>;
};

export function useTransferActions({
  t,
  activeTabRef,
  tabsRef,
  canAcceptFileDrop,
  setTransfers,
  setLogs,
  setErrorMessage,
  refreshPanelsForPaths,
  onTerminalUploadStateChange,
  requestTerminalDropConfirm,
}: Params) {
  const callbacks = useRef({ onTerminalUploadStateChange, requestTerminalDropConfirm, canAcceptFileDrop });
  callbacks.current = { onTerminalUploadStateChange, requestTerminalDropConfirm, canAcceptFileDrop };
  const liveTab = (tab: Tab) => tabsRef.current.find(current => sameConnection(current, tab));
  const syncTransferState = async () => {
    const [queue, nextLogs] = await Promise.all([
      window.go?.app?.App?.GetTransfers?.(),
      window.go?.app?.App?.GetLogs?.(),
    ]);
    setTransfers(queue ?? []);
    setLogs(nextLogs ?? []);
  };

  const handleClearCompletedTransfers = () => {
    void window.go?.app?.App?.ClearCompletedTransfers?.().then((items) => {
      if (items) setTransfers(items);
    });
  };

  const handleClearAllTransfers = () => {
    void window.go?.app?.App?.ClearAllTransfers?.().then((items) => {
      if (items) setTransfers(items);
    });
  };

  const handleCancelTransfer = (itemID: string) => {
    void window.go?.app?.App?.CancelTransfer?.(itemID).then((items) => {
      if (items) setTransfers(items);
    });
  };

  const handleTogglePauseTransfer = (itemID: string) => {
    void window.go?.app?.App?.TogglePauseTransfer?.(itemID).then((items) => {
      if (items) setTransfers(items);
    });
  };

  const handleTogglePauseAllTransfers = () => {
    void window.go?.app?.App?.TogglePauseAllTransfers?.().then((items) => {
      if (items) setTransfers(items);
    });
  };

  const handleClearLogs = () => {
    void window.go?.app?.App?.ClearLogs?.().then((items) => {
      if (items) setLogs(items);
    });
  };

  const handleDropToRemote = async (tab: Tab, paths: string[]) => {
    await handleDropToRemoteDirectory(tab, paths, tab.remotePath);
  };

  const handleDropToRemoteDirectory = async (currentTab: Tab, paths: string[], remoteBase: string) => {
    if (!liveTab(currentTab)?.connected || currentTab.mode !== 'file' || !paths.length) return;
    if (remoteBase !== currentTab.remotePath && !isPathInside(remoteBase, currentTab.remotePath)) return;

    const optimisticItems: TransferItem[] = paths.map((path, index) => ({
      id: `pending-${Date.now()}-${index}`,
      direction: 'upload',
      name: basename(path),
      progress: 0,
      speedBps: 0,
      status: 'running',
    }));

    setTransfers((current) => [...optimisticItems, ...current]);

    try {
      await window.go?.app?.App?.UploadDroppedPaths?.(currentTab.id, paths, remoteBase);
      setErrorMessage('');
      await refreshPanelsForPaths(currentTab, currentTab.localPath, currentTab.remotePath);
      await syncTransferState();
    } catch (error) {
      await syncTransferState();
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleDropToTerminal = async (currentTab: Tab, paths: string[], remotePathOverride?: string) => {
    if (!liveTab(currentTab)?.connected || currentTab.mode !== 'terminal' || currentTab.protocol !== 'ssh') return;
    const remotePath = remotePathOverride?.trim() || currentTab.remotePath;

    const optimisticItems: TransferItem[] = paths.map((path, index) => ({
      id: `pending-terminal-${Date.now()}-${index}`,
      direction: 'upload',
      name: basename(path),
      progress: 0,
      speedBps: 0,
      status: 'running',
    }));

    setTransfers((current) => [...optimisticItems, ...current]);
    callbacks.current.onTerminalUploadStateChange?.(true);

    try {
      await window.go?.app?.App?.UploadDroppedPathsToSite?.({
        id: currentTab.siteId,
        name: currentTab.title,
        folder: '',
        protocol: 'sftp',
        host: currentTab.host,
        port: currentTab.port,
        username: currentTab.username,
        password: currentTab.password,
        ppkPath: currentTab.ppkPath,
        ppkPassphrase: currentTab.ppkPassphrase,
        localPath: currentTab.localPath,
        remotePath,
        lastUsedAt: '',
      }, paths, remotePath);
      await syncTransferState();
      setErrorMessage(`SSH drag upload completed: ${paths.length} item(s) to ${remotePath}`);
    } catch (error) {
      await syncTransferState();
      setErrorMessage(`SSH drag upload failed: ${extractErrorMessage(error, t.connectionFailed)}`);
    }
  };

  const handleDownloadToLocalBase = async (currentTab: Tab, paths: string[], localBase: string) => {
    if (!liveTab(currentTab)?.connected || currentTab.mode !== 'file' || !paths.length) return;
    if (paths.some(path => !isPathInside(path, currentTab.remotePath))) return;

    const optimisticItems: TransferItem[] = paths.map((entryPath, index) => ({
      id: `pending-download-${Date.now()}-${index}`,
      direction: 'download',
      name: basename(entryPath),
      progress: 0,
      speedBps: 0,
      status: 'running',
    }));

    setTransfers((current) => [...optimisticItems, ...current]);

    try {
      await window.go?.app?.App?.DownloadDroppedPaths?.(currentTab.id, paths, localBase);
      setErrorMessage('');
      await refreshPanelsForPaths(currentTab, currentTab.localPath, currentTab.remotePath);
      await syncTransferState();
    } catch (error) {
      await syncTransferState();
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleDropToLocal = async (tab: Tab, paths: string[]) => {
    await handleDownloadToLocalBase(tab, paths, tab.localPath);
  };

  const handleDropToLocalDirectory = async (tab: Tab, paths: string[], localBase: string) => {
    await handleDownloadToLocalBase(tab, paths, localBase);
  };

  const handleDownloadEntryTo = async (tab: Tab, remotePath: string) => {
    const targetDirectory = await window.go?.app?.App?.SelectDirectory?.();
    if (!targetDirectory) {
      return;
    }
    await handleDownloadToLocalBase(tab, [remotePath], targetDirectory);
  };

  useEffect(() => {
    OnFileDrop((_x, _y, paths) => {
      const currentTab = activeTabRef.current ? { ...activeTabRef.current } : null;
      if (currentTab?.mode === 'terminal' && currentTab.protocol === 'ssh') {
        void (async () => {
          const confirmed = await callbacks.current.requestTerminalDropConfirm?.(paths, currentTab.remotePath);
          if (confirmed === false) {
            return;
          }
          await handleDropToTerminal(currentTab, paths, currentTab.remotePath);
        })();
        return;
      }
      if (currentTab?.mode === 'file' && callbacks.current.canAcceptFileDrop()) void handleDropToRemote(currentTab, paths);
    }, true);

    return () => {
      OnFileDropOff();
    };
  }, []);

  useEffect(() => EventsOn('transfer:state', (state: { transfers?: TransferItem[]; logs?: LogItem[] }) => {
    setTransfers(state.transfers ?? []);
    setLogs(state.logs ?? []);
  }), []);

  return {
    handleClearCompletedTransfers,
    handleClearAllTransfers,
    handleCancelTransfer,
    handleTogglePauseTransfer,
    handleTogglePauseAllTransfers,
    handleClearLogs,
    handleDropToRemote,
    handleDropToRemoteDirectory,
    handleDropToTerminal,
    handleDropToLocal,
    handleDropToLocalDirectory,
    handleDownloadEntryTo,
  };
}

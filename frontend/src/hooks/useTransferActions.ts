import { useEffect } from 'react';
import type React from 'react';
import { EventsOn, OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime';
import { basename, extractErrorMessage } from '../appUtils';
import type { LogItem, Tab, TransferItem } from '../types';

type Params = {
  t: { connectionFailed: string };
  activeTabRef: React.MutableRefObject<Tab | null>;
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
  setTransfers,
  setLogs,
  setErrorMessage,
  refreshPanelsForPaths,
  onTerminalUploadStateChange,
  requestTerminalDropConfirm,
}: Params) {
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

  const handleDropToRemote = async (paths: string[]) => {
    const currentTab = activeTabRef.current;
    if (!currentTab) return;
    await handleDropToRemoteDirectory(paths, currentTab.remotePath);
  };

  const handleDropToRemoteDirectory = async (paths: string[], remoteBase: string) => {
    const currentTab = activeTabRef.current;
    if (!currentTab) return;

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
      await refreshPanelsForPaths(currentTab, currentTab.localPath, currentTab.remotePath);
      await syncTransferState();
      setErrorMessage('');
    } catch (error) {
      await syncTransferState();
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleDropToTerminal = async (paths: string[], remotePathOverride?: string) => {
    const currentTab = activeTabRef.current;
    if (!currentTab || currentTab.mode !== 'terminal' || currentTab.protocol !== 'ssh') return;
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
    onTerminalUploadStateChange?.(true);

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
        tags: [],
        favorite: false,
      }, paths, remotePath);
      await syncTransferState();
      setErrorMessage(`SSH drag upload completed: ${paths.length} item(s) to ${remotePath}`);
    } catch (error) {
      await syncTransferState();
      setErrorMessage(`SSH drag upload failed: ${extractErrorMessage(error, t.connectionFailed)}`);
    }
  };

  const handleDownloadToLocalBase = async (paths: string[], localBase: string) => {
    const currentTab = activeTabRef.current;
    if (!currentTab) return;

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
      await refreshPanelsForPaths(currentTab, currentTab.localPath, currentTab.remotePath);
      await syncTransferState();
      setErrorMessage('');
    } catch (error) {
      await syncTransferState();
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleDropToLocal = async (paths: string[]) => {
    const currentTab = activeTabRef.current;
    if (!currentTab) return;
    await handleDownloadToLocalBase(paths, currentTab.localPath);
  };

  const handleDropToLocalDirectory = async (paths: string[], localBase: string) => {
    await handleDownloadToLocalBase(paths, localBase);
  };

  const handleDownloadEntryTo = async (remotePath: string) => {
    const targetDirectory = await window.go?.app?.App?.SelectDirectory?.();
    if (!targetDirectory) {
      return;
    }
    await handleDownloadToLocalBase([remotePath], targetDirectory);
  };

  useEffect(() => {
    let disposed = false;
    let retryCount = 0;
    let retryTimer: number | undefined;
    let fileDropRegistered = false;
    let unsubscribeTransferState: (() => void) | undefined;

    const registerRuntimeListeners = () => {
      if (disposed) {
        return;
      }

      if (!fileDropRegistered && typeof window.runtime?.OnFileDrop === 'function') {
        OnFileDrop((_x, _y, paths) => {
          const currentTab = activeTabRef.current;
          if (currentTab?.mode === 'terminal' && currentTab.protocol === 'ssh') {
            void (async () => {
              const confirmed = await requestTerminalDropConfirm?.(paths, currentTab.remotePath);
              if (confirmed === false) {
                return;
              }
              await handleDropToTerminal(paths, currentTab.remotePath);
            })();
            return;
          }
          void handleDropToRemote(paths);
        }, true);
        fileDropRegistered = true;
      }

      if (
        !unsubscribeTransferState &&
        typeof window.runtime?.EventsOnMultiple === 'function'
      ) {
        unsubscribeTransferState = EventsOn(
          'transfer:state',
          (state: { transfers?: TransferItem[]; logs?: LogItem[] }) => {
            setTransfers(state.transfers ?? []);
            setLogs(state.logs ?? []);
          },
        );
      }

      if ((!fileDropRegistered || !unsubscribeTransferState) && retryCount < 50) {
        retryCount += 1;
        retryTimer = window.setTimeout(registerRuntimeListeners, 100);
      }
    };

    registerRuntimeListeners();

    return () => {
      disposed = true;
      if (retryTimer !== undefined) {
        window.clearTimeout(retryTimer);
      }
      unsubscribeTransferState?.();
      if (fileDropRegistered && typeof window.runtime?.OnFileDropOff === 'function') {
        OnFileDropOff();
      }
    };
  }, []);

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

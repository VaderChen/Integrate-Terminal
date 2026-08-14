import { useEffect } from 'react';
import type React from 'react';
import { extractErrorMessage } from '../appUtils';
import type { ActionDialogState } from '../appTypes';
import type { FileContextMenuRequest } from '../components/FilePanel';
import type { FileEntry, Tab } from '../types';

type Params = {
  t: {
    connectionFailed: string;
    moveCompleted: (count: number, targetDirectory: string) => string;
    moveIntoSelfFailed: string;
  };
  activeTab: Tab | null;
  activeTabRef: React.MutableRefObject<Tab | null>;
  contextMenu: FileContextMenuRequest | null;
  actionDialog: ActionDialogState | null;
  directoryName: string;
  renameValue: string;
  setTabs: React.Dispatch<React.SetStateAction<Tab[]>>;
  setContextMenu: React.Dispatch<React.SetStateAction<FileContextMenuRequest | null>>;
  setActionDialog: React.Dispatch<React.SetStateAction<ActionDialogState | null>>;
  setDirectoryName: React.Dispatch<React.SetStateAction<string>>;
  setRenameValue: React.Dispatch<React.SetStateAction<string>>;
  setErrorMessage: React.Dispatch<React.SetStateAction<string>>;
  refreshPanels: (tab: Tab | null) => Promise<void>;
  refreshPanelsForPaths: (tab: Tab, nextLocalPath: string, nextRemotePath: string) => Promise<void>;
};

export function useFileActions({
  t,
  activeTab,
  activeTabRef,
  contextMenu,
  actionDialog,
  directoryName,
  renameValue,
  setTabs,
  setContextMenu,
  setActionDialog,
  setDirectoryName,
  setRenameValue,
  setErrorMessage,
  refreshPanels,
  refreshPanelsForPaths,
}: Params) {
  const handleOpenDirectory = async (entry: FileEntry) => {
    if (!activeTab || !entry.isDir) return;

    const nextTab: Tab = {
      ...activeTab,
      localPath: entry.side === 'local' ? entry.path : activeTab.localPath,
      remotePath: entry.side === 'remote' ? entry.path : activeTab.remotePath,
    };

    try {
      const nextTabs = await window.go?.app?.App?.UpdateTabPaths?.(activeTab.id, nextTab.localPath, nextTab.remotePath);
      if (nextTabs) {
        setTabs(nextTabs);
        const persistedTab = nextTabs.find((tab: Tab) => tab.id === activeTab.id) ?? nextTab;
        await refreshPanelsForPaths(persistedTab, persistedTab.localPath, persistedTab.remotePath);
      } else {
        await refreshPanelsForPaths(nextTab, nextTab.localPath, nextTab.remotePath);
      }
      setErrorMessage('');
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleFileContextMenu = (request: FileContextMenuRequest) => {
    if (request.side === 'remote' && (!activeTabRef.current || !activeTabRef.current.connected)) {
      setContextMenu(null);
      return;
    }
    setContextMenu(request);
  };

  useEffect(() => {
    if (contextMenu?.side === 'remote' && (!activeTab || !activeTab.connected)) {
      setContextMenu(null);
    }
  }, [activeTab, contextMenu]);

  const resetDialogState = () => {
    setActionDialog(null);
    setDirectoryName('');
    setRenameValue('');
  };

  const handleOpenEntry = async () => {
    const request = contextMenu;
    if (!request?.entry || request.side !== 'local' || request.entry.name === '..') return;

    setContextMenu(null);
    try {
      await window.go?.app?.App?.OpenLocalPath?.(request.entry.path);
      setErrorMessage('');
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleExecuteEntry = async () => {
    const request = contextMenu;
    if (!request?.entry || request.side !== 'local' || request.entry.name === '..' || request.entry.isDir) return;

    setContextMenu(null);
    try {
      await window.go?.app?.App?.ExecuteLocalPath?.(request.entry.path);
      setErrorMessage('');
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleCreateDirectory = () => {
    const request = contextMenu;
    if (!request) return;
    setContextMenu(null);
    setDirectoryName('');
    setActionDialog({ mode: 'mkdir', side: request.side });
  };

  const handleDeleteEntry = () => {
    const request = contextMenu;
    if (!request?.entry) return;
    if (actionDialog?.mode === 'delete') return;
    const entry = request.entry;
    setContextMenu(null);
    const selectedEntries = request.selectedEntries?.length ? request.selectedEntries : [entry];
    const fallbackEntries = selectedEntries.length > 0 ? selectedEntries : [entry];
    setActionDialog((current) => {
      if (current?.mode === 'delete') {
        return current;
      }
      return { mode: 'delete', side: request.side, entry, entries: fallbackEntries };
    });
  };

  const handleRenameEntry = () => {
    const request = contextMenu;
    if (!request?.entry || request.entry.name === '..') return;
    setContextMenu(null);
    setRenameValue(request.entry.name);
    setActionDialog({ mode: 'rename', side: request.side, entry: request.entry });
  };

  const handleConfirmActionDialog = async () => {
    const currentTab = activeTabRef.current;
    if (!actionDialog || !currentTab) return;

    try {
      if (actionDialog.mode === 'mkdir') {
        const name = directoryName.trim();
        if (!name) return;
        const basePath = actionDialog.side === 'local' ? currentTab.localPath : currentTab.remotePath;
        await window.go?.app?.App?.CreateDirectory?.(currentTab.id, actionDialog.side, basePath, name);
      } else if (actionDialog.mode === 'rename') {
        const name = renameValue.trim();
        if (!name) return;
        await window.go?.app?.App?.RenameEntry?.(currentTab.id, actionDialog.side, actionDialog.entry.path, name);
      } else {
        const targetPaths = (actionDialog.entries?.map((entry) => entry.path) ?? [actionDialog.entry.path]).filter(Boolean);
        if (targetPaths.length > 1) {
          await window.go?.app?.App?.DeleteEntries?.(currentTab.id, actionDialog.side, targetPaths);
        } else {
          await window.go?.app?.App?.DeleteEntry?.(currentTab.id, actionDialog.side, actionDialog.entry.path);
        }
      }

      resetDialogState();
      await refreshPanels(currentTab);
      setErrorMessage('');
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleRefreshCurrentPanel = async () => {
    await refreshPanels(activeTabRef.current);
    setErrorMessage('');
  };

  const handleMoveEntriesToDirectory = async (side: 'local' | 'remote', sourcePaths: string[], targetDirectory: string) => {
    const currentTab = activeTabRef.current;
    if (!currentTab || sourcePaths.length === 0) return;

    try {
      await window.go?.app?.App?.MoveEntriesToDirectory?.(currentTab.id, side, sourcePaths, targetDirectory);
      await refreshPanels(currentTab);
      setErrorMessage(`SUCCESS:${t.moveCompleted(sourcePaths.length, targetDirectory)}`);
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleInvalidMoveTarget = () => {
    setErrorMessage(t.moveIntoSelfFailed);
  };

  return {
    handleOpenDirectory,
    handleFileContextMenu,
    handleOpenEntry,
    handleExecuteEntry,
    handleCreateDirectory,
    handleDeleteEntry,
    handleRenameEntry,
    handleConfirmActionDialog,
    handleRefreshCurrentPanel,
    handleMoveEntriesToDirectory,
    handleInvalidMoveTarget,
    resetDialogState,
  };
}

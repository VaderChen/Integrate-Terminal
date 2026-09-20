import { useEffect, useRef } from 'react';
import type React from 'react';
import { extractErrorMessage } from '../appUtils';
import { actionableEntries, isActionableEntry, isPathInside, sameConnection } from '../filePanelState';
import type { ActionDialogState } from '../appTypes';
import type { FileContextMenuRequest } from '../components/FilePanel';
import type { FileEntry, Tab } from '../types';

type Params = {
  t: { connectionFailed: string; moveCompleted: (count: number, targetDirectory: string) => string; moveIntoSelfFailed: string };
  activeTab: Tab | null;
  activeTabRef: React.MutableRefObject<Tab | null>;
  tabsRef: React.MutableRefObject<Tab[]>;
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
  invalidatePanels: () => void;
  refreshPanels: (tab: Tab | null) => Promise<void>;
  refreshPanelsForPaths: (tab: Tab, nextLocalPath: string, nextRemotePath: string) => Promise<void>;
};

export function useFileActions({ t, activeTab, activeTabRef, tabsRef, contextMenu, actionDialog, directoryName, renameValue,
  setTabs, setContextMenu, setActionDialog, setDirectoryName, setRenameValue, setErrorMessage, invalidatePanels,
  refreshPanels, refreshPanelsForPaths }: Params) {
  const navigationRequests = useRef<Record<string, number>>({});
  const confirming = useRef(false);
  const liveTab = (tab: Tab) => tabsRef.current.find(current => sameConnection(current, tab));
  const requestTab = (request: FileContextMenuRequest | null) => {
    const tab = tabsRef.current.find(current => current.id === request?.tabId);
    if (!request || !tab || tab.mode !== 'file' || (request.side === 'remote' && !tab.connected)) return null;
    return (request.side === 'local' ? tab.localPath : tab.remotePath) === request.basePath ? tab : null;
  };

  const handleNavigatePaths = async (tab: Tab, localPath: string, remotePath: string) => {
    if (!liveTab(tab) || tab.mode !== 'file') return;
    const request = (navigationRequests.current[tab.id] ?? 0) + 1;
    navigationRequests.current[tab.id] = request;
    if (activeTabRef.current?.id === tab.id) invalidatePanels();
    try {
      const nextTabs = await window.go?.app?.App?.UpdateTabPaths?.(tab.id, localPath, remotePath);
      if (navigationRequests.current[tab.id] !== request || !liveTab(tab)) return;
      const persisted = nextTabs?.find((item: Tab) => item.id === tab.id);
      const next = { ...tab, localPath: persisted?.localPath ?? localPath, remotePath: persisted?.remotePath ?? remotePath };
      // A path update must not replace another tab's newer state with an older server snapshot.
      setTabs(current => current.map(item => item.id === tab.id ? { ...item, localPath: next.localPath, remotePath: next.remotePath } : item));
      setErrorMessage('');
      await refreshPanelsForPaths(next, next.localPath, next.remotePath);
    } catch (error) {
      if (navigationRequests.current[tab.id] !== request) return;
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
      await refreshPanels(liveTab(tab) ?? null);
    }
  };

  const handleOpenDirectory = async (tab: Tab, entry: FileEntry) => {
    if (!entry.isDir || !sameConnection(activeTabRef.current, tab)) return;
    await handleNavigatePaths(tab, entry.side === 'local' ? entry.path : tab.localPath, entry.side === 'remote' ? entry.path : tab.remotePath);
  };

  const handleFileContextMenu = (request: FileContextMenuRequest) => {
    if (activeTabRef.current?.id !== request.tabId || !requestTab(request)) {
      setContextMenu(null);
      return;
    }
    setContextMenu(request);
  };

  useEffect(() => {
    if (contextMenu && (activeTab?.id !== contextMenu.tabId || !requestTab(contextMenu))) setContextMenu(null);
  }, [activeTab, contextMenu]);

  const resetDialogState = () => {
    setActionDialog(null);
    setDirectoryName('');
    setRenameValue('');
  };

  const handleOpenEntry = async () => {
    const request = contextMenu;
    if (!request?.entry || !requestTab(request) || request.side !== 'local' || !isActionableEntry(request.entry)) return;
    setContextMenu(null);
    try {
      await window.go?.app?.App?.OpenLocalPath?.(request.entry.path);
      setErrorMessage('');
    } catch (error) { setErrorMessage(extractErrorMessage(error, t.connectionFailed)); }
  };

  const handleExecuteEntry = async () => {
    const request = contextMenu;
    if (!request?.entry || !requestTab(request) || request.side !== 'local' || !isActionableEntry(request.entry) || request.entry.isDir) return;
    setContextMenu(null);
    try {
      await window.go?.app?.App?.ExecuteLocalPath?.(request.entry.path);
      setErrorMessage('');
    } catch (error) { setErrorMessage(extractErrorMessage(error, t.connectionFailed)); }
  };

  const handleCreateDirectory = () => {
    const request = contextMenu;
    const tab = requestTab(request);
    if (!request || !tab) return;
    setContextMenu(null);
    setDirectoryName('');
    setActionDialog({ mode: 'mkdir', tab: { ...tab }, basePath: request.basePath, side: request.side });
  };

  const handleDeleteEntry = () => {
    const request = contextMenu;
    const tab = requestTab(request);
    if (!request?.entry || !tab || !isActionableEntry(request.entry) || actionDialog?.mode === 'delete') return;
    const entries = actionableEntries(request.selectedEntries?.length ? request.selectedEntries : [request.entry], request.side, request.basePath);
    if (!entries.length) return;
    setContextMenu(null);
    setActionDialog({ mode: 'delete', tab: { ...tab }, basePath: request.basePath, side: request.side, entry: entries[0], entries });
  };

  const handleRenameEntry = () => {
    const request = contextMenu;
    const tab = requestTab(request);
    if (!request?.entry || !tab || !actionableEntries([request.entry], request.side, request.basePath).length) return;
    setContextMenu(null);
    setRenameValue(request.entry.name);
    setActionDialog({ mode: 'rename', tab: { ...tab }, basePath: request.basePath, side: request.side, entry: request.entry });
  };

  const handleConfirmActionDialog = async () => {
    if (!actionDialog || confirming.current) return;
    const tab = liveTab(actionDialog.tab);
    if (!tab || (actionDialog.side === 'remote' && !tab.connected)) { resetDialogState(); return; }
    confirming.current = true;
    try {
      if (actionDialog.mode === 'mkdir') {
        const name = directoryName.trim();
        if (!name) return;
        await window.go?.app?.App?.CreateDirectory?.(tab.id, actionDialog.side, actionDialog.basePath, name);
      } else if (actionDialog.mode === 'rename') {
        const name = renameValue.trim();
        if (!name || !actionableEntries([actionDialog.entry], actionDialog.side, actionDialog.basePath).length) return;
        await window.go?.app?.App?.RenameEntry?.(tab.id, actionDialog.side, actionDialog.entry.path, name);
      } else {
        const entries = actionableEntries(actionDialog.entries, actionDialog.side, actionDialog.basePath);
        if (!entries.length) return;
        if (entries.length > 1) await window.go?.app?.App?.DeleteEntries?.(tab.id, actionDialog.side, entries.map(entry => entry.path));
        else await window.go?.app?.App?.DeleteEntry?.(tab.id, actionDialog.side, entries[0].path);
      }
      resetDialogState();
      setErrorMessage('');
      await refreshPanels(liveTab(tab) ?? null);
    } catch (error) { setErrorMessage(extractErrorMessage(error, t.connectionFailed)); }
    finally { confirming.current = false; }
  };

  const handleRefreshCurrentPanel = async () => { await refreshPanels(activeTabRef.current); };

  const handleMoveEntriesToDirectory = async (tab: Tab, side: 'local' | 'remote', sourcePaths: string[], targetDirectory: string) => {
    if (!liveTab(tab) || (side === 'remote' && !tab.connected) || !sourcePaths.length) return;
    const basePath = side === 'local' ? tab.localPath : tab.remotePath;
    if (sourcePaths.some(path => !isPathInside(path, basePath)) || !isPathInside(targetDirectory, basePath)) return;
    try {
      await window.go?.app?.App?.MoveEntriesToDirectory?.(tab.id, side, sourcePaths, targetDirectory);
      setErrorMessage(`SUCCESS:${t.moveCompleted(sourcePaths.length, targetDirectory)}`);
      await refreshPanels(liveTab(tab) ?? null);
    } catch (error) { setErrorMessage(extractErrorMessage(error, t.connectionFailed)); }
  };

  return { handleNavigatePaths, handleOpenDirectory, handleFileContextMenu, handleOpenEntry, handleExecuteEntry,
    handleCreateDirectory, handleDeleteEntry, handleRenameEntry, handleConfirmActionDialog, handleRefreshCurrentPanel,
    handleMoveEntriesToDirectory, handleInvalidMoveTarget: () => setErrorMessage(t.moveIntoSelfFailed), resetDialogState };
}

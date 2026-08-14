import type React from 'react';
import type { ConnectDialogState, HostTrustDialogState, TerminalPreferences } from '../appTypes';
import { extractErrorMessage, extractHostTrustPrompt } from '../appUtils';
import type { Site, Tab } from '../types';

type Params = {
  t: { connectionFailed: string };
  tabsRef: React.MutableRefObject<Tab[]>;
  activeTabRef: React.MutableRefObject<Tab | null>;
  setTabs: React.Dispatch<React.SetStateAction<Tab[]>>;
  setActiveTabId: React.Dispatch<React.SetStateAction<string>>;
  setErrorMessage: React.Dispatch<React.SetStateAction<string>>;
  setConnectDialog: React.Dispatch<React.SetStateAction<ConnectDialogState | null>>;
  hostTrustDialog: HostTrustDialogState | null;
  setHostTrustDialog: React.Dispatch<React.SetStateAction<HostTrustDialogState | null>>;
  setConnectingMode: React.Dispatch<React.SetStateAction<'ssh' | 'sftp' | 'telnet' | 'ftp' | null>>;
  setTerminalPreferences: React.Dispatch<React.SetStateAction<Record<string, TerminalPreferences>>>;
};

const defaultTerminalPreferences = { themeId: 'ubuntu', fontScale: 'medium', fontFamilyId: 'SF Mono' };

function quoteShellPath(path: string) {
  return `'${path.replace(/'/g, `'\\''`)}'`;
}

function visibleTabs(tabs: Tab[]) {
  return tabs.filter((tab) => !tab.hidden);
}

function latestVisibleTab(tabs: Tab[]) {
  const items = visibleTabs(tabs);
  return items[items.length - 1] ?? null;
}

export function useConnectionActions(params: Params) {
  const {
    t,
    tabsRef,
    activeTabRef,
    setTabs,
    setActiveTabId,
    setErrorMessage,
    setConnectDialog,
    hostTrustDialog,
    setHostTrustDialog,
    setConnectingMode,
    setTerminalPreferences,
  } = params;

  const ensureTerminalPreferences = (sessionId?: string) => {
    if (!sessionId) return;
    setTerminalPreferences((current) => ({
      ...current,
      [sessionId]: current[sessionId] ?? defaultTerminalPreferences,
    }));
  };

  const handleConfirmConnect = async (site: Site, selectedMode: 'ssh' | 'sftp' | 'telnet' | 'ftp') => {
    setConnectingMode(selectedMode);
    try {
      if (selectedMode === 'ssh') {
        const nextTabs = await window.go?.app?.App?.CreateSSHTab?.(site);
        if (nextTabs) {
          const newTab = latestVisibleTab(nextTabs);
          ensureTerminalPreferences(newTab?.sessionId);
          setTabs(nextTabs);
          setActiveTabId(newTab?.id ?? '');
          setConnectDialog(null);
          setErrorMessage('');
        }
        return;
      }

      if (selectedMode === 'telnet') {
        const nextTabs = await window.go?.app?.App?.CreateTelnetTab?.(site);
        if (nextTabs) {
          const newTab = latestVisibleTab(nextTabs);
          ensureTerminalPreferences(newTab?.sessionId);
          setTabs(nextTabs);
          setActiveTabId(newTab?.id ?? '');
          setConnectDialog(null);
          setErrorMessage('');
        }
        return;
      }

      const nextTabs = await window.go?.app?.App?.CreateTab?.(site);
      if (nextTabs) {
        const newTab = latestVisibleTab(nextTabs);
        setTabs(nextTabs);
        setActiveTabId(newTab?.id ?? '');
        setErrorMessage('');
        setConnectDialog(null);
      }
    } catch (error) {
      const trustPrompt = extractHostTrustPrompt(error);
      if (trustPrompt && (selectedMode === 'ssh' || selectedMode === 'sftp')) {
        setHostTrustDialog({ site, selectedMode, prompt: trustPrompt });
        setErrorMessage('');
        return;
      }
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    } finally {
      setConnectingMode(null);
    }
  };

  const handleApproveHost = async () => {
    if (!hostTrustDialog) return;

    try {
      await window.go?.app?.App?.ApproveHost?.(hostTrustDialog.prompt);
      const pending = hostTrustDialog;
      setHostTrustDialog(null);
      await handleConfirmConnect(pending.site, pending.selectedMode);
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleCloseTab = async (tabId: string) => {
    const closingTab = tabsRef.current.find((tab) => tab.id === tabId);
    const nextTabs = await window.go?.app?.App?.CloseTab?.(tabId);
    if (nextTabs) {
      if (closingTab?.sessionId) {
        setTerminalPreferences((current) => {
          const next = { ...current };
          delete next[closingTab.sessionId];
          return next;
        });
      }
      setTabs(nextTabs);
      setActiveTabId((current) => (current === tabId ? visibleTabs(nextTabs)[0]?.id ?? '' : current));
    }
  };

  const handleTerminalSessionClosed = async (sessionId: string) => {
    const currentTabs = tabsRef.current;
    const closedTab = currentTabs.find((tab) => tab.mode === 'terminal' && tab.sessionId === sessionId);
    if (!closedTab) return;

    const optimisticTabs = currentTabs.filter((tab) => tab.id !== closedTab.id);
    setTabs(optimisticTabs);
    setActiveTabId((current) => (current === closedTab.id ? visibleTabs(optimisticTabs)[0]?.id ?? '' : current));

    const nextTabs = await window.go?.app?.App?.CloseTab?.(closedTab.id);
    if (nextTabs) {
      setTabs(nextTabs);
      setActiveTabId((current) => (current === closedTab.id ? visibleTabs(nextTabs)[0]?.id ?? '' : current));
    }
  };

  const handleOpenSFTPFromTerminal = async () => {
    const currentTab = activeTabRef.current;
    if (!currentTab || currentTab.mode !== 'terminal') return;

    try {
      const nextTabs = await window.go?.app?.App?.CreateTab?.({
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
        remotePath: currentTab.remotePath,
        lastUsedAt: '',
      });

      if (nextTabs) {
        setTabs(nextTabs);
        setActiveTabId(latestVisibleTab(nextTabs)?.id ?? '');
        setErrorMessage('');
      }
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleOpenLocalTerminal = async () => {
    const nextTabs = await window.go?.app?.App?.CreateLocalTerminalTab?.('');
    if (nextTabs) {
      setTabs(nextTabs);
      const newTab = latestVisibleTab(nextTabs);
      setActiveTabId(newTab?.id ?? '');
      ensureTerminalPreferences(newTab?.sessionId);
    }
  };

  const handleOpenSSHFromFileTab = async (tab: Tab) => {
    if (tab.mode === 'terminal') return;

    const nextTabs = await window.go?.app?.App?.CreateSSHTab?.({
      id: tab.siteId,
      name: tab.title,
      folder: '',
      protocol: 'sftp',
      host: tab.host,
      port: tab.port,
      username: tab.username,
      password: tab.password,
      ppkPath: tab.ppkPath,
      ppkPassphrase: tab.ppkPassphrase,
      localPath: tab.localPath,
      remotePath: tab.remotePath,
      lastUsedAt: '',
    });

    if (!nextTabs) return;

    const newTab = latestVisibleTab(nextTabs);
    ensureTerminalPreferences(newTab?.sessionId);
    setTabs(nextTabs);
    setActiveTabId(newTab?.id ?? '');

    if (newTab?.sessionId && tab.remotePath) {
      await window.go?.app?.App?.WriteSSHInput?.(newTab.sessionId, `cd ${quoteShellPath(tab.remotePath)}\n`);
    }
  };

  return {
    handleConfirmConnect,
    handleApproveHost,
    handleCloseTab,
    handleTerminalSessionClosed,
    handleOpenSFTPFromTerminal,
    handleOpenLocalTerminal,
    handleOpenSSHFromFileTab,
  };
}

import { useEffect, useMemo, useRef, useState } from "react";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import {
  faAngleLeft,
  faAngleRight,
  faAngleUp,
  faGear,
  faLock,
  faPlus,
  faUnlock,
  faXmark,
} from "@fortawesome/free-solid-svg-icons";
import type {
  ActionDialogState,
  ConnectDialogState,
  HostTrustDialogState,
  PathContextMenuState,
  SiteFolderDialogState,
  TerminalPreferences,
  TerminalUploadConfirmState,
} from "./appTypes";
import {
  appendLocalNetworkHint,
  buildBlankSite,
  canSaveSite,
  extractErrorMessage,
  fallbackBootstrap,
  sortEntries,
  withParentEntry,
} from "./appUtils";
import {
  ConnectMethodModal,
  FileActionContextMenu,
  FileActionModal,
  HostTrustModal,
  PathContextMenu,
  SiteFolderActionModal,
  TerminalUploadConfirmModal,
} from "./components/AppOverlays";
import { ConnectForm } from "./components/ConnectForm";
import { FilePanel } from "./components/FilePanel";
import type { FileContextMenuRequest } from "./components/FilePanel";
import { useConnectionActions } from "./hooks/useConnectionActions";
import { useFileActions } from "./hooks/useFileActions";
import { useSettingsActions } from "./hooks/useSettingsActions";
import { useSiteLibraryActions } from "./hooks/useSiteLibraryActions";
import { useTransferActions } from "./hooks/useTransferActions";
import { useTerminalEvents } from "./hooks/useTerminalEvents";
import { useUpdateActions } from "./hooks/useUpdateActions";
import { SSHConsolePanel } from "./components/SSHConsolePanel";
import { SettingsModal } from "./components/SettingsModal";
import { SiteList } from "./components/SiteList";
import { SyncDialog } from "./components/SyncDialog";
import { TabBar } from "./components/TabBar";
import { TransferPanel } from "./components/TransferPanel";
import { UpdateDialog } from "./components/UpdateDialog";
import { getMessages, resolveLocale } from "./i18n";
import { EventsOn, Quit } from "../wailsjs/runtime/runtime";
import type {
  Config,
  FileComparison,
  FileEntry,
  FileSortState,
  LogItem,
  Site,
  Tab,
  TransferItem,
} from "./types";

const plainTextInputProps = {
  autoCapitalize: "none" as const,
  autoCorrect: "off" as const,
  autoComplete: "off",
  spellCheck: false,
};

export default function App() {
  const [sites, setSites] = useState<Site[]>([]);
  const [tabs, setTabs] = useState<Tab[]>([]);
  const [activeTabId, setActiveTabId] = useState("");
  const [localFiles, setLocalFiles] = useState<FileEntry[]>([]);
  const [remoteFiles, setRemoteFiles] = useState<FileEntry[]>([]);
  const [transfers, setTransfers] = useState<TransferItem[]>([]);
  const [logs, setLogs] = useState<LogItem[]>([]);
  const [defaultLocalPath, setDefaultLocalPath] = useState(
    fallbackBootstrap.defaultLocalPath,
  );
  const [draftSite, setDraftSite] = useState<Site>(
    buildBlankSite(fallbackBootstrap.defaultLocalPath),
  );
  const [draftSiteBaseline, setDraftSiteBaseline] = useState<Site>(
    buildBlankSite(fallbackBootstrap.defaultLocalPath),
  );
  const [siteEditorOpen, setSiteEditorOpen] = useState(false);
  const [config, setConfig] = useState<Config>(fallbackBootstrap.config);
  const [localSort, setLocalSort] = useState<FileSortState>({
    key: "name",
    direction: "asc",
  });
  const [remoteSort, setRemoteSort] = useState<FileSortState>({
    key: "name",
    direction: "asc",
  });
  const [formExpanded, setFormExpanded] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  // 相容舊沙盒版本保留的金鑰路徑；讀取失敗時提供重新選取目錄的入口。
  const [pendingKeyPaths, setPendingKeyPaths] = useState<string[]>([]);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [contextMenu, setContextMenu] = useState<FileContextMenuRequest | null>(
    null,
  );
  const [pathContextMenu, setPathContextMenu] =
    useState<PathContextMenuState | null>(null);
  const [actionDialog, setActionDialog] = useState<ActionDialogState | null>(
    null,
  );
  const [connectDialog, setConnectDialog] = useState<ConnectDialogState | null>(
    null,
  );
  const [hostTrustDialog, setHostTrustDialog] =
    useState<HostTrustDialogState | null>(null);
  const [terminalUploadConfirmDialog, setTerminalUploadConfirmDialog] =
    useState<TerminalUploadConfirmState | null>(null);
  const [siteFolderDialog, setSiteFolderDialog] =
    useState<SiteFolderDialogState | null>(null);
  const [connectingMode, setConnectingMode] = useState<
    "ssh" | "sftp" | "telnet" | "ftp" | null
  >(null);
  const [directoryName, setDirectoryName] = useState("");
  const [renameValue, setRenameValue] = useState("");
  const [siteFolderName, setSiteFolderName] = useState("");
  const [syncDialogOpen, setSyncDialogOpen] = useState(false);
  const [quitDialogOpen, setQuitDialogOpen] = useState(false);
  const [syncComparisons, setSyncComparisons] = useState<FileComparison[]>([]);
  const [syncBusy, setSyncBusy] = useState<"upload" | "download" | "">("");
  const [syncError, setSyncError] = useState("");

  useEffect(() => {
    const dispose = EventsOn("app:quit-requested", () => setQuitDialogOpen(true));
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.metaKey && event.key.toLowerCase() === "q") {
        event.preventDefault();
        setQuitDialogOpen(true);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      dispose();
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, []);
  const [collapsedPanelsByTabId, setCollapsedPanelsByTabId] = useState<
    Record<string, boolean>
  >({});
  const [transferPanelExpanded, setTransferPanelExpanded] = useState(false);
  const [loading, setLoading] = useState(true);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [terminalPreferences, setTerminalPreferences] = useState<
    Record<string, TerminalPreferences>
  >({});
  const activeTabRef = useRef<Tab | null>(null);
  const tabsRef = useRef<Tab[]>([]);
  const closeTerminalTabOnDisconnectRef = useRef(true);
  const restoreTransferPanelExpandedRef = useRef<boolean | null>(null);
  const restoreTransferPanelTimerRef = useRef<number | null>(null);
  const terminalUploadConfirmResolverRef = useRef<
    ((confirmed: boolean) => void) | null
  >(null);
  const panelRequestIdRef = useRef(0);
  const locale = useMemo(
    () => resolveLocale(config.language),
    [config.language],
  );
  const t = useMemo(() => getMessages(locale), [locale]);
  const brandEyebrowLabel = t.brandEyebrow;
  const draftCanSave = useMemo(() => canSaveSite(draftSite), [draftSite]);
  const draftIsDirty = useMemo(
    () =>
      serializeSiteDraft(draftSite) !== serializeSiteDraft(draftSiteBaseline),
    [draftSite, draftSiteBaseline],
  );
  const selectVisibleTabs = (items: Tab[]) =>
    items.filter((tab) => !tab.hidden);
  const getPreferredVisibleTabId = (items: Tab[], preferredTabId = "") =>
    selectVisibleTabs(items).find((tab) => tab.id === preferredTabId)?.id ??
    selectVisibleTabs(items)[0]?.id ??
    "";

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      try {
        const payload =
          (await window.go?.app?.App?.Bootstrap?.()) ?? fallbackBootstrap;
        if (cancelled) {
          return;
        }

        const nextConfig = {
          ...fallbackBootstrap.config,
          ...payload.config,
          closeTerminalTabOnDisconnect:
            payload.config?.closeTerminalTabOnDisconnect ?? true,
          showHiddenFiles: payload.config?.showHiddenFiles ?? false,
          showTrayIcon: payload.config?.showTrayIcon ?? false,
          rememberWindowPosition:
            payload.config?.rememberWindowPosition ?? false,
          telnetLocalEcho: payload.config?.telnetLocalEcho ?? true,
          restServerEnabled: payload.config?.restServerEnabled ?? false,
          restServerPort: payload.config?.restServerPort ?? 18080,
          restServerAllowlist: payload.config?.restServerAllowlist?.length
            ? payload.config.restServerAllowlist
            : ["127.0.0.1"],
          transferRetryCount: payload.config?.transferRetryCount ?? 2,
          transferConflictStrategy:
            payload.config?.transferConflictStrategy ?? "overwrite",
          language: payload.config?.language ?? "",
          theme: payload.config?.theme ?? "neutral",
          siteFolders: payload.config?.siteFolders ?? [],
        };
        const nextDefaultLocalPath =
          payload.defaultLocalPath || fallbackBootstrap.defaultLocalPath;
        const nextSites = Array.isArray(payload.sites)
          ? payload.sites.map((site) => ({
              ...site,
              tags: site.tags ?? [],
              favorite: site.favorite ?? false,
            }))
          : [];
        const nextTabs = Array.isArray(payload.tabs) ? payload.tabs : [];
        const nextLocalFiles = Array.isArray(payload.localFiles)
          ? payload.localFiles
          : [];
        const nextRemoteFiles = Array.isArray(payload.remoteFiles)
          ? payload.remoteFiles
          : [];
        const nextTransfers = Array.isArray(payload.transfers)
          ? payload.transfers
          : [];
        const nextLogs = Array.isArray(payload.logs) ? payload.logs : [];
        setSites(nextSites);
        setTabs(nextTabs);
        setConfig(nextConfig);
        setDefaultLocalPath(nextDefaultLocalPath);
        setLocalFiles(nextLocalFiles);
        setRemoteFiles(nextRemoteFiles);
        setTransfers(nextTransfers);
        setLogs(nextLogs);
        setActiveTabId(
          getPreferredVisibleTabId(nextTabs, nextConfig.lastActiveTab),
        );
        const blankSite = buildBlankSite(nextDefaultLocalPath);
        setDraftSite(blankSite);
        setDraftSiteBaseline(blankSite);
        setLoading(false);

      } catch (error) {
        if (!cancelled) {
          setErrorMessage(extractErrorMessage(error, t.connectionFailed));
          setLoading(false);
        }
      }
    };

    void load();

    return () => {
      cancelled = true;
    };
  }, []);

  const visibleTabs = useMemo(() => tabs.filter((tab) => !tab.hidden), [tabs]);
  const terminalTabs = useMemo(
    () => visibleTabs.filter((tab) => tab.mode === "terminal" && tab.sessionId),
    [visibleTabs],
  );
  const activeTab = useMemo(
    () =>
      visibleTabs.find((tab) => tab.id === activeTabId) ??
      visibleTabs[0] ??
      null,
    [activeTabId, visibleTabs],
  );
  useEffect(() => {
    if (!errorMessage) {
      setPendingKeyPaths([]);
      return;
    }
    let cancelled = false;
    void (async () => {
      const pending =
        (await window.go?.app?.App?.PendingKeyAuthorizations?.()) ?? [];
      if (!cancelled) {
        setPendingKeyPaths(pending);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [errorMessage]);

  const authorizePendingKeyDirectory = async () => {
    const authorized = await window.go?.app?.App?.AuthorizeKeyDirectory?.(
      pendingKeyPaths[0] ?? "",
    );
    if (authorized) {
      setPendingKeyPaths([]);
      setErrorMessage("");
    }
  };

  const isSuccessBanner =
    errorMessage.startsWith("SSH drag upload completed:") ||
    errorMessage.startsWith("SUCCESS:");
  const rawBannerMessage = errorMessage.startsWith("SUCCESS:")
    ? errorMessage.slice("SUCCESS:".length)
    : errorMessage;
  const bannerMessage = isSuccessBanner
    ? rawBannerMessage
    : appendLocalNetworkHint(rawBannerMessage, t.localNetworkAccessHint);
  const localPanelCollapsed = activeTab
    ? (collapsedPanelsByTabId[activeTab.id] ?? true)
    : true;
  const localPanelHiddenForActiveTab = activeTab?.mode === "terminal";
  const isLocalPanelCollapsed =
    localPanelCollapsed || localPanelHiddenForActiveTab;
  useEffect(() => {
    activeTabRef.current = activeTab;
  }, [activeTab]);

  useEffect(() => {
    tabsRef.current = tabs;
  }, [tabs]);

  useEffect(() => {
    setCollapsedPanelsByTabId((current) => {
      const next = Object.fromEntries(
        tabs.map((tab) => [tab.id, current[tab.id] ?? true]),
      );
      const currentKeys = Object.keys(current);
      const nextKeys = Object.keys(next);
      if (
        currentKeys.length === nextKeys.length &&
        currentKeys.every((key) => current[key] === next[key])
      ) {
        return current;
      }
      return next;
    });
  }, [tabs]);

  useEffect(() => {
    closeTerminalTabOnDisconnectRef.current =
      config.closeTerminalTabOnDisconnect;
  }, [config.closeTerminalTabOnDisconnect]);

  useEffect(() => {
    if (restoreTransferPanelExpandedRef.current === null) {
      return;
    }
    const hasActiveTransfers = transfers.some(
      (item) => item.status === "running" || item.status === "paused",
    );
    if (hasActiveTransfers) {
      if (restoreTransferPanelTimerRef.current !== null) {
        window.clearTimeout(restoreTransferPanelTimerRef.current);
        restoreTransferPanelTimerRef.current = null;
      }
      return;
    }
    if (restoreTransferPanelTimerRef.current !== null) {
      return;
    }
    restoreTransferPanelTimerRef.current = window.setTimeout(() => {
      setTransferPanelExpanded(
        restoreTransferPanelExpandedRef.current ?? false,
      );
      restoreTransferPanelExpandedRef.current = null;
      restoreTransferPanelTimerRef.current = null;
    }, 1200);
  }, [transfers]);

  useEffect(
    () => () => {
      if (restoreTransferPanelTimerRef.current !== null) {
        window.clearTimeout(restoreTransferPanelTimerRef.current);
      }
    },
    [],
  );

  useEffect(() => {
    const closeMenu = () => {
      setContextMenu(null);
      setPathContextMenu(null);
    };
    window.addEventListener("click", closeMenu);
    window.addEventListener("blur", closeMenu);
    return () => {
      window.removeEventListener("click", closeMenu);
      window.removeEventListener("blur", closeMenu);
    };
  }, []);

  useEffect(() => {
    const preventNativeContextMenu = (event: MouseEvent) => {
      event.preventDefault();
    };

    window.addEventListener("contextmenu", preventNativeContextMenu);
    return () => {
      window.removeEventListener("contextmenu", preventNativeContextMenu);
    };
  }, []);

  const visibleLocalFiles = useMemo(
    () =>
      sortEntries(
        withParentEntry(localFiles, activeTab?.localPath ?? "", "local"),
        localSort,
      ),
    [activeTab?.localPath, localFiles, localSort],
  );

  const visibleRemoteFiles = useMemo(
    () =>
      sortEntries(
        withParentEntry(remoteFiles, activeTab?.remotePath ?? "", "remote"),
        remoteSort,
      ),
    [activeTab?.remotePath, remoteFiles, remoteSort],
  );

  const refreshPanels = async (tab: Tab | null) => {
    const requestId = ++panelRequestIdRef.current;
    setContextMenu(null);
    setPathContextMenu(null);
    if (!tab) {
      setLocalFiles([]);
      setRemoteFiles([]);
      return;
    }
    if (tab.mode === "terminal") {
      setLocalFiles([]);
      setRemoteFiles([]);
      return;
    }

    const [local, remote, queue, nextLogs] = await Promise.all([
      window.go?.app?.App?.ListLocal?.(tab.id, tab.localPath),
      window.go?.app?.App?.ListRemote?.(tab.id, tab.remotePath),
      window.go?.app?.App?.GetTransfers?.(),
      window.go?.app?.App?.GetLogs?.(),
    ]);

    if (
      requestId !== panelRequestIdRef.current ||
      activeTabRef.current?.id !== tab.id
    ) {
      return;
    }

    setLocalFiles(local ?? []);
    setRemoteFiles(remote ?? []);
    setTransfers(queue ?? []);
    setLogs(nextLogs ?? []);
  };

  const refreshPanelsForPaths = async (
    tab: Tab,
    nextLocalPath: string,
    nextRemotePath: string,
  ) => {
    const requestId = ++panelRequestIdRef.current;
    setContextMenu(null);
    setPathContextMenu(null);
    const [local, remote, queue, nextLogs] = await Promise.all([
      window.go?.app?.App?.ListLocal?.(tab.id, nextLocalPath),
      window.go?.app?.App?.ListRemote?.(tab.id, nextRemotePath),
      window.go?.app?.App?.GetTransfers?.(),
      window.go?.app?.App?.GetLogs?.(),
    ]);

    if (
      requestId !== panelRequestIdRef.current ||
      activeTabRef.current?.id !== tab.id
    ) {
      return;
    }

    setLocalFiles(local ?? []);
    setRemoteFiles(remote ?? []);
    setTransfers(queue ?? []);
    setLogs(nextLogs ?? []);
  };

  useEffect(() => {
    void refreshPanels(activeTab);
  }, [activeTabId]);

  const {
    handleOpenNewSiteDialog,
    handleOpenEditSiteDialog,
    handleCloseSiteEditor,
    handleSaveSite,
    handleDeleteSite,
    handleCopySite,
    handleToggleFavorite,
    handleSortSitesByName,
    handleOpenCreateSiteFolder,
    handlePromptRenameSiteFolder,
    handleSortSiteFolders,
    handlePromptDeleteSiteFolder,
    handleConfirmSiteFolderDialog,
    handleReorderSites,
    handleMoveSiteToFolder,
    handleReorderSiteFolders,
  } = useSiteLibraryActions({
    t: {
      connectionFailed: t.connectionFailed,
      siteCopySuffix: t.siteCopySuffix,
    },
    defaultLocalPath,
    sites,
    draftSite,
    draftSiteBaseline,
    siteFolderName,
    siteFolderDialog,
    setSites,
    setConfig,
    setDraftSite,
    setDraftSiteBaseline,
    setSiteEditorOpen,
    setFormExpanded,
    setErrorMessage,
    setSiteFolderName,
    setSiteFolderDialog,
  });

  const handleOpenSiteDataDirectory = async () => {
    await window.go?.app?.App?.OpenSiteDataDirectory?.();
  };

  const handleBackupSiteLibrary = async () => {
    return (await window.go?.app?.App?.BackupSiteLibrary?.()) ?? "";
  };

  const handleRestoreSiteLibraryBackup = async () => {
    const result = await window.go?.app?.App?.RestoreSiteLibraryBackup?.();
    if (!result) {
      return false;
    }
    setSites(result.sites ?? []);
    setConfig((current) => ({ ...current, ...result.config }));
    return true;
  };

  const handleReorderTabs = async (tabIDs: string[]) => {
    const nextTabs = await window.go?.app?.App?.ReorderTabs?.(tabIDs);
    if (nextTabs) {
      setTabs(nextTabs);
    }
  };

  const handlePickLocalPath = async () => {
    const currentTab = activeTabRef.current;
    if (!currentTab || currentTab.mode === "terminal") return;

    const selectedPath = await window.go?.app?.App?.SelectDirectory?.();
    if (!selectedPath) {
      return;
    }

    try {
      const nextTabs = await window.go?.app?.App?.UpdateTabPaths?.(
        currentTab.id,
        selectedPath,
        currentTab.remotePath,
      );
      if (nextTabs) {
        setTabs(nextTabs);
        const persistedTab = nextTabs.find(
          (tab: Tab) => tab.id === currentTab.id,
        );
        if (persistedTab) {
          await refreshPanelsForPaths(
            persistedTab,
            persistedTab.localPath,
            persistedTab.remotePath,
          );
        }
      }
      setErrorMessage("");
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleSubmitRemotePath = async (nextRemotePath: string) => {
    const currentTab = activeTabRef.current;
    const trimmedPath = nextRemotePath.trim();
    if (!currentTab || currentTab.mode === "terminal" || !trimmedPath) return;

    try {
      const nextTabs = await window.go?.app?.App?.UpdateTabPaths?.(
        currentTab.id,
        currentTab.localPath,
        trimmedPath,
      );
      if (nextTabs) {
        const persistedTab = nextTabs.find(
          (tab: Tab) => tab.id === currentTab.id,
        );
        if (persistedTab) {
          await refreshPanelsForPaths(
            persistedTab,
            persistedTab.localPath,
            persistedTab.remotePath,
          );
        }
        setTabs(nextTabs);
      }
      setErrorMessage("");
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
      await refreshPanels(currentTab);
    }
  };

  const handleOpenSite = async (site: Site) => {
    setConnectDialog({ site });
  };

  const {
    handleConfirmConnect,
    handleApproveHost,
    handleCloseTab,
    handleTerminalSessionClosed,
    handleOpenSFTPFromTerminal,
    handleOpenLocalTerminal,
    handleOpenSSHFromFileTab,
  } = useConnectionActions({
    t: { connectionFailed: t.connectionFailed },
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
  });

  useTerminalEvents({
    tabs,
    setTabs,
    closeTerminalTabOnDisconnect: config.closeTerminalTabOnDisconnect,
    onSessionClosed: handleTerminalSessionClosed,
  });

  const handlePathContextMenu = (request: PathContextMenuState) => {
    setContextMenu(null);
    setPathContextMenu(request);
  };

  const handleOpenPathFromContextMenu = async () => {
    if (!pathContextMenu) return;

    try {
      if (pathContextMenu.side === "local") {
        await window.go?.app?.App?.OpenLocalPath?.(pathContextMenu.path);
      } else {
        const currentTab = activeTabRef.current;
        if (!currentTab || currentTab.mode === "terminal") return;
        await handleOpenSSHFromFileTab({
          ...currentTab,
          remotePath: pathContextMenu.path,
        });
      }

      setPathContextMenu(null);
      setErrorMessage("");
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleCopyPathFromContextMenu = async () => {
    if (!pathContextMenu) return;

    try {
      await navigator.clipboard.writeText(pathContextMenu.path);
      setPathContextMenu(null);
      setErrorMessage("");
    } catch (error) {
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const {
    handleFontScaleChange,
    handleLanguageChange,
    handleThemeChange,
    handleShowHiddenFilesChange,
    handleShowTrayIconChange,
    handleRememberWindowPositionChange,
    handleTelnetLocalEchoChange,
    handleRESTServerEnabledChange,
    handleRESTServerPortChange,
    handleRESTServerAllowlistChange,
    handleTransferRetryCountChange,
    handleTransferConflictStrategyChange,
    handleRestoreTabsChange,
    handleCloseTerminalTabOnDisconnectChange,
  } = useSettingsActions({
    config,
    setConfig,
    activeTabRef,
    refreshPanels,
  });

  const handleOpenSyncDialog = async () => {
    const currentTab = activeTabRef.current;
    if (!currentTab || currentTab.mode === "terminal") return;
    try {
      const comparisons = await window.go?.app?.App?.CompareDirectories?.(
        currentTab.id,
        currentTab.localPath,
        currentTab.remotePath,
      );
      setSyncComparisons(comparisons ?? []);
      setSyncError("");
      setSyncDialogOpen(true);
    } catch (error) {
      setSyncError(extractErrorMessage(error, t.connectionFailed));
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    }
  };

  const handleSyncDirectories = async (direction: "upload" | "download") => {
    const currentTab = activeTabRef.current;
    if (!currentTab || currentTab.mode === "terminal") return;
    try {
      setSyncBusy(direction);
      setSyncError("");
      await window.go?.app?.App?.SyncDirectories?.(
        currentTab.id,
        currentTab.localPath,
        currentTab.remotePath,
        direction,
      );
      await refreshPanelsForPaths(currentTab, currentTab.localPath, currentTab.remotePath);
      const comparisons = await window.go?.app?.App?.CompareDirectories?.(
        currentTab.id,
        currentTab.localPath,
        currentTab.remotePath,
      );
      setSyncComparisons(comparisons ?? []);
    } catch (error) {
      setSyncError(extractErrorMessage(error, t.connectionFailed));
      setErrorMessage(extractErrorMessage(error, t.connectionFailed));
    } finally {
      setSyncBusy("");
    }
  };

  const updateActions = useUpdateActions({ enabled: !loading, locale });

  const toggleSort = (side: "local" | "remote", key: FileSortState["key"]) => {
    const setter = side === "local" ? setLocalSort : setRemoteSort;
    setter((current) => ({
      key,
      direction:
        current.key === key && current.direction === "asc" ? "desc" : "asc",
    }));
  };

  const {
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
  } = useTransferActions({
    t: { connectionFailed: t.connectionFailed },
    activeTabRef,
    setTransfers,
    setLogs,
    setErrorMessage,
    refreshPanelsForPaths,
    onTerminalUploadStateChange: (active) => {
      if (!active) {
        return;
      }
      if (restoreTransferPanelExpandedRef.current === null) {
        restoreTransferPanelExpandedRef.current = transferPanelExpanded;
      }
      setTransferPanelExpanded(true);
    },
    requestTerminalDropConfirm: (paths, remotePath) =>
      new Promise<boolean>((resolve) => {
        terminalUploadConfirmResolverRef.current = resolve;
        setTerminalUploadConfirmDialog({ paths, remotePath });
      }),
  });

  const handleCloseTerminalUploadConfirm = () => {
    terminalUploadConfirmResolverRef.current?.(false);
    terminalUploadConfirmResolverRef.current = null;
    setTerminalUploadConfirmDialog(null);
  };

  const handleConfirmTerminalUpload = () => {
    terminalUploadConfirmResolverRef.current?.(true);
    terminalUploadConfirmResolverRef.current = null;
    setTerminalUploadConfirmDialog(null);
  };

  const {
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
  } = useFileActions({
    t: {
      connectionFailed: t.connectionFailed,
      moveCompleted: t.moveCompleted,
      moveIntoSelfFailed: t.moveIntoSelfFailed,
    },
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
  });

  if (loading) {
    return <div className="loading-shell">{t.loading}</div>;
  }

  return (
    <div
      className={`app-shell font-scale-${config.fontScale} app-theme-${config.theme} ${sidebarCollapsed ? "sidebar-collapsed" : ""}`}
    >
      <aside className={`sidebar ${sidebarCollapsed ? "collapsed" : ""}`}>
        <div className="brand">
          <div className="brand-top">
            {!sidebarCollapsed ? (
              <p className="eyebrow">{brandEyebrowLabel}</p>
            ) : null}
            <div className="brand-header-actions">
              {!sidebarCollapsed ? (
                <button
                  className="site-view-button brand-settings-button"
                  onClick={() => setSettingsOpen(true)}
                  aria-label={t.settingsButton}
                  title={t.settingsButton}
                >
                  <FontAwesomeIcon icon={faGear} />
                </button>
              ) : null}
              <button
                className="site-view-button brand-toggle-button"
                onClick={() => setSidebarCollapsed((value) => !value)}
                aria-label={sidebarCollapsed ? t.expand : t.collapse}
                title={sidebarCollapsed ? t.expand : t.collapse}
              >
                <FontAwesomeIcon
                  icon={sidebarCollapsed ? faAngleRight : faAngleLeft}
                />
              </button>
            </div>
          </div>
          {!sidebarCollapsed && t.brandTitle ? <h1>{t.brandTitle}</h1> : null}
          {!sidebarCollapsed ? <span>{t.brandSubtitle}</span> : null}
        </div>
        {sidebarCollapsed ? (
          <section className="card sidebar-mini-actions">
            <div
              className="sidebar-mini-site-list"
              role="list"
              aria-label={t.siteListTitle}
            >
              {sites.map((site) => (
                <button
                  key={site.id}
                  className="site-view-button sidebar-mini-site-button"
                  onClick={() => handleOpenSite(site)}
                  aria-label={site.name || site.host}
                  title={`${site.name || site.host} | ${site.protocol.toUpperCase()} | ${site.username}@${site.host}:${site.port}`}
                >
                  <span
                    className={`protocol-badge ${site.protocol}`}
                    aria-hidden="true"
                  >
                    <FontAwesomeIcon
                      icon={site.protocol === "sftp" ? faLock : faUnlock}
                    />
                  </span>
                </button>
              ))}
            </div>
          </section>
        ) : (
          <>
            <SiteList
              locale={locale}
              sites={sites}
              siteFolders={config.siteFolders}
              onOpenSite={handleOpenSite}
              onCopySite={handleCopySite}
              onDeleteSite={handleDeleteSite}
              onSortByName={handleSortSitesByName}
              onCreateFolder={handleOpenCreateSiteFolder}
              onSortFolders={() => void handleSortSiteFolders()}
              onRenameFolder={handlePromptRenameSiteFolder}
              onDeleteFolder={handlePromptDeleteSiteFolder}
              onReorderSites={(siteIDs) => void handleReorderSites(siteIDs)}
              onReorderFolders={(folderNames) =>
                void handleReorderSiteFolders(folderNames)
              }
              onMoveSiteToFolder={(siteId, folder) =>
                void handleMoveSiteToFolder(siteId, folder)
              }
              onEditSite={handleOpenEditSiteDialog}
              onToggleFavorite={handleToggleFavorite}
            />
            <button
              type="button"
              className="card site-editor-open-button"
              onClick={handleOpenNewSiteDialog}
              aria-label={t.quickAdd}
              title={t.quickAdd}
            >
              <FontAwesomeIcon icon={faPlus} />
              <span>{t.quickAdd}</span>
            </button>
          </>
        )}
      </aside>

      {siteEditorOpen ? (
        <div
          className="modal-overlay"
          onMouseDown={(event) => {
            if (event.button === 0 && event.target === event.currentTarget) {
              handleCloseSiteEditor();
            }
          }}
          role="presentation"
        >
          <div
            className="settings-modal site-editor-modal"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
            aria-modal="true"
            aria-labelledby="site-editor-title"
          >
            <ConnectForm
              locale={locale}
              draft={draftSite}
              onChange={setDraftSite}
              onSave={handleSaveSite}
              canSave={draftCanSave}
              isDirty={draftIsDirty}
              expanded={formExpanded}
              onToggle={() => setFormExpanded((value) => !value)}
              onClose={handleCloseSiteEditor}
              dialogTitleId="site-editor-title"
              variant="dialog"
            />
          </div>
        </div>
      ) : null}

      <SettingsModal
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        config={config}
        locale={locale}
        onLanguageChange={handleLanguageChange}
        onThemeChange={handleThemeChange}
        onRestoreTabsChange={handleRestoreTabsChange}
        onCloseTerminalTabOnDisconnectChange={
          handleCloseTerminalTabOnDisconnectChange
        }
        onShowHiddenFilesChange={handleShowHiddenFilesChange}
        onShowTrayIconChange={handleShowTrayIconChange}
        onRememberWindowPositionChange={handleRememberWindowPositionChange}
        onTelnetLocalEchoChange={handleTelnetLocalEchoChange}
        onRESTServerEnabledChange={handleRESTServerEnabledChange}
        onRESTServerPortChange={handleRESTServerPortChange}
        onRESTServerAllowlistChange={handleRESTServerAllowlistChange}
        onTransferRetryCountChange={handleTransferRetryCountChange}
        onTransferConflictStrategyChange={handleTransferConflictStrategyChange}
        onFontScaleChange={handleFontScaleChange}
        onOpenSiteDataDirectory={handleOpenSiteDataDirectory}
        onBackupSiteLibrary={handleBackupSiteLibrary}
        onRestoreSiteLibraryBackup={handleRestoreSiteLibraryBackup}
        updateChecking={updateActions.checking}
        updateFeedback={updateActions.feedback}
        updateFeedbackError={updateActions.feedbackError}
        onCheckForUpdates={() => void updateActions.checkForUpdates("manual")}
      />

      <UpdateDialog
        locale={locale}
        result={updateActions.checkResult}
        actionBusy={updateActions.actionBusy}
        actionResult={updateActions.actionResult}
        actionError={updateActions.actionError}
        onClose={updateActions.closeDialog}
        onStartUpdate={() => void updateActions.startUpdate()}
      />

      <SyncDialog
        open={syncDialogOpen}
        comparisons={syncComparisons}
        busy={syncBusy}
        error={syncError}
        locale={locale}
        onClose={() => {
          if (syncBusy === "") {
            setSyncDialogOpen(false);
            setSyncError("");
          }
        }}
        onSync={handleSyncDirectories}
      />

      <main className="workspace">
        <section className="workspace-tabs">
          <TabBar
            locale={locale}
            tabs={visibleTabs}
            activeTabId={activeTab?.id ?? ""}
            onSelectTab={setActiveTabId}
            onCloseTab={handleCloseTab}
            onReorderTabs={(tabIDs) => void handleReorderTabs(tabIDs)}
            onOpenLocalTerminal={handleOpenLocalTerminal}
          />
        </section>

        <section className="workspace-body">
          {errorMessage ? (
            <div
              className={`error-banner ${isSuccessBanner ? "success-banner" : ""}`}
            >
              <span>{bannerMessage}</span>
              {!isSuccessBanner && pendingKeyPaths.length > 0 ? (
                <button
                  className="site-view-button banner-action-button"
                  onClick={authorizePendingKeyDirectory}
                  title={t.authorizeKeyDirectoryHint}
                >
                  {t.authorizeKeyDirectory}
                </button>
              ) : null}
              <button
                className={`error-banner-close ${isSuccessBanner ? "success-banner-close" : ""}`}
                onClick={() => setErrorMessage("")}
                aria-label={t.close}
                title={t.close}
              >
                <FontAwesomeIcon icon={faXmark} />
              </button>
            </div>
          ) : null}
          <section
            className={`panels-shell ${isLocalPanelCollapsed ? "local-panel-collapsed" : ""}`}
          >
            <button
              className="panel-collapse-handle"
              onClick={() => {
                if (!activeTab) return;
                setCollapsedPanelsByTabId((current) => ({
                  ...current,
                  [activeTab.id]: !(current[activeTab.id] ?? true),
                }));
              }}
              disabled={localPanelHiddenForActiveTab}
              aria-label={
                isLocalPanelCollapsed
                  ? `${t.expand}${t.localFiles}`
                  : `${t.collapse}${t.localFiles}`
              }
              title={
                isLocalPanelCollapsed
                  ? `${t.expand}${t.localFiles}`
                  : `${t.collapse}${t.localFiles}`
              }
            >
              <FontAwesomeIcon
                icon={isLocalPanelCollapsed ? faAngleRight : faAngleLeft}
              />
            </button>
            <section
              className={`panels ${isLocalPanelCollapsed ? "local-panel-collapsed" : ""}`}
            >
              {isLocalPanelCollapsed ? null : (
                <FilePanel
                  locale={locale}
                  title={t.localFiles}
                  path={activeTab?.localPath ?? defaultLocalPath}
                  entries={visibleLocalFiles}
                  side="local"
                  sortState={localSort}
                  onSort={(key) => toggleSort("local", key)}
                  onRefresh={() => void handleRefreshCurrentPanel()}
                  onDropFiles={handleDropToLocal}
                  onDropFilesToDirectory={(paths, targetDirectory) => {
                    void handleDropToLocalDirectory(paths, targetDirectory);
                  }}
                  onMoveEntriesToDirectory={(paths, targetDirectory) => {
                    void handleMoveEntriesToDirectory(
                      "local",
                      paths,
                      targetDirectory,
                    );
                  }}
                  onInvalidMoveToDirectory={handleInvalidMoveTarget}
                  onOpenDirectory={handleOpenDirectory}
                  onPickPath={() => void handlePickLocalPath()}
                  onContextMenuRequest={handleFileContextMenu}
                  onPathContextMenuRequest={handlePathContextMenu}
                />
              )}
              <div className="remote-panel-stack">
                {terminalTabs.map((terminalTab) => {
                  const isActive = activeTab?.id === terminalTab.id;
                  const preferences =
                    terminalPreferences[terminalTab.sessionId];
                  return (
                    <div
                      key={terminalTab.id}
                      className={`terminal-panel-slot ${isActive ? "active" : ""}`}
                      aria-hidden={!isActive}
                    >
                      <SSHConsolePanel
                        locale={locale}
                        sessionId={terminalTab.sessionId}
                        active={isActive}
                        canOpenSFTP={terminalTab.protocol === "ssh"}
                        onDropLocalPaths={(paths, remotePath) => {
                          void (async () => {
                            const confirmed = await new Promise<boolean>(
                              (resolve) => {
                                terminalUploadConfirmResolverRef.current =
                                  resolve;
                                setTerminalUploadConfirmDialog({
                                  paths,
                                  remotePath:
                                    remotePath || terminalTab.remotePath,
                                });
                              },
                            );
                            if (!confirmed) {
                              return;
                            }
                            await handleDropToTerminal(
                              paths,
                              remotePath || terminalTab.remotePath,
                            );
                          })();
                        }}
                        enableLocalEcho={
                          terminalTab.protocol === "telnet" &&
                          config.telnetLocalEcho
                        }
                        themeId={preferences?.themeId ?? "ubuntu"}
                        fontScale={preferences?.fontScale ?? "medium"}
                        fontFamilyId={preferences?.fontFamilyId ?? "SF Mono"}
                        onThemeChange={(themeId) => {
                          setTerminalPreferences((current) => ({
                            ...current,
                            [terminalTab.sessionId]: {
                              themeId,
                              fontScale:
                                current[terminalTab.sessionId]?.fontScale ??
                                "medium",
                              fontFamilyId:
                                current[terminalTab.sessionId]?.fontFamilyId ??
                                "SF Mono",
                            },
                          }));
                        }}
                        onFontScaleChange={(fontScale) => {
                          setTerminalPreferences((current) => ({
                            ...current,
                            [terminalTab.sessionId]: {
                              themeId:
                                current[terminalTab.sessionId]?.themeId ??
                                "ubuntu",
                              fontScale,
                              fontFamilyId:
                                current[terminalTab.sessionId]?.fontFamilyId ??
                                "SF Mono",
                            },
                          }));
                        }}
                        onFontFamilyChange={(fontFamilyId) => {
                          setTerminalPreferences((current) => ({
                            ...current,
                            [terminalTab.sessionId]: {
                              themeId:
                                current[terminalTab.sessionId]?.themeId ??
                                "ubuntu",
                              fontScale:
                                current[terminalTab.sessionId]?.fontScale ??
                                "medium",
                              fontFamilyId,
                            },
                          }));
                        }}
                        onOpenSFTP={() => {
                          void handleOpenSFTPFromTerminal();
                        }}
                        onClose={() => {
                          void handleCloseTab(terminalTab.id);
                        }}
                      />
                    </div>
                  );
                })}
                {activeTab?.mode !== "terminal" ? (
                  <FilePanel
                    locale={locale}
                    title={t.remoteFiles}
                    path={activeTab?.remotePath ?? "/"}
                    entries={visibleRemoteFiles}
                    side="remote"
                    sortState={remoteSort}
                    onSort={(key) => toggleSort("remote", key)}
                    onRefresh={() => void handleRefreshCurrentPanel()}
                    onCompare={() => void handleOpenSyncDialog()}
                    onDropFiles={handleDropToRemote}
                    onDropFilesToDirectory={(paths, targetDirectory) => {
                      void handleDropToRemoteDirectory(paths, targetDirectory);
                    }}
                    onMoveEntriesToDirectory={(paths, targetDirectory) => {
                      void handleMoveEntriesToDirectory(
                        "remote",
                        paths,
                        targetDirectory,
                      );
                    }}
                    onInvalidMoveToDirectory={handleInvalidMoveTarget}
                    onOpenDirectory={handleOpenDirectory}
                    onSubmitPath={(path) => void handleSubmitRemotePath(path)}
                    onContextMenuRequest={handleFileContextMenu}
                    onPathContextMenuRequest={handlePathContextMenu}
                  />
                ) : null}
              </div>
            </section>
          </section>

          <TransferPanel
            locale={locale}
            transfers={transfers}
            logs={logs}
            expanded={transferPanelExpanded}
            terminalMode={activeTab?.mode === "terminal"}
            onExpandedChange={setTransferPanelExpanded}
            onClearCompleted={handleClearCompletedTransfers}
            onClearAll={handleClearAllTransfers}
            onTogglePauseAll={handleTogglePauseAllTransfers}
            onClearLogs={handleClearLogs}
            onCancelTransfer={handleCancelTransfer}
            onTogglePauseTransfer={handleTogglePauseTransfer}
          />
        </section>
      </main>

      {contextMenu ? (
        <FileActionContextMenu
          request={contextMenu}
          locale={locale}
          onClose={() => setContextMenu(null)}
          onOpenEntry={() => void handleOpenEntry()}
          onExecuteEntry={() => void handleExecuteEntry()}
          onCreateDirectory={handleCreateDirectory}
          onRenameEntry={handleRenameEntry}
          onDeleteEntry={handleDeleteEntry}
          onDownloadEntryTo={() => {
            if (!contextMenu?.entry) return;
            setContextMenu(null);
            void handleDownloadEntryTo(contextMenu.entry.path);
          }}
          onRefresh={() => void handleRefreshCurrentPanel()}
        />
      ) : null}
      {pathContextMenu ? (
        <PathContextMenu
          request={pathContextMenu}
          locale={locale}
          onClose={() => setPathContextMenu(null)}
          onOpenPath={() => void handleOpenPathFromContextMenu()}
          onCopyPath={() => void handleCopyPathFromContextMenu()}
        />
      ) : null}
      {connectDialog ? (
        <ConnectMethodModal
          dialog={connectDialog}
          locale={locale}
          connectingMode={connectingMode}
          onClose={() => setConnectDialog(null)}
          onConfirm={(mode) =>
            void handleConfirmConnect(connectDialog.site, mode)
          }
        />
      ) : null}
      {hostTrustDialog ? (
        <HostTrustModal
          dialog={hostTrustDialog}
          locale={locale}
          onApprove={() => void handleApproveHost()}
          onClose={() => setHostTrustDialog(null)}
        />
      ) : null}
      {actionDialog ? (
        <FileActionModal
          dialog={actionDialog}
          locale={locale}
          directoryName={directoryName}
          renameValue={renameValue}
          plainTextInputProps={plainTextInputProps}
          onDirectoryNameChange={setDirectoryName}
          onRenameValueChange={setRenameValue}
          onConfirm={() => void handleConfirmActionDialog()}
          onClose={resetDialogState}
        />
      ) : null}
      {terminalUploadConfirmDialog ? (
        <TerminalUploadConfirmModal
          dialog={terminalUploadConfirmDialog}
          locale={locale}
          onConfirm={handleConfirmTerminalUpload}
          onClose={handleCloseTerminalUploadConfirm}
        />
      ) : null}
      {siteFolderDialog ? (
        <SiteFolderActionModal
          dialog={siteFolderDialog}
          locale={locale}
          folderName={siteFolderName}
          plainTextInputProps={plainTextInputProps}
          onFolderNameChange={setSiteFolderName}
          onConfirm={() => void handleConfirmSiteFolderDialog()}
          onClose={() => {
            setSiteFolderDialog(null);
            setSiteFolderName("");
          }}
        />
      ) : null}
      {quitDialogOpen ? (
        <div className="modal-overlay" onClick={() => setQuitDialogOpen(false)}>
          <section className="settings-modal action-modal quit-confirm-dialog" role="dialog" aria-modal="true" onClick={(event) => event.stopPropagation()}>
            <div className="settings-header modal-header">
              <div>
                <p className="eyebrow">{t.settingsLabel}</p>
                <h2>{t.quitConfirmTitle}</h2>
              </div>
              <button className="ghost icon-button action-cancel-button" onClick={() => setQuitDialogOpen(false)} aria-label={t.close}>
                <FontAwesomeIcon icon={faXmark} />
              </button>
            </div>
            <div className="modal-body quit-confirm-body">
              <p>{t.quitConfirmMessage}</p>
              <div className="quit-confirm-actions">
                <button className="ghost quit-confirm-button quit-confirm-cancel-button" onClick={() => setQuitDialogOpen(false)}>{t.quitConfirmCancel}</button>
              <button className="primary quit-confirm-button quit-confirm-close-button" onClick={() => { void (async () => { await window.go?.app?.App?.StopBackgroundService?.(); await window.go?.app?.App?.ApproveQuit?.(); Quit(); })(); }}>{t.quitConfirmClose}</button>
                <button className="ghost quit-confirm-button quit-confirm-background-button" onClick={() => { void (async () => { await handleShowTrayIconChange(true); setQuitDialogOpen(false); await window.go?.app?.App?.ApproveQuit?.(); Quit(); })(); }}>{t.quitConfirmHide}</button>
              </div>
            </div>
          </section>
        </div>
      ) : null}
    </div>
  );
}

function serializeSiteDraft(site: Site) {
  return JSON.stringify({
    id: site.id,
    name: site.name,
    folder: site.folder,
    protocol: site.protocol,
    host: site.host,
    port: site.port,
    username: site.username,
    password: site.password,
    ppkPath: site.ppkPath,
    ppkPassphrase: site.ppkPassphrase,
    localPath: site.localPath,
    remotePath: site.remotePath,
    tags: site.tags ?? [],
    favorite: site.favorite ?? false,
  });
}

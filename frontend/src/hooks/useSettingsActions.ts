import type React from "react";
import type { Config, Tab } from "../types";

type Params = {
  config: Config;
  setConfig: React.Dispatch<React.SetStateAction<Config>>;
  activeTabRef: React.MutableRefObject<Tab | null>;
  refreshPanels: (tab: Tab | null) => Promise<void>;
};

export function useSettingsActions({
  config,
  setConfig,
  activeTabRef,
  refreshPanels,
}: Params) {
  const saveConfig = async (update: Config | ((current: Config) => Config)) => {
    const nextConfig = typeof update === "function" ? update(config) : update;
    const previousConfig = config;
    setConfig(nextConfig);
    try {
      const saved = await window.go?.app?.App?.SaveConfig?.(nextConfig);
      setConfig(saved ?? nextConfig);
    } catch (error) {
      setConfig(previousConfig);
      throw error;
    }
  };

  const handleFontScaleChange = async (fontScale: Config["fontScale"]) => {
    await saveConfig((current) => ({ ...current, fontScale }));
  };

  const handleLanguageChange = async (language: Config["language"]) => {
    await saveConfig((current) => ({ ...current, language }));
  };

  const handleThemeChange = async (theme: Config["theme"]) => {
    await saveConfig((current) => ({ ...current, theme }));
  };

  const handleShowHiddenFilesChange = async (showHiddenFiles: boolean) => {
    await saveConfig((current) => ({ ...current, showHiddenFiles }));
    await refreshPanels(activeTabRef.current);
  };

  const handleShowTrayIconChange = async (showTrayIcon: boolean) => {
    await saveConfig((current) => ({ ...current, showTrayIcon }));
  };

  const handleRememberWindowPositionChange = async (
    rememberWindowPosition: boolean,
  ) => {
    await saveConfig((current) => ({ ...current, rememberWindowPosition }));
  };

  const handleTelnetLocalEchoChange = async (telnetLocalEcho: boolean) => {
    await saveConfig((current) => ({ ...current, telnetLocalEcho }));
  };

  const handleRESTServerEnabledChange = async (
    restServerEnabled: boolean,
    restServerPort = config.restServerPort,
    restServerAllowlist = config.restServerAllowlist,
  ) => {
    await saveConfig((current) => ({
      ...current,
      restServerEnabled,
      restServerPort,
      restServerAllowlist,
    }));
  };

  const handleRESTServerPortChange = async (restServerPort: number) => {
    await saveConfig((current) => ({ ...current, restServerPort }));
  };

  const handleRESTServerAllowlistChange = async (
    restServerAllowlist: string[],
  ) => {
    await saveConfig((current) => ({ ...current, restServerAllowlist }));
  };

  const handleTransferRetryCountChange = async (transferRetryCount: number) => {
    await saveConfig((current) => ({ ...current, transferRetryCount }));
  };

  const handleTransferConflictStrategyChange = async (
    transferConflictStrategy: Config['transferConflictStrategy'],
  ) => {
    await saveConfig((current) => ({ ...current, transferConflictStrategy }));
  };

  const handleRestoreTabsChange = async (restoreTabsOnStart: boolean) => {
    await saveConfig((current) => ({ ...current, restoreTabsOnStart }));
  };

  const handleCloseTerminalTabOnDisconnectChange = async (
    closeTerminalTabOnDisconnect: boolean,
  ) => {
    await saveConfig((current) => ({
      ...current,
      closeTerminalTabOnDisconnect,
    }));
  };

  return {
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
  };
}

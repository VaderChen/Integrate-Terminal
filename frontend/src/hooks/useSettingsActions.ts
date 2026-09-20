import type React from 'react';
import type { Config, Tab } from '../types';

type Params = {
  config: Config;
  setConfig: React.Dispatch<React.SetStateAction<Config>>;
  activeTabRef: React.MutableRefObject<Tab | null>;
  refreshPanels: (tab: Tab | null) => Promise<void>;
};

export function useSettingsActions({ config, setConfig, activeTabRef, refreshPanels }: Params) {
  const saveConfig = async (nextConfig: Config) => {
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

  const handleFontScaleChange = async (fontScale: Config['fontScale']) => {
    await saveConfig({ ...config, fontScale });
  };

  const handleLanguageChange = async (language: Config['language']) => {
    await saveConfig({ ...config, language });
  };

  const handleThemeChange = async (theme: Config['theme']) => {
    await saveConfig({ ...config, theme });
  };

  const handleShowHiddenFilesChange = async (showHiddenFiles: boolean) => {
    await saveConfig({ ...config, showHiddenFiles });
    await refreshPanels(activeTabRef.current);
  };

  const handleShowTrayIconChange = async (showTrayIcon: boolean) => {
    await saveConfig({ ...config, showTrayIcon });
  };

  const handleRememberWindowPositionChange = async (rememberWindowPosition: boolean) => {
    await saveConfig({ ...config, rememberWindowPosition });
  };

  const handleTelnetLocalEchoChange = async (telnetLocalEcho: boolean) => {
    await saveConfig({ ...config, telnetLocalEcho });
  };

  const handleRESTServerEnabledChange = async (restServerEnabled: boolean, restServerPort = config.restServerPort) => {
    await saveConfig({ ...config, restServerEnabled, restServerPort });
  };

  const handleRESTServerPortChange = async (restServerPort: number) => {
    await saveConfig({ ...config, restServerPort });
  };

  const handleRestoreTabsChange = async (restoreTabsOnStart: boolean) => {
    await saveConfig({ ...config, restoreTabsOnStart });
  };

  const handleCloseTerminalTabOnDisconnectChange = async (closeTerminalTabOnDisconnect: boolean) => {
    await saveConfig({ ...config, closeTerminalTabOnDisconnect });
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
    handleRestoreTabsChange,
    handleCloseTerminalTabOnDisconnectChange,
  };
}

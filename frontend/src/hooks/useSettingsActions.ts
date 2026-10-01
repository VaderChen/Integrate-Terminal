import { useRef } from 'react';
import type React from 'react';
import type { Config, Tab } from '../types';

type Params = {
  config: Config;
  setConfig: React.Dispatch<React.SetStateAction<Config>>;
  activeTabRef: React.MutableRefObject<Tab | null>;
  refreshPanels: (tab: Tab | null) => Promise<void>;
};

export function useSettingsActions({ config, setConfig, activeTabRef, refreshPanels }: Params) {
  const current = useRef(config);
  const savedConfig = useRef(config);
  const queue = useRef<Promise<void>>(Promise.resolve());
  const pending = useRef<Array<Partial<Config>>>([]);
  if (pending.current.length === 0) {
    current.current = config;
    savedConfig.current = config;
  }
  const saveConfig = (patch: Partial<Config>) => {
    pending.current.push(patch);
    current.current = { ...current.current, ...patch };
    setConfig(current.current);
    const operation = queue.current.then(async () => {
      const next = { ...savedConfig.current, ...patch };
      try {
        savedConfig.current = await window.go?.app?.App?.SaveConfig?.(next) ?? next;
      } finally {
        pending.current.shift();
        current.current = pending.current.reduce<Config>((value, change) => ({ ...value, ...change }), savedConfig.current);
        setConfig(current.current);
      }
    });
    queue.current = operation.catch(() => {});
    return operation;
  };

  const handleFontScaleChange = async (fontScale: Config['fontScale']) => {
    await saveConfig({ fontScale });
  };

  const handleLanguageChange = async (language: Config['language']) => {
    await saveConfig({ language });
  };

  const handleThemeChange = async (theme: Config['theme']) => {
    await saveConfig({ theme });
  };

  const handleShowHiddenFilesChange = async (showHiddenFiles: boolean) => {
    await saveConfig({ showHiddenFiles });
    await refreshPanels(activeTabRef.current);
  };

  const handleShowTrayIconChange = async (showTrayIcon: boolean) => {
    await saveConfig({ showTrayIcon });
  };

  const handleRememberWindowPositionChange = async (rememberWindowPosition: boolean) => {
    await saveConfig({ rememberWindowPosition });
  };

  const handleTelnetLocalEchoChange = async (telnetLocalEcho: boolean) => {
    await saveConfig({ telnetLocalEcho });
  };

  const handleRESTServerEnabledChange = async (restServerEnabled: boolean, restServerPort = current.current.restServerPort) => {
    await saveConfig({ restServerEnabled, restServerPort });
  };

  const handleRESTServerPortChange = async (restServerPort: number) => {
    await saveConfig({ restServerPort });
  };

  const handleRestoreTabsChange = async (restoreTabsOnStart: boolean) => {
    await saveConfig({ restoreTabsOnStart });
  };

  const handleCloseTerminalTabOnDisconnectChange = async (closeTerminalTabOnDisconnect: boolean) => {
    await saveConfig({ closeTerminalTabOnDisconnect });
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

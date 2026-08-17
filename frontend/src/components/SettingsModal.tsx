import { useEffect, useState } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faCopy, faDownload, faFolderOpen, faRotateLeft, faXmark } from '@fortawesome/free-solid-svg-icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';
import type { Config, RestServerStatus } from '../types';
import { type Locale, useI18n } from '../i18n';

type SettingsSection = 'general' | 'display' | 'system' | 'skill' | 'about';

const BUILD_VERSION = import.meta.env.VITE_APP_VERSION ?? '1.00.00';

type Props = {
  open: boolean;
  onClose: () => void;
  config: Config;
  locale: Locale;
  onLanguageChange: (language: Config['language']) => void;
  onThemeChange: (theme: Config['theme']) => void;
  onRestoreTabsChange: (restoreTabsOnStart: boolean) => void;
  onCloseTerminalTabOnDisconnectChange: (closeTerminalTabOnDisconnect: boolean) => void;
  onShowHiddenFilesChange: (showHiddenFiles: boolean) => void;
  onShowTrayIconChange: (showTrayIcon: boolean) => void;
  onRememberWindowPositionChange: (rememberWindowPosition: boolean) => void;
  onTelnetLocalEchoChange: (telnetLocalEcho: boolean) => void;
  onRESTServerEnabledChange: (restServerEnabled: boolean, restServerPort?: number, restServerAllowlist?: string[]) => void;
  onRESTServerPortChange: (restServerPort: number) => void;
  onRESTServerAllowlistChange: (restServerAllowlist: string[]) => void;
  onFontScaleChange: (scale: Config['fontScale']) => void;
  onOpenSiteDataDirectory: () => Promise<void>;
  onBackupSiteLibrary: () => Promise<string>;
  onRestoreSiteLibraryBackup: () => Promise<boolean>;
  updateChecking: boolean;
  updateFeedback: string;
  updateFeedbackError: boolean;
  onCheckForUpdates: () => void;
};

export function SettingsModal({
  open,
  onClose,
  config,
  locale,
  onLanguageChange,
  onThemeChange,
  onRestoreTabsChange,
  onCloseTerminalTabOnDisconnectChange,
  onShowHiddenFilesChange,
  onShowTrayIconChange,
  onRememberWindowPositionChange,
  onTelnetLocalEchoChange,
  onRESTServerEnabledChange,
  onRESTServerPortChange,
  onRESTServerAllowlistChange,
  onFontScaleChange,
  onOpenSiteDataDirectory,
  onBackupSiteLibrary,
  onRestoreSiteLibraryBackup,
  updateChecking,
  updateFeedback,
  updateFeedbackError,
  onCheckForUpdates,
}: Props) {
  const t = useI18n(locale);
  const [activeSection, setActiveSection] = useState<SettingsSection>('general');
  const [skillMarkdown, setSkillMarkdown] = useState('');
  const [skillLoading, setSkillLoading] = useState(false);
  const [skillError, setSkillError] = useState('');
  const [skillCopyMessage, setSkillCopyMessage] = useState('');
  const [restAllowlistDraft, setRestAllowlistDraft] = useState(config.restServerAllowlist.join(', '));
  const [trayReminder, setTrayReminder] = useState('');
  const [restStatus, setRestStatus] = useState<RestServerStatus | null>(null);
  const [restPortDraft, setRestPortDraft] = useState(String(config.restServerPort));
  const [siteDataDirectory, setSiteDataDirectory] = useState('');
  const [siteStorageBusy, setSiteStorageBusy] = useState<'backup' | 'restore' | ''>('');
  const [siteStorageFeedback, setSiteStorageFeedback] = useState('');
  const [siteStorageFeedbackError, setSiteStorageFeedbackError] = useState(false);

  useEffect(() => {
    setRestPortDraft(String(config.restServerPort));
  }, [config.restServerPort]);

  useEffect(() => {
    setRestAllowlistDraft(config.restServerAllowlist.join(', '));
  }, [config.restServerAllowlist]);

  useEffect(() => {
    if (!open || activeSection !== 'general') {
      return;
    }

    let cancelled = false;
    void (async () => {
      try {
        const directory = await window.go?.app?.App?.GetSiteDataDirectory?.();
        if (!cancelled) {
          setSiteDataDirectory(directory ?? '');
        }
      } catch (error) {
        if (!cancelled) {
          setSiteStorageFeedback(error instanceof Error && error.message ? error.message : t.settingsSiteStorageOperationFailed);
          setSiteStorageFeedbackError(true);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [activeSection, open, t.settingsSiteStorageOperationFailed]);

  useEffect(() => {
    if (!open || activeSection !== 'skill') {
      return;
    }

    let cancelled = false;
    void (async () => {
      try {
        setSkillLoading(true);
        setSkillError('');
        const [markdown, status] = await Promise.all([
          window.go?.app?.App?.GetRestAPIDocsMarkdown?.(),
          window.go?.app?.App?.GetRESTServerStatus?.(),
        ]);
        if (cancelled) {
          return;
        }
        setSkillMarkdown(markdown ?? '');
        setRestStatus(status ?? null);
      } catch (error) {
        if (cancelled) {
          return;
        }
        setSkillError(error instanceof Error ? error.message : t.connectionFailed);
      } finally {
        if (!cancelled) {
          setSkillLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [activeSection, open, config.restServerEnabled, t.connectionFailed]);

  useEffect(() => {
    if (!skillCopyMessage) {
      return;
    }
    const timer = window.setTimeout(() => setSkillCopyMessage(''), 2200);
    return () => window.clearTimeout(timer);
  }, [skillCopyMessage]);

  useEffect(() => {
    if (!trayReminder) {
      return;
    }
    const timer = window.setTimeout(() => setTrayReminder(''), 2600);
    return () => window.clearTimeout(timer);
  }, [trayReminder]);

  if (!open) return null;

  const handleExportSkillMarkdown = async () => {
    if (!skillMarkdown) {
      return;
    }

    try {
      await window.go?.app?.App?.ExportRestAPIDocsMarkdown?.();
    } catch (error) {
      setSkillError(error instanceof Error ? error.message : t.connectionFailed);
    }
  };

  const handleCopySkillMarkdown = async () => {
    if (!skillMarkdown) {
      return;
    }

    try {
      await navigator.clipboard.writeText(skillMarkdown);
      setSkillCopyMessage(t.settingsSkillCopySuccess);
      setSkillError('');
    } catch {
      setSkillCopyMessage(t.settingsSkillCopyFailed);
    }
  };

  const commitRESTServerPort = async () => {
    const parsed = Number.parseInt(restPortDraft.trim(), 10);
    const nextPort = Number.isFinite(parsed) && parsed > 0 && parsed <= 65535 ? parsed : config.restServerPort;
    setRestPortDraft(String(nextPort));
    if (nextPort !== config.restServerPort) {
      await onRESTServerPortChange(nextPort);
    }
    return nextPort;
  };

  const commitRESTServerAllowlist = async () => {
    const nextAllowlist = parseRESTServerAllowlist(restAllowlistDraft);
    setRestAllowlistDraft(nextAllowlist.join(', '));
    if (nextAllowlist.join('\n') !== config.restServerAllowlist.join('\n')) {
      await onRESTServerAllowlistChange(nextAllowlist);
    }
  };

  const handleToggleRESTServer = async () => {
    if (!config.restServerEnabled) {
      const parsedPort = Number.parseInt(restPortDraft.trim(), 10);
      const nextPort = Number.isFinite(parsedPort) && parsedPort > 0 && parsedPort <= 65535 ? parsedPort : config.restServerPort;
      const nextAllowlist = parseRESTServerAllowlist(restAllowlistDraft);
      setRestPortDraft(String(nextPort));
      setRestAllowlistDraft(nextAllowlist.join(', '));
      await onRESTServerEnabledChange(true, nextPort, nextAllowlist);
      return;
    }
    await onRESTServerEnabledChange(!config.restServerEnabled);
  };

  const handleToggleTrayIcon = async () => {
    if (config.restServerEnabled && config.showTrayIcon) {
      setTrayReminder(t.settingsShowTrayIconRequired);
      return;
    }
    await onShowTrayIconChange(!config.showTrayIcon);
  };

  const handleOpenSiteStorageDirectory = async () => {
    try {
      await onOpenSiteDataDirectory();
      setSiteStorageFeedback('');
      setSiteStorageFeedbackError(false);
    } catch (error) {
      setSiteStorageFeedback(error instanceof Error && error.message ? error.message : t.settingsSiteStorageOperationFailed);
      setSiteStorageFeedbackError(true);
    }
  };

  const handleBackupSites = async () => {
    if (siteStorageBusy) {
      return;
    }
    try {
      setSiteStorageBusy('backup');
      setSiteStorageFeedback('');
      setSiteStorageFeedbackError(false);
      const targetPath = await onBackupSiteLibrary();
      if (targetPath) {
        setSiteStorageFeedback(t.settingsSiteBackupSuccess);
      }
    } catch (error) {
      setSiteStorageFeedback(error instanceof Error && error.message ? error.message : t.settingsSiteStorageOperationFailed);
      setSiteStorageFeedbackError(true);
    } finally {
      setSiteStorageBusy('');
    }
  };

  const handleRestoreSites = async () => {
    if (siteStorageBusy || !window.confirm(t.settingsSiteRestoreConfirm)) {
      return;
    }
    try {
      setSiteStorageBusy('restore');
      setSiteStorageFeedback('');
      setSiteStorageFeedbackError(false);
      const restored = await onRestoreSiteLibraryBackup();
      if (restored) {
        setSiteStorageFeedback(t.settingsSiteRestoreSuccess);
      }
    } catch (error) {
      setSiteStorageFeedback(error instanceof Error && error.message ? error.message : t.settingsSiteStorageOperationFailed);
      setSiteStorageFeedbackError(true);
    } finally {
      setSiteStorageBusy('');
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <section className="settings-modal settings-dialog" onClick={(event) => event.stopPropagation()}>
        <div className="settings-header modal-header">
          <div>
            <p className="eyebrow delete-dialog-eyebrow">{t.settingsLabel}</p>
            <h2>{t.settingsTitle}</h2>
          </div>
          <button className="ghost icon-button action-cancel-button" onClick={onClose} aria-label={t.close}>
            <FontAwesomeIcon icon={faXmark} />
          </button>
        </div>

        <div className="settings-body modal-body settings-layout">
	          <aside className="settings-sidebar">
	            <button className={`settings-nav-item ${activeSection === 'general' ? 'active' : ''}`} onClick={() => setActiveSection('general')}>
	              {t.settingsNavGeneral}
	            </button>
	            <button className={`settings-nav-item ${activeSection === 'display' ? 'active' : ''}`} onClick={() => setActiveSection('display')}>
	              {t.settingsNavDisplay}
	            </button>
	            <button className={`settings-nav-item ${activeSection === 'system' ? 'active' : ''}`} onClick={() => setActiveSection('system')}>
	              {t.settingsNavSystem}
            </button>
	            <button className={`settings-nav-item ${activeSection === 'skill' ? 'active' : ''}`} onClick={() => setActiveSection('skill')}>
	              {t.settingsNavSkill}
	            </button>
	            <button className={`settings-nav-item ${activeSection === 'about' ? 'active' : ''}`} onClick={() => setActiveSection('about')}>
	              {t.settingsNavAbout}
	            </button>
	          </aside>

          <section className="settings-content">
            {activeSection === 'general' ? (
              <>
                <div className="settings-section-card">
                  <div className="settings-section-copy">
                    <strong>{t.settingsRestoreTabs}</strong>
                    <span>{t.settingsRestoreTabsHint}</span>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={config.restoreTabsOnStart}
                    aria-label={t.settingsRestoreTabs}
                    className={`ios-switch ${config.restoreTabsOnStart ? 'active' : ''}`}
                    onClick={() => onRestoreTabsChange(!config.restoreTabsOnStart)}
                    title={config.restoreTabsOnStart ? t.settingsOn : t.settingsOff}
                  >
                    <span className="ios-switch-track" />
                    <span className="ios-switch-thumb" />
                  </button>
                </div>
                <div className="settings-section-card">
                  <div className="settings-section-copy">
                    <strong>{t.settingsHideHiddenFiles}</strong>
                    <span>{t.settingsHideHiddenFilesHint}</span>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={config.showHiddenFiles}
                    aria-label={t.settingsHideHiddenFiles}
                    className={`ios-switch ${config.showHiddenFiles ? 'active' : ''}`}
                    onClick={() => onShowHiddenFilesChange(!config.showHiddenFiles)}
                    title={config.showHiddenFiles ? t.settingsOn : t.settingsOff}
                  >
                    <span className="ios-switch-track" />
                    <span className="ios-switch-thumb" />
                  </button>
                </div>
                <div className="settings-section-card">
                  <div className="settings-section-copy">
                    <strong>{t.settingsCloseTerminalTab}</strong>
                    <span>{t.settingsCloseTerminalTabHint}</span>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={config.closeTerminalTabOnDisconnect}
                    aria-label={t.settingsCloseTerminalTab}
                    className={`ios-switch ${config.closeTerminalTabOnDisconnect ? 'active' : ''}`}
                    onClick={() => onCloseTerminalTabOnDisconnectChange(!config.closeTerminalTabOnDisconnect)}
                    title={config.closeTerminalTabOnDisconnect ? t.settingsOn : t.settingsOff}
                  >
                    <span className="ios-switch-track" />
                    <span className="ios-switch-thumb" />
                  </button>
                </div>
                <div className="settings-section-card settings-section-stack settings-site-storage-card">
                  <div className="settings-section-copy">
                    <div className="settings-site-storage-title-row">
                      <strong>{t.settingsSiteStorageTitle}</strong>
                    </div>
                    <span>{t.settingsSiteStorageHint}</span>
                  </div>
                  <div className="settings-site-storage-path-row">
                    <div className="settings-site-storage-path-copy">
                      <span>{t.settingsSiteStoragePathLabel}</span>
                      <code title={siteDataDirectory}>{siteDataDirectory || t.settingsSiteStorageUnavailable}</code>
                    </div>
                    <button
                      type="button"
                      className="settings-site-storage-icon-button"
                      onClick={() => void handleOpenSiteStorageDirectory()}
                      aria-label={t.settingsSiteStorageOpen}
                      title={t.settingsSiteStorageOpen}
                    >
                      <FontAwesomeIcon icon={faFolderOpen} />
                    </button>
                  </div>
                  <span className="settings-site-storage-sensitive-hint">{t.settingsSiteStorageSensitiveHint}</span>
                  <div className="settings-site-storage-actions">
                    <span className={`settings-site-storage-feedback ${siteStorageFeedbackError ? 'error' : 'success'}`}>
                      {siteStorageFeedback}
                    </span>
                    <button
                      type="button"
                      className="settings-site-storage-action backup"
                      onClick={() => void handleBackupSites()}
                      disabled={siteStorageBusy !== ''}
                      title={t.settingsSiteBackup}
                    >
                      <FontAwesomeIcon icon={faDownload} />
                      <span>{t.settingsSiteBackup}</span>
                    </button>
                    <button
                      type="button"
                      className="settings-site-storage-action restore"
                      onClick={() => void handleRestoreSites()}
                      disabled={siteStorageBusy !== ''}
                      title={t.settingsSiteRestore}
                    >
                      <FontAwesomeIcon icon={faRotateLeft} />
                      <span>{t.settingsSiteRestore}</span>
                    </button>
                  </div>
                </div>
              </>
            ) : null}

            {activeSection === 'display' ? (
              <>
                <div className="settings-section-card settings-section-topline">
                  <div className="settings-section-copy">
                    <strong>{t.settingsLanguage}</strong>
                    <span>{t.settingsLanguageHint}</span>
                  </div>
                  <div className="select-shell settings-select-shell">
                    <select
                      value={config.language || 'auto'}
                      aria-label={t.settingsLanguage}
                      onChange={(event) => onLanguageChange(event.target.value === 'auto' ? '' : (event.target.value as Config['language']))}
                    >
                      <option value="auto">{t.settingsLanguageAuto}</option>
                      <option value="en">{t.settingsLanguageEnglish}</option>
                      <option value="ja">{t.settingsLanguageJapanese}</option>
                      <option value="ko">{t.settingsLanguageKorean}</option>
                      <option value="zh-TW">{t.settingsLanguageTraditionalChinese}</option>
                      <option value="zh-CN">{t.settingsLanguageSimplifiedChinese}</option>
                    </select>
                    <span className="select-arrow">▾</span>
                  </div>
                </div>
                <div className="settings-section-card settings-section-topline">
                  <div className="settings-section-copy">
                    <strong>{t.settingsTheme}</strong>
                    <span>{t.settingsThemeHint}</span>
                  </div>
                  <div className="select-shell settings-select-shell">
                    <select
                      value={config.theme}
                      aria-label={t.settingsTheme}
                      onChange={(event) => onThemeChange(event.target.value as Config['theme'])}
                    >
                      <option value="neutral">{t.settingsThemeNeutral}</option>
                      <option value="light">{t.settingsThemeLight}</option>
                      <option value="dark">{t.settingsThemeDark}</option>
                      <option value="contrast">{t.settingsThemeContrast}</option>
                    </select>
                    <span className="select-arrow">▾</span>
                  </div>
                </div>
                <div className="settings-section-card settings-section-topline">
                  <div className="settings-section-copy">
                    <strong>{t.settingsFontScale}</strong>
                    <span>{t.settingsFontScaleHint}</span>
                  </div>
                  <div className="select-shell settings-select-shell">
                    <select
                      value={config.fontScale}
                      aria-label={t.settingsFontScale}
                      onChange={(event) => onFontScaleChange(event.target.value as Config['fontScale'])}
                    >
                      <option value="xsmall">{t.fontScaleXSmall}</option>
                      <option value="small">{t.fontScaleSmall}</option>
                      <option value="medium">{t.fontScaleMedium}</option>
                      <option value="large">{t.fontScaleLarge}</option>
                      <option value="xlarge">{t.fontScaleXLarge}</option>
                    </select>
                    <span className="select-arrow">▾</span>
                  </div>
                </div>
              </>
            ) : null}

            {activeSection === 'system' ? (
              <>
                <div className="settings-section-card">
                  <div className="settings-section-copy">
                    <strong>{t.settingsRememberWindowPosition}</strong>
                    <span>{t.settingsRememberWindowPositionHint}</span>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={config.rememberWindowPosition}
                    aria-label={t.settingsRememberWindowPosition}
                    className={`ios-switch ${config.rememberWindowPosition ? 'active' : ''}`}
                    onClick={() => onRememberWindowPositionChange(!config.rememberWindowPosition)}
                    title={config.rememberWindowPosition ? t.settingsOn : t.settingsOff}
                  >
                    <span className="ios-switch-track" />
                    <span className="ios-switch-thumb" />
                  </button>
                </div>
                <div className="settings-section-card">
                  <div className="settings-section-copy">
                    <strong>{t.settingsTelnetLocalEcho}</strong>
                    <span>{t.settingsTelnetLocalEchoHint}</span>
                  </div>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={config.telnetLocalEcho}
                    aria-label={t.settingsTelnetLocalEcho}
                    className={`ios-switch ${config.telnetLocalEcho ? 'active' : ''}`}
                    onClick={() => onTelnetLocalEchoChange(!config.telnetLocalEcho)}
                    title={config.telnetLocalEcho ? t.settingsOn : t.settingsOff}
                  >
                    <span className="ios-switch-track" />
                    <span className="ios-switch-thumb" />
                  </button>
                </div>
              </>
            ) : null}

            {activeSection === 'skill' ? (
              <div className="settings-section-card settings-section-stack settings-skill-card">
                <div className="settings-section-copy">
                  <strong>{t.settingsSkillTitle}</strong>
                  <span>{t.settingsSkillHint}</span>
                </div>
                <div className="settings-section-card settings-rest-server-row">
                  <div className="settings-section-copy">
                    <strong>{t.settingsRestServer}</strong>
                    <span>
                      {restStatus?.running
                        ? restStatus.attached
                          ? t.settingsRestServerStatusAttached(restStatus.mcpURL || `http://127.0.0.1:${config.restServerPort}/mcp`)
                          : t.settingsRestServerStatusRunning(restStatus.mcpURL || `http://127.0.0.1:${config.restServerPort}/mcp`)
                        : t.settingsRestServerStatusStopped(config.restServerPort)}
                    </span>
                  </div>
                  <div className="settings-rest-server-controls">
                    <div className="settings-rest-port-field">
                      <input
                        type="number"
                        min={1}
                        max={65535}
                        value={restPortDraft}
                        disabled={config.restServerEnabled}
                        aria-label={t.settingsRestServerPort}
                        title={t.settingsRestServerPortHint}
                        onChange={(event) => setRestPortDraft(event.target.value)}
                        onBlur={() => void commitRESTServerPort()}
                        onKeyDown={(event) => {
                          if (event.key === 'Enter') {
                            event.preventDefault();
                            void commitRESTServerPort();
                          }
                        }}
                      />
                    </div>
                    <button
                      type="button"
                      role="switch"
                      aria-checked={config.restServerEnabled}
                      aria-label={t.settingsRestServer}
                      className={`ios-switch ${config.restServerEnabled ? 'active' : ''}`}
                      onClick={() => void handleToggleRESTServer()}
                      title={config.restServerEnabled ? t.settingsOn : t.settingsOff}
                    >
                      <span className="ios-switch-track" />
                      <span className="ios-switch-thumb" />
                    </button>
                  </div>
                </div>
                <div className="settings-section-card settings-rest-token-card">
                  <div className="settings-section-copy">
                    <strong>{t.settingsRestServerAllowlist}</strong>
                    <span>{t.settingsRestServerAllowlistHint}</span>
                  </div>
                  <div className="settings-rest-token-controls">
                    <div className="settings-rest-token-field">
                      <input
                        type="text"
                        value={restAllowlistDraft}
                        autoComplete="off"
                        spellCheck={false}
                        disabled={config.restServerEnabled}
                        aria-label={t.settingsRestServerAllowlist}
                        placeholder="127.0.0.1"
                        onChange={(event) => setRestAllowlistDraft(event.target.value)}
                        onBlur={() => void commitRESTServerAllowlist()}
                        onKeyDown={(event) => {
                          if (event.key === 'Enter') {
                            event.preventDefault();
                            void commitRESTServerAllowlist();
                          }
                        }}
                      />
                    </div>
                  </div>
                </div>
                <div className="settings-section-card">
                  <div className="settings-section-copy">
                    <strong>{t.settingsShowTrayIcon}</strong>
                    <span>{t.settingsShowTrayIconHint}</span>
                    {trayReminder ? <span className="settings-inline-warning">{trayReminder}</span> : null}
                  </div>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={config.showTrayIcon}
                    aria-label={t.settingsShowTrayIcon}
                    className={`ios-switch ${config.showTrayIcon ? 'active' : ''}`}
                    onClick={() => void handleToggleTrayIcon()}
                    title={config.showTrayIcon ? t.settingsOn : t.settingsOff}
                  >
                    <span className="ios-switch-track" />
                    <span className="ios-switch-thumb" />
                  </button>
                </div>
                <div className={`settings-skill-viewer ${skillError ? 'error' : ''}`} aria-live="polite">
                  {skillLoading ? (
                    <p className="settings-skill-placeholder">{t.loading}</p>
                  ) : skillError ? (
                    <p className="settings-skill-placeholder">{`${t.errorPrefix} ${skillError}`}</p>
                  ) : skillMarkdown ? (
                    <ReactMarkdown
                      remarkPlugins={[remarkGfm]}
                      components={{
                        a: ({ href, children }) => (
                          <a
                            href={href}
                            onClick={(event) => {
                              if (href && /^(https?:|mailto:)/i.test(href)) {
                                event.preventDefault();
                                BrowserOpenURL(href);
                              }
                            }}
                          >
                            {children}
                          </a>
                        ),
                      }}
                    >
                      {skillMarkdown}
                    </ReactMarkdown>
                  ) : (
                    <p className="settings-skill-placeholder">{t.settingsSkillEmpty}</p>
                  )}
                </div>
                <div className="settings-skill-actions">
                  <span className={`settings-skill-feedback ${skillCopyMessage === t.settingsSkillCopyFailed ? 'error' : 'success'}`}>
                    {skillCopyMessage}
                  </span>
                  <button
                    type="button"
                    className="settings-skill-action-button"
                    onClick={() => void handleCopySkillMarkdown()}
                    disabled={!skillMarkdown || skillLoading}
                    aria-label={t.settingsSkillCopy}
                    title={t.settingsSkillCopy}
                  >
                    <FontAwesomeIcon icon={faCopy} />
                  </button>
                  <button
                    type="button"
                    className="settings-skill-action-button accent"
                    onClick={handleExportSkillMarkdown}
                    disabled={!skillMarkdown || skillLoading}
                    aria-label={t.settingsSkillExport}
                    title={t.settingsSkillExport}
                  >
                    <FontAwesomeIcon icon={faDownload} />
                  </button>
                </div>
              </div>
            ) : null}

            {activeSection === 'about' ? (
              <div className="settings-section-card settings-section-stack">
                <div className="settings-about-header">
                  <div className="settings-section-copy">
                    <strong>{t.brandTitle}</strong>
                    <span>{t.brandSubtitle}</span>
                  </div>
                  <button
                    type="button"
                    className="settings-update-check-button"
                    onClick={onCheckForUpdates}
                    disabled={updateChecking}
                    title={t.settingsUpdateCheck}
                  >
                    <FontAwesomeIcon icon={faDownload} />
                    <span>{updateChecking ? t.settingsUpdateChecking : t.settingsUpdateCheck}</span>
                  </button>
                </div>
                {updateFeedback ? (
                  <span className={`settings-update-feedback ${updateFeedbackError ? 'error' : 'success'}`} aria-live="polite">
                    {updateFeedback}
                  </span>
                ) : null}
                <div className="settings-about-meta">
                  <div className="settings-about-row">
                    <strong>{t.settingsAboutVersion}</strong>
                    <span>{BUILD_VERSION}</span>
                  </div>
                  <div className="settings-about-row">
                    <strong>{t.settingsAboutAuthor}</strong>
                    <span>Vader Chen</span>
                  </div>
                </div>
              </div>
            ) : null}
          </section>
        </div>
      </section>
    </div>
  );
}

function parseRESTServerAllowlist(value: string): string[] {
  const entries = value
    .split(/[\s,]+/)
    .map((entry) => entry.trim())
    .filter(Boolean);
  return entries.length > 0 ? Array.from(new Set(entries)) : ['127.0.0.1'];
}

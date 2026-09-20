import { useState } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faXmark } from '@fortawesome/free-solid-svg-icons';
import type { Config, PurchaseStatus } from '../types';
import { type Locale, useI18n } from '../i18n';
import { MCPSettingsPanel } from './MCPSettingsPanel';

type SettingsSection = 'general' | 'purchase' | 'display' | 'system' | 'skill' | 'about';

const BUILD_VERSION = import.meta.env.VITE_APP_VERSION ?? '1.00.00';

type Props = {
  open: boolean;
  onClose: () => void;
  config: Config;
  locale: Locale;
  purchaseStatus: PurchaseStatus;
  onLanguageChange: (language: Config['language']) => void;
  onThemeChange: (theme: Config['theme']) => void;
  onRestoreTabsChange: (restoreTabsOnStart: boolean) => void;
  onCloseTerminalTabOnDisconnectChange: (closeTerminalTabOnDisconnect: boolean) => void;
  onShowHiddenFilesChange: (showHiddenFiles: boolean) => void;
  onShowTrayIconChange: (showTrayIcon: boolean) => void;
  onRememberWindowPositionChange: (rememberWindowPosition: boolean) => void;
  onTelnetLocalEchoChange: (telnetLocalEcho: boolean) => void;
  onRESTServerEnabledChange: (restServerEnabled: boolean, restServerPort?: number) => Promise<void> | void;
  onRESTServerPortChange: (restServerPort: number) => void;
  onFontScaleChange: (scale: Config['fontScale']) => void;
  onPurchaseProUnlock: () => Promise<void>;
  onRestorePurchases: () => Promise<void>;
  onRefreshPurchaseStatus: () => Promise<void>;
};

export function SettingsModal({
  open,
  onClose,
  config,
  locale,
  purchaseStatus,
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
  onFontScaleChange,
  onPurchaseProUnlock,
  onRestorePurchases,
  onRefreshPurchaseStatus,
}: Props) {
  const t = useI18n(locale);
  const [activeSection, setActiveSection] = useState<SettingsSection>('general');
  if (!open) return null;

  const purchaseSourceLabel = resolvePurchaseSourceLabel(purchaseStatus.source, t);

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
	            <button className={`settings-nav-item ${activeSection === 'purchase' ? 'active' : ''}`} onClick={() => setActiveSection('purchase')}>
	              {t.settingsNavPurchase}
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
              </>
            ) : null}

            {activeSection === 'purchase' ? (
              <div className="settings-section-card settings-section-stack">
                <div className="settings-section-copy">
                  <strong>{t.purchaseTitle}</strong>
                  <span>{t.purchaseHint}</span>
                </div>
                <div className="settings-about-meta">
                  <div className="settings-about-row">
                    <strong>{t.purchasePlanLabel}</strong>
                    <span>
                      {purchaseStatus.proUnlock
                        ? `${t.purchasePlanPro} / ${t.purchaseTabLimitUnlimited}`
                        : `${t.purchasePlanFree} / ${t.purchaseTabLimit(2)}`}
                    </span>
                  </div>
                  <div className="settings-about-row">
                    <strong>{t.purchaseSourceLabel}</strong>
                    <span>{purchaseSourceLabel}</span>
                  </div>
                </div>
                <div className="settings-skill-actions">
                  <button className="purchase-action-button purchase-action-button-muted" onClick={() => void onRefreshPurchaseStatus()}>
                    {t.purchaseRefreshButton}
                  </button>
                  <button
                    className="purchase-action-button purchase-action-button-muted"
                    onClick={() => void onRestorePurchases()}
                    disabled={!purchaseStatus.canRestore}
                  >
                    {t.purchaseRestoreButton}
                  </button>
                  <button
                    className="purchase-action-button purchase-action-button-accent"
                    onClick={() => void onPurchaseProUnlock()}
                    disabled={!purchaseStatus.canPurchase}
                  >
                    {t.purchaseBuyButton}
                  </button>
                </div>
              </div>
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
              <MCPSettingsPanel
                config={config}
                locale={locale}
                onRESTServerEnabledChange={onRESTServerEnabledChange}
                onRESTServerPortChange={onRESTServerPortChange}
                onShowTrayIconChange={onShowTrayIconChange}
              />
            ) : null}

            {activeSection === 'about' ? (
              <div className="settings-section-card settings-section-stack">
                <div className="settings-section-copy">
                  <strong>{t.brandTitle}</strong>
                  <span>{t.brandSubtitle}</span>
                </div>
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

function resolvePurchaseSourceLabel(source: string, t: ReturnType<typeof useI18n>) {
  switch (source) {
    case 'storekit2-entitlement':
      return t.purchaseSourceAppStore;
    case 'storekit2-transaction':
      return t.purchaseSourcePurchase;
    case 'storekit2-sync':
      return t.purchaseSourceRestore;
    case 'storekit2-refresh':
    case 'storekit2':
      return t.purchaseSourceRefresh;
    case 'config':
      return t.purchaseSourceLocal;
    case '':
      return '-';
    default:
      return t.purchaseSourceUnknown;
  }
}

import { faXmark } from '@fortawesome/free-solid-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { type Locale, useI18n } from '../i18n';
import type { UpdateActionResult, UpdateCheckResult } from '../types';

type Props = {
  locale: Locale;
  result: UpdateCheckResult | null;
  actionBusy: boolean;
  actionResult: UpdateActionResult | null;
  actionError: string;
  onClose: () => void;
  onStartUpdate: () => void;
};

export function UpdateDialog({
  locale,
  result,
  actionBusy,
  actionResult,
  actionError,
  onClose,
  onStartUpdate,
}: Props) {
  const t = useI18n(locale);

  if (!result) {
    return null;
  }

  return (
    <div
      className="modal-overlay update-modal-overlay"
      onClick={(event) => {
        event.stopPropagation();
        onClose();
      }}
    >
      <section
        className="settings-modal action-modal update-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="update-dialog-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="settings-header">
          <div>
            <p className="eyebrow action-dialog-eyebrow">{t.settingsUpdateCheck}</p>
            <h2 id="update-dialog-title">{t.settingsUpdateAvailableTitle}</h2>
          </div>
          <button
            className="ghost icon-button action-cancel-button"
            onClick={onClose}
            disabled={actionBusy}
            aria-label={t.close}
          >
            <FontAwesomeIcon icon={faXmark} />
          </button>
        </div>
        <div className="update-dialog-body">
          <p className="action-message">
            {t.settingsUpdateAvailableMessage(result.currentVersion, result.latestVersion)}
          </p>
          <div className="update-version-grid">
            <div>
              <span>{t.settingsUpdateCurrentVersion}</span>
              <strong>{result.currentVersion}</strong>
            </div>
            <div>
              <span>{t.settingsUpdateLatestVersion}</span>
              <strong>{result.latestVersion}</strong>
            </div>
            {result.canDownload ? (
              <div className="update-asset-row">
                <span>{t.settingsUpdateAsset}</span>
                <strong>{result.assetName}</strong>
              </div>
            ) : null}
          </div>
          {!result.canDownload ? <p className="update-dialog-note">{t.settingsUpdateNoCompatibleAsset}</p> : null}
          {actionError ? <p className="update-dialog-status error" aria-live="polite">{actionError}</p> : null}
          {actionResult ? (
            <p className="update-dialog-status success" aria-live="polite">
              {actionResult.downloaded ? t.settingsUpdateInstallerOpened : t.settingsUpdateReleaseOpened}
            </p>
          ) : null}
        </div>
        <div className="action-buttons">
          {actionResult ? (
            <button className="primary" onClick={onClose}>{t.close}</button>
          ) : (
            <>
              <button className="ghost action-cancel-button" onClick={onClose} disabled={actionBusy}>
                {t.cancel}
              </button>
              <button className="primary" onClick={onStartUpdate} disabled={actionBusy}>
                {actionBusy
                  ? t.settingsUpdatePreparing
                  : result.canDownload
                    ? t.settingsUpdateDownloadAndOpen
                    : t.settingsUpdateOpenRelease}
              </button>
            </>
          )}
        </div>
      </section>
    </div>
  );
}

import { faXmark } from '@fortawesome/free-solid-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { type Locale, useI18n } from '../i18n';
import type { UpdateActionResult, UpdateCheckResult, UpdateProgress } from '../types';

type Props = {
  progress: UpdateProgress | null;
  locale: Locale;
  result: UpdateCheckResult | null;
  actionBusy: boolean;
  actionResult: UpdateActionResult | null;
  actionError: string;
  onClose: () => void;
  onStartUpdate: () => void;
};

export function UpdateDialog({
  progress,
  locale,
  result,
  actionBusy,
  actionResult,
  actionError,
  onClose,
  onStartUpdate,
}: Props) {
  const t = useI18n(locale);

  const stageLabel = progress?.stage === 'downloading' ? t.settingsUpdateDownloading
    : progress?.stage === 'verifying' ? t.settingsUpdateVerifying
    : progress?.stage === 'installing' ? t.settingsUpdateInstalling : t.settingsUpdatePreparing;
  const percent = progress && progress.totalBytes > 0
    ? Math.min(100, Math.floor(progress.downloadedBytes / progress.totalBytes * 100)) : null;

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
          {actionBusy && progress ? (
            <div className="update-download-progress">
              <header><span role="status">{stageLabel}</span>{progress.stage === 'downloading' && percent !== null ? <strong>{percent}%</strong> : null}</header>
              <progress aria-label={stageLabel} max={100} value={progress.stage === 'downloading' && percent !== null ? percent : undefined} />
              {progress.stage === 'downloading' ? <small>{formatBytes(progress.downloadedBytes)} / {formatBytes(progress.totalBytes)}</small> : null}
            </div>
          ) : null}
          {!result.canDownload ? <p className="update-dialog-note">{t.settingsUpdateNoCompatibleAsset}</p> : null}
          {actionError ? <p className="update-dialog-status error" aria-live="polite">{actionError}</p> : null}
          {actionResult ? (
            <p className="update-dialog-status success" aria-live="polite">
              {actionResult.installScheduled ? t.settingsUpdateInstallerOpened : actionResult.downloaded ? t.settingsUpdateInstallerOpened : t.settingsUpdateReleaseOpened}
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
                  ? stageLabel
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

function formatBytes(bytes: number) {
  return `${(Math.max(0, bytes) / (1024 * 1024)).toFixed(1)} MB`;
}

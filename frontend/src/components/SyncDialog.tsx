import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faXmark } from '@fortawesome/free-solid-svg-icons';
import type { FileComparison } from '../types';
import { type Locale, useI18n } from '../i18n';

type Props = {
  open: boolean;
  comparisons: FileComparison[];
  busy: 'upload' | 'download' | '';
  error: string;
  locale: Locale;
  onClose: () => void;
  onSync: (direction: 'upload' | 'download') => Promise<void>;
};

export function SyncDialog({ open, comparisons, busy, error, locale, onClose, onSync }: Props) {
  const t = useI18n(locale);
  if (!open) return null;

  const differences = comparisons.filter((comparison) => comparison.status !== 'same');
  const localChanges = differences.some((comparison) => comparison.status === 'local-only' || comparison.status === 'different');
  const remoteChanges = differences.some((comparison) => comparison.status === 'remote-only' || comparison.status === 'different');

  return (
    <div className="modal-overlay" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className="settings-modal action-modal sync-dialog" onMouseDown={(event) => event.stopPropagation()}>
        <div className="settings-header">
          <div>
            <p className="eyebrow action-dialog-eyebrow">{t.compareDirectories}</p>
            <h2>{t.syncDialogTitle}</h2>
          </div>
          <button type="button" className="ghost icon-button" onClick={onClose} aria-label={t.close}>
            <FontAwesomeIcon icon={faXmark} />
          </button>
        </div>
        <div className="settings-body sync-dialog-body">
          <p className="action-message">{t.syncDialogHint}</p>
          <div className="sync-summary">{t.syncSummary(differences.length)}</div>
          {error ? <p className="sync-error" role="alert">{`${t.errorPrefix} ${error}`}</p> : null}
          {differences.length === 0 ? (
            <p className="sync-empty">{t.syncNoDifferences}</p>
          ) : (
            <div className="sync-comparison-table" role="table" aria-label={t.syncDialogTitle}>
              <div className="sync-comparison-row sync-comparison-header" role="row">
                <strong>{t.fileName}</strong>
                <strong>{t.syncStatus}</strong>
                <strong>{t.syncLocalValue}</strong>
                <strong>{t.syncRemoteValue}</strong>
              </div>
              {differences.map((comparison) => (
                <div className="sync-comparison-row" role="row" key={comparison.relativePath}>
                  <span title={comparison.relativePath}>{comparison.relativePath}</span>
                  <span className={`sync-status sync-status-${comparison.status}`}>{formatStatus(comparison.status, t)}</span>
                  <span>{formatEntry(comparison.localExists, comparison.localDirectory, comparison.localSize, comparison.localModified, t)}</span>
                  <span>{formatEntry(comparison.remoteExists, comparison.remoteDirectory, comparison.remoteSize, comparison.remoteModified, t)}</span>
                </div>
              ))}
            </div>
          )}
          <div className="action-buttons sync-dialog-actions">
            <button type="button" className="ghost action-cancel-button" onClick={onClose} disabled={busy !== ''}>
              {t.cancel}
            </button>
            <button type="button" className="ghost action-cancel-button" onClick={() => void onSync('download')} disabled={!remoteChanges || busy !== ''}>
              {busy === 'download' ? t.syncPreparing : t.syncDownload}
            </button>
            <button type="button" className="primary action-cancel-button" onClick={() => void onSync('upload')} disabled={!localChanges || busy !== ''}>
              {busy === 'upload' ? t.syncPreparing : t.syncUpload}
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}

function formatStatus(status: FileComparison['status'], t: ReturnType<typeof useI18n>) {
  switch (status) {
    case 'local-only':
      return t.syncLocalOnly;
    case 'remote-only':
      return t.syncRemoteOnly;
    case 'different':
      return t.syncDifferent;
    case 'type-conflict':
      return t.syncTypeConflict;
    default:
      return t.syncSame;
  }
}

function formatEntry(exists: boolean, isDirectory: boolean, size: number, modified: string, t: ReturnType<typeof useI18n>) {
  if (!exists) return t.syncMissing;
  if (isDirectory) return t.syncDirectory;
  const details = [formatBytes(size), modified].filter(Boolean).join(' · ');
  return details || '-';
}

function formatBytes(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  if (size < 1024 * 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  return `${(size / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

import { useState } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faAngleDown, faAngleUp, faPause, faPlay, faXmark } from '@fortawesome/free-solid-svg-icons';
import type { LogItem, TransferItem } from '../types';
import { type Locale, useI18n } from '../i18n';
import { formatDirection, formatStatus } from './ProtocolLabel';

type Props = {
  transfers: TransferItem[];
  logs: LogItem[];
  expanded: boolean;
  terminalMode?: boolean;
  onExpandedChange: (expanded: boolean) => void;
  onClearCompleted: () => void;
  onClearAll: () => void;
  onTogglePauseAll: () => void;
  onClearLogs: () => void;
  onCancelTransfer: (itemID: string) => void;
  onTogglePauseTransfer: (itemID: string) => void;
  locale: Locale;
};

export function TransferPanel({
  transfers,
  logs,
  expanded,
  terminalMode = false,
  onExpandedChange,
  onClearCompleted,
  onClearAll,
  onTogglePauseAll,
  onClearLogs,
  onCancelTransfer,
  onTogglePauseTransfer,
  locale,
}: Props) {
  const t = useI18n(locale);
  const [view, setView] = useState<'queue' | 'log'>('queue');
  const visibleCount = view === 'queue' ? transfers.length : logs.length;
  const hasPausedTransfer = transfers.some((item) => item.status === 'paused');

  return (
    <section className={`card transfer-panel ${expanded ? 'expanded' : ''} ${terminalMode && !expanded ? 'terminal-collapsed' : ''}`}>
      <button
        className="transfer-collapse-handle"
        onClick={() => {
          if (!terminalMode) {
            onExpandedChange(!expanded);
          }
        }}
        disabled={terminalMode}
        aria-label={expanded ? t.collapsePanel : t.expandPanel}
        title={expanded ? t.collapsePanel : t.expandPanel}
      >
        <FontAwesomeIcon icon={expanded ? faAngleUp : faAngleDown} />
      </button>
      <div className="transfer-panel-body">
        <div className="section-title">
          <div className="transfer-header-left">
            <div className="select-shell transfer-mode-select">
              <select value={view} onChange={(event) => setView(event.target.value as 'queue' | 'log')}>
                <option value="queue">{t.transferQueue}</option>
                <option value="log">{t.transferLog}</option>
              </select>
              <span className="select-arrow">▾</span>
            </div>
            <span className="transfer-count-label">{t.transferCount(visibleCount)}</span>
          </div>
          <div className="transfer-actions">
            {view === 'queue' ? (
              <>
                <button className="ghost compact-button" onClick={onTogglePauseAll} disabled={transfers.length === 0}>
                  {hasPausedTransfer ? t.resumeAll : t.pauseAll}
                </button>
                <button className="ghost compact-button" onClick={onClearCompleted}>
                  {t.clearCompleted}
                </button>
                <button className="ghost compact-button" onClick={onClearAll}>
                  {t.clearAll}
                </button>
              </>
            ) : (
              <button className="ghost compact-button" onClick={onClearLogs}>
                {t.clearLogs}
              </button>
            )}
          </div>
        </div>
        {view === 'queue' ? (
        <div className="transfer-list">
          {transfers.map((item, index) => (
            <article key={item.id} className="transfer-item">
              <span className="transfer-index">{index + 1}.</span>
              <strong className="transfer-name">{item.name}</strong>
              <span className="transfer-speed">{formatSpeed(item.speedBps, item.status, t.statusPaused)}</span>
              <span className="transfer-attempt">
                {item.attempt && item.maxAttempts ? t.transferAttempt(item.attempt, item.maxAttempts) : ''}
              </span>
              <div className="transfer-progress-group">
                <div
                  className={`transfer-progress-line ${item.status === 'running' && item.progress === 0 ? 'indeterminate' : ''}`}
                  aria-label={`${formatDirection(item.direction, locale)} ${formatStatus(item.status, locale)} ${item.progress}%`}
                  title={`${formatDirection(item.direction, locale)} · ${formatStatus(item.status, locale)} · ${item.progress}%`}
                >
                  <div style={{ width: `${item.status === 'running' && item.progress === 0 ? 35 : item.progress}%` }} />
                </div>
                <span className="transfer-progress-value">{item.progress}%</span>
              </div>
              {item.error ? <span className="transfer-error" title={item.error}>{item.error}</span> : null}
              <button
                className="ghost compact-icon-button transfer-pause-button"
                onClick={() => onTogglePauseTransfer(item.id)}
                disabled={item.status !== 'running' && item.status !== 'paused'}
                aria-label={item.status === 'paused' ? t.resume : t.pause}
                title={item.status === 'paused' ? t.resume : t.pause}
              >
                <FontAwesomeIcon icon={item.status === 'paused' ? faPlay : faPause} />
              </button>
              <button
                className="ghost compact-icon-button transfer-cancel-button"
                onClick={() => onCancelTransfer(item.id)}
                disabled={item.status !== 'running' && item.status !== 'paused'}
                aria-label={t.cancel}
                title={t.cancel}
              >
                <FontAwesomeIcon icon={faXmark} />
              </button>
            </article>
          ))}
        </div>
        ) : (
          <div className="transfer-log-list">
            {logs.map((entry, index) => (
              <article key={entry.id} className={`transfer-log-item status-${entry.status}`}>
                <span className="transfer-index">{index + 1}.</span>
                <strong>{entry.createdAt}</strong>
                <small>{entry.message}</small>
              </article>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

function formatSpeed(speedBps: number, status: TransferItem['status'], pausedLabel: string) {
  if (status === 'paused') {
    return pausedLabel;
  }
  if (status !== 'running' || speedBps <= 0) {
    return '-';
  }
  return formatBytesPerSecond(speedBps);
}

function formatBytesPerSecond(speedBps: number) {
  if (speedBps < 1024) return `${speedBps} bytes/s`;
  if (speedBps < 1024 * 1024) return `${(speedBps / 1024).toFixed(1)} KB/s`;
  if (speedBps < 1024 * 1024 * 1024) return `${(speedBps / (1024 * 1024)).toFixed(1)} MB/s`;
  return `${(speedBps / (1024 * 1024 * 1024)).toFixed(1)} GB/s`;
}

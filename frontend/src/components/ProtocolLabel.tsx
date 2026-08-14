import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faLock, faUnlock } from '@fortawesome/free-solid-svg-icons';
import { type Locale, useI18n } from '../i18n';
import type { Site } from '../types';

type Props = {
  protocol: Site['protocol'];
  locale: Locale;
};

export function ProtocolLabel({ protocol, locale }: Props) {
  const t = useI18n(locale);
  const encrypted = protocol === 'sftp';

  return (
    <span
      className={`protocol-badge ${protocol}`}
      aria-label={encrypted ? t.protocolSecure : t.protocolInsecure}
      title={encrypted ? t.protocolSecure : t.protocolInsecure}
    >
      <FontAwesomeIcon icon={encrypted ? faLock : faUnlock} />
    </span>
  );
}

export function formatDirection(direction: 'upload' | 'download', locale: Locale) {
  const t = useI18n(locale);
  return direction === 'upload' ? t.directionUpload : t.directionDownload;
}

export function formatStatus(status: 'running' | 'paused' | 'done' | 'failed' | 'cancelled', locale: Locale) {
  const t = useI18n(locale);
  if (status === 'done') return t.statusDone;
  if (status === 'failed') return t.statusFailed;
  if (status === 'cancelled') return t.statusCancelled;
  if (status === 'paused') return t.statusPaused;
  return t.statusRunning;
}

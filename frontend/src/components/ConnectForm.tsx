import type { Site } from '../types';
import { type Locale, useI18n } from '../i18n';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faChevronDown, faChevronUp, faFloppyDisk, faXmark } from '@fortawesome/free-solid-svg-icons';

type Props = {
  draft: Site;
  onChange: (site: Site) => void;
  onSave: () => void;
  canSave: boolean;
  isDirty: boolean;
  expanded: boolean;
  onToggle: () => void;
  onClose?: () => void;
  dialogTitleId?: string;
  variant?: 'card' | 'dialog';
  locale: Locale;
};

export function ConnectForm({ draft, onChange, onSave, canSave, isDirty, expanded, onToggle, onClose, dialogTitleId, variant = 'card', locale }: Props) {
  const t = useI18n(locale);
  const supportsPPK = draft.protocol === 'sftp';
  const textInputProps = {
    autoCapitalize: 'none' as const,
    autoCorrect: 'off' as const,
    autoComplete: 'off',
    spellCheck: false,
  };

  const update = <K extends keyof Site>(key: K, value: Site[K]) => {
    onChange({ ...draft, [key]: value });
  };

  const choosePPK = async () => {
    const selected = await window.go?.app?.App?.SelectPPKFile?.();
    if (selected) {
      update('ppkPath', selected);
    }
  };

  // 相容舊沙盒版本，選取金鑰時仍保留可跨版本使用的路徑資料。
  // 授權整個資料夾一次，即可涵蓋其下所有金鑰，免去逐一重選。
  const authorizeKeyDirectory = async () => {
    await window.go?.app?.App?.AuthorizeKeyDirectory?.(draft.ppkPath ?? "");
  };

  return (
    <section className={`${variant === 'dialog' ? 'site-editor-form' : 'card form-card'}`}>
      <div className="section-title site-editor-header">
        <h3 id={dialogTitleId}>{draft.id ? t.editSite : t.quickAdd}</h3>
        <div className="form-header-actions">
          <button
            className={`site-view-button form-action-button ${isDirty ? 'dirty' : ''}`}
            onClick={onSave}
            aria-label={t.saveSite}
            title={t.saveSite}
            disabled={!canSave}
          >
            <FontAwesomeIcon icon={faFloppyDisk} />
          </button>
          {variant === 'card' ? (
            <button
              className="site-view-button form-action-button toggle-button"
              onClick={onToggle}
              aria-label={expanded ? t.collapse : t.expand}
              title={expanded ? t.collapse : t.expand}
            >
              <FontAwesomeIcon icon={expanded ? faChevronUp : faChevronDown} />
            </button>
          ) : null}
          {onClose ? (
            <button
              className="site-view-button form-action-button toggle-button"
              onClick={onClose}
              aria-label={t.close}
              title={t.close}
            >
              <FontAwesomeIcon icon={faXmark} />
            </button>
          ) : null}
        </div>
      </div>
      {expanded ? (
        <>
          <div className="form-grid">
            <label>
              <span>{t.fieldName}</span>
              <input value={draft.name} onChange={(e) => update('name', e.target.value)} placeholder={t.placeholderSiteName} />
            </label>
            <label>
              <span>{t.fieldFolder}</span>
              <input {...textInputProps} value={draft.folder ?? ''} onChange={(e) => update('folder', e.target.value)} placeholder={t.placeholderFolder} />
            </label>
            <label>
              <span>{t.fieldTags}</span>
              <input
                {...textInputProps}
                value={(draft.tags ?? []).join(', ')}
                onChange={(e) => update('tags', parseTags(e.target.value))}
                placeholder={t.placeholderTags}
              />
            </label>
            <label className="form-checkbox-label">
              <span>{t.fieldFavorite}</span>
              <input
                type="checkbox"
                checked={draft.favorite ?? false}
                onChange={(e) => update('favorite', e.target.checked)}
              />
            </label>
            <label>
              <span>{t.fieldProtocol}</span>
              <div className="select-shell">
                <select
                  value={draft.protocol}
                  onChange={(e) => {
                    const protocol = e.target.value as Site['protocol'];
                    const nextPort =
                      protocol === 'sftp' && draft.port === 21
                        ? 22
                        : protocol === 'ftp' && draft.port === 22
                          ? 21
                          : draft.port;
                    onChange({ ...draft, protocol, port: nextPort });
                  }}
                >
                  <option value="sftp">{t.protocolSSHSFTP}</option>
                  <option value="ftp">{t.protocolTelnetFTP}</option>
                </select>
                <span className="select-arrow" aria-hidden="true">
                  <FontAwesomeIcon icon={faChevronDown} />
                </span>
              </div>
            </label>
            <label>
              <span>{t.fieldHost}</span>
              <input {...textInputProps} value={draft.host} onChange={(e) => update('host', e.target.value)} placeholder={t.placeholderHost} />
            </label>
            <label>
              <span>{t.fieldPort}</span>
              <input
                {...textInputProps}
                type="number"
                value={draft.port}
                onChange={(e) => update('port', Number(e.target.value))}
                placeholder={t.placeholderPort}
              />
            </label>
            <label>
              <span>{t.fieldUsername}</span>
              <input {...textInputProps} value={draft.username} onChange={(e) => update('username', e.target.value)} placeholder={t.placeholderUsername} />
            </label>
            <label>
              <span>{t.fieldPassword}</span>
              <input
                {...textInputProps}
                type="password"
                value={draft.password}
                onChange={(e) => update('password', e.target.value)}
                placeholder={t.placeholderPassword}
              />
            </label>
            {supportsPPK ? (
              <>
                <label>
                  <span>{t.fieldPPKPath}</span>
                  <div className="path-picker">
                    <input
                      {...textInputProps}
                      value={draft.ppkPath}
                      onChange={(e) => update('ppkPath', e.target.value)}
                      placeholder={t.placeholderPPKPath}
                    />
                    <button type="button" className="site-view-button picker-button" onClick={choosePPK}>
                      {t.choosePPK}
                    </button>
                    <button
                      type="button"
                      className="site-view-button picker-button"
                      onClick={authorizeKeyDirectory}
                      title={t.authorizeKeyDirectoryHint}
                    >
                      {t.authorizeKeyDirectory}
                    </button>
                  </div>
                </label>
                <label>
                  <span>{t.fieldPPKPassphrase}</span>
                  <input
                    {...textInputProps}
                    type="password"
                    value={draft.ppkPassphrase}
                    onChange={(e) => update('ppkPassphrase', e.target.value)}
                    placeholder={t.placeholderPPKPassphrase}
                  />
                </label>
              </>
            ) : null}
            <label>
              <span>{t.fieldLocalPath}</span>
              <input {...textInputProps} value={draft.localPath} onChange={(e) => update('localPath', e.target.value)} />
            </label>
            <label>
              <span>{t.fieldRemotePath}</span>
              <input {...textInputProps} value={draft.remotePath} onChange={(e) => update('remotePath', e.target.value)} />
            </label>
          </div>

        </>
      ) : null}
    </section>
  );
}

function parseTags(value: string) {
  return value
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean);
}

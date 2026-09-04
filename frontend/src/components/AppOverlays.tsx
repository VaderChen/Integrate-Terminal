import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faXmark } from '@fortawesome/free-solid-svg-icons';
import { type Locale, useI18n } from '../i18n';
import type { FileEntry, Site } from '../types';
import type { ActionDialogState, ConnectDialogState, HostTrustDialogState, PathContextMenuState, SiteFolderDialogState, TerminalUploadConfirmState } from '../appTypes';
import type { FileContextMenuRequest } from './FilePanel';
import { SiteList } from './SiteList';

type ContextMenuProps = {
  request: FileContextMenuRequest;
  locale: Locale;
  onClose: () => void;
  onOpenEntry: () => void;
  onExecuteEntry: () => void;
  onCreateDirectory: () => void;
  onRenameEntry: () => void;
  onDeleteEntry: () => void;
  onDownloadEntryTo: () => void;
  onRefresh: () => void;
};

export function FileActionContextMenu({
  request,
  locale,
  onClose,
  onOpenEntry,
  onExecuteEntry,
  onCreateDirectory,
  onRenameEntry,
  onDeleteEntry,
  onDownloadEntryTo,
  onRefresh,
}: ContextMenuProps) {
  const t = useI18n(locale);

  return (
    <div className="context-menu" style={{ left: request.x, top: request.y }} onClick={(event) => event.stopPropagation()}>
      {request.entry && request.entry.name !== '..' && request.side === 'local' ? (
        <button className="context-menu-item" onMouseDown={(event) => handleMenuAction(event, onOpenEntry)}>
          {t.openItem}
        </button>
      ) : null}
      {request.entry && request.entry.name !== '..' && request.side === 'local' && !request.entry.isDir ? (
        <button className="context-menu-item" onMouseDown={(event) => handleMenuAction(event, onExecuteEntry)}>
          {t.executeItem}
        </button>
      ) : null}
      {request.entry && request.entry.name !== '..' && request.side === 'local' ? <div className="context-menu-separator" /> : null}
      <button className="context-menu-item" onMouseDown={(event) => handleMenuAction(event, onCreateDirectory)}>
        {t.newFolder}
      </button>
      {request.entry ? (
        <button className="context-menu-item" onMouseDown={(event) => handleMenuAction(event, onRenameEntry)}>
          {t.renameItem}
        </button>
      ) : null}
      {request.entry ? (
        <button className="context-menu-item danger" onMouseDown={(event) => handleMenuAction(event, onDeleteEntry)}>
          {t.deleteItem}
        </button>
      ) : null}
      <div className="context-menu-separator" />
      {request.entry && request.side === 'remote' && request.entry.name !== '..' ? (
        <button className="context-menu-item" onMouseDown={(event) => handleMenuAction(event, onDownloadEntryTo)}>
          {t.downloadTo}
        </button>
      ) : null}
      <button className="context-menu-item" onMouseDown={(event) => handleMenuAction(event, onRefresh)}>
        {t.refreshNow}
      </button>
      <button className="context-menu-item" onMouseDown={(event) => handleMenuAction(event, onClose)}>
        {t.close}
      </button>
    </div>
  );
}

type PathContextMenuProps = {
  request: PathContextMenuState;
  locale: Locale;
  onClose: () => void;
  onOpenPath: () => void;
  onCopyPath: () => void;
};

export function PathContextMenu({ request, locale, onClose, onOpenPath, onCopyPath }: PathContextMenuProps) {
  const t = useI18n(locale);

  return (
    <div className="context-menu" style={{ left: request.x, top: request.y }} onClick={(event) => event.stopPropagation()}>
      <button className="context-menu-item" onMouseDown={(event) => handleMenuAction(event, onOpenPath)}>
        {request.side === 'local' ? t.openInFinder : t.openInSSH}
      </button>
      <button className="context-menu-item" onMouseDown={(event) => handleMenuAction(event, onCopyPath)}>
        {t.copyPath}
      </button>
    </div>
  );
}

type ConnectModalProps = {
  dialog: ConnectDialogState;
  locale: Locale;
  connectingMode: 'ssh' | 'sftp' | 'telnet' | 'ftp' | null;
  onClose: () => void;
  onConfirm: (mode: 'ssh' | 'sftp' | 'telnet' | 'ftp') => void;
};

export function ConnectMethodModal({ dialog, locale, connectingMode, onClose, onConfirm }: ConnectModalProps) {
  const t = useI18n(locale);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <section className="settings-modal action-modal connect-choice-modal" onClick={(event) => event.stopPropagation()}>
        <div className="settings-header">
          <div>
            <p className="eyebrow delete-dialog-eyebrow">{t.connectMethodLabel}</p>
            <h2>{dialog.site.name || dialog.site.host}</h2>
          </div>
          <button className="ghost icon-button action-cancel-button" onClick={onClose} aria-label={t.close}>
            <FontAwesomeIcon icon={faXmark} />
          </button>
        </div>
        <div className="settings-body connect-choice-body">
          <p className="action-message connect-choice-message">
            {dialog.site.protocol === 'sftp' ? t.chooseSecureConnectionMethod : t.chooseLegacyConnectionMethod}
          </p>
          <div className="connect-choice-actions">
            {dialog.site.protocol === 'sftp' ? (
              <>
                <ConnectModeButton mode="ssh" connectingMode={connectingMode} label={t.connectingSSH} idleLabel="SSH" onClick={onConfirm} />
                <ConnectModeButton mode="sftp" connectingMode={connectingMode} label={t.connectingSFTP} idleLabel="SFTP" onClick={onConfirm} ghost />
              </>
            ) : (
              <>
                <ConnectModeButton mode="telnet" connectingMode={connectingMode} label={t.connectingTelnet} idleLabel="Telnet" onClick={onConfirm} />
                <ConnectModeButton mode="ftp" connectingMode={connectingMode} label={t.connectingFTP} idleLabel="FTP" onClick={onConfirm} ghost />
              </>
            )}
          </div>
          <p className="connect-choice-hint">
            {dialog.site.protocol === 'sftp' ? t.sshConnectHint : t.ftpConnectHint}
          </p>
        </div>
      </section>
    </div>
  );
}

type HostTrustModalProps = {
  dialog: HostTrustDialogState;
  locale: Locale;
  onApprove: () => void;
  onClose: () => void;
};

export function HostTrustModal({ dialog, locale, onApprove, onClose }: HostTrustModalProps) {
  const t = useI18n(locale);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <section className="settings-modal action-modal connect-choice-modal host-trust-modal" onClick={(event) => event.stopPropagation()}>
        <div className="settings-header">
          <div>
            <p className="eyebrow delete-dialog-eyebrow">{t.hostTrustLabel}</p>
            <h2>{dialog.site.name || dialog.site.host}</h2>
          </div>
          <button className="ghost icon-button action-cancel-button" onClick={onClose} aria-label={t.close}>
            <FontAwesomeIcon icon={faXmark} />
          </button>
        </div>
        <div className="settings-body connect-choice-body">
          <p className="action-message connect-choice-message">
            {t.hostTrustDescription(
              dialog.prompt.host,
              dialog.prompt.port,
              dialog.prompt.replacesExisting ?? false,
            )}
          </p>
          <div className="host-trust-details">
            <strong>{t.hostTrustFingerprint}</strong>
            <code>{dialog.prompt.fingerprintSHA256}</code>
            <strong>{t.hostTrustKeyType}</strong>
            <code>{dialog.prompt.keyType}</code>
          </div>
          <p className="connect-choice-hint">{t.hostTrustHint}</p>
          <div className="connect-choice-actions">
            <button className="primary action-cancel-button connect-choice-button" onClick={onApprove}>
              {t.hostTrustApprove}
            </button>
            <button className="ghost action-cancel-button connect-choice-button" onClick={onClose}>
              {t.cancel}
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}

type FileActionModalProps = {
  dialog: ActionDialogState;
  locale: Locale;
  directoryName: string;
  renameValue: string;
  plainTextInputProps: Record<string, unknown>;
  onDirectoryNameChange: (value: string) => void;
  onRenameValueChange: (value: string) => void;
  onConfirm: () => void;
  onClose: () => void;
};

export function FileActionModal({
  dialog,
  locale,
  directoryName,
  renameValue,
  plainTextInputProps,
  onDirectoryNameChange,
  onRenameValueChange,
  onConfirm,
  onClose,
}: FileActionModalProps) {
  const t = useI18n(locale);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <section className="settings-modal action-modal" onClick={(event) => event.stopPropagation()}>
        <div className={`settings-header ${dialog.mode === 'delete' ? 'delete-dialog-header' : ''}`}>
          <div>
            <p className={`eyebrow ${dialog.mode === 'delete' ? 'delete-dialog-eyebrow' : 'action-dialog-eyebrow'}`}>
              {dialog.mode === 'mkdir' ? t.newFolder : dialog.mode === 'rename' ? t.renameItem : t.deleteItem}
            </p>
            <h2>{dialog.mode === 'mkdir' ? t.enterDirectoryName : dialog.mode === 'rename' ? t.enterNewName : t.confirmDelete}</h2>
          </div>
          <button className="ghost icon-button" onClick={onClose} aria-label={t.close}>
            x
          </button>
        </div>
        <div className="settings-body">
          {dialog.mode === 'mkdir' ? (
            <label>
              <span>{t.newFolder}</span>
              <input
                {...plainTextInputProps}
                autoFocus
                value={directoryName}
                placeholder={t.folderNamePlaceholder}
                onChange={(event) => onDirectoryNameChange(event.target.value)}
                onKeyDown={(event) => handleEnter(event, onConfirm)}
              />
            </label>
          ) : dialog.mode === 'rename' ? (
            <label>
              <span>{t.renameItem}</span>
              <input
                {...plainTextInputProps}
                autoFocus
                value={renameValue}
                onChange={(event) => onRenameValueChange(event.target.value)}
                onKeyDown={(event) => handleEnter(event, onConfirm)}
              />
            </label>
          ) : (
            <p className="action-message delete-action-message">
              {dialog.entries.length > 1 ? t.deleteTargets(dialog.entries.length) : t.deleteTarget(dialog.entry.name)}
            </p>
          )}
          <div className="action-buttons">
            <button className="ghost action-cancel-button" onClick={onClose}>
              {t.cancel}
            </button>
            <button className={dialog.mode === 'delete' ? 'danger action-confirm-delete-button' : 'primary'} onClick={onConfirm}>
              {dialog.mode === 'delete' ? t.confirm : dialog.mode === 'rename' ? t.confirm : t.create}
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}

type TerminalUploadConfirmModalProps = {
  dialog: TerminalUploadConfirmState;
  locale: Locale;
  onConfirm: () => void;
  onClose: () => void;
};

export function TerminalUploadConfirmModal({ dialog, locale, onConfirm, onClose }: TerminalUploadConfirmModalProps) {
  const t = useI18n(locale);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <section className="settings-modal action-modal" onClick={(event) => event.stopPropagation()}>
        <div className="settings-header">
          <div>
            <h2>{t.confirmUploadToSSH}</h2>
          </div>
          <button className="ghost icon-button" onClick={onClose} aria-label={t.close}>
            x
          </button>
        </div>
        <div className="settings-body">
          <p className="action-message terminal-upload-confirm-message">{t.confirmUploadToSSHDescription(dialog.paths.length, dialog.remotePath)}</p>
          <div className="action-buttons">
            <button className="ghost action-cancel-button" onClick={onClose}>
              {t.cancel}
            </button>
            <button className="primary action-cancel-button" onClick={onConfirm}>
              {t.confirm}
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}

type SiteFolderActionModalProps = {
  dialog: SiteFolderDialogState;
  locale: Locale;
  folderName: string;
  plainTextInputProps: Record<string, unknown>;
  onFolderNameChange: (value: string) => void;
  onConfirm: () => void;
  onClose: () => void;
};

export function SiteFolderActionModal({
  dialog,
  locale,
  folderName,
  plainTextInputProps,
  onFolderNameChange,
  onConfirm,
  onClose,
}: SiteFolderActionModalProps) {
  const t = useI18n(locale);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <section className="settings-modal action-modal" onClick={(event) => event.stopPropagation()}>
        <div className={`settings-header ${dialog.mode === 'delete' ? 'delete-dialog-header' : ''}`}>
          <div>
            <p className={`eyebrow ${dialog.mode === 'delete' ? 'delete-dialog-eyebrow' : 'action-dialog-eyebrow'}`}>
              {dialog.mode === 'create' ? t.siteFolderCreate : dialog.mode === 'rename' ? t.renameItem : t.siteFolderDelete}
            </p>
            <h2>{dialog.mode === 'delete' ? t.confirmDelete : dialog.mode === 'rename' ? t.enterNewName : t.enterDirectoryName}</h2>
          </div>
          <button className="ghost icon-button" onClick={onClose} aria-label={t.close}>
            x
          </button>
        </div>
        <div className="settings-body">
          {dialog.mode === 'delete' ? (
            <p className="action-message delete-action-message">
              {t.siteFolderDeleteConfirm(dialog.folder, dialog.siteCount)}
            </p>
          ) : (
            <label>
              <span>{dialog.mode === 'rename' ? t.renameItem : t.siteFolderNameLabel}</span>
              <input
                {...plainTextInputProps}
                autoFocus
                value={folderName}
                placeholder={t.folderNamePlaceholder}
                onChange={(event) => onFolderNameChange(event.target.value)}
                onKeyDown={(event) => handleEnter(event, onConfirm)}
              />
            </label>
          )}
          <div className="action-buttons">
            <button className="ghost action-cancel-button" onClick={onClose}>
              {t.cancel}
            </button>
            <button className={dialog.mode === 'delete' ? 'danger action-confirm-delete-button' : 'primary'} onClick={onConfirm}>
              {dialog.mode === 'delete' ? t.confirm : dialog.mode === 'rename' ? t.confirm : t.create}
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}

type SiteLibraryModalProps = {
  open: boolean;
  locale: Locale;
  sites: Site[];
  siteFolders?: string[];
  onClose: () => void;
  onOpenSite: (site: Site) => void;
  onCopySite: (site: Site) => void;
  onDeleteSite: (siteId: string) => void;
  onSortByName: () => void;
  onReorderSites: (siteIDs: string[]) => void;
  onReorderFolders?: (folderNames: string[]) => void;
  onMoveSiteToFolder?: (siteId: string, folder: string) => void;
  onEditSite: (site: Site) => void;
  onCreateFolder?: () => void;
  onSortFolders?: () => void;
  onRenameFolder?: (folder: string) => void;
  onDeleteFolder?: (folder: string, siteCount: number) => void;
};

export function SiteLibraryModal({
  open,
  locale,
  sites,
  siteFolders = [],
  onClose,
  onOpenSite,
  onCopySite,
  onDeleteSite,
  onSortByName,
  onReorderSites,
  onReorderFolders = () => undefined,
  onMoveSiteToFolder = () => undefined,
  onEditSite,
  onCreateFolder = () => undefined,
  onSortFolders = () => undefined,
  onRenameFolder = () => undefined,
  onDeleteFolder = () => undefined,
}: SiteLibraryModalProps) {
  const t = useI18n(locale);

  if (!open) {
    return null;
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <section className="settings-modal action-modal site-library-modal" onClick={(event) => event.stopPropagation()}>
        <div className="settings-header modal-header">
          <div>
            <p className="eyebrow action-dialog-eyebrow">{t.siteListTitle}</p>
            <h2>{t.siteListCount(sites.length)}</h2>
          </div>
          <button className="ghost icon-button" onClick={onClose} aria-label={t.close}>
            <FontAwesomeIcon icon={faXmark} />
          </button>
        </div>
        <div className="settings-body modal-body">
          <SiteList
            embedded
            locale={locale}
            sites={sites}
            siteFolders={siteFolders}
            onOpenSite={(site) => {
              onOpenSite(site);
              onClose();
            }}
            onCopySite={onCopySite}
            onDeleteSite={onDeleteSite}
            onSortByName={onSortByName}
            onCreateFolder={onCreateFolder}
            onSortFolders={onSortFolders}
            onRenameFolder={onRenameFolder}
            onDeleteFolder={onDeleteFolder}
            onReorderSites={onReorderSites}
            onReorderFolders={onReorderFolders}
            onMoveSiteToFolder={onMoveSiteToFolder}
            onEditSite={(site) => {
              onEditSite(site);
              onClose();
            }}
          />
        </div>
      </section>
    </div>
  );
}

function ConnectModeButton({
  mode,
  connectingMode,
  label,
  idleLabel,
  onClick,
  ghost = false,
}: {
  mode: 'ssh' | 'sftp' | 'telnet' | 'ftp';
  connectingMode: 'ssh' | 'sftp' | 'telnet' | 'ftp' | null;
  label: string;
  idleLabel: string;
  onClick: (mode: 'ssh' | 'sftp' | 'telnet' | 'ftp') => void;
  ghost?: boolean;
}) {
  return (
    <button
      className={`${ghost ? 'ghost' : 'primary'} action-cancel-button connect-choice-button ${connectingMode === mode ? 'is-connecting' : ''}`}
      onClick={() => onClick(mode)}
      disabled={connectingMode !== null}
    >
      {connectingMode === mode ? <span className="connect-choice-spinner" aria-hidden="true" /> : null}
      <span>{connectingMode === mode ? label : idleLabel}</span>
    </button>
  );
}

function handleMenuAction(event: React.MouseEvent<HTMLButtonElement>, action: () => void) {
  event.preventDefault();
  event.stopPropagation();
  action();
}

function handleEnter(event: React.KeyboardEvent<HTMLInputElement>, onConfirm: () => void) {
  if (event.key !== 'Enter') {
    return;
  }
  event.preventDefault();
  onConfirm();
}

export function isDownloadableRemoteEntry(entry?: FileEntry, side?: 'local' | 'remote') {
  return !!entry && side === 'remote' && entry.name !== '..';
}

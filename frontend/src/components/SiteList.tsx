import { useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import {
  faBars,
  faChevronDown,
  faChevronRight,
  faClone,
  faFolderPlus,
  faGrip,
  faList,
  faPenToSquare,
  faPlug,
  faSortAlphaDown,
  faTrashCan,
} from '@fortawesome/free-solid-svg-icons';
import type { Site } from '../types';
import { type Locale, useI18n } from '../i18n';
import { ProtocolLabel } from './ProtocolLabel';
import { DEFAULT_FOLDER_KEY, groupSitesByFolder, normalizeFolderKey } from './siteListUtils';

type SiteViewMode = 'card' | 'compact' | 'list';

type SiteContextMenuState =
  | { type: 'site'; x: number; y: number; site: Site }
  | { type: 'library'; x: number; y: number }
  | { type: 'folder'; x: number; y: number; folderName: string; label: string; siteCount: number; canDelete: boolean };

type Props = {
  sites: Site[];
  siteFolders: string[];
  onOpenSite: (site: Site) => void;
  onCopySite: (site: Site) => void;
  onDeleteSite: (siteId: string) => void;
  onSortByName: () => void;
  onReorderSites: (siteIDs: string[]) => void;
  onReorderFolders: (folderNames: string[]) => void;
  onMoveSiteToFolder: (siteId: string, folder: string) => void;
  onEditSite: (site: Site) => void;
  onCreateFolder: () => void;
  onSortFolders: () => void;
  onRenameFolder: (folder: string) => void;
  onDeleteFolder: (folder: string, siteCount: number) => void;
  locale: Locale;
  embedded?: boolean;
};

export function SiteList({
  sites,
  siteFolders,
  onOpenSite,
  onCopySite,
  onDeleteSite,
  onSortByName,
  onReorderSites,
  onReorderFolders,
  onMoveSiteToFolder,
  onEditSite,
  onCreateFolder,
  onSortFolders,
  onRenameFolder,
  onDeleteFolder,
  locale,
  embedded = false,
}: Props) {
  const t = useI18n(locale);
  const [viewMode, setViewMode] = useState<SiteViewMode>('compact');
  const [contextMenu, setContextMenu] = useState<SiteContextMenuState | null>(null);
  const [draggedSiteId, setDraggedSiteId] = useState('');
  const [dragOverSiteId, setDragOverSiteId] = useState('');
  const [draggedFolderKey, setDraggedFolderKey] = useState('');
  const [dragOverFolderKey, setDragOverFolderKey] = useState('');
  const [collapsedFolders, setCollapsedFolders] = useState<Record<string, boolean>>({});
  const groupedSites = useMemo(() => groupSitesByFolder(sites, siteFolders, t.defaultSiteFolder), [siteFolders, sites, t.defaultSiteFolder]);
  const appShellFontSize = typeof window !== 'undefined'
    ? window.getComputedStyle(document.querySelector('.app-shell') ?? document.body).fontSize
    : undefined;

  useEffect(() => {
    const closeMenu = () => setContextMenu(null);
    window.addEventListener('click', closeMenu);
    window.addEventListener('blur', closeMenu);
    return () => {
      window.removeEventListener('click', closeMenu);
      window.removeEventListener('blur', closeMenu);
    };
  }, []);

  useEffect(() => {
    setCollapsedFolders((current) => {
      const next = { ...current };
      for (const group of groupedSites) {
        if (!(group.key in next)) {
          next[group.key] = true;
        }
      }
      return next;
    });
  }, [groupedSites]);

  const contextMenuStyle = contextMenu
    ? {
        left: Math.min(contextMenu.x, window.innerWidth - 196),
        top: Math.min(contextMenu.y, window.innerHeight - 180),
        fontSize: appShellFontSize,
      }
    : undefined;

  const moveSite = (sourceID: string, targetID: string) => {
    if (!sourceID || !targetID || sourceID === targetID) {
      return;
    }

    const nextSites = [...sites];
    const sourceIndex = nextSites.findIndex((site) => site.id === sourceID);
    const targetIndex = nextSites.findIndex((site) => site.id === targetID);
    if (sourceIndex < 0 || targetIndex < 0) {
      return;
    }
    if (normalizeFolderKey(nextSites[sourceIndex].folder) !== normalizeFolderKey(nextSites[targetIndex].folder)) {
      return;
    }

    const [moved] = nextSites.splice(sourceIndex, 1);
    nextSites.splice(targetIndex, 0, moved);
    onReorderSites(nextSites.map((site) => site.id));
  };

  const moveFolder = (sourceKey: string, targetKey: string) => {
    if (!sourceKey || !targetKey || sourceKey === targetKey || sourceKey === DEFAULT_FOLDER_KEY || targetKey === DEFAULT_FOLDER_KEY) {
      return;
    }

    const nextFolders = [...siteFolders];
    const sourceIndex = nextFolders.findIndex((folder) => folder.trim() === sourceKey);
    const targetIndex = nextFolders.findIndex((folder) => folder.trim() === targetKey);
    if (sourceIndex < 0 || targetIndex < 0) {
      return;
    }

    const [moved] = nextFolders.splice(sourceIndex, 1);
    nextFolders.splice(targetIndex, 0, moved);
    onReorderFolders(nextFolders);
  };

  const content = (
    <>
      <div className="section-title site-list-title-row">
        <div className="site-list-title-group">
          <h3>{t.siteListTitle}</h3>
          <span>{t.siteListCount(sites.length)}</span>
        </div>
        <div className="site-view-switcher" role="group" aria-label={t.siteViewSwitcher}>
          <button
            type="button"
            className="site-view-button"
            aria-label={t.siteFolderCreate}
            title={t.siteFolderCreate}
            onClick={onCreateFolder}
          >
            <FontAwesomeIcon icon={faFolderPlus} />
          </button>
          <button
            type="button"
            className={`site-view-button ${viewMode === 'card' ? 'active' : ''}`}
            aria-label={t.siteViewCard}
            title={t.siteViewCard}
            onClick={() => setViewMode('card')}
          >
            <FontAwesomeIcon icon={faGrip} />
          </button>
          <button
            type="button"
            className={`site-view-button ${viewMode === 'compact' ? 'active' : ''}`}
            aria-label={t.siteViewCompact}
            title={t.siteViewCompact}
            onClick={() => setViewMode('compact')}
          >
            <FontAwesomeIcon icon={faBars} />
          </button>
          <button
            type="button"
            className={`site-view-button ${viewMode === 'list' ? 'active' : ''}`}
            aria-label={t.siteViewList}
            title={t.siteViewList}
            onClick={() => setViewMode('list')}
          >
            <FontAwesomeIcon icon={faList} />
          </button>
        </div>
      </div>

      <div
        className="site-folder-list"
        onContextMenu={(event) => {
          event.preventDefault();
          event.stopPropagation();
          setContextMenu({ type: 'library', x: event.clientX, y: event.clientY });
        }}
      >
        {groupedSites.map((group) => {
          const collapsed = collapsedFolders[group.key] ?? false;
          return (
            <section key={group.key} className="site-folder-group">
              <button
                type="button"
                className={`site-folder-header ${dragOverFolderKey === group.key ? 'drag-over' : ''}`}
                draggable={group.canDelete}
                onClick={() => setCollapsedFolders((current) => ({ ...current, [group.key]: !collapsed }))}
                aria-expanded={!collapsed}
                onDragStart={(event) => {
                  if (!group.canDelete) {
                    event.preventDefault();
                    return;
                  }
                  setDraggedFolderKey(group.key);
                  setDraggedSiteId('');
                  setDragOverSiteId('');
                  setDragOverFolderKey('');
                  event.dataTransfer.effectAllowed = 'move';
                  event.dataTransfer.setData('text/plain', group.key);
                }}
                onDragOver={(event) => {
                  if (!draggedSiteId && !draggedFolderKey) {
                    return;
                  }
                  event.preventDefault();
                  event.dataTransfer.dropEffect = 'move';
                  if (dragOverFolderKey !== group.key) {
                    setDragOverFolderKey(group.key);
                  }
                }}
                onDragLeave={(event) => {
                  if (event.currentTarget.contains(event.relatedTarget as Node | null)) {
                    return;
                  }
                  if (dragOverFolderKey === group.key) {
                    setDragOverFolderKey('');
                  }
                }}
                onDrop={(event) => {
                  if (!draggedSiteId && !draggedFolderKey) {
                    return;
                  }
                  event.preventDefault();
                  event.stopPropagation();
                  if (draggedFolderKey) {
                    const sourceKey = event.dataTransfer.getData('text/plain') || draggedFolderKey;
                    moveFolder(sourceKey, group.key);
                  } else {
                    const sourceID = event.dataTransfer.getData('text/plain') || draggedSiteId;
                    onMoveSiteToFolder(sourceID, group.folderName);
                  }
                  setDraggedFolderKey('');
                  setDraggedSiteId('');
                  setDragOverSiteId('');
                  setDragOverFolderKey('');
                }}
                onDragEnd={() => {
                  setDraggedFolderKey('');
                  setDraggedSiteId('');
                  setDragOverSiteId('');
                  setDragOverFolderKey('');
                }}
                onContextMenu={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  setContextMenu({
                    type: 'folder',
                    x: event.clientX,
                    y: event.clientY,
                    folderName: group.folderName,
                    label: group.label,
                    siteCount: group.sites.length,
                    canDelete: group.canDelete,
                  });
                }}
              >
                <span className="site-folder-header-main">
                  <FontAwesomeIcon icon={collapsed ? faChevronRight : faChevronDown} />
                  <strong>{group.label}</strong>
                </span>
                <span>{t.siteListCount(group.sites.length)}</span>
              </button>

              {collapsed ? null : (
                <div className={`site-list site-list-${viewMode}`}>
                  {group.sites.map((site) => (
                    <article
                      key={site.id}
                      className={`site-item site-item-${viewMode} ${draggedSiteId === site.id ? 'dragging' : ''} ${dragOverSiteId === site.id ? 'drag-over' : ''}`}
                      draggable
                      onDragStart={(event) => {
                        setDraggedSiteId(site.id);
                        setDragOverSiteId('');
                        event.dataTransfer.effectAllowed = 'move';
                        event.dataTransfer.setData('text/plain', site.id);
                      }}
                      onDragOver={(event) => {
                        if (!draggedSiteId || draggedSiteId === site.id) {
                          return;
                        }
                        event.preventDefault();
                        event.dataTransfer.dropEffect = 'move';
                        if (dragOverSiteId !== site.id) {
                          setDragOverSiteId(site.id);
                        }
                      }}
                      onDragLeave={(event) => {
                        if (event.currentTarget.contains(event.relatedTarget as Node | null)) {
                          return;
                        }
                        if (dragOverSiteId === site.id) {
                          setDragOverSiteId('');
                        }
                      }}
                      onDrop={(event) => {
                        event.preventDefault();
                        const sourceID = event.dataTransfer.getData('text/plain') || draggedSiteId;
                        moveSite(sourceID, site.id);
                        setDraggedFolderKey('');
                        setDraggedSiteId('');
                        setDragOverSiteId('');
                        setDragOverFolderKey('');
                      }}
                      onDragEnd={() => {
                        setDraggedFolderKey('');
                        setDraggedSiteId('');
                        setDragOverSiteId('');
                        setDragOverFolderKey('');
                      }}
                      onContextMenu={(event) => {
                        event.preventDefault();
                        event.stopPropagation();
                        setContextMenu({ type: 'site', x: event.clientX, y: event.clientY, site });
                      }}
                    >
                      <div className="site-main" onClick={() => onOpenSite(site)}>
                        {viewMode !== 'list' ? <ProtocolLabel protocol={site.protocol} locale={locale} /> : null}
                        <strong>{site.name || site.host}</strong>
                        {viewMode !== 'list' ? <small>{site.username}@{site.host}:{site.port}</small> : null}
                      </div>
                      <div className={`site-actions ${viewMode !== 'card' ? 'site-actions-icon' : ''}`}>
                        <button
                          className={`primary ${viewMode !== 'card' ? 'site-icon-button' : ''}`}
                          onClick={() => onOpenSite(site)}
                          aria-label={t.connectSite}
                          title={t.connectSite}
                        >
                          {viewMode === 'card' ? t.connectSite : <FontAwesomeIcon icon={faPlug} />}
                        </button>
                        <button
                          className={viewMode !== 'card' ? 'site-icon-button' : ''}
                          onClick={() => onEditSite(site)}
                          aria-label={t.edit}
                          title={t.edit}
                        >
                          {viewMode === 'card' ? t.edit : <FontAwesomeIcon icon={faPenToSquare} />}
                        </button>
                      </div>
                    </article>
                  ))}
                </div>
              )}
            </section>
          );
        })}
      </div>

      {contextMenu
        ? createPortal(
            <div
              className="context-menu"
              style={contextMenuStyle}
              onClick={(event) => event.stopPropagation()}
            >
              {contextMenu.type === 'site' ? (
                <>
                  <button
                    className="context-menu-item"
                    onMouseDown={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      onCopySite(contextMenu.site);
                      setContextMenu(null);
                    }}
                  >
                    <FontAwesomeIcon icon={faClone} fixedWidth /> {t.copy}
                  </button>
                  <div className="context-menu-separator" />
                  <button
                    className="context-menu-item danger"
                    onMouseDown={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      onDeleteSite(contextMenu.site.id);
                      setContextMenu(null);
                    }}
                  >
                    <FontAwesomeIcon icon={faTrashCan} fixedWidth /> {t.delete}
                  </button>
                  <div className="context-menu-separator" />
                  <button
                    className="context-menu-item"
                    onMouseDown={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      onSortByName();
                      setContextMenu(null);
                    }}
                  >
                    <FontAwesomeIcon icon={faSortAlphaDown} fixedWidth /> {t.sortByName}
                  </button>
                </>
              ) : null}

              {contextMenu.type === 'library' ? (
                <>
                  <button
                    className="context-menu-item"
                    onMouseDown={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      onCreateFolder();
                      setContextMenu(null);
                    }}
                  >
                    <FontAwesomeIcon icon={faFolderPlus} fixedWidth /> {t.siteFolderCreate}
                  </button>
                  <button
                    className="context-menu-item"
                    onMouseDown={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      onSortFolders();
                      setContextMenu(null);
                    }}
                  >
                    <FontAwesomeIcon icon={faSortAlphaDown} fixedWidth /> {t.siteFolderSort}
                  </button>
                </>
              ) : null}

              {contextMenu.type === 'folder' ? (
                <>
                  <button
                    className="context-menu-item"
                    onMouseDown={(event) => {
                      event.preventDefault();
                      event.stopPropagation();
                      onSortFolders();
                      setContextMenu(null);
                    }}
                  >
                    <FontAwesomeIcon icon={faSortAlphaDown} fixedWidth /> {t.siteFolderSort}
                  </button>
                  {contextMenu.canDelete ? (
                    <>
                      <button
                        className="context-menu-item"
                        onMouseDown={(event) => {
                          event.preventDefault();
                          event.stopPropagation();
                          onRenameFolder(contextMenu.folderName);
                          setContextMenu(null);
                        }}
                      >
                        <FontAwesomeIcon icon={faPenToSquare} fixedWidth /> {t.renameItem}
                      </button>
                      <div className="context-menu-separator" />
                      <button
                        className="context-menu-item danger"
                        onMouseDown={(event) => {
                          event.preventDefault();
                          event.stopPropagation();
                          onDeleteFolder(contextMenu.folderName, contextMenu.siteCount);
                          setContextMenu(null);
                        }}
                      >
                        <FontAwesomeIcon icon={faTrashCan} fixedWidth /> {t.siteFolderDelete}
                      </button>
                    </>
                  ) : null}
                </>
              ) : null}
            </div>,
            document.body,
          )
        : null}
    </>
  );

  if (embedded) {
    return content;
  }

  return (
    <section className="card section-gap">
      {content}
    </section>
  );
}

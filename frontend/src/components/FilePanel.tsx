import { useEffect, useMemo, useRef, useState, type CSSProperties, type MouseEvent as ReactMouseEvent } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faCaretDown, faCaretUp, faFileLines, faFolder, faReply, faRotateRight } from '@fortawesome/free-solid-svg-icons';
import type { FileEntry, FileSortKey, FileSortState } from '../types';
import { type Locale, useI18n } from '../i18n';

export type FileContextMenuAction = 'mkdir' | 'delete' | 'refresh';
const LOCAL_DRAG_MIME = 'application/x-integrated-term-local-paths';
const REMOTE_DRAG_MIME = 'application/x-integrated-term-remote-paths';

export type FileContextMenuRequest = {
  x: number;
  y: number;
  entry?: FileEntry;
  side: 'local' | 'remote';
  selectedPaths?: string[];
  selectedEntries?: FileEntry[];
};

export type PathContextMenuRequest = {
  x: number;
  y: number;
  side: 'local' | 'remote';
  path: string;
};

type Props = {
  title: string;
  path: string;
  entries: FileEntry[];
  side: 'local' | 'remote';
  sortState: FileSortState;
  onSort: (key: FileSortKey) => void;
  onRefresh?: () => void;
  onCompare?: () => void;
  onDropFiles?: (paths: string[]) => void;
  onDropFilesToDirectory?: (paths: string[], targetDirectory: string) => void;
  onMoveEntriesToDirectory?: (paths: string[], targetDirectory: string) => void;
  onInvalidMoveToDirectory?: () => void;
  onOpenDirectory?: (entry: FileEntry) => void;
  onPickPath?: () => void;
  onSubmitPath?: (path: string) => void;
  onContextMenuRequest?: (request: FileContextMenuRequest) => void;
  onPathContextMenuRequest?: (request: PathContextMenuRequest) => void;
  locale: Locale;
};

export function FilePanel({
  title,
  path,
  entries,
  side,
  sortState,
  onSort,
  onRefresh,
  onCompare,
  onDropFiles,
  onDropFilesToDirectory,
  onMoveEntriesToDirectory,
  onInvalidMoveToDirectory,
  onOpenDirectory,
  onPickPath,
  onSubmitPath,
  onContextMenuRequest,
  onPathContextMenuRequest,
  locale,
}: Props) {
  const t = useI18n(locale);
  const tableRef = useRef<HTMLDivElement | null>(null);
  const [selectedPaths, setSelectedPaths] = useState<string[]>([]);
  const [anchorPath, setAnchorPath] = useState('');
  const [draggingPaths, setDraggingPaths] = useState<string[]>([]);
  const [dragOverDirectoryPath, setDragOverDirectoryPath] = useState('');
  const [pathDraft, setPathDraft] = useState(path);
  const dropTargetStyle: CSSProperties | undefined =
    side === 'remote' ? ({ '--wails-drop-target': 'drop' } as CSSProperties) : undefined;
  const entryPathSet = useMemo(() => new Set(entries.map((entry) => entry.path)), [entries]);

  useEffect(() => {
    setSelectedPaths((current) => current.filter((path) => entryPathSet.has(path)));
    setAnchorPath((current) => (entryPathSet.has(current) ? current : ''));
  }, [entryPathSet, path, side]);

  useEffect(() => {
    setPathDraft(path);
  }, [path]);

  const handleRowClick = (event: ReactMouseEvent<HTMLDivElement>, entry: FileEntry, index: number) => {
    if (event.shiftKey) {
      const anchorIndex = entries.findIndex((item) => item.path === anchorPath);
      const rangeStart = anchorIndex >= 0 ? Math.min(anchorIndex, index) : index;
      const rangeEnd = anchorIndex >= 0 ? Math.max(anchorIndex, index) : index;
      const range = entries.slice(rangeStart, rangeEnd + 1).map((item) => item.path);
      setSelectedPaths(range);
      setAnchorPath(entry.path);
      return;
    }

    if (event.metaKey || event.ctrlKey) {
      setSelectedPaths((current) =>
        current.includes(entry.path) ? current.filter((pathValue) => pathValue !== entry.path) : [...current, entry.path],
      );
      setAnchorPath(entry.path);
      return;
    }

    setSelectedPaths([entry.path]);
    setAnchorPath(entry.path);
  };

  const clearSelection = () => {
    setSelectedPaths([]);
    setAnchorPath('');
  };

  return (
    <section
      className={`card file-panel ${side === 'remote' ? 'drop-panel' : ''}`}
      style={dropTargetStyle}
      onDragOver={(event) => {
        const hasLocalDrag = !!event.dataTransfer.types.includes(LOCAL_DRAG_MIME);
        const hasRemoteDrag = !!event.dataTransfer.types.includes(REMOTE_DRAG_MIME);
        const allowExternalDrop = side === 'remote';
        if ((side === 'remote' && (hasLocalDrag || allowExternalDrop)) || (side === 'local' && hasRemoteDrag)) {
          event.preventDefault();
          event.dataTransfer.dropEffect = 'copy';
        }
      }}
      onDrop={async (event) => {
        event.preventDefault();
        const paths =
          side === 'remote'
            ? resolveInternalDraggedPaths(event.dataTransfer, LOCAL_DRAG_MIME) ?? (await resolveDroppedPaths(event.dataTransfer))
            : resolveInternalDraggedPaths(event.dataTransfer, REMOTE_DRAG_MIME) ?? [];
        if (paths.length > 0) {
          onDropFiles?.(paths);
        }
      }}
      onContextMenu={(event) => {
        event.preventDefault();
        event.stopPropagation();
        onContextMenuRequest?.({ x: event.clientX, y: event.clientY, side });
      }}
      onClick={(event) => {
        if ((event.target as HTMLElement).closest('.file-row, .file-table-head, .sort-button, .ghost')) {
          return;
        }
        clearSelection();
      }}
    >
      <div className="section-title">
        <div className="file-panel-title-row">
          <h3>{title}</h3>
          {side === 'local' ? (
            <button
              type="button"
              className="ghost file-panel-path-button"
              onClick={onPickPath}
              title={path}
              onContextMenu={(event) => {
                event.preventDefault();
                event.stopPropagation();
                onPathContextMenuRequest?.({ x: event.clientX, y: event.clientY, side, path });
              }}
            >
              {path}
            </button>
          ) : (
            <input
              className="file-panel-path-input"
              value={pathDraft}
              onChange={(event) => setPathDraft(event.target.value)}
              onBlur={() => setPathDraft(path)}
              onContextMenu={(event) => {
                event.preventDefault();
                event.stopPropagation();
                onPathContextMenuRequest?.({ x: event.clientX, y: event.clientY, side, path });
              }}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  onSubmitPath?.(pathDraft);
                }
                if (event.key === 'Escape') {
                  event.preventDefault();
                  setPathDraft(path);
                }
              }}
              spellCheck={false}
            />
          )}
        </div>
        <div className="file-panel-header-actions">
          {onCompare ? (
            <button
              type="button"
              className="ghost compact-button"
              onClick={onCompare}
              aria-label={t.compareDirectories}
              title={t.compareDirectories}
            >
              {t.compareDirectories}
            </button>
          ) : null}
          <button
            className="ghost compact-icon-button"
            onClick={() => {
              if (side === 'remote' && pathDraft !== path) {
                onSubmitPath?.(pathDraft);
                return;
              }
              onRefresh?.();
            }}
            aria-label={t.refresh}
            title={t.refresh}
          >
            <FontAwesomeIcon icon={faRotateRight} />
          </button>
        </div>
      </div>
      <div
        ref={tableRef}
        className="file-table"
        onClick={(event) => {
          if (event.target === event.currentTarget) {
            clearSelection();
          }
        }}
      >
        <div className="file-table-head">
          <button className="sort-button" onClick={() => onSort('name')}>
            <span>{t.fileName}</span>
            {renderSortMark(sortState, 'name')}
          </button>
          <button className="sort-button" onClick={() => onSort('modified')}>
            <span>{t.modified}</span>
            {renderSortMark(sortState, 'modified')}
          </button>
          <button className="sort-button" onClick={() => onSort('size')}>
            <span>{t.size}</span>
            {renderSortMark(sortState, 'size')}
          </button>
        </div>
        {entries.map((entry, index) => (
          <div
            key={entry.path}
            className={`file-row ${entry.isDir ? 'clickable-row' : ''} ${selectedPaths.includes(entry.path) ? 'selected' : ''} ${dragOverDirectoryPath === entry.path ? 'drag-target' : ''}`}
            aria-selected={selectedPaths.includes(entry.path)}
            draggable={entry.name !== '..'}
            onClick={(event) => handleRowClick(event, entry, index)}
            onDragStart={(event) => {
              if (entry.name === '..') {
                event.preventDefault();
                return;
              }

              const dragPaths = selectedPaths.includes(entry.path) ? selectedPaths : [entry.path];
              if (!selectedPaths.includes(entry.path)) {
                setSelectedPaths([entry.path]);
                setAnchorPath(entry.path);
              }
              setDraggingPaths(dragPaths);
              event.dataTransfer.effectAllowed = 'copyMove';
              event.dataTransfer.setData(side === 'local' ? LOCAL_DRAG_MIME : REMOTE_DRAG_MIME, JSON.stringify(dragPaths));
              event.dataTransfer.setData('text/plain', dragPaths.join('\n'));
            }}
            onDragEnd={() => {
              setDraggingPaths([]);
              setDragOverDirectoryPath('');
            }}
            onDragOver={(event) => {
              const moveMimeType = side === 'local' ? LOCAL_DRAG_MIME : REMOTE_DRAG_MIME;
              const crossMimeType = side === 'local' ? REMOTE_DRAG_MIME : LOCAL_DRAG_MIME;
              const internalMovePaths = resolveInternalDraggedPaths(event.dataTransfer, moveMimeType);
              const crossPaths = resolveInternalDraggedPaths(event.dataTransfer, crossMimeType);
              const validPaths = internalMovePaths?.filter((dragPath) => !isInvalidDirectoryMove(side, dragPath, entry.path)) ?? [];
              const hasCrossDrop = (crossPaths?.length ?? 0) > 0 || (side === 'remote' && event.dataTransfer.types.includes('Files'));
              if (!entry.isDir || entry.name === '..') {
                return;
              }
              if (validPaths.length === 0 && !hasCrossDrop) {
                return;
              }
              event.preventDefault();
              event.stopPropagation();
              event.dataTransfer.dropEffect = validPaths.length > 0 ? 'move' : 'copy';
              if (dragOverDirectoryPath !== entry.path) {
                setDragOverDirectoryPath(entry.path);
              }
            }}
            onDragLeave={(event) => {
              if (dragOverDirectoryPath !== entry.path) {
                return;
              }
              const nextTarget = event.relatedTarget;
              if (nextTarget instanceof Node && event.currentTarget.contains(nextTarget)) {
                return;
              }
              setDragOverDirectoryPath('');
            }}
            onDrop={async (event) => {
              if (!entry.isDir || entry.name === '..') {
                return;
              }
              event.preventDefault();
              event.stopPropagation();
              setDragOverDirectoryPath('');

              const moveMimeType = side === 'local' ? LOCAL_DRAG_MIME : REMOTE_DRAG_MIME;
              const crossMimeType = side === 'local' ? REMOTE_DRAG_MIME : LOCAL_DRAG_MIME;
              const internalMovePaths = resolveInternalDraggedPaths(event.dataTransfer, moveMimeType);
              const crossPaths =
                resolveInternalDraggedPaths(event.dataTransfer, crossMimeType) ??
                (side === 'remote' ? await resolveDroppedPaths(event.dataTransfer) : []);

              if (internalMovePaths?.length && onMoveEntriesToDirectory) {
                const filteredPaths = internalMovePaths.filter((dragPath) => !isInvalidDirectoryMove(side, dragPath, entry.path));
                if (filteredPaths.length === 0) {
                  onInvalidMoveToDirectory?.();
                  return;
                }
                onMoveEntriesToDirectory(filteredPaths, entry.path);
                return;
              }

              if (crossPaths?.length) {
                onDropFilesToDirectory?.(crossPaths, entry.path);
              }
            }}
            onContextMenu={(event) => {
              event.preventDefault();
              event.stopPropagation();
              if (!selectedPaths.includes(entry.path)) {
                setSelectedPaths([entry.path]);
                setAnchorPath(entry.path);
              }
              onContextMenuRequest?.({
                x: event.clientX,
                y: event.clientY,
                entry,
                side,
                selectedPaths: selectedPaths.includes(entry.path) ? selectedPaths : [entry.path],
                selectedEntries: (selectedPaths.includes(entry.path) ? selectedPaths : [entry.path])
                  .map((selectedPath) => entries.find((candidate) => candidate.path === selectedPath))
                  .filter((candidate): candidate is FileEntry => Boolean(candidate)),
              });
            }}
            onDoubleClick={() => {
              if (entry.isDir) {
                onOpenDirectory?.({ ...entry, side });
              }
            }}
          >
            <span className={entry.isDir ? 'entry-name dir-entry' : 'entry-name file-entry'}>
              <span className="entry-icon" aria-hidden="true">
                <FontAwesomeIcon icon={resolveEntryIcon(entry)} />
              </span>
              <span className="entry-label">{entry.name}</span>
              {draggingPaths.includes(entry.path) ? <span className="dragging-badge">{selectedPaths.length > 1 ? selectedPaths.length : ''}</span> : null}
            </span>
            <span>{entry.modified}</span>
            <span>{entry.isDir ? '-' : formatSize(entry.size)}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function renderSortMark(sortState: FileSortState, key: FileSortKey) {
  if (sortState.key !== key) return null;
  return (
    <span className="sort-icon" aria-hidden="true">
      <FontAwesomeIcon icon={sortState.direction === 'asc' ? faCaretUp : faCaretDown} />
    </span>
  );
}

function resolveEntryIcon(entry: FileEntry) {
  if (entry.name === '..') return faReply;
  if (entry.isDir) return faFolder;
  return faFileLines;
}

function formatSize(size: number) {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  if (size < 1024 * 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MB`;
  return `${(size / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

async function resolveDroppedPaths(dataTransfer: DataTransfer) {
  const filePaths = Array.from(dataTransfer.files ?? [])
    .map((file) => Reflect.get(file, 'path'))
    .filter((value): value is string => typeof value === 'string' && value.length > 0);

  if (filePaths.length > 0) {
    return Array.from(new Set(filePaths));
  }

  const itemPaths = Array.from(dataTransfer.items ?? [])
    .map((item) => {
      const file = item.getAsFile?.();
      if (!file) return '';
      return Reflect.get(file, 'path');
    })
    .filter((value): value is string => typeof value === 'string' && value.length > 0);

  return Array.from(new Set(itemPaths));
}

function resolveInternalDraggedPaths(dataTransfer: DataTransfer, mimeType: string) {
  const serialized = dataTransfer.getData(mimeType);
  if (!serialized) return null;

  try {
    const parsed = JSON.parse(serialized);
    if (!Array.isArray(parsed)) return null;
    const paths = parsed.filter((value): value is string => typeof value === 'string' && value.length > 0);
    return Array.from(new Set(paths));
  } catch {
    return null;
  }
}

function isInvalidDirectoryMove(side: 'local' | 'remote', sourcePath: string, targetDirectory: string) {
  const normalizedSource = normalizeDirectoryPath(side, sourcePath);
  const normalizedTarget = normalizeDirectoryPath(side, targetDirectory);
  return normalizedTarget === normalizedSource || normalizedTarget.startsWith(`${normalizedSource}/`);
}

function normalizeDirectoryPath(side: 'local' | 'remote', value: string) {
  const slash = '/';
  if (side === 'remote') {
    if (!value || value === slash) return slash;
    return value.endsWith(slash) ? value.slice(0, -1) : value;
  }

  const normalized = value.replace(/\\/g, slash);
  if (!normalized || normalized === slash) return slash;
  return normalized.endsWith(slash) ? normalized.slice(0, -1) : normalized;
}

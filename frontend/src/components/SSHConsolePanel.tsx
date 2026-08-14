import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { createPortal } from 'react-dom';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faBrush, faEraser, faFont, faXmark } from '@fortawesome/free-solid-svg-icons';
import { ClipboardSetText, EventsOn } from '../../wailsjs/runtime/runtime';
import { Terminal } from 'xterm';
import { FitAddon } from '@xterm/addon-fit';
import { type Locale, useI18n } from '../i18n';
import {
  FALLBACK_FONT_FAMILIES,
  FONT_SIZES,
  IS_MAC,
  THEMES,
  copySelection,
  fitTerminal,
  getCurrentPromptPath,
  pasteClipboard,
  sanitizeTerminalReplay,
  toTerminalFontFamily,
  withSelectionTheme,
  writeLocalEcho,
  type FontFamilyId,
  type FontScale,
} from './terminalUtils';
export type { FontFamilyId, FontScale } from './terminalUtils';

const LOCAL_DRAG_MIME = 'application/x-integrated-term-local-paths';

type Props = {
  sessionId: string;
  active: boolean;
  onClose: () => void;
  onOpenSFTP: () => void;
  onDropLocalPaths?: (paths: string[], remotePath: string) => void;
  onExternalDropHoverChange?: (active: boolean) => void;
  canOpenSFTP?: boolean;
  enableLocalEcho?: boolean;
  locale: Locale;
  themeId: string;
  fontScale: FontScale;
  fontFamilyId: FontFamilyId;
  onThemeChange: (themeId: string) => void;
  onFontScaleChange: (fontScale: FontScale) => void;
  onFontFamilyChange: (fontFamilyId: FontFamilyId) => void;
};

export function SSHConsolePanel({
  sessionId,
  active,
  onClose,
  onOpenSFTP,
  onDropLocalPaths,
  onExternalDropHoverChange,
  canOpenSFTP = true,
  enableLocalEcho = false,
  locale,
  themeId,
  fontScale,
  fontFamilyId,
  onThemeChange,
  onFontScaleChange,
  onFontFamilyChange,
}: Props) {
  const t = useI18n(locale);
  const terminalRef = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const lastMeasuredWidthRef = useRef<number>(0);
  const activeRef = useRef(active);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number } | null>(null);
  const [systemFonts, setSystemFonts] = useState<string[]>(FALLBACK_FONT_FAMILIES);
  const appShellFontSize = typeof window !== 'undefined'
    ? window.getComputedStyle(document.querySelector('.app-shell') ?? document.body).fontSize
    : undefined;
  const contextMenuStyle = contextMenu
    ? {
        left: Math.min(contextMenu.x, window.innerWidth - 188),
        top: Math.min(contextMenu.y, window.innerHeight - 220),
        fontSize: appShellFontSize,
      }
    : undefined;

  activeRef.current = active;

  const syncActiveTerminalLayout = () => {
    if (!activeRef.current) return;
    const term = termRef.current;
    const fitAddon = fitAddonRef.current;
    const container = terminalRef.current;
    if (!term || !fitAddon || !container || container.clientWidth <= 0) return;
    lastMeasuredWidthRef.current = container.clientWidth;
    fitTerminal(container, fitAddon);
    void window.go?.app?.App?.ResizeSSHSession?.(sessionId, term.cols, term.rows);
  };

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
    void window.go?.app?.App?.ListSystemFonts?.().then((fonts) => {
      if (!fonts?.length) return;
      setSystemFonts(fonts);
    });
  }, []);

  useEffect(() => {
    if (!terminalRef.current) return;

    const activeTheme = THEMES.find((item) => item.id === themeId)?.theme ?? THEMES[0].theme;

    const term = new Terminal({
      allowTransparency: true,
      cursorBlink: true,
      cursorStyle: 'block',
      fontFamily: toTerminalFontFamily(fontFamilyId),
      fontSize: FONT_SIZES[fontScale],
      fontWeight: '500',
      lineHeight: 1.25,
      scrollback: 5000,
      theme: withSelectionTheme(activeTheme),
    });
    term.attachCustomKeyEventHandler((event) => {
      if (event.type !== 'keydown') {
        return true;
      }

      const key = event.key.toLowerCase();
      const isClipboardCopy = key === 'c'
        && term.hasSelection()
        && (event.metaKey || (!IS_MAC && event.ctrlKey));
      if (isClipboardCopy) {
        event.preventDefault();
        void copySelection(term);
        return false;
      }

      const isClipboardPaste = key === 'v'
        && (event.metaKey || (!IS_MAC && event.ctrlKey));
      if (isClipboardPaste) {
        event.preventDefault();
        void pasteClipboard(sessionId);
        return false;
      }

      return true;
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(terminalRef.current);
    lastMeasuredWidthRef.current = terminalRef.current.clientWidth;
    if (activeRef.current) {
      fitTerminal(terminalRef.current, fitAddon);
      term.focus();
    }
    termRef.current = term;
    fitAddonRef.current = fitAddon;

    let disposeOutput = () => {};
    let disposeClosed = () => {};
    let disposeError = () => {};
    let disposed = false;

    const dataDisposable = term.onData((data) => {
      if (enableLocalEcho) {
        writeLocalEcho(term, data);
      }
      void window.go?.app?.App?.WriteSSHInput?.(sessionId, data);
    });

    void (async () => {
      const backlog = await window.go?.app?.App?.GetSSHOutputBuffer?.(sessionId);
      if (disposed) return;
      if (backlog) {
        term.write(sanitizeTerminalReplay(backlog));
      }
      disposeOutput = EventsOn(`ssh:output:${sessionId}`, (data: string) => {
        term.write(data);
      });
      disposeClosed = EventsOn(`ssh:closed:${sessionId}`, () => {
        term.write(`\r\n${t.sshConnectionClosed}\r\n`);
      });
      disposeError = EventsOn(`ssh:error:${sessionId}`, (message: string) => {
        term.write(`\r\n${t.errorPrefix} ${message}\r\n`);
      });
      if (activeRef.current) {
        await window.go?.app?.App?.ResizeSSHSession?.(sessionId, term.cols, term.rows);
      }
      if (!disposed && activeRef.current) term.focus();
    })();

    const observer = new ResizeObserver(() => {
      const container = terminalRef.current;
      if (!container) {
        return;
      }
      const nextWidth = container.clientWidth;
      if (!activeRef.current || nextWidth <= 0) {
        return;
      }
      if (Math.abs(nextWidth - lastMeasuredWidthRef.current) < 1) {
        return;
      }
      lastMeasuredWidthRef.current = nextWidth;
      fitTerminal(container, fitAddon);
      void window.go?.app?.App?.ResizeSSHSession?.(sessionId, term.cols, term.rows);
    });
    observer.observe(terminalRef.current);

    const terminalElement = terminalRef.current;
    const handleCopy = (event: ClipboardEvent) => {
      const selection = term.getSelection();
      if (!selection) {
        return;
      }
      event.preventDefault();
      event.clipboardData?.setData('text/plain', selection);
      void ClipboardSetText(selection);
    };
    const handlePaste = (event: ClipboardEvent) => {
      const text = event.clipboardData?.getData('text');
      if (!text) {
        return;
      }
      event.preventDefault();
      void window.go?.app?.App?.WriteSSHInput?.(sessionId, text);
    };
    terminalElement.addEventListener('copy', handleCopy);
    terminalElement.addEventListener('paste', handlePaste);

    return () => {
      disposed = true;
      observer.disconnect();
      terminalElement.removeEventListener('copy', handleCopy);
      terminalElement.removeEventListener('paste', handlePaste);
      dataDisposable.dispose();
      disposeOutput();
      disposeClosed();
      disposeError();
      term.dispose();
      termRef.current = null;
      fitAddonRef.current = null;
    };
  }, [enableLocalEcho, sessionId, t.errorPrefix, t.sshConnectionClosed]);

  useEffect(() => {
    if (!active) return;
    const term = termRef.current;
    if (!term) return;
    syncActiveTerminalLayout();
    term.focus();
  }, [active, sessionId]);

  useEffect(() => {
    const term = termRef.current;
    if (!term) return;
    const activeTheme = THEMES.find((item) => item.id === themeId)?.theme ?? THEMES[0].theme;
    term.options.theme = withSelectionTheme(activeTheme);
    term.refresh(0, term.rows - 1);
  }, [themeId]);

  useEffect(() => {
    const term = termRef.current;
    if (!term) return;
    term.options.fontSize = FONT_SIZES[fontScale];
    syncActiveTerminalLayout();
  }, [active, fontScale, sessionId]);

  useEffect(() => {
    const term = termRef.current;
    if (!term) return;
    term.options.fontFamily = toTerminalFontFamily(fontFamilyId);
    syncActiveTerminalLayout();
  }, [active, fontFamilyId, sessionId]);

  return (
    <section className="card file-panel ssh-console-panel">
      <div className="section-title ssh-console-toolbar">
        <div className="ssh-console-theme-group">
          <span className="ssh-console-group-icon" aria-hidden="true">
            <FontAwesomeIcon icon={faBrush} />
          </span>
          {THEMES.map((item) => (
            <button
              key={item.id}
              className={`site-view-button ssh-theme-button ${themeId === item.id ? 'active' : ''}`}
              onClick={() => onThemeChange(item.id)}
              aria-label={`${t.terminalThemeGroup} ${item.label}`}
              title={`${t.terminalThemeGroup} ${item.label}`}
            >
              {item.label}
            </button>
          ))}
        </div>
        <div className="ssh-console-toolbar-spacer" />
        <div className="ssh-console-font-group">
          <span className="ssh-console-group-icon" aria-hidden="true">
            <FontAwesomeIcon icon={faFont} />
          </span>
          <div className="select-shell ssh-font-family-select">
            <select value={fontFamilyId} aria-label={t.terminalFontGroup} onChange={(event) => onFontFamilyChange(event.target.value as FontFamilyId)}>
              {systemFonts.map((font) => (
                <option key={font} value={font}>{font}</option>
              ))}
            </select>
            <span className="select-arrow">▾</span>
          </div>
          <button
            className={`site-view-button ssh-font-button ${fontScale === 'small' ? 'active' : ''}`}
            onClick={() => onFontScaleChange('small')}
          >
            {t.terminalFontSmall}
          </button>
          <button
            className={`site-view-button ssh-font-button ${fontScale === 'medium' ? 'active' : ''}`}
            onClick={() => onFontScaleChange('medium')}
          >
            {t.terminalFontMedium}
          </button>
          <button
            className={`site-view-button ssh-font-button ${fontScale === 'large' ? 'active' : ''}`}
            onClick={() => onFontScaleChange('large')}
          >
            {t.terminalFontLarge}
          </button>
        </div>
        <button className="site-view-button ssh-font-button ssh-clear-button" onClick={() => termRef.current?.clear()} aria-label={t.terminalClear} title={t.terminalClear}>
          <FontAwesomeIcon icon={faEraser} />
        </button>
        <button
          className="site-view-button ssh-font-button ssh-scale-reset-button"
          onClick={async () => {
            await window.go?.app?.App?.ResetWindowToDefaultScale?.();
          }}
          aria-label={t.terminalScaleReset}
          title={t.terminalScaleReset}
        >
          {t.terminalScaleReset}
        </button>
        <button className="site-view-button ssh-font-button ssh-close-button" onClick={onClose} aria-label={t.closeSSH} title={t.closeSSH}>
          <FontAwesomeIcon icon={faXmark} />
        </button>
      </div>
      <div className="ssh-console-body">
        <div
          ref={terminalRef}
          className="ssh-console-surface"
          style={{
            background: THEMES.find((item) => item.id === themeId)?.theme.background ?? '#111827',
            '--wails-drop-target': 'drop',
          } as CSSProperties}
          onDragEnter={(event) => {
            if (event.dataTransfer.types.includes('Files')) {
              onExternalDropHoverChange?.(true);
            }
          }}
          onDragOver={(event) => {
            const hasLocalDrag = event.dataTransfer.types.includes(LOCAL_DRAG_MIME);
            const hasExternalFiles = event.dataTransfer.types.includes('Files');
            if (!hasLocalDrag && !hasExternalFiles) {
              return;
            }
            event.preventDefault();
            event.dataTransfer.dropEffect = 'copy';
            if (hasExternalFiles) {
              onExternalDropHoverChange?.(true);
            }
          }}
          onDragLeave={(event) => {
            if (event.currentTarget.contains(event.relatedTarget as Node | null)) {
              return;
            }
            onExternalDropHoverChange?.(false);
          }}
          onDrop={(event) => {
            event.preventDefault();
            onExternalDropHoverChange?.(false);
            const payload = event.dataTransfer.getData(LOCAL_DRAG_MIME);
            if (!payload) {
              return;
            }
            try {
              const paths = JSON.parse(payload) as string[];
              if (Array.isArray(paths) && paths.length > 0) {
                onDropLocalPaths?.(paths, getCurrentPromptPath(termRef.current));
              }
            } catch {
              return;
            }
          }}
          onMouseDown={() => {
            termRef.current?.focus();
          }}
          onContextMenu={(event) => {
            event.preventDefault();
            setContextMenu({ x: event.clientX, y: event.clientY });
            termRef.current?.focus();
          }}
        />
      </div>
      {contextMenu
        ? createPortal(
            <div className="context-menu ssh-terminal-context-menu" style={contextMenuStyle} onClick={(event) => event.stopPropagation()}>
              <button className="context-menu-item" onMouseDown={(event) => {
                event.preventDefault();
                event.stopPropagation();
                if (termRef.current) {
                  void copySelection(termRef.current);
                }
                setContextMenu(null);
              }}>
                {t.terminalCopy}
              </button>
              <button className="context-menu-item" onMouseDown={(event) => {
                event.preventDefault();
                event.stopPropagation();
                void pasteClipboard(sessionId);
                setContextMenu(null);
              }}>
                {t.terminalPaste}
              </button>
              <button className="context-menu-item" onMouseDown={(event) => {
                event.preventDefault();
                event.stopPropagation();
                termRef.current?.selectAll();
                setContextMenu(null);
              }}>
                {t.terminalSelectAll}
              </button>
              <button className="context-menu-item" onMouseDown={(event) => {
                event.preventDefault();
                event.stopPropagation();
                termRef.current?.clear();
                setContextMenu(null);
              }}>
                {t.terminalClear}
              </button>
              {canOpenSFTP ? <div className="ssh-terminal-context-separator" aria-hidden="true" /> : null}
              {canOpenSFTP ? (
                <button className="context-menu-item" onMouseDown={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  onOpenSFTP();
                  setContextMenu(null);
                }}>
                  {t.terminalOpenSFTP}
                </button>
              ) : null}
            </div>,
            document.body,
          )
        : null}
    </section>
  );
}

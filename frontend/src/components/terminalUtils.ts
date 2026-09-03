import { ClipboardGetText, ClipboardSetText } from '../../wailsjs/runtime/runtime';
import type { FitAddon } from '@xterm/addon-fit';
import type { ITheme } from 'xterm';
import type { Terminal } from 'xterm';

export type FontScale = 'small' | 'medium' | 'large';
export type FontFamilyId = string;

export const THEMES: Array<{ id: string; label: string; color: string; theme: ITheme }> = [
  {
    id: 'ubuntu',
    label: '1',
    color: '#380c2a',
    theme: {
      background: '#300a24',
      foreground: '#eeeeec',
      cursor: '#ffffff',
      black: '#2e3436',
      red: '#cc0000',
      green: '#4e9a06',
      yellow: '#c4a000',
      blue: '#3465a4',
      magenta: '#75507b',
      cyan: '#06989a',
      white: '#d3d7cf',
      brightBlack: '#555753',
      brightRed: '#ef2929',
      brightGreen: '#8ae234',
      brightYellow: '#fce94f',
      brightBlue: '#729fcf',
      brightMagenta: '#ad7fa8',
      brightCyan: '#34e2e2',
      brightWhite: '#eeeeec',
    },
  },
  {
    id: 'powershell',
    label: '2',
    color: '#012456',
    theme: {
      background: '#012456',
      foreground: '#eeedf0',
      cursor: '#ffffff',
      black: '#000000',
      red: '#7e0008',
      green: '#098003',
      yellow: '#c4a000',
      blue: '#010083',
      magenta: '#d33682',
      cyan: '#0e807f',
      white: '#7f7c7f',
      brightBlack: '#808080',
      brightRed: '#ef2929',
      brightGreen: '#1cfe3c',
      brightYellow: '#fdfc79',
      brightBlue: '#268bd2',
      brightMagenta: '#fe13fa',
      brightCyan: '#29fffe',
      brightWhite: '#c2c1c3',
    },
  },
  {
    id: 'commodore',
    label: '3',
    color: '#40318d',
    theme: {
      background: '#40318d',
      foreground: '#aab4ff',
      cursor: '#aab4ff',
      black: '#090300',
      red: '#883932',
      green: '#55a049',
      yellow: '#bfce72',
      blue: '#40318d',
      magenta: '#8b3f96',
      cyan: '#67b6bd',
      white: '#ffffff',
      brightBlack: '#4f4f4f',
      brightRed: '#cc7b75',
      brightGreen: '#a9ff9f',
      brightYellow: '#ffffb6',
      brightBlue: '#706deb',
      brightMagenta: '#d48bff',
      brightCyan: '#8effff',
      brightWhite: '#ffffff',
    },
  },
  {
    id: 'github',
    label: '4',
    color: '#f4f4f4',
    theme: {
      background: '#f4f4f4',
      foreground: '#3e3e3e',
      cursor: '#3e3e3e',
      black: '#3e3e3e',
      red: '#970b16',
      green: '#07962a',
      yellow: '#f8eec7',
      blue: '#003e8a',
      magenta: '#e94691',
      cyan: '#89d1ec',
      white: '#ffffff',
      brightBlack: '#666666',
      brightRed: '#de0000',
      brightGreen: '#87d5a2',
      brightYellow: '#f1d007',
      brightBlue: '#2e6cba',
      brightMagenta: '#ffa29f',
      brightCyan: '#1cfafe',
      brightWhite: '#ffffff',
    },
  },
  {
    id: 'minecraft',
    label: '5',
    color: '#2f5d2f',
    theme: {
      background: '#1e1e1e',
      foreground: '#d0c5a9',
      cursor: '#8fbc5a',
      black: '#000000',
      red: '#b02e26',
      green: '#5e7c16',
      yellow: '#c2b51c',
      blue: '#3c44aa',
      magenta: '#8932b8',
      cyan: '#169c9c',
      white: '#bfbfbf',
      brightBlack: '#555555',
      brightRed: '#ff5555',
      brightGreen: '#55ff55',
      brightYellow: '#ffff55',
      brightBlue: '#5555ff',
      brightMagenta: '#ff55ff',
      brightCyan: '#55ffff',
      brightWhite: '#ffffff',
    },
  },
];

export const FONT_SIZES: Record<FontScale, number> = {
  small: 12,
  medium: 14,
  large: 16,
};

export const FALLBACK_FONT_FAMILIES = ['SF Mono', 'Menlo', 'Monaco', 'Cascadia Mono', 'Consolas'];
export const IS_MAC = typeof navigator !== 'undefined' && /mac/i.test(navigator.platform);

function syncTerminalLayout(container: HTMLDivElement | null) {
  if (!container) {
    return;
  }

  const xtermElement = container.querySelector<HTMLElement>('.xterm');
  if (!xtermElement) {
    return;
  }

  xtermElement.style.transform = '';
  xtermElement.style.transformOrigin = '';
  xtermElement.style.width = '';
  xtermElement.style.height = '';

  const availableHeight = Math.max(container.clientHeight - 20, 0);
  const widthBasedHeight = Math.max(Math.round(container.clientWidth * 0.78), 0);
  const targetHeight = Math.min(availableHeight, widthBasedHeight || availableHeight);
  const viewportElement = container.querySelector<HTMLElement>('.xterm-viewport');
  const screenElement = container.querySelector<HTMLElement>('.xterm-screen');
  const helperElement = container.querySelector<HTMLElement>('.xterm-helpers');

  if (xtermElement) {
    xtermElement.style.height = `${targetHeight}px`;
    xtermElement.style.maxHeight = `${targetHeight}px`;
  }
  if (viewportElement) {
    viewportElement.style.height = `${targetHeight}px`;
    viewportElement.style.maxHeight = `${targetHeight}px`;
  }
  if (screenElement) {
    screenElement.style.height = `${targetHeight}px`;
    screenElement.style.maxHeight = `${targetHeight}px`;
  }
  if (helperElement) {
    helperElement.style.transform = '';
    helperElement.style.transformOrigin = '';
    helperElement.style.height = `${targetHeight}px`;
    helperElement.style.maxHeight = `${targetHeight}px`;
  }
  if (screenElement) {
    screenElement.style.transform = '';
    screenElement.style.transformOrigin = '';
  }
}

export function fitTerminal(container: HTMLDivElement | null, fitAddon: FitAddon | null) {
  if (!container || !fitAddon) {
    return;
  }
  syncTerminalLayout(container);
  fitAddon.fit();
  syncTerminalLayout(container);
}

export function withSelectionTheme(theme: ITheme): ITheme {
  return {
    ...theme,
    selectionBackground: `${theme.cursor ?? '#ffffff'}44`,
    selectionInactiveBackground: `${theme.cursor ?? '#ffffff'}2a`,
  };
}

export async function copySelection(term: Terminal) {
  const selection = term.getSelection();
  if (!selection) return;
  await ClipboardSetText(selection);
}

export async function pasteClipboard(term: Terminal) {
  const text = await ClipboardGetText();
  if (!text) return;
  term.paste(text);
}

export function toTerminalFontFamily(fontFamily: string) {
  const family = fontFamily.trim() || 'SF Mono';
  return `"${family}", monospace`;
}

export function sanitizeTerminalReplay(value: string) {
  return value
    .replace(/\x1b\][\s\S]*?(?:\x07|\x1b\\)/g, '')
    .replace(/\x1b\][^\x07]*(?:$)/g, '');
}

export function writeLocalEcho(term: Terminal, data: string) {
  for (const char of data) {
    if (char === '\r') {
      term.write('\r\n');
      continue;
    }
    if (char === '\u007f') {
      term.write('\b \b');
      continue;
    }
    term.write(char);
  }
}

export function getCurrentPromptPath(term: Terminal | null) {
  if (!term) {
    return '';
  }

  const activeBuffer = term.buffer.active;
  const end = activeBuffer.baseY + activeBuffer.cursorY;
  const start = Math.max(0, end - 10);
  const lines: string[] = [];

  for (let index = start; index <= end; index += 1) {
    const line = activeBuffer.getLine(index);
    if (!line) {
      continue;
    }
    lines.push(line.translateToString(true));
  }

  return extractPromptPath(lines.join('\n'));
}

function extractPromptPath(value: string) {
  const lines = value.split(/\r?\n/).slice(-6);
  for (let index = lines.length - 1; index >= 0; index -= 1) {
    const line = lines[index].trim();
    const match = line.match(/^[^@\s]+@[^:\s]+:(~(?:\/[^\s#$]*)?|\/[^\s#$]*)[#$]\s*$/);
    if (match?.[1]) {
      return match[1];
    }
  }
  return '';
}

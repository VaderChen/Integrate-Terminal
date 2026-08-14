import { useEffect, useRef, type Dispatch, type SetStateAction } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import type { Tab } from '../types';

type Params = {
  tabs: Tab[];
  setTabs: Dispatch<SetStateAction<Tab[]>>;
  closeTerminalTabOnDisconnect: boolean;
  onSessionClosed: (sessionId: string) => Promise<void>;
};

export function useTerminalEvents({ tabs, setTabs, closeTerminalTabOnDisconnect, onSessionClosed }: Params) {
  const closeOnDisconnectRef = useRef(closeTerminalTabOnDisconnect);
  const onSessionClosedRef = useRef(onSessionClosed);
  const promptBufferRef = useRef<Record<string, string>>({});

  useEffect(() => {
    closeOnDisconnectRef.current = closeTerminalTabOnDisconnect;
    onSessionClosedRef.current = onSessionClosed;
  }, [closeTerminalTabOnDisconnect, onSessionClosed]);

  useEffect(() => {
    const terminalTabs = tabs.filter((tab) => tab.mode === 'terminal' && tab.sessionId);
    const disposers = terminalTabs.flatMap((tab) => [
      EventsOn('ssh:closed:' + tab.sessionId, () => {
        if (closeOnDisconnectRef.current) {
          void onSessionClosedRef.current(tab.sessionId);
        }
      }),
      EventsOn('ssh:cwd:' + tab.sessionId, (remotePath: string) => {
        updateRemotePath(setTabs, tab.id, remotePath);
      }),
      EventsOn('ssh:output:' + tab.sessionId, (chunk: string) => {
        const cleaned = stripAnsiSequences((promptBufferRef.current[tab.sessionId] ?? '') + chunk).slice(-2048);
        promptBufferRef.current[tab.sessionId] = cleaned;
        const promptPath = extractPromptPath(cleaned);
        if (promptPath) {
          updateRemotePath(setTabs, tab.id, promptPath);
        }
      }),
    ]);

    return () => disposers.forEach((dispose) => dispose());
  }, [setTabs, tabs]);
}

function updateRemotePath(setTabs: Dispatch<SetStateAction<Tab[]>>, tabId: string, remotePath: string) {
  setTabs((current) => current.map((tab) => (
    tab.id === tabId && tab.remotePath !== remotePath ? { ...tab, remotePath } : tab
  )));
}

function stripAnsiSequences(value: string) {
  return value.replace(/\x1b\[[0-9;?]*[ -/]*[@-~]/g, '').replace(/\x1b\][^\u0007]*(?:\u0007|\x1b\\)/g, '');
}

function extractPromptPath(value: string) {
  const lines = value.split(/\r?\n/).slice(-6);
  for (let index = lines.length - 1; index >= 0; index -= 1) {
    const line = lines[index].trim();
    const match = line.match(/^[^@\s]+@[^:\s]+:(~(?:\/[^\s#$]*)?|\/[^\s#$]*)[#$]\s*$/);
    if (match?.[1]) return match[1];
  }
  return '';
}

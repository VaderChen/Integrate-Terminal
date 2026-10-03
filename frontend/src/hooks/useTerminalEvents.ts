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
  const setTabsRef = useRef(setTabs);
  const subscriptionsRef = useRef(new Map<string, { sessionId: string; dispose: () => void }>());

  useEffect(() => {
    closeOnDisconnectRef.current = closeTerminalTabOnDisconnect;
    onSessionClosedRef.current = onSessionClosed;
    setTabsRef.current = setTabs;
  }, [closeTerminalTabOnDisconnect, onSessionClosed, setTabs]);

  useEffect(() => {
    const terminalTabs = new Map(tabs.filter((tab) => tab.mode === 'terminal' && tab.sessionId).map((tab) => [tab.id, tab]));
    const subscriptions = subscriptionsRef.current;
    for (const [tabId, subscription] of subscriptions) {
      if (terminalTabs.get(tabId)?.sessionId !== subscription.sessionId) {
        subscription.dispose();
        subscriptions.delete(tabId);
      }
    }
    for (const tab of terminalTabs.values()) {
      if (subscriptions.has(tab.id)) continue;
      // 提示字串與訂閱共用生命週期，關閉分頁即釋放，路徑更新不重訂閱。
      let promptBuffer = '';
      const disposers = [
        EventsOn('ssh:closed:' + tab.sessionId, () => {
          if (closeOnDisconnectRef.current) {
            void onSessionClosedRef.current(tab.sessionId);
          }
        }),
        EventsOn('ssh:cwd:' + tab.sessionId, (remotePath: string) => {
          updateRemotePath(setTabsRef.current, tab.id, remotePath);
        }),
        EventsOn('ssh:output:' + tab.sessionId, (chunk: string) => {
          promptBuffer = stripAnsiSequences(promptBuffer + chunk).slice(-2048);
          const promptPath = extractPromptPath(promptBuffer);
          if (promptPath) {
            updateRemotePath(setTabsRef.current, tab.id, promptPath);
          }
        }),
      ];
      subscriptions.set(tab.id, { sessionId: tab.sessionId, dispose: () => disposers.forEach(dispose => dispose()) });
    }
  }, [tabs]);

  useEffect(() => () => {
    for (const subscription of subscriptionsRef.current.values()) subscription.dispose();
    subscriptionsRef.current.clear();
  }, []);
}

function updateRemotePath(setTabs: Dispatch<SetStateAction<Tab[]>>, tabId: string, remotePath: string) {
  setTabs((current) => {
    const index = current.findIndex(tab => tab.id === tabId);
    if (index < 0 || current[index].remotePath === remotePath) return current;
    const next = [...current];
    next[index] = { ...current[index], remotePath };
    return next;
  });
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

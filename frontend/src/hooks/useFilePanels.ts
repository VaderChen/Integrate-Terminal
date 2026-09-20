import { useEffect, useRef, useState } from 'react';
import type React from 'react';
import { createPanelLoader, samePanelLocation, type PanelSnapshot } from '../filePanelState';
import { extractErrorMessage } from '../appUtils';
import type { Tab } from '../types';

export function useFilePanels(activeTab: Tab | null, activeTabRef: React.MutableRefObject<Tab | null>, setErrorMessage: React.Dispatch<React.SetStateAction<string>>) {
  const [snapshot, setSnapshot] = useState<PanelSnapshot | null>(null);
  const loader = useRef<ReturnType<typeof createPanelLoader> | null>(null);
  if (!loader.current) {
    loader.current = createPanelLoader({
      getActiveTab: () => activeTabRef.current,
      readLocal: async tab => await window.go?.app?.App?.ListLocal?.(tab.id, tab.localPath) ?? [],
      readRemote: async tab => await window.go?.app?.App?.ListRemote?.(tab.id, tab.remotePath) ?? [],
      onSnapshot: setSnapshot,
      onError: error => setErrorMessage(extractErrorMessage(error)),
    });
  }
  useEffect(() => {
    if (!activeTab) loader.current!.invalidate();
    else void loader.current!.load(activeTab);
    return () => loader.current!.invalidate();
  }, [activeTab?.id, activeTab?.localPath, activeTab?.remotePath, activeTab?.mode, activeTab?.connected]);

  const ready = samePanelLocation(snapshot, activeTab);
  return {
    localFiles: ready ? snapshot!.localFiles : [],
    remoteFiles: ready ? snapshot!.remoteFiles : [],
    localPanelReady: ready && snapshot!.localReady,
    remotePanelReady: ready && snapshot!.remoteReady,
    invalidatePanels: () => loader.current!.invalidate(),
    refreshPanels: (tab: Tab | null) => loader.current!.load(tab),
    refreshPanelsForPaths: (tab: Tab, localPath: string, remotePath: string) => loader.current!.load({ ...tab, localPath, remotePath }),
  };
}

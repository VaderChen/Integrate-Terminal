import { useState } from 'react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { faTerminal, faXmark } from '@fortawesome/free-solid-svg-icons';
import type { Tab } from '../types';
import { type Locale, useI18n } from '../i18n';

type Props = {
  tabs: Tab[];
  activeTabId: string;
  onSelectTab: (tabId: string) => void;
  onCloseTab: (tabId: string) => void;
  onReorderTabs: (tabIDs: string[]) => void;
  onOpenLocalTerminal: () => void;
  locale: Locale;
};

export function TabBar({ tabs, activeTabId, onSelectTab, onCloseTab, onReorderTabs, onOpenLocalTerminal, locale }: Props) {
  const t = useI18n(locale);
  const [draggedTabId, setDraggedTabId] = useState('');

  const moveTab = (sourceID: string, targetID: string) => {
    if (!sourceID || !targetID || sourceID === targetID) {
      return;
    }
    const nextTabs = [...tabs];
    const sourceIndex = nextTabs.findIndex((tab) => tab.id === sourceID);
    const targetIndex = nextTabs.findIndex((tab) => tab.id === targetID);
    if (sourceIndex < 0 || targetIndex < 0) {
      return;
    }
    const [moved] = nextTabs.splice(sourceIndex, 1);
    nextTabs.splice(targetIndex, 0, moved);
    onReorderTabs(nextTabs.map((tab) => tab.id));
  };

  return (
    <div className="tab-bar" role="tablist" aria-label={t.tabListLabel}>
      <div className="tab-bar-list">
        {tabs.map((tab) => {
          const status = getTabStatus(tab);

          return (
            <button
              key={tab.id}
              role="tab"
              aria-selected={tab.id === activeTabId}
              className={`tab-chip ${tab.id === activeTabId ? 'active' : ''} ${draggedTabId === tab.id ? 'dragging' : ''}`}
              onClick={() => onSelectTab(tab.id)}
              draggable
              onDragStart={(event) => {
                setDraggedTabId(tab.id);
                event.dataTransfer.effectAllowed = 'move';
                event.dataTransfer.setData('text/plain', tab.id);
              }}
              onDragOver={(event) => {
                if (!draggedTabId || draggedTabId === tab.id) {
                  return;
                }
                event.preventDefault();
                event.dataTransfer.dropEffect = 'move';
              }}
              onDrop={(event) => {
                event.preventDefault();
                const sourceID = event.dataTransfer.getData('text/plain') || draggedTabId;
                moveTab(sourceID, tab.id);
                setDraggedTabId('');
              }}
              onDragEnd={() => {
                setDraggedTabId('');
              }}
            >
              <span className={`tab-protocol tab-protocol-${tab.protocol}`}>{formatTabProtocol(tab)}</span>
              <span className={`tab-title tab-title-${status}`}>{tab.title}</span>
              <i
                aria-label={t.closeTab}
                onClick={(event) => {
                  event.stopPropagation();
                  onCloseTab(tab.id);
                }}
              >
                <FontAwesomeIcon icon={faXmark} />
              </i>
            </button>
          );
        })}
      </div>
      <button className="tab-chip tab-chip-action" onClick={onOpenLocalTerminal} aria-label={t.openLocalTerminal} title={t.openLocalTerminal}>
        <span className="tab-title tab-title-connected tab-action-title">
          <FontAwesomeIcon icon={faTerminal} />
        </span>
      </button>
    </div>
  );
}

function getTabStatus(tab: Tab): 'connected' | 'connecting' | 'disconnected' {
  if (tab.mode === 'terminal' && tab.connected) {
    return 'connected';
  }
  if (tab.connected) {
    return 'connected';
  }
  return 'disconnected';
}

function formatTabProtocol(tab: Tab) {
  if (tab.mode === 'terminal' && tab.protocol === 'ssh') {
    return 'SSH';
  }
  if (tab.protocol === 'sftp') {
    return 'SFTP';
  }
  if (tab.protocol === 'ftp') {
    return 'FTP';
  }
  if (tab.protocol === 'ssh') {
    return 'SSH';
  }
  if (tab.protocol === 'local') {
    return 'LOCAL';
  }
  return String(tab.protocol).toUpperCase();
}

const test = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader, hookHarness, elements } = require('./load-source.cjs');
const noop = () => {};

function terminalEvents() {
  const harness = hookHarness();
  const cleanups = new Set();
  const listeners = new Map();
  let subscriptions = 0;
  let disposals = 0;
  let changes = 0;
  let tabs = [{ id: 'A', sessionId: 'session-A', mode: 'terminal', remotePath: '/srv' }];
  const react = { ...harness.react, useEffect(effect, dependencies) {
    harness.react.useEffect(() => {
      const cleanup = effect();
      if (!cleanup) return;
      const dispose = () => { cleanups.delete(dispose); cleanup(); };
      cleanups.add(dispose);
      return dispose;
    }, dependencies);
  } };
  const { useTerminalEvents } = sourceLoader({ mocks: {
    react,
    '../../wailsjs/runtime/runtime': { EventsOn(name, callback) {
      subscriptions++;
      if (!listeners.has(name)) listeners.set(name, new Set());
      listeners.get(name).add(callback);
      return () => { disposals++; listeners.get(name).delete(callback); };
    } },
  } })('src/hooks/useTerminalEvents.ts');
  const setTabs = update => {
    const next = typeof update === 'function' ? update(tabs) : update;
    if (next !== tabs) changes++;
    tabs = next;
  };
  let params = { setTabs, closeTerminalTabOnDisconnect: true, onSessionClosed: async () => {} };
  const render = (next = tabs, overrides = {}) => {
    tabs = next;
    params = { ...params, ...overrides };
    harness.render(() => useTerminalEvents({ ...params, tabs }));
  };
  return {
    render,
    emit: (name, value) => listeners.get(name)?.forEach(callback => callback(value)),
    unmount: () => [...cleanups].forEach(cleanup => cleanup()),
    state: () => ({ tabs, subscriptions, disposals, changes }),
  };
}

test('重複終端路徑不更新狀態，路徑與分頁重排不重建既有訂閱', () => {
  const state = terminalEvents();
  state.render();
  const original = state.state().tabs;
  for (let i = 0; i < 100; i++) {
    state.emit('ssh:cwd:session-A', '/srv');
    state.emit('ssh:output:session-A', '\r\nuser@host:/srv$ ');
  }
  assert.equal(state.state().changes, 0);
  assert.equal(state.state().tabs, original);
  state.emit('ssh:cwd:session-A', '/next');
  state.render();
  state.render([...state.state().tabs, { id: 'file', mode: 'file' }]);
  state.render([...state.state().tabs].reverse());
  assert.equal(state.state().tabs[1].remotePath, '/next');
  assert.equal(state.state().subscriptions, 3);
  assert.equal(state.state().disposals, 0);
  state.unmount();
  assert.equal(state.state().disposals, 3);
});

test('終端重連、關閉與卸載會釋放訂閱，並使用最新斷線偏好', () => {
  const state = terminalEvents();
  const closed = [];
  state.render();
  state.render([{ ...state.state().tabs[0], sessionId: 'session-B' }], {
    closeTerminalTabOnDisconnect: false, onSessionClosed: async id => closed.push(id),
  });
  assert.equal(state.state().subscriptions, 6);
  assert.equal(state.state().disposals, 3);
  state.emit('ssh:cwd:session-A', '/stale');
  state.emit('ssh:closed:session-B');
  assert.equal(state.state().tabs[0].remotePath, '/srv');
  assert.equal(closed.length, 0);
  state.render(undefined, { closeTerminalTabOnDisconnect: true });
  state.emit('ssh:closed:session-B');
  assert.deepEqual(closed, ['session-B']);
  state.render([]);
  assert.equal(state.state().disposals, 6);
  state.emit('ssh:output:session-B', 'user@host:/gone$ ');
  assert.equal(state.state().tabs.length, 0);
  state.unmount();
  assert.equal(state.state().disposals, 6);
});

test('分段提示字串跨路徑更新仍能解析，重連不沿用舊提示資料', () => {
  const state = terminalEvents();
  state.render();
  state.emit('ssh:output:session-A', 'user@host:/pro');
  state.render([{ ...state.state().tabs[0], localPath: '/changed' }]);
  state.emit('ssh:output:session-A', 'ject$ ');
  assert.equal(state.state().tabs[0].remotePath, '/project');
  state.render([{ ...state.state().tabs[0], sessionId: 'new-session' }]);
  state.emit('ssh:output:new-session', 'ject$ ');
  assert.equal(state.state().tabs[0].remotePath, '/project');
  state.unmount();
});

test('既有終端訂閱使用最新狀態更新回呼', () => {
  const state = terminalEvents();
  state.render();
  let updated = state.state().tabs;
  state.render(undefined, { setTabs: update => { updated = update(updated); } });
  state.emit('ssh:cwd:session-A', '/latest');
  assert.equal(updated[0].remotePath, '/latest');
  assert.equal(state.state().tabs[0].remotePath, '/srv');
  assert.equal(state.state().subscriptions, 3);
  state.unmount();
});

test('大量檔案的 Shift 多選、右鍵及拖曳維持原有順序與內容', () => {
  const harness = hookHarness();
  const { FilePanel } = sourceLoader({ mocks: { react: harness.react } })('src/components/FilePanel.tsx');
  const entries = Array.from({ length: 2000 }, (_, i) => ({ name: `file-${i}`, path: `/remote/file-${i}`, side: 'remote', isDir: false, size: i, modified: '' }));
  let requested;
  let props = {
    title: '', location: { id: 'A', localPath: '/local', remotePath: '/remote' }, path: '/remote', entries,
    side: 'remote', sortState: { key: 'name', direction: 'asc' }, onSort: noop, locale: 'zh-TW',
    onContextMenuRequest: value => requested = value,
  };
  const rows = () => elements(harness.render(() => FilePanel(props)), item => item.props?.className?.startsWith('file-row '));
  const event = { preventDefault: noop, stopPropagation: noop, clientX: 0, clientY: 0 };
  rows()[20].props.onClick(event);
  rows()[1500].props.onClick({ ...event, shiftKey: true });
  const selected = rows();
  assert.equal(selected.filter(row => row.props['aria-selected']).length, 1481);
  selected[600].props.onContextMenu(event);
  const expected = entries.slice(20, 1501).map(entry => entry.path);
  assert.deepEqual(Array.from(requested.selectedEntries, entry => entry.path), expected);
  const payloads = new Map();
  selected[600].props.onDragStart({ ...event, dataTransfer: { setData: (type, value) => payloads.set(type, value) } });
  assert.deepEqual(JSON.parse(payloads.get('application/x-integrated-term-remote-paths')).paths, expected);
  props = { ...props, entries: entries.slice(500) };
  rows();
  const refreshed = rows();
  assert.equal(refreshed.filter(row => row.props['aria-selected']).length, 1001);
  refreshed[100].props.onContextMenu(event);
  assert.deepEqual(Array.from(requested.selectedEntries, entry => entry.path), entries.slice(500, 1501).map(entry => entry.path));
});

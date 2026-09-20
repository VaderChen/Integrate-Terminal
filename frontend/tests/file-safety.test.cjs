const test = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader, hookHarness, deferred, elements } = require('./load-source.cjs');
const pure = sourceLoader()('src/filePanelState.ts');
const tab = (id = 'A', overrides = {}) => ({ id, mode: 'file', protocol: 'sftp', host: `${id}.example`, port: 22, username: 'tester', localPath: '/tmp/local', remotePath: '/remote', connected: true, sessionId: '', ...overrides });
const entry = (name, side = 'remote') => ({ name, path: name === '..' ? '/' : `/remote/${name}`, side, isDir: name === '..', size: 0, modified: '' });
const noop = () => {};

test('parent navigation is not actionable in either single or batch selection', () => {
  assert.equal(pure.isActionableEntry(entry('..')), false);
  assert.deepEqual(Array.from(pure.actionableEntries([entry('..'), entry('keep')], 'remote', '/remote'), item => item.name), ['keep']);
  assert.equal(pure.actionableEntries([{ ...entry('fake'), path: '/remote/..' }], 'remote', '/remote').length, 0);
});

test('panel loader discards old host and same-host older request responses', async () => {
  let active = tab();
  let snapshot = null;
  const requests = [];
  const loader = pure.createPanelLoader({ getActiveTab: () => active, onSnapshot: value => snapshot = value, onError: error => { throw error; }, readLocal: async () => [], readRemote: () => { const pending = deferred(); requests.push(pending); return pending.promise; } });
  const a = loader.load(active);
  active = tab('B');
  assert.equal(pure.samePanelLocation(snapshot, active), false);
  const b = loader.load(active);
  requests[1].resolve([entry('B-only')]); await b;
  requests[0].resolve([entry('A-only')]); await a;
  assert.equal(snapshot.id, 'B'); assert.equal(snapshot.remoteFiles[0].name, 'B-only');
  const first = loader.load(active), second = loader.load(active);
  requests[3].resolve([entry('new')]); await second;
  requests[2].resolve([entry('old')]); await first;
  assert.equal(snapshot.remoteFiles[0].name, 'new');
});

test('panel identity masks stale entries immediately before the next effect or response', () => {
  const snapshot = { ...tab('A'), remoteFiles: [entry('A-only')] };
  assert.equal(pure.samePanelLocation(snapshot, tab('B')), false);
  assert.equal(pure.samePanelLocation(snapshot, tab('A', { remotePath: '/elsewhere' })), false);
});

test('background refresh cannot invalidate or overwrite the visible tab', async () => {
  const current = tab('B'); let writes = 0;
  const loader = pure.createPanelLoader({ getActiveTab: () => current, onSnapshot: () => writes++, onError: noop, readLocal: async () => { throw Error('must not read'); }, readRemote: async () => [] });
  await loader.load(tab('A'));
  assert.equal(writes, 0);
});

test('drag payload binds the original host and directory and rejects parent targets', () => {
  const payload = JSON.stringify({ tabId: 'A', side: 'remote', basePath: '/remote', paths: ['/remote/file'] });
  assert.deepEqual(Array.from(pure.resolveFileDrag(payload, tab('A'), 'remote')), ['/remote/file']);
  assert.equal(pure.resolveFileDrag(payload, tab('B'), 'remote'), null);
  assert.equal(pure.resolveFileDrag(payload, tab('A', { remotePath: '/other' }), 'remote'), null);
  assert.equal(pure.decodeFileDrag(JSON.stringify({ tabId: 'A', side: 'remote', basePath: '/remote', paths: ['/'] })), null);
  assert.equal(pure.decodeFileDrag(JSON.stringify(['/remote/file'])), null);
});

function fileActions(options = {}) {
  const harness = hookHarness();
  const calls = [];
  const api = new Proxy({}, { get: (_target, name) => async (...args) => { calls.push([name, ...args]); return undefined; } });
  const load = sourceLoader({ mocks: { react: harness.react }, globals: { window: { go: { app: { App: api } } } } });
  const { useFileActions } = load('src/hooks/useFileActions.ts');
  let dialog = null;
  const original = tab();
  const activeTabRef = { current: original }, tabsRef = { current: [original, tab('B')] };
  const params = { t: {}, activeTab: original, activeTabRef, tabsRef, contextMenu: null, actionDialog: null, directoryName: '', renameValue: '', setTabs: noop, setContextMenu: noop, setActionDialog: value => { dialog = typeof value === 'function' ? value(dialog) : value; }, setDirectoryName: noop, setRenameValue: noop, setErrorMessage: noop, invalidatePanels: noop, refreshPanels: async () => {}, refreshPanelsForPaths: async () => {}, ...options };
  return { calls, params, activeTabRef, tabsRef, getDialog: () => dialog, render: changes => harness.render(() => useFileActions({ ...params, ...changes })) };
}

test('actual file action hook refuses parent-only deletes and strips parent from batches', async () => {
  const hook = fileActions();
  hook.render({ contextMenu: { tabId: 'A', basePath: '/remote', side: 'remote', entry: entry('..') } }).handleDeleteEntry();
  assert.equal(hook.getDialog(), null);
  hook.render({ contextMenu: { tabId: 'A', basePath: '/remote', side: 'remote', entry: entry('file'), selectedEntries: [entry('..'), entry('file')] } }).handleDeleteEntry();
  await hook.render({ actionDialog: hook.getDialog() }).handleConfirmActionDialog();
  assert.deepEqual(hook.calls, [['DeleteEntry', 'A', 'remote', '/remote/file']]);
});

test('delete confirmation remains bound to A after active tab changes to B', async () => {
  const hook = fileActions();
  hook.render({ contextMenu: { tabId: 'A', basePath: '/remote', side: 'remote', entry: entry('file') } }).handleDeleteEntry();
  hook.activeTabRef.current = tab('B');
  await hook.render({ activeTab: tab('B'), actionDialog: hook.getDialog() }).handleConfirmActionDialog();
  assert.deepEqual(hook.calls, [['DeleteEntry', 'A', 'remote', '/remote/file']]);
});

test('closing the source tab cancels its pending delete instead of retargeting B', async () => {
  const hook = fileActions();
  hook.render({ contextMenu: { tabId: 'A', basePath: '/remote', side: 'remote', entry: entry('file') } }).handleDeleteEntry();
  const dialog = hook.getDialog();
  hook.tabsRef.current = [tab('B')]; hook.activeTabRef.current = tab('B');
  await hook.render({ activeTab: tab('B'), actionDialog: dialog }).handleConfirmActionDialog();
  assert.equal(hook.calls.length, 0);
});

test('actual FilePanel selection excludes parent and resets when the origin tab changes', () => {
  const harness = hookHarness();
  let requested;
  const load = sourceLoader({ mocks: { react: harness.react }, globals: {} });
  const { FilePanel } = load('src/components/FilePanel.tsx');
  let props = { title: '', location: tab('A'), path: '/remote', entries: [entry('..'), entry('one'), entry('two')], side: 'remote', sortState: { key: 'name', direction: 'asc' }, onSort: noop, locale: 'en', onContextMenuRequest: value => requested = value };
  const render = () => harness.render(() => FilePanel(props));
  const rows = tree => elements(tree, item => item.props?.className?.startsWith('file-row '));
  const event = { preventDefault: noop, stopPropagation: noop, clientX: 0, clientY: 0 };
  let tree = render();
  rows(tree)[2].props.onClick(event); tree = render();
  rows(tree)[0].props.onClick({ ...event, shiftKey: true }); tree = render();
  rows(tree)[2].props.onContextMenu(event);
  assert.equal(requested.selectedEntries.some(item => item.name === '..'), false);
  assert.equal(requested.tabId, 'A');
  props = { ...props, location: tab('B') }; render(); tree = render();
  assert.equal(rows(tree).filter(row => row.props['aria-selected']).length, 0);
});

test('disconnected tab can browse local files without calling the remote API', async () => {
  const active = tab('A', { connected: false }); let snapshot;
  const loader = pure.createPanelLoader({ getActiveTab: () => active, onSnapshot: value => snapshot = value, onError: error => { throw error; }, readLocal: async () => [entry('local', 'local')], readRemote: async () => { throw Error('remote should not be called'); } });
  await loader.load(active);
  assert.equal(snapshot.localReady, true); assert.equal(snapshot.localFiles.length, 1);
  assert.equal(snapshot.remoteReady, false); assert.equal(snapshot.remoteFiles.length, 0);
});

test('slow or failed remote listing leaves the successful local side usable and reports the error', async () => {
  const active = tab(); let snapshot; const errors = []; const remote = deferred();
  const loader = pure.createPanelLoader({ getActiveTab: () => active, onSnapshot: value => snapshot = value, onError: error => errors.push(error.message), readLocal: async () => [entry('local', 'local')], readRemote: () => remote.promise });
  const pending = loader.load(active);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(snapshot.localReady, true); assert.equal(snapshot.remoteReady, false);
  remote.reject(Error('permission denied')); await pending;
  assert.deepEqual(errors, ['permission denied']); assert.equal(snapshot.localReady, true);
  assert.equal(snapshot.remoteFiles.length, 0);
});

test('containment accepts relative remote directories without accepting their ancestors', () => {
  assert.equal(pure.isPathInside('file', '.'), true);
  assert.equal(pure.isPathInside('./file', '.'), true);
  assert.equal(pure.isPathInside('../file', '..'), true);
  assert.equal(pure.isPathInside('../..', '..'), false);
  assert.equal(pure.isPathInside('../outside', '.'), false);
  assert.equal(pure.isPathInside('/absolute', '.'), false);
});

test('stale directory context cannot open a delete dialog', () => {
  const hook = fileActions();
  hook.render({ contextMenu: { tabId: 'A', basePath: '/old-location', side: 'remote', entry: { ...entry('file'), path: '/old-location/file' } } }).handleDeleteEntry();
  assert.equal(hook.getDialog(), null);
});

const test = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader, hookHarness, deferred, elements } = require('./load-source.cjs');
const noop = () => {};
const tab = (id = 'A', extra = {}) => ({ id, siteId: id, title: id, mode: 'file', protocol: 'sftp', host: `${id}.example`, port: 22, username: 'test', localPath: '/tmp/local', remotePath: '/remote', connected: true, sessionId: '', ...extra });

function setup(apiOverrides = {}, extraParams = {}) {
  const harness = hookHarness();
  const calls = [];
  let nativeDrop;
  const api = { GetTransfers: async () => [], GetLogs: async () => [], DownloadDroppedPaths: async (...args) => calls.push(['download', ...args]), UploadDroppedPathsToSite: async (...args) => calls.push(['terminal-upload', ...args]), ...apiOverrides };
  const load = sourceLoader({
    mocks: { react: harness.react, '../../wailsjs/runtime/runtime': { EventsOn: () => noop, OnFileDrop: listener => nativeDrop = listener, OnFileDropOff: noop } },
    globals: { window: { go: { app: { App: api } } } },
  });
  const { useTransferActions } = load('src/hooks/useTransferActions.ts');
  const source = tab('A');
  const activeTabRef = { current: source }, tabsRef = { current: [source, tab('B')] };
  const params = { t: {}, activeTabRef, tabsRef, canAcceptFileDrop: () => true, setTransfers: noop, setLogs: noop, setErrorMessage: noop, refreshPanelsForPaths: async () => {}, ...extraParams };
  const actions = harness.render(() => useTransferActions(params));
  return { actions, activeTabRef, tabsRef, calls, nativeDrop, source };
}

test('download directory chooser preserves source host when the active tab changes', async () => {
  const picker = deferred();
  const state = setup({ SelectDirectory: () => picker.promise });
  const download = state.actions.handleDownloadEntryTo(state.source, '/remote/file');
  state.activeTabRef.current = tab('B');
  picker.resolve('/tmp/chosen'); await download;
  assert.deepEqual(JSON.parse(JSON.stringify(state.calls)), [['download', 'A', ['/remote/file'], '/tmp/chosen']]);
});

test('closing a download source while the native chooser is open cancels the action', async () => {
  const picker = deferred();
  const state = setup({ SelectDirectory: () => picker.promise });
  const download = state.actions.handleDownloadEntryTo(state.source, '/remote/file');
  state.activeTabRef.current = tab('B'); state.tabsRef.current = [tab('B')];
  picker.resolve('/tmp/chosen'); await download;
  assert.equal(state.calls.length, 0);
});

test('native terminal upload confirmation preserves the original SSH host', async () => {
  const confirmation = deferred();
  const state = setup({}, { requestTerminalDropConfirm: () => confirmation.promise });
  const a = tab('A', { mode: 'terminal', protocol: 'ssh', sessionId: 'session-A' });
  const b = tab('B', { mode: 'terminal', protocol: 'ssh', sessionId: 'session-B' });
  state.activeTabRef.current = a; state.tabsRef.current = [a, b];
  state.nativeDrop(0, 0, ['/tmp/source']);
  state.activeTabRef.current = b;
  confirmation.resolve(true);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(state.calls[0][0], 'terminal-upload');
  assert.equal(state.calls[0][1].host, 'A.example');
});

test('native terminal upload is cancelled if its session closes during confirmation', async () => {
  const confirmation = deferred();
  const state = setup({}, { requestTerminalDropConfirm: () => confirmation.promise });
  const a = tab('A', { mode: 'terminal', protocol: 'ssh', sessionId: 'session-A' });
  state.activeTabRef.current = a; state.tabsRef.current = [a];
  state.nativeDrop(0, 0, ['/tmp/source']);
  state.activeTabRef.current = tab('B'); state.tabsRef.current = [tab('B')];
  confirmation.resolve(true);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(state.calls.length, 0);
});

test('actual FilePanel rejects remote drag from another host before dispatching download', async () => {
  const harness = hookHarness(); let downloads = 0;
  const { FilePanel } = sourceLoader({ mocks: { react: harness.react } })('src/components/FilePanel.tsx');
  const tree = harness.render(() => FilePanel({ title: '', location: tab('B'), path: '/tmp/local', entries: [], side: 'local', sortState: { key: 'name', direction: 'asc' }, onSort: noop, locale: 'en', onDropFiles: () => downloads++ }));
  const panel = elements(tree, item => item.type === 'section')[0];
  await panel.props.onDrop({ preventDefault: noop, dataTransfer: { getData: () => JSON.stringify({ tabId: 'A', side: 'remote', basePath: '/remote', paths: ['/remote/file'] }) } });
  assert.equal(downloads, 0);
});

test('a loading FilePanel cannot dispatch file operations from retained DOM callbacks', async () => {
  const harness = hookHarness(); let actions = 0;
  const { FilePanel } = sourceLoader({ mocks: { react: harness.react } })('src/components/FilePanel.tsx');
  const tree = harness.render(() => FilePanel({ title: '', location: tab('A'), disabled: true, path: '/remote', entries: [{ name: 'dir', path: '/remote/dir', side: 'remote', isDir: true }], side: 'remote', sortState: { key: 'name', direction: 'asc' }, onSort: noop, locale: 'en', onOpenDirectory: () => actions++, onContextMenuRequest: () => actions++, onDropFiles: () => actions++ }));
  const row = elements(tree, item => item.props?.className?.startsWith('file-row '))[0];
  const event = { preventDefault: noop, stopPropagation: noop };
  row.props.onDoubleClick(); row.props.onContextMenu(event);
  await elements(tree, item => item.type === 'section')[0].props.onDrop(event);
  assert.equal(row.props.draggable, false); assert.equal(actions, 0);
});

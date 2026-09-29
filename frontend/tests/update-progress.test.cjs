const { test } = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader, hookHarness, deferred, elements } = require('./load-source.cjs');
const React = require('react');
const { renderToStaticMarkup } = require('react-dom/server');

test('更新下載先訂閱、即時更新，阻擋重複操作並清理失敗訂閱', async () => {
  const harness = hookHarness();
  let listener, disposed = 0, started = 0, attempt = 0;
  let request = deferred();
  const latest = { currentVersion: '1.0.0', latestVersion: '1.0.1', latestTag: 'v1.0.1', updateAvailable: true, canDownload: true, assetName: 'update.dmg' };
  const load = sourceLoader({ mocks: {
    react: harness.react,
    '../../wailsjs/runtime/runtime': { EventsOn(name, callback) { assert.equal(name, 'update:progress'); listener = callback; return () => { disposed++; }; } },
  }, globals: {
    crypto: { randomUUID: () => `attempt-${++attempt}` },
    window: { go: { app: { App: {
      CheckForUpdates: async () => latest,
      StartUpdate: (tag, id) => { started++; assert.ok(listener); assert.equal(tag, latest.latestTag); listener({ requestID: id, stage: 'downloading', downloadedBytes: 25, totalBytes: 100 }); return request.promise; },
    } } } },
  } });
  const { useUpdateActions } = load('src/hooks/useUpdateActions.ts');
  const render = () => harness.render(() => useUpdateActions('zh-TW'));
  await render().check();
  const running = render().start();
  assert.equal(render().progress.downloadedBytes, 25);
  await render().start();
  render().close();
  assert.equal(started, 1); assert.ok(render().result);
  listener({ requestID: 'older-attempt', stage: 'downloading', downloadedBytes: 99, totalBytes: 100 });
  assert.equal(render().progress.downloadedBytes, 25);
  request.reject(new Error('interrupted'));
  await running;
  assert.match(render().error, /interrupted/);
  assert.equal(render().busy, false); assert.equal(render().progress, null); assert.equal(disposed, 1);
  request = deferred();
  const retry = render().start();
  request.resolve({ downloaded: true, installScheduled: true, restarting: true });
  await retry;
  assert.equal(started, 2); assert.equal(disposed, 2); assert.equal(render().actionResult.restarting, true);
});

test('下載進度顯示實際百分比與大小，驗證階段不顯示假百分比', () => {
  const { UpdateDialog } = sourceLoader().call(null, 'src/components/UpdateDialog.tsx');
  const props = { locale: 'zh-TW', result: { currentVersion: '1', latestVersion: '2', canDownload: true, assetName: 'update.dmg' }, actionBusy: true, actionResult: null, actionError: '', onClose() {}, onStartUpdate() {}, progress: { stage: 'downloading', downloadedBytes: 1024*1024, totalBytes: 4*1024*1024 } };
  let html = renderToStaticMarkup(React.createElement(UpdateDialog, props));
  assert.match(html, /25%/); assert.match(html, /1\.0 MB.*4\.0 MB/); assert.match(html, /<progress[^>]*value="25"/);
  html = renderToStaticMarkup(React.createElement(UpdateDialog, { ...props, progress: { ...props.progress, stage: 'verifying' } }));
  assert.match(html, /驗證更新檔案/); assert.doesNotMatch(html, /<progress[^>]*value=/);
});

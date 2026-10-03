const test = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader } = require('./load-source.cjs');
const load = sourceLoader();
const { isPathInside, actionableEntries, decodeFileDrag } = load('src/filePanelState.ts');
const { writeLocalEcho } = load('src/components/terminalUtils.ts');
const { attachTerminalOutput } = load('src/components/terminalOutput.ts');

test('批次路徑處理保留相對根目錄與父層深度規則', () => {
  const cases = [
    ['/base/file', '/base', true], ['/base2/file', '/base', false], ['/base', '/base', false],
    ['../base/file', '../base', true], ['../../base/file', '../base', false],
    ['base/file', '.', true], ['../file', '.', false], ['/file', '.', false],
    ['/a/../file', '/', true], ['..', '../', false], ['../../file', '../..', true],
    ['C:\\base\\file', 'C:\\base', true], ['/ 中文 /檔案 ', '/ 中文 ', true],
  ];
  for (const [path, directory, expected] of cases) {
    assert.equal(isPathInside(path, directory), expected, `${path} inside ${directory}`);
    const entries = [{ name: 'file', path, side: 'local' }];
    assert.equal(actionableEntries(entries, 'local', directory).length, expected ? 1 : 0);
    const drag = decodeFileDrag(JSON.stringify({ tabId: 'A', side: 'local', basePath: directory, paths: [path, path] }));
    assert.equal(drag?.paths.length ?? 0, expected ? 1 : 0);
  }
  assert.equal(decodeFileDrag(JSON.stringify({ tabId: 'A', side: 'local', basePath: '/base', paths: ['/base/ok', '/outside'] })), null);
});

test('本機回顯批次寫入保留 Unicode、退格及 CRLF 的原始位元組順序', () => {
  const writes = [];
  const terminal = { write: value => writes.push(value) };
  writeLocalEcho(terminal, '');
  assert.equal(writes.length, 0);
  const input = '中文😀\r\nabc\u007f\u007f\x1b[31m';
  writeLocalEcho(terminal, input);
  assert.deepEqual(writes, ['中文😀\r\n\nabc\b \b\b \b\x1b[31m']);
  writeLocalEcho(terminal, 'x'.repeat(10000));
  assert.equal(writes.length, 2);
  assert.equal(writes[1].length, 10000);
});

test('終端連續輸出、缺口、空區塊、重複序號與寫入重入保持順序', async () => {
  let receive;
  const writes = [];
  const output = attachTerminalOutput({
    subscribe(listener) { receive = listener; return () => {}; },
    readSnapshot: async () => ({ output: 'snapshot', sequence: 1 }),
    write(chunk, replay) {
      writes.push([chunk, replay]);
      if (chunk === 'two') receive('three', 3);
    },
  });
  receive('stale', 1);
  await output.ready;
  receive('old-three', 3);
  receive('two', 2);
  receive('', 4);
  receive('five', 5);
  receive('duplicate', 5);
  receive('invalid', NaN);
  assert.deepEqual(writes, [['snapshot', true], ['two', false], ['three', false], ['', false], ['five', false]]);
  output.dispose();
  receive('closed', 6);
  assert.equal(writes.length, 5);
});

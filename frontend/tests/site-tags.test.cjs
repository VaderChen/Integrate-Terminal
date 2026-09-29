const test = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader, hookHarness, elements } = require('./load-source.cjs');

test('標籤可連續輸入、儲存、重新開啟並清空，舊站台不必有 tags 欄位', () => {
  let saved;
  let draft = { id: 'smoke', name: '測試站台', folder: '', protocol: 'sftp', host: 'example.test', port: 22, username: 'tester', password: '', ppkPath: '/tmp/test.ppk', ppkPassphrase: '', localPath: '/', remotePath: '/' };
  function openEditor() {
    const harness = hookHarness();
    const { ConnectForm } = sourceLoader({ mocks: { react: harness.react } })('src/components/ConnectForm.tsx');
    const render = () => harness.render(() => ConnectForm({ draft, onChange: next => draft = next, onSave: () => saved = JSON.parse(JSON.stringify(draft)), canSave: true, isDirty: true, expanded: true, onToggle() {}, variant: 'dialog', locale: 'zh-TW' }));
    const field = () => elements(render(), item => item.type === 'input' && item.props.placeholder === '以逗號分隔，例如：正式、客戶 A')[0];
    return { render, field };
  }
  let editor = openEditor();
  assert.equal(editor.field().props.value, '');
  editor.field().props.onChange({ target: { value: '正式,' } });
  assert.equal(editor.field().props.value, '正式,');
  editor.field().props.onChange({ target: { value: '正式, 客戶 A，正式、測試' } });
  assert.deepEqual(Array.from(draft.tags), ['正式', '客戶 A', '測試']);
  elements(editor.render(), item => item.type === 'button' && item.props['aria-label'] === '儲存站台')[0].props.onClick();
  draft = saved;
  editor = openEditor();
  assert.equal(editor.field().props.value, '正式, 客戶 A, 測試');
  editor.field().props.onChange({ target: { value: '' } });
  assert.deepEqual(Array.from(draft.tags), []);
});

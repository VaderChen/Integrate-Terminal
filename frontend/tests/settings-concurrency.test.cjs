const { test } = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader, hookHarness, deferred } = require('./load-source.cjs');

for (const failFirst of [false, true]) test(`連續設定修改保留欄位，前一筆${failFirst ? '失敗' : '成功'}後佇列繼續`, async () => {
 const harness = hookHarness();
 let config = { theme: 'neutral', language: '', showHiddenFiles: false };
 const first = deferred(); const calls = [];
 const load = sourceLoader({ mocks: { react: harness.react }, globals: { window: { go: { app: { App: {
  SaveConfig: async next => { calls.push({ ...next }); if(calls.length === 1) return first.promise; return next; },
 } } } } } });
 const { useSettingsActions } = load('src/hooks/useSettingsActions.ts');
 const render = () => harness.render(() => useSettingsActions({config, setConfig: next => {config = next;}, activeTabRef:{current:null}, refreshPanels:async()=>{}}));
 const actions=render();
 const theme=actions.handleThemeChange('dark');
 const outcome=theme.then(()=>null,e=>e);
 const language=actions.handleLanguageChange('en');
 render();
 await Promise.resolve();
 assert.equal(calls.length,1);
 assert.equal(config.theme,'dark'); assert.equal(config.language,'en');
 if(failFirst) first.reject(new Error('write failed')); else first.resolve(calls[0]);
 await outcome; await language;
 assert.equal(calls.length,2);
 assert.equal(calls[1].theme, failFirst ? 'neutral' : 'dark');
 assert.equal(config.language,'en');
 assert.equal(config.theme, failFirst ? 'neutral' : 'dark');
 const hidden=render().handleShowHiddenFilesChange(true); await hidden;
 assert.equal(config.showHiddenFiles,true); assert.equal(config.language,'en');
});

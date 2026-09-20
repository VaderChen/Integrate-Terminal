const test = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader, hookHarness, deferred, elements } = require('./load-source.cjs');
const fakeToken = 'FAKE-TOKEN-FOR-REGRESSION-ONLY';
const executable = "/Applications/Integ TERM's App.app/Contents/MacOS/IntegTERM";
const tick = () => new Promise(resolve => setImmediate(resolve));

function setup(overrides = {}) {
  const harness = hookHarness();
  const clipboard = [], writes = [];
  let running = false, docsCalls = 0;
  const props = {
    locale: 'en',
    config: { restServerEnabled: false, restServerPort: 18080, showTrayIcon: true },
    onRESTServerEnabledChange: async (enabled, port) => {
      writes.push({ enabled, port });
      props.config = { ...props.config, restServerEnabled: enabled, restServerPort: port };
      running = enabled;
    },
    onRESTServerPortChange: async port => {
      writes.push({ port }); props.config = { ...props.config, restServerPort: port };
    },
    onShowTrayIconChange: async () => {},
    ...overrides.props,
  };
  const api = {
    GetRestAPIDocsMarkdown: async () => { docsCalls++; return `# MCP\nhttp://127.0.0.1:${props.config.restServerPort}/mcp\nBearer ${fakeToken}\n{"token":"${fakeToken}"}`; },
    GetRESTServerStatus: async () => ({ enabled: props.config.restServerEnabled, running, attached: false, port: props.config.restServerPort, baseURL: `http://127.0.0.1:${props.config.restServerPort}` }),
    GetRESTServerToken: async () => fakeToken,
    GetMCPStdioExecutable: async () => executable,
    ExportRestAPIDocsMarkdown: async () => '',
    ...overrides.api,
  };
  const load = sourceLoader({ mocks: { react: harness.react }, globals: { window: { go: { app: { App: api } } }, navigator: { clipboard: { writeText: async value => { clipboard.push(value); } } } } });
  const { MCPSettingsPanel } = load('src/components/MCPSettingsPanel.tsx');
  const render = () => harness.render(() => MCPSettingsPanel(props));
  const button = (tree, label) => elements(tree, item => item.type === 'button' && item.props['aria-label'] === label)[0];
  const network = () => { elements(render(), item => item.props?.role === 'tab')[1].props.onClick(); return render(); };
  const state = { props, api, render, button, network, clipboard, writes, setRunning: value => { running = value; }, docsCalls: () => docsCalls };
  render();
  return state;
}

test('local stdio JSON preserves an executable path with spaces and quotes without starting HTTP', async () => {
  const state = setup(); await tick();
  const tree = state.render();
  const json = elements(tree, item => item.props?.className === 'settings-mcp-config')[0].props.children;
  assert.deepEqual(JSON.parse(json), { mcpServers: { integterm: { command: executable, args: ['mcp'] } } });
  assert.equal(state.writes.length, 0);
  assert.equal(elements(tree, item => item.props?.role === 'switch').length, 0);
  state.button(tree, 'Copy configuration').props.onClick(); await tick();
  assert.equal(state.clipboard[0], json);
});

test('document and HTTP config never expose the token, even after explicitly revealing its dedicated input', async () => {
  const state = setup(); await tick(); state.network();
  let tree = state.render();
  let input = elements(tree, item => item.type === 'input' && item.props['aria-label'] === 'API Token')[0];
  assert.equal(input.props.type, 'password');
  const previews = () => elements(tree, item => item.type === 'pre').map(item => item.props.children).join('\n');
  assert.equal(previews().includes(fakeToken), false);
  assert.ok(previews().includes('<YOUR_API_TOKEN>'));
  state.button(tree, 'Show Token').props.onClick(); tree = state.render();
  input = elements(tree, item => item.type === 'input' && item.props['aria-label'] === 'API Token')[0];
  assert.equal(input.props.type, 'text'); assert.equal(input.props.value, fakeToken);
  assert.equal(previews().includes(fakeToken), false);
  state.button(tree, 'Copy Content').props.onClick();
  state.button(tree, 'Copy configuration').props.onClick(); await tick();
  assert.ok(state.clipboard.every(value => !value.includes(fakeToken)));
  state.button(tree, 'Copy Token').props.onClick(); await tick();
  assert.equal(state.clipboard.at(-1), fakeToken);
});

test('leaving the HTTP tab resets explicit token visibility before returning', async () => {
  const state = setup(); await tick(); state.network();
  state.button(state.render(), 'Show Token').props.onClick(); state.render();
  elements(state.render(), item => item.props?.role === 'tab')[0].props.onClick(); state.render();
  state.network();
  const input = elements(state.render(), item => item.type === 'input' && item.props['aria-label'] === 'API Token')[0];
  assert.equal(input.props.type, 'password');
});

test('editing a stopped HTTP port refreshes both the document and client endpoint', async () => {
  const state = setup(); await tick(); state.network();
  let input = elements(state.render(), item => item.type === 'input' && item.props.type === 'number')[0];
  input.props.onChange({ target: { value: '22555' } });
  input = elements(state.render(), item => item.type === 'input' && item.props.type === 'number')[0];
  input.props.onBlur({ relatedTarget: null }); await tick(); state.render(); await tick();
  const previews = elements(state.render(), item => item.type === 'pre').map(item => item.props.children);
  assert.ok(state.docsCalls() > 1);
  assert.ok(previews.every(value => value.includes('22555')));
  assert.deepEqual(state.writes, [{ port: 22555 }]);
});

test('HTTP enable sends the draft port in one save and refreshes status after asynchronous service startup', async () => {
  const saved = deferred();
  const state = setup(); await tick(); state.network();
  state.props.onRESTServerEnabledChange = async (enabled, port) => {
    state.writes.push({ enabled, port });
    state.props.config = { ...state.props.config, restServerEnabled: enabled, restServerPort: port };
    await saved.promise;
  };
  let input = elements(state.render(), item => item.type === 'input' && item.props.type === 'number')[0];
  input.props.onChange({ target: { value: '22556' } });
  input = elements(state.render(), item => item.type === 'input' && item.props.type === 'number')[0];
  input.props.onBlur({ relatedTarget: { hasAttribute: name => name === 'data-mcp-toggle' } });
  state.button(state.render(), 'MCP Server').props.onClick();
  state.render(); await tick();
  assert.deepEqual(state.writes, [{ enabled: true, port: 22556 }]);
  assert.equal(state.button(state.render(), 'MCP Server').props.disabled, true);
  state.setRunning(true); saved.resolve(); await tick();
  const tree = state.render();
  assert.ok(elements(tree, item => item.type === 'span').some(item => typeof item.props.children === 'string' && item.props.children.includes('Running now at http://127.0.0.1:22556/mcp')));
  assert.equal(state.button(tree, 'MCP Server').props.disabled, false);
});

test('token retrieval failure suppresses a legacy document containing its unknown secret', async () => {
  const state = setup({ api: { GetRESTServerToken: async () => { throw 'Keychain is unavailable'; } } });
  await tick(); const tree = state.render();
  assert.equal(elements(tree, item => item.type === 'pre').some(item => String(item.props.children).includes(fakeToken)), false);
  assert.equal(elements(tree, item => item.props?.role === 'alert')[0].props.children, 'Keychain is unavailable');
});

test('an older metadata response cannot overwrite the document after changing the port', async () => {
  const oldDocs = deferred(); let calls = 0;
  const state = setup({ api: { GetRestAPIDocsMarkdown: () => ++calls === 1 ? oldDocs.promise : Promise.resolve('NEW PORT 22557') } });
  state.props.config = { ...state.props.config, restServerPort: 22557 }; state.render(); await tick();
  oldDocs.resolve('OLD PORT 18080'); await tick();
  const tree = state.render();
  assert.equal(elements(tree, item => item.props?.className === 'settings-skill-viewer')[0].props.children, 'NEW PORT 22557');
});

test('the settings hook saves HTTP enabled and draft port together', async () => {
  const writes = [];
  const load = sourceLoader({ globals: { window: { go: { app: { App: { SaveConfig: async config => { writes.push(config); return config; } } } } } } });
  const { useSettingsActions } = load('src/hooks/useSettingsActions.ts');
  const actions = useSettingsActions({ config: { restServerEnabled: false, restServerPort: 18080, theme: 'dark' }, setConfig: () => {}, activeTabRef: { current: null }, refreshPanels: async () => {} });
  await actions.handleRESTServerEnabledChange(true, 22558);
  assert.deepEqual(JSON.parse(JSON.stringify(writes)), [{ restServerEnabled: true, restServerPort: 22558, theme: 'dark' }]);
});

test('invalid port input does not enable HTTP with a truncated or guessed port', async () => {
  const state = setup(); await tick(); state.network();
  const input = elements(state.render(), item => item.type === 'input' && item.props.type === 'number')[0];
  input.props.onChange({ target: { value: '18080.5' } });
  state.button(state.render(), 'MCP Server').props.onClick();
  assert.equal(state.writes.length, 0);
  assert.equal(elements(state.render(), item => item.props?.role === 'alert')[0].props.children, 'The port must be an integer from 1 to 65535.');
});

test('native service-start failures are visible and release the disabled settings controls', async () => {
  const state = setup({ props: { onRESTServerEnabledChange: async () => { throw 'bind: address already in use'; } } });
  await tick(); state.network();
  state.button(state.render(), 'MCP Server').props.onClick(); await tick();
  const tree = state.render();
  assert.equal(elements(tree, item => item.props?.role === 'alert')[0].props.children, 'bind: address already in use');
  assert.equal(state.button(tree, 'MCP Server').props.disabled, false);
});

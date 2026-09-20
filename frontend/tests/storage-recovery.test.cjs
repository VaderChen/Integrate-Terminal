const test = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader, hookHarness, deferred, elements } = require('./load-source.cjs');

const noop = () => {};
const flushPromises = () => new Promise(resolve => setImmediate(resolve));

function textContent(element) {
  if (typeof element === 'string' || typeof element === 'number') return String(element);
  if (Array.isArray(element)) return element.map(textContent).join('');
  return element && typeof element === 'object' ? textContent(element.props?.children) : '';
}

function createApp() {
  const harness = hookHarness();
  const requests = [];
  let reportTransientError;
  const mocks = {
    react: harness.react,
    '@fortawesome/react-fontawesome': { FontAwesomeIcon: noop },
    '@fortawesome/free-solid-svg-icons': {},
    './components/AppOverlays': {},
    './hooks/useTerminalEvents': { useTerminalEvents: noop },
    './hooks/useFilePanels': {
      useFilePanels: () => ({
        localFiles: [], remoteFiles: [], localPanelReady: false, remotePanelReady: false,
        invalidatePanels: noop, refreshPanels: noop, refreshPanelsForPaths: noop,
      }),
    },
  };
  const components = {};
  for (const name of ['ConnectForm', 'FilePanel', 'SSHConsolePanel', 'SettingsModal', 'SiteList', 'TabBar', 'TransferPanel']) {
    components[name] = () => null;
    mocks[`./components/${name}`] = { [name]: components[name] };
  }
  for (const name of ['useConnectionActions', 'useFileActions', 'useSettingsActions', 'useSiteLibraryActions', 'usePurchaseActions', 'useTransferActions']) {
    mocks[`./hooks/${name}`] = { [name]: params => {
      if (name === 'useTransferActions') reportTransientError = params.setErrorMessage;
      return {};
    } };
  }
  const load = sourceLoader({
    mocks,
    globals: {
      navigator: { language: 'en', languages: ['en'] },
      window: {
        addEventListener: noop, removeEventListener: noop, setTimeout, clearTimeout,
        go: { app: { App: { Bootstrap: () => {
          const pending = deferred(); requests.push(pending); return pending.promise;
        } } } },
      },
    },
  });
  const App = load('src/App.tsx').default;
  const { fallbackBootstrap } = load('src/appUtils.ts');
  const render = () => harness.render(App);
  const payload = overrides => ({ ...fallbackBootstrap, ...overrides });
  const alert = tree => elements(tree, item => item.props?.role === 'alert')[0];
  const retry = tree => {
    const banner = alert(tree);
    assert.ok(banner, 'storage failure remains visible');
    const buttons = elements(banner, item => item.type === 'button');
    assert.equal(buttons.length, 1, 'storage failure has a retry action');
    assert.equal(buttons[0].props.disabled, false, 'retry is available after the failed request');
    buttons[0].props.onClick();
    return render();
  };
  return { render, requests, payload, alert, retry, components, reportTransientError: message => reportTransientError(message) };
}

for (const failureMode of ['payload storageError', 'rejected Bootstrap']) {
  test(`App keeps ${failureMode} visible and recovers saved tabs through retry`, async () => {
    const app = createApp();
    app.render();
    assert.equal(app.requests.length, 1);
    if (failureMode === 'payload storageError') {
      app.requests[0].resolve(app.payload({ storageError: 'Keychain access denied' }));
    } else {
      app.requests[0].reject('Keychain access denied');
    }
    await flushPromises();
    let tree = app.render();
    assert.match(textContent(app.alert(tree)), /Keychain access denied/);

    // The recoverable storage failure must not be dismissed by the ordinary
    // transfer/connection error banner's independent close action.
    app.reportTransientError('Unrelated transfer failure');
    tree = app.render();
    const closeTransient = elements(tree, item => item.type === 'button' && item.props?.className?.includes('error-banner-close'));
    assert.equal(closeTransient.length, 1);
    closeTransient[0].props.onClick();
    tree = app.render();
    assert.match(textContent(app.alert(tree)), /Keychain access denied/);
    assert.equal(app.requests.length, 1, 'ordinary renders do not silently reload or clear storage errors');

    app.retry(tree);
    assert.equal(app.requests.length, 2, 'retry calls the real App Bootstrap effect again');
    app.requests[1].reject('Keychain is still locked');
    await flushPromises();
    tree = app.render();
    assert.match(textContent(app.alert(tree)), /Keychain is still locked/);

    app.retry(tree);
    assert.equal(app.requests.length, 3, 'another failed attempt leaves retry usable');
    const restoredTabs = [
      { id: 'first', title: 'First', mode: 'file', protocol: 'sftp', connected: false, localPath: '/tmp', remotePath: '/' },
      { id: 'hidden-service', hidden: true, mode: 'file', protocol: 'sftp', connected: false },
      { id: 'preferred', title: 'Restored', mode: 'file', protocol: 'sftp', connected: false, localPath: '/tmp', remotePath: '/srv' },
    ];
    const restoredSites = [{ id: 'saved-site', name: 'Saved server', protocol: 'sftp' }];
    const recovered = app.payload({ storageError: '', tabs: restoredTabs, sites: restoredSites });
    recovered.config = { ...recovered.config, lastActiveTab: 'preferred' };
    app.requests[2].resolve(recovered);
    await flushPromises();
    tree = app.render();

    assert.equal(app.alert(tree), undefined, 'successful reload clears the persistent storage error');
    const tabBar = elements(tree, item => item.type === app.components.TabBar)[0];
    assert.deepEqual(Array.from(tabBar.props.tabs, tab => tab.id), ['first', 'preferred']);
    assert.equal(tabBar.props.activeTabId, 'preferred', 'saved active tab is restored');
    const siteList = elements(tree, item => item.type === app.components.SiteList)[0];
    assert.deepEqual(Array.from(siteList.props.sites, site => site.id), ['saved-site']);
    app.render();
    assert.equal(app.requests.length, 3, 'successful recovery does not cause a Bootstrap loop');
  });
}

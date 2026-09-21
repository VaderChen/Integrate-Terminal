const test = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader, hookHarness, elements } = require('./load-source.cjs');

function setup(interactive = false) {
  const harness = hookHarness();
  const listeners = new Map();
  const document = {
    body: {}, activeElement: null,
    documentElement: { clientWidth: 320, clientHeight: 400 },
    addEventListener: (name, handler) => listeners.set(name, handler),
    removeEventListener: (name, handler) => { if (listeners.get(name) === handler) listeners.delete(name); },
  };
  const portalRoot = {};
  let tree;
  const button = {
    contains: target => target === button,
    closest: () => portalRoot,
    getBoundingClientRect: () => ({ left: 280, top: 340, bottom: 364 }),
    focus: () => { document.activeElement = button; trigger().props.onFocus(); },
  };
  const action = { focus: () => { document.activeElement = action; } };
  const panel = {
    contains: target => target === panel || target === action,
    getBoundingClientRect: () => ({ width: 288, height: 100 }),
    querySelector: () => action,
  };
  const load = sourceLoader({
    mocks: {
      react: { ...harness.react, useId: () => 'help-test', useLayoutEffect: harness.react.useEffect },
      'react-dom': { createPortal: (child, root) => { assert.equal(root, portalRoot); return child; } },
    },
    globals: { document, window: { addEventListener() {}, removeEventListener() {} } },
  });
  const { InfoBubble } = load('src/components/InfoBubble.tsx');
  const props = { label: '說明', children: '泡泡內的說明', ...(interactive ? { triggerText: '文件', closeLabel: '關閉' } : {}) };
  const trigger = () => elements(tree, item => item.type === 'button' && item.props['aria-label'] === '說明')[0];
  const popup = () => elements(tree, item => item.props?.role === (interactive ? 'dialog' : 'tooltip'))[0];
  const render = () => {
    tree = harness.render(() => {
      const result = InfoBubble(props);
      for (const item of elements(result, item => item.ref)) item.ref.current = item.type === 'button' ? button : panel;
      return result;
    });
    return tree;
  };
  render();
  return { render, trigger, popup, listeners, document, button, action };
}

test('說明滑鼠移入立即出現、不使用原生延遲 title，並限制於視窗內', async () => {
  const state = setup();
  assert.equal(state.popup(), undefined);
  assert.equal(state.trigger().props.title, undefined);
  state.trigger().props.onMouseEnter(); state.render(); state.render();
  assert.equal(state.popup().props.children[1], '泡泡內的說明');
  assert.equal(state.trigger().props['aria-describedby'], state.popup().props.id);
  assert.equal(state.popup().props.style.left, 20);
  assert.equal(state.popup().props.style.top, 232);
  state.trigger().props.onMouseLeave();
  await new Promise(resolve => setTimeout(resolve, 150));
  state.render();
  assert.equal(state.popup(), undefined);
});

test('鍵盤聚焦可顯示說明，Escape 關閉並清理監聽', () => {
  const state = setup();
  state.button.focus(); state.render();
  assert.ok(state.popup());
  let prevented = false, stopped = false;
  state.listeners.get('keydown')({ key: 'Escape', preventDefault: () => { prevented = true; }, stopPropagation: () => { stopped = true; } });
  state.render();
  assert.equal(state.popup(), undefined);
  assert.ok(prevented && stopped);
  assert.equal(state.listeners.size, 0);
});

test('文件泡泡可由鍵盤進入操作，內部點擊保持開啟，外部點擊關閉', () => {
  const state = setup(true);
  state.button.focus(); state.render();
  assert.equal(state.trigger().props['aria-expanded'], true);
  state.trigger().props.onKeyDown({ key: 'Tab', shiftKey: false, preventDefault() {} });
  assert.equal(state.document.activeElement, state.action);
  state.trigger().props.onBlur({ relatedTarget: state.action }); state.render();
  state.listeners.get('pointerdown')({ target: state.action }); state.render();
  assert.ok(state.popup());
  state.listeners.get('keydown')({ key: 'Escape', preventDefault() {}, stopPropagation() {} }); state.render();
  assert.equal(state.popup(), undefined);
  assert.equal(state.document.activeElement, state.button);
  state.trigger().props.onClick(); state.render();
  state.listeners.get('pointerdown')({ target: {} }); state.render();
  assert.equal(state.popup(), undefined);
});

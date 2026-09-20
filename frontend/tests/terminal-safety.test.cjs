const test = require('node:test');
const assert = require('node:assert/strict');
const { sourceLoader, deferred } = require('./load-source.cjs');
const { attachTerminalOutput } = sourceLoader()('src/components/terminalOutput.ts');

function outputSession() {
  const snapshot = deferred(); const writes = []; let listener; let unsubscribed = false;
  const output = attachTerminalOutput({ subscribe: callback => { listener = callback; return () => unsubscribed = true; }, readSnapshot: () => snapshot.promise, write: (data, replay) => writes.push({ data, replay }) });
  return { output, snapshot, writes, emit: (data, sequence) => listener(data, sequence), unsubscribed: () => unsubscribed };
}

test('output arriving between snapshot capture and response is replayed exactly once', async () => {
  const state = outputSession();
  state.emit('already in snapshot', 1); state.emit('during request', 2);
  state.snapshot.resolve({ output: 'snapshot', sequence: 1 }); await state.output.ready;
  state.emit('after snapshot', 3); state.emit('duplicate', 2);
  assert.deepEqual(state.writes, [{ data: 'snapshot', replay: true }, { data: 'during request', replay: false }, { data: 'after snapshot', replay: false }]);
});

test('out-of-order live events wait for missing sequence and retain exact byte ordering', async () => {
  const state = outputSession();
  state.snapshot.resolve({ output: '', sequence: 0 }); await state.output.ready;
  state.emit('second', 2); assert.equal(state.writes.length, 0);
  state.emit('first', 1);
  assert.deepEqual(state.writes.map(item => item.data), ['first', 'second']);
});

test('snapshot covers events received while loading without duplicating ANSI/control data', async () => {
  const state = outputSession();
  state.emit('\x1b[', 10); state.emit('31mred', 11); state.emit('next', 12);
  state.snapshot.resolve({ output: '\x1b[31mred', sequence: 11 }); await state.output.ready;
  assert.deepEqual(state.writes.map(item => item.data), ['\x1b[31mred', 'next']);
});

test('disposed terminal ignores late snapshot and events', async () => {
  const state = outputSession(); state.output.dispose();
  state.snapshot.resolve({ output: 'old tab', sequence: 5 }); await state.output.ready;
  state.emit('late event', 6);
  assert.equal(state.writes.length, 0); assert.equal(state.unsubscribed(), true);
});

test('clipboard uses xterm paste, preserving bracketed paste, newline normalization and control characters', async () => {
  const xtermClipboard = sourceLoader()('node_modules/xterm/src/browser/Clipboard.ts');
  let clipboard = 'first\r\nsecond\n\x03';
  const { pasteClipboard } = sourceLoader({ mocks: { '../../wailsjs/runtime/runtime': { ClipboardGetText: async () => clipboard } } })('src/components/terminalUtils.ts');
  const sent = [];
  const core = { decPrivateModes: { bracketedPasteMode: true }, triggerDataEvent: data => sent.push(data) };
  const term = { paste: text => xtermClipboard.paste(text, { value: '' }, core, { rawOptions: {} }) };
  await pasteClipboard(term);
  assert.equal(sent[0], '\x1b[200~first\rsecond\r\x03\x1b[201~');
  core.decPrivateModes.bracketedPasteMode = false; clipboard = 'single\n';
  await pasteClipboard(term);
  assert.equal(sent[1], 'single\r');
  clipboard = ''; await pasteClipboard(term); assert.equal(sent.length, 2);
});

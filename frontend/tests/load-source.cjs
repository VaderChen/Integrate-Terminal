const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');
const { createRequire } = require('node:module');
const frontend = path.resolve(__dirname, '..');
const nativeRequire = createRequire(path.join(frontend, 'package.json'));

function sourceLoader({ mocks = {}, globals = {} } = {}) {
  const cache = new Map();
  function load(filename) {
    const resolved = path.resolve(frontend, filename);
    if (cache.has(resolved)) return cache.get(resolved).exports;
    const module = { exports: {} };
    cache.set(resolved, module);
    const source = fs.readFileSync(resolved, 'utf8');
    const compiled = ts.transpileModule(source, {
      compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
    }).outputText;
    const requireSource = name => {
      if (name in mocks) return mocks[name];
      if (!name.startsWith('.')) return nativeRequire(name);
      const base = path.resolve(path.dirname(resolved), name);
      const dependency = [base, base + '.ts', base + '.tsx', base + '.js'].find(candidate => fs.existsSync(candidate) && fs.statSync(candidate).isFile());
      if (!dependency) throw new Error(`Cannot resolve ${name} from ${resolved}`);
      return load(dependency);
    };
    vm.runInNewContext(compiled, { ...globals, module, exports: module.exports, require: requireSource, console, setTimeout, clearTimeout }, { filename: resolved });
    return module.exports;
  }
  return load;
}

function hookHarness() {
  const slots = [];
  let cursor = 0;
  let effects = [];
  const react = {
    useState(initial) {
      const index = cursor++;
      if (!(index in slots)) slots[index] = typeof initial === 'function' ? initial() : initial;
      return [slots[index], next => { slots[index] = typeof next === 'function' ? next(slots[index]) : next; }];
    },
    useRef(initial) {
      const index = cursor++;
      return slots[index] ?? (slots[index] = { current: initial });
    },
    useMemo(factory) { return factory(); },
    useEffect(effect, dependencies) {
      const index = cursor++;
      const previous = slots[index];
      if (!previous || !dependencies || dependencies.some((value, i) => value !== previous.dependencies?.[i])) {
        effects.push(() => {
          previous?.cleanup?.();
          slots[index] = { dependencies, cleanup: effect() };
        });
      }
    },
  };
  return {
    react,
    render(fn) {
      cursor = 0;
      const result = fn();
      const pending = effects;
      effects = [];
      pending.forEach(effect => effect());
      return result;
    },
  };
}
function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
function elements(element, predicate) {
  if (!element || typeof element !== 'object') return [];
  if (Array.isArray(element)) return element.flatMap(item => elements(item, predicate));
  return [...(predicate(element) ? [element] : []), ...elements(element.props?.children, predicate)];
}
module.exports = { sourceLoader, hookHarness, deferred, elements };

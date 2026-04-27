const fs = require('fs');
const path = require('path');
const vm = require('vm');

const appPath = path.join(__dirname, 'app.js');
const code = fs.readFileSync(appPath, 'utf8');

let errors = 0;

try {
  new vm.Script(code, { filename: 'app.js' });
  console.log('PASS: app.js has no syntax errors');
} catch (e) {
  console.error('FAIL: syntax error in app.js:', e.message);
  errors++;
}

const dom = {
  _els: {},
  createElement: function(tag) {
    return {
      tagName: tag,
      innerHTML: '',
      textContent: '',
      className: '',
      style: {},
      children: [],
      _listeners: {},
      setAttribute: function() {},
      getAttribute: function() { return ''; },
      appendChild: function(c) { this.children.push(c); return c; },
      addEventListener: function(evt, fn) {
        if (!this._listeners[evt]) this._listeners[evt] = [];
        this._listeners[evt].push(fn);
      },
      removeEventListener: function() {},
      classList: { add: function(){}, remove: function(){}, toggle: function(){}, contains: function(){ return false; } },
      querySelectorAll: function() { return { forEach: function(){} }; },
      querySelector: function() { return null; },
      cloneNode: function() { return dom.createElement(tag); },
      remove: function() {},
      focus: function() {},
      blur: function() {},
      play: function() { return Promise.resolve(); },
      pause: function() {},
      get value() { return ''; },
      set value(v) {}
    };
  }
};

function makeEl(id) {
  var el = dom.createElement('div');
  el.id = id;
  dom._els[id] = el;
  return el;
}

makeEl('app');

const sandbox = {
  document: {
    getElementById: function(id) { return dom._els[id] || null; },
    createElement: dom.createElement,
    querySelectorAll: function() { return { forEach: function(){} }; },
    querySelector: function() { return null; },
    body: makeEl('body'),
    head: makeEl('head'),
    addEventListener: function() {},
    createTextNode: function(t) { return { textContent: t }; }
  },
  window: {
    addEventListener: function() {},
    removeEventListener: function() {},
    location: { hash: '', href: '', pathname: '/' },
    history: { pushState: function(){}, replaceState: function(){} },
    innerWidth: 1024,
    innerHeight: 768,
    MediaSource: undefined,
    Hls: undefined
  },
  localStorage: {
    _data: {},
    getItem: function(k) { return this._data[k] || null; },
    setItem: function(k, v) { this._data[k] = String(v); },
    removeItem: function(k) { delete this._data[k]; }
  },
  location: { hash: '', href: '', pathname: '/' },
  history: { pushState: function(){}, replaceState: function(){} },
  navigator: { userAgent: 'node-smoke-test' },
  console: console,
  setTimeout: setTimeout,
  setInterval: function() { return 0; },
  clearInterval: function() {},
  clearTimeout: function() {},
  fetch: function() { return Promise.resolve({ ok: false, json: function() { return Promise.resolve({}); } }); },
  URL: { createObjectURL: function() { return 'blob:test'; } },
  Promise: Promise,
  Hls: undefined,
  module: { exports: {} }
};

sandbox.window.document = sandbox.document;
sandbox.self = sandbox.window;

try {
  const ctx = vm.createContext(sandbox);
  vm.runInContext(code, ctx, { filename: 'app.js', timeout: 5000 });
  console.log('PASS: app.js executes without runtime errors');

  var exported = sandbox.module.exports;
  if (exported && exported.pages) {
    var pageNames = Object.keys(exported.pages);
    console.log('PASS: pages object exported with keys:', pageNames.join(', '));
    var expected = ['dashboard', 'streams', 'channels', 'recordings', 'favorites', 'activity', 'settings', 'users', 'player', 'epgsources'];
    for (var i = 0; i < expected.length; i++) {
      if (pageNames.indexOf(expected[i]) < 0) {
        console.error('FAIL: missing page:', expected[i]);
        errors++;
      }
    }
  } else {
    console.error('FAIL: pages object not exported');
    errors++;
  }

  if (exported && typeof exported.esc === 'function') {
    var result = exported.esc('<script>alert(1)</script>');
    if (result.indexOf('<script>') >= 0) {
      console.error('FAIL: esc() does not escape HTML');
      errors++;
    } else {
      console.log('PASS: esc() properly escapes HTML');
    }
  }

  if (exported && typeof exported.formatTime === 'function') {
    var t1 = exported.formatTime(65);
    if (t1 === '1:05') {
      console.log('PASS: formatTime(65) = "1:05"');
    } else {
      console.error('FAIL: formatTime(65) =', t1, 'expected "1:05"');
      errors++;
    }
  }

  var appEl = dom._els['app'];
  if (appEl && appEl.innerHTML && appEl.innerHTML.indexOf('Failed to load') >= 0) {
    console.error('FAIL: render produced "Failed to load" text');
    errors++;
  } else {
    console.log('PASS: no "Failed to load" in initial render');
  }
} catch (e) {
  console.error('FAIL: runtime error:', e.message);
  errors++;
}

if (errors > 0) {
  console.error('\n' + errors + ' test(s) failed');
  process.exit(1);
} else {
  console.log('\nAll smoke tests passed');
}

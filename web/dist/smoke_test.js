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
      canPlayType: function() { return ''; },
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
    var expected = ['dashboard', 'streams', 'channels', 'library', 'guide', 'recordings', 'favorites', 'activity', 'sources', 'sourceprofiles', 'epgsources', 'wireguard', 'settings', 'users', 'clients', 'logos', 'tmdb', 'probe', 'playurl', 'player', 'hdhrdevices', 'invites', 'apikeys', 'developer'];
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

  function assert(cond, msg) {
    if (cond) {
      console.log('PASS: ' + msg);
    } else {
      console.error('FAIL: ' + msg);
      errors++;
    }
  }

  console.log('\n--- Playback Integration Tests ---');

  var PlayerRegistry = exported.PlayerRegistry;
  var detectCapabilities = exported.detectCapabilities;

  assert(typeof PlayerRegistry !== 'undefined' && PlayerRegistry !== null, 'PlayerRegistry exists');
  assert(typeof detectCapabilities === 'function', 'detectCapabilities is a function');

  var allModes = ['mse', 'hls', 'dash', 'webrtc', 'stream'];
  allModes.forEach(function(mode) {
    assert(PlayerRegistry.get(mode) !== null, mode + ' plugin registered');
  });

  allModes.forEach(function(mode) {
    var plugin = PlayerRegistry.get(mode);
    assert(typeof plugin.isSupported === 'function', mode + ' has isSupported()');
    assert(typeof plugin.serverParams === 'function', mode + ' has serverParams()');
    assert(typeof plugin.create === 'function', mode + ' has create()');
    assert(typeof plugin.label === 'string', mode + ' has label');
  });

  var expectedDelivery = { mse: 'mse', hls: 'hls', dash: 'dash', webrtc: 'webrtc', stream: 'stream' };
  allModes.forEach(function(mode) {
    var params = PlayerRegistry.get(mode).serverParams();
    assert(params.delivery === expectedDelivery[mode], mode + ' serverParams delivery = "' + expectedDelivery[mode] + '"');
  });

  var caps = detectCapabilities();
  assert(typeof caps === 'object', 'capabilities is object');
  assert('mse' in caps, 'has mse capability');
  assert('mse_h264' in caps, 'has mse_h264 capability');
  assert('mse_h265' in caps, 'has mse_h265 capability');
  assert('hls_native' in caps, 'has hls_native capability');
  assert('hls_js' in caps, 'has hls_js capability');
  assert('webrtc' in caps, 'has webrtc capability');

  assert(caps.mse === false, 'mse is false in Node.js');
  assert(caps.webrtc === false, 'webrtc is false in Node.js');

  allModes.forEach(function(mode) {
    var plugin = PlayerRegistry.get(mode);
    var mockVideo = {
      src: '',
      play: function() { return Promise.resolve(); },
      pause: function() {},
      srcObject: null,
      addEventListener: function() {},
      removeEventListener: function() {}
    };
    var instance = plugin.create(mockVideo);
    assert(typeof instance.start === 'function', mode + ' instance has start()');
    assert(typeof instance.stop === 'function', mode + ' instance has stop()');
  });

  var available = PlayerRegistry.available();
  assert(Array.isArray(available), 'PlayerRegistry.available() returns array');
  assert(available.indexOf('stream') >= 0, 'Direct/stream plugin always available');

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

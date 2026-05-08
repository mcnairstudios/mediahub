(function() {
  'use strict';

  var TOKEN_KEY = 'mediahub_token';
  var USER_KEY = 'mediahub_user';

  var bwTotalBytes = 0;
  var bwTotalSec = 0;
  var bwEstimate = 0;
  var _sourceTypes = [];
  var _sourceTypesLoaded = false;
  try {
    var bwStored = JSON.parse(sessionStorage.getItem('mediahub_bw') || '{}');
    if (bwStored.ts && Date.now() - bwStored.ts < 300000) {
      bwTotalBytes = bwStored.bytes || 0;
      bwTotalSec = bwStored.sec || 0;
      if (bwTotalSec > 0) bwEstimate = Math.round((bwTotalBytes * 8) / bwTotalSec / 1000);
    }
  } catch (e) {}
  var origFetch = window.fetch;
  window.fetch = function() {
    var start = performance.now();
    return origFetch.apply(this, arguments).then(function(resp) {
      var clone = resp.clone();
      clone.blob().then(function(b) {
        var elapsed = (performance.now() - start) / 1000;
        if (b.size > 2000 && elapsed > 0.005) {
          bwTotalBytes += b.size;
          bwTotalSec += elapsed;
          bwEstimate = Math.round((bwTotalBytes * 8) / bwTotalSec / 1000);
          try { sessionStorage.setItem('mediahub_bw', JSON.stringify({ bytes: bwTotalBytes, sec: bwTotalSec, ts: Date.now() })); } catch (e) {}
          var el = document.getElementById('bw-indicator');
          if (el) {
            var label = bwEstimate > 100000 ? (bwEstimate / 1000).toFixed(0) + ' Gbps'
              : bwEstimate > 1000 ? (bwEstimate / 1000).toFixed(1) + ' Mbps'
              : bwEstimate + ' kbps';
            el.textContent = '\u21C5 ' + label;
            el.style.color = bwEstimate > 50000 ? 'var(--success)' : bwEstimate > 10000 ? 'var(--accent)' : bwEstimate > 3000 ? 'var(--warning)' : 'var(--danger)';
          }
        }
      }).catch(function() {});
      return resp;
    });
  };

  function esc(s) {
    if (s == null) return '';
    var d = document.createElement('div');
    d.textContent = String(s);
    return d.innerHTML;
  }

  function setupAlphabetBar(gridContainer) {
    var existing = document.querySelector('.alpha-bar');
    if (existing) existing.remove();
    var existingKeyHandler = gridContainer._alphaKeyHandler;
    if (existingKeyHandler) {
      document.removeEventListener('keydown', existingKeyHandler);
    }

    var cards = gridContainer.querySelectorAll('[data-sort-name]');
    if (cards.length === 0) return;

    var letters = {};
    for (var i = 0; i < cards.length; i++) {
      var first = (cards[i].dataset.sortName || '')[0];
      if (first && /[A-Z]/.test(first)) letters[first] = true;
    }
    var available = Object.keys(letters).sort();
    if (available.length < 2) return;

    var bar = document.createElement('div');
    bar.className = 'alpha-bar';
    available.forEach(function(ch) {
      var span = document.createElement('span');
      span.textContent = ch;
      span.addEventListener('click', function() {
        jumpToLetter(ch, gridContainer);
      });
      bar.appendChild(span);
    });
    document.body.appendChild(bar);

    function jumpToLetter(letter, container) {
      var items = container.querySelectorAll('[data-sort-name]');
      for (var j = 0; j < items.length; j++) {
        var name = (items[j].dataset.sortName || '').toUpperCase();
        if (name >= letter) {
          items[j].scrollIntoView({ behavior: 'smooth', block: 'center' });
          bar.querySelectorAll('span').forEach(function(s) { s.classList.remove('active'); });
          var match = bar.querySelector('span[data-letter="' + letter + '"]');
          if (match) match.classList.add('active');
          break;
        }
      }
    }

    available.forEach(function(ch) {
      var spans = bar.querySelectorAll('span');
      for (var k = 0; k < spans.length; k++) {
        if (spans[k].textContent === ch) spans[k].setAttribute('data-letter', ch);
      }
    });

    function onKey(e) {
      if (!document.body.contains(gridContainer)) {
        document.removeEventListener('keydown', onKey);
        var staleBar = document.querySelector('.alpha-bar');
        if (staleBar) staleBar.remove();
        return;
      }
      if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) return;
      if (e.key.length !== 1 || e.ctrlKey || e.metaKey || e.altKey) return;
      var letter = e.key.toUpperCase();
      if (/[A-Z]/.test(letter)) {
        jumpToLetter(letter, gridContainer);
      }
    }
    gridContainer._alphaKeyHandler = onKey;
    document.addEventListener('keydown', onKey);
  }

  function decodeJWT(token) {
    if (!token) return null;
    try {
      var parts = token.split('.');
      if (parts.length !== 3) return null;
      var payload = parts[1].replace(/-/g, '+').replace(/_/g, '/');
      while (payload.length % 4) payload += '=';
      var json = atob(payload);
      return JSON.parse(json);
    } catch (e) { return null; }
  }

  function userFromToken(token) {
    var claims = decodeJWT(token);
    if (!claims) return null;
    return {
      username: claims.username || '',
      role: claims.role || 'standard',
      is_admin: claims.role === 'admin',
      user_id: claims.user_id || ''
    };
  }

  var api = {
    get token() { return localStorage.getItem(TOKEN_KEY); },
    set token(v) {
      if (v) {
        localStorage.setItem(TOKEN_KEY, v);
        var u = userFromToken(v);
        if (u) localStorage.setItem(USER_KEY, JSON.stringify(u));
      } else {
        localStorage.removeItem(TOKEN_KEY);
        localStorage.removeItem(USER_KEY);
      }
    },

    get user() {
      try { return JSON.parse(localStorage.getItem(USER_KEY)); } catch (e) { return null; }
    },
    set user(v) { if (v) localStorage.setItem(USER_KEY, JSON.stringify(v)); else localStorage.removeItem(USER_KEY); },

    async request(method, path, body) {
      var headers = { 'Content-Type': 'application/json' };
      if (this.token) headers['Authorization'] = 'Bearer ' + this.token;
      if (bwEstimate > 0) headers['X-Client-Bandwidth'] = String(bwEstimate);
      var opts = { method: method, headers: headers };
      if (body !== undefined) opts.body = JSON.stringify(body);
      var resp;
      try {
        resp = await fetch(path, opts);
      } catch (err) {
        console.error('Network error for ' + method + ' ' + path + ':', err);
        throw new Error('network error: ' + (err.message || 'request failed'));
      }
      if (resp.status === 401 && path !== '/api/auth/login') {
        this.token = null;
        this.user = null;
        router.navigate('login');
        throw new Error('session expired');
      }
      return resp;
    },

    async get(path) { return this.request('GET', path); },
    async post(path, body) { return this.request('POST', path, body); },
    async put(path, body) { return this.request('PUT', path, body); },
    async del(path) { return this.request('DELETE', path); }
  };

  async function _loadSourceTypes() {
    if (_sourceTypesLoaded) return _sourceTypes;
    try {
      var resp = await api.get('/api/source-types');
      var data = await resp.json();
      if (Array.isArray(data)) _sourceTypes = data;
      _sourceTypesLoaded = true;
    } catch (e) {
      console.error('Failed to load source types:', e);
    }
    return _sourceTypes;
  }

  function _sourceTypeColor(type) {
    for (var i = 0; i < _sourceTypes.length; i++) {
      if (_sourceTypes[i].type === type) return _sourceTypes[i].color || 'var(--text-muted)';
    }
    return 'var(--text-muted)';
  }

  function _sourceTypeShortLabel(type) {
    for (var i = 0; i < _sourceTypes.length; i++) {
      if (_sourceTypes[i].type === type) return _sourceTypes[i].short_label || type.toUpperCase();
    }
    return type.toUpperCase();
  }

  function _sourceTypeLabel(type) {
    for (var i = 0; i < _sourceTypes.length; i++) {
      if (_sourceTypes[i].type === type) return _sourceTypes[i].label || type;
    }
    return type;
  }

  function _showGenericAddSourceModal(sourceType) {
    var st = null;
    for (var i = 0; i < _sourceTypes.length; i++) {
      if (_sourceTypes[i].type === sourceType) { st = _sourceTypes[i]; break; }
    }
    if (!st) return;

    var existingOverlay = document.getElementById('generic-source-overlay');
    if (existingOverlay) existingOverlay.remove();

    var fields = st.config_fields || [];
    var fieldsHtml = '';
    for (var f = 0; f < fields.length; f++) {
      var field = fields[f];
      if (field.type === 'hidden') continue;
      if (field.type === 'custom') continue;
      var inputHtml = '';
      var fieldId = 'gen-src-' + field.key;
      var req = field.required ? ' required' : '';
      if (field.type === 'bool') {
        inputHtml = '<label style="display:flex;align-items:center;gap:8px;cursor:pointer">' +
          '<input type="checkbox" id="' + fieldId + '"' + (field.default === 'true' ? ' checked' : '') + ' style="width:18px;height:18px;accent-color:var(--accent)">' +
          ' ' + esc(field.label) + '</label>';
      } else if (field.type === 'select') {
        var opts = '';
        var options = field.options || [];
        for (var o = 0; o < options.length; o++) {
          opts += '<option value="' + esc(options[o].value) + '"' + (options[o].value === field.default ? ' selected' : '') + '>' + esc(options[o].label) + '</option>';
        }
        inputHtml = '<label class="form-label">' + esc(field.label) + (field.required ? ' *' : '') + '</label>' +
          '<select id="' + fieldId + '" class="form-input">' + opts + '</select>';
      } else {
        var inputType = field.type === 'password' ? 'password' : field.type === 'number' ? 'number' : field.type === 'url' ? 'url' : 'text';
        inputHtml = '<label class="form-label">' + esc(field.label) + (field.required ? ' *' : '') + '</label>' +
          '<input type="' + inputType + '" id="' + fieldId + '" class="form-input"' +
          (field.placeholder ? ' placeholder="' + esc(field.placeholder) + '"' : '') +
          (field.default ? ' value="' + esc(field.default) + '"' : '') +
          req + '>';
      }
      if (field.help_text) {
        inputHtml += '<div style="font-size:12px;color:var(--text-muted);margin-top:4px">' + esc(field.help_text) + '</div>';
      }
      fieldsHtml += '<div class="form-group">' + inputHtml + '</div>';
    }

    var overlay = document.createElement('div');
    overlay.id = 'generic-source-overlay';
    overlay.className = 'modal-overlay';
    overlay.innerHTML =
      '<div class="source-modal-form" style="max-width:480px">' +
      '<h2>Add ' + esc(st.label) + ' Source</h2>' +
      '<div class="form-group">' +
        '<label class="form-label">Name *</label>' +
        '<input type="text" id="gen-src-name" class="form-input" placeholder="My ' + esc(st.label) + '" required>' +
      '</div>' +
      fieldsHtml +
      '<div style="display:flex;gap:10px;justify-content:flex-end;margin-top:20px">' +
        '<button class="btn btn-ghost" id="gen-src-cancel">Cancel</button>' +
        '<button class="btn btn-primary" id="gen-src-create">Create</button>' +
      '</div>' +
      '</div>';

    overlay.addEventListener('click', function(e) {
      if (e.target === overlay) overlay.remove();
    });
    document.body.appendChild(overlay);

    document.getElementById('gen-src-cancel').addEventListener('click', function() { overlay.remove(); });
    document.getElementById('gen-src-create').addEventListener('click', async function() {
      var nameEl = document.getElementById('gen-src-name');
      var name = nameEl ? nameEl.value.trim() : '';
      if (!name) { toast('Name is required', 'error'); return; }

      var body = { name: name };
      for (var f = 0; f < fields.length; f++) {
        var field = fields[f];
        if (field.type === 'hidden' || field.type === 'custom') continue;
        var el = document.getElementById('gen-src-' + field.key);
        if (!el) continue;
        if (field.type === 'bool') {
          body[field.key] = el.checked ? 'true' : 'false';
        } else {
          var val = el.value.trim();
          if (field.required && !val) {
            toast(field.label + ' is required', 'error');
            return;
          }
          if (val) body[field.key] = val;
        }
      }

      try {
        var resp = await api.post('/api/source-plugins/' + encodeURIComponent(sourceType), body);
        if (!resp.ok) {
          var err = await resp.json().catch(function() { return {}; });
          toast(err.error || 'Failed to create source', 'error');
          return;
        }
        toast(st.label + ' source created', 'success');
        overlay.remove();
        _sourceTypesLoaded = false;
        sessionStorage.removeItem('mediahub_sources');
        router.navigate('streams');
      } catch (e) {
        toast('Failed to create source: ' + e.message, 'error');
      }
    });
  }

  var PlayerRegistry = {
    _players: {},
    register: function(mode, plugin) { this._players[mode] = plugin; },
    get: function(mode) { return this._players[mode] || null; },
    available: function() {
      var modes = [];
      for (var k in this._players) {
        if (!this._players[k].isSupported || this._players[k].isSupported()) modes.push(k);
      }
      return modes;
    }
  };

  var _capabilities = null;
  function detectCapabilities() {
    if (_capabilities) return _capabilities;
    var v = document.createElement('video');
    _capabilities = {
      mse: !!window.MediaSource,
      mse_h264: !!window.MediaSource && MediaSource.isTypeSupported('video/mp4; codecs="avc1.640028"'),
      mse_h265: !!window.MediaSource && MediaSource.isTypeSupported('video/mp4; codecs="hev1.1.6.L120"'),
      hls_native: v.canPlayType('application/vnd.apple.mpegurl') !== '',
      hls_js: typeof videojs !== 'undefined',
      webrtc: !!window.RTCPeerConnection
    };
    return _capabilities;
  }

  function toast(msg, type) {
    var el = document.getElementById('toast');
    if (!el) {
      el = document.createElement('div');
      el.id = 'toast';
      el.className = 'toast';
      document.body.appendChild(el);
    }
    el.textContent = msg;
    el.className = 'toast toast-' + (type || 'success') + ' visible';
    clearTimeout(el._timer);
    el._timer = setTimeout(function() { el.classList.remove('visible'); }, 3000);
  }

  function showFormModal(title, bodyHtml, opts) {
    opts = opts || {};
    var id = opts.id || 'form-modal-' + Date.now();
    var maxWidth = opts.maxWidth || '520px';
    var existing = document.getElementById(id);
    if (existing) existing.remove();
    var html = '<div class="modal-overlay" id="' + id + '">' +
      '<div class="modal-content" style="max-width:' + maxWidth + '">' +
      '<div class="modal-header"><span>' + esc(title) + '</span><button class="modal-close-x" title="Close">\u2715</button></div>' +
      '<div class="modal-body">' + bodyHtml + '</div>' +
      '<div class="modal-footer">' +
      '<button class="btn btn-ghost modal-cancel-btn">Cancel</button>' +
      '<button class="btn btn-primary modal-save-btn">' + esc(opts.saveLabel || 'Save') + '</button>' +
      '</div></div></div>';
    document.body.insertAdjacentHTML('beforeend', html);
    var overlay = document.getElementById(id);
    overlay.querySelector('.modal-cancel-btn').addEventListener('click', function() { overlay.remove(); });
    overlay.querySelector('.modal-close-x').addEventListener('click', function() { overlay.remove(); });
    overlay.addEventListener('click', function(e) { if (e.target === overlay) overlay.remove(); });
    document.addEventListener('keydown', function onKey(e) { if (e.key === 'Escape') { var el = document.getElementById(id); if (el) el.remove(); document.removeEventListener('keydown', onKey); } });
    return overlay;
  }

  var icons = {
    dashboard: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="9" rx="1"/><rect x="14" y="3" width="7" height="5" rx="1"/><rect x="14" y="12" width="7" height="9" rx="1"/><rect x="3" y="16" width="7" height="5" rx="1"/></svg>',
    streams: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M2 8l10-5 10 5-10 5z"/><path d="M2 12l10 5 10-5"/><path d="M2 16l10 5 10-5"/></svg>',
    channels: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/></svg>',
    recordings: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3" fill="currentColor"/></svg>',
    settings: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg>',
    users: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 00-3-3.87M16 3.13a4 4 0 010 7.75"/></svg>',
    search: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>',
    play: '<svg viewBox="0 0 24 24" fill="currentColor"><polygon points="5,3 19,12 5,21"/></svg>',
    pause: '<svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg>',
    menu: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/></svg>',
    stats: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 20V10M12 20V4M6 20v-6"/></svg>',
    sources: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 4h16v2H4zM4 11h16v2H4zM4 18h16v2H4z"/><circle cx="8" cy="5" r="1" fill="currentColor"/><circle cx="8" cy="12" r="1" fill="currentColor"/><circle cx="8" cy="19" r="1" fill="currentColor"/></svg>',
    refresh: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 4v6h6"/><path d="M23 20v-6h-6"/><path d="M20.49 9A9 9 0 005.64 5.64L1 10M23 14l-4.64 4.36A9 9 0 013.51 15"/></svg>',
    trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 01-2 2H7a2 2 0 01-2-2V6m3 0V4a2 2 0 012-2h4a2 2 0 012 2v2"/></svg>',
    plus: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>',
    empty: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M9 9l6 6M15 9l-6 6"/></svg>',
    wireguard: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2L3 7v6c0 5.25 3.82 10.15 9 11 5.18-.85 9-5.75 9-11V7l-9-5z"/><path d="M12 8v4M12 16h.01"/></svg>',
    epg: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 10h18M9 4v16"/></svg>',
    edit: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>',
    upload: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>',
    star: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>',
    starFilled: '<svg viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>',
    favorites: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>',
    addChannel: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M12 10v0"/><line x1="12" y1="7" x2="12" y2="13"/><line x1="9" y1="10" x2="15" y2="10"/></svg>',
    library: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 19.5A2.5 2.5 0 016.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z"/></svg>',
    guide: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>',
    clients: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/><circle cx="12" cy="10" r="3"/></svg>',
    probe: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/><path d="M11 8v6M8 11h6"/></svg>',
    download: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>',
    stop: '<svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="6" width="12" height="12" rx="1"/></svg>',
    key: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 11-7.778 7.778 5.5 5.5 0 017.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg>',
    copy: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>',
    video: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="23 7 16 12 23 17 23 7"/><rect x="1" y="5" width="15" height="14" rx="2"/></svg>',
    audio: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>',
    subtitle: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="18" rx="2"/><path d="M7 15h4M13 15h4M7 11h10"/></svg>',
    sourceprofiles: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2L3 7v6c0 5.25 3.82 10.15 9 11 5.18-.85 9-5.75 9-11V7l-9-5z"/><path d="M9 12l2 2 4-4"/></svg>',
    logos: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/></svg>',
    tmdb: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M8 12h8M12 8v8"/></svg>',
    hdhr: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="7" width="20" height="10" rx="2"/><path d="M6 11h4M14 11h4M6 14h4"/></svg>',
    invite: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M16 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="8.5" cy="7" r="4"/><line x1="20" y1="8" x2="20" y2="14"/><line x1="23" y1="11" x2="17" y2="11"/></svg>',
    apikey: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 11-7.778 7.778 5.5 5.5 0 017.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg>'
  };

  var router = {
    current: null,
    navigate: function(page, params) {
      this.current = page;
      this.params = params || {};
      history.pushState({ page: page, params: this.params }, '', '#/' + page);
      render();
    },
    init: function() {
      var self = this;
      window.addEventListener('popstate', function(e) {
        if (e.state && e.state.page) {
          self.current = e.state.page;
          self.params = e.state.params || {};
        } else {
          var hash = location.hash.replace('#/', '') || 'dashboard';
          self.current = hash;
          self.params = {};
        }
        render();
      });
      var hash = location.hash.replace('#/', '') || 'dashboard';
      this.current = hash;
      this.params = {};
    }
  };

  var MAX_RETRIES = 5;

  var playerState = {
    hlsInstance: null,
    videoEl: null,
    bufferWatchInterval: null,
    statsInterval: null,
    statsVisible: false,
    currentStreamID: null,
    sessionID: null,
    isLive: false,
    retryCount: 0,
    retryTimeout: null,
    hlsUrl: null,
    delivery: null,

    recordingID: null,

    mseState: null,

    activePlayer: null,
    deliveryOverride: null,
    deliverySwitchable: false,
    sourceType: null,
    signalInfo: null,
    signalInterval: null,

    cleanup: function() {
      if (this.bufferWatchInterval) { clearInterval(this.bufferWatchInterval); this.bufferWatchInterval = null; }
      if (this.statsInterval) { clearInterval(this.statsInterval); this.statsInterval = null; }
      if (this.signalInterval) { clearInterval(this.signalInterval); this.signalInterval = null; }
      this.signalInfo = null;
      this.sourceType = null;
      if (this.retryTimeout) { clearTimeout(this.retryTimeout); this.retryTimeout = null; }
      if (this.activePlayer) { this.activePlayer.stop(); this.activePlayer = null; }
      if (this.recordingID) {
        api.del('/api/recordings/completed/' + this.recordingID + '/play').catch(function() {});
        this.recordingID = null;
      } else if (this.currentStreamID) {
        api.del('/api/play/' + this.currentStreamID).catch(function() {});
      }
      if (this.videoEl) {
        this.videoEl.pause();
        this.videoEl.removeAttribute('src');
        this.videoEl.load();
      }
      this.currentStreamID = null;
      this.videoEl = null;
      this.sessionID = null;
      this.retryCount = 0;
      this.hlsUrl = null;
      this.delivery = null;
    }
  };

  function navItems() {
    var user = api.user;
    var isAdmin = user && user.is_admin;
    var items = [
      { id: 'dashboard', label: 'Dashboard', icon: 'dashboard', adminOnly: true },
      { id: 'activity', label: 'Activity', icon: 'stats', adminOnly: true },
      { section: 'Content' },
      { id: 'channels', label: 'Channels', icon: 'channels' },
      { id: 'library', label: 'Library', icon: 'library' },
      { id: 'guide', label: 'EPG Guide', icon: 'guide' },
      { id: 'recordings', label: 'Recordings', icon: 'recordings' },
      { id: 'favorites', label: 'Favorites', icon: 'favorites' },
      { section: 'Sources', adminOnly: true },
      { id: 'streams', label: 'Streams', icon: 'streams', adminOnly: true },
      { id: 'epgsources', label: 'EPG Sources', icon: 'epg', adminOnly: true },
      { section: 'Stream Management', adminOnly: true },
      { id: 'sourceprofiles', label: 'Source Profiles', icon: 'sourceprofiles', adminOnly: true },
      { id: 'clients', label: 'Clients', icon: 'clients', adminOnly: true },
      { id: 'hdhrdevices', label: 'HDHR Devices', icon: 'hdhr', adminOnly: true },
      { id: 'logos', label: 'Logos', icon: 'logos', adminOnly: true },
      { section: 'System', adminOnly: true },
      { id: 'settings', label: 'Settings', icon: 'settings', adminOnly: true },
      { id: 'users', label: 'Users', icon: 'users', adminOnly: true },
      { id: 'wireguard', label: 'WireGuard', icon: 'wireguard', adminOnly: true },
      { id: 'developer', label: 'Developer', icon: 'probe', adminOnly: true }
    ];
    return items.filter(function(n, i, arr) {
      if (n.adminOnly && !isAdmin) return false;
      if (n.section) {
        var next = null;
        for (var j = i + 1; j < arr.length; j++) {
          if (!arr[j].adminOnly || isAdmin) { next = arr[j]; break; }
        }
        if (!next || next.section) return false;
      }
      return true;
    });
  }

  function renderSidebar() {
    var items = navItems();
    var user = api.user;
    var html = '<div class="sidebar" id="sidebar">';
    html += '<div class="sidebar-header">Media<span>Hub</span></div>';
    html += '<div class="sidebar-nav">';
    for (var i = 0; i < items.length; i++) {
      var it = items[i];
      if (it.section) {
        html += '<div class="nav-section">' + esc(it.section) + '</div>';
        continue;
      }
      var active = router.current === it.id ? ' active' : '';
      html += '<div class="nav-item' + active + '" data-page="' + it.id + '">';
      html += icons[it.icon] || '';
      html += '<span>' + esc(it.label) + '</span></div>';
    }
    html += '</div>';
    html += '<div class="sidebar-footer">';
    html += '<div id="bw-indicator" style="font-size:11px;color:var(--text-muted);margin-bottom:6px"></div>';
    if (user) html += '<span>' + esc(user.username) + '</span> &middot; <span class="logout" id="logout-btn">Logout</span>';
    html += '</div></div>';
    return html;
  }

  function render() {
    var existingBar = document.querySelector('.alpha-bar');
    if (existingBar) existingBar.remove();
    var app = document.getElementById('app');
    if (!api.token) {
      playerState.cleanup();
      app.innerHTML = renderLogin();
      bindLogin();
      return;
    }

    if (router.current === 'login') {
      router.current = (api.user && api.user.is_admin) ? 'dashboard' : 'channels';
    }

    var page = router.current || ((api.user && api.user.is_admin) ? 'dashboard' : 'channels');

    var adminPages = { dashboard: true, activity: true, sources: true, sourceprofiles: true, epgsources: true, streams: true, clients: true, hdhrdevices: true, logos: true, settings: true, users: true, wireguard: true, developer: true, probe: true, playurl: true };
    if (adminPages[page] && !(api.user && api.user.is_admin)) {
      page = 'channels';
      router.current = page;
    }
    if (!favoritesLoaded) loadFavorites();

    if (page === 'player') {
      app.innerHTML = renderSidebar() + '<div class="main" id="page"></div>';
      app.innerHTML += '<button class="mobile-toggle" id="mobile-toggle">' + icons.menu + '</button>';
      bindSidebar();
      renderPlayer();
      return;
    }

    app.innerHTML = renderSidebar() + '<div class="main" id="page"></div>';
    app.innerHTML += '<button class="mobile-toggle" id="mobile-toggle">' + icons.menu + '</button>';
    bindSidebar();
    if (bwEstimate > 0) {
      var bwEl = document.getElementById('bw-indicator');
      if (bwEl) {
        var bwLabel = bwEstimate > 100000 ? (bwEstimate / 1000).toFixed(0) + ' Gbps'
          : bwEstimate > 1000 ? (bwEstimate / 1000).toFixed(1) + ' Mbps'
          : bwEstimate + ' kbps';
        bwEl.textContent = '\u21C5 ' + bwLabel;
        bwEl.style.color = bwEstimate > 50000 ? 'var(--success)' : bwEstimate > 10000 ? 'var(--accent)' : bwEstimate > 3000 ? 'var(--warning)' : 'var(--danger)';
      }
    }

    var pageEl = document.getElementById('page');
    if (!pageEl) return;

    if (page === 'dashboard') renderDashboard(pageEl);
    else if (page === 'streams') renderStreams(pageEl);
    else if (page === 'channels') renderChannels(pageEl);
    else if (page === 'library') renderLibrary(pageEl);
    else if (page === 'guide') renderGuide(pageEl);
    else if (page === 'recordings') renderRecordings(pageEl);
    else if (page === 'favorites') renderFavorites(pageEl);
    else if (page === 'activity') renderActivity(pageEl);
    else if (page === 'sources') renderSources(pageEl);
    else if (page === 'sourceprofiles') renderSourceProfiles(pageEl);
    else if (page === 'epgsources') renderEPGSources(pageEl);
    else if (page === 'wireguard') renderWireGuard(pageEl);
    else if (page === 'settings') renderSettings(pageEl);
    else if (page === 'users') renderUsers(pageEl);
    else if (page === 'clients') renderClients(pageEl);
    else if (page === 'logos') renderLogos(pageEl);
    else if (page === 'probe') renderProbe(pageEl);
    else if (page === 'playurl') renderPlayURL(pageEl);
    else if (page === 'hdhrdevices') renderHDHRDevices(pageEl);
    else if (page === 'apikeys') renderAPIKeys(pageEl);
    else if (page === 'developer') renderDeveloper(pageEl);
    else renderDashboard(pageEl);
  }

  function bindSidebar() {
    var navItems = document.querySelectorAll('.nav-item[data-page]');
    for (var i = 0; i < navItems.length; i++) {
      navItems[i].addEventListener('click', function() {
        closePlayerOverlay();
        router.navigate(this.getAttribute('data-page'));
        var sidebar = document.getElementById('sidebar');
        if (sidebar) sidebar.classList.remove('open');
      });
    }
    var logoutBtn = document.getElementById('logout-btn');
    if (logoutBtn) {
      logoutBtn.addEventListener('click', function() {
        closePlayerOverlay();
        api.token = null;
        api.user = null;
        streamFavorites = {};
        favoritesLoaded = false;
        router.navigate('login');
      });
    }
    var toggle = document.getElementById('mobile-toggle');
    if (toggle) {
      toggle.addEventListener('click', function() {
        var sidebar = document.getElementById('sidebar');
        if (sidebar) sidebar.classList.toggle('open');
      });
    }
  }

  function renderLogin() {
    return '<div class="login-container"><div class="login-card">' +
      '<div class="login-title">Media<span style="color:var(--accent)">Hub</span></div>' +
      '<div class="login-subtitle">Sign in to continue</div>' +
      '<div class="login-error" id="login-error"></div>' +
      '<form id="login-form">' +
      '<div class="form-group"><label class="form-label">Username</label>' +
      '<input class="form-input" id="login-user" type="text" placeholder="admin" autocomplete="username" autofocus></div>' +
      '<div class="form-group"><label class="form-label">Password</label>' +
      '<input class="form-input" id="login-pass" type="password" placeholder="password" autocomplete="current-password"></div>' +
      '<button class="btn btn-primary" style="width:100%;justify-content:center;padding:12px" type="submit">Sign In</button>' +
      '</form>' +
      '<div id="google-oauth-section" style="display:none">' +
      '<div style="display:flex;align-items:center;gap:12px;margin:20px 0"><div style="flex:1;height:1px;background:var(--border)"></div><span style="color:var(--text-muted);font-size:13px">or</span><div style="flex:1;height:1px;background:var(--border)"></div></div>' +
      '<button id="google-signin-btn" style="width:100%;display:flex;align-items:center;justify-content:center;gap:10px;padding:12px 16px;border:1px solid var(--border);border-radius:8px;background:#fff;color:#3c4043;font-size:14px;font-weight:500;cursor:pointer;transition:background 0.15s,box-shadow 0.15s">' +
      '<svg width="18" height="18" viewBox="0 0 48 48"><path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"/><path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"/><path fill="#FBBC05" d="M10.53 28.59a14.5 14.5 0 010-9.18l-7.98-6.19a24.08 24.08 0 000 21.56l7.98-6.19z"/><path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"/></svg>' +
      'Sign in with Google</button>' +
      '</div>' +
      '</div></div>';
  }

  function bindLogin() {
    var form = document.getElementById('login-form');
    if (!form) return;
    form.addEventListener('submit', async function(e) {
      e.preventDefault();
      var errEl = document.getElementById('login-error');
      errEl.style.display = 'none';
      var user = document.getElementById('login-user').value.trim();
      var pass = document.getElementById('login-pass').value;
      if (!user || !pass) {
        errEl.textContent = 'Username and password required';
        errEl.style.display = 'block';
        return;
      }
      try {
        var resp = await api.post('/api/auth/login', { username: user, password: pass });
        if (!resp.ok) {
          var data = await resp.json().catch(function() { return {}; });
          errEl.textContent = data.error || 'Invalid credentials';
          errEl.style.display = 'block';
          return;
        }
        var result = await resp.json();
        api.token = result.access_token;
        var u = api.user;
        router.navigate((u && u.is_admin) ? 'dashboard' : 'channels');
      } catch (err) {
        errEl.textContent = 'Connection failed';
        errEl.style.display = 'block';
      }
    });

    fetch('/api/auth/google').then(function(resp) {
      if (!resp.ok) return;
      return resp.json();
    }).then(function(data) {
      if (!data || !data.url) return;
      var section = document.getElementById('google-oauth-section');
      if (section) section.style.display = 'block';
      var btn = document.getElementById('google-signin-btn');
      if (btn) {
        btn.addEventListener('mouseenter', function() { btn.style.background = '#f8f9fa'; btn.style.boxShadow = '0 1px 3px rgba(0,0,0,0.12)'; });
        btn.addEventListener('mouseleave', function() { btn.style.background = '#fff'; btn.style.boxShadow = 'none'; });
        btn.addEventListener('click', function() { window.location.href = data.url; });
      }
    }).catch(function() {});
  }

  async function renderDashboard(el) {
    var isAdmin = api.user && api.user.is_admin;
    el.innerHTML = '<h1 class="page-title">Dashboard</h1>' +
      '<div class="stat-grid" id="dash-stats">' +
      '<div class="stat-card stat-link" data-page="streams"><div class="stat-value" id="stat-streams">-</div><div class="stat-label">Streams</div></div>' +
      '<div class="stat-card stat-link" data-page="channels"><div class="stat-value" id="stat-channels">-</div><div class="stat-label">Channels</div></div>' +
      (isAdmin ? '<div class="stat-card stat-link" data-page="activity"><div class="stat-value" id="stat-active">-</div><div class="stat-label">Active Now</div></div>' : '') +
      '<div class="stat-card stat-link" data-page="guide"><div class="stat-value" id="stat-epg-programs">-</div><div class="stat-label">EPG Programs</div></div>' +
      '</div>' +
      '<div class="stat-grid" id="dash-stats-row2" style="margin-top:8px">' +
      '<div class="stat-card"><div class="stat-value" id="stat-wg">-</div><div class="stat-label"><span style="display:inline-block;width:14px;height:14px;vertical-align:-2px">' + icons.wireguard + '</span> WireGuard</div></div>' +
      '<div class="stat-card"><div class="stat-value" id="stat-metadata">-</div><div class="stat-label">Metadata</div></div>' +
      '<div class="stat-card"><div class="stat-value" id="stat-uptime">-</div><div class="stat-label">Uptime</div></div>' +
      '</div>' +
      '<div class="dash-section" id="dash-sources-section">' +
      '<div class="dash-section-title">' + icons.sources + ' Sources</div>' +
      '<div class="dash-source-grid" id="dash-sources"><div class="skeleton" style="height:80px"></div></div>' +
      '</div>' +
      '<div class="dash-section" id="dash-epg-section">' +
      '<div class="dash-section-title">' + icons.epg + ' EPG Status</div>' +
      '<div id="dash-epg"><div class="skeleton" style="height:60px"></div></div>' +
      '</div>' +
      '<div style="display:none" id="dash-system-section">' +
      '<div class="dash-section-title">' + icons.settings + ' System</div>' +
      '<div id="dash-system"><div class="skeleton" style="height:60px"></div></div>' +
      '</div>';

    el.querySelectorAll('[data-page]').forEach(function(btn) {
      btn.addEventListener('click', function() { router.navigate(this.getAttribute('data-page')); });
    });

    try {
      await _loadSourceTypes();
      var resp = await api.get('/api/dashboard/stats');
      var stats = await resp.json();

      var sEl = document.getElementById('stat-streams');
      var cEl = document.getElementById('stat-channels');
      var rcEl = document.getElementById('stat-recordings');
      var acEl = document.getElementById('stat-active');
      var epgEl = document.getElementById('stat-epg-programs');

      if (sEl) sEl.textContent = stats.total_streams || 0;
      if (cEl) cEl.textContent = stats.total_channels || 0;
      if (rcEl) rcEl.textContent = stats.recordings ? stats.recordings.total : 0;
      if (acEl) acEl.textContent = stats.active_sessions || 0;
      if (epgEl) epgEl.textContent = stats.epg ? (stats.epg.program_count || 0).toLocaleString() : 0;

      var sourcesEl = document.getElementById('dash-sources');
      if (sourcesEl && stats.sources) {
        var sh = stats.source_health || {};
        var healthBanner = '';
        if (sh.total > 0) {
          var hParts = [];
          if (sh.healthy > 0) hParts.push('<span style="color:var(--success)">' + sh.healthy + ' healthy</span>');
          if (sh.stale > 0) hParts.push('<span style="color:var(--warning)">' + sh.stale + ' stale</span>');
          if (sh.error > 0) hParts.push('<span style="color:var(--danger)">' + sh.error + ' error</span>');
          if (sh.disabled > 0) hParts.push('<span style="color:var(--text-muted)">' + sh.disabled + ' disabled</span>');
          healthBanner = '<div style="display:flex;align-items:center;gap:8px;margin-bottom:12px;padding:10px 16px;background:var(--bg-input);border:1px solid var(--border);border-radius:var(--radius);font-size:13px">' +
            '<span style="font-weight:600;color:#fff">' + sh.total + ' sources:</span> ' +
            hParts.join(' <span style="color:var(--text-muted)">/</span> ') +
            '<a href="#/sources" style="margin-left:auto;font-size:12px;color:var(--accent);cursor:pointer">View all</a>' +
            '</div>';
        }
        var sHtml = healthBanner;
        for (var si = 0; si < stats.sources.length; si++) {
          var src = stats.sources[si];
          var color = _sourceTypeColor(src.type);
          var label = _sourceTypeShortLabel(src.type);
          var healthColor = 'var(--success)';
          var healthLabel = '';
          if (src.health === 'error') { healthColor = 'var(--danger)'; healthLabel = 'Error'; }
          else if (src.health === 'stale') { healthColor = 'var(--warning)'; healthLabel = 'Stale'; }
          else if (src.health === 'disabled') { healthColor = 'var(--text-muted)'; healthLabel = 'Off'; }
          var statusDot = '<span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:' + healthColor + '"></span>';
          var metaParts = [(src.stream_count || 0) + ' streams'];
          if (src.last_refreshed) {
            var rDate = new Date(src.last_refreshed);
            var rMin = Math.floor((Date.now() - rDate.getTime()) / 60000);
            var rAgo = rMin < 1 ? 'just now' : rMin < 60 ? rMin + 'm ago' : rMin < 1440 ? Math.floor(rMin / 60) + 'h ago' : Math.floor(rMin / 1440) + 'd ago';
            metaParts.push('refreshed ' + rAgo);
          }
          if (healthLabel) metaParts.push('<span style="color:' + healthColor + '">' + healthLabel + '</span>');
          sHtml += '<div class="dash-source-card" style="cursor:pointer;border-left:3px solid ' + healthColor + '" data-source-type="' + esc(src.type) + '" data-source-id="' + esc(src.id) + '">' +
            '<div class="dash-source-icon" style="background:' + color + '20;color:' + color + '">' + label + '</div>' +
            '<div class="dash-source-info">' +
            '<div class="dash-source-name">' + statusDot + ' ' + esc(src.name) + '</div>' +
            '<div class="dash-source-meta">' + metaParts.join(' &bull; ') + '</div>' +
            '</div></div>';
        }
        sourcesEl.innerHTML = sHtml || '<div style="color:var(--text-muted)">No sources configured</div>';
        sourcesEl.querySelectorAll('.dash-source-card[data-source-id]').forEach(function(card) {
          card.addEventListener('click', function() {
            router.navigate('streams', { sourceType: this.getAttribute('data-source-type'), sourceId: this.getAttribute('data-source-id') });
          });
        });
      }

      var epgContEl = document.getElementById('dash-epg');
      if (epgContEl) {
        try {
          var epgResp = await api.get('/api/epg/sources');
          var epgSources = await epgResp.json();
          if (!Array.isArray(epgSources) || epgSources.length === 0) {
            epgContEl.innerHTML = '<div style="color:var(--text-muted)">No EPG sources configured</div>';
          } else {
            var epgHtml = '<div class="dash-source-grid">';
            for (var ei = 0; ei < epgSources.length; ei++) {
              var es = epgSources[ei];
              var eAgo = 'Never';
              var eDot = 'var(--text-muted)';
              if (es.last_refreshed) {
                var eDate = new Date(es.last_refreshed);
                var eMin = Math.floor((Date.now() - eDate.getTime()) / 60000);
                eAgo = eMin < 1 ? 'just now' : eMin < 60 ? eMin + 'm ago' : eMin < 1440 ? Math.floor(eMin / 60) + 'h ago' : Math.floor(eMin / 1440) + 'd ago';
                eDot = eMin < 1440 ? 'var(--success)' : eMin < 2880 ? 'var(--warning)' : 'var(--danger)';
              }
              if (es.last_error) { eDot = 'var(--danger)'; eAgo = 'Error'; }
              epgHtml += '<div class="dash-source-card" style="cursor:pointer" onclick="location.hash=\'#/epgsources\'">' +
                '<div class="dash-source-icon" style="background:' + eDot + '20;color:' + eDot + '">' +
                '<span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:' + eDot + '"></span></div>' +
                '<div class="dash-source-info">' +
                '<div class="dash-source-name">' + esc(es.name) + ' <span style="font-weight:400;color:var(--text-muted);font-size:12px">(' + esc(eAgo) + ')</span></div>' +
                '<div class="dash-source-meta">' + (es.channel_count || 0) + ' channels, ' + ((es.program_count || 0)).toLocaleString() + ' programs</div>' +
                '</div></div>';
            }
            epgContEl.innerHTML = epgHtml + '</div>';
          }
        } catch (e) {
          epgContEl.innerHTML = '<div style="color:var(--text-muted)">Could not load EPG sources</div>';
        }
      }

      var recContEl = document.getElementById('dash-recordings');
      if (recContEl && stats.recordings) {
        var rec = stats.recordings;
        var recParts = [];
        if (rec.active > 0) recParts.push('<span class="badge badge-live"><span class="recording-dot" style="width:6px;height:6px;display:inline-block;margin-right:4px"></span>' + rec.active + ' recording</span>');
        if (rec.pending > 0) recParts.push('<span class="badge badge-warning">' + rec.pending + ' pending</span>');
        if (rec.scheduled > 0) recParts.push('<span class="badge badge-warning">' + rec.scheduled + ' scheduled</span>');
        if (rec.completed > 0) recParts.push('<span class="badge badge-enabled">' + rec.completed + ' completed</span>');
        if (rec.failed > 0) recParts.push('<span class="badge badge-danger" style="background:rgba(248,113,113,.15);color:var(--danger)">' + rec.failed + ' failed</span>');
        if (rec.cancelled > 0) recParts.push('<span class="badge" style="background:rgba(139,143,163,.15);color:var(--text-muted)">' + rec.cancelled + ' cancelled</span>');
        recContEl.innerHTML = recParts.length > 0
          ? '<div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center">' + recParts.join('') + '</div>'
          : '<div style="color:var(--text-muted)">No recordings</div>';
      }

      var wgTile = document.getElementById('stat-wg');
      if (wgTile) {
        if (stats.wireguard && stats.wireguard.connected) {
          wgTile.textContent = 'Connected';
          wgTile.style.color = 'var(--success)';
          wgTile.style.fontSize = '16px';
        } else if (stats.wireguard) {
          wgTile.textContent = 'Disconnected';
          wgTile.style.color = 'var(--warning)';
          wgTile.style.fontSize = '16px';
        } else {
          wgTile.textContent = 'N/A';
          wgTile.style.fontSize = '16px';
        }
      }

      var uptimeTile = document.getElementById('stat-uptime');
      if (uptimeTile) {
        var uptimeSec = stats.uptime_seconds || 0;
        if (uptimeSec >= 86400) {
          uptimeTile.textContent = Math.floor(uptimeSec / 86400) + 'd ' + Math.floor((uptimeSec % 86400) / 3600) + 'h';
        } else if (uptimeSec >= 3600) {
          uptimeTile.textContent = Math.floor(uptimeSec / 3600) + 'h ' + Math.floor((uptimeSec % 3600) / 60) + 'm';
        } else {
          uptimeTile.textContent = Math.floor(uptimeSec / 60) + 'm';
        }
      }

      var metaTile = document.getElementById('stat-metadata');
      if (metaTile) {
        try {
          var queueResp = await api.get('/api/tmdb/queue').then(function(r) { return r.json(); }).catch(function() { return {}; });
          var resolved = queueResp.resolved || 0;
          var totalVOD = stats.total_streams || 0;
          var pct = totalVOD > 0 ? Math.round(resolved / totalVOD * 100) : 0;
          metaTile.textContent = resolved.toLocaleString() + ' / ' + totalVOD.toLocaleString() + ' (' + pct + '%)';
          metaTile.style.fontSize = '14px';
          metaTile.closest('.stat-card').querySelector('.stat-label').textContent = 'TMDB Matched';
        } catch (e) {
          metaTile.textContent = '-';
        }
      }
    } catch (e) {}
  }

  async function renderStreams(el) {
    el.innerHTML = '<h1 class="page-title">Streams</h1>' +
      '<div id="stream-source-picker" style="margin-bottom:16px"><div class="skeleton" style="height:40px"></div></div>' +
      '<div id="stream-list"></div>';

    try {
      await _loadSourceTypes();
      var resp = await api.get('/api/sources');
      var sources = await resp.json();
      if (!Array.isArray(sources)) sources = [];
      buildSourcePicker(el, sources);
      if (router.params && router.params.sourceType && router.params.sourceId) {
        loadSourceStreams(router.params.sourceType, router.params.sourceId, router.params.sourceType === 'tvpstreams');
        router.params = {};
      }
    } catch (e) {
      document.getElementById('stream-source-picker').innerHTML =
        '<div class="empty-state">' + icons.empty + '<p>Failed to load sources</p></div>';
    }
  }

  function _navigateToAddSource(sourceType) {
    _showGenericAddSourceModal(sourceType);
  }

  function buildSourcePicker(el, sources) {
    var picker = document.getElementById('stream-source-picker');
    if (!picker) return;

    if (sources.length === 0) {
      picker.innerHTML = '<div style="padding:20px;text-align:center;color:var(--text-muted)">' +
        '<p style="margin-bottom:12px">No sources configured.</p>' +
        '<button class="btn btn-primary" id="stream-add-first-source-btn">' + icons.plus + ' Add your first source</button>' +
        '<div id="stream-add-first-dropdown" style="display:none;position:absolute;left:50%;transform:translateX(-50%);margin-top:4px;background:var(--bg-card);border:1px solid var(--border);border-radius:8px;box-shadow:0 4px 16px rgba(0,0,0,0.2);z-index:100;min-width:200px;overflow:hidden"></div>' +
        '</div>';
      var firstBtn = document.getElementById('stream-add-first-source-btn');
      var firstDd = document.getElementById('stream-add-first-dropdown');
      if (firstBtn && firstDd) {
        var ddHtml = '';
        for (var k = 0; k < _sourceTypes.length; k++) {
          ddHtml += '<div class="stream-add-source-item" data-source-type="' + esc(_sourceTypes[k].type) + '" style="padding:10px 16px;cursor:pointer;white-space:nowrap;transition:background 0.15s">' + esc(_sourceTypes[k].label) + '</div>';
        }
        firstDd.innerHTML = ddHtml;
        firstBtn.addEventListener('click', function(e) {
          e.stopPropagation();
          firstDd.style.display = firstDd.style.display === 'none' ? 'block' : 'none';
        });
        firstDd.addEventListener('click', function(e) {
          var item = e.target.closest('.stream-add-source-item');
          if (!item) return;
          firstDd.style.display = 'none';
          _navigateToAddSource(item.getAttribute('data-source-type'));
        });
        firstDd.querySelectorAll('.stream-add-source-item').forEach(function(item) {
          item.addEventListener('mouseenter', function() { this.style.background = 'var(--bg-hover)'; });
          item.addEventListener('mouseleave', function() { this.style.background = ''; });
        });
        document.addEventListener('click', function _closeFirstDd(e) {
          if (!firstBtn.contains(e.target) && !firstDd.contains(e.target)) {
            firstDd.style.display = 'none';
          }
          if (!document.contains(firstBtn)) document.removeEventListener('click', _closeFirstDd);
        });
      }
      return;
    }

    var html = '<div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center">';
    for (var i = 0; i < sources.length; i++) {
      var src = sources[i];
      var typeBadge = _sourceTypeShortLabel(src.type);
      html += '<button class="btn btn-ghost stream-source-tab" data-source-type="' + esc(src.type) + '" data-source-id="' + esc(src.id) + '" data-source-name="' + esc(src.name) + '">' +
        esc(src.name) + ' <span class="stream-badge" style="font-size:10px">' + typeBadge + '</span>' +
        '<span class="stream-group-count">' + (src.stream_count || 0) + '</span></button>';
    }
    html += '<div style="position:relative;display:inline-block">' +
      '<button class="btn btn-ghost" id="stream-add-source-btn" title="Add Source" style="padding:4px 10px;font-size:18px;font-weight:bold;border:1px dashed var(--border);border-radius:6px;line-height:1">+</button>' +
      '<div id="stream-add-source-dropdown" style="display:none;position:absolute;top:100%;left:0;margin-top:4px;background:var(--bg-card);border:1px solid var(--border);border-radius:8px;box-shadow:0 4px 16px rgba(0,0,0,0.2);z-index:100;min-width:200px;overflow:hidden">' +
      '</div></div>';
    html += '</div>';
    picker.innerHTML = html;

    var addBtn = document.getElementById('stream-add-source-btn');
    var addDd = document.getElementById('stream-add-source-dropdown');
    if (addBtn && addDd) {
      var ddHtml = '';
      for (var k = 0; k < _sourceTypes.length; k++) {
        ddHtml += '<div class="stream-add-source-item" data-source-type="' + esc(_sourceTypes[k].type) + '" style="padding:10px 16px;cursor:pointer;white-space:nowrap;transition:background 0.15s">' + esc(_sourceTypes[k].label) + '</div>';
      }
      ddHtml += '<div style="border-top:1px solid var(--border);margin:4px 0"></div>';
      ddHtml += '<div class="stream-add-source-item" data-action="discover-hdhr" style="padding:10px 16px;cursor:pointer;white-space:nowrap;transition:background 0.15s">Discover HDHomeRun</div>';
      addDd.innerHTML = ddHtml;
      addBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        addDd.style.display = addDd.style.display === 'none' ? 'block' : 'none';
      });
      addDd.addEventListener('click', function(e) {
        var item = e.target.closest('.stream-add-source-item');
        if (!item) return;
        addDd.style.display = 'none';
        if (item.getAttribute('data-action') === 'discover-hdhr') {
          window._autoDiscoverHDHR = true;
          router.navigate('sources');
          return;
        }
        _navigateToAddSource(item.getAttribute('data-source-type'));
      });
      addDd.querySelectorAll('.stream-add-source-item').forEach(function(item) {
        item.addEventListener('mouseenter', function() { this.style.background = 'var(--bg-hover)'; });
        item.addEventListener('mouseleave', function() { this.style.background = ''; });
      });
      document.addEventListener('click', function _closeAddDd(e) {
        if (!addBtn.contains(e.target) && !addDd.contains(e.target)) {
          addDd.style.display = 'none';
        }
        if (!document.contains(addBtn)) document.removeEventListener('click', _closeAddDd);
      });
    }

    picker.addEventListener('click', function(e) {
      if (e.target.closest('#stream-add-source-btn') || e.target.closest('#stream-add-source-dropdown')) return;
      var btn = e.target.closest('.stream-source-tab');
      if (!btn) return;
      var tabs = picker.querySelectorAll('.stream-source-tab');
      for (var t = 0; t < tabs.length; t++) tabs[t].classList.remove('active');
      btn.classList.add('active');
      var sourceType = btn.dataset.sourceType;
      var sourceId = btn.dataset.sourceId;
      var isTvp = sourceType === 'tvpstreams';
      loadSourceStreams(sourceType, sourceId, isTvp);
    });

    if (sources.length === 1) {
      picker.querySelector('.stream-source-tab').click();
    }
  }

  function buildSourceActionToolbar() {
    var toolbar = document.createElement('div');
    toolbar.style.cssText = 'display:flex;align-items:center;gap:4px;';
    var btnStyle = 'font-size:11px;padding:3px 8px;border-radius:4px;border:1px solid var(--border);background:var(--bg-input);color:var(--text-muted);cursor:pointer;transition:opacity 0.15s;opacity:0.8;';

    var editBtn = document.createElement('button');
    editBtn.textContent = 'Edit';
    editBtn.title = 'Edit source';
    editBtn.style.cssText = btnStyle + 'color:#4fc3f7;border-color:#4fc3f7;';
    editBtn.addEventListener('mouseenter', function() { this.style.opacity = '1'; this.style.background = 'rgba(79,195,247,0.15)'; });
    editBtn.addEventListener('mouseleave', function() { this.style.opacity = '0.8'; this.style.background = 'var(--bg-input)'; });
    editBtn.addEventListener('click', function() {
      var picker = document.getElementById('stream-source-picker');
      if (!picker) return;
      var activeTab = picker.querySelector('.stream-source-tab.active');
      if (!activeTab) return;
      var id = activeTab.dataset.sourceId;
      var type = activeTab.dataset.sourceType;
      var name = activeTab.dataset.sourceName;
      window._autoEditSource = { id: id, type: type, name: name };
      router.navigate('sources');
    });

    var refreshBtn = document.createElement('button');
    refreshBtn.textContent = 'Refresh';
    refreshBtn.title = 'Refresh source';
    refreshBtn.style.cssText = btnStyle + 'color:#66bb6a;border-color:#66bb6a;';
    refreshBtn.addEventListener('mouseenter', function() { this.style.opacity = '1'; this.style.background = 'rgba(102,187,106,0.15)'; });
    refreshBtn.addEventListener('mouseleave', function() { this.style.opacity = '0.8'; this.style.background = 'var(--bg-input)'; });
    refreshBtn.addEventListener('click', function() {
      var picker = document.getElementById('stream-source-picker');
      if (!picker) return;
      var activeTab = picker.querySelector('.stream-source-tab.active');
      if (!activeTab) return;
      var id = activeTab.dataset.sourceId;
      var type = activeTab.dataset.sourceType;
      api.post('/api/sources/' + id + '/refresh', {}).then(function(r) {
        if (r.ok) {
          toast('Source refresh started');
        } else {
          toast('Failed to refresh source', 'error');
        }
      }).catch(function() {
        toast('Failed to refresh source', 'error');
      });
      var cacheKey = 'streams_' + type + '_' + id;
      try { sessionStorage.removeItem(cacheKey); sessionStorage.removeItem(cacheKey + '_ts'); } catch (ex) {}
      setTimeout(function() {
        loadSourceStreams(type, id, type === 'tvpstreams');
      }, 3000);
    });

    var deleteBtn = document.createElement('button');
    deleteBtn.textContent = 'Delete';
    deleteBtn.title = 'Delete source';
    deleteBtn.style.cssText = btnStyle + 'color:var(--danger);border-color:var(--danger);';
    deleteBtn.addEventListener('mouseenter', function() { this.style.opacity = '1'; this.style.background = 'rgba(244,67,54,0.15)'; });
    deleteBtn.addEventListener('mouseleave', function() { this.style.opacity = '0.8'; this.style.background = 'var(--bg-input)'; });
    deleteBtn.addEventListener('click', async function() {
      var picker = document.getElementById('stream-source-picker');
      if (!picker) return;
      var activeTab = picker.querySelector('.stream-source-tab.active');
      if (!activeTab) return;
      var id = activeTab.dataset.sourceId;
      var type = activeTab.dataset.sourceType;
      var name = activeTab.dataset.sourceName;
      if (!confirm('Delete source "' + name + '"? All its streams will be removed.')) return;
      try {
        var r = await api.del('/api/sources/' + type + '/' + id);
        if (!r.ok && r.status !== 204) {
          r = await api.del('/api/source-plugins/' + type + '/' + id);
        }
        if (r.ok || r.status === 204) {
          toast('Source deleted');
          activeTab.remove();
          var remaining = picker.querySelectorAll('.stream-source-tab');
          if (remaining.length > 0) {
            remaining[0].click();
          } else {
            var container = document.getElementById('stream-list');
            if (container) container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No sources configured.</p></div>';
          }
        } else {
          toast('Failed to delete source', 'error');
        }
      } catch (err) {
        toast('Failed to delete source', 'error');
      }
    });

    var scanBtn = document.createElement('button');
    scanBtn.textContent = 'Scan';
    scanBtn.title = 'Scan for channels';
    scanBtn.style.cssText = btnStyle + 'color:#ffa726;border-color:#ffa726;';
    scanBtn.style.display = 'none';
    scanBtn.addEventListener('mouseenter', function() { this.style.opacity = '1'; this.style.background = 'rgba(255,167,38,0.15)'; });
    scanBtn.addEventListener('mouseleave', function() { this.style.opacity = '0.8'; this.style.background = 'var(--bg-input)'; });
    scanBtn.addEventListener('click', function() {
      var picker = document.getElementById('stream-source-picker');
      if (!picker) return;
      var activeTab = picker.querySelector('.stream-source-tab.active');
      if (!activeTab) return;
      var id = activeTab.dataset.sourceId;
      var type = activeTab.dataset.sourceType;
      if (type !== 'hdhr' && type !== 'satip') return;
      api.post('/api/sources/' + type + '/' + id + '/scan').then(function(r) {
        if (r.ok) {
          toast('Scan started');
        } else {
          toast('Scan failed', 'error');
        }
      }).catch(function() {
        toast('Scan failed', 'error');
      });
      var cacheKey = 'streams_' + type + '_' + id;
      try { sessionStorage.removeItem(cacheKey); sessionStorage.removeItem(cacheKey + '_ts'); } catch (ex) {}
      setTimeout(function() {
        loadSourceStreams(type, id, type === 'tvpstreams');
      }, 5000);
    });

    var retuneBtn = document.createElement('button');
    retuneBtn.textContent = 'Retune';
    retuneBtn.title = 'Retune HDHR device';
    retuneBtn.style.cssText = btnStyle + 'color:#ce93d8;border-color:#ce93d8;';
    retuneBtn.style.display = 'none';
    retuneBtn.addEventListener('mouseenter', function() { this.style.opacity = '1'; this.style.background = 'rgba(206,147,216,0.15)'; });
    retuneBtn.addEventListener('mouseleave', function() { this.style.opacity = '0.8'; this.style.background = 'var(--bg-input)'; });
    retuneBtn.addEventListener('click', function() {
      var picker = document.getElementById('stream-source-picker');
      if (!picker) return;
      var activeTab = picker.querySelector('.stream-source-tab.active');
      if (!activeTab) return;
      var id = activeTab.dataset.sourceId;
      if (!confirm('Retune this HDHR source?')) return;
      api.post('/api/sources/hdhr/' + id + '/retune').then(function(r) {
        if (r.ok) {
          toast('Retune started');
        } else {
          toast('Retune failed', 'error');
        }
      }).catch(function() {
        toast('Retune failed', 'error');
      });
    });

    var clearBtn = document.createElement('button');
    clearBtn.textContent = 'Clear';
    clearBtn.title = 'Clear all streams';
    clearBtn.style.cssText = btnStyle + 'color:#ff9800;border-color:#ff9800;';
    clearBtn.style.display = 'none';
    clearBtn.addEventListener('mouseenter', function() { this.style.opacity = '1'; this.style.background = 'rgba(255,152,0,0.15)'; });
    clearBtn.addEventListener('mouseleave', function() { this.style.opacity = '0.8'; this.style.background = 'var(--bg-input)'; });
    clearBtn.addEventListener('click', function() {
      var picker = document.getElementById('stream-source-picker');
      if (!picker) return;
      var activeTab = picker.querySelector('.stream-source-tab.active');
      if (!activeTab) return;
      var id = activeTab.dataset.sourceId;
      var type = activeTab.dataset.sourceType;
      if (type !== 'hdhr' && type !== 'satip') return;
      if (!confirm('Clear all streams for this source?')) return;
      api.post('/api/sources/' + type + '/' + id + '/clear').then(function(r) {
        if (r.ok) {
          toast('Streams cleared');
          var cacheKey = 'streams_' + type + '_' + id;
          try { sessionStorage.removeItem(cacheKey); sessionStorage.removeItem(cacheKey + '_ts'); } catch (ex) {}
          setTimeout(function() { loadSourceStreams(type, id, false); }, 1000);
        } else {
          toast('Clear failed', 'error');
        }
      }).catch(function() {
        toast('Clear failed', 'error');
      });
    });

    var activeTab = document.querySelector('.stream-source-tab.active');
    var sourceType = activeTab ? activeTab.dataset.sourceType : '';
    if (sourceType === 'hdhr' || sourceType === 'satip') {
      scanBtn.style.display = '';
      clearBtn.style.display = '';
    }
    if (sourceType === 'hdhr') {
      retuneBtn.style.display = '';
    }

    toolbar.appendChild(editBtn);
    toolbar.appendChild(refreshBtn);
    toolbar.appendChild(scanBtn);
    toolbar.appendChild(retuneBtn);
    toolbar.appendChild(clearBtn);
    toolbar.appendChild(deleteBtn);
    return toolbar;
  }

  async function loadSourceStreams(sourceType, sourceId, isTvpStreams) {
    var container = document.getElementById('stream-list');
    if (!container) return;
    container.innerHTML = '<div class="skeleton" style="height:200px"></div>';

    try {
      var cacheKey = 'streams_' + sourceType + '_' + sourceId;
      var streams = null;
      var CACHE_TTL = 5 * 60 * 1000;
      try {
        var tsStr = sessionStorage.getItem(cacheKey + '_ts');
        if (tsStr && (Date.now() - parseInt(tsStr, 10)) < CACHE_TTL) {
          var cached = sessionStorage.getItem(cacheKey);
          if (cached) streams = JSON.parse(cached);
        }
      } catch (ex) {}

      if (!streams) {
        var resp = await api.get('/api/streams?source_type=' + encodeURIComponent(sourceType) + '&source_id=' + encodeURIComponent(sourceId));
        streams = await resp.json();
        if (!Array.isArray(streams)) streams = [];
        try {
          sessionStorage.setItem(cacheKey, JSON.stringify(streams));
          sessionStorage.setItem(cacheKey + '_ts', String(Date.now()));
        } catch (ex) {}
      }

      if (isTvpStreams) {
        buildTvpStreamGroups(container, streams);
      } else {
        buildLiveStreamGroups(container, streams);
      }
    } catch (e) {
      container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load streams</p></div>';
    }
  }

  var streamFavorites = {};
  var favoritesLoaded = false;

  async function loadFavorites(force) {
    if (favoritesLoaded && !force) return;
    try {
      var resp = await api.get('/api/favorites');
      var favs = await resp.json();
      streamFavorites = {};
      if (Array.isArray(favs)) {
        for (var i = 0; i < favs.length; i++) {
          streamFavorites[favs[i].stream_id] = true;
        }
      }
      favoritesLoaded = true;
    } catch (e) {
      streamFavorites = {};
    }
  }

  async function toggleFavorite(streamID) {
    if (streamFavorites[streamID]) {
      await api.del('/api/favorites/' + streamID);
      delete streamFavorites[streamID];
    } else {
      await api.post('/api/favorites', { stream_id: streamID });
      streamFavorites[streamID] = true;
    }
  }

  function streamLogoHtml(s) {
    var logoUrl = s.tvg_logo || '';
    if (!logoUrl) return '';
    return '<img class="stream-group-logo" src="' + esc(logoUrl) + '" loading="lazy" alt="">';
  }

  function streamBadgesHtml(s) {
    var badges = '';
    if (s.video_codec) badges += '<span class="stream-badge stream-badge-codec">' + esc(s.video_codec) + '</span>';
    if (s.height) {
      var label = s.height >= 2160 ? '4K' : s.height >= 1080 ? '1080p' : s.height >= 720 ? '720p' : s.height + 'p';
      badges += '<span class="stream-badge stream-badge-res">' + label + '</span>';
    }
    if (s.audio_codec) badges += '<span class="stream-badge stream-badge-codec">' + esc(s.audio_codec) + '</span>';
    if (s.tags && s.tags.length) {
      for (var ti = 0; ti < s.tags.length; ti++) {
        badges += '<span class="stream-badge stream-badge-tag">' + esc(s.tags[ti]) + '</span>';
      }
    }
    return badges;
  }

  function streamIsLive(s) {
    return !s.vod_type;
  }

  function buildStreamRow(s) {
    var logo = streamLogoHtml(s);
    var badges = streamBadgesHtml(s);
    var isFav = streamFavorites[s.id];
    var favClass = isFav ? ' favorited' : '';
    var favChar = isFav ? '\u2B50' : '\u2606';
    var favColor = isFav ? '#eab308' : 'var(--text-muted)';
    var displayName = s.name;
    if (s.vod_type === 'episode' || (s.season && s.episode)) {
      var se = 'S' + String(s.season || 0).padStart(2, '0') + 'E' + String(s.episode || 0).padStart(2, '0');
      displayName = se + (s.episode_name ? ' - ' + s.episode_name : ' - ' + s.name);
    }
    var groupLabel = s.group ? ' <span style="color:var(--text-muted);font-size:12px">' + esc(s.group) + '</span>' : '';
    var encLabel = s.encrypted ? ' <span style="background:#ef4444;color:#fff;font-size:10px;padding:1px 5px;border-radius:3px;font-weight:600">ENC</span>' : '';
    return '<tr>' +
      '<td>' + logo + '</td>' +
      '<td>' + esc(displayName) + (badges || '') + encLabel + groupLabel + '</td>' +
      '<td style="width:100px"><div style="display:flex;gap:4px;justify-content:flex-end">' +
      '<button class="stream-fav-btn' + favClass + '" data-fav="1" data-sid="' + esc(s.id) + '" style="color:' + favColor + '">' + favChar + '</button>' +
      '<button class="stream-add-btn" data-qadd="1" data-sid="' + esc(s.id) + '" data-sname="' + esc(s.name) + '" title="Add to channel">+</button>' +
      '<button class="stream-play-btn" data-sid="' + esc(s.id) + '" data-sname="' + esc(s.name) + '" data-live="' + (streamIsLive(s) ? '1' : '0') + '" title="Play">\u25B6</button>' +
      '</div></td></tr>';
  }

  function buildSeriesRows(streams) {
    var rows = [];
    var seasons = {};
    for (var i = 0; i < streams.length; i++) {
      var s = streams[i];
      var sn = s.season || 0;
      if (!seasons[sn]) seasons[sn] = [];
      seasons[sn].push(s);
    }
    var seasonNums = Object.keys(seasons).map(Number).sort(function(a, b) { return a - b; });
    for (var si = 0; si < seasonNums.length; si++) {
      var num = seasonNums[si];
      var eps = seasons[num];
      eps.sort(function(a, b) { return (a.episode || 0) - (b.episode || 0); });
      if (seasonNums.length > 1 || num > 0) {
        rows.push('<tr><td colspan="3" class="series-season-header">Season ' + num + '</td></tr>');
      }
      for (var ei = 0; ei < eps.length; ei++) {
        rows.push(buildStreamRow(eps[ei]));
      }
    }
    return rows;
  }

  function bindStreamGroupEvents(container) {
    container.addEventListener('toggle', function(e) {
      var details = e.target;
      if (!details.open || details.tagName !== 'DETAILS') return;
      if (details.dataset.loaded) return;
      details.dataset.loaded = '1';

      var groupKey = details.dataset.group;
      var section = details.dataset.section;
      var maps = container._groupMaps || {};
      var sectionMap = maps[section] || {};
      var streamData = sectionMap[groupKey] || [];
      var filterTerm = container._searchTerm || '';
      if (filterTerm) {
        streamData = streamData.filter(function(s) {
          return (s.name || '').toLowerCase().indexOf(filterTerm) >= 0 ||
                 (s.group || '').toLowerCase().indexOf(filterTerm) >= 0;
        });
      }
      var tableEl = document.createElement('table');
      tableEl.className = 'stream-group-table';

      if (section === 'movies') {
        streamData.sort(function(a, b) {
          if ((a.year || '') !== (b.year || '')) return (b.year || '').localeCompare(a.year || '');
          return a.name.localeCompare(b.name);
        });
        var rows = [];
        for (var mi = 0; mi < streamData.length; mi++) rows.push(buildStreamRow(streamData[mi]));
        tableEl.innerHTML = '<tbody>' + rows.join('') + '</tbody>';
      } else if (section === 'series') {
        tableEl.innerHTML = '<tbody>' + buildSeriesRows(streamData).join('') + '</tbody>';
      } else {
        streamData.sort(function(a, b) { return a.name.localeCompare(b.name); });
        var lRows = [];
        for (var li = 0; li < streamData.length; li++) lRows.push(buildStreamRow(streamData[li]));
        tableEl.innerHTML = '<tbody>' + lRows.join('') + '</tbody>';
      }
      details.appendChild(tableEl);
    }, true);

    container.addEventListener('click', function(e) {
      var btn = e.target.closest('button[data-sid]');
      if (!btn) return;
      e.stopPropagation();
      if (btn.dataset.fav) {
        var sid = btn.dataset.sid;
        toggleFavorite(sid).then(function() {
          var nowFav = streamFavorites[sid];
          btn.textContent = nowFav ? '\u2B50' : '\u2606';
          btn.style.color = nowFav ? '#eab308' : 'var(--text-muted)';
          if (nowFav) btn.classList.add('favorited');
          else btn.classList.remove('favorited');
        }).catch(function() { toast('Failed to update favorite', 'error'); });
        return;
      }
      if (btn.dataset.qadd) {
        showAddToChannelModal(btn.dataset.sid, btn.dataset.sname);
        return;
      }
      var isLive = btn.dataset.live !== '0';
      startPlay(btn.dataset.sid, btn.dataset.sname, isLive);
    });
  }

  function buildTvpStreamGroups(container, allStreams) {
    var movies = [];
    var movieGroups = {};
    var seriesGroups = {};

    for (var i = 0; i < allStreams.length; i++) {
      var s = allStreams[i];
      var classified = s.vod_type;
      if (!classified) {
        classified = (s.season > 0 || s.episode > 0) ? 'series' : 'movie';
      }
      if (classified === 'movie') {
        movies.push(s);
        var mg = s.group || '(Ungrouped)';
        if (!movieGroups[mg]) movieGroups[mg] = [];
        movieGroups[mg].push(s);
      } else if (classified === 'series' || classified === 'episode') {
        var sg = s.group || s.name || '(Unknown Series)';
        if (!seriesGroups[sg]) seriesGroups[sg] = [];
        seriesGroups[sg].push(s);
      } else {
        movies.push(s);
        var lg = s.group || '(Ungrouped)';
        if (!movieGroups[lg]) movieGroups[lg] = [];
        movieGroups[lg].push(s);
      }
    }

    var searchTerm = '';
    var searchTimer = null;
    var activeTab = 'movies';

    var summaryEl = document.createElement('h3');
    var tabBar = document.createElement('div');
    tabBar.style.cssText = 'display:flex;gap:4px;margin-bottom:12px';
    tabBar.innerHTML =
      '<button class="btn btn-primary stream-tvp-tab" data-tab="movies">Movies (' + movies.length + ')</button>' +
      '<button class="btn btn-ghost stream-tvp-tab" data-tab="series">TV Series (' + Object.keys(seriesGroups).length + ')</button>';

    var searchInput = document.createElement('input');
    searchInput.type = 'text';
    searchInput.placeholder = 'Search streams...';
    searchInput.className = 'form-input';
    searchInput.style.cssText = 'min-width:200px;max-width:320px;padding:6px 10px;font-size:13px;';

    var groupsContainer = document.createElement('div');

    function renderMoviesTab() {
      var html = [];
      var visibleCount = 0;
      var groupKeys = Object.keys(movieGroups).sort();
      for (var gi = 0; gi < groupKeys.length; gi++) {
        var gk = groupKeys[gi];
        var display = gk.replace(/^(TV|Movie)\|/, '');
        var items = movieGroups[gk];
        if (searchTerm) {
          var groupMatch = display.toLowerCase().indexOf(searchTerm) >= 0;
          if (!groupMatch) {
            var matchCount = 0;
            for (var mi = 0; mi < items.length; mi++) {
              if ((items[mi].name || '').toLowerCase().indexOf(searchTerm) >= 0) matchCount++;
            }
            if (matchCount === 0) continue;
            visibleCount += matchCount;
            html.push('<details class="stream-group" data-section="movies" data-group="' + esc(gk) + '" open><summary>' +
              esc(display) + '<span class="stream-group-count">' + matchCount + ' / ' + items.length + '</span></summary></details>');
            continue;
          }
        }
        visibleCount += items.length;
        html.push('<details class="stream-group" data-section="movies" data-group="' + esc(gk) + '"><summary>' +
          esc(display) + '<span class="stream-group-count">' + items.length + '</span></summary></details>');
      }
      summaryEl.textContent = visibleCount.toLocaleString() + ' movies in ' + html.length + ' group' + (html.length !== 1 ? 's' : '');
      groupsContainer.innerHTML = html.length > 0 ? html.join('') :
        '<div style="padding:40px 16px;text-align:center;color:var(--text-muted)">' +
        (searchTerm ? 'No movies match "' + esc(searchInput.value) + '"' : 'No movies found') + '</div>';
    }

    function renderSeriesTab() {
      var html = [];
      var visibleCount = 0;
      var seriesKeys = Object.keys(seriesGroups).sort();
      for (var si = 0; si < seriesKeys.length; si++) {
        var sk = seriesKeys[si];
        var display = sk.replace(/^(TV|Movie)\|/, '');
        var items = seriesGroups[sk];
        if (searchTerm) {
          var groupMatch = display.toLowerCase().indexOf(searchTerm) >= 0;
          if (!groupMatch) {
            var matchCount = 0;
            for (var mi = 0; mi < items.length; mi++) {
              if ((items[mi].name || '').toLowerCase().indexOf(searchTerm) >= 0) matchCount++;
            }
            if (matchCount === 0) continue;
            visibleCount += matchCount;
            html.push('<details class="stream-group" data-section="series" data-group="' + esc(sk) + '" open><summary>' +
              esc(display) + ' <span class="stream-badge" style="background:rgba(52,211,153,.15);color:var(--success);font-size:10px;margin-left:4px">SERIES</span>' +
              '<span class="stream-group-count">' + matchCount + ' / ' + items.length + '</span></summary></details>');
            continue;
          }
        }
        visibleCount += items.length;
        html.push('<details class="stream-group" data-section="series" data-group="' + esc(sk) + '"><summary>' +
          esc(display) + ' <span class="stream-badge" style="background:rgba(52,211,153,.15);color:var(--success);font-size:10px;margin-left:4px">SERIES</span>' +
          '<span class="stream-group-count">' + items.length + '</span></summary></details>');
      }
      summaryEl.textContent = visibleCount.toLocaleString() + ' episodes in ' + html.length + ' series';
      groupsContainer.innerHTML = html.length > 0 ? html.join('') :
        '<div style="padding:40px 16px;text-align:center;color:var(--text-muted)">' +
        (searchTerm ? 'No series match "' + esc(searchInput.value) + '"' : 'No TV series found') + '</div>';
    }

    function renderActiveTab() {
      if (activeTab === 'movies') renderMoviesTab();
      else renderSeriesTab();
    }

    tabBar.addEventListener('click', function(e) {
      var btn = e.target.closest('.stream-tvp-tab');
      if (!btn) return;
      activeTab = btn.dataset.tab;
      var tabs = tabBar.querySelectorAll('.stream-tvp-tab');
      for (var t = 0; t < tabs.length; t++) {
        tabs[t].className = 'btn ' + (tabs[t].dataset.tab === activeTab ? 'btn-primary' : 'btn-ghost') + ' stream-tvp-tab';
      }
      searchTerm = '';
      searchInput.value = '';
      renderActiveTab();
    });

    searchInput.addEventListener('input', function() {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(function() {
        searchTerm = searchInput.value.toLowerCase();
        groupsContainer._searchTerm = searchTerm;
        renderActiveTab();
        searchInput.focus();
      }, 300);
    });

    groupsContainer._groupMaps = { movies: movieGroups, series: seriesGroups };
    groupsContainer._searchTerm = '';
    bindStreamGroupEvents(groupsContainer);

    container.innerHTML = '';
    container.appendChild(tabBar);
    var headerDiv = document.createElement('div');
    headerDiv.className = 'stream-groups-header';
    headerDiv.appendChild(summaryEl);
    var toolbar = buildSourceActionToolbar();
    headerDiv.appendChild(toolbar);
    var searchWrap = document.createElement('div');
    searchWrap.style.cssText = 'display:flex;align-items:center;gap:8px;';
    searchWrap.appendChild(searchInput);
    headerDiv.appendChild(searchWrap);
    container.appendChild(headerDiv);
    container.appendChild(groupsContainer);

    renderActiveTab();
  }

  function buildLiveStreamGroups(container, allStreams) {
    var liveGroups = {};

    for (var i = 0; i < allStreams.length; i++) {
      var s = allStreams[i];
      var lg = s.group || '(No Group)';
      if (!liveGroups[lg]) liveGroups[lg] = [];
      liveGroups[lg].push(s);
    }

    var searchTerm = '';
    var searchTimer = null;
    var summaryEl = document.createElement('h3');
    var groupsContainer = document.createElement('div');

    var searchInput = document.createElement('input');
    searchInput.type = 'text';
    searchInput.placeholder = 'Search streams...';
    searchInput.className = 'form-input';
    searchInput.style.cssText = 'min-width:200px;max-width:320px;padding:6px 10px;font-size:13px;';

    function renderGroups() {
      var html = [];
      var visibleCount = 0;

      var liveKeys = Object.keys(liveGroups).sort(function(a, b) {
        if (a === '(No Group)') return 1;
        if (b === '(No Group)') return -1;
        return a.localeCompare(b);
      });
      for (var li = 0; li < liveKeys.length; li++) {
        var lk = liveKeys[li];
        var lDisplay = lk.replace(/^(TV|Movie)\|/, '');
        var items = liveGroups[lk];
        if (searchTerm) {
          var groupMatch = lDisplay.toLowerCase().indexOf(searchTerm) >= 0;
          if (!groupMatch) {
            var matchingStreams = 0;
            for (var si = 0; si < items.length; si++) {
              if ((items[si].name || '').toLowerCase().indexOf(searchTerm) >= 0) matchingStreams++;
            }
            if (matchingStreams === 0) continue;
            visibleCount += matchingStreams;
            html.push('<details class="stream-group" data-section="live" data-group="' + esc(lk) + '" open><summary>' +
              esc(lDisplay) + '<span class="stream-group-count">' + matchingStreams + ' / ' + items.length + '</span></summary></details>');
            continue;
          }
        }
        visibleCount += items.length;
        html.push('<details class="stream-group" data-section="live" data-group="' + esc(lk) + '"><summary>' +
          esc(lDisplay) + '<span class="stream-group-count">' + items.length + '</span></summary></details>');
      }

      summaryEl.textContent = visibleCount.toLocaleString() + ' streams in ' + html.length + ' group' + (html.length !== 1 ? 's' : '');
      if (html.length === 0) {
        groupsContainer.innerHTML = '<div style="padding:40px 16px;text-align:center;color:var(--text-muted)">' +
          (searchTerm ? 'No streams match "' + esc(searchInput.value) + '"' : 'No streams found') + '</div>';
        return;
      }
      groupsContainer.innerHTML = html.join('');
    }

    searchInput.addEventListener('input', function() {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(function() {
        searchTerm = searchInput.value.toLowerCase();
        groupsContainer._searchTerm = searchTerm;
        renderGroups();
        searchInput.focus();
      }, 300);
    });

    groupsContainer._groupMaps = { live: liveGroups };
    groupsContainer._searchTerm = '';
    bindStreamGroupEvents(groupsContainer);

    container.innerHTML = '';
    var headerDiv = document.createElement('div');
    headerDiv.className = 'stream-groups-header';
    headerDiv.appendChild(summaryEl);
    var toolbar = buildSourceActionToolbar();
    headerDiv.appendChild(toolbar);
    var searchWrap = document.createElement('div');
    searchWrap.style.cssText = 'display:flex;align-items:center;gap:8px;';
    searchWrap.appendChild(searchInput);
    headerDiv.appendChild(searchWrap);
    container.appendChild(headerDiv);
    container.appendChild(groupsContainer);

    renderGroups();
  }

  function matchStream(s, term) {
    return (s.name || '').toLowerCase().indexOf(term) >= 0 ||
           (s.group || '').toLowerCase().indexOf(term) >= 0 ||
           (s.url || '').toLowerCase().indexOf(term) >= 0;
  }

  async function showAddToChannelModal(streamID, streamName) {
    var existing = document.getElementById('add-channel-modal');
    if (existing) existing.remove();

    var channels = [];
    try {
      var resp = await api.get('/api/channels');
      channels = await resp.json();
      if (!Array.isArray(channels)) channels = [];
    } catch (e) { channels = []; }

    var html = '<div class="modal-overlay" id="add-channel-modal">' +
      '<div class="modal-content" style="max-width:480px">' +
      '<div class="modal-header"><span>Add "' + esc(streamName) + '" to Channel</span><button class="modal-close-x addch-close-x" title="Close">\u2715</button></div>' +
      '<div class="modal-body">' +
      '<button class="btn btn-primary" id="create-new-ch-btn" style="width:100%;margin-bottom:16px;padding:10px">+ Create New Channel with this Stream</button>';

    if (channels.length > 0) {
      html += '<div style="color:var(--text-muted);font-size:12px;margin-bottom:8px;text-transform:uppercase;letter-spacing:0.5px">Or add to existing channel</div>';
      html += '<input class="form-input" id="channel-pick-search" placeholder="Search channels..." style="margin-bottom:8px">';
      html += '<div class="channel-pick-list" id="channel-pick-list">';
      for (var i = 0; i < channels.length; i++) {
        var ch = channels[i];
        var alreadyAssigned = (ch.stream_ids || []).indexOf(streamID) >= 0;
        var badge = alreadyAssigned ? ' <span class="badge badge-live">assigned</span>' : '';
        var logoImg = ch.logo_url ? '<img src="' + esc(ch.logo_url) + '" style="width:24px;height:24px;object-fit:contain;border-radius:2px;flex-shrink:0" onerror="this.style.display=\'none\'">' : '';
        html += '<div class="channel-pick-item' + (alreadyAssigned ? ' disabled' : '') + '" data-channel-id="' + esc(ch.id) + '" data-name="' + esc(ch.name) + '"' +
          (alreadyAssigned ? '' : ' style="cursor:pointer"') + '>' +
          logoImg + '<span style="flex:1">' + esc(ch.name) + ' <span style="color:var(--text-muted)">(#' + ch.number + ')</span></span>' + badge + '</div>';
      }
      html += '</div>';
    }

    html += '</div><div class="modal-footer">' +
      '<button class="btn btn-ghost" id="close-channel-modal">Cancel</button>' +
      '</div></div></div>';

    document.body.insertAdjacentHTML('beforeend', html);

    document.getElementById('create-new-ch-btn').addEventListener('click', function() {
      document.getElementById('add-channel-modal').remove();
      openStandaloneChannelModal({ preStreamID: streamID, preStreamName: streamName });
    });

    var pickSearch = document.getElementById('channel-pick-search');
    if (pickSearch) {
      pickSearch.addEventListener('input', function() {
        var q = this.value.toLowerCase();
        document.querySelectorAll('.channel-pick-item').forEach(function(item) {
          var name = (item.getAttribute('data-name') || '').toLowerCase();
          item.style.display = name.indexOf(q) >= 0 ? '' : 'none';
        });
      });
    }

    document.getElementById('close-channel-modal').addEventListener('click', function() {
      document.getElementById('add-channel-modal').remove();
    });
    document.querySelector('.addch-close-x').addEventListener('click', function() {
      document.getElementById('add-channel-modal').remove();
    });
    document.getElementById('add-channel-modal').addEventListener('click', function(e) {
      if (e.target === this) this.remove();
    });

    document.querySelectorAll('.channel-pick-item:not(.disabled)').forEach(function(item) {
      item.addEventListener('click', function() {
        var channelID = this.getAttribute('data-channel-id');
        var ch = channels.find(function(c) { return c.id === channelID; });
        if (!ch) return;
        var ids = (ch.stream_ids || []).slice();
        ids.push(streamID);
        api.post('/api/channels/' + channelID + '/streams', { stream_ids: ids }).then(function() {
          toast('Stream added to channel');
          document.getElementById('add-channel-modal').remove();
        }).catch(function() {
          toast('Failed to assign stream', 'error');
        });
      });
    });
  }

  var channelGroups = [];
  var channelStreams = null;
  var epgChannelIDs = null;
  var logoCache = null;

  var channelFormBody =
    '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="ch-name" placeholder="BBC One"></div>' +
    '<div style="display:flex;gap:12px"><div class="form-group" style="flex:1"><label class="form-label">Number</label><input class="form-input" id="ch-number" type="number" min="0" placeholder="1"></div>' +
    '<div class="form-group" style="flex:1"><label class="form-label">Group</label><select class="form-input" id="ch-group"><option value="">None</option></select></div></div>' +
    '<div class="form-group"><label class="form-label">Logo</label>' +
    '<div id="ch-logo-picker" style="position:relative">' +
    '<div id="ch-logo-display" style="display:none;align-items:center;gap:8px;padding:6px 10px;background:var(--bg-input);border:1px solid var(--border);border-radius:var(--radius)">' +
    '<img id="ch-logo-img" style="width:28px;height:28px;object-fit:contain;border-radius:2px;background:var(--bg-hover)">' +
    '<span id="ch-logo-name" style="flex:1;font-size:13px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap"></span>' +
    '<button type="button" id="ch-logo-change" class="btn btn-ghost btn-sm" style="font-size:11px;padding:2px 8px">Change</button>' +
    '<button type="button" id="ch-logo-clear" style="background:none;border:none;color:var(--text-muted);cursor:pointer;font-size:16px;padding:0 4px">&times;</button></div>' +
    '<input class="form-input" id="ch-logo-search" placeholder="Search logos or paste URL..." autocomplete="off">' +
    '<input type="hidden" id="ch-logo" value="">' +
    '<div id="ch-logo-dropdown" style="display:none;position:absolute;top:100%;left:0;right:0;max-height:240px;overflow-y:auto;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);z-index:10;margin-top:2px"></div></div></div>' +
    '<div class="form-group"><label class="form-label">EPG ID (tvg_id)</label>' +
    '<div style="position:relative"><input class="form-input" id="ch-tvgid" placeholder="Type to search EPG channels..." autocomplete="off">' +
    '<div id="ch-tvgid-dropdown" style="display:none;position:absolute;top:100%;left:0;right:0;max-height:200px;overflow-y:auto;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);z-index:10;margin-top:2px"></div></div></div>' +
    '<div class="form-group"><label class="form-label">Streams</label>' +
    '<div style="position:relative"><input class="form-input" id="ch-stream-search" placeholder="Search streams..." autocomplete="off">' +
    '<div id="ch-stream-dropdown" style="display:none;position:absolute;top:100%;left:0;right:0;max-height:200px;overflow-y:auto;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);z-index:10;margin-top:2px"></div></div>' +
    '<div id="ch-selected-streams" style="display:flex;flex-wrap:wrap;gap:4px;margin-top:6px"></div></div>' +
    '<div class="form-group"><label class="form-label"><input type="checkbox" id="ch-enabled" checked> Enabled</label></div>';

  var chFormSelectedStreamIDs = [];

  function chFormOpenModal(title, saveLabel) {
    var modal = showFormModal(title, channelFormBody, { id: 'channel-modal', saveLabel: saveLabel, maxWidth: '600px' });
    modal.querySelector('.modal-save-btn').id = 'save-channel-btn';
    return modal;
  }

  async function chFormLoadEPGChannelIDs() {
    if (epgChannelIDs) return epgChannelIDs;
    try { var r = await api.get('/api/epg/channel-ids'); epgChannelIDs = await r.json(); if (!Array.isArray(epgChannelIDs)) epgChannelIDs = []; } catch (e) { epgChannelIDs = []; }
    return epgChannelIDs;
  }

  async function chFormLoadStreams() {
    if (channelStreams) return channelStreams;
    try { var r = await api.get('/api/streams?fields=slim'); channelStreams = await r.json(); if (!Array.isArray(channelStreams)) channelStreams = []; } catch (e) { channelStreams = []; }
    return channelStreams;
  }

  async function chFormLoadLogos() {
    if (logoCache) return logoCache;
    var logos = [];
    try {
      var epgIds = await chFormLoadEPGChannelIDs();
      for (var i = 0; i < epgIds.length; i++) {
        if (epgIds[i].icon) logos.push({ url: epgIds[i].icon, name: epgIds[i].name || epgIds[i].id });
      }
    } catch (e) {}
    try {
      var streams = await chFormLoadStreams();
      var seen = {};
      for (var j = 0; j < logos.length; j++) seen[logos[j].url] = true;
      for (var k = 0; k < streams.length; k++) {
        var slogo = streams[k].tvg_logo || streams[k].logo_url;
        if (slogo && !seen[slogo]) {
          seen[slogo] = true;
          logos.push({ url: slogo, name: streams[k].name });
        }
      }
    } catch (e) {}
    logoCache = logos;
    return logos;
  }

  function chFormSetLogoDisplay(url, name) {
    var display = document.getElementById('ch-logo-display');
    var searchInput = document.getElementById('ch-logo-search');
    var hidden = document.getElementById('ch-logo');
    if (!display || !searchInput || !hidden) return;
    if (url) {
      hidden.value = url;
      var img = document.getElementById('ch-logo-img');
      var nameEl = document.getElementById('ch-logo-name');
      if (img) img.src = url;
      if (nameEl) nameEl.textContent = name || url;
      display.style.display = 'flex';
      searchInput.style.display = 'none';
    } else {
      hidden.value = '';
      display.style.display = 'none';
      searchInput.style.display = '';
      searchInput.value = '';
    }
  }

  function chFormSetupLogoPicker() {
    var searchInput = document.getElementById('ch-logo-search');
    var dropdown = document.getElementById('ch-logo-dropdown');
    var changeBtn = document.getElementById('ch-logo-change');
    var clearBtn = document.getElementById('ch-logo-clear');
    if (!searchInput || !dropdown) return;
    if (changeBtn) changeBtn.addEventListener('click', function() { chFormSetLogoDisplay('', ''); searchInput.focus(); });
    if (clearBtn) clearBtn.addEventListener('click', function() { chFormSetLogoDisplay('', ''); });
    function showDropdown(filter) {
      chFormLoadLogos().then(function(logos) {
        var s = (filter || '').toLowerCase();
        var isUrl = s.indexOf('http') === 0;
        if (isUrl) { document.getElementById('ch-logo').value = filter; dropdown.style.display = 'none'; return; }
        var m = s.length < 1 ? logos.slice(0, 50) : logos.filter(function(l) { return (l.name || '').toLowerCase().indexOf(s) >= 0 || (l.url || '').toLowerCase().indexOf(s) >= 0; }).slice(0, 50);
        if (m.length === 0) { dropdown.innerHTML = '<div style="padding:8px 10px;color:var(--text-muted);font-size:13px">No logos found. Paste a URL instead.</div>'; dropdown.style.display = 'block'; return; }
        dropdown.innerHTML = m.map(function(l) {
          return '<div class="logo-opt" data-url="' + esc(l.url) + '" data-name="' + esc(l.name) + '" style="display:flex;align-items:center;gap:8px;padding:6px 10px;cursor:pointer;border-bottom:1px solid var(--border);font-size:13px">' +
            '<img src="' + esc(l.url) + '" style="width:24px;height:24px;object-fit:contain;border-radius:2px;background:var(--bg-hover);flex-shrink:0" onerror="this.style.display=\'none\'">' +
            '<span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + esc(l.name) + '</span></div>';
        }).join('');
        dropdown.style.display = 'block';
        dropdown.querySelectorAll('.logo-opt').forEach(function(opt) {
          opt.addEventListener('click', function() { chFormSetLogoDisplay(this.getAttribute('data-url'), this.getAttribute('data-name')); dropdown.style.display = 'none'; });
          opt.addEventListener('mouseenter', function() { this.style.background = 'var(--bg-hover)'; });
          opt.addEventListener('mouseleave', function() { this.style.background = ''; });
        });
      });
    }
    searchInput.addEventListener('focus', function() { showDropdown(searchInput.value); });
    searchInput.addEventListener('input', function() { var val = searchInput.value.trim(); if (val.indexOf('http') === 0) { document.getElementById('ch-logo').value = val; } showDropdown(val); });
    document.addEventListener('click', function(e) { if (!searchInput.contains(e.target) && !dropdown.contains(e.target)) dropdown.style.display = 'none'; });
  }

  function chFormSetupTvgIdDropdown() {
    var input = document.getElementById('ch-tvgid');
    var dropdown = document.getElementById('ch-tvgid-dropdown');
    if (!input || !dropdown) return;
    function show(filter) {
      chFormLoadEPGChannelIDs().then(function(ids) {
        var s = (filter || '').toLowerCase();
        var m = ids.filter(function(item) { return (item.id || '').toLowerCase().indexOf(s) >= 0 || (item.name || '').toLowerCase().indexOf(s) >= 0; }).slice(0, 50);
        if (m.length === 0) { dropdown.style.display = 'none'; return; }
        dropdown.innerHTML = m.map(function(item) {
          var ico = item.icon ? '<img src="' + esc(item.icon) + '" style="width:20px;height:20px;object-fit:contain;border-radius:2px;flex-shrink:0" onerror="this.style.display=\'none\'">' : '';
          var label = item.name ? esc(item.name) + ' <span style="color:var(--text-muted);font-size:11px">(' + esc(item.id) + ')</span>' : esc(item.id);
          return '<div class="tvgid-opt" data-id="' + esc(item.id) + '" data-icon="' + esc(item.icon || '') + '" data-name="' + esc(item.name || '') + '" style="display:flex;align-items:center;gap:8px;padding:6px 10px;cursor:pointer;border-bottom:1px solid var(--border);font-size:13px">' + ico + '<span>' + label + '</span></div>';
        }).join('');
        dropdown.style.display = 'block';
        dropdown.querySelectorAll('.tvgid-opt').forEach(function(opt) {
          opt.addEventListener('click', function() { input.value = this.getAttribute('data-id'); dropdown.style.display = 'none'; var icon = this.getAttribute('data-icon'); var epgName = this.getAttribute('data-name'); if (icon && !document.getElementById('ch-logo').value) { chFormSetLogoDisplay(icon, epgName || input.value); } });
          opt.addEventListener('mouseenter', function() { this.style.background = 'var(--bg-hover)'; });
          opt.addEventListener('mouseleave', function() { this.style.background = ''; });
        });
      });
    }
    input.addEventListener('focus', function() { show(input.value); });
    input.addEventListener('input', function() { show(input.value); });
    document.addEventListener('click', function(e) { if (!input.contains(e.target) && !dropdown.contains(e.target)) dropdown.style.display = 'none'; });
  }

  function chFormSetupStreamSearch() {
    var searchInput = document.getElementById('ch-stream-search');
    var dropdown = document.getElementById('ch-stream-dropdown');
    if (!searchInput || !dropdown) return;
    function show(filter) {
      chFormLoadStreams().then(function(streams) {
        var s = (filter || '').toLowerCase();
        var m = streams.filter(function(st) { return chFormSelectedStreamIDs.indexOf(st.id) < 0 && (st.name || '').toLowerCase().indexOf(s) >= 0; }).slice(0, 50);
        if (m.length === 0) { dropdown.innerHTML = '<div style="padding:8px 10px;color:var(--text-muted);font-size:13px">No matching streams</div>'; dropdown.style.display = 'block'; return; }
        dropdown.innerHTML = m.map(function(st) {
          var badge = st.source_type ? ' <span style="background:var(--bg-hover);padding:1px 6px;border-radius:3px;font-size:10px;color:var(--text-muted)">' + esc(st.source_type) + '</span>' : '';
          return '<div class="stream-opt" data-id="' + esc(st.id) + '" style="padding:6px 10px;cursor:pointer;border-bottom:1px solid var(--border);font-size:13px">' + esc(st.name) + badge + '</div>';
        }).join('');
        dropdown.style.display = 'block';
        dropdown.querySelectorAll('.stream-opt').forEach(function(opt) {
          opt.addEventListener('click', function() { chFormSelectedStreamIDs.push(this.getAttribute('data-id')); chFormRenderSelectedStreams(); searchInput.value = ''; dropdown.style.display = 'none'; });
          opt.addEventListener('mouseenter', function() { this.style.background = 'var(--bg-hover)'; });
          opt.addEventListener('mouseleave', function() { this.style.background = ''; });
        });
      });
    }
    searchInput.addEventListener('focus', function() { show(searchInput.value); });
    searchInput.addEventListener('input', function() { show(searchInput.value); });
    document.addEventListener('click', function(e) { if (!searchInput.contains(e.target) && !dropdown.contains(e.target)) dropdown.style.display = 'none'; });
  }

  function chFormRenderSelectedStreams() {
    var selContainer = document.getElementById('ch-selected-streams');
    if (!selContainer) return;
    if (chFormSelectedStreamIDs.length === 0) { selContainer.innerHTML = '<span style="color:var(--text-muted);font-size:12px">No streams assigned</span>'; return; }
    var streams = channelStreams || [];
    selContainer.innerHTML = chFormSelectedStreamIDs.map(function(id) {
      var s = streams.find(function(st) { return st.id === id; });
      var name = s ? s.name : id.substring(0, 12) + '...';
      return '<span style="display:inline-flex;align-items:center;gap:4px;background:var(--bg-hover);padding:2px 8px;border-radius:12px;font-size:12px">' + esc(name) + '<button class="stream-rm" data-id="' + esc(id) + '" style="background:none;border:none;color:var(--text-muted);cursor:pointer;padding:0 2px;font-size:14px">&times;</button></span>';
    }).join('');
    selContainer.querySelectorAll('.stream-rm').forEach(function(btn) {
      btn.addEventListener('click', function() { var rid = this.getAttribute('data-id'); chFormSelectedStreamIDs = chFormSelectedStreamIDs.filter(function(id) { return id !== rid; }); chFormRenderSelectedStreams(); });
    });
  }

  function chFormPopulate(ch, groups) {
    var grps = groups || channelGroups;
    var groupSelect = document.getElementById('ch-group');
    if (groupSelect) {
      groupSelect.innerHTML = '<option value="">None</option>';
      for (var gi = 0; gi < grps.length; gi++) {
        var g = grps[gi];
        var sel = ch && ch.group_id === g.id ? ' selected' : '';
        groupSelect.innerHTML += '<option value="' + esc(g.id) + '"' + sel + '>' + esc(g.name) + '</option>';
      }
    }
    var tvgInput = document.getElementById('ch-tvgid');
    if (tvgInput) tvgInput.value = (ch && ch.tvg_id) || '';
    chFormSetupTvgIdDropdown();
    chFormSetupLogoPicker();
    if (ch && ch.logo_url) { chFormSetLogoDisplay(ch.logo_url, ch.name || ''); } else { chFormSetLogoDisplay('', ''); }
    chFormSelectedStreamIDs = ch && ch.stream_ids ? ch.stream_ids.slice() : [];
    chFormRenderSelectedStreams();
    chFormSetupStreamSearch();
  }

  function openStandaloneChannelModal(opts) {
    opts = opts || {};
    var editId = opts.editId || null;
    var ch = opts.channel || null;
    var preStreamID = opts.preStreamID || null;
    var preStreamName = opts.preStreamName || null;
    var onSaved = opts.onSaved || function() {};
    var title = editId ? 'Edit Channel' : 'New Channel';
    var saveLabel = editId ? 'Update' : 'Create';
    chFormOpenModal(title, saveLabel);
    document.getElementById('ch-name').value = (ch && ch.name) || preStreamName || '';
    document.getElementById('ch-number').value = (ch && ch.number) || '';
    if (document.getElementById('ch-tvgid')) document.getElementById('ch-tvgid').value = (ch && ch.tvg_id) || '';
    document.getElementById('ch-enabled').checked = ch ? ch.is_enabled !== false : true;
    if (preStreamID) { chFormSelectedStreamIDs = [preStreamID]; }
    chFormPopulate(ch);
    if (preStreamID) { chFormRenderSelectedStreams(); }
    document.getElementById('save-channel-btn').addEventListener('click', async function() {
      var name = document.getElementById('ch-name').value.trim();
      var number = parseInt(document.getElementById('ch-number').value) || 0;
      var groupId = document.getElementById('ch-group').value;
      var logoUrl = document.getElementById('ch-logo').value.trim();
      var tvgId = document.getElementById('ch-tvgid') ? document.getElementById('ch-tvgid').value.trim() : '';
      var enabled = document.getElementById('ch-enabled').checked;
      if (!name) { toast('Name required', 'error'); return; }
      var payload = { name: name, number: number, group_id: groupId, logo_url: logoUrl, tvg_id: tvgId, is_enabled: enabled, stream_ids: chFormSelectedStreamIDs };
      try {
        var r;
        if (editId) { r = await api.put('/api/channels/' + editId, payload); } else { r = await api.post('/api/channels', payload); }
        if (r.ok) { toast(editId ? 'Channel updated' : 'Channel created'); var m = document.getElementById('channel-modal'); if (m) m.remove(); onSaved(); }
        else { var data = await r.json().catch(function() { return {}; }); toast(data.error || 'Failed to save channel', 'error'); }
      } catch (err) { toast('Failed to save channel', 'error'); }
    });
  }

  async function renderChannels(el) {
    var user = api.user;
    var isAdmin = user && user.is_admin;

    el.innerHTML = '<div class="skeleton" style="height:200px"></div>';

    var openGroups = {};
    var groupsInitialized = false;

    try {
      var groupResp = await api.get('/api/channel-groups');
      channelGroups = await groupResp.json();
      if (!Array.isArray(channelGroups)) channelGroups = [];
    } catch (e) { channelGroups = []; }

    channelStreams = null;
    logoCache = null;

    var nowData = {};
    try {
      var nowResp = await api.get('/api/epg/now');
      var nowList = await nowResp.json();
      if (Array.isArray(nowList)) {
        for (var ni = 0; ni < nowList.length; ni++) {
          nowData[nowList[ni].channel_id] = nowList[ni];
        }
      }
    } catch (e) {}

    var channels = [];
    try {
      var resp = await api.get('/api/channels');
      channels = await resp.json();
      if (!Array.isArray(channels)) channels = [];
    } catch (e) {
      el.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load channels</p></div>';
      return;
    }

    var searchTerm = '';

    function openEditChannel(ch) {
      openStandaloneChannelModal({ editId: ch.id, channel: ch, onSaved: function() { renderChannels(el); } });
    }

    function openAddChannel(preStreamID, preStreamName) {
      openStandaloneChannelModal({ preStreamID: preStreamID, preStreamName: preStreamName, onSaved: function() { renderChannels(el); } });
    }

        async function deleteChannel(id, name) {
      if (!confirm('Delete channel "' + name + '"?')) return;
      try {
        var r = await api.del('/api/channels/' + id);
        if (r.ok || r.status === 204) {
          toast('Channel deleted');
          renderChannels(el);
        } else {
          toast('Failed to delete channel', 'error');
        }
      } catch (err) {
        toast('Failed to delete channel', 'error');
      }
    }

    function showGroupManageModal() {
      var existing = document.getElementById('group-manage-modal');
      if (existing) existing.remove();
      var html = '<div class="modal-overlay" id="group-manage-modal"><div class="modal-content" style="max-width:500px"><div class="modal-header"><span>Channel Groups</span><button class="modal-close-x gm-close-x" title="Close">\u2715</button></div><div class="modal-body">' +
        '<p style="color:var(--text-muted);font-size:12px;margin:0 0 8px">Drag to reorder groups</p>' +
        '<div id="gm-group-list"></div>' +
        '<div style="display:flex;gap:8px;margin-top:12px"><input class="form-input" id="gm-new-name" placeholder="New group name" style="flex:1"><button class="btn btn-primary" id="gm-create-btn">Add</button></div>' +
        '</div><div class="modal-footer"><button class="btn btn-ghost" id="gm-close">Close</button></div></div></div>';
      document.body.insertAdjacentHTML('beforeend', html);
      var overlay = document.getElementById('group-manage-modal');
      overlay.querySelector('#gm-close').addEventListener('click', function() { overlay.remove(); renderChannels(el); });
      overlay.querySelector('.gm-close-x').addEventListener('click', function() { overlay.remove(); renderChannels(el); });
      overlay.addEventListener('click', function(e) { if (e.target === overlay) { overlay.remove(); renderChannels(el); } });
      overlay.querySelector('#gm-create-btn').addEventListener('click', async function() {
        var name = document.getElementById('gm-new-name').value.trim();
        if (!name) { toast('Name required', 'error'); return; }
        var r = await api.post('/api/channel-groups', { name: name });
        if (r.ok) { toast('Group created'); document.getElementById('gm-new-name').value = ''; await refreshGMGroups(); renderGMList(); } else { toast('Failed', 'error'); }
      });
      renderGMList();
      async function refreshGMGroups() {
        try { var r = await api.get('/api/channel-groups'); channelGroups = await r.json(); if (!Array.isArray(channelGroups)) channelGroups = []; } catch (e) { channelGroups = []; }
      }
      var dragSrcIdx = null;
      function renderGMList() {
        var gmContainer = document.getElementById('gm-group-list');
        if (!gmContainer) return;
        if (channelGroups.length === 0) { gmContainer.innerHTML = '<p style="color:var(--text-muted)">No groups yet</p>'; return; }
        gmContainer.innerHTML = channelGroups.map(function(g, idx) {
          var countBadge = g.channel_count ? ' <span style="color:var(--text-muted);font-size:11px">(' + g.channel_count + ' ch)</span>' : '';
          return '<div class="gm-row" draggable="true" data-idx="' + idx + '" style="display:flex;align-items:center;gap:6px;padding:8px 6px;border-bottom:1px solid var(--border);cursor:grab;user-select:none">' +
            '<span style="color:var(--text-muted);font-size:14px;cursor:grab;padding:0 4px">\u2630</span>' +
            '<input class="form-input gm-name-input" data-id="' + esc(g.id) + '" value="' + esc(g.name) + '" style="flex:1;padding:4px 8px;font-size:13px">' +
            countBadge +
            '<button class="btn btn-sm btn-danger gm-del" data-id="' + esc(g.id) + '" data-name="' + esc(g.name) + '" style="flex-shrink:0">' + icons.trash + '</button></div>';
        }).join('');
        gmContainer.querySelectorAll('.gm-name-input').forEach(function(input) {
          input.addEventListener('change', async function() {
            var gid = this.getAttribute('data-id');
            var newName = this.value.trim();
            if (!newName) { toast('Name required', 'error'); return; }
            await api.put('/api/channel-groups/' + gid, { name: newName });
            await refreshGMGroups();
          });
        });
        gmContainer.querySelectorAll('.gm-row').forEach(function(row) {
          row.addEventListener('dragstart', function(e) { dragSrcIdx = parseInt(this.getAttribute('data-idx')); e.dataTransfer.effectAllowed = 'move'; this.style.opacity = '0.4'; });
          row.addEventListener('dragend', function() { this.style.opacity = '1'; gmContainer.querySelectorAll('.gm-row').forEach(function(r) { r.style.borderTop = ''; }); });
          row.addEventListener('dragover', function(e) { e.preventDefault(); e.dataTransfer.dropEffect = 'move'; gmContainer.querySelectorAll('.gm-row').forEach(function(r) { r.style.borderTop = ''; }); this.style.borderTop = '2px solid var(--accent)'; });
          row.addEventListener('dragleave', function() { this.style.borderTop = ''; });
          row.addEventListener('drop', async function(e) {
            e.preventDefault(); this.style.borderTop = '';
            var targetIdx = parseInt(this.getAttribute('data-idx'));
            if (dragSrcIdx === null || dragSrcIdx === targetIdx) return;
            var moved = channelGroups.splice(dragSrcIdx, 1)[0];
            channelGroups.splice(targetIdx, 0, moved);
            await api.post('/api/channel-groups/reorder', { group_ids: channelGroups.map(function(g) { return g.id; }) });
            await refreshGMGroups(); renderGMList();
          });
        });
        gmContainer.querySelectorAll('.gm-del').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var id = this.getAttribute('data-id'); var name = this.getAttribute('data-name');
            if (!confirm('Delete group "' + name + '"?')) return;
            var r = await api.del('/api/channel-groups/' + id);
            if (r.ok || r.status === 204) { toast('Group deleted'); await refreshGMGroups(); renderGMList(); }
          });
        });
      }
    }

    function getSelectedChannelIDs() {
      var checks = document.querySelectorAll('.ch-select:checked');
      var ids = [];
      for (var ci = 0; ci < checks.length; ci++) ids.push(checks[ci].dataset.id);
      return ids;
    }

    function updateBulkCount() {
      var count = getSelectedChannelIDs().length;
      var bulkActions = document.getElementById('bulk-actions');
      var bulkCount = document.getElementById('bulk-count');
      if (bulkActions) bulkActions.style.display = count > 0 ? 'flex' : 'none';
      if (bulkCount) bulkCount.textContent = count + ' selected';
    }

    function buildChannelRow(c) {
      var logoHtml = c.logo_url
        ? '<img class="logo" src="' + esc(c.logo_url) + '" style="cursor:pointer;max-width:100px;max-height:40px;object-fit:contain;border-radius:2px;vertical-align:middle" alt="">'
        : (c.stream_ids && c.stream_ids.length > 0 ? '<span style="cursor:pointer;font-size:20px;color:var(--accent)">' + icons.play + '</span>' : '');

      var nameHtml = '<div style="display:flex;flex-direction:column;gap:2px">';
      nameHtml += '<span style="font-weight:600">' + esc(c.name);
      if (!c.is_enabled) nameHtml += ' <span class="badge badge-disabled" style="font-size:10px;margin-left:6px">Off</span>';
      nameHtml += '</span>';
      var nowInfo = nowData[c.id];
      if (nowInfo && nowInfo.title) {
        nameHtml += '<span style="font-size:12px;color:var(--text-muted)">' + esc(nowInfo.title) + '</span>';
      }
      nameHtml += '</div>';

      var streamCount = c.stream_ids ? c.stream_ids.length : 0;

      var checkTd = isAdmin ? '<td><input type="checkbox" class="ch-select" data-id="' + esc(c.id) + '"></td>' : '';
      var actions = '';
      if (c.stream_ids && c.stream_ids.length > 0) {
        actions += '<button class="btn btn-sm btn-primary play-btn" data-id="' + esc(c.stream_ids[0]) + '" data-name="' + esc(c.name) + '" style="padding:4px 8px">' + icons.play + '</button>';
      }
      if (isAdmin) {
        actions += '<button class="btn btn-sm btn-ghost ch-edit-btn" data-id="' + esc(c.id) + '" title="Edit">' + icons.edit + '</button>' +
          '<button class="btn btn-sm btn-danger ch-delete-btn" data-id="' + esc(c.id) + '" data-name="' + esc(c.name) + '" title="Delete">' + icons.trash + '</button>';
      }

      return '<tr>' +
        checkTd +
        '<td style="width:110px;padding-right:0;text-align:center" class="ch-logo-cell" data-id="' + esc(c.id) + '">' + logoHtml + '</td>' +
        '<td>' + nameHtml + '</td>' +
        '<td>' + streamCount + '</td>' +
        '<td style="white-space:nowrap"><div style="display:flex;gap:4px;align-items:center">' + actions + '</div></td>' +
        '</tr>';
    }

    function renderTable() {
      var filtered = channels;
      if (searchTerm) {
        var q = searchTerm.toLowerCase();
        var groupNameMap = {};
        for (var gni = 0; gni < channelGroups.length; gni++) {
          groupNameMap[channelGroups[gni].id] = (channelGroups[gni].name || '').toLowerCase();
        }
        filtered = channels.filter(function(c) {
          var gn = groupNameMap[c.group_id] || '';
          return (c.name || '').toLowerCase().indexOf(q) >= 0 ||
                 gn.indexOf(q) >= 0 ||
                 (c.tvg_id || '').toLowerCase().indexOf(q) >= 0;
        });
      }

      filtered.sort(function(a, b) { return (a.number || 0) - (b.number || 0); });

      var grouped = {};
      var ungrouped = [];
      var groupMap = {};
      for (var gmi = 0; gmi < channelGroups.length; gmi++) {
        groupMap[channelGroups[gmi].id] = channelGroups[gmi];
      }

      for (var fi = 0; fi < filtered.length; fi++) {
        var c = filtered[fi];
        var gid = c.group_id;
        if (gid && groupMap[gid]) {
          if (!grouped[gid]) grouped[gid] = [];
          grouped[gid].push(c);
        } else {
          ungrouped.push(c);
        }
      }

      if (!groupsInitialized) {
        for (var ogi = 0; ogi < channelGroups.length; ogi++) {
          openGroups[channelGroups[ogi].id] = true;
        }
        openGroups['__ungrouped__'] = true;
        groupsInitialized = true;
      }

      var colCount = isAdmin ? 5 : 4;
      var checkCol = isAdmin ? '<th style="width:32px"><input type="checkbox" id="ch-select-all"></th>' : '';
      var countText = searchTerm
        ? 'Channels (' + filtered.length + ' of ' + channels.length + ')'
        : 'Channels (' + channels.length + ')';

      var tableContainer = document.getElementById('ch-table-container');
      if (!tableContainer) return;

      var countEl = document.getElementById('ch-count');
      if (countEl) countEl.textContent = countText;

      var html = '<table class="list-table"><thead><tr>' +
        checkCol + '<th style="width:110px;padding-right:0;text-align:center"></th><th>Channel</th><th>Streams</th><th style="width:120px"></th>' +
        '</tr></thead>';

      var sortedGroups = channelGroups.slice().sort(function(a, b) { return (a.sort_order || 0) - (b.sort_order || 0); });

      function buildGroupSection(secGid, label, items) {
        var isOpen = !!openGroups[secGid];
        var arrowClass = 'group-header-arrow' + (isOpen ? ' open' : '');
        var section = '<tbody data-gid="' + esc(secGid) + '">' +
          '<tr class="group-header-row" data-gid="' + esc(secGid) + '"><td colspan="' + colCount + '">' +
          '<span class="' + arrowClass + '">\u25B6</span>' +
          esc(label) +
          '<span class="group-header-count">' + items.length + '</span>' +
          '</td></tr>';
        for (var si = 0; si < items.length; si++) {
          var rowStyle = isOpen ? '' : ' style="display:none"';
          section += buildChannelRow(items[si]).replace('<tr>', '<tr class="ch-group-item" data-gid="' + esc(secGid) + '"' + rowStyle + '>');
        }
        section += '</tbody>';
        return section;
      }

      var hasContent = false;
      for (var sgi = 0; sgi < sortedGroups.length; sgi++) {
        var group = sortedGroups[sgi];
        var items = grouped[group.id];
        if (!items || items.length === 0) continue;
        hasContent = true;
        html += buildGroupSection(group.id, group.name || 'Unknown', items);
      }

      if (ungrouped.length > 0) {
        hasContent = true;
        html += buildGroupSection('__ungrouped__', 'Ungrouped', ungrouped);
      }

      if (!hasContent) {
        html += '<tbody><tr><td colspan="' + colCount + '" style="padding:40px 16px;text-align:center;color:var(--text-muted)">' +
          (searchTerm ? 'No matching channels' : 'No channels yet') +
          '</td></tr></tbody>';
      }

      html += '</table>';
      tableContainer.innerHTML = html;

      tableContainer.querySelectorAll('.group-header-row').forEach(function(row) {
        row.addEventListener('click', function(e) {
          if (e.target.closest && e.target.closest('button')) return;
          var rowGid = this.getAttribute('data-gid');
          openGroups[rowGid] = !openGroups[rowGid];
          var arrow = this.querySelector('.group-header-arrow');
          if (arrow) arrow.className = 'group-header-arrow' + (openGroups[rowGid] ? ' open' : '');
          tableContainer.querySelectorAll('.ch-group-item[data-gid="' + rowGid + '"]').forEach(function(r) {
            r.style.display = openGroups[rowGid] ? '' : 'none';
          });
        });
      });

      if (isAdmin) {
        var selectAll = document.getElementById('ch-select-all');
        if (selectAll) {
          selectAll.addEventListener('change', function() {
            var checks = document.querySelectorAll('.ch-select');
            for (var ci = 0; ci < checks.length; ci++) checks[ci].checked = selectAll.checked;
            updateBulkCount();
          });
        }
        tableContainer.querySelectorAll('.ch-select').forEach(function(cb) {
          cb.addEventListener('change', function() { updateBulkCount(); });
        });
      }

      tableContainer.querySelectorAll('.ch-logo-cell').forEach(function(cell) {
        cell.addEventListener('click', function(e) {
          e.stopPropagation();
          var chId = this.getAttribute('data-id');
          var ch = channels.find(function(cc) { return cc.id === chId; });
          if (ch && ch.stream_ids && ch.stream_ids.length > 0) {
            startPlay(ch.stream_ids[0], ch.name, true);
          }
        });
      });

      tableContainer.querySelectorAll('.play-btn').forEach(function(btn) {
        btn.addEventListener('click', function(e) {
          e.stopPropagation();
          startPlay(this.getAttribute('data-id'), this.getAttribute('data-name'), true);
        });
      });

      if (isAdmin) {
        tableContainer.querySelectorAll('.ch-edit-btn').forEach(function(btn) {
          btn.addEventListener('click', function() {
            var chId = this.getAttribute('data-id');
            var ch = channels.find(function(cc) { return cc.id === chId; });
            if (ch) openEditChannel(ch);
          });
        });

        tableContainer.querySelectorAll('.ch-delete-btn').forEach(function(btn) {
          btn.addEventListener('click', function() {
            deleteChannel(this.getAttribute('data-id'), this.getAttribute('data-name'));
          });
        });
      }
    }

    el.innerHTML = '';

    var headerRow = document.createElement('div');
    headerRow.style.cssText = 'display:flex;align-items:center;justify-content:space-between;margin-bottom:16px';

    var chCountEl = document.createElement('h2');
    chCountEl.id = 'ch-count';
    chCountEl.style.margin = '0';
    chCountEl.textContent = 'Channels (' + channels.length + ')';
    headerRow.appendChild(chCountEl);

    var rightActions = document.createElement('div');
    rightActions.style.cssText = 'display:flex;align-items:center;gap:8px';

    var searchInput = document.createElement('input');
    searchInput.type = 'text';
    searchInput.placeholder = 'Search...';
    searchInput.style.cssText = 'padding:6px 10px;background:var(--bg-input);border:1px solid var(--border);border-radius:var(--radius);color:var(--text);font-size:13px;width:220px;outline:none';
    var searchTimer = null;
    searchInput.addEventListener('input', function() {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(function() {
        searchTerm = searchInput.value;
        renderTable();
      }, 300);
    });
    rightActions.appendChild(searchInput);

    if (isAdmin) {
      var addBtn = document.createElement('button');
      addBtn.className = 'btn btn-primary btn-sm';
      addBtn.style.cssText = 'font-size:18px;padding:4px 12px';
      addBtn.textContent = '+';
      addBtn.title = 'Add Channel';
      addBtn.addEventListener('click', function() { openAddChannel(null, null); });
      rightActions.appendChild(addBtn);
    }

    headerRow.appendChild(rightActions);
    el.appendChild(headerRow);

    if (isAdmin) {
      var toolbarRow = document.createElement('div');
      toolbarRow.style.cssText = 'display:flex;gap:8px;margin-bottom:12px;flex-wrap:wrap;align-items:center';

      var groupsBtn = document.createElement('button');
      groupsBtn.className = 'btn btn-ghost btn-sm';
      groupsBtn.textContent = 'Manage Groups';
      groupsBtn.addEventListener('click', function() { showGroupManageModal(); });
      toolbarRow.appendChild(groupsBtn);

      var autoNumBtn = document.createElement('button');
      autoNumBtn.className = 'btn btn-ghost btn-sm';
      autoNumBtn.textContent = '# Auto Number';
      autoNumBtn.title = 'Auto-assign channel numbers sequentially';
      autoNumBtn.addEventListener('click', async function() {
        if (!confirm('Auto-assign sequential channel numbers to all channels?')) return;
        try {
          var r = await api.post('/api/channels/batch', { auto_number: true });
          if (r.ok) { toast('Channels renumbered'); renderChannels(el); }
          else { toast('Failed to renumber', 'error'); }
        } catch (err) { toast('Failed to renumber', 'error'); }
      });
      toolbarRow.appendChild(autoNumBtn);

      var autoEpgBtn = document.createElement('button');
      autoEpgBtn.className = 'btn btn-ghost btn-sm';
      autoEpgBtn.textContent = 'Auto Match EPG';
      autoEpgBtn.title = 'Auto-match EPG channel IDs to channels by name';
      autoEpgBtn.addEventListener('click', async function() {
        if (!confirm('Auto-match EPG channel IDs to channels by name?')) return;
        try {
          var r = await api.post('/api/epg/auto-match');
          if (r.ok) { var data = await r.json(); toast('Matched ' + (data.matched || 0) + ' channels'); renderChannels(el); }
          else { toast('Auto-match failed', 'error'); }
        } catch (err) { toast('Auto-match failed', 'error'); }
      });
      toolbarRow.appendChild(autoEpgBtn);

      var bulkSpan = document.createElement('span');
      bulkSpan.id = 'bulk-actions';
      bulkSpan.style.cssText = 'display:none;gap:8px;align-items:center;margin-left:auto';

      var bulkEnableBtn = document.createElement('button');
      bulkEnableBtn.className = 'btn btn-ghost btn-sm';
      bulkEnableBtn.style.color = 'var(--success)';
      bulkEnableBtn.textContent = 'Enable Selected';
      bulkEnableBtn.addEventListener('click', async function() {
        var ids = getSelectedChannelIDs();
        if (ids.length === 0) return;
        try {
          var r = await api.post('/api/channels/batch', { channel_ids: ids, is_enabled: true });
          if (r.ok) { toast(ids.length + ' channels enabled'); renderChannels(el); }
          else { toast('Failed to enable channels', 'error'); }
        } catch (err) { toast('Failed to enable channels', 'error'); }
      });
      bulkSpan.appendChild(bulkEnableBtn);

      var bulkDisableBtn = document.createElement('button');
      bulkDisableBtn.className = 'btn btn-ghost btn-sm';
      bulkDisableBtn.style.color = 'var(--danger)';
      bulkDisableBtn.textContent = 'Disable Selected';
      bulkDisableBtn.addEventListener('click', async function() {
        var ids = getSelectedChannelIDs();
        if (ids.length === 0) return;
        try {
          var r = await api.post('/api/channels/batch', { channel_ids: ids, is_enabled: false });
          if (r.ok) { toast(ids.length + ' channels disabled'); renderChannels(el); }
          else { toast('Failed to disable channels', 'error'); }
        } catch (err) { toast('Failed to disable channels', 'error'); }
      });
      bulkSpan.appendChild(bulkDisableBtn);

      var bulkGroupSelect = document.createElement('select');
      bulkGroupSelect.className = 'form-input';
      bulkGroupSelect.style.cssText = 'width:auto;font-size:12px';
      bulkGroupSelect.innerHTML = '<option value="">Assign Group...</option>';
      for (var bgi = 0; bgi < channelGroups.length; bgi++) {
        bulkGroupSelect.innerHTML += '<option value="' + esc(channelGroups[bgi].id) + '">' + esc(channelGroups[bgi].name) + '</option>';
      }
      bulkGroupSelect.addEventListener('change', async function() {
        var bgid = this.value; if (!bgid) return;
        var ids = getSelectedChannelIDs(); if (ids.length === 0) { toast('Select channels first', 'error'); this.value = ''; return; }
        try { var r = await api.post('/api/channels/assign-group', { channel_ids: ids, group_id: bgid }); if (r.ok) { toast(ids.length + ' channels assigned to group'); renderChannels(el); } else { toast('Failed to assign group', 'error'); } } catch (err) { toast('Failed to assign group', 'error'); }
        this.value = '';
      });
      bulkSpan.appendChild(bulkGroupSelect);

      var bulkCountSpan = document.createElement('span');
      bulkCountSpan.id = 'bulk-count';
      bulkCountSpan.style.cssText = 'font-size:12px;color:var(--text-muted)';
      bulkSpan.appendChild(bulkCountSpan);

      toolbarRow.appendChild(bulkSpan);
      el.appendChild(toolbarRow);
    }

    var tableContainer = document.createElement('div');
    tableContainer.id = 'ch-table-container';
    el.appendChild(tableContainer);

    renderTable();
  }

  async function showStreamDetail(streamID, streamName) {
    var overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.style.cssText = 'align-items:flex-start;padding:40px 20px;overflow-y:auto';
    overlay.innerHTML = '<div class="detail-modal" style="max-width:900px;width:100%;margin:0 auto">' +
      '<div style="background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius-lg);overflow:hidden">' +
      '<div class="detail-backdrop" style="height:300px;background:var(--bg-hover);position:relative;display:flex;align-items:flex-end">' +
      '<div style="position:absolute;inset:0;background:linear-gradient(transparent 30%,var(--bg-card))"></div>' +
      '<div style="position:relative;padding:20px 24px;display:flex;gap:20px;align-items:flex-end;width:100%">' +
      '<div class="detail-poster" style="width:120px;height:180px;background:var(--bg-hover);border-radius:var(--radius);flex-shrink:0"></div>' +
      '<div style="flex:1;min-width:0"><h2 style="color:#fff;font-size:22px;margin-bottom:4px">' + esc(streamName) + '</h2>' +
      '<div class="detail-meta" style="color:var(--text-dim);font-size:13px">Loading...</div></div></div></div>' +
      '<div class="detail-body" style="padding:24px">' +
      '<div style="display:flex;gap:8px;margin-bottom:20px">' +
      '<button class="btn btn-primary detail-play-btn" style="gap:6px">' + icons.play + ' Play</button>' +
      '<button class="btn btn-ghost detail-fav-btn">' + icons.star + ' Favorite</button>' +
      '<button class="btn btn-ghost detail-close-btn" style="margin-left:auto">Close</button></div>' +
      '<div class="detail-overview" style="color:var(--text);font-size:14px;line-height:1.7;margin-bottom:20px"></div>' +
      '<div class="detail-crew" style="margin-bottom:20px"></div>' +
      '<div class="detail-cast"></div>' +
      '</div></div></div>';

    document.body.appendChild(overlay);

    overlay.addEventListener('click', function(e) {
      if (e.target === overlay) { overlay.remove(); }
    });
    overlay.querySelector('.detail-close-btn').addEventListener('click', function() { overlay.remove(); });
    overlay.querySelector('.detail-play-btn').addEventListener('click', function() {
      overlay.remove();
      startPlay(streamID, streamName, false);
    });

    var favBtn = overlay.querySelector('.detail-fav-btn');
    if (streamFavorites[streamID]) {
      favBtn.innerHTML = icons.starFilled + ' Favorited';
      favBtn.classList.add('favorited');
    }
    favBtn.addEventListener('click', function() {
      toggleFavorite(streamID).then(function() {
        var nowFav = streamFavorites[streamID];
        favBtn.innerHTML = (nowFav ? icons.starFilled : icons.star) + (nowFav ? ' Favorited' : ' Favorite');
        if (nowFav) favBtn.classList.add('favorited');
        else favBtn.classList.remove('favorited');
      }).catch(function() { toast('Failed to update favorite', 'error'); });
    });

    try {
      var resp = await api.get('/api/streams/' + encodeURIComponent(streamID) + '/detail');
      if (!resp.ok) return;
      var data = await resp.json();

      if (data.backdrop_url) {
        var backdropEl = overlay.querySelector('.detail-backdrop');
        backdropEl.style.backgroundImage = 'url(' + data.backdrop_url + ')';
        backdropEl.style.backgroundSize = 'cover';
        backdropEl.style.backgroundPosition = 'center top';
      }

      if (data.poster_url) {
        var posterEl = overlay.querySelector('.detail-poster');
        posterEl.style.backgroundImage = 'url(' + data.poster_url + ')';
        posterEl.style.backgroundSize = 'cover';
        posterEl.style.backgroundPosition = 'center';
        posterEl.style.borderRadius = 'var(--radius)';
      }

      var titleEl = overlay.querySelector('.detail-backdrop h2');
      var displayTitle = data.title || data.name || streamName;
      var yearStr = '';
      if (data.release_date) yearStr = data.release_date.substring(0, 4);
      else if (data.first_air_date) yearStr = data.first_air_date.substring(0, 4);
      if (yearStr) displayTitle += ' (' + yearStr + ')';
      titleEl.textContent = displayTitle;

      var metaParts = [];
      if (data.certification) metaParts.push(data.certification);
      if (data.rating) metaParts.push('\u2605 ' + data.rating.toFixed(1));
      if (data.runtime) {
        var hrs = Math.floor(data.runtime / 60);
        var mins = data.runtime % 60;
        metaParts.push(hrs > 0 ? hrs + 'h ' + mins + 'm' : mins + 'm');
      }
      if (data.genres && data.genres.length > 0) metaParts.push(data.genres.join(' \u2022 '));
      var metaEl = overlay.querySelector('.detail-meta');
      metaEl.textContent = metaParts.join('  \u2014  ');

      if (data.overview) {
        overlay.querySelector('.detail-overview').textContent = data.overview;
      }

      if (data.crew && data.crew.length > 0) {
        var directors = data.crew.filter(function(c) { return c.job === 'Director'; });
        var writers = data.crew.filter(function(c) { return c.job === 'Writer' || c.job === 'Screenplay'; });
        var crewHtml = '';
        if (directors.length > 0) {
          crewHtml += '<div style="margin-bottom:8px"><span style="color:var(--text-dim);font-size:12px;text-transform:uppercase;letter-spacing:.5px">Directed by</span><br>' +
            '<span style="color:var(--text);font-size:14px">' + directors.map(function(d) { return esc(d.name); }).join(', ') + '</span></div>';
        }
        if (writers.length > 0) {
          crewHtml += '<div><span style="color:var(--text-dim);font-size:12px;text-transform:uppercase;letter-spacing:.5px">Written by</span><br>' +
            '<span style="color:var(--text);font-size:14px">' + writers.map(function(w) { return esc(w.name); }).join(', ') + '</span></div>';
        }
        overlay.querySelector('.detail-crew').innerHTML = crewHtml;
      }

      if (data.cast && data.cast.length > 0) {
        var castHtml = '<div style="color:var(--text-dim);font-size:12px;text-transform:uppercase;letter-spacing:.5px;margin-bottom:10px">Cast</div>' +
          '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:10px">';
        var maxCast = Math.min(data.cast.length, 12);
        for (var ci = 0; ci < maxCast; ci++) {
          var member = data.cast[ci];
          var photoStyle = 'width:40px;height:40px;border-radius:50%;background:var(--bg-hover);flex-shrink:0;background-size:cover;background-position:center';
          if (member.profile_url) photoStyle += ';background-image:url(' + member.profile_url + ')';
          castHtml += '<div style="display:flex;gap:10px;align-items:center">' +
            '<div style="' + photoStyle + '"></div>' +
            '<div style="min-width:0"><div style="color:var(--text);font-size:13px;font-weight:500;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">' + esc(member.name) + '</div>' +
            '<div style="color:var(--text-muted);font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">' + esc(member.character || '') + '</div></div></div>';
        }
        castHtml += '</div>';
        overlay.querySelector('.detail-cast').innerHTML = castHtml;
      }

      if (data.seasons && data.seasons.length > 0) {
        var seasonsHtml = '<div style="margin-top:20px;border-top:1px solid var(--border);padding-top:16px">' +
          '<div style="color:var(--text-dim);font-size:12px;text-transform:uppercase;letter-spacing:.5px;margin-bottom:10px">Seasons</div>';
        for (var si = 0; si < data.seasons.length; si++) {
          var sn = data.seasons[si];
          if (sn.season_number === 0) continue;
          seasonsHtml += '<div style="display:flex;gap:12px;align-items:center;padding:8px 0;border-bottom:1px solid var(--border)">';
          if (sn.poster_url) {
            seasonsHtml += '<img src="' + sn.poster_url + '" style="width:48px;height:72px;object-fit:cover;border-radius:4px">';
          }
          seasonsHtml += '<div><div style="color:#fff;font-weight:500;font-size:14px">' + esc(sn.name) + '</div>' +
            '<div style="color:var(--text-muted);font-size:12px">' + sn.episode_count + ' episodes</div></div></div>';
        }
        seasonsHtml += '</div>';
        overlay.querySelector('.detail-cast').insertAdjacentHTML('afterend', seasonsHtml);
      }
    } catch (e) {
      overlay.querySelector('.detail-overview').textContent = 'Could not load details.';
    }
  }

  async function startPlay(streamID, name, isLive) {
    playerState.cleanup();
    closePlayerOverlay();
    playerState.currentStreamID = streamID;
    playerState.isLive = isLive !== false;
    openPlayerOverlay(streamID, name || streamID, isLive);
  }

  function closePlayerOverlay() {
    var existing = document.getElementById('player-overlay');
    if (existing) {
      playerState.cleanup();
      existing.parentNode.removeChild(existing);
    }
  }

  function openPlayerOverlay(streamID, name, isLive) {
    var overlay = document.createElement('div');
    overlay.id = 'player-overlay';
    overlay.className = 'player-overlay';

    var modal = document.createElement('div');
    modal.className = 'player-overlay-modal';

    modal.innerHTML =
      '<div class="player-wrapper" id="player-wrapper">' +
        '<video id="video-el" autoplay playsinline></video>' +
        '<div class="player-spinner" id="player-spinner">' +
          '<div class="spinner-ring"></div>' +
        '</div>' +
        '<div class="player-float-bar" id="player-float-bar">' +
          '<span class="player-title" id="player-title">' + esc(name) + '</span>' +
          '<span class="player-status" id="player-status">Idle</span>' +
          '<button class="player-icon-btn" id="record-btn" title="Record">\u23FA</button>' +
          '<button class="player-icon-btn" id="stats-btn" title="Stats (S)">\u2139</button>' +
          '<button class="player-icon-btn" id="stop-btn" title="Close">\u2715</button>' +
        '</div>' +
        '<div class="stats-overlay" id="stats-overlay"></div>' +
        '<div class="player-ctrl-bar" id="player-ctrl-bar">' +
          '<div class="player-seek-row" id="player-seek-row">' +
            '<div class="player-seek-track">' +
              '<div class="player-seek-buffered" id="seek-buffered"></div>' +
              '<div class="player-seek-played" id="seek-played"></div>' +
            '</div>' +
            '<div class="player-seek-thumb" id="seek-thumb"></div>' +
          '</div>' +
          '<div class="player-ctrl-btns">' +
            '<button class="player-ctrl-btn" id="play-pause-btn">\u25B6</button>' +
            '<span class="player-time" id="player-time">0:00 / 0:00</span>' +
            '<div style="flex:1"></div>' +
            '<div style="position:relative;display:inline-flex;align-items:center" id="audio-track-container">' +
              '<button class="player-ctrl-btn" id="audio-track-btn" title="Audio tracks" style="display:none"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg></button>' +
              '<div id="audio-track-menu" style="display:none;position:absolute;bottom:100%;right:0;margin-bottom:8px;background:rgba(20,20,20,0.95);border:1px solid rgba(255,255,255,0.15);border-radius:6px;min-width:220px;max-height:240px;overflow-y:auto;z-index:100;padding:4px 0"></div>' +
            '</div>' +
            '<button class="player-ctrl-btn" id="vol-btn">\uD83D\uDD0A</button>' +
            '<button class="player-ctrl-btn" id="kb-help-btn" title="Keyboard shortcuts (?)">?</button>' +
            '<button class="player-ctrl-btn" id="fs-btn">\u26F6</button>' +
          '</div>' +
        '</div>' +
        '<div class="kb-help-overlay" id="kb-help-overlay">' +
          '<div class="kb-help-inner">' +
            '<div class="kb-help-title">Keyboard Shortcuts</div>' +
            '<div class="kb-help-grid">' +
              '<div class="kb-help-section">Playback</div>' +
              '<span class="kb-help-key">Space</span><span class="kb-help-desc">Play / Pause</span>' +
              '<span class="kb-help-key">M</span><span class="kb-help-desc">Toggle mute</span>' +
              '<span class="kb-help-key">\u2191</span><span class="kb-help-desc">Volume up 10%</span>' +
              '<span class="kb-help-key">\u2193</span><span class="kb-help-desc">Volume down 10%</span>' +
              '<div class="kb-help-section">Seeking (VOD only)</div>' +
              '<span class="kb-help-key">\u2190</span><span class="kb-help-desc">Seek back 10 seconds</span>' +
              '<span class="kb-help-key">\u2192</span><span class="kb-help-desc">Seek forward 10 seconds</span>' +
              '<span class="kb-help-key">0 - 9</span><span class="kb-help-desc">Seek to 0% - 90% of duration</span>' +
              '<div class="kb-help-section">Display</div>' +
              '<span class="kb-help-key">F</span><span class="kb-help-desc">Toggle fullscreen</span>' +
              '<span class="kb-help-key">I</span><span class="kb-help-desc">Toggle stats overlay</span>' +
              '<span class="kb-help-key">?</span><span class="kb-help-desc">Show / hide this help</span>' +
              '<span class="kb-help-key">Esc</span><span class="kb-help-desc">Exit fullscreen or close player</span>' +
            '</div>' +
          '</div>' +
        '</div>' +
      '</div>';

    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    overlay.addEventListener('click', function(e) {
      if (e.target === overlay) closePlayerOverlay();
    });

    var escHandler = function(e) {
      if (!document.getElementById('player-overlay')) {
        document.removeEventListener('keydown', escHandler);
        return;
      }
      if (e.key === 'Escape') {
        closePlayerOverlay();
        document.removeEventListener('keydown', escHandler);
      }
    };
    document.addEventListener('keydown', escHandler);

    initPlayer(streamID);
  }

  function renderPlayer() {
    var pageEl = document.getElementById('page');
    if (!pageEl) return;
    var params = router.params || {};
    var streamID = params.streamID || playerState.currentStreamID;
    var name = params.name || streamID || 'Player';

    if (!streamID) {
      pageEl.innerHTML = '<h1 class="page-title">Player</h1>' +
        '<div class="empty-state">' + icons.empty + '<p>Select a stream or channel to play</p></div>';
      return;
    }

    pageEl.innerHTML =
      '<div class="player-wrapper" id="player-wrapper">' +
        '<video id="video-el" autoplay playsinline></video>' +
        '<div class="player-spinner" id="player-spinner">' +
          '<div class="spinner-ring"></div>' +
        '</div>' +
        '<div class="player-float-bar" id="player-float-bar">' +
          '<span class="player-title" id="player-title">' + esc(name) + '</span>' +
          '<span class="player-status" id="player-status">Idle</span>' +
          '<button class="player-icon-btn" id="record-btn" title="Record">\u23FA</button>' +
          '<button class="player-icon-btn" id="stats-btn" title="Stats (S)">\u2139</button>' +
          '<button class="player-icon-btn" id="stop-btn" title="Close">\u2715</button>' +
        '</div>' +
        '<div class="stats-overlay" id="stats-overlay"></div>' +
        '<div class="player-ctrl-bar" id="player-ctrl-bar">' +
          '<div class="player-seek-row" id="player-seek-row">' +
            '<div class="player-seek-track">' +
              '<div class="player-seek-buffered" id="seek-buffered"></div>' +
              '<div class="player-seek-played" id="seek-played"></div>' +
            '</div>' +
            '<div class="player-seek-thumb" id="seek-thumb"></div>' +
          '</div>' +
          '<div class="player-ctrl-btns">' +
            '<button class="player-ctrl-btn" id="play-pause-btn">\u25B6</button>' +
            '<span class="player-time" id="player-time">0:00 / 0:00</span>' +
            '<div style="flex:1"></div>' +
            '<div style="position:relative;display:inline-flex;align-items:center" id="audio-track-container">' +
              '<button class="player-ctrl-btn" id="audio-track-btn" title="Audio tracks" style="display:none"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg></button>' +
              '<div id="audio-track-menu" style="display:none;position:absolute;bottom:100%;right:0;margin-bottom:8px;background:rgba(20,20,20,0.95);border:1px solid rgba(255,255,255,0.15);border-radius:6px;min-width:220px;max-height:240px;overflow-y:auto;z-index:100;padding:4px 0"></div>' +
            '</div>' +
            '<button class="player-ctrl-btn" id="vol-btn">\uD83D\uDD0A</button>' +
            '<button class="player-ctrl-btn" id="kb-help-btn" title="Keyboard shortcuts (?)">?</button>' +
            '<button class="player-ctrl-btn" id="fs-btn">\u26F6</button>' +
          '</div>' +
        '</div>' +
        '<div class="kb-help-overlay" id="kb-help-overlay">' +
          '<div class="kb-help-inner">' +
            '<div class="kb-help-title">Keyboard Shortcuts</div>' +
            '<div class="kb-help-grid">' +
              '<div class="kb-help-section">Playback</div>' +
              '<span class="kb-help-key">Space</span><span class="kb-help-desc">Play / Pause</span>' +
              '<span class="kb-help-key">M</span><span class="kb-help-desc">Toggle mute</span>' +
              '<span class="kb-help-key">\u2191</span><span class="kb-help-desc">Volume up 10%</span>' +
              '<span class="kb-help-key">\u2193</span><span class="kb-help-desc">Volume down 10%</span>' +
              '<div class="kb-help-section">Seeking (VOD only)</div>' +
              '<span class="kb-help-key">\u2190</span><span class="kb-help-desc">Seek back 10 seconds</span>' +
              '<span class="kb-help-key">\u2192</span><span class="kb-help-desc">Seek forward 10 seconds</span>' +
              '<span class="kb-help-key">0 - 9</span><span class="kb-help-desc">Seek to 0% - 90% of duration</span>' +
              '<div class="kb-help-section">Display</div>' +
              '<span class="kb-help-key">F</span><span class="kb-help-desc">Toggle fullscreen</span>' +
              '<span class="kb-help-key">I</span><span class="kb-help-desc">Toggle stats overlay</span>' +
              '<span class="kb-help-key">?</span><span class="kb-help-desc">Show / hide this help</span>' +
              '<span class="kb-help-key">Esc</span><span class="kb-help-desc">Exit fullscreen or close player</span>' +
            '</div>' +
          '</div>' +
        '</div>' +
      '</div>';

    initPlayer(streamID);
  }

  async function initPlayer(streamID) {
    var videoEl = document.getElementById('video-el');
    if (!videoEl) return;
    playerState.videoEl = videoEl;

    var statusEl = document.getElementById('player-status');
    var spinner = document.getElementById('player-spinner');
    if (statusEl) { statusEl.style.color = '#ffa726'; statusEl.textContent = 'Buffering...'; }
    if (spinner) spinner.style.display = 'flex';

    videoEl.addEventListener('playing', function() { if (spinner) spinner.style.display = 'none'; if (statusEl) { statusEl.textContent = 'Playing'; statusEl.style.color = '#34d399'; } });
    videoEl.addEventListener('waiting', function() { if (spinner) spinner.style.display = 'flex'; if (statusEl) { statusEl.textContent = 'Buffering...'; statusEl.style.color = '#ffa726'; } });
    videoEl.addEventListener('seeked', function() { if (spinner) spinner.style.display = 'none'; });

    var isRecording = streamID.indexOf('rec:') === 0;
    var recID = isRecording ? streamID.substring(4) : null;

    try {
      var resp;
      var deliveryPref = playerState.deliveryOverride || null;
      if (isRecording) {
        resp = await api.post('/api/recordings/completed/' + recID + '/play', deliveryPref ? { delivery: deliveryPref } : undefined);
      } else {
        resp = await api.post('/api/play/' + streamID, deliveryPref ? { delivery: deliveryPref } : undefined);
      }
      if (!resp.ok) {
        var errData = await resp.json().catch(function() { return {}; });
        if (statusEl) { statusEl.style.color = '#ff6b6b'; statusEl.textContent = 'Failed'; }
        if (spinner) spinner.style.display = 'none';
        toast('Failed to start playback: ' + (errData.error || resp.statusText), 'error');
        return;
      }
      var data = await resp.json();
      playerState.sessionID = data.session_id;

      var delivery = data.delivery || 'hls';
      var endpoints = data.endpoints || {};
      playerState.delivery = delivery;
      playerState.decision = data.decision || {};
      playerState.probeInfo = data.probe_info || {};
      playerState.deliverySwitchable = !!data.delivery_switchable;
      playerState.sourceType = data.source_type || null;

      showDeliverySwitcher(delivery, streamID);

      if (delivery === 'hls') {
        var hlsUrl = endpoints.playlist || (isRecording
          ? '/api/recordings/completed/' + recID + '/play/hls/playlist.m3u8'
          : '/api/play/' + streamID + '/hls/playlist.m3u8');
        playerState.hlsUrl = hlsUrl;
        endpoints.playlist = hlsUrl;
      }

      var plugin = PlayerRegistry.get(delivery);
      if (!plugin) plugin = PlayerRegistry.get('mse');
      if (!plugin || !plugin.isSupported()) {
        toast('No supported player for delivery mode: ' + delivery, 'error');
        return;
      }
      var instance = plugin.create(videoEl);
      playerState.activePlayer = instance;
      instance.start(endpoints, streamID);
    } catch (e) {
      if (statusEl) { statusEl.style.color = '#ff6b6b'; statusEl.textContent = 'Error'; }
      if (spinner) spinner.style.display = 'none';
      toast('Playback error: ' + e.message, 'error');
      return;
    }

    var seekPath = isRecording ? '/api/recordings/completed/' + recID + '/seek' : '/api/play/' + streamID + '/seek';
    bindPlayerControls(videoEl, streamID, seekPath);
    startBufferWatch(videoEl);
    startStatsWatch(videoEl);
    // Pre-fetch signal for SAT>IP sources so it's ready when stats panel opens
    if ((playerState.sourceType === 'satip' || playerState.sourceType === 'hdhr') && playerState.currentStreamID) {
      api.get('/api/play/' + playerState.currentStreamID + '/signal')
        .then(function(resp) { return resp.ok ? resp.json() : null; })
        .then(function(data) { if (data) playerState.signalInfo = data; })
        .catch(function() {});
    }
  }

  function showDeliverySwitcher(currentDelivery, streamID) {
    var floatBar = document.getElementById('player-float-bar');
    if (!floatBar) return;
    var existing = document.getElementById('delivery-switcher');
    if (existing) existing.remove();

    var available = PlayerRegistry.available();
    if (available.length <= 1) return;

    var labels = { mse: 'MSE', hls: 'HLS', dash: 'DASH', webrtc: 'WebRTC', stream: 'Direct' };
    var sel = document.createElement('select');
    sel.id = 'delivery-switcher';
    sel.className = 'delivery-switcher';
    sel.style.cssText = 'background:rgba(0,0,0,0.6);color:#fff;border:1px solid rgba(255,255,255,0.2);border-radius:4px;padding:2px 6px;font-size:12px;cursor:pointer;margin-left:8px;pointer-events:auto;';
    for (var i = 0; i < available.length; i++) {
      var opt = document.createElement('option');
      opt.value = available[i];
      opt.textContent = labels[available[i]] || available[i];
      if (available[i] === currentDelivery) opt.selected = true;
      sel.appendChild(opt);
    }
    sel.addEventListener('click', function(e) { e.stopPropagation(); });
    sel.addEventListener('mousedown', function(e) { e.stopPropagation(); });
    sel.addEventListener('change', async function(e) {
      e.stopPropagation();
      var newDelivery = sel.value;
      if (newDelivery === playerState.delivery) return;
      playerState.deliveryOverride = newDelivery;
      if (playerState.activePlayer) playerState.activePlayer.stop();
      if (playerState.currentStreamID) {
        await api.del('/api/play/' + playerState.currentStreamID).catch(function() {});
      }
      initPlayer(streamID);
    });
    var stopBtn = document.getElementById('stop-btn');
    if (stopBtn) {
      floatBar.insertBefore(sel, stopBtn);
    } else {
      floatBar.appendChild(sel);
    }
  }

  var HLSPlayer = {
    label: 'HLS',
    isSupported: function() { return typeof videojs !== 'undefined' || document.createElement('video').canPlayType('application/vnd.apple.mpegurl') !== ''; },
    serverParams: function() { return {delivery: 'hls'}; },
    create: function(videoEl) {
      var vjsPlayer = null;
      var lastUrl = null;
      var lastStreamID = null;
      return {
        start: function(endpoints, streamID) {
          lastStreamID = streamID;
          var url = endpoints.playlist || ('/api/play/' + streamID + '/hls/playlist.m3u8');
          lastUrl = url;
          if (vjsPlayer) { vjsPlayer.dispose(); vjsPlayer = null; }
          playerState.hlsInstance = null;

          if (typeof videojs !== 'undefined') {
            // Ensure the video element has an id for video.js
            if (!videoEl.id) videoEl.id = 'mediahub-vjs-' + Date.now();

            // Configure VHS xhr to inject auth token
            if (typeof videojs.Vhs !== 'undefined' && videojs.Vhs.xhr) {
              videojs.Vhs.xhr.beforeRequest = function(options) {
                if (api.token) {
                  if (!options.headers) options.headers = {};
                  options.headers['Authorization'] = 'Bearer ' + api.token;
                }
                return options;
              };
            }

            vjsPlayer = videojs(videoEl, {
              controls: false,
              autoplay: true,
              preload: 'auto',
              html5: {
                vhs: {
                  overrideNative: true,
                  enableLowInitialPlaylist: false
                }
              }
            });

            vjsPlayer.src({ src: url, type: 'application/x-mpegURL' });

            vjsPlayer.ready(function() {
              vjsPlayer.play().catch(function() {});
            });

            vjsPlayer.on('error', function() {
              var err = vjsPlayer.error();
              console.error('[HLS] video.js error:', err && err.message);
              handleRetry();
            });

            playerState.hlsInstance = vjsPlayer;
          } else if (videoEl.canPlayType('application/vnd.apple.mpegurl')) {
            videoEl.src = url;
            videoEl.play().catch(function() {});
          } else {
            console.error('[HLS] no HLS support available');
          }
        },
        stop: function() {
          if (vjsPlayer) {
            try { vjsPlayer.dispose(); } catch(e) { console.warn('[HLS] dispose error:', e); }
            vjsPlayer = null;
          }
          playerState.hlsInstance = null;
        },
        retry: function() {
          if (lastUrl && videoEl) {
            this.start({ playlist: lastUrl }, lastStreamID);
          }
        }
      };
    }
  };
  PlayerRegistry.register('hls', HLSPlayer);

  function handleRetry() {
    if (!playerState.currentStreamID) return;
    var statusEl = document.getElementById('player-status');

    if (playerState.retryCount >= MAX_RETRIES) {
      if (statusEl) {
        statusEl.style.color = '#ff6b6b';
        statusEl.innerHTML = '';
        statusEl.textContent = 'Errored ';
        var retryLink = document.createElement('a');
        retryLink.textContent = 'Retry';
        retryLink.href = '#';
        retryLink.style.cssText = 'color:#4fc3f7;cursor:pointer;text-decoration:underline;';
        retryLink.onclick = function(e) {
          e.preventDefault();
          playerState.retryCount = 0;
          handleRetry();
        };
        statusEl.appendChild(retryLink);
      }
      return;
    }

    playerState.retryCount++;
    if (statusEl) {
      statusEl.style.color = '#ffa726';
      statusEl.textContent = 'Retrying... (' + playerState.retryCount + '/' + MAX_RETRIES + ')';
    }

    var delay = Math.min(2000 * playerState.retryCount, 6000);
    if (playerState.retryTimeout) clearTimeout(playerState.retryTimeout);
    playerState.retryTimeout = setTimeout(function() {
      if (!playerState.currentStreamID || !playerState.videoEl) return;
      if (playerState.activePlayer && playerState.activePlayer.retry) {
        playerState.activePlayer.retry();
      } else {
        initPlayer(playerState.currentStreamID);
      }
    }, delay);
  }

  function detectVideoCodec(buf) {
    var d = new Uint8Array(buf);
    for (var i = 0; i < d.length - 8; i++) {
      if (d[i] === 0x68 && d[i+1] === 0x76 && d[i+2] === 0x63 && d[i+3] === 0x43) {
        var profileIdc = d[i+5] & 0x1F;
        var tierFlag = (d[i+5] >> 5) & 1;
        var levelIdc = d[i+16];
        var tier = tierFlag ? 'H' : 'L';
        // Detect whether the sample entry uses hev1 or hvc1 tag
        var hevcTag = 'hvc1';
        for (var h = 0; h < i && h < d.length - 4; h++) {
          if (d[h] === 0x68 && d[h+1] === 0x65 && d[h+2] === 0x76 && d[h+3] === 0x31) { hevcTag = 'hev1'; break; }
          if (d[h] === 0x68 && d[h+1] === 0x76 && d[h+2] === 0x63 && d[h+3] === 0x31) { hevcTag = 'hvc1'; break; }
        }
        return hevcTag + '.' + profileIdc + '.4.' + tier + String(levelIdc);
      }
      if (d[i] === 0x61 && d[i+1] === 0x76 && d[i+2] === 0x63 && d[i+3] === 0x43) {
        var hex = function(n) { return n.toString(16).padStart(2, '0'); };
        return 'avc1.' + hex(d[i+5]) + hex(d[i+6]) + hex(d[i+7]);
      }
      if (d[i] === 0x61 && d[i+1] === 0x76 && d[i+2] === 0x31 && d[i+3] === 0x43) {
        var av1cOff = i + 4;
        var profile = (d[av1cOff + 1] >> 5) & 0x07;
        var level = d[av1cOff + 1] & 0x1F;
        var tierBit = (d[av1cOff + 2] >> 7) & 1;
        var highBd = (d[av1cOff + 2] >> 6) & 1;
        var twelveBit = (d[av1cOff + 2] >> 5) & 1;
        var bd = highBd ? (twelveBit ? 12 : 10) : 8;
        var tierStr = tierBit ? 'H' : 'M';
        return 'av01.' + profile + '.' + String(level).padStart(2, '0') + tierStr + '.' + (bd <= 8 ? '08' : String(bd));
      }
    }
    return 'avc1.640028';
  }

  function mseWaitUpdate(sb) {
    return new Promise(function(resolve, reject) {
      if (!sb.updating) { resolve(); return; }
      sb.addEventListener('updateend', resolve, {once: true});
      sb.addEventListener('error', function() { reject(new Error('append error')); }, {once: true});
    });
  }

  function mseEvictBuffer(sb, vidEl) {
    if (!sb || !vidEl || sb.updating) return Promise.resolve();
    var keepBehind = 10;
    if (sb.buffered.length > 0 && vidEl.currentTime - sb.buffered.start(0) > keepBehind) {
      var removeEnd = vidEl.currentTime - keepBehind;
      sb.remove(sb.buffered.start(0), removeEnd);
      return mseWaitUpdate(sb);
    }
    return Promise.resolve();
  }

  var MSEPlayer = {
    label: 'MSE (fMP4)',
    isSupported: function() { return !!window.MediaSource; },
    serverParams: function() { return {delivery: 'mse'}; },
    create: function(videoEl) {
      var mseState = null;
      var lastStreamID = null;
      var lastEndpoints = null;

      function mseCleanup() {
        if (!mseState) return;
        mseState.stopped = true;
        mseState.appendQueues.video = [];
        mseState.appendQueues.audio = [];
        mseState.appending.video = false;
        mseState.appending.audio = false;
        if (mseState.mediaSource && mseState.mediaSource.readyState === 'open') {
          try { mseState.mediaSource.endOfStream(); } catch(e) {}
        }
        mseState.mediaSource = null;
        mseState.videoSB = null;
        mseState.audioSB = null;
        playerState.mseState = null;
        mseState = null;
      }

      function startMSEInternal(vidEl, streamID, endpoints) {
        if (!('MediaSource' in window)) {
          toast('Browser does not support MSE playback', 'error');
          return;
        }

        var basePath = '/api/play/' + streamID + '/mse/';
        var videoInitUrl = (endpoints && endpoints.video_init) || (basePath + 'video/init');
        var audioInitUrl = (endpoints && endpoints.audio_init) || (basePath + 'audio/init');
        var videoSegUrl = (endpoints && endpoints.video_segment) || (basePath + 'video/segment');
        var audioSegUrl = (endpoints && endpoints.audio_segment) || (basePath + 'audio/segment');
        var debugUrl = basePath + 'debug';

        mseState = {
          stopped: false,
          mediaSource: null,
          videoSB: null,
          audioSB: null,
          appendQueues: { video: [], audio: [] },
          appending: { video: false, audio: false },
          playStarted: false,
          debugInfo: null
        };
        playerState.mseState = mseState;

        var statusEl = document.getElementById('player-status');

        function authHeaders() {
          return { 'Authorization': 'Bearer ' + api.token };
        }

        function processQueue(track) {
          var sb = track === 'video' ? mseState.videoSB : mseState.audioSB;
          if (!sb || mseState.appending[track]) return;
          mseState.appending[track] = true;
          function next() {
            if (mseState.appendQueues[track].length === 0) { mseState.appending[track] = false; return; }
            var data = mseState.appendQueues[track].shift();
            mseEvictBuffer(sb, vidEl).then(function() {
              return mseWaitUpdate(sb);
            }).then(function() {
              sb.appendBuffer(data);
              return mseWaitUpdate(sb);
            }).then(next).catch(function(e) {
              if (e.name === 'QuotaExceededError') {
                mseEvictBuffer(sb, vidEl).then(function() {
                  mseState.appendQueues[track].unshift(data);
                  setTimeout(next, 500);
                });
                return;
              }
              mseState.appending[track] = false;
              var errMsg = e.message || String(e);
              if (errMsg.indexOf('append') >= 0 || errMsg.indexOf('CHUNK_DEMUXER') >= 0 || errMsg.indexOf('error') >= 0) {
                var pStatus = document.getElementById('player-status');
                if (pStatus) {
                  pStatus.style.color = '#ff6b6b';
                  pStatus.innerHTML = '';
                  pStatus.textContent = 'Playback error ';
                  var retryLink = document.createElement('a');
                  retryLink.textContent = 'Retry';
                  retryLink.href = '#';
                  retryLink.style.cssText = 'color:#4fc3f7;cursor:pointer;text-decoration:underline;';
                  retryLink.onclick = function(ev) {
                    ev.preventDefault();
                    playerState.retryCount = 0;
                    handleRetry();
                  };
                  pStatus.appendChild(retryLink);
                }
                var pSpinner = document.getElementById('player-spinner');
                if (pSpinner) pSpinner.style.display = 'none';
              }
            });
          }
          next();
        }

        function fetchInitWithRetry(url, maxRetries, delayMs) {
          var attempt = 0;
          function tryFetch() {
            if (mseState.stopped) return Promise.reject(new Error('stopped'));
            return fetch(url, { headers: authHeaders(), cache: 'no-store' }).then(function(r) {
              if (r.ok) return r.arrayBuffer();
              if ((r.status === 503 || r.status === 404) && attempt < maxRetries) {
                attempt++;
                if (statusEl) statusEl.textContent = 'Waiting for pipeline... (' + attempt + '/' + maxRetries + ')';
                return new Promise(function(resolve) { setTimeout(resolve, delayMs); }).then(tryFetch);
              }
              throw new Error('HTTP ' + r.status);
            });
          }
          return tryFetch();
        }

        function pollSegments(track, baseUrl, gen) {
          var seq = 1;
          function poll() {
            if (!mseState || mseState.stopped || !playerState.currentStreamID) return;
            fetch(baseUrl + '?seq=' + seq + '&gen=' + gen, { headers: authHeaders(), cache: 'no-store' })
              .then(function(resp) {
                if (mseState.stopped) return;
                if (resp.status === 410) {
                  if (statusEl) { statusEl.textContent = 'Reconnecting...'; statusEl.style.color = '#ffa726'; }
                  mseState.stopped = true;
                  mseCleanup();
                  setTimeout(function() { startMSEInternal(vidEl, streamID, endpoints); }, 1000);
                  return;
                }
                if (!resp.ok) {
                  setTimeout(poll, 300);
                  return;
                }
                return resp.arrayBuffer();
              })
              .then(function(buf) {
                if (!buf || mseState.stopped) return;
                mseState.appendQueues[track].push(new Uint8Array(buf));
                processQueue(track);
                seq++;
                if (track === 'video' && seq === 2 && !mseState.playStarted) {
                  mseState.playStarted = true;
                  var playTimer = null;
                  var tryPlay = function() {
                    playTimer = null;
                    if (!mseState || mseState.stopped || !vidEl.parentNode) return;
                    try {
                      if (mseState.videoSB && mseState.videoSB.buffered.length > 0) {
                        var end = mseState.videoSB.buffered.end(mseState.videoSB.buffered.length - 1);
                        vidEl.currentTime = Math.max(end - 1, mseState.videoSB.buffered.start(0));
                        vidEl.play().catch(function() {
                          if (!mseState || mseState.stopped || !vidEl.parentNode) return;
                          playTimer = setTimeout(tryPlay, 300);
                        });
                      } else {
                        playTimer = setTimeout(tryPlay, 100);
                      }
                    } catch(e) {}
                  };
                  playTimer = setTimeout(tryPlay, 100);
                }
                setTimeout(poll, 50);
              })
              .catch(function() {
                if (mseState && !mseState.stopped) setTimeout(poll, 1000);
              });
          }
          poll();
        }

        if (statusEl) { statusEl.textContent = 'Initializing...'; statusEl.style.color = '#ffa726'; }

        fetch(debugUrl, { headers: authHeaders(), cache: 'no-store' })
          .then(function(r) { return r.ok ? r.json() : {}; })
          .then(function(debugInfo) {
            mseState.debugInfo = debugInfo;
            var hasVideo = !debugInfo || debugInfo.has_video_init !== false;
            var audioOnly = debugInfo && debugInfo.has_video_init === false && debugInfo.has_audio_init !== false;
            if (hasVideo) {
              return fetchInitWithRetry(videoInitUrl, 60, 500);
            }
            return null;
          })
          .then(function(videoBuf) {
            if (mseState.stopped) return;
            var audioCodecStr = (mseState.debugInfo && mseState.debugInfo.audio_codec) || 'mp4a.40.2';
            var audioMime = 'audio/mp4; codecs="' + audioCodecStr + '"';
            var hasVideo = videoBuf != null;
            var videoMime = '';
            if (hasVideo) {
              var videoCodec = detectVideoCodec(videoBuf);
              videoMime = 'video/mp4; codecs="' + videoCodec + '"';
            }

            mseState.mediaSource = new MediaSource();
            vidEl.src = URL.createObjectURL(mseState.mediaSource);

            mseState.mediaSource.addEventListener('sourceopen', function() {
              var probe = playerState.probeInfo || {};
              if (probe.duration_ms > 0) {
                try { mseState.mediaSource.duration = probe.duration_ms / 1000; } catch(e) {}
              }
              if (hasVideo) {
                mseState.videoSB = mseState.mediaSource.addSourceBuffer(videoMime);
                mseState.videoSB.mode = 'segments';
              }

              var hasAudio = !mseState.debugInfo || mseState.debugInfo.has_audio_init !== false;
              if (hasAudio) {
                try {
                  mseState.audioSB = mseState.mediaSource.addSourceBuffer(audioMime);
                  mseState.audioSB.mode = 'segments';
                } catch (e) { hasAudio = false; }
              }

              var videoReady = hasVideo
                ? mseWaitUpdate(mseState.videoSB).then(function() {
                    mseState.videoSB.appendBuffer(videoBuf);
                    return mseWaitUpdate(mseState.videoSB);
                  })
                : Promise.resolve();
              videoReady.then(function() {
                if (hasAudio && mseState.audioSB) {
                  return fetchInitWithRetry(audioInitUrl, 30, 500).then(function(audioBuf) {
                    mseState.audioSB.appendBuffer(audioBuf);
                    return mseWaitUpdate(mseState.audioSB);
                  }).catch(function() {});
                }
              }).then(function() {
                if (statusEl) { statusEl.textContent = 'Buffering...'; statusEl.style.color = '#ffa726'; }
                var gen = String((mseState.debugInfo && mseState.debugInfo.generation) || 1);
                if (hasVideo && mseState.videoSB) pollSegments('video', videoSegUrl, gen);
                if (hasAudio && mseState.audioSB) pollSegments('audio', audioSegUrl, gen);
              }).catch(function() {
                if (statusEl) { statusEl.textContent = 'MSE init error'; statusEl.style.color = '#ff6b6b'; }
              });
            }, {once: true});
          })
          .catch(function(e) {
            if (statusEl) {
              statusEl.style.color = '#ff6b6b';
              var msg = e.message || String(e);
              if (msg.indexOf('HTTP 503') >= 0) statusEl.textContent = 'Pipeline not ready';
              else if (msg.indexOf('HTTP 404') >= 0) statusEl.textContent = 'Session not found';
              else statusEl.textContent = 'MSE Error: ' + msg;
            }
            handleRetry();
          });
      }

      return {
        start: function(endpoints, streamID) {
          lastStreamID = streamID;
          lastEndpoints = endpoints;
          startMSEInternal(videoEl, streamID, endpoints);
        },
        stop: function() {
          mseCleanup();
        },
        retry: function() {
          if (lastStreamID) {
            mseCleanup();
            initPlayer(lastStreamID);
          }
        }
      };
    }
  };
  PlayerRegistry.register('mse', MSEPlayer);

  var DirectPlayer = {
    label: 'Direct',
    isSupported: function() { return true; },
    serverParams: function() { return {delivery: 'stream'}; },
    create: function(videoEl) {
      var lastStreamID = null;
      var lastEndpoints = null;
      return {
        start: function(endpoints, streamID) {
          lastStreamID = streamID;
          lastEndpoints = endpoints;
          videoEl.src = (endpoints && endpoints.stream) || ('/api/play/' + streamID + '/stream');
          videoEl.play().catch(function() {});
        },
        stop: function() { videoEl.pause(); videoEl.src = ''; },
        retry: function() {
          if (lastStreamID) {
            this.start(lastEndpoints, lastStreamID);
          }
        }
      };
    }
  };
  PlayerRegistry.register('stream', DirectPlayer);

  var DASHPlayer = {
    label: 'DASH',
    isSupported: function() { return !!window.MediaSource && typeof videojs !== 'undefined'; },
    serverParams: function() { return {delivery: 'dash'}; },
    create: function(videoEl) {
      var vjsPlayer = null;
      var lastStreamID = null;
      var lastEndpoints = null;
      return {
        start: function(endpoints, streamID) {
          lastStreamID = streamID;
          lastEndpoints = endpoints;
          var url = (endpoints && endpoints.manifest) || ('/api/play/' + streamID + '/dash/manifest.mpd');
          if (vjsPlayer) { vjsPlayer.dispose(); vjsPlayer = null; }

          console.log('[DASH] videojs available:', typeof videojs !== 'undefined');
          if (typeof videojs !== 'undefined') {
            if (!videoEl.id) videoEl.id = 'mediahub-vjs-' + Date.now();

            // Configure VHS xhr to inject auth token
            if (typeof videojs.Vhs !== 'undefined' && videojs.Vhs.xhr) {
              videojs.Vhs.xhr.beforeRequest = function(options) {
                if (api.token) {
                  if (!options.headers) options.headers = {};
                  options.headers['Authorization'] = 'Bearer ' + api.token;
                }
                return options;
              };
            }

            vjsPlayer = videojs(videoEl, {
              controls: false,
              autoplay: true,
              preload: 'auto',
              html5: {
                vhs: {
                  overrideNative: true
                }
              }
            });

            vjsPlayer.src({ src: url, type: 'application/dash+xml' });

            vjsPlayer.ready(function() {
              console.log('[DASH] video.js player ready');
              vjsPlayer.play().catch(function() {});
            });

            vjsPlayer.on('error', function() {
              var err = vjsPlayer.error();
              console.error('[DASH] video.js error:', err && err.message);
              handleRetry();
            });
          }
        },
        stop: function() {
          if (vjsPlayer) {
            try { vjsPlayer.dispose(); } catch(e) { console.warn('[DASH] dispose error:', e); }
            vjsPlayer = null;
          }
        },
        retry: function() {
          if (vjsPlayer) {
            try { vjsPlayer.dispose(); } catch(e) {}
            vjsPlayer = null;
          }
          if (lastStreamID) { this.start(lastEndpoints, lastStreamID); }
        }
      };
    }
  };
  PlayerRegistry.register('dash', DASHPlayer);

  var WebRTCPlayer = {
    label: 'WebRTC',
    isSupported: function() { return !!window.RTCPeerConnection; },
    serverParams: function() { return {delivery: 'webrtc'}; },
    create: function(videoEl) {
      var pc = null;
      var cachedEndpoints = null;
      var cachedStreamID = null;
      var retryCount = 0;
      var maxRetries = 3;
      var retryTimer = null;
      var statsTimer = null;
      var stopped = false;
      var self;

      function cleanup() {
        if (retryTimer) { clearTimeout(retryTimer); retryTimer = null; }
        if (statsTimer) { clearInterval(statsTimer); statsTimer = null; }
        if (pc) {
          pc.ontrack = null;
          pc.oniceconnectionstatechange = null;
          pc.onconnectionstatechange = null;
          pc.close();
          pc = null;
        }
        videoEl.srcObject = null;
      }

      function scheduleRetry() {
        if (stopped || retryCount >= maxRetries) return;
        retryCount++;
        var delay = Math.min(1000 * Math.pow(2, retryCount - 1), 8000);
        retryTimer = setTimeout(function() {
          retryTimer = null;
          if (!stopped && cachedEndpoints && cachedStreamID) {
            self.start(cachedEndpoints, cachedStreamID);
          }
        }, delay);
      }

      self = {
        start: function(endpoints, streamID) {
          cleanup();
          if (stopped) return;
          cachedEndpoints = endpoints;
          cachedStreamID = streamID;
          var whepUrl = endpoints.whep || ('/api/play/' + streamID + '/webrtc/whep');
          pc = new RTCPeerConnection({
            iceServers: [{urls: 'stun:stun.l.google.com:19302'}],
            iceCandidatePoolSize: 1
          });
          var probe = playerState.probeInfo || {};
          if (probe.video) pc.addTransceiver('video', {direction: 'recvonly'});
          pc.addTransceiver('audio', {direction: 'recvonly'});

          pc.ontrack = function(e) {
            if (e.streams && e.streams[0]) {
              videoEl.srcObject = e.streams[0];
              e.streams[0].onremovetrack = function(evt) {
                var stream = evt.target;
                if (!stopped && stream.getTracks().length === 0) scheduleRetry();
              };
            }
          };

          pc.oniceconnectionstatechange = function() {
            if (!pc) return;
            var state = pc.iceConnectionState;
            if (state === 'connected' || state === 'completed') {
              retryCount = 0;
            } else if (state === 'disconnected') {
              setTimeout(function() {
                if (pc && pc.iceConnectionState === 'disconnected' && !stopped) {
                  scheduleRetry();
                }
              }, 2000);
            } else if (state === 'failed') {
              if (!stopped) scheduleRetry();
            }
          };

          pc.onconnectionstatechange = function() {
            if (!pc) return;
            if (pc.connectionState === 'failed') {
              if (!stopped) scheduleRetry();
            }
          };

          pc.createOffer().then(function(offer) {
            return pc.setLocalDescription(offer);
          }).then(function() {
            return fetch(whepUrl, {
              method: 'POST',
              headers: {'Content-Type': 'application/sdp', 'Authorization': 'Bearer ' + (api && api.token || '')},
              body: pc.localDescription.sdp
            });
          }).then(function(r) {
            if (!r.ok) throw new Error('WHEP signalling failed: ' + r.status);
            return r.text();
          }).then(function(sdp) {
            if (!pc || stopped) return;
            return pc.setRemoteDescription({type: 'answer', sdp: sdp});
          }).then(function() {
            if (!pc || stopped) return;
            videoEl.play().catch(function(){});
          }).catch(function(err) {
            if (!stopped) scheduleRetry();
          });
        },
        stop: function() {
          stopped = true;
          cleanup();
        },
        retry: function() {
          stopped = false;
          retryCount = 0;
          cleanup();
          if (cachedEndpoints && cachedStreamID) {
            self.start(cachedEndpoints, cachedStreamID);
          }
        }
      };
      return self;
    }
  };
  PlayerRegistry.register('webrtc', WebRTCPlayer);

  function startBufferWatch(videoEl) {
    if (playerState.bufferWatchInterval) clearInterval(playerState.bufferWatchInterval);
    playerState.bufferWatchInterval = setInterval(function() {
      try {
        if (!videoEl || !videoEl.parentNode || videoEl.paused || !videoEl.buffered.length) return;
        var buffered = videoEl.buffered;
        var ahead = 0;
        for (var i = 0; i < buffered.length; i++) {
          if (buffered.start(i) <= videoEl.currentTime && videoEl.currentTime <= buffered.end(i)) {
            ahead = buffered.end(i) - videoEl.currentTime;
            break;
          }
        }
        var rate;
        if (ahead < 6) {
          rate = 0.9;
        } else if (ahead < 8) {
          rate = 0.95;
        } else if (ahead < 9) {
          rate = 0.99;
        } else {
          rate = 1.0;
        }
        if (videoEl.playbackRate !== rate) videoEl.playbackRate = rate;
      } catch(e) {}
    }, 250);
  }

  function startStatsWatch(videoEl) {
    if (playerState.statsInterval) clearInterval(playerState.statsInterval);
    if (playerState.signalInterval) clearInterval(playerState.signalInterval);
    playerState.statsInterval = setInterval(function() {
      var overlay = document.getElementById('stats-overlay');
      if (!overlay || !overlay.classList.contains('visible')) return;
      updateStats(videoEl, overlay);
    }, 500);
    // Poll signal info every 5 seconds for satip sources when stats visible
    if (playerState.sourceType === 'satip' || playerState.sourceType === 'hdhr') {
      playerState.signalInterval = setInterval(function() {
        var overlay = document.getElementById('stats-overlay');
        if (!overlay || !overlay.classList.contains('visible')) return;
        if (!playerState.currentStreamID) return;
        api.get('/api/play/' + playerState.currentStreamID + '/signal')
          .then(function(resp) { return resp.ok ? resp.json() : null; })
          .then(function(data) { if (data) playerState.signalInfo = data; })
          .catch(function() {});
      }, 5000);
    }
  }

  function updateStats(videoEl, overlay) {
    if (!videoEl) return;
    var buf = 0;
    if (videoEl.buffered.length) {
      for (var i = 0; i < videoEl.buffered.length; i++) {
        if (videoEl.buffered.start(i) <= videoEl.currentTime && videoEl.currentTime <= videoEl.buffered.end(i)) {
          buf = videoEl.buffered.end(i) - videoEl.currentTime;
          break;
        }
      }
    }

    var w = videoEl.videoWidth || 0;
    var h = videoEl.videoHeight || 0;
    var lines = [];

    var delivery = playerState.delivery || 'unknown';
    var mse = playerState.mseState;
    var debugInfo = mse && mse.debugInfo;
    var decision = playerState.decision || {};
    var probe = playerState.probeInfo || {};

    var inVideo = (probe.video && probe.video.codec) || decision.video_codec || '';
    var inAudio = (probe.audio && probe.audio.codec) || '';
    var inRes = (probe.video && probe.video.width) ? probe.video.width + 'x' + probe.video.height : '';
    var inParts = [];
    if (inVideo) inParts.push(esc(inVideo) + (inRes ? ' ' + inRes : ''));
    if (inAudio) inParts.push(esc(inAudio));
    if (inParts.length > 0) {
      lines.push('In: ' + inParts.join(' / '));
    }

    var outParts = [];
    if (inVideo) outParts.push(decision.needs_transcode ? esc(String(decision.video_codec)) : 'copy');
    if (inAudio || decision.needs_audio_transcode) outParts.push(decision.needs_audio_transcode ? esc(String(decision.audio_codec)) : 'copy');
    lines.push('Out: ' + esc(delivery.toUpperCase()) + ' ' + outParts.join(' / ') + (w > 0 ? ' ' + w + 'x' + h : ''));

    var bufColor = buf > 8 ? '#4caf50' : buf > 4 ? '#ffb300' : '#ff6b6b';
    var playbackParts = ['<span style="color:' + bufColor + '">buf ' + buf.toFixed(1) + 's</span>'];

    if (videoEl.playbackRate !== 1.0) {
      var rateColor = videoEl.playbackRate < 0.93 ? '#ff6b6b' : '#ffb300';
      playbackParts.push('<span style="color:' + rateColor + '">' + videoEl.playbackRate.toFixed(2) + 'x rate</span>');
    }

    var quality = videoEl.getVideoPlaybackQuality ? videoEl.getVideoPlaybackQuality() : null;
    if (quality) {
      var dropColor = quality.droppedVideoFrames > 50 ? '#ff6b6b' : quality.droppedVideoFrames > 0 ? '#ffb300' : '#4caf50';
      playbackParts.push('<span style="color:' + dropColor + '">' + quality.droppedVideoFrames + ' dropped</span>');
    }

    lines.push(playbackParts.join(' | '));

    if (playerState.hlsInstance && typeof playerState.hlsInstance.tech === 'function') {
      try {
        var tech = playerState.hlsInstance.tech({ IWillNotUseThisInPlugins: true });
        if (tech && tech.vhs) {
          var playlists = tech.vhs.playlists;
          var media = playlists && playlists.media && playlists.media();
          if (media && media.attributes && media.attributes.BANDWIDTH) {
            lines.push('Bitrate: ' + (media.attributes.BANDWIDTH / 1000).toFixed(0) + ' kbps');
          }
          var master = playlists && playlists.master;
          if (master && master.playlists) {
            var idx = master.playlists.indexOf(media);
            lines.push('Level: ' + (idx + 1) + '/' + master.playlists.length);
          }
        }
      } catch(e) {}
    }

    if (debugInfo) {
      var mseInfo = [];
      if (debugInfo.video_segments != null) mseInfo.push('vseg=' + debugInfo.video_segments);
      if (debugInfo.audio_segments != null) mseInfo.push('aseg=' + debugInfo.audio_segments);
      if (debugInfo.generation) mseInfo.push('gen=' + debugInfo.generation);
      if (mseInfo.length) lines.push(mseInfo.join(' | '));
    }

    // Two-column layout: left = playback info, right = DVB signal (matching tvproxy)
    var right = [];
    var sig = playerState.signalInfo;
    if (sig && sig.available) {
      var lockColor = sig.lock ? '#4caf50' : '#ff6b6b';
      var lvl = sig.level_pct || 0;
      var qlt = sig.quality_pct || 0;
      var lvlColor = lvl > 60 ? '#4caf50' : lvl > 30 ? '#ffb300' : '#ff6b6b';
      var qltColor = qlt > 60 ? '#4caf50' : qlt > 30 ? '#ffb300' : '#ff6b6b';
      function sigBar(pct, color) {
        return '<span style="display:inline-block;width:50px;height:5px;background:#333;vertical-align:middle;border-radius:2px">'
          + '<span style="display:block;width:' + pct + '%;height:100%;background:' + color + ';border-radius:2px"></span></span>';
      }
      right.push('<span style="color:' + lockColor + '">' + (sig.lock ? 'Locked' : 'No Lock') + '</span>');
      right.push('Lvl ' + sigBar(lvl, lvlColor) + ' <span style="color:' + lvlColor + '">' + lvl + '%</span>');
      right.push('Qlt ' + sigBar(qlt, qltColor) + ' <span style="color:' + qltColor + '">' + qlt + '%</span>');
      if (sig.symbol_pct != null) {
        var symColor = sig.symbol_pct > 60 ? '#4caf50' : sig.symbol_pct > 30 ? '#ffb300' : '#ff6b6b';
        right.push('Sym ' + sigBar(sig.symbol_pct, symColor) + ' <span style="color:' + symColor + '">' + sig.symbol_pct + '%</span>');
      }
      if (sig.ber != null && sig.symbol_pct == null) right.push('BER ' + sig.ber);
      if (sig.freq_mhz) right.push(sig.freq_mhz + ' MHz ' + esc((sig.msys || '').toUpperCase()));
      if (sig.bitrate_kbps) right.push((sig.bitrate_kbps / 1000).toFixed(1) + ' Mbps');
      if (sig.tuner != null) right.push('Tuner ' + sig.tuner);
    } else if (playerState.sourceType === 'satip' || playerState.sourceType === 'hdhr') {
      right.push('<span style="color:#999">querying...</span>');
    }

    var html = '<div style="display:flex;gap:24px;">';
    html += '<div>' + lines.join('<br>') + '</div>';
    if (right.length) html += '<div>' + right.join('<br>') + '</div>';
    html += '</div>';
    overlay.innerHTML = html;
  }

  function bindPlayerControls(videoEl, streamID, seekPath) {
    if (!seekPath) seekPath = '/api/play/' + streamID + '/seek';
    var playPauseBtn = document.getElementById('play-pause-btn');
    var statsBtn = document.getElementById('stats-btn');
    var stopBtn = document.getElementById('stop-btn');
    var recordBtn = document.getElementById('record-btn');
    var volBtn = document.getElementById('vol-btn');
    var fsBtn = document.getElementById('fs-btn');
    var playerTime = document.getElementById('player-time');
    var seekRow = document.getElementById('player-seek-row');
    var seekPlayed = document.getElementById('seek-played');
    var seekBuffered = document.getElementById('seek-buffered');
    var seekThumb = document.getElementById('seek-thumb');
    var statusEl = document.getElementById('player-status');
    var spinner = document.getElementById('player-spinner');
    var wrapper = document.getElementById('player-wrapper');
    var floatBar = document.getElementById('player-float-bar');
    var ctrlBar = document.getElementById('player-ctrl-bar');

    if (wrapper) {
      wrapper.addEventListener('mouseenter', function() {
        if (floatBar) floatBar.style.opacity = '1';
        if (ctrlBar) ctrlBar.style.opacity = '1';
      });
      wrapper.addEventListener('mouseleave', function() {
        if (floatBar) floatBar.style.opacity = '0';
        if (ctrlBar) ctrlBar.style.opacity = '0';
      });
    }

    videoEl.addEventListener('waiting', function() {
      if (spinner) spinner.style.display = 'flex';
      if (statusEl) { statusEl.style.color = '#ffa726'; statusEl.textContent = 'Buffering'; }
    });
    videoEl.addEventListener('playing', function() {
      if (spinner) spinner.style.display = 'none';
      if (statusEl) { statusEl.style.color = '#4caf50'; statusEl.textContent = 'Playing'; }
      if (playPauseBtn) playPauseBtn.textContent = '\u23F8';
      playerState.retryCount = 0;
    });
    videoEl.addEventListener('seeked', function() {
      if (spinner) spinner.style.display = 'none';
    });
    videoEl.addEventListener('pause', function() {
      if (playPauseBtn) playPauseBtn.textContent = '\u25B6';
      if (statusEl) { statusEl.style.color = '#ffa726'; statusEl.textContent = 'Paused'; }
    });
    videoEl.addEventListener('error', function() {
      if (statusEl) { statusEl.style.color = '#ff6b6b'; statusEl.textContent = 'Error'; }
      handleRetry();
    });

    videoEl.addEventListener('click', function(e) {
      if (e.target !== videoEl) return;
      if (videoEl.paused) videoEl.play().catch(function() {}); else videoEl.pause();
    });

    if (playPauseBtn) {
      playPauseBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        if (videoEl.paused) { videoEl.play().catch(function() {}); }
        else { videoEl.pause(); }
      });
    }

    if (volBtn) {
      volBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        videoEl.muted = !videoEl.muted;
        volBtn.textContent = videoEl.muted ? '\uD83D\uDD07' : '\uD83D\uDD0A';
      });
    }

    if (fsBtn) {
      fsBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        if (document.fullscreenElement) document.exitFullscreen();
        else if (wrapper) wrapper.requestFullscreen().catch(function() {});
      });
    }

    var savedVol = parseFloat(localStorage.getItem('mediahub_volume') || '0.5');
    videoEl.volume = savedVol;
    videoEl.addEventListener('volumechange', function() {
      localStorage.setItem('mediahub_volume', String(videoEl.volume));
    });

    if (seekRow) {
      seekRow.addEventListener('mouseenter', function() {
        var track = seekRow.querySelector('.player-seek-track');
        if (track) track.style.height = '6px';
        if (seekThumb) seekThumb.style.opacity = '1';
      });
      seekRow.addEventListener('mouseleave', function() {
        var track = seekRow.querySelector('.player-seek-track');
        if (track) track.style.height = '4px';
        if (seekThumb) seekThumb.style.opacity = '0';
      });
      seekRow.addEventListener('click', function(e) {
        e.stopPropagation();
        var rect = seekRow.getBoundingClientRect();
        var pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
        if (seekPlayed) seekPlayed.style.width = (pct * 100) + '%';
        if (seekThumb) { seekThumb.style.left = (pct * 100) + '%'; seekThumb.style.opacity = '1'; }
        var dur = videoEl.duration;
        if (dur && isFinite(dur) && dur > 0) {
          videoEl.currentTime = pct * dur;
          api.post(seekPath, { position_ms: Math.round(pct * dur * 1000) }).catch(function() {});
        }
      });
    }

    var kbHelpBtn = document.getElementById('kb-help-btn');
    if (kbHelpBtn) {
      kbHelpBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        var helpEl = document.getElementById('kb-help-overlay');
        if (helpEl) helpEl.classList.toggle('visible');
      });
    }
    var kbHelpEl = document.getElementById('kb-help-overlay');
    if (kbHelpEl) {
      kbHelpEl.addEventListener('click', function(e) {
        if (e.target === kbHelpEl) kbHelpEl.classList.remove('visible');
      });
    }

    // Audio track selector
    var audioTrackBtn = document.getElementById('audio-track-btn');
    var audioTrackMenu = document.getElementById('audio-track-menu');
    if (audioTrackBtn && audioTrackMenu) {
      var audioMenuOpen = false;
      var closeAudioMenu = function() {
        audioTrackMenu.style.display = 'none';
        audioMenuOpen = false;
      };
      audioTrackBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        if (audioMenuOpen) { closeAudioMenu(); return; }
        audioTrackMenu.style.display = 'block';
        audioMenuOpen = true;
      });
      audioTrackMenu.addEventListener('click', function(e) { e.stopPropagation(); });
      document.addEventListener('click', function() { if (audioMenuOpen) closeAudioMenu(); });

      var realStreamID = streamID;
      if (realStreamID && realStreamID.indexOf('rec:') === 0) realStreamID = null;
      if (realStreamID) {
        // Small delay to let the session start before fetching tracks
        setTimeout(function() {
          api.get('/api/play/' + realStreamID + '/tracks')
            .then(function(resp) { return resp.ok ? resp.json() : null; })
            .then(function(data) {
              if (!data || !data.audio_tracks || data.audio_tracks.length <= 1) return;
              audioTrackBtn.style.display = '';
              var tracks = data.audio_tracks;
              var html = '';
              for (var ti = 0; ti < tracks.length; ti++) {
                var t = tracks[ti];
                var langName = t.language ? t.language.toUpperCase() : '';
                var label = langName || ('Track ' + (ti + 1));
                if (t.is_ad) label += ' (AD)';
                var chLabel = '';
                if (t.channels === 1) chLabel = 'Mono';
                else if (t.channels === 2) chLabel = 'Stereo';
                else if (t.channels === 6) chLabel = '5.1';
                else if (t.channels === 8) chLabel = '7.1';
                else if (t.channels > 0) chLabel = t.channels + 'ch';
                var codecLabel = (t.codec || '').toUpperCase();
                var meta = [chLabel, codecLabel].filter(Boolean).join(' / ');
                var activeCls = t.is_active ? ' audio-track-active' : '';
                html += '<div class="audio-track-item' + activeCls + '" data-index="' + t.index + '" style="padding:6px 12px;cursor:pointer;display:flex;align-items:center;gap:4px;font-size:13px;color:#fff;white-space:nowrap">' +
                  '<span class="audio-track-dot" style="width:8px;height:8px;border-radius:50%;margin-right:6px;flex-shrink:0;' + (t.is_active ? 'background:#3b82f6' : 'background:transparent') + '"></span>' +
                  '<span style="flex:1">' + label + '</span>' +
                  (meta ? '<span style="color:rgba(255,255,255,0.5);font-size:11px;margin-left:8px">' + meta + '</span>' : '') +
                  '</div>';
              }
              audioTrackMenu.innerHTML = html;

              audioTrackMenu.querySelectorAll('.audio-track-item').forEach(function(item) {
                item.addEventListener('mouseenter', function() { item.style.background = 'rgba(255,255,255,0.1)'; });
                item.addEventListener('mouseleave', function() {
                  item.style.background = item.classList.contains('audio-track-active') ? 'rgba(59,130,246,0.15)' : '';
                });
                if (item.classList.contains('audio-track-active')) {
                  item.style.background = 'rgba(59,130,246,0.15)';
                }
                item.addEventListener('click', function(e) {
                  e.stopPropagation();
                  var idx = parseInt(item.getAttribute('data-index'), 10);
                  api.post('/api/play/' + realStreamID + '/tracks/audio', { track: idx })
                    .then(function(resp) {
                      if (resp.ok) {
                        audioTrackMenu.querySelectorAll('.audio-track-item').forEach(function(el) {
                          var isThis = parseInt(el.getAttribute('data-index'), 10) === idx;
                          el.classList.toggle('audio-track-active', isThis);
                          el.style.background = isThis ? 'rgba(59,130,246,0.15)' : '';
                          var dot = el.querySelector('.audio-track-dot');
                          if (dot) dot.style.background = isThis ? '#3b82f6' : 'transparent';
                        });
                        closeAudioMenu();
                      } else {
                        resp.json().then(function(d) { toast('Failed to switch audio: ' + (d.error || 'unknown'), 'error'); }).catch(function() {});
                      }
                    })
                    .catch(function() { toast('Failed to switch audio track', 'error'); });
                });
              });
            })
            .catch(function() {});
        }, 500);
      }
    }

    var playerKeyHandler = function(e) {
      if (!document.getElementById('player-wrapper')) {
        document.removeEventListener('keydown', playerKeyHandler);
        return;
      }
      var tag = e.target.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || e.target.isContentEditable) return;
      var dur = videoEl.duration;
      var isVOD = dur && isFinite(dur) && dur > 0;
      var helpOverlay = document.getElementById('kb-help-overlay');
      var key = e.key;

      if (key === ' ') {
        e.preventDefault();
        if (videoEl.paused) videoEl.play().catch(function() {}); else videoEl.pause();
      } else if (key === 'f' || key === 'F') {
        e.preventDefault();
        if (document.fullscreenElement) document.exitFullscreen();
        else if (wrapper) wrapper.requestFullscreen().catch(function() {});
      } else if (key === 'm' || key === 'M') {
        e.preventDefault();
        videoEl.muted = !videoEl.muted;
        if (volBtn) volBtn.textContent = videoEl.muted ? '\uD83D\uDD07' : '\uD83D\uDD0A';
      } else if (key === 'ArrowLeft') {
        if (isVOD) {
          e.preventDefault();
          var newTime = Math.max(0, videoEl.currentTime - 10);
          videoEl.currentTime = newTime;
          api.post(seekPath, { position_ms: Math.round(newTime * 1000) }).catch(function() {});
        }
      } else if (key === 'ArrowRight') {
        if (isVOD) {
          e.preventDefault();
          var newTime = Math.min(dur, videoEl.currentTime + 10);
          videoEl.currentTime = newTime;
          api.post(seekPath, { position_ms: Math.round(newTime * 1000) }).catch(function() {});
        }
      } else if (key === 'ArrowUp') {
        e.preventDefault();
        videoEl.volume = Math.min(1, videoEl.volume + 0.1);
        videoEl.muted = false;
        if (volBtn) volBtn.textContent = '\uD83D\uDD0A';
      } else if (key === 'ArrowDown') {
        e.preventDefault();
        videoEl.volume = Math.max(0, videoEl.volume - 0.1);
        if (volBtn) volBtn.textContent = videoEl.volume === 0 ? '\uD83D\uDD07' : '\uD83D\uDD0A';
      } else if (key === 'i' || key === 'I') {
        var statsEl = document.getElementById('stats-overlay');
        if (statsEl) statsEl.classList.toggle('visible');
      } else if (key === 's' || key === 'S') {
        var statsEl = document.getElementById('stats-overlay');
        if (statsEl) statsEl.classList.toggle('visible');
      } else if (key === '?') {
        if (helpOverlay) helpOverlay.classList.toggle('visible');
      } else if (key === 'Escape') {
        if (helpOverlay && helpOverlay.classList.contains('visible')) {
          helpOverlay.classList.remove('visible');
        } else if (document.fullscreenElement) {
          document.exitFullscreen();
        } else {
          closePlayerOverlay();
        }
      } else if (key >= '0' && key <= '9' && isVOD) {
        e.preventDefault();
        var pct = parseInt(key, 10) / 10;
        var seekTo = pct * dur;
        videoEl.currentTime = seekTo;
        api.post(seekPath, { position_ms: Math.round(seekTo * 1000) }).catch(function() {});
      }
    };
    document.addEventListener('keydown', playerKeyHandler);

    var ctrlTimer = setInterval(function() {
      if (!document.getElementById('player-wrapper')) {
        clearInterval(ctrlTimer);
        document.removeEventListener('keydown', playerKeyHandler);
        return;
      }
      var cur = videoEl.currentTime || 0;
      var dur = videoEl.duration;
      // Use probe duration for VOD when videoEl.duration is infinite/growing
      var probeDurSec = (playerState.probeInfo && playerState.probeInfo.duration_ms) ? playerState.probeInfo.duration_ms / 1000 : 0;
      var effectiveDur = isFinite(dur) && dur > 0 ? dur : 0;
      if (probeDurSec > 0 && (!isFinite(dur) || dur <= 0 || effectiveDur < probeDurSec * 0.9)) {
        effectiveDur = probeDurSec;
      }
      if (playerTime) playerTime.textContent = formatTime(cur) + ' / ' + formatTime(effectiveDur);
      if (effectiveDur > 0) {
        var pct = cur / effectiveDur;
        if (seekPlayed) seekPlayed.style.width = (pct * 100) + '%';
        if (seekThumb) seekThumb.style.left = (pct * 100) + '%';
        var bufEnd = 0;
        if (videoEl.buffered && videoEl.buffered.length > 0) bufEnd = videoEl.buffered.end(videoEl.buffered.length - 1);
        if (seekBuffered) seekBuffered.style.width = ((bufEnd / effectiveDur) * 100) + '%';
      }
    }, 250);

    if (statsBtn) {
      statsBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        var overlay = document.getElementById('stats-overlay');
        if (overlay) overlay.classList.toggle('visible');
      });
    }

    if (stopBtn) {
      stopBtn.addEventListener('click', function(e) {
        e.stopPropagation();
        closePlayerOverlay();
      });
    }

    if (recordBtn) {
      var recording = false;
      recordBtn.addEventListener('click', async function(e) {
        e.stopPropagation();
        if (recording) {
          await api.del('/api/play/' + streamID + '/record').catch(function() {});
          recordBtn.textContent = '\u23FA';
          recordBtn.style.color = '';
          recording = false;
          toast('Recording stopped');
        } else {
          var resp = await api.post('/api/play/' + streamID + '/record', { title: 'Manual Recording' }).catch(function() {});
          if (resp && resp.ok) {
            recordBtn.style.color = '#e53935';
            recording = true;
            toast('Recording started');
          } else {
            toast('Failed to start recording', 'error');
          }
        }
      });
    }
  }

  function formatTime(sec) {
    if (!isFinite(sec) || sec < 0) return '0:00';
    var m = Math.floor(sec / 60);
    var s = Math.floor(sec % 60);
    return m + ':' + (s < 10 ? '0' : '') + s;
  }

  function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return '-';
    var units = ['B', 'KB', 'MB', 'GB', 'TB'];
    var i = 0;
    var val = bytes;
    while (val >= 1024 && i < units.length - 1) { val /= 1024; i++; }
    return val.toFixed(i === 0 ? 0 : 1) + ' ' + units[i];
  }

  function formatDurationSec(sec) {
    if (!sec || sec <= 0) return '-';
    var h = Math.floor(sec / 3600);
    var m = Math.floor((sec % 3600) / 60);
    var s = Math.floor(sec % 60);
    if (h > 0) return h + 'h ' + m + 'm';
    if (m > 0) return m + 'm ' + s + 's';
    return s + 's';
  }

  var recordingRefreshTimer = null;

  async function renderRecordings(el) {
    var isAdmin = api.user && (api.user.is_admin || api.user.role === 'admin');
    el.innerHTML = '<h1 class="page-title">Recordings</h1>' +
      '<div class="stat-grid" id="rec-stats">' +
      '<div class="stat-card"><div class="stat-value" id="rec-stat-active">-</div><div class="stat-label">Recording</div></div>' +
      '<div class="stat-card"><div class="stat-value" id="rec-stat-scheduled">-</div><div class="stat-label">Scheduled</div></div>' +
      '<div class="stat-card"><div class="stat-value" id="rec-stat-completed">-</div><div class="stat-label">Completed</div></div>' +
      '<div class="stat-card"><div class="stat-value" id="rec-stat-failed">-</div><div class="stat-label">Failed</div></div>' +
      '</div>' +
      (isAdmin ? '<div style="margin-bottom:16px"><button class="btn btn-primary" id="schedule-rec-btn">' + icons.plus + ' Schedule Recording</button></div>' : '') +
      '<div id="schedule-form" style="display:none" class="card">' +
      '<div class="card-title">Schedule Recording</div>' +
      '<div class="form-group"><label class="form-label">Channel / Stream</label><select class="form-input" id="rec-channel"></select></div>' +
      '<div class="form-group"><label class="form-label">Title</label><input class="form-input" id="rec-title" placeholder="Recording title"></div>' +
      '<div style="display:flex;gap:12px">' +
      '<div class="form-group" style="flex:1"><label class="form-label">Start</label><input class="form-input" id="rec-start" type="datetime-local"></div>' +
      '<div class="form-group" style="flex:1"><label class="form-label">Stop</label><input class="form-input" id="rec-stop" type="datetime-local"></div></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-rec-btn">Schedule</button>' +
      '<button class="btn btn-ghost" id="cancel-rec-btn">Cancel</button></div></div>' +
      '<div id="recording-list"><div class="skeleton" style="height:200px"></div></div>' +
      '<div id="rec-section-active"></div>' +
      '<div id="rec-section-scheduled"></div>' +
      '<div id="rec-section-completed"></div>';

    if (isAdmin) {
      var schedBtn = document.getElementById('schedule-rec-btn');
      var schedForm = document.getElementById('schedule-form');
      if (schedBtn) {
        schedBtn.addEventListener('click', async function() {
          schedForm.style.display = schedForm.style.display === 'none' ? 'block' : 'none';
          if (schedForm.style.display === 'block') {
            var channelSel = document.getElementById('rec-channel');
            channelSel.innerHTML = '<option value="">Loading...</option>';
            try {
              var chResp = await api.get('/api/channels');
              var channels = await chResp.json();
              if (!Array.isArray(channels)) channels = [];
              channelSel.innerHTML = '<option value="">Select channel</option>';
              for (var ci = 0; ci < channels.length; ci++) {
                channelSel.innerHTML += '<option value="' + esc(channels[ci].id) + '">' + esc(channels[ci].name) + ' (#' + channels[ci].number + ')</option>';
              }
            } catch (e) {
              channelSel.innerHTML = '<option value="">Failed to load channels</option>';
            }
            var now = new Date();
            var startStr = new Date(now.getTime() - now.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
            var stopTime = new Date(now.getTime() + 3600000 - now.getTimezoneOffset() * 60000);
            var stopStr = stopTime.toISOString().slice(0, 16);
            document.getElementById('rec-start').value = startStr;
            document.getElementById('rec-stop').value = stopStr;
          }
        });
      }
      document.getElementById('cancel-rec-btn').addEventListener('click', function() { schedForm.style.display = 'none'; });
      document.getElementById('create-rec-btn').addEventListener('click', async function() {
        var channelId = document.getElementById('rec-channel').value;
        var title = document.getElementById('rec-title').value.trim();
        var start = document.getElementById('rec-start').value;
        var stop = document.getElementById('rec-stop').value;
        if (!channelId) { toast('Select a channel', 'error'); return; }
        if (!start || !stop) { toast('Start and stop times required', 'error'); return; }
        try {
          var r = await api.post('/api/recordings/schedule', {
            channel_id: channelId,
            title: title || 'Scheduled Recording',
            start_at: new Date(start).toISOString(),
            stop_at: new Date(stop).toISOString()
          });
          if (r.ok) {
            toast('Recording scheduled');
            schedForm.style.display = 'none';
            renderRecordings(el);
          } else {
            var data = await r.json().catch(function() { return {}; });
            toast(data.error || 'Failed to schedule recording', 'error');
          }
        } catch (err) {
          toast('Failed to schedule recording', 'error');
        }
      });
    }

    function statusBadgeFor(st) {
      if (st === 'recording') return '<span class="badge badge-rec-active"><span class="recording-dot"></span> Recording</span>';
      if (st === 'completed') return '<span class="badge badge-rec-completed">Completed</span>';
      if (st === 'scheduled') return '<span class="badge badge-rec-scheduled">Scheduled</span>';
      if (st === 'pending') return '<span class="badge badge-rec-pending">Pending</span>';
      if (st === 'cancelled') return '<span class="badge badge-rec-cancelled">Cancelled</span>';
      if (st === 'failed') return '<span class="badge badge-rec-failed">Failed</span>';
      return '<span class="badge badge-disabled">' + esc(st || 'unknown') + '</span>';
    }

    function renderRecRow(r) {
      var dateStr = '-';
      if (r.started_at) dateStr = new Date(r.started_at).toLocaleString();
      else if (r.scheduled_start) dateStr = new Date(r.scheduled_start).toLocaleString();
      var durStr = '-';
      if (r.started_at && r.stopped_at) {
        durStr = formatDurationSec((new Date(r.stopped_at) - new Date(r.started_at)) / 1000);
      } else if (r.status === 'recording' && r.started_at) {
        var elapsed = (Date.now() - new Date(r.started_at).getTime()) / 1000;
        durStr = '<span class="rec-duration-live" data-started="' + esc(r.started_at) + '">' + formatDurationSec(elapsed) + '</span>';
      } else if (r.status === 'scheduled' && r.scheduled_start) {
        var countdown = (new Date(r.scheduled_start).getTime() - Date.now()) / 1000;
        durStr = countdown > 0 ? '<span class="rec-countdown" data-start="' + esc(r.scheduled_start) + '" style="color:var(--accent)">in ' + formatDurationSec(countdown) + '</span>' : '<span style="color:var(--warning)">starting...</span>';
      } else if (r.status === 'scheduled' && r.scheduled_stop) {
        var schedDur = (new Date(r.scheduled_stop) - new Date(r.scheduled_start)) / 1000;
        if (schedDur > 0) durStr = formatDurationSec(schedDur);
      }
      var sizeStr = formatBytes(r.file_size);
      var title = r.title || r.stream_name || r.channel_name || r.stream_id;
      var subtitle = r.channel_name || '';
      if (title === subtitle) subtitle = '';
      var titleHtml = '<div style="font-weight:600">' + esc(title) + '</div>';
      if (subtitle) titleHtml += '<div style="font-size:12px;color:var(--text-muted)">' + esc(subtitle) + '</div>';
      var codecBadges = '';
      if (r.video_codec) codecBadges += '<span class="badge badge-delivery" style="font-size:10px;padding:1px 5px">' + esc(r.video_codec.toUpperCase()) + '</span> ';
      if (r.audio_codec) codecBadges += '<span class="badge badge-delivery" style="font-size:10px;padding:1px 5px">' + esc(r.audio_codec.toUpperCase()) + '</span> ';
      if (r.container) codecBadges += '<span class="badge" style="background:var(--bg-hover);color:var(--text-dim);font-size:10px;padding:1px 5px;border-radius:4px">' + esc(r.container.toUpperCase()) + '</span>';
      if (codecBadges) titleHtml += '<div style="margin-top:2px">' + codecBadges + '</div>';
      if (r.status === 'completed' && r.file_exists === false) {
        titleHtml += '<div style="font-size:11px;color:var(--danger);margin-top:2px">File missing from disk</div>';
      }
      var actions = '';
      if (r.status === 'recording' && isAdmin) {
        actions += '<button class="btn btn-sm btn-danger rec-stop-btn" data-id="' + esc(r.stream_id) + '" title="Stop">' + (icons.stop || 'Stop') + '</button>';
      }
      if (r.status === 'completed' && r.file_exists !== false) {
        actions += '<button class="btn btn-sm btn-primary rec-play-btn" data-id="' + esc(r.id) + '" data-title="' + esc(title) + '" title="Play">' + icons.play + '</button>';
        actions += '<a class="btn btn-sm btn-ghost" href="/api/recordings/completed/' + esc(r.id) + '/stream" target="_blank" download title="Download">' + icons.download + '</a>';
      }
      if (isAdmin && (r.status === 'completed' || r.status === 'failed' || r.status === 'cancelled')) {
        actions += '<button class="btn btn-sm btn-icon btn-danger rec-del-btn" data-id="' + esc(r.id) + '" title="Delete">' + icons.trash + '</button>';
      }
      if (isAdmin && (r.status === 'scheduled' || r.status === 'pending')) {
        actions += '<button class="btn btn-sm btn-icon btn-danger rec-cancel-btn" data-id="' + esc(r.id) + '" title="Cancel">' + icons.trash + '</button>';
      }
      return '<tr><td>' + titleHtml + '</td><td>' + statusBadgeFor(r.status) + '</td><td style="font-size:12px">' + esc(dateStr) + '</td><td>' + durStr + '</td><td>' + esc(sizeStr) + '</td><td><div class="actions-cell">' + actions + '</div></td></tr>';
    }

    function renderRecSection(sectionEl, sectionTitle, recs) {
      if (!sectionEl) return;
      if (recs.length === 0) { sectionEl.innerHTML = ''; return; }
      var html = '<div class="card" style="margin-bottom:16px"><div class="card-title">' + esc(sectionTitle) + ' (' + recs.length + ')</div><table class="list-table"><thead><tr><th>Title</th><th>Status</th><th>Date</th><th>Duration</th><th>Size</th><th></th></tr></thead><tbody>';
      for (var i = 0; i < recs.length; i++) html += renderRecRow(recs[i]);
      html += '</tbody></table></div>';
      sectionEl.innerHTML = html;
    }

    async function loadRecordings() {
      try {
        var resp = await api.get('/api/recordings');
        var recordings = await resp.json();
        if (!Array.isArray(recordings)) recordings = [];
        var counts = { recording: 0, scheduled: 0, pending: 0, completed: 0, failed: 0, cancelled: 0 };
        var activeRecs = [], scheduledRecs = [], completedRecs = [];
        for (var ci = 0; ci < recordings.length; ci++) {
          var rec = recordings[ci];
          var status = rec.status || 'unknown';
          if (counts[status] !== undefined) counts[status]++;
          else counts.failed++;
          if (status === 'recording') activeRecs.push(rec);
          else if (status === 'scheduled' || status === 'pending') scheduledRecs.push(rec);
          else completedRecs.push(rec);
        }
        var el1 = document.getElementById('rec-stat-active');
        var el2 = document.getElementById('rec-stat-scheduled');
        var el3 = document.getElementById('rec-stat-completed');
        var el4 = document.getElementById('rec-stat-failed');
        if (el1) el1.textContent = counts.recording;
        if (el2) el2.textContent = counts.scheduled + counts.pending;
        if (el3) el3.textContent = counts.completed;
        if (el4) el4.textContent = counts.failed;
        var fallback = document.getElementById('recording-list');
        if (fallback) fallback.innerHTML = '';
        if (recordings.length === 0) {
          if (fallback) fallback.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No recordings yet</p></div>';
          ['rec-section-active','rec-section-scheduled','rec-section-completed'].forEach(function(id) { var e = document.getElementById(id); if (e) e.innerHTML = ''; });
          return;
        }
        activeRecs.sort(function(a, b) { return (b.started_at || '').localeCompare(a.started_at || ''); });
        scheduledRecs.sort(function(a, b) { return (a.scheduled_start || '').localeCompare(b.scheduled_start || ''); });
        completedRecs.sort(function(a, b) { return (b.stopped_at || b.started_at || '').localeCompare(a.stopped_at || a.started_at || ''); });
        renderRecSection(document.getElementById('rec-section-active'), 'Active', activeRecs);
        renderRecSection(document.getElementById('rec-section-scheduled'), 'Scheduled', scheduledRecs);
        renderRecSection(document.getElementById('rec-section-completed'), 'Completed', completedRecs);

        el.querySelectorAll('.rec-play-btn').forEach(function(btn) { btn.addEventListener('click', function() { playRecording(this.getAttribute('data-id'), this.getAttribute('data-title')); }); });
        el.querySelectorAll('.rec-stop-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var streamID = this.getAttribute('data-id');
            if (!confirm('Stop this recording?')) return;
            try { var sr = await api.del('/api/play/' + encodeURIComponent(streamID) + '/record'); if (sr.ok || sr.status === 204) { toast('Recording stopped'); loadRecordings(); } else { toast('Failed to stop recording', 'error'); } } catch (e) { toast('Failed to stop recording', 'error'); }
          });
        });
        el.querySelectorAll('.rec-del-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var recID = this.getAttribute('data-id');
            if (!confirm('Delete this recording? The file will be removed from disk.')) return;
            var delResp = await api.del('/api/recordings/completed/' + recID).catch(function() {});
            if (delResp && (delResp.status === 204 || delResp.ok)) { toast('Recording deleted'); loadRecordings(); } else { toast('Failed to delete recording', 'error'); }
          });
        });
        el.querySelectorAll('.rec-cancel-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var recID = this.getAttribute('data-id');
            if (!confirm('Cancel this scheduled recording?')) return;
            var delResp = await api.del('/api/recordings/schedule/' + recID).catch(function() {});
            if (delResp && (delResp.status === 204 || delResp.ok)) { toast('Recording cancelled'); loadRecordings(); } else { toast('Failed to cancel recording', 'error'); }
          });
        });
      } catch (e) {
        var container = document.getElementById('recording-list');
        if (container) container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load recordings</p></div>';
      }
    }

    await loadRecordings();

    if (recordingRefreshTimer) clearInterval(recordingRefreshTimer);
    recordingRefreshTimer = setInterval(function() {
      if (router.current !== 'recordings') { clearInterval(recordingRefreshTimer); recordingRefreshTimer = null; return; }
      document.querySelectorAll('.rec-duration-live').forEach(function(span) {
        var started = span.getAttribute('data-started');
        if (started) span.textContent = formatDurationSec((Date.now() - new Date(started).getTime()) / 1000);
      });
      document.querySelectorAll('.rec-countdown').forEach(function(span) {
        var startAt = span.getAttribute('data-start');
        if (startAt) { var remaining = (new Date(startAt).getTime() - Date.now()) / 1000; span.textContent = remaining > 0 ? 'in ' + formatDurationSec(remaining) : 'starting...'; if (remaining <= 0) span.style.color = 'var(--warning)'; }
      });
    }, 1000);
  }

  async function playRecording(recID, title) {
    playerState.cleanup();

    var sessionKey = 'rec:' + recID;
    playerState.currentStreamID = sessionKey;
    playerState.isLive = false;
    playerState.recordingID = recID;
    router.navigate('player', { streamID: sessionKey, name: title || 'Recording', isLive: false, recordingID: recID });
  }

  async function renderFavorites(el) {
    el.innerHTML = '<h1 class="page-title">Favorites</h1>' +
      '<div class="search-bar">' + icons.search + '<input id="fav-search" placeholder="Search favorites..."></div>' +
      '<div id="fav-list"><div class="skeleton" style="height:200px"></div></div>';

    try {
      await loadFavorites(true);
      var streamIDs = Object.keys(streamFavorites);
      var allStreams = [];
      if (streamIDs.length > 0) {
        var streamResp = await api.get('/api/streams?fields=slim');
        var all = await streamResp.json();
        if (Array.isArray(all)) {
          allStreams = all.filter(function(s) { return streamIDs.indexOf(s.id) >= 0; });
        }
      }

      renderFavoriteTable(allStreams, '');
      document.getElementById('fav-search').addEventListener('input', function() {
        renderFavoriteTable(allStreams, this.value.toLowerCase());
      });
    } catch (e) {
      document.getElementById('fav-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load favorites</p></div>';
    }
  }

  function renderFavoriteTable(streams, filter) {
    var container = document.getElementById('fav-list');
    if (!container) return;
    var filtered = streams;
    if (filter) {
      filtered = streams.filter(function(s) {
        return (s.name || '').toLowerCase().indexOf(filter) >= 0 ||
               (s.group || '').toLowerCase().indexOf(filter) >= 0;
      });
    }
    if (filtered.length === 0) {
      container.innerHTML = '<div class="empty-state">' + icons.star + '<p>No favorites yet</p></div>';
      return;
    }
    var html = '<table class="list-table"><thead><tr>' +
      '<th></th><th>Name</th><th>Group</th><th>Source</th><th></th>' +
      '</tr></thead><tbody>';
    for (var i = 0; i < filtered.length; i++) {
      var s = filtered[i];
      var logo = s.tvg_logo ? '<img class="logo" src="' + esc(s.tvg_logo) + '" alt="">' : '';
      html += '<tr>' +
        '<td>' + logo + '</td>' +
        '<td>' + esc(s.name) + '</td>' +
        '<td>' + esc(s.group || '-') + '</td>' +
        '<td>' + esc(s.source_type || '-') + '</td>' +
        '<td class="actions-cell">' +
        '<button class="btn btn-sm btn-primary play-btn" data-id="' + esc(s.id) + '" data-name="' + esc(s.name) + '">' + icons.play + '</button>' +
        '<button class="btn btn-sm btn-icon btn-danger remove-fav-btn" data-id="' + esc(s.id) + '" title="Remove favorite">' + icons.trash + '</button>' +
        '</td></tr>';
    }
    html += '</tbody></table>';
    container.innerHTML = html;
    container.querySelectorAll('.play-btn').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        startPlay(this.getAttribute('data-id'), this.getAttribute('data-name'), true);
      });
    });
    container.querySelectorAll('.remove-fav-btn').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        var sid = this.getAttribute('data-id');
        api.del('/api/favorites/' + sid).then(function() {
          delete streamFavorites[sid];
          streams = streams.filter(function(s) { return s.id !== sid; });
          renderFavoriteTable(streams, document.getElementById('fav-search') ? document.getElementById('fav-search').value.toLowerCase() : '');
          toast('Removed from favorites');
        }).catch(function() { toast('Failed to remove favorite', 'error'); });
      });
    });
  }

  async function renderSourceProfiles(el) {
    el.innerHTML = '<h1 class="page-title">Source Profiles</h1>' +
      '<div style="margin-bottom:16px"><button class="btn btn-primary" id="add-srcprofile-btn">' + icons.plus + ' Add Source Profile</button></div>' +
      '<div id="srcprofile-list"><div class="skeleton" style="height:200px"></div></div>';

    var editingId = null;

    var spFormBody =
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="sp-name" placeholder="SAT>IP DVB-T2"></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="sp-deinterlace"> Deinterlace</label>' +
      '<div class="field-hint">Apply deinterlace filter for interlaced content (576i/1080i)</div></div>' +
      '<div class="form-group" id="sp-deinterlace-method-group" style="display:none"><label class="form-label">Deinterlace Method</label>' +
      '<select class="form-input" id="sp-deinterlace-method"><option value="auto">Auto (best available)</option><option value="bob">Bob (double framerate)</option><option value="weave">Weave (merge fields)</option></select></div>' +
      '<div class="form-group"><label class="form-label">RTSP Protocol</label>' +
      '<select class="form-input" id="sp-rtsp-proto"><option value="tcp">TCP (recommended)</option><option value="udp">UDP</option></select></div>' +
      '<div class="form-group"><label class="form-label">RTSP Latency (ms)</label><input class="form-input" id="sp-rtsp-latency" type="number" value="0" min="0" placeholder="0">' +
      '<div class="field-hint">0 = minimum latency. Higher values add buffer for unstable connections.</div></div>' +
      '<div class="form-group"><label class="form-label">HTTP Timeout (sec)</label><input class="form-input" id="sp-http-timeout" type="number" value="30" min="1" placeholder="30">' +
      '<div class="field-hint">10s for local HDHR, 30s for remote IPTV.</div></div>' +
      '<div class="form-group"><label class="form-label">User Agent Override</label><input class="form-input" id="sp-user-agent" placeholder="Empty = use global default"></div>' +
      '<div class="form-group"><label class="form-label">Encoder Bitrate Override (kbps)</label><input class="form-input" id="sp-bitrate" type="number" value="0" min="0" placeholder="0 (auto by resolution)">' +
      '<div class="field-hint">0 = auto-scale by resolution. Override for bandwidth-limited connections.</div></div>';

    function openSPModal(title, saveLabel) {
      var modal = showFormModal(title, spFormBody, { id: 'srcprofile-modal', saveLabel: saveLabel });
      modal.querySelector('.modal-save-btn').addEventListener('click', async function() {
        var name = document.getElementById('sp-name').value.trim();
        if (!name) { toast('Name required', 'error'); return; }
        var payload = {
          name: name,
          deinterlace: document.getElementById('sp-deinterlace').checked,
          deinterlace_method: document.getElementById('sp-deinterlace-method').value,
          rtsp_protocols: document.getElementById('sp-rtsp-proto').value,
          rtsp_latency: parseInt(document.getElementById('sp-rtsp-latency').value) || 0,
          http_timeout_sec: parseInt(document.getElementById('sp-http-timeout').value) || 30,
          http_user_agent: document.getElementById('sp-user-agent').value.trim(),
          encoder_bitrate_kbps: parseInt(document.getElementById('sp-bitrate').value) || 0
        };
        try {
          var r;
          if (editingId) {
            r = await api.put('/api/source-profiles/' + editingId, payload);
          } else {
            r = await api.post('/api/source-profiles', payload);
          }
          if (r.ok) {
            toast(editingId ? 'Profile updated' : 'Profile created');
            modal.remove();
            editingId = null;
            renderSourceProfiles(el);
          } else {
            var data = await r.json().catch(function() { return {}; });
            toast(data.error || 'Failed to save profile', 'error');
          }
        } catch (err) {
          toast('Failed to save profile', 'error');
        }
      });
      document.getElementById('sp-deinterlace').addEventListener('change', function() {
        document.getElementById('sp-deinterlace-method-group').style.display = this.checked ? 'block' : 'none';
      });
      return modal;
    }

    document.getElementById('add-srcprofile-btn').addEventListener('click', function() {
      editingId = null;
      openSPModal('New Source Profile', 'Create');
    });

    try {
      var resp = await api.get('/api/source-profiles');
      var profiles = await resp.json();
      if (!Array.isArray(profiles)) profiles = [];
      var container = document.getElementById('srcprofile-list');
      if (!container) return;

      if (profiles.length === 0) {
        container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No source profiles configured</p></div>';
        return;
      }

      var html = '<table class="list-table"><thead><tr>' +
        '<th>Name</th><th>Deinterlace</th><th>RTSP Protocol</th><th>HTTP Timeout</th><th></th>' +
        '</tr></thead><tbody>';
      for (var i = 0; i < profiles.length; i++) {
        var p = profiles[i];
        html += '<tr>' +
          '<td>' + esc(p.name) + (p.is_system ? ' <span class="badge badge-info">System</span>' : '') + '</td>' +
          '<td>' + (p.deinterlace ? '<span class="badge badge-enabled">Yes</span>' : '') + '</td>' +
          '<td>' + esc(p.rtsp_protocols || 'tcp') + '</td>' +
          '<td>' + (p.http_timeout_sec ? p.http_timeout_sec + 's' : '30s') + '</td>' +
          '<td><div class="actions-cell">' +
          '<button class="btn btn-sm btn-ghost sp-edit-btn" data-id="' + esc(p.id) + '" data-profile=\'' + esc(JSON.stringify(p)) + '\'>' + icons.edit + '</button>' +
          (p.is_system ? '' : '<button class="btn btn-sm btn-icon btn-danger sp-del-btn" data-id="' + esc(p.id) + '" data-name="' + esc(p.name) + '">' + icons.trash + '</button>') +
          '</div></td></tr>';
      }
      html += '</tbody></table>';
      container.innerHTML = html;

      container.querySelectorAll('.sp-edit-btn').forEach(function(btn) {
        btn.addEventListener('click', function() {
          var p = JSON.parse(this.getAttribute('data-profile'));
          editingId = this.getAttribute('data-id');
          openSPModal('Edit Source Profile', 'Update');
          document.getElementById('sp-name').value = p.name || '';
          document.getElementById('sp-deinterlace').checked = !!p.deinterlace;
          document.getElementById('sp-deinterlace-method').value = p.deinterlace_method || 'auto';
          document.getElementById('sp-deinterlace-method-group').style.display = p.deinterlace ? 'block' : 'none';
          document.getElementById('sp-rtsp-proto').value = p.rtsp_protocols || 'tcp';
          document.getElementById('sp-rtsp-latency').value = p.rtsp_latency || '0';
          document.getElementById('sp-http-timeout').value = p.http_timeout_sec || '30';
          document.getElementById('sp-user-agent').value = p.http_user_agent || '';
          document.getElementById('sp-bitrate').value = p.encoder_bitrate_kbps || '0';
        });
      });

      container.querySelectorAll('.sp-del-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var name = this.getAttribute('data-name');
          if (!confirm('Delete source profile "' + name + '"?')) return;
          try {
            var r = await api.del('/api/source-profiles/' + id);
            if (r.ok || r.status === 204) {
              toast('Profile deleted');
              renderSourceProfiles(el);
            } else {
              toast('Failed to delete profile', 'error');
            }
          } catch (err) {
            toast('Failed to delete profile', 'error');
          }
        });
      });
    } catch (e) {
      document.getElementById('srcprofile-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load source profiles</p></div>';
    }
  }

  async function renderSources(el) {
    el.innerHTML = '<h1 class="page-title">Sources</h1>' +
      '<div style="margin-bottom:16px;display:flex;gap:8px">' +
      '<button class="btn btn-primary" id="add-m3u-btn">' + icons.plus + ' Add M3U Source</button>' +
      '<button class="btn btn-primary" id="add-tvp-btn">' + icons.plus + ' Add TVP Streams Source</button>' +
      '<button class="btn btn-primary" id="add-xtream-btn">' + icons.plus + ' Add Xtream Source</button>' +
      '<button class="btn btn-primary" id="add-hdhr-btn">' + icons.plus + ' Add HDHomeRun</button>' +
      '<button class="btn btn-primary" id="add-satip-btn">' + icons.plus + ' Add SAT>IP Source</button>' +
      '<button class="btn btn-primary" id="add-trailers-btn">' + icons.plus + ' Add TMDB Trailers</button>' +
      '<button class="btn btn-primary" id="add-demo-btn">' + icons.plus + ' Add Demo Streams</button>' +
      '<button class="btn btn-primary" id="add-spacex-btn">' + icons.plus + ' Add Space Launches</button>' +
      '<button class="btn btn-primary" id="add-radiogarden-btn">' + icons.plus + ' Add Radio Garden</button>' +
      '<button class="btn btn-ghost" id="discover-hdhr-btn">Discover HDHomeRun</button>' +
      '</div>' +
      '<div id="source-health-panel" style="margin-bottom:16px"></div>' +
      '<div id="source-list"><div class="skeleton" style="height:200px"></div></div>' +
      '<div id="add-m3u-form" style="display:none" class="card">' +
      '<div class="card-title">New M3U Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="src-name" placeholder="My IPTV Provider"></div>' +
      '<div class="form-group"><label class="form-label">URL</label><input class="form-input" id="src-url" placeholder="http://example.com/playlist.m3u"></div>' +
      '<div class="form-group"><label class="form-label">Username (optional)</label><input class="form-input" id="src-username"></div>' +
      '<div class="form-group"><label class="form-label">Password (optional)</label><input class="form-input" id="src-password" type="password"></div>' +
      '<div class="form-group"><label class="form-label">Source Profile</label><select class="form-input" id="src-profile"><option value="">None</option></select></div>' +
      '<div class="form-group"><label class="form-label">EPG Source</label><select class="form-input" id="src-epg-source"><option value="">None (no auto-match)</option></select></div>' +
      '<div class="form-group"><label class="form-label">Auto Refresh</label><select class="form-input" id="src-refresh"><option value="none">None (manual only)</option><option value="hourly">Hourly</option><option value="daily">Daily</option><option value="weekly">Weekly</option></select></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="src-wireguard"> Route through WireGuard</label></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-m3u-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-m3u-btn">Cancel</button></div></div>' +
      '<div id="add-tvp-form" style="display:none" class="card">' +
      '<div class="card-title">New TVP Streams Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="tvp-name" placeholder="My Media Library"></div>' +
      '<div class="form-group"><label class="form-label">URL</label><input class="form-input" id="tvp-url" placeholder="https://streams.example.com/playlist.m3u"></div>' +
      '<div class="form-group"><label class="form-label">Enrollment Token</label><input class="form-input" id="tvp-token" placeholder="One-time enrollment token"></div>' +
      '<div class="form-group"><label class="form-label">Source Profile</label><select class="form-input" id="tvp-profile"><option value="">None</option></select></div>' +
      '<div class="form-group"><label class="form-label">EPG Source</label><select class="form-input" id="tvp-epg-source"><option value="">None (no auto-match)</option></select></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="tvp-wireguard"> Route through WireGuard</label></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-tvp-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-tvp-btn">Cancel</button></div></div>' +
      '<div id="add-xtream-form" style="display:none" class="card">' +
      '<div class="card-title">New Xtream Codes Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="xt-name" placeholder="My IPTV Provider"></div>' +
      '<div class="form-group"><label class="form-label">Server URL</label><input class="form-input" id="xt-server" placeholder="http://provider.example.com:8080"></div>' +
      '<div class="form-group"><label class="form-label">Username</label><input class="form-input" id="xt-username"></div>' +
      '<div class="form-group"><label class="form-label">Password</label><input class="form-input" id="xt-password" type="password"></div>' +
      '<div class="form-group"><label class="form-label">Max Streams (0 = unlimited)</label><input class="form-input" id="xt-maxstreams" type="number" value="0" min="0"></div>' +
      '<div class="form-group"><label class="form-label">Source Profile</label><select class="form-input" id="xt-profile"><option value="">None</option></select></div>' +
      '<div class="form-group"><label class="form-label">EPG Source</label><select class="form-input" id="xt-epg-source"><option value="">None (no auto-match)</option></select></div>' +
      '<div class="form-group"><label class="form-label">Auto Refresh</label><select class="form-input" id="xt-refresh"><option value="none">None (manual only)</option><option value="hourly">Hourly</option><option value="daily">Daily</option><option value="weekly">Weekly</option></select></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="xt-wireguard"> Route through WireGuard</label></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-xtream-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-xtream-btn">Cancel</button></div></div>' +
      '<div id="add-hdhr-form" style="display:none" class="card">' +
      '<div class="card-title">New HDHomeRun Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="hdhr-name" placeholder="HDHomeRun" value="HDHomeRun"></div>' +
      '<div class="form-group"><label class="form-label">Source Profile</label><select class="form-input" id="hdhr-profile"><option value="">None</option></select></div>' +
      '<div class="form-group"><label class="form-label">EPG Source</label><select class="form-input" id="hdhr-epg-source"><option value="">None (no auto-match)</option></select></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="hdhr-enabled" checked> Enabled</label></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-hdhr-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-hdhr-btn">Cancel</button></div></div>' +
      '<div id="add-satip-form" style="display:none" class="card">' +
      '<div class="card-title">New SAT>IP Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="satip-name" placeholder="Home SAT>IP Server"></div>' +
      '<div class="form-group"><label class="form-label">Host / IP Address</label><input class="form-input" id="satip-host" placeholder="192.168.1.100"></div>' +
      '<div class="form-group"><label class="form-label">HTTP Port</label><input class="form-input" id="satip-port" type="number" value="8875" min="1" max="65535"></div>' +
      '<div class="form-group"><label class="form-label">System</label><select class="form-input" id="satip-system"><option value="">Loading...</option></select></div>' +
      '<div class="form-group"><label class="form-label">Transmitter</label><select class="form-input" id="satip-transmitter" disabled><option value="">-- Select System First --</option></select></div>' +
      '<div class="form-group"><label class="form-label">Max Streams (0 = unlimited)</label><input class="form-input" id="satip-maxstreams" type="number" value="0" min="0"></div>' +
      '<div class="form-group"><label class="form-label">Source Profile</label><select class="form-input" id="satip-profile"><option value="">None</option></select></div>' +
      '<div class="form-group"><label class="form-label">EPG Source</label><select class="form-input" id="satip-epg-source"><option value="">None (no auto-match)</option></select></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="satip-enabled" checked> Enabled</label></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-satip-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-satip-btn">Cancel</button></div></div>' +
      '<div id="add-trailers-form" style="display:none" class="card">' +
      '<div class="card-title">New TMDB Trailers Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="trailers-name" placeholder="TMDB Trailers" value="TMDB Trailers"></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-trailers-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-trailers-btn">Cancel</button></div></div>' +
      '<div id="add-demo-form" style="display:none" class="card">' +
      '<div class="card-title">New Demo Streams Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="demo-name" placeholder="Demo Streams" value="Demo Streams"></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-demo-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-demo-btn">Cancel</button></div></div>' +
      '<div id="add-spacex-form" style="display:none" class="card">' +
      '<div class="card-title">New Space Launches Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="spacex-name" placeholder="Space Launches" value="Space Launches"></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-spacex-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-spacex-btn">Cancel</button></div></div>' +
      '<div id="add-radiogarden-form" style="display:none" class="card">' +
      '<div class="card-title">New Radio Garden Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="rg-name" placeholder="London Radio"></div>' +
      '<div class="form-group"><label class="form-label">Location Search</label><input class="form-input" id="rg-search" placeholder="Search for a city..."><div id="rg-search-results" style="max-height:200px;overflow-y:auto;margin-top:4px;border:1px solid var(--border);border-radius:var(--radius);display:none"></div></div>' +
      '<div class="form-group"><label class="form-label">Selected Places</label><div id="rg-places-list" style="display:flex;flex-wrap:wrap;gap:6px;min-height:28px"></div></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-radiogarden-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-radiogarden-btn">Cancel</button></div></div>' +
      '<div id="hdhr-discover-modal" style="display:none"></div>';

    var allForms = ['add-m3u-form', 'add-tvp-form', 'add-xtream-form', 'add-hdhr-form', 'add-satip-form', 'add-trailers-form', 'add-demo-form', 'add-spacex-form', 'add-radiogarden-form'];
    var editingSourceId = null;
    var editingSourceType = null;

    function hideAllForms() {
      allForms.forEach(function(id) {
        var f = document.getElementById(id);
        if (f) {
          f.style.display = 'none';
          f.classList.remove('source-modal-form');
        }
      });
      var overlay = document.getElementById('source-form-overlay');
      if (overlay) overlay.remove();
      editingSourceId = null;
      editingSourceType = null;
    }

    function showSourceFormAsModal(formId) {
      var existingOverlay = document.getElementById('source-form-overlay');
      if (existingOverlay) existingOverlay.remove();
      allForms.forEach(function(id) {
        var f = document.getElementById(id);
        if (f) { f.style.display = 'none'; f.classList.remove('source-modal-form'); }
      });
      var f = document.getElementById(formId);
      if (!f) return;
      var overlay = document.createElement('div');
      overlay.id = 'source-form-overlay';
      overlay.className = 'modal-overlay';
      overlay.addEventListener('click', function(e) { if (e.target === overlay) hideAllForms(); });
      document.body.appendChild(overlay);
      f.style.display = 'block';
      f.classList.add('source-modal-form');
    }

    var sourceProfileSelectIds = ['src-profile', 'tvp-profile', 'xt-profile', 'hdhr-profile', 'satip-profile'];
    var epgSourceSelectIds = ['src-epg-source', 'tvp-epg-source', 'xt-epg-source', 'hdhr-epg-source', 'satip-epg-source'];
    (async function loadSourceProfiles() {
      try {
        var r = await api.get('/api/source-profiles');
        var profiles = await r.json();
        if (!Array.isArray(profiles)) profiles = [];
        sourceProfileSelectIds.forEach(function(selId) {
          var sel = document.getElementById(selId);
          if (!sel) return;
          var current = sel.value;
          sel.innerHTML = '<option value="">None</option>';
          profiles.forEach(function(p) {
            var opt = document.createElement('option');
            opt.value = p.id;
            opt.textContent = p.name;
            if (current && p.id === current) opt.selected = true;
            sel.appendChild(opt);
          });
        });
      } catch (e) {}
      try {
        var er = await api.get('/api/epg/sources');
        var epgSources = await er.json();
        if (!Array.isArray(epgSources)) epgSources = [];
        epgSourceSelectIds.forEach(function(selId) {
          var sel = document.getElementById(selId);
          if (!sel) return;
          var current = sel.value;
          sel.innerHTML = '<option value="">None (no auto-match)</option>';
          epgSources.forEach(function(s) {
            var opt = document.createElement('option');
            opt.value = s.id;
            opt.textContent = s.name + (s.channel_count ? ' (' + s.channel_count + ' channels)' : '');
            if (current && s.id === current) opt.selected = true;
            sel.appendChild(opt);
          });
        });
      } catch (e) {}
    })();

    function resetFormFields(formId) {
      var f = document.getElementById(formId);
      if (!f) return;
      f.querySelectorAll('input[type="text"], input[type="password"], input[type="number"]').forEach(function(inp) {
        inp.value = inp.defaultValue || '';
      });
      f.querySelectorAll('input[type="checkbox"]').forEach(function(inp) {
        inp.checked = inp.defaultChecked;
      });
      f.querySelectorAll('select').forEach(function(sel) {
        sel.selectedIndex = 0;
      });
      if (formId === 'add-satip-form') {
        var txSel = document.getElementById('satip-transmitter');
        if (txSel) {
          txSel.innerHTML = '<option value="">-- Select System First --</option>';
          txSel.disabled = true;
        }
      }
    }

    function setFormTitle(formId, title) {
      var f = document.getElementById(formId);
      if (!f) return;
      var t = f.querySelector('.card-title');
      if (t) t.textContent = title;
    }

    function setSubmitBtnText(btnId, text) {
      var b = document.getElementById(btnId);
      if (b) b.textContent = text;
    }

    document.getElementById('add-m3u-btn').addEventListener('click', function() {
      resetFormFields('add-m3u-form');
      setFormTitle('add-m3u-form', 'New M3U Source');
      setSubmitBtnText('create-m3u-btn', 'Create');
      showSourceFormAsModal('add-m3u-form');
    });
    document.getElementById('cancel-m3u-btn').addEventListener('click', function() { hideAllForms(); });
    document.getElementById('add-tvp-btn').addEventListener('click', function() {
      resetFormFields('add-tvp-form');
      setFormTitle('add-tvp-form', 'New TVP Streams Source');
      setSubmitBtnText('create-tvp-btn', 'Create');
      showSourceFormAsModal('add-tvp-form');
    });
    document.getElementById('cancel-tvp-btn').addEventListener('click', function() { hideAllForms(); });
    document.getElementById('add-xtream-btn').addEventListener('click', function() {
      resetFormFields('add-xtream-form');
      setFormTitle('add-xtream-form', 'New Xtream Codes Source');
      setSubmitBtnText('create-xtream-btn', 'Create');
      showSourceFormAsModal('add-xtream-form');
    });
    document.getElementById('cancel-xtream-btn').addEventListener('click', function() { hideAllForms(); });
    document.getElementById('add-hdhr-btn').addEventListener('click', function() {
      resetFormFields('add-hdhr-form');
      setFormTitle('add-hdhr-form', 'New HDHomeRun Source');
      setSubmitBtnText('create-hdhr-btn', 'Create');
      showSourceFormAsModal('add-hdhr-form');
    });
    document.getElementById('cancel-hdhr-btn').addEventListener('click', function() { hideAllForms(); });
    document.getElementById('add-satip-btn').addEventListener('click', function() {
      resetFormFields('add-satip-form');
      setFormTitle('add-satip-form', 'New SAT>IP Source');
      setSubmitBtnText('create-satip-btn', 'Create');
      showSourceFormAsModal('add-satip-form');
    });
    document.getElementById('cancel-satip-btn').addEventListener('click', function() { hideAllForms(); });
    document.getElementById('add-trailers-btn').addEventListener('click', function() {
      resetFormFields('add-trailers-form');
      setFormTitle('add-trailers-form', 'New TMDB Trailers Source');
      setSubmitBtnText('create-trailers-btn', 'Create');
      showSourceFormAsModal('add-trailers-form');
    });
    document.getElementById('cancel-trailers-btn').addEventListener('click', function() { hideAllForms(); });
    document.getElementById('add-demo-btn').addEventListener('click', function() {
      resetFormFields('add-demo-form');
      setFormTitle('add-demo-form', 'New Demo Streams Source');
      setSubmitBtnText('create-demo-btn', 'Create');
      showSourceFormAsModal('add-demo-form');
    });
    document.getElementById('cancel-demo-btn').addEventListener('click', function() { hideAllForms(); });
    document.getElementById('add-spacex-btn').addEventListener('click', function() {
      resetFormFields('add-spacex-form');
      setFormTitle('add-spacex-form', 'New Space Launches Source');
      setSubmitBtnText('create-spacex-btn', 'Create');
      showSourceFormAsModal('add-spacex-form');
    });
    document.getElementById('cancel-spacex-btn').addEventListener('click', function() { hideAllForms(); });
    var rgSelectedPlaces = [];

    function renderRgPlaces() {
      var container = document.getElementById('rg-places-list');
      if (!container) return;
      container.innerHTML = rgSelectedPlaces.length === 0
        ? '<span style="color:var(--text-muted);font-size:13px">No locations selected</span>'
        : rgSelectedPlaces.map(function(p, i) {
        return '<span style="background:var(--bg-input);border:1px solid var(--border);border-radius:16px;padding:4px 8px 4px 12px;font-size:13px;display:inline-flex;align-items:center;gap:6px">' +
          (p.name || p.id) +
          '<button class="rg-remove-place" data-index="' + i + '" style="background:var(--danger);color:#fff;border:none;border-radius:50%;cursor:pointer;width:18px;height:18px;font-size:14px;padding:0;line-height:1;display:flex;align-items:center;justify-content:center" title="Remove">&minus;</button></span>';
      }).join('');
      container.querySelectorAll('.rg-remove-place').forEach(function(btn) {
        btn.addEventListener('click', function() {
          rgSelectedPlaces.splice(parseInt(this.getAttribute('data-index')), 1);
          renderRgPlaces();
        });
      });
    }

    document.getElementById('add-radiogarden-btn').addEventListener('click', function() {
      resetFormFields('add-radiogarden-form');
      setFormTitle('add-radiogarden-form', 'New Radio Garden Source');
      setSubmitBtnText('create-radiogarden-btn', 'Create');
      rgSelectedPlaces = [];
      renderRgPlaces();
      document.getElementById('rg-search-results').innerHTML = '';
      showSourceFormAsModal('add-radiogarden-form');
    });
    document.getElementById('cancel-radiogarden-btn').addEventListener('click', function() { hideAllForms(); });

    var rgSearchTimeout = null;
    document.getElementById('rg-search').addEventListener('input', function() {
      var query = this.value.trim().toLowerCase();
      var resultsDiv = document.getElementById('rg-search-results');
      if (rgSearchTimeout) clearTimeout(rgSearchTimeout);
      if (query.length < 2) { resultsDiv.innerHTML = ''; resultsDiv.style.display = 'none'; return; }
      rgSearchTimeout = setTimeout(async function() {
        try {
          var r = await api.get('/api/sources/radiogarden/places?q=' + encodeURIComponent(query));
          if (!r.ok) { resultsDiv.innerHTML = '<div style="color:var(--danger);padding:4px">Failed to search places</div>'; return; }
          var matches = await r.json();
          if (!Array.isArray(matches) || matches.length === 0) { resultsDiv.innerHTML = '<div style="padding:4px;color:var(--text-muted)">No places found</div>'; return; }
          resultsDiv.innerHTML = matches.map(function(p) {
            return '<div class="rg-place-option" data-id="' + p.id + '" data-title="' + (p.title || '').replace(/"/g, '&quot;') + '" data-country="' + (p.country || '').replace(/"/g, '&quot;') + '" style="padding:8px 10px;cursor:pointer;border-bottom:1px solid var(--border);display:flex;justify-content:space-between;align-items:center;background:var(--bg-card);border-radius:4px;margin-bottom:2px">' +
              '<span>' + (p.title || '') + ', ' + (p.country || '') + ' <span style="font-size:0.8em;color:var(--text-muted)">(' + (p.size || 0) + ' stations)</span></span>' +
              '<button type="button" style="background:var(--accent);color:#fff;border:none;border-radius:4px;padding:2px 10px;font-size:12px;cursor:pointer">+ Add</button></div>';
          }).join('');
          resultsDiv.style.display = 'block';
          resultsDiv.querySelectorAll('.rg-place-option').forEach(function(opt) {
            opt.addEventListener('click', function() {
              var placeId = this.getAttribute('data-id');
              var placeTitle = this.getAttribute('data-title');
              if (rgSelectedPlaces.some(function(p) { return p.id === placeId; })) {
                toast(placeTitle + ' already added', 'info');
                return;
              }
              rgSelectedPlaces.push({ id: placeId, name: placeTitle });
              renderRgPlaces();
              if (!document.getElementById('rg-name').value && rgSelectedPlaces.length === 1) {
                document.getElementById('rg-name').value = 'Radio Garden';
              }
              document.getElementById('rg-search').value = '';
              resultsDiv.innerHTML = '';
              resultsDiv.style.display = 'none';
              toast(placeTitle + ' added', 'success');
            });
          });
        } catch (err) {
          resultsDiv.innerHTML = '<div style="color:var(--danger);padding:4px">Search failed</div>';
        }
      }, 300);
    });

    async function loadSystems(selectedSystem) {
      var sel = document.getElementById('satip-system');
      try {
        var r = await api.get('/api/satip/systems');
        var systems = await r.json();
        sel.innerHTML = '<option value="">-- Select System --</option>';
        systems.forEach(function(s) {
          var opt = document.createElement('option');
          opt.value = s;
          opt.textContent = s.toUpperCase().replace('-', ' ');
          if (selectedSystem && s === selectedSystem) opt.selected = true;
          sel.appendChild(opt);
        });
      } catch (err) {
        sel.innerHTML = '<option value="">-- No systems available --</option>';
      }
    }
    loadSystems();

    async function loadTransmitters(system, selectedFile) {
      var sel = document.getElementById('satip-transmitter');
      sel.innerHTML = '<option value="">Loading...</option>';
      sel.disabled = true;
      if (!system) {
        sel.innerHTML = '<option value="">-- Select System First --</option>';
        return;
      }
      try {
        var r = await api.get('/api/satip/transmitters?system=' + encodeURIComponent(system));
        var data = await r.json();
        sel.innerHTML = '<option value="">-- Select Transmitter --</option>';
        data.forEach(function(t) {
          var opt = document.createElement('option');
          opt.value = t.file;
          opt.textContent = t.name;
          if (selectedFile && t.file === selectedFile) opt.selected = true;
          sel.appendChild(opt);
        });
        sel.disabled = false;
      } catch (err) {
        sel.innerHTML = '<option value="">Failed to load transmitters</option>';
      }
    }

    document.getElementById('satip-system').addEventListener('change', function() {
      loadTransmitters(this.value);
    });

    document.getElementById('create-hdhr-btn').addEventListener('click', async function() {
      var name = document.getElementById('hdhr-name').value.trim();
      var enabled = document.getElementById('hdhr-enabled').checked;
      var hdhrProfileId = document.getElementById('hdhr-profile').value;
      var hdhrEpgSourceId = document.getElementById('hdhr-epg-source').value;
      if (!name) { toast('Name required', 'error'); return; }
      try {
        var payload = { name: name, is_enabled: enabled, source_profile_id: hdhrProfileId || '', epg_source_id: hdhrEpgSourceId || '' };
        var r;
        if (editingSourceId && editingSourceType === 'hdhr') {
          r = await api.put('/api/sources/hdhr/' + editingSourceId, payload);
        } else {
          r = await api.post('/api/sources/hdhr', payload);
        }
        if (r.ok) {
          toast(editingSourceId ? 'Source updated' : 'HDHomeRun source created');
          hideAllForms();
          editingSourceId = null;
          editingSourceType = null;
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to save source', 'error');
        }
      } catch (err) {
        toast('Failed to save source', 'error');
      }
    });

    document.getElementById('create-satip-btn').addEventListener('click', async function() {
      var name = document.getElementById('satip-name').value.trim();
      var host = document.getElementById('satip-host').value.trim();
      var port = parseInt(document.getElementById('satip-port').value) || 8875;
      var transmitter = document.getElementById('satip-transmitter').value;
      var maxStreams = parseInt(document.getElementById('satip-maxstreams').value) || 0;
      var enabled = document.getElementById('satip-enabled').checked;
      var satipProfileId = document.getElementById('satip-profile').value;
      var satipEpgSourceId = document.getElementById('satip-epg-source').value;
      if (!name || !host) { toast('Name and host required', 'error'); return; }
      try {
        var payload = {
          name: name, host: host, http_port: port,
          transmitter_file: transmitter, max_streams: maxStreams, is_enabled: enabled,
          source_profile_id: satipProfileId || '', epg_source_id: satipEpgSourceId || ''
        };
        var r;
        if (editingSourceId && editingSourceType === 'satip') {
          r = await api.put('/api/sources/satip/' + editingSourceId, payload);
        } else {
          r = await api.post('/api/sources/satip', payload);
        }
        if (r.ok) {
          toast(editingSourceId ? 'Source updated' : 'SAT>IP source created');
          hideAllForms();
          editingSourceId = null;
          editingSourceType = null;
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to save source', 'error');
        }
      } catch (err) {
        toast('Failed to save source', 'error');
      }
    });

    document.getElementById('create-trailers-btn').addEventListener('click', async function() {
      var name = document.getElementById('trailers-name').value.trim();
      if (!name) { toast('Name required', 'error'); return; }
      try {
        var payload = { name: name };
        var r;
        if (editingSourceId && editingSourceType === 'trailers') {
          r = await api.put('/api/sources/trailers/' + editingSourceId, payload);
        } else {
          r = await api.post('/api/sources/trailers', payload);
        }
        if (r.ok) {
          toast(editingSourceId ? 'Source updated' : 'Apple Trailers source created, refreshing...');
          hideAllForms();
          editingSourceId = null;
          editingSourceType = null;
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to save source', 'error');
        }
      } catch (err) {
        toast('Failed to save source', 'error');
      }
    });

    document.getElementById('create-demo-btn').addEventListener('click', async function() {
      var name = document.getElementById('demo-name').value.trim();
      if (!name) { toast('Name required', 'error'); return; }
      try {
        var payload = { name: name };
        var r;
        if (editingSourceId && editingSourceType === 'demo') {
          r = await api.put('/api/sources/demo/' + editingSourceId, payload);
        } else {
          r = await api.post('/api/sources/demo', payload);
        }
        if (r.ok) {
          toast(editingSourceId ? 'Source updated' : 'Demo streams source created, refreshing...');
          hideAllForms();
          editingSourceId = null;
          editingSourceType = null;
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to save source', 'error');
        }
      } catch (err) {
        toast('Failed to save source', 'error');
      }
    });

    document.getElementById('create-spacex-btn').addEventListener('click', async function() {
      var name = document.getElementById('spacex-name').value.trim();
      if (!name) { toast('Name required', 'error'); return; }
      try {
        var payload = { name: name };
        var r;
        if (editingSourceId && editingSourceType === 'spacex') {
          r = await api.put('/api/sources/spacex/' + editingSourceId, payload);
        } else {
          r = await api.post('/api/sources/spacex', payload);
        }
        if (r.ok) {
          toast(editingSourceId ? 'Source updated' : 'SpaceX source created, refreshing...');
          hideAllForms();
          editingSourceId = null;
          editingSourceType = null;
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to save source', 'error');
        }
      } catch (err) {
        toast('Failed to save source', 'error');
      }
    });

    document.getElementById('create-radiogarden-btn').addEventListener('click', async function() {
      var name = document.getElementById('rg-name').value.trim();
      if (!name) { toast('Name required', 'error'); return; }
      if (rgSelectedPlaces.length === 0) { toast('Please search and select at least one location', 'error'); return; }
      try {
        var payload = { name: name, places: rgSelectedPlaces };
        var r;
        if (editingSourceId && editingSourceType === 'radiogarden') {
          r = await api.put('/api/sources/radiogarden/' + editingSourceId, payload);
        } else {
          r = await api.post('/api/sources/radiogarden', payload);
        }
        if (r.ok) {
          toast(editingSourceId ? 'Source updated' : 'Radio Garden source created, refreshing...');
          hideAllForms();
          editingSourceId = null;
          editingSourceType = null;
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to save source', 'error');
        }
      } catch (err) {
        toast('Failed to save source', 'error');
      }
    });

    document.getElementById('discover-hdhr-btn').addEventListener('click', async function() {
      var overlay = document.createElement('div');
      overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.75);z-index:9999;display:flex;align-items:center;justify-content:center;backdrop-filter:blur(6px);';
      overlay.onclick = function(e) { if (e.target === overlay) overlay.remove(); };
      var modal = document.createElement('div');
      modal.style.cssText = 'width:90%;max-width:600px;max-height:80vh;background:var(--bg-card);border-radius:16px;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 24px 80px rgba(0,0,0,0.6);';
      var header = document.createElement('div');
      header.style.cssText = 'padding:20px 24px;border-bottom:1px solid var(--border);display:flex;align-items:center;justify-content:space-between;';
      header.innerHTML = '<div style="font-size:18px;font-weight:700;">Discover HDHomeRun Devices</div>';
      var closeBtn = document.createElement('button');
      closeBtn.style.cssText = 'background:none;border:none;color:var(--text-secondary);font-size:20px;cursor:pointer;';
      closeBtn.textContent = '\u2715';
      closeBtn.onclick = function() { overlay.remove(); };
      header.appendChild(closeBtn);
      modal.appendChild(header);
      var body = document.createElement('div');
      body.style.cssText = 'padding:24px;overflow-y:auto;flex:1;';
      body.innerHTML = '<div style="text-align:center;padding:24px;color:var(--text-secondary)">Scanning network...</div>';
      modal.appendChild(body);
      overlay.appendChild(modal);
      document.body.appendChild(overlay);
      try {
        var resp = await api.post('/api/sources/hdhr/discover');
        var devices = await resp.json();
        body.innerHTML = '';
        if (!devices || devices.length === 0) {
          body.innerHTML = '<div style="text-align:center;padding:24px;color:var(--text-secondary)">No HDHomeRun devices found on the network</div>';
          return;
        }
        devices.forEach(function(d) {
          var row = document.createElement('div');
          row.style.cssText = 'display:flex;align-items:center;gap:16px;padding:12px 16px;border-radius:8px;border:1px solid var(--border);margin-bottom:8px;';
          var info = document.createElement('div');
          info.style.cssText = 'flex:1;min-width:0;';
          var nameEl = document.createElement('div');
          nameEl.style.cssText = 'font-size:15px;font-weight:600;';
          nameEl.textContent = d.name || d.model || d.host;
          info.appendChild(nameEl);
          var metaParts = [d.host];
          if (d.model) metaParts.push(d.model);
          if (d.properties && d.properties.tuner_count > 0) metaParts.push(d.properties.tuner_count + ' tuner' + (d.properties.tuner_count > 1 ? 's' : ''));
          var metaEl = document.createElement('div');
          metaEl.style.cssText = 'font-size:12px;color:var(--text-secondary);margin-top:2px;';
          metaEl.textContent = metaParts.join(' \u2022 ');
          info.appendChild(metaEl);
          row.appendChild(info);
          if (d.already_added) {
            var badge = document.createElement('span');
            badge.style.cssText = 'font-size:12px;color:var(--text-secondary);font-weight:600;';
            badge.textContent = 'Already added';
            row.appendChild(badge);
          } else {
            var addBtn = document.createElement('button');
            addBtn.className = 'btn btn-primary';
            addBtn.style.cssText = 'padding:6px 16px;font-size:13px;flex-shrink:0;';
            addBtn.textContent = 'Add';
            addBtn.onclick = async function() {
              addBtn.disabled = true;
              addBtn.textContent = '...';
              try {
                var ar = await api.post('/api/sources/hdhr/add-device', { host: d.host });
                if (ar.ok || ar.status === 201) {
                  addBtn.textContent = 'Added';
                  addBtn.disabled = true;
                  d.already_added = true;
                  renderSources(el);
                } else {
                  addBtn.textContent = 'Failed';
                }
              } catch(e) {
                addBtn.textContent = 'Failed';
              }
            };
            row.appendChild(addBtn);
          }
          body.appendChild(row);
        });
      } catch(e) {
        body.innerHTML = '<div style="text-align:center;padding:24px;color:var(--danger)">Discovery failed: ' + esc(e.message) + '</div>';
      }
    });

    document.getElementById('create-m3u-btn').addEventListener('click', async function() {
      var name = document.getElementById('src-name').value.trim();
      var srcUrl = document.getElementById('src-url').value.trim();
      var username = document.getElementById('src-username').value.trim();
      var password = document.getElementById('src-password').value;
      var wg = document.getElementById('src-wireguard').checked;
      var srcProfileId = document.getElementById('src-profile').value;
      var epgSourceId = document.getElementById('src-epg-source').value;
      var refreshInterval = document.getElementById('src-refresh').value;
      if (!name || !srcUrl) { toast('Name and URL required', 'error'); return; }
      try {
        var payload = { name: name, url: srcUrl, username: username, use_wireguard: wg, source_profile_id: srcProfileId || '', epg_source_id: epgSourceId || '', refresh_interval: refreshInterval || 'none' };
        if (password) payload.password = password;
        var r;
        if (editingSourceId && editingSourceType === 'm3u') {
          r = await api.put('/api/sources/m3u/' + editingSourceId, payload);
        } else {
          r = await api.post('/api/sources/m3u', payload);
        }
        if (r.ok) {
          toast(editingSourceId ? 'Source updated' : 'Source created, refreshing...');
          hideAllForms();
          editingSourceId = null;
          editingSourceType = null;
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to save source', 'error');
        }
      } catch (err) {
        toast('Failed to save source', 'error');
      }
    });

    document.getElementById('create-tvp-btn').addEventListener('click', async function() {
      var name = document.getElementById('tvp-name').value.trim();
      var tvpUrl = document.getElementById('tvp-url').value.trim();
      var token = document.getElementById('tvp-token').value.trim();
      var wg = document.getElementById('tvp-wireguard').checked;
      var tvpProfileId = document.getElementById('tvp-profile').value;
      var tvpEpgSourceId = document.getElementById('tvp-epg-source').value;
      if (!name || !tvpUrl) { toast('Name and URL required', 'error'); return; }
      try {
        var payload = { name: name, url: tvpUrl, enrollment_token: token, use_wireguard: wg, source_profile_id: tvpProfileId || '', epg_source_id: tvpEpgSourceId || '' };
        var r;
        if (editingSourceId && editingSourceType === 'tvpstreams') {
          r = await api.put('/api/sources/tvpstreams/' + editingSourceId, payload);
        } else {
          r = await api.post('/api/sources/tvpstreams', payload);
        }
        if (r.ok) {
          toast(editingSourceId ? 'Source updated' : 'TVP Streams source created' + (token ? ', enrolling...' : ', refreshing...'));
          hideAllForms();
          editingSourceId = null;
          editingSourceType = null;
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to save source', 'error');
        }
      } catch (err) {
        toast('Failed to save source', 'error');
      }
    });

    document.getElementById('create-xtream-btn').addEventListener('click', async function() {
      var name = document.getElementById('xt-name').value.trim();
      var server = document.getElementById('xt-server').value.trim();
      var username = document.getElementById('xt-username').value.trim();
      var password = document.getElementById('xt-password').value;
      var maxStreams = parseInt(document.getElementById('xt-maxstreams').value) || 0;
      var wg = document.getElementById('xt-wireguard').checked;
      var xtProfileId = document.getElementById('xt-profile').value;
      var xtEpgSourceId = document.getElementById('xt-epg-source').value;
      var xtRefresh = document.getElementById('xt-refresh').value;
      if (!name || !server || !username) { toast('Name, server, and username required', 'error'); return; }
      if (!editingSourceId && !password) { toast('Password required', 'error'); return; }
      try {
        var payload = { name: name, server: server, username: username, max_streams: maxStreams, use_wireguard: wg, source_profile_id: xtProfileId || '', epg_source_id: xtEpgSourceId || '', refresh_interval: xtRefresh || 'none' };
        if (password) payload.password = password;
        var r;
        if (editingSourceId && editingSourceType === 'xtream') {
          r = await api.put('/api/sources/xtream/' + editingSourceId, payload);
        } else {
          r = await api.post('/api/sources/xtream', payload);
        }
        if (r.ok) {
          toast(editingSourceId ? 'Source updated' : 'Xtream source created, refreshing...');
          hideAllForms();
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to save source', 'error');
        }
      } catch (err) {
        toast('Failed to save source', 'error');
      }
    });

    // Load source health panel
    try {
      var healthResp = await api.get('/api/sources/health');
      var healthData = await healthResp.json();
      var healthPanel = document.getElementById('source-health-panel');
      if (healthPanel && healthData.summary) {
        var hs = healthData.summary;
        var hpHtml = '<div class="card" style="padding:16px">' +
          '<div style="display:flex;align-items:center;gap:16px;flex-wrap:wrap">' +
          '<span style="font-size:14px;font-weight:600;color:#fff">Source Health</span>';
        if (hs.healthy > 0) hpHtml += '<span style="display:inline-flex;align-items:center;gap:4px"><span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:var(--success)"></span><span style="font-size:13px;color:var(--success)">' + hs.healthy + ' healthy</span></span>';
        if (hs.stale > 0) hpHtml += '<span style="display:inline-flex;align-items:center;gap:4px"><span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:var(--warning)"></span><span style="font-size:13px;color:var(--warning)">' + hs.stale + ' stale</span></span>';
        if (hs.error > 0) hpHtml += '<span style="display:inline-flex;align-items:center;gap:4px"><span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:var(--danger)"></span><span style="font-size:13px;color:var(--danger)">' + hs.error + ' error</span></span>';
        if (hs.disabled > 0) hpHtml += '<span style="display:inline-flex;align-items:center;gap:4px"><span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:var(--text-muted)"></span><span style="font-size:13px;color:var(--text-muted)">' + hs.disabled + ' disabled</span></span>';
        hpHtml += '</div>';
        if (healthData.sources && healthData.sources.length > 0) {
          hpHtml += '<table class="list-table" style="margin-top:12px"><thead><tr>' +
            '<th>Source</th><th>Type</th><th>Status</th><th>Streams</th><th>Last Refresh</th><th>Error</th><th>Action</th>' +
            '</tr></thead><tbody>';
          for (var hi = 0; hi < healthData.sources.length; hi++) {
            var hsrc = healthData.sources[hi];
            var hDot = 'var(--success)';
            var hBadge = '<span class="badge badge-enabled">Healthy</span>';
            if (hsrc.health === 'error') { hDot = 'var(--danger)'; hBadge = '<span class="badge badge-live">Error</span>'; }
            else if (hsrc.health === 'stale') { hDot = 'var(--warning)'; hBadge = '<span class="badge badge-warning">Stale</span>'; }
            else if (hsrc.health === 'disabled') { hDot = 'var(--text-muted)'; hBadge = '<span class="badge badge-disabled">Disabled</span>'; }
            var hRefreshed = 'Never';
            if (hsrc.last_refreshed) {
              var hDate = new Date(hsrc.last_refreshed);
              var hMin = Math.floor((Date.now() - hDate.getTime()) / 60000);
              hRefreshed = hMin < 1 ? 'just now' : hMin < 60 ? hMin + 'm ago' : hMin < 1440 ? Math.floor(hMin / 60) + 'h ago' : Math.floor(hMin / 1440) + 'd ago';
            }
            hpHtml += '<tr>' +
              '<td><span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:' + hDot + ';margin-right:6px"></span>' + esc(hsrc.name) + '</td>' +
              '<td><span class="badge badge-enabled">' + esc(hsrc.type).toUpperCase() + '</span></td>' +
              '<td>' + hBadge + '</td>' +
              '<td>' + (hsrc.stream_count || 0) + '</td>' +
              '<td>' + esc(hRefreshed) + '</td>' +
              '<td style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--danger)">' + (hsrc.last_error ? esc(hsrc.last_error) : '-') + '</td>' +
              '<td><button class="btn btn-sm btn-ghost health-refresh-btn" data-id="' + esc(hsrc.id) + '">' + icons.refresh + '</button></td>' +
              '</tr>';
          }
          hpHtml += '</tbody></table>';
        }
        hpHtml += '</div>';
        healthPanel.innerHTML = hpHtml;
        healthPanel.querySelectorAll('.health-refresh-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var sid = this.getAttribute('data-id');
            this.disabled = true;
            this.innerHTML = '...';
            try { await api.post('/api/sources/' + sid + '/refresh'); toast('Refresh started'); }
            catch(e) { toast('Refresh failed', 'error'); }
            setTimeout(function() { renderSources(el); }, 2000);
          });
        });
      }
    } catch (e) { /* health panel optional */ }

    try {
      var resp = await api.get('/api/sources');
      var sources = await resp.json();
      if (!Array.isArray(sources)) sources = [];
      var container = document.getElementById('source-list');
      if (!container) return;

      if (sources.length === 0) {
        container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No sources configured</p></div>';
        return;
      }

      var sourceConfigs = {};
      var html = '';
      for (var i = 0; i < sources.length; i++) {
        var s = sources[i];
        sourceConfigs[s.id] = { config: s.config || {}, name: s.name, type: s.type, is_enabled: s.is_enabled };
        var statusBadge = s.is_enabled ? '<span class="badge badge-enabled">ON</span>' : '<span class="badge badge-disabled">OFF</span>';
        if (s.last_error) {
          statusBadge = '<span class="badge badge-live" title="' + esc(s.last_error) + '">ERROR</span>';
        }
        var lastRefreshed = s.last_refreshed ? new Date(s.last_refreshed).toLocaleString() : 'Never';

        var typeActions = '';
        if (s.type === 'hdhr') {
          typeActions = '<button class="btn btn-sm btn-ghost hdhr-devices-btn" data-id="' + esc(s.id) + '">Devices</button>' +
            '<button class="btn btn-sm btn-ghost hdhr-scan-btn" data-id="' + esc(s.id) + '">Scan</button>' +
            '<button class="btn btn-sm btn-ghost hdhr-retune-btn" data-id="' + esc(s.id) + '">Retune</button>' +
            '<button class="btn btn-sm btn-ghost hdhr-clear-btn" data-id="' + esc(s.id) + '" style="color:var(--danger)">Clear</button>';
        } else if (s.type === 'satip') {
          typeActions = '<button class="btn btn-sm btn-ghost satip-scan-btn" data-id="' + esc(s.id) + '">Scan</button>' +
            '<button class="btn btn-sm btn-ghost satip-clear-btn" data-id="' + esc(s.id) + '" style="color:var(--danger)">Clear</button>';
        } else if (s.type === 'xtream') {
          typeActions = '<button class="btn btn-sm btn-ghost xtream-info-btn" data-id="' + esc(s.id) + '">Account Info</button>';
        }

        var tlsBadge = '';
        if (s.type === 'tvpstreams') {
          var enrolled = s.config && s.config.tls_enrolled === 'true';
          tlsBadge = enrolled
            ? ' <span class="badge badge-enabled" data-tls-id="' + esc(s.id) + '">TLS</span>'
            : ' <span class="badge badge-disabled">No TLS</span>';
        }

        html += '<div class="card" style="margin-bottom:12px">' +
          '<div style="display:flex;align-items:center;gap:12px">' +
          '<div style="flex:1;min-width:0">' +
          '<div style="display:flex;align-items:center;gap:8px;margin-bottom:4px">' +
          '<span style="font-size:15px;font-weight:600">' + esc(s.name) + '</span>' +
          '<span class="badge badge-enabled">' + esc(s.type || 'unknown').toUpperCase() + '</span>' +
          statusBadge + tlsBadge +
          '</div>' +
          '<div style="font-size:12px;color:var(--text-secondary)">' +
          (s.stream_count || 0) + ' streams' +
          ' &bull; Last refreshed: ' + esc(lastRefreshed) +
          '</div>' +
          '</div>' +
          '<div style="display:flex;gap:4px;align-items:center;flex-shrink:0">' +
          typeActions +
          '<button class="btn-icon edit-source-btn" data-id="' + esc(s.id) + '" title="Edit">' + icons.edit + '</button>' +
          (s.type === 'm3u' ? '<button class="btn-icon upload-m3u-btn" data-id="' + esc(s.id) + '" title="Upload M3U File">' + icons.upload + '</button>' : '') +
          '<button class="btn-icon refresh-source-btn" data-id="' + esc(s.id) + '" data-type="' + esc(s.type) + '" title="Refresh">' + icons.refresh + '</button>' +
          '<button class="btn-icon delete-source-btn" data-id="' + esc(s.id) + '" data-type="' + esc(s.type) + '" data-name="' + esc(s.name) + '" title="Delete" style="color:var(--danger)">' + icons.trash + '</button>' +
          '</div></div>' +
          '<div id="source-detail-' + esc(s.id) + '" style="display:none;margin-top:12px;border-top:1px solid var(--border);padding-top:12px"></div>' +
          '</div>';
      }
      container.innerHTML = html;

      container.querySelectorAll('[data-tls-id]').forEach(function(badge) {
        var id = badge.getAttribute('data-tls-id');
        api.get('/api/sources/tvpstreams/' + id + '/tls').then(function(r) {
          return r.json();
        }).then(function(tls) {
          if (tls.fingerprint) {
            badge.title = 'Fingerprint: ' + tls.fingerprint;
          }
        }).catch(function() {});
      });

      container.querySelectorAll('.hdhr-devices-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var detail = document.getElementById('source-detail-' + id);
          if (!detail) return;
          if (detail.style.display !== 'none') { detail.style.display = 'none'; return; }
          detail.style.display = 'block';
          detail.innerHTML = '<div style="color:var(--text-secondary)">Loading devices...</div>';
          try {
            var r = await api.get('/api/sources/hdhr/' + id + '/devices');
            var devices = await r.json();
            if (!devices || devices.length === 0) {
              detail.innerHTML = '<div style="color:var(--text-secondary)">No devices configured. Use "Discover HDHomeRun" to find devices.</div>';
              return;
            }
            var dhtml = '<div style="font-weight:600;margin-bottom:8px">Devices (' + devices.length + ')</div>';
            devices.forEach(function(d) {
              dhtml += '<div style="display:flex;align-items:center;gap:12px;padding:8px 12px;border-radius:6px;border:1px solid var(--border);margin-bottom:4px">' +
                '<div style="flex:1"><div style="font-weight:600">' + esc(d.model || d.host) + '</div>' +
                '<div style="font-size:12px;color:var(--text-secondary)">' + esc(d.host) +
                (d.device_id ? ' &bull; ID: ' + esc(d.device_id) : '') +
                (d.tuner_count ? ' &bull; ' + d.tuner_count + ' tuners' : '') +
                (d.firmware_version ? ' &bull; FW: ' + esc(d.firmware_version) : '') +
                '</div></div></div>';
            });
            detail.innerHTML = dhtml;
          } catch(e) {
            detail.innerHTML = '<div style="color:var(--danger)">Failed to load devices</div>';
          }
        });
      });

      container.querySelectorAll('.hdhr-scan-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          this.disabled = true;
          this.textContent = 'Scanning...';
          try {
            var r = await api.post('/api/sources/hdhr/' + id + '/scan');
            if (r.ok || r.status === 202) {
              toast('Scan started');
              var self = this;
              setTimeout(function() { self.disabled = false; self.textContent = 'Scan'; renderSources(el); }, 5000);
            } else {
              toast('Failed to start scan', 'error');
              this.disabled = false;
              this.textContent = 'Scan';
            }
          } catch(e) {
            toast('Failed to start scan', 'error');
            this.disabled = false;
            this.textContent = 'Scan';
          }
        });
      });

      container.querySelectorAll('.hdhr-retune-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var detail = document.getElementById('source-detail-' + id);
          if (!detail) return;
          detail.style.display = 'block';
          detail.innerHTML = '<div style="color:var(--text-secondary)">Starting retune...</div>' +
            '<div style="margin-top:8px;background:var(--border);border-radius:4px;height:6px;overflow:hidden">' +
            '<div id="retune-progress-' + id + '" style="background:var(--primary);height:100%;width:0%;transition:width 0.5s"></div></div>' +
            '<div id="retune-status-' + id + '" style="font-size:12px;color:var(--text-secondary);margin-top:4px"></div>';
          this.disabled = true;
          try {
            await api.post('/api/sources/hdhr/' + id + '/retune');
            var pollInterval = setInterval(async function() {
              try {
                var sr = await api.get('/api/sources/hdhr/' + id + '/status');
                var status = await sr.json();
                var bar = document.getElementById('retune-progress-' + id);
                var label = document.getElementById('retune-status-' + id);
                if (!bar || !label) { clearInterval(pollInterval); return; }
                if (status.state === 'done') {
                  bar.style.width = '100%';
                  label.textContent = status.message || 'Retune complete';
                  clearInterval(pollInterval);
                  btn.disabled = false;
                  setTimeout(function() { renderSources(el); }, 1000);
                } else if (status.state === 'error') {
                  bar.style.width = '100%';
                  bar.style.background = 'var(--danger)';
                  label.textContent = status.message || 'Retune failed';
                  label.style.color = 'var(--danger)';
                  clearInterval(pollInterval);
                  btn.disabled = false;
                } else {
                  bar.style.width = (status.progress || 10) + '%';
                  label.textContent = status.message || 'Scanning...';
                }
              } catch(e) { clearInterval(pollInterval); btn.disabled = false; }
            }, 3000);
          } catch(e) {
            detail.innerHTML = '<div style="color:var(--danger)">Failed to start retune</div>';
            this.disabled = false;
          }
        });
      });

      container.querySelectorAll('.hdhr-clear-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          if (!confirm('Clear all streams from this HDHomeRun source?')) return;
          try {
            await api.post('/api/sources/hdhr/' + id + '/clear');
            toast('Streams cleared');
            renderSources(el);
          } catch(e) { toast('Failed to clear', 'error'); }
        });
      });

      container.querySelectorAll('.satip-scan-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var detail = document.getElementById('source-detail-' + id);
          if (!detail) return;
          detail.style.display = 'block';
          detail.innerHTML = '<div style="color:var(--text-secondary)">Starting scan...</div>' +
            '<div style="margin-top:8px;background:var(--border);border-radius:4px;height:6px;overflow:hidden">' +
            '<div id="satip-progress-' + id + '" style="background:var(--primary);height:100%;width:0%;transition:width 0.5s"></div></div>' +
            '<div id="satip-status-' + id + '" style="font-size:12px;color:var(--text-secondary);margin-top:4px"></div>';
          this.disabled = true;
          try {
            await api.post('/api/sources/satip/' + id + '/scan');
            var pollInterval = setInterval(async function() {
              try {
                var sr = await api.get('/api/sources/satip/' + id + '/status');
                var status = await sr.json();
                var bar = document.getElementById('satip-progress-' + id);
                var label = document.getElementById('satip-status-' + id);
                if (!bar || !label) { clearInterval(pollInterval); return; }
                if (status.state === 'done') {
                  bar.style.width = '100%';
                  label.textContent = status.message || 'Scan complete';
                  clearInterval(pollInterval);
                  btn.disabled = false;
                  setTimeout(function() { renderSources(el); }, 1000);
                } else if (status.state === 'error') {
                  bar.style.width = '100%';
                  bar.style.background = 'var(--danger)';
                  label.textContent = status.message || 'Scan failed';
                  label.style.color = 'var(--danger)';
                  clearInterval(pollInterval);
                  btn.disabled = false;
                } else {
                  bar.style.width = (status.progress || 10) + '%';
                  label.textContent = status.message || 'Scanning...';
                }
              } catch(e) { clearInterval(pollInterval); btn.disabled = false; }
            }, 3000);
          } catch(e) {
            detail.innerHTML = '<div style="color:var(--danger)">Failed to start scan</div>';
            this.disabled = false;
          }
        });
      });

      container.querySelectorAll('.satip-clear-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          if (!confirm('Clear all streams from this SAT>IP source?')) return;
          try {
            await api.post('/api/sources/satip/' + id + '/clear');
            toast('Streams cleared');
            renderSources(el);
          } catch(e) { toast('Failed to clear', 'error'); }
        });
      });

      container.querySelectorAll('.xtream-info-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var detail = document.getElementById('source-detail-' + id);
          if (!detail) return;
          if (detail.style.display !== 'none') { detail.style.display = 'none'; return; }
          detail.style.display = 'block';
          detail.innerHTML = '<div style="color:var(--text-secondary)">Loading account info...</div>';
          try {
            var r = await api.get('/api/sources/xtream/' + id + '/info');
            if (!r.ok) {
              var errMsg = 'Failed to load account info';
              try { var errBody = await r.json(); if (errBody && errBody.error) errMsg += ': ' + errBody.error; } catch(e2) {}
              detail.innerHTML = '<div style="color:var(--danger)">' + esc(errMsg) + '</div>';
              return;
            }
            var info = await r.json();
            detail.innerHTML = '<div style="font-weight:600;margin-bottom:8px">Account Info</div>' +
              '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:8px">' +
              '<div style="padding:8px 12px;border-radius:6px;border:1px solid var(--border)">' +
              '<div style="font-size:11px;color:var(--text-secondary);text-transform:uppercase">Status</div>' +
              '<div style="font-size:15px;font-weight:600">' + esc(info.status) + '</div></div>' +
              '<div style="padding:8px 12px;border-radius:6px;border:1px solid var(--border)">' +
              '<div style="font-size:11px;color:var(--text-secondary);text-transform:uppercase">Max Connections</div>' +
              '<div style="font-size:15px;font-weight:600">' + esc(info.max_connections) + '</div></div>' +
              (info.active_connections ? '<div style="padding:8px 12px;border-radius:6px;border:1px solid var(--border)">' +
              '<div style="font-size:11px;color:var(--text-secondary);text-transform:uppercase">Active Connections</div>' +
              '<div style="font-size:15px;font-weight:600">' + esc(info.active_connections) + '</div></div>' : '') +
              '<div style="padding:8px 12px;border-radius:6px;border:1px solid var(--border)">' +
              '<div style="font-size:11px;color:var(--text-secondary);text-transform:uppercase">Live Categories</div>' +
              '<div style="font-size:15px;font-weight:600">' + info.live_categories + '</div></div>' +
              '<div style="padding:8px 12px;border-radius:6px;border:1px solid var(--border)">' +
              '<div style="font-size:11px;color:var(--text-secondary);text-transform:uppercase">Live Streams</div>' +
              '<div style="font-size:15px;font-weight:600">' + info.live_streams + '</div></div>' +
              '<div style="padding:8px 12px;border-radius:6px;border:1px solid var(--border)">' +
              '<div style="font-size:11px;color:var(--text-secondary);text-transform:uppercase">VOD Movies</div>' +
              '<div style="font-size:15px;font-weight:600">' + info.vod_streams + '</div></div>' +
              '<div style="padding:8px 12px;border-radius:6px;border:1px solid var(--border)">' +
              '<div style="font-size:11px;color:var(--text-secondary);text-transform:uppercase">TV Series</div>' +
              '<div style="font-size:15px;font-weight:600">' + info.series_count + '</div></div>' +
              '<div style="padding:8px 12px;border-radius:6px;border:1px solid var(--border)">' +
              '<div style="font-size:11px;color:var(--text-secondary);text-transform:uppercase">Protocol</div>' +
              '<div style="font-size:15px;font-weight:600">' + esc(info.server_protocol || '-') + '</div></div>' +
              '</div>';
          } catch(e) {
            detail.innerHTML = '<div style="color:var(--danger)">' + esc('Failed to load account info: ' + (e.message || 'unknown error')) + '</div>';
          }
        });
      });

      container.querySelectorAll('.edit-source-btn').forEach(function(btn) {
        btn.addEventListener('click', function() {
          var id = btn.getAttribute('data-id');
          var entry = sourceConfigs[id] || {};
          var type = entry.type || '';
          var name = entry.name || '';
          var config = entry.config || {};

          hideAllForms();
          editingSourceId = id;
          editingSourceType = type;

          if (type === 'm3u') {
            setFormTitle('add-m3u-form', 'Edit M3U Source');
            setSubmitBtnText('create-m3u-btn', 'Update');
            showSourceFormAsModal('add-m3u-form');
            document.getElementById('src-name').value = name || '';
            document.getElementById('src-url').value = config.url || '';
            document.getElementById('src-username').value = config.username || '';
            document.getElementById('src-password').value = '';
            document.getElementById('src-wireguard').checked = config.use_wireguard === 'true';
            document.getElementById('src-profile').value = config.source_profile_id || '';
            document.getElementById('src-epg-source').value = config.epg_source_id || '';
            document.getElementById('src-refresh').value = config.refresh_interval || 'none';
          } else if (type === 'tvpstreams') {
            setFormTitle('add-tvp-form', 'Edit TVP Streams Source');
            setSubmitBtnText('create-tvp-btn', 'Update');
            showSourceFormAsModal('add-tvp-form');
            document.getElementById('tvp-name').value = name || '';
            document.getElementById('tvp-url').value = config.url || '';
            document.getElementById('tvp-token').value = '';
            document.getElementById('tvp-wireguard').checked = config.use_wireguard === 'true';
            document.getElementById('tvp-profile').value = config.source_profile_id || '';
            document.getElementById('tvp-epg-source').value = config.epg_source_id || '';
          } else if (type === 'xtream') {
            setFormTitle('add-xtream-form', 'Edit Xtream Codes Source');
            setSubmitBtnText('create-xtream-btn', 'Update');
            showSourceFormAsModal('add-xtream-form');
            document.getElementById('xt-name').value = name || '';
            document.getElementById('xt-server').value = config.server || '';
            document.getElementById('xt-username').value = config.username || '';
            document.getElementById('xt-password').value = '';
            document.getElementById('xt-maxstreams').value = config.max_streams || '0';
            document.getElementById('xt-wireguard').checked = config.use_wireguard === 'true';
            document.getElementById('xt-profile').value = config.source_profile_id || '';
            document.getElementById('xt-epg-source').value = config.epg_source_id || '';
            document.getElementById('xt-refresh').value = config.refresh_interval || 'none';
          } else if (type === 'hdhr') {
            setFormTitle('add-hdhr-form', 'Edit HDHomeRun Source');
            setSubmitBtnText('create-hdhr-btn', 'Update');
            showSourceFormAsModal('add-hdhr-form');
            document.getElementById('hdhr-name').value = name || '';
            document.getElementById('hdhr-enabled').checked = entry.is_enabled;
            document.getElementById('hdhr-profile').value = config.source_profile_id || '';
            document.getElementById('hdhr-epg-source').value = config.epg_source_id || '';
          } else if (type === 'satip') {
            setFormTitle('add-satip-form', 'Edit SAT>IP Source');
            setSubmitBtnText('create-satip-btn', 'Update');
            showSourceFormAsModal('add-satip-form');
            document.getElementById('satip-name').value = name || '';
            document.getElementById('satip-host').value = config.host || '';
            document.getElementById('satip-port').value = config.http_port || '8875';
            var txFile = config.transmitter_file || '';
            if (txFile && txFile.indexOf('/') !== -1) {
              var sysVal = txFile.substring(0, txFile.indexOf('/'));
              loadSystems(sysVal).then(function() { loadTransmitters(sysVal, txFile); });
            } else {
              loadSystems();
            }
            document.getElementById('satip-maxstreams').value = config.max_streams || '0';
            document.getElementById('satip-profile').value = config.source_profile_id || '';
            document.getElementById('satip-epg-source').value = config.epg_source_id || '';
            document.getElementById('satip-enabled').checked = entry.is_enabled;
          } else if (type === 'trailers') {
            setFormTitle('add-trailers-form', 'Edit TMDB Trailers Source');
            setSubmitBtnText('create-trailers-btn', 'Update');
            showSourceFormAsModal('add-trailers-form');
            document.getElementById('trailers-name').value = name || '';
          } else if (type === 'demo') {
            setFormTitle('add-demo-form', 'Edit Demo Streams Source');
            setSubmitBtnText('create-demo-btn', 'Update');
            showSourceFormAsModal('add-demo-form');
            document.getElementById('demo-name').value = name || '';
          } else if (type === 'spacex') {
            setFormTitle('add-spacex-form', 'Edit Space Launches Source');
            setSubmitBtnText('create-spacex-btn', 'Update');
            showSourceFormAsModal('add-spacex-form');
            document.getElementById('spacex-name').value = name || '';
          } else if (type === 'radiogarden') {
            setFormTitle('add-radiogarden-form', 'Edit Radio Garden Source');
            setSubmitBtnText('create-radiogarden-btn', 'Update');
            showSourceFormAsModal('add-radiogarden-form');
            document.getElementById('rg-name').value = name || '';
            // Parse places from config — support new JSON format and old single-place format
            rgSelectedPlaces = [];
            if (config.places) {
              try { rgSelectedPlaces = JSON.parse(config.places); } catch(e) { rgSelectedPlaces = []; }
            }
            if (rgSelectedPlaces.length === 0 && config.place_id) {
              rgSelectedPlaces = [{ id: config.place_id, name: config.place_name || '' }];
            }
            renderRgPlaces();
            document.getElementById('rg-search').value = '';
            document.getElementById('rg-search-results').innerHTML = '';
          }
        });
      });

      container.querySelectorAll('.refresh-source-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var self = this;
          self.style.pointerEvents = 'none';
          self.style.animation = 'playerspin 1s linear infinite';
          self.style.color = 'var(--accent)';
          var card = self.closest('.card');
          var nameEl = card ? card.querySelector('[style*="font-weight:600"]') : null;
          var refreshBadge = null;
          if (nameEl && !nameEl.querySelector('.refresh-badge')) {
            refreshBadge = document.createElement('span');
            refreshBadge.className = 'badge badge-warning refresh-badge';
            refreshBadge.style.cssText = 'margin-left:8px;font-size:11px';
            refreshBadge.textContent = 'Refreshing...';
            nameEl.appendChild(refreshBadge);
          }
          function stopRefreshUI() {
            self.style.animation = '';
            self.style.color = '';
            self.style.pointerEvents = '';
            if (refreshBadge && refreshBadge.parentNode) refreshBadge.parentNode.removeChild(refreshBadge);
          }
          try {
            var r = await api.post('/api/sources/' + id + '/refresh', {});
            if (r.ok || r.status === 202) {
              toast('Refresh started');
              var pollCount = 0;
              var pollTimer = setInterval(async function() {
                pollCount++;
                if (pollCount > 60) { clearInterval(pollTimer); stopRefreshUI(); renderSources(el); return; }
                try {
                  var sr = await api.get('/api/sources');
                  var srcs = await sr.json();
                  if (!Array.isArray(srcs)) return;
                  var src = srcs.find(function(s) { return s.id === id; });
                  if (src && src.last_refreshed) {
                    var refreshTime = new Date(src.last_refreshed).getTime();
                    if (Date.now() - refreshTime < 10000) {
                      clearInterval(pollTimer);
                      stopRefreshUI();
                      renderSources(el);
                    }
                  }
                } catch (pe) {}
              }, 3000);
            } else {
              toast('Failed to refresh', 'error');
              stopRefreshUI();
            }
          } catch (err) {
            toast('Failed to refresh', 'error');
            stopRefreshUI();
          }
        });
      });

      container.querySelectorAll('.upload-m3u-btn').forEach(function(btn) {
        btn.addEventListener('click', function() {
          var id = this.getAttribute('data-id');
          var input = document.createElement('input');
          input.type = 'file';
          input.accept = '.m3u,.m3u8,text/plain';
          input.onchange = async function() {
            if (!input.files || !input.files[0]) return;
            var form = new FormData();
            form.append('file', input.files[0]);
            try {
              var r = await fetch('/api/sources/m3u/' + id + '/upload', {
                method: 'POST',
                headers: { 'Authorization': 'Bearer ' + api.token },
                body: form
              });
              var data = await r.json();
              if (r.ok) {
                toast('Uploaded: ' + data.parsed + ' streams parsed');
                setTimeout(function() { renderSources(el); }, 1000);
              } else {
                toast(data.error || 'Upload failed', 'error');
              }
            } catch (err) {
              toast('Upload failed', 'error');
            }
          };
          input.click();
        });
      });

      container.querySelectorAll('.delete-source-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var type = this.getAttribute('data-type');
          var name = this.getAttribute('data-name');
          if (!confirm('Delete source "' + name + '"? All its streams will be removed.')) return;
          try {
            var r = await api.del('/api/sources/' + type + '/' + id);
            if (r.ok || r.status === 204) {
              toast('Source deleted');
              renderSources(el);
            } else {
              toast('Failed to delete source', 'error');
            }
          } catch (err) {
            toast('Failed to delete source', 'error');
          }
        });
      });
      if (window._autoOpenSourceForm) {
        var autoFormId = window._autoOpenSourceForm;
        window._autoOpenSourceForm = null;
        var autoFormBtnMap = {
          'add-m3u-form': 'add-m3u-btn',
          'add-tvp-form': 'add-tvp-btn',
          'add-xtream-form': 'add-xtream-btn',
          'add-hdhr-form': 'add-hdhr-btn',
          'add-satip-form': 'add-satip-btn',
          'add-radiogarden-form': 'add-radiogarden-btn',
          'add-spacex-form': 'add-spacex-btn',
          'add-trailers-form': 'add-trailers-btn',
          'add-demo-form': 'add-demo-btn'
        };
        var triggerBtnId = autoFormBtnMap[autoFormId];
        if (triggerBtnId) {
          var triggerBtn = document.getElementById(triggerBtnId);
          if (triggerBtn) triggerBtn.click();
        }
      }
      if (window._autoEditSource) {
        var autoEdit = window._autoEditSource;
        window._autoEditSource = null;
        var editBtn = container.querySelector('.edit-source-btn[data-id="' + autoEdit.id + '"]');
        if (editBtn) editBtn.click();
      }
      if (window._autoDiscoverHDHR) {
        window._autoDiscoverHDHR = null;
        var discoverBtn = document.getElementById('discover-hdhr-btn');
        if (discoverBtn) discoverBtn.click();
      }
    } catch (e) {
      document.getElementById('source-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load sources</p></div>';
    }
  }

  function getSetting(settings, key) {
    if (Array.isArray(settings)) {
      var found = settings.find(function(s) { return s.key === key; });
      return found ? found.value : '';
    }
    if (settings && typeof settings === 'object') return settings[key] || '';
    return '';
  }

  async function saveSetting(key, value) {
    var payload = {};
    payload[key] = value;
    try {
      var r = await api.put('/api/settings', payload);
      if (r.ok) toast('Setting saved');
      else toast('Failed to save setting', 'error');
    } catch (err) {
      toast('Failed to save setting', 'error');
    }
  }

  function makeSelect(id, options, currentValue) {
    var html = '<select id="' + esc(id) + '" class="settings-select">';
    for (var i = 0; i < options.length; i++) {
      var o = options[i];
      if (o.group) {
        html += '<optgroup label="' + esc(o.group) + '">';
        for (var j = 0; j < o.items.length; j++) {
          var item = o.items[j];
          var sel = item.value === currentValue ? ' selected' : '';
          html += '<option value="' + esc(item.value) + '"' + sel + '>' + esc(item.label) + '</option>';
        }
        html += '</optgroup>';
      } else {
        var sel2 = o.value === currentValue ? ' selected' : '';
        html += '<option value="' + esc(o.value) + '"' + sel2 + '>' + esc(o.label) + '</option>';
      }
    }
    html += '</select>';
    return html;
  }

  function bindAutoSave(container, selectId, settingKey) {
    var sel = document.getElementById(selectId);
    if (!sel && container) sel = container.querySelector('#' + selectId);
    if (!sel) return;
    sel.addEventListener('change', function() {
      saveSetting(settingKey, this.value);
    });
  }

  function bindToggle(container, checkboxId, settingKey) {
    var cb = container.querySelector('#' + checkboxId);
    if (!cb) return;
    cb.addEventListener('change', function() {
      saveSetting(settingKey, cb.checked ? 'true' : 'false');
    });
  }

  function bindTextSave(container, inputId, settingKey) {
    var inp = container.querySelector('#' + inputId);
    if (!inp) return;
    var timeout;
    inp.addEventListener('input', function() {
      clearTimeout(timeout);
      timeout = setTimeout(function() {
        saveSetting(settingKey, inp.value);
      }, 800);
    });
  }

  async function renderSettings(el) {
    el.innerHTML = '<h1 class="page-title">Settings</h1>' +
      '<div id="settings-content"><div class="skeleton" style="height:400px"></div></div>';

    var container = document.getElementById('settings-content');
    if (!container) return;

    var settings = {};
    var caps = null;

    try {
      var results = await Promise.all([
        api.get('/api/settings').then(function(r) { return r.json(); }),
        api.get('/api/capabilities').then(function(r) { return r.json(); }).catch(function() { return null; })
      ]);
      settings = results[0] || {};
      caps = results[1];
    } catch (e) {
      container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load settings</p></div>';
      return;
    }

    var html = '';

    html += '<div class="settings-section">' +
      '<div class="settings-section-header">Platform Capabilities</div>' +
      '<div class="settings-section-body">' +
      '<div id="caps-content">';

    if (caps && (caps.platforms || caps.video_encoders || caps.video_decoders)) {
      html += '<div style="margin-bottom:16px">' +
        '<span style="font-weight:500;font-size:14px;margin-right:8px">Detected Platforms</span>';
      if (caps.platforms && caps.platforms.length > 0) {
        for (var pi = 0; pi < caps.platforms.length; pi++) {
          html += '<span class="platform-badge platform-badge-hw">' + esc(caps.platforms[pi]) + '</span> ';
        }
      } else {
        html += '<span class="platform-badge platform-badge-none">Software only</span>';
      }
      if (caps.max_bit_depth > 0) {
        html += ' <span class="platform-badge platform-badge-sw">Max ' + caps.max_bit_depth + '-bit</span>';
      }
      html += '</div>';

      var hwEncoders = (caps.video_encoders || []).filter(function(e) { return e.hw; });
      var swEncoders = (caps.video_encoders || []).filter(function(e) { return !e.hw; });

      if (hwEncoders.length > 0 || swEncoders.length > 0) {
        html += '<div style="display:flex;gap:24px;margin-bottom:16px;flex-wrap:wrap">';
        if (hwEncoders.length > 0) {
          html += '<div style="flex:1;min-width:200px">' +
            '<div style="font-weight:500;font-size:13px;margin-bottom:8px;color:var(--text-muted)">Hardware Encoders</div>' +
            '<div class="codec-badges">';
          for (var he = 0; he < hwEncoders.length; he++) {
            html += '<span class="codec-badge codec-badge-hw" title="' + esc(hwEncoders[he].codec.toUpperCase() + ' - ' + hwEncoders[he].platform) + '">' + esc(hwEncoders[he].name) + '</span>';
          }
          html += '</div></div>';
        }
        if (swEncoders.length > 0) {
          html += '<div style="flex:1;min-width:200px">' +
            '<div style="font-weight:500;font-size:13px;margin-bottom:8px;color:var(--text-muted)">Software Encoders</div>' +
            '<div class="codec-badges">';
          for (var se = 0; se < swEncoders.length; se++) {
            html += '<span class="codec-badge codec-badge-sw" title="' + esc(swEncoders[se].codec.toUpperCase()) + '">' + esc(swEncoders[se].name) + '</span>';
          }
          html += '</div></div>';
        }
        html += '</div>';
      }

      var hwDecoders = (caps.video_decoders || []).filter(function(d) { return d.hw; });
      var swDecoders = (caps.video_decoders || []).filter(function(d) { return !d.hw; });

      if (hwDecoders.length > 0 || swDecoders.length > 0) {
        html += '<div style="display:flex;gap:24px;margin-bottom:16px;flex-wrap:wrap">';
        if (hwDecoders.length > 0) {
          html += '<div style="flex:1;min-width:200px">' +
            '<div style="font-weight:500;font-size:13px;margin-bottom:8px;color:var(--text-muted)">Hardware Decoders</div>' +
            '<div class="codec-badges">';
          for (var hd = 0; hd < hwDecoders.length; hd++) {
            html += '<span class="codec-badge codec-badge-hw" title="' + esc(hwDecoders[hd].platform) + '">' + esc(hwDecoders[hd].name + ' (' + hwDecoders[hd].codec.toUpperCase() + ')') + '</span>';
          }
          html += '</div></div>';
        }
        if (swDecoders.length > 0) {
          html += '<div style="flex:1;min-width:200px">' +
            '<div style="font-weight:500;font-size:13px;margin-bottom:8px;color:var(--text-muted)">Software Decoders</div>' +
            '<div class="codec-badges">';
          for (var sd = 0; sd < swDecoders.length; sd++) {
            html += '<span class="codec-badge codec-badge-sw">' + esc(swDecoders[sd].name + ' (' + swDecoders[sd].codec.toUpperCase() + ')') + '</span>';
          }
          html += '</div></div>';
        }
        html += '</div>';
      }

      if (caps.audio_encoders && caps.audio_encoders.length > 0) {
        html += '<div style="margin-bottom:16px">' +
          '<div style="font-weight:500;font-size:13px;margin-bottom:8px;color:var(--text-muted)">Audio Encoders</div>' +
          '<div class="codec-badges">';
        for (var ae = 0; ae < caps.audio_encoders.length; ae++) {
          html += '<span class="codec-badge codec-badge-sw">' + esc(caps.audio_encoders[ae].name) + '</span>';
        }
        html += '</div></div>';
      }

      var codecNames = { h264: 'H.264', h265: 'H.265 / HEVC', av1: 'AV1' };
      var encoderCodecs = ['h264', 'h265', 'av1'];
      var allVideoEncoders = caps.video_encoders || [];

      var hasEncoderDropdowns = false;
      for (var ci = 0; ci < encoderCodecs.length; ci++) {
        var matching = allVideoEncoders.filter(function(e) { return e.codec === encoderCodecs[ci]; });
        if (matching.length > 0) { hasEncoderDropdowns = true; break; }
      }

      if (hasEncoderDropdowns) {
        html += '<div style="border-top:1px solid var(--border);padding-top:16px;margin-top:8px">' +
          '<div style="font-weight:500;font-size:14px;margin-bottom:4px">Encoder Selection</div>' +
          '<div class="settings-section-desc">Choose which encoder to use for each codec. Hardware encoders are faster and use less CPU.</div>';

        for (var ec = 0; ec < encoderCodecs.length; ec++) {
          var codec = encoderCodecs[ec];
          var codecMatching = allVideoEncoders.filter(function(e) { return e.codec === codec; });
          if (codecMatching.length === 0) continue;

          var hwOpts = codecMatching.filter(function(e) { return e.hw; });
          var swOpts = codecMatching.filter(function(e) { return !e.hw; });
          var currentEnc = getSetting(settings, 'encoder_' + codec);

          var selectOpts = [{ value: '', label: 'Auto (fallback chain)' }];
          if (hwOpts.length > 0) {
            var hwItems = hwOpts.map(function(e) { return { value: e.name, label: e.name + ' (' + e.platform + ')' }; });
            selectOpts.push({ group: 'Hardware', items: hwItems });
          }
          if (swOpts.length > 0) {
            var swItems = swOpts.map(function(e) { return { value: e.name, label: e.name }; });
            selectOpts.push({ group: 'Software', items: swItems });
          }

          html += '<div class="settings-field">' +
            '<label>' + codecNames[codec] + '</label>' +
            makeSelect('enc-' + codec, selectOpts, currentEnc) +
            '</div>';
        }
        html += '</div>';
      }

      var decoderCodecNames = { h264: 'H.264', h265: 'H.265 / HEVC', av1: 'AV1', mpeg2: 'MPEG-2' };
      var decoderCodecs = ['h264', 'h265', 'av1', 'mpeg2'];
      var allVideoDecoders = caps.video_decoders || [];

      var hasDecoderDropdowns = false;
      for (var di = 0; di < decoderCodecs.length; di++) {
        var dmatching = allVideoDecoders.filter(function(d) { return d.codec === decoderCodecs[di]; });
        if (dmatching.length > 0) { hasDecoderDropdowns = true; break; }
      }

      if (hasDecoderDropdowns) {
        html += '<div style="border-top:1px solid var(--border);padding-top:16px;margin-top:16px">' +
          '<div style="font-weight:500;font-size:14px;margin-bottom:4px">Decoder Selection</div>' +
          '<div class="settings-section-desc">Choose which decoder to use for each source codec. Hardware decoders offload CPU.</div>';

        for (var dc = 0; dc < decoderCodecs.length; dc++) {
          var dcodec = decoderCodecs[dc];
          var dcodecMatching = allVideoDecoders.filter(function(d) { return d.codec === dcodec; });
          if (dcodecMatching.length === 0) continue;

          var dhwOpts = dcodecMatching.filter(function(d) { return d.hw; });
          var dswOpts = dcodecMatching.filter(function(d) { return !d.hw; });
          var currentDec = getSetting(settings, 'decoder_' + dcodec);

          var dselectOpts = [{ value: '', label: 'Auto (fallback chain)' }];
          if (dhwOpts.length > 0) {
            var dhwItems = dhwOpts.map(function(d) {
              var val = d.name + '_' + d.platform;
              return { value: val, label: d.name + ' (' + d.platform + ')' };
            });
            dselectOpts.push({ group: 'Hardware', items: dhwItems });
          }
          if (dswOpts.length > 0) {
            var dswItems = dswOpts.map(function(d) { return { value: d.name, label: d.name }; });
            dselectOpts.push({ group: 'Software', items: dswItems });
          }

          html += '<div class="settings-field">' +
            '<label>' + decoderCodecNames[dcodec] + '</label>' +
            makeSelect('dec-' + dcodec, dselectOpts, currentDec) +
            '</div>';
        }
        html += '</div>';
      }
    } else {
      html += '<div style="color:var(--text-muted);font-size:13px">Could not load platform capabilities.</div>';
    }

    html += '</div></div></div>';

    html += '<div class="settings-section">' +
      '<div class="settings-section-header">Playback Settings</div>' +
      '<div class="settings-section-body">' +
      '<div class="settings-section-desc">Default output settings for playback sessions.</div>' +
      '<div class="settings-field">' +
        '<label>Delivery Mode</label>' +
        makeSelect('setting-delivery', [
          { value: 'mse', label: 'MSE (browser)' },
          { value: 'hls', label: 'HLS (Jellyfin / Apple TV)' },
          { value: 'stream', label: 'Stream (direct)' }
        ], getSetting(settings, 'delivery') || 'mse') +
      '</div>' +
      '<div class="settings-field">' +
        '<label>Video Codec</label>' +
        makeSelect('setting-video-codec', [
          { value: 'copy', label: 'Passthrough (copy)' },
          { value: 'h264', label: 'H.264' },
          { value: 'h265', label: 'H.265 / HEVC' },
          { value: 'av1', label: 'AV1' }
        ], getSetting(settings, 'default_video_codec') || 'copy') +
      '</div>' +
      '</div></div>';

    html += '<div class="settings-section">' +
      '<div class="settings-section-header">Network Settings</div>' +
      '<div class="settings-section-body">' +
      '<div class="settings-field">' +
        '<label>Base URL</label>' +
        '<input type="text" id="setting-base-url" value="' + esc(getSetting(settings, 'base_url')) + '" placeholder="http://192.168.1.100:8080">' +
      '</div>' +
      '<div class="settings-field">' +
        '<label>DLNA Enabled</label>' +
        '<input type="checkbox" id="setting-dlna"' + (getSetting(settings, 'dlna_enabled') === 'true' ? ' checked' : '') + '>' +
        '<span class="field-hint">Advertise as DLNA MediaServer on the network</span>' +
      '</div>' +
      '<div class="settings-field">' +
        '<label>Jellyfin Enabled</label>' +
        '<input type="checkbox" id="setting-jellyfin"' + (getSetting(settings, 'jellyfin_enabled') === 'true' ? ' checked' : '') + '>' +
        '<span class="field-hint">Enable Jellyfin API emulation for native clients</span>' +
      '</div>' +
      '</div></div>';

    html += '<div class="settings-section">' +
      '<div class="settings-section-header">Language</div>' +
      '<div class="settings-section-body">' +
      '<div class="settings-field">' +
        '<label>Audio Language</label>' +
        '<input type="text" id="setting-audio-lang" value="' + esc(getSetting(settings, 'audio_language')) + '" placeholder="eng">' +
        '<span class="field-hint">ISO 639 language code for preferred audio track. Empty = first available non-AD track.</span>' +
      '</div>' +
      '<div class="settings-field">' +
        '<label>Subtitle Language</label>' +
        '<input type="text" id="setting-subtitle-lang" value="' + esc(getSetting(settings, 'subtitle_language')) + '" placeholder="eng">' +
        '<span class="field-hint">ISO 639 language code for preferred subtitle track. Empty = no subtitles.</span>' +
      '</div>' +
      '</div></div>';

    html += '<div class="settings-section">' +
      '<div class="settings-section-header">Integrations</div>' +
      '<div class="settings-section-body">' +
      '<div class="settings-section-desc">Third-party API keys for metadata enrichment.</div>' +
      '<div class="settings-field">' +
        '<label>TMDB API Key</label>' +
        '<input type="text" id="setting-tmdb-key" value="' + esc(getSetting(settings, 'tmdb_api_key')) + '" placeholder="Enter your TMDB API key (v3 auth)">' +
        '<span class="field-hint"><a href="https://www.themoviedb.org/settings/api" target="_blank">Get API key</a></span>' +
      '</div>' +
      '</div></div>';

    var baseUrl = getSetting(settings, 'base_url') || window.location.origin;
    html += '<div class="settings-section">' +
      '<div class="settings-section-header">Google OAuth</div>' +
      '<div class="settings-section-body">' +
      '<div class="settings-section-desc">Allow users with an email on their account to sign in with Google. <a href="https://console.cloud.google.com/apis/credentials" target="_blank">Create OAuth credentials</a></div>' +
      '<div class="settings-field">' +
        '<label>Client ID</label>' +
        '<input type="text" id="setting-google-client-id" value="' + esc(getSetting(settings, 'google_client_id')) + '" placeholder="xxxx.apps.googleusercontent.com">' +
      '</div>' +
      '<div class="settings-field">' +
        '<label>Client Secret</label>' +
        '<input type="password" id="setting-google-client-secret" value="' + esc(getSetting(settings, 'google_client_secret')) + '" placeholder="GOCSPX-...">' +
      '</div>' +
      '<div style="margin-top:8px;padding:10px 12px;background:var(--surface);border:1px solid var(--border);border-radius:6px;font-size:13px">' +
        '<div style="color:var(--text-muted);margin-bottom:4px">Redirect URI (add this in Google Console)</div>' +
        '<code style="color:var(--accent);word-break:break-all">' + esc(baseUrl) + '/api/auth/google/callback</code>' +
      '</div>' +
      '</div></div>';

    html += '<div class="settings-section">' +
      '<div class="settings-section-header">Import / Export</div>' +
      '<div class="settings-section-body">' +
      '<div class="settings-section-desc">Export your configuration as JSON. "Channels" exports channels, groups, and stream assignments. "Full" includes everything (clients, source profiles, sources, settings, EPG sources). Import merges with existing data — duplicates are skipped.</div>' +
      '<div style="display:flex;gap:8px;flex-wrap:wrap">' +
      '<button class="btn btn-secondary" id="export-channels-btn">Export Channels</button>' +
      '<button class="btn btn-secondary" id="export-full-btn">Export Full</button>' +
      '<button class="btn btn-primary" id="import-btn">Import</button>' +
      '<input type="file" accept=".json" id="import-file-input" style="display:none">' +
      '</div>' +
      '</div></div>';

    html += '<div class="settings-section">' +
      '<div class="settings-section-header">Database Management</div>' +
      '<div class="settings-section-body">' +
      '<div class="settings-section-desc">Soft Reset removes all derived data (channels, streams, EPG programs, clients, source profiles, favorites, recordings) but keeps your sources, EPG sources, users, and settings. Hard Reset restores factory defaults — all data is deleted and default credentials are restored (admin/admin).</div>' +
      '<div style="display:flex;gap:8px">' +
      '<button class="btn btn-danger" id="soft-reset-btn">Soft Reset</button>' +
      '<button class="btn btn-danger" id="hard-reset-btn">Hard Reset</button>' +
      '</div>' +
      '</div></div>';

    container.innerHTML = html;

    var exportChannelsBtn = document.getElementById('export-channels-btn');
    if (exportChannelsBtn) {
      exportChannelsBtn.addEventListener('click', async function() {
        exportChannelsBtn.disabled = true;
        try {
          var resp = await fetch('/api/settings/export?scope=channels', { headers: { 'Authorization': 'Bearer ' + api.token } });
          var blob = await resp.blob();
          var a = document.createElement('a');
          a.href = URL.createObjectURL(blob);
          a.download = 'mediahub-channels-' + new Date().toISOString().slice(0, 10) + '.json';
          a.click();
          URL.revokeObjectURL(a.href);
        } catch (err) { toast(err.message, 'error'); }
        exportChannelsBtn.disabled = false;
      });
    }

    var exportFullBtn = document.getElementById('export-full-btn');
    if (exportFullBtn) {
      exportFullBtn.addEventListener('click', async function() {
        exportFullBtn.disabled = true;
        try {
          var resp = await fetch('/api/settings/export?scope=full', { headers: { 'Authorization': 'Bearer ' + api.token } });
          var blob = await resp.blob();
          var a = document.createElement('a');
          a.href = URL.createObjectURL(blob);
          a.download = 'mediahub-full-' + new Date().toISOString().slice(0, 10) + '.json';
          a.click();
          URL.revokeObjectURL(a.href);
        } catch (err) { toast(err.message, 'error'); }
        exportFullBtn.disabled = false;
      });
    }

    var importFileInput = document.getElementById('import-file-input');
    var importBtn = document.getElementById('import-btn');
    if (importBtn && importFileInput) {
      importBtn.addEventListener('click', function() { importFileInput.click(); });
      importFileInput.addEventListener('change', async function() {
        var file = importFileInput.files[0];
        if (!file) return;
        try {
          var text = await file.text();
          var data = JSON.parse(text);
          var summary = [];
          if (data.channels) summary.push(data.channels.length + ' channels');
          if (data.channel_groups) summary.push(data.channel_groups.length + ' groups');
          if (data.clients) summary.push(data.clients.length + ' clients');
          if (data.source_profiles) summary.push(data.source_profiles.length + ' source profiles');
          if (data.source_configs) summary.push(data.source_configs.length + ' sources');
          if (data.epg_sources) summary.push(data.epg_sources.length + ' EPG sources');
          if (data.settings) summary.push(Object.keys(data.settings).length + ' settings');
          if (!confirm('Import ' + (summary.join(', ') || 'data') + '?\n\nExisting items with the same name will be skipped.')) {
            importFileInput.value = '';
            return;
          }
          var result = await api.post('/api/settings/import', data);
          var body = await result.json();
          toast('Import complete: ' + (body.imported || 0) + ' items imported');
          router.navigate('settings');
        } catch (err) { toast('Import failed: ' + err.message, 'error'); }
        importFileInput.value = '';
      });
    }

    var softResetBtn = document.getElementById('soft-reset-btn');
    if (softResetBtn) {
      softResetBtn.addEventListener('click', async function() {
        if (!confirm('Soft Reset will delete all channels, streams, EPG programs, clients, source profiles, favorites, and recordings. Sources, EPG sources, users, and settings will be preserved.\n\nAre you sure?')) return;
        softResetBtn.disabled = true;
        try {
          await api.post('/api/settings/soft-reset');
          toast('Soft reset complete');
        } catch (err) { toast(err.message, 'error'); }
        softResetBtn.disabled = false;
      });
    }

    var hardResetBtn = document.getElementById('hard-reset-btn');
    if (hardResetBtn) {
      hardResetBtn.addEventListener('click', async function() {
        if (!confirm('Hard Reset will delete ALL data and restore factory defaults. You will be logged out.\n\nAre you sure?')) return;
        var resetConfirm = prompt('This will delete ALL data. Type RESET to confirm:');
        if (resetConfirm !== 'RESET') { toast('Hard reset cancelled', 'error'); return; }
        hardResetBtn.disabled = true;
        try {
          await api.post('/api/settings/hard-reset');
          toast('Hard reset complete. Logging out...');
          setTimeout(function() {
            api.token = null;
            api.user = null;
            router.navigate('login');
          }, 1500);
        } catch (err) {
          toast(err.message, 'error');
          hardResetBtn.disabled = false;
        }
      });
    }

    if (caps && caps.video_encoders) {
      var encCodecs = ['h264', 'h265', 'av1'];
      for (var bi = 0; bi < encCodecs.length; bi++) {
        bindAutoSave(container, 'enc-' + encCodecs[bi], 'encoder_' + encCodecs[bi]);
      }
    }
    if (caps && caps.video_decoders) {
      var decCodecs = ['h264', 'h265', 'av1', 'mpeg2'];
      for (var bdi = 0; bdi < decCodecs.length; bdi++) {
        bindAutoSave(container, 'dec-' + decCodecs[bdi], 'decoder_' + decCodecs[bdi]);
      }
    }

    bindAutoSave(container, 'setting-delivery', 'delivery');
    bindAutoSave(container, 'setting-video-codec', 'default_video_codec');

    bindTextSave(container, 'setting-base-url', 'base_url');
    bindToggle(container, 'setting-dlna', 'dlna_enabled');
    bindToggle(container, 'setting-jellyfin', 'jellyfin_enabled');
    bindTextSave(container, 'setting-audio-lang', 'audio_language');
    bindTextSave(container, 'setting-subtitle-lang', 'subtitle_language');
    bindTextSave(container, 'setting-tmdb-key', 'tmdb_api_key');
    bindTextSave(container, 'setting-google-client-id', 'google_client_id');
    bindTextSave(container, 'setting-google-client-secret', 'google_client_secret');
  }

  async function renderUsers(el) {
    var currentUser = api.user;
    el.innerHTML = '<h1 class="page-title">Users</h1>' +
      '<div style="margin-bottom:16px;display:flex;gap:8px">' +
      '<button class="btn btn-primary" id="add-user-btn">' + icons.plus + ' Add User</button>' +
      '<button class="btn btn-ghost" id="invite-user-btn">' + icons.invite + ' Invite User</button>' +
      '</div>' +
      '<div id="user-list"><div class="skeleton" style="height:200px"></div></div>' +
      '<div id="invite-section"></div>';

    var userFormBody =
      '<div class="form-group"><label class="form-label">Username</label><input class="form-input" id="new-username" placeholder="username"></div>' +
      '<div class="form-group"><label class="form-label">Password</label><input class="form-input" id="new-password" type="password" placeholder="password"></div>' +
      '<div class="form-group"><label class="form-label">Email</label><input class="form-input" id="new-email" type="email" placeholder="user@example.com (optional, for Google SSO)"></div>' +
      '<div class="form-group"><label class="form-label">Role</label>' +
      '<select class="form-input" id="new-role"><option value="standard">Standard</option><option value="admin">Admin</option><option value="jellyfin">Jellyfin</option></select></div>';

    document.getElementById('add-user-btn').addEventListener('click', function() {
      var modal = showFormModal('New User', userFormBody, { id: 'user-modal', saveLabel: 'Create' });
      modal.querySelector('.modal-save-btn').addEventListener('click', async function() {
        var un = document.getElementById('new-username').value.trim();
        var pw = document.getElementById('new-password').value;
        var em = document.getElementById('new-email').value.trim();
        var role = document.getElementById('new-role').value;
        if (!un || !pw) { toast('Username and password required', 'error'); return; }
        var createBody = { username: un, password: pw, role: role };
        if (em) createBody.email = em;
        try {
          var r = await api.post('/api/users', createBody);
          if (r.ok) {
            toast('User created');
            modal.remove();
            renderUsers(el);
          } else {
            var data = await r.json().catch(function() { return {}; });
            toast(data.error || 'Failed to create user', 'error');
          }
        } catch (err) {
          toast('Failed to create user', 'error');
        }
      });
    });

    try {
      var resp = await api.get('/api/users');
      var users = await resp.json();
      if (!Array.isArray(users)) users = [];
      var container = document.getElementById('user-list');
      if (!container) return;
      if (users.length === 0) {
        container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No users</p></div>';
        return;
      }

      function roleBadgeClass(role) {
        if (role === 'admin') return 'badge-admin';
        if (role === 'jellyfin') return 'badge-jellyfin';
        return 'badge-standard';
      }

      function renderUserTable() {
        var html = '<table class="list-table"><thead><tr>' +
          '<th>Username</th><th>Email</th><th>Role</th><th></th>' +
          '</tr></thead><tbody>';
        for (var i = 0; i < users.length; i++) {
          var u = users[i];
          var username = u.Username || u.username;
          var email = u.Email || u.email || '';
          var role = u.Role || u.role || 'standard';
          var uid = u.ID || u.id;
          var isMe = currentUser && currentUser.username === username;
          var isAdmin = role === 'admin' || u.IsAdmin || u.is_admin;
          var roleClass = roleBadgeClass(role);

          html += '<tr' + (isMe ? ' style="background:rgba(91,110,239,0.06)"' : '') + '>' +
            '<td><div style="display:flex;align-items:center;gap:8px">' +
            '<div style="width:32px;height:32px;border-radius:50%;background:var(--accent);display:flex;align-items:center;justify-content:center;color:#fff;font-weight:600;font-size:13px;flex-shrink:0">' + esc(username.charAt(0).toUpperCase()) + '</div>' +
            '<div>' + esc(username) +
            (isMe ? ' <span class="badge badge-you">You</span>' : '') +
            '</div></div></td>' +
            '<td>' + (email ? '<span style="color:var(--text-secondary);font-size:13px">' + esc(email) + '</span>' : '<span style="color:var(--text-muted);font-size:13px">-</span>') + '</td>' +
            '<td><div class="user-role-cell" data-uid="' + esc(uid) + '">' +
            '<span class="badge ' + roleClass + '" id="role-badge-' + esc(uid) + '">' + esc(role) + '</span>' +
            '<select class="user-role-select" id="role-select-' + esc(uid) + '" data-uid="' + esc(uid) + '" style="display:none">' +
            '<option value="admin"' + (role === 'admin' ? ' selected' : '') + '>Admin</option>' +
            '<option value="standard"' + (role === 'standard' ? ' selected' : '') + '>Standard</option>' +
            '<option value="jellyfin"' + (role === 'jellyfin' ? ' selected' : '') + '>Jellyfin</option>' +
            '</select></div></td>' +
            '<td><div class="actions-cell">' +
            '<button class="btn btn-sm btn-ghost user-edit-role-btn" data-uid="' + esc(uid) + '" title="Change role">' + icons.edit + '</button>' +
            '<button class="btn btn-sm btn-ghost user-email-btn" data-uid="' + esc(uid) + '" data-email="' + esc(email) + '" data-username="' + esc(username) + '" title="Edit email">' + icons.edit + '</button>' +
            '<button class="btn btn-sm btn-ghost user-pw-btn" data-uid="' + esc(uid) + '" data-username="' + esc(username) + '" title="Change password">' + icons.key + '</button>' +
            (!isMe ? '<button class="btn btn-sm btn-icon btn-danger user-del-btn" data-uid="' + esc(uid) + '" data-username="' + esc(username) + '" title="Delete user">' + icons.trash + '</button>' : '') +
            '</div></td></tr>';
        }
        html += '</tbody></table>';
        container.innerHTML = html;
        bindUserEvents();
      }

      function bindUserEvents() {
        container.querySelectorAll('.user-edit-role-btn').forEach(function(btn) {
          btn.addEventListener('click', function() {
            var uid = this.getAttribute('data-uid');
            var badge = document.getElementById('role-badge-' + uid);
            var sel = document.getElementById('role-select-' + uid);
            if (!badge || !sel) return;
            if (sel.style.display === 'none') {
              badge.style.display = 'none';
              sel.style.display = 'inline-block';
              sel.focus();
              var saveHandler = async function() {
                var newRole = sel.value;
                try {
                  var r = await api.put('/api/users/' + uid, { role: newRole });
                  if (r.ok) {
                    toast('Role updated');
                    renderUsers(el);
                  } else {
                    var data = await r.json().catch(function() { return {}; });
                    toast(data.error || 'Failed to update role', 'error');
                    sel.style.display = 'none';
                    badge.style.display = '';
                  }
                } catch (err) {
                  toast('Failed to update role', 'error');
                  sel.style.display = 'none';
                  badge.style.display = '';
                }
              };
              sel.addEventListener('change', saveHandler, { once: true });
              sel.addEventListener('blur', function() {
                setTimeout(function() {
                  sel.style.display = 'none';
                  badge.style.display = '';
                }, 150);
              }, { once: true });
            } else {
              sel.style.display = 'none';
              badge.style.display = '';
            }
          });
        });

        container.querySelectorAll('.user-email-btn').forEach(function(btn) {
          btn.addEventListener('click', function() {
            var uid = this.getAttribute('data-uid');
            var username = this.getAttribute('data-username');
            var currentEmail = this.getAttribute('data-email') || '';
            showEmailModal(uid, username, currentEmail);
          });
        });

        container.querySelectorAll('.user-pw-btn').forEach(function(btn) {
          btn.addEventListener('click', function() {
            var uid = this.getAttribute('data-uid');
            var username = this.getAttribute('data-username');
            showPasswordModal(uid, username);
          });
        });

        container.querySelectorAll('.user-del-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var uid = this.getAttribute('data-uid');
            var username = this.getAttribute('data-username');
            if (!confirm('Delete user "' + username + '"? This cannot be undone.')) return;
            try {
              var r = await api.del('/api/users/' + uid);
              if (r.ok || r.status === 204) {
                toast('User deleted');
                renderUsers(el);
              } else {
                toast('Failed to delete user', 'error');
              }
            } catch (err) {
              toast('Failed to delete user', 'error');
            }
          });
        });
      }

      function showPasswordModal(uid, username) {
        var existing = document.getElementById('pw-modal');
        if (existing) existing.remove();

        var html = '<div class="modal-overlay" id="pw-modal">' +
          '<div class="modal-content">' +
          '<div class="modal-header"><span>Change Password - ' + esc(username) + '</span><button class="modal-close-x pw-close-x" title="Close">\u2715</button></div>' +
          '<div class="modal-body">' +
          '<div class="form-group"><label class="form-label">New Password</label>' +
          '<input class="form-input" id="pw-new" type="password" placeholder="Enter new password"></div>' +
          '<div class="form-group"><label class="form-label">Confirm Password</label>' +
          '<input class="form-input" id="pw-confirm" type="password" placeholder="Confirm new password"></div>' +
          '</div>' +
          '<div class="modal-footer">' +
          '<button class="btn btn-ghost" id="pw-cancel">Cancel</button>' +
          '<button class="btn btn-primary" id="pw-save">Change Password</button>' +
          '</div></div></div>';

        document.body.insertAdjacentHTML('beforeend', html);
        document.getElementById('pw-new').focus();

        document.getElementById('pw-cancel').addEventListener('click', function() {
          document.getElementById('pw-modal').remove();
        });
        document.querySelector('.pw-close-x').addEventListener('click', function() {
          document.getElementById('pw-modal').remove();
        });
        document.getElementById('pw-modal').addEventListener('click', function(e) {
          if (e.target === this) this.remove();
        });
        document.getElementById('pw-save').addEventListener('click', async function() {
          var newPw = document.getElementById('pw-new').value;
          var confirmPw = document.getElementById('pw-confirm').value;
          if (!newPw) { toast('Password required', 'error'); return; }
          if (newPw !== confirmPw) { toast('Passwords do not match', 'error'); return; }
          try {
            var r = await api.put('/api/users/' + uid + '/password', { password: newPw });
            if (r.ok) {
              toast('Password changed');
              document.getElementById('pw-modal').remove();
            } else {
              var data = await r.json().catch(function() { return {}; });
              toast(data.error || 'Failed to change password', 'error');
            }
          } catch (err) {
            toast('Failed to change password', 'error');
          }
        });
      }

      function showEmailModal(uid, username, currentEmail) {
        var existing = document.getElementById('email-modal');
        if (existing) existing.remove();

        var html = '<div class="modal-overlay" id="email-modal">' +
          '<div class="modal-content">' +
          '<div class="modal-header"><span>Edit Email - ' + esc(username) + '</span><button class="modal-close-x email-close-x" title="Close">\u2715</button></div>' +
          '<div class="modal-body">' +
          '<div class="form-group"><label class="form-label">Email</label>' +
          '<input class="form-input" id="email-input" type="email" value="' + esc(currentEmail) + '" placeholder="user@example.com (optional, for Google SSO)"></div>' +
          '</div>' +
          '<div class="modal-footer">' +
          '<button class="btn btn-ghost" id="email-cancel">Cancel</button>' +
          '<button class="btn btn-primary" id="email-save">Save</button>' +
          '</div></div></div>';

        document.body.insertAdjacentHTML('beforeend', html);
        document.getElementById('email-input').focus();

        document.getElementById('email-cancel').addEventListener('click', function() {
          document.getElementById('email-modal').remove();
        });
        document.querySelector('.email-close-x').addEventListener('click', function() {
          document.getElementById('email-modal').remove();
        });
        document.getElementById('email-modal').addEventListener('click', function(e) {
          if (e.target === this) this.remove();
        });
        document.getElementById('email-save').addEventListener('click', async function() {
          var newEmail = document.getElementById('email-input').value.trim();
          try {
            var r = await api.put('/api/users/' + uid, { email: newEmail });
            if (r.ok) {
              toast('Email updated');
              document.getElementById('email-modal').remove();
              renderUsers(el);
            } else {
              var data = await r.json().catch(function() { return {}; });
              toast(data.error || 'Failed to update email', 'error');
            }
          } catch (err) {
            toast('Failed to update email', 'error');
          }
        });
      }

      renderUserTable();
    } catch (e) {
      document.getElementById('user-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load users</p></div>';
    }

    document.getElementById('invite-user-btn').addEventListener('click', function() {
      showInviteModal(function() { loadInviteSection(); });
    });

    function showInviteModal(onSuccess) {
      var existing = document.getElementById('invite-modal');
      if (existing) existing.remove();
      var html = '<div class="modal-overlay" id="invite-modal">' +
        '<div class="modal-content" style="max-width:420px">' +
        '<div class="modal-header"><span>Create Invite</span><button class="modal-close-x invite-close-x" title="Close">\u2715</button></div>' +
        '<div class="modal-body">' +
        '<div class="form-group"><label class="form-label">Role</label>' +
        '<select class="form-input" id="invite-role">' +
        '<option value="standard">Standard</option>' +
        '<option value="admin">Admin</option>' +
        '<option value="jellyfin">Jellyfin</option>' +
        '</select></div>' +
        '<div class="form-group"><label class="form-label">Expires In</label>' +
        '<input class="form-input" id="invite-expires" value="24h" placeholder="e.g. 24h, 7d, 1h"></div>' +
        '<div id="invite-result" style="display:none"></div>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button class="btn btn-ghost" id="invite-cancel">Cancel</button>' +
        '<button class="btn btn-primary" id="invite-create">Create</button>' +
        '</div></div></div>';
      document.body.insertAdjacentHTML('beforeend', html);
      document.getElementById('invite-cancel').addEventListener('click', function() {
        document.getElementById('invite-modal').remove();
      });
      document.querySelector('.invite-close-x').addEventListener('click', function() {
        document.getElementById('invite-modal').remove();
      });
      document.getElementById('invite-modal').addEventListener('click', function(e) {
        if (e.target === this) this.remove();
      });
      document.getElementById('invite-create').addEventListener('click', async function() {
        var role = document.getElementById('invite-role').value;
        var expiresIn = document.getElementById('invite-expires').value.trim() || '24h';
        try {
          var r = await api.post('/api/invites', { role: role, expires_in: expiresIn });
          if (r.ok) {
            var data = await r.json();
            var resultEl = document.getElementById('invite-result');
            resultEl.style.display = 'block';
            resultEl.innerHTML = '<div class="form-group" style="margin-top:12px">' +
              '<label class="form-label">Invite Token (copy now)</label>' +
              '<div style="display:flex;gap:8px">' +
              '<input class="form-input" id="invite-token-display" value="' + esc(data.token || '') + '" readonly style="font-family:monospace;font-size:12px">' +
              '<button class="btn btn-ghost" id="invite-copy-btn" title="Copy">' + icons.copy + '</button>' +
              '</div></div>';
            document.getElementById('invite-copy-btn').addEventListener('click', function() {
              var inp = document.getElementById('invite-token-display');
              if (inp) {
                inp.select();
                try { navigator.clipboard.writeText(inp.value); toast('Copied to clipboard'); } catch (e) { toast('Select and copy manually', 'error'); }
              }
            });
            document.getElementById('invite-create').style.display = 'none';
            document.getElementById('invite-cancel').textContent = 'Close';
            document.getElementById('invite-cancel').addEventListener('click', function() {
              document.getElementById('invite-modal').remove();
              if (onSuccess) onSuccess();
            });
          } else {
            var err = await r.json().catch(function() { return {}; });
            toast(err.error || 'Failed to create invite', 'error');
          }
        } catch (e) {
          toast('Failed to create invite', 'error');
        }
      });
    }

    async function loadInviteSection() {
      var section = document.getElementById('invite-section');
      if (!section) return;
      try {
        var resp = await api.get('/api/invites');
        var invites = await resp.json();
        if (!Array.isArray(invites) || invites.length === 0) {
          section.innerHTML = '';
          return;
        }
        var html = '<div style="margin-top:32px"><h2 style="font-size:16px;font-weight:600;margin-bottom:12px;color:var(--text)">Pending Invites</h2>' +
          '<table class="list-table"><thead><tr>' +
          '<th>Token</th><th>Role</th><th>Created</th><th>Expires</th><th>Used</th><th></th>' +
          '</tr></thead><tbody>';
        for (var i = 0; i < invites.length; i++) {
          var inv = invites[i];
          var token = inv.token || '';
          var truncated = token.length > 16 ? token.substring(0, 16) + '...' : token;
          var created = inv.created_at ? new Date(inv.created_at).toLocaleString() : '-';
          var expires = inv.expires_at ? new Date(inv.expires_at).toLocaleString() : '-';
          var used = inv.used || inv.is_used;
          html += '<tr>' +
            '<td><span style="font-family:monospace;font-size:12px">' + esc(truncated) + '</span></td>' +
            '<td><span class="badge badge-' + esc(inv.role || 'standard') + '">' + esc(inv.role || 'standard') + '</span></td>' +
            '<td>' + esc(created) + '</td>' +
            '<td>' + esc(expires) + '</td>' +
            '<td>' + (used ? '<span class="badge badge-admin">Yes</span>' : '<span class="badge">No</span>') + '</td>' +
            '<td><div class="actions-cell">' +
            (!used ? '<button class="btn btn-sm btn-icon btn-danger invite-del-btn" data-token="' + esc(token) + '" title="Delete">' + icons.trash + '</button>' : '') +
            '</div></td></tr>';
        }
        html += '</tbody></table></div>';
        section.innerHTML = html;
        section.querySelectorAll('.invite-del-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var token = this.getAttribute('data-token');
            if (!confirm('Delete this invite?')) return;
            try {
              var r = await api.del('/api/invites/' + token);
              if (r.ok || r.status === 204) {
                toast('Invite deleted');
                loadInviteSection();
              } else {
                toast('Failed to delete invite', 'error');
              }
            } catch (e) {
              toast('Failed to delete invite', 'error');
            }
          });
        });
      } catch (e) {
        section.innerHTML = '';
      }
    }

    loadInviteSection();
  }

  async function renderWireGuard(el) {
    el.innerHTML = '<h1 class="page-title">WireGuard</h1>' +
      '<div id="wg-status-bar" style="margin-bottom:16px"><div class="skeleton" style="height:48px"></div></div>' +
      '<div style="margin-bottom:16px"><button class="btn btn-primary" id="add-wg-btn">' + icons.plus + ' Add Profile</button></div>' +
      '<div id="wg-list"><div class="skeleton" style="height:200px"></div></div>';

    var wgEditId = null;
    var wgFormBody =
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="wg-name" placeholder="My VPN"></div>' +
      '<div class="form-group"><label class="form-label">Private Key</label><input class="form-input" id="wg-privkey" placeholder="base64 private key"></div>' +
      '<div class="form-group"><label class="form-label">Endpoint</label><input class="form-input" id="wg-endpoint" placeholder="vpn.example.com:51820"></div>' +
      '<div class="form-group"><label class="form-label">Peer Public Key</label><input class="form-input" id="wg-pubkey" placeholder="base64 public key"></div>' +
      '<div class="form-group"><label class="form-label">Address</label><input class="form-input" id="wg-address" placeholder="10.0.0.2/24"></div>' +
      '<div class="form-group"><label class="form-label">Allowed IPs</label><input class="form-input" id="wg-allowedips" value="0.0.0.0/0" placeholder="0.0.0.0/0"></div>' +
      '<div class="form-group"><label class="form-label">DNS (optional)</label><input class="form-input" id="wg-dns" placeholder="1.1.1.1"></div>';

    function openWGModal(title, saveLabel) {
      var modal = showFormModal(title, wgFormBody, { id: 'wg-modal', saveLabel: saveLabel });
      modal.querySelector('.modal-save-btn').addEventListener('click', async function() {
        var payload = {
          name: document.getElementById('wg-name').value.trim(),
          private_key: document.getElementById('wg-privkey').value.trim(),
          endpoint: document.getElementById('wg-endpoint').value.trim(),
          public_key: document.getElementById('wg-pubkey').value.trim(),
          address: document.getElementById('wg-address').value.trim(),
          allowed_ips: document.getElementById('wg-allowedips').value.trim(),
          dns: document.getElementById('wg-dns').value.trim()
        };
        if (!payload.name) { toast('Name required', 'error'); return; }
        try {
          var r;
          if (wgEditId) {
            r = await api.put('/api/wireguard/profiles/' + wgEditId, payload);
          } else {
            if (!payload.private_key || !payload.endpoint || !payload.public_key || !payload.address) {
              toast('All fields except DNS are required', 'error');
              return;
            }
            r = await api.post('/api/wireguard/profiles', payload);
          }
          if (r.ok) {
            toast(wgEditId ? 'Profile updated' : 'Profile created');
            modal.remove();
            renderWireGuard(el);
          } else {
            var data = await r.json().catch(function() { return {}; });
            toast(data.error || 'Failed to save profile', 'error');
          }
        } catch (err) {
          toast('Failed to save profile', 'error');
        }
      });
      return modal;
    }

    document.getElementById('add-wg-btn').addEventListener('click', function() {
      wgEditId = null;
      openWGModal('New WireGuard Profile', 'Create');
    });

    try {
      var statusResp = await api.get('/api/wireguard/status');
      var status = await statusResp.json();
      var statusBar = document.getElementById('wg-status-bar');
      if (statusBar) {
        if (status.connected) {
          var hasHandshake = status.last_handshake_sec > 0;
          var stateLabel = hasHandshake ? 'Connected' : 'Connecting...';
          var stateColor = hasHandshake ? 'var(--success)' : 'var(--warning)';
          var dotColor = hasHandshake ? 'var(--success)' : 'var(--warning)';
          var handshakeText = '';
          if (hasHandshake) {
            var secAgo = Math.floor(Date.now() / 1000) - status.last_handshake_sec;
            if (secAgo < 60) handshakeText = secAgo + 's ago';
            else if (secAgo < 3600) handshakeText = Math.floor(secAgo / 60) + 'm ago';
            else handshakeText = Math.floor(secAgo / 3600) + 'h ago';
          } else {
            handshakeText = 'No handshake';
          }
          var handshakeColor = hasHandshake ? '#94a3b8' : '#ef4444';
          statusBar.innerHTML = '<div class="card" style="background:rgba(52,211,153,0.08);border:1px solid rgba(52,211,153,0.3);padding:12px 16px;display:flex;align-items:center;gap:12px;flex-wrap:wrap">' +
            '<span style="display:inline-block;width:12px;height:12px;border-radius:50%;background:' + dotColor + '"></span>' +
            '<div style="flex:1;min-width:200px">' +
            '<div style="color:' + stateColor + ';font-weight:600;font-size:14px">' + stateLabel + '</div>' +
            '<div style="color:#cbd5e1;font-size:13px;margin-top:2px">' + esc(status.profile_name) + ' &mdash; ' + esc(status.endpoint) + '</div>' +
            '</div>' +
            '<div style="display:flex;gap:16px;align-items:center;flex-shrink:0">' +
            '<div style="text-align:center">' +
            '<div style="color:#94a3b8;font-size:11px;text-transform:uppercase;letter-spacing:.5px">Proxy</div>' +
            '<div style="color:#e2e8f0;font-size:13px;font-weight:600;font-family:monospace">127.0.0.1:' + status.proxy_port + '</div>' +
            '</div>' +
            (status.exit_ip ? '<div style="text-align:center">' +
            '<div style="color:#94a3b8;font-size:11px;text-transform:uppercase;letter-spacing:.5px">Exit IP</div>' +
            '<div style="color:#e2e8f0;font-size:13px;font-weight:600;font-family:monospace">' + esc(status.exit_ip) + '</div>' +
            '</div>' : '') +
            '<div style="text-align:center">' +
            '<div style="color:#94a3b8;font-size:11px;text-transform:uppercase;letter-spacing:.5px">TX / RX</div>' +
            '<div style="color:#e2e8f0;font-size:13px;font-weight:600">' + formatBytes(status.tx_bytes || 0) + ' / ' + formatBytes(status.rx_bytes || 0) + '</div>' +
            '</div>' +
            '<div style="text-align:center">' +
            '<div style="color:#94a3b8;font-size:11px;text-transform:uppercase;letter-spacing:.5px">Handshake</div>' +
            '<div style="color:' + handshakeColor + ';font-size:13px;font-weight:600">' + handshakeText + '</div>' +
            '</div>' +
            '<button class="btn btn-sm btn-ghost" id="wg-reconnect-btn" title="Reconnect">' + icons.refresh + ' Reconnect</button>' +
            '</div>' +
            '</div>';
          document.getElementById('wg-reconnect-btn').addEventListener('click', async function() {
            try {
              var r = await api.post('/api/wireguard/reconnect', {});
              if (r.ok) {
                toast('WireGuard reconnecting...');
                setTimeout(function() { renderWireGuard(el); }, 2000);
              } else {
                var d = await r.json().catch(function() { return {}; });
                toast(d.error || 'Reconnect failed', 'error');
              }
            } catch (err) {
              toast('Reconnect failed', 'error');
            }
          });
        } else {
          statusBar.innerHTML = '<div class="card" style="background:rgba(251,191,36,0.08);border:1px solid rgba(251,191,36,0.3);padding:12px 16px;display:flex;align-items:center;gap:12px">' +
            '<span style="display:inline-block;width:12px;height:12px;border-radius:50%;background:var(--warning)"></span>' +
            '<div style="color:#cbd5e1"><strong style="color:var(--warning)">Disconnected</strong> <span style="color:#94a3b8">&mdash; No active WireGuard tunnel</span></div>' +
            '</div>';
        }
      }
    } catch (e) {
      var sb = document.getElementById('wg-status-bar');
      if (sb) sb.innerHTML = '';
    }

    try {
      var resp = await api.get('/api/wireguard/profiles');
      var profiles = await resp.json();
      if (!Array.isArray(profiles)) profiles = [];
      var container = document.getElementById('wg-list');
      if (!container) return;

      if (profiles.length === 0) {
        container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No WireGuard profiles configured</p></div>';
        return;
      }

      var html = '<table class="list-table"><thead><tr>' +
        '<th>Name</th><th>Endpoint</th><th>Address</th><th>Status</th><th></th>' +
        '</tr></thead><tbody>';
      for (var i = 0; i < profiles.length; i++) {
        var p = profiles[i];
        var badge = p.is_active
          ? '<span class="badge badge-enabled">ACTIVE</span>'
          : '<span class="badge badge-disabled">INACTIVE</span>';
        html += '<tr>' +
          '<td>' + esc(p.name) + '</td>' +
          '<td>' + esc(p.endpoint) + '</td>' +
          '<td>' + esc(p.address) + '</td>' +
          '<td>' + badge + '</td>' +
          '<td style="display:flex;gap:4px">' +
          (p.is_active
            ? ''
            : '<button class="btn btn-sm btn-primary wg-activate-btn" data-id="' + esc(p.id) + '" title="Activate">Activate</button>') +
          '<button class="btn btn-sm btn-ghost wg-test-btn" data-id="' + esc(p.id) + '" title="Test">Test</button>' +
          '<button class="btn btn-sm btn-ghost wg-edit-btn" data-id="' + esc(p.id) + '" data-name="' + esc(p.name) + '" data-endpoint="' + esc(p.endpoint) + '" data-pubkey="' + esc(p.public_key) + '" data-address="' + esc(p.address) + '" data-allowedips="' + esc(p.allowed_ips) + '" data-dns="' + esc(p.dns || '') + '" title="Edit">Edit</button>' +
          '<button class="btn btn-sm btn-danger wg-delete-btn" data-id="' + esc(p.id) + '" data-name="' + esc(p.name) + '" title="Delete">' + icons.trash + '</button>' +
          '</td></tr>';
      }
      html += '</tbody></table>';
      container.innerHTML = html;

      container.querySelectorAll('.wg-activate-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          this.textContent = 'Connecting...';
          this.disabled = true;
          try {
            var r = await api.post('/api/wireguard/profiles/' + id + '/activate', {});
            if (r.ok) {
              toast('WireGuard tunnel activated');
              renderWireGuard(el);
            } else {
              var data = await r.json().catch(function() { return {}; });
              toast(data.error || 'Failed to activate', 'error');
              this.textContent = 'Activate';
              this.disabled = false;
            }
          } catch (err) {
            toast('Failed to activate', 'error');
            this.textContent = 'Activate';
            this.disabled = false;
          }
        });
      });

      container.querySelectorAll('.wg-test-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var origText = this.textContent;
          this.textContent = 'Testing...';
          this.disabled = true;
          try {
            var r = await api.post('/api/wireguard/profiles/' + id + '/test', {});
            var result = await r.json();
            if (result.success) {
              toast('Connection OK (' + Math.round(result.latency_ms) + 'ms)');
            } else {
              toast('Test failed: ' + (result.error || 'unknown error'), 'error');
            }
          } catch (err) {
            toast('Test failed', 'error');
          }
          this.textContent = origText;
          this.disabled = false;
        });
      });

      container.querySelectorAll('.wg-edit-btn').forEach(function(btn) {
        btn.addEventListener('click', function() {
          wgEditId = this.getAttribute('data-id');
          openWGModal('Edit WireGuard Profile', 'Update');
          document.getElementById('wg-name').value = this.getAttribute('data-name') || '';
          document.getElementById('wg-privkey').value = '';
          document.getElementById('wg-endpoint').value = this.getAttribute('data-endpoint') || '';
          document.getElementById('wg-pubkey').value = this.getAttribute('data-pubkey') || '';
          document.getElementById('wg-address').value = this.getAttribute('data-address') || '';
          document.getElementById('wg-allowedips').value = this.getAttribute('data-allowedips') || '0.0.0.0/0';
          document.getElementById('wg-dns').value = this.getAttribute('data-dns') || '';
        });
      });

      container.querySelectorAll('.wg-delete-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var name = this.getAttribute('data-name');
          if (!confirm('Delete WireGuard profile "' + name + '"?')) return;
          try {
            var r = await api.del('/api/wireguard/profiles/' + id);
            if (r.ok || r.status === 204) {
              toast('Profile deleted');
              renderWireGuard(el);
            } else {
              toast('Failed to delete profile', 'error');
            }
          } catch (err) {
            toast('Failed to delete profile', 'error');
          }
        });
      });
    } catch (e) {
      var wgList = document.getElementById('wg-list');
      if (wgList) wgList.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load profiles</p></div>';
    }
  }

  async function renderEPGSources(el) {
    el.innerHTML = '<h1 class="page-title">EPG Sources</h1>' +
      '<div style="margin-bottom:16px"><button class="btn btn-primary" id="add-epg-btn">' + icons.plus + ' Add EPG Source</button></div>' +
      '<div id="epg-list"><div class="skeleton" style="height:200px"></div></div>';

    var epgEditId = null;
    var epgFormBody =
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="epg-name" placeholder="UK XMLTV"></div>' +
      '<div class="form-group"><label class="form-label">XMLTV URL</label><input class="form-input" id="epg-url" placeholder="http://example.com/guide.xml"></div>' +
      '<div class="form-group"><label class="form-label">Auto Refresh</label><select class="form-input" id="epg-refresh"><option value="none">None (manual only)</option><option value="hourly">Hourly</option><option value="daily" selected>Daily</option><option value="weekly">Weekly</option></select></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="epg-wireguard"> Route through WireGuard</label></div>';

    function openEPGModal(title, saveLabel) {
      var modal = showFormModal(title, epgFormBody, { id: 'epg-modal', saveLabel: saveLabel });
      modal.querySelector('.modal-save-btn').addEventListener('click', async function() {
        var name = document.getElementById('epg-name').value.trim();
        var url = document.getElementById('epg-url').value.trim();
        var refreshInterval = document.getElementById('epg-refresh').value;
        var wg = document.getElementById('epg-wireguard').checked;
        if (!name || !url) { toast('Name and URL required', 'error'); return; }
        try {
          var r;
          if (epgEditId) {
            r = await api.put('/api/epg/sources/' + epgEditId, { name: name, url: url, refresh_interval: refreshInterval, use_wireguard: wg });
          } else {
            r = await api.post('/api/epg/sources', { name: name, url: url, refresh_interval: refreshInterval, use_wireguard: wg });
          }
          if (r.ok) {
            toast(epgEditId ? 'EPG source updated' : 'EPG source created');
            modal.remove();
            renderEPGSources(el);
          } else {
            var data = await r.json().catch(function() { return {}; });
            toast(data.error || 'Failed to save EPG source', 'error');
          }
        } catch (err) {
          toast('Failed to save EPG source', 'error');
        }
      });
      return modal;
    }

    document.getElementById('add-epg-btn').addEventListener('click', function() {
      epgEditId = null;
      openEPGModal('New EPG Source', 'Create');
    });

    try {
      var resp = await api.get('/api/epg/sources');
      var sources = await resp.json();
      if (!Array.isArray(sources)) sources = [];
      var container = document.getElementById('epg-list');
      if (!container) return;

      if (sources.length === 0) {
        container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No EPG sources configured</p></div>';
        return;
      }

      var epgSourceConfigs = {};
      var html = '<table class="list-table"><thead><tr>' +
        '<th>Name</th><th>Channels</th><th>Programs</th><th>Last Refreshed</th><th>Status</th><th></th>' +
        '</tr></thead><tbody>';
      for (var i = 0; i < sources.length; i++) {
        var s = sources[i];
        var statusBadge = s.is_enabled ? '<span class="badge badge-enabled">ON</span>' : '<span class="badge badge-disabled">OFF</span>';
        if (s.last_error) {
          statusBadge = '<span class="badge badge-live" title="' + esc(s.last_error) + '">ERROR</span>';
        }
        var lastRefreshed = s.last_refreshed ? new Date(s.last_refreshed).toLocaleString() : 'Never';
        epgSourceConfigs[s.id] = { name: s.name, url: s.url, refresh_interval: s.refresh_interval, use_wireguard: s.use_wireguard };
        html += '<tr>' +
          '<td><a href="#" class="epg-source-name" data-id="' + esc(s.id) + '" data-name="' + esc(s.name) + '" style="color:var(--accent);cursor:pointer">' + esc(s.name) + '</a></td>' +
          '<td>' + (s.channel_count || 0) + '</td>' +
          '<td>' + (s.program_count || 0) + '</td>' +
          '<td>' + esc(lastRefreshed) + '</td>' +
          '<td>' + statusBadge + '</td>' +
          '<td style="display:flex;gap:4px">' +
          '<button class="btn-icon epg-edit-btn" data-id="' + esc(s.id) + '" title="Edit">' + icons.edit + '</button>' +
          '<button class="btn-icon epg-refresh-btn" data-id="' + esc(s.id) + '" title="Refresh">' + icons.refresh + '</button>' +
          '<button class="btn-icon epg-delete-btn" data-id="' + esc(s.id) + '" data-name="' + esc(s.name) + '" title="Delete" style="color:var(--danger)">' + icons.trash + '</button>' +
          '</td></tr>';
      }
      html += '</tbody></table>';
      container.innerHTML = html;

      container.querySelectorAll('.epg-source-name').forEach(function(link) {
        link.addEventListener('click', async function(e) {
          e.preventDefault();
          var id = this.getAttribute('data-id');
          var name = this.getAttribute('data-name');
          var detailId = 'epg-detail-' + id;
          var existing = document.getElementById(detailId);
          if (existing) { existing.remove(); return; }
          var row = this.closest('tr');
          var detailRow = document.createElement('tr');
          detailRow.id = detailId;
          var td = document.createElement('td');
          td.colSpan = 6;
          td.innerHTML = '<div style="padding:12px"><div class="spinner-ring" style="margin:20px auto"></div></div>';
          detailRow.appendChild(td);
          row.after(detailRow);
          try {
            var gdResp = await api.get('/api/epg/programs?source_id=' + encodeURIComponent(id));
            var currentPrograms = await gdResp.json();
            if (!Array.isArray(currentPrograms)) currentPrograms = [];

            var chMetaResp = await api.get('/api/epg/channel-ids');
            var chMetaList = await chMetaResp.json();
            var chMeta = {};
            if (Array.isArray(chMetaList)) {
              for (var mi = 0; mi < chMetaList.length; mi++) {
                chMeta[chMetaList[mi].id] = chMetaList[mi];
              }
            }

            function fmtTime(d) {
              var dt = new Date(d);
              var hh = dt.getHours(); var mm = dt.getMinutes();
              return (hh < 10 ? '0' : '') + hh + ':' + (mm < 10 ? '0' : '') + mm;
            }

            var byChannel = {};
            for (var pi = 0; pi < currentPrograms.length; pi++) {
              var p = currentPrograms[pi];
              byChannel[p.channel_id] = p;
            }
            var channelIDs = Object.keys(byChannel).sort();

            if (channelIDs.length === 0) {
              td.innerHTML = '<div style="padding:12px;color:var(--text-muted)">' + esc(name) + ' — No current programs</div>';
            } else {
              var tableHtml = '<div style="margin:8px 0">' +
                '<div style="font-weight:600;margin-bottom:8px;padding:0 8px">' + esc(name) + ' — ' + channelIDs.length + ' channels with current programs</div>' +
                '<div style="max-height:400px;overflow-y:auto"><table class="list-table" style="width:100%"><thead><tr>' +
                '<th style="width:40px"></th><th>Channel</th><th>Current Program</th><th>Time</th>' +
                '</tr></thead><tbody>';
              for (var k = 0; k < channelIDs.length; k++) {
                var chId = channelIDs[k];
                var prog = byChannel[chId];
                var meta = chMeta[chId] || {};
                var iconUrl = meta.icon || '';
                var logoImg = iconUrl
                  ? '<img src="/logo?url=' + encodeURIComponent(iconUrl) + '" style="width:28px;height:28px;object-fit:contain;border-radius:4px;background:var(--bg-hover)" loading="lazy" alt="">'
                  : '<div style="width:28px;height:28px;border-radius:4px;background:var(--bg-hover)"></div>';
                var displayName = meta.name || chId;
                var timeStr = fmtTime(prog.start_time) + ' - ' + fmtTime(prog.end_time);
                tableHtml += '<tr><td style="width:40px;text-align:center">' + logoImg + '</td>' +
                  '<td style="white-space:nowrap" title="' + esc(chId) + '">' + esc(displayName) + '</td>' +
                  '<td>' + esc(prog.title) + '</td>' +
                  '<td style="white-space:nowrap">' + timeStr + '</td></tr>';
              }
              tableHtml += '</tbody></table></div></div>';
              td.innerHTML = tableHtml;
            }
          } catch(err) {
            td.innerHTML = '<div style="padding:12px;color:var(--danger)">Failed to load EPG data</div>';
          }
        });
      });

      container.querySelectorAll('.epg-refresh-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          try {
            var r = await api.post('/api/epg/sources/' + id + '/refresh', {});
            if (r.ok || r.status === 202) {
              toast('EPG refresh started');
            } else {
              toast('Failed to refresh EPG', 'error');
            }
          } catch (err) {
            toast('Failed to refresh EPG', 'error');
          }
        });
      });

      container.querySelectorAll('.epg-edit-btn').forEach(function(btn) {
        btn.addEventListener('click', function() {
          var id = this.getAttribute('data-id');
          var cfg = epgSourceConfigs[id] || {};
          epgEditId = id;
          openEPGModal('Edit EPG Source', 'Update');
          document.getElementById('epg-name').value = cfg.name || '';
          document.getElementById('epg-url').value = cfg.url || '';
          document.getElementById('epg-refresh').value = cfg.refresh_interval || 'daily';
          document.getElementById('epg-wireguard').checked = !!cfg.use_wireguard;
        });
      });

      container.querySelectorAll('.epg-delete-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var name = this.getAttribute('data-name');
          if (!confirm('Delete EPG source "' + name + '"? All its program data will be removed.')) return;
          try {
            var r = await api.del('/api/epg/sources/' + id);
            if (r.ok || r.status === 204) {
              toast('EPG source deleted');
              renderEPGSources(el);
            } else {
              toast('Failed to delete EPG source', 'error');
            }
          } catch (err) {
            toast('Failed to delete EPG source', 'error');
          }
        });
      });
    } catch (e) {
      var epgList = document.getElementById('epg-list');
      if (epgList) epgList.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load EPG sources</p></div>';
    }
  }

  async function renderActivity(el) {
    el.innerHTML = '<div id="activity-root"><div style="text-align:center;padding:48px;color:var(--text-muted)">Loading activity...</div></div>';

    var pollTimer = null;
    var durationTimer = null;

    function fmtDuration(startedAt) {
      var secs = Math.floor((Date.now() - new Date(startedAt).getTime()) / 1000);
      if (secs < 0) secs = 0;
      var hrs = Math.floor(secs / 3600);
      var mins = Math.floor((secs % 3600) / 60);
      var s = secs % 60;
      return hrs > 0 ? hrs + ':' + String(mins).padStart(2, '0') + ':' + String(s).padStart(2, '0')
                     : mins + ':' + String(s).padStart(2, '0');
    }

    function deliveryBadgeStyle(delivery) {
      if (delivery === 'hls') return 'background:var(--accent);color:#fff;';
      if (delivery === 'mse') return 'background:#6366f1;color:#fff;';
      if (delivery === 'stream') return 'background:#0ea5e9;color:#fff;';
      return 'background:var(--bg-hover);color:var(--text);';
    }

    function fmtIdleSecs(secs) {
      if (secs < 60) return 'Just now';
      var mins = Math.floor(secs / 60);
      if (mins < 2) return '1 min ago';
      return mins + ' min ago';
    }

    function renderViewers(data) {
      var root = document.getElementById('activity-root');
      if (!root) return;
      root.innerHTML = '';

      var viewers = data.viewers || [];
      var recentUsers = data.recent_users || [];
      var sessionCount = data.session_count || 0;
      var streamCount = viewers.length;
      var userCount = recentUsers.length;

      var statsHtml = '<div class="stat-grid" style="margin-bottom:20px">' +
        '<div class="stat-card"><div class="stat-value">' + streamCount + '</div><div class="stat-label">Active Streams</div></div>' +
        '<div class="stat-card"><div class="stat-value">' + sessionCount + '</div><div class="stat-label">Sessions</div></div>' +
        '<div class="stat-card"><div class="stat-value">' + userCount + '</div><div class="stat-label">Users Online</div></div>' +
        '</div>';
      root.innerHTML = statsHtml;

      if (viewers.length === 0 && recentUsers.length === 0) {
        root.innerHTML += '<div class="empty-state"><p>No active sessions</p></div>';
        return;
      }

      if (viewers.length > 0) {
        root.innerHTML += '<h3 style="margin:0 0 12px 0;font-size:1em;color:var(--text-dim)">Active Streams</h3>';
        var grid = '';
        for (var i = 0; i < viewers.length; i++) {
          var v = viewers[i];
          var displayName = v.stream_name || v.channel_name || 'Unknown Stream';

          grid += '<div style="background:var(--bg-card);border:1px solid var(--border);border-radius:8px;padding:16px;display:flex;flex-direction:column;gap:8px">';

          grid += '<div style="display:flex;align-items:center;justify-content:space-between;gap:8px">';
          grid += '<span style="font-size:1.05em;font-weight:600;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;flex:1" title="' + esc(displayName) + '">' + esc(displayName) + '</span>';
          grid += '<span style="display:inline-flex;align-items:center;gap:4px;font-size:0.85em;white-space:nowrap;flex-shrink:0">' +
            '<span style="width:8px;height:8px;border-radius:50%;background:var(--success);flex-shrink:0"></span>Playing</span>';
          grid += '</div>';

          grid += '<div style="display:flex;flex-wrap:wrap;gap:4px;align-items:center">';
          if (v.username) {
            grid += '<span class="badge" style="background:rgba(91,110,239,.1);color:var(--accent);font-size:0.8em;padding:2px 8px;border-radius:4px">' + esc(v.username) + '</span>';
          }
          if (v.delivery) {
            grid += '<span class="badge" style="' + deliveryBadgeStyle(v.delivery) + 'font-size:0.8em;padding:2px 8px;border-radius:4px">' + esc(v.delivery.toUpperCase()) + '</span>';
          }
          if (v.client_name) {
            grid += '<span class="badge" style="background:var(--bg-hover);color:var(--text);font-size:0.8em;padding:2px 8px;border-radius:4px">' + esc(v.client_name) + '</span>';
          }
          if (v.transcoding) {
            grid += '<span class="badge" style="background:rgba(251,191,36,.15);color:var(--warning);font-size:0.8em;padding:2px 8px;border-radius:4px">Transcode</span>';
          } else if (v.video_codec) {
            grid += '<span class="badge" style="background:rgba(52,211,153,.15);color:var(--success);font-size:0.8em;padding:2px 8px;border-radius:4px">Copy</span>';
          }
          grid += '</div>';

          if (v.video_codec || v.audio_codec || v.resolution) {
            grid += '<div style="display:flex;flex-wrap:wrap;gap:4px">';
            if (v.video_codec) {
              grid += '<span class="badge badge-delivery" style="font-size:0.75em;padding:1px 6px">' + esc(v.video_codec.toUpperCase()) + '</span>';
            }
            if (v.audio_codec) {
              grid += '<span class="badge badge-delivery" style="font-size:0.75em;padding:1px 6px">' + esc(v.audio_codec.toUpperCase()) + '</span>';
            }
            if (v.resolution) {
              grid += '<span class="badge" style="background:rgba(52,211,153,.1);color:var(--success);font-size:0.75em;padding:1px 6px;border-radius:4px">' + esc(v.resolution) + '</span>';
            }
            grid += '</div>';
          }

          grid += '<div style="display:grid;grid-template-columns:auto 1fr;gap:2px 12px;font-size:0.85em;color:var(--text-muted)">';
          grid += '<span style="font-weight:500">Duration</span>';
          grid += '<span class="activity-duration" data-started="' + esc(v.started_at) + '">' + fmtDuration(v.started_at) + '</span>';
          grid += '<span style="font-weight:500">IP</span>';
          grid += '<span>' + esc(v.remote_addr || '-') + '</span>';
          if (v.channel_name && v.stream_name && v.channel_name !== v.stream_name) {
            grid += '<span style="font-weight:500">Channel</span>';
            grid += '<span>' + esc(v.channel_name) + '</span>';
          }
          grid += '</div>';

          grid += '</div>';
        }

        root.innerHTML += '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(min(340px,100%),1fr));gap:12px;margin-bottom:24px">' + grid + '</div>';
      }

      if (recentUsers.length > 0) {
        root.innerHTML += '<h3 style="margin:0 0 12px 0;font-size:1em;color:var(--text-dim)">Recent Users (20 min window)</h3>';
        var userHtml = '<table class="list-table"><thead><tr><th>User</th><th>Source</th><th>IP</th><th>Last Active</th></tr></thead><tbody>';
        for (var j = 0; j < recentUsers.length; j++) {
          var u = recentUsers[j];
          userHtml += '<tr><td style="font-weight:600">' + esc(u.username) + '</td>';
          userHtml += '<td><span class="badge" style="background:var(--bg-hover);color:var(--text);font-size:0.8em;padding:2px 8px;border-radius:4px">' + esc(u.source) + '</span></td>';
          userHtml += '<td style="color:var(--text-muted)">' + esc(u.remote_addr || '-') + '</td>';
          userHtml += '<td style="color:var(--text-muted)">' + fmtIdleSecs(u.idle_secs || 0) + '</td></tr>';
        }
        userHtml += '</tbody></table>';
        root.innerHTML += '<div class="card">' + userHtml + '</div>';
      }
    }

    function updateDurations() {
      var spans = document.querySelectorAll('.activity-duration');
      for (var i = 0; i < spans.length; i++) {
        var started = spans[i].getAttribute('data-started');
        if (started) spans[i].textContent = fmtDuration(started);
      }
    }

    async function fetchAndRender() {
      try {
        var resp = await api.get('/api/activity');
        if (!resp.ok) throw new Error('HTTP ' + resp.status);
        var data = await resp.json();
        renderViewers(data);
      } catch (e) {
        var root = document.getElementById('activity-root');
        if (root) root.innerHTML = '<div style="text-align:center;padding:48px;color:var(--danger)">Failed to load activity: ' + esc(e.message) + '</div>';
      }
    }

    await fetchAndRender();

    durationTimer = setInterval(updateDurations, 1000);

    pollTimer = setInterval(function() {
      if (router.current !== 'activity') {
        clearInterval(pollTimer);
        clearInterval(durationTimer);
        pollTimer = null;
        durationTimer = null;
        return;
      }
      fetchAndRender();
    }, 5000);
  }

  var LibData = (function() {
    function cacheKey(prefix, sourceID) { return 'mh_' + prefix + '_' + (sourceID || 'all'); }
    function loadCache(key) { try { var r = localStorage.getItem(key); return r ? JSON.parse(r) : null; } catch(e) { return null; } }
    function saveCache(key, data) { try { localStorage.setItem(key, JSON.stringify(data)); } catch(e) {} }

    async function getCategories(sourceID, vodType) {
      var ck = cacheKey('cats_' + vodType, sourceID);
      var cached = loadCache(ck);
      if (cached) {
        api.get('/api/vod/categories?type=' + vodType + '&source_id=' + encodeURIComponent(sourceID)).then(function(r) { return r.json(); }).then(function(fresh) { saveCache(ck, fresh); }).catch(function(){});
        return cached;
      }
      var resp = await api.get('/api/vod/categories?type=' + vodType + '&source_id=' + encodeURIComponent(sourceID));
      var data = await resp.json();
      if (!Array.isArray(data)) data = [];
      saveCache(ck, data);
      return data;
    }

    async function getItems(sourceID, vodType, group) {
      var sfx = '&source_id=' + encodeURIComponent(sourceID);
      if (group) sfx += '&group=' + encodeURIComponent(group);
      var ck = cacheKey(vodType + (group ? '_' + group : ''), sourceID);
      var cached = loadCache(ck);
      if (cached && cached.items) {
        api.get('/api/vod/library?type=' + vodType + '&fields=slim' + sfx).then(function(r) { return r.json(); }).then(function(fresh) { if (fresh && fresh.items) saveCache(ck, fresh); }).catch(function(){});
        return cached;
      }
      var resp = await api.get('/api/vod/library?type=' + vodType + '&fields=slim' + sfx);
      var data = await resp.json();
      if (!data || !data.items) data = { items: [], genres: [], decades: [], certifications: [], tags: [] };
      saveCache(ck, data);
      return data;
    }

    return { getCategories: getCategories, getItems: getItems };
  })();

  var LibProcessor = (function() {
    function addPosterUrls(items) {
      for (var i = 0; i < items.length; i++) {
        if (items[i].tmdb_id && !items[i].poster_url) {
          items[i].poster_url = '/api/tmdb/i/' + items[i].tmdb_id + '/poster.jpg';
        }
      }
      return items;
    }

    function groupMovies(items) {
      var collections = {};
      var movies = [];
      var genericGroups = { movies:1, films:1, vod:1, video:1, movie:1, film:1, all:1, uncategorized:1 };
      for (var i = 0; i < items.length; i++) {
        var m = items[i];
        var g = m.group || '';
        if (g && !genericGroups[g.toLowerCase()]) {
          var count = 0;
          for (var j = 0; j < items.length; j++) { if (items[j].group === g) count++; }
          if (count > 1) {
            if (!collections[g]) collections[g] = { name: g, movies: [], poster_url: m.poster_url || '' };
            collections[g].movies.push(m);
            if (m.poster_url && !collections[g].poster_url) collections[g].poster_url = m.poster_url;
            continue;
          }
        }
        movies.push(m);
      }
      var colList = [];
      for (var k in collections) colList.push(collections[k]);
      return { movies: movies, collections: colList };
    }

    function groupSeries(items) {
      var map = {};
      for (var i = 0; i < items.length; i++) {
        var item = items[i];
        var key = item.series || item.name;
        if (!map[key]) {
          map[key] = { name: key, episodes: [], seasons: {}, poster_url: '', year: item.year || '' };
        }
        map[key].episodes.push(item);
        if (!map[key].poster_url && item.poster_url) map[key].poster_url = item.poster_url;
        var sk = item.season > 0 ? item.season : null;
        if (sk !== null) {
          if (!map[key].seasons[sk]) map[key].seasons[sk] = [];
          map[key].seasons[sk].push(item);
        }
      }
      var list = [];
      for (var k in map) {
        var show = map[k];
        if (Object.keys(show.seasons).length === 0 && show.episodes.length > 0) show.seasons[1] = show.episodes;
        list.push(show);
      }
      list.sort(function(a, b) { return a.name.localeCompare(b.name); });
      return list;
    }

    return { addPosterUrls: addPosterUrls, groupMovies: groupMovies, groupSeries: groupSeries };
  })();

  var LibGrid = (function() {
    function renderPosterGrid(container, items, onItemClick) {
      if (!items || items.length === 0) {
        container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No items found</p></div>';
        return;
      }
      var html = '<div class="poster-grid">';
      for (var i = 0; i < items.length; i++) {
        var item = items[i];
        html += '<div class="poster-card" data-idx="' + i + '">';
        if (item.poster_url) {
          html += '<img class="poster-img" src="' + esc(item.poster_url) + '" loading="lazy" alt="" onerror="this.style.display=\'none\';this.nextElementSibling.style.display=\'flex\'">';
          html += '<div class="poster-placeholder" style="display:none">' + esc(item.name) + '</div>';
        } else {
          html += '<div class="poster-placeholder">' + esc(item.name) + '</div>';
        }
        html += '<div class="poster-info"><div class="poster-title">' + esc(item.name) + '</div>';
        if (item.year) html += '<div class="poster-meta"><span class="poster-year">' + esc(item.year) + '</span></div>';
        if (item.badge) html += '<div class="poster-badge">' + esc(item.badge) + '</div>';
        html += '</div></div>';
      }
      html += '</div>';
      container.innerHTML = html;
      container.querySelectorAll('.poster-card[data-idx]').forEach(function(card) {
        card.onmouseenter = function() { card.style.transform = 'scale(1.03)'; card.style.boxShadow = '0 8px 30px rgba(0,0,0,0.3)'; };
        card.onmouseleave = function() { card.style.transform = ''; card.style.boxShadow = ''; };
        card.onclick = function() { onItemClick(items[parseInt(card.dataset.idx, 10)]); };
      });
    }

    function renderCategoryList(container, categories, onCategoryClick) {
      if (!categories || categories.length === 0) {
        container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No categories found</p></div>';
        return;
      }
      categories.sort(function(a, b) { return a.name.localeCompare(b.name); });
      var html = '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(250px,1fr));gap:8px;">';
      for (var i = 0; i < categories.length; i++) {
        var cat = categories[i];
        html += '<div class="cat-item" data-idx="' + i + '" style="padding:12px 16px;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius);cursor:pointer;display:flex;justify-content:space-between;align-items:center;transition:background 0.15s;">';
        html += '<span style="font-size:14px;font-weight:500;color:var(--text-primary)">' + esc(cat.name) + '</span>';
        html += '<span style="font-size:12px;color:var(--text-muted);background:rgba(255,255,255,0.08);padding:2px 8px;border-radius:10px;">' + cat.count + '</span>';
        html += '</div>';
      }
      html += '</div>';
      container.innerHTML = html;
      container.querySelectorAll('.cat-item').forEach(function(el) {
        el.onmouseenter = function() { el.style.background = 'rgba(255,255,255,0.05)'; };
        el.onmouseleave = function() { el.style.background = 'var(--bg-card)'; };
        el.onclick = function() { onCategoryClick(categories[parseInt(el.dataset.idx, 10)]); };
      });
    }

    return { renderPosterGrid: renderPosterGrid, renderCategoryList: renderCategoryList };
  })();

  var LibFilter = (function() {
    var pillBase = 'display:inline-block;padding:4px 12px;border-radius:16px;cursor:pointer;font-size:11px;font-weight:500;transition:all 0.15s;user-select:none;white-space:nowrap;';
    var pillStyles = {
      genre:   { off: pillBase + 'border:1px solid rgba(59,130,246,0.3);background:rgba(59,130,246,0.08);color:#3b82f6;', on: pillBase + 'border:1px solid #3b82f6;background:#3b82f6;color:#fff;' },
      decade:  { off: pillBase + 'border:1px solid rgba(34,197,94,0.3);background:rgba(34,197,94,0.08);color:#22c55e;', on: pillBase + 'border:1px solid #22c55e;background:#22c55e;color:#fff;' },
      cert:    { off: pillBase + 'border:1px solid rgba(249,115,22,0.3);background:rgba(249,115,22,0.08);color:#f97316;', on: pillBase + 'border:1px solid #f97316;background:#f97316;color:#fff;' },
      tag:     { off: pillBase + 'border:1px solid rgba(168,85,247,0.3);background:rgba(168,85,247,0.08);color:#a855f7;', on: pillBase + 'border:1px solid #a855f7;background:#a855f7;color:#fff;' },
      special: { off: pillBase + 'border:1px solid rgba(234,179,8,0.3);background:rgba(234,179,8,0.08);color:#eab308;', on: pillBase + 'border:1px solid #eab308;background:#eab308;color:#000;' },
      clear:   { off: pillBase + 'border:1px solid rgba(255,255,255,0.15);background:rgba(255,255,255,0.05);color:var(--text-muted);', on: pillBase + 'border:1px solid rgba(255,255,255,0.15);background:rgba(255,255,255,0.05);color:var(--text-muted);' }
    };

    function makePill(label, group, isActive, onClick) {
      var styles = pillStyles[group] || pillStyles.genre;
      var btn = document.createElement('button');
      btn.textContent = label;
      btn.style.cssText = isActive ? styles.on : styles.off;
      btn.onmouseenter = function() { if (!isActive) btn.style.opacity = '0.8'; };
      btn.onmouseleave = function() { btn.style.opacity = '1'; };
      btn.onclick = onClick;
      return btn;
    }

    function makePillDropdown(label, options, group, activeKeys, onToggle) {
      var wrap = document.createElement('div');
      wrap.style.cssText = 'position:relative;display:inline-block;';
      var styles = pillStyles[group] || pillStyles.genre;
      var activeCount = 0;
      for (var i = 0; i < options.length; i++) { if (activeKeys[group + ':' + options[i]]) activeCount++; }
      var trigger = document.createElement('button');
      trigger.textContent = activeCount > 0 ? label + ' (' + activeCount + ') \u25BE' : label + ' \u25BE';
      trigger.style.cssText = activeCount > 0 ? styles.on : styles.off;
      var popover = document.createElement('div');
      popover.style.cssText = 'position:absolute;top:calc(100% + 4px);left:0;background:#1a1d23;border:1px solid var(--border);border-radius:12px;padding:8px;z-index:50;min-width:200px;max-width:400px;display:none;flex-wrap:wrap;gap:6px;box-shadow:0 8px 30px rgba(0,0,0,0.4);';
      options.forEach(function(opt) {
        var key = group + ':' + opt;
        var isOn = !!activeKeys[key];
        var pill = document.createElement('button');
        pill.textContent = opt;
        pill.style.cssText = isOn ? styles.on : styles.off;
        pill.onclick = function(e) {
          e.stopPropagation();
          onToggle(key, opt);
        };
        popover.appendChild(pill);
      });
      trigger.onclick = function(e) {
        e.stopPropagation();
        popover.style.display = popover.style.display === 'flex' ? 'none' : 'flex';
      };
      document.addEventListener('click', function() { popover.style.display = 'none'; });
      wrap.appendChild(trigger);
      wrap.appendChild(popover);
      return wrap;
    }

    function buildFilterBar(container, filterMeta, activeFilters, onFilterChange) {
      var bar = document.createElement('div');
      bar.style.cssText = 'display:flex;gap:6px;flex-wrap:wrap;align-items:center;margin-bottom:12px;';

      var searchInput = document.createElement('input');
      searchInput.type = 'text';
      searchInput.placeholder = 'Search...';
      searchInput.style.cssText = 'padding:5px 12px;border-radius:16px;border:1px solid var(--border);background:var(--bg-input,#1a1d23);color:var(--text-primary);font-size:12px;width:160px;outline:none;';
      if (activeFilters._search) searchInput.value = activeFilters._search;
      searchInput.oninput = function() {
        activeFilters._search = searchInput.value;
        onFilterChange();
      };
      bar.appendChild(searchInput);

      var hasAnyFilter = false;
      var genres = (filterMeta.genres || []);
      var decades = (filterMeta.decades || []);
      var certs = (filterMeta.certifications || []);
      var tags = (filterMeta.tags || []);

      if (decades.length > 0) {
        bar.appendChild(makeSeparator());
        decades.forEach(function(d) {
          var key = 'decade:' + d;
          bar.appendChild(makePill(d, 'decade', !!activeFilters[key], function() {
            if (activeFilters[key]) { delete activeFilters[key]; }
            else {
              Object.keys(activeFilters).forEach(function(k) { if (k.indexOf('decade:') === 0) delete activeFilters[k]; });
              activeFilters[key] = true;
            }
            onFilterChange();
          }));
        });
      }

      if (certs.length > 0) {
        bar.appendChild(makeSeparator());
        certs.forEach(function(c) {
          var key = 'cert:' + c;
          bar.appendChild(makePill(c, 'cert', !!activeFilters[key], function() {
            if (activeFilters[key]) { delete activeFilters[key]; }
            else { activeFilters[key] = true; }
            onFilterChange();
          }));
        });
      }

      if (genres.length > 0) {
        bar.appendChild(makeSeparator());
        if (genres.length <= 8) {
          genres.forEach(function(g) {
            var key = 'genre:' + g;
            bar.appendChild(makePill(g, 'genre', !!activeFilters[key], function() {
              if (activeFilters[key]) { delete activeFilters[key]; }
              else { activeFilters[key] = true; }
              onFilterChange();
            }));
          });
        } else {
          bar.appendChild(makePillDropdown('Genres', genres, 'genre', activeFilters, function(key) {
            if (activeFilters[key]) { delete activeFilters[key]; }
            else { activeFilters[key] = true; }
            onFilterChange();
          }));
        }
      }

      if (tags.length > 0) {
        bar.appendChild(makeSeparator());
        tags.forEach(function(t) {
          var key = 'tag:' + t;
          bar.appendChild(makePill(t, 'tag', !!activeFilters[key], function() {
            if (activeFilters[key]) { delete activeFilters[key]; }
            else { activeFilters[key] = true; }
            onFilterChange();
          }));
        });
      }

      bar.appendChild(makeSeparator());
      var collKey = 'special:collections';
      bar.appendChild(makePill('Collections', 'special', !!activeFilters[collKey], function() {
        if (activeFilters[collKey]) { delete activeFilters[collKey]; }
        else { activeFilters[collKey] = true; }
        onFilterChange();
      }));

      for (var fk in activeFilters) {
        if (fk !== '_search' && activeFilters[fk]) { hasAnyFilter = true; break; }
      }
      if (hasAnyFilter) {
        bar.appendChild(makePill('\u2715 Clear', 'clear', false, function() {
          var search = activeFilters._search;
          Object.keys(activeFilters).forEach(function(k) { delete activeFilters[k]; });
          if (search) activeFilters._search = search;
          onFilterChange();
        }));
      }

      container.appendChild(bar);
      return { searchInput: searchInput };
    }

    function makeSeparator() {
      var sep = document.createElement('span');
      sep.style.cssText = 'width:1px;height:18px;background:var(--border);align-self:center;flex-shrink:0;';
      return sep;
    }

    function matchItem(item, activeFilters) {
      var search = (activeFilters._search || '').trim().toLowerCase();
      if (search && item.name.toLowerCase().indexOf(search) === -1) return false;

      var genreKeys = [];
      var decadeKeys = [];
      var certKeys = [];
      var tagKeys = [];
      var wantCollections = false;

      for (var k in activeFilters) {
        if (!activeFilters[k] || k === '_search') continue;
        if (k.indexOf('genre:') === 0) genreKeys.push(k.substring(6));
        else if (k.indexOf('decade:') === 0) decadeKeys.push(k.substring(7));
        else if (k.indexOf('cert:') === 0) certKeys.push(k.substring(5));
        else if (k.indexOf('tag:') === 0) tagKeys.push(k.substring(4));
        else if (k === 'special:collections') wantCollections = true;
      }

      if (wantCollections && !item._isCollection) return false;

      if (decadeKeys.length > 0) {
        var yr = item.year || item._year || '';
        if (yr.length < 4) return false;
        var itemDecade = yr.substring(0, 3) + '0s';
        var decadeMatch = false;
        for (var di = 0; di < decadeKeys.length; di++) { if (decadeKeys[di] === itemDecade) { decadeMatch = true; break; } }
        if (!decadeMatch) return false;
      }

      if (certKeys.length > 0) {
        var itemCert = item._certification || '';
        var certMatch = false;
        for (var ci = 0; ci < certKeys.length; ci++) { if (certKeys[ci] === itemCert) { certMatch = true; break; } }
        if (!certMatch) return false;
      }

      if (genreKeys.length > 0) {
        var itemGenres = item._genres || [];
        var genreMatch = false;
        for (var gi = 0; gi < genreKeys.length; gi++) {
          for (var gj = 0; gj < itemGenres.length; gj++) {
            if (genreKeys[gi] === itemGenres[gj]) { genreMatch = true; break; }
          }
          if (genreMatch) break;
        }
        if (!genreMatch) return false;
      }

      if (tagKeys.length > 0) {
        var itemTags = item._tags || [];
        var tagMatch = false;
        for (var ti = 0; ti < tagKeys.length; ti++) {
          for (var tj = 0; tj < itemTags.length; tj++) {
            if (tagKeys[ti] === itemTags[tj]) { tagMatch = true; break; }
          }
          if (tagMatch) break;
        }
        if (!tagMatch) return false;
      }

      return true;
    }

    return { buildFilterBar: buildFilterBar, matchItem: matchItem };
  })();

  async function renderLibrary(el) {
    el.innerHTML = '<h1 class="page-title">Library</h1>' +
      '<div id="lib-bar" style="display:flex;gap:8px;align-items:center;margin-bottom:12px;flex-wrap:wrap"></div>' +
      '<div id="lib-filters"></div>' +
      '<div id="lib-count" style="font-size:12px;color:var(--text-muted);margin-bottom:8px;"></div>' +
      '<div id="lib-content"><div class="skeleton" style="height:400px"></div></div>';

    try {
      var sourcesResp = await api.get('/api/sources');
      var allSources = await sourcesResp.json();
      if (!Array.isArray(allSources)) allSources = [];
      var vodSources = allSources.filter(function(s) {
        return s.type === 'tvpstreams' || s.type === 'xtream';
      }).sort(function(a, b) {
        var order = { tvpstreams: 0, xtream: 1 };
        return (order[a.type] || 9) - (order[b.type] || 9) || a.name.localeCompare(b.name);
      });

      var content = document.getElementById('lib-content');
      if (!content) return;

      if (vodSources.length === 0) {
        content.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No VOD sources configured</p></div>';
        return;
      }

      var selectedSource = el._selectedSource || vodSources[0].id;
      var selectedTab = el._selectedTab || 'movies';
      var selectedGroup = el._selectedGroup || null;
      if (!el._filters) el._filters = {};

      var bar = document.getElementById('lib-bar');
      if (bar) {
        if (vodSources.length > 1) {
          var srcSel = document.createElement('select');
          srcSel.className = 'form-input';
          srcSel.style.cssText = 'width:auto;padding:6px 10px;font-size:13px;';
          vodSources.forEach(function(s) {
            var opt = document.createElement('option');
            opt.value = s.id;
            opt.textContent = s.name;
            srcSel.appendChild(opt);
          });
          srcSel.value = selectedSource;
          srcSel.onchange = function() { el._selectedSource = srcSel.value; el._selectedGroup = null; el._selectedTab = 'movies'; el._filters = {}; renderLibrary(el); };
          bar.appendChild(srcSel);
        }

        var moviesBtn = document.createElement('button');
        moviesBtn.className = 'btn ' + (selectedTab === 'movies' ? 'btn-primary' : 'btn-ghost');
        moviesBtn.textContent = 'Movies';
        moviesBtn.onclick = function() { el._selectedTab = 'movies'; el._selectedGroup = null; el._filters = {}; renderLibrary(el); };
        bar.appendChild(moviesBtn);

        var seriesBtn = document.createElement('button');
        seriesBtn.className = 'btn ' + (selectedTab === 'series' ? 'btn-primary' : 'btn-ghost');
        seriesBtn.textContent = 'TV Series';
        seriesBtn.onclick = function() { el._selectedTab = 'series'; el._selectedGroup = null; el._filters = {}; renderLibrary(el); };
        bar.appendChild(seriesBtn);

        if (selectedGroup) {
          var backBtn = document.createElement('button');
          backBtn.className = 'btn btn-ghost';
          backBtn.textContent = '\u2190 Back to categories';
          backBtn.onclick = function() { el._selectedGroup = null; el._filters = {}; renderLibrary(el); };
          bar.appendChild(backBtn);
          var groupLabel = document.createElement('span');
          groupLabel.style.cssText = 'font-size:14px;font-weight:600;color:var(--text-primary);';
          groupLabel.textContent = selectedGroup;
          bar.appendChild(groupLabel);
        }
      }

      var srcInfo = vodSources.find(function(s) { return s.id === selectedSource; });
      var isLargeSource = srcInfo && srcInfo.type === 'xtream';

      if (isLargeSource && !selectedGroup) {
        var cats = await LibData.getCategories(selectedSource, selectedTab === 'series' ? 'series' : 'movie');
        LibGrid.renderCategoryList(content, cats, function(cat) {
          el._selectedGroup = cat.name;
          renderLibrary(el);
        });
      } else {
        var libData = await LibData.getItems(selectedSource, selectedTab === 'series' ? 'series' : 'movie', selectedGroup);
        var items = libData.items || [];
        items = LibProcessor.addPosterUrls(items);

        var filterMeta = {
          genres: libData.genres || [],
          decades: libData.decades || [],
          certifications: libData.certifications || [],
          tags: libData.tags || []
        };

        var displayItems;
        var onItemClick;

        if (selectedTab === 'series') {
          var seriesList = LibProcessor.groupSeries(items);
          displayItems = seriesList.map(function(show) {
            var showGenres = [];
            var showTags = [];
            for (var ei = 0; ei < show.episodes.length; ei++) {
              var ep = show.episodes[ei];
              if (ep.genres) { for (var gi = 0; gi < ep.genres.length; gi++) { if (showGenres.indexOf(ep.genres[gi]) === -1) showGenres.push(ep.genres[gi]); } }
              if (ep.tags) { for (var ti = 0; ti < ep.tags.length; ti++) { if (showTags.indexOf(ep.tags[ti]) === -1) showTags.push(ep.tags[ti]); } }
            }
            return { name: show.name, poster_url: show.poster_url, year: show.year, _show: show, _genres: showGenres, _tags: showTags, _certification: '', _isCollection: false };
          });
          onItemClick = function(item) { showSeriesModal(item._show); };
        } else {
          if (selectedGroup) {
            displayItems = items.map(function(m) {
              return { name: m.name, poster_url: m.poster_url, year: m.year, id: m.id, _movie: m, _genres: m.genres || [], _tags: m.tags || [], _certification: m.certification || '', _isCollection: false };
            });
            displayItems.sort(function(a, b) { return a.name.localeCompare(b.name); });
            onItemClick = function(item) { showMovieModal(item._movie); };
          } else {
            var grouped = LibProcessor.groupMovies(items);
            displayItems = [];
            grouped.collections.forEach(function(col) {
              var colGenres = [];
              var colTags = [];
              var colCert = '';
              for (var mi = 0; mi < col.movies.length; mi++) {
                var mov = col.movies[mi];
                if (mov.genres) { for (var gi = 0; gi < mov.genres.length; gi++) { if (colGenres.indexOf(mov.genres[gi]) === -1) colGenres.push(mov.genres[gi]); } }
                if (mov.tags) { for (var ti = 0; ti < mov.tags.length; ti++) { if (colTags.indexOf(mov.tags[ti]) === -1) colTags.push(mov.tags[ti]); } }
                if (mov.certification && !colCert) colCert = mov.certification;
              }
              displayItems.push({ name: col.name, poster_url: col.poster_url, badge: col.movies.length + ' films', _collection: col, _genres: colGenres, _tags: colTags, _certification: colCert, _isCollection: true, _year: col.movies[0] ? col.movies[0].year : '' });
            });
            grouped.movies.forEach(function(m) {
              displayItems.push({ name: m.name, poster_url: m.poster_url, year: m.year, id: m.id, _movie: m, _genres: m.genres || [], _tags: m.tags || [], _certification: m.certification || '', _isCollection: false });
            });
            displayItems.sort(function(a, b) { return a.name.localeCompare(b.name); });
            onItemClick = function(item) {
              if (item._collection) showCollectionModal(item._collection);
              else if (item._movie) showMovieModal(item._movie);
            };
          }
        }

        var filtersContainer = document.getElementById('lib-filters');
        var countEl = document.getElementById('lib-count');
        var activeFilters = el._filters;

        function renderFilteredGrid() {
          var filtered = displayItems.filter(function(item) {
            return LibFilter.matchItem(item, activeFilters);
          });
          if (countEl) {
            if (filtered.length !== displayItems.length) {
              countEl.textContent = filtered.length + ' of ' + displayItems.length + ' titles';
            } else {
              countEl.textContent = displayItems.length + ' titles';
            }
          }
          LibGrid.renderPosterGrid(content, filtered, onItemClick);
        }

        function onFilterChange() {
          if (filtersContainer) {
            filtersContainer.innerHTML = '';
            LibFilter.buildFilterBar(filtersContainer, filterMeta, activeFilters, onFilterChange);
          }
          renderFilteredGrid();
        }

        if (filtersContainer && (filterMeta.genres.length > 0 || filterMeta.decades.length > 0 || filterMeta.certifications.length > 0 || filterMeta.tags.length > 0 || displayItems.some(function(d) { return d._isCollection; }))) {
          LibFilter.buildFilterBar(filtersContainer, filterMeta, activeFilters, onFilterChange);
        }

        renderFilteredGrid();
      }

    } catch (e) {
      var ec = document.getElementById('lib-content');
      if (ec) ec.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load library</p></div>';
    }
  }


  function showTMDBMatchModal(streamID, streamName, currentTMDBID, mediaType, onMatched) {
    var existingModal = document.getElementById('tmdb-match-modal');
    if (existingModal) existingModal.remove();
    var isSeriesType = mediaType === 'series' || mediaType === 'tv' || mediaType === 'episode';
    var searchType = isSeriesType ? 'tv' : 'movie';
    var bodyHtml = '';
    if (currentTMDBID) {
      bodyHtml += '<div style="margin-bottom:16px;padding:12px;background:rgba(59,130,246,0.1);border:1px solid rgba(59,130,246,0.2);border-radius:8px;display:flex;align-items:center;justify-content:space-between"><div><span style="font-size:13px;color:var(--text-muted)">Current match:</span> <strong style="color:var(--text-primary)">TMDB #' + esc(String(currentTMDBID)) + '</strong></div><button class="btn btn-ghost" id="tmdb-clear-match" style="color:#ef4444;font-size:13px">Clear Match</button></div>';
    }
    bodyHtml += '<div class="form-group"><label class="form-label">Search TMDB</label><div style="display:flex;gap:8px"><input class="form-input" id="tmdb-search-input" placeholder="Search for ' + (isSeriesType ? 'TV show' : 'movie') + '..." value="' + esc(streamName) + '" style="flex:1"><select class="form-input" id="tmdb-search-type" style="width:auto;flex-shrink:0"><option value="movie"' + (searchType === 'movie' ? ' selected' : '') + '>Movie</option><option value="tv"' + (searchType === 'tv' ? ' selected' : '') + '>TV Show</option></select><button class="btn btn-primary" id="tmdb-search-btn" style="flex-shrink:0">Search</button></div></div><div id="tmdb-search-results" style="margin-top:12px;max-height:400px;overflow-y:auto"></div>';
    var tmdbModal = showFormModal(currentTMDBID ? 'Edit TMDB Match' : 'Match to TMDB', bodyHtml, { id: 'tmdb-match-modal', maxWidth: '600px', saveLabel: 'Close' });
    var tmdbSaveBtn = tmdbModal.querySelector('.modal-save-btn');
    if (tmdbSaveBtn) tmdbSaveBtn.style.display = 'none';
    var tmdbCancelBtn = tmdbModal.querySelector('.modal-cancel-btn');
    if (tmdbCancelBtn) tmdbCancelBtn.textContent = 'Close';
    var searchInput = document.getElementById('tmdb-search-input');
    var searchBtn = document.getElementById('tmdb-search-btn');
    var searchTypeSelect = document.getElementById('tmdb-search-type');
    var resultsContainer = document.getElementById('tmdb-search-results');
    function doTMDBSearch() {
      var q = searchInput.value.trim();
      if (!q) return;
      var stype = searchTypeSelect.value;
      resultsContainer.innerHTML = '<div style="text-align:center;padding:20px;color:var(--text-muted)">Searching...</div>';
      api.get('/api/tmdb/search?q=' + encodeURIComponent(q) + '&type=' + encodeURIComponent(stype)).then(function(resp) { return resp.json(); }).then(function(results) {
        if (!results || results.length === 0) { resultsContainer.innerHTML = '<div style="text-align:center;padding:20px;color:var(--text-muted)">No results found</div>'; return; }
        var html = '';
        for (var i = 0; i < results.length; i++) {
          var r = results[i];
          var posterHtml = r.poster_url ? '<img src="' + esc(r.poster_url) + '" style="width:48px;height:72px;object-fit:cover;border-radius:6px;flex-shrink:0;background:#2d3748" loading="lazy">' : '<div style="width:48px;height:72px;background:#2d3748;border-radius:6px;flex-shrink:0;display:flex;align-items:center;justify-content:center;color:var(--text-muted);font-size:10px">No img</div>';
          var isSelected = String(r.id) === String(currentTMDBID);
          var bdrStyle = isSelected ? 'border:2px solid #3b82f6;' : 'border:1px solid var(--border);';
          var ratingHtml = r.vote_average > 0 ? '<span style="color:#eab308;font-size:12px">\u2605 ' + r.vote_average.toFixed(1) + '</span>' : '';
          html += '<div class="tmdb-result-item" data-tmdb-id="' + r.id + '" style="display:flex;gap:12px;padding:10px;border-radius:8px;' + bdrStyle + 'cursor:pointer;margin-bottom:8px;transition:background 0.15s">' + posterHtml + '<div style="flex:1;min-width:0"><div style="font-weight:600;color:var(--text-primary);font-size:14px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + esc(r.title || '') + '</div><div style="display:flex;gap:8px;align-items:center;margin-top:2px">' + (r.year ? '<span style="color:var(--text-muted);font-size:13px">' + esc(r.year) + '</span>' : '') + ratingHtml + '</div>' + (r.overview ? '<div style="color:var(--text-muted);font-size:12px;margin-top:4px;overflow:hidden;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical">' + esc(r.overview.substring(0, 150)) + '</div>' : '') + (isSelected ? '<div style="color:#3b82f6;font-size:11px;font-weight:600;margin-top:4px">CURRENT MATCH</div>' : '') + '</div></div>';
        }
        resultsContainer.innerHTML = html;
        var resultItems = resultsContainer.querySelectorAll('.tmdb-result-item');
        for (var j = 0; j < resultItems.length; j++) {
          (function(el) {
            el.addEventListener('mouseenter', function() { el.style.background = 'rgba(255,255,255,0.05)'; });
            el.addEventListener('mouseleave', function() { el.style.background = ''; });
            el.addEventListener('click', function() {
              var tmdbId = parseInt(el.dataset.tmdbId, 10);
              var selType = searchTypeSelect.value === 'tv' ? 'series' : 'movie';
              el.style.opacity = '0.5';
              api.post('/api/streams/' + encodeURIComponent(streamID) + '/tmdb-match', { tmdb_id: tmdbId, media_type: selType }).then(function(resp) { return resp.json(); }).then(function(data) {
                if (data.error) { toast(data.error, 'error'); el.style.opacity = '1'; return; }
                toast('TMDB match updated'); tmdbModal.remove(); if (onMatched) onMatched(tmdbId, selType);
              }).catch(function(err) { toast('Failed to update match: ' + err.message, 'error'); el.style.opacity = '1'; });
            });
          })(resultItems[j]);
        }
      }).catch(function(err) { resultsContainer.innerHTML = '<div style="text-align:center;padding:20px;color:#ef4444">Search failed: ' + esc(err.message) + '</div>'; });
    }
    searchBtn.addEventListener('click', doTMDBSearch);
    searchInput.addEventListener('keydown', function(e) { if (e.key === 'Enter') doTMDBSearch(); });
    var clearMatchBtn = document.getElementById('tmdb-clear-match');
    if (clearMatchBtn) {
      clearMatchBtn.addEventListener('click', function() {
        api.post('/api/streams/' + encodeURIComponent(streamID) + '/tmdb-match', { tmdb_id: 0 }).then(function(resp) { return resp.json(); }).then(function(data) {
          if (data.error) { toast(data.error, 'error'); return; }
          toast('TMDB match cleared'); tmdbModal.remove(); if (onMatched) onMatched(0, '');
        }).catch(function(err) { toast('Failed to clear match: ' + err.message, 'error'); });
      });
    }
    setTimeout(doTMDBSearch, 100);
  }

  function showMovieModal(item) {
    var overlay = document.createElement('div');
    overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.75);z-index:9999;display:flex;align-items:center;justify-content:center;backdrop-filter:blur(6px);';
    overlay.onclick = function(e) { if (e.target === overlay) overlay.remove(); };
    document.addEventListener('keydown', function onKey(e) { if (e.key === 'Escape') { overlay.remove(); document.removeEventListener('keydown', onKey); } });

    var modal = document.createElement('div');
    modal.style.cssText = 'width:90%;max-width:1080px;max-height:92vh;background:#1a1d23;border-radius:16px;overflow:hidden;position:relative;display:flex;flex-direction:column;box-shadow:0 24px 80px rgba(0,0,0,0.6);';

    var backdrop = document.createElement('div');
    backdrop.style.cssText = 'width:100%;height:360px;background:linear-gradient(135deg,#1a1a2e 0%,#16213e 50%,#0f3460 100%);position:relative;overflow:hidden;flex-shrink:0;transition:background-image 0.5s;';
    if (item.backdrop_url) {
      backdrop.style.backgroundImage = 'url(' + item.backdrop_url + ')';
      backdrop.style.backgroundSize = 'cover';
      backdrop.style.backgroundPosition = 'center 20%';
    }

    var closeBtn = document.createElement('button');
    closeBtn.textContent = '\u2715';
    closeBtn.style.cssText = 'position:absolute;top:16px;right:16px;background:rgba(0,0,0,0.6);border:none;color:#fff;font-size:18px;width:40px;height:40px;border-radius:50%;cursor:pointer;z-index:3;transition:background 0.2s;';
    closeBtn.onmouseenter = function() { closeBtn.style.background = 'rgba(255,255,255,0.2)'; };
    closeBtn.onmouseleave = function() { closeBtn.style.background = 'rgba(0,0,0,0.6)'; };
    closeBtn.onclick = function() { overlay.remove(); };
    backdrop.appendChild(closeBtn);

    backdrop.appendChild(Object.assign(document.createElement('div'), {
      style: 'position:absolute;bottom:0;left:0;right:0;height:200px;background:linear-gradient(transparent,#1a1d23);'
    }));

    var titleBlock = document.createElement('div');
    titleBlock.style.cssText = 'position:absolute;bottom:24px;left:32px;right:32px;z-index:1;';

    var titleEl = document.createElement('div');
    titleEl.style.cssText = 'font-size:32px;font-weight:800;color:#fff;text-shadow:0 2px 12px rgba(0,0,0,0.7);letter-spacing:-0.5px;';
    titleEl.textContent = item.name;
    titleBlock.appendChild(titleEl);

    var metaRow = document.createElement('div');
    metaRow.style.cssText = 'display:flex;align-items:center;justify-content:space-between;margin-top:8px;';

    var metaLine = document.createElement('div');
    metaLine.style.cssText = 'display:flex;align-items:center;gap:10px;font-size:14px;color:rgba(255,255,255,0.75);flex-wrap:wrap;';
    if (item.year) {
      var yearSpan = document.createElement('span');
      yearSpan.textContent = item.year;
      metaLine.appendChild(yearSpan);
    }
    if (item.rating > 0) {
      var rColor = item.rating >= 7 ? '#22c55e' : item.rating >= 5 ? '#eab308' : '#ef4444';
      var rSpan = document.createElement('span');
      rSpan.style.cssText = 'color:' + rColor + ';font-weight:700;';
      rSpan.textContent = '\u2605 ' + item.rating.toFixed(1);
      metaLine.appendChild(rSpan);
    }
    if (item.certification) {
      var certSpan = document.createElement('span');
      certSpan.style.cssText = 'border:1px solid rgba(255,255,255,0.3);padding:1px 6px;border-radius:4px;font-size:12px;font-weight:600;';
      certSpan.textContent = item.certification;
      metaLine.appendChild(certSpan);
    }
    if (item.genres && item.genres.length) {
      var gSpan = document.createElement('span');
      gSpan.textContent = item.genres.slice(0, 3).join(' \u2022 ');
      metaLine.appendChild(gSpan);
    }
    if (item.tags && item.tags.length) {
      for (var tgi = 0; tgi < item.tags.length; tgi++) {
        var tagSpan = document.createElement('span');
        tagSpan.style.cssText = 'background:rgba(234,179,8,0.2);color:#eab308;padding:2px 8px;border-radius:6px;font-size:11px;font-weight:600;';
        tagSpan.textContent = item.tags[tgi];
        metaLine.appendChild(tagSpan);
      }
    }
    metaRow.appendChild(metaLine);

    var actionIcons = document.createElement('div');
    actionIcons.style.cssText = 'display:flex;gap:8px;align-items:center;flex-shrink:0;';
    var iconBtnStyle = 'background:rgba(0,0,0,0.5);border:none;color:#fff;width:40px;height:40px;border-radius:50%;cursor:pointer;font-size:18px;display:flex;align-items:center;justify-content:center;transition:background 0.2s;';

    var playIcon = document.createElement('button');
    playIcon.style.cssText = iconBtnStyle;
    playIcon.title = 'Play';
    playIcon.textContent = '\u25B6';
    playIcon.onmouseenter = function() { playIcon.style.background = '#3b82f6'; };
    playIcon.onmouseleave = function() { playIcon.style.background = 'rgba(0,0,0,0.5)'; };
    playIcon.onclick = function() { overlay.remove(); startPlay(item.id, item.name, false); };
    actionIcons.appendChild(playIcon);

    var favIcon = document.createElement('button');
    favIcon.style.cssText = iconBtnStyle;
    favIcon.title = 'Favorite';
    favIcon.textContent = streamFavorites[item.id] ? '\u2B50' : '\u2606';
    favIcon.onmouseenter = function() { favIcon.style.background = '#eab308'; };
    favIcon.onmouseleave = function() { favIcon.style.background = 'rgba(0,0,0,0.5)'; };
    favIcon.onclick = function() {
      toggleFavorite(item.id).then(function() {
        favIcon.textContent = streamFavorites[item.id] ? '\u2B50' : '\u2606';
      }).catch(function() { toast('Failed to update favorite', 'error'); });
    };
    actionIcons.appendChild(favIcon);
    if (api.user && api.user.is_admin) {
      var tmdbMatchIcon = document.createElement('button');
      tmdbMatchIcon.style.cssText = iconBtnStyle;
      tmdbMatchIcon.title = item.tmdb_id ? 'Edit TMDB Match' : 'Match to TMDB';
      tmdbMatchIcon.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>';
      tmdbMatchIcon.onmouseenter = function() { tmdbMatchIcon.style.background = '#10b981'; };
      tmdbMatchIcon.onmouseleave = function() { tmdbMatchIcon.style.background = 'rgba(0,0,0,0.5)'; };
      tmdbMatchIcon.onclick = function() {
        var vodType = item.vod_type || 'movie';
        showTMDBMatchModal(item.id, item.name, item.tmdb_id, vodType, function(newTmdbId) {
          overlay.remove();
        });
      };
      actionIcons.appendChild(tmdbMatchIcon);
    }

    metaRow.appendChild(actionIcons);
    titleBlock.appendChild(metaRow);
    backdrop.appendChild(titleBlock);
    modal.appendChild(backdrop);

    var body = document.createElement('div');
    body.style.cssText = 'padding:28px 32px;overflow-y:auto;flex:1;';

    var tmdbMeta = document.createElement('div');
    tmdbMeta.style.cssText = 'display:flex;align-items:center;gap:16px;margin-bottom:20px;flex-wrap:wrap;min-height:24px;';

    var pills = [];
    if (item.rating > 0) {
      var sc = item.rating >= 7 ? '#22c55e' : item.rating >= 5 ? '#eab308' : '#ef4444';
      pills.push('<span style="background:' + sc + '20;color:' + sc + ';padding:3px 10px;border-radius:6px;font-weight:700;font-size:13px">\u2605 ' + item.rating.toFixed(1) + '</span>');
    }
    if (item.year) pills.push('<span style="color:#9ca3af;font-size:13px">' + esc(item.year) + '</span>');
    if (item.certification) pills.push('<span style="background:rgba(255,255,255,0.1);color:#fff;padding:3px 10px;border-radius:6px;font-size:12px;font-weight:600;border:1px solid rgba(255,255,255,0.2)">' + esc(item.certification) + '</span>');
    if (item.genres && item.genres.length) {
      item.genres.slice(0, 4).forEach(function(g) {
        pills.push('<span style="background:rgba(59,130,246,0.15);color:#60a5fa;padding:3px 10px;border-radius:6px;font-size:12px">' + esc(g) + '</span>');
      });
    }
    if (pills.length) tmdbMeta.innerHTML = pills.join('');
    body.appendChild(tmdbMeta);

    var descArea = document.createElement('div');
    descArea.style.cssText = 'margin-bottom:24px;';
    if (item.overview) {
      var descEl = document.createElement('p');
      descEl.style.cssText = 'color:#b0b8c8;font-size:15px;line-height:1.7;margin:0;';
      descEl.textContent = item.overview;
      descArea.appendChild(descEl);
    }
    body.appendChild(descArea);

    if (item.alternates && item.alternates.length > 0) {
      var altSection = document.createElement('div');
      altSection.style.cssText = 'margin-bottom:24px;';
      altSection.appendChild(Object.assign(document.createElement('div'), { style: 'font-size:13px;font-weight:600;color:var(--text-muted);margin-bottom:8px;text-transform:uppercase;letter-spacing:1px;', textContent: 'Alternative Sources' }));
      var allSources = [{ id: item.id, name: item.name, group: item.group || '' }].concat(item.alternates);
      allSources.forEach(function(alt, i) {
        var row = document.createElement('div');
        row.style.cssText = 'display:flex;align-items:center;gap:10px;padding:8px 12px;border-radius:8px;transition:background 0.15s;' + (i === 0 ? 'background:rgba(59,130,246,0.15);' : '');
        row.onmouseenter = function() { row.style.background = i === 0 ? 'rgba(59,130,246,0.2)' : 'rgba(255,255,255,0.05)'; };
        row.onmouseleave = function() { row.style.background = i === 0 ? 'rgba(59,130,246,0.15)' : ''; };
        var label = alt.group || alt.name;
        row.appendChild(Object.assign(document.createElement('div'), { style: 'flex:1;font-size:14px;color:var(--text-primary);min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;', textContent: label }));
        if (i === 0) row.appendChild(Object.assign(document.createElement('div'), { style: 'font-size:11px;color:#3b82f6;font-weight:600;flex-shrink:0;', textContent: 'CURRENT' }));
        var altPlayBtn = document.createElement('button');
        altPlayBtn.textContent = '\u25B6';
        altPlayBtn.style.cssText = 'background:rgba(59,130,246,0.8);border:none;color:#fff;width:32px;height:32px;border-radius:50%;cursor:pointer;font-size:14px;flex-shrink:0;transition:background 0.2s;';
        altPlayBtn.onmouseenter = function() { altPlayBtn.style.background = '#3b82f6'; };
        altPlayBtn.onmouseleave = function() { altPlayBtn.style.background = 'rgba(59,130,246,0.8)'; };
        altPlayBtn.onclick = function(e) { e.stopPropagation(); overlay.remove(); startPlay(alt.id, item.name, false); };
        row.appendChild(altPlayBtn);
        altSection.appendChild(row);
      });
      body.appendChild(altSection);
    }

    var castArea = document.createElement('div');
    castArea.style.cssText = 'margin-bottom:24px;display:none;';
    body.appendChild(castArea);

    modal.appendChild(body);
    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    api.get('/api/streams/' + encodeURIComponent(item.id) + '/detail').then(function(resp) {
      return resp.json();
    }).then(function(data) {
      if (!data) return;

      if (data.backdrop_url && !item.backdrop_url) {
        backdrop.style.backgroundImage = 'url(' + data.backdrop_url + ')';
        backdrop.style.backgroundSize = 'cover';
        backdrop.style.backgroundPosition = 'center 20%';
      }

      if (data.crew && data.crew.length > 0) {
        var directors = data.crew.filter(function(c) { return c.job === 'Director'; });
        var writers = data.crew.filter(function(c) { return c.job === 'Writer' || c.job === 'Screenplay'; });
        var crewHtml = '';
        if (directors.length > 0) {
          crewHtml += '<div style="margin-bottom:12px"><span style="font-size:12px;font-weight:600;color:var(--text-muted);text-transform:uppercase;letter-spacing:1px">Directed by</span><br>' +
            '<span style="color:#e2e8f0;font-size:14px">' + directors.map(function(d) { return esc(d.name); }).join(', ') + '</span></div>';
        }
        if (writers.length > 0) {
          crewHtml += '<div><span style="font-size:12px;font-weight:600;color:var(--text-muted);text-transform:uppercase;letter-spacing:1px">Written by</span><br>' +
            '<span style="color:#e2e8f0;font-size:14px">' + writers.map(function(w) { return esc(w.name); }).join(', ') + '</span></div>';
        }
        if (crewHtml) {
          var crewSection = document.createElement('div');
          crewSection.style.cssText = 'margin-bottom:24px;';
          crewSection.innerHTML = crewHtml;
          body.insertBefore(crewSection, castArea);
        }
      }

      if (data.cast && data.cast.length > 0) {
        castArea.style.display = 'block';
        var castHtml = '<div style="font-size:12px;font-weight:600;color:var(--text-muted);text-transform:uppercase;letter-spacing:1px;margin-bottom:10px">Cast</div>' +
          '<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:10px">';
        var maxCast = Math.min(data.cast.length, 12);
        for (var ci = 0; ci < maxCast; ci++) {
          var member = data.cast[ci];
          var photoStyle = 'width:40px;height:40px;border-radius:50%;background:#2d3748;flex-shrink:0;background-size:cover;background-position:center';
          if (member.profile_url) photoStyle += ';background-image:url(' + member.profile_url + ')';
          castHtml += '<div style="display:flex;gap:10px;align-items:center">' +
            '<div style="' + photoStyle + '"></div>' +
            '<div style="min-width:0"><div style="color:#e2e8f0;font-size:13px;font-weight:500;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">' + esc(member.name) + '</div>' +
            '<div style="color:#9ca3af;font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">' + esc(member.character || '') + '</div></div></div>';
        }
        castHtml += '</div>';
        castArea.innerHTML = castHtml;
      }
    }).catch(function() {});
  }

  function showSeriesModal(show) {
    var overlay = document.createElement('div');
    overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.8);z-index:9999;display:flex;align-items:center;justify-content:center;backdrop-filter:blur(6px);';
    overlay.onclick = function(e) { if (e.target === overlay) overlay.remove(); };
    document.addEventListener('keydown', function onKey(e) { if (e.key === 'Escape') { overlay.remove(); document.removeEventListener('keydown', onKey); } });

    var modal = document.createElement('div');
    modal.style.cssText = 'width:90%;max-width:1080px;max-height:92vh;background:#1a1d23;border-radius:16px;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 24px 80px rgba(0,0,0,0.6);';

    var backdrop = document.createElement('div');
    backdrop.style.cssText = 'width:100%;height:280px;background:linear-gradient(135deg,#1a1a2e,#0f3460);position:relative;overflow:hidden;flex-shrink:0;';
    if (show.backdrop_url) {
      backdrop.style.backgroundImage = 'url(' + show.backdrop_url + ')';
      backdrop.style.backgroundSize = 'cover';
      backdrop.style.backgroundPosition = 'center 20%';
    } else if (show.poster_url) {
      backdrop.style.backgroundImage = 'url(' + show.poster_url + ')';
      backdrop.style.backgroundSize = 'cover';
      backdrop.style.backgroundPosition = 'center 20%';
    }

    var closeBtn = document.createElement('button');
    closeBtn.textContent = '\u2715';
    closeBtn.style.cssText = 'position:absolute;top:16px;right:16px;background:rgba(0,0,0,0.6);border:none;color:#fff;font-size:18px;width:40px;height:40px;border-radius:50%;cursor:pointer;z-index:3;';
    closeBtn.onclick = function() { overlay.remove(); };
    backdrop.appendChild(closeBtn);

    if (api.user && api.user.is_admin) {
      var tmdbMatchBtn = document.createElement('button');
      tmdbMatchBtn.style.cssText = 'position:absolute;top:16px;right:64px;background:rgba(0,0,0,0.6);border:none;color:#fff;font-size:13px;height:40px;padding:0 14px;border-radius:20px;cursor:pointer;z-index:3;display:flex;align-items:center;gap:6px;transition:background 0.2s;';
      var ep0 = show.episodes && show.episodes[0];
      var seriesTmdbId = ep0 ? ep0.tmdb_id : '';
      tmdbMatchBtn.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>' + (seriesTmdbId ? 'Edit Match' : 'Match');
      tmdbMatchBtn.onmouseenter = function() { tmdbMatchBtn.style.background = '#10b981'; };
      tmdbMatchBtn.onmouseleave = function() { tmdbMatchBtn.style.background = 'rgba(0,0,0,0.6)'; };
      tmdbMatchBtn.onclick = function() {
        var firstEp = show.episodes && show.episodes[0];
        var sid = firstEp ? firstEp.id : '';
        showTMDBMatchModal(sid, show.name, seriesTmdbId, 'series', function(newTmdbId) {
          overlay.remove();
        });
      };
      backdrop.appendChild(tmdbMatchBtn);
    }

    backdrop.appendChild(Object.assign(document.createElement('div'), {
      style: 'position:absolute;bottom:0;left:0;right:0;height:150px;background:linear-gradient(transparent,#1a1d23);'
    }));

    var titleBlock = document.createElement('div');
    titleBlock.style.cssText = 'position:absolute;bottom:24px;left:32px;z-index:1;';
    titleBlock.innerHTML = '<div style="font-size:32px;font-weight:800;color:#fff;text-shadow:0 2px 12px rgba(0,0,0,0.7)">' + esc(show.name) + '</div>';
    var seasonCount = Object.keys(show.seasons).length;
    titleBlock.innerHTML += '<div style="color:rgba(255,255,255,0.7);font-size:14px;margin-top:4px">' + seasonCount + ' season' + (seasonCount > 1 ? 's' : '') + ' \u2022 ' + show.episodes.length + ' episodes</div>';
    backdrop.appendChild(titleBlock);
    modal.appendChild(backdrop);

    var body = document.createElement('div');
    body.style.cssText = 'padding:24px 32px;overflow-y:auto;flex:1;';

    var tmdbMeta = document.createElement('div');
    tmdbMeta.style.cssText = 'display:flex;align-items:center;gap:10px;margin-bottom:16px;flex-wrap:wrap;min-height:24px;';
    var sPills = [];
    if (show.year) sPills.push('<span style="color:#9ca3af;font-size:13px">' + esc(show.year) + '</span>');
    if (show.rating > 0) {
      var sc = show.rating >= 7 ? '#22c55e' : show.rating >= 5 ? '#eab308' : '#ef4444';
      sPills.push('<span style="background:' + sc + '20;color:' + sc + ';padding:3px 10px;border-radius:6px;font-weight:700;font-size:13px">\u2605 ' + show.rating.toFixed(1) + '</span>');
    }
    if (show.genres && show.genres.length) {
      show.genres.slice(0, 3).forEach(function(g) {
        sPills.push('<span style="background:rgba(59,130,246,0.15);color:#60a5fa;padding:3px 10px;border-radius:6px;font-size:12px">' + esc(g) + '</span>');
      });
    }
    if (sPills.length) tmdbMeta.innerHTML = sPills.join(' ');
    body.appendChild(tmdbMeta);

    if (show.overview) {
      var desc = document.createElement('p');
      desc.style.cssText = 'color:#b0b8c8;font-size:14px;line-height:1.6;margin:0 0 16px 0;';
      desc.textContent = show.overview;
      body.appendChild(desc);
    }

    var seasonKeys = Object.keys(show.seasons);
    if (seasonKeys.length === 0 && show.episodes.length > 0) {
      show.seasons[1] = show.episodes;
      seasonKeys = ['1'];
    }
    seasonKeys.sort(function(a, b) {
      var aNum = parseInt(a, 10);
      var bNum = parseInt(b, 10);
      if (!isNaN(aNum) && !isNaN(bNum)) return aNum - bNum;
      if (!isNaN(aNum)) return -1;
      if (!isNaN(bNum)) return 1;
      return a.localeCompare(b);
    });

    var tabBar = document.createElement('div');
    tabBar.style.cssText = 'display:flex;gap:8px;margin-bottom:16px;flex-wrap:wrap;';

    var epList = document.createElement('div');

    function renderSeason(key) {
      epList.innerHTML = '';
      tabBar.querySelectorAll('button').forEach(function(btn) {
        btn.style.background = btn.dataset.season == key ? '#3b82f6' : 'rgba(255,255,255,0.1)';
      });
      var eps = show.seasons[key] || [];
      eps.sort(function(a, b) { return (a.episode || 0) - (b.episode || 0); });
      eps.forEach(function(ep) {
        var row = document.createElement('div');
        row.style.cssText = 'display:flex;align-items:flex-start;gap:16px;padding:12px 16px;border-radius:8px;cursor:pointer;transition:background 0.15s;';
        row.onmouseenter = function() { row.style.background = 'rgba(255,255,255,0.05)'; };
        row.onmouseleave = function() { row.style.background = ''; };

        var epNum = document.createElement('span');
        epNum.style.cssText = 'font-size:24px;font-weight:700;color:var(--text-muted);min-width:40px;text-align:center;flex-shrink:0;padding-top:2px;';
        epNum.textContent = ep.episode || '?';
        row.appendChild(epNum);

        if (ep.episode_still) {
          var stillImg = document.createElement('img');
          stillImg.src = ep.episode_still;
          stillImg.style.cssText = 'width:120px;height:68px;object-fit:cover;border-radius:6px;flex-shrink:0;background:#1a1a2e;';
          stillImg.onerror = function() { this.style.display = 'none'; };
          row.appendChild(stillImg);
        }

        var epInfo = document.createElement('div');
        epInfo.style.cssText = 'flex:1;min-width:0;';
        epInfo.appendChild(Object.assign(document.createElement('div'), { style: 'font-size:14px;font-weight:600;color:var(--text-primary);', textContent: ep.episode_name || ('Episode ' + ep.episode) }));
        if (ep.episode_overview) {
          var epDesc = document.createElement('div');
          epDesc.style.cssText = 'font-size:12px;color:#9ca3af;margin-top:3px;line-height:1.4;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;';
          epDesc.textContent = ep.episode_overview;
          epInfo.appendChild(epDesc);
        }

        row.appendChild(epInfo);

        var playBtn = document.createElement('button');
        playBtn.style.cssText = 'background:#3b82f6;border:none;color:#fff;width:36px;height:36px;border-radius:50%;cursor:pointer;font-size:16px;flex-shrink:0;margin-top:2px;';
        playBtn.textContent = '\u25B6';
        playBtn.onclick = function(e) {
          e.stopPropagation();
          overlay.remove();
          startPlay(ep.id, show.name + ' S' + String(ep.season).padStart(2, '0') + 'E' + String(ep.episode).padStart(2, '0'), false);
        };
        row.appendChild(playBtn);

        row.onclick = function() {
          overlay.remove();
          startPlay(ep.id, show.name + ' S' + String(ep.season).padStart(2, '0') + 'E' + String(ep.episode).padStart(2, '0'), false);
        };

        epList.appendChild(row);
      });
    }

    seasonKeys.forEach(function(key) {
      var btn = document.createElement('button');
      btn.style.cssText = 'background:rgba(255,255,255,0.1);border:none;color:#fff;padding:6px 16px;border-radius:6px;cursor:pointer;font-size:13px;font-weight:600;';
      var num = parseInt(key, 10);
      btn.textContent = isNaN(num) ? key : 'Season ' + num;
      btn.dataset.season = key;
      btn.onclick = function() { renderSeason(key); };
      tabBar.appendChild(btn);
    });

    body.appendChild(tabBar);
    body.appendChild(epList);
    modal.appendChild(body);
    overlay.appendChild(modal);
    document.body.appendChild(overlay);

    var ep0 = show.episodes[0];
    var tmdbId = show.tmdb_id || (ep0 && ep0.tmdb_id);

    function enrichAndRender() {
      if (seasonKeys.length > 0) renderSeason(seasonKeys[0]);
    }

    if (tmdbId) {
      api.get('/api/tmdb/detail/' + encodeURIComponent(tmdbId) + '?type=series').then(function(resp) {
        return resp.ok ? resp.json() : null;
      }).then(function(data) {
        if (data && data.seasons) {
          data.seasons.forEach(function(sn) {
            if (sn.season_number === 0) return;
            var seasonEps = show.seasons[sn.season_number];
            if (seasonEps && sn.episodes) {
              sn.episodes.forEach(function(tmdbEp) {
                var matchEp = seasonEps.find(function(e) { return e.episode === tmdbEp.episode_number; });
                if (matchEp) {
                  if (tmdbEp.name) matchEp.episode_name = tmdbEp.name;
                  if (tmdbEp.overview) matchEp.episode_overview = tmdbEp.overview;
                }
              });
            }
          });
          if (data.overview && !show.overview) {
            var d = document.createElement('p');
            d.style.cssText = 'color:#b0b8c8;font-size:14px;line-height:1.6;margin:0 0 16px 0;';
            d.textContent = data.overview;
            body.insertBefore(d, tabBar);
          }
          if (data.rating > 0) {
            var dsc = data.rating >= 7 ? '#22c55e' : data.rating >= 5 ? '#eab308' : '#ef4444';
            tmdbMeta.innerHTML = '<span style="background:' + dsc + '20;color:' + dsc + ';padding:3px 10px;border-radius:6px;font-weight:700;font-size:13px">\u2605 ' + data.rating.toFixed(1) + '</span>';
          }
          if (data.backdrop_url) {
            backdrop.style.backgroundImage = 'url(' + data.backdrop_url + ')';
            backdrop.style.backgroundSize = 'cover';
            backdrop.style.backgroundPosition = 'center 20%';
          } else if (tmdbId) {
            var bgImg = new Image();
            bgImg.onload = function() { backdrop.style.backgroundImage = 'url(' + bgImg.src + ')'; backdrop.style.backgroundSize = 'cover'; backdrop.style.backgroundPosition = 'center 20%'; };
            bgImg.src = '/api/tmdb/i/' + tmdbId + '/backdrop.jpg';
          }
        }
        enrichAndRender();
      }).catch(function() { enrichAndRender(); });
    } else if (ep0 && ep0.id) {
      api.get('/api/streams/' + encodeURIComponent(ep0.id) + '/detail').then(function(resp) {
        return resp.json();
      }).then(function(data) {
        if (!data) return;
        if (data.backdrop_url && !show.backdrop_url) {
          backdrop.style.backgroundImage = 'url(' + data.backdrop_url + ')';
          backdrop.style.backgroundSize = 'cover';
          backdrop.style.backgroundPosition = 'center 20%';
        }
        if (data.overview && !show.overview) {
          var d = document.createElement('p');
          d.style.cssText = 'color:#b0b8c8;font-size:14px;line-height:1.6;margin:0 0 16px 0;';
          d.textContent = data.overview;
          body.insertBefore(d, tabBar);
        }
        var detailPills = [];
        if (data.rating > 0 && !show.rating) {
          var dsc = data.rating >= 7 ? '#22c55e' : data.rating >= 5 ? '#eab308' : '#ef4444';
          detailPills.push('<span style="background:' + dsc + '20;color:' + dsc + ';padding:3px 10px;border-radius:6px;font-weight:700;font-size:13px">\u2605 ' + data.rating.toFixed(1) + '</span>');
        }
        if (data.first_air_date && !show.year) detailPills.push('<span style="color:#9ca3af;font-size:13px">' + data.first_air_date.substring(0, 4) + '</span>');
        if (data.genres && data.genres.length && (!show.genres || !show.genres.length)) {
          data.genres.slice(0, 3).forEach(function(g) {
            detailPills.push('<span style="background:rgba(59,130,246,0.15);color:#60a5fa;padding:3px 10px;border-radius:6px;font-size:12px">' + esc(g) + '</span>');
          });
        }
        if (detailPills.length) tmdbMeta.innerHTML = detailPills.join(' ');

        if (data.seasons && data.seasons.length > 0) {
          data.seasons.forEach(function(sn) {
            if (sn.season_number === 0) return;
            if (sn.episodes && sn.episodes.length > 0) {
              var seasonEps = show.seasons[sn.season_number];
              if (seasonEps) {
                sn.episodes.forEach(function(tmdbEp) {
                  var matchEp = seasonEps.find(function(e) { return e.episode === tmdbEp.episode_number; });
                  if (matchEp) {
                    if (tmdbEp.name) matchEp.episode_name = tmdbEp.name;
                    if (tmdbEp.overview) matchEp.episode_overview = tmdbEp.overview;
                    if (tmdbEp.still_url) matchEp.episode_still = tmdbEp.still_url;
                  }
                });
              }
            }
          });
          enrichAndRender();
        }
      }).catch(function() { enrichAndRender(); });
    } else {
      enrichAndRender();
    }
  }

  function showCollectionModal(col) {
    var overlay = document.createElement('div');
    overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.8);z-index:9999;display:flex;align-items:center;justify-content:center;backdrop-filter:blur(6px);';
    overlay.onclick = function(e) { if (e.target === overlay) overlay.remove(); };
    document.addEventListener('keydown', function onKey(e) { if (e.key === 'Escape') { overlay.remove(); document.removeEventListener('keydown', onKey); } });

    var modal = document.createElement('div');
    modal.style.cssText = 'width:90%;max-width:1080px;max-height:92vh;background:#1a1d23;border-radius:16px;overflow:hidden;display:flex;flex-direction:column;box-shadow:0 24px 80px rgba(0,0,0,0.6);';

    var backdrop = document.createElement('div');
    backdrop.style.cssText = 'width:100%;height:280px;background:linear-gradient(135deg,#1a1a2e,#0f3460);position:relative;overflow:hidden;flex-shrink:0;';
    if (col.backdrop_url) {
      backdrop.style.backgroundImage = 'url(' + col.backdrop_url + ')';
      backdrop.style.backgroundSize = 'cover';
      backdrop.style.backgroundPosition = 'center 20%';
    }

    backdrop.appendChild(Object.assign(document.createElement('div'), { style: 'position:absolute;bottom:0;left:0;right:0;height:150px;background:linear-gradient(transparent,#1a1d23);' }));

    var closeBtn = document.createElement('button');
    closeBtn.textContent = '\u2715';
    closeBtn.style.cssText = 'position:absolute;top:16px;right:16px;background:rgba(0,0,0,0.6);border:none;color:#fff;font-size:18px;width:40px;height:40px;border-radius:50%;cursor:pointer;z-index:3;';
    closeBtn.onclick = function() { overlay.remove(); };
    backdrop.appendChild(closeBtn);

    var titleBlock = document.createElement('div');
    titleBlock.style.cssText = 'position:absolute;bottom:24px;left:32px;z-index:1;';
    titleBlock.innerHTML = '<div style="font-size:32px;font-weight:800;color:#fff;text-shadow:0 2px 12px rgba(0,0,0,0.7)">' + esc(col.name) + '</div>';
    titleBlock.innerHTML += '<div style="color:rgba(255,255,255,0.7);font-size:14px;margin-top:4px">' + col.movies.length + ' film' + (col.movies.length > 1 ? 's' : '') + '</div>';
    backdrop.appendChild(titleBlock);
    modal.appendChild(backdrop);

    var body = document.createElement('div');
    body.style.cssText = 'padding:24px 32px;overflow-y:auto;flex:1;';

    col.movies.forEach(function(movie) {
      var row = document.createElement('div');
      row.style.cssText = 'display:flex;align-items:flex-start;gap:16px;padding:12px 16px;border-radius:8px;cursor:pointer;transition:background 0.15s;';
      row.onmouseenter = function() { row.style.background = 'rgba(255,255,255,0.05)'; };
      row.onmouseleave = function() { row.style.background = ''; };

      if (movie.poster_url) {
        var img = document.createElement('img');
        img.src = movie.poster_url;
        img.style.cssText = 'width:60px;height:90px;object-fit:cover;border-radius:6px;flex-shrink:0;';
        row.appendChild(img);
      }

      var info = document.createElement('div');
      info.style.cssText = 'flex:1;min-width:0;';
      info.appendChild(Object.assign(document.createElement('div'), { style: 'font-size:14px;font-weight:600;color:var(--text-primary);', textContent: movie.name }));

      var meta = [];
      if (movie.year) meta.push(movie.year);
      if (movie.certification) meta.push(movie.certification);
      if (movie.rating > 0) meta.push('\u2605 ' + movie.rating.toFixed(1));
      if (meta.length) info.appendChild(Object.assign(document.createElement('div'), { style: 'font-size:12px;color:var(--text-muted);margin-top:2px;', textContent: meta.join(' \u2022 ') }));

      if (movie.genres && movie.genres.length) {
        var gRow = document.createElement('div');
        gRow.style.cssText = 'display:flex;gap:4px;flex-wrap:wrap;margin-top:3px;';
        movie.genres.slice(0, 3).forEach(function(g) {
          var span = document.createElement('span');
          span.style.cssText = 'font-size:10px;padding:1px 5px;border-radius:3px;background:rgba(59,130,246,0.15);color:#60a5fa;';
          span.textContent = g;
          gRow.appendChild(span);
        });
        info.appendChild(gRow);
      }

      if (movie.overview) {
        info.appendChild(Object.assign(document.createElement('div'), { style: 'font-size:11px;color:#9ca3af;margin-top:4px;line-height:1.4;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;', textContent: movie.overview }));
      }

      row.appendChild(info);

      var playBtn = document.createElement('button');
      playBtn.style.cssText = 'background:#3b82f6;border:none;color:#fff;width:36px;height:36px;border-radius:50%;cursor:pointer;font-size:16px;flex-shrink:0;margin-top:2px;';
      playBtn.textContent = '\u25B6';
      playBtn.onclick = function(e) { e.stopPropagation(); overlay.remove(); startPlay(movie.id, movie.name, false); };
      row.appendChild(playBtn);

      row.onclick = function() { overlay.remove(); showMovieModal(movie); };
      body.appendChild(row);
    });

    modal.appendChild(body);
    overlay.appendChild(modal);
    document.body.appendChild(overlay);
  }

  async function renderGuide(el) {
    var HOUR_WIDTH = 240;
    var PX_PER_MIN = HOUR_WIDTH / 60;
    var CHANNEL_COL = 200;
    var currentHours = 6;
    var windowOffset = 0;

    var channels, groups, guideData, scheduledRecs;
    var windowStart, windowStop, windowMinutes, totalWidth, programs, now;
    var guideLoading = false;

    el.innerHTML = '<div style="padding:40px;text-align:center"><div class="spinner-ring"></div> Loading guide...</div>';

    try {
      var chResp = await api.get('/api/channels');
      channels = await chResp.json();
      if (!Array.isArray(channels)) channels = [];

      var grResp = await api.get('/api/channel-groups');
      groups = await grResp.json();
      if (!Array.isArray(groups)) groups = [];

      var gdResp = await api.get('/api/epg/guide?hours=' + currentHours);
      guideData = await gdResp.json();

      try {
        var srResp = await api.get('/api/recordings/schedule');
        scheduledRecs = await srResp.json();
      } catch (e) { scheduledRecs = []; }
      if (!Array.isArray(scheduledRecs)) scheduledRecs = [];
    } catch (err) {
      el.innerHTML = '<div class="empty-state">' + icons.epg + '<p style="color:var(--danger)">Failed to load: ' + esc(err.message) + '</p></div>';
      return;
    }

    var scheduledSet = {};
    scheduledRecs.forEach(function(sr) {
      if (sr.status === 'scheduled' || sr.status === 'recording') {
        var key = sr.channel_id + '|' + new Date(sr.scheduled_start).toISOString();
        scheduledSet[key] = sr.id;
      }
    });

    channels = channels.filter(function(c) { return c.is_enabled; });
    channels.sort(function(a, b) { return (a.name || '').localeCompare(b.name || ''); });

    if (channels.length === 0) {
      el.innerHTML = '<div class="empty-state"><p>No channels configured</p><p style="color:var(--text-muted);font-size:13px">Add channels first, then assign EPG IDs to see the guide.</p></div>';
      return;
    }

    var groupMap = {};
    groups.forEach(function(g) { groupMap[g.id] = g; });

    var grouped = {};
    var ungrouped = [];
    channels.forEach(function(c) {
      if (c.group_id && groupMap[c.group_id]) {
        if (!grouped[c.group_id]) grouped[c.group_id] = [];
        grouped[c.group_id].push(c);
      } else {
        ungrouped.push(c);
      }
    });

    var sortedGroupIds = Object.keys(grouped).sort(function(a, b) {
      var ga = groupMap[a] || {}; var gb = groupMap[b] || {};
      if ((ga.sort_order || 0) !== (gb.sort_order || 0)) return (ga.sort_order || 0) - (gb.sort_order || 0);
      return (ga.name || '').localeCompare(gb.name || '');
    });

    function parseGuideData() {
      windowStart = new Date(guideData.start).getTime();
      windowStop = new Date(guideData.stop).getTime();
      windowMinutes = (windowStop - windowStart) / 60000;
      totalWidth = windowMinutes * PX_PER_MIN;
      programs = guideData.programs || {};
      now = Date.now();
    }
    parseGuideData();

    function guideFormatTime(d) {
      var dt = new Date(d);
      var hh = dt.getHours();
      var mm = dt.getMinutes();
      return (hh < 10 ? '0' : '') + hh + ':' + (mm < 10 ? '0' : '') + mm;
    }

    function guideFormatDate(ts) {
      var d = new Date(ts);
      var months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
      return months[d.getMonth()] + ' ' + d.getDate() + ', ' + guideFormatTime(ts);
    }

    var channelCounter = 0;
    function buildChannelRow(ch) {
      channelCounter++;
      var tvgId = ch.tvg_id || ch.id;
      var chPrograms = programs[tvgId] || [];
      var programsHtml = '';

      for (var i = 0; i < chPrograms.length; i++) {
        var p = chPrograms[i];
        var pStart = new Date(p.start).getTime();
        var pStop = new Date(p.stop).getTime();
        var startMin = Math.max(0, (pStart - windowStart) / 60000);
        var endMin = Math.min(windowMinutes, (pStop - windowStart) / 60000);
        var leftPx = startMin * PX_PER_MIN;
        var widthPx = (endMin - startMin) * PX_PER_MIN - 2;
        if (widthPx < 2) continue;

        var isLive = now >= pStart && now < pStop;
        var isPast = now >= pStop;
        var cls = 'epg-program' + (isLive ? ' live' : '') + (isPast ? ' past' : '');
        var timeStr = guideFormatTime(pStart) + ' - ' + guideFormatTime(pStop);
        var tooltip = esc(p.title) + ' (' + timeStr + ')';
        if (p.description) tooltip += '&#10;' + esc(p.description.substring(0, 200));

        var schedKey = ch.id + '|' + new Date(p.start).toISOString();
        var isScheduled = !!scheduledSet[schedKey];
        var recBtnCls = 'epg-record-btn' + (isScheduled ? ' scheduled' : '');
        var recBtnHtml = isPast ? '' : '<button class="' + recBtnCls + '" data-ptitle="' + esc(p.title) + '" data-pstart="' + esc(p.start) + '" data-pstop="' + esc(p.stop) + '"' + (isScheduled ? ' data-scheduled="' + esc(scheduledSet[schedKey]) + '"' : '') + '>\u23FA</button>';
        programsHtml += '<div class="' + cls + '" style="left:' + leftPx + 'px;width:' + widthPx + 'px" title="' + tooltip + '"' +
          ' data-desc="' + esc(p.description || '') + '"' +
          ' data-cats="' + esc((p.categories || []).join(', ')) + '"' +
          ' data-pstart="' + esc(p.start || '') + '"' +
          ' data-pstop="' + esc(p.stop || '') + '"' +
          ' data-series-id="' + esc(p.series_id || '') + '"' +
          ' data-episode-num="' + esc(p.episode_num || '') + '">' +
          recBtnHtml +
          (p.series_id ? '<button class="epg-series-btn" data-series-id="' + esc(p.series_id) + '" title="View all episodes" style="position:absolute;top:2px;right:2px;font-size:10px;opacity:0.8;background:none;border:none;cursor:pointer;color:var(--accent);padding:0 2px;">\u{1F517}</button>' : '') +
          '<div class="epg-program-title">' + esc(p.title) + '</div>' +
          '<div class="epg-program-time">' + timeStr + '</div>' +
          '</div>';
      }

      if (chPrograms.length === 0 && tvgId) {
        programsHtml = '<div class="epg-program" style="left:0;width:' + (totalWidth - 2) + 'px;opacity:0.3"><div class="epg-program-title">No EPG data</div></div>';
      }

      var logoHtml = ch.logo_url
        ? '<img class="epg-channel-logo" src="' + esc(ch.logo_url) + '" loading="lazy" alt="">'
        : '<div class="epg-channel-logo"></div>';

      return '<div class="epg-row">' +
        '<div class="epg-channel" data-chid="' + esc(String(ch.id)) + '" data-tvgid="' + esc(tvgId) + '" data-chname="' + esc(ch.name) + '" title="' + esc(ch.name) + '">' +
          '<span class="epg-channel-num">' + channelCounter + '</span>' +
          logoHtml +
          '<span class="epg-channel-name">' + esc(ch.name) + '</span>' +
        '</div>' +
        '<div class="epg-programs" style="width:' + totalWidth + 'px">' + programsHtml + '</div>' +
      '</div>';
    }

    function buildRows() {
      channelCounter = 0;
      var rowsHtml = '';
      for (var gi = 0; gi < sortedGroupIds.length; gi++) {
        var gid = sortedGroupIds[gi];
        var grp = groupMap[gid];
        rowsHtml += '<div class="epg-group-row">' + esc(grp.name) + '</div>';
        var grpChannels = grouped[gid];
        for (var ci = 0; ci < grpChannels.length; ci++) {
          rowsHtml += buildChannelRow(grpChannels[ci]);
        }
      }
      if (ungrouped.length > 0) {
        if (sortedGroupIds.length > 0) {
          rowsHtml += '<div class="epg-group-row">Ungrouped</div>';
        }
        for (var ui = 0; ui < ungrouped.length; ui++) {
          rowsHtml += buildChannelRow(ungrouped[ui]);
        }
      }
      return rowsHtml;
    }

    function formatOffset(hrs) {
      if (hrs === 0) return 'Now';
      var sign = hrs > 0 ? '+' : '-';
      var abs = Math.abs(hrs);
      if (abs >= 24 && abs % 24 === 0) return 'Now ' + sign + (abs / 24) + 'd';
      return 'Now ' + sign + abs + 'h';
    }

    function renderFull() {
      var hourMarksHtml = '';
      for (var m = 0; m < windowMinutes; m += 60) {
        hourMarksHtml += '<div class="epg-hour-mark" style="width:' + HOUR_WIDTH + 'px">' + guideFormatTime(windowStart + m * 60000) + '</div>';
      }

      var rowsHtml = buildRows();

      var nowMin = (now - windowStart) / 60000;
      var nowPx = nowMin * PX_PER_MIN;
      var nowLineHtml = (nowMin >= 0 && nowMin <= windowMinutes)
        ? '<div class="epg-now-line" style="left:' + (CHANNEL_COL + nowPx) + 'px"></div>'
        : '';

      var ws = new Date(windowStart);
      var days = ['Sunday','Monday','Tuesday','Wednesday','Thursday','Friday','Saturday'];
      var dayStr = days[ws.getDay()] + ' ' + ws.toLocaleDateString(undefined, { day: 'numeric', month: 'short' });

      el.innerHTML = '';

      var deltaPresets = [-24, -6, -3, -1, 1, 3, 6, 24];
      var deltaLabels = { '-24': '-1d', '-6': '-6h', '-3': '-3h', '-1': '-1h', '1': '+1h', '3': '+3h', '6': '+6h', '24': '+1d' };

      var navHtml = '';
      for (var di = 0; di < deltaPresets.length; di++) {
        var d = deltaPresets[di];
        if (d > 0 && di === 4) {
          navHtml += '<button class="btn btn-sm btn-primary epg-nav-btn" data-delta="0">' + esc(formatOffset(windowOffset)) + '</button>';
        }
        navHtml += '<button class="btn btn-sm btn-ghost epg-nav-btn" data-delta="' + d + '">' + esc(deltaLabels[String(d)]) + '</button>';
      }
      if (deltaPresets.length === 8 && deltaPresets[4] <= 0) {
        navHtml += '<button class="btn btn-sm btn-primary epg-nav-btn" data-delta="0">' + esc(formatOffset(windowOffset)) + '</button>';
      }

      var toolbar = document.createElement('div');
      toolbar.className = 'epg-toolbar';
      toolbar.innerHTML =
        '<div class="epg-nav">' + navHtml + '</div>' +
        '<span class="epg-day-label">' + esc(dayStr) + '</span>' +
        '<span class="epg-time-label">' + esc(guideFormatDate(windowStart) + ' \u2014 ' + guideFormatDate(windowStop)) + '</span>' +
        '<span style="font-size:13px;color:var(--text-muted)">' + channels.length + ' channels</span>';
      el.appendChild(toolbar);

      var scrollEl = document.createElement('div');
      scrollEl.className = 'epg-scroll';
      scrollEl.innerHTML = '<div class="epg-header-row">' +
        '<div class="epg-corner">Channel</div>' +
        '<div class="epg-timeline">' + hourMarksHtml + '</div>' +
        '</div>' +
        '<div style="position:relative">' +
        nowLineHtml +
        rowsHtml +
        '</div>';

      scrollEl.addEventListener('click', function(e) {
        var recBtn = e.target.closest('.epg-record-btn');
        if (recBtn) {
          e.stopPropagation();
          if (recBtn.classList.contains('recording')) return;
          if (recBtn.classList.contains('scheduled')) {
            var recID = recBtn.dataset.scheduled;
            if (recID) {
              recBtn.disabled = true;
              api.del('/api/recordings/schedule/' + recID).then(function(resp) {
                if (resp.ok || resp.status === 204) {
                  recBtn.classList.remove('scheduled');
                  delete recBtn.dataset.scheduled;
                  toast('Recording cancelled');
                } else {
                  toast('Failed to cancel', 'error');
                }
                recBtn.disabled = false;
              }).catch(function() { recBtn.disabled = false; });
            }
            return;
          }
          var row = recBtn.closest('.epg-row');
          if (!row) return;
          var ch = row.querySelector('.epg-channel');
          if (!ch) return;
          var pStart = recBtn.dataset.pstart || '';
          var pStop = recBtn.dataset.pstop || '';
          var body = { channel_id: ch.dataset.chid, title: recBtn.dataset.ptitle || '', start_at: pStart, stop_at: pStop };
          recBtn.classList.add('scheduled');
          recBtn.disabled = true;
          api.post('/api/recordings/schedule', body).then(function(resp) {
            if (resp.ok) {
              resp.json().then(function(rec) {
                recBtn.dataset.scheduled = rec.id;
                toast('Recording scheduled');
              }).catch(function() { toast('Recording scheduled'); });
            } else {
              recBtn.classList.remove('scheduled'); recBtn.disabled = false; toast('Failed to schedule', 'error');
            }
          }).catch(function() {
            recBtn.classList.remove('scheduled'); recBtn.disabled = false;
          });
          return;
        }

        var seriesBtn = e.target.closest('.epg-series-btn');
        if (seriesBtn) {
          e.stopPropagation();
          var sid = seriesBtn.dataset.seriesId;
          if (sid) showSeriesModal(sid);
          return;
        }

        var prog = e.target.closest('.epg-program');
        if (prog) {
          var row = prog.closest('.epg-row');
          if (!row) return;
          var ch = row.querySelector('.epg-channel');
          if (!ch) return;
          var titleEl = prog.querySelector('.epg-program-title');
          var timeEl = prog.querySelector('.epg-program-time');
          var progTitle = titleEl ? titleEl.textContent : '';
          var progTime = timeEl ? timeEl.textContent : '';
          var progDesc = prog.dataset.desc || '';
          var progCats = prog.dataset.cats || '';
          var pStart = prog.dataset.pstart || '';
          var pStop = prog.dataset.pstop || '';
          showGuideModal({
            title: progTitle,
            time: progTime,
            description: progDesc,
            categories: progCats,
            channelName: ch.dataset.chname || '',
            channelID: ch.dataset.chid,
            isLive: prog.classList.contains('live'),
            isFuture: pStart ? new Date(pStart).getTime() > Date.now() : false,
            isPast: prog.classList.contains('past'),
            start: pStart,
            stop: pStop,
            seriesID: prog.dataset.seriesId || '',
            episodeNum: prog.dataset.episodeNum || '',
          });
          return;
        }

        var ch = e.target.closest('.epg-channel');
        if (ch) {
          showGuideModal({
            title: ch.dataset.chname || 'Channel',
            channelName: ch.dataset.chname || '',
            channelID: ch.dataset.chid,
            isLive: true,
          });
        }
      });

      toolbar.addEventListener('click', function(e) {
        var btn = e.target.closest('.epg-nav-btn');
        if (!btn) return;
        var delta = parseInt(btn.dataset.delta, 10);
        if (isNaN(delta)) return;
        navigate(delta);
      });

      el.appendChild(scrollEl);

      if (nowMin >= 0 && nowMin <= windowMinutes) {
        var scrollTarget = nowPx - scrollEl.clientWidth / 2 + CHANNEL_COL;
        if (scrollTarget > 0) scrollEl.scrollLeft = scrollTarget;
      }
    }

    async function navigate(delta) {
      if (guideLoading) return;
      if (delta === 0) {
        windowOffset = 0;
      } else {
        windowOffset += delta;
      }
      guideLoading = true;
      el.innerHTML = '<div style="padding:40px;text-align:center"><div class="spinner-ring"></div> Loading guide...</div>';
      try {
        var startParam = '';
        if (windowOffset !== 0) {
          var offsetMs = windowOffset * 3600000;
          var baseStart = new Date(Date.now() + offsetMs);
          baseStart = new Date(baseStart.getTime() - (baseStart.getTime() % (30 * 60000)));
          startParam = '&start=' + baseStart.toISOString();
        }
        var gdResp = await api.get('/api/epg/guide?hours=' + currentHours + startParam);
        guideData = await gdResp.json();
        parseGuideData();
        guideLoading = false;
        renderFull();
      } catch (err) {
        guideLoading = false;
        el.innerHTML = '<div class="empty-state">' + icons.epg + '<p style="color:var(--danger)">Failed to load: ' + esc(err.message) + '</p></div>';
      }
    }

    function showGuideModal(opts) {
      var overlay = document.createElement('div');
      overlay.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.7);z-index:9999;display:flex;align-items:center;justify-content:center;backdrop-filter:blur(4px);';
      overlay.onclick = function(e) { if (e.target === overlay) overlay.remove(); };
      document.addEventListener('keydown', function onKey(e) { if (e.key === 'Escape') { overlay.remove(); document.removeEventListener('keydown', onKey); } });

      var modal = document.createElement('div');
      modal.style.cssText = 'width:90%;max-width:560px;max-height:80vh;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius-lg);overflow:hidden;position:relative;display:flex;flex-direction:column;';

      var header = document.createElement('div');
      header.style.cssText = 'padding:20px 24px 12px;border-bottom:1px solid var(--border);';

      var closeBtn = document.createElement('button');
      closeBtn.textContent = '\u2715';
      closeBtn.style.cssText = 'position:absolute;top:12px;right:12px;background:none;border:none;color:var(--text-dim);font-size:16px;cursor:pointer;padding:4px 8px;border-radius:4px;';
      closeBtn.onclick = function() { overlay.remove(); };
      header.appendChild(closeBtn);

      var titleEl = document.createElement('div');
      titleEl.style.cssText = 'font-size:18px;font-weight:700;color:#fff;padding-right:32px;';
      titleEl.textContent = opts.title || '';
      header.appendChild(titleEl);

      if (opts.channelName) {
        var chanEl = document.createElement('div');
        chanEl.style.cssText = 'font-size:13px;color:var(--text-dim);margin-top:4px;';
        chanEl.textContent = opts.channelName;
        header.appendChild(chanEl);
      }

      if (opts.isLive) {
        var liveBadge = document.createElement('span');
        liveBadge.style.cssText = 'display:inline-block;background:var(--danger);color:#fff;font-size:10px;font-weight:700;padding:2px 8px;border-radius:4px;margin-top:8px;letter-spacing:1px;';
        liveBadge.textContent = 'LIVE';
        header.appendChild(liveBadge);
      }

      modal.appendChild(header);

      var body = document.createElement('div');
      body.style.cssText = 'padding:16px 24px 20px;overflow-y:auto;flex:1;';

      if (opts.time) {
        var timeEl = document.createElement('div');
        timeEl.style.cssText = 'font-size:13px;color:var(--text-dim);margin-bottom:12px;';
        timeEl.textContent = opts.time;
        body.appendChild(timeEl);
      }

      if (opts.categories) {
        var catsEl = document.createElement('div');
        catsEl.style.cssText = 'font-size:12px;color:var(--accent);margin-bottom:12px;';
        catsEl.textContent = opts.categories;
        body.appendChild(catsEl);
      }

      if (opts.seriesID) {
        var seriesEl = document.createElement('div');
        seriesEl.style.cssText = 'font-size:12px;color:var(--accent);margin-bottom:8px;display:flex;align-items:center;gap:6px;cursor:pointer;';
        seriesEl.innerHTML = '\u{1F517} <span style="text-decoration:underline">View all episodes</span>' + (opts.episodeNum ? ' <span style="opacity:0.7">| Episode: ' + esc(opts.episodeNum) + '</span>' : '');
        seriesEl.title = 'Series: ' + opts.seriesID;
        seriesEl.onclick = function() { overlay.remove(); showSeriesModal(opts.seriesID); };
        body.appendChild(seriesEl);
      }

      if (opts.description) {
        var descEl = document.createElement('div');
        descEl.style.cssText = 'font-size:14px;color:var(--text);line-height:1.6;';
        descEl.textContent = opts.description;
        body.appendChild(descEl);
      }

      var footer = document.createElement('div');
      footer.style.cssText = 'padding:12px 24px 16px;border-top:1px solid var(--border);display:flex;gap:8px;justify-content:flex-end;';

      if (opts.channelID && opts.isLive) {
        var playBtn = document.createElement('button');
        playBtn.className = 'btn btn-primary';
        playBtn.style.cssText = 'gap:6px;';
        playBtn.innerHTML = icons.play + ' Watch';
        playBtn.onclick = function() {
          overlay.remove();
          var chID = opts.channelID;
          api.get('/api/channels').then(function(resp) { return resp.json(); }).then(function(chs) {
            if (!Array.isArray(chs)) return;
            for (var ci = 0; ci < chs.length; ci++) {
              if (chs[ci].id === chID && chs[ci].stream_ids && chs[ci].stream_ids.length > 0) {
                startPlay(chs[ci].stream_ids[0], opts.channelName, true);
                return;
              }
            }
            toast('No streams assigned to channel', 'error');
          }).catch(function() { toast('Failed to start playback', 'error'); });
        };
        footer.appendChild(playBtn);
      }

      if (opts.start && opts.stop && opts.channelID && !opts.isPast) {
        var recBtn = document.createElement('button');
        recBtn.className = 'btn btn-ghost';
        recBtn.style.cssText = 'color:var(--danger);gap:6px;';
        recBtn.textContent = '\u23FA Record';
        recBtn.onclick = function() {
          var body = { channel_id: opts.channelID, title: opts.title || '', start_at: opts.start, stop_at: opts.stop };
          api.post('/api/recordings/schedule', body).then(function(resp) {
            if (resp.ok) { toast('Recording scheduled'); overlay.remove(); }
            else { toast('Failed to schedule recording', 'error'); }
          }).catch(function() { toast('Failed to schedule recording', 'error'); });
        };
        footer.appendChild(recBtn);
      }

      modal.appendChild(body);

      if (footer.childNodes.length > 0) {
        modal.appendChild(footer);
      }

      overlay.appendChild(modal);
      document.body.appendChild(overlay);
    }

    function showSeriesModal(seriesID) {
      var sOvr = document.createElement('div');
      sOvr.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.7);z-index:9999;display:flex;align-items:center;justify-content:center;backdrop-filter:blur(4px);';
      sOvr.onclick = function(e) { if (e.target === sOvr) sOvr.remove(); };
      document.addEventListener('keydown', function onKey(e) { if (e.key === 'Escape') { sOvr.remove(); document.removeEventListener('keydown', onKey); } });
      var sMdl = document.createElement('div');
      sMdl.style.cssText = 'width:90%;max-width:640px;max-height:80vh;background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius-lg);overflow:hidden;position:relative;display:flex;flex-direction:column;';
      var sHdr = document.createElement('div');
      sHdr.style.cssText = 'padding:20px 24px 12px;border-bottom:1px solid var(--border);';
      var sCls = document.createElement('button');
      sCls.textContent = '✕';
      sCls.style.cssText = 'position:absolute;top:12px;right:12px;background:none;border:none;color:var(--text-dim);font-size:16px;cursor:pointer;padding:4px 8px;border-radius:4px;';
      sCls.onclick = function() { sOvr.remove(); };
      sHdr.appendChild(sCls);
      var sTtl = document.createElement('div');
      sTtl.style.cssText = 'font-size:18px;font-weight:700;color:#fff;padding-right:32px;';
      sTtl.textContent = 'Series Episodes';
      sHdr.appendChild(sTtl);
      sMdl.appendChild(sHdr);
      var sBdy = document.createElement('div');
      sBdy.style.cssText = 'padding:16px 24px 20px;overflow-y:auto;flex:1;';
      sBdy.innerHTML = '<div style="text-align:center;padding:20px;color:var(--text-muted)"><div class="spinner-ring"></div> Loading episodes...</div>';
      sMdl.appendChild(sBdy);
      sOvr.appendChild(sMdl);
      document.body.appendChild(sOvr);
      api.get('/api/epg/series?series_id=' + encodeURIComponent(seriesID)).then(function(resp) {
        return resp.json();
      }).then(function(episodes) {
        if (!Array.isArray(episodes) || episodes.length === 0) {
          sBdy.innerHTML = '<div style="text-align:center;padding:20px;color:var(--text-muted)">No episodes found for this series.</div>';
          return;
        }
        episodes.sort(function(a, b) { return new Date(a.start).getTime() - new Date(b.start).getTime(); });
        var sNow = Date.now();
        var chNames = guideData.channel_names || {};
        var sHtml = '<div style="font-size:13px;color:var(--text-muted);margin-bottom:12px;">' + episodes.length + ' episode' + (episodes.length !== 1 ? 's' : '') + ' found</div>';
        for (var si = 0; si < episodes.length; si++) {
          var ep = episodes[si];
          var epSt = new Date(ep.start).getTime();
          var epEn = new Date(ep.stop).getTime();
          var sLive = sNow >= epSt && sNow < epEn;
          var sPast = sNow >= epEn;
          var sFut = epSt > sNow;
          var sBdg = '';
          if (sLive) sBdg = '<span style="background:var(--danger);color:#fff;font-size:10px;font-weight:700;padding:1px 6px;border-radius:3px;margin-left:6px;">LIVE</span>';
          else if (sFut) sBdg = '<span style="color:var(--accent);font-size:11px;margin-left:6px;">Upcoming</span>';
          var eNum = ep.episode_num ? '<span style="color:var(--text-muted);font-size:11px;margin-left:4px;">(' + esc(ep.episode_num) + ')</span>' : '';
          var nBdg = ep.is_new ? '<span style="color:#22c55e;font-size:10px;font-weight:700;margin-left:4px;">NEW</span>' : '';
          var cNm = chNames[ep.channel_id] ? '<span style="color:var(--text-muted);font-size:11px;"> on ' + esc(chNames[ep.channel_id]) + '</span>' : '';
          var dStr = new Date(ep.start).toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
          var tStr = guideFormatTime(epSt) + ' - ' + guideFormatTime(epEn);
          sHtml += '<div style="padding:10px 0;border-bottom:1px solid var(--border);' + (sPast ? 'opacity:0.5;' : '') + '">' +
            '<div style="display:flex;align-items:center;gap:4px;flex-wrap:wrap;">' +
            '<span style="font-weight:600;color:#fff;">' + esc(ep.title) + '</span>' + eNum + nBdg + sBdg + '</div>' +
            (ep.subtitle ? '<div style="font-size:13px;color:var(--text-dim);margin-top:2px;">' + esc(ep.subtitle) + '</div>' : '') +
            '<div style="font-size:12px;color:var(--text-muted);margin-top:4px;">' + esc(dStr) + ' ' + esc(tStr) + cNm + '</div>' +
            (ep.description ? '<div style="font-size:12px;color:var(--text-dim);margin-top:4px;line-height:1.4;">' + esc(ep.description.length > 150 ? ep.description.substring(0, 150) + '...' : ep.description) + '</div>' : '') +
            '</div>';
        }
        sBdy.innerHTML = sHtml;
      }).catch(function(err) {
        sBdy.innerHTML = '<div style="text-align:center;padding:20px;color:var(--danger)">Failed to load episodes: ' + esc(err.message) + '</div>';
      });
    }

    if (channels.length === 0) {
      el.innerHTML = '<div class="epg-empty">No channels configured. Add channels first.</div>';
      return;
    }

    renderFull();
  }

  async function renderClients(el) {
    el.innerHTML = '<h1 class="page-title">Client Profiles</h1>' +
      '<div style="margin-bottom:16px"><button class="btn btn-primary" id="add-client-btn">' + icons.plus + ' Add Profile</button></div>' +
      '<div id="client-list"><div class="skeleton" style="height:200px"></div></div>';

    var addBtn = document.getElementById('add-client-btn');
    addBtn.addEventListener('click', function() {
      showClientModal(null);
    });

    function deliveryLabel(v) { return ({ stream: 'Stream', mse: 'MSE', hls: 'HLS' })[v] || v || 'Stream'; }
    function videoLabel(v) { return ({ 'default': 'Copy', copy: 'Copy', h264: 'H.264', h265: 'H.265', hvc1: 'H.265', hev1: 'H.265', av1: 'AV1' })[v] || v || 'Copy'; }
    function audioLabel(v) { return ({ copy: 'Copy', aac: 'AAC', ac3: 'AC3', mp3: 'MP3', opus: 'Opus', mp2: 'MP2' })[v] || v || 'AAC'; }
    function containerLabel(v) { return ({ mp4: 'MP4', mpegts: 'MPEG-TS', matroska: 'MKV', webm: 'WebM' })[v] || v || 'MP4'; }

    async function loadClients() {
      try {
        var resp = await api.get('/api/clients');
        var profiles = await resp.json();
        if (!Array.isArray(profiles)) profiles = [];
        for (var fi = 0; fi < profiles.length; fi++) {
          var prof = profiles[fi].profile;
          if (prof) {
            if (!profiles[fi].delivery) profiles[fi].delivery = prof.delivery || '';
            if (!profiles[fi].video_codec) profiles[fi].video_codec = prof.video_codec || '';
            if (!profiles[fi].audio_codec) profiles[fi].audio_codec = prof.audio_codec || '';
            if (!profiles[fi].container) profiles[fi].container = prof.container || '';
            if (profiles[fi].output_height == null) profiles[fi].output_height = prof.output_height || 0;
          }
        }
        var container = document.getElementById('client-list');
        if (!container) return;

        if (profiles.length === 0) {
          container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No client profiles configured</p></div>';
          return;
        }

        profiles.sort(function(a, b) {
          if (a.is_system && !b.is_system) return -1;
          if (!a.is_system && b.is_system) return 1;
          return (a.priority || 0) - (b.priority || 0);
        });

        var html = '<table class="list-table"><thead><tr>' +
          '<th>Name</th><th>Delivery</th><th>Video</th><th>Audio</th><th>Container</th><th>Height</th><th>Type</th><th></th>' +
          '</tr></thead><tbody>';
        for (var i = 0; i < profiles.length; i++) {
          var p = profiles[i];
          var typeBadges = '';
          if (p.is_system) typeBadges += '<span class="badge badge-system">System</span> ';
          if (p.is_client) typeBadges += '<span class="badge badge-client">Client</span> ';

          var heightStr = p.output_height && p.output_height > 0 ? p.output_height + 'p' : 'Source';

          var actions = '';
          actions += '<button class="btn btn-sm btn-ghost client-edit-btn" data-id="' + esc(p.id) + '" title="Edit">' + icons.edit + '</button>';
          if (!p.is_system) {
            actions += '<button class="btn btn-sm btn-icon btn-danger client-del-btn" data-id="' + esc(p.id) + '" data-name="' + esc(p.name) + '" title="Delete">' + icons.trash + '</button>';
          }

          var matchStr = '';
          if (p.match_rules && p.match_rules.length > 0) {
            matchStr = '<div style="font-size:11px;color:var(--text-muted);margin-top:2px">' +
              p.match_rules.map(function(r) { return esc(r.header_name + ' ' + r.match_type + ' ' + r.match_value); }).join(', ') + '</div>';
          }

          html += '<tr>' +
            '<td>' + esc(p.name) + matchStr + '</td>' +
            '<td><span class="badge badge-delivery">' + deliveryLabel(p.delivery) + '</span></td>' +
            '<td>' + videoLabel(p.video_codec) + '</td>' +
            '<td>' + audioLabel(p.audio_codec) + '</td>' +
            '<td>' + containerLabel(p.container) + '</td>' +
            '<td>' + esc(heightStr) + '</td>' +
            '<td>' + typeBadges + '</td>' +
            '<td><div class="actions-cell">' + actions + '</div></td>' +
            '</tr>';
        }
        html += '</tbody></table>';
        container.innerHTML = html;

        container.querySelectorAll('.client-edit-btn').forEach(function(btn) {
          btn.addEventListener('click', function() {
            var id = this.getAttribute('data-id');
            var profile = profiles.find(function(p) { return p.id === id; });
            if (profile) showClientModal(profile);
          });
        });

        container.querySelectorAll('.client-del-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var id = this.getAttribute('data-id');
            var name = this.getAttribute('data-name');
            if (!confirm('Delete client profile "' + name + '"?')) return;
            try {
              var r = await api.del('/api/clients/' + id);
              if (r.ok || r.status === 204) {
                toast('Profile deleted');
                loadClients();
              } else {
                toast('Failed to delete profile', 'error');
              }
            } catch (err) {
              toast('Failed to delete profile', 'error');
            }
          });
        });
      } catch (e) {
        var container = document.getElementById('client-list');
        if (container) container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load client profiles</p></div>';
      }
    }

    function showClientModal(profile) {
      var existing = document.getElementById('client-modal');
      if (existing) existing.remove();

      var isEdit = !!profile;
      var p = profile || {};

      var html = '<div class="modal-overlay" id="client-modal">' +
        '<div class="modal-content" style="max-width:520px">' +
        '<div class="modal-header">' + (isEdit ? 'Edit' : 'New') + ' Client Profile</div>' +
        '<div class="modal-body">' +
        '<div class="form-group"><label class="form-label">Name</label>' +
        '<input class="form-input" id="cp-name" value="' + esc(p.name || '') + '" placeholder="Profile name"' + (p.is_client ? ' disabled' : '') + '></div>' +
        '<div class="form-group"><label class="form-label">Delivery</label>' +
        '<select class="form-input" id="cp-delivery">' +
        '<option value="stream"' + (p.delivery === 'stream' ? ' selected' : '') + '>Stream (direct)</option>' +
        '<option value="mse"' + (p.delivery === 'mse' ? ' selected' : '') + '>MSE (browser)</option>' +
        '<option value="hls"' + (p.delivery === 'hls' ? ' selected' : '') + '>HLS (segmented)</option>' +
        '<option value="dash"' + (p.delivery === 'dash' ? ' selected' : '') + '>DASH (adaptive)</option>' +
        '<option value="webrtc"' + (p.delivery === 'webrtc' ? ' selected' : '') + '>WebRTC (low latency)</option>' +
        '<option value="user"' + (p.delivery === 'user' ? ' selected' : '') + '>User Choice</option>' +
        '</select></div>' +
        '<div class="form-group"><label class="form-label">Video Codec</label>' +
        '<select class="form-input" id="cp-video">' +
        '<option value="default"' + (!p.video_codec || p.video_codec === 'default' || p.video_codec === 'copy' ? ' selected' : '') + '>Copy (match source)</option>' +
        '<option value="h264"' + (p.video_codec === 'h264' ? ' selected' : '') + '>H.264</option>' +
        '<option value="h265"' + (p.video_codec === 'h265' || p.video_codec === 'hvc1' ? ' selected' : '') + '>H.265 / HEVC</option>' +
        '<option value="av1"' + (p.video_codec === 'av1' ? ' selected' : '') + '>AV1</option>' +
        '</select></div>' +
        '<div class="form-group"><label class="form-label">Audio Codec</label>' +
        '<select class="form-input" id="cp-audio">' +
        '<option value="copy"' + (p.audio_codec === 'copy' ? ' selected' : '') + '>Copy (passthrough)</option>' +
        '<option value="aac"' + (!p.audio_codec || p.audio_codec === 'aac' ? ' selected' : '') + '>AAC</option>' +
        '<option value="ac3"' + (p.audio_codec === 'ac3' ? ' selected' : '') + '>AC3</option>' +
        '<option value="mp3"' + (p.audio_codec === 'mp3' ? ' selected' : '') + '>MP3</option>' +
        '<option value="opus"' + (p.audio_codec === 'opus' ? ' selected' : '') + '>Opus</option>' +
        '</select></div>' +
        '<div class="form-group"><label class="form-label">Container</label>' +
        '<select class="form-input" id="cp-container">' +
        '<option value="mp4"' + (!p.container || p.container === 'mp4' ? ' selected' : '') + '>MP4</option>' +
        '<option value="mpegts"' + (p.container === 'mpegts' ? ' selected' : '') + '>MPEG-TS</option>' +
        '<option value="matroska"' + (p.container === 'matroska' ? ' selected' : '') + '>Matroska</option>' +
        '<option value="webm"' + (p.container === 'webm' ? ' selected' : '') + '>WebM</option>' +
        '</select></div>' +
        '<div class="form-group"><label class="form-label">Max Output Height</label>' +
        '<select class="form-input" id="cp-height">' +
        '<option value="0"' + (!p.output_height ? ' selected' : '') + '>Source (no scaling)</option>' +
        '<option value="2160"' + (p.output_height === 2160 ? ' selected' : '') + '>4K (2160p)</option>' +
        '<option value="1080"' + (p.output_height === 1080 ? ' selected' : '') + '>1080p</option>' +
        '<option value="720"' + (p.output_height === 720 ? ' selected' : '') + '>720p</option>' +
        '<option value="480"' + (p.output_height === 480 ? ' selected' : '') + '>480p</option>' +
        '</select></div>';

      html += '<div class="form-group"><label class="form-label">Match Rules (all must match)</label>' +
          '<div id="cp-rules-container"></div>' +
          '<button class="btn btn-sm" id="cp-add-rule" type="button">+ Add Rule</button></div>';

      html += '</div>' +
        '<div class="modal-footer">' +
        '<button class="btn btn-ghost" id="cp-cancel">Cancel</button>' +
        '<button class="btn btn-primary" id="cp-save">' + (isEdit ? 'Update' : 'Create') + '</button>' +
        '</div></div></div>';

      document.body.insertAdjacentHTML('beforeend', html);

      var rules = (p.match_rules && p.match_rules.length > 0)
        ? p.match_rules.map(function(r) { return { header_name: r.header_name || '', match_type: r.match_type || 'contains', match_value: r.match_value || '' }; })
        : [{ header_name: '', match_type: 'contains', match_value: '' }];
      var rulesContainer = document.getElementById('cp-rules-container');

      function renderRuleRows() {
        rulesContainer.innerHTML = '';
        for (var ri = 0; ri < rules.length; ri++) {
          (function(idx) {
            var rule = rules[idx];
            var row = document.createElement('div');
            row.style.cssText = 'display:flex;gap:8px;align-items:center;margin-bottom:8px';

            var headerInp = document.createElement('input');
            headerInp.className = 'form-input';
            headerInp.type = 'text';
            headerInp.placeholder = 'User-Agent';
            headerInp.value = rule.header_name;
            headerInp.style.cssText = 'flex:1;min-width:120px';
            headerInp.addEventListener('input', function() { rules[idx].header_name = headerInp.value; });

            var typeSelect = document.createElement('select');
            typeSelect.className = 'form-input';
            typeSelect.style.width = '120px';
            var matchTypes = ['contains', 'equals', 'prefix', 'exists', 'regex'];
            for (var ti = 0; ti < matchTypes.length; ti++) {
              var opt = document.createElement('option');
              opt.value = matchTypes[ti];
              opt.textContent = matchTypes[ti];
              if (rule.match_type === matchTypes[ti]) opt.selected = true;
              typeSelect.appendChild(opt);
            }

            var valueInp = document.createElement('input');
            valueInp.className = 'form-input';
            valueInp.type = 'text';
            valueInp.placeholder = 'Match value';
            valueInp.value = rule.match_value;
            valueInp.style.cssText = 'flex:1;min-width:120px';
            if (rule.match_type === 'exists') valueInp.style.display = 'none';
            valueInp.addEventListener('input', function() { rules[idx].match_value = valueInp.value; });

            typeSelect.addEventListener('change', function() {
              rules[idx].match_type = typeSelect.value;
              valueInp.style.display = typeSelect.value === 'exists' ? 'none' : '';
            });

            var removeBtn = document.createElement('button');
            removeBtn.className = 'btn btn-sm btn-danger';
            removeBtn.textContent = '\u2715';
            removeBtn.addEventListener('click', function() {
              rules.splice(idx, 1);
              if (rules.length === 0) rules.push({ header_name: '', match_type: 'contains', match_value: '' });
              renderRuleRows();
            });

            row.appendChild(headerInp);
            row.appendChild(typeSelect);
            row.appendChild(valueInp);
            row.appendChild(removeBtn);
            rulesContainer.appendChild(row);
          })(ri);
        }
      }
      renderRuleRows();

      document.getElementById('cp-add-rule').addEventListener('click', function() {
        rules.push({ header_name: '', match_type: 'contains', match_value: '' });
        renderRuleRows();
      });

      document.getElementById('cp-cancel').addEventListener('click', function() {
        document.getElementById('client-modal').remove();
      });
      document.getElementById('client-modal').addEventListener('click', function(e) {
        if (e.target === this) this.remove();
      });
      document.getElementById('cp-save').addEventListener('click', async function() {
        var matchRules = rules.map(function(r) {
          return { header_name: r.header_name, match_type: r.match_type, match_value: r.match_type === 'exists' ? '' : r.match_value };
        });
        var payload = {
          name: document.getElementById('cp-name').value.trim(),
          profile: {
            delivery: document.getElementById('cp-delivery').value,
            video_codec: document.getElementById('cp-video').value,
            audio_codec: document.getElementById('cp-audio').value,
            container: document.getElementById('cp-container').value,
            output_height: parseInt(document.getElementById('cp-height').value) || 0
          },
          match_rules: matchRules
        };
        if (!payload.name) { toast('Name required', 'error'); return; }
        try {
          var r;
          if (isEdit) {
            r = await api.put('/api/clients/' + p.id, payload);
          } else {
            r = await api.post('/api/clients', payload);
          }
          if (r.ok) {
            toast(isEdit ? 'Profile updated' : 'Profile created');
            document.getElementById('client-modal').remove();
            loadClients();
          } else {
            var data = await r.json().catch(function() { return {}; });
            toast(data.error || 'Failed to save profile', 'error');
          }
        } catch (err) {
          toast('Failed to save profile', 'error');
        }
      });
    }

    await loadClients();
  }

  async function renderProbe(el) {
    el.innerHTML = '<h1 class="page-title">Stream Probe</h1>' +
      '<div class="card">' +
      '<div class="card-title">Probe a Stream URL</div>' +
      '<div style="display:flex;gap:8px;margin-bottom:16px">' +
      '<input class="form-input" id="probe-url" placeholder="http://example.com/stream.m3u8 or /path/to/file.mp4" style="flex:1">' +
      '<button class="btn btn-primary" id="probe-btn">Probe</button>' +
      '</div>' +
      '<div id="probe-result"></div>' +
      '</div>';

    document.getElementById('probe-btn').addEventListener('click', doProbe);
    document.getElementById('probe-url').addEventListener('keydown', function(e) {
      if (e.key === 'Enter') doProbe();
    });

    async function doProbe() {
      var url = document.getElementById('probe-url').value.trim();
      if (!url) { toast('Enter a URL to probe', 'error'); return; }

      var resultEl = document.getElementById('probe-result');
      resultEl.innerHTML = '<div style="display:flex;align-items:center;gap:12px;padding:20px;color:var(--text-dim)">' +
        '<div class="spinner-ring" style="width:24px;height:24px;border-width:3px"></div>' +
        '<span>Probing stream...</span></div>';

      try {
        var resp = await api.post('/api/probe', { url: url });
        if (!resp.ok) {
          var errData = await resp.json().catch(function() { return {}; });
          resultEl.innerHTML = '<div style="padding:16px;color:var(--danger)">' +
            'Probe failed: ' + esc(errData.error || 'HTTP ' + resp.status) + '</div>';
          return;
        }
        var data = await resp.json();
        renderProbeResult(resultEl, data, url);
      } catch (e) {
        resultEl.innerHTML = '<div style="padding:16px;color:var(--danger)">Probe failed: ' + esc(e.message) + '</div>';
      }
    }

    function renderProbeResult(container, data, url) {
      var html = '<div style="display:flex;gap:12px;margin-bottom:16px;flex-wrap:wrap">';

      if (data.duration && data.duration > 0) {
        html += '<div class="stat-card" style="flex:1;min-width:120px;padding:12px">' +
          '<div class="stat-value" style="font-size:18px">' + formatDurationSec(data.duration) + '</div>' +
          '<div class="stat-label">Duration</div></div>';
      }
      if (data.format) {
        html += '<div class="stat-card" style="flex:1;min-width:120px;padding:12px">' +
          '<div class="stat-value" style="font-size:18px">' + esc(data.format) + '</div>' +
          '<div class="stat-label">Format</div></div>';
      }
      if (data.bitrate) {
        html += '<div class="stat-card" style="flex:1;min-width:120px;padding:12px">' +
          '<div class="stat-value" style="font-size:18px">' + (data.bitrate / 1000).toFixed(0) + ' kbps</div>' +
          '<div class="stat-label">Bitrate</div></div>';
      }
      html += '</div>';

      var videoTracks = (data.streams || []).filter(function(s) { return s.type === 'video'; });
      var audioTracks = (data.streams || []).filter(function(s) { return s.type === 'audio'; });
      var subTracks = (data.streams || []).filter(function(s) { return s.type === 'subtitle'; });

      if (videoTracks.length > 0) {
        html += '<div class="card" style="margin-bottom:12px">' +
          '<div class="card-title" style="display:flex;align-items:center;gap:8px">' + icons.video + ' Video Tracks</div>';
        for (var vi = 0; vi < videoTracks.length; vi++) {
          var v = videoTracks[vi];
          var props = [];
          if (v.codec) props.push('<span class="badge badge-delivery">' + esc(v.codec.toUpperCase()) + '</span>');
          if (v.width && v.height) props.push(v.width + 'x' + v.height);
          if (v.fps) props.push(v.fps.toFixed(1) + ' fps');
          if (v.bit_depth) props.push(v.bit_depth + '-bit');
          if (v.profile) props.push(v.profile);
          if (v.pixel_format) props.push(v.pixel_format);
          html += '<div style="display:flex;align-items:center;gap:8px;padding:8px 0;border-bottom:1px solid var(--border)">' +
            '<span style="color:var(--text-muted);font-size:12px;min-width:24px">#' + (v.index || vi) + '</span>' +
            '<div>' + props.join(' <span style="color:var(--text-muted)">&bull;</span> ') + '</div></div>';
        }
        html += '</div>';
      }

      if (audioTracks.length > 0) {
        html += '<div class="card" style="margin-bottom:12px">' +
          '<div class="card-title" style="display:flex;align-items:center;gap:8px">' + icons.audio + ' Audio Tracks</div>';
        for (var ai = 0; ai < audioTracks.length; ai++) {
          var a = audioTracks[ai];
          var aProps = [];
          if (a.codec) aProps.push('<span class="badge badge-delivery">' + esc(a.codec.toUpperCase()) + '</span>');
          if (a.channels) aProps.push(a.channels + 'ch');
          if (a.sample_rate) aProps.push(a.sample_rate + ' Hz');
          if (a.language) aProps.push('<span style="color:var(--accent)">' + esc(a.language) + '</span>');
          if (a.title) aProps.push(esc(a.title));
          html += '<div style="display:flex;align-items:center;gap:8px;padding:8px 0;border-bottom:1px solid var(--border)">' +
            '<span style="color:var(--text-muted);font-size:12px;min-width:24px">#' + (a.index || ai) + '</span>' +
            '<div>' + aProps.join(' <span style="color:var(--text-muted)">&bull;</span> ') + '</div></div>';
        }
        html += '</div>';
      }

      if (subTracks.length > 0) {
        html += '<div class="card" style="margin-bottom:12px">' +
          '<div class="card-title" style="display:flex;align-items:center;gap:8px">' + icons.subtitle + ' Subtitle Tracks</div>';
        for (var si = 0; si < subTracks.length; si++) {
          var s = subTracks[si];
          var sProps = [];
          if (s.codec) sProps.push(esc(s.codec));
          if (s.language) sProps.push('<span style="color:var(--accent)">' + esc(s.language) + '</span>');
          if (s.title) sProps.push(esc(s.title));
          html += '<div style="display:flex;align-items:center;gap:8px;padding:8px 0;border-bottom:1px solid var(--border)">' +
            '<span style="color:var(--text-muted);font-size:12px;min-width:24px">#' + (s.index || si) + '</span>' +
            '<div>' + sProps.join(' <span style="color:var(--text-muted)">&bull;</span> ') + '</div></div>';
        }
        html += '</div>';
      }

      if (videoTracks.length === 0 && audioTracks.length === 0) {
        html += '<div style="padding:16px;color:var(--text-muted)">No media tracks found in stream.</div>';
      }

      html += '<div style="margin-top:12px">' +
        '<button class="btn btn-ghost" id="probe-copy-url" style="gap:6px">' + icons.copy + ' Copy URL</button>' +
        '</div>';

      container.innerHTML = html;

      var copyBtn = document.getElementById('probe-copy-url');
      if (copyBtn) {
        copyBtn.addEventListener('click', function() {
          if (navigator.clipboard) {
            navigator.clipboard.writeText(url).then(function() { toast('URL copied'); }).catch(function() { toast('Failed to copy', 'error'); });
          } else {
            toast('Clipboard not available', 'error');
          }
        });
      }
    }
  }

  async function renderLogos(el) {
    el.innerHTML = '<h1 class="page-title">Channel Logos</h1>' +
      '<div style="display:flex;gap:8px;margin-bottom:16px">' +
      '<button class="btn btn-primary" id="refresh-logos-btn">' + icons.refresh + ' Refresh from EPG</button>' +
      '</div>' +
      '<div class="search-bar">' + icons.search + '<input id="logo-search" placeholder="Search channels..."></div>' +
      '<div id="logo-list"><div class="skeleton" style="height:400px"></div></div>';

    var refreshBtn = document.getElementById('refresh-logos-btn');
    if (refreshBtn) {
      refreshBtn.addEventListener('click', async function() {
        refreshBtn.disabled = true;
        try {
          var r = await api.post('/api/logos/refresh-from-epg');
          var data = await r.json();
          toast('Updated ' + (data.updated || 0) + ' channel logos');
          renderLogos(el);
        } catch (err) {
          toast('Failed to refresh logos', 'error');
        }
        refreshBtn.disabled = false;
      });
    }

    try {
      var resp = await api.get('/api/logos');
      var logos = await resp.json();
      if (!Array.isArray(logos)) logos = [];

      function renderLogoGrid(filter) {
        var container = document.getElementById('logo-list');
        if (!container) return;
        var filtered = logos;
        if (filter) {
          filtered = logos.filter(function(l) {
            return (l.channel_name || '').toLowerCase().indexOf(filter) >= 0;
          });
        }
        if (filtered.length === 0) {
          container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No channels found</p></div>';
          return;
        }
        filtered.sort(function(a, b) { return (a.number || 0) - (b.number || 0); });
        var html = '<table class="list-table"><thead><tr>' +
          '<th style="width:60px">Logo</th><th>#</th><th>Channel</th><th>Source</th><th>TVG ID</th><th style="width:120px">Actions</th>' +
          '</tr></thead><tbody>';
        for (var i = 0; i < filtered.length; i++) {
          var l = filtered[i];
          var logoImg = l.logo_url
            ? '<img src="' + esc(l.logo_url) + '" style="width:40px;height:40px;object-fit:contain;border-radius:4px;background:var(--bg-hover)" alt="" onerror="this.style.display=\'none\'">'
            : '<div style="width:40px;height:40px;background:var(--bg-hover);border-radius:4px;display:flex;align-items:center;justify-content:center;color:var(--text-muted);font-size:10px">none</div>';
          var sourceBadge = '<span class="badge badge-' + (l.source === 'epg' ? 'enabled' : l.source === 'manual' ? 'info' : 'disabled') + '">' + esc(l.source) + '</span>';
          html += '<tr>' +
            '<td>' + logoImg + '</td>' +
            '<td>' + esc(l.number || '-') + '</td>' +
            '<td>' + esc(l.channel_name) + '</td>' +
            '<td>' + sourceBadge + '</td>' +
            '<td style="font-size:12px;color:var(--text-muted)">' + esc(l.tvg_id || '-') + '</td>' +
            '<td><button class="btn btn-sm btn-ghost logo-edit-btn" data-id="' + esc(l.channel_id) + '" data-name="' + esc(l.channel_name) + '" data-url="' + esc(l.logo_url || '') + '">' + icons.edit + '</button></td>' +
            '</tr>';
        }
        html += '</tbody></table>';
        container.innerHTML = html;

        container.querySelectorAll('.logo-edit-btn').forEach(function(btn) {
          btn.addEventListener('click', function() {
            var channelId = this.getAttribute('data-id');
            var channelName = this.getAttribute('data-name');
            var currentUrl = this.getAttribute('data-url');
            var newUrl = prompt('Logo URL for "' + channelName + '":', currentUrl);
            if (newUrl === null) return;
            api.put('/api/logos/' + channelId, { logo_url: newUrl }).then(function(r) {
              if (r.ok) {
                toast('Logo updated');
                renderLogos(el);
              } else {
                r.json().then(function(d) { toast(d.error || 'Failed', 'error'); });
              }
            }).catch(function() { toast('Failed to update logo', 'error'); });
          });
        });
      }

      renderLogoGrid('');
      document.getElementById('logo-search').addEventListener('input', function() {
        renderLogoGrid(this.value.toLowerCase());
      });
    } catch (e) {
      document.getElementById('logo-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load logos</p></div>';
    }
  }

  async function renderTMDBPage(el) {
    el.innerHTML = '<h1 class="page-title">TMDB Metadata</h1>' +
      '<div id="tmdb-content"><div class="skeleton" style="height:400px"></div></div>';

    var container = document.getElementById('tmdb-content');
    if (!container) return;

    try {
      var results = await Promise.all([
        api.get('/api/tmdb/queue').then(function(r) { return r.json(); }),
        api.get('/api/tmdb/sync').then(function(r) { return r.json(); }),
        api.get('/api/tmdb/recent').then(function(r) { return r.json(); })
      ]);
      var queue = results[0] || {};
      var sync = results[1] || {};
      var recent = results[2] || [];

      var html = '';

      html += '<div class="settings-section">' +
        '<div class="settings-section-header">Queue Status</div>' +
        '<div class="settings-section-body">' +
        '<div style="display:flex;gap:24px;flex-wrap:wrap">' +
        '<div class="stat-card" style="flex:1;min-width:120px"><div class="stat-value">' + (queue.metadata || 0) + '</div><div class="stat-label">Metadata Queue</div></div>' +
        '<div class="stat-card" style="flex:1;min-width:120px"><div class="stat-value">' + (queue.images || 0) + '</div><div class="stat-label">Image Queue</div></div>';

      if (sync.syncing) {
        html += '<div class="stat-card" style="flex:1;min-width:120px"><div class="stat-value">' + (sync.completed || 0) + ' / ' + (sync.total || 0) + '</div><div class="stat-label">Sync Progress</div></div>';
      }

      html += '</div>' +
        '<div style="margin-top:16px">' +
        '<button class="btn btn-danger" id="tmdb-resync-btn">' + icons.refresh + ' Re-sync All</button>' +
        '<span class="field-hint" style="margin-left:8px">Clears all cached metadata and images, then re-enqueues all streams with TMDB IDs for resolution.</span>' +
        '</div>' +
        '</div></div>';

      html += '<div class="settings-section">' +
        '<div class="settings-section-header">Recently Resolved</div>' +
        '<div class="settings-section-body">';

      if (!Array.isArray(recent) || recent.length === 0) {
        html += '<div style="color:var(--text-muted);font-size:13px">No recently resolved items.</div>';
      } else {
        html += '<table class="list-table"><thead><tr>' +
          '<th>Title</th><th>Type</th><th>TMDB ID</th>' +
          '</tr></thead><tbody>';
        for (var i = 0; i < recent.length; i++) {
          var item = recent[i];
          var typeBadge = '<span class="badge badge-' + (item.media_type === 'movie' ? 'enabled' : 'info') + '">' + esc(item.media_type) + '</span>';
          html += '<tr>' +
            '<td>' + esc(item.title || 'Unknown') + '</td>' +
            '<td>' + typeBadge + '</td>' +
            '<td><a href="https://www.themoviedb.org/' + (item.media_type === 'series' ? 'tv' : 'movie') + '/' + item.tmdb_id + '" target="_blank" style="color:var(--accent)">' + item.tmdb_id + '</a></td>' +
            '</tr>';
        }
        html += '</tbody></table>';
      }

      html += '</div></div>';

      container.innerHTML = html;

      var resyncBtn = document.getElementById('tmdb-resync-btn');
      if (resyncBtn) {
        resyncBtn.addEventListener('click', async function() {
          if (!confirm('This will clear all cached TMDB metadata and images, then re-fetch everything. This may take a while. Continue?')) return;
          resyncBtn.disabled = true;
          try {
            var r = await api.post('/api/tmdb/resync');
            var data = await r.json();
            toast('Re-sync started: ' + (data.enqueued || 0) + ' items enqueued');
            setTimeout(function() { renderTMDBPage(el); }, 2000);
          } catch (err) {
            toast('Failed to start re-sync', 'error');
          }
          resyncBtn.disabled = false;
        });
      }
    } catch (e) {
      container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load TMDB status</p></div>';
    }
  }

  async function renderHDHRDevices(el) {
    el.innerHTML = '<h1 class="page-title">HDHR Devices</h1>' +
      '<div style="margin-bottom:16px;display:flex;gap:8px">' +
      '<button class="btn btn-primary" id="add-hdhr-btn">' + icons.plus + ' Add Device</button>' +
      '<button class="btn btn-ghost" id="autosplit-hdhr-btn">Auto-Split</button>' +
      '</div>' +
      '<div id="hdhr-list"><div class="skeleton" style="height:200px"></div></div>';

    var channelGroups = [];
    try {
      var gResp = await api.get('/api/channel-groups');
      channelGroups = await gResp.json();
      if (!Array.isArray(channelGroups)) channelGroups = [];
    } catch (e) {}

    document.getElementById('add-hdhr-btn').addEventListener('click', function() {
      showHDHRModal(null);
    });

    document.getElementById('autosplit-hdhr-btn').addEventListener('click', async function() {
      try {
        var r = await api.post('/api/hdhr/devices/auto-split');
        if (r.ok) {
          var data = await r.json().catch(function() { return {}; });
          toast(data.message || 'Auto-split complete');
          loadHDHRDevices();
        } else {
          var err = await r.json().catch(function() { return {}; });
          toast(err.error || 'Auto-split failed', 'error');
        }
      } catch (e) {
        toast('Auto-split failed', 'error');
      }
    });

    async function loadHDHRDevices() {
      try {
        var resp = await api.get('/api/hdhr/devices');
        var devices = await resp.json();
        if (!Array.isArray(devices)) devices = [];
        var container = document.getElementById('hdhr-list');
        if (!container) return;

        if (devices.length === 0) {
          container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No HDHR devices configured</p></div>';
          return;
        }

        var html = '<table class="list-table"><thead><tr>' +
          '<th>Name</th><th>Port</th><th>Groups</th><th>Max Channels</th><th>Enabled</th><th></th>' +
          '</tr></thead><tbody>';
        for (var i = 0; i < devices.length; i++) {
          var d = devices[i];
          var groupNames = [];
          if (d.group_ids && d.group_ids.length > 0) {
            for (var gi = 0; gi < d.group_ids.length; gi++) {
              var grp = channelGroups.find(function(g) { return g.id === d.group_ids[gi]; });
              groupNames.push(grp ? grp.name : d.group_ids[gi]);
            }
          }
          html += '<tr>' +
            '<td>' + esc(d.name) + '</td>' +
            '<td>' + esc(d.port) + '</td>' +
            '<td>' + (groupNames.length > 0 ? groupNames.map(function(n) { return '<span class="badge">' + esc(n) + '</span>'; }).join(' ') : '<span style="color:var(--text-muted)">-</span>') + '</td>' +
            '<td>' + esc(d.max_channels || 0) + '</td>' +
            '<td>' + (d.is_enabled ? '<span class="badge badge-admin">Yes</span>' : '<span class="badge">No</span>') + '</td>' +
            '<td><div class="actions-cell">' +
            '<button class="btn btn-sm btn-ghost hdhr-edit-btn" data-id="' + esc(d.id) + '" title="Edit">' + icons.edit + '</button>' +
            '<button class="btn btn-sm btn-icon btn-danger hdhr-del-btn" data-id="' + esc(d.id) + '" data-name="' + esc(d.name) + '" title="Delete">' + icons.trash + '</button>' +
            '</div></td></tr>';
        }
        html += '</tbody></table>';
        container.innerHTML = html;

        container.querySelectorAll('.hdhr-edit-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var id = this.getAttribute('data-id');
            try {
              var r = await api.get('/api/hdhr/devices/' + id);
              var device = await r.json();
              showHDHRModal(device);
            } catch (e) {
              toast('Failed to load device', 'error');
            }
          });
        });

        container.querySelectorAll('.hdhr-del-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var id = this.getAttribute('data-id');
            var name = this.getAttribute('data-name');
            if (!confirm('Delete HDHR device "' + name + '"?')) return;
            try {
              var r = await api.del('/api/hdhr/devices/' + id);
              if (r.ok || r.status === 204) {
                toast('Device deleted');
                loadHDHRDevices();
              } else {
                toast('Failed to delete device', 'error');
              }
            } catch (e) {
              toast('Failed to delete device', 'error');
            }
          });
        });
      } catch (e) {
        var container = document.getElementById('hdhr-list');
        if (container) container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load HDHR devices</p></div>';
      }
    }

    function showHDHRModal(device) {
      var existing = document.getElementById('hdhr-modal');
      if (existing) existing.remove();

      var isEdit = !!device;
      var d = device || {};
      var selectedGroupIds = d.group_ids || [];

      var groupCheckboxes = '';
      for (var gi = 0; gi < channelGroups.length; gi++) {
        var g = channelGroups[gi];
        var checked = selectedGroupIds.indexOf(g.id) >= 0 ? ' checked' : '';
        groupCheckboxes += '<label style="display:flex;align-items:center;gap:6px;margin-bottom:4px;cursor:pointer">' +
          '<input type="checkbox" class="hdhr-group-cb" value="' + esc(g.id) + '"' + checked + '> ' + esc(g.name) + '</label>';
      }
      if (channelGroups.length === 0) {
        groupCheckboxes = '<span style="color:var(--text-muted)">No channel groups available</span>';
      }

      var html = '<div class="modal-overlay" id="hdhr-modal">' +
        '<div class="modal-content" style="max-width:480px">' +
        '<div class="modal-header">' + (isEdit ? 'Edit' : 'New') + ' HDHR Device</div>' +
        '<div class="modal-body">' +
        '<div class="form-group"><label class="form-label">Name</label>' +
        '<input class="form-input" id="hdhr-name" value="' + esc(d.name || '') + '" placeholder="Device name"></div>' +
        '<div class="form-group"><label class="form-label">Port</label>' +
        '<input class="form-input" id="hdhr-port" type="number" value="' + esc(d.port || '') + '" placeholder="65001"></div>' +
        '<div class="form-group"><label class="form-label">Max Channels</label>' +
        '<input class="form-input" id="hdhr-max" type="number" value="' + esc(d.max_channels || '') + '" placeholder="480"></div>' +
        '<div class="form-group"><label class="form-label">Channel Groups</label>' +
        '<div id="hdhr-groups" style="max-height:150px;overflow-y:auto">' + groupCheckboxes + '</div></div>' +
        '<div class="form-group"><label style="display:flex;align-items:center;gap:8px;cursor:pointer">' +
        '<input type="checkbox" id="hdhr-enabled"' + (d.is_enabled !== false ? ' checked' : '') + '> Enabled</label></div>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button class="btn btn-ghost" id="hdhr-cancel">Cancel</button>' +
        '<button class="btn btn-primary" id="hdhr-save">' + (isEdit ? 'Update' : 'Create') + '</button>' +
        '</div></div></div>';

      document.body.insertAdjacentHTML('beforeend', html);
      document.getElementById('hdhr-name').focus();

      document.getElementById('hdhr-cancel').addEventListener('click', function() {
        document.getElementById('hdhr-modal').remove();
      });
      document.getElementById('hdhr-modal').addEventListener('click', function(e) {
        if (e.target === this) this.remove();
      });
      document.getElementById('hdhr-save').addEventListener('click', async function() {
        var name = document.getElementById('hdhr-name').value.trim();
        var port = parseInt(document.getElementById('hdhr-port').value) || 0;
        var maxChannels = parseInt(document.getElementById('hdhr-max').value) || 0;
        var enabled = document.getElementById('hdhr-enabled').checked;
        var groupIds = [];
        document.querySelectorAll('.hdhr-group-cb:checked').forEach(function(cb) {
          groupIds.push(cb.value);
        });
        if (!name) { toast('Name required', 'error'); return; }
        var payload = { name: name, port: port, group_ids: groupIds, max_channels: maxChannels, is_enabled: enabled };
        try {
          var r;
          if (isEdit) {
            r = await api.put('/api/hdhr/devices/' + d.id, payload);
          } else {
            r = await api.post('/api/hdhr/devices', payload);
          }
          if (r.ok) {
            toast(isEdit ? 'Device updated' : 'Device created');
            document.getElementById('hdhr-modal').remove();
            loadHDHRDevices();
          } else {
            var data = await r.json().catch(function() { return {}; });
            toast(data.error || 'Failed to save device', 'error');
          }
        } catch (err) {
          toast('Failed to save device', 'error');
        }
      });
    }

    await loadHDHRDevices();
  }

  async function renderInvites(el) {
    renderUsers(el);
  }

  async function renderAPIKeys(el) {
    el.innerHTML = '<h1 class="page-title">API Keys</h1>' +
      '<div style="margin-bottom:16px"><button class="btn btn-primary" id="add-apikey-btn">' + icons.plus + ' Create API Key</button></div>' +
      '<div id="apikey-list"><div class="skeleton" style="height:200px"></div></div>';

    document.getElementById('add-apikey-btn').addEventListener('click', function() {
      showAPIKeyModal();
    });

    async function loadAPIKeys() {
      try {
        var resp = await api.get('/api/auth/apikeys');
        var keys = await resp.json();
        if (!Array.isArray(keys)) keys = [];
        var container = document.getElementById('apikey-list');
        if (!container) return;

        if (keys.length === 0) {
          container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No API keys</p></div>';
          return;
        }

        var html = '<table class="list-table"><thead><tr>' +
          '<th>Name</th><th>Key</th><th>Created</th><th></th>' +
          '</tr></thead><tbody>';
        for (var i = 0; i < keys.length; i++) {
          var k = keys[i];
          var maskedKey = k.key_prefix ? k.key_prefix + '...' : (k.key ? k.key.substring(0, 8) + '...' : '********');
          var created = k.created_at ? new Date(k.created_at).toLocaleString() : '-';
          html += '<tr>' +
            '<td>' + esc(k.name) + '</td>' +
            '<td><span style="font-family:monospace;font-size:12px;color:var(--text-secondary)">' + esc(maskedKey) + '</span></td>' +
            '<td>' + esc(created) + '</td>' +
            '<td><div class="actions-cell">' +
            '<button class="btn btn-sm btn-icon btn-danger apikey-revoke-btn" data-id="' + esc(k.id) + '" data-name="' + esc(k.name) + '" title="Revoke">' + icons.trash + '</button>' +
            '</div></td></tr>';
        }
        html += '</tbody></table>';
        container.innerHTML = html;

        container.querySelectorAll('.apikey-revoke-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var id = this.getAttribute('data-id');
            var name = this.getAttribute('data-name');
            if (!confirm('Revoke API key "' + name + '"? This cannot be undone.')) return;
            try {
              var r = await api.del('/api/auth/apikey/' + id);
              if (r.ok || r.status === 204) {
                toast('API key revoked');
                loadAPIKeys();
              } else {
                toast('Failed to revoke API key', 'error');
              }
            } catch (e) {
              toast('Failed to revoke API key', 'error');
            }
          });
        });
      } catch (e) {
        var container = document.getElementById('apikey-list');
        if (container) container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load API keys</p></div>';
      }
    }

    function showAPIKeyModal() {
      var existing = document.getElementById('apikey-modal');
      if (existing) existing.remove();

      var html = '<div class="modal-overlay" id="apikey-modal">' +
        '<div class="modal-content" style="max-width:420px">' +
        '<div class="modal-header">Create API Key</div>' +
        '<div class="modal-body">' +
        '<div class="form-group"><label class="form-label">Name</label>' +
        '<input class="form-input" id="apikey-name" placeholder="Key name (e.g. CI integration)"></div>' +
        '<div id="apikey-result" style="display:none"></div>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button class="btn btn-ghost" id="apikey-cancel">Cancel</button>' +
        '<button class="btn btn-primary" id="apikey-create">Create</button>' +
        '</div></div></div>';

      document.body.insertAdjacentHTML('beforeend', html);
      document.getElementById('apikey-name').focus();

      document.getElementById('apikey-cancel').addEventListener('click', function() {
        document.getElementById('apikey-modal').remove();
      });
      document.getElementById('apikey-modal').addEventListener('click', function(e) {
        if (e.target === this) this.remove();
      });
      document.getElementById('apikey-create').addEventListener('click', async function() {
        var name = document.getElementById('apikey-name').value.trim();
        if (!name) { toast('Name required', 'error'); return; }
        try {
          var r = await api.post('/api/auth/apikey', { name: name });
          if (r.ok) {
            var data = await r.json();
            var resultEl = document.getElementById('apikey-result');
            resultEl.style.display = 'block';
            resultEl.innerHTML = '<div style="margin-top:12px;padding:12px;background:var(--surface-alt);border-radius:8px;border:1px solid var(--warning)">' +
              '<div style="color:var(--warning);font-weight:600;margin-bottom:8px">This key will not be shown again</div>' +
              '<div style="display:flex;gap:8px">' +
              '<input class="form-input" id="apikey-display" value="' + esc(data.key || data.api_key || '') + '" readonly style="font-family:monospace;font-size:12px">' +
              '<button class="btn btn-ghost" id="apikey-copy-btn" title="Copy">' + icons.copy + '</button>' +
              '</div></div>';
            document.getElementById('apikey-copy-btn').addEventListener('click', function() {
              var inp = document.getElementById('apikey-display');
              if (inp) {
                inp.select();
                try { navigator.clipboard.writeText(inp.value); toast('Copied to clipboard'); } catch (e) { toast('Select and copy manually', 'error'); }
              }
            });
            document.getElementById('apikey-create').style.display = 'none';
            document.getElementById('apikey-cancel').textContent = 'Close';
            document.getElementById('apikey-cancel').addEventListener('click', function() {
              document.getElementById('apikey-modal').remove();
              loadAPIKeys();
            });
          } else {
            var err = await r.json().catch(function() { return {}; });
            toast(err.error || 'Failed to create API key', 'error');
          }
        } catch (e) {
          toast('Failed to create API key', 'error');
        }
      });
    }

    await loadAPIKeys();
  }

  async function renderPlayURL(el) {
    el.innerHTML = '<h1 class="page-title">Play URL</h1>' +
      '<div class="card">' +
      '<div class="card-title">Play a Stream URL</div>' +
      '<p style="font-size:13px;color:var(--text-muted);margin-bottom:12px">Paste any media URL to detect its codec and play it. Probes first, then starts playback.</p>' +
      '<div style="display:flex;gap:8px;margin-bottom:12px">' +
      '<input class="form-input" id="playurl-input" placeholder="http://example.com/stream.m3u8 or rtsp://..." style="flex:1">' +
      '<button class="btn btn-ghost" id="playurl-probe-btn">' + icons.probe + ' Probe</button>' +
      '<button class="btn btn-primary" id="playurl-btn">' + icons.play + ' Play</button>' +
      '</div>' +
      '<div id="playurl-probe-result"></div>' +
      '<div id="playurl-status"></div>' +
      '</div>';

    var playUrlInput = document.getElementById('playurl-input');
    var playUrlBtn = document.getElementById('playurl-btn');
    var playUrlProbeBtn = document.getElementById('playurl-probe-btn');
    var playUrlStatus = document.getElementById('playurl-status');
    var playUrlProbeResult = document.getElementById('playurl-probe-result');

    async function doProbe() {
      var url = playUrlInput.value.trim();
      if (!url) { toast('Enter a URL to probe', 'error'); return; }
      playUrlProbeResult.innerHTML = '<div style="display:flex;align-items:center;gap:12px;padding:16px;color:var(--text-dim)">' +
        '<div class="spinner-ring" style="width:20px;height:20px;border-width:2px"></div>' +
        '<span>Probing stream...</span></div>';
      playUrlStatus.innerHTML = '';
      try {
        var resp = await api.post('/api/probe', { url: url });
        if (!resp.ok) {
          var errData = await resp.json().catch(function() { return {}; });
          playUrlProbeResult.innerHTML = '<div style="padding:12px;color:var(--danger);font-size:13px">Probe failed: ' + esc(errData.error || 'HTTP ' + resp.status) + '</div>';
          return;
        }
        var data = await resp.json();
        var html = '<div style="background:var(--bg-input);border:1px solid var(--border);border-radius:var(--radius);padding:12px;margin-bottom:12px">';
        html += '<div style="font-weight:600;margin-bottom:8px;font-size:13px">Detected Streams</div>';
        html += '<div style="display:flex;flex-wrap:wrap;gap:6px;margin-bottom:8px">';
        if (data.video) {
          html += '<span class="badge badge-delivery" style="font-size:12px;padding:2px 8px">' + esc((data.video.codec || '').toUpperCase()) + '</span>';
          if (data.video.width && data.video.height) html += '<span class="badge" style="background:rgba(52,211,153,.1);color:var(--success);font-size:12px;padding:2px 8px;border-radius:4px">' + data.video.width + 'x' + data.video.height + '</span>';
          if (data.video.fps) html += '<span class="badge" style="background:var(--bg-hover);color:var(--text);font-size:12px;padding:2px 8px;border-radius:4px">' + data.video.fps.toFixed(1) + ' fps</span>';
          if (data.video.bit_depth && data.video.bit_depth > 8) html += '<span class="badge" style="background:rgba(251,191,36,.15);color:var(--warning);font-size:12px;padding:2px 8px;border-radius:4px">' + data.video.bit_depth + '-bit</span>';
        }
        if (data.audio && data.audio.length > 0) {
          for (var ai = 0; ai < data.audio.length; ai++) {
            var a = data.audio[ai];
            html += '<span class="badge badge-delivery" style="font-size:12px;padding:2px 8px">' + esc((a.codec || '').toUpperCase()) + '</span>';
            if (a.channels) html += '<span class="badge" style="background:var(--bg-hover);color:var(--text);font-size:12px;padding:2px 8px;border-radius:4px">' + a.channels + 'ch</span>';
            if (a.language) html += '<span class="badge" style="background:rgba(91,110,239,.1);color:var(--accent);font-size:12px;padding:2px 8px;border-radius:4px">' + esc(a.language) + '</span>';
          }
        }
        html += '</div>';
        if (data.duration_ms > 0) {
          html += '<div style="font-size:12px;color:var(--text-muted)">Duration: ' + formatDurationSec(data.duration_ms / 1000) + '</div>';
        }
        html += '</div>';
        playUrlProbeResult.innerHTML = html;
      } catch (e) {
        playUrlProbeResult.innerHTML = '<div style="padding:12px;color:var(--danger);font-size:13px">Probe failed: ' + esc(e.message) + '</div>';
      }
    }

    async function doPlay() {
      var url = playUrlInput.value.trim();
      if (!url) { toast('Enter a URL to play', 'error'); return; }
      playUrlStatus.innerHTML = '<div style="display:flex;align-items:center;gap:12px;padding:20px;color:var(--text-dim)">' +
        '<div class="spinner-ring" style="width:24px;height:24px;border-width:3px"></div>' +
        '<span>Starting playback...</span></div>';
      try {
        var resp = await api.post('/api/play/url', { url: url });
        if (!resp.ok) {
          var errData = await resp.json().catch(function() { return {}; });
          playUrlStatus.innerHTML = '<div style="padding:16px;color:var(--danger)">' + esc(errData.error || 'Failed to start playback') + '</div>';
          return;
        }
        var data = await resp.json();
        var decisionHtml = '';
        if (data.decision) {
          decisionHtml = '<div style="display:flex;flex-wrap:wrap;gap:6px;margin-bottom:12px">';
          decisionHtml += '<span class="badge" style="' + (data.decision.needs_transcode ? 'background:rgba(251,191,36,.15);color:var(--warning)' : 'background:rgba(52,211,153,.15);color:var(--success)') + ';font-size:12px;padding:2px 8px;border-radius:4px">' + (data.decision.needs_transcode ? 'Transcode' : 'Copy') + '</span>';
          if (data.decision.video_codec) decisionHtml += '<span class="badge badge-delivery" style="font-size:12px;padding:2px 8px">' + esc(String(data.decision.video_codec).toUpperCase()) + '</span>';
          if (data.decision.audio_codec) decisionHtml += '<span class="badge badge-delivery" style="font-size:12px;padding:2px 8px">' + esc(String(data.decision.audio_codec).toUpperCase()) + '</span>';
          if (data.decision.container) decisionHtml += '<span class="badge" style="background:var(--bg-hover);color:var(--text);font-size:12px;padding:2px 8px;border-radius:4px">' + esc(data.decision.container) + '</span>';
          decisionHtml += '</div>';
        }
        playUrlStatus.innerHTML = '<div class="card" style="margin-top:12px">' +
          '<div class="card-title">Playback Started</div>' +
          '<div style="margin-bottom:8px;font-size:13px">Session: <code style="background:var(--bg-input);padding:2px 6px;border-radius:4px">' + esc(data.session_id || '-') + '</code></div>' +
          '<div style="margin-bottom:8px;font-size:13px">Delivery: <span class="badge" style="font-size:12px;padding:2px 8px;border-radius:4px">' + esc(data.delivery || '-') + '</span></div>' +
          decisionHtml +
          '<div style="display:flex;gap:8px;margin-top:12px">' +
          '<button class="btn btn-primary" id="playurl-open">Open in Player</button>' +
          '<button class="btn btn-danger" id="playurl-stop">Stop</button>' +
          '</div></div>';
        var streamId = data.stream_id;
        document.getElementById('playurl-open').addEventListener('click', function() {
          playerState.cleanup();
          playerState.streamID = streamId;
          playerState.sessionID = data.session_id;
          playerState.delivery = data.delivery;
          router.current = 'player';
          router.params = { streamID: streamId, delivery: data.delivery, endpoints: data.endpoints, isLive: true };
          render();
        });
        document.getElementById('playurl-stop').addEventListener('click', async function() {
          await api.del('/api/play/' + encodeURIComponent(streamId)).catch(function() {});
          toast('Playback stopped');
          playUrlStatus.innerHTML = '';
        });
      } catch (e) {
        playUrlStatus.innerHTML = '<div style="padding:16px;color:var(--danger)">' + esc(e.message) + '</div>';
      }
    }

    playUrlProbeBtn.addEventListener('click', doProbe);
    playUrlBtn.addEventListener('click', doPlay);
    playUrlInput.addEventListener('keydown', function(e) { if (e.key === 'Enter') doPlay(); });
  }

  function renderDeveloper(el) {
    var activeTab = (router.params && router.params.tab) || 'probe';
    el.innerHTML = '<h1 class="page-title">Developer Tools</h1>' +
      '<div style="display:flex;gap:4px;margin-bottom:16px;border-bottom:1px solid var(--border);padding-bottom:8px">' +
      '<button class="btn dev-tab' + (activeTab === 'probe' ? ' btn-primary' : ' btn-ghost') + '" data-tab="probe">Probe</button>' +
      '<button class="btn dev-tab' + (activeTab === 'playurl' ? ' btn-primary' : ' btn-ghost') + '" data-tab="playurl">Play URL</button>' +
      '<button class="btn dev-tab' + (activeTab === 'apikeys' ? ' btn-primary' : ' btn-ghost') + '" data-tab="apikeys">API Keys</button>' +
      '<button class="btn dev-tab' + (activeTab === 'debug' ? ' btn-primary' : ' btn-ghost') + '" data-tab="debug">Debug</button>' +
      '</div>' +
      '<div id="dev-content"></div>';
    var contentEl = document.getElementById('dev-content');
    function loadTab(tab) {
      contentEl.innerHTML = '';
      if (tab === 'probe') renderProbe(contentEl);
      else if (tab === 'playurl') renderPlayURL(contentEl);
      else if (tab === 'apikeys') renderAPIKeys(contentEl);
      else if (tab === 'debug') {
        contentEl.innerHTML = '<div class="card"><div class="card-title">Debug Settings</div>' +
          '<div class="form-group"><label class="form-label">Debug Logging</label>' +
          '<button class="btn btn-sm" id="toggle-debug">Toggle</button></div>' +
          '<div class="form-group"><label class="form-label">pprof Endpoints</label>' +
          '<div style="font-size:13px;color:var(--text-secondary)">/debug/pprof/ (enabled when debug_enabled=true)</div></div></div>';
        document.getElementById('toggle-debug').addEventListener('click', async function() {
          var resp = await api.get('/api/settings');
          var settings = await resp.json();
          var current = settings.debug_enabled === 'true' || settings.debug_enabled === '1';
          await api.put('/api/settings', { debug_enabled: current ? 'false' : 'true' });
          toast(current ? 'Debug disabled' : 'Debug enabled');
        });
      }
    }
    el.querySelectorAll('.dev-tab').forEach(function(btn) {
      btn.addEventListener('click', function() {
        el.querySelectorAll('.dev-tab').forEach(function(b) { b.className = 'btn dev-tab btn-ghost'; });
        this.className = 'btn dev-tab btn-primary';
        loadTab(this.getAttribute('data-tab'));
      });
    });
    loadTab(activeTab);
  }

  var pages = {
    dashboard: renderDashboard,
    streams: renderStreams,
    channels: renderChannels,
    library: renderLibrary,
    guide: renderGuide,
    recordings: renderRecordings,
    favorites: renderFavorites,
    activity: renderActivity,
    sources: renderSources,
    sourceprofiles: renderSourceProfiles,
    epgsources: renderEPGSources,
    wireguard: renderWireGuard,
    settings: renderSettings,
    users: renderUsers,
    clients: renderClients,
    logos: renderLogos,
    tmdb: renderTMDBPage,
    probe: renderProbe,
    playurl: renderPlayURL,
    player: renderPlayer,
    hdhrdevices: renderHDHRDevices,
    invites: renderInvites,
    apikeys: renderAPIKeys,
    developer: renderDeveloper
  };

  router.init();

  (function handleOAuthCallback() {
    var hash = location.hash || '';
    var match = hash.match(/[?&]token=([^&]+)/);
    if (match) {
      var token = decodeURIComponent(match[1]);
      var refreshMatch = hash.match(/[?&]refresh=([^&]+)/);
      api.token = token;
      if (refreshMatch) {
        try { localStorage.setItem('mediahub_refresh', decodeURIComponent(refreshMatch[1])); } catch (e) {}
      }
      history.replaceState(null, '', '#/dashboard');
      router.current = 'dashboard';
      router.params = {};
    } else {
      var errorMatch = hash.match(/[?&]error=([^&]+)/);
      if (errorMatch) {
        var errorCode = decodeURIComponent(errorMatch[1]);
        if (errorCode === 'no_account') {
          setTimeout(function() { toast('No account with this email. Ask an admin to add your email.', 'error'); }, 200);
        } else {
          setTimeout(function() { toast('Sign in failed: ' + errorCode, 'error'); }, 200);
        }
        history.replaceState(null, '', '#/login');
        router.current = 'login';
        router.params = {};
      }
    }
  })();

  render();

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = { pages: pages, esc: esc, formatTime: formatTime, api: api, PlayerRegistry: PlayerRegistry, detectCapabilities: detectCapabilities };
  }
})();

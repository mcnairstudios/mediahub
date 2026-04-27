(function() {
  'use strict';

  var TOKEN_KEY = 'mediahub_token';
  var USER_KEY = 'mediahub_user';

  function esc(s) {
    if (s == null) return '';
    var d = document.createElement('div');
    d.textContent = String(s);
    return d.innerHTML;
  }

  var api = {
    get token() { return localStorage.getItem(TOKEN_KEY); },
    set token(v) { if (v) localStorage.setItem(TOKEN_KEY, v); else localStorage.removeItem(TOKEN_KEY); },

    get user() {
      try { return JSON.parse(localStorage.getItem(USER_KEY)); } catch (e) { return null; }
    },
    set user(v) { if (v) localStorage.setItem(USER_KEY, JSON.stringify(v)); else localStorage.removeItem(USER_KEY); },

    async request(method, path, body) {
      var headers = { 'Content-Type': 'application/json' };
      if (this.token) headers['Authorization'] = 'Bearer ' + this.token;
      var opts = { method: method, headers: headers };
      if (body !== undefined) opts.body = JSON.stringify(body);
      var resp = await fetch(path, opts);
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
    wireguard: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2L3 7v6c0 5.25 3.82 10.15 9 11 5.18-.85 9-5.75 9-11V7l-9-5z"/><path d="M12 8v4M12 16h.01"/></svg>'
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

  var playerState = {
    hlsInstance: null,
    videoEl: null,
    bufferWatchInterval: null,
    statsInterval: null,
    statsVisible: false,
    currentStreamID: null,
    sessionID: null,
    isLive: false,

    cleanup: function() {
      if (this.bufferWatchInterval) { clearInterval(this.bufferWatchInterval); this.bufferWatchInterval = null; }
      if (this.statsInterval) { clearInterval(this.statsInterval); this.statsInterval = null; }
      if (this.hlsInstance) { this.hlsInstance.destroy(); this.hlsInstance = null; }
      if (this.currentStreamID) {
        api.del('/api/play/' + this.currentStreamID).catch(function() {});
        this.currentStreamID = null;
      }
      this.videoEl = null;
      this.sessionID = null;
    }
  };

  function navItems() {
    var user = api.user;
    var isAdmin = user && user.is_admin;
    var items = [
      { id: 'dashboard', label: 'Dashboard', icon: 'dashboard' },
      { id: 'streams', label: 'Streams', icon: 'streams' },
      { id: 'channels', label: 'Channels', icon: 'channels' },
      { id: 'recordings', label: 'Recordings', icon: 'recordings' }
    ];
    if (isAdmin) {
      items.push({ id: 'sources', label: 'Sources', icon: 'sources' });
      items.push({ id: 'wireguard', label: 'WireGuard', icon: 'wireguard' });
      items.push({ id: 'settings', label: 'Settings', icon: 'settings' });
      items.push({ id: 'users', label: 'Users', icon: 'users' });
    }
    return items;
  }

  function renderSidebar() {
    var items = navItems();
    var user = api.user;
    var html = '<div class="sidebar" id="sidebar">';
    html += '<div class="sidebar-header">Media<span>Hub</span></div>';
    html += '<div class="sidebar-nav">';
    for (var i = 0; i < items.length; i++) {
      var it = items[i];
      var active = router.current === it.id ? ' active' : '';
      html += '<div class="nav-item' + active + '" data-page="' + it.id + '">';
      html += icons[it.icon] || '';
      html += '<span>' + esc(it.label) + '</span></div>';
    }
    html += '</div>';
    html += '<div class="sidebar-footer">';
    if (user) html += '<span>' + esc(user.username) + '</span> &middot; <span class="logout" id="logout-btn">Logout</span>';
    html += '</div></div>';
    return html;
  }

  function render() {
    var app = document.getElementById('app');
    if (!api.token) {
      playerState.cleanup();
      app.innerHTML = renderLogin();
      bindLogin();
      return;
    }

    if (router.current === 'login') {
      router.current = 'dashboard';
    }

    var page = router.current || 'dashboard';
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

    var pageEl = document.getElementById('page');
    if (!pageEl) return;

    if (page === 'dashboard') renderDashboard(pageEl);
    else if (page === 'streams') renderStreams(pageEl);
    else if (page === 'channels') renderChannels(pageEl);
    else if (page === 'recordings') renderRecordings(pageEl);
    else if (page === 'sources') renderSources(pageEl);
    else if (page === 'settings') renderSettings(pageEl);
    else if (page === 'users') renderUsers(pageEl);
    else renderDashboard(pageEl);
  }

  function bindSidebar() {
    var navItems = document.querySelectorAll('.nav-item[data-page]');
    for (var i = 0; i < navItems.length; i++) {
      navItems[i].addEventListener('click', function() {
        playerState.cleanup();
        router.navigate(this.getAttribute('data-page'));
        var sidebar = document.getElementById('sidebar');
        if (sidebar) sidebar.classList.remove('open');
      });
    }
    var logoutBtn = document.getElementById('logout-btn');
    if (logoutBtn) {
      logoutBtn.addEventListener('click', function() {
        playerState.cleanup();
        api.token = null;
        api.user = null;
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
      '</form></div></div>';
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
        api.user = { username: user, is_admin: true };
        router.navigate('dashboard');
      } catch (err) {
        errEl.textContent = 'Connection failed';
        errEl.style.display = 'block';
      }
    });
  }

  async function renderDashboard(el) {
    el.innerHTML = '<h1 class="page-title">Dashboard</h1>' +
      '<div class="stat-grid">' +
      '<div class="stat-card"><div class="stat-value" id="stat-streams">-</div><div class="stat-label">Streams</div></div>' +
      '<div class="stat-card"><div class="stat-value" id="stat-channels">-</div><div class="stat-label">Channels</div></div>' +
      '<div class="stat-card"><div class="stat-value" id="stat-recordings">-</div><div class="stat-label">Recordings</div></div>' +
      '</div>' +
      '<div class="card"><div class="card-title">Quick Links</div>' +
      '<div style="display:flex;gap:8px;flex-wrap:wrap">' +
      '<button class="btn btn-ghost" data-page="streams">Browse Streams</button>' +
      '<button class="btn btn-ghost" data-page="channels">Browse Channels</button>' +
      '<button class="btn btn-ghost" data-page="recordings">View Recordings</button>' +
      '</div></div>';

    el.querySelectorAll('[data-page]').forEach(function(btn) {
      btn.addEventListener('click', function() { router.navigate(this.getAttribute('data-page')); });
    });

    try {
      var results = await Promise.allSettled([
        api.get('/api/streams').then(function(r) { return r.json(); }),
        api.get('/api/channels').then(function(r) { return r.json(); }),
        api.get('/api/recordings').then(function(r) { return r.json(); })
      ]);
      var streams = results[0].status === 'fulfilled' ? results[0].value : [];
      var channels = results[1].status === 'fulfilled' ? results[1].value : [];
      var recordings = results[2].status === 'fulfilled' ? results[2].value : [];
      var s = document.getElementById('stat-streams');
      var c = document.getElementById('stat-channels');
      var rc = document.getElementById('stat-recordings');
      if (s) s.textContent = Array.isArray(streams) ? streams.length : 0;
      if (c) c.textContent = Array.isArray(channels) ? channels.length : 0;
      if (rc) rc.textContent = Array.isArray(recordings) ? recordings.length : 0;
    } catch (e) {}
  }

  async function renderStreams(el) {
    el.innerHTML = '<h1 class="page-title">Streams</h1>' +
      '<div class="search-bar">' + icons.search + '<input id="stream-search" placeholder="Search streams..."></div>' +
      '<div id="stream-list"><div class="skeleton" style="height:200px"></div></div>';

    try {
      var resp = await api.get('/api/streams');
      var streams = await resp.json();
      if (!Array.isArray(streams)) streams = [];
      renderStreamTable(streams, '');
      document.getElementById('stream-search').addEventListener('input', function() {
        renderStreamTable(streams, this.value.toLowerCase());
      });
    } catch (e) {
      document.getElementById('stream-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load streams</p></div>';
    }
  }

  function renderStreamTable(streams, filter) {
    var container = document.getElementById('stream-list');
    if (!container) return;
    var filtered = streams;
    if (filter) {
      filtered = streams.filter(function(s) {
        return (s.name || '').toLowerCase().indexOf(filter) >= 0 ||
               (s.group || '').toLowerCase().indexOf(filter) >= 0 ||
               (s.url || '').toLowerCase().indexOf(filter) >= 0;
      });
    }
    if (filtered.length === 0) {
      container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No streams found</p></div>';
      return;
    }
    var html = '<table class="list-table"><thead><tr>' +
      '<th></th><th>Name</th><th>Group</th><th>Type</th><th></th>' +
      '</tr></thead><tbody>';
    for (var i = 0; i < filtered.length; i++) {
      var s = filtered[i];
      var logo = s.logo_url ? '<img class="logo" src="' + esc(s.logo_url) + '" alt="">' : '';
      var typeBadge = s.is_live ? '<span class="badge badge-live">LIVE</span>' : '<span class="badge badge-vod">VOD</span>';
      html += '<tr class="clickable" data-stream-id="' + esc(s.id) + '">' +
        '<td>' + logo + '</td>' +
        '<td>' + esc(s.name) + '</td>' +
        '<td>' + esc(s.group || '-') + '</td>' +
        '<td>' + typeBadge + '</td>' +
        '<td><button class="btn btn-sm btn-primary play-btn" data-id="' + esc(s.id) + '" data-name="' + esc(s.name) + '">' + icons.play + '</button></td>' +
        '</tr>';
    }
    html += '</tbody></table>';
    container.innerHTML = html;
    container.querySelectorAll('.play-btn').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        startPlay(this.getAttribute('data-id'), this.getAttribute('data-name'), true);
      });
    });
  }

  async function renderChannels(el) {
    el.innerHTML = '<h1 class="page-title">Channels</h1>' +
      '<div class="search-bar">' + icons.search + '<input id="channel-search" placeholder="Search channels..."></div>' +
      '<div id="channel-list"><div class="skeleton" style="height:200px"></div></div>';

    try {
      var resp = await api.get('/api/channels');
      var channels = await resp.json();
      if (!Array.isArray(channels)) channels = [];
      renderChannelTable(channels, '');
      document.getElementById('channel-search').addEventListener('input', function() {
        renderChannelTable(channels, this.value.toLowerCase());
      });
    } catch (e) {
      document.getElementById('channel-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load channels</p></div>';
    }
  }

  function renderChannelTable(channels, filter) {
    var container = document.getElementById('channel-list');
    if (!container) return;
    var filtered = channels;
    if (filter) {
      filtered = channels.filter(function(c) {
        return (c.name || '').toLowerCase().indexOf(filter) >= 0 ||
               (c.group_name || '').toLowerCase().indexOf(filter) >= 0;
      });
    }
    if (filtered.length === 0) {
      container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No channels found</p></div>';
      return;
    }
    var html = '<table class="list-table"><thead><tr>' +
      '<th></th><th>#</th><th>Name</th><th>Group</th><th>Status</th><th></th>' +
      '</tr></thead><tbody>';
    for (var i = 0; i < filtered.length; i++) {
      var c = filtered[i];
      var logo = c.logo_url ? '<img class="logo" src="' + esc(c.logo_url) + '" alt="">' : '';
      var status = c.is_enabled !== false ? '<span class="badge badge-enabled">ON</span>' : '<span class="badge badge-disabled">OFF</span>';
      html += '<tr class="clickable" data-channel-id="' + esc(c.id) + '">' +
        '<td>' + logo + '</td>' +
        '<td>' + esc(c.number || i + 1) + '</td>' +
        '<td>' + esc(c.name) + '</td>' +
        '<td>' + esc(c.group_name || '-') + '</td>' +
        '<td>' + status + '</td>' +
        '<td><button class="btn btn-sm btn-primary play-btn" data-id="' + esc(c.stream_id || c.id) + '" data-name="' + esc(c.name) + '">' + icons.play + '</button></td>' +
        '</tr>';
    }
    html += '</tbody></table>';
    container.innerHTML = html;
    container.querySelectorAll('.play-btn').forEach(function(btn) {
      btn.addEventListener('click', function(e) {
        e.stopPropagation();
        startPlay(this.getAttribute('data-id'), this.getAttribute('data-name'), true);
      });
    });
  }

  async function startPlay(streamID, name, isLive) {
    playerState.cleanup();
    playerState.currentStreamID = streamID;
    playerState.isLive = isLive !== false;
    router.navigate('player', { streamID: streamID, name: name || streamID, isLive: isLive });
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

    pageEl.innerHTML = '<h1 class="page-title">' + esc(name) + '</h1>' +
      '<div class="player-wrapper" id="player-wrapper">' +
      '<div class="stats-overlay" id="stats-overlay"></div>' +
      '<video id="video-el" autoplay playsinline></video>' +
      '</div>' +
      '<div class="player-controls" id="player-controls">' +
      '<button class="btn btn-sm btn-ghost" id="play-pause-btn">' + icons.pause + '</button>' +
      '<span class="time" id="time-current">0:00</span>' +
      '<input type="range" class="seek-bar" id="seek-bar" min="0" max="1000" value="0">' +
      '<span class="time" id="time-duration">0:00</span>' +
      '<button class="btn btn-sm btn-ghost" id="stats-btn" title="Toggle Stats">' + icons.stats + '</button>' +
      '</div>' +
      '<div style="display:flex;gap:8px">' +
      '<button class="btn btn-danger btn-sm" id="stop-btn">Stop</button>' +
      '<button class="btn btn-ghost btn-sm" id="record-btn">Record</button>' +
      '</div>';

    initPlayer(streamID);
  }

  async function initPlayer(streamID) {
    var videoEl = document.getElementById('video-el');
    if (!videoEl) return;
    playerState.videoEl = videoEl;

    try {
      var resp = await api.post('/api/play/' + streamID);
      if (!resp.ok) {
        var errData = await resp.json().catch(function() { return {}; });
        toast('Failed to start playback: ' + (errData.error || resp.statusText), 'error');
        return;
      }
      var data = await resp.json();
      playerState.sessionID = data.session_id;

      var delivery = data.delivery || 'hls';
      var endpoints = data.endpoints || {};

      if (delivery === 'hls') {
        var hlsUrl = endpoints.playlist || ('/api/play/' + streamID + '/hls/playlist.m3u8');
        if (typeof Hls !== 'undefined' && Hls.isSupported()) {
          startHLS(videoEl, hlsUrl);
        } else if (videoEl.canPlayType('application/vnd.apple.mpegurl')) {
          videoEl.src = hlsUrl;
          videoEl.play().catch(function() {});
        } else {
          startMSE(videoEl, streamID, endpoints);
        }
      } else if (delivery === 'mse') {
        if ('MediaSource' in window) {
          startMSE(videoEl, streamID, endpoints);
        } else {
          toast('Browser does not support MSE playback', 'error');
        }
      } else {
        toast('Unknown delivery mode: ' + delivery, 'error');
        return;
      }
    } catch (e) {
      toast('Playback error: ' + e.message, 'error');
      return;
    }

    bindPlayerControls(videoEl, streamID);
    startBufferWatch(videoEl);
    startStatsWatch(videoEl);
  }

  function startHLS(videoEl, url) {
    var hls = new Hls({
      liveSyncDurationCount: 3,
      liveMaxLatencyDurationCount: 6,
      maxBufferLength: 30,
      maxMaxBufferLength: 60,
      startLevel: -1,
      xhrSetup: function(xhr) {
        if (api.token) xhr.setRequestHeader('Authorization', 'Bearer ' + api.token);
      }
    });
    hls.loadSource(url);
    hls.attachMedia(videoEl);
    hls.on(Hls.Events.MANIFEST_PARSED, function() {
      videoEl.play().catch(function() {});
    });
    hls.on(Hls.Events.ERROR, function(event, data) {
      if (data.fatal) {
        if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
          hls.startLoad();
        } else if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
          hls.recoverMediaError();
        }
      }
    });
    playerState.hlsInstance = hls;
  }

  function startMSE(videoEl, streamID, endpoints) {
    if (!('MediaSource' in window)) {
      toast('Browser does not support MSE playback', 'error');
      return;
    }

    var videoInitUrl = (endpoints && endpoints.video_init) || ('/api/play/' + streamID + '/mse/video/init');
    var audioInitUrl = (endpoints && endpoints.audio_init) || ('/api/play/' + streamID + '/mse/audio/init');
    var videoSegUrl = (endpoints && endpoints.video_segment) || ('/api/play/' + streamID + '/mse/video/segment');
    var audioSegUrl = (endpoints && endpoints.audio_segment) || ('/api/play/' + streamID + '/mse/audio/segment');

    var ms = new MediaSource();
    videoEl.src = URL.createObjectURL(ms);
    ms.addEventListener('sourceopen', function() {
      var videoMime = 'video/mp4; codecs="avc1.640028"';
      if (!MediaSource.isTypeSupported(videoMime)) {
        videoMime = 'video/mp4; codecs="avc1.42E01E"';
      }
      var audioMime = 'audio/mp4; codecs="mp4a.40.2"';

      var videoSB = ms.addSourceBuffer(videoMime);
      var audioSB = null;
      try { audioSB = ms.addSourceBuffer(audioMime); } catch (e) {}

      function makeFeeder(sb) {
        var queue = [];
        var feeding = false;
        function feedNext() {
          if (feeding || !queue.length || sb.updating) return;
          feeding = true;
          var chunk = queue.shift();
          try { sb.appendBuffer(chunk); } catch (e) { feeding = false; }
        }
        sb.addEventListener('updateend', function() {
          feeding = false;
          if (queue.length) feedNext();
        });
        return { push: function(data) { queue.push(data); feedNext(); } };
      }

      var videoFeeder = makeFeeder(videoSB);
      var audioFeeder = audioSB ? makeFeeder(audioSB) : null;

      fetchInit(videoInitUrl, videoFeeder);
      if (audioFeeder) fetchInit(audioInitUrl, audioFeeder);

      pollSegments(videoSegUrl, videoFeeder, videoEl);
      if (audioFeeder) pollSegments(audioSegUrl, audioFeeder, videoEl);
    });
  }

  function fetchInit(url, feeder) {
    var stopped = false;
    function attempt() {
      if (stopped || !playerState.currentStreamID) return;
      fetch(url, { headers: { 'Authorization': 'Bearer ' + api.token } })
        .then(function(resp) {
          if (!resp.ok) { setTimeout(attempt, 500); return; }
          return resp.arrayBuffer();
        })
        .then(function(buf) {
          if (buf) feeder.push(new Uint8Array(buf));
        })
        .catch(function() { setTimeout(attempt, 1000); });
    }
    attempt();
    var origCleanup = playerState.cleanup.bind(playerState);
    playerState.cleanup = function() { stopped = true; origCleanup(); };
  }

  function pollSegments(baseUrl, feeder, videoEl) {
    var segIdx = 1;
    var stopped = false;

    function poll() {
      if (stopped || !playerState.currentStreamID) return;
      fetch(baseUrl + '?seq=' + segIdx, {
        headers: { 'Authorization': 'Bearer ' + api.token }
      }).then(function(resp) {
        if (!resp.ok) {
          setTimeout(poll, 500);
          return;
        }
        return resp.arrayBuffer();
      }).then(function(buf) {
        if (!buf) return;
        feeder.push(new Uint8Array(buf));
        segIdx++;
        if (!videoEl.paused) { /* keep going */ }
        else videoEl.play().catch(function() {});
        setTimeout(poll, 50);
      }).catch(function() {
        setTimeout(poll, 1000);
      });
    }
    poll();

    var origCleanup = playerState.cleanup.bind(playerState);
    playerState.cleanup = function() {
      stopped = true;
      origCleanup();
    };
  }

  function startBufferWatch(videoEl) {
    if (playerState.bufferWatchInterval) clearInterval(playerState.bufferWatchInterval);
    playerState.bufferWatchInterval = setInterval(function() {
      if (!videoEl || videoEl.paused || !videoEl.buffered.length) return;
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
    }, 250);
  }

  function startStatsWatch(videoEl) {
    if (playerState.statsInterval) clearInterval(playerState.statsInterval);
    playerState.statsInterval = setInterval(function() {
      var overlay = document.getElementById('stats-overlay');
      if (!overlay || !overlay.classList.contains('visible')) return;
      updateStats(videoEl, overlay);
    }, 500);
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
    var bufClass = buf > 8 ? 'good' : buf > 4 ? 'warn' : 'bad';
    var rateStr = videoEl.playbackRate.toFixed(2);
    var rateClass = videoEl.playbackRate < 0.93 ? 'bad' : videoEl.playbackRate < 1.0 ? 'warn' : 'good';

    var w = videoEl.videoWidth || 0;
    var h = videoEl.videoHeight || 0;

    var lines = [
      '<span class="label">Resolution: </span><span class="value">' + w + 'x' + h + '</span>',
      '<span class="label">Buffer: </span><span class="' + bufClass + '">' + buf.toFixed(1) + 's</span>',
      '<span class="label">Rate: </span><span class="' + rateClass + '">' + rateStr + 'x</span>'
    ];

    if (playerState.hlsInstance) {
      var hls = playerState.hlsInstance;
      var level = hls.currentLevel >= 0 ? hls.levels[hls.currentLevel] : null;
      if (level) {
        lines.push('<span class="label">Bitrate: </span><span class="value">' + (level.bitrate / 1000).toFixed(0) + ' kbps</span>');
      }
      lines.push('<span class="label">Level: </span><span class="value">' + (hls.currentLevel + 1) + '/' + hls.levels.length + '</span>');
    }

    overlay.innerHTML = lines.join('<br>');
  }

  function bindPlayerControls(videoEl, streamID) {
    var playPauseBtn = document.getElementById('play-pause-btn');
    var seekBar = document.getElementById('seek-bar');
    var timeCurrent = document.getElementById('time-current');
    var timeDuration = document.getElementById('time-duration');
    var statsBtn = document.getElementById('stats-btn');
    var stopBtn = document.getElementById('stop-btn');
    var recordBtn = document.getElementById('record-btn');

    if (playPauseBtn) {
      playPauseBtn.addEventListener('click', function() {
        if (videoEl.paused) {
          videoEl.play().catch(function() {});
          playPauseBtn.innerHTML = icons.pause;
        } else {
          videoEl.pause();
          playPauseBtn.innerHTML = icons.play;
        }
      });
    }

    videoEl.addEventListener('timeupdate', function() {
      if (timeCurrent) timeCurrent.textContent = formatTime(videoEl.currentTime);
      if (timeDuration) timeDuration.textContent = formatTime(videoEl.duration);
      if (seekBar && !seekBar._dragging && isFinite(videoEl.duration) && videoEl.duration > 0) {
        seekBar.value = (videoEl.currentTime / videoEl.duration * 1000).toFixed(0);
      }
    });

    if (seekBar) {
      seekBar.addEventListener('mousedown', function() { seekBar._dragging = true; });
      seekBar.addEventListener('mouseup', function() { seekBar._dragging = false; });
      seekBar.addEventListener('change', function() {
        if (isFinite(videoEl.duration) && videoEl.duration > 0) {
          var pos = (seekBar.value / 1000) * videoEl.duration;
          videoEl.currentTime = pos;
          api.post('/api/play/' + streamID + '/seek', { position_ms: Math.round(pos * 1000) }).catch(function() {});
        }
      });
    }

    if (statsBtn) {
      statsBtn.addEventListener('click', function() {
        var overlay = document.getElementById('stats-overlay');
        if (overlay) overlay.classList.toggle('visible');
      });
    }

    if (stopBtn) {
      stopBtn.addEventListener('click', function() {
        playerState.cleanup();
        router.navigate('streams');
      });
    }

    if (recordBtn) {
      var recording = false;
      recordBtn.addEventListener('click', async function() {
        if (recording) {
          await api.del('/api/play/' + streamID + '/record').catch(function() {});
          recordBtn.textContent = 'Record';
          recordBtn.classList.remove('btn-danger');
          recordBtn.classList.add('btn-ghost');
          recording = false;
          toast('Recording stopped');
        } else {
          var resp = await api.post('/api/play/' + streamID + '/record', { title: 'Manual Recording' }).catch(function() {});
          if (resp && resp.ok) {
            recordBtn.innerHTML = '<span class="recording-status"><span class="recording-dot"></span>Recording</span>';
            recordBtn.classList.remove('btn-ghost');
            recordBtn.classList.add('btn-danger');
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

  async function renderRecordings(el) {
    el.innerHTML = '<h1 class="page-title">Recordings</h1>' +
      '<div id="recording-list"><div class="skeleton" style="height:200px"></div></div>';

    try {
      var resp = await api.get('/api/recordings');
      var recordings = await resp.json();
      if (!Array.isArray(recordings)) recordings = [];
      var container = document.getElementById('recording-list');
      if (!container) return;
      if (recordings.length === 0) {
        container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No recordings yet</p></div>';
        return;
      }
      var html = '<table class="list-table"><thead><tr>' +
        '<th>Title</th><th>Status</th><th>Started</th><th>Duration</th>' +
        '</tr></thead><tbody>';
      for (var i = 0; i < recordings.length; i++) {
        var r = recordings[i];
        var statusClass = r.status === 'recording' ? 'badge-live' : r.status === 'completed' ? 'badge-enabled' : 'badge-disabled';
        var statusText = r.status === 'recording' ? '<span class="recording-status"><span class="recording-dot"></span>' + esc(r.status) + '</span>' : esc(r.status || 'unknown');
        html += '<tr>' +
          '<td>' + esc(r.title || r.stream_id) + '</td>' +
          '<td><span class="badge ' + statusClass + '">' + statusText + '</span></td>' +
          '<td>' + esc(r.started_at ? new Date(r.started_at).toLocaleString() : '-') + '</td>' +
          '<td>' + esc(r.duration || '-') + '</td>' +
          '</tr>';
      }
      html += '</tbody></table>';
      container.innerHTML = html;
    } catch (e) {
      document.getElementById('recording-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load recordings</p></div>';
    }
  }

  async function renderSources(el) {
    el.innerHTML = '<h1 class="page-title">Sources</h1>' +
      '<div style="margin-bottom:16px"><button class="btn btn-primary" id="add-source-btn">' + icons.plus + ' Add M3U Source</button></div>' +
      '<div id="source-list"><div class="skeleton" style="height:200px"></div></div>' +
      '<div id="add-source-form" style="display:none" class="card">' +
      '<div class="card-title">New M3U Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="src-name" placeholder="My IPTV Provider"></div>' +
      '<div class="form-group"><label class="form-label">URL</label><input class="form-input" id="src-url" placeholder="http://example.com/playlist.m3u"></div>' +
      '<div class="form-group"><label class="form-label">Username (optional)</label><input class="form-input" id="src-username"></div>' +
      '<div class="form-group"><label class="form-label">Password (optional)</label><input class="form-input" id="src-password" type="password"></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="src-wireguard"> Route through WireGuard</label></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-source-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-source-btn">Cancel</button></div></div>';

    var addBtn = document.getElementById('add-source-btn');
    var formEl = document.getElementById('add-source-form');
    addBtn.addEventListener('click', function() { formEl.style.display = formEl.style.display === 'none' ? 'block' : 'none'; });
    document.getElementById('cancel-source-btn').addEventListener('click', function() { formEl.style.display = 'none'; });

    document.getElementById('create-source-btn').addEventListener('click', async function() {
      var name = document.getElementById('src-name').value.trim();
      var url = document.getElementById('src-url').value.trim();
      var username = document.getElementById('src-username').value.trim();
      var password = document.getElementById('src-password').value;
      var wg = document.getElementById('src-wireguard').checked;
      if (!name || !url) { toast('Name and URL required', 'error'); return; }
      try {
        var r = await api.post('/api/sources/m3u', { name: name, url: url, username: username, password: password, use_wireguard: wg });
        if (r.ok) {
          toast('Source created, refreshing...');
          formEl.style.display = 'none';
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to create source', 'error');
        }
      } catch (err) {
        toast('Failed to create source', 'error');
      }
    });

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

      var html = '<table class="list-table"><thead><tr>' +
        '<th>Name</th><th>Type</th><th>Streams</th><th>Last Refreshed</th><th>Status</th><th></th>' +
        '</tr></thead><tbody>';
      for (var i = 0; i < sources.length; i++) {
        var s = sources[i];
        var statusBadge = s.is_enabled ? '<span class="badge badge-enabled">ON</span>' : '<span class="badge badge-disabled">OFF</span>';
        if (s.last_error) {
          statusBadge = '<span class="badge badge-live" title="' + esc(s.last_error) + '">ERROR</span>';
        }
        var lastRefreshed = s.last_refreshed ? new Date(s.last_refreshed).toLocaleString() : 'Never';
        html += '<tr>' +
          '<td>' + esc(s.name) + '</td>' +
          '<td><span class="badge badge-enabled">' + esc(s.type || 'unknown').toUpperCase() + '</span></td>' +
          '<td>' + (s.stream_count || 0) + '</td>' +
          '<td>' + esc(lastRefreshed) + '</td>' +
          '<td>' + statusBadge + '</td>' +
          '<td style="display:flex;gap:4px">' +
          '<button class="btn btn-sm btn-ghost refresh-source-btn" data-id="' + esc(s.id) + '" data-type="' + esc(s.type) + '" title="Refresh">' + icons.refresh + '</button>' +
          '<button class="btn btn-sm btn-danger delete-source-btn" data-id="' + esc(s.id) + '" data-type="' + esc(s.type) + '" data-name="' + esc(s.name) + '" title="Delete">' + icons.trash + '</button>' +
          '</td></tr>';
      }
      html += '</tbody></table>';
      container.innerHTML = html;

      container.querySelectorAll('.refresh-source-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          try {
            var r = await api.post('/api/sources/' + id + '/refresh', {});
            if (r.ok || r.status === 202) {
              toast('Refresh started');
            } else {
              toast('Failed to refresh', 'error');
            }
          } catch (err) {
            toast('Failed to refresh', 'error');
          }
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
    } catch (e) {
      document.getElementById('source-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load sources</p></div>';
    }
  }

  async function renderSettings(el) {
    el.innerHTML = '<h1 class="page-title">Settings</h1>' +
      '<div id="settings-list"><div class="skeleton" style="height:200px"></div></div>';

    try {
      var resp = await api.get('/api/settings');
      var settings = await resp.json();
      var container = document.getElementById('settings-list');
      if (!container) return;

      var entries;
      if (Array.isArray(settings)) {
        entries = settings.map(function(s) { return { key: s.key, value: s.value }; });
      } else if (settings && typeof settings === 'object') {
        entries = Object.keys(settings).map(function(k) { return { key: k, value: settings[k] }; });
      } else {
        entries = [];
      }

      if (entries.length === 0) {
        container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No settings configured</p></div>';
        return;
      }

      var html = '<div class="settings-grid">';
      for (var i = 0; i < entries.length; i++) {
        var e = entries[i];
        html += '<div class="setting-row">' +
          '<span class="setting-key">' + esc(e.key) + '</span>' +
          '<input class="setting-value" data-key="' + esc(e.key) + '" value="' + esc(e.value) + '">' +
          '</div>';
      }
      html += '</div><div style="margin-top:16px"><button class="btn btn-primary" id="save-settings-btn">Save Settings</button></div>';
      container.innerHTML = html;

      document.getElementById('save-settings-btn').addEventListener('click', async function() {
        var inputs = container.querySelectorAll('.setting-value');
        var payload = {};
        inputs.forEach(function(inp) { payload[inp.getAttribute('data-key')] = inp.value; });
        try {
          var r = await api.put('/api/settings', payload);
          if (r.ok) toast('Settings saved');
          else toast('Failed to save settings', 'error');
        } catch (err) {
          toast('Failed to save settings', 'error');
        }
      });
    } catch (e) {
      document.getElementById('settings-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load settings</p></div>';
    }
  }

  async function renderUsers(el) {
    el.innerHTML = '<h1 class="page-title">Users</h1>' +
      '<div style="margin-bottom:16px"><button class="btn btn-primary" id="add-user-btn">Add User</button></div>' +
      '<div id="user-list"><div class="skeleton" style="height:200px"></div></div>' +
      '<div id="add-user-form" style="display:none" class="card">' +
      '<div class="card-title">New User</div>' +
      '<div class="form-group"><label class="form-label">Username</label><input class="form-input" id="new-username"></div>' +
      '<div class="form-group"><label class="form-label">Password</label><input class="form-input" id="new-password" type="password"></div>' +
      '<div class="form-group"><label class="form-label">Role</label>' +
      '<select class="form-input" id="new-role"><option value="standard">Standard</option><option value="admin">Admin</option></select></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-user-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-user-btn">Cancel</button></div></div>';

    var addBtn = document.getElementById('add-user-btn');
    var formEl = document.getElementById('add-user-form');
    addBtn.addEventListener('click', function() { formEl.style.display = formEl.style.display === 'none' ? 'block' : 'none'; });
    document.getElementById('cancel-user-btn').addEventListener('click', function() { formEl.style.display = 'none'; });

    document.getElementById('create-user-btn').addEventListener('click', async function() {
      var un = document.getElementById('new-username').value.trim();
      var pw = document.getElementById('new-password').value;
      var role = document.getElementById('new-role').value;
      if (!un || !pw) { toast('Username and password required', 'error'); return; }
      try {
        var r = await api.post('/api/users', { username: un, password: pw, role: role });
        if (r.ok) {
          toast('User created');
          formEl.style.display = 'none';
          renderUsers(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to create user', 'error');
        }
      } catch (err) {
        toast('Failed to create user', 'error');
      }
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
      var html = '<table class="list-table"><thead><tr>' +
        '<th>Username</th><th>Role</th><th>ID</th>' +
        '</tr></thead><tbody>';
      for (var i = 0; i < users.length; i++) {
        var u = users[i];
        html += '<tr><td>' + esc(u.Username || u.username) + '</td>' +
          '<td><span class="badge ' + (u.IsAdmin || u.is_admin ? 'badge-live' : 'badge-enabled') + '">' + esc(u.Role || u.role || 'standard') + '</span></td>' +
          '<td style="color:var(--text-muted);font-size:11px">' + esc(u.ID || u.id) + '</td></tr>';
      }
      html += '</tbody></table>';
      container.innerHTML = html;
    } catch (e) {
      document.getElementById('user-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load users</p></div>';
    }
  }

  async function renderWireGuard(el) {
    el.innerHTML = '<h1 class="page-title">WireGuard</h1>' +
      '<div id="wg-status-bar" style="margin-bottom:16px"><div class="skeleton" style="height:48px"></div></div>' +
      '<div style="margin-bottom:16px"><button class="btn btn-primary" id="add-wg-btn">' + icons.plus + ' Add Profile</button></div>' +
      '<div id="wg-list"><div class="skeleton" style="height:200px"></div></div>' +
      '<div id="wg-form" style="display:none" class="card">' +
      '<div class="card-title" id="wg-form-title">New WireGuard Profile</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="wg-name" placeholder="My VPN"></div>' +
      '<div class="form-group"><label class="form-label">Private Key</label><input class="form-input" id="wg-privkey" placeholder="base64 private key"></div>' +
      '<div class="form-group"><label class="form-label">Endpoint</label><input class="form-input" id="wg-endpoint" placeholder="vpn.example.com:51820"></div>' +
      '<div class="form-group"><label class="form-label">Peer Public Key</label><input class="form-input" id="wg-pubkey" placeholder="base64 public key"></div>' +
      '<div class="form-group"><label class="form-label">Address</label><input class="form-input" id="wg-address" placeholder="10.0.0.2/24"></div>' +
      '<div class="form-group"><label class="form-label">Allowed IPs</label><input class="form-input" id="wg-allowedips" value="0.0.0.0/0" placeholder="0.0.0.0/0"></div>' +
      '<div class="form-group"><label class="form-label">DNS (optional)</label><input class="form-input" id="wg-dns" placeholder="1.1.1.1"></div>' +
      '<div style="display:flex;gap:8px">' +
      '<button class="btn btn-primary" id="save-wg-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-wg-btn">Cancel</button></div></div>';

    var wgEditId = null;
    var addBtn = document.getElementById('add-wg-btn');
    var formEl = document.getElementById('wg-form');
    addBtn.addEventListener('click', function() {
      wgEditId = null;
      document.getElementById('wg-form-title').textContent = 'New WireGuard Profile';
      document.getElementById('save-wg-btn').textContent = 'Create';
      document.getElementById('wg-name').value = '';
      document.getElementById('wg-privkey').value = '';
      document.getElementById('wg-endpoint').value = '';
      document.getElementById('wg-pubkey').value = '';
      document.getElementById('wg-address').value = '';
      document.getElementById('wg-allowedips').value = '0.0.0.0/0';
      document.getElementById('wg-dns').value = '';
      formEl.style.display = 'block';
    });
    document.getElementById('cancel-wg-btn').addEventListener('click', function() { formEl.style.display = 'none'; });

    document.getElementById('save-wg-btn').addEventListener('click', async function() {
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
          formEl.style.display = 'none';
          renderWireGuard(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to save profile', 'error');
        }
      } catch (err) {
        toast('Failed to save profile', 'error');
      }
    });

    try {
      var statusResp = await api.get('/api/wireguard/status');
      var status = await statusResp.json();
      var statusBar = document.getElementById('wg-status-bar');
      if (statusBar) {
        if (status.connected) {
          statusBar.innerHTML = '<div class="card" style="background:var(--success-bg, #e6f4ea);border:1px solid var(--success-border, #34a853);padding:12px 16px;display:flex;align-items:center;gap:12px">' +
            '<span style="display:inline-block;width:12px;height:12px;border-radius:50%;background:#34a853"></span>' +
            '<strong>Connected</strong> &mdash; ' + esc(status.profile_name) + ' (' + esc(status.endpoint) + ')' +
            '<span style="margin-left:auto;font-size:0.85em;opacity:0.7">Proxy port: ' + status.proxy_port + '</span>' +
            '</div>';
        } else {
          statusBar.innerHTML = '<div class="card" style="background:var(--warning-bg, #fef7e0);border:1px solid var(--warning-border, #f9ab00);padding:12px 16px;display:flex;align-items:center;gap:12px">' +
            '<span style="display:inline-block;width:12px;height:12px;border-radius:50%;background:#f9ab00"></span>' +
            '<strong>Disconnected</strong> &mdash; No active WireGuard tunnel' +
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
          document.getElementById('wg-form-title').textContent = 'Edit WireGuard Profile';
          document.getElementById('save-wg-btn').textContent = 'Update';
          document.getElementById('wg-name').value = this.getAttribute('data-name') || '';
          document.getElementById('wg-privkey').value = '';
          document.getElementById('wg-endpoint').value = this.getAttribute('data-endpoint') || '';
          document.getElementById('wg-pubkey').value = this.getAttribute('data-pubkey') || '';
          document.getElementById('wg-address').value = this.getAttribute('data-address') || '';
          document.getElementById('wg-allowedips').value = this.getAttribute('data-allowedips') || '0.0.0.0/0';
          document.getElementById('wg-dns').value = this.getAttribute('data-dns') || '';
          formEl.style.display = 'block';
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

  var pages = {
    dashboard: renderDashboard,
    streams: renderStreams,
    channels: renderChannels,
    recordings: renderRecordings,
    sources: renderSources,
    wireguard: renderWireGuard,
    settings: renderSettings,
    users: renderUsers,
    player: renderPlayer
  };

  router.init();
  render();

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = { pages: pages, esc: esc, formatTime: formatTime, api: api };
  }
})();

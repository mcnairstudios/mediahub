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
    wireguard: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2L3 7v6c0 5.25 3.82 10.15 9 11 5.18-.85 9-5.75 9-11V7l-9-5z"/><path d="M12 8v4M12 16h.01"/></svg>',
    epg: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 10h18M9 4v16"/></svg>',
    edit: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>',
    star: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>',
    starFilled: '<svg viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>',
    favorites: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>',
    addChannel: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M12 10v0"/><line x1="12" y1="7" x2="12" y2="13"/><line x1="9" y1="10" x2="15" y2="10"/></svg>',
    library: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 19.5A2.5 2.5 0 016.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z"/></svg>',
    guide: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>',
    clients: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/><circle cx="12" cy="10" r="3"/></svg>',
    probe: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/><path d="M11 8v6M8 11h6"/></svg>',
    download: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>',
    key: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 11-7.778 7.778 5.5 5.5 0 017.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg>',
    copy: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>',
    video: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="23 7 16 12 23 17 23 7"/><rect x="1" y="5" width="15" height="14" rx="2"/></svg>',
    audio: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>',
    subtitle: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="18" rx="2"/><path d="M7 15h4M13 15h4M7 11h10"/></svg>'
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

    cleanup: function() {
      if (this.bufferWatchInterval) { clearInterval(this.bufferWatchInterval); this.bufferWatchInterval = null; }
      if (this.statsInterval) { clearInterval(this.statsInterval); this.statsInterval = null; }
      if (this.retryTimeout) { clearTimeout(this.retryTimeout); this.retryTimeout = null; }
      if (this.hlsInstance) { this.hlsInstance.destroy(); this.hlsInstance = null; }
      if (this.mseState) {
        this.mseState.stopped = true;
        this.mseState.appendQueues = { video: [], audio: [] };
        this.mseState.appending = { video: false, audio: false };
        if (this.mseState.mediaSource && this.mseState.mediaSource.readyState === 'open') {
          try { this.mseState.mediaSource.endOfStream(); } catch(e) {}
        }
        this.mseState = null;
      }
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
      { id: 'dashboard', label: 'Dashboard', icon: 'dashboard' },
      { id: 'streams', label: 'Streams', icon: 'streams' },
      { id: 'channels', label: 'Channels', icon: 'channels' },
      { id: 'library', label: 'Library', icon: 'library' },
      { id: 'guide', label: 'Guide', icon: 'guide' },
      { id: 'recordings', label: 'Recordings', icon: 'recordings' },
      { id: 'favorites', label: 'Favorites', icon: 'favorites' }
    ];
    if (isAdmin) {
      items.push({ id: 'activity', label: 'Activity', icon: 'stats' });
      items.push({ id: 'sources', label: 'Sources', icon: 'sources' });
      items.push({ id: 'epgsources', label: 'EPG Sources', icon: 'epg' });
      items.push({ id: 'clients', label: 'Clients', icon: 'clients' });
      items.push({ id: 'probe', label: 'Probe', icon: 'probe' });
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
    else if (page === 'library') renderLibrary(pageEl);
    else if (page === 'guide') renderGuide(pageEl);
    else if (page === 'recordings') renderRecordings(pageEl);
    else if (page === 'favorites') renderFavorites(pageEl);
    else if (page === 'activity') renderActivity(pageEl);
    else if (page === 'sources') renderSources(pageEl);
    else if (page === 'epgsources') renderEPGSources(pageEl);
    else if (page === 'wireguard') renderWireGuard(pageEl);
    else if (page === 'settings') renderSettings(pageEl);
    else if (page === 'users') renderUsers(pageEl);
    else if (page === 'clients') renderClients(pageEl);
    else if (page === 'probe') renderProbe(pageEl);
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
    var isAdmin = api.user && api.user.is_admin;
    el.innerHTML = '<h1 class="page-title">Dashboard</h1>' +
      '<div class="stat-grid" id="dash-stats">' +
      '<div class="stat-card"><div class="stat-value" id="stat-streams">-</div><div class="stat-label">Streams</div></div>' +
      '<div class="stat-card"><div class="stat-value" id="stat-channels">-</div><div class="stat-label">Channels</div></div>' +
      '<div class="stat-card"><div class="stat-value" id="stat-recordings">-</div><div class="stat-label">Recordings</div></div>' +
      (isAdmin ? '<div class="stat-card"><div class="stat-value" id="stat-active">-</div><div class="stat-label">Active Now</div></div>' : '') +
      '<div class="stat-card"><div class="stat-value" id="stat-epg-programs">-</div><div class="stat-label">EPG Programs</div></div>' +
      '</div>' +
      '<div class="dash-section" id="dash-sources-section">' +
      '<div class="dash-section-title">' + icons.sources + ' Sources</div>' +
      '<div class="dash-source-grid" id="dash-sources"><div class="skeleton" style="height:80px"></div></div>' +
      '</div>' +
      '<div class="dash-section" id="dash-epg-section">' +
      '<div class="dash-section-title">' + icons.epg + ' EPG Status</div>' +
      '<div id="dash-epg"><div class="skeleton" style="height:60px"></div></div>' +
      '</div>' +
      '<div class="dash-section" id="dash-rec-section">' +
      '<div class="dash-section-title">' + icons.recordings + ' Recordings</div>' +
      '<div id="dash-recordings"><div class="skeleton" style="height:60px"></div></div>' +
      '</div>' +
      (isAdmin ? '<div class="dash-section" id="dash-wg-section">' +
      '<div class="dash-section-title">' + icons.wireguard + ' Connectivity</div>' +
      '<div id="dash-wg"><div class="skeleton" style="height:60px"></div></div>' +
      '</div>' : '') +
      '<div class="card"><div class="card-title">Quick Links</div>' +
      '<div style="display:flex;gap:8px;flex-wrap:wrap">' +
      '<button class="btn btn-ghost" data-page="streams">Browse Streams</button>' +
      '<button class="btn btn-ghost" data-page="channels">Browse Channels</button>' +
      '<button class="btn btn-ghost" data-page="library">VOD Library</button>' +
      '<button class="btn btn-ghost" data-page="guide">Program Guide</button>' +
      '<button class="btn btn-ghost" data-page="recordings">View Recordings</button>' +
      '</div></div>';

    el.querySelectorAll('[data-page]').forEach(function(btn) {
      btn.addEventListener('click', function() { router.navigate(this.getAttribute('data-page')); });
    });

    try {
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
        var srcTypeColors = { m3u: 'var(--accent)', tvpstreams: 'var(--success)', xtream: 'var(--warning)', hdhr: '#4fc3f7', satip: '#ce93d8' };
        var srcTypeLabels = { m3u: 'M3U', tvpstreams: 'TVP', xtream: 'XT', hdhr: 'HDHR', satip: 'SAT' };
        var sHtml = '';
        for (var si = 0; si < stats.sources.length; si++) {
          var src = stats.sources[si];
          var color = srcTypeColors[src.type] || 'var(--text-muted)';
          var label = srcTypeLabels[src.type] || src.type.toUpperCase();
          var statusDot = src.is_enabled
            ? '<span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:var(--success)"></span>'
            : '<span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:var(--text-muted)"></span>';
          sHtml += '<div class="dash-source-card">' +
            '<div class="dash-source-icon" style="background:' + color + '20;color:' + color + '">' + label + '</div>' +
            '<div class="dash-source-info">' +
            '<div class="dash-source-name">' + statusDot + ' ' + esc(src.name) + '</div>' +
            '<div class="dash-source-meta">' + (src.stream_count || 0) + ' streams</div>' +
            '</div></div>';
        }
        sourcesEl.innerHTML = sHtml || '<div style="color:var(--text-muted)">No sources configured</div>';
      }

      var epgContEl = document.getElementById('dash-epg');
      if (epgContEl && stats.epg) {
        var epgInfo = stats.epg;
        epgContEl.innerHTML = '<div style="display:flex;gap:16px;flex-wrap:wrap">' +
          '<div class="stat-card" style="flex:1;min-width:120px;padding:12px"><div class="stat-value" style="font-size:20px">' + epgInfo.source_count + '</div><div class="stat-label">Sources</div></div>' +
          '<div class="stat-card" style="flex:1;min-width:120px;padding:12px"><div class="stat-value" style="font-size:20px">' + epgInfo.channel_count + '</div><div class="stat-label">Channels Mapped</div></div>' +
          '<div class="stat-card" style="flex:1;min-width:120px;padding:12px"><div class="stat-value" style="font-size:20px">' + epgInfo.program_count.toLocaleString() + '</div><div class="stat-label">Programs</div></div>' +
          (epgInfo.error_count > 0 ? '<div class="stat-card" style="flex:1;min-width:120px;padding:12px;border-color:var(--danger)"><div class="stat-value" style="font-size:20px;color:var(--danger)">' + epgInfo.error_count + '</div><div class="stat-label">Errors</div></div>' : '') +
          '</div>';
      } else if (epgContEl) {
        epgContEl.innerHTML = '<div style="color:var(--text-muted)">No EPG sources configured</div>';
      }

      var recContEl = document.getElementById('dash-recordings');
      if (recContEl && stats.recordings) {
        var rec = stats.recordings;
        var recParts = [];
        if (rec.active > 0) recParts.push('<span class="badge badge-live"><span class="recording-dot" style="width:6px;height:6px;display:inline-block;margin-right:4px"></span>' + rec.active + ' recording</span>');
        if (rec.scheduled > 0) recParts.push('<span class="badge badge-warning">' + rec.scheduled + ' scheduled</span>');
        if (rec.completed > 0) recParts.push('<span class="badge badge-enabled">' + rec.completed + ' completed</span>');
        recContEl.innerHTML = recParts.length > 0
          ? '<div style="display:flex;gap:8px;flex-wrap:wrap;align-items:center">' + recParts.join('') + '</div>'
          : '<div style="color:var(--text-muted)">No recordings</div>';
      }

      var wgEl = document.getElementById('dash-wg');
      if (wgEl && stats.wireguard) {
        if (stats.wireguard.connected) {
          wgEl.innerHTML = '<div style="display:flex;align-items:center;gap:8px">' +
            '<span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:var(--success)"></span>' +
            '<span style="color:var(--success);font-weight:600">WireGuard Connected</span></div>';
        } else {
          wgEl.innerHTML = '<div style="display:flex;align-items:center;gap:8px">' +
            '<span style="display:inline-block;width:10px;height:10px;border-radius:50%;background:var(--warning)"></span>' +
            '<span style="color:var(--text-dim)">WireGuard Disconnected</span></div>';
        }
      } else if (wgEl) {
        wgEl.innerHTML = '<div style="color:var(--text-muted)">Not configured</div>';
      }
    } catch (e) {}
  }

  async function renderStreams(el) {
    el.innerHTML = '<h1 class="page-title">Streams</h1>' +
      '<div id="stream-source-picker" style="margin-bottom:16px"><div class="skeleton" style="height:40px"></div></div>' +
      '<div id="stream-list"></div>';

    try {
      await loadFavorites();
      var resp = await api.get('/api/sources');
      var sources = await resp.json();
      if (!Array.isArray(sources)) sources = [];
      buildSourcePicker(el, sources);
    } catch (e) {
      document.getElementById('stream-source-picker').innerHTML =
        '<div class="empty-state">' + icons.empty + '<p>Failed to load sources</p></div>';
    }
  }

  function buildSourcePicker(el, sources) {
    var picker = document.getElementById('stream-source-picker');
    if (!picker) return;

    if (sources.length === 0) {
      picker.innerHTML = '<div style="padding:20px;text-align:center;color:var(--text-muted)">No sources configured. Add a source first.</div>';
      return;
    }

    var html = '<div style="display:flex;gap:8px;flex-wrap:wrap">';
    for (var i = 0; i < sources.length; i++) {
      var src = sources[i];
      var typeBadge = src.type === 'tvpstreams' ? 'TVP' : src.type === 'xtream' ? 'Xtream' : src.type === 'hdhr' ? 'HDHR' : src.type === 'satip' ? 'SAT>IP' : 'M3U';
      html += '<button class="btn btn-ghost stream-source-tab" data-source-type="' + esc(src.type) + '" data-source-id="' + esc(src.id) + '">' +
        esc(src.name) + ' <span class="stream-badge" style="font-size:10px">' + typeBadge + '</span>' +
        '<span class="stream-group-count">' + (src.stream_count || 0) + '</span></button>';
    }
    html += '</div>';
    picker.innerHTML = html;

    picker.addEventListener('click', function(e) {
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

  async function loadSourceStreams(sourceType, sourceId, isTvpStreams) {
    var container = document.getElementById('stream-list');
    if (!container) return;
    container.innerHTML = '<div class="skeleton" style="height:200px"></div>';

    try {
      var resp = await api.get('/api/streams?source_type=' + encodeURIComponent(sourceType) + '&source_id=' + encodeURIComponent(sourceId));
      var streams = await resp.json();
      if (!Array.isArray(streams)) streams = [];

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

  async function loadFavorites() {
    try {
      var resp = await api.get('/api/favorites');
      var favs = await resp.json();
      streamFavorites = {};
      if (Array.isArray(favs)) {
        for (var i = 0; i < favs.length; i++) {
          streamFavorites[favs[i].stream_id] = true;
        }
      }
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
    return '<img class="stream-group-logo" src="/logo?url=' + encodeURIComponent(logoUrl) + '" loading="lazy" alt="">';
  }

  function streamBadgesHtml(s) {
    var badges = '';
    if (s.video_codec) badges += '<span class="stream-badge stream-badge-codec">' + esc(s.video_codec) + '</span>';
    if (s.height) {
      var label = s.height >= 2160 ? '4K' : s.height >= 1080 ? '1080p' : s.height >= 720 ? '720p' : s.height + 'p';
      badges += '<span class="stream-badge stream-badge-res">' + label + '</span>';
    }
    if (s.audio_codec) badges += '<span class="stream-badge stream-badge-codec">' + esc(s.audio_codec) + '</span>';
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
    return '<tr>' +
      '<td>' + logo + '</td>' +
      '<td>' + esc(displayName) + badges + groupLabel + '</td>' +
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
      var tableEl = document.createElement('table');
      tableEl.className = 'stream-group-table';

      if (section === 'movies') {
        streamData.sort(function(a, b) {
          if ((a.year || '') !== (b.year || '')) return (a.year || '').localeCompare(b.year || '');
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
        });
        return;
      }
      if (btn.dataset.qadd) {
        showAddToChannelModal(btn.dataset.sid, btn.dataset.sname);
        return;
      }
      var isLive = btn.dataset.live !== '0';
      if (!isLive) {
        showStreamDetail(btn.dataset.sid, btn.dataset.sname);
      } else {
        startPlay(btn.dataset.sid, btn.dataset.sname, isLive);
      }
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
        if (searchTerm && display.toLowerCase().indexOf(searchTerm) === -1) continue;
        var items = movieGroups[gk];
        visibleCount += items.length;
        html.push('<details class="stream-group" data-section="movies" data-group="' + esc(gk) + '"><summary>' +
          esc(display) + '<span class="stream-group-count">' + items.length + '</span></summary></details>');
      }
      summaryEl.textContent = visibleCount.toLocaleString() + ' movies in ' + html.length + ' group' + (html.length !== 1 ? 's' : '');
      groupsContainer.innerHTML = html.length > 0 ? html.join('') :
        '<div style="padding:40px 16px;text-align:center;color:var(--text-muted)">' +
        (searchTerm ? 'No groups match "' + esc(searchInput.value) + '"' : 'No movies found') + '</div>';
    }

    function renderSeriesTab() {
      var html = [];
      var visibleCount = 0;
      var seriesKeys = Object.keys(seriesGroups).sort();
      for (var si = 0; si < seriesKeys.length; si++) {
        var sk = seriesKeys[si];
        var display = sk.replace(/^(TV|Movie)\|/, '');
        if (searchTerm && display.toLowerCase().indexOf(searchTerm) === -1) continue;
        var items = seriesGroups[sk];
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
        renderActiveTab();
      }, 300);
    });

    groupsContainer._groupMaps = { movies: movieGroups, series: seriesGroups };
    bindStreamGroupEvents(groupsContainer);

    container.innerHTML = '';
    container.appendChild(tabBar);
    var headerDiv = document.createElement('div');
    headerDiv.className = 'stream-groups-header';
    headerDiv.appendChild(summaryEl);
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
        if (searchTerm && lDisplay.toLowerCase().indexOf(searchTerm) === -1) continue;
        var items = liveGroups[lk];
        visibleCount += items.length;
        html.push('<details class="stream-group" data-section="live" data-group="' + esc(lk) + '"><summary>' +
          esc(lDisplay) + '<span class="stream-group-count">' + items.length + '</span></summary></details>');
      }

      summaryEl.textContent = visibleCount.toLocaleString() + ' streams in ' + html.length + ' group' + (html.length !== 1 ? 's' : '');
      if (html.length === 0) {
        groupsContainer.innerHTML = '<div style="padding:40px 16px;text-align:center;color:var(--text-muted)">' +
          (searchTerm ? 'No groups match "' + esc(searchInput.value) + '"' : 'No streams found') + '</div>';
        return;
      }
      groupsContainer.innerHTML = html.join('');
    }

    searchInput.addEventListener('input', function() {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(function() {
        searchTerm = searchInput.value.toLowerCase();
        renderGroups();
      }, 300);
    });

    groupsContainer._groupMaps = { live: liveGroups };
    bindStreamGroupEvents(groupsContainer);

    container.innerHTML = '';
    var headerDiv = document.createElement('div');
    headerDiv.className = 'stream-groups-header';
    headerDiv.appendChild(summaryEl);
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
      '<div class="modal-content">' +
      '<div class="modal-header">Add "' + esc(streamName) + '" to Channel</div>' +
      '<div class="modal-body">';

    if (channels.length === 0) {
      html += '<p>No channels available. Create a channel first.</p>';
    } else {
      html += '<div class="channel-pick-list">';
      for (var i = 0; i < channels.length; i++) {
        var ch = channels[i];
        var alreadyAssigned = (ch.stream_ids || []).indexOf(streamID) >= 0;
        var badge = alreadyAssigned ? ' <span class="badge badge-live">assigned</span>' : '';
        html += '<div class="channel-pick-item' + (alreadyAssigned ? ' disabled' : '') + '" data-channel-id="' + esc(ch.id) + '"' +
          (alreadyAssigned ? '' : ' style="cursor:pointer"') + '>' +
          esc(ch.name) + ' (#' + ch.number + ')' + badge + '</div>';
      }
      html += '</div>';
    }

    html += '</div><div class="modal-footer">' +
      '<button class="btn btn-ghost" id="close-channel-modal">Cancel</button>' +
      '</div></div></div>';

    document.body.insertAdjacentHTML('beforeend', html);

    document.getElementById('close-channel-modal').addEventListener('click', function() {
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
  var channelStreams = [];

  async function renderChannels(el) {
    var user = api.user;
    var isAdmin = user && user.is_admin;

    var headerButtons = '';
    if (isAdmin) {
      headerButtons = '<div style="display:flex;gap:8px;margin-bottom:16px">' +
        '<button class="btn btn-primary" id="add-channel-btn">' + icons.plus + ' Add Channel</button>' +
        '<button class="btn btn-ghost" id="manage-groups-btn">Manage Groups</button>' +
        '</div>';
    }

    el.innerHTML = '<h1 class="page-title">Channels</h1>' +
      headerButtons +
      '<div class="search-bar">' + icons.search + '<input id="channel-search" placeholder="Search channels..."></div>' +
      '<div id="channel-list"><div class="skeleton" style="height:200px"></div></div>' +
      '<div id="channel-form" style="display:none" class="card">' +
      '<div class="card-title" id="channel-form-title">New Channel</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="ch-name" placeholder="BBC One"></div>' +
      '<div class="form-group"><label class="form-label">Number</label><input class="form-input" id="ch-number" type="number" min="0" placeholder="1"></div>' +
      '<div class="form-group"><label class="form-label">Group</label><select class="form-input" id="ch-group"><option value="">None</option></select></div>' +
      '<div class="form-group"><label class="form-label">Logo URL</label><input class="form-input" id="ch-logo" placeholder="http://example.com/logo.png"></div>' +
      '<div class="form-group"><label class="form-label">Streams</label>' +
      '<select class="form-input" id="ch-streams" multiple style="height:120px"></select>' +
      '<span class="field-hint">Hold Ctrl/Cmd to select multiple</span></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="ch-enabled" checked> Enabled</label></div>' +
      '<div style="display:flex;gap:8px">' +
      '<button class="btn btn-primary" id="save-channel-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-channel-btn">Cancel</button></div></div>' +
      '<div id="group-panel" style="display:none" class="card">' +
      '<div style="display:flex;justify-content:space-between;align-items:center">' +
      '<div class="card-title">Channel Groups</div>' +
      '<button class="btn btn-ghost" id="close-groups-btn" style="padding:4px 8px">&times;</button></div>' +
      '<div id="group-list"></div>' +
      '<div style="display:flex;gap:8px;margin-top:12px">' +
      '<input class="form-input" id="new-group-name" placeholder="Group name" style="flex:1">' +
      '<button class="btn btn-primary" id="create-group-btn">Add</button>' +
      '</div></div>';

    var channelEditId = null;
    var formEl = document.getElementById('channel-form');
    var groupPanel = document.getElementById('group-panel');

    try {
      var groupResp = await api.get('/api/channel-groups');
      channelGroups = await groupResp.json();
      if (!Array.isArray(channelGroups)) channelGroups = [];
    } catch (e) { channelGroups = []; }

    try {
      var streamResp = await api.get('/api/streams');
      channelStreams = await streamResp.json();
      if (!Array.isArray(channelStreams)) channelStreams = [];
    } catch (e) { channelStreams = []; }

    function populateForm(ch) {
      var groupSelect = document.getElementById('ch-group');
      groupSelect.innerHTML = '<option value="">None</option>';
      for (var gi = 0; gi < channelGroups.length; gi++) {
        var g = channelGroups[gi];
        var sel = ch && ch.group_id === g.id ? ' selected' : '';
        groupSelect.innerHTML += '<option value="' + esc(g.id) + '"' + sel + '>' + esc(g.name) + '</option>';
      }
      var streamSelect = document.getElementById('ch-streams');
      streamSelect.innerHTML = '';
      var existingStreams = ch && ch.stream_ids ? ch.stream_ids : [];
      for (var si = 0; si < channelStreams.length; si++) {
        var st = channelStreams[si];
        var selected = existingStreams.indexOf(st.id) >= 0 ? ' selected' : '';
        streamSelect.innerHTML += '<option value="' + esc(st.id) + '"' + selected + '>' + esc(st.name) + '</option>';
      }
    }

    if (isAdmin) {
      document.getElementById('add-channel-btn').addEventListener('click', function() {
        channelEditId = null;
        document.getElementById('channel-form-title').textContent = 'New Channel';
        document.getElementById('save-channel-btn').textContent = 'Create';
        document.getElementById('ch-name').value = '';
        document.getElementById('ch-number').value = '';
        document.getElementById('ch-logo').value = '';
        document.getElementById('ch-enabled').checked = true;
        populateForm(null);
        formEl.style.display = 'block';
        groupPanel.style.display = 'none';
      });

      document.getElementById('manage-groups-btn').addEventListener('click', function() {
        formEl.style.display = 'none';
        groupPanel.style.display = groupPanel.style.display === 'none' ? 'block' : 'none';
        renderGroupList();
      });

      document.getElementById('cancel-channel-btn').addEventListener('click', function() { formEl.style.display = 'none'; });
      document.getElementById('close-groups-btn').addEventListener('click', function() { groupPanel.style.display = 'none'; });

      document.getElementById('save-channel-btn').addEventListener('click', async function() {
        var name = document.getElementById('ch-name').value.trim();
        var number = parseInt(document.getElementById('ch-number').value) || 0;
        var groupId = document.getElementById('ch-group').value;
        var logoUrl = document.getElementById('ch-logo').value.trim();
        var enabled = document.getElementById('ch-enabled').checked;
        var streamSelect = document.getElementById('ch-streams');
        var selectedStreams = [];
        for (var i = 0; i < streamSelect.options.length; i++) {
          if (streamSelect.options[i].selected) selectedStreams.push(streamSelect.options[i].value);
        }
        if (!name) { toast('Name required', 'error'); return; }
        var payload = { name: name, number: number, group_id: groupId, logo_url: logoUrl, is_enabled: enabled, stream_ids: selectedStreams };
        try {
          var r;
          if (channelEditId) {
            r = await api.put('/api/channels/' + channelEditId, payload);
          } else {
            r = await api.post('/api/channels', payload);
          }
          if (r.ok) {
            toast(channelEditId ? 'Channel updated' : 'Channel created');
            formEl.style.display = 'none';
            renderChannels(el);
          } else {
            var data = await r.json().catch(function() { return {}; });
            toast(data.error || 'Failed to save channel', 'error');
          }
        } catch (err) {
          toast('Failed to save channel', 'error');
        }
      });

      document.getElementById('create-group-btn').addEventListener('click', async function() {
        var name = document.getElementById('new-group-name').value.trim();
        if (!name) { toast('Group name required', 'error'); return; }
        try {
          var r = await api.post('/api/channel-groups', { name: name });
          if (r.ok) {
            toast('Group created');
            document.getElementById('new-group-name').value = '';
            var groupResp2 = await api.get('/api/channel-groups');
            channelGroups = await groupResp2.json();
            if (!Array.isArray(channelGroups)) channelGroups = [];
            renderGroupList();
          } else {
            var data = await r.json().catch(function() { return {}; });
            toast(data.error || 'Failed to create group', 'error');
          }
        } catch (err) {
          toast('Failed to create group', 'error');
        }
      });
    }

    function renderGroupList() {
      var container = document.getElementById('group-list');
      if (!container) return;
      if (channelGroups.length === 0) {
        container.innerHTML = '<p style="color:var(--text-muted)">No groups yet</p>';
        return;
      }
      var html = '';
      for (var i = 0; i < channelGroups.length; i++) {
        var g = channelGroups[i];
        html += '<div style="display:flex;align-items:center;justify-content:space-between;padding:6px 0;border-bottom:1px solid var(--border)">' +
          '<span>' + esc(g.name) + '</span>' +
          '<button class="btn btn-sm btn-danger group-delete-btn" data-id="' + esc(g.id) + '" data-name="' + esc(g.name) + '">' + icons.trash + '</button>' +
          '</div>';
      }
      container.innerHTML = html;
      container.querySelectorAll('.group-delete-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var name = this.getAttribute('data-name');
          if (!confirm('Delete group "' + name + '"?')) return;
          try {
            var r = await api.del('/api/channel-groups/' + id);
            if (r.ok || r.status === 204) {
              toast('Group deleted');
              var groupResp3 = await api.get('/api/channel-groups');
              channelGroups = await groupResp3.json();
              if (!Array.isArray(channelGroups)) channelGroups = [];
              renderGroupList();
            } else {
              toast('Failed to delete group', 'error');
            }
          } catch (err) {
            toast('Failed to delete group', 'error');
          }
        });
      });
    }

    try {
      var resp = await api.get('/api/channels');
      var channels = await resp.json();
      if (!Array.isArray(channels)) channels = [];

      var groupMap = {};
      for (var gi = 0; gi < channelGroups.length; gi++) {
        groupMap[channelGroups[gi].id] = channelGroups[gi].name;
      }

      renderChannelTable(channels, '', groupMap, isAdmin, el, channelEditId, formEl, populateForm, function(id) { channelEditId = id; });
      document.getElementById('channel-search').addEventListener('input', function() {
        renderChannelTable(channels, this.value.toLowerCase(), groupMap, isAdmin, el, channelEditId, formEl, populateForm, function(id) { channelEditId = id; });
      });
    } catch (e) {
      document.getElementById('channel-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load channels</p></div>';
    }
  }

  function renderChannelTable(channels, filter, groupMap, isAdmin, el, channelEditId, formEl, populateForm, setEditId) {
    var container = document.getElementById('channel-list');
    if (!container) return;
    var filtered = channels;
    if (filter) {
      filtered = channels.filter(function(c) {
        var groupName = groupMap && groupMap[c.group_id] ? groupMap[c.group_id] : '';
        return (c.name || '').toLowerCase().indexOf(filter) >= 0 ||
               groupName.toLowerCase().indexOf(filter) >= 0;
      });
    }
    if (filtered.length === 0) {
      container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No channels found</p></div>';
      return;
    }

    filtered.sort(function(a, b) { return (a.number || 0) - (b.number || 0); });

    var html = '<table class="list-table"><thead><tr>' +
      '<th></th><th>#</th><th>Name</th><th>Group</th><th>Streams</th><th>Status</th><th></th>' +
      '</tr></thead><tbody>';
    for (var i = 0; i < filtered.length; i++) {
      var c = filtered[i];
      var logo = c.logo_url ? '<img class="logo" src="' + esc(c.logo_url) + '" alt="">' : '';
      var groupName = groupMap && groupMap[c.group_id] ? groupMap[c.group_id] : '-';
      var streamCount = c.stream_ids ? c.stream_ids.length : 0;
      var status = c.is_enabled !== false ? '<span class="badge badge-enabled">ON</span>' : '<span class="badge badge-disabled">OFF</span>';
      var actions = '<button class="btn btn-sm btn-primary play-btn" data-id="' + esc(c.stream_ids && c.stream_ids.length ? c.stream_ids[0] : c.id) + '" data-name="' + esc(c.name) + '">' + icons.play + '</button>';
      if (isAdmin) {
        actions += '<button class="btn btn-sm btn-ghost ch-edit-btn" data-id="' + esc(c.id) + '" title="Edit">' + icons.edit + '</button>' +
          '<button class="btn btn-sm btn-danger ch-delete-btn" data-id="' + esc(c.id) + '" data-name="' + esc(c.name) + '" title="Delete">' + icons.trash + '</button>';
      }
      html += '<tr>' +
        '<td>' + logo + '</td>' +
        '<td>' + esc(c.number || '-') + '</td>' +
        '<td>' + esc(c.name) + '</td>' +
        '<td>' + esc(groupName) + '</td>' +
        '<td>' + streamCount + '</td>' +
        '<td>' + status + '</td>' +
        '<td style="display:flex;gap:4px">' + actions + '</td>' +
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

    if (isAdmin) {
      container.querySelectorAll('.ch-edit-btn').forEach(function(btn) {
        btn.addEventListener('click', function() {
          var chId = this.getAttribute('data-id');
          var ch = null;
          for (var j = 0; j < channels.length; j++) {
            if (channels[j].id === chId) { ch = channels[j]; break; }
          }
          if (!ch) return;
          setEditId(chId);
          document.getElementById('channel-form-title').textContent = 'Edit Channel';
          document.getElementById('save-channel-btn').textContent = 'Update';
          document.getElementById('ch-name').value = ch.name || '';
          document.getElementById('ch-number').value = ch.number || '';
          document.getElementById('ch-logo').value = ch.logo_url || '';
          document.getElementById('ch-enabled').checked = ch.is_enabled !== false;
          populateForm(ch);
          formEl.style.display = 'block';
        });
      });

      container.querySelectorAll('.ch-delete-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var name = this.getAttribute('data-name');
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
        });
      });
    }
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
      });
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
            '<button class="player-ctrl-btn" id="vol-btn">\uD83D\uDD0A</button>' +
            '<button class="player-ctrl-btn" id="fs-btn">\u26F6</button>' +
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

    var isRecording = streamID.indexOf('rec:') === 0;
    var recID = isRecording ? streamID.substring(4) : null;

    try {
      var resp;
      if (isRecording) {
        resp = await api.post('/api/recordings/completed/' + recID + '/play');
      } else {
        resp = await api.post('/api/play/' + streamID);
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

      if (delivery === 'hls') {
        var hlsUrl = endpoints.playlist || (isRecording
          ? '/api/recordings/completed/' + recID + '/play/hls/playlist.m3u8'
          : '/api/play/' + streamID + '/hls/playlist.m3u8');
        playerState.hlsUrl = hlsUrl;
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
      if (statusEl) { statusEl.style.color = '#ff6b6b'; statusEl.textContent = 'Error'; }
      if (spinner) spinner.style.display = 'none';
      toast('Playback error: ' + e.message, 'error');
      return;
    }

    var seekPath = isRecording ? '/api/recordings/completed/' + recID + '/seek' : '/api/play/' + streamID + '/seek';
    bindPlayerControls(videoEl, streamID, seekPath);
    startBufferWatch(videoEl);
    startStatsWatch(videoEl);
  }

  function startHLS(videoEl, url) {
    if (playerState.hlsInstance) { playerState.hlsInstance.destroy(); playerState.hlsInstance = null; }

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
        } else {
          handleRetry();
        }
      }
    });
    playerState.hlsInstance = hls;
  }

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

      if (playerState.delivery === 'hls' && playerState.hlsUrl) {
        startHLS(playerState.videoEl, playerState.hlsUrl);
      } else if (playerState.delivery === 'mse') {
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
        return 'hvc1.' + profileIdc + '.4.' + tier + String(levelIdc);
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

  function startMSE(videoEl, streamID, endpoints) {
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

    var mseState = {
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
        mseEvictBuffer(sb, videoEl).then(function() {
          return mseWaitUpdate(sb);
        }).then(function() {
          sb.appendBuffer(data);
          return mseWaitUpdate(sb);
        }).then(next).catch(function(e) {
          if (e.name === 'QuotaExceededError') {
            mseEvictBuffer(sb, videoEl).then(function() {
              mseState.appendQueues[track].unshift(data);
              setTimeout(next, 500);
            });
            return;
          }
          mseState.appending[track] = false;
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
        if (mseState.stopped || !playerState.currentStreamID) return;
        fetch(baseUrl + '?seq=' + seq + '&gen=' + gen, { headers: authHeaders(), cache: 'no-store' })
          .then(function(resp) {
            if (mseState.stopped) return;
            if (resp.status === 410) {
              if (statusEl) { statusEl.textContent = 'Reconnecting...'; statusEl.style.color = '#ffa726'; }
              mseState.stopped = true;
              mseCleanup();
              setTimeout(function() { startMSE(videoEl, streamID, endpoints); }, 1000);
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
            if (track === 'video' && seq === 3 && !mseState.playStarted) {
              mseState.playStarted = true;
              var tryPlay = function() {
                if (mseState.videoSB && mseState.videoSB.buffered.length > 0) {
                  videoEl.currentTime = mseState.videoSB.buffered.start(0);
                  videoEl.play().catch(function() { setTimeout(tryPlay, 300); });
                } else {
                  setTimeout(tryPlay, 100);
                }
              };
              setTimeout(tryPlay, 100);
            }
            setTimeout(poll, 50);
          })
          .catch(function() {
            if (!mseState.stopped) setTimeout(poll, 1000);
          });
      }
      poll();
    }

    function mseCleanup() {
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
    }

    var origCleanup = playerState.cleanup.bind(playerState);
    playerState.cleanup = function() { mseCleanup(); origCleanup(); };

    if (statusEl) { statusEl.textContent = 'Initializing...'; statusEl.style.color = '#ffa726'; }

    fetch(debugUrl, { headers: authHeaders(), cache: 'no-store' })
      .then(function(r) { return r.ok ? r.json() : {}; })
      .then(function(debugInfo) {
        mseState.debugInfo = debugInfo;
        return fetchInitWithRetry(videoInitUrl, 60, 500);
      })
      .then(function(videoBuf) {
        if (mseState.stopped) return;
        var videoCodec = detectVideoCodec(videoBuf);
        var videoMime = 'video/mp4; codecs="' + videoCodec + '"';
        if (typeof MediaSource !== 'undefined' && !MediaSource.isTypeSupported(videoMime)) {
          videoMime = 'video/mp4; codecs="avc1.42E01E"';
        }
        var audioCodecStr = (mseState.debugInfo && mseState.debugInfo.audio_codec) || 'mp4a.40.2';
        var audioMime = 'audio/mp4; codecs="' + audioCodecStr + '"';

        mseState.mediaSource = new MediaSource();
        videoEl.src = URL.createObjectURL(mseState.mediaSource);

        mseState.mediaSource.addEventListener('sourceopen', function() {
          mseState.videoSB = mseState.mediaSource.addSourceBuffer(videoMime);
          mseState.videoSB.mode = 'segments';

          var hasAudio = !mseState.debugInfo || mseState.debugInfo.has_audio_init !== false;
          if (hasAudio) {
            try {
              mseState.audioSB = mseState.mediaSource.addSourceBuffer(audioMime);
              mseState.audioSB.mode = 'segments';
            } catch (e) { hasAudio = false; }
          }

          mseWaitUpdate(mseState.videoSB).then(function() {
            mseState.videoSB.appendBuffer(videoBuf);
            return mseWaitUpdate(mseState.videoSB);
          }).then(function() {
            if (hasAudio && mseState.audioSB) {
              return fetchInitWithRetry(audioInitUrl, 30, 500).then(function(audioBuf) {
                mseState.audioSB.appendBuffer(audioBuf);
                return mseWaitUpdate(mseState.audioSB);
              }).catch(function() {});
            }
          }).then(function() {
            if (statusEl) { statusEl.textContent = 'Buffering...'; statusEl.style.color = '#ffa726'; }
            var gen = String((mseState.debugInfo && mseState.debugInfo.generation) || 1);
            pollSegments('video', videoSegUrl, gen);
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

    var w = videoEl.videoWidth || 0;
    var h = videoEl.videoHeight || 0;
    var lines = [];

    var delivery = playerState.delivery || 'unknown';
    var mse = playerState.mseState;
    var debugInfo = mse && mse.debugInfo;

    if (debugInfo && debugInfo.codec_string) {
      lines.push('In: ' + esc(debugInfo.codec_string));
    }

    lines.push('Out: ' + esc(delivery.toUpperCase()) + (w > 0 ? ' ' + w + 'x' + h : ''));

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

    if (playerState.hlsInstance) {
      var hls = playerState.hlsInstance;
      var level = hls.currentLevel >= 0 ? hls.levels[hls.currentLevel] : null;
      if (level) {
        lines.push('Bitrate: ' + (level.bitrate / 1000).toFixed(0) + ' kbps');
      }
      lines.push('Level: ' + (hls.currentLevel + 1) + '/' + hls.levels.length);
    }

    if (debugInfo) {
      var mseInfo = [];
      if (debugInfo.video_segments != null) mseInfo.push('vseg=' + debugInfo.video_segments);
      if (debugInfo.audio_segments != null) mseInfo.push('aseg=' + debugInfo.audio_segments);
      if (debugInfo.generation) mseInfo.push('gen=' + debugInfo.generation);
      if (mseInfo.length) lines.push(mseInfo.join(' | '));
    }

    overlay.innerHTML = lines.join('<br>');
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

    videoEl.addEventListener('click', function() {
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

    document.addEventListener('keydown', function(e) {
      if (e.key === 's' || e.key === 'S') {
        var overlay = document.getElementById('stats-overlay');
        if (overlay) overlay.classList.toggle('visible');
      }
    });

    var ctrlTimer = setInterval(function() {
      if (!document.getElementById('player-wrapper')) { clearInterval(ctrlTimer); return; }
      var cur = videoEl.currentTime || 0;
      var dur = videoEl.duration;
      var effectiveDur = isFinite(dur) && dur > 0 ? dur : 0;
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
        var wasRecording = !!playerState.recordingID;
        playerState.cleanup();
        router.navigate(wasRecording ? 'recordings' : 'streams');
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
      '<div id="recording-list"><div class="skeleton" style="height:200px"></div></div>';

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

    async function loadRecordings() {
      try {
        var resp = await api.get('/api/recordings');
        var recordings = await resp.json();
        if (!Array.isArray(recordings)) recordings = [];
        var container = document.getElementById('recording-list');
        if (!container) return;

        var counts = { recording: 0, scheduled: 0, completed: 0, failed: 0 };
        for (var ci = 0; ci < recordings.length; ci++) {
          var status = recordings[ci].status || 'unknown';
          if (counts[status] !== undefined) counts[status]++;
          else counts.failed++;
        }
        var el1 = document.getElementById('rec-stat-active');
        var el2 = document.getElementById('rec-stat-scheduled');
        var el3 = document.getElementById('rec-stat-completed');
        var el4 = document.getElementById('rec-stat-failed');
        if (el1) el1.textContent = counts.recording;
        if (el2) el2.textContent = counts.scheduled;
        if (el3) el3.textContent = counts.completed;
        if (el4) el4.textContent = counts.failed;

        if (recordings.length === 0) {
          container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No recordings yet</p></div>';
          return;
        }

        recordings.sort(function(a, b) {
          var statusOrder = { recording: 0, scheduled: 1, completed: 2, failed: 3 };
          var sa = statusOrder[a.status] !== undefined ? statusOrder[a.status] : 4;
          var sb = statusOrder[b.status] !== undefined ? statusOrder[b.status] : 4;
          if (sa !== sb) return sa - sb;
          var ta = a.started_at || a.scheduled_start || '';
          var tb = b.started_at || b.scheduled_start || '';
          return tb.localeCompare(ta);
        });

        var html = '<table class="list-table"><thead><tr>' +
          '<th>Title</th><th>Channel</th><th>Status</th><th>Date</th><th>Duration</th><th>Size</th><th>Actions</th>' +
          '</tr></thead><tbody>';
        for (var i = 0; i < recordings.length; i++) {
          var r = recordings[i];
          var statusBadge = '';
          if (r.status === 'recording') {
            statusBadge = '<span class="badge badge-rec-active"><span class="recording-dot"></span> Recording</span>';
          } else if (r.status === 'completed') {
            statusBadge = '<span class="badge badge-rec-completed">Completed</span>';
          } else if (r.status === 'scheduled') {
            statusBadge = '<span class="badge badge-rec-scheduled">Scheduled</span>';
          } else if (r.status === 'failed' || r.status === 'cancelled') {
            statusBadge = '<span class="badge badge-rec-failed">' + esc(r.status) + '</span>';
          } else {
            statusBadge = '<span class="badge badge-disabled">' + esc(r.status || 'unknown') + '</span>';
          }

          var dateStr = '-';
          if (r.started_at) dateStr = new Date(r.started_at).toLocaleString();
          else if (r.scheduled_start) dateStr = new Date(r.scheduled_start).toLocaleString();

          var durStr = '-';
          if (r.started_at && r.stopped_at) {
            var durSec = (new Date(r.stopped_at) - new Date(r.started_at)) / 1000;
            durStr = formatDurationSec(durSec);
          } else if (r.status === 'recording' && r.started_at) {
            var elapsed = (Date.now() - new Date(r.started_at).getTime()) / 1000;
            durStr = '<span class="rec-duration-live" data-started="' + esc(r.started_at) + '">' + formatDurationSec(elapsed) + '</span>';
          }

          var sizeStr = formatBytes(r.file_size);

          var actions = '';
          if (r.status === 'completed') {
            actions += '<button class="btn btn-sm btn-primary rec-play-btn" data-id="' + esc(r.id) + '" data-title="' + esc(r.title || r.stream_name || r.id) + '" title="Play">' + icons.play + '</button>';
            actions += '<a class="btn btn-sm btn-ghost" href="/api/recordings/completed/' + esc(r.id) + '/stream" target="_blank" download title="Download">' + icons.download + '</a>';
          }
          if (isAdmin && (r.status === 'completed' || r.status === 'failed')) {
            actions += '<button class="btn btn-sm btn-icon btn-danger rec-del-btn" data-id="' + esc(r.id) + '" title="Delete">' + icons.trash + '</button>';
          }
          if (isAdmin && r.status === 'scheduled') {
            actions += '<button class="btn btn-sm btn-icon btn-danger rec-cancel-btn" data-id="' + esc(r.id) + '" title="Cancel">' + icons.trash + '</button>';
          }

          html += '<tr>' +
            '<td>' + esc(r.title || r.stream_name || r.stream_id) + '</td>' +
            '<td>' + esc(r.channel_name || '-') + '</td>' +
            '<td>' + statusBadge + '</td>' +
            '<td style="font-size:12px">' + esc(dateStr) + '</td>' +
            '<td>' + durStr + '</td>' +
            '<td>' + esc(sizeStr) + '</td>' +
            '<td><div class="actions-cell">' + actions + '</div></td>' +
            '</tr>';
        }
        html += '</tbody></table>';
        container.innerHTML = html;

        container.querySelectorAll('.rec-play-btn').forEach(function(btn) {
          btn.addEventListener('click', function() {
            playRecording(this.getAttribute('data-id'), this.getAttribute('data-title'));
          });
        });

        container.querySelectorAll('.rec-del-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var recID = this.getAttribute('data-id');
            if (!confirm('Delete this recording? The file will be removed from disk.')) return;
            var delResp = await api.del('/api/recordings/completed/' + recID).catch(function() {});
            if (delResp && (delResp.status === 204 || delResp.ok)) {
              toast('Recording deleted');
              loadRecordings();
            } else {
              toast('Failed to delete recording', 'error');
            }
          });
        });

        container.querySelectorAll('.rec-cancel-btn').forEach(function(btn) {
          btn.addEventListener('click', async function() {
            var recID = this.getAttribute('data-id');
            if (!confirm('Cancel this scheduled recording?')) return;
            var delResp = await api.del('/api/recordings/schedule/' + recID).catch(function() {});
            if (delResp && (delResp.status === 204 || delResp.ok)) {
              toast('Recording cancelled');
              loadRecordings();
            } else {
              toast('Failed to cancel recording', 'error');
            }
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
      if (router.current !== 'recordings') {
        clearInterval(recordingRefreshTimer);
        recordingRefreshTimer = null;
        return;
      }
      var liveDurs = document.querySelectorAll('.rec-duration-live');
      liveDurs.forEach(function(span) {
        var started = span.getAttribute('data-started');
        if (started) {
          var elapsed = (Date.now() - new Date(started).getTime()) / 1000;
          span.textContent = formatDurationSec(elapsed);
        }
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
      await loadFavorites();
      var favResp = await api.get('/api/favorites');
      var favs = await favResp.json();
      if (!Array.isArray(favs)) favs = [];

      var streamIDs = favs.map(function(f) { return f.stream_id; });
      var allStreams = [];
      if (streamIDs.length > 0) {
        var streamResp = await api.get('/api/streams');
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
      '<th></th><th>Name</th><th>Group</th><th>Source</th><th>Actions</th>' +
      '</tr></thead><tbody>';
    for (var i = 0; i < filtered.length; i++) {
      var s = filtered[i];
      var logo = s.tvg_logo ? '<img class="logo" src="/logo?url=' + encodeURIComponent(s.tvg_logo) + '" alt="">' : '';
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
        });
      });
    });
  }

  async function renderSources(el) {
    el.innerHTML = '<h1 class="page-title">Sources</h1>' +
      '<div style="margin-bottom:16px;display:flex;gap:8px">' +
      '<button class="btn btn-primary" id="add-m3u-btn">' + icons.plus + ' Add M3U Source</button>' +
      '<button class="btn btn-primary" id="add-tvp-btn">' + icons.plus + ' Add TVP Streams Source</button>' +
      '<button class="btn btn-primary" id="add-xtream-btn">' + icons.plus + ' Add Xtream Source</button>' +
      '<button class="btn btn-primary" id="add-hdhr-btn">' + icons.plus + ' Add HDHomeRun</button>' +
      '<button class="btn btn-primary" id="add-satip-btn">' + icons.plus + ' Add SAT>IP Source</button>' +
      '<button class="btn btn-ghost" id="discover-hdhr-btn">Discover HDHomeRun</button>' +
      '</div>' +
      '<div id="source-list"><div class="skeleton" style="height:200px"></div></div>' +
      '<div id="add-m3u-form" style="display:none" class="card">' +
      '<div class="card-title">New M3U Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="src-name" placeholder="My IPTV Provider"></div>' +
      '<div class="form-group"><label class="form-label">URL</label><input class="form-input" id="src-url" placeholder="http://example.com/playlist.m3u"></div>' +
      '<div class="form-group"><label class="form-label">Username (optional)</label><input class="form-input" id="src-username"></div>' +
      '<div class="form-group"><label class="form-label">Password (optional)</label><input class="form-input" id="src-password" type="password"></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="src-wireguard"> Route through WireGuard</label></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-m3u-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-m3u-btn">Cancel</button></div></div>' +
      '<div id="add-tvp-form" style="display:none" class="card">' +
      '<div class="card-title">New TVP Streams Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="tvp-name" placeholder="My Media Library"></div>' +
      '<div class="form-group"><label class="form-label">URL</label><input class="form-input" id="tvp-url" placeholder="https://streams.example.com/playlist.m3u"></div>' +
      '<div class="form-group"><label class="form-label">Enrollment Token</label><input class="form-input" id="tvp-token" placeholder="One-time enrollment token"></div>' +
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
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="xt-wireguard"> Route through WireGuard</label></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-xtream-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-xtream-btn">Cancel</button></div></div>' +
      '<div id="add-hdhr-form" style="display:none" class="card">' +
      '<div class="card-title">New HDHomeRun Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="hdhr-name" placeholder="HDHomeRun" value="HDHomeRun"></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="hdhr-enabled" checked> Enabled</label></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-hdhr-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-hdhr-btn">Cancel</button></div></div>' +
      '<div id="add-satip-form" style="display:none" class="card">' +
      '<div class="card-title">New SAT>IP Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="satip-name" placeholder="Home SAT>IP Server"></div>' +
      '<div class="form-group"><label class="form-label">Host / IP Address</label><input class="form-input" id="satip-host" placeholder="192.168.1.100"></div>' +
      '<div class="form-group"><label class="form-label">HTTP Port</label><input class="form-input" id="satip-port" type="number" value="8875" min="1" max="65535"></div>' +
      '<div class="form-group"><label class="form-label">Transmitter File (optional)</label><input class="form-input" id="satip-transmitter" placeholder="dvb-t/uk-Crystal_Palace"></div>' +
      '<div class="form-group"><label class="form-label">Max Streams (0 = unlimited)</label><input class="form-input" id="satip-maxstreams" type="number" value="0" min="0"></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="satip-enabled" checked> Enabled</label></div>' +
      '<div style="display:flex;gap:8px"><button class="btn btn-primary" id="create-satip-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-satip-btn">Cancel</button></div></div>' +
      '<div id="hdhr-discover-modal" style="display:none"></div>';

    var allForms = ['add-m3u-form', 'add-tvp-form', 'add-xtream-form', 'add-hdhr-form', 'add-satip-form'];
    function hideAllForms() {
      allForms.forEach(function(id) { var f = document.getElementById(id); if (f) f.style.display = 'none'; });
    }

    document.getElementById('add-m3u-btn').addEventListener('click', function() {
      var f = document.getElementById('add-m3u-form');
      var wasHidden = f.style.display === 'none';
      hideAllForms();
      if (wasHidden) f.style.display = 'block';
    });
    document.getElementById('cancel-m3u-btn').addEventListener('click', function() {
      document.getElementById('add-m3u-form').style.display = 'none';
    });
    document.getElementById('add-tvp-btn').addEventListener('click', function() {
      var f = document.getElementById('add-tvp-form');
      var wasHidden = f.style.display === 'none';
      hideAllForms();
      if (wasHidden) f.style.display = 'block';
    });
    document.getElementById('cancel-tvp-btn').addEventListener('click', function() {
      document.getElementById('add-tvp-form').style.display = 'none';
    });
    document.getElementById('add-xtream-btn').addEventListener('click', function() {
      var f = document.getElementById('add-xtream-form');
      var wasHidden = f.style.display === 'none';
      hideAllForms();
      if (wasHidden) f.style.display = 'block';
    });
    document.getElementById('cancel-xtream-btn').addEventListener('click', function() {
      document.getElementById('add-xtream-form').style.display = 'none';
    });
    document.getElementById('add-hdhr-btn').addEventListener('click', function() {
      var f = document.getElementById('add-hdhr-form');
      var wasHidden = f.style.display === 'none';
      hideAllForms();
      if (wasHidden) f.style.display = 'block';
    });
    document.getElementById('cancel-hdhr-btn').addEventListener('click', function() {
      document.getElementById('add-hdhr-form').style.display = 'none';
    });
    document.getElementById('add-satip-btn').addEventListener('click', function() {
      var f = document.getElementById('add-satip-form');
      var wasHidden = f.style.display === 'none';
      hideAllForms();
      if (wasHidden) f.style.display = 'block';
    });
    document.getElementById('cancel-satip-btn').addEventListener('click', function() {
      document.getElementById('add-satip-form').style.display = 'none';
    });

    document.getElementById('create-hdhr-btn').addEventListener('click', async function() {
      var name = document.getElementById('hdhr-name').value.trim();
      var enabled = document.getElementById('hdhr-enabled').checked;
      if (!name) { toast('Name required', 'error'); return; }
      try {
        var r = await api.post('/api/sources/hdhr', { name: name, is_enabled: enabled });
        if (r.ok) {
          toast('HDHomeRun source created');
          document.getElementById('add-hdhr-form').style.display = 'none';
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to create source', 'error');
        }
      } catch (err) {
        toast('Failed to create source', 'error');
      }
    });

    document.getElementById('create-satip-btn').addEventListener('click', async function() {
      var name = document.getElementById('satip-name').value.trim();
      var host = document.getElementById('satip-host').value.trim();
      var port = parseInt(document.getElementById('satip-port').value) || 8875;
      var transmitter = document.getElementById('satip-transmitter').value.trim();
      var maxStreams = parseInt(document.getElementById('satip-maxstreams').value) || 0;
      var enabled = document.getElementById('satip-enabled').checked;
      if (!name || !host) { toast('Name and host required', 'error'); return; }
      try {
        var r = await api.post('/api/sources/satip', {
          name: name, host: host, http_port: port,
          transmitter_file: transmitter, max_streams: maxStreams, is_enabled: enabled
        });
        if (r.ok) {
          toast('SAT>IP source created');
          document.getElementById('add-satip-form').style.display = 'none';
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to create source', 'error');
        }
      } catch (err) {
        toast('Failed to create source', 'error');
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
      var url = document.getElementById('src-url').value.trim();
      var username = document.getElementById('src-username').value.trim();
      var password = document.getElementById('src-password').value;
      var wg = document.getElementById('src-wireguard').checked;
      if (!name || !url) { toast('Name and URL required', 'error'); return; }
      try {
        var r = await api.post('/api/sources/m3u', { name: name, url: url, username: username, password: password, use_wireguard: wg });
        if (r.ok) {
          toast('Source created, refreshing...');
          document.getElementById('add-m3u-form').style.display = 'none';
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to create source', 'error');
        }
      } catch (err) {
        toast('Failed to create source', 'error');
      }
    });

    document.getElementById('create-tvp-btn').addEventListener('click', async function() {
      var name = document.getElementById('tvp-name').value.trim();
      var url = document.getElementById('tvp-url').value.trim();
      var token = document.getElementById('tvp-token').value.trim();
      var wg = document.getElementById('tvp-wireguard').checked;
      if (!name || !url) { toast('Name and URL required', 'error'); return; }
      try {
        var r = await api.post('/api/sources/tvpstreams', { name: name, url: url, enrollment_token: token, use_wireguard: wg });
        if (r.ok) {
          toast('TVP Streams source created' + (token ? ', enrolling...' : ', refreshing...'));
          document.getElementById('add-tvp-form').style.display = 'none';
          renderSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to create source', 'error');
        }
      } catch (err) {
        toast('Failed to create source', 'error');
      }
    });

    document.getElementById('create-xtream-btn').addEventListener('click', async function() {
      var name = document.getElementById('xt-name').value.trim();
      var server = document.getElementById('xt-server').value.trim();
      var username = document.getElementById('xt-username').value.trim();
      var password = document.getElementById('xt-password').value;
      var maxStreams = parseInt(document.getElementById('xt-maxstreams').value) || 0;
      var wg = document.getElementById('xt-wireguard').checked;
      if (!name || !server || !username || !password) { toast('Name, server, username, and password required', 'error'); return; }
      try {
        var r = await api.post('/api/sources/xtream', { name: name, server: server, username: username, password: password, max_streams: maxStreams, use_wireguard: wg });
        if (r.ok) {
          toast('Xtream source created, refreshing...');
          document.getElementById('add-xtream-form').style.display = 'none';
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

      var html = '';
      for (var i = 0; i < sources.length; i++) {
        var s = sources[i];
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
            if (!r.ok) { detail.innerHTML = '<div style="color:var(--danger)">Failed to load account info</div>'; return; }
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
            detail.innerHTML = '<div style="color:var(--danger)">Failed to load account info</div>';
          }
        });
      });

      container.querySelectorAll('.refresh-source-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var id = this.getAttribute('data-id');
          var self = this;
          self.style.opacity = '0.5';
          self.style.pointerEvents = 'none';
          try {
            var r = await api.post('/api/sources/' + id + '/refresh', {});
            if (r.ok || r.status === 202) {
              toast('Refresh started');
              setTimeout(function() { self.style.opacity = '1'; self.style.pointerEvents = ''; renderSources(el); }, 5000);
            } else {
              toast('Failed to refresh', 'error');
              self.style.opacity = '1';
              self.style.pointerEvents = '';
            }
          } catch (err) {
            toast('Failed to refresh', 'error');
            self.style.opacity = '1';
            self.style.pointerEvents = '';
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
    var sel = container.querySelector('#' + selectId);
    if (!sel) return;
    sel.addEventListener('change', function() {
      saveSetting(settingKey, sel.value);
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
            var dhwItems = dhwOpts.map(function(d) { return { value: d.name, label: d.name + ' (' + d.platform + ')' }; });
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
        ], getSetting(settings, 'default_delivery') || 'mse') +
      '</div>' +
      '<div class="settings-field">' +
        '<label>Container</label>' +
        makeSelect('setting-container', [
          { value: 'mp4', label: 'MP4' },
          { value: 'mpegts', label: 'MPEG-TS' }
        ], getSetting(settings, 'default_container') || 'mp4') +
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
      '<div class="settings-field">' +
        '<label>Audio Codec</label>' +
        makeSelect('setting-audio-codec', [
          { value: 'aac', label: 'AAC' },
          { value: 'copy', label: 'Passthrough (copy)' },
          { value: 'mp3', label: 'MP3' },
          { value: 'opus', label: 'Opus' },
          { value: 'ac3', label: 'AC3' }
        ], getSetting(settings, 'default_audio_codec') || 'aac') +
      '</div>' +
      '</div></div>';

    html += '<div class="settings-section">' +
      '<div class="settings-section-header">Recording Settings</div>' +
      '<div class="settings-section-body">' +
      '<div class="settings-section-desc">Default codec for scheduled recordings. Container is always MP4.</div>' +
      '<div class="settings-field">' +
        '<label>Recording Codec</label>' +
        makeSelect('setting-rec-codec', [
          { value: 'copy', label: 'Passthrough (copy)' },
          { value: 'h264', label: 'H.264' },
          { value: 'h265', label: 'H.265 / HEVC' },
          { value: 'av1', label: 'AV1' }
        ], getSetting(settings, 'recording_video_codec') || 'copy') +
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
        '<label>User Agent</label>' +
        '<input type="text" id="setting-user-agent" value="' + esc(getSetting(settings, 'user_agent')) + '" placeholder="MediaHub">' +
      '</div>' +
      '<div class="settings-field">' +
        '<label>DLNA Enabled</label>' +
        '<input type="checkbox" id="setting-dlna"' + (getSetting(settings, 'dlna_enabled') === 'true' ? ' checked' : '') + '>' +
        '<span class="field-hint">Advertise as DLNA MediaServer on the network</span>' +
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

    container.innerHTML = html;

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

    bindAutoSave(container, 'setting-delivery', 'default_delivery');
    bindAutoSave(container, 'setting-container', 'default_container');
    bindAutoSave(container, 'setting-video-codec', 'default_video_codec');
    bindAutoSave(container, 'setting-audio-codec', 'default_audio_codec');
    bindAutoSave(container, 'setting-rec-codec', 'recording_video_codec');

    bindTextSave(container, 'setting-base-url', 'base_url');
    bindTextSave(container, 'setting-user-agent', 'user_agent');
    bindToggle(container, 'setting-dlna', 'dlna_enabled');
    bindTextSave(container, 'setting-tmdb-key', 'tmdb_api_key');
  }

  async function renderUsers(el) {
    var currentUser = api.user;
    el.innerHTML = '<h1 class="page-title">Users</h1>' +
      '<div style="margin-bottom:16px"><button class="btn btn-primary" id="add-user-btn">' + icons.plus + ' Add User</button></div>' +
      '<div id="user-list"><div class="skeleton" style="height:200px"></div></div>' +
      '<div id="add-user-form" style="display:none" class="card">' +
      '<div class="card-title">New User</div>' +
      '<div class="form-group"><label class="form-label">Username</label><input class="form-input" id="new-username" placeholder="username"></div>' +
      '<div class="form-group"><label class="form-label">Password</label><input class="form-input" id="new-password" type="password" placeholder="password"></div>' +
      '<div class="form-group"><label class="form-label">Role</label>' +
      '<select class="form-input" id="new-role"><option value="standard">Standard</option><option value="admin">Admin</option><option value="jellyfin">Jellyfin</option></select></div>' +
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

      function roleBadgeClass(role) {
        if (role === 'admin') return 'badge-admin';
        if (role === 'jellyfin') return 'badge-jellyfin';
        return 'badge-standard';
      }

      function renderUserTable() {
        var html = '<table class="list-table"><thead><tr>' +
          '<th>Username</th><th>Role</th><th>Actions</th>' +
          '</tr></thead><tbody>';
        for (var i = 0; i < users.length; i++) {
          var u = users[i];
          var username = u.Username || u.username;
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
            '<td><div class="user-role-cell" data-uid="' + esc(uid) + '">' +
            '<span class="badge ' + roleClass + '" id="role-badge-' + esc(uid) + '">' + esc(role) + '</span>' +
            '<select class="user-role-select" id="role-select-' + esc(uid) + '" data-uid="' + esc(uid) + '" style="display:none">' +
            '<option value="admin"' + (role === 'admin' ? ' selected' : '') + '>Admin</option>' +
            '<option value="standard"' + (role === 'standard' ? ' selected' : '') + '>Standard</option>' +
            '<option value="jellyfin"' + (role === 'jellyfin' ? ' selected' : '') + '>Jellyfin</option>' +
            '</select></div></td>' +
            '<td><div class="actions-cell">' +
            '<button class="btn btn-sm btn-ghost user-edit-role-btn" data-uid="' + esc(uid) + '" title="Change role">' + icons.edit + '</button>' +
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
          '<div class="modal-header">Change Password - ' + esc(username) + '</div>' +
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

      renderUserTable();
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
          statusBar.innerHTML = '<div class="card" style="background:rgba(52,211,153,0.08);border:1px solid rgba(52,211,153,0.3);padding:12px 16px;display:flex;align-items:center;gap:12px">' +
            '<span style="display:inline-block;width:12px;height:12px;border-radius:50%;background:var(--success)"></span>' +
            '<div style="flex:1;min-width:0">' +
            '<div style="color:var(--success);font-weight:600;font-size:14px">Connected</div>' +
            '<div style="color:var(--text);font-size:13px;margin-top:2px">' + esc(status.profile_name) + ' &mdash; ' + esc(status.endpoint) + '</div>' +
            '</div>' +
            '<div style="text-align:right;flex-shrink:0">' +
            '<div style="color:var(--text-dim);font-size:11px;text-transform:uppercase;letter-spacing:.5px">Proxy</div>' +
            '<div style="color:var(--text);font-size:14px;font-weight:600;font-family:monospace">127.0.0.1:' + status.proxy_port + '</div>' +
            '</div>' +
            '</div>';
        } else {
          statusBar.innerHTML = '<div class="card" style="background:rgba(251,191,36,0.08);border:1px solid rgba(251,191,36,0.3);padding:12px 16px;display:flex;align-items:center;gap:12px">' +
            '<span style="display:inline-block;width:12px;height:12px;border-radius:50%;background:var(--warning)"></span>' +
            '<div style="color:var(--text)"><strong style="color:var(--warning)">Disconnected</strong> <span style="color:var(--text-dim)">&mdash; No active WireGuard tunnel</span></div>' +
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

  async function renderEPGSources(el) {
    el.innerHTML = '<h1 class="page-title">EPG Sources</h1>' +
      '<div style="margin-bottom:16px"><button class="btn btn-primary" id="add-epg-btn">' + icons.plus + ' Add EPG Source</button></div>' +
      '<div id="epg-list"><div class="skeleton" style="height:200px"></div></div>' +
      '<div id="epg-form" style="display:none" class="card">' +
      '<div class="card-title" id="epg-form-title">New EPG Source</div>' +
      '<div class="form-group"><label class="form-label">Name</label><input class="form-input" id="epg-name" placeholder="UK XMLTV"></div>' +
      '<div class="form-group"><label class="form-label">XMLTV URL</label><input class="form-input" id="epg-url" placeholder="http://example.com/guide.xml"></div>' +
      '<div class="form-group"><label class="form-label"><input type="checkbox" id="epg-wireguard"> Route through WireGuard</label></div>' +
      '<div style="display:flex;gap:8px">' +
      '<button class="btn btn-primary" id="save-epg-btn">Create</button>' +
      '<button class="btn btn-ghost" id="cancel-epg-btn">Cancel</button></div></div>';

    var epgEditId = null;
    var addBtn = document.getElementById('add-epg-btn');
    var formEl = document.getElementById('epg-form');
    addBtn.addEventListener('click', function() {
      epgEditId = null;
      document.getElementById('epg-form-title').textContent = 'New EPG Source';
      document.getElementById('save-epg-btn').textContent = 'Create';
      document.getElementById('epg-name').value = '';
      document.getElementById('epg-url').value = '';
      document.getElementById('epg-wireguard').checked = false;
      formEl.style.display = 'block';
    });
    document.getElementById('cancel-epg-btn').addEventListener('click', function() { formEl.style.display = 'none'; });

    document.getElementById('save-epg-btn').addEventListener('click', async function() {
      var name = document.getElementById('epg-name').value.trim();
      var url = document.getElementById('epg-url').value.trim();
      var wg = document.getElementById('epg-wireguard').checked;
      if (!name || !url) { toast('Name and URL required', 'error'); return; }
      try {
        var r;
        if (epgEditId) {
          r = await api.put('/api/epg/sources/' + epgEditId, { name: name, url: url, use_wireguard: wg });
        } else {
          r = await api.post('/api/epg/sources', { name: name, url: url, use_wireguard: wg });
        }
        if (r.ok) {
          toast(epgEditId ? 'EPG source updated' : 'EPG source created');
          formEl.style.display = 'none';
          renderEPGSources(el);
        } else {
          var data = await r.json().catch(function() { return {}; });
          toast(data.error || 'Failed to save EPG source', 'error');
        }
      } catch (err) {
        toast('Failed to save EPG source', 'error');
      }
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
        html += '<tr>' +
          '<td>' + esc(s.name) + '</td>' +
          '<td>' + (s.channel_count || 0) + '</td>' +
          '<td>' + (s.program_count || 0) + '</td>' +
          '<td>' + esc(lastRefreshed) + '</td>' +
          '<td>' + statusBadge + '</td>' +
          '<td style="display:flex;gap:4px">' +
          '<button class="btn btn-sm btn-ghost epg-refresh-btn" data-id="' + esc(s.id) + '" title="Refresh">' + icons.refresh + '</button>' +
          '<button class="btn btn-sm btn-ghost epg-edit-btn" data-id="' + esc(s.id) + '" data-name="' + esc(s.name) + '" data-url="' + esc(s.url) + '" data-wg="' + (s.use_wireguard ? '1' : '0') + '" title="Edit">' + icons.edit + '</button>' +
          '<button class="btn btn-sm btn-danger epg-delete-btn" data-id="' + esc(s.id) + '" data-name="' + esc(s.name) + '" title="Delete">' + icons.trash + '</button>' +
          '</td></tr>';
      }
      html += '</tbody></table>';
      container.innerHTML = html;

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
          epgEditId = this.getAttribute('data-id');
          document.getElementById('epg-form-title').textContent = 'Edit EPG Source';
          document.getElementById('save-epg-btn').textContent = 'Update';
          document.getElementById('epg-name').value = this.getAttribute('data-name') || '';
          document.getElementById('epg-url').value = this.getAttribute('data-url') || '';
          document.getElementById('epg-wireguard').checked = this.getAttribute('data-wg') === '1';
          formEl.style.display = 'block';
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

  var activityRefreshTimer = null;

  async function renderActivity(el) {
    el.innerHTML = '<h1 class="page-title">Activity</h1>' +
      '<div class="stat-grid">' +
      '<div class="stat-card"><div class="stat-value" id="stat-active-count">-</div><div class="stat-label">Active Viewers</div></div>' +
      '</div>' +
      '<div id="activity-list"><div class="skeleton" style="height:200px"></div></div>';

    async function refresh() {
      try {
        var resp = await api.get('/api/activity');
        var viewers = await resp.json();
        if (!Array.isArray(viewers)) viewers = [];
        var countEl = document.getElementById('stat-active-count');
        if (countEl) countEl.textContent = viewers.length;
        var container = document.getElementById('activity-list');
        if (!container) return;
        if (viewers.length === 0) {
          container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>No active viewers</p></div>';
          return;
        }
        var html = '<table class="list-table"><thead><tr>' +
          '<th>Stream</th><th>User</th><th>Delivery</th><th>Client</th><th>Duration</th><th>Address</th>' +
          '</tr></thead><tbody>';
        for (var i = 0; i < viewers.length; i++) {
          var v = viewers[i];
          html += '<tr>' +
            '<td>' + esc(v.stream_name) + '</td>' +
            '<td>' + esc(v.username || '-') + '</td>' +
            '<td><span class="badge">' + esc(v.delivery || '-') + '</span></td>' +
            '<td>' + esc(v.client_name || '-') + '</td>' +
            '<td>' + esc(v.duration || '-') + '</td>' +
            '<td>' + esc(v.remote_addr || '-') + '</td>' +
            '</tr>';
        }
        html += '</tbody></table>';
        container.innerHTML = html;
      } catch (e) {
        var container = document.getElementById('activity-list');
        if (container) container.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load activity</p></div>';
      }
    }

    await refresh();
    if (activityRefreshTimer) clearInterval(activityRefreshTimer);
    activityRefreshTimer = setInterval(function() {
      if (router.current !== 'activity') {
        clearInterval(activityRefreshTimer);
        activityRefreshTimer = null;
        return;
      }
      refresh();
    }, 5000);
  }

  async function renderLibrary(el) {
    el.innerHTML = '<h1 class="page-title">Library</h1>' +
      '<div id="lib-source-picker" style="margin-bottom:16px"><div class="skeleton" style="height:40px"></div></div>' +
      '<div id="lib-content"></div>';

    try {
      await loadFavorites();
      var resp = await api.get('/api/sources');
      var sources = await resp.json();
      if (!Array.isArray(sources)) sources = [];
      var tvpSources = sources.filter(function(s) { return s.type === 'tvpstreams'; });

      var picker = document.getElementById('lib-source-picker');
      if (tvpSources.length === 0) {
        picker.innerHTML = '<div style="color:var(--text-muted)">No TVP Streams sources configured. Add one in Sources to see your library.</div>';
        return;
      }

      if (tvpSources.length === 1) {
        picker.innerHTML = '';
        loadLibrarySource(tvpSources[0].type, tvpSources[0].id);
      } else {
        var html = '<div style="display:flex;gap:8px;flex-wrap:wrap">';
        for (var i = 0; i < tvpSources.length; i++) {
          var src = tvpSources[i];
          html += '<button class="btn btn-ghost lib-source-tab" data-source-type="' + esc(src.type) + '" data-source-id="' + esc(src.id) + '">' +
            esc(src.name) + ' <span class="stream-group-count">' + (src.stream_count || 0) + '</span></button>';
        }
        html += '</div>';
        picker.innerHTML = html;

        picker.addEventListener('click', function(e) {
          var btn = e.target.closest('.lib-source-tab');
          if (!btn) return;
          var tabs = picker.querySelectorAll('.lib-source-tab');
          for (var t = 0; t < tabs.length; t++) tabs[t].classList.remove('active');
          btn.classList.add('active');
          loadLibrarySource(btn.dataset.sourceType, btn.dataset.sourceId);
        });

        picker.querySelector('.lib-source-tab').click();
      }
    } catch (e) {
      document.getElementById('lib-source-picker').innerHTML =
        '<div class="empty-state">' + icons.empty + '<p>Failed to load sources</p></div>';
    }
  }

  async function loadLibrarySource(sourceType, sourceId) {
    var content = document.getElementById('lib-content');
    if (!content) return;
    content.innerHTML = '<div class="skeleton" style="height:400px"></div>';

    try {
      var resp = await api.get('/api/streams?source_type=' + encodeURIComponent(sourceType) + '&source_id=' + encodeURIComponent(sourceId));
      var streams = await resp.json();
      if (!Array.isArray(streams)) streams = [];

      var movies = [];
      var seriesMap = {};
      for (var i = 0; i < streams.length; i++) {
        var s = streams[i];
        var classified = s.vod_type || ((s.season > 0 || s.episode > 0) ? 'series' : 'movie');
        if (classified === 'movie') {
          movies.push(s);
        } else if (classified === 'series' || classified === 'episode') {
          var seriesKey = s.group || s.name || '(Unknown Series)';
          if (!seriesMap[seriesKey]) seriesMap[seriesKey] = { name: seriesKey, streams: [], poster: null };
          seriesMap[seriesKey].streams.push(s);
          if (s.tvg_logo && !seriesMap[seriesKey].poster) seriesMap[seriesKey].poster = s.tvg_logo;
        } else {
          movies.push(s);
        }
      }

      var allGenres = {};
      var allDecades = {};
      var allCollections = {};
      for (var mi = 0; mi < movies.length; mi++) {
        var m = movies[mi];
        if (m.group) {
          var parts = m.group.split(/[|,]/);
          for (var pi = 0; pi < parts.length; pi++) {
            var g = parts[pi].trim();
            if (g) allGenres[g] = true;
          }
        }
        if (m.year) {
          var decade = Math.floor(parseInt(m.year) / 10) * 10;
          if (decade > 1900) allDecades[decade] = true;
        }
        if (m.collection_name) allCollections[m.collection_name] = true;
      }

      var seriesList = Object.keys(seriesMap).sort().map(function(k) { return seriesMap[k]; });

      var activeTab = 'movies';
      var searchTerm = '';
      var filterGenre = '';
      var filterDecade = '';
      var searchTimer = null;

      var genreOpts = Object.keys(allGenres).sort();
      var decadeOpts = Object.keys(allDecades).sort().reverse();

      function buildFilterBar() {
        return '<div class="filter-bar">' +
          '<input class="form-input" id="lib-search" type="text" placeholder="Search..." style="flex:1;min-width:200px;max-width:320px;padding:8px 12px;font-size:13px">' +
          '<select class="filter-select" id="lib-genre"><option value="">All Genres</option>' +
          genreOpts.map(function(g) { return '<option value="' + esc(g) + '">' + esc(g) + '</option>'; }).join('') +
          '</select>' +
          '<select class="filter-select" id="lib-decade"><option value="">All Decades</option>' +
          decadeOpts.map(function(d) { return '<option value="' + d + '">' + d + 's</option>'; }).join('') +
          '</select>' +
          '</div>';
      }

      function matchFilters(s) {
        if (searchTerm && (s.name || '').toLowerCase().indexOf(searchTerm) === -1) return false;
        if (filterGenre && (s.group || '').indexOf(filterGenre) === -1) return false;
        if (filterDecade && (s.year || '').indexOf(String(filterDecade)) !== 0) return false;
        return true;
      }

      function matchSeriesFilter(series) {
        if (searchTerm && series.name.toLowerCase().indexOf(searchTerm) === -1) return false;
        return true;
      }

      function posterUrl(s) {
        if (s.tvg_logo) return '/logo?url=' + encodeURIComponent(s.tvg_logo);
        return '';
      }

      function renderMovieGrid() {
        var filtered = movies.filter(matchFilters);
        filtered.sort(function(a, b) { return a.name.localeCompare(b.name); });
        var grid = document.getElementById('lib-grid');
        if (!grid) return;

        if (filtered.length === 0) {
          grid.innerHTML = '<div class="empty-state">' + icons.empty + '<p>' + (searchTerm || filterGenre || filterDecade ? 'No movies match your filters' : 'No movies found') + '</p></div>';
          return;
        }

        var html = '<div class="poster-grid">';
        for (var i = 0; i < filtered.length; i++) {
          var m = filtered[i];
          var url = posterUrl(m);
          var ratingHtml = '';
          html += '<div class="poster-card" data-sid="' + esc(m.id) + '" data-sname="' + esc(m.name) + '">';
          if (url) {
            html += '<img class="poster-img" src="' + url + '" loading="lazy" alt="">';
          } else {
            html += '<div class="poster-placeholder">&#127916;</div>';
          }
          if (m.height) {
            var resLabel = m.height >= 2160 ? '4K' : m.height >= 1080 ? '1080p' : m.height >= 720 ? '720p' : m.height + 'p';
            html += '<div class="poster-badge">' + resLabel + '</div>';
          }
          html += '<div class="poster-info"><div class="poster-title">' + esc(m.name) + '</div>' +
            '<div class="poster-meta">';
          if (m.year) html += '<span class="poster-year">' + esc(m.year) + '</span>';
          html += '</div></div></div>';
        }
        html += '</div>';
        grid.innerHTML = html;

        var summary = document.getElementById('lib-summary');
        if (summary) summary.textContent = filtered.length + ' movie' + (filtered.length !== 1 ? 's' : '');

        grid.querySelectorAll('.poster-card').forEach(function(card) {
          card.addEventListener('click', function() {
            showStreamDetail(this.getAttribute('data-sid'), this.getAttribute('data-sname'));
          });
        });
      }

      function renderSeriesGrid() {
        var filtered = seriesList.filter(matchSeriesFilter);
        var grid = document.getElementById('lib-grid');
        if (!grid) return;

        if (filtered.length === 0) {
          grid.innerHTML = '<div class="empty-state">' + icons.empty + '<p>' + (searchTerm ? 'No series match your search' : 'No TV series found') + '</p></div>';
          return;
        }

        var html = '<div class="poster-grid">';
        for (var i = 0; i < filtered.length; i++) {
          var series = filtered[i];
          var url = series.poster ? '/logo?url=' + encodeURIComponent(series.poster) : '';
          var epCount = series.streams.length;
          html += '<div class="poster-card poster-series-card" data-series-key="' + esc(series.name) + '">';
          if (url) {
            html += '<img class="poster-img" src="' + url + '" loading="lazy" alt="">';
          } else {
            html += '<div class="poster-placeholder">&#127909;</div>';
          }
          html += '<div class="poster-badge">' + epCount + ' ep' + (epCount !== 1 ? 's' : '') + '</div>';
          html += '<div class="poster-info"><div class="poster-title">' + esc(series.name) + '</div>' +
            '<div class="poster-meta"><span class="poster-year">TV Series</span></div></div></div>';
        }
        html += '</div>';
        grid.innerHTML = html;

        var summary = document.getElementById('lib-summary');
        if (summary) summary.textContent = filtered.length + ' series';

        grid.querySelectorAll('.poster-series-card').forEach(function(card) {
          card.addEventListener('click', function() {
            var key = this.getAttribute('data-series-key');
            var series = seriesMap[key];
            if (!series) return;
            showSeriesDetail(series);
          });
        });
      }

      function showSeriesDetail(series) {
        var overlay = document.createElement('div');
        overlay.className = 'modal-overlay';
        overlay.style.cssText = 'align-items:flex-start;padding:40px 20px;overflow-y:auto';
        var seasonGroups = {};
        for (var i = 0; i < series.streams.length; i++) {
          var ep = series.streams[i];
          var sn = ep.season || 0;
          if (!seasonGroups[sn]) seasonGroups[sn] = [];
          seasonGroups[sn].push(ep);
        }
        var seasonNums = Object.keys(seasonGroups).map(Number).sort(function(a, b) { return a - b; });
        var bodyHtml = '';
        for (var si = 0; si < seasonNums.length; si++) {
          var num = seasonNums[si];
          var eps = seasonGroups[num];
          eps.sort(function(a, b) { return (a.episode || 0) - (b.episode || 0); });
          bodyHtml += '<div style="margin-bottom:16px">';
          if (seasonNums.length > 1 || num > 0) {
            bodyHtml += '<div style="font-weight:600;color:#fff;margin-bottom:8px;font-size:14px">Season ' + num + '</div>';
          }
          for (var ei = 0; ei < eps.length; ei++) {
            var e = eps[ei];
            var epLabel = 'E' + String(e.episode || 0).padStart(2, '0');
            var epName = e.episode_name || e.name;
            bodyHtml += '<div style="display:flex;align-items:center;gap:12px;padding:8px 0;border-bottom:1px solid var(--border);cursor:pointer" class="series-ep-item" data-sid="' + esc(e.id) + '" data-sname="' + esc(epName) + '">' +
              '<span style="color:var(--text-muted);font-size:12px;min-width:32px">' + epLabel + '</span>' +
              '<span style="flex:1;color:var(--text);font-size:13px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + esc(epName) + '</span>' +
              '<button class="stream-play-btn" style="flex-shrink:0">&#9654;</button>' +
              '</div>';
          }
          bodyHtml += '</div>';
        }

        overlay.innerHTML = '<div style="max-width:600px;width:100%;margin:0 auto">' +
          '<div style="background:var(--bg-card);border:1px solid var(--border);border-radius:var(--radius-lg);overflow:hidden">' +
          '<div style="padding:20px 24px;border-bottom:1px solid var(--border);display:flex;align-items:center;justify-content:space-between">' +
          '<h2 style="color:#fff;font-size:18px;margin:0">' + esc(series.name) + '</h2>' +
          '<button class="btn btn-ghost series-close-btn" style="padding:4px 8px">&times;</button></div>' +
          '<div style="padding:16px 24px;max-height:60vh;overflow-y:auto">' + bodyHtml + '</div></div></div>';

        document.body.appendChild(overlay);
        overlay.addEventListener('click', function(e) {
          if (e.target === overlay) overlay.remove();
        });
        overlay.querySelector('.series-close-btn').addEventListener('click', function() { overlay.remove(); });
        overlay.querySelectorAll('.series-ep-item').forEach(function(item) {
          item.addEventListener('click', function() {
            overlay.remove();
            startPlay(this.getAttribute('data-sid'), this.getAttribute('data-sname'), false);
          });
        });
      }

      function renderActiveTab() {
        if (activeTab === 'movies') renderMovieGrid();
        else renderSeriesGrid();
      }

      content.innerHTML = '<div style="display:flex;gap:8px;margin-bottom:12px">' +
        '<button class="btn btn-primary lib-tab" data-tab="movies">Movies (' + movies.length + ')</button>' +
        '<button class="btn btn-ghost lib-tab" data-tab="series">TV Series (' + seriesList.length + ')</button>' +
        '</div>' +
        buildFilterBar() +
        '<div style="margin-bottom:12px"><span id="lib-summary" style="font-size:13px;color:var(--text-dim)"></span></div>' +
        '<div id="lib-grid"></div>';

      content.querySelectorAll('.lib-tab').forEach(function(btn) {
        btn.addEventListener('click', function() {
          activeTab = this.dataset.tab;
          content.querySelectorAll('.lib-tab').forEach(function(t) {
            t.className = 'btn ' + (t.dataset.tab === activeTab ? 'btn-primary' : 'btn-ghost') + ' lib-tab';
          });
          searchTerm = '';
          filterGenre = '';
          filterDecade = '';
          var si = document.getElementById('lib-search');
          if (si) si.value = '';
          var gi = document.getElementById('lib-genre');
          if (gi) gi.value = '';
          var di = document.getElementById('lib-decade');
          if (di) di.value = '';
          renderActiveTab();
        });
      });

      var searchEl = document.getElementById('lib-search');
      if (searchEl) {
        searchEl.addEventListener('input', function() {
          clearTimeout(searchTimer);
          var val = this.value.toLowerCase();
          searchTimer = setTimeout(function() {
            searchTerm = val;
            renderActiveTab();
          }, 300);
        });
      }

      var genreEl = document.getElementById('lib-genre');
      if (genreEl) {
        genreEl.addEventListener('change', function() {
          filterGenre = this.value;
          renderActiveTab();
        });
      }

      var decadeEl = document.getElementById('lib-decade');
      if (decadeEl) {
        decadeEl.addEventListener('change', function() {
          filterDecade = this.value;
          renderActiveTab();
        });
      }

      renderActiveTab();
    } catch (e) {
      content.innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load library</p></div>';
    }
  }

  async function renderGuide(el) {
    el.innerHTML = '<h1 class="page-title">Program Guide</h1>' +
      '<div class="search-bar">' + icons.search + '<input id="guide-search" placeholder="Search channels..."></div>' +
      '<div id="guide-content"><div class="skeleton" style="height:400px"></div></div>';

    try {
      var resp = await api.get('/api/epg/now');
      var programs = await resp.json();
      if (!Array.isArray(programs)) programs = [];

      var allPrograms = programs;

      function renderGuideList(filter) {
        var content = document.getElementById('guide-content');
        if (!content) return;

        var filtered = allPrograms;
        if (filter) {
          filtered = allPrograms.filter(function(p) {
            return (p.channel_name || '').toLowerCase().indexOf(filter) >= 0 ||
                   (p.title || '').toLowerCase().indexOf(filter) >= 0;
          });
        }

        if (filtered.length === 0) {
          content.innerHTML = '<div class="empty-state">' + icons.epg + '<p>' +
            (allPrograms.length === 0 ? 'No EPG data available. Add an EPG source and refresh it.' : 'No channels match your search') +
            '</p></div>';
          return;
        }

        var html = '<div class="epg-now-list">';
        for (var i = 0; i < filtered.length; i++) {
          var p = filtered[i];
          var startTime = new Date(p.start_time);
          var endTime = new Date(p.end_time);
          var timeStr = startTime.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) + ' - ' +
            endTime.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
          var progressPct = Math.round((p.progress || 0) * 100);
          var cats = (p.categories && p.categories.length > 0) ? p.categories.join(', ') : '';

          html += '<div class="epg-now-item" data-channel-id="' + esc(p.channel_id) + '">' +
            '<div class="epg-now-channel">' +
            '<div class="epg-now-channel-name">' + esc(p.channel_name) + '</div>' +
            '</div>' +
            '<div class="epg-now-program">' +
            '<div class="epg-now-title">' + esc(p.title) + (p.rating ? ' <span style="color:var(--text-muted);font-size:11px">' + esc(p.rating) + '</span>' : '') + '</div>' +
            (p.subtitle ? '<div class="epg-now-subtitle">' + esc(p.subtitle) + '</div>' : '') +
            (cats ? '<div class="epg-now-subtitle">' + esc(cats) + '</div>' : '') +
            '<div class="epg-progress"><div class="epg-progress-bar" style="width:' + progressPct + '%"></div></div>' +
            '</div>' +
            '<div class="epg-now-time">' + esc(timeStr) + '</div>' +
            '</div>';
        }
        html += '</div>';
        content.innerHTML = html;
      }

      renderGuideList('');

      var searchEl = document.getElementById('guide-search');
      if (searchEl) {
        var guideSearchTimer = null;
        searchEl.addEventListener('input', function() {
          clearTimeout(guideSearchTimer);
          var val = this.value.toLowerCase();
          guideSearchTimer = setTimeout(function() {
            renderGuideList(val);
          }, 300);
        });
      }
    } catch (e) {
      document.getElementById('guide-content').innerHTML =
        '<div class="empty-state">' + icons.epg + '<p>Failed to load program guide</p></div>';
    }
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
        var resp = await api.get('/api/client-profiles');
        var profiles = await resp.json();
        if (!Array.isArray(profiles)) profiles = [];
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
          '<th>Name</th><th>Delivery</th><th>Video</th><th>Audio</th><th>Container</th><th>Height</th><th>Type</th><th>Actions</th>' +
          '</tr></thead><tbody>';
        for (var i = 0; i < profiles.length; i++) {
          var p = profiles[i];
          var typeBadges = '';
          if (p.is_system) typeBadges += '<span class="badge badge-system">System</span> ';
          if (p.is_client) typeBadges += '<span class="badge badge-client">Client</span> ';

          var heightStr = p.output_height && p.output_height > 0 ? p.output_height + 'p' : 'Source';

          var actions = '';
          if (!p.is_system) {
            actions += '<button class="btn btn-sm btn-ghost client-edit-btn" data-id="' + esc(p.id) + '" title="Edit">' + icons.edit + '</button>';
          }
          if (!p.is_system && !p.is_client) {
            actions += '<button class="btn btn-sm btn-icon btn-danger client-del-btn" data-id="' + esc(p.id) + '" data-name="' + esc(p.name) + '" title="Delete">' + icons.trash + '</button>';
          }

          var matchStr = '';
          if (p.match_rules && p.match_rules.length > 0) {
            matchStr = '<div style="font-size:11px;color:var(--text-muted);margin-top:2px">' +
              p.match_rules.map(function(r) { return esc(r.header + ': ' + r.pattern); }).join(', ') + '</div>';
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
              var r = await api.del('/api/client-profiles/' + id);
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
        '</select></div>' +
        '<div class="form-group"><label class="form-label">Max Output Height</label>' +
        '<select class="form-input" id="cp-height">' +
        '<option value="0"' + (!p.output_height ? ' selected' : '') + '>Source (no scaling)</option>' +
        '<option value="2160"' + (p.output_height === 2160 ? ' selected' : '') + '>4K (2160p)</option>' +
        '<option value="1080"' + (p.output_height === 1080 ? ' selected' : '') + '>1080p</option>' +
        '<option value="720"' + (p.output_height === 720 ? ' selected' : '') + '>720p</option>' +
        '<option value="480"' + (p.output_height === 480 ? ' selected' : '') + '>480p</option>' +
        '</select></div>';

      if (p.match_rules && p.match_rules.length > 0) {
        html += '<div class="form-group"><label class="form-label">Match Rules</label>' +
          '<div style="background:var(--bg-input);border:1px solid var(--border);border-radius:var(--radius);padding:10px 12px;font-size:12px;font-family:monospace;color:var(--text-dim)">';
        for (var mi = 0; mi < p.match_rules.length; mi++) {
          var rule = p.match_rules[mi];
          html += esc(rule.header) + ': ' + esc(rule.pattern) + '<br>';
        }
        html += '</div></div>';
      }

      html += '</div>' +
        '<div class="modal-footer">' +
        '<button class="btn btn-ghost" id="cp-cancel">Cancel</button>' +
        '<button class="btn btn-primary" id="cp-save">' + (isEdit ? 'Update' : 'Create') + '</button>' +
        '</div></div></div>';

      document.body.insertAdjacentHTML('beforeend', html);

      document.getElementById('cp-cancel').addEventListener('click', function() {
        document.getElementById('client-modal').remove();
      });
      document.getElementById('client-modal').addEventListener('click', function(e) {
        if (e.target === this) this.remove();
      });
      document.getElementById('cp-save').addEventListener('click', async function() {
        var payload = {
          name: document.getElementById('cp-name').value.trim(),
          delivery: document.getElementById('cp-delivery').value,
          video_codec: document.getElementById('cp-video').value,
          audio_codec: document.getElementById('cp-audio').value,
          container: document.getElementById('cp-container').value,
          output_height: parseInt(document.getElementById('cp-height').value) || 0
        };
        if (!payload.name) { toast('Name required', 'error'); return; }
        try {
          var r;
          if (isEdit) {
            r = await api.put('/api/client-profiles/' + p.id, payload);
          } else {
            r = await api.post('/api/client-profiles', payload);
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
            navigator.clipboard.writeText(url).then(function() { toast('URL copied'); });
          } else {
            toast('Clipboard not available', 'error');
          }
        });
      }
    }
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
    epgsources: renderEPGSources,
    wireguard: renderWireGuard,
    settings: renderSettings,
    users: renderUsers,
    clients: renderClients,
    probe: renderProbe,
    player: renderPlayer
  };

  router.init();
  render();

  if (typeof module !== 'undefined' && module.exports) {
    module.exports = { pages: pages, esc: esc, formatTime: formatTime, api: api };
  }
})();

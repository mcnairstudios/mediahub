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
    addChannel: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M12 10v0"/><line x1="12" y1="7" x2="12" y2="13"/><line x1="9" y1="10" x2="15" y2="10"/></svg>'
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

    recordingID: null,

    cleanup: function() {
      if (this.bufferWatchInterval) { clearInterval(this.bufferWatchInterval); this.bufferWatchInterval = null; }
      if (this.statsInterval) { clearInterval(this.statsInterval); this.statsInterval = null; }
      if (this.hlsInstance) { this.hlsInstance.destroy(); this.hlsInstance = null; }
      if (this.recordingID) {
        api.del('/api/recordings/completed/' + this.recordingID + '/play').catch(function() {});
        this.recordingID = null;
      } else if (this.currentStreamID) {
        api.del('/api/play/' + this.currentStreamID).catch(function() {});
      }
      this.currentStreamID = null;
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
      { id: 'recordings', label: 'Recordings', icon: 'recordings' },
      { id: 'favorites', label: 'Favorites', icon: 'favorites' }
    ];
    if (isAdmin) {
      items.push({ id: 'activity', label: 'Activity', icon: 'stats' });
      items.push({ id: 'sources', label: 'Sources', icon: 'sources' });
      items.push({ id: 'epgsources', label: 'EPG Sources', icon: 'epg' });
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
    else if (page === 'favorites') renderFavorites(pageEl);
    else if (page === 'activity') renderActivity(pageEl);
    else if (page === 'sources') renderSources(pageEl);
    else if (page === 'epgsources') renderEPGSources(pageEl);
    else if (page === 'wireguard') renderWireGuard(pageEl);
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
    var isAdmin = api.user && api.user.is_admin;
    el.innerHTML = '<h1 class="page-title">Dashboard</h1>' +
      '<div class="stat-grid">' +
      '<div class="stat-card"><div class="stat-value" id="stat-streams">-</div><div class="stat-label">Streams</div></div>' +
      '<div class="stat-card"><div class="stat-value" id="stat-channels">-</div><div class="stat-label">Channels</div></div>' +
      '<div class="stat-card"><div class="stat-value" id="stat-recordings">-</div><div class="stat-label">Recordings</div></div>' +
      (isAdmin ? '<div class="stat-card"><div class="stat-value" id="stat-active">-</div><div class="stat-label">Active Now</div></div>' : '') +
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
      var fetches = [
        api.get('/api/sources').then(function(r) { return r.json(); }),
        api.get('/api/channels').then(function(r) { return r.json(); }),
        api.get('/api/recordings').then(function(r) { return r.json(); })
      ];
      if (isAdmin) fetches.push(api.get('/api/activity').then(function(r) { return r.json(); }));
      var results = await Promise.allSettled(fetches);
      var sources = results[0].status === 'fulfilled' ? results[0].value : [];
      var channels = results[1].status === 'fulfilled' ? results[1].value : [];
      var recordings = results[2].status === 'fulfilled' ? results[2].value : [];
      var totalStreams = 0;
      if (Array.isArray(sources)) {
        for (var si = 0; si < sources.length; si++) totalStreams += (sources[si].stream_count || 0);
      }
      var s = document.getElementById('stat-streams');
      var c = document.getElementById('stat-channels');
      var rc = document.getElementById('stat-recordings');
      if (s) s.textContent = totalStreams;
      if (c) c.textContent = Array.isArray(channels) ? channels.length : 0;
      if (rc) rc.textContent = Array.isArray(recordings) ? recordings.length : 0;
      if (isAdmin && results.length > 3) {
        var activeViewers = results[3].status === 'fulfilled' ? results[3].value : [];
        var ac = document.getElementById('stat-active');
        if (ac) ac.textContent = Array.isArray(activeViewers) ? activeViewers.length : 0;
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
      var typeBadge = src.type === 'tvpstreams' ? 'TVP' : src.type === 'xtream' ? 'Xtream' : 'M3U';
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

      var streamData = JSON.parse(details.dataset.streams || '[]');
      var section = details.dataset.section;
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
      startPlay(btn.dataset.sid, btn.dataset.sname, isLive);
    });
  }

  function buildTvpStreamGroups(container, allStreams) {
    var movies = [];
    var movieGroups = {};
    var seriesGroups = {};

    for (var i = 0; i < allStreams.length; i++) {
      var s = allStreams[i];
      if (s.vod_type === 'movie') {
        movies.push(s);
        var mg = s.group || '(Ungrouped)';
        if (!movieGroups[mg]) movieGroups[mg] = [];
        movieGroups[mg].push(s);
      } else if (s.vod_type === 'series' || s.vod_type === 'episode') {
        var sg = s.group || s.name || '(Unknown Series)';
        if (!seriesGroups[sg]) seriesGroups[sg] = [];
        seriesGroups[sg].push(s);
      } else {
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

    function escJson(arr) {
      return esc(JSON.stringify(arr));
    }

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
        html.push('<details class="stream-group" data-section="movies" data-streams="' + escJson(items) + '"><summary>' +
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
        html.push('<details class="stream-group" data-section="series" data-streams="' + escJson(items) + '"><summary>' +
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

    function escJson(arr) {
      return esc(JSON.stringify(arr));
    }

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
        html.push('<details class="stream-group" data-section="live" data-streams="' + escJson(items) + '"><summary>' +
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
        toast('Failed to start playback: ' + (errData.error || resp.statusText), 'error');
        return;
      }
      var data = await resp.json();
      playerState.sessionID = data.session_id;

      var delivery = data.delivery || 'hls';
      var endpoints = data.endpoints || {};

      if (delivery === 'hls') {
        var hlsUrl = endpoints.playlist || (isRecording
          ? '/api/recordings/completed/' + recID + '/play/hls/playlist.m3u8'
          : '/api/play/' + streamID + '/hls/playlist.m3u8');
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

    var seekID = isRecording ? recID : streamID;
    var seekPath = isRecording ? '/api/recordings/completed/' + recID + '/seek' : '/api/play/' + streamID + '/seek';
    bindPlayerControls(videoEl, streamID, seekPath);
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

      recordings.sort(function(a, b) {
        var ta = a.started_at || a.scheduled_start || '';
        var tb = b.started_at || b.scheduled_start || '';
        return tb.localeCompare(ta);
      });

      var isAdmin = api.user && api.user.role === 'admin';
      var html = '<table class="list-table"><thead><tr>' +
        '<th>Title</th><th>Channel</th><th>Status</th><th>Date</th><th>Duration</th><th>Size</th><th>Actions</th>' +
        '</tr></thead><tbody>';
      for (var i = 0; i < recordings.length; i++) {
        var r = recordings[i];
        var statusClass = r.status === 'recording' ? 'badge-live' : r.status === 'completed' ? 'badge-enabled' : r.status === 'scheduled' ? 'badge-warning' : 'badge-disabled';
        var statusText = r.status === 'recording' ? '<span class="recording-status"><span class="recording-dot"></span>Recording</span>' : esc(r.status || 'unknown');

        var dateStr = '-';
        if (r.started_at) dateStr = new Date(r.started_at).toLocaleString();
        else if (r.scheduled_start) dateStr = new Date(r.scheduled_start).toLocaleString();

        var durStr = '-';
        if (r.started_at && r.stopped_at) {
          var durSec = (new Date(r.stopped_at) - new Date(r.started_at)) / 1000;
          durStr = formatDurationSec(durSec);
        } else if (r.status === 'recording' && r.started_at) {
          var elapsed = (Date.now() - new Date(r.started_at).getTime()) / 1000;
          durStr = formatDurationSec(elapsed) + '...';
        }

        var sizeStr = formatBytes(r.file_size);

        var actions = '';
        if (r.status === 'completed') {
          actions += '<button class="btn btn-sm btn-primary rec-play-btn" data-id="' + esc(r.id) + '" data-title="' + esc(r.title || r.stream_name || r.id) + '">Play</button> ';
          actions += '<a class="btn btn-sm btn-ghost" href="/api/recordings/completed/' + esc(r.id) + '/stream" target="_blank" download>Download</a> ';
        }
        if (isAdmin) {
          actions += '<button class="btn btn-sm btn-danger rec-del-btn" data-id="' + esc(r.id) + '">Delete</button>';
        }

        html += '<tr>' +
          '<td>' + esc(r.title || r.stream_name || r.stream_id) + '</td>' +
          '<td>' + esc(r.channel_name || '-') + '</td>' +
          '<td><span class="badge ' + statusClass + '">' + statusText + '</span></td>' +
          '<td>' + esc(dateStr) + '</td>' +
          '<td>' + esc(durStr) + '</td>' +
          '<td>' + esc(sizeStr) + '</td>' +
          '<td>' + actions + '</td>' +
          '</tr>';
      }
      html += '</tbody></table>';
      container.innerHTML = html;

      container.querySelectorAll('.rec-play-btn').forEach(function(btn) {
        btn.addEventListener('click', function() {
          var recID = this.getAttribute('data-id');
          var title = this.getAttribute('data-title');
          playRecording(recID, title);
        });
      });

      container.querySelectorAll('.rec-del-btn').forEach(function(btn) {
        btn.addEventListener('click', async function() {
          var recID = this.getAttribute('data-id');
          if (!confirm('Delete this recording? The file will be removed from disk.')) return;
          var delResp = await api.del('/api/recordings/completed/' + recID).catch(function() {});
          if (delResp && (delResp.status === 204 || delResp.ok)) {
            toast('Recording deleted');
            renderRecordings(el);
          } else {
            toast('Failed to delete recording', 'error');
          }
        });
      });
    } catch (e) {
      document.getElementById('recording-list').innerHTML = '<div class="empty-state">' + icons.empty + '<p>Failed to load recordings</p></div>';
    }
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
      '<button class="btn btn-ghost" id="cancel-xtream-btn">Cancel</button></div></div>';

    document.getElementById('add-m3u-btn').addEventListener('click', function() {
      var f = document.getElementById('add-m3u-form');
      document.getElementById('add-tvp-form').style.display = 'none';
      document.getElementById('add-xtream-form').style.display = 'none';
      f.style.display = f.style.display === 'none' ? 'block' : 'none';
    });
    document.getElementById('cancel-m3u-btn').addEventListener('click', function() {
      document.getElementById('add-m3u-form').style.display = 'none';
    });
    document.getElementById('add-tvp-btn').addEventListener('click', function() {
      var f = document.getElementById('add-tvp-form');
      document.getElementById('add-m3u-form').style.display = 'none';
      document.getElementById('add-xtream-form').style.display = 'none';
      f.style.display = f.style.display === 'none' ? 'block' : 'none';
    });
    document.getElementById('cancel-tvp-btn').addEventListener('click', function() {
      document.getElementById('add-tvp-form').style.display = 'none';
    });
    document.getElementById('add-xtream-btn').addEventListener('click', function() {
      var f = document.getElementById('add-xtream-form');
      document.getElementById('add-m3u-form').style.display = 'none';
      document.getElementById('add-tvp-form').style.display = 'none';
      f.style.display = f.style.display === 'none' ? 'block' : 'none';
    });
    document.getElementById('cancel-xtream-btn').addEventListener('click', function() {
      document.getElementById('add-xtream-form').style.display = 'none';
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

      var html = '<table class="list-table"><thead><tr>' +
        '<th>Name</th><th>Type</th><th>Streams</th><th>Last Refreshed</th><th>Status</th><th>TLS</th><th></th>' +
        '</tr></thead><tbody>';
      for (var i = 0; i < sources.length; i++) {
        var s = sources[i];
        var statusBadge = s.is_enabled ? '<span class="badge badge-enabled">ON</span>' : '<span class="badge badge-disabled">OFF</span>';
        if (s.last_error) {
          statusBadge = '<span class="badge badge-live" title="' + esc(s.last_error) + '">ERROR</span>';
        }
        var lastRefreshed = s.last_refreshed ? new Date(s.last_refreshed).toLocaleString() : 'Never';
        var tlsCell = '';
        if (s.type === 'tvpstreams') {
          var enrolled = s.config && s.config.tls_enrolled === 'true';
          tlsCell = enrolled
            ? '<span class="badge badge-enabled" data-tls-id="' + esc(s.id) + '">Enrolled</span>'
            : '<span class="badge badge-disabled">Not enrolled</span>';
        } else {
          tlsCell = '<span style="color:var(--text-secondary)">N/A</span>';
        }
        html += '<tr>' +
          '<td>' + esc(s.name) + '</td>' +
          '<td><span class="badge badge-enabled">' + esc(s.type || 'unknown').toUpperCase() + '</span></td>' +
          '<td>' + (s.stream_count || 0) + '</td>' +
          '<td>' + esc(lastRefreshed) + '</td>' +
          '<td>' + statusBadge + '</td>' +
          '<td>' + tlsCell + '</td>' +
          '<td style="display:flex;gap:4px">' +
          '<button class="btn-icon refresh-source-btn" data-id="' + esc(s.id) + '" data-type="' + esc(s.type) + '" title="Refresh">' + icons.refresh + '</button>' +
          '<button class="btn-icon delete-source-btn" data-id="' + esc(s.id) + '" data-type="' + esc(s.type) + '" data-name="' + esc(s.name) + '" title="Delete" style="color:var(--danger)">' + icons.trash + '</button>' +
          '</td></tr>';
      }
      html += '</tbody></table>';
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

  var pages = {
    dashboard: renderDashboard,
    streams: renderStreams,
    channels: renderChannels,
    recordings: renderRecordings,
    favorites: renderFavorites,
    activity: renderActivity,
    sources: renderSources,
    epgsources: renderEPGSources,
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

// PopcornHub Frontend Client Application
(() => {
  const API_BASE = window.location.origin;
  const WS_PROTOCOL = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const WS_BASE = `${WS_PROTOCOL}//${window.location.host}`;

  // Application State
  const state = {
    user: null,
    accessToken: localStorage.getItem('ph_token') || '',
    refreshToken: localStorage.getItem('ph_refresh') || '',
    groups: [],
    currentGroup: null,
    catalog: [],
    presenceSocket: null,
    presenceHeartbeatTimer: null,
    room: null,
    roomSocket: null,
    roomIsPlaying: false,
    roomBasePos: 0,
    roomUpdatedAt: 0, // client timestamp of last state update
    driftCheckInterval: null,
  };

  // DOM Elements
  const el = {
    userHeader: document.getElementById('user-header'),
    headerAvatar: document.getElementById('header-avatar'),
    headerUsername: document.getElementById('header-username'),
    logoutBtn: document.getElementById('logout-btn'),
    connStatus: document.getElementById('connection-status'),

    // Sections
    authSection: document.getElementById('auth-section'),
    dashboardSection: document.getElementById('dashboard-section'),
    roomSection: document.getElementById('room-section'),

    // Auth
    tabLogin: document.getElementById('tab-login'),
    tabRegister: document.getElementById('tab-register'),
    loginForm: document.getElementById('login-form'),
    registerForm: document.getElementById('register-form'),
    loginEmail: document.getElementById('login-email'),
    loginPassword: document.getElementById('login-password'),
    loginError: document.getElementById('login-error'),
    regName: document.getElementById('reg-name'),
    regEmail: document.getElementById('reg-email'),
    regPassword: document.getElementById('reg-password'),
    regError: document.getElementById('reg-error'),

    // Dashboard
    navGroupsBtn: document.getElementById('nav-groups-btn'),
    navMediaBtn: document.getElementById('nav-media-btn'),
    groupsView: document.getElementById('groups-view'),
    mediaView: document.getElementById('media-view'),
    groupsList: document.getElementById('groups-list'),
    catalogGrid: document.getElementById('catalog-grid'),

    // Modals
    modalCreateGroup: document.getElementById('modal-create-group'),
    modalJoinGroup: document.getElementById('modal-join-group'),
    openCreateGroupModal: document.getElementById('open-create-group-modal'),
    openJoinGroupModal: document.getElementById('open-join-group-modal'),
    createGroupForm: document.getElementById('create-group-form'),
    createGroupName: document.getElementById('create-group-name'),
    joinGroupForm: document.getElementById('join-group-form'),
    joinInviteCode: document.getElementById('join-invite-code'),
    joinError: document.getElementById('join-error'),

    // Group Details
    noGroupSelected: document.getElementById('no-group-selected'),
    groupSelectedContent: document.getElementById('group-selected-content'),
    currentGroupName: document.getElementById('current-group-name'),
    currentInviteCode: document.getElementById('current-invite-code'),
    copyInviteCode: document.getElementById('copy-invite-code'),
    membersCount: document.getElementById('members-count'),
    groupMembersList: document.getElementById('group-members-list'),
    presenceCards: document.getElementById('presence-cards'),
    groupMediaList: document.getElementById('group-media-list'),

    // Room
    roomMediaTitle: document.getElementById('room-media-title'),
    roomGroupTag: document.getElementById('room-group-tag'),
    leaveRoomBtn: document.getElementById('leave-room-btn'),
    sharedVideo: document.getElementById('shared-video'),
    videoWrapper: document.getElementById('video-wrapper'),
    reactionsContainer: document.getElementById('reactions-container'),
    playerNotification: document.getElementById('player-notification'),
    ctrlPlayPause: document.getElementById('ctrl-play-pause'),
    ctrlRewind: document.getElementById('ctrl-rewind'),
    ctrlForward: document.getElementById('ctrl-forward'),
    currentTimeText: document.getElementById('current-time-text'),
    durationTimeText: document.getElementById('duration-time-text'),
    seekSlider: document.getElementById('seek-slider'),
    ctrlSyncForce: document.getElementById('ctrl-sync-force'),
    activeUsersCount: document.getElementById('active-users-count'),
    activeViewersList: document.getElementById('active-viewers-list'),
    chatMessages: document.getElementById('chat-messages'),
    chatForm: document.getElementById('chat-form'),
    chatInput: document.getElementById('chat-input'),
    syncStatusText: document.getElementById('sync-status-text'),
    toastNotification: document.getElementById('toast-notification'),
  };

  // ==========================================
  // API Client Helpers
  // ==========================================
  async function apiRequest(endpoint, options = {}) {
    options.headers = options.headers || {};
    options.headers['Content-Type'] = 'application/json';

    if (state.accessToken) {
      options.headers['Authorization'] = `Bearer ${state.accessToken}`;
    }

    let res = await fetch(`${API_BASE}${endpoint}`, options);

    if (res.status === 401 && state.refreshToken) {
      // Try refresh
      const refreshed = await tryRefreshToken();
      if (refreshed) {
        options.headers['Authorization'] = `Bearer ${state.accessToken}`;
        res = await fetch(`${API_BASE}${endpoint}`, options);
      } else {
        logout();
        throw new Error('Session expired. Please log in again.');
      }
    }

    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(data.error || `HTTP ${res.status}`);
    }
    return data;
  }

  async function tryRefreshToken() {
    try {
      const res = await fetch(`${API_BASE}/api/v1/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: state.refreshToken }),
      });
      if (res.ok) {
        const tokens = await res.json();
        setTokens(tokens.access_token, tokens.refresh_token);
        return true;
      }
    } catch (e) {
      console.warn('Failed to refresh token', e);
    }
    return false;
  }

  function setTokens(access, refresh) {
    state.accessToken = access;
    state.refreshToken = refresh;
    localStorage.setItem('ph_token', access);
    localStorage.setItem('ph_refresh', refresh);
  }

  function clearTokens() {
    state.accessToken = '';
    state.refreshToken = '';
    localStorage.removeItem('ph_token');
    localStorage.removeItem('ph_refresh');
  }

  // ==========================================
  // Auth & Init
  // ==========================================
  async function checkAuth() {
    if (!state.accessToken) {
      showAuthScreen();
      return;
    }

    try {
      state.user = await apiRequest('/api/v1/auth/me');
      showDashboardScreen();
      initPresenceSocket();
      loadUserGroups();
      loadCatalog();
    } catch (err) {
      clearTokens();
      showAuthScreen();
    }
  }

  function showAuthScreen() {
    el.authSection.style.display = 'flex';
    el.dashboardSection.style.display = 'none';
    el.roomSection.style.display = 'none';
    el.userHeader.style.display = 'none';
    closePresenceSocket();
  }

  function showDashboardScreen() {
    el.authSection.style.display = 'none';
    el.dashboardSection.style.display = 'block';
    el.roomSection.style.display = 'none';
    el.userHeader.style.display = 'flex';

    el.headerUsername.textContent = state.user.name;
    el.headerAvatar.src = state.user.avatar_url || 'https://api.dicebear.com/7.x/bottts/svg?seed=' + state.user.name;
  }

  function logout() {
    clearTokens();
    state.user = null;
    closePresenceSocket();
    closeRoomSocket();
    showAuthScreen();
  }

  // Auth Forms
  el.tabLogin.addEventListener('click', () => {
    el.tabLogin.classList.add('active');
    el.tabRegister.classList.remove('active');
    el.loginForm.style.display = 'block';
    el.registerForm.style.display = 'none';
  });

  el.tabRegister.addEventListener('click', () => {
    el.tabRegister.classList.add('active');
    el.tabLogin.classList.remove('active');
    el.loginForm.style.display = 'none';
    el.registerForm.style.display = 'block';
  });

  el.loginForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    el.loginError.style.display = 'none';
    try {
      const res = await apiRequest('/api/v1/auth/login', {
        method: 'POST',
        body: JSON.stringify({
          email: el.loginEmail.value,
          password: el.loginPassword.value,
        }),
      });
      setTokens(res.tokens.access_token, res.tokens.refresh_token);
      state.user = res.user;
      showDashboardScreen();
      initPresenceSocket();
      loadUserGroups();
      loadCatalog();
    } catch (err) {
      el.loginError.textContent = err.message;
      el.loginError.style.display = 'block';
    }
  });

  el.registerForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    el.regError.style.display = 'none';
    try {
      const res = await apiRequest('/api/v1/auth/register', {
        method: 'POST',
        body: JSON.stringify({
          name: el.regName.value,
          email: el.regEmail.value,
          password: el.regPassword.value,
        }),
      });
      setTokens(res.tokens.access_token, res.tokens.refresh_token);
      state.user = res.user;
      showDashboardScreen();
      initPresenceSocket();
      loadUserGroups();
      loadCatalog();
    } catch (err) {
      el.regError.textContent = err.message;
      el.regError.style.display = 'block';
    }
  });

  el.logoutBtn.addEventListener('click', logout);

  // Navigation Tabs
  el.navGroupsBtn.addEventListener('click', () => {
    el.navGroupsBtn.classList.add('active');
    el.navMediaBtn.classList.remove('active');
    el.groupsView.style.display = 'block';
    el.mediaView.style.display = 'none';
  });

  el.navMediaBtn.addEventListener('click', () => {
    el.navMediaBtn.classList.add('active');
    el.navGroupsBtn.classList.remove('active');
    el.groupsView.style.display = 'none';
    el.mediaView.style.display = 'block';
  });

  // ==========================================
  // Presence System («Кто что смотрит»)
  // ==========================================
  function initPresenceSocket() {
    if (state.presenceSocket) {
      return;
    }

    const wsUrl = `${WS_BASE}/ws/presence?token=${encodeURIComponent(state.accessToken)}`;
    state.presenceSocket = new WebSocket(wsUrl);

    state.presenceSocket.onopen = () => {
      console.log('Presence WebSocket connected');
      // Heartbeat timer every 15s
      clearInterval(state.presenceHeartbeatTimer);
      state.presenceHeartbeatTimer = setInterval(() => {
        if (state.presenceSocket && state.presenceSocket.readyState === WebSocket.OPEN) {
          state.presenceSocket.send(JSON.stringify({ type: 'HEARTBEAT' }));
        }
      }, 15000);
    };

    state.presenceSocket.onmessage = (event) => {
      try {
        const presence = JSON.parse(event.data);
        handlePresenceUpdate(presence);
      } catch (e) {
        console.warn('Failed to parse presence payload', e);
      }
    };

    state.presenceSocket.onclose = () => {
      clearInterval(state.presenceHeartbeatTimer);
      state.presenceSocket = null;
      // Reconnect after 3s if still logged in
      if (state.accessToken) {
        setTimeout(initPresenceSocket, 3000);
      }
    };
  }

  function closePresenceSocket() {
    clearInterval(state.presenceHeartbeatTimer);
    if (state.presenceSocket) {
      state.presenceSocket.close();
      state.presenceSocket = null;
    }
  }

  function handlePresenceUpdate(presence) {
    // If the presence update is for a member in the currently selected group, refresh presence cards
    if (state.currentGroup) {
      loadGroupPresence(state.currentGroup.id);
    }
  }

  async function loadGroupPresence(groupId) {
    try {
      const presences = await apiRequest(`/api/v1/groups/${groupId}/presence`);
      renderPresenceCards(presences);
    } catch (e) {
      console.warn('Failed to load group presence', e);
    }
  }

  function renderPresenceCards(presences) {
    // Filter only those who are currently watching
    const activeWatchers = (presences || []).filter(p => p.status === 'watching' && p.user_id !== state.user.id);

    if (activeWatchers.length === 0) {
      el.presenceCards.innerHTML = '<div class="empty-presence">Сейчас никто ничего не смотрит. Выберите фильм ниже и начните совместный просмотр!</div>';
      return;
    }

    el.presenceCards.innerHTML = activeWatchers.map(p => `
      <div class="presence-card">
        <div class="presence-user-info">
          <img src="${p.avatar_url || 'https://api.dicebear.com/7.x/bottts/svg?seed=' + p.user_name}" alt="User">
          <div class="presence-text">
            <strong>${escapeHtml(p.user_name)}</strong>
            <div class="activity-desc">🎬 Смотрит: «${escapeHtml(p.media_title || 'Фильм')}»</div>
          </div>
        </div>
        <button class="btn-join-watch" data-room-id="${p.room_id}">
          🚀 Присоединиться
        </button>
      </div>
    `).join('');

    // Attach join room clicks
    el.presenceCards.querySelectorAll('.btn-join-watch').forEach(btn => {
      btn.addEventListener('click', () => {
        const roomId = btn.getAttribute('data-room-id');
        if (roomId) {
          joinWatchRoom(roomId);
        }
      });
    });
  }

  // ==========================================
  // Groups Management
  // ==========================================
  async function loadUserGroups() {
    try {
      state.groups = await apiRequest('/api/v1/groups');
      renderGroupsList();

      if (state.groups.length > 0 && !state.currentGroup) {
        selectGroup(state.groups[0]);
      }
    } catch (e) {
      console.error('Failed to load groups', e);
    }
  }

  function renderGroupsList() {
    if (state.groups.length === 0) {
      el.groupsList.innerHTML = '<div class="empty-state">У вас пока нет групп. Создайте группу или присоединитесь по инвайт-коду!</div>';
      return;
    }

    el.groupsList.innerHTML = state.groups.map(g => `
      <div class="group-item-card ${state.currentGroup && state.currentGroup.id === g.id ? 'active' : ''}" data-id="${g.id}">
        <div class="group-item-name">${escapeHtml(g.name)}</div>
        <div class="group-item-meta">
          <span>👥 ${g.members ? g.members.length : 1} уч.</span>
          <span>🔑 ${escapeHtml(g.invite_code)}</span>
        </div>
      </div>
    `).join('');

    el.groupsList.querySelectorAll('.group-item-card').forEach(card => {
      card.addEventListener('click', () => {
        const id = card.getAttribute('data-id');
        const grp = state.groups.find(g => g.id === id);
        if (grp) selectGroup(grp);
      });
    });
  }

  async function selectGroup(group) {
    state.currentGroup = group;
    renderGroupsList();

    el.noGroupSelected.style.display = 'none';
    el.groupSelectedContent.style.display = 'block';

    el.currentGroupName.textContent = group.name;
    el.currentInviteCode.textContent = group.invite_code;

    // Load members
    try {
      const members = await apiRequest(`/api/v1/groups/${group.id}/members`);
      el.membersCount.textContent = members.length;
      el.groupMembersList.innerHTML = members.map(m => `
        <div class="member-chip">
          <img src="${m.avatar_url || 'https://api.dicebear.com/7.x/bottts/svg?seed=' + m.name}" alt="${escapeHtml(m.name)}">
          <div class="member-chip-info">
            <span class="member-chip-name">${escapeHtml(m.name)}</span>
            <span class="member-chip-role">${m.role === 'owner' ? '👑 Создатель' : 'Участник'}</span>
          </div>
        </div>
      `).join('');
    } catch (e) {
      console.warn('Failed to load members', e);
    }

    // Load group presence
    loadGroupPresence(group.id);

    // Render quick start movies for this group
    renderGroupMedia();
  }

  function renderGroupMedia() {
    if (!state.catalog || state.catalog.length === 0) return;

    el.groupMediaList.innerHTML = state.catalog.map(item => `
      <div class="media-card">
        <img class="media-poster" src="${item.poster_url}" alt="${escapeHtml(item.title)}">
        <div class="media-body">
          <div class="media-title">${escapeHtml(item.title)}</div>
          <button class="btn btn-primary-sm btn-start-watch" data-media-id="${item.id}">
            ▶ Смотреть вместе
          </button>
        </div>
      </div>
    `).join('');

    el.groupMediaList.querySelectorAll('.btn-start-watch').forEach(btn => {
      btn.addEventListener('click', () => {
        const mediaId = btn.getAttribute('data-media-id');
        startWatchInGroup(state.currentGroup.id, mediaId);
      });
    });
  }

  // Modals
  el.openCreateGroupModal.addEventListener('click', () => {
    el.modalCreateGroup.style.display = 'flex';
  });

  el.openJoinGroupModal.addEventListener('click', () => {
    el.modalJoinGroup.style.display = 'flex';
    el.joinError.style.display = 'none';
  });

  document.querySelectorAll('[data-close]').forEach(btn => {
    btn.addEventListener('click', () => {
      const modalId = btn.getAttribute('data-close');
      document.getElementById(modalId).style.display = 'none';
    });
  });

  el.createGroupForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    try {
      const newGroup = await apiRequest('/api/v1/groups', {
        method: 'POST',
        body: JSON.stringify({ name: el.createGroupName.value }),
      });
      el.modalCreateGroup.style.display = 'none';
      el.createGroupName.value = '';
      await loadUserGroups();
      selectGroup(newGroup);
    } catch (err) {
      alert(err.message);
    }
  });

  el.joinGroupForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    el.joinError.style.display = 'none';
    try {
      const joined = await apiRequest('/api/v1/groups/join', {
        method: 'POST',
        body: JSON.stringify({ invite_code: el.joinInviteCode.value }),
      });
      el.modalJoinGroup.style.display = 'none';
      el.joinInviteCode.value = '';
      await loadUserGroups();
      selectGroup(joined);
    } catch (err) {
      el.joinError.textContent = err.message;
      el.joinError.style.display = 'block';
    }
  });

  el.copyInviteCode.addEventListener('click', () => {
    const code = el.currentInviteCode.textContent;
    navigator.clipboard.writeText(code);
    showToast(`Код ${code} скопирован в буфер!`);
  });

  // ==========================================
  // Media Catalog
  // ==========================================
  async function loadCatalog() {
    try {
      state.catalog = await apiRequest('/api/v1/media');
      renderCatalog();
      if (state.currentGroup) {
        renderGroupMedia();
      }
    } catch (e) {
      console.warn('Failed to load catalog', e);
    }
  }

  function renderCatalog() {
    el.catalogGrid.innerHTML = state.catalog.map(item => `
      <div class="media-card">
        <img class="media-poster" src="${item.poster_url}" alt="${escapeHtml(item.title)}">
        <div class="media-body">
          <div>
            <div class="media-title">${escapeHtml(item.title)}</div>
            <div class="media-meta-tags">
              ${(item.metadata.genre || []).map(g => `<span class="meta-tag">${escapeHtml(g)}</span>`).join('')}
              <span class="meta-tag">${item.metadata.year || ''}</span>
              <span class="meta-tag">⏱ ${Math.round((item.duration_sec || 0)/60)} мин</span>
            </div>
          </div>
          <button class="btn btn-primary full-width btn-catalog-watch" data-media-id="${item.id}">
            🍿 Смотреть с группой
          </button>
        </div>
      </div>
    `).join('');

    el.catalogGrid.querySelectorAll('.btn-catalog-watch').forEach(btn => {
      btn.addEventListener('click', () => {
        const mediaId = btn.getAttribute('data-media-id');
        if (!state.currentGroup) {
          alert('Сначала выберите или создайте группу!');
          el.navGroupsBtn.click();
          return;
        }
        startWatchInGroup(state.currentGroup.id, mediaId);
      });
    });
  }

  // ==========================================
  // Collaborative Watch Room & Drift Correction
  // ==========================================
  async function startWatchInGroup(groupId, mediaId) {
    try {
      const room = await apiRequest(`/api/v1/groups/${groupId}/room`, {
        method: 'POST',
        body: JSON.stringify({ media_id: mediaId }),
      });
      joinWatchRoom(room.id);
    } catch (err) {
      alert(err.message);
    }
  }

  async function joinWatchRoom(roomId) {
    try {
      const roomDetails = await apiRequest(`/api/v1/rooms/${roomId}`);
      state.room = roomDetails;

      el.dashboardSection.style.display = 'none';
      el.roomSection.style.display = 'block';

      el.roomMediaTitle.textContent = roomDetails.media.title;
      el.roomGroupTag.textContent = `Группа: ${roomDetails.group.name}`;

      // Set video source
      el.sharedVideo.src = roomDetails.media.stream_url;
      el.sharedVideo.load();

      initRoomSocket(roomId);
    } catch (err) {
      alert('Не удалось подключиться к комнате: ' + err.message);
    }
  }

  function initRoomSocket(roomId) {
    closeRoomSocket();

    const wsUrl = `${WS_BASE}/ws/rooms/${roomId}?token=${encodeURIComponent(state.accessToken)}`;
    state.roomSocket = new WebSocket(wsUrl);

    state.roomSocket.onopen = () => {
      console.log('Room WebSocket connected');
      startDriftCorrectionLoop();
    };

    state.roomSocket.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        handleRoomMessage(msg);
      } catch (e) {
        console.warn('Failed to parse room message', e);
      }
    };

    state.roomSocket.onclose = () => {
      console.log('Room WebSocket closed');
      stopDriftCorrectionLoop();
    };
  }

  function closeRoomSocket() {
    stopDriftCorrectionLoop();
    if (state.roomSocket) {
      state.roomSocket.close();
      state.roomSocket = null;
    }
  }

  function sendRoomAction(action) {
    if (state.roomSocket && state.roomSocket.readyState === WebSocket.OPEN) {
      state.roomSocket.send(JSON.stringify(action));
    }
  }

  function handleRoomMessage(msg) {
    const video = el.sharedVideo;

    switch (msg.type) {
      case 'SYNC_STATE':
        state.roomIsPlaying = msg.is_playing;
        state.roomBasePos = msg.position_sec;
        state.roomUpdatedAt = Date.now();

        // Calculate initial offset
        const targetPos = getComputedServerPosition();
        if (Math.abs(video.currentTime - targetPos) > 1.5) {
          video.currentTime = targetPos;
        }

        if (state.roomIsPlaying && video.paused) {
          video.play().catch(() => {});
          el.ctrlPlayPause.textContent = '⏸️';
        } else if (!state.roomIsPlaying && !video.paused) {
          video.pause();
          el.ctrlPlayPause.textContent = '▶️';
        }

        if (msg.active_users) {
          renderActiveViewers(msg.active_users);
        }
        break;

      case 'PLAY':
        state.roomIsPlaying = true;
        state.roomBasePos = msg.position_sec;
        state.roomUpdatedAt = Date.now();

        video.currentTime = msg.position_sec;
        video.play().catch(() => {});
        el.ctrlPlayPause.textContent = '⏸️';

        showPlayerToast(`▶️ ${msg.initiated_by_name} запустил(а) видео`);
        break;

      case 'PAUSE':
        state.roomIsPlaying = false;
        state.roomBasePos = msg.position_sec;
        state.roomUpdatedAt = Date.now();

        video.currentTime = msg.position_sec;
        video.pause();
        el.ctrlPlayPause.textContent = '▶️';

        showPlayerToast(`⏸️ ${msg.initiated_by_name} поставил(а) на паузу`);
        break;

      case 'SEEK':
        state.roomBasePos = msg.position_sec;
        state.roomUpdatedAt = Date.now();

        video.currentTime = msg.position_sec;
        showPlayerToast(`⏩ ${msg.initiated_by_name} перемотал(а) на ${formatTime(msg.position_sec)}`);
        break;

      case 'RATE_LIMITED':
        showToast(msg.message || 'Слишком много команд! Подождите 1.5 сек.');
        break;

      case 'USER_JOINED':
        showPlayerToast(`👋 ${msg.message}`);
        appendChatMessage('system', msg.message);
        // Request sync to refresh active viewers
        sendRoomAction({ type: 'SYNC_STATE' });
        break;

      case 'USER_LEFT':
        showPlayerToast(`🚪 ${msg.message}`);
        appendChatMessage('system', msg.message);
        sendRoomAction({ type: 'SYNC_STATE' });
        break;

      case 'CHAT_MESSAGE':
        appendChatMessage('user', msg.text, msg.initiated_by_name, msg.payload ? msg.payload.avatar_url : '');
        break;

      case 'REACTION':
        renderFloatingReaction(msg.emoji);
        break;
    }
  }

  function getComputedServerPosition() {
    if (!state.roomIsPlaying) {
      return state.roomBasePos;
    }
    const elapsedSec = (Date.now() - state.roomUpdatedAt) / 1000;
    return state.roomBasePos + elapsedSec;
  }

  // Drift Correction Engine (Roadmap Stage 3)
  function startDriftCorrectionLoop() {
    stopDriftCorrectionLoop();
    state.driftCheckInterval = setInterval(() => {
      const video = el.sharedVideo;
      if (!video || isNaN(video.duration) || video.duration === 0) return;

      const expectedPos = getComputedServerPosition();
      const diff = video.currentTime - expectedPos; // positive if local video is ahead, negative if behind
      const absDiff = Math.abs(diff);

      // Update sync text indicator
      if (absDiff < 0.3) {
        el.syncStatusText.textContent = 'Синхронизировано';
        el.syncStatusText.style.color = 'var(--accent-green)';
      } else {
        el.syncStatusText.textContent = `Коррекция (${diff > 0 ? '+' : ''}${diff.toFixed(1)}с)`;
        el.syncStatusText.style.color = 'var(--primary)';
      }

      // Drift Correction Algorithm
      if (state.roomIsPlaying) {
        if (absDiff < 0.3) {
          video.playbackRate = 1.0;
        } else if (absDiff >= 0.3 && absDiff <= 2.0) {
          // Smooth drift correction via speed adjustment
          if (diff < 0) {
            // Local is behind -> speed up slightly
            video.playbackRate = 1.05;
          } else {
            // Local is ahead -> slow down slightly
            video.playbackRate = 0.95;
          }
        } else if (absDiff > 2.0) {
          // Hard seek for significant drift
          video.currentTime = expectedPos;
          video.playbackRate = 1.0;
        }
      } else {
        video.playbackRate = 1.0;
        if (absDiff > 1.0) {
          video.currentTime = expectedPos;
        }
      }
    }, 1000);
  }

  function stopDriftCorrectionLoop() {
    if (state.driftCheckInterval) {
      clearInterval(state.driftCheckInterval);
      state.driftCheckInterval = null;
    }
  }

  // Custom Player Controls
  el.ctrlPlayPause.addEventListener('click', () => {
    const video = el.sharedVideo;
    const isPlaying = !video.paused;

    if (isPlaying) {
      sendRoomAction({
        type: 'PAUSE',
        position_sec: video.currentTime,
      });
    } else {
      sendRoomAction({
        type: 'PLAY',
        position_sec: video.currentTime,
      });
    }
  });

  el.ctrlRewind.addEventListener('click', () => {
    const newPos = Math.max(0, el.sharedVideo.currentTime - 10);
    sendRoomAction({
      type: 'SEEK',
      position_sec: newPos,
    });
  });

  el.ctrlForward.addEventListener('click', () => {
    const newPos = Math.min(el.sharedVideo.duration || 9999, el.sharedVideo.currentTime + 10);
    sendRoomAction({
      type: 'SEEK',
      position_sec: newPos,
    });
  });

  el.seekSlider.addEventListener('change', () => {
    const newPos = parseFloat(el.seekSlider.value);
    sendRoomAction({
      type: 'SEEK',
      position_sec: newPos,
    });
  });

  el.ctrlSyncForce.addEventListener('click', () => {
    sendRoomAction({ type: 'SYNC_STATE' });
    showToast('Запрошена принудительная синхронизация');
  });

  el.sharedVideo.addEventListener('timeupdate', () => {
    const video = el.sharedVideo;
    el.currentTimeText.textContent = formatTime(video.currentTime);
    if (video.duration) {
      el.durationTimeText.textContent = formatTime(video.duration);
      el.seekSlider.max = video.duration;
      el.seekSlider.value = video.currentTime;
    }
  });

  el.leaveRoomBtn.addEventListener('click', () => {
    closeRoomSocket();
    el.sharedVideo.pause();
    el.roomSection.style.display = 'none';
    el.dashboardSection.style.display = 'block';
  });

  // Reactions
  document.querySelectorAll('.reaction-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      const emoji = btn.getAttribute('data-emoji');
      sendRoomAction({
        type: 'REACTION',
        emoji: emoji,
      });
    });
  });

  function renderFloatingReaction(emoji) {
    const span = document.createElement('span');
    span.className = 'floating-emoji';
    span.textContent = emoji;

    // Randomize horizontal position between 10% and 80%
    const randomLeft = Math.floor(Math.random() * 70) + 10;
    span.style.left = `${randomLeft}%`;

    el.reactionsContainer.appendChild(span);
    setTimeout(() => {
      span.remove();
    }, 3000);
  }

  function showPlayerToast(text) {
    el.playerNotification.textContent = text;
    el.playerNotification.style.display = 'block';
    setTimeout(() => {
      el.playerNotification.style.display = 'none';
    }, 3000);
  }

  // Active Viewers in Room
  function renderActiveViewers(users) {
    el.activeUsersCount.textContent = users.length;
    el.activeViewersList.innerHTML = users.map(u => `
      <div class="viewer-badge" title="${escapeHtml(u.user_name)}">
        <img src="${u.avatar_url || 'https://api.dicebear.com/7.x/bottts/svg?seed=' + u.user_name}" alt="User">
        <span>${escapeHtml(u.user_name)}</span>
      </div>
    `).join('');
  }

  // Room Chat
  el.chatForm.addEventListener('submit', (e) => {
    e.preventDefault();
    const text = el.chatInput.value.trim();
    if (!text) return;

    sendRoomAction({
      type: 'CHAT_MESSAGE',
      text: text,
    });
    el.chatInput.value = '';
  });

  function appendChatMessage(type, text, author = '', avatar = '') {
    const div = document.createElement('div');
    if (type === 'system') {
      div.className = 'chat-system-msg';
      div.textContent = text;
    } else {
      div.className = 'chat-msg';
      div.innerHTML = `
        <span class="chat-msg-author">${escapeHtml(author)}:</span>
        <span class="chat-msg-text">${escapeHtml(text)}</span>
      `;
    }
    el.chatMessages.appendChild(div);
    el.chatMessages.scrollTop = el.chatMessages.scrollHeight;
  }

  // Utilities
  function showToast(message) {
    el.toastNotification.textContent = message;
    el.toastNotification.style.display = 'block';
    setTimeout(() => {
      el.toastNotification.style.display = 'none';
    }, 3500);
  }

  function formatTime(seconds) {
    if (isNaN(seconds)) return '00:00';
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  }

  function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/[&<>'"]/g, tag => ({
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      "'": '&#39;',
      '"': '&quot;',
    }[tag] || tag));
  }

  // Start initialization
  checkAuth();
})();

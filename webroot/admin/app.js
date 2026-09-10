const commands = [
  { name: 'play', path: '/cmd/ytmd/play', methods: ['POST'], description: 'Change the player state to play' },
  { name: 'pause', path: '/cmd/ytmd/pause', methods: ['POST'], description: 'Change the player state to pause' },
  { name: 'toggle-play', path: '/cmd/ytmd/toggle-play', methods: ['POST'], description: 'Toggle play/pause' },
  { name: 'previous', path: '/cmd/ytmd/previous', methods: ['POST'], description: 'Play the previous song' },
  { name: 'next', path: '/cmd/ytmd/next', methods: ['POST'], description: 'Play the next song' },
  { name: 'like-state', path: '/cmd/ytmd/like-state', methods: ['GET'], description: 'Get the current like state' },
  { name: 'like', path: '/cmd/ytmd/like', methods: ['POST'], description: 'Set the current song as liked' },
  { name: 'dislike', path: '/cmd/ytmd/dislike', methods: ['POST'], description: 'Set the current song as disliked' },
  { name: 'seek-to', path: '/cmd/ytmd/seek-to', methods: ['POST'], description: 'Seek to a specific time' },
  { name: 'go-back', path: '/cmd/ytmd/go-back', methods: ['POST'], description: 'Move the current song backwards' },
  { name: 'go-forward', path: '/cmd/ytmd/go-forward', methods: ['POST'], description: 'Move the current song forwards' },
  { name: 'shuffle', path: '/cmd/ytmd/shuffle', methods: ['GET', 'POST'], description: 'Get or change shuffle state' },
  { name: 'repeat-mode', path: '/cmd/ytmd/repeat-mode', methods: ['GET'], description: 'Get the current repeat mode' },
  { name: 'switch-repeat', path: '/cmd/ytmd/switch-repeat', methods: ['POST'], description: 'Switch repeat mode' },
  { name: 'volume', path: '/cmd/ytmd/volume', methods: ['GET', 'POST'], description: 'Get or set the player volume' },
  { name: 'fullscreen', path: '/cmd/ytmd/fullscreen', methods: ['GET', 'POST'], description: 'Get or set fullscreen mode' },
  { name: 'toggle-mute', path: '/cmd/ytmd/toggle-mute', methods: ['POST'], description: 'Toggle mute state' },
  { name: 'song', path: '/cmd/ytmd/song', methods: ['GET'], description: 'Get the current song details' },
  { name: 'queue', path: '/cmd/ytmd/queue', methods: ['GET', 'POST', 'PATCH', 'DELETE'], description: 'Get queue info or modify the queue' },
  { name: 'queue-next', path: '/cmd/ytmd/queue/next', methods: ['GET'], description: 'Get information about the next song in the queue' },
  { name: 'queue-index', path: '/cmd/ytmd/queue/{index}', methods: ['PATCH', 'DELETE'], description: 'Move or remove a queue item by index' },
  { name: 'search', path: '/cmd/ytmd/search', methods: ['POST'], description: 'Search for a song' },
];

const select = document.getElementById('commandSelect');
const output = document.getElementById('commandOutput');
const argInput = document.getElementById('argInput');
const runButton = document.getElementById('runCommand');
const urlInput = document.getElementById('commandUrl');
const copyButton = document.getElementById('copyUrl');
const tabs = document.querySelectorAll('.tab');
const pages = document.querySelectorAll('.page');
const queueList = document.getElementById('queueList');
const clearQueueButton = document.getElementById('clearQueueButton');
const blacklistInput = document.getElementById('blacklistInput');
const addBlacklistButton = document.getElementById('addBlacklistButton');
const blacklistList = document.getElementById('blacklistList');
const settingsOutput = document.getElementById('settingsOutput');
const maxDurationInput = document.getElementById('maxDurationInput');
const saveMaxDurationButton = document.getElementById('saveMaxDurationButton');
const currentMeta = document.getElementById('currentMeta');
const playState = document.getElementById('playState');
const currentTime = document.getElementById('currentTime');
const totalTime = document.getElementById('totalTime');
const progressBar = document.getElementById('progressBar');
const currentHeadline = currentMeta?.parentElement || null;
const playerButtons = {
  prev: document.getElementById('prevButton'),
  back: document.getElementById('backButton'),
  play: document.getElementById('playButton'),
  pause: document.getElementById('pauseButton'),
  forward: document.getElementById('forwardButton'),
  next: document.getElementById('nextButton'),
  shuffle: document.getElementById('shuffleButton'),
};
const songRequestInput = document.getElementById('songRequestInput');
const songRequestForm = document.getElementById('songRequestForm');
const seedStatusLine = document.getElementById('seedStatusLine');
const seedPlaylistInput = document.getElementById('seedPlaylistInput');
const addSeedPlaylistButton = document.getElementById('addSeedPlaylistButton');
const seedInputFeedback = document.getElementById('seedInputFeedback');
const clearQueueOnRequestCheckbox = document.getElementById('clearQueueOnRequestCheckbox');
const seedPlaylistList = document.getElementById('seedPlaylistList');

let seedStatusTimer = null;

function updateSelection(selected) {
  if (!selected) return;

  const defaultMethod = selected.methods[0]?.toLowerCase() || 'get';
  const methodPath = `${selected.path}/${defaultMethod}`;
  const fullUrl = `${window.location.origin}${methodPath}`;
  urlInput.value = fullUrl;
  output.textContent = `${selected.methods.join(', ')} ${methodPath}\n\n${selected.description}`;
}

function renderCommands() {
  select.innerHTML = '';

  commands.forEach((cmd) => {
    const option = document.createElement('option');
    option.value = cmd.name;
    option.textContent = `${cmd.name} (${cmd.methods.join(', ')})`;
    select.appendChild(option);
  });

  select.addEventListener('change', () => {
    const selected = commands.find((cmd) => cmd.name === select.value);
    updateSelection(selected);
  });

  if (commands.length > 0) {
    const first = commands[0];
    select.value = first.name;
    updateSelection(first);
  }
}

async function runCommand() {
  const selected = commands.find((cmd) => cmd.name === select.value);
  if (!selected) return;

  const method = selected.methods[0];
  output.textContent = 'Running…';

  try {
    const body = argInput.value.trim();
    const init = {
      method,
      headers: { 'Content-Type': 'application/json' },
    };

    if (method !== 'GET' && body) {
      const parsed = body.startsWith('{') ? JSON.parse(body) : { value: body };
      init.body = JSON.stringify(parsed);
    }

    const response = await fetch(selected.path, init);
    const text = await response.text();

    let formattedText = text;
    try {
      const parsed = JSON.parse(text);
      formattedText = JSON.stringify(parsed, null, 2);
    } catch (_) {
      formattedText = text || `Status ${response.status}`;
    }

    output.textContent = formattedText;
    if (!response.ok) {
      output.textContent = `Request failed with status ${response.status}\n\n${formattedText}`;
    }
  } catch (err) {
    output.textContent = `Request failed: ${err.message}`;
  }
}

tabs.forEach((tab) => {
  tab.addEventListener('click', () => {
    tabs.forEach((item) => item.classList.toggle('active', item === tab));
    pages.forEach((page) => page.classList.toggle('active', page.id === `page-${tab.dataset.page}`));
    if (tab.dataset.page === 'seed') {
      loadSeedPanel();
      startSeedStatusPolling();
    } else {
      stopSeedStatusPolling();
    }
  });
});

async function loadSeedPanel() {
  try {
    const response = await fetch('/api/seed/status');
    const payload = await readCommandEnvelope(response);
    renderSeedPanel(payload.data || {});
  } catch (err) {
    if (seedStatusLine) {
      seedStatusLine.textContent = `Seed mode: error (${err.message})`;
    }
  }
}

function renderSeedPanel(data) {
  if (seedStatusLine) {
    if (data.seedModeActive) {
      const requests = Number(data.activeRequestCount) || 0;
      const requestLabel = requests > 0
        ? `${requests} request${requests === 1 ? '' : 's'} waiting`
        : 'no requests waiting';
      let playingLabel = 'idle';
      if (data.playingKind === 'request') {
        playingLabel = 'playing a request';
      } else if (data.playingKind === 'seed') {
        playingLabel = 'playing seed';
      } else if (data.playingKind === 'other') {
        playingLabel = 'playing other track';
      }
      seedStatusLine.textContent = `Jukebox mode: Active · ${playingLabel} · ${requestLabel}`;
    } else {
      seedStatusLine.textContent = 'Jukebox mode: Inactive (seed fallback off)';
    }
    seedStatusLine.classList.toggle('seed-status--active', Boolean(data.seedModeActive));
  }
  if (clearQueueOnRequestCheckbox) {
    clearQueueOnRequestCheckbox.checked = Boolean(data.clearQueueOnRequest);
  }
  renderSeedPlaylists(data.seedPlaylists || []);
}

function renderSeedPlaylists(playlists) {
  if (!seedPlaylistList) return;
  seedPlaylistList.innerHTML = '';
  if (!playlists.length) {
    const item = document.createElement('li');
    item.className = 'seed-list__empty';
    item.textContent = 'No seed playlists saved yet';
    seedPlaylistList.appendChild(item);
    return;
  }

  playlists.forEach((playlist) => {
    const item = document.createElement('li');
    item.className = 'seed-list__item';

    const label = document.createElement('span');
    label.className = 'seed-list__label';
    const displayName = playlist.name || playlist.id || '';
    const labelText = `${playlist.trackCount || 0} - ${displayName}`;
    label.textContent = labelText;
    label.title = playlist.id && playlist.name && playlist.name !== playlist.id
      ? `${labelText} (${playlist.id})`
      : labelText;

    const playButton = document.createElement('button');
    playButton.type = 'button';
    playButton.className = 'queue-action-btn queue-action-btn--play';
    playButton.textContent = '⏵';
    playButton.setAttribute('aria-label', 'Enqueue seed playlist');
    playButton.addEventListener('click', () => enqueueSeedPlaylist(playlist.id));

    const purgeButton = document.createElement('button');
    purgeButton.type = 'button';
    purgeButton.className = 'seed-list__action';
    purgeButton.textContent = 'Purge';
    purgeButton.addEventListener('click', () => removeSeedTracksFromQueue(playlist.id));

    const removeButton = document.createElement('button');
    removeButton.type = 'button';
    removeButton.className = 'queue-action-btn queue-action-btn--delete';
    removeButton.textContent = 'X';
    removeButton.setAttribute('aria-label', 'Remove playlist');
    removeButton.addEventListener('click', () => removeSeedPlaylist(playlist.id));

    const actions = document.createElement('div');
    actions.className = 'seed-list__actions';
    actions.append(purgeButton, playButton, removeButton);

    item.append(label, actions);
    seedPlaylistList.appendChild(item);
  });
}

async function addSeedPlaylist() {
  if (!seedPlaylistInput) return;
  const input = seedPlaylistInput.value.trim();
  if (!input) return;
  if (seedInputFeedback) seedInputFeedback.textContent = 'Adding playlist…';
  try {
    const response = await fetch('/api/seed/playlists', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ input }),
    });
    const payload = await readCommandEnvelope(response);
    seedPlaylistInput.value = '';
    if (seedInputFeedback) seedInputFeedback.textContent = payload.message || 'Playlist saved';
    await loadSeedPanel();
    refreshQueueFromProxy();
  } catch (err) {
    if (seedInputFeedback) seedInputFeedback.textContent = err.message;
  }
}

async function enqueueSeedPlaylist(playlistId) {
  if (seedInputFeedback) seedInputFeedback.textContent = 'Adding tracks to queue…';
  try {
    const response = await fetch(`/cmd/jb/seed/enqueue?playlistId=${encodeURIComponent(playlistId)}`, {
      method: 'POST',
      headers: { Accept: 'application/json' },
    });
    const payload = await readCommandEnvelope(response);
    if (seedInputFeedback) seedInputFeedback.textContent = payload.message || 'Playlist queued';
    await loadSeedPanel();
    refreshQueueFromProxy();
  } catch (err) {
    if (seedInputFeedback) seedInputFeedback.textContent = err.message;
  }
}

async function removeSeedTracksFromQueue(playlistId) {
  try {
    const response = await fetch(`/cmd/jb/seed/queue?playlistId=${encodeURIComponent(playlistId)}`, {
      method: 'DELETE',
      headers: { Accept: 'application/json' },
    });
    const payload = await readCommandEnvelope(response);
    if (seedInputFeedback) seedInputFeedback.textContent = payload.message || 'Seed tracks removed from queue';
    await loadSeedPanel();
    refreshQueueFromProxy();
  } catch (err) {
    if (seedInputFeedback) seedInputFeedback.textContent = err.message;
  }
}

async function removeSeedPlaylist(playlistId) {
  try {
    const response = await fetch(`/api/seed/playlists/${encodeURIComponent(playlistId)}`, {
      method: 'DELETE',
      headers: { Accept: 'application/json' },
    });
    const payload = await readCommandEnvelope(response);
    if (seedInputFeedback) seedInputFeedback.textContent = payload.message || 'Playlist removed';
    await loadSeedPanel();
  } catch (err) {
    if (seedInputFeedback) seedInputFeedback.textContent = err.message;
  }
}

async function saveSeedSettings(clearQueueOnRequest) {
  try {
    await fetch('/api/seed/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ clearQueueOnRequest }),
    }).then(readCommandEnvelope);
  } catch (err) {
    if (seedInputFeedback) seedInputFeedback.textContent = err.message;
  }
}

function startSeedStatusPolling() {
  stopSeedStatusPolling();
  seedStatusTimer = window.setInterval(loadSeedPanel, 5000);
}

function stopSeedStatusPolling() {
  if (seedStatusTimer) {
    window.clearInterval(seedStatusTimer);
    seedStatusTimer = null;
  }
}

if (addSeedPlaylistButton) {
  addSeedPlaylistButton.addEventListener('click', addSeedPlaylist);
}
if (seedPlaylistInput) {
  seedPlaylistInput.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') {
      event.preventDefault();
      addSeedPlaylist();
    }
  });
}
if (clearQueueOnRequestCheckbox) {
  clearQueueOnRequestCheckbox.addEventListener('change', () => {
    saveSeedSettings(clearQueueOnRequestCheckbox.checked);
  });
}

async function loadSettings() {
  try {
    const response = await fetch('/api/config');
    const payload = await response.json();
    if (!response.ok || payload.exitCode !== 0) {
      throw new Error(payload.message || 'Unable to load settings');
    }
    renderBlacklist(payload.data?.blacklist || []);
    if (Number.isFinite(payload.data?.maxDuration)) {
      maxDurationInput.value = String(payload.data.maxDuration);
    }
    settingsOutput.textContent = JSON.stringify(payload.data || {}, null, 2);
  } catch (err) {
    settingsOutput.textContent = `Failed to load settings: ${err.message}`;
  }
}

function getCurrentBlacklist() {
  return Array.from(blacklistList.querySelectorAll('li[data-entry]')).map((item) => item.dataset.entry || '');
}

function renderBlacklist(items) {
  blacklistList.innerHTML = '';
  if (!items.length) {
    const item = document.createElement('li');
    item.textContent = 'No blacklist entries yet';
    blacklistList.appendChild(item);
    return;
  }

  items.forEach((entry) => {
    const item = document.createElement('li');
    item.dataset.entry = entry;
    item.innerHTML = `<span>${entry}</span><button type="button" aria-label="Remove ${entry}">✕</button>`;
    blacklistList.appendChild(item);
  });

  blacklistList.querySelectorAll('button').forEach((button) => {
    button.addEventListener('click', () => removeBlacklistEntry(button.closest('li')?.dataset.entry || ''));
  });
}

async function saveSettings(blacklist, maxDuration = Number(maxDurationInput.value) || 600) {
  const response = await fetch('/api/config', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ blacklist: blacklist.filter(Boolean), maxDuration }),
  });
  const payload = await response.json();
  if (!response.ok || payload.exitCode !== 0) {
    throw new Error(payload.message || 'Unable to save settings');
  }
  settingsOutput.textContent = `Saved settings\n${JSON.stringify(payload.data || {}, null, 2)}`;
  renderBlacklist(payload.data?.blacklist || []);
}

async function addBlacklistEntry() {
  const value = blacklistInput.value.trim();
  if (!value) return;
  const current = getCurrentBlacklist();
  const next = [...new Set([...current, value])];
  blacklistInput.value = '';
  try {
    await saveSettings(next);
  } catch (err) {
    settingsOutput.textContent = `Failed to save settings: ${err.message}`;
  }
}

async function removeBlacklistEntry(value) {
  const current = getCurrentBlacklist();
  const next = current.filter((entry) => entry !== value);
  try {
    await saveSettings(next);
  } catch (err) {
    settingsOutput.textContent = `Failed to save settings: ${err.message}`;
  }
}

addBlacklistButton.addEventListener('click', addBlacklistEntry);
blacklistInput.addEventListener('keydown', (event) => {
  if (event.key === 'Enter') {
    event.preventDefault();
    addBlacklistEntry();
  }
});

saveMaxDurationButton.addEventListener('click', async () => {
  try {
    const blacklist = getCurrentBlacklist();
    await saveSettings(blacklist, Number(maxDurationInput.value) || 600);
  } catch (err) {
    settingsOutput.textContent = `Failed to save settings: ${err.message}`;
  }
});

runButton.addEventListener('click', runCommand);
copyButton.addEventListener('click', async () => {
  try {
    await navigator.clipboard.writeText(urlInput.value);
    copyButton.textContent = 'Copied';
    window.setTimeout(() => {
      copyButton.textContent = 'Copy';
    }, 1200);
  } catch (err) {
    copyButton.textContent = 'Copy failed';
  }
});
let playbackState = {
  isPaused: true,
  position: 0,
  duration: 0,
  title: '',
  artist: '',
  videoId: '',
  hasSong: false,
  shuffle: false,
  queue: [],
  queueStatus: 'empty', // ok | empty | error | unavailable
};
let queueRefreshToken = 0;
let nowPlayingRefreshToken = 0;
let previousQueueExpanded = false;
let previousQueueRendered = false;
let queueActionFeedback = null;
let queueActionFeedbackTimer = null;
let queueDragActive = false;
let queueDragFromIndex = null;
let queueMoveInFlight = false;
let queueRefreshDebounceTimer = null;
let pollTimer = null;
let progressStallTimer = null;

const POLL_INTERVAL_WS_MS = 5000; // queue has no WS event; song switches already refresh sooner
const POLL_INTERVAL_NO_WS_MS = 10000;
// YTMD position ticks are often ~1s; keep headroom so we don't false-pause between ticks.
const PROGRESS_STALL_MS = 1800;

function isNowPlayingSocketOpen() {
  return window.__jukeboksSocket?.readyState === WebSocket.OPEN;
}

function scheduleQueueRefreshFromWS(delayMs = 800) {
  window.clearTimeout(queueRefreshDebounceTimer);
  queueRefreshDebounceTimer = window.setTimeout(() => {
    refreshQueueFromProxy();
  }, delayMs);
}

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function refreshPlaybackFromProxyIfNeeded() {
  if (!isNowPlayingSocketOpen()) {
    refreshNowPlayingFromProxy();
    refreshShuffleFromProxy();
  }
}

function startPolling() {
  window.clearInterval(pollTimer);
  const intervalMs = isNowPlayingSocketOpen() ? POLL_INTERVAL_WS_MS : POLL_INTERVAL_NO_WS_MS;
  pollTimer = window.setInterval(() => {
    if (isNowPlayingSocketOpen()) {
      refreshQueueFromProxy();
    } else {
      refreshNowPlayingFromProxy();
      refreshQueueFromProxy();
      refreshShuffleFromProxy();
    }
  }, intervalMs);
}

function setAdminPaused(isPaused) {
  const next = Boolean(isPaused);
  if (playbackState.isPaused === next) {
    if (playbackState.hasSong) {
      playState.textContent = next ? '⏸' : '▶';
    }
    updatePlaybackButtons();
    return;
  }
  playbackState.isPaused = next;
  if (playbackState.hasSong) {
    playState.textContent = next ? '⏸' : '▶';
  }
  updatePlaybackButtons();
}

// YTMD keeps sending POSITION_CHANGED while playing and stops when paused.
function notePlaybackProgress() {
  window.clearTimeout(progressStallTimer);
  progressStallTimer = window.setTimeout(() => {
    if (!playbackState.hasSong || playbackState.isPaused) return;
    setAdminPaused(true);
  }, PROGRESS_STALL_MS);
}

function applyPositionFromWS(position) {
  const next = Math.max(0, Number(position) || 0);
  const advanced = next > playbackState.position + 0.05;
  playbackState.position = next;
  if (advanced && playbackState.hasSong) {
    setAdminPaused(false);
    notePlaybackProgress();
  } else if (playbackState.hasSong) {
    playState.textContent = playbackState.isPaused ? '⏸' : '▶';
  }
  currentTime.textContent = formatTime(playbackState.position);
  updateProgressBar();
}

function resolveIsPaused(...candidates) {
  for (const obj of candidates) {
    if (obj && typeof obj.isPlaying === 'boolean') return !obj.isPlaying;
  }
  for (const obj of candidates) {
    if (obj && typeof obj.isPaused === 'boolean') return obj.isPaused;
  }
  return null;
}

function applyPlayerStateFromWS(data) {
  const isPaused = resolveIsPaused(data);
  if (isPaused === null) return;
  setAdminPaused(isPaused);
  if (!isPaused) {
    notePlaybackProgress();
  } else {
    window.clearTimeout(progressStallTimer);
  }
  if (data.position != null) {
    playbackState.position = Math.max(0, Number(data.position) || 0);
    currentTime.textContent = formatTime(playbackState.position);
    updateProgressBar();
  }
}

async function readCommandEnvelope(response) {
  let payload = null;
  try {
    payload = await response.json();
  } catch (_) {
    payload = null;
  }

  if (!response.ok) {
    if (response.status === 404 && response.url.includes('/api/seed')) {
      throw new Error('Seed API not found — restart jukeboks (just run) to load the latest server');
    }
    const message = payload?.message || `status ${response.status}`;
    throw new Error(message);
  }

  if (payload && typeof payload.exitCode === 'number' && payload.exitCode !== 0) {
    throw new Error(payload.message || 'Command failed');
  }

  return payload;
}

function setPreviousQueueExpanded(expanded, button = null, sublist = null) {
  previousQueueExpanded = Boolean(expanded);
  if (button) {
    button.textContent = previousQueueExpanded ? 'Hide previous' : 'Previous…';
    button.setAttribute('aria-expanded', previousQueueExpanded ? 'true' : 'false');
  }
  if (sublist) {
    sublist.classList.toggle('is-collapsed', !previousQueueExpanded);
    sublist.hidden = !previousQueueExpanded;
    sublist.setAttribute('data-expanded', previousQueueExpanded ? 'true' : 'false');
  }
}

function handleQueueToggle(event) {
  const button = event.target.closest('[data-toggle="previous"]');
  if (!button) return;
  const sublist = button.parentElement?.querySelector('[data-previous-list]');
  if (!sublist) return;
  event.preventDefault();
  setPreviousQueueExpanded(sublist.hidden, button, sublist);
}

function shouldHandleQueueAction(event) {
  return Boolean(event.target.closest('[data-queue-action]'));
}

function setQueueActionFeedback(message, kind = 'success') {
  queueActionFeedback = { message: String(message || ''), kind };
  renderQueue();

  if (queueActionFeedbackTimer) {
    window.clearTimeout(queueActionFeedbackTimer);
  }
  const ttl = kind === 'error' ? 4000 : 1400;
  queueActionFeedbackTimer = window.setTimeout(() => {
    queueActionFeedback = null;
    renderQueue();
  }, ttl);
}

function resetQueueActionButtonState(button, action) {
  if (!button) return;
  button.classList.remove('is-pending', 'is-success', 'is-error');
  button.disabled = false;
  button.textContent = action === 'delete' ? 'X' : '⏵';
}

async function handleQueueItemAction(event) {
  const button = event.target.closest('[data-queue-action]');
  if (!button) return;

  const action = button.dataset.queueAction;
  const queueIndex = Number(button.dataset.queueIndex);
  if (!Number.isFinite(queueIndex)) return;

  event.preventDefault();

  button.classList.add('is-pending');
  button.disabled = true;
  button.textContent = '…';

  const path = action === 'delete'
    ? `/cmd/ytmd/queue/${queueIndex}/delete`
    : '/cmd/ytmd/queue/patch';
  const method = action === 'delete' ? 'DELETE' : 'PATCH';
  const init = {
    method,
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
  };

  if (action !== 'delete') {
    init.body = JSON.stringify({ index: queueIndex });
  }

  try {
    await readCommandEnvelope(await fetch(path, init));

    button.classList.remove('is-pending');
    button.classList.add('is-success');
    button.textContent = '✓';
    setQueueActionFeedback(action === 'delete' ? 'Deleted queue item' : 'Started queue item', 'success');

    window.setTimeout(() => {
      refreshQueueFromProxy();
      refreshNowPlayingFromProxy();
    }, 700);
  } catch (err) {
    button.classList.remove('is-pending');
    button.classList.add('is-error');
    button.textContent = '!';
    setQueueActionFeedback(err?.message || (action === 'delete' ? 'Unable to delete queue item' : 'Unable to start queue item'), 'error');
    console.error('Queue action failed', err);
  } finally {
    window.setTimeout(() => {
      resetQueueActionButtonState(button, action);
    }, 900);
  }
}

async function handleClearQueue() {
  if (!clearQueueButton) return;
  if (!window.confirm('Clear the entire YTMD queue?')) {
    return;
  }

  clearQueueButton.disabled = true;
  try {
    await readCommandEnvelope(await fetch('/cmd/ytmd/queue/delete', {
      method: 'DELETE',
      headers: { Accept: 'application/json' },
    }));
    setQueueActionFeedback('Queue cleared', 'success');
    playbackState.queue = [];
    playbackState.queueStatus = 'empty';
    renderQueue();
    window.setTimeout(() => {
      refreshQueueFromProxy();
      refreshNowPlayingFromProxy();
    }, 500);
  } catch (err) {
    setQueueActionFeedback(err?.message || 'Unable to clear queue', 'error');
    console.error('Clear queue failed', err);
  } finally {
    clearQueueButton.disabled = false;
  }
}

async function moveQueueItem(fromIndex, toIndex) {
  if (!Number.isFinite(fromIndex) || !Number.isFinite(toIndex) || fromIndex === toIndex) {
    return;
  }
  if (queueMoveInFlight) {
    return;
  }

  queueMoveInFlight = true;
  if (queueList) {
    queueList.classList.add('is-reordering');
  }

  try {
    await readCommandEnvelope(await fetch(`/cmd/ytmd/queue/${fromIndex}/patch`, {
      method: 'PATCH',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: JSON.stringify({ toIndex }),
    }));
    setQueueActionFeedback('Moved queue item', 'success');
    window.setTimeout(() => {
      refreshQueueFromProxy();
      refreshNowPlayingFromProxy();
    }, 700);
  } catch (err) {
    setQueueActionFeedback(err?.message || 'Unable to move queue item', 'error');
    console.error('Queue move failed', err);
    refreshQueueFromProxy();
  } finally {
    queueMoveInFlight = false;
    if (queueList) {
      queueList.classList.remove('is-reordering');
    }
  }
}

function clearQueueDropTargets() {
  if (!queueList) return;
  queueList.querySelectorAll('.queue-item.is-drop-target').forEach((el) => {
    el.classList.remove('is-drop-target');
  });
}

function clearQueueDragState() {
  queueDragActive = false;
  queueDragFromIndex = null;
  document.body.classList.remove('is-queue-dragging');
  if (!queueList) return;
  queueList.querySelectorAll('.queue-item.is-dragging').forEach((el) => {
    el.classList.remove('is-dragging');
  });
  clearQueueDropTargets();
}

function setQueueDropTarget(item) {
  clearQueueDropTargets();
  if (!item) return;
  const toIndex = Number(item.dataset.queueIndex);
  if (Number.isFinite(toIndex) && toIndex !== queueDragFromIndex) {
    item.classList.add('is-drop-target');
  }
}

function bindQueueDragDrop() {
  if (!queueList || queueList.dataset.dndBound === '1') return;
  queueList.dataset.dndBound = '1';

  queueList.addEventListener('dragstart', (event) => {
    const handle = event.target.closest('.queue-drag-handle');
    const item = event.target.closest('.queue-item[data-queue-draggable="true"]');
    if (!handle || !item || !queueList.contains(item)) {
      event.preventDefault();
      return;
    }
    const fromIndex = Number(item.dataset.queueIndex);
    if (!Number.isFinite(fromIndex)) {
      event.preventDefault();
      return;
    }
    queueDragActive = true;
    queueDragFromIndex = fromIndex;
    item.classList.add('is-dragging');
    document.body.classList.add('is-queue-dragging');
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', String(fromIndex));
  });

  queueList.addEventListener('dragover', (event) => {
    const target = event.target.closest('.queue-item[data-queue-draggable="true"]');
    if (!target || !queueList.contains(target)) return;
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
    setQueueDropTarget(target);
  });

  queueList.addEventListener('drop', async (event) => {
    const target = event.target.closest('.queue-item[data-queue-draggable="true"]');
    if (!target || !queueList.contains(target)) return;
    event.preventDefault();
    const fromIndex = queueDragFromIndex;
    const toIndex = Number(target.dataset.queueIndex);
    clearQueueDragState();
    if (Number.isFinite(fromIndex) && Number.isFinite(toIndex) && fromIndex !== toIndex) {
      await moveQueueItem(fromIndex, toIndex);
    } else {
      scheduleQueueRefreshFromWS(0);
    }
  });

  queueList.addEventListener('dragend', () => {
    const wasDragging = queueDragActive;
    clearQueueDragState();
    if (wasDragging && !queueMoveInFlight) {
      scheduleQueueRefreshFromWS(0);
    }
  });
}

if (queueList) {
  queueList.addEventListener('click', (event) => {
    if (shouldHandleQueueAction(event)) {
      handleQueueItemAction(event);
      return;
    }
    handleQueueToggle(event);
  });
  bindQueueDragDrop();
}

if (clearQueueButton) {
  clearQueueButton.addEventListener('click', () => {
    handleClearQueue();
  });
}

function formatTime(seconds) {
  const totalSeconds = Number.isFinite(seconds) ? Math.max(0, Math.floor(seconds)) : 0;
  const mins = String(Math.floor(totalSeconds / 60)).padStart(2, '0');
  const secs = String(totalSeconds % 60).padStart(2, '0');
  return `${mins}:${secs}`;
}

function getCurrentMetaTrack() {
  if (!currentMeta) return null;

  let track = currentMeta.querySelector('.now-playing__headline-track');
  if (track) {
    return track;
  }

  track = document.createElement('span');
  track.className = 'now-playing__headline-track';
  track.textContent = currentMeta.textContent || '';
  currentMeta.textContent = '';
  currentMeta.appendChild(track);
  return track;
}

function setCurrentMetaText(value) {
  const track = getCurrentMetaTrack();
  if (!track) return;
  track.textContent = value;
}

function getCurrentMetaText() {
  const track = getCurrentMetaTrack();
  return track ? track.textContent || '' : '';
}

function updateCurrentMetaMarquee() {
  if (!currentMeta || !currentHeadline) return;
  const currentMetaTrack = getCurrentMetaTrack();
  if (!currentMetaTrack) return;

  currentMeta.classList.remove('is-marquee');
  currentMeta.style.removeProperty('--marquee-distance');

  window.requestAnimationFrame(() => {
    const marqueeEndPadding = 24;
    const availableWidth = currentHeadline.clientWidth;
    const textWidth = currentMetaTrack.scrollWidth;
    const overflowWidth = textWidth - availableWidth;

    if (overflowWidth > 4) {
      currentMeta.style.setProperty('--marquee-distance', `${overflowWidth + marqueeEndPadding}px`);
      currentMeta.classList.add('is-marquee');
    }
  });
}

function setToggleButtonState(button, isOn) {
  if (!button) return;
  button.classList.toggle('is-active', Boolean(isOn));
  button.setAttribute('aria-pressed', isOn ? 'true' : 'false');
}

function updatePlaybackButtons() {
  const isPlayingNow = playbackState.hasSong && !playbackState.isPaused;
  if (playerButtons.play && playerButtons.pause) {
    // Highlight the action available: Pause while playing, Play while paused.
    playerButtons.play.classList.toggle('is-active', playbackState.hasSong && playbackState.isPaused);
    playerButtons.pause.classList.toggle('is-active', isPlayingNow);
  }
  document.body.classList.toggle('is-playing', isPlayingNow);
  setToggleButtonState(playerButtons.shuffle, playbackState.shuffle);
}

function updateProgressBar() {
  const maxDuration = Math.max(1, playbackState.duration || 100);
  progressBar.max = String(maxDuration);
  progressBar.value = String(Math.min(maxDuration, playbackState.position || 0));
  currentTime.textContent = formatTime(playbackState.position);
  totalTime.textContent = formatTime(playbackState.duration);
}


function renderQueue() {
  if (!queueList) return;
  if (queueDragActive || queueMoveInFlight) return;

  const feedbackMarkup = queueActionFeedback
    ? `<div class="queue-feedback queue-feedback--${escapeHtml(queueActionFeedback.kind)}">${escapeHtml(queueActionFeedback.message)}</div>`
    : '';

  if (playbackState.queueStatus === 'unavailable') {
    queueList.innerHTML = `${feedbackMarkup}<div class="queue-empty-state">YTMD queue unavailable. Check that the player is reachable.</div>`;
    return;
  }

  if (playbackState.queueStatus === 'error') {
    queueList.innerHTML = `${feedbackMarkup}<div class="queue-empty-state">Unable to load queue. Retrying…</div>`;
    return;
  }

  if (!playbackState.queue?.length || playbackState.queueStatus === 'empty') {
    queueList.innerHTML = `${feedbackMarkup}<div class="queue-empty-state">Queue is empty.</div>`;
    return;
  }

  const visibleQueueItems = playbackState.queue.filter((item) => item && (item.title || item.artist || item.videoId));
  const previousItems = visibleQueueItems.filter((item) => item.kind === 'previous');
  const currentItem = visibleQueueItems.find((item) => item.kind === 'current');
  const nextItems = visibleQueueItems.filter((item) => item.kind === 'next');
  const previousExpanded = previousItems.length > 0 ? previousQueueExpanded : false;
  if (previousItems.length === 0) {
    previousQueueExpanded = false;
  }

  const buildItem = (item, kind) => {
    const displayText = escapeHtml([item.artist, item.title].filter(Boolean).join(' - '));
    const classes = [`queue-item`, kind === 'current' ? 'is-current' : '', kind === 'previous' ? 'is-previous' : ''].filter(Boolean).join(' ');
    const queueIndex = Number.isFinite(item?.queueIndex) ? item.queueIndex : '';
    const actionsDisabled = !Number.isFinite(item?.queueIndex);
    const canDrag = kind === 'next' && Number.isFinite(item?.queueIndex);
    const dragAttrs = canDrag
      ? ` data-queue-draggable="true" data-queue-index="${queueIndex}"`
      : '';
    const dragHandle = canDrag
      ? `<span class="queue-drag-handle" draggable="true" title="Drag to reorder" aria-label="Drag to reorder">☰</span>`
      : '';
    return `
      <div class="${classes}"${dragAttrs}>
        <div class="queue-row">
          ${dragHandle}
          <div class="queue-meta">${displayText || 'Untitled'}</div>
          <div class="queue-actions">
            <button class="queue-action-btn queue-action-btn--play" type="button" data-queue-action="play" data-queue-index="${queueIndex}" aria-label="Play from queue"${actionsDisabled ? ' disabled' : ''}>⏵</button>
            <button class="queue-action-btn queue-action-btn--delete" type="button" data-queue-action="delete" data-queue-index="${queueIndex}" aria-label="Delete from queue"${actionsDisabled ? ' disabled' : ''}>X</button>
          </div>
        </div>
      </div>
    `;
  };

  const previousMarkup = previousItems.length > 0
    ? `
      <div class="queue-item is-previous">
        <button class="queue-toggle" type="button" data-toggle="previous" aria-expanded="${previousExpanded ? 'true' : 'false'}">${previousExpanded ? 'Hide previous' : 'Previous…'}</button>
        <div class="queue-sublist${previousExpanded ? '' : ' is-collapsed'}" data-previous-list ${previousExpanded ? '' : 'hidden'}>
          ${previousItems.map((entry) => buildItem(entry, 'previous')).join('')}
        </div>
      </div>
    `
    : '';
  previousQueueRendered = previousItems.length > 0;

  const currentMarkup = currentItem
    ? buildItem(currentItem, 'current')
    : '<div class="queue-item"><div class="queue-meta">No current song highlighted.</div></div>';
  const nextMarkup = nextItems.length > 0
    ? nextItems.map((entry) => buildItem(entry, 'next')).join('')
    : '<div class="queue-item"><div class="queue-meta">No upcoming songs.</div></div>';

  queueList.innerHTML = `${feedbackMarkup}${previousMarkup}${currentMarkup}${nextMarkup}`;
}

async function refreshQueueFromProxy() {
  if (queueDragActive || queueMoveInFlight) {
    scheduleQueueRefreshFromWS(400);
    return;
  }

  const requestId = ++queueRefreshToken;
  try {
    const response = await fetch('/api/queue', { headers: { Accept: 'application/json' } });
    const payload = await response.json();
    if (requestId !== queueRefreshToken || queueDragActive || queueMoveInFlight) {
      if (queueDragActive || queueMoveInFlight) {
        scheduleQueueRefreshFromWS(400);
      }
      return;
    }

    const status = payload?.data?.status || 'unavailable';
    const items = Array.isArray(payload?.data?.items) ? payload.data.items : [];

    if (status === 'unavailable') {
      // Keep last good queue on transient YTMD blips; retry once.
      if (playbackState.queue?.length && playbackState.queueStatus === 'ok') {
        scheduleQueueRefreshFromWS(1200);
        return;
      }
      playbackState.queue = [];
      playbackState.queueStatus = 'unavailable';
      renderQueue();
      return;
    }

    playbackState.queue = items;
    playbackState.queueStatus = status === 'ok' && items.length === 0 ? 'empty' : status;
    renderQueue();
  } catch (err) {
    if (requestId !== queueRefreshToken || queueDragActive || queueMoveInFlight) {
      if (queueDragActive || queueMoveInFlight) {
        scheduleQueueRefreshFromWS(400);
      }
      return;
    }
    if (playbackState.queue?.length && playbackState.queueStatus === 'ok') {
      scheduleQueueRefreshFromWS(1200);
      return;
    }
    playbackState.queue = [];
    playbackState.queueStatus = 'unavailable';
    renderQueue();
  }
}

function applyNowPlaying(payload) {
  const source = payload?.song ?? payload?.data?.song ?? payload?.data ?? payload;
  const song = source?.song ?? source;
  // Do not take play/pause from PLAYER_INFO / GET /song — isPaused is stale and
  // PLAYER_INFO.isPlaying is derived from it. Use PLAYER_STATE_CHANGED + position ticks.
  const position = Number(
    payload?.position
      ?? source?.position
      ?? song?.elapsedSeconds
      ?? song?.position
      ?? song?.currentTime
      ?? song?.progress
      ?? 0,
  );
  const duration = Number(source?.songDuration ?? song?.songDuration ?? song?.duration ?? song?.length ?? 0);
  const nextTitle = song?.title || song?.name || song?.track || source?.title || source?.name || '';
  const nextArtist = song?.artist || song?.artistName || song?.channel || song?.author || source?.artist || source?.artistName || '';
  const nextVideoId = song?.videoId || song?.id || source?.videoId || source?.id || source?.video?.id || '';

  if (typeof payload?.shuffle === 'boolean') {
    playbackState.shuffle = payload.shuffle;
  } else if (typeof source?.shuffle === 'boolean') {
    playbackState.shuffle = source.shuffle;
  }

  if (nextTitle || nextArtist || duration || position) {
    playbackState.hasSong = true;
    playbackState.position = position || playbackState.position;
    playbackState.duration = duration || playbackState.duration;
    playbackState.title = nextTitle || playbackState.title;
    playbackState.artist = nextArtist || playbackState.artist;
    playbackState.videoId = nextVideoId || playbackState.videoId;
  }

  const metaText = `${playbackState.artist || 'Unknown artist'} • ${playbackState.title || 'Unknown title'}`;
  if (getCurrentMetaText() !== metaText) {
    setCurrentMetaText(metaText);
    updateCurrentMetaMarquee();
  }
  playState.textContent = playbackState.isPaused ? '⏸' : '▶';
  currentTime.textContent = formatTime(playbackState.position);
  totalTime.textContent = formatTime(playbackState.duration);
  updatePlaybackButtons();
  updateProgressBar();
}

async function refreshNowPlayingFromProxy() {
  const requestId = ++nowPlayingRefreshToken;
  try {
    const response = await fetch('/cmd/ytmd/song/get', { headers: { Accept: 'application/json' } });
    if (!response.ok) {
      throw new Error(`status ${response.status}`);
    }

    const payload = await response.json();
    if (requestId !== nowPlayingRefreshToken) {
      return;
    }

    applyNowPlaying(payload);
  } catch (err) {
    if (requestId !== nowPlayingRefreshToken) {
      return;
    }

    setCurrentMetaText('YTMD unavailable');
    updateCurrentMetaMarquee();
    playbackState.isPaused = true;
    playState.textContent = '⏸';
    currentTime.textContent = '00:00';
    totalTime.textContent = '00:00';
    updatePlaybackButtons();
  }
}

function connectNowPlaying() {
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
  const host = window.location.hostname || 'localhost';
  const port = new URLSearchParams(window.location.search).get('ytmdPort') || '26538';
  const wsUrl = `${protocol}://${host}:${port}/api/v1/ws`;

  if (window.__jukeboksSocket && window.__jukeboksSocket.readyState === WebSocket.OPEN) {
    return;
  }

  const socket = new WebSocket(wsUrl);
  socket.onopen = () => {
    setCurrentMetaText('Listening for YTMD…');
    currentMeta.classList.remove('is-marquee');
    startPolling();
  };
  socket.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      if (data?.type === 'SHUFFLE_CHANGED' && typeof data.shuffle === 'boolean') {
        playbackState.shuffle = data.shuffle;
        updatePlaybackButtons();
        return;
      }
      if (data?.type === 'POSITION_CHANGED') {
        applyPositionFromWS(data.position);
        return;
      }
      if (data?.type === 'PLAYER_STATE_CHANGED') {
        applyPlayerStateFromWS(data);
        return;
      }
      if (data?.type === 'PLAYER_INFO' || data?.type === 'VIDEO_CHANGED') {
        applyNowPlaying(data);
        scheduleQueueRefreshFromWS();
        return;
      }
      if (data?.song || data?.artist || data?.title) {
        applyNowPlaying(data);
        scheduleQueueRefreshFromWS();
        return;
      }
      if (data?.position != null && !data?.song && !data?.title && !data?.artist) {
        applyPositionFromWS(data.position);
        return;
      }
      if (typeof data?.isPaused === 'boolean') {
        applyPlayerStateFromWS(data);
      }
    } catch (err) {
      console.error('Failed to parse YTMD socket payload', err);
    }
  };
  socket.onerror = () => {
    setCurrentMetaText('YTMD socket unavailable');
  };
  socket.onclose = () => {
    startPolling();
    window.setTimeout(connectNowPlaying, 2000);
  };
  window.__jukeboksSocket = socket;
}

async function sendPlayerCommand(path) {
  try {
    await fetch(path, { method: 'POST', headers: { Accept: 'application/json' } });
    window.setTimeout(() => {
      // WS already owns play/pause; HTTP song.isPaused is stale and was wiping the glow.
      if (!isNowPlayingSocketOpen()) {
        refreshNowPlayingFromProxy();
      }
      refreshQueueFromProxy();
      refreshShuffleFromProxy();
    }, 600);
  } catch (err) {
    output.textContent = `Control request failed: ${err.message}`;
  }
}

async function submitSongRequest() {
  if (!songRequestInput) return;
  const input = songRequestInput.value.trim();
  if (!input) return;
  try {
    const response = await fetch(
      `/cmd/jb/songrequest?input=${encodeURIComponent(input)}`,
      { headers: { Accept: 'application/json' } },
    );
    const payload = await readCommandEnvelope(response);
    songRequestInput.value = '';
    setQueueActionFeedback(payload.message || 'Added to queue', 'success');
    refreshQueueFromProxy();
  } catch (err) {
    setQueueActionFeedback(err.message || 'Song request failed', 'error');
  }
}

async function refreshShuffleFromProxy() {
  try {
    const response = await fetch('/cmd/ytmd/shuffle/get', { headers: { Accept: 'application/json' } });
    const payload = await readCommandEnvelope(response);
    const data = payload?.data ?? payload;
    if (typeof data?.state === 'boolean') {
      playbackState.shuffle = data.state;
      updatePlaybackButtons();
    }
  } catch (err) {
    console.error('Failed to refresh shuffle state', err);
  }
}

async function seekToPosition(seconds) {
  const targetSeconds = Math.max(0, Math.floor(Number(seconds) || 0));
  playbackState.position = targetSeconds;
  updateProgressBar();
  const path = `/cmd/ytmd/seek-to/post?seconds=${targetSeconds}`;
  await sendPlayerCommand(path);
}

Object.entries(playerButtons).forEach(([key, button]) => {
  if (!button) return;
  let path = '';
  switch (key) {
    case 'prev':
      path = '/cmd/ytmd/previous/post';
      break;
    case 'back':
      path = '/cmd/ytmd/go-back/post';
      break;
    case 'play':
      path = '/cmd/ytmd/play/post';
      break;
    case 'pause':
      path = '/cmd/ytmd/pause/post';
      break;
    case 'forward':
      path = '/cmd/ytmd/go-forward/post';
      break;
    case 'next':
      path = '/cmd/ytmd/next/post';
      break;
    case 'shuffle':
      path = '/cmd/ytmd/shuffle/post';
      break;
  }

  button.addEventListener('click', async () => {
    if (key === 'play') {
      setAdminPaused(false);
      notePlaybackProgress();
    } else if (key === 'pause') {
      window.clearTimeout(progressStallTimer);
      setAdminPaused(true);
    } else if (key === 'shuffle') {
      playbackState.shuffle = !playbackState.shuffle;
      updatePlaybackButtons();
    }

    if (key === 'back') {
      await seekToPosition(playbackState.position - 10);
      return;
    }
    if (key === 'forward') {
      await seekToPosition(playbackState.position + 10);
      return;
    }
    if (path) {
      await sendPlayerCommand(path);
    }
  });
});

if (songRequestForm) {
  songRequestForm.addEventListener('submit', (event) => {
    event.preventDefault();
    submitSongRequest();
  });
}
if (songRequestInput) {
  // keydown covers tools/browsers that don't synthesize form submit from Enter
  songRequestInput.addEventListener('keydown', (event) => {
    if (event.key === 'Enter') {
      event.preventDefault();
      submitSongRequest();
    }
  });
}

progressBar.addEventListener('input', () => {
  playbackState.position = Number(progressBar.value) || 0;
  updateProgressBar();
});

progressBar.addEventListener('change', async () => {
  await seekToPosition(progressBar.value);
});

renderCommands();
loadSettings();
refreshNowPlayingFromProxy();
refreshQueueFromProxy();
refreshShuffleFromProxy();
connectNowPlaying();
startPolling();
window.addEventListener('resize', updateCurrentMetaMarquee);
window.addEventListener('focus', () => {
  refreshPlaybackFromProxyIfNeeded();
  connectNowPlaying();
  updateCurrentMetaMarquee();
});
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) {
    refreshPlaybackFromProxyIfNeeded();
    connectNowPlaying();
    updateCurrentMetaMarquee();
  }
});

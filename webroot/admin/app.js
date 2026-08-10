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
const tableBody = document.getElementById('commandTableBody');
const queueList = document.getElementById('queueList');
const blacklistInput = document.getElementById('blacklistInput');
const addBlacklistButton = document.getElementById('addBlacklistButton');
const blacklistList = document.getElementById('blacklistList');
const settingsOutput = document.getElementById('settingsOutput');
const configPathLabel = document.getElementById('configPathLabel');
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
};

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
  tableBody.innerHTML = '';

  commands.forEach((cmd) => {
    const option = document.createElement('option');
    option.value = cmd.name;
    option.textContent = `${cmd.name} (${cmd.methods.join(', ')})`;
    option.dataset.description = cmd.description;
    select.appendChild(option);

    const row = document.createElement('tr');
    row.innerHTML = `
      <td><strong>${cmd.name}</strong></td>
      <td>${cmd.methods.map((method) => `<span class="method-pill">${method}</span>`).join('')}</td>
      <td><code>${cmd.path}</code></td>
      <td>${cmd.description}</td>
      <td><button class="command-link" type="button" data-command="${cmd.name}">Use</button></td>
    `;
    tableBody.appendChild(row);
  });

  select.addEventListener('change', () => {
    const selected = commands.find((cmd) => cmd.name === select.value);
    updateSelection(selected);
  });

  tableBody.querySelectorAll('.command-link').forEach((button) => {
    button.addEventListener('click', () => {
      const selected = commands.find((cmd) => cmd.name === button.dataset.command);
      if (!selected) return;
      select.value = selected.name;
      updateSelection(selected);
      argInput.focus();
    });
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
  });
});

async function loadSettings() {
  try {
    const response = await fetch('/api/config');
    const payload = await response.json();
    if (!response.ok || payload.exitCode !== 0) {
      throw new Error(payload.message || 'Unable to load settings');
    }
    renderBlacklist(payload.data?.blacklist || []);
    configPathLabel.textContent = 'Config file: jukeboks.json';
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
  queue: [],
};
let queueRefreshToken = 0;
let nowPlayingRefreshToken = 0;
let previousQueueExpanded = false;
let previousQueueRendered = false;
let queueActionFeedback = null;
let queueActionFeedbackTimer = null;

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
  queueActionFeedback = { message, kind };
  renderQueue();

  if (queueActionFeedbackTimer) {
    window.clearTimeout(queueActionFeedbackTimer);
  }
  queueActionFeedbackTimer = window.setTimeout(() => {
    queueActionFeedback = null;
    renderQueue();
  }, 1400);
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
    const response = await fetch(path, init);
    if (!response.ok) {
      throw new Error(`status ${response.status}`);
    }

    button.classList.remove('is-pending');
    button.classList.add('is-success');
    button.textContent = '✓';
    setQueueActionFeedback(action === 'delete' ? 'Deleted queue item' : 'Started queue item', 'success');

    if (action !== 'delete') {
      playbackState.queue = classifyQueueEntries(playbackState.queue, queueIndex, playbackState.queue);
      renderQueue();
    }

    window.setTimeout(() => {
      refreshQueueFromProxy();
      refreshNowPlayingFromProxy();
    }, 700);
  } catch (err) {
    button.classList.remove('is-pending');
    button.classList.add('is-error');
    button.textContent = '!';
    setQueueActionFeedback(action === 'delete' ? 'Unable to delete queue item' : 'Unable to start queue item', 'error');
    console.error('Queue action failed', err);
  } finally {
    window.setTimeout(() => {
      resetQueueActionButtonState(button, action);
    }, 900);
  }
}

if (queueList) {
  queueList.addEventListener('click', (event) => {
    if (shouldHandleQueueAction(event)) {
      handleQueueItemAction(event);
      return;
    }
    handleQueueToggle(event);
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

function updatePlaybackButtons() {
  if (!playerButtons.play || !playerButtons.pause) return;
  playerButtons.play.classList.toggle('is-active', playbackState.hasSong && !playbackState.isPaused);
  playerButtons.pause.classList.toggle('is-active', playbackState.hasSong && playbackState.isPaused);
}

function updateProgressBar() {
  const maxDuration = Math.max(1, playbackState.duration || 100);
  progressBar.max = String(maxDuration);
  progressBar.value = String(Math.min(maxDuration, playbackState.position || 0));
  currentTime.textContent = formatTime(playbackState.position);
  totalTime.textContent = formatTime(playbackState.duration);
}

function extractTextFromRuns(value) {
  if (!value) return '';
  if (typeof value === 'string') return value;
  if (Array.isArray(value)) {
    return value.map((entry) => extractTextFromRuns(entry)).filter(Boolean).join('');
  }
  if (typeof value === 'object') {
    if (Array.isArray(value.runs)) {
      return value.runs.map((entry) => extractTextFromRuns(entry)).filter(Boolean).join('');
    }
    if (typeof value.text === 'string') {
      return value.text;
    }
    if (typeof value.simpleText === 'string') {
      return value.simpleText;
    }
    if (typeof value.label === 'string') {
      return value.label;
    }
    if (typeof value.displayText === 'string') {
      return value.displayText;
    }
    if (typeof value.name === 'string') {
      return value.name;
    }
    if (typeof value.title === 'string') {
      return value.title;
    }
  }
  return '';
}

function normalizeBylineArtist(value) {
  const text = String(value || '').trim();
  if (!text) {
    return '';
  }

  const separators = [' • ', '•', ' - ', ' – ', ' / ', ' | ', ' — '];
  for (const separator of separators) {
    if (text.includes(separator)) {
      const parts = text.split(separator).map((part) => part.trim()).filter(Boolean);
      if (parts.length > 0) {
        return parts[0];
      }
    }
  }

  return text;
}

function escapeRegExp(value) {
  return String(value || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function canonicalizeQueueTitle(title, artist = '') {
  let normalized = String(title || '').trim();
  if (!normalized) {
    return '';
  }

  normalized = normalized
    .replace(/[\u2010\u2011\u2012\u2013\u2014\u2015]/g, '-')
    .replace(/\s+/g, ' ')
    .replace(/[-–—:|/•]+/g, ' ')
    .replace(/\s+/g, ' ');

  const artistText = String(artist || '').trim();
  if (artistText) {
    const artistPrefix = new RegExp(`^${escapeRegExp(artistText)}(?:\\s*)`, 'i');
    normalized = normalized.replace(artistPrefix, '');
  }

  normalized = normalized
    .replace(/\s*\((?:official|hd|music|lyric|digital)?\s*(?:video|music video|audio|lyric video|video clip)\s*\)$/i, '')
    .replace(/\s*\[(?:official|hd|music|lyric|digital)?\s*(?:video|music video|audio|lyric video|video clip)\s*\]$/i, '')
    .replace(/\s*[-–—:|/•]\s*(?:official|hd|music|lyric|digital)?\s*(?:video|music video|audio|lyric video|video clip)\s*$/i, '')
    .replace(/\b(?:official|hd|music|lyric|digital)\s+(?:video|music video|audio|lyric video|video clip)\b/gi, '')
    .replace(/\s+/g, ' ')
    .replace(/\s+([,.;:!?])/g, '$1');

  return normalized.trim();
}

function pickPreferredText(values) {
  for (const value of values) {
    const text = extractTextFromRuns(value);
    if (text && text.trim()) {
      return text.trim();
    }
  }
  return '';
}

function isMeaningfulQueueText(value) {
  const text = String(value || '').trim().toLowerCase();
  return Boolean(text) && !['untitled', 'unknown', 'unknown title', 'unknown artist', 'n/a', 'na'].includes(text);
}

function hasMeaningfulQueueMetadata(entry) {
  const normalized = normalizeQueueItem(entry);
  if (!normalized) return false;
  const title = String(normalized.title || '').trim();
  const artist = String(normalized.artist || '').trim();
  return isMeaningfulQueueText(title) || isMeaningfulQueueText(artist);
}

function extractQueueIndexFromContext(value, fallback = 0) {
  if (!value || typeof value !== 'object') {
    return fallback;
  }

  if (Array.isArray(value)) {
    for (const entry of value) {
      const resolved = extractQueueIndexFromContext(entry, fallback);
      if (resolved !== fallback) {
        return resolved;
      }
    }
    return fallback;
  }

  const candidates = [
    value.currentIndex,
    value.index,
    value.current,
    value.selectedIndex,
    value.position,
  ];

  for (const candidate of candidates) {
    const num = Number(candidate);
    if (Number.isFinite(num) && num >= 0) {
      return num;
    }
  }

  for (const [key, entry] of Object.entries(value)) {
    if (key === 'currentIndex' || key === 'index' || key === 'current' || key === 'selectedIndex' || key === 'position') {
      continue;
    }
    const resolved = extractQueueIndexFromContext(entry, fallback);
    if (resolved !== fallback) {
      return resolved;
    }
  }

  return fallback;
}

function findQueueRendererCandidate(value, seen = new WeakSet()) {
  if (!value || typeof value !== 'object') return null;
  if (seen.has(value)) return null;
  seen.add(value);

  if (Array.isArray(value)) {
    for (const entry of value) {
      const resolved = findQueueRendererCandidate(entry, seen);
      if (resolved) {
        return resolved;
      }
    }
    return null;
  }

  const directKeys = ['playlistPanelVideoRenderer', 'videoRenderer', 'musicResponsiveListItemRenderer', 'playlistPanelRenderer'];
  for (const key of directKeys) {
    const candidate = value[key];
    if (candidate && typeof candidate === 'object') {
      return candidate;
    }
  }

  for (const key of ['primaryRenderer', 'renderer', 'playlistPanelVideoWrapperRenderer', 'counterpartRenderer']) {
    const candidate = value[key];
    const resolved = findQueueRendererCandidate(candidate, seen);
    if (resolved) {
      return resolved;
    }
  }

  for (const candidate of Object.values(value)) {
    if (!candidate || typeof candidate !== 'object') {
      continue;
    }
    const resolved = findQueueRendererCandidate(candidate, seen);
    if (resolved) {
      return resolved;
    }
  }

  return null;
}

function isQueueEntryLike(value) {
  if (!value || typeof value !== 'object') return false;
  if (Array.isArray(value)) return false;

  const directQueueKeys = ['playlistPanelVideoWrapperRenderer', 'playlistPanelVideoRenderer', 'videoRenderer', 'musicResponsiveListItemRenderer', 'playlistPanelRenderer'];
  const hasDirectQueueShape = directQueueKeys.some((key) => Boolean(value[key]));
  const renderer = findQueueRendererCandidate(value);
  const normalized = normalizeQueueItem(value);
  const hasMeaningfulText = hasMeaningfulQueueMetadata(value);
  const hasDirectMetadata = Boolean(
    value.title
    || value.titleText
    || value.alternativeTitle
    || value.altTitle
    || value.secondaryTitle
    || value.songTitle
    || value.longBylineText
    || value.shortBylineText
    || value.videoId
    || value.id
    || value.selected
    || value.isSelected
    || value.isCurrent
    || value.current
    || value.navigationEndpoint?.watchEndpoint?.videoId
    || value.watchEndpoint?.videoId
  );

  return Boolean(
    hasMeaningfulText
    || (hasDirectQueueShape && Boolean(renderer) && hasDirectMetadata)
  );
}

function collectQueueEntries(value, collected = [], seen = new WeakSet(), seenKeys = new Set(), queueIndex = null, treatArraysAsQueue = false) {
  if (Array.isArray(value)) {
    if (!treatArraysAsQueue) {
      return collected;
    }
    value.forEach((entry, index) => collectQueueEntries(entry, collected, seen, seenKeys, index, false));
    return collected;
  }

  if (!value || typeof value !== 'object') {
    return collected;
  }

  if (seen.has(value)) {
    return collected;
  }
  seen.add(value);

  if (isQueueEntryLike(value)) {
    const normalizedEntry = normalizeQueueItem(value);
    const hasMeaningfulText = hasMeaningfulQueueMetadata(value);
    if (normalizedEntry && hasMeaningfulText) {
      const signatures = collectQueueEntrySignatures(value, normalizedEntry);
      const isDuplicate = signatures.some((signature) => seenKeys.has(signature));
      if (!isDuplicate) {
        signatures.forEach((signature) => seenKeys.add(signature));
        value.__queueKey = signatures[0] || '';
        collected.push({ item: value, queueIndex: Number.isFinite(queueIndex) ? queueIndex : 0 });
      }
    }
  }

  Object.values(value).forEach((entry) => {
    if (!entry || typeof entry !== 'object') {
      return;
    }
    collectQueueEntries(entry, collected, seen, seenKeys, queueIndex, false);
  });

  return collected;
}

function collectQueueEntrySignatures(item, normalizedEntry) {
  const signatures = [];
  const canonicalTitle = canonicalizeQueueTitle(normalizedEntry?.title || '', normalizedEntry?.artist || '');
  const canonicalArtist = normalizeBylineArtist(normalizedEntry?.artist || '');
  const titleKey = normalizeText(canonicalTitle);
  const artistKey = normalizeText(canonicalArtist);

  if (titleKey && artistKey) {
    signatures.push(`pair:${artistKey}::${titleKey}`);
  }

  if (normalizedEntry?.videoId && (!titleKey || !artistKey)) {
    signatures.push(`video:${String(normalizedEntry.videoId).toLowerCase()}`);
  }

  const titleVariants = collectQueueTitleVariants(item, normalizedEntry);
  const artistVariants = collectQueueArtistVariants(item, normalizedEntry);
  for (const title of titleVariants) {
    for (const artist of artistVariants) {
      if (title && artist) {
        signatures.push(`title:${title.toLowerCase()}|artist:${artist.toLowerCase()}`);
      }
    }
  }

  if (titleVariants[0] && artistVariants[0]) {
    signatures.push(`title:${titleVariants[0].toLowerCase()}|artist:${artistVariants[0].toLowerCase()}`);
  }

  return [...new Set(signatures.filter(Boolean))];
}

function collectQueueTitleVariants(item, normalizedEntry) {
  const values = [];
  const register = (value) => {
    const text = String(value || '').trim();
    if (text) {
      values.push(text);
    }
  };

  const renderer = findQueueRendererCandidate(item) || item?.playlistPanelVideoRenderer || item?.videoRenderer || item?.musicResponsiveListItemRenderer || item?.playlistPanelRenderer || null;
  register(normalizedEntry?.title);
  register(item?.title);
  register(item?.titleText);
  register(item?.displayTitle);
  register(item?.shortTitle);
  register(item?.songTitle);
  register(item?.track);
  register(item?.name);
  register(item?.alternativeTitle);
  register(item?.altTitle);
  register(item?.secondaryTitle);
  register(renderer?.title && extractTextFromRuns(renderer?.title));
  register(renderer?.titleText && extractTextFromRuns(renderer?.titleText));
  register(renderer?.displayTitle && extractTextFromRuns(renderer?.displayTitle));
  register(renderer?.shortTitle && extractTextFromRuns(renderer?.shortTitle));
  register(renderer?.songTitle && extractTextFromRuns(renderer?.songTitle));
  register(renderer?.track && extractTextFromRuns(renderer?.track));
  register(renderer?.name && extractTextFromRuns(renderer?.name));
  register(renderer?.alternativeTitle && extractTextFromRuns(renderer?.alternativeTitle));
  register(renderer?.altTitle && extractTextFromRuns(renderer?.altTitle));
  register(renderer?.secondaryTitle && extractTextFromRuns(renderer?.secondaryTitle));

  return [...new Set(values.map((value) => value.toLowerCase()))];
}

function collectQueueArtistVariants(item, normalizedEntry) {
  const values = [];
  const register = (value) => {
    const text = String(value || '').trim();
    if (text) {
      values.push(text);
    }
  };

  const renderer = findQueueRendererCandidate(item) || item?.playlistPanelVideoRenderer || item?.videoRenderer || item?.musicResponsiveListItemRenderer || item?.playlistPanelRenderer || null;
  register(normalizedEntry?.artist);
  register(item?.artist);
  register(item?.artistName);
  register(item?.author);
  register(item?.channel);
  register(renderer?.artist && extractTextFromRuns(renderer?.artist));
  register(renderer?.artistName && extractTextFromRuns(renderer?.artistName));
  register(renderer?.author && extractTextFromRuns(renderer?.author));
  register(renderer?.channel && extractTextFromRuns(renderer?.channel));
  register(renderer?.longBylineText && extractTextFromRuns(renderer?.longBylineText));
  register(renderer?.shortBylineText && extractTextFromRuns(renderer?.shortBylineText));
  register(renderer?.bylineText && extractTextFromRuns(renderer?.bylineText));
  register(renderer?.authorText && extractTextFromRuns(renderer?.authorText));
  register(renderer?.ownerText && extractTextFromRuns(renderer?.ownerText));

  return [...new Set(values.map((value) => value.toLowerCase()))];
}

function normalizeQueueItem(item) {
  if (!item) return null;
  if (typeof item === 'string') {
    return { title: item, artist: '', selected: false, videoId: '' };
  }

  const renderer = findQueueRendererCandidate(item) || item.playlistPanelVideoRenderer || item.videoRenderer || item.musicResponsiveListItemRenderer || item.playlistPanelRenderer || item;
  const titleCandidates = [
    renderer?.title,
    renderer?.titleText,
    renderer?.displayTitle,
    renderer?.shortTitle,
    renderer?.songTitle,
    renderer?.track,
    renderer?.name,
    item?.title,
    item?.titleText,
    item?.displayTitle,
    item?.shortTitle,
    item?.songTitle,
    item?.track,
    item?.name,
    renderer?.alternativeTitle,
    renderer?.altTitle,
    renderer?.secondaryTitle,
    item?.alternativeTitle,
    item?.altTitle,
    item?.secondaryTitle,
  ];
  const artistCandidates = [
    renderer?.longBylineText,
    renderer?.shortBylineText,
    renderer?.bylineText,
    renderer?.authorText,
    renderer?.ownerText,
    renderer?.artist,
    renderer?.artistName,
    item?.artist,
    item?.artistName,
    item?.author,
    item?.channel,
  ];
  const rawArtistText = pickPreferredText(artistCandidates);
  const artist = normalizeBylineArtist(rawArtistText);
  const title = canonicalizeQueueTitle(pickPreferredText(titleCandidates), artist);
  const duration = extractTextFromRuns(renderer?.lengthText || renderer?.lengthText?.runs || renderer?.durationText || renderer?.durationText?.runs || item?.duration || item?.durationText || item?.length || item?.lengthText);
  const selected = Boolean(
    renderer?.selected
    || renderer?.isSelected
    || renderer?.isCurrent
    || renderer?.current
    || item?.selected
    || item?.isSelected
    || item?.isCurrent
    || item?.current
  );
  const videoId = renderer?.videoId
    || renderer?.id
    || renderer?.navigationEndpoint?.watchEndpoint?.videoId
    || item?.videoId
    || item?.id
    || item?.navigationEndpoint?.watchEndpoint?.videoId
    || item?.watchEndpoint?.videoId
    || '';

  return {
    title: title.trim(),
    artist: artist.trim(),
    duration: duration.trim(),
    selected,
    videoId,
    rawItem: item,
  };
}

function normalizeText(value) {
  return String(value || '').trim().toLowerCase();
}

function createSongIdentity(source) {
  if (!source) return { title: '', artist: '', videoId: '' };
  const title = normalizeText(source.title || source.name || source.track || source.songTitle || '');
  const artist = normalizeText(source.artist || source.artistName || source.channel || source.author || source.songArtist || '');
  const videoId = normalizeText(source.videoId || source.id || source.video?.id || source.url || '');
  return { title, artist, videoId };
}

function extractQueueIdentity(item) {
  if (!item) return { title: '', artist: '', videoId: '' };
  const renderer = findQueueRendererCandidate(item) || item.playlistPanelVideoRenderer || item.videoRenderer || item.musicResponsiveListItemRenderer || item.playlistPanelRenderer || item;
  const artistCandidates = [
    renderer?.longBylineText,
    renderer?.shortBylineText,
    renderer?.bylineText,
    renderer?.authorText,
    renderer?.ownerText,
    renderer?.artist,
    renderer?.artistName,
    item?.artist,
    item?.artistName,
    item?.author,
    item?.channel,
  ];
  const rawArtistText = pickPreferredText(artistCandidates);
  const artist = normalizeText(normalizeBylineArtist(rawArtistText));
  const title = normalizeText(canonicalizeQueueTitle(pickPreferredText([
    renderer?.title,
    renderer?.titleText,
    renderer?.displayTitle,
    renderer?.shortTitle,
    renderer?.songTitle,
    renderer?.track,
    renderer?.name,
    item?.title,
    item?.titleText,
    item?.displayTitle,
    item?.shortTitle,
    item?.songTitle,
    item?.track,
    item?.name,
    renderer?.alternativeTitle,
    renderer?.altTitle,
    renderer?.secondaryTitle,
    item?.alternativeTitle,
    item?.altTitle,
    item?.secondaryTitle,
  ]), pickPreferredText(artistCandidates)));
  const videoId = normalizeText(renderer?.videoId || renderer?.id || renderer?.navigationEndpoint?.watchEndpoint?.videoId || item?.videoId || item?.id || '');
  return { title, artist, videoId };
}

function matchesCurrentSong(normalized, currentTitle, currentArtist, currentVideoId) {
  if (!normalized) return false;
  const entry = extractQueueIdentity(normalized.rawItem || normalized.item || normalized);
  const title = normalizeText(currentTitle);
  const artist = normalizeText(currentArtist);
  const videoId = normalizeText(currentVideoId);
  const selected = Boolean(normalized.selected || normalized.rawItem?.selected || normalized.rawItem?.isCurrent || normalized.rawItem?.current || normalized.item?.selected || normalized.item?.isCurrent || normalized.item?.current);

  if (selected) return true;

  if (videoId && entry.videoId) {
    if (entry.videoId === videoId) return true;
  }

  if (!title && !artist) return false;
  if (title && entry.title) {
    const titleMatch = entry.title === title || entry.title.includes(title) || title.includes(entry.title);
    if (titleMatch) return true;
  }
  if (artist && entry.artist) {
    const artistMatch = entry.artist === artist || entry.artist.includes(artist) || artist.includes(entry.artist);
    if (artistMatch) return true;
  }
  return false;
}

function classifyQueueEntries(entries, currentIndex = 0, context = null) {
  const currentTitle = playbackState.title || '';
  const currentArtist = playbackState.artist || '';
  const currentVideoId = playbackState.videoId || '';
  const fallbackIndex = Number.isFinite(currentIndex) && currentIndex >= 0 ? currentIndex : 0;
  const normalizedIndex = extractQueueIndexFromContext(context || entries, fallbackIndex);

  const normalizedEntries = entries
    .map((entry, index) => {
      const normalized = normalizeQueueItem(entry?.item || entry);
      if (!normalized || (!normalized.title && !normalized.artist && !normalized.videoId)) return null;
      const sourceQueueIndex = Number.isFinite(entry?.queueIndex) ? entry.queueIndex : (Number.isFinite(entry?.sourceIndex) ? entry.sourceIndex : index);
      return {
        ...normalized,
        queueIndex: sourceQueueIndex,
        sourceIndex: sourceQueueIndex,
      };
    })
    .filter(Boolean);

  if (normalizedEntries.length === 0) {
    return [];
  }

  const dedupedEntries = [];
  const seenIdentityKeys = new Set();
  for (const entry of normalizedEntries) {
    const identityKey = buildQueueIdentityKey(entry);
    if (!identityKey || seenIdentityKeys.has(identityKey)) {
      continue;
    }
    seenIdentityKeys.add(identityKey);
    dedupedEntries.push(entry);
  }

  const explicitCurrentIndex = Number.isFinite(normalizedIndex) && normalizedIndex >= 0 && normalizedIndex < dedupedEntries.length
    ? normalizedIndex
    : -1;

  let candidateCurrentIndex = -1;
  const currentEntry = dedupedEntries.find((entry, index) => {
    if (entry.selected) {
      candidateCurrentIndex = index;
      return true;
    }
    if (currentVideoId && entry.videoId && entry.videoId.toLowerCase() === currentVideoId.toLowerCase()) {
      candidateCurrentIndex = index;
      return true;
    }
    if (matchesCurrentSong(entry, currentTitle, currentArtist, currentVideoId)) {
      candidateCurrentIndex = index;
      return true;
    }
    return false;
  });

  const currentEntryIndex = currentEntry ? dedupedEntries.indexOf(currentEntry) : -1;
  const resolvedCurrentIndex = currentEntryIndex >= 0
    ? currentEntryIndex
    : (explicitCurrentIndex >= 0 ? explicitCurrentIndex : (candidateCurrentIndex >= 0 ? candidateCurrentIndex : 0));

  return dedupedEntries.map((entry, index) => {
    const queueIndex = Number.isFinite(entry?.sourceIndex) ? entry.sourceIndex : index;
    if (index === resolvedCurrentIndex) {
      return { item: entry, kind: 'current', queueIndex };
    }

    return index < resolvedCurrentIndex
      ? { item: entry, kind: 'previous', queueIndex }
      : { item: entry, kind: 'next', queueIndex };
  });
}

function buildQueueIdentityKey(entry) {
  const artist = normalizeBylineArtist(entry?.artist || '');
  const title = canonicalizeQueueTitle(entry?.title || '', artist);
  const videoId = normalizeText(entry?.videoId || '').trim();
  const canonicalTitle = normalizeText(title).trim();
  const canonicalArtist = normalizeText(artist).trim();
  if (canonicalTitle || canonicalArtist) {
    return `pair:${canonicalArtist}::${canonicalTitle}`.toLowerCase();
  }
  if (videoId) {
    return `video:${videoId}`.toLowerCase();
  }
  return '';
}

function renderQueue() {
  if (!queueList) return;

  const isPaused = Boolean(playbackState.hasSong && playbackState.isPaused);

  const feedbackMarkup = queueActionFeedback
    ? `<div class="queue-feedback queue-feedback--${queueActionFeedback.kind}">${queueActionFeedback.message}</div>`
    : '';

  if (isPaused) {
    queueList.innerHTML = `${feedbackMarkup}<div class="queue-empty-state">Queue unavailable while the player is Paused.</div>`;
    return;
  }

  if (!playbackState.queue?.length) {
    queueList.innerHTML = `${feedbackMarkup}<div class="queue-empty-state">Queue telemetry unavailable. Awaiting playback activity.</div>`;
    return;
  }

  const visibleQueueItems = (playbackState.queue || []).filter((item) => {
    const normalized = normalizeQueueItem(item?.item || item);
    return hasMeaningfulQueueMetadata(item?.item || item) && Boolean(normalized?.title || normalized?.artist);
  });
  const previousItems = visibleQueueItems.filter((item) => item.kind === 'previous');
  const currentItem = visibleQueueItems.find((item) => item.kind === 'current');
  const nextItems = visibleQueueItems.filter((item) => item.kind === 'next');
  const previousExpanded = previousItems.length > 0 ? previousQueueExpanded : false;
  if (previousItems.length === 0) {
    previousQueueExpanded = false;
  }

  const buildItem = (item, kind) => {
    const normalized = normalizeQueueItem(item?.item || item);
    if (!normalized || (!normalized.title && !normalized.artist && !normalized.videoId)) return '';
    const displayText = [normalized.artist, normalized.title].filter(Boolean).join(' - ');
    const classes = [`queue-item`, kind === 'current' ? 'is-current' : '', kind === 'previous' ? 'is-previous' : ''].filter(Boolean).join(' ');
    const queueIndex = Number.isFinite(item?.queueIndex) ? item.queueIndex : '';
    return `
      <div class="${classes}">
        <div class="queue-row">
          <div class="queue-meta">${displayText || 'Untitled'}</div>
          <div class="queue-actions">
            <button class="queue-action-btn queue-action-btn--play" type="button" data-queue-action="play" data-queue-index="${queueIndex}" aria-label="Play from queue">⏵</button>
            <button class="queue-action-btn queue-action-btn--delete" type="button" data-queue-action="delete" data-queue-index="${queueIndex}" aria-label="Delete from queue">X</button>
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

  const currentMarkup = currentItem ? buildItem(currentItem, 'current') : '<div class="queue-item"><div class="queue-meta">No current song.</div></div>';
  const nextMarkup = nextItems.length > 0
    ? nextItems.map((entry) => buildItem(entry, 'next')).join('')
    : '<div class="queue-item"><div class="queue-meta">No upcoming songs.</div></div>';

  queueList.innerHTML = `${feedbackMarkup}${previousMarkup}${currentMarkup}${nextMarkup}`;
}

function applyNowPlaying(payload) {
  const source = payload?.song ?? payload?.data?.song ?? payload?.data ?? payload;
  const song = source?.song ?? source;
  const isPaused = typeof source?.isPaused === 'boolean'
    ? source.isPaused
    : typeof song?.isPaused === 'boolean'
      ? song.isPaused
      : (typeof song?.isPlaying === 'boolean' ? !song.isPlaying : false);
  const position = Number(source?.position ?? song?.position ?? song?.currentTime ?? song?.progress ?? 0);
  const duration = Number(source?.songDuration ?? song?.songDuration ?? song?.duration ?? song?.length ?? 0);
  const nextTitle = song?.title || song?.name || song?.track || source?.title || source?.name || '';
  const nextArtist = song?.artist || song?.artistName || song?.channel || song?.author || source?.artist || source?.artistName || '';
  const nextVideoId = song?.videoId || song?.id || source?.videoId || source?.id || source?.video?.id || '';

  if (nextTitle || nextArtist || duration || position || typeof source?.isPaused === 'boolean' || typeof song?.isPaused === 'boolean' || typeof song?.isPlaying === 'boolean') {
    playbackState.hasSong = true;
    playbackState.isPaused = isPaused;
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
  if (playbackState.queue?.length) {
    playbackState.queue = classifyQueueEntries(playbackState.queue, 0, playbackState.queue);
    renderQueue();
  }
}

async function refreshQueueFromProxy() {
  const requestId = ++queueRefreshToken;
  try {
    const response = await fetch('/cmd/ytmd/queue/get', { headers: { Accept: 'application/json' } });
    if (!response.ok) {
      throw new Error(`status ${response.status}`);
    }

    const payload = await response.json();
    const queueData = payload?.data ?? payload;
    const queueEntries = collectQueueEntries(queueData?.items ?? queueData?.entries ?? queueData?.contents ?? queueData?.queue ?? queueData?.content, [], new WeakSet(), new Set(), 0, true);
    const currentIndex = Number(queueData?.currentIndex ?? queueData?.index ?? queueData?.current ?? 0);

    if (requestId !== queueRefreshToken) {
      return;
    }

    const hasMeaningfulQueuePayload = queueEntries.length > 0 || (queueData && typeof queueData === 'object' && (Array.isArray(queueData.items) || Array.isArray(queueData.entries) || Array.isArray(queueData.contents) || Array.isArray(queueData.queue) || Array.isArray(queueData.content)));

    if (hasMeaningfulQueuePayload && queueEntries.length > 0) {
      playbackState.queue = classifyQueueEntries(queueEntries, currentIndex, queueData);
      renderQueue();
      return;
    }

    playbackState.queue = [];
    renderQueue();
  } catch (err) {
    if (requestId !== queueRefreshToken) {
      return;
    }

    playbackState.queue = [];
    renderQueue();
  }
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
    playState.textContent = '⏸';
    currentTime.textContent = '00:00';
    totalTime.textContent = '00:00';
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
    refreshNowPlayingFromProxy();
  };
  socket.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      if (data?.type === 'PLAYER_INFO' || data?.type === 'VIDEO_CHANGED' || data?.type === 'PLAYER_STATE_CHANGED' || data?.type === 'POSITION_CHANGED' || data?.song || data?.position || data?.isPaused || data?.artist || data?.title) {
        applyNowPlaying(data);
        window.setTimeout(() => {
          refreshQueueFromProxy();
        }, 250);
      }
    } catch (err) {
      console.error('Failed to parse YTMD socket payload', err);
    }
  };
  socket.onerror = () => {
    setCurrentMetaText('YTMD socket unavailable');
  };
  socket.onclose = () => {
    window.setTimeout(connectNowPlaying, 2000);
  };
  window.__jukeboksSocket = socket;
}

async function sendPlayerCommand(path) {
  try {
    await fetch(path, { method: 'POST', headers: { Accept: 'application/json' } });
    window.setTimeout(() => {
      refreshNowPlayingFromProxy();
      refreshQueueFromProxy();
    }, 600);
  } catch (err) {
    output.textContent = `Control request failed: ${err.message}`;
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
  }

  button.addEventListener('click', async () => {
    if (key === 'play') {
      playbackState.isPaused = false;
      updatePlaybackButtons();
      playState.textContent = '▶';
    } else if (key === 'pause') {
      playbackState.isPaused = true;
      updatePlaybackButtons();
      playState.textContent = '⏸';
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
connectNowPlaying();
window.setInterval(() => {
  refreshNowPlayingFromProxy();
  refreshQueueFromProxy();
}, 10000);
window.addEventListener('resize', updateCurrentMetaMarquee);
window.addEventListener('focus', () => {
  refreshNowPlayingFromProxy();
  connectNowPlaying();
  updateCurrentMetaMarquee();
});
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) {
    refreshNowPlayingFromProxy();
    connectNowPlaying();
    updateCurrentMetaMarquee();
  }
});

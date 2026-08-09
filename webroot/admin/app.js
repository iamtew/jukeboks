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
const currentMeta = document.getElementById('currentMeta');
const playState = document.getElementById('playState');
const currentTime = document.getElementById('currentTime');
const totalTime = document.getElementById('totalTime');
const progressBar = document.getElementById('progressBar');
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
  hasSong: false,
};

function formatTime(seconds) {
  const totalSeconds = Number.isFinite(seconds) ? Math.max(0, Math.floor(seconds)) : 0;
  const mins = String(Math.floor(totalSeconds / 60)).padStart(2, '0');
  const secs = String(totalSeconds % 60).padStart(2, '0');
  return `${mins}:${secs}`;
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

  if (nextTitle || nextArtist || duration || position || typeof source?.isPaused === 'boolean' || typeof song?.isPaused === 'boolean' || typeof song?.isPlaying === 'boolean') {
    playbackState.hasSong = true;
    playbackState.isPaused = isPaused;
    playbackState.position = position || playbackState.position;
    playbackState.duration = duration || playbackState.duration;
    playbackState.title = nextTitle || playbackState.title;
    playbackState.artist = nextArtist || playbackState.artist;
  }

  const metaText = `${playbackState.artist || 'Unknown artist'} • ${playbackState.title || 'Unknown title'}`;
  currentMeta.textContent = metaText;
  currentMeta.classList.toggle('is-marquee', metaText.length > 28);
  playState.textContent = playbackState.isPaused ? '⏸' : '▶';
  currentTime.textContent = formatTime(playbackState.position);
  totalTime.textContent = formatTime(playbackState.duration);
  updatePlaybackButtons();
  updateProgressBar();
}

async function refreshNowPlayingFromProxy() {
  try {
    const response = await fetch('/cmd/ytmd/song/get', { headers: { Accept: 'application/json' } });
    if (!response.ok) {
      throw new Error(`status ${response.status}`);
    }

    const payload = await response.json();
    applyNowPlaying(payload);
  } catch (err) {
    currentMeta.textContent = 'YTMD unavailable';
    currentMeta.classList.remove('is-marquee');
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
    currentMeta.textContent = 'Listening for YTMD…';
    currentMeta.classList.remove('is-marquee');
    refreshNowPlayingFromProxy();
  };
  socket.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      if (data?.type === 'PLAYER_INFO' || data?.type === 'VIDEO_CHANGED' || data?.type === 'PLAYER_STATE_CHANGED' || data?.type === 'POSITION_CHANGED' || data?.song || data?.position || data?.isPaused || data?.artist || data?.title) {
        applyNowPlaying(data);
      }
    } catch (err) {
      console.error('Failed to parse YTMD socket payload', err);
    }
  };
  socket.onerror = () => {
    currentMeta.textContent = 'YTMD socket unavailable';
  };
  socket.onclose = () => {
    window.setTimeout(connectNowPlaying, 2000);
  };
  window.__jukeboksSocket = socket;
}

async function sendPlayerCommand(path) {
  try {
    await fetch(path, { method: 'POST', headers: { Accept: 'application/json' } });
    window.setTimeout(refreshNowPlayingFromProxy, 600);
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
refreshNowPlayingFromProxy();
connectNowPlaying();
window.setInterval(refreshNowPlayingFromProxy, 10000);
window.addEventListener('focus', () => {
  refreshNowPlayingFromProxy();
  connectNowPlaying();
});
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) {
    refreshNowPlayingFromProxy();
    connectNowPlaying();
  }
});

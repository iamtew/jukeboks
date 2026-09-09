(function () {
  const output = document.getElementById("output");

  // Group UI elements by responsibility so the rest of the file stays readable.
  const ui = {
    songInfo: document.getElementById("songInfo"),
    artist: document.getElementById("artist"),
    artistWrap: document.getElementById("artistWrap"),
    title: document.getElementById("title"),
    titleWrap: document.getElementById("titleWrap"),
    album: document.getElementById("album"),
    albumWrap: document.getElementById("albumWrap"),
    playState: document.getElementById("playState"),
    currentTime: document.getElementById("currentTime"),
    totalTime: document.getElementById("totalTime"),
    marker: document.getElementById("marker"),
    bar: document.getElementById("bar"),
    fill: document.getElementById("fill"),
  };

  // Runtime state for the overlay.
  const state = {
    songDuration: 0,
    position: 0,
    pauseFadeTimer: null,
    isPlaybackActive: false,
  };

  // Timing constants for fade and marquee behavior.
  const timings = {
    fadeOutDelay: 180,
    fadeOutDuration: 600,
    fadeInDuration: 180,
  };

  // Enable dev mode if ?dev=true and allow host/port overrides via query params.
  const params = new URLSearchParams(window.location.search);
  const devMode = params.get("dev") === "true";
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const host = params.get("host") || window.location.hostname || "localhost";
  const port = params.get("port") || "26538";
  const pulseParam = params.get("pulse");
  const pulseSpeed = pulseParam === null ? 3 : Math.max(0, Math.min(10, Number(pulseParam) || 0));
  const bgParam = params.get("bg");
  const bgOpacityLevel = bgParam === null ? 0 : Math.max(0, Math.min(6, Number(bgParam) || 0));
  const WS_URL = `${protocol}://${host}:${port}/api/v1/ws`;

  if (devMode) {
    document.body.classList.add("dev");
    output.style.display = "block";
  } else {
    output.style.display = "none";
  }

  function applyBackgroundOpacity() {
    if (!ui.songInfo) return;

    const normalizedOpacity = bgOpacityLevel === 0 ? 0 : 0.08 + (bgOpacityLevel / 6) * 0.42;
    ui.songInfo.style.background = `linear-gradient(135deg,
      rgba(14, 34, 62, ${normalizedOpacity}),
      rgba(48, 92, 144, ${Math.min(0.5, normalizedOpacity * 0.8)}))`;
    ui.songInfo.style.borderColor = normalizedOpacity > 0 ? "rgba(170, 220, 255, 0.22)" : "rgba(170, 220, 255, 0)";
    ui.songInfo.style.boxShadow = normalizedOpacity > 0
      ? "inset 0 1px 0 rgba(255, 255, 255, 0.18), 0 10px 30px rgba(0, 0, 0, 0.25)"
      : "none";
  }

  applyBackgroundOpacity();

  let ws;

  // Connect to the WebSocket feed and handle the incoming player updates.
  function connect() {
    ws = new WebSocket(WS_URL);

    ws.onopen = () => {
      log({ status: "connected", url: WS_URL });
    };

    ws.onmessage = (msg) => {
      try {
        const data = JSON.parse(msg.data);

        if (data.type === "PLAYER_INFO" || data.type === "VIDEO_CHANGED") {
          if (data.song) {
            setOverlayField(ui.artist, ui.artistWrap, data.song.artist || "");
            setOverlayField(ui.title, ui.titleWrap, data.song.title || "");
            setOverlayField(ui.album, ui.albumWrap, data.song.album || "");

            if (data.song.songDuration) {
              state.songDuration = data.song.songDuration;
            }

            updateRemainingTime();

            ui.playState.textContent = data.song.isPaused ? "⏸" : "⏵";
            handlePlaybackState(Boolean(data.song.isPaused));
          }

          updateOutput(data);
        }

        if (data.type === "POSITION_CHANGED") {
          state.position = data.position || 0;
          ui.currentTime.textContent = formatTime(state.position);
          updateRemainingTime();
          updateMarker();
        }

        if (data.type === "PLAYER_STATE_CHANGED") {
          const isPaused = typeof data.isPlaying === "boolean" ? !data.isPlaying : Boolean(data.isPaused);
          ui.playState.textContent = isPaused ? "⏸" : "⏵";
          handlePlaybackState(isPaused);
        }

      } catch (err) {
        log({ error: "Failed to parse message", raw: msg.data });
      }
    };

    ws.onclose = () => {
      log({ status: "disconnected" });
      setTimeout(connect, 2000);
    };

    ws.onerror = (err) => {
      log({ error: "WebSocket error", details: err });
    };
  }

  // Update the progress bar and marker position from the latest position event.
  function updateMarker() {
    if (state.songDuration <= 0) return;

    const barWidth = ui.bar.clientWidth;
    const pct = state.position / state.songDuration;
    const x = Math.min(barWidth - 6, Math.max(0, barWidth * pct));
    const fillWidth = Math.max(0, Math.min(barWidth, barWidth * pct));

    ui.marker.style.transform = `translateX(${x}px)`;
    ui.fill.style.width = `${fillWidth}px`;
  }

  function updateRemainingTime() {
    if (!ui.totalTime) return;

    const remaining = Math.max(0, state.songDuration - state.position);
    ui.totalTime.textContent = formatTime(remaining);
  }

  // Chromatic glow is a CSS keyframe; JS only toggles the class and duration.
  function startGlowPulse() {
    if (!ui.songInfo) return;
    if (pulseSpeed <= 0) {
      stopGlowPulse();
      return;
    }
    const speedScale = Math.max(0.1, pulseSpeed / 5);
    ui.songInfo.style.setProperty("--glow-duration", `${(2 / speedScale).toFixed(2)}s`);
    ui.songInfo.classList.add("is-glowing");
    state.isPlaybackActive = true;
  }

  function stopGlowPulse() {
    state.isPlaybackActive = false;
    ui.songInfo?.classList.remove("is-glowing");
  }

  function formatTime(sec) {
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  }

  function updateOutput(data) {
    if (!devMode) return;
    output.textContent = JSON.stringify(data, null, 2);
  }

  function getMarqueeTrack(textEl) {
    if (!textEl) return null;

    let track = textEl.querySelector('.overlay-marquee-track');
    if (track) {
      return track;
    }

    track = document.createElement('span');
    track.className = 'overlay-marquee-track';
    track.textContent = textEl.textContent || '';
    textEl.textContent = '';
    textEl.appendChild(track);
    return track;
  }

  function setMarqueeText(textEl, value) {
    const track = getMarqueeTrack(textEl);
    if (!track) return false;

    const nextValue = String(value || '');
    if (track.textContent === nextValue) {
      return false;
    }

    track.textContent = nextValue;
    return true;
  }

  function updateTextMarquee(textEl, wrapEl) {
    if (!textEl || !wrapEl) return;

    const track = getMarqueeTrack(textEl);
    if (!track) return;

    textEl.classList.remove('is-marquee');
    textEl.style.removeProperty('--marquee-distance');

    window.requestAnimationFrame(() => {
      const wrapWidth = wrapEl.clientWidth;
      const textWidth = track.scrollWidth;
      const overflowWidth = textWidth - wrapWidth;

      if (overflowWidth > 4) {
        textEl.style.setProperty('--marquee-distance', `${overflowWidth}px`);
        textEl.classList.add('is-marquee');
      }
    });
  }

  function setOverlayField(textEl, wrapEl, value) {
    const changed = setMarqueeText(textEl, value);
    if (changed) {
      updateTextMarquee(textEl, wrapEl);
    }
  }

  function log(obj) {
    if (!devMode) return;
    output.textContent = JSON.stringify(obj, null, 2);
  }

  function setSongInfoOpacity(opacity, duration) {
    if (!ui.songInfo) return;
    ui.songInfo.style.transition = `opacity ${duration}ms ease`;
    ui.songInfo.style.opacity = String(opacity);
  }

  function resetPauseFade() {
    if (state.pauseFadeTimer) {
      clearTimeout(state.pauseFadeTimer);
      state.pauseFadeTimer = null;
    }
  }

  // Pause handling: dim the overlay after a short delay and stop the glow while faint.
  function handlePlaybackState(isPaused) {
    resetPauseFade();

    if (isPaused) {
      state.pauseFadeTimer = window.setTimeout(() => {
        setSongInfoOpacity(0.25, timings.fadeOutDuration);
        stopGlowPulse();
      }, timings.fadeOutDelay);
      return;
    }

    setSongInfoOpacity(1, timings.fadeInDuration);
    startGlowPulse();
  }

  // Measure marquee overflow once the layout is ready.
  window.requestAnimationFrame(() => {
    updateTextMarquee(ui.title, ui.titleWrap);
    updateTextMarquee(ui.artist, ui.artistWrap);
    updateTextMarquee(ui.album, ui.albumWrap);
  });

  window.addEventListener("resize", () => {
    updateTextMarquee(ui.title, ui.titleWrap);
    updateTextMarquee(ui.artist, ui.artistWrap);
    updateTextMarquee(ui.album, ui.albumWrap);
  });

  connect();
})();

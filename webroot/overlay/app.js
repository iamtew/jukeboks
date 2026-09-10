(function () {
  const output = document.getElementById("output");

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

  const state = {
    songDuration: 0,
    position: 0,
    pauseFadeTimer: null,
    progressStallTimer: null,
    isPaused: true,
    fadeGeneration: 0,
  };

  const timings = {
    fadeOutDelay: 180,
    fadeOutDuration: 600,
    fadeInDuration: 180,
  };
  // YTMD position ticks are often ~1s; keep headroom so we don't false-pause between ticks.
  const PROGRESS_STALL_MS = 1800;

  const params = new URLSearchParams(window.location.search);
  const devMode = params.get("dev") === "true";
  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const host = params.get("host") || window.location.hostname || "localhost";
  const port = params.get("port") || "26538";
  const pulseParam = params.get("pulse");
  const pulseSpeed = pulseParam === null ? 3 : Math.max(0, Math.min(10, Number(pulseParam) || 0));
  const bgParam = params.get("bg");
  // Default glass on; ?bg=0 / false for text-only.
  const useGlass = !(bgParam === "0" || bgParam === "false");
  const WS_URL = `${protocol}://${host}:${port}/api/v1/ws`;

  if (devMode) {
    document.documentElement.classList.add("dev");
    document.body.classList.add("dev");
    if (output) output.style.display = "block";
  } else if (output) {
    output.style.display = "none";
  }

  // Glass = translucent fill (alpha). That is what shows checkerboard/OBS through.
  // Do not use backdrop-filter here: when it can't sample, Chromium paints an opaque slab.
  function applyGlass() {
    if (!ui.songInfo) return;

    if (!useGlass) {
      ui.songInfo.style.background = "transparent";
      ui.songInfo.style.borderColor = "transparent";
      ui.songInfo.style.boxShadow = "none";
      ui.songInfo.style.backdropFilter = "none";
      ui.songInfo.style.webkitBackdropFilter = "none";
      return;
    }

    ui.songInfo.style.background = `linear-gradient(135deg,
      rgba(14, 34, 62, 0.66),
      rgba(48, 92, 144, 0.33))`;
    ui.songInfo.style.borderColor = "rgba(170, 220, 255, 0.28)";
    ui.songInfo.style.boxShadow =
      "inset 0 1px 0 rgba(255, 255, 255, 0.2), 0 10px 30px rgba(0, 0, 0, 0.28)";
    ui.songInfo.style.backdropFilter = "none";
    ui.songInfo.style.webkitBackdropFilter = "none";
  }

  applyGlass();

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

  function startGlowPulse() {
    if (!ui.songInfo) return;
    if (pulseSpeed <= 0) {
      ui.songInfo.classList.remove("is-glowing");
      return;
    }
    const speedScale = Math.max(0.1, pulseSpeed / 5);
    ui.songInfo.style.setProperty("--glow-duration", `${(2 / speedScale).toFixed(2)}s`);
    ui.songInfo.classList.add("is-glowing");
  }

  function stopGlowPulse() {
    ui.songInfo?.classList.remove("is-glowing");
  }

  // Paused → fade. Playing → full opacity.
  function handlePlaybackState(isPaused) {
    const next = Boolean(isPaused);
    if (state.isPaused === next) {
      if (ui.playState) ui.playState.textContent = next ? "⏸" : "⏵";
      return;
    }
    state.isPaused = next;
    if (ui.playState) ui.playState.textContent = next ? "⏸" : "⏵";
    resetPauseFade();
    state.fadeGeneration += 1;
    const generation = state.fadeGeneration;

    if (next) {
      state.pauseFadeTimer = window.setTimeout(() => {
        if (generation !== state.fadeGeneration) return;
        setSongInfoOpacity(0.25, timings.fadeOutDuration);
        stopGlowPulse();
      }, timings.fadeOutDelay);
      return;
    }

    setSongInfoOpacity(1, timings.fadeInDuration);
    startGlowPulse();
  }

  // YTMD sends POSITION_CHANGED while playing and stops when paused — that is the truth signal.
  function notePlaybackProgress() {
    window.clearTimeout(state.progressStallTimer);
    state.progressStallTimer = window.setTimeout(() => {
      if (state.isPaused) return;
      handlePlaybackState(true);
    }, PROGRESS_STALL_MS);
  }

  function formatTime(sec) {
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  }

  function updateMarker() {
    if (state.songDuration <= 0 || !ui.bar || !ui.marker || !ui.fill) return;
    const barWidth = ui.bar.clientWidth;
    const pct = state.position / state.songDuration;
    const x = Math.min(barWidth - 6, Math.max(0, barWidth * pct));
    const fillWidth = Math.max(0, Math.min(barWidth, barWidth * pct));
    ui.marker.style.transform = `translateX(${x}px)`;
    ui.fill.style.width = `${fillWidth}px`;
  }

  function updateRemainingTime() {
    if (!ui.totalTime) return;
    ui.totalTime.textContent = formatTime(Math.max(0, state.songDuration - state.position));
  }

  function updateOutput(data) {
    if (!devMode || !output) return;
    output.textContent = JSON.stringify(data, null, 2);
  }

  function log(obj) {
    if (!devMode || !output) return;
    output.textContent = JSON.stringify(obj, null, 2);
  }

  function getMarqueeTrack(textEl) {
    if (!textEl) return null;
    let track = textEl.querySelector(".overlay-marquee-track");
    if (track) return track;
    track = document.createElement("span");
    track.className = "overlay-marquee-track";
    track.textContent = textEl.textContent || "";
    textEl.textContent = "";
    textEl.appendChild(track);
    return track;
  }

  function setMarqueeText(textEl, value) {
    const track = getMarqueeTrack(textEl);
    if (!track) return false;
    const nextValue = String(value || "");
    if (track.textContent === nextValue) return false;
    track.textContent = nextValue;
    return true;
  }

  function updateTextMarquee(textEl, wrapEl) {
    if (!textEl || !wrapEl) return;
    const track = getMarqueeTrack(textEl);
    if (!track) return;
    textEl.classList.remove("is-marquee");
    textEl.style.removeProperty("--marquee-distance");
    window.requestAnimationFrame(() => {
      const overflowWidth = track.scrollWidth - wrapEl.clientWidth;
      if (overflowWidth > 4) {
        textEl.style.setProperty("--marquee-distance", `${overflowWidth}px`);
        textEl.classList.add("is-marquee");
      }
    });
  }

  function setOverlayField(textEl, wrapEl, value) {
    if (setMarqueeText(textEl, value)) {
      updateTextMarquee(textEl, wrapEl);
    }
  }

  let ws;

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

            if (typeof data.position === "number") {
              state.position = data.position;
            } else if (typeof data.song.elapsedSeconds === "number") {
              state.position = data.song.elapsedSeconds;
            }
            ui.currentTime.textContent = formatTime(state.position);
            updateMarker();
            updateRemainingTime();
            // Do not take play/pause from PLAYER_INFO — song.isPaused / isPlaying are stale.
            // POSITION_CHANGED stall + PLAYER_STATE_CHANGED are the truth.
          }
          updateOutput(data);
        }

        if (data.type === "POSITION_CHANGED") {
          const next = Math.max(0, Number(data.position) || 0);
          const advanced = next > state.position + 0.05;
          state.position = next;
          ui.currentTime.textContent = formatTime(state.position);
          updateRemainingTime();
          updateMarker();
          if (advanced) {
            handlePlaybackState(false);
            notePlaybackProgress();
          }
        }

        if (data.type === "PLAYER_STATE_CHANGED") {
          const isPaused = typeof data.isPlaying === "boolean" ? !data.isPlaying : Boolean(data.isPaused);
          handlePlaybackState(isPaused);
          if (isPaused) {
            window.clearTimeout(state.progressStallTimer);
          } else {
            notePlaybackProgress();
          }
          if (data.position != null) {
            state.position = Math.max(0, Number(data.position) || 0);
            ui.currentTime.textContent = formatTime(state.position);
            updateRemainingTime();
            updateMarker();
          }
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

  // Assume paused until position advances (matches admin; avoids stale isPaused).
  if (ui.playState) ui.playState.textContent = "⏸";
  setSongInfoOpacity(0.25, 0);
  stopGlowPulse();

  connect();
})();

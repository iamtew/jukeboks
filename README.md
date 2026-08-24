# jukeboks

Streamer.bot • YouTube Music • OBS — a lightweight control layer for YouTube Music Desktop.

Jukeboks is a local Go HTTP server that sits in front of [YouTube Music Desktop (YTMD / pear-desktop)](https://github.com/pear-devs/pear-desktop/) and turns it into a streamer control plane. Chat bots can drive playback, OBS can show a now-playing overlay, and a small admin dock can babysit the queue.

It is not Spotify, Discord, Twitch, or a multi-user voting jukebox. No database. No auth. One process, one config file, one `webroot/` of vanilla HTML/CSS/JS.

## Architecture

```
Streamer.bot / OBS / browser
        | REST /cmd /api /static
        v
   jukeboks :42420  ----proxy /api/v1---->  YTMD :26538
        |                                   ^
        |                                   | WebSocket /api/v1/ws (direct)
        +-- jukeboks.json + webroot         +-- Admin / Overlay live feed
```

| Piece | Port / path | Job |
|-------|-------------|-----|
| **jukeboks** | `:42420` (default) | HTTP proxy, policy, config API, static UIs |
| **YTMD** | `:26538` (default) | Music player + its own REST + WebSocket |
| **jukeboks.json** | next to the exe, or CWD | Blacklist + maxDuration |
| **webroot/** | served from disk | Landing, Admin, Overlay |

**WebSocket gotcha:** Admin and Overlay open a WebSocket **straight to YTMD** (`ws://…:26538/api/v1/ws`). Jukeboks does **not** proxy WebSockets. REST control goes through jukeboks so blacklist / duration policy can block bad requests. Live now-playing telemetry bypasses jukeboks. If OBS cannot see the song, check that the browser source can reach YTMD's WS port — not just `:42420`.

## Run

1. Start YTMD with its companion / API enabled (default `localhost:26538`).
2. From the repo root: `just run` (or run the packaged exe from a folder that contains `webroot/`).
3. Open `http://localhost:42420/` — Admin and Overlay links are on the landing page.
4. In OBS: Browser Source → Overlay URL. The OBS machine must be able to reach YTMD's WebSocket port.

```text
just run
# flags:
#   -port 42420
#   -ytmd_host localhost
#   -ytmd_port 26538
#   -webroot path/to/webroot     # optional; default is next to the exe, else ./webroot
#   -config path/to/jukeboks.json
```

If the preferred port is busy, jukeboks tries `port` … `port+9`. Ctrl+C / SIGTERM shuts down with a 5s timeout.

Packaging: `just package` builds `dist/jukeboks.exe` and copies `webroot/` beside it. Runtime looks for `webroot/` and `jukeboks.json` next to the executable when `webroot/` is there (so a Start Menu shortcut still works), otherwise in the current working directory. `just run` from the repo root keeps using `./webroot` and `./jukeboks.json`.

## Routes

| Route | What it does |
|-------|----------------|
| `GET /health` | `{ "exitCode": 0, "message": "ok" }` |
| `/cmd/ytmd`, `/cmd/ytmd/*` | Proxy to YTMD `/api/v1/*` with policy gates |
| `GET /cmd/jb/songinfo` | Human-readable current song for Streamer.bot / chat |
| `GET /cmd/jb/queueinfo` | Short queue summary (paused → refuses with `player_paused`) |
| Other `/cmd/jb/*` | Scaffold stub |
| `GET/POST/PUT /api/config` | Load / save `jukeboks.json` |
| `/`, `/admin*`, `/overlay*` | Static files from `webroot/` |

JSON envelope for API-ish responses (HTTP 200 even on app-level errors — Streamer.bot depends on that). Proxy failures talking to YTMD stay **502**:

```json
{ "exitCode": 0, "message": "optional", "data": {} }
```

Policy (blacklist + `maxDuration`, default 600 seconds, default blacklist entry **Rick Astley**) applies to proxied `/cmd/ytmd` requests only.

## Layout

```text
cmd/jukeboks/          flags, listen, shutdown
internal/config/       load/save/normalize, mtime-aware store
internal/policy/       blacklist + maxDuration
internal/ytmd/         upstream client, route map, song/queue parsers
internal/server/       mux, handlers, static files, proxy
webroot/               landing, admin, overlay
```

Zero third-party Go dependencies. Go 1.22. Tests: `just test`.

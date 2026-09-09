# jukeboks

A lightweight control layer for YouTube Music Desktop.

Jukeboks is a local Go HTTP server that sits in front of [YouTube Music Desktop (YTMD / pear-desktop)](https://github.com/pear-devs/pear-desktop/) and turns it into a streamer control plane. Chat bots can drive playback, OBS can show a now-playing overlay, and a small admin dock can babysit the queue.

No database. No auth. One process, one config file, one `webroot/` of vanilla HTML/CSS/JS.

**Default base URL:** `http://localhost:42420`

Can easily be integrated into your stream using a bot, for example:
- [Streamer.bot](https://streamer.bot/)
- [Kruiz Control](https://github.com/Kruiser8/Kruiz-Control)

---

## Custom commands (`/cmd/jb/*`)

These are the jukeboks-native endpoints. Use them from Streamer.bot / chat when you want a human-readable string (and structured `data`) instead of raw YTMD JSON.

All responses use the envelope below. App-level failures still return **HTTP 200** with `exitCode: 1` (Streamer.bot-friendly). Upstream/proxy hard failures talking to YTMD are **502**.

```json
{ "exitCode": 0, "message": "optional human text", "data": {} }
```

### `GET /cmd/jb/songinfo`

Current track summary for chat / bots.

| | |
|---|---|
| **Method** | `GET` only |
| **Upstream** | Retries `GET /api/v1/song` (3×, 250ms) |
| **Success** | `exitCode: 0` |
| **Message example** | `Song: Title — Artist. Playback state: playing.` |
| **Data** | `{ "title", "artist", "state" }` where `state` is `playing` or `paused` |
| **Failure** | `exitCode: 1`, `data.reason: "no_song"` when YTMD returns no usable title/artist |

```text
http://localhost:42420/cmd/jb/songinfo
```

### `GET /cmd/jb/queueinfo`

Short upcoming-queue summary for chat / bots.

| | |
|---|---|
| **Method** | `GET` only |
| **Upstream** | Retries `GET /api/v1/song` + `GET /api/v1/queue` |
| **Success** | `exitCode: 0` |
| **Message example** | `Queue: Artist - Title, … . N songs, MM:SS total.` |
| **Data** | `{ "songs", "totalSeconds", "display", "count" }` — up to 3 titles in `display`, full list in `songs` |
| **Paused** | `exitCode: 1`, `data.reason: "player_paused"` — refuses while the player is paused (bots should not announce a stale queue as “now”) |
| **Empty** | `exitCode: 1`, `data.reason: "empty_queue"` |

Behavior notes:

- Parses YTMD’s YouTube-renderer-shaped queue (`playlistPanelVideoRenderer`, wrappers, etc.).
- Starts the summary from the **current** track (prefers `videoId`, then title+artist).
- Keeps **duplicate** tracks that appear as separate `items[]` slots (same song queued twice stays twice).

```text
http://localhost:42420/cmd/jb/queueinfo
```

### `GET /api/queue`

Normalized queue rows for the admin UI (works while paused).

| | |
|---|---|
| **Method** | `GET` only |
| **Upstream** | Retries `GET /api/v1/song` + `GET /api/v1/queue` |
| **Success** | `exitCode: 0`, `data.status: "ok"` or `"empty"` |
| **Data** | `{ "status", "items": [{ "title", "artist", "videoId", "queueIndex", "kind" }] }` — `kind` is `previous` / `current` / `next` |
| **Error** | `exitCode: 1`, `data.status: "error"` or `"unavailable"` |

```text
http://localhost:42420/api/queue
```

### `GET /cmd/jb/songrequest`

Add a song to the **end** of the YTMD queue. Accepts a bare YouTube video ID, any supported YouTube / YouTube Music URL, or a free-text song query via a single query parameter.

| | |
|---|---|
| **Method** | `GET` only |
| **Parameter** | `input` (required) — bare 11-char video ID, YouTube URL, or song search query |
| **Upstream** | `POST /api/v1/search` (text queries), then `POST /api/v1/queue` with `{ "videoId", "insertPosition": "INSERT_AT_END" }` |
| **Success** | `exitCode: 0` |
| **Message example** | `Added to queue: Title — Artist.` |
| **Data** | `{ "videoId", "title", "artist" }` — title/artist when YTMD returns them |
| **Missing input** | `exitCode: 1`, message mentions missing input |
| **No search results** | `exitCode: 1`, message mentions `no matching search results` (text query with no sufficiently relevant YTMD hits) |
| **Unsupported URL** | `exitCode: 1`, `unsupported or invalid YouTube URL` — non-YouTube links, playlist-only links, and other URL-like input that cannot be resolved to a video ID are rejected (not searched) |
| **Input too long** | `exitCode: 1`, `input too long` — rejects inputs over 2048 bytes before upstream calls |
| **Policy** | Respects `jukeboks.json` blacklist and `maxDuration` — looks up title/artist/duration via YTMD search before queueing |
| **Blocked** | `exitCode: 1`, e.g. `request blocked by blacklist entry "Rick Astley"` or duration exceeds `maxDuration` |
| **Duplicate** | `exitCode: 1`, `data.reason: "duplicate"` — skips if the video ID is already in the queue or currently playing |
| **Seed mode** | Tracks seed IDs and an ordered request FIFO separately, synced from the live YTMD queue. Requests append via `INSERT_AT_END` then move after the current track / existing requests (FIFO), ahead of seed tracks. If `clearQueueOnRequest` is enabled, upcoming seed tracks are removed first |

```text
http://localhost:42420/cmd/jb/songrequest?input=dQw4w9WgXcQ
http://localhost:42420/cmd/jb/songrequest?input=https://youtu.be/dQw4w9WgXcQ
http://localhost:42420/cmd/jb/songrequest?input=https://music.youtube.com/watch?v=dQw4w9WgXcQ
http://localhost:42420/cmd/jb/songrequest?input=never+gonna+give+you+up
```

Supported `input` formats: bare video ID, `youtube.com/watch?v=`, `music.youtube.com/watch?v=`, `youtu.be/`, `/embed/`, `/shorts/`, scheme-less `youtu.be/...` links, or any text query (best matching YTMD search result is queued when title/artist overlap the query). URL-like input that is not a supported YouTube video link fails instead of falling through to search.

### Seed playlists

Seed playlists are fallback content for when the queue is idle. Admins save playlists in the **Seed** admin tab; clicking a saved playlist enqueues its tracks and enters **seed mode**. While seed mode is active, song requests are inserted after the current track. If there are already requested songs queued next, new requests go after those — keeping requests ahead of seed tracks.

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/seed/status` | GET | Seed mode state, saved playlists, `clearQueueOnRequest` |
| `/api/seed/resolve` | POST | Resolve URL/ID to `{ id, name, trackCount }` without saving |
| `/api/seed/playlists` | POST | Resolve and save a seed playlist to `jukeboks.json` |
| `/api/seed/playlists/{id}` | DELETE | Remove a saved seed playlist |
| `/api/seed/settings` | POST | Update `{ clearQueueOnRequest }` |
| `/cmd/jb/seed/enqueue?playlistId=…` | POST | Enqueue a saved seed playlist and enter seed mode |
| `/cmd/jb/seed/queue?playlistId=…` | DELETE | Remove that playlist's tracks from the YTMD queue |

Config fields in `jukeboks.json`:

```json
{
  "blacklist": ["Rick Astley"],
  "maxDuration": 600,
  "clearQueueOnRequest": false,
  "seedPlaylists": [
    { "id": "PLxxxxxxxx", "name": "My Seed Mix", "trackCount": 42 }
  ]
}
```

Seed mode state is runtime-only (resets when jukeboks restarts). Blacklist and `maxDuration` apply when enqueueing seed tracks.

Playlist lookup order: YouTube Music public browse API (supports `PL…` playlists and `OLAK5uy_…` albums), then YTMD search, then YTMD `playPlaylist` + queue read for playlists only visible inside your logged-in YTMD session.

### Other `/cmd/jb/*` (scaffold)

Any path under `/cmd/jb/` that is not `songinfo`, `queueinfo`, or `songrequest` returns a stub:

```json
{ "exitCode": 0, "message": "custom jukeboks command scaffold" }
```

Reserved for future custom commands. Not wired yet.

---

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
| **jukeboks** | `:42420` (default) | HTTP proxy, policy, config API, static UIs, custom `/cmd/jb` |
| **YTMD** | `:26538` (default) | Music player + its own REST + WebSocket |
| **jukeboks.json** | next to the exe, or CWD | Blacklist + `maxDuration` |
| **webroot/** | served from disk | Landing, Admin, Overlay |

**WebSocket gotcha:** Admin and Overlay open a WebSocket **straight to YTMD** (`ws://…:26538/api/v1/ws`). Jukeboks does **not** proxy WebSockets. REST control goes through jukeboks so blacklist / duration policy can block bad requests. Live now-playing telemetry bypasses jukeboks. If OBS cannot see the song, check that the browser source can reach YTMD’s WS port — not just `:42420`.

Optional query on Admin/Overlay: `?ytmdPort=26538` if YTMD listens elsewhere.

---

## Other jukeboks routes

| Route | Methods | What it does |
|-------|---------|----------------|
| `GET /health` | GET | `{ "exitCode": 0, "message": "ok" }` |
| `/cmd/ytmd`, `/cmd/ytmd/*` | * | Policy-gated proxy → YTMD `/api/v1/*` (see catalogs below) |
| `GET /api/config` | GET | Load `jukeboks.json` → `{ exitCode, data: { blacklist, maxDuration } }` |
| `POST` / `PUT /api/config` | POST, PUT | Save config body; returns saved payload |
| `GET /api/queue` | GET | Normalized admin queue rows (`items` + `kind`) |
| `/`, `/admin`, `/overlay`, … | GET | Static files from `webroot/` |

### Config shape (`jukeboks.json`)

```json
{
  "blacklist": ["Rick Astley"],
  "maxDuration": 600
}
```

| Field | Default | Meaning |
|-------|---------|---------|
| `blacklist` | `["Rick Astley"]` | Case-insensitive exact/substring match against query params (`query`, `title`, `artist`, `name`, `url`) and stringy JSON body values |
| `maxDuration` | `600` | Max allowed seconds from `duration` / `seconds` / `length` (query or body). Values `≤ 0` or `> 86400` normalize back to 600 |

Policy runs on **`/cmd/ytmd/*`** and on **`/cmd/jb/songrequest`** — not on other `/cmd/jb/*` or `/api/config`.

Blocked requests still return **HTTP 200** with `exitCode: 1` and a message like `request blocked by blacklist entry "Rick Astley"`.

---

## YTMD proxy (`/cmd/ytmd/*`)

Path rewrite: `/cmd/ytmd/play` → `http://ytmd:26538/api/v1/play`.

### Streamer.bot-friendly method suffixes

Anything that struggles with non-GET HTTP can encode the upstream method in the path:

| Client URL | Upstream |
|------------|----------|
| `GET /cmd/ytmd/play/post` | `POST /api/v1/play` |
| `GET /cmd/ytmd/queue/get` | `GET /api/v1/queue` |
| `GET /cmd/ytmd/queue/patch` | `PATCH /api/v1/queue` |
| `GET /cmd/ytmd/queue/3/delete` | `DELETE /api/v1/queue/3` |
| `GET /cmd/ytmd/seek-to/post?seconds=30` | `POST /api/v1/seek-to` with JSON body from query |

Supported suffixes: `get`, `post`, `put`, `patch`, `delete`.

Alternatives:

- Call with the real HTTP method (`POST /cmd/ytmd/play`).
- On GET, optional `?_method=POST` (etc.) override.

If a mutating method has an empty body, **query params are coerced into a JSON object** (`true`/`false`, ints, floats, else strings).

### Mutating / mixed commands (quick reference)

| jukeboks path | Typical method | Upstream | Notes |
|---------------|----------------|----------|-------|
| `/cmd/ytmd/play` | POST (`…/post`) | `POST /api/v1/play` | |
| `/cmd/ytmd/pause` | POST | `POST /api/v1/pause` | |
| `/cmd/ytmd/toggle-play` | POST | `POST /api/v1/toggle-play` | |
| `/cmd/ytmd/previous` | POST | `POST /api/v1/previous` | |
| `/cmd/ytmd/next` | POST | `POST /api/v1/next` | |
| `/cmd/ytmd/like` | POST | `POST /api/v1/like` | |
| `/cmd/ytmd/dislike` | POST | `POST /api/v1/dislike` | |
| `/cmd/ytmd/seek-to` | POST | `POST /api/v1/seek-to` | Query/body: `seconds` |
| `/cmd/ytmd/go-back` | POST | `POST /api/v1/go-back` | |
| `/cmd/ytmd/go-forward` | POST | `POST /api/v1/go-forward` | |
| `/cmd/ytmd/switch-repeat` | POST | `POST /api/v1/switch-repeat` | |
| `/cmd/ytmd/toggle-mute` | POST | `POST /api/v1/toggle-mute` | |
| `/cmd/ytmd/shuffle` | GET or POST | `GET` / `POST /api/v1/shuffle` | |
| `/cmd/ytmd/volume` | GET or POST | `GET` / `POST /api/v1/volume` | |
| `/cmd/ytmd/fullscreen` | GET or POST | `GET` / `POST /api/v1/fullscreen` | |
| `/cmd/ytmd/search` | POST | `POST /api/v1/search` | |
| `/cmd/ytmd/queue` | GET/POST/PATCH/DELETE | `/api/v1/queue` | See queue table |
| `/cmd/ytmd/queue/{index}` | PATCH/DELETE | `/api/v1/queue/{index}` | Move / remove |

#### Queue mutations

| Action | Example | Body / notes |
|--------|---------|--------------|
| Add song | `POST /cmd/ytmd/queue` or `…/queue/post` | `{ "videoId": "…", "insertPosition": "INSERT_AT_END" \| "INSERT_AFTER_CURRENT_VIDEO" }` |
| Jump to index | `PATCH /cmd/ytmd/queue` or `…/queue/patch` | `{ "index": 0 }` |
| Clear entire queue | `DELETE /cmd/ytmd/queue` or `…/queue/delete` | |
| Move item | `PATCH /cmd/ytmd/queue/{index}` | `{ "toIndex": N }` |
| Remove item | `DELETE /cmd/ytmd/queue/{index}` or `…/queue/{index}/delete` | |

Admin Queue tab uses these routes; indices match YTMD `items[]` slots (duplicates keep distinct indices).

The proxy is generic: any `/cmd/ytmd/<path>` maps to `/api/v1/<path>` even if it is not listed in Admin’s API panel. Prefer documented paths; YTMD’s own OpenAPI lives at `http://localhost:26538/doc` when the player is running.

---

## Admin & Overlay

| URL | Role |
|-----|------|
| `http://localhost:42420/` | Landing links |
| `http://localhost:42420/admin/` | Now-playing dock, transport, Queue / Seed / Settings / API tabs |
| `http://localhost:42420/overlay/` | OBS Browser Source now-playing |

**Admin Queue**

- Now-playing and shuffle update via YTMD WebSocket; queue falls back to `GET /cmd/ytmd/queue/get` on a 30s timer (or 10s when the socket is down).
- Highlights current via `selected` / `videoId` / exact title+artist.
- Works while **paused** (unlike `/cmd/jb/queueinfo`).
- Per-row Play → `PATCH …/queue/patch` `{ index }`; Delete → `DELETE …/queue/{index}/delete`.
- Upcoming rows: drag handle reorder → `PATCH …/queue/{index}` `{ toIndex }` (YTMD absolute indices).
- Clear queue → `DELETE …/queue/delete` (confirm).
- Honors `exitCode` on actions (policy blocks surface as error feedback).

**Settings** — edits `maxDuration` + blacklist → `POST /api/config`.

**API tab** — catalog of proxied commands; builds method-suffixed URLs for copy/paste into Streamer.bot.

---

## Run

1. Start YTMD with its companion / API enabled (default `localhost:26538`).
2. From the repo root: `just run` (or run the packaged exe from a folder that contains `webroot/`).
3. Open `http://localhost:42420/` — Admin and Overlay links are on the landing page.
4. In OBS: Browser Source → Overlay URL. The OBS machine must reach YTMD’s WebSocket port.

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

| Recipe | What |
|--------|------|
| `just run` | `go run ./cmd/jukeboks` |
| `just test` | `go test ./...` |
| `just build` | `jukeboks.exe` in repo root |
| `just package` | `dist/jukeboks.exe` + `dist/webroot/` |
| `just package-zip` | Zip of `dist/` (exe + webroot at archive root) |
| `just verify-package` | Rebuilds `dist/` and validates install layout (and zip if present) |
| `just fmt` | `gofmt` on `cmd` + `internal` |

Runtime looks for `webroot/` and `jukeboks.json` next to the executable when packaged (Start Menu shortcuts still work), otherwise in the current working directory. `just run` from the repo root uses `./webroot` and `./jukeboks.json`.

---

## Layout

```text
cmd/jukeboks/          flags, listen, shutdown
internal/config/       load/save/normalize, mtime-aware store
internal/policy/       blacklist + maxDuration
internal/ytmd/         upstream client, route map, song/queue parsers
internal/server/       mux, handlers, static files, proxy
webroot/               landing, admin, overlay
```

Zero third-party Go dependencies. Go 1.22+.

---

## YTMD GET commands (proxied)

Everything below is available through jukeboks as a **GET** (or `…/get` suffix). These are the read-only / state-fetch paths from YTMD’s API that the proxy exposes. Prefer `/get` suffixes from Streamer.bot.

Upstream YTMD OpenAPI (when running): `http://localhost:26538/doc`

| jukeboks (GET) | Upstream | Description |
|----------------|----------|-------------|
| `/cmd/ytmd/song` or `/cmd/ytmd/song/get` | `GET /api/v1/song` | Current song details (raw YTMD payload in `data`) |
| `/cmd/ytmd/song-info` or `/cmd/ytmd/song-info/get` | `GET /api/v1/song-info` | Current song info (YTMD alias of song) |
| `/cmd/ytmd/queue` or `/cmd/ytmd/queue/get` | `GET /api/v1/queue` | Full queue (`items`, `autoPlaying`, `continuation` — renderer soup) |
| `/cmd/ytmd/queue-info` or `/cmd/ytmd/queue-info/get` | `GET /api/v1/queue-info` | Queue info (YTMD alias of queue) |
| `/cmd/ytmd/queue/next` or `/cmd/ytmd/queue/next/get` | `GET /api/v1/queue/next` | Next song in queue (relative +1): title, artist, videoId, duration, … |
| `/cmd/ytmd/like-state` or `/cmd/ytmd/like-state/get` | `GET /api/v1/like-state` | Current like state |
| `/cmd/ytmd/shuffle` or `/cmd/ytmd/shuffle/get` | `GET /api/v1/shuffle` | Shuffle state |
| `/cmd/ytmd/repeat-mode` or `/cmd/ytmd/repeat-mode/get` | `GET /api/v1/repeat-mode` | Repeat mode |
| `/cmd/ytmd/volume` or `/cmd/ytmd/volume/get` | `GET /api/v1/volume` | Volume state |
| `/cmd/ytmd/fullscreen` or `/cmd/ytmd/fullscreen/get` | `GET /api/v1/fullscreen` | Fullscreen state |

### Related: not proxied as REST GETs through jukeboks

| Endpoint | Notes |
|----------|--------|
| `ws://localhost:26538/api/v1/ws` | YTMD WebSocket — Admin/Overlay connect **directly**; jukeboks does not proxy WS |
| `GET /cmd/jb/songinfo` | Custom jukeboks summary (see top) — not a YTMD passthrough |
| `GET /cmd/jb/queueinfo` | Custom jukeboks summary (see top) — not a YTMD passthrough |
| `GET /api/queue` | Normalized admin queue (works while paused) |
| `GET /cmd/jb/songrequest?input=…` | Add song to queue end (bare ID, YouTube URL, or text search query) |
| `GET /health` | jukeboks liveness |
| `GET /api/config` | jukeboks config |

### Copy-paste GET URLs (default ports)

```text
http://localhost:42420/cmd/ytmd/song/get
http://localhost:42420/cmd/ytmd/song-info/get
http://localhost:42420/cmd/ytmd/queue/get
http://localhost:42420/cmd/ytmd/queue-info/get
http://localhost:42420/cmd/ytmd/queue/next/get
http://localhost:42420/cmd/ytmd/like-state/get
http://localhost:42420/cmd/ytmd/shuffle/get
http://localhost:42420/cmd/ytmd/repeat-mode/get
http://localhost:42420/cmd/ytmd/volume/get
http://localhost:42420/cmd/ytmd/fullscreen/get
```

Custom (chat-friendly) GETs:

```text
http://localhost:42420/cmd/jb/songinfo
http://localhost:42420/cmd/jb/queueinfo
http://localhost:42420/api/queue
http://localhost:42420/cmd/jb/songrequest?input=dQw4w9WgXcQ
http://localhost:42420/health
http://localhost:42420/api/config
```

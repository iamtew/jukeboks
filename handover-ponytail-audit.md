# Handover: ponytail-audit (do not start in this chat)

Repo-wide over-engineering audit already run. Next agent should apply cuts (or re-audit) from this list — **do not re-discover from scratch unless verifying**.

Scope: complexity only. Not correctness/security/performance.

## Ranked findings

`yagni:` Client-side YTMD queue archaeology (~normalize/canonical/variant/signature walks). Serve normalized queue from Go (`/cmd/jb/queueinfo` already parses) or keep only `items[].playlistPanelVideoRenderer` + classify. [`webroot/admin/app.js`]

`delete:` Dead queue helpers never called: `collectQueueEntries`, `collectQueueEntrySignatures`, `createSongIdentity`, `buildQueueIdentityKey`. Nothing. [`webroot/admin/app.js`]

`native:` Hand-rolled pointer-capture drag (`handleQueuePointer*`, `elementFromPoint`). HTML5 `dragstart`/`drop`, or drop reorder. [`webroot/admin/app.js`, `webroot/admin/style.css`]

`delete:` `SeedRequestInsertIndex` (+ tests) — prod uses `seed.State.RequestInsertIndex` only. Nothing. [`internal/ytmd/playlist.go`, `internal/ytmd/queue_test.go`]

`delete:` Exported `LooksLikeURL` + `ExtractVideoID` (test-only wrappers; prod uses `ClassifySongRequestInput`). Test classify only. [`internal/ytmd/videoid.go`, `videoid_test.go`]

`yagni:` `lookupPlaylistViaPlayPlaylist` third fallback (mutates live YTMD queue after innertube+search). Drop; keep innertube + search. [`internal/ytmd/innertube.go`, `playlist.go`]

`yagni:` API panel select **and** full command table both drive `commands[]`. Keep select + Run. [`webroot/admin/index.html`, `app.js`]

`yagni:` `writeFileAtomic` CreateTemp/Sync/Chmod/Rename for single-process config. `os.WriteFile`. [`internal/config/config.go`]

`delete:` Non-functional Autoplay control (forced inactive / not in YTMD API). Remove button + CSS + `playerButtons.autoplay`. [`webroot/admin/index.html`, `style.css`, `app.js`]

`delete:` `QueueItemCount`, `State.Clear`, `RequestIDSet` (no prod callers); `ReconcileQueue` alias → call `SyncFromQueue`. [`internal/ytmd/playlist.go`, `internal/seed/state.go`]

`delete:` `findFirstSearchResult` (prod uses `findBestSearchResult` only). Retarget tests. [`internal/ytmd/search.go`, `search_test.go`]

`yagni:` Custom `http.Transport` dialer/keepalive in `NewClient`. `&http.Client{Timeout: …}`. [`internal/ytmd/client.go`]

`yagni:` `httplog` request scope (`WithScope`/`FromContext`) just to gate upstream logs. Log when enabled, or pass a bool. [`internal/httplog`, `internal/server/logging.go`]

`yagni:` Thin export wrappers (`CollectQueueEntries`, `RouteForPath`, `Build*Response`, …) that only rename privates. One name each. [`internal/ytmd`]

`shrink:` Duplicated song-payload flatten in `ParseCurrentSong` / `CurrentSongVideoID` / queue info. One helper. [`internal/ytmd/song.go`, `queue.go`]

`native:` Overlay glow via `requestAnimationFrame` + `textShadow`. CSS `@keyframes`. [`webroot/overlay/app.js`]

`stdlib:` `netJoin`. `net.JoinHostPort`. [`cmd/jukeboks/main.go`]

`delete:` Unused `.queue-label` CSS; dead `option.dataset.description` / static `configPathLabel` writes. Nothing. [`webroot/admin`]

## Estimate

`net: ~-900 lines, -0 Go deps` (optional −1 if GH-pages `marked` chrome goes).

## Already shipped (do not redo)

Seed playlist list: real names via `musicResponsiveHeaderRenderer` extraction + single-line ellipsis UI (`internal/ytmd/playlist.go`, `playlist_test.go`, `webroot/admin/app.js`, `style.css`).

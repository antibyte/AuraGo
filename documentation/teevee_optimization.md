# TeeVee Optimization Notes

## What TeeVee Is

TeeVee is a built-in virtual desktop app in AuraGo. It plays IPTV streams from the public `iptv-org.github.io` dataset inside a browser-based desktop window.

## Files

| File | Purpose |
|------|---------|
| `ui/js/desktop/apps/teevee.js` | Main app logic (catalog fetch, join, filter, playback, favorites) |
| `ui/css/teevee.css` | App-specific styles |
| `ui/js/desktop/core/media-helpers.js` | Shared utilities used by TeeVee and Radio |
| `ui/js/desktop/core/module-loader.js` | Lazy asset registry |
| `ui/desktop_teevee_test.go` | Static and behavioral marker tests |
| `internal/desktop/types.go` | Built-in app registration |

## Architecture

- **Pure client-side SPA.** Runs inside the AuraGo virtual desktop window surface.
- **Data source:** `iptv-org.github.io/api` endpoints:
  - `channels.json`
  - `streams.json`
  - `categories.json`
- **Join:** `joinStreamsWithChannels()` merges streams with channel/category metadata, filters out NSFW/closed/replaced streams, and builds a country index.
- **Cache:** 30-minute in-memory cache via `window.TeeVeeCatalogCache`.
- **State:** Favorites and recent channels are stored in `localStorage`.
- **Playback:** Native Safari HLS or `hls.js` for other browsers.

## Optimizations Applied

1. **Shared helpers** (`media-helpers.js`)
   - Deduplicated `clean`, `escapeHTML`, `normalizeSearch`, `countryFlag`, `countryDisplayName`, `debounce`, `createToast`, `updateMediaSession`, `hashString` between TeeVee and Radio.

2. **Stable identities**
   - Channel IDs and favorite keys are now `channelID + ':' + hash(url)` instead of index-based or raw URL keys.
   - Favorites are stored under `aurago.teevee.favorites.v2` with automatic migration from the legacy `v1` URL-based store.

3. **Performance**
   - Country index is built once during catalog join and cached.
   - Search/filter changes only re-render the channel list.
   - Channel list is paginated (`VISIBLE_BATCH = 40`) with an IntersectionObserver sentinel that loads more on scroll.
   - Catalog requests use `AbortController` with a 20s timeout.
   - Refresh uses `no-store` cache; initial load uses `force-cache`.

4. **Robustness**
   - Playback is only reset after the new source attaches successfully.
   - Non-fatal HLS errors are counted; playback stops after repeated failures.
   - `stalled` events are surfaced to the user.
   - Playback errors are categorized: timeout, network, CORS, format.

5. **UX**
   - Favorite button on every channel card.
   - Favorite toggle in the context menu.
   - Keyboard shortcuts: `Space` (play/pause), `F` (fullscreen), `M` (mute).
   - "Headers" badge has a tooltip explaining why the stream is blocked.

## Testing

Run the relevant tests with:

```bash
go test ./ui/...
go test ./internal/desktop/...
```

## Security Notes

- Stream URLs come from the upstream iptv-org dataset; playback relies on that dataset's trustworthiness.
- `crossOrigin = 'anonymous'` is set on the video element for privacy.
- The CSP `media-src` allows arbitrary `http:`/`https:` sources, which is required for IPTV but means a malicious catalog entry could point media anywhere.

package server

// The Retro Games player page (GET|HEAD /api/desktop/retro-games/player) is
// the only place where archive.org's code runs. It runs archive.org's
// Emularity loader in a page with an opaque origin: the CSP below carries a
// sandbox directive without allow-same-origin (the app frames the page with
// the same sandbox), so the loader and the emulators it pulls in cannot read
// AuraGo's cookies, storage, DOM or APIs, and the remaining directives only
// let them load code from and connect to archive.org hosts. The page itself is
// self-contained (inline CSS, two nonce'd inline scripts, no AuraGo URL) and
// takes the identifier, the muted flag and the sound label from values the
// server rendered, never from its own address. It talks to the app only via
// postMessage ({rg: 1, ...} both ways).

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"aurago/internal/retrogames"
)

// retroGamesPlayerCSP is the player page's Content-Security-Policy; %s is the
// per-response script nonce. archive.org's loader needs: its scripts under
// /services/emularity/ and on the emularity-*.ux-b hosts, wasm compilation,
// game data from archive.org/cors/*.us hosts and data: URLs (VICE fetches its
// wasm from one), workers from blob: URLs, and the splash image.
const retroGamesPlayerCSP = "sandbox allow-scripts allow-pointer-lock; " +
	"default-src 'none'; " +
	"script-src 'nonce-%s' https://archive.org/services/emularity/ https://emularity-engine.ux-b.archive.org https://emularity-config.ux-b.archive.org https://emularity-bios.ux-b.archive.org 'wasm-unsafe-eval'; " +
	"connect-src data: https://archive.org https://cors.archive.org https://*.us.archive.org https://emularity-engine.ux-b.archive.org https://emularity-config.ux-b.archive.org https://emularity-bios.ux-b.archive.org; " +
	"img-src data: blob: https://archive.org https://*.archive.org; " +
	"media-src data: blob:; " +
	"worker-src blob:; " +
	"style-src 'unsafe-inline'; " +
	"font-src data:; " +
	"base-uri https://archive.org; " +
	"form-action 'none'; " +
	"frame-ancestors 'self'; " +
	"object-src 'none'"

const (
	retroGamesPlayerDefaultSound  = "Click for sound"
	retroGamesPlayerSoundMaxRunes = 60
)

// retroGamesPlayerPage is the player document. html/template escapes the
// identifier and the sound label for their contexts; the scripts read them
// from the meta tags and the pill, so no value is spliced into script text.
//
// <base> comes first: the loader uses protocol-relative URLs, which on an
// http:// AuraGo would otherwise hit archive.org over http:// (301 without
// CORS headers). BrowserFS must load before loader.js. The bootstrap keeps
// every AudioContext the emulators create, so the first click or key can
// resume one the browser's autoplay policy suspended.
const retroGamesPlayerPage = `<!doctype html>
<html lang="en"><head><base href="https://archive.org/">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="rg-id" content="{{.ID}}">
<meta name="rg-muted" content="{{if .Muted}}1{{else}}0{{end}}">
<title>Retro Games</title>
<style>
html,body{margin:0;width:100%;height:100%;overflow:hidden;background:#000;color:#ddd;font:14px/1.4 system-ui,sans-serif}
#stage{position:absolute;inset:0}
#canvas{position:absolute;inset:0;display:block;width:100% !important;height:100% !important;object-fit:contain;image-rendering:pixelated;outline:none;background:#000}
.emularity-splash-screen{position:absolute;inset:0;z-index:2;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:12px;padding:16px;box-sizing:border-box;background:#000;color:#ddd;text-align:center}
.emularity-splash-image{max-width:40%;max-height:40%;image-rendering:pixelated}
.emularity-splash-title{font-size:15px;color:#eee}
.emularity-progress-indicator{max-width:90%;font-size:12px;color:#999}
#rg-sound{position:absolute;left:50%;bottom:16px;z-index:3;transform:translateX(-50%);max-width:calc(100% - 32px);box-sizing:border-box;padding:6px 14px;border:1px solid rgba(255,255,255,.35);border-radius:999px;background:rgba(0,0,0,.8);color:#fff;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;pointer-events:none}
#rg-sound[hidden]{display:none}
</style>
</head><body>
<div id="stage"><canvas id="canvas" tabindex="0"></canvas></div>
<div id="rg-sound" role="status" hidden>{{.Sound}}</div>
<script nonce="{{.Nonce}}">
const rgAudio = [];
for (const name of ['AudioContext', 'webkitAudioContext']) {
  const Native = window[name];
  if (typeof Native !== 'function') continue;
  const Tracked = function (...args) {
    const context = new Native(...args);
    rgAudio.push(context);
    return context;
  };
  Tracked.prototype = Native.prototype;
  window[name] = Tracked;
}
</script>
<script src="https://archive.org/services/emularity/engine/browserfs.min.js"></script>
<script src="https://archive.org/services/emularity/config/loader.js"></script>
<script nonce="{{.Nonce}}">
(() => {
  const id = document.querySelector('meta[name=rg-id]').content;
  let muted = document.querySelector('meta[name=rg-muted]').content === '1';
  const canvas = document.getElementById('canvas');
  const pill = document.getElementById('rg-sound');
  let emu = null;
  let state = '';
  let audioBlocked = false;

  function post(message) {
    window.parent.postMessage(message, '*');
  }

  function setState(next) {
    if (state === next || state === 'failed') return;
    state = next;
    post({ rg: 1, type: 'state', state: next });
    if (next === 'running') {
      checkAudio();
      setTimeout(checkAudio, 2000);
    }
  }

  function setAudioBlocked(blocked) {
    if (audioBlocked === blocked) return;
    audioBlocked = blocked;
    pill.hidden = !blocked;
    post({ rg: 1, type: 'audio', blocked: blocked });
  }

  function checkAudio() {
    if (state !== 'running' || muted) return;
    if (rgAudio.some((context) => context.state === 'suspended')) setAudioBlocked(true);
  }

  function resumeAudio() {
    if (muted) return;
    for (const context of rgAudio) {
      if (context.state === 'suspended') context.resume().catch(() => {});
    }
    setAudioBlocked(false);
  }

  function setMuted(next) {
    muted = next;
    if (emu) emu.setMute(muted);
    if (muted) setAudioBlocked(false);
    else checkAudio();
  }

  const watcher = new MutationObserver(() => {
    const title = document.querySelector('.emularity-splash-title');
    if (title && title.textContent.trim().startsWith('Failed')) fail();
  });
  watcher.observe(document.documentElement, { childList: true, subtree: true, characterData: true });

  function fail() {
    watcher.disconnect();
    setState('failed');
  }

  function start() {
    setState('loading');
    try {
      emu = new IALoader(canvas, id, { before_run: () => setState('running') });
      emu.setMute(muted);
      emu.start({ hasCustomCSS: true });
    } catch (error) {
      fail();
    }
    canvas.focus();
  }

  window.addEventListener('pointerdown', resumeAudio, true);
  window.addEventListener('keydown', resumeAudio, true);
  window.addEventListener('focus', () => canvas.focus());
  window.addEventListener('message', (event) => {
    const data = event.data;
    if (event.source !== window.parent || !data || data.rg !== 1) return;
    if (data.type === 'mute' && typeof data.muted === 'boolean') setMuted(data.muted);
    else if (data.type === 'focus') canvas.focus();
  });

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start);
  else start();
})();
</script>
</body></html>
`

var retroGamesPlayerTemplate = template.Must(template.New("retro-games-player").Parse(retroGamesPlayerPage))

type retroGamesPlayerData struct {
	Nonce string
	ID    string
	Muted bool
	Sound string
}

// retroGamesPlayerNonce returns 16 random bytes in unpadded URL-safe base64:
// valid as a CSP nonce-source and needing no escaping in an attribute.
func retroGamesPlayerNonce() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// retroGamesPlayerSoundLabel strips control characters (and invalid UTF-8),
// trims, caps the label at 60 runes and falls back to the English default.
func retroGamesPlayerSoundLabel(raw string) string {
	label := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(raw, ""))
	label = strings.TrimSpace(label)
	if utf8.RuneCountInString(label) > retroGamesPlayerSoundMaxRunes {
		label = strings.TrimSpace(string([]rune(label)[:retroGamesPlayerSoundMaxRunes]))
	}
	if label == "" {
		return retroGamesPlayerDefaultSound
	}
	return label
}

// ServeRetroGamesPlayer writes the player page for the request's id, muted
// and sound query parameters. It neither authenticates nor checks that Retro
// Games is on: handleRetroGames does both before it calls this. It is exported
// so the desktop browser tests (package ui) can serve the real page.
func ServeRetroGamesPlayer(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	id := query.Get("id")
	if !retrogames.ValidIdentifier(id) {
		retroGamesError(w, http.StatusBadRequest, "invalid_identifier", "Invalid game identifier")
		return
	}
	nonce, err := retroGamesPlayerNonce()
	if err != nil {
		retroGamesError(w, http.StatusInternalServerError, "retro_games_failed", "Retro Games failed")
		return
	}
	var page bytes.Buffer
	if err := retroGamesPlayerTemplate.Execute(&page, retroGamesPlayerData{
		Nonce: nonce,
		ID:    id,
		Muted: query.Get("muted") == "1",
		Sound: retroGamesPlayerSoundLabel(query.Get("sound")),
	}); err != nil {
		retroGamesError(w, http.StatusInternalServerError, "retro_games_failed", "Retro Games failed")
		return
	}
	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Cross-Origin-Resource-Policy", "same-origin")
	// securityHeadersMiddleware sends X-Frame-Options: DENY for every path;
	// only this answer replaces it (frame-ancestors 'self' in the CSP is the
	// real rule), so refusals on this route stay unframable.
	header.Set("X-Frame-Options", "SAMEORIGIN")
	header.Set("Content-Security-Policy", fmt.Sprintf(retroGamesPlayerCSP, nonce))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(page.Bytes())
}

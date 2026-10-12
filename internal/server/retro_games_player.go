package server

// The Retro Games player page (GET|HEAD /api/desktop/retro-games/player) is
// the only place where archive.org's code runs: Emularity's loader.js,
// BrowserFS and the emulators they pull in.
//
// What it protects is AuraGo's origin. The CSP's sandbox directive has no
// allow-same-origin (the app's iframe sandbox matches), so the page runs in
// an opaque origin and cannot read AuraGo's cookies, storage, DOM or API
// responses; script-src allows only the hashes of the page's two static
// inline scripts and archive.org hosts, and connect-src only archive.org
// hosts. Without allow-top-navigation and allow-popups it cannot navigate the
// desktop or open windows.
//
// What it does not protect: the page's own content (identifier, label) is not
// secret. archive.org's code runs in the same realm as the inline scripts, so
// it can change their state and post any {rg: 1} message to the app; the app
// must check message types and values against fixed enums and never render
// message data. The frame can navigate itself (CSP has no navigate-to).
//
// The page is self-contained (inline CSS and scripts, no AuraGo URL) and takes
// the identifier, the muted flag and the sound label from values the server
// rendered, never from its own address.

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"aurago/internal/retrogames"
)

// retroGamesPlayerCSPFormat is the player page's Content-Security-Policy; %s
// takes the hash sources of the inline scripts. archive.org's loader needs:
// its scripts under /services/emularity/ and on the emularity-*.ux-b hosts,
// wasm compilation, game data from archive.org/cors/*.us hosts and data: URLs
// (VICE fetches its wasm from one), workers from blob: URLs, and the splash
// image.
const retroGamesPlayerCSPFormat = "sandbox allow-scripts allow-pointer-lock; " +
	"default-src 'none'; " +
	"script-src %s https://archive.org/services/emularity/ https://emularity-engine.ux-b.archive.org https://emularity-config.ux-b.archive.org https://emularity-bios.ux-b.archive.org 'wasm-unsafe-eval'; " +
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

// retroGamesPlayerCSP is computed once: the inline scripts are constants, so
// their hashes never change.
var retroGamesPlayerCSP = fmt.Sprintf(retroGamesPlayerCSPFormat, retroGamesScriptHashSources(retroGamesPlayerBootstrap, retroGamesPlayerStart))

const (
	retroGamesPlayerDefaultSound  = "Click for sound"
	retroGamesPlayerSoundMaxRunes = 60
)

// retroGamesPlayerBootstrap runs before archive.org's scripts. It owns the
// hooks those scripts must find in place:
//   - every AudioContext is tracked (statechange reports to onAudioState), so
//     input can resume one the autoplay policy suspended;
//   - mute works at the Web Audio level: whatever a tracked context connects
//     to its destination goes through one master GainNode per context
//     (archive.org's own mute falls back to eval, which the CSP blocks);
//   - a script that fails to load, or a blocked script or worker, marks the
//     start as failed (fail() runs onFail once the start script set it).
const retroGamesPlayerBootstrap = `
const rgPlayer = (() => {
  const player = {
    contexts: [],
    masters: new Map(),
    muted: document.querySelector('meta[name=rg-muted]').content === '1',
    failed: false,
    onFail: null,
    onAudioState: null,
  };

  player.fail = () => {
    player.failed = true;
    if (player.onFail) player.onFail();
  };
  window.addEventListener('error', (event) => {
    if (event.target instanceof HTMLScriptElement) player.fail();
  }, true);
  document.addEventListener('securitypolicyviolation', (event) => {
    if (event.effectiveDirective === 'script-src-elem' || event.effectiveDirective === 'worker-src') player.fail();
  });

  for (const name of ['AudioContext', 'webkitAudioContext']) {
    const Native = window[name];
    if (typeof Native !== 'function') continue;
    const Tracked = function (...args) {
      const context = new Native(...args);
      player.contexts.push(context);
      context.addEventListener('statechange', () => {
        if (player.onAudioState) player.onAudioState();
      });
      return context;
    };
    Tracked.prototype = Native.prototype;
    window[name] = Tracked;
  }

  player.setMuted = (muted) => {
    player.muted = muted;
    for (const gain of player.masters.values()) gain.gain.value = muted ? 0 : 1;
  };

  if (window.AudioNode && window.AudioDestinationNode) {
    const connect = AudioNode.prototype.connect;
    const disconnect = AudioNode.prototype.disconnect;
    const routed = (node) => node instanceof AudioDestinationNode && player.contexts.includes(node.context);
    const master = (context) => {
      let gain = player.masters.get(context);
      if (!gain) {
        gain = context.createGain();
        gain.gain.value = player.muted ? 0 : 1;
        connect.call(gain, context.destination);
        player.masters.set(context, gain);
      }
      return gain;
    };
    AudioNode.prototype.connect = function (destination, ...rest) {
      if (!routed(destination)) return connect.apply(this, arguments);
      connect.call(this, master(destination.context), ...rest);
      return destination;
    };
    AudioNode.prototype.disconnect = function (destination, ...rest) {
      if (!routed(destination)) return disconnect.apply(this, arguments);
      return disconnect.call(this, master(destination.context), ...rest);
    };
  }
  return player;
})();
`

// retroGamesPlayerStart runs after loader.js: state reports, failure
// detection, the sound hint, canvas fitting, app messages and the start.
//
// The canvas element itself is sized to the largest box with the canvas's
// own aspect that fits the stage (no object-fit): Emscripten maps mouse
// positions through getBoundingClientRect against canvas.width/height, so a
// letterboxed element box would skew point-and-click games. Emscripten may
// rewrite or remove the canvas's inline size, so style changes re-apply it;
// sizes are whole pixels and only written when they differ, which ends the
// observer loop.
const retroGamesPlayerStart = `
(() => {
  const id = document.querySelector('meta[name=rg-id]').content;
  const stage = document.getElementById('stage');
  const canvas = document.getElementById('canvas');
  const pill = document.getElementById('rg-sound');
  const failedTitle = /^(Failed|Invalid media|Non-system disk)/;
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
    if (state !== 'running' || rgPlayer.muted) return;
    setAudioBlocked(rgPlayer.contexts.some((context) => context.state === 'suspended'));
  }

  function resumeAudio() {
    const resumes = rgPlayer.contexts
      .filter((context) => context.state === 'suspended')
      .map((context) => context.resume());
    if (resumes.length) Promise.allSettled(resumes).then(checkAudio);
  }

  function setMuted(muted) {
    rgPlayer.setMuted(muted);
    if (muted) setAudioBlocked(false);
    else checkAudio();
  }

  const watcher = new MutationObserver(() => {
    const title = document.querySelector('.emularity-splash-title');
    if (title && failedTitle.test(title.textContent.trim())) fail();
  });

  function fail() {
    watcher.disconnect();
    setState('failed');
  }

  function setSize(name, pixels) {
    const value = pixels + 'px';
    if (canvas.style.getPropertyValue(name) !== value || canvas.style.getPropertyPriority(name) !== 'important') {
      canvas.style.setProperty(name, value, 'important');
    }
  }

  function fitCanvas() {
    const width = canvas.width;
    const height = canvas.height;
    if (!width || !height || !stage.clientWidth || !stage.clientHeight) return;
    const scale = Math.min(stage.clientWidth / width, stage.clientHeight / height);
    setSize('width', Math.max(1, Math.floor(width * scale)));
    setSize('height', Math.max(1, Math.floor(height * scale)));
  }

  function start() {
    setState('loading');
    if (state === 'failed') return;
    try {
      const emu = new IALoader(canvas, id, { before_run: () => setState('running') });
      emu.start({ hasCustomCSS: true });
    } catch (error) {
      fail();
    }
    canvas.focus();
  }

  watcher.observe(document.documentElement, { childList: true, subtree: true, characterData: true });
  rgPlayer.onFail = fail;
  rgPlayer.onAudioState = checkAudio;
  if (rgPlayer.failed) fail();

  new ResizeObserver(fitCanvas).observe(stage);
  new MutationObserver(fitCanvas).observe(canvas, { attributes: true, attributeFilter: ['width', 'height', 'style'] });
  fitCanvas();

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
`

// retroGamesPlayerPage is the player document. html/template escapes the
// identifier and the sound label for their contexts. The inline scripts are
// the constants above, inserted verbatim (template.JS) so they match their
// hashes; they read the values from the meta tags and the pill.
//
// <base> comes first: the loader uses protocol-relative URLs, which on an
// http:// AuraGo would otherwise hit archive.org over http:// (301 without
// CORS headers). BrowserFS must load before loader.js. No lang attribute:
// the page has no text of its own besides the label.
const retroGamesPlayerPage = `<!doctype html>
<html><head><base href="https://archive.org/">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="rg-id" content="{{.ID}}">
<meta name="rg-muted" content="{{if .Muted}}1{{else}}0{{end}}">
<title>Retro Games</title>
<style>
html,body{margin:0;width:100%;height:100%;overflow:hidden;background:#000;color:#ddd;font:14px/1.4 system-ui,sans-serif}
#stage{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;overflow:hidden}
#canvas{display:block;flex:none;image-rendering:pixelated;outline:none;background:#000}
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
<script>{{.Bootstrap}}</script>
<script src="https://archive.org/services/emularity/engine/browserfs.min.js"></script>
<script src="https://archive.org/services/emularity/config/loader.js"></script>
<script>{{.Start}}</script>
</body></html>
`

var retroGamesPlayerTemplate = template.Must(template.New("retro-games-player").Parse(retroGamesPlayerPage))

type retroGamesPlayerData struct {
	ID        string
	Muted     bool
	Sound     string
	Bootstrap template.JS
	Start     template.JS
}

// retroGamesScriptHashSources returns the CSP hash sources ('sha256-…') of
// inline script bodies.
func retroGamesScriptHashSources(scripts ...string) string {
	sources := make([]string, 0, len(scripts))
	for _, script := range scripts {
		sum := sha256.Sum256([]byte(script))
		sources = append(sources, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
	}
	return strings.Join(sources, " ")
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

// ServeRetroGamesPlayerUngated writes the player page for the request's id,
// muted and sound query parameters. It neither authenticates nor checks that
// Retro Games is on: handleRetroGames does both before it calls this. It is
// exported so the desktop browser tests (package ui) can serve the real page.
func ServeRetroGamesPlayerUngated(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	id := query.Get("id")
	if !retrogames.ValidIdentifier(id) {
		retroGamesError(w, http.StatusBadRequest, "invalid_identifier", "Invalid game identifier")
		return
	}
	var page bytes.Buffer
	if err := retroGamesPlayerTemplate.Execute(&page, retroGamesPlayerData{
		ID:        id,
		Muted:     query.Get("muted") == "1",
		Sound:     retroGamesPlayerSoundLabel(query.Get("sound")),
		Bootstrap: template.JS(retroGamesPlayerBootstrap),
		Start:     template.JS(retroGamesPlayerStart),
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
	header.Set("Content-Security-Policy", retroGamesPlayerCSP)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(page.Bytes())
}

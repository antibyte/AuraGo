package gamemaker

import "strings"

// CraftRecipeTopics returns the ordered list of optional craft recipe topics
// available through game_maker_project(operation="reference", topic=...).
func CraftRecipeTopics() []string {
	return []string{"juice", "particles", "procedural-audio", "procgen", "pathfinding", "save", "tweening", "ui-hud"}
}

// CraftRecipe returns concise, framework-free JS/TS patterns for a topic.
func CraftRecipe(topic string) (map[string]any, bool) {
	key := strings.ToLower(strings.TrimSpace(topic))
	recipes := map[string]map[string]any{
		"juice": {
			"title":   "Game Feel & Juice (Screen Shake, Hitstop, Squash & Coyote Time)",
			"summary": "Small responsive feedbacks that make movement, impacts and scoring feel tactile.",
			"notes": []string{
				"Apply screen shake via a trauma scalar (0..1) squared for offset, decaying each frame.",
				"Freeze simulation for 35-60ms (hitstop) on heavy hits while still rendering particles and shake.",
				"Add 100ms coyote time (grace period after leaving a ledge) and 120ms jump buffering (remember jump pressed just before landing).",
			},
			"snippet": `// Screen shake + hitstop + floating text + coyote/jump buffer
const juice = { shake: 0, hitstop: 0, texts: [] };
function addShake(amount) { juice.shake = Math.min(1, juice.shake + amount); }
function triggerHitstop(sec = 0.045) { juice.hitstop = Math.max(juice.hitstop, sec); }
function popText(x, y, text, color = '#facc15') {
  juice.texts.push({ x, y, text, color, life: 0.75, max: 0.75 });
}
function applyCameraShake(ctx, dt) {
  juice.shake = Math.max(0, juice.shake - dt * 2.8);
  const mag = juice.shake * juice.shake * 14;
  if (mag > 0.2) ctx.translate((Math.random() - 0.5) * mag * 2, (Math.random() - 0.5) * mag * 2);
}
// Platformer coyote + jump buffer:
let coyote = 0, jumpBuf = 0;
function updateJumpWindow(dt, onGround, jumpJustPressed, performJump) {
  coyote = onGround ? 0.11 : Math.max(0, coyote - dt);
  jumpBuf = jumpJustPressed ? 0.12 : Math.max(0, jumpBuf - dt);
  if (coyote > 0 && jumpBuf > 0) { coyote = 0; jumpBuf = 0; performJump(); }
}`,
		},
		"particles": {
			"title":   "Lightweight Pooled Particle & Shockwave System",
			"summary": "Zero-allocation burst particles, trails and expanding impact rings for Canvas 2D or custom overlays.",
			"notes": []string{
				"Reuse particle entries or cap active count (~300) so bursts never stutter.",
				"Combine fast bright sparks with slower fading smoke/dust and a crisp ring shockwave.",
			},
			"snippet": `const particles = [];
function emitBurst(x, y, { count = 14, color = '#38bdf8', speed = 220, life = 0.45, gravity = 0, size = 5 } = {}) {
  for (let i = 0; i < count && particles.length < 320; i++) {
    const a = Math.random() * Math.PI * 2;
    const v = speed * (0.35 + Math.random() * 0.85);
    particles.push({ kind: 'dot', x, y, vx: Math.cos(a) * v, vy: Math.sin(a) * v, life, max: life, color, gravity, size });
  }
}
function emitRing(x, y, color = '#facc15', radius = 44, life = 0.3) {
  particles.push({ kind: 'ring', x, y, radius, life, max: life, color });
}
function stepAndDrawParticles(ctx, dt) {
  for (let i = particles.length - 1; i >= 0; i--) {
    const p = particles[i];
    p.life -= dt;
    if (p.life <= 0) { particles.splice(i, 1); continue; }
    const t = 1 - p.life / p.max;
    ctx.save();
    ctx.globalAlpha = Math.max(0, 1 - t);
    if (p.kind === 'ring') {
      ctx.strokeStyle = p.color; ctx.lineWidth = 3 * (1 - t);
      ctx.beginPath(); ctx.arc(p.x, p.y, 6 + p.radius * t, 0, Math.PI * 2); ctx.stroke();
    } else {
      p.vy += p.gravity * dt; p.x += p.vx * dt; p.y += p.vy * dt;
      const s = p.size * (1 - t * 0.5);
      ctx.fillStyle = p.color; ctx.fillRect(p.x - s / 2, p.y - s / 2, s, s);
    }
    ctx.restore();
  }
}`,
		},
		"procedural-audio": {
			"title":   "Procedural ZzFX SFX & Gesture-Safe Web Audio Arpeggiator",
			"summary": "Instant zero-asset sound effects via bundled vendor/zzfx.js plus optional adaptive background music.",
			"notes": []string{
				"Every project can import vendor/zzfx.js or use game_maker_asset generate kind='sfx' for WAV files.",
				"Always unlock/start audio after a user gesture (click, tap or keydown) and offer a mute toggle (M).",
			},
			"snippet": `import { playSfx, zzfx, getAudioContext } from '../vendor/zzfx.js';
// Play built-in presets: jump, pickup, coin, laser, shoot, explosion, hit, damage, powerup, win, lose, click, step, bounce, alarm
// playSfx('coin');
// Or pass custom ZzFX parameters: zzfx(0.5, 0.05, 440, 0.01, 0.05, 0.15, 1);

let muted = false, musicTimer = 0, stepIdx = 0;
const scale = [261.63, 311.13, 349.23, 392.00, 466.16, 523.25];
export function sfx(name) { if (!muted) playSfx(name); }
export function toggleMute() { muted = !muted; return muted; }
export function stepAmbientBeat(dt, intensity = 1) {
  if (muted) return;
  musicTimer -= dt * intensity;
  if (musicTimer > 0) return;
  musicTimer = 0.24;
  const ctx = getAudioContext();
  if (!ctx || ctx.state !== 'running') return;
  const osc = ctx.createOscillator(), gain = ctx.createGain();
  osc.type = 'triangle';
  osc.frequency.value = scale[(stepIdx++ * 2) % scale.length] * (stepIdx % 8 === 0 ? 0.5 : 1);
  gain.gain.setValueAtTime(0.035, ctx.currentTime);
  gain.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + 0.18);
  osc.connect(gain).connect(ctx.destination);
  osc.start(); osc.stop(ctx.currentTime + 0.19);
}`,
		},
		"procgen": {
			"title":   "Seeded PRNG, Dungeon/Arena Layout & Wave Pacing",
			"summary": "Deterministic random generation, walkable room carving and escalating encounter directors.",
			"notes": []string{
				"Use a seeded PRNG so daily challenges or retries feel fair and reproducible.",
				"Validate reachability (flood fill from spawn) after carving walls or obstacles.",
			},
			"snippet": `function mulberry32(seed) {
  let a = seed >>> 0;
  return () => {
    a |= 0; a = (a + 0x6D2B79F5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
function generateArena(cols, rows, seed = 1337) {
  const rnd = mulberry32(seed);
  const grid = Array.from({ length: rows }, (_, y) =>
    Array.from({ length: cols }, (_, x) => (x === 0 || y === 0 || x === cols - 1 || y === rows - 1 ? 1 : 0))
  );
  for (let i = 0; i < Math.floor(cols * rows * 0.12); i++) {
    const x = 2 + Math.floor(rnd() * (cols - 4));
    const y = 2 + Math.floor(rnd() * (rows - 4));
    if (Math.hypot(x - cols / 2, y - rows / 2) > 2.5) grid[y][x] = 1;
  }
  return grid;
}`,
		},
		"pathfinding": {
			"title":   "Grid BFS Distance Field & Line-of-Sight Steering",
			"summary": "Fast flow-field / BFS pathfinding so multiple enemies navigate around walls without per-enemy A* overhead.",
			"notes": []string{
				"Compute one BFS distance map from the player cell whenever the player changes tile; every enemy simply steps toward the neighbor with the lowest distance.",
				"Check Bresenham/DDA grid line-of-sight before switching enemies from patrol to aggro.",
			},
			"snippet": `function computeDistanceField(grid, targetX, targetY) {
  const h = grid.length, w = grid[0].length;
  const dist = Array.from({ length: h }, () => new Int16Array(w).fill(-1));
  if (targetY < 0 || targetY >= h || targetX < 0 || targetX >= w || grid[targetY][targetX] === 1) return dist;
  const q = [[targetX, targetY]];
  dist[targetY][targetX] = 0;
  for (let head = 0; head < q.length; head++) {
    const [x, y] = q[head], d = dist[y][x] + 1;
    for (const [dx, dy] of [[1,0],[-1,0],[0,1],[0,-1]]) {
      const nx = x + dx, ny = y + dy;
      if (nx >= 0 && nx < w && ny >= 0 && ny < h && grid[ny][nx] === 0 && dist[ny][nx] === -1) {
        dist[ny][nx] = d; q.push([nx, ny]);
      }
    }
  }
  return dist;
}
function bestStepTowardPlayer(dist, x, y) {
  let best = [0, 0], minD = dist[y]?.[x] ?? 9999;
  for (const [dx, dy] of [[1,0],[-1,0],[0,1],[0,-1]]) {
    const d = dist[y + dy]?.[x + dx];
    if (d !== undefined && d >= 0 && (minD < 0 || d < minD)) { minD = d; best = [dx, dy]; }
  }
  return best;
}`,
		},
		"save": {
			"title":   "Safe LocalStorage Highscore & Progress Persistence",
			"summary": "Sandbox-safe persistence helper that never crashes in opaque-origin preview iframes.",
			"notes": []string{
				"Preview iframes or strict privacy modes can throw SecurityError on localStorage access; always wrap in try/catch with an in-memory fallback.",
				"Include a schema version number so old saves never break updated games.",
			},
			"snippet": `const memoryFallback = new Map();
export function loadSave(key, defaults) {
  try {
    const raw = window.localStorage.getItem(key);
    if (raw) return { ...defaults, ...JSON.parse(raw) };
  } catch {}
  return { ...defaults, ...(memoryFallback.get(key) || {}) };
}
export function writeSave(key, data) {
  const payload = { ...data, updatedAt: Date.now() };
  memoryFallback.set(key, payload);
  try { window.localStorage.setItem(key, JSON.stringify(payload)); } catch {}
  return payload;
}`,
		},
		"tweening": {
			"title":   "Micro Tween Engine & Frame-Rate-Independent Damping",
			"summary": "Smooth UI pops, elastic bounces, camera follow and value interpolation without external libraries.",
			"notes": []string{
				"Use easeOutBack for juicy score/card pop-ins and exponential damp() for frame-rate-independent camera smoothing.",
			},
			"snippet": `export const easeOutCubic = (t) => 1 - Math.pow(1 - t, 3);
export const easeOutBack = (t) => { const c1 = 1.70158, c3 = c1 + 1; return 1 + c3 * Math.pow(t - 1, 3) + c1 * Math.pow(t - 1, 2); };
export const damp = (current, target, lambda, dt) => current + (target - current) * (1 - Math.exp(-lambda * dt));

const tweens = [];
export function tween(target, props, duration = 0.25, ease = easeOutCubic, onDone) {
  const from = {};
  for (const k of Object.keys(props)) from[k] = target[k] ?? 0;
  tweens.push({ target, from, to: props, duration: Math.max(0.001, duration), elapsed: 0, ease, onDone });
}
export function stepTweens(dt) {
  for (let i = tweens.length - 1; i >= 0; i--) {
    const tw = tweens[i];
    tw.elapsed += dt;
    const t = Math.min(1, tw.elapsed / tw.duration), e = tw.ease(t);
    for (const k of Object.keys(tw.to)) tw.target[k] = tw.from[k] + (tw.to[k] - tw.from[k]) * e;
    if (t >= 1) { tweens.splice(i, 1); tw.onDone?.(); }
  }
}`,
		},
		"ui-hud": {
			"title":   "Responsive HUD, Title, Pause & Result Overlay Pattern",
			"summary": "Accessible HTML/CSS overlay that works across desktop (1024x768) and portrait mobile (430x932) viewports.",
			"notes": []string{
				"Keep the HUD compact in the top corners with pointer-events: none so it never blocks gameplay clicks.",
				"Make every overlay action clickable/tappable AND bound to keyboard shortcuts (Space/Enter to start or retry, P to pause).",
			},
			"snippet": `// Recommended CSS/DOM pattern for #hud and #overlay:
// <div id="hud" style="position:fixed;top:12px;left:12px;pointer-events:none;z-index:5"></div>
// <div id="overlay" style="position:fixed;inset:0;display:grid;place-items:center;background:#050811b8;z-index:10">...</div>
function bindGameOverlay(overlayEl, { onStart, onResume, onRestart }) {
  const titleEl = overlayEl.querySelector('[data-title]');
  const bodyEl = overlayEl.querySelector('[data-body]');
  const btnEl = overlayEl.querySelector('[data-action]');
  let mode = 'title';
  const show = (nextMode, title, body, label) => {
    mode = nextMode; titleEl.textContent = title; bodyEl.textContent = body; btnEl.textContent = label;
    overlayEl.hidden = false;
  };
  const hide = () => { overlayEl.hidden = true; };
  btnEl.addEventListener('click', () => {
    if (mode === 'paused') onResume();
    else if (mode === 'ended') onRestart();
    else onStart();
  });
  return { show, hide };
}`,
		},
	}
	recipe, ok := recipes[key]
	return recipe, ok
}

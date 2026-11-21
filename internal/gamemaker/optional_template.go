package gamemaker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// StudioTemplateNames returns every starter template name available to
// game_maker_project use_template, keeping the framework-free canvas starter
// first while preserving templateNames() ordering for legacy callers.
func StudioTemplateNames() []string {
	return append([]string{"canvas"}, templateNames()...)
}

// StudioTemplateCatalog returns concise descriptions of all optional starter templates.
func StudioTemplateCatalog() []map[string]any {
	return []map[string]any{
		{"name": "canvas", "dimension": "2d", "engine": "html", "description": "Framework-free HTML5 Canvas 2D starter (JS module, ZzFX SFX, particles, screen shake, touch/pointer & keyboard, title/pause/result flow)"},
		{"name": "minimal", "dimension": "2d", "engine": "phaser", "description": "Minimal Phaser 4 GameScene starter with movement, pickups and HUD"},
		{"name": "shooter", "dimension": "2d", "engine": "phaser", "description": "2D arcade shooter starter with projectiles, enemies and score"},
		{"name": "platformer", "dimension": "2d", "engine": "phaser", "description": "2D side-scrolling platformer starter with gravity, platforms and hazards"},
		{"name": "topdown", "dimension": "2d", "engine": "phaser", "description": "2D top-down adventure/arena starter with 4-way movement"},
		{"name": "blocks", "dimension": "2d", "engine": "phaser", "description": "2D brick/paddle breakout starter with bounce physics"},
		{"name": "board", "dimension": "2d", "engine": "phaser", "description": "2D grid/tile board game starter with pointer selection"},
		{"name": "three", "dimension": "3d", "engine": "three", "description": "Clean procedural Three.js r186 TypeScript starter with camera, lights and loop"},
		{"name": "fps", "dimension": "3d", "engine": "three", "description": "3D first-person shooter starter using startGame and low-poly models"},
		{"name": "exploration", "dimension": "3d", "engine": "three", "description": "3D third-person exploration starter with collectibles and patrolling enemies"},
		{"name": "transport", "dimension": "3d", "engine": "three", "description": "3D vehicle cargo pickup and delivery starter"},
		{"name": "flight", "dimension": "3d", "engine": "three", "description": "3D flight rings and aerial navigation starter"},
		{"name": "space", "dimension": "3d", "engine": "three", "description": "3D space combat and asteroid arena starter"},
		{"name": "voxel", "dimension": "3d", "engine": "three", "description": "First-person Voxel block-world starter with crafting, mining and saves"},
	}
}

// UseTemplate replaces entry files only on an explicit agent request.
func (s *Service) UseTemplate(ctx context.Context, jobID, name string) error {
	if !slices.Contains(StudioTemplateNames(), name) {
		return fmt.Errorf("unknown template %q", name)
	}
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	if err := s.CheckJobMutation(ctx, jobID); err != nil {
		return err
	}
	project, _, err := s.ProjectForJob(ctx, jobID)
	if err != nil {
		return err
	}
	stage, err := s.JobDirectory(jobID)
	if err != nil {
		return err
	}
	candidate, err := os.MkdirTemp(s.stagingDir, ".template-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(candidate)
	if err := copyTree(stage, candidate, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes); err != nil {
		return err
	}
	if name == "canvas" {
		if err := installCanvasTemplate(candidate, project); err != nil {
			return err
		}
		if err := validateTreeLimits(candidate, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := swapTemplateStage(stage, candidate); err != nil {
			return err
		}
		_, _ = s.emit(ctx, project.ID, jobID, "file_changed", map[string]any{"path": "index.html", "template": name})
		return nil
	}

	dimension, variant, engine, version := "2d", "", "phaser", PhaserVersion
	if name == "three" || name == "voxel" || guided3D(name) {
		dimension, engine, version = "3d", "three", ThreeVersion
	}
	if name == "voxel" {
		variant = "voxel"
	}
	project.Dimension, project.Variant = dimension, variant
	plan := ExampleGamePlan(project)
	plan.Template, plan.Assets = name, nil
	if desc := strings.TrimSpace(project.Description); desc != "" {
		plan.Objective = desc
	} else if title := strings.TrimSpace(project.Name); title != "" {
		plan.Objective = title
	}
	if guided3D(name) {
		if plan.Gameplay == nil {
			plan.Gameplay = &GameSettings{Goal: 5, Speed: 5, Duration: 120}
		}
		if assets, ok := stageGuidedTemplateModels(candidate, name, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes, s.opts.MaxAssetBytes, s.opts.MaxFileBytes); ok {
			plan.Assets = assets
		}
	}
	if err := installRuntime(candidate, dimension, variant); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(candidate, "src"), 0o750); err != nil {
		return err
	}
	if name == "three" {
		if err := os.WriteFile(filepath.Join(candidate, "src", "main.ts"), []byte(threeScaffold), 0o640); err != nil {
			return err
		}
	} else if err := installGameTemplate(candidate, plan); err != nil {
		return err
	}
	manifest, _ := json.MarshalIndent(gameManifest{Name: project.Name, Dimension: dimension, Variant: variant, Engine: engine, Version: version, Entry: "index.html", Description: project.Description}, "", "  ")
	for path, data := range map[string][]byte{"game.json": manifest, "index.html": []byte(scaffoldHTML(dimension, project.Name))} {
		if err := os.WriteFile(filepath.Join(candidate, path), data, 0o640); err != nil {
			return err
		}
	}
	if err := validateTreeLimits(candidate, s.opts.MaxFilesPerProject, s.opts.MaxProjectBytes); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := swapTemplateStage(stage, candidate); err != nil {
		return err
	}
	_, _ = s.emit(ctx, project.ID, jobID, "file_changed", map[string]any{"path": "index.html", "template": name})
	return nil
}

func swapTemplateStage(stage, candidate string) error {
	backup := candidate + "-previous"
	if err := os.Rename(stage, backup); err != nil {
		return err
	}
	if err := os.Rename(candidate, stage); err != nil {
		if rollback := os.Rename(backup, stage); rollback != nil {
			return fmt.Errorf("template commit failed: %w; restore failed: %v", err, rollback)
		}
		return err
	}
	_ = os.RemoveAll(backup)
	return nil
}

func stageGuidedTemplateModels(candidate, template string, maxFiles int, maxProjectBytes, maxAssetBytes, maxFileBytes int64) ([]PlanAsset, bool) {
	defaults := defaultModelRoles(template)
	ids := make([]string, 0, len(defaults))
	for _, d := range defaults {
		if d.AssetID != "" {
			ids = append(ids, d.AssetID)
		}
	}
	selected, manifest, err := validateModelSelection(ids, ModelPackID)
	if err != nil {
		return nil, false
	}
	rel := "assets/builtin/" + manifest.ID + "/" + manifest.Version
	wanted := map[string][]byte{}
	license, err := bundledModelFile("LICENSE.txt", manifest.ID)
	if err != nil {
		return nil, false
	}
	wanted["LICENSE.txt"] = license
	var addedBytes int64
	addedBytes += int64(len(license))
	for _, asset := range selected {
		for _, file := range modelFiles(asset) {
			if _, exists := wanted[file.File]; exists {
				continue
			}
			data, err := bundledModelFile(file.File, manifest.ID)
			if err != nil || int64(len(data)) != file.Bytes || sha256Bytes(data) != file.SHA256 || int64(len(data)) > maxAssetBytes {
				return nil, false
			}
			wanted[file.File] = data
			addedBytes += int64(len(data))
		}
		copyManifest := manifest
		copyManifest.Categories = nil
		copyManifest.Assets = []ModelAsset{asset}
		data, err := json.MarshalIndent(copyManifest, "", "  ")
		if err != nil || int64(len(data)+1) > maxFileBytes {
			return nil, false
		}
		data = append(data, '\n')
		wanted["assets/"+asset.ID+".json"] = data
		addedBytes += int64(len(data))
	}
	if maxProjectBytes > 0 && addedBytes > maxProjectBytes/2 {
		return nil, false
	}
	if maxFiles > 0 && len(wanted)+20 > maxFiles {
		return nil, false
	}
	for name, data := range wanted {
		target := filepath.Join(candidate, filepath.FromSlash(rel+"/"+name))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return nil, false
		}
		if err := os.WriteFile(target, data, 0o640); err != nil {
			return nil, false
		}
	}
	out := make([]PlanAsset, 0, len(defaults))
	for _, d := range defaults {
		out = append(out, PlanAsset{
			Role:      d.Role,
			PackID:    ModelPackID,
			Version:   manifest.Version,
			AssetID:   d.AssetID,
			Scale:     1,
			Direction: "+Z",
			Collider:  "catalog",
		})
	}
	return out, true
}

func installCanvasTemplate(candidate string, project Project) error {
	for _, obsolete := range []string{"src/main.ts", "src/common.ts", "dist/game.js", "dist/game.js.map"} {
		_ = os.Remove(filepath.Join(candidate, filepath.FromSlash(obsolete)))
	}
	if err := installRuntimeAssets(candidate, []runtimeAsset{
		{embeddedPath: "runtime/THIRD_PARTY_NOTICES.md", projectPath: "THIRD_PARTY_NOTICES.md"},
		{embeddedPath: "runtime/zzfx.js", projectPath: "vendor/zzfx.js"},
	}); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(candidate, "src"), 0o750); err != nil {
		return err
	}
	title := strings.TrimSpace(project.Name)
	if title == "" {
		title = "Canvas Arcade"
	}
	objective := strings.TrimSpace(project.Description)
	if objective == "" {
		objective = "Collect orbs, dodge hazards and build your combo."
	}
	manifest, _ := json.MarshalIndent(gameManifest{
		Name:        title,
		Dimension:   "2d",
		Engine:      "html",
		Entry:       "index.html",
		Description: project.Description,
	}, "", "  ")
	for rel, data := range map[string][]byte{
		"game.json":   append(manifest, '\n'),
		"index.html":  []byte(canvasScaffoldHTML(title, objective)),
		"src/game.js": []byte(canvasScaffoldJS(title, objective)),
	} {
		if err := os.WriteFile(filepath.Join(candidate, filepath.FromSlash(rel)), data, 0o640); err != nil {
			return err
		}
	}
	return nil
}

func canvasScaffoldHTML(title, objective string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no">
  <title>%s</title>
  <style>
    * { box-sizing: border-box; }
    html, body { width: 100%%; height: 100%%; margin: 0; overflow: hidden; background: #090d16; color: #f8fafc; font: 15px/1.4 system-ui, sans-serif; }
    #game-root { position: relative; width: 100%%; height: 100%%; display: grid; place-items: center; }
    canvas { width: 100%%; height: 100%%; max-width: 100vw; max-height: 100vh; object-fit: contain; display: block; touch-action: none; }
    #hud { position: fixed; top: 12px; left: 12px; padding: 8px 14px; border-radius: 10px; background: #0f172acc; border: 1px solid #38bdf844; font-weight: 600; letter-spacing: 0.02em; pointer-events: none; }
    #overlay { position: fixed; inset: 0; display: grid; place-items: center; background: #050811b8; backdrop-filter: blur(6px); }
    #overlay[hidden] { display: none; }
    .card { max-width: 420px; padding: 24px 28px; border-radius: 16px; background: #111827ee; border: 1px solid #38bdf855; text-align: center; box-shadow: 0 20px 50px #00000080; }
    .card h1 { margin: 0 0 8px; font-size: 24px; color: #7dd3fc; }
    .card p { margin: 0 0 18px; color: #cbd5e1; }
    .card button { padding: 10px 22px; border: 0; border-radius: 10px; background: #0ea5e9; color: #04131f; font-weight: 700; font-size: 15px; cursor: pointer; }
  </style>
</head>
<body>
  <div id="game-root">
    <canvas id="game-canvas" width="960" height="540"></canvas>
    <div id="hud">Score 0</div>
    <div id="overlay">
      <div class="card">
        <h1 id="overlay-title">%s</h1>
        <p id="overlay-body">%s</p>
        <button id="overlay-btn" type="button">Start (Space / Tap)</button>
      </div>
    </div>
  </div>
  <script type="module" src="src/game.js"></script>
</body>
</html>
`, htmlEscape(title), htmlEscape(title), htmlEscape(objective))
}

func canvasScaffoldJS(title, objective string) string {
	titleJSON, _ := json.Marshal(title)
	objJSON, _ := json.Marshal(objective)
	return fmt.Sprintf(`import { playSfx } from '../vendor/zzfx.js';

const W = 960, H = 540;
const canvas = document.getElementById('game-canvas');
const ctx = canvas.getContext('2d');
const hud = document.getElementById('hud');
const overlay = document.getElementById('overlay');
const overlayTitle = document.getElementById('overlay-title');
const overlayBody = document.getElementById('overlay-body');
const overlayBtn = document.getElementById('overlay-btn');

const keys = new Set();
const pointer = { active: false, x: W / 2, y: H / 2 };
let state = 'title'; // 'title' | 'playing' | 'paused' | 'lost'
let score = 0, lives = 3, elapsed = 0, shake = 0, hitstop = 0, invuln = 0;
const player = { x: W / 2, y: H / 2, r: 16, vx: 0, vy: 0 };
const orbs = [], hazards = [], particles = [];

function rand(min, max) { return min + Math.random() * (max - min); }
function emit(x, y, color, count = 12) {
  for (let i = 0; i < count; i++) {
    const a = rand(0, Math.PI * 2), s = rand(60, 240);
    particles.push({ x, y, vx: Math.cos(a) * s, vy: Math.sin(a) * s, life: 0.45, max: 0.45, color });
  }
}
function spawnOrb() { orbs.push({ x: rand(50, W - 50), y: rand(50, H - 50), r: 11, pulse: rand(0, 6) }); }
function spawnHazard() {
  const a = rand(0, Math.PI * 2), sp = rand(110, 180);
  hazards.push({ x: rand(60, W - 60), y: rand(60, H - 60), vx: Math.cos(a) * sp, vy: Math.sin(a) * sp, r: 14 });
}
function resetRun() {
  score = 0; lives = 3; elapsed = 0; shake = 0; hitstop = 0; invuln = 0;
  player.x = W / 2; player.y = H / 2;
  orbs.length = 0; hazards.length = 0; particles.length = 0;
  for (let i = 0; i < 4; i++) spawnOrb();
  for (let i = 0; i < 4; i++) spawnHazard();
  state = 'playing';
  overlay.hidden = true;
  playSfx('click');
}
function showOverlay(t, b, btn) {
  overlayTitle.textContent = t; overlayBody.textContent = b; overlayBtn.textContent = btn;
  overlay.hidden = false;
}

addEventListener('keydown', (e) => {
  const k = e.key.toLowerCase();
  keys.add(k);
  if ((k === ' ' || k === 'enter') && (state === 'title' || state === 'lost')) { e.preventDefault(); resetRun(); }
  else if (k === 'p' && (state === 'playing' || state === 'paused')) {
    state = state === 'playing' ? 'paused' : 'playing';
    if (state === 'paused') showOverlay('Paused', 'Press P to resume', 'Resume'); else overlay.hidden = true;
  } else if (k === 'r') resetRun();
});
addEventListener('keyup', (e) => keys.delete(e.key.toLowerCase()));
const toCanvas = (cx, cy) => {
  const r = canvas.getBoundingClientRect();
  return { x: (cx - r.left) * (W / Math.max(1, r.width)), y: (cy - r.top) * (H / Math.max(1, r.height)) };
};
canvas.addEventListener('pointerdown', (e) => { pointer.active = true; Object.assign(pointer, toCanvas(e.clientX, e.clientY)); if (state !== 'playing') resetRun(); });
canvas.addEventListener('pointermove', (e) => { if (pointer.active) Object.assign(pointer, toCanvas(e.clientX, e.clientY)); });
addEventListener('pointerup', () => { pointer.active = false; });
overlayBtn.addEventListener('click', () => { if (state === 'paused') { state = 'playing'; overlay.hidden = true; } else resetRun(); });

window.__gameDebug = () => ({ state, score, lives, elapsed: Number(elapsed.toFixed(2)), player: { x: Math.round(player.x), y: Math.round(player.y) }, orbs: orbs.length, hazards: hazards.length });

function update(dt) {
  if (state !== 'playing') return;
  if (hitstop > 0) { hitstop -= dt; return; }
  elapsed += dt; invuln = Math.max(0, invuln - dt); shake = Math.max(0, shake - dt * 28);
  let dx = (keys.has('d') || keys.has('arrowright') ? 1 : 0) - (keys.has('a') || keys.has('arrowleft') ? 1 : 0);
  let dy = (keys.has('s') || keys.has('arrowdown') ? 1 : 0) - (keys.has('w') || keys.has('arrowup') ? 1 : 0);
  if (dx === 0 && dy === 0 && pointer.active) {
    const px = pointer.x - player.x, py = pointer.y - player.y, d = Math.hypot(px, py);
    if (d > 8) { dx = px / d; dy = py / d; }
  } else if (dx || dy) { const l = Math.hypot(dx, dy); dx /= l; dy /= l; }
  player.x = Math.max(player.r, Math.min(W - player.r, player.x + dx * 320 * dt));
  player.y = Math.max(player.r, Math.min(H - player.r, player.y + dy * 320 * dt));

  for (let i = orbs.length - 1; i >= 0; i--) {
    orbs[i].pulse += dt * 4;
    if (Math.hypot(orbs[i].x - player.x, orbs[i].y - player.y) < player.r + orbs[i].r) {
      emit(orbs[i].x, orbs[i].y, '#facc15', 14);
      orbs.splice(i, 1); spawnOrb();
      score += 10; shake = Math.max(shake, 3);
      if (score %% 50 === 0 && hazards.length < 12) spawnHazard();
      playSfx('coin');
    }
  }
  for (const h of hazards) {
    h.x += h.vx * dt; h.y += h.vy * dt;
    if (h.x < h.r || h.x > W - h.r) h.vx *= -1;
    if (h.y < h.r || h.y > H - h.r) h.vy *= -1;
    if (invuln <= 0 && Math.hypot(h.x - player.x, h.y - player.y) < player.r + h.r) {
      lives--; invuln = 1.1; shake = 10; hitstop = 0.045;
      emit(player.x, player.y, '#fb7185', 20);
      playSfx(lives <= 0 ? 'lose' : 'damage');
      if (lives <= 0) { state = 'lost'; showOverlay('Game Over', 'Final Score: ' + score, 'Play Again'); break; }
    }
  }
  for (let i = particles.length - 1; i >= 0; i--) {
    const p = particles[i]; p.x += p.vx * dt; p.y += p.vy * dt; p.life -= dt;
    if (p.life <= 0) particles.splice(i, 1);
  }
  hud.textContent = 'Score ' + score + ' · Lives ' + lives + ' · WASD / Arrows / Drag';
}

function draw() {
  ctx.save();
  if (shake > 0) ctx.translate((Math.random() - 0.5) * shake * 2, (Math.random() - 0.5) * shake * 2);
  ctx.fillStyle = '#090d16'; ctx.fillRect(0, 0, W, H);
  ctx.strokeStyle = '#1e293b'; ctx.lineWidth = 1;
  for (let x = 0; x < W; x += 60) { ctx.beginPath(); ctx.moveTo(x, 0); ctx.lineTo(x, H); ctx.stroke(); }
  for (let y = 0; y < H; y += 60) { ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(W, y); ctx.stroke(); }
  for (const o of orbs) {
    ctx.fillStyle = '#facc15'; ctx.beginPath(); ctx.arc(o.x, o.y, o.r + Math.sin(o.pulse) * 2, 0, Math.PI * 2); ctx.fill();
  }
  for (const h of hazards) {
    ctx.fillStyle = '#fb7185'; ctx.beginPath(); ctx.arc(h.x, h.y, h.r, 0, Math.PI * 2); ctx.fill();
  }
  for (const p of particles) {
    ctx.globalAlpha = Math.max(0, p.life / p.max); ctx.fillStyle = p.color; ctx.fillRect(p.x - 3, p.y - 3, 6, 6);
  }
  ctx.globalAlpha = invuln > 0 && Math.floor(invuln * 14) %% 2 ? 0.4 : 1;
  ctx.fillStyle = '#38bdf8'; ctx.beginPath(); ctx.arc(player.x, player.y, player.r, 0, Math.PI * 2); ctx.fill();
  ctx.restore();
}

let last = performance.now();
window.__AURAGO_GAME_DIAGNOSTICS__?.diagnostic?.('ready', { canvas: true });
showOverlay(%s, %s, 'Start (Space / Tap)');
function frame(now) {
  const dt = Math.min(0.05, Math.max(0, (now - last) / 1000)); last = now;
  update(dt); draw();
  requestAnimationFrame(frame);
}
requestAnimationFrame(frame);
`, string(titleJSON), string(objJSON))
}

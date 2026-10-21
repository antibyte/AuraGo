# AuraGolf P2 – Look & Welt-Requisiten: Umsetzungsplan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Die 18 bestehenden Bahnen werden in die Welten Piratenbucht, Dschungeltempel und Mondstation umgekleidet: Diorama-Inseln, instanzierte Welt-Requisiten, gebackenes AO. Der Renderer bekommt Umgebungslicht, Himmel, Wasser, Partikel, Leuchtspur, Einloch-Feier und Kamerafahrt; Physik und Kollision bleiben bitgleich.

**Architecture:**
- Der Blender-Generator wird modular (`assets/minigolf/kit/`). Kollisionsrelevanter Code zieht wortgleich nach `courses_v1.py` um.
- Neue Module bauen Kits je Welt, umkleidete Inseln, Bandenverkleidungen, Platzierungen und Vorschaubilder.
- Das Manifest bleibt Version 1 und bekommt nur additive Felder.
- Zur Laufzeit trennt sich `minigolf-scene.js` in kleine Module (Look-Daten, Kamerafahrt, Himmel, Wasser, Requisiten, Effekte) und bleibt die öffentliche API.

**Tech Stack:**
- Blender 5.2.2 (bpy, Cycles-AO-Bake, glTF-Export), Python ohne Fremdpakete.
- three.js 0.186.1 (PMREM, ShaderMaterial, InstancedMesh, UnrealBloom), esbuild-Bundle.
- Node-Testskripte (`node:assert`), gltf-validator, Go/rod-Browser-Smoke.

**Spec:** [2026-10-19-p2-look-welt-requisiten-design.md](2026-10-19-p2-look-welt-requisiten-design.md)

---

## Arbeitsumgebung und Regeln

- Worktree `C:\Users\Andi\Documents\repo\AuraGo\.claude\worktrees\friendly-napier-a5b89a`, Branch `claude/friendly-napier-a5b89a` (basiert auf local main mit P1, P4, P5). Nie im Haupt-Checkout arbeiten; dort arbeiten andere Sessions.
- `node_modules` ist eine Junction auf den Haupt-Checkout (git-ignoriert, nie stagen).
- **Rollen** (CLAUDE.md, ohne Modell-Overrides):
  - Code: maker-agent.
  - Tests ausführen: test-agent.
  - Eine kombinierte Prüfung je Paket: reader-agent (nur Critical/Important).
  - Doku: docs-agent.
  - Fehlersuche: debug-agent.
- Nach jeder Änderung committen, nur eigene Pfade stagen (`git add <pfade>`, nie `-A`), vor dem Commit `git diff --cached --stat` und `git diff --cached --check`. Commit-Trailer: `Co-Authored-By: <eigenes Modell> <noreply@anthropic.com>`.
- **Prozesse:** Nie Prozesse nach Image-Namen beenden (`taskkill /IM`, `pkill node`), nur eigene PIDs.
- **Lange Läufe:** Blender und Browser-Tests über 2 Minuten im Hintergrund starten und auf die Meldung warten.
- **Blender aufrufen:** `& "D:\Blender 5.2\blender.exe" --background --factory-startup --python <script> -- <args>`. Unter Windows bricht der GLB-Export gelegentlich an einer Dateisperre ab; dann denselben Lauf wiederholen.
- **Bundle:** Wer `ui/js/desktop/apps/minigolf-*.js` ändert, baut im selben Commit neu (`node scripts/build-minigolf.mjs`) und committet `ui/js/vendor/minigolf/{game.esm.js,manifest.json}`.
- **Nicht anfassen:**
  - Root-`AGENTS.md`: Der Haupt-Checkout führt sie als assume-unchanged, ein Fast-Forward darüber bricht ab.
  - `minigolf-physics.js`.
- Zwischenablage für Skripte und Renders: Scratchpad der Session oder `disposable/_p2/` (ignoriert).

## Dateien

| Datei | Verantwortung | Tasks |
|---|---|---|
| `scripts/fixtures/minigolf-p1-colliders.json` | P1-Kollider-Fingerabdrücke je Bahn | 2 |
| `assets/minigolf/generate.py` | CLI, Ablauf, Manifest | 3, 8 |
| `assets/minigolf/kit/courses_v1.py` | 18 alte Bahndefinitionen, Spur/Banden/Loch/Hindernisse (Code wortgleich) | 3, 6 |
| `assets/minigolf/kit/export.py` | Export, Knotennamen, Zusammenfassen, GLB-Lesen | 3, 7 |
| `assets/minigolf/kit/materials.py` | Material-Arten mit glTF-Extras, Rückseiten, Emissionsstufen | 4 |
| `assets/minigolf/kit/kits.py` | Kit je Welt bauen, backen, exportieren, Metadaten | 4 |
| `assets/minigolf/kit/props_<look>.py`, `ship_pirate.py`, `head_temple.py` (+ neue `props_<look>_*.py`) | Requisiten-Serie und `PROP_META` | 4, 5a–5c |
| `assets/minigolf/kit/dress.py`, `dress_specials.py`, `trim.py` | Insel, Bandenverkleidung, Sonderteile, Platzierung, Uferzone | 6, 7 |
| `assets/minigolf/kit/previews.py` | Vorschaubilder | 9 |
| `ui/3d/minigolf/` (`course-NN.glb`, `props-<look>.glb`, `previews/*.webp`, `manifest.json`) | Laufzeit-Assets | 4, 8, 9 |
| `scripts/test-minigolf-assets.mjs` | Budgets, Fingerabdruck, Kits, Platzierungen, Vorschaubilder | 2, 8 |
| `ui/js/desktop/apps/minigolf-looks.js` | Look-Daten, Qualitätsstufen, Platzierungsmatrix | 10 |
| `ui/js/desktop/apps/minigolf-camera.js` | Kamerafahrt (reine Mathematik) | 10 |
| `scripts/test-minigolf-look.mjs` | Node-Tests für Looks, Stufen, Matrix, Kamerafahrt | 10 |
| `ui/js/desktop/apps/minigolf-sky.js` | Himmel und PMREM-Umgebung | 11 |
| `ui/js/desktop/apps/minigolf-water.js` | Offenes Wasser und Uferzone | 12 |
| `ui/js/desktop/apps/minigolf-props.js` | Kit-Instancing und Material-Vertrag | 13 |
| `ui/js/desktop/apps/minigolf-fx.js` | Partikel, Leuchtspur, Feier, Feuerwerk | 14 |
| `ui/js/desktop/apps/minigolf-scene.js` | Einbindung, Kamerafahrt, Stufen, Statistik | 15 |
| `ui/js/desktop/apps/minigolf-game.js` | vier kleine Aufrufe | 15 |
| `ui/desktop_minigolf_browser_test.go` | Smoke: Look, Requisiten, Umgebung, Kamerafahrt | 16 |
| `ui/lang/desktop/*.json` | drei Weltnamen in 16 Sprachen | 17 |
| `documentation/minigolf.md`, `assets/minigolf/README.md`, `assets/minigolf/kit/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `.gitignore`, `package.json` | Doku, DOX, Ignore, Testskript | 3, 10, 18 |

## Pakete und Reihenfolge

| Paket | Tasks | Ausführung |
|---|---|---|
| A Vorbedingungen | 1, 2 | Hauptsession, test-agent, maker-agent |
| B Generator-Grundlage | 3, 4 | maker-agent, nacheinander |
| C Requisiten-Serie | 5a, 5b, 5c | drei maker-agents parallel (je Welt eigene Dateien) |
| D Umkleiden und Assets | 6, 7, 8, 9 | maker-agent, nacheinander |
| Prüfung 1 | Generator (Tasks 3–9) | reader-agent |
| E Laufzeit | 10–15 | maker-agent; darf nach Task 4 parallel zu C/D laufen |
| F Browser und Messung | 16 | maker-agent, test-agent |
| Prüfung 2 | Laufzeit (Tasks 10–16) | reader-agent |
| G Abschluss | 17, 18, 19 | maker-agent, docs-agent, Hauptsession |

---

### Task 1: Ausgangsmessung (vor jeder Renderer-Änderung)

**Files:** keine Quelländerung; Ergebnis in `reports/minigolf/p2-baseline/benchmark.txt` (ignoriert).

- [ ] **Step 1: Browser-Test-Vorbedingung herstellen**

Der Lauf auf local main bricht sporadisch an `desktop_minigolf_browser_test.go:375` ab („finishing the coach marks did not hand the keyboard back to the stage“).
- **Befund des debug-agents:** ein Wettlauf im Test. Der Klick beendet die Einführung sofort, der Fokus kommt aber erst im nächsten Animationsframe zurück (`minigolf-tutorial.js:79-98` über `focusStage`). Der Test prüft `document.activeElement` direkt nach dem ersten erfolgreichen `MustWait`.
- **Korrektur:** In `ui/desktop_minigolf_browser_test.go` die Zeilen 374–376 ersetzen durch

```go
	// The coach hands focus back from the game's next animation frame, not from the click, so wait for it.
	page.Timeout(5 * time.Second).MustWait(`()=>document.activeElement===document.querySelector('.mg-canvas canvas')`)
```

- **Commit:** `test(minigolf): wait for the coach marks to hand focus back`.

- [ ] **Step 2: Benchmark auf dem unveränderten Stand**

```powershell
New-Item -ItemType Directory -Force reports\minigolf\p2-baseline | Out-Null
$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_MINIGOLF_BENCHMARK='1'; $env:AURAGO_BROWSER_ARTIFACT_DIR="$PWD\reports\minigolf\p2-baseline"
go test -count=1 -timeout 30m ./ui -run 'TestDesktopMinigolfBrowser$' -v 2>&1 | Tee-Object reports\minigolf\p2-baseline\go-test.log
```

Erwartet:
- `PASS`;
- Logzeilen `minigolf 1920x1080 quality=high|medium|low synchronized={...}`.

Die drei JSON-Zeilen, den Renderer-String und `git rev-parse HEAD` in `benchmark.txt` schreiben. Maßgeblich für den späteren Vergleich sind die Bildintervalle (FPS); `triangles`/`calls` ändern in Task 15 ihre Bedeutung.

---

### Task 2: P1-Kollider-Fingerabdruck festhalten

**Files:**
- Create: `scripts/fixtures/minigolf-p1-colliders.json`
- Modify: `scripts/test-minigolf-assets.mjs`

- [ ] **Step 1: Fingerabdrücke aus dem heutigen Manifest erzeugen**

```bash
mkdir -p scripts/fixtures
node -e "
const fs=require('fs'),crypto=require('crypto');
const m=JSON.parse(fs.readFileSync('ui/3d/minigolf/manifest.json','utf8'));
const out={};for(const c of m.courses)out[c.id]=crypto.createHash('sha256').update(JSON.stringify(c.colliders)).digest('hex');
fs.writeFileSync('scripts/fixtures/minigolf-p1-colliders.json',JSON.stringify(out,null,2)+'\n');
console.log(Object.keys(out).length)"
```

Erwartet: `18`.

- [ ] **Step 2: Prüfung in den Asset-Test einbauen**

In `scripts/test-minigolf-assets.mjs` nach dem Laden des Manifests:

```js
const p1Colliders = JSON.parse(await readFile(path.join(root, 'scripts/fixtures/minigolf-p1-colliders.json'), 'utf8'));
```

In der Bahnschleife direkt nach `assert.equal(course.model, ...)`:

```js
  // P2 dresses courses without touching physics: the decoded collider data must
  // stay identical to the P1 state (P3 replaces this fixture with new courses).
  assert.equal(createHash('sha256').update(JSON.stringify(course.colliders)).digest('hex'), p1Colliders[course.id], `${course.id}: collision differs from the P1 fingerprint`);
```

- [ ] **Step 3: Test laufen lassen**

Run: `node scripts/test-minigolf-assets.mjs`
Expected: PASS, Ausgabe „Mini golf: 18 courses, …“.

- [ ] **Step 4: Commit**

```bash
git add scripts/fixtures/minigolf-p1-colliders.json scripts/test-minigolf-assets.mjs
git commit -m "test(minigolf): pin the P1 collision fingerprint before the P2 redress"
```

---

### Task 3: Generator aufteilen (reines Refactoring, bitgleiche Ausgabe)

**Files:**
- Create: `assets/minigolf/kit/courses_v1.py`, `assets/minigolf/kit/export.py`
- Modify: `assets/minigolf/generate.py`, `.gitignore`
- Delete: `assets/minigolf/aura-golf-courses.blend`

- [ ] **Step 1: Funktionen verschieben (Text wortgleich, nur Imports ergänzen)**
  - `kit/courses_v1.py` bekommt alles, was eine Bahn baut:
    - Konstanten: `SAMPLES_PER_SEGMENT`, `LANE_WIDTH`, `LANE_ROWS`, `BALL_RADIUS`, `CUP_RADIUS`, `COURSES`, `HOLE_DIGITS`.
    - Helfer: `runtime`, `blender`, `q`, `theme`, `material`, `link_collection`, `mesh_object`, `mesh_from_circle`, `ring`, `box_mesh`, `cylinder`, `ico_sphere`, `cone`, `catmull_path`, `lane_center_height`.
    - Spur und Insel: `make_surface`, `add_island`, `island_top_height`.
    - Deko: `add_tree`, `add_flower`, `add_lighthouse`, `add_arch`, `add_bridge`, `add_lantern`, `add_fence`, `add_bench`, `add_driftwood`, `add_buoy`, `add_pebble_cluster`, `add_shard`.
    - Loch und Nummer: `cup_lining`, `hole_number`.
    - Bahn: `add_obstacle`, `add_special`, `add_decor`, `add_templates`, `add_studio`, `build_course`.
  - `kit/export.py` bekommt alles rund um den Export: `scene_bounds`, `canonical_export_names`, `restore_export_names`, `glb_triangles`, `glb_fixed_colliders`.
    - Dazu eine neue Funktion, die nur den bisherigen Export-Aufruf aus `build_course` kapselt; `build_course` ruft sie auf:

```python
def export_glb(path, objects):
    """Export ``objects`` (already selected in the active scene) as one GLB."""
    names = canonical_export_names(objects)
    try:
        bpy.ops.export_scene.gltf(filepath=path, export_format="GLB", use_selection=True, use_active_scene=True,
                                  export_yup=True, export_extras=True, export_cameras=False, export_lights=False,
                                  export_materials="EXPORT", export_current_frame=True)
    finally:
        restore_export_names(names)
```

- [ ] **Step 2: `generate.py` als CLI**

```python
"""Generate AuraGo's original MIT-licensed mini golf assets with Blender 5.2.

Run from the repository root:
  blender --background --factory-startup --python assets/minigolf/generate.py -- [--only kits,courses,previews|all] [--course N] [--save-blend]
"""

import argparse
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import bpy  # noqa: E402

from kit import courses_v1  # noqa: E402

ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
RUNTIME = os.path.join(ROOT, "ui", "3d", "minigolf")
BUILD = os.path.join(HERE, "build")


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
    ap = argparse.ArgumentParser()
    ap.add_argument("--only", default="kits,courses")
    ap.add_argument("--course", type=int, default=0, help="rebuild one course (1-18) and patch its manifest entry")
    ap.add_argument("--save-blend", action="store_true")
    return ap.parse_args(argv)


def write_manifest(manifest):
    with open(os.path.join(RUNTIME, "manifest.json"), "w", encoding="utf-8", newline="\n") as f:
        json.dump(manifest, f, separators=(",", ":"))
        f.write("\n")


def main():
    args = parse_args()
    bpy.ops.object.select_all(action="SELECT")
    bpy.ops.object.delete(use_global=False)
    os.makedirs(RUNTIME, exist_ok=True)
    courses = [courses_v1.build_course(i, c) for i, c in enumerate(courses_v1.COURSES, 1)]
    write_manifest({"version": 1, "ballRadius": courses_v1.BALL_RADIUS, "courses": courses})
    if args.save_blend:
        os.makedirs(BUILD, exist_ok=True)
        bpy.context.preferences.filepaths.save_version = 0
        bpy.ops.wm.save_as_mainfile(filepath=os.path.join(BUILD, "aura-golf.blend"))
    print(f"Wrote {len(courses)} courses to {RUNTIME}")


if __name__ == "__main__":
    main()
```

Die Optionen `--only` und `--course` werden in Task 4 und 8 belegt.

- [ ] **Step 3: `.blend` aus der Versionierung nehmen**

```bash
git rm assets/minigolf/aura-golf-courses.blend
printf '\n# AuraGolf generator scratch output (P2)\n/assets/minigolf/build/\n' >> .gitignore
```

- [ ] **Step 4: Bitgleichheit beweisen**

```powershell
& "D:\Blender 5.2\blender.exe" --background --factory-startup --python assets/minigolf/generate.py
git status --short ui/3d/minigolf
node scripts/test-minigolf-assets.mjs
```

Erwartet:
- `Wrote 18 courses …`;
- `git status` zeigt keine Änderung unter `ui/3d/minigolf`;
- Asset-Test PASS.

- [ ] **Step 5: Commit**

```bash
git add assets/minigolf/generate.py assets/minigolf/kit/courses_v1.py assets/minigolf/kit/export.py .gitignore
git commit -m "refactor(minigolf): split the course generator into kit modules"
```

---

### Task 4: Material-Vertrag und Kit-Export

**Files:**
- Modify: `assets/minigolf/kit/materials.py`, `assets/minigolf/kit/props_pirate.py`, `props_temple.py`, `props_moon.py`
- Create: `assets/minigolf/kit/kits.py`
- Modify: `assets/minigolf/generate.py`
- Create (generiert): `ui/3d/minigolf/props-pirate.glb`, `props-temple.glb`, `props-moon.glb`

- [ ] **Step 1: Material-Arten mit Extras**

Änderungen in `materials.py`:
- Neue Arten `foliage` (wie `matte`, Rauheit 0,85, beidseitig) und `cloth` (wie `matte`, beidseitig).
- Jede Material-Datenblock-Erzeugung setzt:

```python
mat["aurago_kind"] = kind
mat.use_backface_culling = kind not in ("foliage", "cloth", "glass")
if kind in ("emissive", "glow"):
    mat["aurago_strength"] = strength
if flicker:
    mat["aurago_flicker"] = flicker
if kind in ("foliage", "cloth") and sway:
    mat["aurago_sway"] = sway
```

- `get(kind, strength=3.0, bump=0.0, flicker=0.0, sway=0.0)`:
  - Emissionsstärken werden auf die nächste Stufe aus `EMISSION_STEPS = (1.0, 1.6, 2.5)` gerundet.
  - Der Datenblockname enthält Stufe, Flackern und Wiegen, z. B. `kit-emissive-1.6-f0.35`, `kit-foliage-s0.06`.
- Flammen nutzen `get("emissive", 1.6, flicker=0.35)`.
- Palmwedel, Farne und Lianen nutzen `foliage` mit `sway` 0,05–0,08.
- Segel und Fahnen nutzen `cloth`.
- In den drei `props_<look>.py` die Aufrufe entsprechend umstellen. Ziel: höchstens 3 Materialien je Requisit.

- [ ] **Step 2: `PROP_META` je Welt**

Jedes `props_<look>.py` exportiert neben `PROPS` ein Wörterbuch mit Tags und Sonderangaben, z. B. Piraten:

```python
PROP_META = {
    "ship": {"tags": ["hero"]}, "palm": {"tags": ["tall"]}, "barrel": {"tags": []}, "chest": {"tags": []},
    "cannon": {"tags": []}, "dock": {"tags": ["flat"]}, "tuft": {"tags": ["scatter"]}, "shell": {"tags": ["scatter", "flat"]},
    "starfish": {"tags": ["scatter", "flat"]}, "rock": {"tags": []}, "crab": {"tags": ["scatter"]},
    "xmark": {"tags": ["scatter", "flat"]}, "footprint": {"tags": ["scatter", "flat"]},
}
```

- `foam` (Schaumring) wird aus `PROPS` entfernt; Uferschaum zeichnet die Laufzeit.
- Tempel: `head` = `hero`, `fern`/`bush`/`tile`/`pebble` = `scatter`, `tile` zusätzlich `flat`.
- Mond: `habitat` = `hero`, `pebble`/`crate` = `scatter`.
- Drehende Teile tragen `"spin": {"axis": [0, 1, 0], "speed": 0.25}` (Achse in glTF-Koordinaten, Y oben).

- [ ] **Step 3: `kits.py`**

```python
"""Per-world prop kits (MIT License).

Builds every registered prop of a world, bakes AO per prop on a temporary
ground plate (contact shading at the foot), exports ``props-<look>.glb`` with one
top-level node per prop and returns the manifest metadata.
"""

import hashlib
import importlib
import math
import os

import bpy

from . import bake, export, shapes

MODULES = {"pirate": "kit.props_pirate", "temple": "kit.props_temple", "moon": "kit.props_moon"}
SPACING = 40.0
MAX_PRIMITIVES = 3
MAX_MATERIALS = 8


def _measure(obj):
    radius = max((math.hypot(v.co.x, v.co.y) for v in obj.data.vertices), default=0.0)
    height = max((v.co.z for v in obj.data.vertices), default=0.0)
    return round(radius, 3), round(height, 3)


def _strip(obj):
    for key in [k for k in obj.keys() if not k.startswith("aurago_")]:
        del obj[key]


def build_kit(look, out_dir, samples=160):
    module = importlib.import_module(MODULES[look])
    collection = bpy.data.collections.new("kit-" + look)
    bpy.context.scene.collection.children.link(collection)
    shapes.set_collection(collection)
    objs, plates = [], []
    for index, name in enumerate(sorted(module.PROPS)):
        obj = module.PROPS[name](seed=1)
        x, y = (index % 6) * SPACING, (index // 6) * SPACING
        obj.location = (x, y, 0.0)
        plates.append(shapes.make_object("bake-plate-" + name, [(x - 8, y - 8, 0), (x + 8, y - 8, 0), (x + 8, y + 8, 0), (x - 8, y + 8, 0)], [(0, 1, 2, 3)], smooth=False))
        objs.append(obj)
    bpy.context.view_layer.update()
    bake.process(objs, samples=samples)
    for plate in plates:
        bpy.data.objects.remove(plate, do_unlink=True)
    meta, materials = {}, set()
    for obj in objs:
        obj.location = (0.0, 0.0, 0.0)
        name = obj.name[len("prop-%s-" % look):]
        slots = [slot.material for slot in obj.material_slots if slot.material]
        if len(slots) > MAX_PRIMITIVES:
            raise RuntimeError("%s uses %d materials (max %d)" % (obj.name, len(slots), MAX_PRIMITIVES))
        materials.update(m.name for m in slots)
        info = module.PROP_META.get(name, {})
        radius, height = _measure(obj)
        extras = {"radius": radius, "height": height, "tags": sorted(info.get("tags", []))}
        for key in ("spin", "clearance"):
            if key in info:
                extras[key] = info[key]
        _strip(obj)
        obj["aurago_prop"] = extras
        meta[name] = dict(extras, triangles=shapes.tri_count(obj))
    if len(materials) > MAX_MATERIALS:
        raise RuntimeError("%s kit uses %d materials (max %d)" % (look, len(materials), MAX_MATERIALS))
    bpy.ops.object.select_all(action="DESELECT")
    for obj in objs:
        obj.select_set(True)
    bpy.context.view_layer.objects.active = objs[0]
    path = os.path.join(out_dir, "props-%s.glb" % look)
    export.export_glb(path, objs)
    data = open(path, "rb").read()
    entry = {"model": os.path.basename(path), "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest(),
             "triangles": export.glb_triangles(path), "props": meta}
    print("kit %s: %d bytes, %d triangles, %d props" % (look, len(data), entry["triangles"], len(meta)))
    return entry, objs
```

- `generate.py`: Bei `kits` in `--only` werden alle drei Kits in je einer leeren Szene gebaut (`bpy.ops.wm.read_factory_settings(use_empty=True)` vor jeder Welt). Die Einträge wandern als `manifest["kits"][look]` ins Manifest.
- Nur `--only kits`: Das Manifest wird gelesen, um `kits` ergänzt und zurückgeschrieben; Bahnen bleiben unangetastet.
- `kits.build_kit` gibt die Objekte zurück, damit Task 7 sie als Verdecker wiederverwenden kann.

- [ ] **Step 4: Kits erzeugen und prüfen**

```powershell
& "D:\Blender 5.2\blender.exe" --background --factory-startup --python assets/minigolf/generate.py -- --only kits
```

Erwartet:
- drei Zeilen `kit <look>: … bytes …`;
- jedes Kit < 3 MiB und < 60 000 Dreiecke;
- keine `RuntimeError` zu Materialgrenzen.

- [ ] **Step 5: Commit**

```bash
git add assets/minigolf/kit assets/minigolf/generate.py ui/3d/minigolf/props-*.glb ui/3d/minigolf/manifest.json
git commit -m "feat(minigolf): per-world prop kits with material extras"
```

---

### Task 5a/5b/5c: Requisiten-Serie je Welt (drei maker-agents parallel)

**Gemeinsame Regeln:**
- Jeder Agent ändert nur die Dateien seiner Welt: `props_<look>.py`, eigene neue Module `props_<look>_*.py` und Hero-Module.
- Gemeinsame Dateien (`shapes.py`, `palette.py`, `materials.py`) nur mit dem Edit-Werkzeug und nur im eigenen Abschnitt; `palette.py` nur im Block der eigenen Welt.
- Kein Commit durch die Agents; die Hauptsession committet nach jedem Agenten.

**Budgets je Requisit:**
- ≤ 15 000 Dreiecke, Hero ≤ 16 000;
- ≤ 3 Materialien;
- Kit gesamt ≤ 60 000 Dreiecke und ≤ 3 MiB.

**Iteration:** Nahaufnahmen über den Proben-CLI ansehen und iterieren, bis jedes Requisit im Diorama-Stil der freigegebenen Proben sitzt:

```powershell
& "D:\Blender 5.2\blender.exe" --background --factory-startup --python assets/minigolf/kit/probe.py -- --out <scratch> --prop <look>:<name> --quick
```

**Abnahme je Agent:**
1. `generate.py -- --only kits` läuft ohne Fehler.
2. Proben-Render der eigenen Welt (`probe.py -- --world <look> --quick`) angesehen.
3. Kurzer Bericht mit Dreiecken je Requisit.

- [ ] **Task 5a – Piratenbucht**
  - **Neu:**
    - `lighthouse`: rot-weiß gestreift, Laterne `emissive` 1,6, Höhe 2,4 m, `tall`.
    - `skullrock`: Totenkopffelsen mit Höhlenaugen, 1,4 m.
    - `post`: Hafenpfosten mit Tauwicklung.
    - `buoy`: Segelboje.
    - `flagpole`: Piratenfahne, `cloth` mit Wiegen.
    - `seaweed`: Seetang am Ufer, `foliage`, `scatter`.
  - **Bessern:**
    - Palmen weniger generisch: gebogener, ringgegliederter Stamm, Wedel mit Knick, Kokosnüsse; Varianten über `seed` (zwei Registrierungen `palm` und `palm2`).
    - Sand-Streudeko ergänzen: Muschelgruppen, Fußspuren.

- [ ] **Task 5b – Dschungeltempel**
  - **Neu:**
    - `gate`: Stufentor, `span`, `clearance` 1,4, lichte Breite ≥ 2,6 m.
    - `sunwheel`: Steinrad mit Sonnengesicht, für `windmill`.
    - `idol`: goldene Figur auf Sockel, `glow`.
    - `stairs`: Treppenstück als Randdeko.
    - `vines`: Lianenvorhang, `foliage`.
  - **Überarbeiten:**
    - `head`: kein moosbedeckter Quader mehr. Gerundete Schädelform über Lathe/Subdivision, ausgeprägte Brauen, Nase, Lippen, Ohrpflöcke; Moos als Kappe mit unregelmäßigem Rand; Augen `glow`.
    - `waterfall`: fließendes Band aus gekrümmten Streifen mit Hell-Dunkel-Verlauf (Material `glow` 1,0, schwach), Becken mit Schaum, Felsen seitlich; kein Stufenblock.
  - **Palette:** Orangestich und Kalkgrün im Palettenblock der Welt dämpfen. Moos zwei Stufen dunkler und gelber, Sandstein näher an Beige.

- [ ] **Task 5c – Mondstation**
  - **Neu:**
    - `tower`: Funkturm mit blinkender Spitze, `emissive`, `flicker` 0,6.
    - `neonpost`: Neonpfosten, Ersatz für Laternen.
    - `crater`: flacher Kraterring, `flat`.
    - `dishhead` mit `spin` getrennt von `dishbase`, damit die Schüssel dreht.
    - `antenna`: kleiner Mast.
  - **Bessern:**
    - Kristalle als echte 3D-Säulengruppen (5–7 Prismen, angeschrägte Spitzen, `glow`).
    - Mehr Regolith-Varianz bei Felsbrocken und Kieseln (Flecken, Krater-Dellen über das Rauschen in `colors.paint`).

---

### Task 6: Umkleiden Teil 1 – Insel, Bandenverkleidung, Spurfarben

**Files:**
- Create: `assets/minigolf/kit/dress.py`, `assets/minigolf/kit/trim.py`
- Modify: `assets/minigolf/kit/courses_v1.py`

- [ ] **Step 1: Kantenproben teilen**

In `courses_v1.py` die Kantenrechnung aus der Bandenschleife von `make_surface` in eine Funktion ziehen. Die Bandenschleife ruft sie mit identischer Arithmetik auf:

```python
def rail_edge(course, path, i, sign):
    """Inner and outer rail edge at path sample i (Blender XY) and the lane height there."""
    n = len(path)
    p = path[i]; prev = path[max(0, i - 1)]; nxt = path[min(n - 1, i + 1)]
    dx = nxt[0] - prev[0]; dy = nxt[1] - prev[1]; ln = max(.0001, math.hypot(dx, dy)); sx = dy / ln; sy = -dx / ln
    h = lane_center_height(course, path, i); t = i / (n - 1)
    edge = sign * LANE_WIDTH / 2; outer = edge + sign * .14
    bank = course.get("bank", 0)
    if bank:
        h += bank * edge * math.sin(math.pi * max(0, min(1, (t - .18) / .66)))
    return (p[0] + sx * edge, p[1] + sy * edge), (p[0] + sx * outer, p[1] + sy * outer), h, (sx * sign, sy * sign)
```

Danach muss Task-3-Step-4 erneut bitgleich bleiben. Erst neu erzeugen und `git status ui/3d/minigolf` prüfen, dann weiter.

- [ ] **Step 2: `build_course` erhält einen Umkleider**

- `build_course(number, course, dresser=None)`: Mit `dresser` ersetzen Aufrufe von `dresser.*` die alten Deko-Aufrufe:
  - `add_decor` → `dresser.island(...)` und `dresser.place(...)`;
  - `add_special` → `dresser.specials(...)`;
  - Materialien von Spur und Banden → `dresser.lane_material(surface)`;
  - Lochboden über `dresser.top_height(x, y)` statt `island_top_height`.
- Ohne `dresser` bleibt alles wie heute; Task 3 bleibt damit prüfbar.
- Spur- und Bandennetze werden nie von Fase, Glättung oder `shade_smooth` erfasst.

- [ ] **Step 3: Insel (`dress.py`)**
  - **Umriss:** `island.Outline(center=(0, 0.1), half=(4.9, 5.35), power=2.6, wobble=0.05, seed=number)`.
  - **Gitter:** `nx=56, ny=60`.
  - **Höhenfunktion `top_height(x, y)`:**
    - Grundhöhe −0,04 plus Rauschen ±0,03.
    - Spurbett: Innerhalb 1,1 m um die Spurmitte gilt Spurhöhe − 0,06; zwischen 1,1 und 2,0 m weicher Übergang. Die Spurhöhe kommt aus `lane_center_height` des nächsten Pfadpunkts.
    - Lochmulde: Radius 0,45 m, Tiefe auf Lochhöhe − 0,34.
    - Sprunglücken (`gap`): ein Graben quer über die ganze Insel. Der Abstand entlang der Tangente am Lückenmittelpunkt ist kleiner als die halbe Lückenlänge + 0,05. Tiefe: bei `pirate` und `temple` auf Wasserhöhe − 0,3 (−1,45), bei `moon` −3,0.
  - **Schichten:** je Welt drei Lagen in den Weltfarben.
    - Piraten: Sand, Lehm, Fels.
    - Tempel: Moos/Erde, Lehm, Sandstein.
    - Mond: Regolith, Basalt, dunkler Fels; `tip` −3,2 für die schwebende Unterseite.
  - **Wasserhöhe** `water_z = -1.15` (Wasservolumen unverändert).
  - **Uferzone `water-shore`** (nicht `moon`):
    - Streifen von `waterline` 0,9 m nach außen, Höhe `water_z + 0.01`.
    - `uv.x` = 0 an der Linie, 1 außen.
    - Material `get("water")`, kein Bake, nur `Col`.

- [ ] **Step 4: Bandenverkleidung (`trim.py`)**

```python
def trim_profile():
    """(across, up) offsets from the inner rail edge at lane height; inner face flush, rounded top <= +2 cm."""
    pts = [(0.0, -0.05), (0.0, 0.215)]
    for k in range(1, 8):
        a = math.pi - k * math.pi / 8
        pts.append((0.085 + 0.085 * math.cos(a), 0.215 + 0.045 * math.sin(a)))
    pts += [(0.17, 0.215), (0.17, -0.05)]
    return pts
```

- **Querschnitte:** Pro Pfadprobe einen Querschnitt aus `rail_edge` aufspannen. `across` geht entlang des Außenvektors (vierter Rückgabewert), `up` ist Z ab `h`. Die Ringe verbinden, Enden schließen. Knoten `rail-trim-left` und `rail-trim-right`.
- **Welt-Varianten** im selben Knoten, als eigene Teile vor dem Zusammenfügen:
  - Piraten: Holzmaserung als Vertexfarb-Muster; Pfosten (0,07 × 0,07 × 0,32 m, gefast) alle 1,2 m außen.
  - Tempel: Fugen alle 0,6 m als dunkle Vertexfarbe; eine Moosleiste oben.
  - Mond: Metall; Neonleiste oben, 2 cm breit, als eigenes Teil `get("emissive", 1.6)` in Cyan, Magenta auf der rechten Seite; orange Klammern alle 1,5 m.
- `rail-left` und `rail-right` bekommen `obj["aurago_hidden"] = True`.

- [ ] **Step 5: Spur- und Bandenfarben**

`lane_material` liefert `materials.get("matte")`. Die Spur bekommt im Punktbereich eine Farbe:
- Filz: Weltfarbe mit leichtem Mittelstreifen (Mitte heller);
- Sand: Sandfarbe;
- Banden: Trimfarbe.

`courses_v1` behält `mesh_object` (flach schattiert). Diese Objekte gehen beim Backen in `bake.process` und bekommen so `Col` und AO.

- [ ] **Step 6: Zwischenprüfung an einer Bahn**

```powershell
& "D:\Blender 5.2\blender.exe" --background --factory-startup --python assets/minigolf/generate.py -- --only courses --course 7
node scripts/test-minigolf-assets.mjs
```

Erwartet:
- Fingerabdruck `coast-07` unverändert;
- Bahn-GLB < 768 KiB und < 24 000 Dreiecke (Zwischenstand ohne Platzierungen).

Andere Fehlermeldungen des Asset-Tests (neue Felder) erst in Task 8.

- [ ] **Step 7: Commit**

```bash
git add assets/minigolf/kit assets/minigolf/generate.py ui/3d/minigolf/course-07.glb ui/3d/minigolf/manifest.json
git commit -m "feat(minigolf): diorama islands and world rail trims for the legacy courses"
```

---

### Task 7: Umkleiden Teil 2 – Sonderteile, Platzierung, Bake mit Verdeckern, Zusammenfassen

**Files:**
- Create: `assets/minigolf/kit/dress_specials.py`
- Modify: `assets/minigolf/kit/dress.py`, `assets/minigolf/kit/export.py`, `assets/minigolf/kit/courses_v1.py`

- [ ] **Step 1: Sonderteile je Layout (`dress_specials.py`)**
  - Umsetzung nach der Tabelle in Spec 3.4. Sonderteile sind entweder:
    - **feste Platzierungen** aus dem Kit (z. B. `("gate", t=0.52, span=True)`, Palmen, Pfosten, Leuchtturm, Neonpfosten, Kristalle, Pads, Funkturm), oder
    - **eigene, an der Spur ausgerichtete Geometrie** mit Kit-Formen und Fase in der Lage der alten Teile:
      - Pier-Planken und Pfähle,
      - Brückenbrüstungen,
      - Sprungrampen,
      - Sonnenrad.
  - **Sonnenrad (`windmill`):** vier Segmente als eigene Knoten mit den Namen `windmill-sail` (Blender macht `.001` usw. daraus; die Laufzeit prüft `startsWith`). Sie liegen in der Blender-XZ-Ebene wie die alten Flügel; Nabe als Deko.
  - Funktionsnamen und Hindernisse bleiben unverändert. Hindernisboxen bekommen Weltmaterial und Fase.
  - Die Ausdehnungen bleiben dabei exakt, denn der Asset-Test vergleicht maximale Abstände. Der Fasen-Modifikator schneidet nur Ecken.

- [ ] **Step 2: Platzierung (`dress.place`)**

```python
def place(look, number, path, outline, top_height, fixed, catalog, hero_name, plan):
    """Deterministic prop placements in Blender coordinates: [(name, x, y, z, yaw, scale)].

    ``fixed`` are special placements that are kept first; ``plan`` is a list of
    (name, count) for medium props followed by scatter names; ``catalog`` maps
    name -> {"radius", "tags"} from the kit build.
    """
    rng = random.Random(1000 + number)
    corridor = 1.0
    cells = [(x * .25, y * .25) for y in range(-24, 25) for x in range(-24, 25)]
    cells = [c for c in cells if outline.contains_signed(*c) < -0.3]
    occupied = []

    def path_distance(x, y):
        best = 1e9
        for a, b in zip(path, path[1:]):
            ax, ay, bx, by = a[0], a[1], b[0], b[1]
            vx, vy = bx - ax, by - ay
            t = max(0.0, min(1.0, ((x - ax) * vx + (y - ay) * vy) / max(1e-9, vx * vx + vy * vy)))
            best = min(best, math.hypot(x - ax - t * vx, y - ay - t * vy))
        return best

    def clearance(x, y):
        return min(path_distance(x, y) - corridor, -outline.contains_signed(x, y) - 0.3)

    def free(x, y, r):
        return clearance(x, y) >= r and all(math.hypot(x - ox, y - oy) >= r + orad + 0.05 for ox, oy, orad in occupied)

    def facing(x, y):
        best = min(path, key=lambda p: math.hypot(p[0] - x, p[1] - y))
        return math.atan2(best[1] - y, best[0] - x) - math.pi / 2

    out = []
    for name, x, y, yaw, scale in fixed:
        out.append((name, x, y, top_height(x, y), yaw, scale))
        occupied.append((x, y, catalog[name]["radius"] * scale))
    if hero_name:
        r = catalog[hero_name]["radius"]
        spots = sorted(cells, key=lambda c: (-clearance(*c), c[1], c[0]))
        for x, y in spots:
            scale = min(1.0, clearance(x, y) / r) if r else 1.0
            if scale >= 0.7 and free(x, y, r * scale):
                out.append((hero_name, x, y, top_height(x, y), facing(x, y), round(scale, 3)))
                occupied.append((x, y, r * scale))
                break
    for name, count in plan:
        r = catalog[name]["radius"]
        order = cells[:]
        rng.shuffle(order)
        placed = 0
        for x, y in order:
            if placed >= count:
                break
            scale = round(rng.uniform(0.85, 1.15), 3)
            if free(x, y, r * scale):
                out.append((name, x, y, top_height(x, y), rng.uniform(0, math.tau), scale))
                occupied.append((x, y, r * scale))
                placed += 1
    return out[:160]
```

- **Pläne je Welt** in `dress.py`: Mittelgroße Requisiten stehen zuerst, Streudeko zuletzt; die Gesamtgrenze ist 160 Instanzen.
  - Piraten: `palm`×3, `palm2`×1, `barrel`×3, `chest`×1, `cannon`×1, `rock`×3, `post`×2, `crab`×2, Streudeko `tuft`×24, `shell`×14, `starfish`×6, `seaweed`×8, `footprint`×6, `xmark`×1. Hero `ship`, bei `lighthouse` `lighthouse`.
  - Tempel: `totem`×2, `pillar`×2, `torch`×4, `liana`×3, `rock`×3, `idol`×1, `waterfall`×1, Streudeko `fern`×18, `bush`×10, `tile`×12, `pebble`×14. Hero `head`, bei `flower_arch` zusätzlich das feste `gate`.
  - Mond: `dishbase`+`dishhead`×1 (gleiche Lage), `rover`×1, `solar`×2, `crate`×3, `boulder`×4, `neonpost`×4, `crystal`×3, `antenna`×2, Streudeko `crater`×6, `pebble`×18. Hero `habitat`, bei `finale` zusätzlich `tower`.
- `dishhead` wird immer mit derselben Lage wie `dishbase` gesetzt (eigene kleine Funktion), nicht frei verteilt.
- **Ausgabe:** Laufzeitkoordinaten `[name, x, z, -y, yaw, scale]`, gerundet auf 3 Stellen; `name` ohne Präfix.

- [ ] **Step 3: Backen mit Verdeckern**
  1. Vor `bake.process` werden die Kit-Objekte der Welt (aus `kits.build_kit`, einmal je Lauf gebaut) für jede Platzierung als `obj.copy()` mit geteiltem Mesh in die Bahnszene gelegt, mit Lage, Gier und Skalierung.
  2. `bake.process(course_bake_objects, ...)` mit diesen Kopien sichtbar, aber nicht in `objs`.
  3. Danach die Kopien löschen.
  - **Gebacken werden:** Insel, Verkleidung, Sonderteil-Geometrie, Loch, Fahne, Bahnnummer, Spur und Banden.
  - **Nur `Col` ohne AO** (`passthrough`): Ball- und Putter-Vorlage, Hindernisse, Uferzone.

- [ ] **Step 4: Statische Deko zusammenfassen (`export.py`)**

```python
FUNCTIONAL = ("surface-", "rail-", "flag", "ball-template", "putter-template", "obstacle-", "windmill-sail", "water-shore")


def join_scenery(objects):
    """Join every non-functional mesh per material into ``scenery-<material>`` nodes; returns the new object list."""
    keep, loose = [], []
    for obj in objects:
        (keep if obj.type != "MESH" or obj.name.startswith(FUNCTIONAL) or obj.get("aurago_obstacle") else loose).append(obj)
    groups = {}
    for obj in loose:
        bpy.context.view_layer.objects.active = obj
        bpy.ops.object.select_all(action="DESELECT")
        obj.select_set(True)
        with bpy.context.temp_override(active_object=obj, selected_objects=[obj], selected_editable_objects=[obj]):
            bpy.ops.mesh.separate(type="MATERIAL")
    for obj in [o for o in bpy.context.scene.objects if o.type == "MESH" and o not in keep and o.name.split(".")[0] not in FUNCTIONAL]:
        mat = obj.material_slots[0].material.name if obj.material_slots and obj.material_slots[0].material else "none"
        groups.setdefault(mat, []).append(obj)
    joined = []
    for mat, objs in sorted(groups.items()):
        target = objs[0]
        with bpy.context.temp_override(active_object=target, selected_objects=objs, selected_editable_objects=objs):
            bpy.ops.object.join()
        target.name = "scenery-" + mat
        joined.append(target)
    return keep + joined
```

Die Eigenschaften `aurago_role` der zusammengefassten Objekte entfallen. `scene_bounds` läuft vor dem Zusammenfassen. Ergebnisziel: höchstens 24 Mesh-Knoten je Bahn-GLB.

- [ ] **Step 5: Probelauf einer Bahn je Welt**

```powershell
foreach ($n in 2, 7, 18) { & "D:\Blender 5.2\blender.exe" --background --factory-startup --python assets/minigolf/generate.py -- --only courses --course $n }
```

Prüfen:
- Fingerabdrücke unverändert (Asset-Test);
- Größen und Dreiecke im Budget;
- ein Kontroll-Render je Welt (`--save-blend`, dann `probe`-ähnlicher Render oder Blender-MCP-Vorschau) ohne schwebende Requisiten und ohne Requisiten auf der Spur.

- [ ] **Step 6: Commit**

```bash
git add assets/minigolf/kit assets/minigolf/generate.py ui/3d/minigolf/course-02.glb ui/3d/minigolf/course-07.glb ui/3d/minigolf/course-18.glb ui/3d/minigolf/manifest.json
git commit -m "feat(minigolf): world specials, placements and contact-baked course dressing"
```

---

### Task 8: Manifest-Felder, Vollauf, Asset-Test

**Files:**
- Modify: `assets/minigolf/generate.py`, `assets/minigolf/kit/courses_v1.py`, `scripts/test-minigolf-assets.mjs`
- Regenerate: `ui/3d/minigolf/*`

- [ ] **Step 1: Felder schreiben**

- Je Bahn ergänzt `build_course` mit Umkleider den Eintrag um drei Felder:
  - `look`: aus `{"coast": "pirate", "garden": "temple", "night": "moon"}`;
  - `path`: jeder dritte Stützpunkt plus der letzte;
  - `props`.

```python
path_rt = [q(runtime((p[0], p[1], lane_center_height(course, path, i)))) for i, p in enumerate(path) if i % 3 == 0 or i == len(path) - 1]
entry.update({"look": look, "path": [[round(v, 3) for v in p] for p in path_rt], "props": placements_runtime})
```

- Top-Level:
  - `manifest["kits"]` aus Task 4;
  - `manifest["worlds"]` mit den drei Einträgen in Reihenfolge des ersten Auftretens:

```python
manifest["worlds"] = [{"id": world, "look": LOOK[world], "preview": "previews/%s.webp" % LOOK[world]} for world in dict.fromkeys(c["world"] for c in courses)]
```

- `--course N` liest das bestehende Manifest, ersetzt nur Eintrag N und schreibt zurück.

- [ ] **Step 2: Asset-Test erweitern**

In `scripts/test-minigolf-assets.mjs`:

1. Neue Konstanten nach dem Laden des Manifests:

```js
const LOOK_OF = { coast: 'pirate', garden: 'temple', night: 'moon' };
const KIB = 1024, MIB = 1024 * 1024;
function pathDistance(points, x, z) {
  let best = Infinity;
  for (let i = 1; i < points.length; i++) {
    const [ax, , az] = points[i - 1], [bx, , bz] = points[i], vx = bx - ax, vz = bz - az;
    const t = Math.max(0, Math.min(1, ((x - ax) * vx + (z - az) * vz) / Math.max(1e-9, vx * vx + vz * vz)));
    best = Math.min(best, Math.hypot(x - ax - t * vx, z - az - t * vz));
  }
  return best;
}
```

2. In der Bahnschleife die alte Dreiecksgrenze `100_000` ersetzen und ergänzen:

```js
  assert.ok(course.bytes <= 768 * KIB, `${course.id}: course GLB exceeds 768 KiB (${course.bytes})`);
  assert.ok(course.triangles <= 24_000, `${course.id}: triangle budget (${course.triangles})`);
  const meshNodes = glb.json.nodes.filter(node => node.mesh !== undefined).length;
  assert.ok(meshNodes <= 24, `${course.id}: ${meshNodes} mesh nodes (max 24)`);
  assert.equal(course.look, LOOK_OF[course.world], `${course.id}: look`);
  assert.ok(Array.isArray(course.path) && course.path.length >= 17 && course.path.length <= 33, `${course.id}: path samples`);
  assert.ok(Math.hypot(course.path[0][0] - course.tee[0], course.path[0][2] - course.tee[2]) <= .3, `${course.id}: path starts at the tee`);
  assert.ok(Math.hypot(course.path.at(-1)[0] - course.cup[0], course.path.at(-1)[2] - course.cup[2]) <= .5, `${course.id}: path ends at the cup`);
  for (const name of ['rail-left', 'rail-right']) assert.equal(glb.nodeByName.get(name)?.node.extras?.aurago_hidden, true, `${course.id}: ${name} hidden at runtime`);
  for (const name of ['rail-trim-left', 'rail-trim-right']) assert.ok(glb.nodeByName.has(name), `${course.id}: missing ${name}`);
  const catalog = manifest.kits?.[course.look]?.props;
  assert.ok(catalog, `${course.id}: kit catalog for ${course.look}`);
  assert.ok(course.props.length > 0 && course.props.length <= 160, `${course.id}: ${course.props.length} placements`);
  for (const [name, x, y, z, yaw, scale] of course.props) {
    const prop = catalog[name];
    assert.ok(prop, `${course.id}: unknown prop ${name}`);
    assert.ok([x, y, z, yaw, scale].every(Number.isFinite) && scale > 0, `${course.id}: ${name} placement numbers`);
    assert.ok(Math.hypot(x, z) <= 6.0, `${course.id}: ${name} outside the island`);
    if (!prop.tags.includes('span')) assert.ok(pathDistance(course.path, x, z) >= 1.0 + prop.radius * scale - .05, `${course.id}: ${name} blocks the lane corridor`);
  }
```

3. Nach der Bahnschleife Kits, Vorschaubilder und Gesamtsumme prüfen:

```js
for (const look of ['pirate', 'temple', 'moon']) {
  const kit = manifest.kits[look];
  const bytes = await readFile(path.join(assetRoot, kit.model));
  assert.equal(kit.model, `props-${look}.glb`);
  assert.equal(bytes.length, kit.bytes, `${look} kit bytes`);
  assert.equal(createHash('sha256').update(bytes).digest('hex'), kit.sha256, `${look} kit SHA-256`);
  assert.ok(bytes.length <= 3 * MIB, `${look} kit exceeds 3 MiB`);
  const glb = parseGLB(bytes);
  assert.equal(glb.triangleCount, kit.triangles, `${look} kit triangle count`);
  assert.ok(kit.triangles <= 60_000, `${look} kit triangle budget`);
  for (const [name, prop] of Object.entries(kit.props)) {
    const node = glb.nodeByName.get(`prop-${look}-${name}`);
    assert.ok(node && !glb.parents.has(node.index), `${look}: prop-${look}-${name} must be a top-level node`);
    const mesh = glb.json.meshes[node.node.mesh];
    assert.ok(mesh.primitives.length <= 3, `${look}/${name}: ${mesh.primitives.length} primitives`);
    for (const primitive of mesh.primitives) assert.ok(primitive.attributes.COLOR_0 !== undefined, `${look}/${name}: COLOR_0`);
    assert.ok(prop.triangles <= (prop.tags.includes('hero') ? 16_000 : 15_000), `${look}/${name}: ${prop.triangles} triangles`);
    assert.deepEqual(node.node.extras?.aurago_prop?.tags, prop.tags, `${look}/${name}: node extras`);
  }
  const result = await validator.validateBytes(bytes, { uri: kit.model, maxIssues: 100, externalResourceFunction: () => Promise.reject(Error('external resource forbidden')) });
  assert.equal(result.issues.numErrors, 0, `${look} kit: glTF validation errors`);
}
assert.deepEqual(manifest.worlds, [
  { id: 'garden', look: 'temple', preview: 'previews/temple.webp' },
  { id: 'coast', look: 'pirate', preview: 'previews/pirate.webp' },
  { id: 'night', look: 'moon', preview: 'previews/moon.webp' }
]);
for (const world of manifest.worlds) {
  const image = await readFile(path.join(assetRoot, world.preview));
  assert.equal(image.toString('ascii', 0, 4) + image.toString('ascii', 8, 12), 'RIFFWEBP', `${world.preview}: WebP`);
  assert.ok(image.length <= 80 * 1024, `${world.preview}: ${image.length} bytes`);
}
async function treeBytes(dir) {
  let sum = 0;
  for (const entry of await readdir(dir, { withFileTypes: true })) sum += entry.isDirectory() ? await treeBytes(path.join(dir, entry.name)) : (await stat(path.join(dir, entry.name))).size;
  return sum;
}
const packageBytes = await treeBytes(assetRoot);
assert.ok(packageBytes <= 32 * MIB, `AuraGolf runtime assets exceed 32 MiB: ${packageBytes}`);
```

- Import ergänzen: `import { readFile, readdir, stat } from 'node:fs/promises';`.
- Die alte 12-MiB-Prüfung (`runtimeBytes < 12 * 1024 * 1024`) entfernen; die Konsolenausgabe um `packageBytes` ergänzen.

- [ ] **Step 3: Vollauf (Hintergrund, etwa 15 min)**

```powershell
& "D:\Blender 5.2\blender.exe" --background --factory-startup --python assets/minigolf/generate.py
```

Erwartet:
- 3 Kit-Zeilen und 18 Bahnzeilen;
- `Wrote 18 courses`.

Bei Dateisperre wiederholen.

- [ ] **Step 4: Tests**

```bash
node scripts/test-minigolf-assets.mjs
node scripts/test-minigolf-courses.mjs
node scripts/test-minigolf-physics.mjs
```

Erwartet:
- alle PASS;
- Bahnen-Test mit den unveränderten P1-Routen;
- Asset-Test meldet höchstens fehlende Vorschaubilder bis Task 9. Diese drei Vorschau-Asserts dürfen bis Task 9 rot sein; keinen anderen Fehler hinnehmen.

- [ ] **Step 5: Determinismus**

Den Vollauf ein zweites Mal ausführen, danach muss `git status --short ui/3d/minigolf` leer sein (alles bereits committet) oder dieselben Hashes zeigen. Weichen Hashes ab, die Ursache beheben (unsortierte Iteration, Bake-Rauschen), bevor es weitergeht.

- [ ] **Step 6: Commit**

```bash
git add assets/minigolf ui/3d/minigolf scripts/test-minigolf-assets.mjs
git commit -m "feat(minigolf): redress all 18 courses as pirate, temple and moon dioramas"
```

---

### Task 9: Vorschaubilder

**Files:**
- Create: `assets/minigolf/kit/previews.py`, `ui/3d/minigolf/previews/{pirate,temple,moon}.webp`
- Modify: `assets/minigolf/generate.py`

- [ ] **Step 1: Renderer**

`previews.render(look, number, out_path)`:
1. Baut die umgekleidete Bahn `number` mit echten Kit-Kopien statt Instanzen.
2. Setzt Wasser (`island.build_water`) bzw. Weltraum, Sonne und Himmel über `stage` wie die Proben.
3. Kamera: 43° Sichtfeld, 38° Höhe, Azimut −35°, Bildausschnitt auf die Insel.
4. Rendert Cycles mit 96 Samples und OIDN in 640×360.
5. Speichert WebP mit Qualität 82.

Bahnen:
- `pirate` ← 12 (Lighthouse Way);
- `temple` ← 2 (Flower Arch mit Stufentor);
- `moon` ← 18 (Grand Finale).

- [ ] **Step 2: Erzeugen und prüfen**

```powershell
& "D:\Blender 5.2\blender.exe" --background --factory-startup --python assets/minigolf/generate.py -- --only previews
node scripts/test-minigolf-assets.mjs
```

Erwartet: drei Dateien, je ≤ 80 KB; Asset-Test PASS. Die Bilder ansehen (Read-Werkzeug) und bei Bedarf Kamera oder Licht nachstellen.

- [ ] **Step 3: Commit**

```bash
git add assets/minigolf/kit/previews.py assets/minigolf/generate.py ui/3d/minigolf/previews
git commit -m "feat(minigolf): rendered world previews for the round cards"
```

---

### Prüfung 1 (reader-agent): Generator

Eine kombinierte Durchsicht der Tasks 3–9 gegen Spec Abschnitte 3, 4 und 6.

**Schwerpunkte:**
- Spur- und Bandencode wortgleich;
- Determinismus (keine unsortierte Iteration, feste Seeds);
- Budgets;
- Materialvertrag;
- Korridorregel;
- keine toten Module.

**Ausgabe:** Nur Critical/Important. Befunde behebt ein maker-agent in einem Commit; keine zweite Prüfrunde.

---

### Task 10: Look-Daten, Kamerafahrt, Node-Tests

**Files:**
- Create: `ui/js/desktop/apps/minigolf-looks.js`, `ui/js/desktop/apps/minigolf-camera.js`, `scripts/test-minigolf-look.mjs`
- Modify: `package.json` (Skript `test:minigolf`)

- [ ] **Step 1: Failing test schreiben (`scripts/test-minigolf-look.mjs`)**

```js
import assert from 'node:assert/strict';
import { LOOKS, LOOK_IDS, TIERS, lookOf, placementMatrix, tierConfig } from '../ui/js/desktop/apps/minigolf-looks.js';
import { createIntro, introDuration, pathLength } from '../ui/js/desktop/apps/minigolf-camera.js';

const close = (actual, expected, message, eps = 1e-6) => expected.forEach((value, i) => assert.ok(Math.abs(actual[i] - value) <= eps, `${message}[${i}]: ${actual[i]} != ${value}`));

// Every look carries the data the renderer reads.
assert.deepEqual(LOOK_IDS, ['pirate', 'temple', 'moon']);
for (const id of LOOK_IDS) {
    const look = LOOKS[id];
    for (const key of ['exposure', 'background', 'sky', 'sun', 'environment', 'bloom', 'water', 'fx']) assert.ok(look[key] !== undefined, `${id}.${key}`);
    assert.equal(look.sun.direction.length, 3, `${id} sun direction`);
    assert.ok(look.fx.confetti.length >= 4, `${id} confetti colours`);
    assert.ok(['lagoon', 'jungle', 'void'].includes(look.water.mode), `${id} water mode`);
}
assert.equal(LOOKS.moon.water.mode, 'void');
assert.equal(LOOKS.moon.fog, null);

// Legacy worlds map onto the new looks until P3 renames them.
assert.equal(lookOf({ world: 'coast' }), 'pirate');
assert.equal(lookOf({ world: 'garden' }), 'temple');
assert.equal(lookOf({ world: 'night' }), 'moon');
assert.equal(lookOf({ world: 'coast', look: 'moon' }), 'moon');
assert.equal(lookOf({ world: 'pirate' }), 'pirate');
assert.equal(lookOf(null), 'pirate');

// Quality tiers.
assert.deepEqual(['low', 'medium', 'high'].map(tier => TIERS[tier].particles), [60, 160, 320]);
assert.equal(TIERS.low.scatter, false);
assert.equal(TIERS.low.shadows, 0);
assert.equal(TIERS.high.shadows, 2048);
assert.equal(tierConfig('nonsense'), TIERS.medium);

// Placement [name, x, y, z, yaw, scale] -> column-major matrix (yaw about +Y).
const m = placementMatrix(['barrel', 1, 2, 3, Math.PI / 2, 2]);
const apply = (p) => [0, 1, 2].map(r => m[r] * p[0] + m[4 + r] * p[1] + m[8 + r] * p[2] + m[12 + r]);
close(apply([1, 0, 0]), [1, 2, 1], 'yaw +90 deg maps +X to -Z');
close(apply([0, 1, 0]), [1, 4, 3], 'uniform scale on Y');

// Intro flight.
const straight = Array.from({ length: 11 }, (_, i) => [0, .16, -i]);
assert.equal(pathLength(straight), 10);
assert.equal(introDuration(straight), 3.5);
assert.equal(introDuration(straight.slice(0, 2)), 2.5);
assert.equal(introDuration(Array.from({ length: 31 }, (_, i) => [0, 0, -i])), 4.5);
assert.equal(createIntro([[0, 0, 0]], [0, 0, 0], [0, 0, 0]), null);
const end = [10, 13, 9], target = [0, 0, -5];
const intro = createIntro(straight, end, target);
const position = [], look = [];
assert.equal(intro.sample(0, position, look), false);
close(position, [0, 3.36, 2.2], 'start behind and above the tee');
close(look, [0, .16, -2.5], 'start looks 2.5 m ahead');
intro.sample(intro.duration * .3, position, look);
assert.ok(position[1] > 3 && position[2] < 0, 'phase A flies along the lane');
assert.equal(intro.sample(intro.duration, position, look), true);
close(position, end, 'ends in the overview pose');
close(look, target, 'ends on the overview target');
console.log('AuraGolf look tests: ok');
```

- [ ] **Step 2: Test laufen lassen, er scheitert**

Run: `node scripts/test-minigolf-look.mjs`
Expected: FAIL mit `ERR_MODULE_NOT_FOUND` für `minigolf-looks.js`.

- [ ] **Step 3: `minigolf-looks.js`**

```js
// Per-look rendering data for AuraGolf (P2). Pure data and math without
// three.js, so the scene modules and the Node tests share it.
const deepFreeze = value => {
    if (value && typeof value === 'object' && !Object.isFrozen(value)) { Object.freeze(value); Object.values(value).forEach(deepFreeze); }
    return value;
};

export const LOOK_IDS = Object.freeze(['pirate', 'temple', 'moon']);
const LEGACY_LOOK = Object.freeze({ coast: 'pirate', garden: 'temple', night: 'moon' });

export function lookOf(course) {
    if (course && LOOK_IDS.includes(course.look)) return course.look;
    if (course && LOOK_IDS.includes(course.world)) return course.world;
    return LEGACY_LOOK[course?.world] || 'pirate';
}

export const LOOKS = deepFreeze({
    pirate: {
        exposure: 1.0,
        background: 0x9ad8e6,
        sky: { zenith: 0x3fa7e0, horizon: 0xc4f0f2, below: 0x2a9fb0, sun: 0xfff3c9, clouds: .6, cloudColor: 0xffffff, haze: 0, silhouette: 0, silhouetteColor: 0x000000, stars: 0, earth: false },
        fog: { color: 0xa9e2ea, density: .010 },
        sun: { color: 0xfff1d6, intensity: 2.6, direction: [-.42, .84, .34] },
        rim: null,
        environment: .85,
        bloom: { strength: .14, radius: .4, threshold: .95 },
        water: { mode: 'lagoon', shallow: 0x62e3d6, deep: 0x0f6f93, foam: 0xf5fffb, level: -1.15 },
        fx: { grass: 0x9fd86a, sand: 0xf0d49a, wood: 0x9a6038, stone: 0xb7ab98, wall: 'chips', splash: 0xd4f7f4, ambient: 'glint', ambientColor: 0xfff8d8, gold: 0xffd35a, confetti: [0xd9483b, 0xf2b33d, 0x2ec4c9, 0xf4e9d0, 0x5dbb63] }
    },
    temple: {
        exposure: 1.0,
        background: 0xe8b98a,
        sky: { zenith: 0x3f8f9a, horizon: 0xffc58a, below: 0x2d7f74, sun: 0xffb061, clouds: .25, cloudColor: 0xffd9b0, haze: .55, silhouette: .85, silhouetteColor: 0x24453a, stars: 0, earth: false },
        fog: { color: 0xe7b98c, density: .014 },
        sun: { color: 0xffbf7a, intensity: 2.2, direction: [.78, .36, -.5] },
        rim: null,
        environment: .7,
        bloom: { strength: .22, radius: .5, threshold: .85 },
        water: { mode: 'jungle', shallow: 0x3cc2a8, deep: 0x125f5c, foam: 0xeefcf0, level: -1.15 },
        fx: { grass: 0x7fbf4a, sand: 0xd9b27a, wood: 0x8a5a32, stone: 0xc9b089, wall: 'dust', splash: 0xc7efe2, ambient: 'fireflies', ambientColor: 0xd9ff8a, gold: 0xffc94a, confetti: [0xe8b33a, 0x8cc04a, 0xff8a2a, 0x2aa39a, 0xf4e2c0] }
    },
    moon: {
        exposure: 1.05,
        background: 0x050816,
        sky: { zenith: 0x02040d, horizon: 0x0b1030, below: 0x02040d, sun: 0xcfe0ff, clouds: 0, cloudColor: 0x000000, haze: 0, silhouette: 0, silhouetteColor: 0x000000, stars: 1, earth: true },
        fog: null,
        sun: { color: 0xd6e4ff, intensity: 2.4, direction: [-.86, .42, .28] },
        rim: { color: 0x7fb4ff, intensity: .9, direction: [.6, .35, -.72] },
        environment: .55,
        bloom: { strength: .42, radius: .55, threshold: .75 },
        water: { mode: 'void', level: -1.15 },
        fx: { grass: 0x9a98b0, sand: 0xb7b4c8, wood: 0xffd27a, stone: 0xc0bcd6, wall: 'sparks', splash: 0xc9c6dd, ambient: 'dust', ambientColor: 0xb8c4ff, gold: 0x38f2ff, confetti: [0x38f2ff, 0xff4fd8, 0xff8a3d, 0xe8ecf2, 0x7fb4ff] }
    }
});

export const TIERS = deepFreeze({
    low: { dpr: 1, shadows: 0, bloom: false, clouds: false, twinkle: false, waterMotion: false, glints: false, scatter: false, motion: false, particles: 60, trail: false },
    medium: { dpr: 1.5, shadows: 1024, bloom: true, clouds: true, twinkle: false, waterMotion: true, glints: false, scatter: true, motion: true, particles: 160, trail: true },
    high: { dpr: 2, shadows: 2048, bloom: true, clouds: true, twinkle: true, waterMotion: true, glints: true, scatter: true, motion: true, particles: 320, trail: true }
});

export const tierConfig = tier => TIERS[tier] || TIERS.medium;

// Column-major 4x4 for a manifest placement [name, x, y, z, yaw, scale]; yaw turns about +Y.
export function placementMatrix(entry, out = new Array(16)) {
    const x = entry[1], y = entry[2], z = entry[3], yaw = entry[4] || 0, scale = entry[5] || 1;
    const c = Math.cos(yaw) * scale, s = Math.sin(yaw) * scale;
    out[0] = c; out[1] = 0; out[2] = -s; out[3] = 0;
    out[4] = 0; out[5] = scale; out[6] = 0; out[7] = 0;
    out[8] = s; out[9] = 0; out[10] = c; out[11] = 0;
    out[12] = x; out[13] = y; out[14] = z; out[15] = 1;
    return out;
}
```

- [ ] **Step 4: `minigolf-camera.js`**

```js
// Course intro flight (P2): pure math on [x, y, z] arrays so Node tests can run it.
const BEHIND = 2.2, ABOVE = 3.2, AHEAD = 2.5, PHASE_A = .6;
const smooth = t => t * t * (3 - 2 * t);
const distance = (a, b) => Math.hypot(b[0] - a[0], b[1] - a[1], b[2] - a[2]);

export function pathLength(path) {
    let total = 0;
    for (let i = 1; i < (path?.length || 0); i++) total += distance(path[i - 1], path[i]);
    return total;
}

export const introDuration = path => Math.min(4.5, Math.max(2.5, .35 * pathLength(path)));

export function createIntro(path, endPosition, endTarget) {
    if (!Array.isArray(path) || path.length < 2) return null;
    const cumulative = [0];
    for (let i = 1; i < path.length; i++) cumulative.push(cumulative[i - 1] + distance(path[i - 1], path[i]));
    const total = cumulative[cumulative.length - 1];
    if (!(total > .5)) return null;
    const duration = introDuration(path), here = [0, 0, 0], ahead = [0, 0, 0];
    function along(d, out) {
        let i = 1;
        while (i < path.length - 1 && cumulative[i] < d) i++;
        const span = cumulative[i] - cumulative[i - 1] || 1, k = Math.max(0, Math.min(1, (d - cumulative[i - 1]) / span));
        for (let a = 0; a < 3; a++) out[a] = path[i - 1][a] + (path[i][a] - path[i - 1][a]) * k;
        return out;
    }
    function poseA(u, position, target) {
        const d = smooth(Math.max(0, Math.min(1, u))) * total;
        along(d, here); along(Math.min(total, d + AHEAD), ahead);
        let hx = ahead[0] - here[0], hz = ahead[2] - here[2], length = Math.hypot(hx, hz);
        if (length < 1e-4) {
            const a = path[path.length - 2], b = path[path.length - 1];
            hx = b[0] - a[0]; hz = b[2] - a[2]; length = Math.hypot(hx, hz) || 1;
        }
        hx /= length; hz /= length;
        position[0] = here[0] - hx * BEHIND; position[1] = here[1] + ABOVE; position[2] = here[2] - hz * BEHIND;
        target[0] = ahead[0]; target[1] = ahead[1]; target[2] = ahead[2];
    }
    const endA = [[0, 0, 0], [0, 0, 0]];
    poseA(1, endA[0], endA[1]);
    return {
        duration,
        // Writes the camera pose for `time` seconds into position/target; returns true when finished.
        sample(time, position, target) {
            const t = Math.max(0, Math.min(1, time / duration));
            if (t <= PHASE_A) { poseA(t / PHASE_A, position, target); return false; }
            const k = smooth((t - PHASE_A) / (1 - PHASE_A));
            for (let a = 0; a < 3; a++) {
                position[a] = endA[0][a] + (endPosition[a] - endA[0][a]) * k;
                target[a] = endA[1][a] + (endTarget[a] - endA[1][a]) * k;
            }
            return t >= 1;
        }
    };
}
```

- [ ] **Step 5: Test grün, Skript eintragen**

In `package.json` in `test:minigolf` nach `test-minigolf-model.mjs` ` && node scripts/test-minigolf-look.mjs` einfügen.

Run: `node scripts/test-minigolf-look.mjs`
Expected: `AuraGolf look tests: ok`

- [ ] **Step 6: Commit**

```bash
git add ui/js/desktop/apps/minigolf-looks.js ui/js/desktop/apps/minigolf-camera.js scripts/test-minigolf-look.mjs package.json
git commit -m "feat(minigolf): look data, quality tiers and intro flight math"
```

---

### Task 11: Himmel und Umgebungslicht

**Files:** Create `ui/js/desktop/apps/minigolf-sky.js`

- [ ] **Step 1: Modul schreiben**

```js
import * as THREE from 'three';

const NOISE = `
float golfHash(vec2 p) { p = fract(p * vec2(123.34, 456.21)); p += dot(p, p + 45.32); return fract(p.x * p.y); }
float golfNoise(vec2 p) {
    vec2 i = floor(p), f = fract(p), u = f * f * (3.0 - 2.0 * f);
    return mix(mix(golfHash(i), golfHash(i + vec2(1.0, 0.0)), u.x), mix(golfHash(i + vec2(0.0, 1.0)), golfHash(i + vec2(1.0, 1.0)), u.x), u.y);
}
float golfFbm(vec2 p) { return golfNoise(p) * 0.62 + golfNoise(p * 2.03 + 7.1) * 0.38; }
float golfHash3(vec3 p) { p = fract(p * 0.3183099 + 0.1); p *= 17.0; return fract(p.x * p.y * p.z * (p.x + p.y + p.z)); }
float golfNoise3(vec3 x) {
    vec3 i = floor(x), f = fract(x); f = f * f * (3.0 - 2.0 * f);
    return mix(mix(mix(golfHash3(i), golfHash3(i + vec3(1, 0, 0)), f.x), mix(golfHash3(i + vec3(0, 1, 0)), golfHash3(i + vec3(1, 1, 0)), f.x), f.y),
               mix(mix(golfHash3(i + vec3(0, 0, 1)), golfHash3(i + vec3(1, 0, 1)), f.x), mix(golfHash3(i + vec3(0, 1, 1)), golfHash3(i + vec3(1, 1, 1)), f.x), f.y), f.z);
}
float golfFbm3(vec3 p) { return golfNoise3(p) * 0.55 + golfNoise3(p * 2.1 + 3.7) * 0.3 + golfNoise3(p * 4.3 + 9.2) * 0.15; }
`;

const DOME_VERT = `varying vec3 vGolfDir;
void main() { vGolfDir = position; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }`;

const DOME_FRAG = `uniform vec3 zenith; uniform vec3 horizon; uniform vec3 below; uniform vec3 sunColor; uniform vec3 sunDir;
uniform vec3 cloudColor; uniform vec3 silhouetteColor;
uniform float clouds; uniform float cloudOn; uniform float haze; uniform float silhouette; uniform float band; uniform float golfTime;
varying vec3 vGolfDir;
${NOISE}
void main() {
    vec3 d = normalize(vGolfDir);
    float h = d.y;
    vec3 color = h >= 0.0 ? mix(horizon, zenith, smoothstep(0.0, 0.55, h)) : mix(horizon, below, smoothstep(0.0, 0.25, -h));
    float s = max(dot(d, sunDir), 0.0);
    color += sunColor * (pow(s, 48.0) * 0.35 + pow(s, 900.0) * 1.6);
    if (haze > 0.0) color = mix(color, horizon * 1.05, haze * exp(-abs(h) * 9.0));
    if (silhouette > 0.0) {
        vec2 around = normalize(d.xz + vec2(1e-4));
        float ridge = 0.03 + 0.035 * golfFbm(around * 2.5 + 3.0) + 0.012 * golfFbm(around * 9.0 + 1.3);
        color = mix(color, silhouetteColor, silhouette * smoothstep(ridge + 0.004, ridge - 0.004, h) * step(-0.02, h));
    }
    if (clouds > 0.0 && cloudOn > 0.5 && h > 0.0) {
        vec2 uv = d.xz / (h + 0.12) * 0.55 + vec2(golfTime * 0.006, golfTime * 0.002);
        color = mix(color, cloudColor, clouds * smoothstep(0.52, 0.78, golfFbm(uv)) * smoothstep(0.0, 0.18, h));
    }
    if (band > 0.0) color += vec3(0.10, 0.11, 0.2) * band * exp(-pow(dot(d, normalize(vec3(0.3, 0.55, 0.78))) * 5.0, 2.0)) * (0.6 + 0.4 * golfFbm(d.xy * 6.0));
    gl_FragColor = vec4(color, 1.0);
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
}`;

const STAR_VERT = `attribute float golfSize; attribute float golfPhase; uniform float golfTime; uniform float twinkle; varying float vGolfFade;
void main() {
    vGolfFade = 1.0 - twinkle * 0.45 * (0.5 + 0.5 * sin(golfTime * (1.3 + golfPhase) + golfPhase * 7.0));
    gl_PointSize = golfSize;
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}`;
const STAR_FRAG = `varying float vGolfFade;
void main() { vec2 c = gl_PointCoord - 0.5; float d = dot(c, c); if (d > 0.25) discard; gl_FragColor = vec4(vec3(0.95, 0.96, 1.0) * vGolfFade, smoothstep(0.25, 0.0, d)); #include <colorspace_fragment> }`;

const EARTH_VERT = `varying vec3 vGolfNormal; varying vec3 vGolfObject; varying vec3 vGolfView;
void main() {
    vec4 world = modelMatrix * vec4(position, 1.0);
    vGolfNormal = normalize(mat3(modelMatrix) * normal); vGolfObject = position; vGolfView = cameraPosition - world.xyz;
    gl_Position = projectionMatrix * viewMatrix * world;
}`;
const EARTH_FRAG = `uniform vec3 sunDir; uniform float golfTime; varying vec3 vGolfNormal; varying vec3 vGolfObject; varying vec3 vGolfView;
${NOISE}
void main() {
    vec3 n = normalize(vGolfNormal);
    float land = smoothstep(0.50, 0.56, golfFbm3(vGolfObject * 0.28 + 3.0));
    vec3 ground = mix(vec3(0.16, 0.45, 0.16), vec3(0.55, 0.47, 0.30), smoothstep(0.6, 0.8, golfFbm3(vGolfObject * 0.6)));
    vec3 color = mix(vec3(0.05, 0.22, 0.55), ground, land);
    color = mix(color, vec3(1.0), 0.85 * smoothstep(0.55, 0.75, golfFbm3(vGolfObject * 0.45 + vec3(golfTime * 0.01))));
    float lit = clamp(dot(n, sunDir) * 1.2 + 0.08, 0.0, 1.0);
    color *= lit;
    color += vec3(0.25, 0.5, 1.0) * pow(1.0 - max(dot(n, normalize(vGolfView)), 0.0), 3.0) * (0.35 + lit);
    gl_FragColor = vec4(color, 1.0);
    #include <colorspace_fragment>
}`;

function seeded(seed) {
    return () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; };
}

// Sky dome (and for moon: stars and the Earth) for one look; `state` shares golfTime with the scene.
export function createSky(look, cfg, state = { golfTime: { value: 0 } }, backdrop = true) {
    const group = new THREE.Group();
    group.name = 'aurago-sky';
    const sunDir = new THREE.Vector3().fromArray(look.sun.direction).normalize();
    const uniforms = {
        zenith: { value: new THREE.Color(look.sky.zenith) }, horizon: { value: new THREE.Color(look.sky.horizon) }, below: { value: new THREE.Color(look.sky.below) },
        sunColor: { value: new THREE.Color(look.sky.sun) }, sunDir: { value: sunDir }, cloudColor: { value: new THREE.Color(look.sky.cloudColor) },
        silhouetteColor: { value: new THREE.Color(look.sky.silhouetteColor) }, clouds: { value: look.sky.clouds }, cloudOn: { value: cfg.clouds ? 1 : 0 },
        haze: { value: look.sky.haze }, silhouette: { value: look.sky.silhouette }, band: { value: look.sky.stars ? .8 : 0 }, golfTime: state.golfTime
    };
    const dome = new THREE.Mesh(new THREE.SphereGeometry(150, 48, 24), new THREE.ShaderMaterial({ uniforms, vertexShader: DOME_VERT, fragmentShader: DOME_FRAG, side: THREE.BackSide, depthWrite: false }));
    dome.renderOrder = -10; dome.frustumCulled = false;
    group.add(dome);
    const disposables = [dome.geometry, dome.material];
    let starUniforms = null;
    if (backdrop && look.sky.stars) {
        const random = seeded(4242), count = 900, positions = new Float32Array(count * 3), sizes = new Float32Array(count), phases = new Float32Array(count);
        for (let i = 0; i < count; i++) {
            const y = random() * 2 - 1, theta = random() * Math.PI * 2, r = Math.sqrt(1 - y * y);
            positions.set([Math.cos(theta) * r * 140, y * 140, Math.sin(theta) * r * 140], i * 3);
            sizes[i] = 1.2 + random() * random() * 2.6; phases[i] = random() * 3;
        }
        const geometry = new THREE.BufferGeometry();
        geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
        geometry.setAttribute('golfSize', new THREE.BufferAttribute(sizes, 1));
        geometry.setAttribute('golfPhase', new THREE.BufferAttribute(phases, 1));
        starUniforms = { golfTime: state.golfTime, twinkle: { value: cfg.twinkle ? 1 : 0 } };
        const stars = new THREE.Points(geometry, new THREE.ShaderMaterial({ uniforms: starUniforms, vertexShader: STAR_VERT, fragmentShader: STAR_FRAG, transparent: true, depthWrite: false }));
        stars.renderOrder = -9; stars.frustumCulled = false;
        group.add(stars); disposables.push(geometry, stars.material);
    }
    if (backdrop && look.sky.earth) {
        const earth = new THREE.Mesh(new THREE.SphereGeometry(10, 48, 32), new THREE.ShaderMaterial({ uniforms: { sunDir: { value: sunDir }, golfTime: state.golfTime }, vertexShader: EARTH_VERT, fragmentShader: EARTH_FRAG }));
        earth.position.set(-.55, .32, -.77).normalize().multiplyScalar(120);
        earth.renderOrder = -8;
        group.add(earth); disposables.push(earth.geometry, earth.material);
    }
    return {
        group,
        setTier(next) { uniforms.cloudOn.value = next.clouds ? 1 : 0; if (starUniforms) starUniforms.twinkle.value = next.twinkle ? 1 : 0; },
        dispose() { group.removeFromParent(); for (const item of disposables) item.dispose(); }
    };
}

// Prefiltered environment from the look's sky and a bright sun disc; built once per look.
export function createEnvironment(renderer, look) {
    const pmrem = new THREE.PMREMGenerator(renderer);
    const envScene = new THREE.Scene();
    const sky = createSky(look, { clouds: false, twinkle: false }, undefined, false);
    envScene.add(sky.group);
    const sun = new THREE.Mesh(new THREE.SphereGeometry(8, 16, 8), new THREE.MeshBasicMaterial({ color: new THREE.Color(look.sun.color).multiplyScalar(look.sun.intensity * 4) }));
    sun.position.fromArray(look.sun.direction).normalize().multiplyScalar(100);
    envScene.add(sun);
    const target = pmrem.fromScene(envScene, .035, .1, 400);
    sky.dispose(); sun.geometry.dispose(); sun.material.dispose(); pmrem.dispose();
    return target;
}
```

- [ ] **Step 2: Syntax prüfen und committen**

```bash
node --check ui/js/desktop/apps/minigolf-sky.js
git add ui/js/desktop/apps/minigolf-sky.js
git commit -m "feat(minigolf): per-world sky dome and local PMREM environment"
```

---

### Task 12: Wasser

**Files:** Create `ui/js/desktop/apps/minigolf-water.js`

- [ ] **Step 1: Modul schreiben**

```js
import * as THREE from 'three';

const OPEN_VERT = `varying vec3 vGolfWorld;
#include <fog_pars_vertex>
void main() {
    vec4 world = modelMatrix * vec4(position, 1.0);
    vGolfWorld = world.xyz;
    vec4 mvPosition = viewMatrix * world;
    gl_Position = projectionMatrix * mvPosition;
    #include <fog_vertex>
}`;
const OPEN_FRAG = `uniform vec3 shallow; uniform vec3 deep; uniform vec3 skyColor; uniform float inner; uniform float outer; uniform float glints;
uniform float golfTime; uniform float golfMotion; uniform float golfWaterMotion;
varying vec3 vGolfWorld;
#include <fog_pars_fragment>
void main() {
    float t = golfTime * golfMotion * golfWaterMotion;
    float depth = smoothstep(inner, outer, length(vGolfWorld.xz));
    vec3 color = mix(shallow, deep, depth);
    float wave = sin(vGolfWorld.x * 1.3 + t * 0.9) * sin(vGolfWorld.z * 1.1 - t * 0.7) + 0.5 * sin((vGolfWorld.x + vGolfWorld.z) * 2.7 + t * 1.6);
    color *= 0.95 + 0.05 * wave;
    vec3 view = normalize(cameraPosition - vGolfWorld);
    color = mix(color, skyColor, pow(1.0 - clamp(view.y, 0.0, 1.0), 3.0) * 0.45);
    color += vec3(1.0) * glints * pow(max(0.0, wave - 0.95), 4.0) * 3.0 * (1.0 - depth * 0.6);
    gl_FragColor = vec4(color, 1.0);
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
    #include <fog_fragment>
}`;
const SHORE_VERT = `varying vec2 vGolfUv; varying vec3 vGolfWorld;
#include <fog_pars_vertex>
void main() {
    vGolfUv = uv;
    vec4 world = modelMatrix * vec4(position, 1.0);
    vGolfWorld = world.xyz;
    vec4 mvPosition = viewMatrix * world;
    gl_Position = projectionMatrix * mvPosition;
    #include <fog_vertex>
}`;
const SHORE_FRAG = `uniform vec3 shallow; uniform vec3 foam; uniform float golfTime; uniform float golfMotion; uniform float golfWaterMotion;
varying vec2 vGolfUv; varying vec3 vGolfWorld;
#include <fog_pars_fragment>
void main() {
    float t = golfTime * golfMotion * golfWaterMotion, u = vGolfUv.x;
    float edge = 0.16 + 0.05 * sin(t * 1.4 + vGolfWorld.x * 2.1 + vGolfWorld.z * 1.7);
    float band = 1.0 - smoothstep(edge, edge + 0.07, u);
    float ring = 1.0 - smoothstep(0.0, 0.035, abs(u - (0.45 + 0.08 * sin(t * 0.8 + vGolfWorld.x * 0.7))));
    float amount = clamp(band + ring * 0.55, 0.0, 1.0);
    gl_FragColor = vec4(mix(shallow * 1.06, foam, amount), (1.0 - smoothstep(0.55, 1.0, u)) * mix(0.55, 0.95, amount));
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
    #include <fog_fragment>
}`;

// Open water disc plus the course's shore strip; the moon look has no water.
export function createWater(look, cfg, state) {
    const group = new THREE.Group();
    group.name = 'aurago-water';
    const water = look.water;
    if (!water || water.mode === 'void') return { group, attachShore() {}, setTier() {}, dispose() { group.removeFromParent(); } };
    const uniforms = THREE.UniformsUtils.merge([THREE.UniformsLib.fog, {
        shallow: { value: new THREE.Color(water.shallow) }, deep: { value: new THREE.Color(water.deep) }, foam: { value: new THREE.Color(water.foam) },
        skyColor: { value: new THREE.Color(look.sky.horizon) }, inner: { value: 4.8 }, outer: { value: 24 }, glints: { value: cfg.glints ? 1 : 0 },
        golfWaterMotion: { value: cfg.waterMotion ? 1 : 0 }
    }]);
    uniforms.golfTime = state.golfTime;
    uniforms.golfMotion = state.golfMotion;
    const material = new THREE.ShaderMaterial({ uniforms, vertexShader: OPEN_VERT, fragmentShader: OPEN_FRAG, fog: true });
    const disc = new THREE.Mesh(new THREE.CircleGeometry(60, 72).rotateX(-Math.PI / 2), material);
    disc.name = 'water-open'; disc.position.y = water.level; disc.renderOrder = -1;
    group.add(disc);
    const shores = [];
    return {
        group,
        attachShore(node) {
            const shore = new THREE.ShaderMaterial({ uniforms, vertexShader: SHORE_VERT, fragmentShader: SHORE_FRAG, transparent: true, depthWrite: false, fog: true });
            node.material?.dispose?.(); node.material = shore; node.renderOrder = 1; node.castShadow = false; node.receiveShadow = false;
            shores.push(shore);
        },
        setTier(next) { uniforms.glints.value = next.glints ? 1 : 0; uniforms.golfWaterMotion.value = next.waterMotion ? 1 : 0; },
        dispose() { group.removeFromParent(); disc.geometry.dispose(); material.dispose(); for (const shore of shores) shore.dispose(); }
    };
}
```

- [ ] **Step 2: Syntax prüfen und committen**

```bash
node --check ui/js/desktop/apps/minigolf-water.js
git add ui/js/desktop/apps/minigolf-water.js
git commit -m "feat(minigolf): lagoon water with animated shore foam"
```

---

### Task 13: Requisiten-Instancing und Material-Vertrag

**Files:** Create `ui/js/desktop/apps/minigolf-props.js`

- [ ] **Step 1: Modul schreiben**

```js
import * as THREE from 'three';
import { placementMatrix } from './minigolf-looks.js';

// Time and a motion switch shared by every patched material of one scene.
export function createShaderState() {
    return { golfTime: { value: 0 }, golfMotion: { value: 1 } };
}

// Applies the kit material contract (glTF extras -> userData): emission tinted by
// the vertex colour, optional flame flicker, foliage sway, no transmission pass.
export function patchMaterial(material, state, height = 1) {
    const data = material?.userData || {};
    const kind = data.aurago_kind;
    if (!kind) return material;
    if (material.transmission > 0) { material.transmission = 0; material.transparent = true; material.opacity = .55; material.depthWrite = false; }
    const glow = kind === 'emissive' || kind === 'glow';
    const sway = kind === 'foliage' || kind === 'cloth' ? Number(data.aurago_sway) || 0 : 0;
    const flicker = glow ? Math.max(0, Math.min(1, Number(data.aurago_flicker) || 0)) : 0;
    if (glow) material.emissive.setRGB(1, 1, 1);
    if (!glow && !sway) return material;
    const uniforms = { golfTime: state.golfTime, golfMotion: state.golfMotion, golfSway: { value: sway }, golfHeight: { value: Math.max(.1, Number(height) || 1) }, golfFlicker: { value: flicker } };
    material.onBeforeCompile = shader => {
        Object.assign(shader.uniforms, uniforms);
        shader.vertexShader = shader.vertexShader
            .replace('#include <common>', '#include <common>\nuniform float golfTime;\nuniform float golfMotion;\nuniform float golfSway;\nuniform float golfHeight;\nvarying float vGolfPhase;')
            .replace('#include <begin_vertex>', `#include <begin_vertex>
#ifdef USE_INSTANCING
vGolfPhase = instanceMatrix[3].x * 0.9 + instanceMatrix[3].z * 1.3;
#else
vGolfPhase = 0.0;
#endif
float golfLift = clamp(transformed.y / golfHeight, 0.0, 1.0);
golfLift = golfLift * golfLift * golfSway * golfMotion;
transformed.x += golfLift * sin(golfTime * 1.6 + vGolfPhase);
transformed.z += golfLift * 0.6 * sin(golfTime * 1.27 + vGolfPhase * 1.4);`);
        shader.fragmentShader = shader.fragmentShader
            .replace('#include <common>', '#include <common>\nuniform float golfTime;\nuniform float golfMotion;\nuniform float golfFlicker;\nvarying float vGolfPhase;')
            .replace('#include <emissivemap_fragment>', `#include <emissivemap_fragment>
#ifdef USE_COLOR
totalEmissiveRadiance *= vColor.rgb;
#endif
totalEmissiveRadiance *= 1.0 - golfFlicker * golfMotion * (0.5 + 0.5 * sin(golfTime * 11.0 + vGolfPhase) * sin(golfTime * 6.7 + vGolfPhase * 1.9));`);
    };
    material.customProgramCacheKey = () => `auragolf-kit-${glow ? 'g' : ''}${sway ? 's' : ''}`;
    return material;
}

// One InstancedMesh per prop primitive. Geometry stays owned by the cached kit;
// dispose() releases only the instance buffers and the cloned materials.
export function buildProps(kit, look, placements, { scatter = true, state }) {
    const group = new THREE.Group();
    group.name = 'aurago-props';
    const byName = new Map();
    for (const entry of placements || []) {
        if (!Array.isArray(entry) || typeof entry[0] !== 'string') continue;
        if (!byName.has(entry[0])) byName.set(entry[0], []);
        byName.get(entry[0]).push(entry);
    }
    const spinners = [], materials = [], matrix = new THREE.Matrix4(), turn = new THREE.Matrix4(), scratch = new Array(16);
    let instances = 0;
    kit.updateMatrixWorld(true);
    for (const [name, entries] of byName) {
        const node = kit.getObjectByName(`prop-${look}-${name}`);
        if (!node) continue;
        const info = node.userData?.aurago_prop || {};
        const tags = Array.isArray(info.tags) ? info.tags : [];
        if (!scatter && tags.includes('scatter')) continue;
        node.traverse(mesh => {
            if (!mesh.isMesh) return;
            const material = patchMaterial(mesh.material.clone(), state, info.height);
            materials.push(material);
            const base = mesh.matrixWorld.clone();
            const inst = new THREE.InstancedMesh(mesh.geometry, material, entries.length);
            entries.forEach((entry, index) => inst.setMatrixAt(index, matrix.fromArray(placementMatrix(entry, scratch)).multiply(base)));
            inst.name = `props-${name}`; inst.castShadow = !tags.includes('flat'); inst.receiveShadow = true;
            inst.computeBoundingSphere();
            group.add(inst);
            if (info.spin) spinners.push({ inst, entries, base, axis: new THREE.Vector3().fromArray(info.spin.axis || [0, 1, 0]).normalize(), speed: Number(info.spin.speed) || .3 });
        });
        instances += entries.length;
    }
    return {
        group, instances, scatter,
        update(elapsed) {
            for (const spinner of spinners) {
                turn.makeRotationAxis(spinner.axis, elapsed * spinner.speed);
                spinner.entries.forEach((entry, index) => spinner.inst.setMatrixAt(index, matrix.fromArray(placementMatrix(entry, scratch)).multiply(turn).multiply(spinner.base)));
                spinner.inst.instanceMatrix.needsUpdate = true;
            }
        },
        dispose() { group.removeFromParent(); for (const child of group.children) child.dispose(); for (const material of materials) material.dispose(); }
    };
}
```

- [ ] **Step 2: Syntax prüfen und committen**

```bash
node --check ui/js/desktop/apps/minigolf-props.js
git add ui/js/desktop/apps/minigolf-props.js
git commit -m "feat(minigolf): instanced world props and the kit material contract"
```

---

### Task 14: Effekte

**Files:** Create `ui/js/desktop/apps/minigolf-fx.js`

- [ ] **Step 1: Modul schreiben**

```js
import * as THREE from 'three';
import { TIERS } from './minigolf-looks.js';

const POINT_VERT = `attribute float golfSize; attribute float golfAlpha; attribute vec3 golfColor; uniform float golfScale;
varying vec3 vGolfColor; varying float vGolfAlpha;
void main() {
    vGolfColor = golfColor; vGolfAlpha = golfAlpha;
    vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
    gl_PointSize = golfSize * golfScale / max(0.1, -mvPosition.z);
    gl_Position = projectionMatrix * mvPosition;
}`;
const POINT_FRAG = `varying vec3 vGolfColor; varying float vGolfAlpha;
void main() {
    vec2 c = gl_PointCoord - 0.5; float d = dot(c, c);
    if (d > 0.25) discard;
    gl_FragColor = vec4(vGolfColor, vGolfAlpha * smoothstep(0.25, 0.04, d));
    #include <colorspace_fragment>
}`;
const TRAIL_POINTS = 28;
const tint = new THREE.Color();
const between = (a, b) => a + Math.random() * (b - a);

function createPool(max, blending, scale) {
    const geometry = new THREE.BufferGeometry();
    const attribute = (size) => new THREE.BufferAttribute(new Float32Array(max * size), size).setUsage(THREE.DynamicDrawUsage);
    const position = attribute(3), color = attribute(3), size = attribute(1), alpha = attribute(1);
    geometry.setAttribute('position', position); geometry.setAttribute('golfColor', color); geometry.setAttribute('golfSize', size); geometry.setAttribute('golfAlpha', alpha);
    geometry.setDrawRange(0, 0);
    const material = new THREE.ShaderMaterial({ uniforms: { golfScale: scale }, vertexShader: POINT_VERT, fragmentShader: POINT_FRAG, transparent: true, depthWrite: false, blending });
    const points = new THREE.Points(geometry, material);
    points.frustumCulled = false; points.renderOrder = 4;
    const velocity = new Float32Array(max * 3), life = new Float32Array(max), span = new Float32Array(max), base = new Float32Array(max);
    const grow = new Float32Array(max), gravity = new Float32Array(max), drag = new Float32Array(max), opacity = new Float32Array(max);
    let count = 0, limit = max;
    function move(from, to) {
        position.setXYZ(to, position.getX(from), position.getY(from), position.getZ(from));
        color.setXYZ(to, color.getX(from), color.getY(from), color.getZ(from));
        for (let a = 0; a < 3; a++) velocity[to * 3 + a] = velocity[from * 3 + a];
        life[to] = life[from]; span[to] = span[from]; base[to] = base[from]; grow[to] = grow[from]; gravity[to] = gravity[from]; drag[to] = drag[from]; opacity[to] = opacity[from];
    }
    return {
        points,
        get count() { return count; },
        setLimit(value) { limit = Math.max(0, Math.min(max, value)); count = Math.min(count, limit); },
        clear() { count = 0; geometry.setDrawRange(0, 0); },
        emit(x, y, z, vx, vy, vz, hex, sizeValue, lifeValue, g = 0, d = 0, growValue = 0, opacityValue = 1) {
            if (count >= limit) return false;
            const i = count++;
            tint.set(hex);
            position.setXYZ(i, x, y, z); color.setXYZ(i, tint.r, tint.g, tint.b);
            velocity[i * 3] = vx; velocity[i * 3 + 1] = vy; velocity[i * 3 + 2] = vz;
            base[i] = sizeValue; life[i] = span[i] = lifeValue; gravity[i] = g; drag[i] = d; grow[i] = growValue; opacity[i] = opacityValue;
            return true;
        },
        update(dt) {
            for (let i = 0; i < count; i++) {
                life[i] -= dt;
                if (life[i] <= 0) { move(--count, i); i--; continue; }
                const keep = Math.max(0, 1 - drag[i] * dt);
                velocity[i * 3] *= keep; velocity[i * 3 + 1] = velocity[i * 3 + 1] * keep - gravity[i] * dt; velocity[i * 3 + 2] *= keep;
                position.setXYZ(i, position.getX(i) + velocity[i * 3] * dt, position.getY(i) + velocity[i * 3 + 1] * dt, position.getZ(i) + velocity[i * 3 + 2] * dt);
                const t = life[i] / span[i];
                size.setX(i, base[i] * (1 + grow[i] * (1 - t)));
                alpha.setX(i, opacity[i] * Math.min(1, t * 3));
            }
            geometry.setDrawRange(0, count);
            position.needsUpdate = true; color.needsUpdate = true; size.needsUpdate = true; alpha.needsUpdate = true;
        },
        dispose() { geometry.dispose(); material.dispose(); }
    };
}

function createConfetti(max) {
    const mesh = new THREE.InstancedMesh(new THREE.PlaneGeometry(.075, .045), new THREE.MeshBasicMaterial({ side: THREE.DoubleSide, transparent: true, depthWrite: false }), max);
    mesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage); mesh.frustumCulled = false; mesh.count = 0; mesh.renderOrder = 4;
    mesh.setColorAt(0, tint.set(0xffffff));
    const pos = new Float32Array(max * 3), vel = new Float32Array(max * 3), axis = new Float32Array(max * 3), angle = new Float32Array(max), spin = new Float32Array(max), life = new Float32Array(max), phase = new Float32Array(max);
    const colors = new Array(max).fill(0xffffff), matrix = new THREE.Matrix4(), quaternion = new THREE.Quaternion(), vector = new THREE.Vector3(), scale = new THREE.Vector3(), turn = new THREE.Vector3();
    let count = 0, limit = max;
    return {
        mesh,
        get count() { return count; },
        setLimit(value) { limit = Math.max(0, Math.min(max, value)); count = Math.min(count, limit); },
        clear() { count = 0; mesh.count = 0; },
        emit(x, y, z, vx, vy, vz, hex) {
            if (count >= limit) return;
            const i = count++;
            pos.set([x, y, z], i * 3); vel.set([vx, vy, vz], i * 3);
            turn.set(between(-1, 1), between(-1, 1), between(-1, 1)).normalize(); axis.set([turn.x, turn.y, turn.z], i * 3);
            angle[i] = between(0, Math.PI * 2); spin[i] = between(4, 11); life[i] = between(2.2, 3.2); phase[i] = between(0, 6.28); colors[i] = hex;
        },
        update(dt, elapsed) {
            for (let i = 0; i < count; i++) {
                life[i] -= dt;
                if (life[i] <= 0) {
                    const last = --count;
                    for (let a = 0; a < 3; a++) { pos[i * 3 + a] = pos[last * 3 + a]; vel[i * 3 + a] = vel[last * 3 + a]; axis[i * 3 + a] = axis[last * 3 + a]; }
                    angle[i] = angle[last]; spin[i] = spin[last]; life[i] = life[last]; phase[i] = phase[last]; colors[i] = colors[last]; i--; continue;
                }
                const keep = Math.max(0, 1 - 1.6 * dt);
                vel[i * 3] = vel[i * 3] * keep + Math.sin(elapsed * 7 + phase[i]) * .6 * dt;
                vel[i * 3 + 1] = vel[i * 3 + 1] * keep - 2.4 * dt;
                vel[i * 3 + 2] = vel[i * 3 + 2] * keep + Math.cos(elapsed * 6 + phase[i]) * .6 * dt;
                for (let a = 0; a < 3; a++) pos[i * 3 + a] += vel[i * 3 + a] * dt;
                angle[i] += spin[i] * dt;
                quaternion.setFromAxisAngle(vector.fromArray(axis, i * 3), angle[i]);
                const fade = Math.min(1, life[i] * 2);
                matrix.compose(vector.fromArray(pos, i * 3), quaternion, scale.setScalar(fade));
                mesh.setMatrixAt(i, matrix); mesh.setColorAt(i, tint.set(colors[i]));
            }
            mesh.count = count;
            mesh.instanceMatrix.needsUpdate = true;
            if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
        },
        dispose() { mesh.geometry.dispose(); mesh.material.dispose(); mesh.dispose(); }
    };
}

function createRing() {
    const mesh = new THREE.Mesh(new THREE.RingGeometry(.86, 1, 64).rotateX(-Math.PI / 2), new THREE.MeshBasicMaterial({ transparent: true, opacity: 0, depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide }));
    mesh.visible = false; mesh.renderOrder = 3;
    let time = 0, duration = 0, size = 1, still = false;
    return {
        mesh,
        play(x, y, z, hex, reduced, maxSize = 1.6, length = .8) {
            mesh.position.set(x, y, z); mesh.material.color.set(hex); mesh.visible = true;
            time = 0; duration = reduced ? 1 : length; size = maxSize; still = reduced;
        },
        update(dt) {
            if (!mesh.visible) return;
            time += dt;
            const t = Math.min(1, time / duration);
            mesh.scale.setScalar(still ? size * .6 : Math.max(.01, size * t));
            mesh.material.opacity = still ? Math.sin(Math.PI * t) * .8 : (1 - t) * .9;
            if (t >= 1) mesh.visible = false;
        },
        dispose() { mesh.geometry.dispose(); mesh.material.dispose(); }
    };
}

function createTrail() {
    const geometry = new THREE.BufferGeometry();
    const position = new THREE.BufferAttribute(new Float32Array(TRAIL_POINTS * 6), 3).setUsage(THREE.DynamicDrawUsage);
    const fade = new THREE.BufferAttribute(new Float32Array(TRAIL_POINTS * 2), 1).setUsage(THREE.DynamicDrawUsage);
    const index = [];
    for (let i = 0; i < TRAIL_POINTS - 1; i++) { const a = i * 2; index.push(a, a + 1, a + 2, a + 1, a + 3, a + 2); }
    geometry.setIndex(index); geometry.setAttribute('position', position); geometry.setAttribute('golfFade', fade); geometry.setDrawRange(0, 0);
    const material = new THREE.ShaderMaterial({
        uniforms: { golfColor: { value: new THREE.Color() } },
        vertexShader: 'attribute float golfFade; varying float vGolfFade; void main(){ vGolfFade = golfFade; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }',
        fragmentShader: 'uniform vec3 golfColor; varying float vGolfFade; void main(){ gl_FragColor = vec4(golfColor * 1.4, vGolfFade * 0.75);\n#include <colorspace_fragment>\n}',
        transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, side: THREE.DoubleSide
    });
    const mesh = new THREE.Mesh(geometry, material);
    mesh.frustumCulled = false; mesh.renderOrder = 3;
    const points = new Float32Array(TRAIL_POINTS * 3);
    let count = 0, shrink = 0;
    return {
        mesh,
        get count() { return count; },
        reset() { count = 0; geometry.setDrawRange(0, 0); },
        update(dt, ball, moving, speed, hex, enabled) {
            if (!enabled) { if (count) this.reset(); return; }
            material.uniforms.golfColor.value.set(hex);
            if (moving && speed > .6) {
                shrink = 0;
                if (!count || Math.hypot(ball.x - points[0], ball.z - points[2]) > .045) {
                    points.copyWithin(3, 0, (TRAIL_POINTS - 1) * 3);
                    points[0] = ball.x; points[1] = ball.y - .1; points[2] = ball.z;
                    count = Math.min(TRAIL_POINTS, count + 1);
                }
            } else {
                shrink += dt;
                while (shrink > 1 / 40 && count) { shrink -= 1 / 40; count--; }
            }
            for (let i = 0; i < count; i++) {
                const a = Math.max(0, i - 1), b = Math.min(count - 1, i + 1);
                const dx = points[a * 3] - points[b * 3], dz = points[a * 3 + 2] - points[b * 3 + 2], length = Math.hypot(dx, dz) || 1;
                const k = 1 - i / TRAIL_POINTS, width = .065 * k, px = -dz / length * width, pz = dx / length * width;
                position.setXYZ(i * 2, points[i * 3] + px, points[i * 3 + 1], points[i * 3 + 2] + pz);
                position.setXYZ(i * 2 + 1, points[i * 3] - px, points[i * 3 + 1], points[i * 3 + 2] - pz);
                fade.setX(i * 2, k * k); fade.setX(i * 2 + 1, k * k);
            }
            geometry.setDrawRange(0, Math.max(0, (count - 1) * 6));
            position.needsUpdate = true; fade.needsUpdate = true;
        },
        dispose() { geometry.dispose(); material.dispose(); }
    };
}

// Particles, trail, cup celebration and win fireworks for one scene.
export function createFx(parent, initialLook) {
    let look = initialLook, cfg = TIERS.medium, reduced = false, flagBoost = 0, ambientAt = 0, sandAt = 0, elapsed = 0;
    const rockets = [], scale = { value: 300 };
    const soft = createPool(320, THREE.NormalBlending, scale), glow = createPool(320, THREE.AdditiveBlending, scale);
    const confetti = createConfetti(160), ring = createRing(), ripple = createRing(), trail = createTrail();
    parent.add(soft.points, glow.points, confetti.mesh, ring.mesh, ripple.mesh, trail.mesh);
    function spray(pool, count, x, y, z, hex, o) {
        for (let i = 0; i < count; i++) {
            const angle = Math.random() * Math.PI * 2, out = between(o.out[0], o.out[1]);
            pool.emit(x, y, z, Math.cos(angle) * out, between(o.up[0], o.up[1]), Math.sin(angle) * out, Array.isArray(hex) ? hex[i % hex.length] : hex,
                between(o.size[0], o.size[1]), between(o.life[0], o.life[1]), o.gravity ?? 0, o.drag ?? 0, o.grow ?? 0, o.opacity ?? 1);
        }
    }
    function confettiBurst(x, y, z, count) {
        for (let i = 0; i < count; i++) {
            const angle = Math.random() * Math.PI * 2, out = between(.6, 2.2);
            confetti.emit(x, y, z, Math.cos(angle) * out, between(2.5, 5), Math.sin(angle) * out, look.fx.confetti[i % look.fx.confetti.length]);
        }
    }
    function burst(type, position, speed = 1, info = {}) {
        if (!position) return;
        const [x, y, z] = position, fx = look.fx, ground = y - .15, space = look.water.mode === 'void';
        if (type === 'cup') flagBoost = 2;
        if (type === 'cup' || type === 'win') ring.play(x, y + .012, z, fx.gold, reduced);
        if (reduced) return;
        if (type === 'strike') {
            if (space) return spray(soft, 10, x, ground, z, fx.sand, { up: [.3, .8], out: [.4, 1.1], size: [.12, .22], life: [.6, 1], gravity: .2, drag: 2.5, grow: 1.2, opacity: .6 });
            if (info.surface === 'sand') return spray(soft, 16, x, ground, z, fx.sand, { up: [.5, 1.2], out: [.5, 1.4], size: [.1, .2], life: [.6, 1.1], gravity: 2, drag: 2, grow: 1, opacity: .75 });
            return spray(soft, 8, x, ground, z, fx.grass, { up: [1, 2], out: [.4, 1], size: [.04, .07], life: [.4, .7], gravity: 7, drag: .5 });
        }
        if (type === 'impact') {
            const count = Math.round(4 + Math.min(1, speed) * 10);
            if (fx.wall === 'sparks') return spray(glow, count, x, y, z, 0xfff1b0, { up: [.6, 2], out: [1.5, 3.5], size: [.035, .06], life: [.25, .5], gravity: 5, drag: 1 });
            if (fx.wall === 'dust') return spray(soft, count, x, y, z, fx.stone, { up: [.1, .5], out: [.3, .8], size: [.1, .18], life: [.5, .9], gravity: -.15, drag: 3, grow: 1, opacity: .55 });
            return spray(soft, count, x, y, z, fx.wood, { up: [.8, 1.8], out: [.6, 1.6], size: [.035, .06], life: [.4, .7], gravity: 7, drag: .6 });
        }
        if (type === 'water') {
            if (space) return spray(soft, 26, x, y, z, fx.splash, { up: [-.2, .4], out: [.3, 1], size: [.16, .3], life: [.8, 1.4], gravity: 0, drag: 1.5, grow: 1.5, opacity: .5 });
            ripple.play(x, look.water.level + .02, z, 0xffffff, false, .9, .6);
            return spray(soft, 26, x, look.water.level + .02, z, fx.splash, { up: [2, 4.5], out: [.4, 1.4], size: [.06, .12], life: [.6, 1], gravity: 9.8, drag: .2 });
        }
        if (type === 'cup') {
            confettiBurst(x, y + .1, z, Math.round(cfg.particles * .45));
            return spray(glow, 24, x, y, z, fx.gold, { up: [1.2, 3], out: [.05, .3], size: [.05, .09], life: [.6, 1.1], gravity: -.3, drag: .4 });
        }
        if (type === 'win') {
            for (const [dx, dz] of [[-1.2, .4], [1.1, -.3], [0, 1.2]]) confettiBurst(x + dx, y + .2, z + dz, Math.round(cfg.particles * .2));
            [0, .35, .7].forEach((delay, i) => rockets.push({ delay, time: 0, x: x + (i - 1) * 1.6, y, z: z - 1 + i * .5, hex: fx.confetti[i % fx.confetti.length], done: false }));
        }
    }
    function ambient(dt, ctx) {
        ambientAt += dt;
        if (ambientAt < .25) return;
        ambientAt = 0;
        const angle = Math.random() * Math.PI * 2, fx = look.fx;
        if (fx.ambient === 'glint') {
            const r = between(5.4, 10);
            glow.emit(Math.cos(angle) * r, ctx.waterLevel + .05, Math.sin(angle) * r, 0, 0, 0, fx.ambientColor, .06, .6);
        } else if (fx.ambient === 'fireflies') {
            const r = between(0, 4.5);
            glow.emit(Math.cos(angle) * r, ctx.groundY + between(.3, 1.8), Math.sin(angle) * r, between(-.1, .1), between(-.05, .1), between(-.1, .1), fx.ambientColor, .05, 2.5);
        } else {
            const r = between(0, 6);
            soft.emit(Math.cos(angle) * r, ctx.groundY + between(-.5, 2), Math.sin(angle) * r, between(-.05, .05), between(0, .05), between(-.05, .05), fx.ambientColor, .12, 3, 0, 0, 0, .25);
        }
    }
    return {
        burst,
        get flagBoost() { return flagBoost; },
        setLook(next) { look = next; this.clear(); },
        setTier(next) { cfg = next; soft.setLimit(next.particles); glow.setLimit(next.particles); confetti.setLimit(Math.round(next.particles / 2)); },
        setReduced(value) { reduced = !!value; if (reduced) { soft.clear(); glow.clear(); confetti.clear(); rockets.length = 0; } },
        setScale(height, fovDegrees) { scale.value = height / (2 * Math.tan(fovDegrees * Math.PI / 360)); },
        clear() { soft.clear(); glow.clear(); confetti.clear(); trail.reset(); rockets.length = 0; flagBoost = 0; ring.mesh.visible = false; ripple.mesh.visible = false; },
        // ctx: { ball: Vector3, moving, speed, color, surface, waterLevel, groundY }
        update(dt, ctx) {
            elapsed += dt;
            flagBoost = Math.max(0, flagBoost - dt);
            if (!reduced && cfg.motion) {
                ambient(dt, ctx);
                if (ctx.moving && ctx.surface === 'sand' && ctx.speed > .8 && (sandAt += dt) > 1 / 30) {
                    sandAt = 0;
                    soft.emit(ctx.ball.x, ctx.ball.y - .12, ctx.ball.z, between(-.2, .2), between(.2, .5), between(-.2, .2), look.fx.sand, between(.08, .14), between(.4, .7), 1, 2, 1, .6);
                }
            }
            for (const rocket of rockets) {
                if (rocket.done || (rocket.time += dt) < rocket.delay) continue;
                const t = rocket.time - rocket.delay;
                if (t < .7) glow.emit(rocket.x, rocket.y + 3 * t / .7, rocket.z, 0, 0, 0, 0xfff1d0, .06, .25);
                else {
                    rocket.done = true;
                    for (let i = 0; i < 50; i++) {
                        const u = Math.random() * 2 - 1, theta = Math.random() * Math.PI * 2, r = Math.sqrt(1 - u * u), speed = between(2, 3.5);
                        glow.emit(rocket.x, rocket.y + 3, rocket.z, Math.cos(theta) * r * speed, u * speed, Math.sin(theta) * r * speed, rocket.hex, between(.05, .08), between(1.2, 1.6), 1.5, .6);
                    }
                }
            }
            for (let i = rockets.length - 1; i >= 0; i--) if (rockets[i].done) rockets.splice(i, 1);
            soft.update(dt); glow.update(dt); confetti.update(dt, elapsed); ring.update(dt); ripple.update(dt);
            trail.update(dt, ctx.ball, ctx.moving, ctx.speed, ctx.color, cfg.trail);
        },
        stats() { return { particles: soft.count + glow.count + confetti.count, trail: trail.count }; },
        dispose() { soft.dispose(); glow.dispose(); confetti.dispose(); ring.dispose(); ripple.dispose(); trail.dispose(); for (const item of [soft.points, glow.points, confetti.mesh, ring.mesh, ripple.mesh, trail.mesh]) item.removeFromParent(); }
    };
}
```

- [ ] **Step 2: Syntax prüfen und committen**

```bash
node --check ui/js/desktop/apps/minigolf-fx.js
git add ui/js/desktop/apps/minigolf-fx.js
git commit -m "feat(minigolf): particle pools, ball trail, cup celebration and fireworks"
```

---

### Task 15: Szene einbinden, Kamerafahrt, Spiel-Aufrufe, Bundle

**Files:**
- Modify: `ui/js/desktop/apps/minigolf-scene.js`, `ui/js/desktop/apps/minigolf-game.js`
- Rebuild: `ui/js/vendor/minigolf/game.esm.js`, `ui/js/vendor/minigolf/manifest.json`

- [ ] **Step 1: Impact-Analyse**

```bash
node reports/memory-correction-index/run.cjs impact "createScene" --direction upstream --repo AuraGo
node reports/memory-correction-index/run.cjs impact "mountGame" --direction upstream --repo AuraGo
```

Fehlt das Werkzeug, `graft callers createScene` verwenden. Erwartete Aufrufer:
- `createScene` ← `minigolf-game.js`;
- `mountGame` ← `minigolf.js` und die Browser-Fixture.

- [ ] **Step 2: `minigolf-scene.js` umbauen**
  - **Imports ergänzen:**

```js
import { LOOKS, lookOf, tierConfig } from './minigolf-looks.js';
import { createIntro } from './minigolf-camera.js';
import { createSky, createEnvironment } from './minigolf-sky.js';
import { createWater } from './minigolf-water.js';
import { buildProps, createShaderState, patchMaterial } from './minigolf-props.js';
import { createFx } from './minigolf-fx.js';
```

  - **Entfernen** (ersetzt durch die Module):
    - die alte `LOOKS`-Tabelle, `buildSky`, `addWaterFoam`;
    - das `HemisphereLight` `ambient`;
    - `effectGeometry`/`effectMaterial`/`effects`/`particleData`/`dummy`/`effectColor`;
    - `waterTime`, `waterNodes`, `trailAt`, `update.lastAmbient`;
    - die alte Partikelschleife und die Wasser-Shader-Patch-Schleife in `load`.
  - **Renderer:** `renderer.toneMapping = THREE.NeutralToneMapping; renderer.toneMappingExposure = 1; renderer.info.autoReset = false;`
  - **Sonne:** Schattenkamera `{ left: -9, right: 9, top: 9, bottom: -9, near: .1, far: 60 }`.
  - **Gegenlicht:** `const rim = new THREE.DirectionalLight(0xffffff, 0); scene.add(rim);`.
  - **Zustand:**

```js
const shaderState = createShaderState();
const fx = createFx(scene, LOOKS.pirate);
const kits = new Map(), environments = new Map();
let lookId = 'pirate', look = LOOKS.pirate, sky = null, water = null, props = null;
let intro = null, introTime = 0, pendingTurn = null;
const introPosition = [0, 0, 0], introTarget = [0, 0, 0];
const cfg = () => tierConfig(tier());
function kitFor(id) {
    if (!kits.has(id)) kits.set(id, fetch(options.assetURL(`/3d/minigolf/props-${id}.glb`))
        .then(response => { if (!response.ok) throw Error('kit'); return response.arrayBuffer(); })
        .then(bytes => new GLTFLoader().parseAsync(bytes, ''))
        .then(gltf => gltf.scene)
        .catch(() => { kits.delete(id); return null; }));
    return kits.get(id);
}
```

  - **`applyTier()`** ergänzen (am Ende, vor `resize()`):

```js
const next = cfg();
sky?.setTier(next); water?.setTier(next); fx.setTier(next);
shaderState.golfMotion.value = next.motion && !reduced ? 1 : 0;
if (props && course && props.scatter !== next.scatter) rebuildProps();
```

    mit

```js
async function rebuildProps() {
    const kit = course?.props?.length ? await kitFor(lookId) : null;
    props?.dispose(); props = null;
    if (kit && !disposed) { props = buildProps(kit, lookId, course.props, { scatter: cfg().scatter, state: shaderState }); scene.add(props.group); }
}
```

  - **`resize()`** ergänzen: `fx.setScale(height * dpr, camera.fov);`.
  - **`applyLook()`:**

```js
function applyLook() {
    scene.background = new THREE.Color(look.background);
    scene.fog = look.fog ? new THREE.FogExp2(look.fog.color, look.fog.density) : null;
    renderer.toneMappingExposure = look.exposure;
    sun.color.set(look.sun.color); sun.intensity = look.sun.intensity;
    sun.position.fromArray(look.sun.direction).normalize().multiplyScalar(22); sun.target.position.set(0, 0, 0);
    rim.intensity = look.rim?.intensity ?? 0;
    if (look.rim) { rim.color.set(look.rim.color); rim.position.fromArray(look.rim.direction).normalize().multiplyScalar(20); }
    if (!environments.has(lookId)) environments.set(lookId, createEnvironment(renderer, look));
    scene.environment = environments.get(lookId).texture;
    scene.environmentIntensity = look.environment;
    bloom.strength = look.bloom.strength; bloom.radius = look.bloom.radius; bloom.threshold = look.bloom.threshold;
    fx.setLook(look);
}
```

  - **`load(next)`:**
    - Kit und Bahn parallel laden:

```js
const nextLook = lookOf(next);
const kitPromise = next.props?.length ? kitFor(nextLook) : Promise.resolve(null);
// … bestehender fetch + arrayBuffer der Bahn …
const [gltf, kit] = await Promise.all([new GLTFLoader().parseAsync(bytes, ''), kitPromise]);
if (disposed || generation !== version) { disposeTree(gltf.scene); return false; }
```

    - Beim Aufräumen zusätzlich `props?.dispose(); props = null; sky?.dispose(); water?.dispose();`.
    - Danach:

```js
lookId = nextLook; look = LOOKS[lookId];
applyLook();
sky = createSky(look, cfg(), shaderState); scene.add(sky.group);
water = createWater(look, cfg(), shaderState); scene.add(water.group);
```

    - In der `courseRoot.traverse`-Schleife:

```js
if (node.isMesh) { node.castShadow = true; node.receiveShadow = true; node.material = patchMaterial(node.material, shaderState); }
if (node.userData?.aurago_hidden) node.visible = false;
if (node.name === 'water-shore' && node.isMesh) water.attachShore(node);
if (node.name === 'flag' || node.name.startsWith('flag-cloth')) flagNodes.push({ node, z: node.rotation.z });
```

    - Nach Vorlagen und Putter:

```js
props = kit ? buildProps(kit, lookId, next.props, { scatter: cfg().scatter, state: shaderState }) : null;
if (props) scene.add(props.group);
fx.clear();
setBall(next.tee); focusBall(true); resize();
startIntro();
return true;
```

  - **Kamerafahrt:**

```js
function overviewPose() {
    const target = new THREE.Vector3(...course.cup).add(new THREE.Vector3(...course.tee)).multiplyScalar(.5);
    return { position: target.clone().add(new THREE.Vector3(10, 13, 14)).toArray(), target: target.toArray() };
}
function startIntro() {
    intro = null; introTime = 0; pendingTurn = null;
    if (reduced || !course?.path) return;
    const end = overviewPose();
    intro = createIntro(course.path, end.position, end.target);
}
function finishIntro() {
    intro = null;
    const turn = pendingTurn; pendingTurn = null;
    if (turn) frameTurn(...turn);
}
function skipIntro() {
    if (!intro) return false;
    intro = null;
    focusBall(true);
    finishIntro();
    return true;
}
```

    Die Kamerafahrt greift an vorhandenen Stellen ein:
    - `focusBall(overview)`: Als erste Zeile `if (intro) { intro = null; pendingTurn = null; }`. `skipIntro` setzt `intro` vorher auf `null` und behält so `pendingTurn`.
    - `frameTurn(angle, position)`: Als erste Zeile `if (intro) { pendingTurn = [angle, position]; return; }`.
    - `down(event)`: Als erste Zeile `skipIntro();`, danach unverändert.
    - Mausrad: `const wheel = () => { skipIntro(); }; canvas.addEventListener('wheel', wheel, { passive: true });`, beim Aufräumen wieder entfernen.
    - Controls-Listener `'start'`: `userOrbiting = true; camTween = null; skipIntro();`.
    - `setReducedMotion(value)`: zusätzlich `fx.setReduced(reduced); shaderState.golfMotion.value = cfg().motion && !reduced ? 1 : 0; if (reduced) skipIntro();`.
  - **`burst`:**

```js
function burst(type, position, speed = 1, info = {}) {
    if (type === 'strike') swing = .25 + Math.min(1, speed) * .6;
    fx.burst(type, position, speed, info);
}
```

  - **`update(dt, snapshot)`:** Den Abschnitt `if (!reduced) { … }` bis einschließlich der Partikelschleife ersetzen durch:

```js
const motion = !reduced && cfg().motion;
if (motion) shaderState.golfTime.value += dt;
if (!reduced) {
    if (windmill) windmill.rotation.z += dt * .35;
    for (const flag of flagNodes) flag.node.rotation.z = flag.z + Math.sin(elapsed * (2.2 + fx.flagBoost * 3)) * (.045 + fx.flagBoost * .12);
}
if (motion) props?.update(elapsed);
const velocity = snapshot?.velocity || [0, 0, 0];
fx.update(dt, { ball: ball.position, moving: !!snapshot?.moving, speed: Math.hypot(velocity[0], velocity[1], velocity[2]), color: colorValue(snapshot?.color), surface: snapshot?.surface, waterLevel: course?.water?.[0]?.max?.[1] ?? -1.15, groundY: (course?.tee?.[1] ?? .16) - .16 });
if (intro && !userOrbiting) {
    introTime += dt;
    const done = intro.sample(introTime, introPosition, introTarget);
    camera.position.fromArray(introPosition); controls.target.fromArray(introTarget);
    if (done) finishIntro();
}
```

  - **Vor dem Rendern** `renderer.info.reset();` (wegen `autoReset = false`).
  - **`stats()`** ergänzen: `look: lookId, props: props?.instances || 0, env: !!scene.environment, intro: !!intro, trail: fx.stats().trail`. Außerdem `particles: fx.stats().particles` statt `particleData.length`.
  - **Rückgabeobjekt** um `skipIntro` ergänzen.
  - **`dispose()`:**

```js
props?.dispose(); sky?.dispose(); water?.dispose(); fx.dispose();
for (const target of environments.values()) target.dispose();
for (const pending of kits.values()) pending.then(kit => kit && disposeTree(kit));
canvas.removeEventListener('wheel', wheel);
```

    Das kommt vor `disposeTree(scene)`. Die Requisitengruppe ist dann schon entfernt, sodass die Kit-Geometrien nur einmal freigegeben werden.

- [ ] **Step 3: Vier Aufrufe in `minigolf-game.js`**

1. In `key(event)` direkt nach `if (context.isActive?.() === false || event.ctrlKey || event.metaKey || event.altKey) return;` einfügen:

```js
        if (event.type === 'keydown') scene?.skipIntro?.();
```

2. `scene.burst('strike', ballPosition, amount * 2);` → `scene.burst('strike', ballPosition, amount * 2, { surface: lastSurface });`
3. `scene.burst('impact', event.position, event.speed * .25);` → `scene.burst('impact', event.position, event.speed * .25, { surface: lastSurface });`
4. Im Snapshot in `frame()`: `color: …` ergänzen um `, surface: lastSurface`.

- [ ] **Step 4: Prüfen, bauen, Node-Tests**

```bash
for f in ui/js/desktop/apps/minigolf-*.js; do node --check "$f" || exit 1; done
node scripts/build-minigolf.mjs
npm run test:minigolf
node scripts/build-minigolf.mjs --check
```

Erwartet:
- Bundle < 6 MiB;
- alle Node-Tests grün.

- [ ] **Step 5: Sichtprüfung**

Den Browser-Test mit `AURAGO_BROWSER_ARTIFACT_DIR` laufen lassen (Task 16 Step 3). Er schreibt `minigolf-<welt>.png`. Die drei Bilder ansehen auf:
- Himmel, Wasser, Requisiten;
- Fehlfarben, schwarze Flächen, Z-Fighting am Ufer;
- falsch gedrehte Instanzen.

- [ ] **Step 6: GitNexus-Änderungen prüfen und committen**

```bash
node reports/memory-correction-index/run.cjs detect-changes --scope all --repo AuraGo
git add ui/js/desktop/apps/minigolf-scene.js ui/js/desktop/apps/minigolf-game.js ui/js/vendor/minigolf/game.esm.js ui/js/vendor/minigolf/manifest.json
git commit -m "feat(minigolf): diorama renderer with environment light, intro flight and effects"
```

---

### Task 16: Browser-Smoke und Messung

**Files:** Modify `ui/desktop_minigolf_browser_test.go`

- [ ] **Step 1: Kamerafahrt in bestehenden Abläufen berücksichtigen**

- Nach jedem Warten auf eine geladene Bahn, auf das Kameraprüfungen folgen, die Kamerafahrt beenden. Das betrifft die Ballfokus-Prüfungen um Zeile 218–284.

```go
	skipIntro := `()=>{const c=document.querySelector('.mg-canvas canvas');if(c)c.dispatchEvent(new WheelEvent('wheel',{bubbles:true}));return true}`
	page.MustEval(skipIntro)
	page.MustWait(`()=>!golfFixture.inspect().graphics?.intro`)
```

- Danach den ganzen Test laufen lassen und jede weitere Störung durch die Kamerafahrt auf dieselbe Weise beheben, nie durch längere Timeouts.

- [ ] **Step 2: Neue Prüfungen in der Bahnschleife**

Das headless Chrome dieser Maschine meldet `prefers-reduced-motion: reduce`; dann startet keine Kamerafahrt. Die Kamerafahrt-Prüfung emuliert deshalb vorher normale Bewegung und setzt sie danach zurück:

```go
	proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "no-preference"}}}.Call(page)
	// … Bahn laden, intro===true prüfen, überspringen …
	proto.EmulationSetEmulatedMedia{}.Call(page)
```

Vor der Schleife:

```go
	lookOf := map[string]string{"garden": "temple", "coast": "pirate", "night": "moon"}
	introChecked := false
```

Nach dem Warten auf die geladene Bahn:

```go
		page.Timeout(15*time.Second).MustWait(`look=>{const g=golfFixture.inspect().graphics;return g.look===look&&g.props>0&&g.env===true}`, lookOf[course.World])
		if !introChecked {
			if !page.MustEval(`()=>golfFixture.inspect().graphics.intro===true`).Bool() {
				t.Fatalf("%s: loading a course did not start the intro flight", course.ID)
			}
			page.MustEval(skipIntro)
			page.Timeout(2 * time.Second).MustWait(`()=>golfFixture.inspect().graphics.intro===false`)
			introChecked = true
		}
```

- [ ] **Step 3: Lauf mit Bildern**

```powershell
$env:AURAGO_RUN_BROWSER_SMOKE='1'; $env:AURAGO_BROWSER_ARTIFACT_DIR="$PWD\reports\minigolf\p2-after"
go test -count=1 -timeout 30m ./ui -run 'TestDesktopMinigolf' -v
```

Erwartet:
- PASS;
- drei Weltbilder in `reports/minigolf/p2-after`.

Die Bilder ansehen.

- [ ] **Step 4: Benchmark gegen die Ausgangsmessung**

Gleicher Befehl wie Task 1 Step 2, mit `AURAGO_MINIGOLF_BENCHMARK=1` und Ausgabeordner `reports/minigolf/p2-after`. Je Stufe die FPS (aus den Bildintervallen) mit `p2-baseline/benchmark.txt` vergleichen.

Ist eine Stufe langsamer:
1. Die teuerste Ursache auf dieser Stufe abschalten oder vereinfachen, in dieser Reihenfolge:
   - Wolken-/Silhouettenrauschen,
   - Streudeko,
   - Glanzlichter,
   - Partikelobergrenze.
2. Erneut messen.

Ergebnis (Renderer, FPS vorher/nachher je Stufe) in `reports/minigolf/p2-after/benchmark.txt`.

- [ ] **Step 5: Weitere Wächter**

```powershell
go test -count=1 ./ui -run 'TestShippedWebAssetsLoadNothingExternal|TestDesktopMinigolfRegistrationAndLocalResources|TestDesktopMinigolfLocales|TestUIAvoidsCookieConsentClassPrefix'
npm run build:ui -- --check
```

Erwartet: PASS.

- [ ] **Step 6: Commit**

```bash
git add ui/desktop_minigolf_browser_test.go
git commit -m "test(minigolf): smoke-check world looks, props, environment and the intro flight"
```

---

### Prüfung 2 (reader-agent): Laufzeit

Eine kombinierte Durchsicht der Tasks 10–16 gegen Spec Abschnitt 5.

**Schwerpunkte:**
- Freigabe aller GPU-Ressourcen (Kit-Cache, PMREM, Instanzen, Pools);
- keine Allokationen pro Frame;
- reduzierte Bewegung;
- Stufenumschaltung;
- Kamerafahrt und Turn-Überblendung;
- optionale API-Aufrufe;
- keine externen Ressourcen.

**Ausgabe:** Nur Critical/Important. Ein maker-agent behebt sie in einem Commit (mit neu gebautem Bundle); keine zweite Runde.

---

### Task 17: Weltnamen in 16 Sprachen

**Files:** Modify `ui/lang/desktop/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`

- [ ] **Step 1: Werte ersetzen (Schlüssel bleiben)**

| Sprache | `minigolf.garden` | `minigolf.coast` | `minigolf.night` |
|---|---|---|---|
| cs | Chrám v džungli | Pirátská zátoka | Měsíční stanice |
| da | Jungletemplet | Piratbugten | Månestationen |
| de | Dschungeltempel | Piratenbucht | Mondstation |
| el | Ναός της ζούγκλας | Όρμος των πειρατών | Σεληνιακός σταθμός |
| en | Jungle Temple | Pirate Bay | Moon Station |
| es | Templo de la jungla | Bahía pirata | Estación lunar |
| fr | Temple de la jungle | Baie des pirates | Station lunaire |
| hi | जंगल मंदिर | समुद्री डाकू खाड़ी | चंद्र स्टेशन |
| it | Tempio nella giungla | Baia dei pirati | Stazione lunare |
| ja | ジャングルの神殿 | 海賊の入り江 | 月面基地 |
| nl | Jungletempel | Piratenbaai | Maanstation |
| no | Jungeltempelet | Piratbukta | Månestasjonen |
| pl | Świątynia w dżungli | Zatoka piratów | Stacja księżycowa |
| pt | Templo da selva | Baía pirata | Estação lunar |
| sv | Djungeltemplet | Piratviken | Månstationen |
| zh | 丛林神庙 | 海盗湾 | 月球基地 |

- Ersetzen mit dem Edit-Werkzeug, ohne die Dateien neu zu formatieren.
- Danach `grep -n "Sunlit Garden\|Coastal Breeze\|Lantern Garden\|Sonnengarten\|Küstenbrise\|Lichtergarten" ui/lang/desktop/*.json ui/js/desktop/apps/minigolf-*.js documentation/minigolf.md` ausführen. Treffer, die einen Weltnamen meinen, ebenfalls anpassen.

- [ ] **Step 2: Prüfen und committen**

```bash
for f in ui/lang/desktop/*.json; do python -m json.tool "$f" > /dev/null || exit 1; done
go test -count=1 ./ui -run 'TestDesktopMinigolfLocales|TestTranslations_NoConflictingCrossSectionValues'
git add ui/lang/desktop/*.json
git commit -m "i18n(minigolf): name the three worlds after their new look"
```

---

### Task 18: Dokumentation und DOX (docs-agent)

**Files:**
- Modify: `documentation/minigolf.md`, `assets/minigolf/README.md`, `assets/minigolf/kit/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`

- [ ] **Step 1: Inhalte**
  - `documentation/minigolf.md`:
    - Welten und Look;
    - Kamerafahrt und wie man sie überspringt;
    - Effekte;
    - Qualitätsstufen-Tabelle aus Spec 5.9;
    - Abschnitt „Assets and builds“: Kits, Vorschaubilder, Generator-CLI, Budgets, Benchmark-Ergebnis aus Task 16.
  - `assets/minigolf/README.md`:
    - Module des Generators;
    - Befehle (`generate.py -- --only …`, `--course N`, `--save-blend`);
    - Determinismus-Prüfung;
    - neue Größen-Ausgangswerte;
    - Hinweis, dass die `.blend` nicht mehr versioniert ist.
  - `assets/minigolf/kit/AGENTS.md`: vom Probe-Vertrag zum Generator-Vertrag erweitern:
    - Material-Arten und Extras;
    - `PROP_META`;
    - Kit-Export;
    - Umkleiden;
    - Bitgleichheit von `surface-*`/`rail-*`;
    - Platzierungsregeln;
    - Budgets;
    - Verifikation.
  - `ui/js/desktop/apps/AGENTS.md`, Abschnitt AuraGolf:
    - neue Module und ihr Zuschnitt;
    - `look`/`path`/`props`/`kits`/`worlds`;
    - `aurago_hidden`, `water-shore`;
    - Kit-Cache und Freigabe;
    - `skipIntro` (optional aufrufbar);
    - `stats()`-Felder;
    - Tonemapping Neutral;
    - Verifikation inkl. `scripts/test-minigolf-look.mjs`.
- Root-`AGENTS.md` nicht ändern. Der Index-Eintrag für `assets/minigolf/kit/AGENTS.md` ist eine offene Folgeaufgabe (assume-unchanged im Haupt-Checkout); in der Übergabe nennen.

- [ ] **Step 2: Commit**

```bash
git add -f assets/minigolf/kit/AGENTS.md ui/js/desktop/apps/AGENTS.md
git add documentation/minigolf.md assets/minigolf/README.md
git commit -m "docs(minigolf): P2 worlds, renderer, generator and asset contracts"
```

---

### Task 19: Landen auf local main (Hauptsession)

- [ ] **Step 1: Gesamtprüfung auf dem Branch**

```bash
npm run test:minigolf
node scripts/build-minigolf.mjs --check
npm run build:ui -- --check
go test -count=1 ./ui -run 'TestDesktopMinigolf|TestShippedWebAssetsLoadNothingExternal'
```

Erwartet:
- alles PASS;
- Browser-Smoke aus Task 16 grün auf dem finalen Stand.

- [ ] **Step 2: Rebase auf main**

```bash
git fetch . main:main 2>/dev/null || true
git rebase main
```

- Konflikte in `minigolf-scene.js`/`minigolf-game.js`/Sprachdateien auflösen, dabei fremde Änderungen behalten.
- Nach jedem Rebase das Bundle neu bauen und `npm run test:minigolf` erneut ausführen.

- [ ] **Step 3: Fast-Forward im Haupt-Checkout**

```bash
cd /c/Users/Andi/Documents/repo/AuraGo
git status --short -- $(git -C .claude/worktrees/friendly-napier-a5b89a diff --name-only main..claude/friendly-napier-a5b89a)
git merge --ff-only claude/friendly-napier-a5b89a
```

- Zeigt `git status` dort geänderte Dateien aus unserem Diff (andere Sessions), nicht überschreiben. Warten oder die betroffene Session fragen, dann erneut versuchen.
- Nicht pushen.

- [ ] **Step 4: Übergabe**

Notiz in `C:\Users\Andi\AppData\Local\Claude-Data\claude_pzs8sxrjxfjjc\side-session-notes\local_d2b90f51-2636-41a1-bc30-7dd6d3de0022\p2-look-interfaces.md`:
- gelandeter Commit;
- Manifest-Felder;
- Kit-Vertrag;
- `dress.py`/`export.py` für P3;
- offene Punkte: Root-AGENTS-Index, Bahnnamen in P3, Meshopt als Reserve.

---

## Selbstprüfung

- **Spec-Abdeckung:**

  | Spec | Tasks |
  |---|---|
  | 1 Ergebnis | 6–9, 11–15, 17 |
  | 2 Entscheidungen | Stil 5a–c; Umfang 6–8; Zuordnung 8, 10; Weltnamen 17; Mond 6, 12; Tonemapping 15; Instancing 4, 13; Kontaktschatten 7; Banden 6; Vorschau 9; Kompression 4/8 (Budgets); Blender-Datei 3; Materialien 4 |
  | 3 Generator | 3–9 |
  | 4 Manifest | 4, 8 |
  | 5 Laufzeit | 10–15; Qualitätsstufen 10 + 15; API 15 |
  | 6 Budgets | 8, 16 |
  | 7 Tests | 2, 8, 10, 16 |
  | 8 Risiken | Fingerabdruck 2; Determinismus 8 Step 5; Bildrate 16 Step 4 |
  | 9 Übergabe | 19 Step 4 |

- **Platzhalter:**
  - Die Kunst-Tasks 5a–c beschreiben Requisiten über Maße, Tags, Materialien und Budgets statt Code. Modellierung ist iterative Bildarbeit, ihr Prüfmaß sind Render und Budgets.
  - Task 1 Step 1 hängt vom Befund des debug-agents ab; beide Fälle sind beschrieben.
- **Namenskonsistenz:**
  - `createShaderState`/`patchMaterial`/`buildProps` (13 ↔ 15);
  - `createSky(look, cfg, state, backdrop)`/`createEnvironment(renderer, look)` (11 ↔ 15);
  - `createWater(look, cfg, state)` mit `attachShore`/`setTier` (12 ↔ 15);
  - `createFx(parent, look)` mit `burst`/`update(ctx)`/`setTier`/`setReduced`/`setScale`/`stats`/`flagBoost` (14 ↔ 15);
  - `createIntro(path, endPosition, endTarget).sample(time, position, target)` (10 ↔ 15);
  - `placementMatrix(entry)` (10 ↔ 13);
  - `TIERS`-Felder `scatter`/`motion`/`clouds`/`twinkle`/`waterMotion`/`glints`/`particles`/`trail` (10 ↔ 11–15).

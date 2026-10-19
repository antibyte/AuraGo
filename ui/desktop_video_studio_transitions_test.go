package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/videostudio"
)

// TestVideoStudioShadersCoverTransitions checks the GL transition sources without a browser (plan 2e Task 11; index
// §4.7, §4.24):
//   - every registry transition has a GLSL ES 3.00 source that mixes u_from and u_to; ids outside the registry give null;
//   - every uniform a source declares and reads comes from VideoStudioPicture.transitionUniforms with the declared type
//     (u_from, u_to and u_size come from drawTransition), at several frames and sizes;
//   - the 20 new sources follow the D1 rules: the pixel from vsPixel (rows from the top), texelFetch reads, and u_P
//     instead of u_progress.
func TestVideoStudioShadersCoverTransitions(t *testing.T) {
	runVideoStudioEffectsScript(t, `
require('./js/desktop/apps/video-studio-shaders.js');
const P = window.VideoStudioPicture, S = window.VideoStudioShaders;
const ids = E.transitions().map(spec => spec.id);
assert.equal(ids.length, 24, 'the registry lists 24 transitions');
const OLD = ['dissolve', 'black', 'wipeleft', 'wiperight'];
const drawn = ['u_from', 'u_to', 'u_size'];
const word = name => new RegExp('\\b' + name + '\\b', 'g');
const list = value => (Array.isArray(value) || ArrayBuffer.isView(value) ? Array.from(value) : null);
const fits = (type, value) => {
  if (type === 'float') return typeof value === 'number' && Number.isFinite(value);
  if (type === 'int') return Number.isInteger(value);
  const items = /^vec[234]$/.test(type) ? list(value) : null;
  return !!items && items.length === Number(type.slice(3)) && items.every(Number.isFinite);
};
const fresh = new Set();
ids.forEach(id => {
  const source = S.transition(id);
  assert(typeof source === 'string' && source.startsWith('#version 300 es'), id + ' has a GLSL ES 3.00 source');
  assert(source.includes('uniform sampler2D u_from;') && source.includes('uniform sampler2D u_to;'), id + ' mixes the two track layers');
  const declared = Array.from(source.matchAll(/uniform (\w+) (u_\w+);/g), m => ({type: m[1], name: m[2]}));
  const read = declared.filter(u => !drawn.includes(u.name) && source.match(word(u.name)).length > 1);
  [[0, 20], [5, 20], [19, 20], [3, 7], [6, 7]].forEach(([i, D]) => [{width: 96, height: 54}, {width: 75, height: 43}].forEach(size => {
    const uniforms = P.transitionUniforms(id, i, D, size), label = id + ' at i ' + i + ' of ' + D + ', ' + size.width + 'x' + size.height;
    read.forEach(u => assert(fits(u.type, uniforms[u.name]), label + ': transitionUniforms gives ' + JSON.stringify(uniforms[u.name]) + ' for ' + u.type + ' ' + u.name));
  }));
  if (OLD.includes(id)) return;
  fresh.add(source);
  assert(source.includes('ivec2 p = vsPixel();') && source.includes('texelFetch(u_from') && source.includes('texelFetch(u_to'), id + ' reads texels by index, rows from the top');
  assert.equal(source.match(word('u_progress')).length, 1, id + ' declares u_progress (TRANSITION_HEAD) but never reads it (D1: u_P)');
});
assert.equal(fresh.size, 20, 'the 20 new transitions have 20 distinct sources');
['not-a-transition', 'toString', '__proto__', '', undefined].forEach(id => assert.equal(S.transition(id), null, String(id) + ' has no source'));
console.log('ok');
`)
}

// Generated transition icons (2e, plan index §4.9). Each of the 24 registry transitions gets one 30×18 SVG:
//   - the SVGs are distinct, and each shows the outgoing and the incoming colour;
//   - the through-colour types show their colour;
//   - the directional types, and only they, carry white direction strokes;
//   - B sits on the side where the measured direction (index §4.2) says it enters;
//   - nothing external (no url(, href, id=, gradient or style), and unknown ids fall back to the plain A|B split.
func TestVideoStudioTransitionIcons(t *testing.T) {
	runVideoStudioEffectsScript(t, `
require('./js/desktop/apps/video-studio-icons.js');
const Icons = window.VideoStudioIcons;
E.adopt(undefined, undefined);
const ids = E.transitions().map(spec => spec.id);
assert.equal(ids.length, 24);
const icons = ids.map(id => Icons.transition(id, 30)), fallback = Icons.transition('not-a-transition', 30);
assert.equal(new Set(icons).size, 24, 'every transition has its own icon');
icons.forEach((svg, i) => {
  const id = ids[i];
  assert(svg.startsWith('<svg class="vs-tx-icon" width="30" height="18" viewBox="0 0 30 18" aria-hidden="true" focusable="false">') && svg.endsWith('</svg>'), id + ': ' + svg.slice(0, 140));
  assert.notEqual(svg, fallback, id + ' has a picture of its own');
  for (const bad of ['url(', 'href', 'id=', 'Gradient', '<image', '<use', '<style', 'style=']) assert(!svg.includes(bad), id + ' contains ' + bad);
  assert(svg.includes('fill="#4f5fd6"') && svg.includes('fill="#c66f31"'), id + ' shows the outgoing and the incoming clip');
});
// The colours a fade passes through.
assert(Icons.transition('black').includes('fill="#000"'));
assert(Icons.transition('fadewhite').includes('fill="#fff"'));
assert(Icons.transition('fadegrays').includes('fill="#818181"') && Icons.transition('fadegrays').includes('fill="#777777"'), 'the grey of A, then the grey of B');
assert(Icons.transition('circlecrop').includes('fill="#000"') && Icons.transition('rectcrop').includes('fill="#000"'), 'the crops pass through black');
// White strokes mark a direction on exactly the directional types.
const directional = ['wipeleft', 'wiperight', 'wipeup', 'wipedown', 'smoothleft', 'smoothright', 'diagtl', 'diagbr', 'radial', 'slideleft', 'slideright', 'slideup', 'slidedown', 'squeezeh', 'zoomin', 'circleopen', 'circleclose'];
for (const id of ids) assert.equal(Icons.transition(id).includes('stroke="#fff"'), directional.includes(id), id);
// B sits where the measured direction says it enters (index §4.2: user wipeleft = B from the left, wipeup = B from the
// bottom, smoothleft = B from the right, diagtl = B at the bottom-right corner, slideleft = B from the right, …).
const B = '#c66f31';
const enters = {
  wipeleft: '<rect x="0" y="0" width="12" height="18" fill="' + B + '"/>', wiperight: '<rect x="18" y="0" width="12" height="18" fill="' + B + '"/>',
  wipeup: '<rect x="0" y="11" width="30" height="7" fill="' + B + '"/>', wipedown: '<rect x="0" y="0" width="30" height="7" fill="' + B + '"/>',
  smoothleft: '<rect x="19" y="0" width="11" height="18" fill="' + B + '"/>', smoothright: '<rect x="0" y="0" width="11" height="18" fill="' + B + '"/>',
  diagtl: '<polygon points="30,9 30,18 18,18" fill="' + B + '"/>', diagbr: '<polygon points="0,0 12,0 0,9" fill="' + B + '"/>',
  slideleft: '<rect x="17" y="0" width="13" height="18" fill="' + B + '"/>', slideright: '<rect x="0" y="0" width="13" height="18" fill="' + B + '"/>',
  slideup: '<rect x="0" y="10" width="30" height="8" fill="' + B + '"/>', slidedown: '<rect x="0" y="0" width="30" height="8" fill="' + B + '"/>'
};
for (const [id, shape] of Object.entries(enters)) assert(Icons.transition(id).includes(shape), id + ': B enters on the measured side');
// 'none', the size and unknown ids.
assert(Icons.transition('none').includes('stroke-dasharray') && !Icons.transition('none').includes('#4f5fd6'), 'none is an empty dashed frame');
assert(Icons.transition('dissolve', 50).startsWith('<svg class="vs-tx-icon" width="50" height="30" '));
assert(Icons.transition('dissolve').startsWith('<svg class="vs-tx-icon" width="30" height="18" '), 'the default size is 30');
assert(fallback.includes('<rect x="0" y="0" width="15" height="18" fill="#4f5fd6"/><rect x="15" y="0" width="15" height="18" fill="#c66f31"/>'));
for (const odd of ['', '__proto__', 'constructor', 'toString', null, undefined]) assert.equal(Icons.transition(odd, 30), fallback, String(odd));
assert(Icons.svg('play', 16).startsWith('<svg'), 'the stroke icons stay');
console.log('ok');
`)
}

// The transition picker (2e, plan index §4.9) in the inspector markup:
//   - Collapsed by default: the summary [data-action="transition-picker"] shows the current type's icon and label_key.
//   - Open (v.transitionPickerOpen): 'none' comes first, then one role="group" per transitionGroups() category in
//     registry order (5, 9, 6 and 4 types). Each type has its generated icon and label_key label, and only the current
//     one is pressed.
//   - Unavailable types are disabled with the transitionUnavailable title; the current one stays enabled and pressed.
//   - Without a following clip the summary is disabled and the panel stays closed.
//   - transitionLength keeps the 2a slider rules.
//
// The test also pins the app wiring (toggle, view flag, Esc before the drawer), the timeline tooltip from label_key and
// the CSS (no 2a grid, no gradient icons).
func TestVideoStudioTransitionPicker(t *testing.T) {
	read := func(path ...string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(path...))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	app := read("js", "desktop", "apps", "video-studio.js")
	timeline := read("js", "desktop", "apps", "video-studio-timeline.js")
	css := read("css", "desktop-app-video-studio.css")
	for _, want := range []string{"function setTransitionPicker(s, open) {", "action === 'transition-picker'", "transitionPickerOpen: !!s.transitionPickerOpen", "applyTransition(s, transition.dataset.transition, frames); setTransitionPicker(s, false); return; }"} {
		if !strings.Contains(app, want) {
			t.Errorf("video-studio.js lacks %s", want)
		}
	}
	escape := strings.Index(app, "s.transitionPickerOpen && target.closest('[data-inspector]')")
	drawer := strings.Index(app, "if (event.key === 'Escape' && s.app.classList.contains('vs-show-inspector'))")
	if escape < 0 || drawer < 0 || escape > drawer {
		t.Error("handleKeys must close the transition picker on Escape before it closes the narrow drawer")
	}
	if !strings.Contains(timeline, "window.VideoStudioEffects.transition(clip.transition.type)") {
		t.Error("the timeline's transition tooltip must read the registry label_key")
	}
	for _, gone := range []string{".vs-transition-grid", "linear-gradient(90deg, #4f5fd6"} {
		if strings.Contains(css, gone) {
			t.Errorf("the stylesheet still holds the 2a rule %s", gone)
		}
	}
	for _, want := range []string{".vs-tx-picker", ".vs-tx-summary", ".vs-tx-icon", ".vs-tx-list"} {
		if !strings.Contains(css, want) {
			t.Errorf("the stylesheet lacks %s", want)
		}
	}
	runVideoStudioEffectsScript(t, `
// video-studio-anim.js comes with videoStudioEffectsPrelude (Task 4).
require('./js/desktop/apps/video-studio-icons.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-inspector.js');
const I = window.VideoStudioInspector, Icons = window.VideoStudioIcons;
const esc = value => String(value == null ? '' : value).replace(/[&<>"']/g, c => '&#' + c.charCodeAt(0) + ';');
const box = {offset: 0, x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1, fade_in: 0, fade_out: 0, fit: 'contain'};
// a (frames 0..59) and b on one video track; with a transition, b starts that many frames earlier.
const pair = transition => ({version: 3, fps: 30, width: 1280, height: 720, assets: [{id: 'v', kind: 'video', duration_frames: 300, has_audio: false}], tracks: [{id: 't', kind: 'video', name: 'Video', clips: [
  Object.assign({id: 'a', asset_id: 'v', start: 0, duration: 60, transition}, box),
  Object.assign({id: 'b', asset_id: 'v', start: transition ? 60 - transition.duration : 60, duration: 60, transition: null}, box)]}]});
const view = (project, extra) => {
  const track = project.tracks[0], host = {innerHTML: ''};
  I.render(host, Object.assign({selected: {clip: track.clips[0], track}, asset: project.assets[0], project, locale: 'en', caps: {},
    transitionInfo: {next: track.clips[1] || null}, esc, icon: name => '<i data-icon="' + name + '"></i>',
    tr: key => (key === 'transitionUnavailable' ? 'UNAVAILABLE' : '[' + key + ']')}, extra || {}));
  return host.innerHTML;
};
const tag = (html, id) => (html.match(new RegExp('<button[^>]*data-transition="' + id + '"[^>]*>')) || [''])[0];
const ids = html => Array.from(html.matchAll(/data-transition="([a-z]+)"/g), m => m[1]);
const summary = html => (html.match(/<button[^>]*data-action="transition-picker"[^>]*>[\s\S]*?<\/button>/) || [''])[0];
E.adopt(undefined, undefined);

// Collapsed by default: only the summary, with the current type's icon and label.
let html = view(pair({type: 'fadewhite', duration: 10}));
assert(summary(html).includes('aria-expanded="false"'), summary(html));
assert(summary(html).includes(Icons.transition('fadewhite', 30) + '<span>[transition_fadewhite]</span>'), 'the summary shows the icon and the label of the current type');
assert(summary(html).includes('title="[transitionPicker]"') && summary(html).includes('aria-label="[transitionPicker]: [transition_fadewhite]"'));
assert(!html.includes('data-tx-picker') && ids(html).length === 0, 'the panel is collapsed by default');
assert(!html.includes('vs-transition-grid') && !html.includes('<i class="vs-tx-'), 'the 2a grid and its CSS icons are gone');

// Open: none first, then one group per category in registry order, each type with its icon and label_key label.
html = view(pair({type: 'fadewhite', duration: 10}), {transitionPickerOpen: true});
assert(summary(html).includes('aria-expanded="true"'));
assert(html.includes('data-tx-picker'));
const groups = E.transitionGroups();
assert.deepEqual(groups.map(group => [group.category, group.label_key, group.items.length]), [['basic', 'transitionCategory_basic', 5], ['wipe', 'transitionCategory_wipe', 9], ['motion', 'transitionCategory_motion', 6], ['shape', 'transitionCategory_shape', 4]]);
assert.deepEqual(ids(html), ['none'].concat(groups.flatMap(group => group.items.map(item => item.id))), 'none first, then the groups in order');
assert.deepEqual(Array.from(html.matchAll(/role="group" aria-label="\[(transitionCategory_[a-z]+)\]"/g), m => m[1]), groups.map(group => group.label_key));
assert(html.includes('<h4>[transitionCategory_motion]</h4>'));
assert.deepEqual(ids(html).filter(id => tag(html, id).includes('aria-pressed="true"')), ['fadewhite'], 'only the current type is pressed');
assert(html.includes('data-transition="none" aria-pressed="false">' + Icons.transition('none', 30) + '<span>[none]</span></button>'));
for (const spec of E.transitions()) {
  assert(html.includes('data-transition="' + spec.id + '" aria-pressed="' + (spec.id === 'fadewhite') + '">' + Icons.transition(spec.id, 30) + '<span>[' + spec.label_key + ']</span></button>'), spec.id);
}
assert(html.includes('<span>[wipeleft]</span>') && html.includes('<span>[transition_wipeup]</span>'), 'the 2a and 2e label keys');

// Unavailable types are disabled with a title; the current one stays enabled and pressed.
E.adopt(E.BUILTIN, {effects: [], transitions: ['fadegrays', 'radial'], blend_modes: [], anim_presets: []});
html = view(pair({type: 'radial', duration: 10}), {transitionPickerOpen: true});
assert(/ disabled/.test(tag(html, 'fadegrays')) && tag(html, 'fadegrays').includes('title="UNAVAILABLE"'), tag(html, 'fadegrays'));
assert(!/ disabled/.test(tag(html, 'radial')) && tag(html, 'radial').includes('aria-pressed="true"') && tag(html, 'radial').includes('title="UNAVAILABLE"'), 'the current type stays selectable');
assert(!tag(html, 'wipeup').includes('title=') && !/ disabled/.test(tag(html, 'wipeup')), 'available types carry no title');
E.adopt(undefined, undefined);

// Without a following clip the summary is disabled, the panel stays closed and the hint explains why.
const lone = pair(null);
lone.tracks[0].clips.pop();
html = view(lone, {transitionPickerOpen: true});
assert(/<button[^>]*data-action="transition-picker"[^>]* disabled>/.test(html), summary(html));
assert(!html.includes('data-tx-picker') && summary(html).includes('<span>[none]</span>'));
assert(html.includes('[transitionNeedsNextHint]'));

// The length slider keeps the 2a rules (transitionLength: step, largest value, default).
assert.deepEqual(I.transitionLength({fps: 30}, {duration: 60}, {duration: 60}), {step: 3, max: 59, value: 15});
assert.deepEqual(I.transitionLength({fps: 30}, {duration: 60, transition: {type: 'dissolve', duration: 10}}, {duration: 60}), {step: 3, max: 59, value: 10}, 'an existing transition keeps its length');
assert.deepEqual(I.transitionLength({fps: 30}, {duration: 60}, null), {step: 3, max: 90, value: 15});
assert.deepEqual(I.transitionLength({fps: 30}, {duration: 4}, {duration: 5}), {step: 3, max: 3, value: 3}, 'short clips cap the default');
assert.deepEqual(I.transitionLength({fps: 25}, {duration: 200}, {duration: 200}), {step: 3, max: 75, value: 13});
html = view(pair({type: 'dissolve', duration: 10}));
assert(/<input type="range" data-transition-duration min="3" max="59" step="1" value="10" >/.test(html), 'the slider follows transitionLength');
assert(/<input type="range" data-transition-duration[^>]* disabled>/.test(view(lone)), 'no slider without a following clip');
console.log('ok');
`)
}

// Every desktop locale carries the picker name and the category names of every registry transition (Task 16). The en
// and de texts are the plan's, and the other locales translate the picker name.
func TestVideoStudioTransitionPickerLocales(t *testing.T) {
	keys, seen := []string{"transitionPicker"}, map[string]bool{}
	for _, spec := range videostudio.EffectRegistry().Transitions {
		if key := "transitionCategory_" + spec.Category; !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	if len(keys) != 5 {
		t.Fatalf("picker keys %v, want transitionPicker and four categories", keys)
	}
	english, german := readDesktopLocale(t, "en"), readDesktopLocale(t, "de")
	for key, want := range map[string][2]string{
		"transitionPicker":          {"Choose a transition", "Übergang wählen"},
		"transitionCategory_basic":  {"Basic transitions", "Einfache Übergänge"},
		"transitionCategory_wipe":   {"Wipes", "Wischblenden"},
		"transitionCategory_motion": {"Push and zoom", "Schieben und Zoomen"},
		"transitionCategory_shape":  {"Shapes", "Formen"},
	} {
		if english["videoStudio."+key] != want[0] || german["videoStudio."+key] != want[1] {
			t.Errorf("videoStudio.%s: en %q, de %q; want %q, %q", key, english["videoStudio."+key], german["videoStudio."+key], want[0], want[1])
		}
	}
	for _, lang := range videoStudioLocales {
		values := readDesktopLocale(t, lang)
		for _, key := range keys {
			if strings.TrimSpace(values["videoStudio."+key]) == "" {
				t.Errorf("%s lacks videoStudio.%s", lang, key)
			}
		}
		if lang != "en" && values["videoStudio.transitionPicker"] == english["videoStudio.transitionPicker"] {
			t.Errorf("%s copies the English videoStudio.transitionPicker", lang)
		}
	}
}

// Picker labels break between words only (2f F1, plan index §4.26.9). In the 3-column narrow drawer a word of 14 or more
// characters was broken inside itself (2e Task 16 review). The rule:
//   - every space-separated word of every registry transition label, and of None, has at most 13 characters (runes);
//   - it applies in every locale except ja and zh, which have no spaces (CJK may break anywhere);
//   - the six shortened labels are pinned: de is binding, the others are the texts accepted in brief.md.
func TestVideoStudioTransitionLabelsFitThePicker(t *testing.T) {
	const longest = 13
	keys := []string{"none"}
	for _, spec := range videostudio.EffectRegistry().Transitions {
		keys = append(keys, spec.LabelKey)
	}
	if len(keys) != 25 {
		t.Fatalf("label keys %v, want None and 24 transitions", keys)
	}
	for _, lang := range videoStudioLocales {
		if lang == "ja" || lang == "zh" {
			continue
		}
		values := readDesktopLocale(t, lang)
		for _, key := range keys {
			text := values["videoStudio."+key]
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s lacks videoStudio.%s", lang, key)
				continue
			}
			for _, word := range strings.Fields(text) {
				if n := utf8.RuneCountInString(word); n > longest {
					t.Errorf("%s videoStudio.%s = %q: the word %q has %d characters, at most %d fit a picker button", lang, key, text, word, n, longest)
				}
			}
		}
	}
	for _, c := range []struct{ lang, key, want string }{
		{"de", "transition_radial", "Uhrblende"},
		{"de", "transition_squeezeh", "Vertikal stauchen"},
		{"da", "transition_squeezeh", "Klem lodret"},
		{"no", "transition_squeezeh", "Klem loddrett"},
		{"no", "dissolve", "Overtoning"},
		{"it", "transition_pixelize", "Effetto pixel"},
	} {
		if got := readDesktopLocale(t, c.lang)["videoStudio."+c.key]; got != c.want {
			t.Errorf("%s videoStudio.%s = %q, want %q", c.lang, c.key, got, c.want)
		}
	}
}

(function () {
    'use strict';

    // The inspector's Effects section and its edits (spec §5.3, §5.4, §8.1; plan 2c Task 14).
    // - section(v, clip, track) draws a visual clip's effect stack below the Picture section, with the values at the
    //   playhead (v.keyframes = P.keyframeView). It offers:
    //   - "+ Effect" by category;
    //   - entries that collapse, switch on and off, reset and remove;
    //   - a grip to drag entries, and Alt+↑/↓ on the header to move them;
    //   - sliders with keyframe tools, and the LUT picker.
    // - The crop fields and flip toggles sit in the Picture section (video-studio-inspector.js); their edits run here.
    // - Every project change is a VideoStudioKeyframeEdit operation inside P.applyEdit: one undo step, and a refusal
    //   explains itself in a toast. A slider drag changes the project live and records one step when it ends.
    // menuModel, entryName, dropIndex, formatValue and section are DOM-free and Node-tested.
    const P = window.VideoStudioParts;
    const Fx = () => window.VideoStudioEffects;
    const K = () => window.VideoStudioKeyframeEdit;
    const I = () => window.VideoStudioInspector;
    const T = () => window.VideoStudioTimeline;
    const ENTRY_MIME = 'application/x-aurago-vs-fx-entry';
    const REASON_TEXT = {
        effectUnavailable: "Not available: this server's FFmpeg lacks this effect. The preview shows it, but the export refuses it.",
        effectsFull: 'A clip holds at most 8 effects.'
    };
    const outsideText = s => P.tr(s, 'keyframeOutside', 'Move the playhead into the clip to set keyframes.');

    // ---- Markup (DOM-free).
    // Decimals a slider value shows, from the registry step (0.01: 2, 0.001: 3, 10: 0).
    function decimals(step) { const text = String(Number(step) || 1); return text.includes('.') ? Math.min(4, text.split('.')[1].length) : 0; }
    // A param value as its slider output shows it: unit 'ev' with a sign and " EV", 'K' in kelvin, 'height' as a
    // percentage of the picture height, anything else as the number at the step's precision.
    function formatValue(param, value, locale) {
        const number = Number(value) || 0, fmt = (n, d) => n.toLocaleString(locale || 'en', { minimumFractionDigits: d, maximumFractionDigits: d });
        if (param.unit === 'height') return fmt(number * 100, Math.max(0, decimals(param.step) - 2)) + ' %';
        if (param.unit === 'K') return fmt(number, 0) + ' K';
        if (param.unit === 'ev') return (number > 0 ? '+' : '') + fmt(number, 2) + ' EV';
        return fmt(number, decimals(param.step));
    }
    // An entry's name: its effect's label, numbered among the entries of the same effect when the clip has several.
    function entryName(clip, index, tr) {
        const list = (clip && clip.effects) || [], entry = list[index], spec = entry ? Fx().effect(entry.id) : null;
        const name = tr(spec ? spec.label_key : String(entry ? entry.id : ''), entry ? entry.id : '');
        const same = list.filter(item => item && entry && item.id === entry.id);
        return same.length > 1 ? name + ' ' + (same.indexOf(entry) + 1) : name;
    }
    // The "+ Effect" menu (spec §8.1): registry effects grouped by category, in menu order. An item is disabled with a
    // reason when the server's FFmpeg lacks the effect (effectUnavailable) or the stack is full (effectsFull).
    function menuModel(clip) {
        const full = ((clip && clip.effects) || []).length >= Fx().LIMITS.effectsPerClip, groups = [];
        Fx().effects().forEach(spec => {
            let group = groups.find(item => item.category === spec.category);
            if (!group) groups.push(group = { category: spec.category, label_key: 'effectCategory_' + spec.category, items: [] });
            const reason = !spec.available ? 'effectUnavailable' : full ? 'effectsFull' : '';
            group.items.push({ id: spec.id, label_key: spec.label_key, available: !!spec.available, disabled: !!reason, reason });
        });
        return groups;
    }
    // The stack position an entry dragged from `from` takes when dropped on entry `over` (before it, or after it).
    function dropIndex(from, over, after) { const slot = over + (after ? 1 : 0); return slot > from ? slot - 1 : slot; }
    // The LUT picker (spec §5.7): none (pass-through), the built-in looks and the project's LUT assets, plus "Upload .cube".
    function lutPicker(v, entry, label) {
        const current = String((entry.params && entry.params.lut) || ''), luts = (v.effects && v.effects.luts) || [];
        const option = (value, text) => `<option value="${v.esc(value)}"${value === current ? ' selected' : ''}>${v.esc(text)}</option>`;
        const looks = Fx().looks().map(look => option('builtin:' + look.id, v.tr(look.label_key, look.id))).join('');
        const own = luts.map(lut => option(lut.id, lut.name)).join('');
        return `<div class="vs-fx-lut"><label class="vs-field vs-field-wide"><span>${v.esc(label)}</span><select data-fx-param="lut">${option('', v.tr('lutNone', 'None (no change)'))}<optgroup label="${v.esc(v.tr('lutLooks', 'Built-in looks'))}">${looks}</optgroup>${own ? `<optgroup label="${v.esc(v.tr('lutProject', 'Project LUTs'))}">${own}</optgroup>` : ''}</select></label><button type="button" class="vs-chip-button" data-fx-action="lut-upload">${v.icon('upload', 14)}<span>${v.esc(v.tr('lutUpload', 'Upload .cube'))}</span></button></div>`;
    }
    // One param control: a slider, or the picker for a LUT param.
    // - A keyframable slider gets keyframe tools.
    // - It freezes while the playhead is outside the clip and the param is animated.
    // - extra adds classes to the slider (Task 15).
    function paramRow(v, index, entry, param, extra) {
        if (!param) return '';
        const label = v.tr(param.label_key, param.id);
        if (param.type === 'lut') return lutPicker(v, entry, label);
        if (param.type !== 'number') return '';
        const path = 'effects.' + index + '.' + param.id, kf = v.keyframes, control = kf ? kf.controls[path] : null;
        const stored = entry.params ? entry.params[param.id] : undefined;
        const value = control ? Number(control.value) : Number(stored != null ? stored : param.default);
        const frozen = !!kf && !kf.inside && !!control && control.animated;
        const lock = frozen ? ` disabled title="${v.esc(v.tr('keyframeOutside', 'Move the playhead into the clip to set keyframes.'))}"` : '';
        const slider = `<label class="vs-slider vs-fx-slider vs-fx-p-${v.esc(param.id)}${extra || ''}"><span>${v.esc(label)}</span><input type="range" data-fx-param="${v.esc(param.id)}" min="${param.min}" max="${param.max}" step="${param.step || 0.01}" value="${value}"${lock}><output>${v.esc(formatValue(param, value, v.locale))}</output></label>`;
        return `<div class="vs-kf-line vs-fx-line">${slider}${param.keyframable ? I().keyframeTools(v, path, label) : ''}</div>`;
    }
    // The color entry (spec §8.1, §8.3) is drawn as follows:
    // - three wheels (lift, gamma, gain), each with one set of keyframe tools for its x and y;
    // - their master sliders;
    // - then exposure, contrast, saturation, temperature (track blue → orange: higher K warms) and tint
    //   (green → magenta).
    // Values show at the playhead. A wheel freezes like a slider while the playhead is outside the clip and the wheel is
    // animated, and on a locked track.
    const WHEELS = [['lift', 'colorWheelLift'], ['gamma', 'colorWheelGamma'], ['gain', 'colorWheelGain']];
    const COLOR_SLIDERS = ['exposure', 'contrast', 'saturation', 'temperature', 'tint'];
    function colorPanel(v, index, entry, spec) {
        const param = id => (spec.params || []).find(item => item.id === id), kf = v.keyframes;
        const control = id => (kf ? kf.controls['effects.' + index + '.' + id] : null);
        const value = id => { const c = control(id), stored = entry.params ? entry.params[id] : undefined; return c ? Number(c.value) : Number(stored != null ? stored : param(id).default); };
        const animated = id => { const c = control(id); return !!c && c.animated; };
        const wheels = WHEELS.map(([id, key]) => {
            const label = v.tr(key, id), paths = 'effects.' + index + '.' + id + '_x effects.' + index + '.' + id + '_y';
            const off = !!kf && (kf.locked || (!kf.inside && (animated(id + '_x') || animated(id + '_y'))));
            const wheel = window.VideoStudioColorWheel.markup({ name: id, label, x: value(id + '_x'), y: value(id + '_y'), tr: v.tr, esc: v.esc, disabled: off });
            return `<div class="vs-wheel-cell" data-wheel-cell="${id}"><span class="vs-wheel-title">${v.esc(v.tr(param(id).label_key, id))}</span>${wheel}${I().keyframeTools(v, paths, label)}</div>`;
        }).join('');
        const masters = WHEELS.map(([id]) => paramRow(v, index, entry, param(id))).join('');
        return `<div class="vs-fx-color"><div class="vs-wheel-row" role="group" aria-label="${v.esc(v.tr('colorWheels', 'Lift, gamma and gain'))}">${wheels}</div>${masters}${COLOR_SLIDERS.map(id => paramRow(v, index, entry, param(id))).join('')}</div>`;
    }
    function paramsMarkup(v, clip, index, entry, spec) {
        if (entry.id === 'color' && window.VideoStudioColorWheel) return colorPanel(v, index, entry, spec);
        return (spec.params || []).map(param => paramRow(v, index, entry, param)).join('');
    }
    function entryMarkup(v, clip, index) {
        const entry = clip.effects[index], spec = Fx().effect(entry.id), fx = v.effects || {};
        const name = entryName(clip, index, (key, fallback) => v.tr(key, fallback)), open = !(fx.collapsed && fx.collapsed(entry));
        const text = (key, fallback) => v.esc(v.tr(key, fallback).split('{{name}}').join(name));
        const bodyId = 'vs-fx-' + v.esc(clip.id) + '-' + index;
        const badges = (spec && !Fx().effectAvailable(entry.id) ? `<span class="vs-fx-badge is-warn" data-fx-badge="unavailable" title="${text('effectUnavailable', REASON_TEXT.effectUnavailable)}">${text('effectBadgeUnavailable', 'No export')}</span>` : '')
            + (fx.failed && fx.failed.has(entry.id) ? `<span class="vs-fx-badge" data-fx-badge="failed" title="${text('effectPreviewFailed', 'The preview cannot draw {{name}} and shows the picture without it. The export is not affected.')}">${text('effectBadgeFailed', 'Not in preview')}</span>` : '');
        const head = `<div class="vs-fx-head"><span class="vs-fx-grip" data-fx-grip draggable="true" aria-hidden="true" title="${text('effectDrag', 'Drag to change the order')}">${v.icon('grip', 14)}</span><button type="button" class="vs-fx-toggle" data-fx-head aria-expanded="${open}" aria-controls="${bodyId}" title="${text('effectHeadHint', '{{name}}: show or hide the settings. Alt+↑ / Alt+↓ moves it in the stack.')}">${v.icon('chevronDown', 14)}<span class="vs-fx-name">${v.esc(name)}</span></button>${badges}<button type="button" role="switch" class="vs-fx-switch" data-fx-action="on" aria-checked="${!!entry.on}" aria-label="${text('effectOn', '{{name}} on')}" title="${text('effectOn', '{{name}} on')}"><span></span></button><button type="button" class="vs-kf-button" data-fx-action="reset" aria-label="${text('effectReset', 'Reset {{name}}')}" title="${text('effectReset', 'Reset {{name}}')}">${v.icon('reset', 14)}</button><button type="button" class="vs-kf-button" data-fx-action="remove" aria-label="${text('effectRemove', 'Remove {{name}}')}" title="${text('effectRemove', 'Remove {{name}}')}">${v.icon('trash', 14)}</button></div>`;
        return `<li class="vs-fx-entry${entry.on ? '' : ' is-off'}" data-fx-entry="${index}" data-fx-id="${v.esc(entry.id)}">${head}<div class="vs-fx-body" id="${bodyId}"${open ? '' : ' hidden'}>${spec ? paramsMarkup(v, clip, index, entry, spec) : ''}</div></li>`;
    }
    // The Effects section of a visual clip ('' for audio clips). v.effects is P.effectView(s): failed preview programs,
    // the project's LUTs and the collapsed entries.
    function section(v, clip, track) {
        if (!clip || !track || track.kind === 'audio') return '';
        const list = Array.isArray(clip.effects) ? clip.effects : [], limit = Fx().LIMITS.effectsPerClip, full = list.length >= limit;
        const hint = list.length && Fx().cropAnimated(clip) ? `<p class="vs-hint vs-fx-hint" data-fx-hint>${v.icon('info', 14)}<span>${v.esc(v.tr('effectsCropAnimatedHint', 'While the crop is animated, the effects work on the whole picture and the crop is applied after them.'))}</span></p>` : '';
        const stack = list.length ? `<ol class="vs-fx-stack" data-fx-stack>${list.map((_, index) => entryMarkup(v, clip, index)).join('')}</ol>` : `<p class="vs-hint" data-fx-empty>${v.esc(v.tr('effectsEmpty', 'No effects yet. Add one; the stack works from top to bottom.'))}</p>`;
        const add = `<button type="button" class="vs-chip-button vs-fx-add" data-fx-action="add" aria-haspopup="menu"${full ? ` disabled title="${v.esc(v.tr('effectsFull', REASON_TEXT.effectsFull))}"` : ''}>${v.icon('plus', 14)}<span>${v.esc(v.tr('effectAdd', 'Add effect'))}</span></button>`;
        return `<section class="vs-insp-section vs-fx-section" data-fx-section><h3>${v.icon('effects', 14)}${v.esc(v.tr('effects', 'Effects'))}<span class="vs-fx-count">${list.length}/${limit}</span></h3>${hint}${stack}<div class="vs-fx-actions">${add}</div><input type="file" data-fx-lut-file accept=".cube" hidden></section>`;
    }

    // ---- App parts.
    function selectedVisual(s) {
        const found = s.project && s.selectedClipId ? T().clipFor(s.project, s.selectedClipId) : null;
        return found && found.track.kind !== 'audio' ? found : null;
    }
    // The stack index of the entry an element sits in, or -1.
    function entryIndex(el) {
        const item = el && typeof el.closest === 'function' ? el.closest('[data-fx-entry]') : null;
        return item ? Number(item.dataset.fxEntry) : -1;
    }
    // Inspector re-renders replace the controls: keyboard focus returns to the control a selector names.
    function refocus(s, selector) {
        const el = selector && typeof s.q === 'function' ? s.q('[data-inspector] ' + selector) : null;
        if (el && !el.disabled) el.focus({ preventScroll: true });
    }
    // Puts a clip back in place after a refused live change (outside P.applyEdit nothing restores it).
    function restoreClip(clip, saved) {
        const before = JSON.parse(saved);
        Object.keys(clip).forEach(key => { if (!(key in before)) delete clip[key]; });
        Object.assign(clip, before);
    }
    // One edit of the selected clip in one undo step. focus (a selector, or a function of the result) gets the focus.
    function stackEdit(s, label, op, focus) {
        const found = selectedVisual(s);
        if (!found || s.readonly) return false;
        const ok = P.applyEdit(s, label, project => op(project, found.clip.id));
        refocus(s, typeof focus === 'function' ? focus(ok) : focus);
        return ok;
    }
    // The Effects section's view state:
    // - preview programs that failed (badges);
    // - the project's LUT assets for the picker;
    // - the collapsed entries. s.fxCollapsed holds entry objects, which survive in-place edits but not an undo.
    function effectView(s) {
        const failed = new Set((s.preview && typeof s.preview.failedPrograms === 'function' ? s.preview.failedPrograms() : []).filter(item => item && item.kind === 'effect').map(item => item.id));
        const luts = ((s.project && s.project.assets) || []).filter(asset => asset.kind === 'lut').map(asset => ({ id: asset.id, name: P.assetDisplayName(s, asset) }));
        const collapsed = s.fxCollapsed || (s.fxCollapsed = new WeakSet());
        return { failed, luts, collapsed: entry => collapsed.has(entry) };
    }
    // Adds a registry effect to a clip in one undo step: the "+ Effect" menu, and plan 2c Task 17's library (look items
    // pass params {lut}). When the clip is the selected one, the new entry's header takes focus.
    function addEffect(s, clipId, effectId, params) {
        if (!s.project || s.readonly) return false;
        const ok = P.applyEdit(s, 'add effect', project => K().addEffect(project, clipId, effectId, params));
        const found = ok ? T().clipFor(s.project, clipId) : null;
        if (found && clipId === s.selectedClipId) refocus(s, `[data-fx-entry="${found.clip.effects.length - 1}"] [data-fx-head]`);
        return ok;
    }
    // "+ Effect": P.openMenu with the menu model.
    // - Categories get small headings and separators.
    // - Disabled items carry their reason as a title.
    // - Every item carries data-fx-add="<id>".
    function openAddMenu(s, button) {
        const found = selectedVisual(s);
        if (!found || s.readonly) return;
        const groups = menuModel(found.clip), items = [], heads = [];
        groups.forEach((group, g) => {
            if (g) items.push({ separator: true });
            heads.push({ index: items.length, text: P.tr(s, group.label_key, group.category) });
            group.items.forEach(item => items.push({ id: item.id, reason: item.reason, label: P.tr(s, item.label_key, item.id), disabled: item.disabled, run: () => addEffect(s, found.clip.id, item.id) }));
        });
        const rect = button.getBoundingClientRect();
        const el = P.openMenu(s, rect.left, rect.bottom, items, P.tr(s, 'effectAdd', 'Add effect'), () => s.q('[data-inspector] [data-fx-action="add"]'));
        if (!el) return;
        items.forEach((item, index) => {
            const node = item.id ? el.querySelector(`[data-menu-index="${index}"]`) : null;
            if (!node) return;
            node.dataset.fxAdd = item.id;
            if (item.reason) node.title = P.tr(s, item.reason, REASON_TEXT[item.reason]);
        });
        heads.forEach(head => { const node = el.querySelector(`[data-menu-index="${head.index}"]`); if (node) node.insertAdjacentHTML('beforebegin', `<div class="vs-fx-menu-heading" aria-hidden="true">${P.esc(s, head.text)}</div>`); });
        // The headings make the menu taller than openMenu measured: keep it inside the editor.
        const app = s.app.getBoundingClientRect(), box = el.getBoundingClientRect();
        el.style.top = Math.max(4, Math.min(parseFloat(el.style.top) || 0, app.height - box.height - 4)) + 'px';
    }
    // The header toggle collapses or expands the entry in place (no re-render, so focus stays).
    function toggleCollapsed(s, head) {
        const found = selectedVisual(s), index = entryIndex(head), entry = found && Array.isArray(found.clip.effects) ? found.clip.effects[index] : null;
        const item = head.closest('[data-fx-entry]'), body = item ? item.querySelector('.vs-fx-body') : null;
        if (!entry || !body) return;
        const set = s.fxCollapsed || (s.fxCollapsed = new WeakSet()), open = body.hidden;
        if (open) set.delete(entry); else set.add(entry);
        body.hidden = !open;
        head.setAttribute('aria-expanded', String(open));
    }
    // Moves an entry of the selected clip (drag, Alt+↑/↓); focus stays on the moved entry's header. A move or a removal shifts
    // the stack indexes; P.mutate then drops the selected strip keys of effects.<i>.* (P.dropEffectKeySelection).
    function moveEntry(s, from, to) {
        const found = selectedVisual(s), list = found && Array.isArray(found.clip.effects) ? found.clip.effects : [];
        if (from < 0 || from >= list.length || to < 0 || to >= list.length || to === from) return false;
        return stackEdit(s, 'move effect', (project, id) => K().moveEffect(project, id, from, to), ok => `[data-fx-entry="${ok ? to : from}"] [data-fx-head]`);
    }
    // Clicks in the Effects section and on the Picture section's flip toggles; true when the click was one of them.
    function effectCommand(s, event) {
        const target = event.target && typeof event.target.closest === 'function' ? event.target : null;
        if (!target || !s.project) return false;
        const flip = target.closest('[data-inspector] [data-flip]');
        if (flip) {
            if (!flip.disabled) stackEdit(s, 'flip', (project, id) => K().setFlip(project, id, flip.dataset.flip, flip.getAttribute('aria-pressed') !== 'true'), `[data-flip="${flip.dataset.flip}"]`);
            return true;
        }
        const head = target.closest('[data-inspector] [data-fx-head]');
        if (head) { toggleCollapsed(s, head); return true; }
        const button = target.closest('[data-inspector] [data-fx-action]');
        if (!button) return false;
        if (button.disabled) return true;
        const action = button.dataset.fxAction, index = entryIndex(button), at = selector => `[data-fx-entry="${index}"] ${selector}`;
        if (action === 'add') openAddMenu(s, button);
        else if (action === 'on') stackEdit(s, 'effect on', (project, id) => K().setEffectOn(project, id, index, button.getAttribute('aria-checked') !== 'true'), at('[data-fx-action="on"]'));
        else if (action === 'reset') stackEdit(s, 'reset effect', (project, id) => K().resetEffect(project, id, index), at('[data-fx-action="reset"]'));
        else if (action === 'remove') {
            stackEdit(s, 'remove effect', (project, id) => K().removeEffect(project, id, index), ok => {
                const found = selectedVisual(s), count = found && Array.isArray(found.clip.effects) ? found.clip.effects.length : 0;
                if (!ok) return at('[data-fx-action="remove"]');
                return count ? `[data-fx-entry="${Math.min(index, count - 1)}"] [data-fx-head]` : '[data-fx-action="add"]';
            });
        } else if (action === 'lut-upload') {
            if (s.readonly) return true;
            s.fxUpload = { clipId: s.selectedClipId, index };
            const file = s.q('[data-inspector] [data-fx-lut-file]');
            if (file) file.click();
        } else return false;
        return true;
    }
    // Alt+ArrowUp / Alt+ArrowDown on a focused entry header move the entry (spec §8.1).
    // - Matched by event.code like the keyframe shortcuts.
    // - The event is stopped here, so P.handleKeyframeKeys (which jumps between keyframes everywhere else) never sees it;
    //   the top and bottom entries keep the key without moving.
    function handleEffectStackKeys(s, event) {
        const target = event.target, head = target && typeof target.closest === 'function' ? target.closest('[data-fx-head]') : null;
        if (!head || !s.project || s.stageMode === 'source' || !event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return false;
        if (event.code !== 'ArrowUp' && event.code !== 'ArrowDown') return false;
        event.preventDefault();
        event.stopPropagation();
        const from = entryIndex(head);
        moveEntry(s, from, from + (event.code === 'ArrowUp' ? -1 : 1));
        return true;
    }
    // The selected clip's entry a control belongs to, while the editor may change it.
    function paramTarget(s, el) {
        const found = selectedVisual(s), index = entryIndex(el);
        if (!found || found.track.locked || s.readonly || !Array.isArray(found.clip.effects) || !found.clip.effects[index]) return null;
        return { found, index };
    }
    const paramSelector = el => `[data-fx-entry="${entryIndex(el)}"] [data-fx-param="${el.dataset.fxParam}"]`;
    // A slider drag changes the project live (auto-key through setEffectParams; no undo step, no re-render). A refusal
    // puts the clip back and explains itself once per gesture.
    function liveParam(s, input) {
        const target = paramTarget(s, input);
        if (!target) return;
        if (!input._vsBefore) { input._vsBefore = P.snapshot(s); s.fieldGesture = true; }
        clearTimeout(s.autosaveTimer);
        const clip = target.found.clip, saved = JSON.stringify(clip), id = input.dataset.fxParam;
        const result = K().setEffectParams(s.project, clip.id, target.index, { [id]: Number(input.value) }, s.frame);
        if (result !== true) {
            restoreClip(clip, saved);
            if (!input._vsRefused) { input._vsRefused = true; P.showReason(s, result); }
            return;
        }
        const spec = Fx().effect(clip.effects[target.index].id), param = spec && (spec.params || []).find(item => item.id === id);
        if (param) P.updateOutput(input, formatValue(param, Number(input.value), P.uiLocale()));
        s.dirty = true; s.revision++;
        P.renderToolbar(s);
        if (s.preview) s.preview.seek(s.frame);
    }
    // The end of a slider gesture records one undo step (P.recordChange). A change without a gesture (some keyboard
    // paths) is one applyEdit.
    function commitParam(s, input) {
        const before = input._vsBefore, selector = paramSelector(input);
        delete input._vsBefore; delete input._vsRefused;
        s.fieldGesture = false;
        if (!before) {
            const target = paramTarget(s, input);
            if (target) stackEdit(s, 'effect', (project, id) => K().setEffectParams(project, id, target.index, { [input.dataset.fxParam]: Number(input.value) }, s.frame), selector);
            return;
        }
        if (JSON.stringify(before) !== JSON.stringify(s.project)) P.recordChange(s, 'effect', before);
        refocus(s, selector);
    }
    // The LUT picker: one undo step.
    function selectParam(s, select) {
        const target = paramTarget(s, select);
        if (target) stackEdit(s, 'LUT', (project, id) => K().setEffectParams(project, id, target.index, { [select.dataset.fxParam]: select.value }, s.frame), paramSelector(select));
    }
    // A crop field (percent of the source picture): one undo step through setCrop (auto-key while the side is animated).
    function commitCrop(s, input) {
        const value = Number(input.value) / 100, side = input.dataset.crop;
        if (!Number.isFinite(value) || !stackEdit(s, 'crop', (project, id) => K().setCrop(project, id, { [side]: Math.max(0, Math.min(0.95, value)) }, s.frame), `[data-crop="${side}"]`)) P.renderInspector(s);
    }
    // "Upload .cube" (spec §5.7, §9). The file goes through the media import (P.uploadFile); the server parses it before
    // anything is written (Task 5). The new LUT asset then becomes the entry's look in one undo step. Nothing changes when
    // the entry is gone or is no LUT entry any more.
    async function uploadLUT(s, clipId, index, file) {
        if (!file || s.readonly || !s.project) return false;
        if (!/\.cube$/i.test(String(file.name || ''))) { P.showNotice(s, 'lutUploadType', 'Choose a .cube file.', true); return false; }
        const projectId = s.projectId, known = new Set(s.project.assets.map(asset => asset.id));
        await P.uploadFile(s, file);
        if (s.disposed || projectId !== s.projectId || !s.project) return false;
        const added = s.project.assets.find(asset => !known.has(asset.id) && asset.kind === 'lut');
        const found = added ? T().clipFor(s.project, clipId) : null, entry = found && Array.isArray(found.clip.effects) ? found.clip.effects[index] : null;
        if (!entry || entry.id !== 'lut') return false;
        return P.applyEdit(s, 'LUT', project => K().setEffectParams(project, clipId, index, { lut: added.id }, s.frame));
    }
    // ---- Colour wheels (Task 15).
    // - A drag changes the project live (auto-key of x and y; no undo step) and records one step when it ends.
    // - Keys and resets are one edit each.
    // - A refused live change puts the clip back and explains itself once per drag.
    const wheelValues = (el, x, y) => ({ [el.dataset.wheel + '_x']: x, [el.dataset.wheel + '_y']: y });
    const wheelSelector = el => `[data-fx-entry="${entryIndex(el)}"] [data-wheel="${el.dataset.wheel}"]`;
    const wheelText = s => (key, fallback) => P.tr(s, key, fallback);
    function liveWheel(s, el, x, y) {
        const target = paramTarget(s, el);
        if (!target) return;
        if (!el._vsBefore) { el._vsBefore = P.snapshot(s); s.fieldGesture = true; }
        clearTimeout(s.autosaveTimer);
        const clip = target.found.clip, saved = JSON.stringify(clip);
        const result = K().setEffectParams(s.project, clip.id, target.index, wheelValues(el, x, y), s.frame);
        if (result !== true) {
            restoreClip(clip, saved);
            if (!el._vsRefused) { el._vsRefused = true; P.showReason(s, result); }
            return;
        }
        window.VideoStudioColorWheel.update(el, x, y, wheelText(s));
        s.dirty = true; s.revision++;
        P.renderToolbar(s);
        if (s.preview) s.preview.seek(s.frame);
    }
    function commitWheel(s, el, cancelled) {
        const before = el._vsBefore, selector = wheelSelector(el);
        delete el._vsBefore; delete el._vsRefused;
        s.fieldGesture = false;
        if (!before) return;
        if (cancelled) { s.project = P.hydrateProject(s, before); P.renderUI(s); }
        else if (JSON.stringify(before) !== JSON.stringify(s.project)) P.recordChange(s, 'color wheel', before);
        refocus(s, selector);
    }
    function changeWheel(s, el, x, y) {
        const target = paramTarget(s, el);
        if (target) stackEdit(s, 'color wheel', (project, id) => K().setEffectParams(project, id, target.index, wheelValues(el, x, y), s.frame), wheelSelector(el));
    }
    // Delegated listeners on the inspector (they end with s.abort):
    // - sliders: input changes the project live, change records one step;
    // - the LUT picker and the crop fields commit on change;
    // - the hidden file input uploads a .cube for the entry that asked;
    // - entries drag by their grip and drop before or after another entry.
    function wireEffectStack(s) {
        const host = s.q('[data-inspector]'), signal = s.abort.signal;
        if (!host) return;
        host.addEventListener('input', event => {
            const input = event.target;
            if (input && input.matches && input.matches('input[type="range"][data-fx-param]')) liveParam(s, input);
        }, { signal });
        host.addEventListener('change', event => {
            const input = event.target;
            if (!input || !input.matches) return;
            if (input.matches('input[type="range"][data-fx-param]')) commitParam(s, input);
            else if (input.matches('select[data-fx-param]')) selectParam(s, input);
            else if (input.matches('[data-crop]')) commitCrop(s, input);
            else if (input.matches('[data-fx-lut-file]')) {
                const file = input.files && input.files[0], target = s.fxUpload;
                input.value = '';
                s.fxUpload = null;
                if (file && target) uploadLUT(s, target.clipId, target.index, file);
            }
        }, { signal });
        const clear = classes => host.querySelectorAll('.' + classes.join(',.')).forEach(el => el.classList.remove(...classes));
        const over = event => {
            const item = s.fxDrag != null && event.target && event.target.closest ? event.target.closest('[data-fx-entry]') : null;
            if (!item) return null;
            const rect = item.getBoundingClientRect();
            return { item, index: Number(item.dataset.fxEntry), after: event.clientY > rect.top + rect.height / 2 };
        };
        host.addEventListener('dragstart', event => {
            const grip = event.target && event.target.closest ? event.target.closest('[data-fx-grip]') : null;
            if (!grip) return;
            const found = selectedVisual(s), index = entryIndex(grip);
            if (!found || found.track.locked || s.readonly || index < 0) { event.preventDefault(); return; }
            s.fxDrag = index;
            event.dataTransfer.setData(ENTRY_MIME, String(index));
            event.dataTransfer.effectAllowed = 'move';
            const item = grip.closest('[data-fx-entry]');
            if (event.dataTransfer.setDragImage) event.dataTransfer.setDragImage(item, 16, 16);
            item.classList.add('is-dragging');
        }, { signal });
        host.addEventListener('dragover', event => {
            const hit = over(event);
            if (!hit) return;
            event.preventDefault();
            event.dataTransfer.dropEffect = 'move';
            clear(['is-drop-before', 'is-drop-after']);
            hit.item.classList.add(hit.after ? 'is-drop-after' : 'is-drop-before');
        }, { signal });
        host.addEventListener('drop', event => {
            const hit = over(event), from = s.fxDrag;
            if (!hit) return;
            event.preventDefault();
            event.stopPropagation();
            s.fxDrag = null;
            clear(['is-drop-before', 'is-drop-after', 'is-dragging']);
            moveEntry(s, from, dropIndex(from, hit.index, hit.after));
        }, { signal });
        host.addEventListener('dragend', () => { s.fxDrag = null; clear(['is-drop-before', 'is-drop-after', 'is-dragging']); }, { signal });
        if (window.VideoStudioColorWheel) window.VideoStudioColorWheel.wire(host, { live: (el, x, y) => liveWheel(s, el, x, y), commit: (el, cancelled) => commitWheel(s, el, cancelled), change: (el, x, y) => changeWheel(s, el, x, y) }, signal);
    }
    // Keeps the effect sliders and the crop fields on the playhead while it moves (paused), like the box fields. Called by
    // P.syncKeyframeControls with its keyframe view.
    // - Animated values show the value at the playhead and freeze while the playhead is outside the clip.
    // - A control under the pointer or caret keeps what the user is doing.
    function syncEffectControls(s, view, host) {
        const found = selectedVisual(s);
        if (!found || !view || !host) return;
        const outside = outsideText(s), active = document.activeElement, clip = found.clip;
        host.querySelectorAll('[data-fx-entry] input[type="range"][data-fx-param]').forEach(input => {
            const index = entryIndex(input), c = view.controls['effects.' + index + '.' + input.dataset.fxParam];
            if (!c || !c.animated) return;
            input.disabled = view.locked || !view.inside;
            input.title = view.inside ? '' : outside;
            if (input._vsBefore || input === active) return;
            input.value = String(c.value);
            const entry = clip.effects && clip.effects[index], spec = entry && Fx().effect(entry.id), param = spec && (spec.params || []).find(item => item.id === input.dataset.fxParam);
            if (param) P.updateOutput(input, formatValue(param, c.value, P.uiLocale()));
        });
        host.querySelectorAll('[data-crop]').forEach(input => {
            const c = view.controls['crop.' + input.dataset.crop];
            if (!c) return;
            const frozen = !view.inside && c.animated;
            input.disabled = view.locked || frozen;
            input.title = frozen ? outside : '';
            if (input !== active) input.value = String(Math.round(Number(c.value) * 1e8) / 1e6);
        });
        // Animated wheels follow the playhead too, and freeze outside the clip.
        if (window.VideoStudioColorWheel) host.querySelectorAll('[data-fx-entry] [data-wheel]').forEach(el => {
            const base = 'effects.' + entryIndex(el) + '.' + el.dataset.wheel, cx = view.controls[base + '_x'], cy = view.controls[base + '_y'];
            if (!cx || !cy || (!cx.animated && !cy.animated)) return;
            const off = view.locked || !view.inside;
            el.setAttribute('aria-disabled', String(off));
            el.tabIndex = off ? -1 : 0;
            if (!el._vsBefore) window.VideoStudioColorWheel.update(el, cx.value, cy.value, (key, fallback) => P.tr(s, key, fallback));
        });
    }

    window.VideoStudioInspectorEffects = { section, menuModel, entryName, dropIndex, formatValue };
    Object.assign(P, { effectView, addEffect, effectCommand, handleEffectStackKeys, wireEffectStack, uploadLUT, syncEffectControls });
})();

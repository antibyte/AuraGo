(function () {
    'use strict';

    // Plan mode storyboard (DOM only). VideoStudioPlan (video-studio-plan.js) owns state, requests and edits and hands
    // this file a view (VideoStudioPlan.view(s)): the board shell in the centre column, the loading, error, unsupported
    // and empty states, the header with the idea and the agent status line, the plan-level fields (style, characters,
    // music, clip sound), one card per scene with its transition chip, the inline issues, the bottom bar and the start
    // and revise dialogs. Controls carry data-plan-* attributes only: the shell's click, input and change handlers react
    // to data-action, data-field, data-transition and similar names, so none of those appear here.
    const P = window.VideoStudioParts;
    const tr = (s, key, fallback) => P.tr(s, key, fallback);
    const esc = (s, value) => P.esc(s, value);
    const icon = (name, size, cls) => P.icon(name, size, cls);
    const fill = (text, vars) => P.fill(text, vars);
    const label = (s, key, fallback) => esc(s, tr(s, key, fallback));
    const own = (map, key) => Object.prototype.hasOwnProperty.call(map, key);

    const TARGETS = Array.from({ length: 23 }, (_, index) => 10 + index * 5);
    const MAX_SCENES = 12;
    const MAX_CHARACTERS = 6;
    const REASONS = {
        provider_images_public_url_only: 'This video provider accepts start images only as public web addresses.',
        provider_unsupported: 'The configured video provider does not support plan mode.',
        generation_unavailable: 'Video generation is not set up.'
    };
    const PHASES = { outline: 'Scene outline', details: 'Scene details', check: 'Checking the plan', revise: 'Revising a scene' };
    const KINDS = { cut: 'Cut', continue: 'Seamless', bridge: 'Bridge', blend: 'Blend' };
    const SOURCES = { generated: 'Generated image', library: 'Library image', none: 'Text only', chain: 'Continues from scene {{number}}', bridge: 'Shared frame with scene {{number}}' };
    const CLIP_AUDIO = { keep: 'Keep', duck: 'Lower', mute: 'Mute' };
    // Where an issue shows: the slot of its code in the scene card (issues with a scene_id) or in the plan section. A slot
    // that is not on screen (the first card has no transition chip) falls back to the card, plan issues to the header.
    const CODE_SLOTS = {
        title_missing: 'title', title_too_long: 'title', prompt_missing: 'video_prompt', prompt_too_long: 'video_prompt',
        negative_prompt_too_long: 'negative_prompt', duration_unsupported: 'timing', use_seconds_invalid: 'timing',
        transition_unknown: 'transition', transition_unsupported: 'transition', first_scene_transition: 'transition',
        chain_too_long: 'transition', blend_type_unknown: 'transition', blend_type_unavailable: 'transition',
        blend_overlap_invalid: 'transition', keyframe_source_invalid: 'keyframe', keyframe_prompt_missing: 'keyframe',
        keyframe_asset_invalid: 'keyframe', narration_unavailable: 'narration', narration_too_long: 'narration',
        style_too_long: 'style', too_many_characters: 'characters', character_invalid: 'characters',
        character_reference_invalid: 'characters', music_unavailable: 'music', clip_audio_invalid: 'clip_audio'
    };
    const PLAN_SLOTS = new Set(['style', 'characters', 'music', 'clip_audio']);

    function num(value) {
        const n = Math.round((Number(value) || 0) * 100) / 100;
        try { return n.toLocaleString(P.uiLocale(), { maximumFractionDigits: 1 }); } catch (_) { return String(n); }
    }
    function secondsLabel(s, value) { return fill(tr(s, 'plan.secondsValue', '{{value}} s'), { value: num(value) }); }
    function durationText(s, scene) { return fill(tr(s, 'plan.duration', '{{use}} of {{generate}} s'), { use: num(scene.use_seconds), generate: num(scene.generate_seconds) }); }
    function option(s, value, text, selected, disabled) { return `<option value="${esc(s, value)}"${selected ? ' selected' : ''}${disabled ? ' disabled' : ''}>${esc(s, text)}</option>`; }
    function formId(s) { return 'vs-plan-create-' + s.id; }

    function shellMarkup(s) {
        return `<section class="vs-plan" data-plan-board aria-label="${label(s, 'plan.board', 'Storyboard')}" hidden></section>`;
    }
    // Full redraw. The focused field (data-plan-key), its caret and the scroll position survive.
    function render(s, view) {
        const host = s.q('[data-plan-board]');
        if (!host) return;
        const active = document.activeElement;
        const key = active && host.contains(active) && active.dataset ? active.dataset.planKey || '' : '';
        let selection = null;
        if (key) { try { selection = typeof active.selectionStart === 'number' ? [active.selectionStart, active.selectionEnd] : null; } catch (_) { selection = null; } }
        const scroller = host.querySelector('.vs-plan-scroll');
        const top = scroller ? scroller.scrollTop : 0;
        host.innerHTML = `<div class="vs-plan-scroll"><div class="vs-plan-inner">${body(s, view)}</div></div>${bar(s, view)}`;
        const next = host.querySelector('.vs-plan-scroll');
        if (next) next.scrollTop = top;
        if (key) {
            const el = host.querySelector(`[data-plan-key="${CSS.escape(key)}"]`);
            if (el && !el.disabled) {
                el.focus({ preventScroll: true });
                if (selection && typeof el.setSelectionRange === 'function') { try { el.setSelectionRange(selection[0], selection[1]); } catch (_) { /* not a text field */ } }
            }
        }
        renderIssues(s, view);
    }
    function body(s, view) {
        if (view.phase === 'loading') return `<div class="vs-plan-state" role="status">${icon('spinner', 18, 'vs-icon vs-spin')}<span>${label(s, 'plan.loading', 'Loading the plan…')}</span></div>`;
        if (view.phase === 'error') return `<div class="vs-plan-state" role="alert">${icon('alert', 18)}<span>${label(s, 'plan.loadFailed', 'The plan could not be loaded.')}</span><button type="button" class="vs-chip-button" data-plan-action="retry">${label(s, 'plan.retry', 'Try again')}</button></div>`;
        if (view.phase === 'unsupported') return unsupported(s, view);
        if (view.phase === 'empty') return empty(s, view);
        if (view.phase === 'planning' && !(view.plan.scenes || []).length) return head(s, view);
        return head(s, view) + settings(s, view) + scenes(s, view);
    }
    function unsupported(s, view) {
        const reason = String(view.availability.reason || '');
        const text = own(REASONS, reason) ? tr(s, 'plan.reason.' + reason, REASONS[reason]) : '';
        return `<div class="vs-plan-empty" data-plan-unsupported><span class="vs-modal-icon">${icon('alert', 22)}</span><h2>${label(s, 'plan.unsupportedTitle', 'Plan mode is not available')}</h2>${text ? `<p>${esc(s, text)}</p>` : ''}<p class="vs-hint">${label(s, 'plan.unsupportedHint', 'Choose a supported video provider in Settings > Video Generation.')}</p></div>`;
    }
    function empty(s, view) {
        const intro = `<span class="vs-modal-icon">${icon('sparkle', 22)}</span><h2>${label(s, 'plan.emptyTitle', 'Plan a clip with the agent')}</h2>`;
        // A damaged plan.json (Part B: GET 409 plan_invalid) shows this state with a notice; its create sends replace.
        const damaged = view.damaged ? `<p class="vs-plan-issue is-blocking" data-plan-damaged>${icon('alert', 13)}<span>${label(s, 'plan.damaged', 'The stored plan is damaged. Create a new plan to replace it.')}</span></p>` : '';
        if (view.readonly) return `<div class="vs-plan-empty" data-plan-empty>${intro}${damaged || `<p>${label(s, 'plan.emptyReadonly', 'This project has no plan yet.')}</p>`}<p class="vs-hint">${label(s, 'plan.readonlyHint', 'Read-only: the storyboard can be viewed but not changed.')}</p></div>`;
        return `<form class="vs-plan-empty" id="${esc(s, formId(s))}" data-plan-empty data-plan-create-form>${intro}${damaged}<p>${label(s, 'plan.emptyCopy', 'Describe your idea. The agent plans the scenes as a storyboard. Nothing is generated or paid for yet.')}</p>${ideaFields(s, '', 30)}${view.profileLine ? `<p class="vs-hint">${icon('info', 13)} ${esc(s, view.profileLine)}</p>` : ''}</form>`;
    }
    function ideaFields(s, idea, target) {
        const values = TARGETS.includes(target) ? TARGETS : TARGETS.concat(target).sort((a, b) => a - b);
        return `<label class="vs-field vs-field-wide"><span>${label(s, 'plan.idea', 'Idea')}</span><textarea data-plan-idea rows="5" maxlength="4000" required placeholder="${label(s, 'plan.ideaHint', '30 seconds: a fisherman heads out at dawn and the day wakes over the bay')}">${esc(s, idea)}</textarea></label>`
            + `<label class="vs-field vs-field-wide"><span>${label(s, 'plan.target', 'Target length')}</span><select data-plan-target>${values.map(value => option(s, value, secondsLabel(s, value), value === target)).join('')}</select></label>`;
    }
    function head(s, view) {
        const plan = view.plan, agent = view.agent;
        const profile = view.profileLine ? `<small>${esc(s, view.profileLine)}</small>` : '';
        let status;
        if (view.phase === 'planning') {
            // A job that just started has no phase yet (Part C's Start): a revise job has one phase, create starts with the outline.
            const phase = agent && own(PHASES, agent.phase) ? agent.phase : agent && agent.mode === 'revise' ? 'revise' : 'outline';
            const stop = `<button type="button" class="vs-chip-button vs-plan-stop" data-plan-action="stop">${icon('close', 14)}<span>${label(s, 'plan.stop', 'Stop')}</span></button>`;
            status = `${icon('spinner', 15, 'vs-icon vs-spin')}<strong>${label(s, 'plan.planning', 'Planning')}</strong><span data-plan-phase="${esc(s, phase)}">${label(s, 'plan.phase.' + phase, PHASES[phase])}</span>${profile}${stop}`;
        } else {
            const stopped = !!agent && (agent.state === 'stopped' || agent.error === 'stopped');
            const failed = !!agent && agent.state === 'failed';
            const ended = stopped ? label(s, 'plan.agentStopped', 'Planning was stopped. The plan keeps what was planned so far.') : failed ? label(s, 'plan.agentFailed', 'Planning ended early. The plan keeps what was planned so far.') : '';
            status = `${icon('sparkle', 15)}${profile}${ended ? `<span class="vs-plan-ended">${ended}</span>` : ''}`;
        }
        return `<header class="vs-plan-head"><div class="vs-plan-idea"><p class="vs-eyebrow">${label(s, 'plan.idea', 'Idea')}</p><p>${esc(s, plan.idea || '')}</p></div><div class="vs-plan-agent" data-plan-agent role="status">${status}</div>${view.readonly ? `<p class="vs-hint">${label(s, 'plan.readonlyHint', 'Read-only: the storyboard can be viewed but not changed.')}</p>` : ''}<div data-plan-issues="plan:general"></div></header>`;
    }
    function settings(s, view) {
        const plan = view.plan, dis = view.locked ? ' disabled' : '';
        const characters = plan.characters || [];
        const name = label(s, 'plan.characterName', 'Name');
        const look = label(s, 'plan.characterDescription', 'Look and clothing');
        const remove = label(s, 'plan.removeCharacter', 'Remove character');
        const rows = characters.map((character, index) => `<div class="vs-plan-character"><input type="text" data-plan-field="character.name" data-plan-index="${index}" data-plan-key="character:${index}:name" maxlength="80" value="${esc(s, character.name || '')}" placeholder="${name}" aria-label="${name}"${dis}><textarea data-plan-field="character.description" data-plan-index="${index}" data-plan-key="character:${index}:description" rows="2" maxlength="600" placeholder="${look}" aria-label="${look}"${dis}>${esc(s, character.description || '')}</textarea>${view.locked ? '' : `<button type="button" class="vs-icon-button" data-plan-action="remove-character" data-plan-index="${index}" aria-label="${remove}" title="${remove}">${icon('trash', 15)}</button>`}</div>`).join('');
        const add = view.locked ? '' : `<button type="button" class="vs-chip-button" data-plan-action="add-character"${characters.length >= MAX_CHARACTERS ? ' disabled' : ''}>${icon('plus', 14)}<span>${label(s, 'plan.addCharacter', 'Add character')}</span></button>`;
        const music = plan.audio.music || {};
        const musicField = view.availability.music || music.prompt ? `<label class="vs-field"><span>${label(s, 'plan.music', 'Music prompt')}</span><textarea data-plan-field="music.prompt" data-plan-key="plan:music" rows="2" maxlength="2000"${dis}>${esc(s, music.prompt || '')}</textarea></label><div data-plan-issues="plan:music"></div>` : '';
        const clip = own(CLIP_AUDIO, plan.audio.clip_audio) ? plan.audio.clip_audio : 'duck';
        return `<details class="vs-plan-settings" data-plan-settings${view.settingsOpen ? ' open' : ''}><summary>${icon('sliders', 15)}<span>${label(s, 'plan.planSettings', 'Style and sound')}</span></summary>`
            + `<label class="vs-field"><span>${label(s, 'plan.style', 'Style')}</span><textarea data-plan-field="style" data-plan-key="plan:style" rows="3" maxlength="2000"${dis}>${esc(s, plan.style || '')}</textarea></label><div data-plan-issues="plan:style"></div>`
            + `<div class="vs-plan-characters"><p class="vs-plan-label">${label(s, 'plan.characters', 'Characters')}</p>${rows}${add}<div data-plan-issues="plan:characters"></div></div>`
            + musicField
            + `<label class="vs-field"><span>${label(s, 'plan.clipAudio', 'Clip sound')}</span><select data-plan-field="clip_audio" data-plan-key="plan:clip_audio"${dis}>${Object.keys(CLIP_AUDIO).map(value => option(s, value, tr(s, 'plan.clipAudio.' + value, CLIP_AUDIO[value]), value === clip)).join('')}</select></label><div data-plan-issues="plan:clip_audio"></div></details>`;
    }
    function scenes(s, view) {
        const list = view.plan.scenes || [];
        if (!list.length) return `<div class="vs-plan-none"><p class="vs-hint">${label(s, 'plan.noScenes', 'This plan has no scenes yet.')}</p>${view.locked ? '' : `<button type="button" class="vs-chip-button" data-plan-action="add-scene">${icon('plus', 14)}<span>${label(s, 'plan.addScene', 'Add scene')}</span></button>`}</div>`;
        return `<ol class="vs-plan-scenes" data-plan-scenes>${list.map((scene, index) => `<li class="vs-plan-item">${index ? transition(s, view, scene) : ''}${card(s, view, scene, index)}</li>`).join('')}</ol>`;
    }
    // The chip before every scene but the first: the kinds the profile allows (plus a stored one it does not, so its
    // issue stays readable); a blend adds the type and the overlap.
    function transition(s, view, scene) {
        const id = esc(s, scene.id), dis = view.locked ? ' disabled' : '';
        const value = scene.transition_in || {};
        const kind = value.kind || 'cut';
        const kinds = view.kinds.includes(kind) ? view.kinds : view.kinds.concat(kind);
        const name = label(s, 'plan.transition', 'Transition');
        let blend = '';
        if (kind === 'blend') {
            const types = !value.type || view.blendTypes.some(item => item.id === value.type) ? view.blendTypes : view.blendTypes.concat({ id: value.type, label: value.type });
            const typeName = label(s, 'plan.blendType', 'Blend type');
            blend = `<select data-plan-field="transition.type" data-plan-scene="${id}" data-plan-key="${id}:transition.type" aria-label="${typeName}" title="${typeName}"${dis}>${types.map(item => option(s, item.id, item.label, item.id === value.type)).join('')}</select>`
                + `<label class="vs-plan-inline"><span>${label(s, 'plan.blendSeconds', 'Overlap (s)')}</span><input type="number" min="0.5" max="2" step="0.1" data-plan-field="transition.seconds" data-plan-scene="${id}" data-plan-key="${id}:transition.seconds" value="${esc(s, Number(value.seconds) || '')}"${dis}></label>`;
        }
        return `<div class="vs-plan-transition" data-plan-transition="${id}">${icon('transition', 15)}<select data-plan-field="transition.kind" data-plan-scene="${id}" data-plan-key="${id}:transition.kind" aria-label="${name}" title="${name}"${dis}>${kinds.map(item => option(s, item, tr(s, 'plan.kind.' + item, own(KINDS, item) ? KINDS[item] : item), item === kind)).join('')}</select>${blend}<div data-plan-issues="${id}:transition"></div></div>`;
    }
    function card(s, view, scene, index) {
        const id = esc(s, scene.id), dis = view.locked ? ' disabled' : '';
        const attrs = field => `data-plan-field="${field}" data-plan-scene="${id}" data-plan-key="${id}:${field}"`;
        const count = (view.plan.scenes || []).length;
        const generate = Number(scene.generate_seconds) || 0;
        const durations = !generate || view.durations.includes(generate) ? view.durations : view.durations.concat(generate).sort((a, b) => a - b);
        const keyframe = scene.keyframe || {};
        const narration = scene.narration || {};
        const kind = (scene.transition_in && scene.transition_in.kind) || 'cut';
        const joined = index > 0 && (kind === 'continue' || kind === 'bridge');
        const title = label(s, 'plan.title', 'Title');
        const number = esc(s, fill(tr(s, 'plan.sceneNumber', 'Scene {{number}}'), { number: index + 1 }));
        const negative = label(s, 'plan.negativePrompt', 'Negative prompt');
        const iconAction = (action, iconName, key, fallback, disabled, iconCls, cls) => `<button type="button"${cls ? ` class="${cls}"` : ''} data-plan-action="${action}" data-plan-scene="${id}" aria-label="${label(s, key, fallback)}" title="${label(s, key, fallback)}"${disabled ? ' disabled' : ''}>${icon(iconName, 14, iconCls)}</button>`;
        const actions = view.locked ? '' : `<div class="vs-clip-actions vs-plan-actions">`
            + `<button type="button" data-plan-action="revise" data-plan-scene="${id}">${icon('sparkle', 14)}<span>${label(s, 'plan.revise', 'Revise scene')}</span></button>`
            + `<button type="button" data-plan-action="text-only" data-plan-scene="${id}"${keyframe.source === 'none' || joined ? ' disabled' : ''}>${icon('text', 14)}<span>${label(s, 'plan.source.none', 'Text only')}</span></button>`
            + iconAction('up', 'chevronDown', 'plan.moveUp', 'Move up', index === 0, 'vs-icon vs-plan-up')
            + iconAction('down', 'chevronDown', 'plan.moveDown', 'Move down', index === count - 1)
            + iconAction('add-after', 'plus', 'plan.addScene', 'Add scene', count >= MAX_SCENES)
            + iconAction('remove', 'trash', 'plan.removeScene', 'Remove scene', false, undefined, 'vs-danger')
            + `</div>`;
        const narrationField = view.availability.tts || narration.text ? `<label class="vs-field"><span>${label(s, 'plan.narration', 'Narration')}</span><textarea ${attrs('narration.text')} rows="2" maxlength="600"${dis}>${esc(s, narration.text || '')}</textarea></label><div data-plan-issues="${id}:narration"></div>` : '';
        return `<article class="vs-plan-card" data-plan-card="${id}" aria-label="${number}">`
            + `<div class="vs-plan-card-head"><span class="vs-plan-num" aria-hidden="true">${index + 1}</span><input type="text" ${attrs('title')} maxlength="120" value="${esc(s, scene.title || '')}" placeholder="${title}" aria-label="${title}"${dis}><span class="vs-plan-chip" data-plan-duration="${id}">${esc(s, durationText(s, scene))}</span></div><div data-plan-issues="${id}:title"></div>`
            + `<label class="vs-field"><span>${label(s, 'plan.videoPrompt', 'Video prompt')}</span><textarea ${attrs('video_prompt')} rows="3" maxlength="${view.maxPrompt}"${dis}>${esc(s, scene.video_prompt || '')}</textarea></label><div data-plan-issues="${id}:video_prompt"></div>`
            + `<details class="vs-plan-neg" data-plan-negative="${id}"${view.openNegative.has(scene.id) ? ' open' : ''}><summary>${negative}</summary><textarea ${attrs('negative_prompt')} rows="2" maxlength="${view.maxPrompt}" aria-label="${negative}"${dis}>${esc(s, scene.negative_prompt || '')}</textarea></details><div data-plan-issues="${id}:negative_prompt"></div>`
            + `<div class="vs-plan-grid"><label class="vs-field"><span>${label(s, 'plan.generateSeconds', 'Generate')}</span><select ${attrs('generate_seconds')}${dis}>${generate ? '' : option(s, '', '–', true, true)}${durations.map(value => option(s, value, secondsLabel(s, value), value === generate)).join('')}</select></label><label class="vs-field"><span>${label(s, 'plan.useSeconds', 'Use (s)')}</span><input type="number" min="0.5" step="0.5"${generate ? ` max="${generate}"` : ''} ${attrs('use_seconds')} value="${esc(s, Number(scene.use_seconds) || '')}"${dis}></label></div><div data-plan-issues="${id}:timing"></div>`
            + keyframeField(s, view, scene, index, dis) + `<div data-plan-issues="${id}:keyframe"></div>`
            + narrationField
            + `<div data-plan-issues="${id}:card"></div>${actions}</article>`;
    }
    // Start image: a seamless or bridge scene shows its source read-only ("continues from scene n"); otherwise a choice of
    // generated (with image generation), library (with project images) and text only, plus a stored value the choice
    // lacks so its issue stays readable.
    function keyframeField(s, view, scene, index, dis) {
        const id = esc(s, scene.id);
        const keyframe = scene.keyframe || {};
        const kind = (scene.transition_in && scene.transition_in.kind) || 'cut';
        const name = label(s, 'plan.keyframe', 'Start image');
        const promptField = `<label class="vs-field"><span>${label(s, 'plan.imagePrompt', 'Image prompt')}</span><textarea data-plan-field="keyframe.image_prompt" data-plan-scene="${id}" data-plan-key="${id}:keyframe.image_prompt" rows="2" maxlength="${view.maxPrompt}"${dis}>${esc(s, keyframe.image_prompt || '')}</textarea></label>`;
        if (index > 0 && (kind === 'continue' || kind === 'bridge')) {
            const source = kind === 'continue' ? 'chain' : 'bridge';
            return `<div class="vs-field"><span>${name}</span><p class="vs-plan-fixed" data-plan-source="${source}">${icon('link', 14)} ${esc(s, fill(tr(s, 'plan.source.' + source, SOURCES[source]), { number: index }))}</p></div>${source === 'bridge' ? promptField : ''}`;
        }
        const chosen = keyframe.source || 'none';
        const sources = ['generated', 'library', 'none'].filter(value => value === chosen || value === 'none' || (value === 'generated' && view.availability.image_generation) || (value === 'library' && view.imageAssets.length > 0));
        if (!sources.includes(chosen)) sources.push(chosen);
        let extra = '';
        if (chosen === 'generated') extra = promptField;
        else if (chosen === 'library') {
            const assets = !keyframe.asset_id || view.imageAssets.some(asset => asset.id === keyframe.asset_id) ? view.imageAssets : view.imageAssets.concat({ id: keyframe.asset_id, name: keyframe.asset_id });
            extra = `<label class="vs-field"><span>${label(s, 'plan.libraryImage', 'Image')}</span><select data-plan-field="keyframe.asset_id" data-plan-scene="${id}" data-plan-key="${id}:keyframe.asset_id"${dis}>${option(s, '', tr(s, 'plan.chooseImage', 'Choose an image'), !keyframe.asset_id)}${assets.map(asset => option(s, asset.id, asset.name, asset.id === keyframe.asset_id)).join('')}</select></label>`;
        }
        return `<label class="vs-field"><span>${name}</span><select data-plan-field="keyframe.source" data-plan-scene="${id}" data-plan-key="${id}:keyframe.source"${dis}>${sources.map(value => option(s, value, fill(tr(s, 'plan.source.' + value, own(SOURCES, value) ? SOURCES[value] : value), { number: index }), value === chosen)).join('')}</select></label>${extra}`;
    }
    function bar(s, view) {
        if (view.phase === 'empty') return view.readonly ? '' : `<footer class="vs-plan-bar" data-plan-bar><div class="vs-plan-facts"></div><button type="submit" class="vs-primary" form="${esc(s, formId(s))}" data-plan-submit>${icon('sparkle', 16)}<span>${label(s, 'plan.create', 'Create plan')}</span></button></footer>`;
        if (!view.plan || (view.phase !== 'review' && view.phase !== 'planning')) return '';
        const replan = view.locked ? '' : `<button type="button" class="vs-primary" data-plan-action="replan">${icon('sparkle', 16)}<span>${label(s, 'plan.replan', 'Plan again')}</span></button>`;
        return `<footer class="vs-plan-bar" data-plan-bar><div class="vs-plan-facts" data-plan-facts>${facts(s, view)}</div>${replan}</footer>`;
    }
    function facts(s, view) {
        const plan = view.plan;
        const blocking = (view.issues || []).filter(issue => issue.blocking).length;
        const total = fill(tr(s, 'plan.total', '{{total}} s of {{target}} s'), { total: num(view.total), target: num(plan.target_seconds) });
        const count = P.trCount(s, 'plan.scenes', (plan.scenes || []).length, '1 scene', '{{count}} scenes');
        const problems = blocking ? P.trCount(s, 'plan.blocking', blocking, '1 problem to fix', '{{count}} problems to fix') : tr(s, 'plan.noBlocking', 'No problems');
        const saved = view.locked ? '' : view.saving ? tr(s, 'saving', 'Saving…') : view.dirty ? '' : tr(s, 'saved', 'All changes saved');
        return `<span data-plan-total>${icon('clock', 14)} ${esc(s, total)}</span><span data-plan-count>${icon('film', 14)} ${esc(s, count)}</span><span data-plan-blocking class="${blocking ? 'is-blocking' : 'is-ok'}">${icon(blocking ? 'alert' : 'check', 14)} ${esc(s, problems)}</span>${saved ? `<span data-plan-save>${esc(s, saved)}</span>` : ''}`;
    }
    // Live parts only (bottom bar, duration chips): an edit never redraws the fields, so the caret stays.
    function refresh(s, view) {
        const host = s.q('[data-plan-board]');
        if (!host || !view.plan) return;
        const box = host.querySelector('[data-plan-facts]');
        if (box) box.innerHTML = facts(s, view);
        (view.plan.scenes || []).forEach(scene => {
            const chip = host.querySelector(`[data-plan-duration="${CSS.escape(String(scene.id))}"]`);
            if (chip) chip.textContent = durationText(s, scene);
        });
    }
    function renderIssues(s, view) {
        const host = s.q('[data-plan-board]');
        if (!host) return;
        host.querySelectorAll('[data-plan-issues]').forEach(slot => slot.replaceChildren());
        if (view.plan) {
            const ids = new Set((view.plan.scenes || []).map(scene => scene.id));
            (view.issues || []).forEach(issue => {
                const code = String(issue.code || '');
                const field = String(issue.field || '');
                // prompt_too_long also names the image prompt (scenes[i].keyframe.image_prompt, 0-based) and the music
                // prompt (audio.music.prompt); the validator's field tells them apart.
                const slot = code === 'prompt_too_long' && field.endsWith('.keyframe.image_prompt') ? 'keyframe' : code === 'prompt_too_long' && field === 'audio.music.prompt' ? 'music' : own(CODE_SLOTS, code) ? CODE_SLOTS[code] : '';
                const scene = issue.scene_id && ids.has(issue.scene_id) ? issue.scene_id : '';
                const keys = scene ? [scene + ':' + (slot || 'card'), scene + ':card'] : ['plan:' + (PLAN_SLOTS.has(slot) ? slot : 'general'), 'plan:general'];
                const target = keys.map(key => host.querySelector(`[data-plan-issues="${CSS.escape(key)}"]`)).find(Boolean);
                if (!target) return;
                const text = tr(s, 'plan.issue.' + code, issue.message || code);
                target.insertAdjacentHTML('beforeend', `<p class="vs-plan-issue ${issue.blocking ? 'is-blocking' : 'is-warning'}" data-plan-issue-code="${esc(s, code)}">${icon(issue.blocking ? 'alert' : 'info', 13)}<span>${esc(s, text)}</span></p>`);
            });
        }
        refresh(s, view);
    }
    // "Plan again": the dialog is the confirmation; it starts from the current idea and target.
    function openStart(s, view, submit) {
        const plan = view.plan || {};
        const id = esc(s, 'vs-plan-start-' + s.id);
        const host = P.openModal(s, `<form class="vs-modal vs-plan-modal" data-plan-start-form role="dialog" aria-modal="true" aria-labelledby="${id}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${label(s, 'close', 'Close')}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('sparkle', 22)}</span><h2 id="${id}">${label(s, 'plan.replanTitle', 'Plan again?')}</h2><p>${label(s, 'plan.replanCopy', 'The agent plans a new storyboard. It replaces the current plan and your edits.')}</p>${ideaFields(s, plan.idea || '', Math.round(Number(plan.target_seconds)) || 30)}<div class="vs-modal-actions"><button type="button" data-modal-close>${label(s, 'cancel', 'Cancel')}</button><button type="submit" class="vs-primary">${icon('sparkle', 16)}<span>${label(s, 'plan.replan', 'Plan again')}</span></button></div></form>`);
        const form = host.querySelector('[data-plan-start-form]');
        form.addEventListener('submit', event => {
            event.preventDefault();
            const idea = form.querySelector('[data-plan-idea]').value.trim();
            const target = Number(form.querySelector('[data-plan-target]').value) || 30;
            if (!idea) return;
            host.replaceChildren();
            s.app.focus({ preventScroll: true });
            submit({ idea, target_seconds: target });
        });
    }
    function openRevise(s, view, sceneId, submit) {
        const index = ((view.plan && view.plan.scenes) || []).findIndex(scene => scene.id === sceneId);
        if (index < 0) return;
        const id = esc(s, 'vs-plan-revise-' + s.id);
        const host = P.openModal(s, `<form class="vs-modal vs-plan-modal" data-plan-revise-form role="dialog" aria-modal="true" aria-labelledby="${id}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${label(s, 'close', 'Close')}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('sparkle', 22)}</span><h2 id="${id}">${esc(s, fill(tr(s, 'plan.reviseTitle', 'Revise scene {{number}}'), { number: index + 1 }))}</h2><p>${label(s, 'plan.reviseCopy', 'Tell the agent what to change. It changes only this scene and its transitions.')}</p><label class="vs-field vs-field-wide"><span>${label(s, 'plan.reviseNote', 'What should change?')}</span><textarea data-plan-note rows="4" maxlength="1000" required></textarea></label><div class="vs-modal-actions"><button type="button" data-modal-close>${label(s, 'cancel', 'Cancel')}</button><button type="submit" class="vs-primary">${icon('sparkle', 16)}<span>${label(s, 'plan.revise', 'Revise scene')}</span></button></div></form>`);
        const form = host.querySelector('[data-plan-revise-form]');
        form.addEventListener('submit', event => {
            event.preventDefault();
            const note = form.querySelector('[data-plan-note]').value.trim();
            if (!note) return;
            host.replaceChildren();
            s.app.focus({ preventScroll: true });
            submit({ scene_id: sceneId, note });
        });
    }

    window.VideoStudioPlanBoard = { shellMarkup, render, refresh, renderIssues, openStart, openRevise };
})();

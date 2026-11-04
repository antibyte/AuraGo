// Storymaker app model: the pure part. JSON Pointer to text position (S.model.locate walks the JSON text, so an issue path
// finds its line in story.json), issue lists, the catalog index, the story edits of the inspector and the library (every edit
// works on a clone and keeps the document in the shape the server expects: no unknown field is ever added) and the frame
// helpers of the scrubber. No DOM; a Go test runs it in Node. Classic script, publishes S.model.
(function (root) {
    'use strict';

    const S = root.StorymakerParts = root.StorymakerParts || {};
    const M = S.model = {};

    // ---- JSON text positions ----------------------------------------------------------------------------------------------

    // parsePointer("/scenes/1/beats/0") -> ["scenes","1","beats","0"]; "" -> []; anything that is not a pointer -> null.
    function parsePointer(pointer) {
        if (typeof pointer !== 'string') return null;
        if (pointer === '') return [];
        if (pointer.charAt(0) !== '/') return null;
        return pointer.slice(1).split('/').map(token => token.replace(/~1/g, '/').replace(/~0/g, '~'));
    }

    function skipSpace(text, i) {
        while (i < text.length) {
            const c = text.charCodeAt(i);
            if (c === 32 || c === 9 || c === 10 || c === 13) i++; else break;
        }
        return i;
    }

    // End (exclusive) of the string that starts at text[i] === '"'.
    function stringEnd(text, i) {
        i++;
        while (i < text.length) {
            const c = text.charCodeAt(i);
            if (c === 92) i += 2;
            else if (c === 34) return i + 1;
            else i++;
        }
        return text.length;
    }

    // End (exclusive) of the JSON value that starts at text[i]. Tolerant: an unterminated value ends at the end of the text.
    function valueEnd(text, i) {
        const c = text.charAt(i);
        if (c === '"') return stringEnd(text, i);
        if (c === '{' || c === '[') {
            const close = c === '{' ? '}' : ']';
            i = skipSpace(text, i + 1);
            while (i < text.length) {
                const d = text.charAt(i);
                if (d === close) return i + 1;
                if (d === ',') { i = skipSpace(text, i + 1); continue; }
                if (c === '{') {
                    if (d !== '"') return text.length;
                    i = skipSpace(text, stringEnd(text, i));
                    if (text.charAt(i) !== ':') return text.length;
                    i = skipSpace(text, i + 1);
                }
                i = skipSpace(text, valueEnd(text, i));
            }
            return text.length;
        }
        let j = i;
        while (j < text.length && ',]} \t\r\n'.indexOf(text.charAt(j)) < 0) j++;
        return j > i ? j : Math.min(text.length, i + 1);
    }

    // locate(text, pointer) -> {from, to, found, depth}: the range of the value the pointer names. A pointer that leaves the
    // document (a missing field) gives the range of the deepest value that exists, with found false.
    function locate(text, pointer) {
        const tokens = parsePointer(pointer);
        const source = String(text == null ? '' : text);
        let from = skipSpace(source, 0);
        let to = valueEnd(source, from);
        if (!tokens) return { from, to, found: false, depth: 0 };
        let depth = 0;
        for (const token of tokens) {
            const c = source.charAt(from);
            let hit = null;
            if (c === '{') {
                let i = skipSpace(source, from + 1);
                while (i < to) {
                    const d = source.charAt(i);
                    if (d === '}') break;
                    if (d === ',') { i = skipSpace(source, i + 1); continue; }
                    if (d !== '"') break;
                    const keyEnd = stringEnd(source, i);
                    let key = '';
                    try { key = JSON.parse(source.slice(i, keyEnd)); } catch (_) { key = source.slice(i + 1, keyEnd - 1); }
                    i = skipSpace(source, keyEnd);
                    if (source.charAt(i) !== ':') break;
                    i = skipSpace(source, i + 1);
                    const end = valueEnd(source, i);
                    if (key === token) { hit = { from: i, to: end }; break; }
                    i = skipSpace(source, end);
                }
            } else if (c === '[') {
                const want = /^(0|[1-9][0-9]*)$/.test(token) ? Number(token) : -1;
                let i = skipSpace(source, from + 1), index = 0;
                while (i < to && want >= 0) {
                    const d = source.charAt(i);
                    if (d === ']') break;
                    if (d === ',') { i = skipSpace(source, i + 1); continue; }
                    const end = valueEnd(source, i);
                    if (index === want) { hit = { from: i, to: end }; break; }
                    index++;
                    i = skipSpace(source, end);
                }
            }
            if (!hit) return { from, to, found: false, depth };
            from = hit.from;
            to = hit.to;
            depth++;
        }
        return { from, to, found: true, depth };
    }

    // line (1-based) and column (1-based) of a character offset
    function lineColumn(text, offset) {
        const upTo = String(text).slice(0, Math.max(0, offset));
        const lines = upTo.split('\n');
        return { line: lines.length, column: lines[lines.length - 1].length + 1 };
    }

    // The line and column a decode message names ("... at line 3, column 14"), else null.
    function positionFromMessage(message) {
        const m = /line\s+(\d+)(?:\D+column\s+(\d+))?/i.exec(String(message || ''));
        if (!m) return null;
        return { line: Number(m[1]), column: m[2] ? Number(m[2]) : 1 };
    }

    // offset of a (line, column) pair, clamped into the text
    function offsetOf(text, line, column) {
        const source = String(text);
        let offset = 0;
        for (let n = 1; n < line; n++) {
            const next = source.indexOf('\n', offset);
            if (next < 0) return source.length;
            offset = next + 1;
        }
        const end = source.indexOf('\n', offset);
        const lineEnd = end < 0 ? source.length : end;
        return Math.min(offset + Math.max(0, (column || 1) - 1), lineEnd);
    }

    // ---- issues -----------------------------------------------------------------------------------------------------------

    const SEVERITY_ORDER = { error: 0, warning: 1, minor: 2 };

    function cleanIssue(issue) {
        if (!issue || typeof issue !== 'object') return null;
        const severity = SEVERITY_ORDER[issue.severity] !== undefined ? issue.severity : 'warning';
        return {
            severity,
            code: String(issue.code || ''),
            path: typeof issue.path === 'string' ? issue.path : '',
            message: String(issue.message || ''),
            scene_id: typeof issue.scene_id === 'string' ? issue.scene_id : '',
            fix: typeof issue.fix === 'string' ? issue.fix : ''
        };
    }

    // mergeIssues(...lists): one list, no duplicates (severity, code, path, message), errors first, order kept within a severity.
    function mergeIssues() {
        const seen = new Set();
        const out = [];
        Array.prototype.slice.call(arguments).forEach(list => {
            (Array.isArray(list) ? list : []).forEach(raw => {
                const issue = cleanIssue(raw);
                if (!issue) return;
                const key = issue.severity + '|' + issue.code + '|' + issue.path + '|' + issue.message;
                if (seen.has(key)) return;
                seen.add(key);
                out.push(issue);
            });
        });
        return out
            .map((issue, order) => ({ issue, order }))
            .sort((a, b) => SEVERITY_ORDER[a.issue.severity] - SEVERITY_ORDER[b.issue.severity] || a.order - b.order)
            .map(entry => entry.issue);
    }

    // Phase 1 renders are silent, so a scene without narration audio is not worth showing.
    function visibleIssues(list) {
        return (Array.isArray(list) ? list : []).filter(issue => issue.code !== 'narration_missing');
    }

    // The render browser as an export needs it, from GET /status: "ready" (a launch probe succeeded), "download" (none is ready but the
    // first export downloads the server's pinned Chromium, status.browser.can_download) or "missing".
    function browserState(status) {
        const browser = status && status.browser || {};
        if (browser.ready === true) return 'ready';
        return browser.can_download === true ? 'download' : 'missing';
    }

    // The size of the exported MP4 by aspect, the rule of render.CanvasSize (the preview compiles at a smaller width).
    function exportSize(aspect) {
        if (aspect === '9:16') return [1080, 1920];
        if (aspect === '1:1') return [1080, 1080];
        return [1920, 1080];
    }

    function issueCounts(list) {
        const counts = { error: 0, warning: 0, minor: 0 };
        (Array.isArray(list) ? list : []).forEach(issue => { if (counts[issue.severity] !== undefined) counts[issue.severity]++; });
        return counts;
    }

    // The scene index an issue belongs to (scene_id first, else /scenes/N in its path), -1 when it is not about a scene.
    function sceneIndexOfIssue(issue, story) {
        const scenes = story && Array.isArray(story.scenes) ? story.scenes : [];
        if (issue && issue.scene_id) {
            const byId = scenes.findIndex(scene => scene && scene.id === issue.scene_id);
            if (byId >= 0) return byId;
        }
        const m = /^\/scenes\/(\d+)(?:\/|$)/.exec(issue && issue.path || '');
        if (m && Number(m[1]) < scenes.length) return Number(m[1]);
        return -1;
    }

    // ---- catalog ----------------------------------------------------------------------------------------------------------

    // indexCatalog(catalog) -> {version, byId, byKind}; unknown shapes give an empty index.
    function indexCatalog(catalog) {
        const index = { version: '', byId: {}, byKind: {}, fonts: [], palettes: [], stage: null, rig: null };
        if (!catalog || typeof catalog !== 'object') return index;
        index.version = String(catalog.version || '');
        index.fonts = Array.isArray(catalog.fonts) ? catalog.fonts : [];
        index.palettes = Array.isArray(catalog.palettes) ? catalog.palettes : [];
        index.stage = catalog.stage || null;
        index.rig = catalog.rig || null;
        (Array.isArray(catalog.entries) ? catalog.entries : []).forEach(entry => {
            if (!entry || typeof entry.id !== 'string' || typeof entry.kind !== 'string') return;
            index.byId[entry.id] = entry;
            (index.byKind[entry.kind] = index.byKind[entry.kind] || []).push(entry);
        });
        return index;
    }

    function entriesOf(index, kind) {
        return index && index.byKind && index.byKind[kind] ? index.byKind[kind] : [];
    }

    // The spots of an environment in the order the catalog lists them.
    function spotNames(entry) {
        return entry && entry.spots && typeof entry.spots === 'object' ? Object.keys(entry.spots) : [];
    }

    // ---- story edits ------------------------------------------------------------------------------------------------------

    function cloneStory(story) {
        return JSON.parse(JSON.stringify(story));
    }

    function canonicalText(story) {
        return JSON.stringify(story, null, 2) + '\n';
    }

    function scenesOf(story) {
        return story && Array.isArray(story.scenes) ? story.scenes : [];
    }

    // s1, s2 ... the smallest number above the count that is not taken
    // newSceneId(story, prefix, alsoTaken): the next free id. alsoTaken are ids that exist outside the story: the server keeps a code
    // scene's file when its scene is deleted (there is no DELETE for code files), and a new code scene is created with If-None-Match: *.
    function newSceneId(story, prefix, alsoTaken) {
        const taken = new Set(scenesOf(story).map(scene => scene && scene.id).concat(alsoTaken || []));
        const base = prefix || 's';
        for (let n = scenesOf(story).length + 1; n < 100000; n++) if (!taken.has(base + n)) return base + n;
        return base + Date.now();
    }

    function addScene(story, afterIndex, options) {
        const o = options || {};
        const scenes = story.scenes = scenesOf(story);
        const at = Math.min(Math.max(afterIndex + 1, 0), scenes.length);
        const previous = scenes[Math.min(afterIndex, scenes.length - 1)];
        const scene = { id: newSceneId(story, o.kind === 'code' ? 'code' : 's', o.taken) };
        if (o.kind === 'code') scene.kind = 'code';
        else if (previous && previous.environment) scene.environment = previous.environment;
        scenes.splice(at, 0, scene);
        return at;
    }

    function removeScene(story, index) {
        const scenes = scenesOf(story);
        if (index < 0 || index >= scenes.length || scenes.length <= 1) return -1;
        scenes.splice(index, 1);
        return Math.min(index, scenes.length - 1);
    }

    function moveScene(story, from, to) {
        const scenes = scenesOf(story);
        if (from < 0 || from >= scenes.length || to < 0 || to >= scenes.length || from === to) return -1;
        const [scene] = scenes.splice(from, 1);
        scenes.splice(to, 0, scene);
        return to;
    }

    // Empty values leave the document: the server's defaults (and omitted fields) stay the single source of the empty state.
    function setSceneField(scene, name, value) {
        if (value === '' || value == null) delete scene[name];
        else scene[name] = value;
    }

    // durations: "auto" or a number of seconds; anything else reads as auto
    function durationOf(scene) {
        const d = scene ? scene.duration : undefined;
        if (typeof d === 'number' && d > 0) return { auto: false, seconds: d };
        return { auto: true, seconds: 0 };
    }

    function setDuration(scene, auto, seconds) {
        if (auto) scene.duration = 'auto';
        else scene.duration = Math.max(0.5, Math.round(Number(seconds) * 10) / 10 || 5);
    }

    function castSlug(entry) {
        const raw = String(entry.id).replace(/^[a-z]+\./, '').toLowerCase().replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '');
        return (raw || 'actor').slice(0, 48);
    }

    // insertActor(story, sceneIndex, characterEntry, displayName) -> {ok, castId, reason}; the same cast member cannot stand
    // twice in a scene (the validator calls that a duplicate id).
    function insertActor(story, sceneIndex, entry, displayName) {
        const scene = scenesOf(story)[sceneIndex];
        if (!scene || scene.kind === 'code' || !entry) return { ok: false, reason: 'no_scene' };
        story.cast = Array.isArray(story.cast) ? story.cast : [];
        let member = story.cast.find(candidate => candidate && candidate.character === entry.id);
        if (!member) {
            member = { id: castSlug(entry), name: String(displayName || entry.name || entry.id).slice(0, 80), character: entry.id };
            let n = 2;
            while (story.cast.some(candidate => candidate && candidate.id === member.id)) member.id = castSlug(entry).slice(0, 44) + '-' + n++;
            story.cast.push(member);
        }
        scene.actors = Array.isArray(scene.actors) ? scene.actors : [];
        if (scene.actors.some(actor => actor && actor.cast === member.id)) return { ok: false, reason: 'already_in_scene', castId: member.id };
        scene.actors.push({ cast: member.id });
        return { ok: true, castId: member.id };
    }

    // The first of the base spots (right, left, ...) that nothing stands on yet, else the centre.
    function freeSpot(scene, environmentEntry) {
        const used = new Set();
        (Array.isArray(scene.actors) ? scene.actors : []).forEach(actor => { if (actor && typeof actor.at === 'string') used.add(actor.at); });
        (Array.isArray(scene.props) ? scene.props : []).forEach(prop => { if (prop && typeof prop.at === 'string') used.add(prop.at); });
        const offered = spotNames(environmentEntry);
        const order = ['right', 'left', 'far-right', 'far-left', 'center'].filter(name => !offered.length || offered.indexOf(name) >= 0);
        return order.find(name => !used.has(name)) || 'center';
    }

    function insertProp(story, sceneIndex, entry, environmentEntry) {
        const scene = scenesOf(story)[sceneIndex];
        if (!scene || scene.kind === 'code' || !entry) return { ok: false, reason: 'no_scene' };
        scene.props = Array.isArray(scene.props) ? scene.props : [];
        scene.props.push({ item: entry.id, at: freeSpot(scene, environmentEntry) });
        return { ok: true };
    }

    function removeActor(story, sceneIndex, k) {
        const scene = scenesOf(story)[sceneIndex];
        if (!scene || !Array.isArray(scene.actors) || k < 0 || k >= scene.actors.length) return false;
        scene.actors.splice(k, 1);
        if (!scene.actors.length) delete scene.actors;
        return true;
    }

    function removeProp(story, sceneIndex, k) {
        const scene = scenesOf(story)[sceneIndex];
        if (!scene || !Array.isArray(scene.props) || k < 0 || k >= scene.props.length) return false;
        scene.props.splice(k, 1);
        if (!scene.props.length) delete scene.props;
        return true;
    }

    // ---- frames -----------------------------------------------------------------------------------------------------------

    // The scene that is on screen at a frame: the last one that has started (transitions overlap, so the later wins).
    function sceneAtFrame(report, frame) {
        const scenes = report && Array.isArray(report.scenes) ? report.scenes : [];
        let found = -1;
        for (let i = 0; i < scenes.length; i++) if (scenes[i].start_frame <= frame) found = i;
        return found;
    }

    // The frame a click on a scene lands on: 40 % into the scene, like the thumbnail.
    function frameForScene(report, index) {
        const scenes = report && Array.isArray(report.scenes) ? report.scenes : [];
        const scene = scenes[index];
        if (!scene) return 0;
        const length = Math.max(1, scene.end_frame - scene.start_frame);
        return Math.min(Math.max(0, report.frame_count - 1), scene.start_frame + Math.floor(length * 0.4));
    }

    function previewWidth(aspect) {
        if (aspect === '9:16') return 540;
        if (aspect === '1:1') return 720;
        return 960;
    }

    function aspectRatio(aspect) {
        if (aspect === '9:16') return 9 / 16;
        if (aspect === '1:1') return 1;
        return 16 / 9;
    }

    M.parsePointer = parsePointer;
    M.locate = locate;
    M.lineColumn = lineColumn;
    M.positionFromMessage = positionFromMessage;
    M.offsetOf = offsetOf;
    M.mergeIssues = mergeIssues;
    M.visibleIssues = visibleIssues;
    M.issueCounts = issueCounts;
    M.exportSize = exportSize;
    M.browserState = browserState;
    M.sceneIndexOfIssue = sceneIndexOfIssue;
    M.indexCatalog = indexCatalog;
    M.entriesOf = entriesOf;
    M.spotNames = spotNames;
    M.cloneStory = cloneStory;
    M.canonicalText = canonicalText;
    M.scenesOf = scenesOf;
    M.newSceneId = newSceneId;
    M.addScene = addScene;
    M.removeScene = removeScene;
    M.moveScene = moveScene;
    M.setSceneField = setSceneField;
    M.durationOf = durationOf;
    M.setDuration = setDuration;
    M.insertActor = insertActor;
    M.insertProp = insertProp;
    M.removeActor = removeActor;
    M.removeProp = removeProp;
    M.freeSpot = freeSpot;
    M.sceneAtFrame = sceneAtFrame;
    M.frameForScene = frameForScene;
    M.previewWidth = previewWidth;
    M.aspectRatio = aspectRatio;
})(typeof globalThis !== 'undefined' ? globalThis : window);

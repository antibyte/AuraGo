/* Runenpfad journal story: the speakers and graphs of Edda's chronicle round. DOM-free data only.
 * Virtual speakers without a body in the room (journal, lore, core; opened through RunenpfadDialogue.openVirtual), the two epilogue
 * folk (Edda, Gundar), the observatory chapter of the brass helper (meeting automaton_hall) and the core's last question.
 * Appends to RunenpfadNpcStory (after folk-story.js) and RunenpfadNpcs.NPCS. Graphs marked `virtual: true` have no meeting start:
 * dialogue.js counts all their nodes as reachable. Texts are in the locale files (runenpfad.npc.<npc>.<node>[.c<n>]). */
(function (root) {
    'use strict';

    const S = root.RunenpfadNpcStory || (root.RunenpfadNpcStory = { MEETINGS: [], GRAPHS: {} });
    const MEETINGS = S.MEETINGS;
    const GRAPHS = S.GRAPHS;
    const FAREWELL = 'runenpfad.npc.c.farewell';
    const READ_ON = 'runenpfad.npc.c.read_on';
    const CLOSE_BOOK = 'runenpfad.npc.c.close_book';
    const STEP_BACK = 'runenpfad.npc.c.step_back';
    const PLAY = 'runenpfad.npc.c.play';
    const HUMAN = { pack: 'human-characters-animated', anim: 'ranger_idle' };

    // ---- speakers ----
    const SPEAKERS = {
        journal: { color: '#e8d9b0', portrait: 'edda' },
        lore: { color: '#b8b09a' },
        core: { color: '#ffb347', fallback: { pack: 'robots-drones-animated', anim: 'service_robot_idle' } },
        edda: { color: '#9ad8ff' },
        gambler: { color: '#c4b5fd' }
    };
    const npcs = root.RunenpfadNpcs;
    if (npcs && npcs.NPCS) {
        Object.keys(SPEAKERS).forEach(function (id) {
            const sp = SPEAKERS[id];
            const fb = sp.fallback || HUMAN;
            const def = {
                faction: null, nameKey: 'runenpfad.npc.' + id + '.name', titleKey: 'runenpfad.npc.' + id + '.title', color: sp.color, height: 74,
                fallback: { pack: fb.pack, anim: fb.anim, tint: sp.fallback ? null : sp.color }, offers: [], duelCoins: 0
            };
            if (sp.portrait) def.portrait = sp.portrait;
            npcs.NPCS[id] = def;
        });
    }

    // ---- journal: ten pages, each one two nodes (p<n>a, p<n>b) ----
    const journal = {};
    for (let n = 1; n <= 10; n++) {
        journal['p' + n + 'a'] = { choices: [{ text: READ_ON, to: 'p' + n + 'b' }] };
        journal['p' + n + 'b'] = { choices: [{ text: CLOSE_BOOK, to: null }] };
    }
    GRAPHS.journal = { virtual: true, nodes: journal };

    // ---- lore: readable inscriptions, one node per sign ----
    const lore = {};
    ['tomb_mural', 'grotto_mark', 'den_scratch', 'ice_note', 'boiler_note', 'scroll_1', 'scroll_2', 'scroll_3', 'obs_chart', 'obs_plaque', 'obs_gears', 'obs_door'].forEach(function (id) {
        lore[id] = { choices: [{ text: STEP_BACK, to: null }] };
    });
    GRAPHS.lore = { virtual: true, nodes: lore };

    // ---- the core's last question: three answers, then the moment is released (once: obs.done) ----
    MEETINGS.push({ id: 'core_last', npcId: 'core', levels: [], starts: [{ to: 'query' }] });
    const answer = function (n) {
        return { enter: [{ flag: { 'obs.answer': n } }], choices: [
            { text: 'runenpfad.npc.core.let_go', if: { notFlag: 'obs.done' }, to: 'release' },
            { text: FAREWELL, to: null }] };
    };
    GRAPHS.core = {
        nodes: {
            // "QUERY. One walker falls. If she falls, nobody walks. If nobody walks, the path does not wear. But the path is for walking. The sum does not close. State the answer."
            query: { choices: [
                { to: 'a_catch' },   // "She does not fall. Someone catches her."
                { to: 'a_walk' },   // "A path is kept by walking it, not by counting it."
                { to: 'a_wrong' }] },   // "Your sum was wrong from the first line."
            // "NEW TERM: someone catches her. ..."
            a_catch: answer(1),
            // "NEW RULE: kept by walking. ..."
            a_walk: answer(2),
            // "CHECKING LINE ONE. 'Nobody walks' was never the goal. ..."
            a_wrong: answer(3),
            // "RELEASING THE MOMENT. The path may wear. The path ..."
            release: { mood: 'happy', choices: [
                { effects: [{ flag: 'obs.done' }, { coins: 100 }, { ui: 'scene:true_end' }] }] }   // "Go home."
        }
    };

    // ---- the brass helper's chapter in the wardens' hall: Cog opens the observatory ----
    if (GRAPHS.automaton) MEETINGS.push({ id: 'automaton_hall', npcId: 'automaton', levels: [], starts: [
        { if: { flag: 'obs.done' }, to: 'done' },
        { if: { flag: 'obs.open' }, to: 'wait' },
        { to: 'call' }] });
    if (GRAPHS.automaton) Object.assign(GRAPHS.automaton.nodes, {
        // "Click. Warden. The core is quiet. The core is not finished. One gear still turns. ..."
        call: { marker: 'quest', choices: [
            { to: 'where' },   // "Where is it?"
            { text: FAREWELL, to: null }] },
        // "Under the old observatory in the desert, behind the Sandstone Bastion. ..."
        where: { choices: [
            { effects: [{ flag: 'obs.open' }, { notice: 'runenpfad.notice.chapter_open' }, { ui: 'scene:obs_intro' }] }] },   // "I will go."
        // "The observatory door is open. Whirr. Bring Edda's pages, all ten. ..."
        wait: { choices: [
            { text: FAREWELL, to: null }] },
        // "Click. The gear has stopped. The sum is closed. ..."
        done: { mood: 'happy', marker: 'heart', choices: [
            { text: FAREWELL, to: null }] }
    });

    // ---- Gundar, the dice player in the tavern ----
    MEETINGS.push({ id: 'gambler_home', npcId: 'gambler', levels: [], starts: [{ to: 'hello' }] });
    GRAPHS.gambler = {
        nodes: {
            // "Gundar. I roll dice and I tell lies, and only one of those is a game."
            hello: { marker: 'talk', choices: [
                { text: PLAY, effects: [{ ui: 'dice:gambler' }] },
                { text: FAREWELL, to: null }] }
        }
    };

    // ---- Edda, back in the hall after the true ending ----
    MEETINGS.push({ id: 'edda_home', npcId: 'edda', levels: [], starts: [
        { if: { notFlag: 'edda.met' }, to: 'hello' },
        { to: 'again' }] });
    GRAPHS.edda = {
        nodes: {
            // "So. You read my journal. All of it? ..."
            hello: { mood: 'happy', marker: 'heart', enter: [{ flag: 'edda.met' }], choices: [
                { to: 'skadi' },   // "What about Skadi?"
                { to: 'now' },   // "What happens now?"
                { text: FAREWELL, to: null }] },
            // "I asked a machine to walk the road for us, and you ..."
            again: { mood: 'happy', choices: [
                { text: 'runenpfad.npc.edda.hello.c1', to: 'skadi' },   // "What about Skadi?"
                { text: 'runenpfad.npc.edda.hello.c2', to: 'now' },   // "What happens now?"
                { text: FAREWELL, to: null }] },
            // "Sulking on the hall roof. Three catches now. ..."
            skadi: { choices: [
                { text: FAREWELL, to: null }] },
            // "Two children asked me this morning how one becomes a warden. ..."
            now: { choices: [
                { text: FAREWELL, to: null }] }
        }
    };
})(typeof window !== 'undefined' ? window : globalThis);

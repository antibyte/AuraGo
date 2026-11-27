/* Runenpfad ember story, the villagers: how the established residents of Pathrest, Edda and Cog react to the ember dragon chapter
 * (round 11). DOM-free data only. Their meetings get extra starts (inserted before the ordinary starts, behind the one-time ones) and
 * their graphs get the nodes dr_s1 (step 1: the well steams), dr_s2 (step 2: the night of the roar), dr_slain and dr_sealed (step 5, by fate).
 * Comments hold the English source text for the i18n files. Loaded after journal-story.js and ember-story.js; every addition is skipped
 * when its meeting or graph is missing. The conditions step, fate, tablets and seen are read by dialogue.js through ember.js. */
(function (root) {
    'use strict';

    const S = root.RunenpfadNpcStory || (root.RunenpfadNpcStory = { MEETINGS: [], GRAPHS: {} });
    const MEETINGS = S.MEETINGS;
    const GRAPHS = S.GRAPHS;
    const FAREWELL = { text: 'runenpfad.npc.c.farewell', to: null };
    const THANKS = { text: 'runenpfad.npc.c.thanks', to: null };
    const TRADE = { text: 'runenpfad.npc.c.trade', effects: [{ trade: true }] };
    const CHRONICLE = { text: 'runenpfad.npc.elder.again.c1', effects: [{ ui: 'chronicle' }] };
    const WORKSHOP = { text: 'runenpfad.npc.smith.hello.c1', effects: [{ ui: 'workshop' }] };
    const SONG = { text: 'runenpfad.npc.bard.ask_song', to: 'songs' };
    const OLDER = { text: 'runenpfad.npc.bard.r2.c1', to: 'r2' };
    const PLAY = { text: 'runenpfad.npc.c.play', effects: [{ ui: 'dice:gambler' }] };
    const RATS = { text: 'runenpfad.npc.innkeeper.hello.c2', if: { not: { loot: 'tavern_cellar' } }, to: 'rats' };
    const NOW = { text: 'runenpfad.npc.edda.hello.c2', to: 'now' };
    const SLAIN = { fate: 'slain' };
    const SEALED = { fate: 'sealed' };

    // Step starts, highest first: the fate starts cover step 5, step 2 covers 2 and 3, step 1 the first night of steam.
    // `first` starts go before the fate starts (items in the pack), `mid` between the fate and the step starts.
    function dr(first, mid) {
        return (first || []).concat([
            { if: SLAIN, to: 'dr_slain' },
            { if: SEALED, to: 'dr_sealed' }], mid || [], [
            { if: { step: 2 }, to: 'dr_s2' },
            { if: { step: 1 }, to: 'dr_s1' }]);
    }
    // Inserts starts before the start that leads to `anchor` (the last start when the anchor is not found, index 0 without one).
    function insert(meetingId, anchor, starts) {
        const m = MEETINGS.filter(function (x) { return x.id === meetingId; })[0];
        if (!m) return;
        let at = 0;
        if (anchor) {
            at = m.starts.map(function (st) { return st.to; }).indexOf(anchor);
            if (at < 0) at = m.starts.length - 1;
        }
        m.starts.splice.apply(m.starts, [at, 0].concat(starts));
    }
    function add(npcId, nodes) {
        if (GRAPHS[npcId]) Object.assign(GRAPHS[npcId].nodes, nodes);
    }

    // ---- Runa: the archive (step 1), the night (step 2), the legend after three tablets, the aftermath ----
    insert('elder_home', 'epilogue',
        dr([{ if: { step: 2, tablets: 3, not: { seen: 'dr_legend' } }, to: 'legend' }]));
    add('elder', {
        // "Have you seen the well? It steams in the frost, and at night my cups walk toward the edge of the shelf. I have opened the wardens' archive in the hall. Two tablets lie there that nobody has read in a hundred years. I would like them read."
        dr_s1: { choices: [CHRONICLE, FAREWELL] },
        // "The barn burned, the bell cracked, and Tilda's girl is down that well. I will not say 'dragon' until someone shows me one. But I have stopped saying 'nothing'. Read the tablets, child. Whoever wrote them was afraid of the same thing."
        dr_s2: { choices: [CHRONICLE, FAREWELL] },
        // "Three tablets read already? Then you know more than I do, and you ought to hear the rest from someone who loves the tale. Sit, child. This is how Pathrest began."
        legend: { marker: 'quest', choices: [
            { effects: [{ ui: 'scene:dr_legend' }] },   // "Tell me."
            { to: null }] },   // "Later."
        // "The barn is rebuilt, the well is calm, and the village is cheering. I let it. But I heard the frost wind on the pass last night, and it sounded like grief. You ended a danger, child, and an old being with it. Both are true. Thank you."
        dr_slain: { choices: [CHRONICLE, FAREWELL] },
        // "The bell rang through the whole valley, and I felt the floor settle like a cat going to sleep. Pathrest is a watch post again, as it was meant to be. Skadi came down from the pass to listen. I have never seen her so close to the roofs."
        dr_sealed: { mood: 'happy', choices: [CHRONICLE, FAREWELL] }
    });

    // ---- Berta ----
    insert('innkeeper_home', null, dr());
    add('innkeeper', {
        // "Even the milk in my cold cellar has gone warm, and the kettle boils before I light the fire. Oswin would say it is talking to him. I am starting to believe Oswin. Sit down, eat something. Stew settles the nerves."
        dr_s1: { choices: [TRADE, RATS, FAREWELL] },
        // "Two kobolds in my cellar, an old miner at my corner table, and half my barn in the yard as ash. Nokk and Pip eat like sparrows and say sorry for every crumb. Best guests I ever had. The kitchen hearth is yours, if you bring me honest ingredients."
        dr_s2: { choices: [TRADE, RATS, FAREWELL] },
        // "The Rusty Rune has not been this full since the road broke! They call it dragon-slayer stew, which is nonsense, it is the same stew. Skadi sat on the hall roof all night. I left a bowl on the sill, and by morning it was frozen solid. I took it as thanks. I took it as a scolding, too."
        dr_slain: { choices: [TRADE, FAREWELL] },
        // "The bell had hardly stopped before every cup in the house sat still. Do you know how long it has been since my cups sat still? Eat, sit. The kobolds are settling in and Pip has claimed my best chair. And Skadi listened from the hall roof until dark. I left her a bowl. It came back frozen. I am told that means thank you."
        dr_sealed: { mood: 'happy', choices: [TRADE, FAREWELL] }
    });

    // ---- Tilda: Mira is missing (step 2), found, or home again ----
    insert('shopkeeper_home', null, dr(null, [{ if: { flag: 'mira.found', not: { step: 5 } }, to: 'dr_found' }]));
    add('shopkeeper', {
        // "Prices up, nerves down: that is what sells this week. Mira keeps leaning over the well to see why it is warm. She is eight, she asks why about everything, but now she asks the well. If you see her near it, send her home."
        dr_s1: { choices: [TRADE, FAREWELL] },
        // "Mira is gone. She went down the well after that warm light and left her lantern on the rim. Eight years old, brave, cheeky, and I have never wanted so badly to scold someone. Find her, warden. Please. Name your price. No. Never mind a price."
        dr_s2: { choices: [TRADE, FAREWELL] },
        // "She came home with soot on her nose and a story about a dragon. I have not decided whether to hug her or ground her, so I did both. You brought my girl back. Fair prices for you, always. The thanks is free."
        dr_found: { mood: 'happy', marker: 'heart', choices: [TRADE, FAREWELL] },
        // "Mira did not cheer with the others. She sat on the well rim with her feet in the air and asked me whether it hurt him. I did not know what to say. Children see the part of a story the rest of us step over. I am proud of her, and of you. Both are awkward things to say aloud."
        dr_slain: { choices: [TRADE, FAREWELL] },
        // "Mira swears the well hums a lullaby now. I told her wells do not hum. She told me to put my ear against the stones. I did. Do not tell anyone. I have my daughter, a quiet well and a lullaby, and I owe you for all three."
        dr_sealed: { mood: 'happy', choices: [TRADE, FAREWELL] }
    });

    // ---- Hakon: the bell bronze and the dragon scale are his to work ----
    insert('smith_home', null, dr([
        { if: { item: { bell_bronze: 1 }, notFlag: 'dr.bell' }, to: 'dr_bronze' },
        { if: { item: { dragon_scale: 1 }, notFlag: 'dr.shield' }, to: 'dr_scale' }]));
    add('smith', {
        // "The anvil rings wrong. Hit iron and it answers with a hum underneath that is not mine. Thirty years I have known this anvil's voice. Something below is answering it."
        dr_s1: { choices: [WORKSHOP, FAREWELL] },
        // "That cracked bell will not mend with talk. It wants real bell bronze, the old stuff, and bronze like that came out of the deep galleries. Bring me some and I will pour it. And a dragon's scale would make a shield worth the name, if anybody were mad enough to fetch one."
        dr_s2: { choices: [WORKSHOP, FAREWELL] },
        // "That is it. Old bronze, and it still rings when I flick it. Come to the forge room, I will show you the strike. Watch the needle, hit when it crosses the middle. Three blows. Two good ones will do."
        dr_bronze: { marker: 'quest', choices: [WORKSHOP, FAREWELL] },
        // "A dragon scale. Big as a dinner plate, and warm to the hand. Forty coins for the coal and I will make a shield that turns fire. Same forge, same strike. Do not drop it on your foot."
        dr_scale: { marker: 'quest', choices: [WORKSHOP, FAREWELL] },
        // "The anvil is quiet. Quiet like a held breath. I thought I would be glad. I put my hand on it and it was cold, not wrong cold, just empty. Odd thing to miss, a hum. Do not repeat that."
        dr_slain: { choices: [WORKSHOP, FAREWELL] },
        // "The bell rings true now: bronze, copper and a little Hakon. The anvil hums again, softly, like a sleeper snoring. I find I can work to it. Better than the quiet I feared."
        dr_sealed: { mood: 'happy', choices: [WORKSHOP, FAREWELL] }
    });

    // ---- Finn ----
    insert('bard_home', 'song', dr());
    add('bard', {
        // "A new rumour! The well steams, the cups walk, and I am writing a song with the word 'ominous' in it. Berta says nothing rhymes with 'ominous'. Berta is wrong. I just have not found it yet."
        dr_s1: { mood: 'happy', choices: [SONG, OLDER, THANKS] },
        // "I was mid-verse when the roar came. I did not finish the verse. I have a new respect for songs that end 'and then the dragon'. Strange thing: my grandmother had a lullaby about four lights and a bell. I only remember the tune. It would not leave my head all night."
        dr_s2: { choices: [SONG, OLDER, THANKS] },
        // "I finished the song: 'The Fall of the Ember Wyrm', twelve verses, and people clap. But I stop at verse nine every time. Skadi sits on the hall roof and listens, and I cannot sing it to her face. I am cutting verse nine. Perhaps the whole song."
        dr_slain: { choices: [SONG, OLDER, THANKS] },
        // "'The Song That Rang the Bell', eleven verses, and every one ends with Brandur asleep, which is a poor climax and a very good outcome. Skadi listened to all eleven. She did not cry. She hummed along a little off key, and it was the loveliest thing I have heard."
        dr_sealed: { mood: 'happy', choices: [SONG, OLDER, THANKS] }
    });

    // ---- Gundar ----
    insert('gambler_home', null, dr());
    add('gambler', {
        // "My dice rolled themselves off the table twice today. Double six both times. I do not trust a table that cheats better than I do."
        dr_s1: { choices: [PLAY, FAREWELL] },
        // "Someone burned Berta's barn and it was not me, and nobody believes me, which is a first. I will bet you ten coins you do not come back from that well. No. I have bet against you before. I would lose."
        dr_s2: { choices: [PLAY, FAREWELL] },
        // "I bet against you and lost again, four coins to Finn, three to Berta. Then I put one more on the well rim, for the dragon. Silly. But he played a long game and lost it, and a gambler tips his hat to that."
        dr_slain: { choices: [PLAY, FAREWELL] },
        // "I bet Finn two coins that the bell would not carry. It rang and rang. I paid up laughing. You do not often lose to a bell. Sit with me later, I will show you a dice trick. It does not work on anyone who knows you."
        dr_sealed: { mood: 'happy', choices: [PLAY, FAREWELL] }
    });

    // ---- Edda, back in the hall after the observatory: the dragon and her friend Skadi ----
    insert('edda_home', 'again', dr());
    add('edda', {
        // "The well steams, and my old cups walk. I wrote 'warm' in a margin thirty years ago and never asked why. Runa has the archive open now, little one. Read what the first wardens left. I did, and I do not think it is only a story."
        dr_s1: { choices: [NOW, FAREWELL] },
        // "So you have been down my well, I can tell by the soot, little one. Forty years I have lived on top of that and never looked. I count waystones, and I never counted what lies under my own floor. Take the map from my study. It is a good map. Be polite down there."
        dr_s2: { choices: [NOW, FAREWELL] },
        // "You ended it, then. I will not say you should not have. I will say Skadi sat on the hall roof all night and did not sulk, and that is how I know. She sat with her wings folded and said nothing. Her brother, little one. I have buried friends and I still do not know what to say."
        dr_slain: { choices: [NOW, FAREWELL] },
        // "You sang him back to sleep. Good. Skadi came down from the pass and asked me to read her the tablet of the twins. Twice. Then she flew to the well and breathed frost on its stones, very gently. In her way, that is a thank-you and a lullaby."
        dr_sealed: { mood: 'happy', choices: [NOW, FAREWELL] }
    });

    // ---- Cog: a question on each of his nodes (the quest nodes call, wait and the done nodes), then the answers ----
    function link(npcId, nodeIds, choices) {
        const g = GRAPHS[npcId];
        if (!g) return;
        nodeIds.forEach(function (id) {
            const list = g.nodes[id] && g.nodes[id].choices;
            if (list) list.splice.apply(list, [list.length - 1, 0].concat(choices));
        });
    }
    link('automaton', ['call', 'wait', 'done', 'done_1', 'done_2', 'done_3'], [
        { text: 'runenpfad.npc.automaton.dr_ask', if: { step: 1, not: { step: 5 } }, to: 'dr_s2' },   // "What does Cog hear in the roots?"
        { text: 'runenpfad.npc.automaton.dr_ask', if: { step: 5 }, to: 'dr_s5' }]);
    add('automaton', {
        // "Click. Cog listens to the roots every night. Something large breathes down there, slow and warm. Whirr. It is not a gear. Cog checked. Cog does not like things that breathe without being wound."
        dr_s2: { choices: [FAREWELL] },
        // "Click. Cog listened to the roots last night. The breathing has changed. Whirr. Cog can tell you how."
        dr_s5: { choices: [
            { text: 'runenpfad.npc.automaton.dr_s5.c1', if: SLAIN, to: 'dr_slain' },   // "How has it changed?"
            { text: 'runenpfad.npc.automaton.dr_s5.c1', if: SEALED, to: 'dr_sealed' },
            FAREWELL] },
        // "Click. The breathing has stopped. Whirr. Cog counted for a long time to be sure. Cog thinks something old is missing from the count. Cog is sorry, though Cog is not sure whom to tell."
        dr_slain: { choices: [FAREWELL] },
        // "Click. The breathing is slow now, one breath for every toll of the bell. Whirr. It sounds like counting. Cog likes it. It is the first counting Cog has liked since the core."
        dr_sealed: { mood: 'happy', choices: [FAREWELL] }
    });

    // ---- the two recruits of the epilogue (their ordinary meetings are in journal-story.js) ----
    ['recruit_f', 'recruit_m'].forEach(function (id) {
        insert(id + '_home', 'hello', [{ if: SLAIN, to: 'dr_slain' }, { if: SEALED, to: 'dr_sealed' }]);
    });
    add('recruit_f', {
        // "They say you ended the ember dragon. Edda says it is rude to ask how that felt. I asked anyway. Skadi was on the roof when I did, and she looked at me, and I decided to count waystones instead."
        dr_slain: { choices: [FAREWELL] },
        // "I held the end of the bell rope for the last toll! Only the end. Hakon held the rest. It rang so deep I could taste it. Then Skadi hummed along on the roof, and Edda cried and said it was the dust."
        dr_sealed: { mood: 'happy', choices: [FAREWELL] }
    });
    add('recruit_m', {
        // "I went down the well steps once, afterwards. It is only a cold hole now. I did not like how I felt. I thought it would feel like winning. Edda says that is normal, and that I should not tell Asta."
        dr_slain: { choices: [FAREWELL] },
        // "Nobody told me a bell could be gentle. It rang, and the whole hall sat down inside itself. Then something breathed out, long, on the roof, and I think it was Skadi. Dragons can sigh with relief. That is a thing I know now."
        dr_sealed: { mood: 'happy', choices: [FAREWELL] }
    });
})(typeof window !== 'undefined' ? window : globalThis);

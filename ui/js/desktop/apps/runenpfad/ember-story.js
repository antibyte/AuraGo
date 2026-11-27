/* Runenpfad ember story: the chapter "The Ember Wyrm under Pathrest" (round 11). DOM-free data only.
 * Brandur's choice dialogue (virtual speaker, opened by the lair, see brandur.js), the new residents Mira, Nokk, Pip and Bram
 * (their NPC definitions live in folk.js), and the readable inscriptions of the cave, the archive and Edda's cottage.
 * Appends to RunenpfadNpcStory (after journal-story.js, because it adds to GRAPHS.lore) and RunenpfadNpcs.NPCS.
 * Comments hold the English source text for the i18n files; the villagers' step and fate lines live in ember-story-folk.js.
 * Steps and fates are read through the conditions `step`, `fate`, `tablets` and `seen` (dialogue.js, ember.js). */
(function (root) {
    'use strict';

    const S = root.RunenpfadNpcStory || (root.RunenpfadNpcStory = { MEETINGS: [], GRAPHS: {} });
    const MEETINGS = S.MEETINGS;
    const GRAPHS = S.GRAPHS;
    const FAREWELL = 'runenpfad.npc.c.farewell';
    const STEP_BACK = 'runenpfad.npc.c.step_back';
    const SLAIN = { fate: 'slain' };
    const SEALED = { fate: 'sealed' };

    // ---- Brandur: the speaker without a body (portrait-brandur) and the choice after the fight ----
    const npcs = root.RunenpfadNpcs;
    if (npcs && npcs.NPCS) {
        npcs.NPCS.brandur = {
            faction: null, nameKey: 'runenpfad.npc.brandur.name', titleKey: 'runenpfad.npc.brandur.title', color: '#ff8a3d', height: 74, portrait: 'brandur',
            fallback: { pack: 'monsters-animals-animated', anim: 'goblin_idle', tint: '#ff8a3d' }, offers: [], duelCoins: 0
        };
    }

    // The three ways out of the first two nodes: end it, sing him to sleep (needs the mended bell), or step away.
    const decide = function () {
        return [
            { text: 'runenpfad.npc.brandur.beaten.c1', to: 'slay' },   // "End it."
            { text: 'runenpfad.npc.brandur.beaten.c2', if: { flag: 'dr.bell' }, to: 'seal' },   // "Sing you back to sleep."
            { text: 'runenpfad.npc.brandur.beaten.c2', if: { notFlag: 'dr.bell' }, to: 'seal_no' },
            { text: 'runenpfad.npc.brandur.beaten.c4', to: null }];   // "Not yet."
    };
    GRAPHS.brandur = {
        virtual: true,
        nodes: {
            // "Warden. I know that stance. The first ones stood like that, small and stubborn, and sang me to sleep. Now nobody sings, and I am so tired. Choose, then. I will not beg."
            beaten: { choices: decide() },
            // "You return. I have not moved. Where would I go? The fire in me is small now. Choose, warden, or leave me with my thoughts. Both are kind."
            again: { choices: decide() },
            // "Then it is the fire. Fair. I drank the road's light and wanted more, and I will not dress that up. Only remember my sister in the ice. She will feel the cold when I go out. Strike true."
            slay: { choices: [
                { effects: [{ ember: 'slay' }, { ui: 'scene:dr_slain' }] },   // "Strike."
                { to: 'again' }] },   // "Wait."
            // "Sleep. Yes. I remember the song: four voices and a bell, and a warm place under the roots. Sing it true, warden, and I will not wake hungry. Tell my sister the hearth is warm. Tell her I am sorry about the ice."
            seal: { mood: 'happy', choices: [
                { effects: [{ ember: 'seal' }, { ui: 'scene:dr_sealed' }] },   // "Sing."
                { text: 'runenpfad.npc.brandur.slay.c2', to: 'again' }] },
            // "The bell. I hear it even from here: bronze with a crack in its throat. It cannot carry the song. Mend it, warden, and come back. I will lie here, I have not the strength to rise. But do not be long. Hunger is patient too."
            seal_no: { choices: [
                { to: null }] },   // "I will mend the bell."
            // "(Brandur sleeps among the four pillars. The roots above him glow gently with every slow breath. Once, in his dream, he murmurs a name that sounds like Skadi.)"
            sleeps: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] },
            // "(The embers have gone out. Only the great shape remains, and the hoard that will warm nobody now. Across the stone a thin line of frost has drawn itself, as if someone had wept here.)"
            bones: { choices: [{ text: FAREWELL, to: null }] }
        }
    };

    // ---- Mira, Tilda's daughter: found in the crystal grotto (mira_deeps), then at home in the shop (mira_home) ----
    MEETINGS.push(
        { id: 'mira_deeps', npcId: 'mira', levels: [], starts: [{ to: 'hello' }] },
        { id: 'mira_home', npcId: 'mira', levels: [], starts: [
            { if: SLAIN, to: 'slain' },
            { if: SEALED, to: 'sealed' },
            { to: 'home' }] });
    GRAPHS.mira = {
        nodes: {
            // "You are not Mama. Did Mama send you? I am fine. I only came for the warm light. It is under the roots, you know, and it sings a bit. I left my lantern on the rim on purpose, so she would know where to look. That was clever. Was that clever?"
            hello: { mood: 'happy', choices: [
                { to: 'found' },   // "Very clever. Come on, your mother is sick with worry."
                { to: 'light' }] },   // "What is the warm light?"
            // "It breathes. In and out, slow, like the cat when she sleeps on my feet. I wanted to see what was breathing. I did not want to wake it. Much."
            light: { mood: 'happy', choices: [
                { to: 'found' }] },   // "Let us go home."
            // "Fine, fine. But I am telling Mama that I led you here. And I am coming back when I am bigger. Tell Mama there are bats. No. Do not tell Mama about the bats."
            found: { mood: 'happy', enter: [{ flag: 'mira.found' }], choices: [
                { effects: [{ leave: true }], to: null }] },   // "Run home, I will catch up."
            // "Mama says I am grounded until I am twelve. I say a warden fetched me, so it is practically an adventure licence. Do you think the dragon likes children? I only wanted to see the warm light. It was so cosy."
            home: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] },
            // "Everybody cheered, but I heard him before you came. Down there. He was singing, not nicely, a sad kind of singing, like our cat when she is locked out. Is it really quiet down there now? I liked the warm."
            slain: { choices: [{ text: FAREWELL, to: null }] },
            // "The bell! It rang so deep my teeth hummed. He is asleep now, isn't he? I told Mama the warm light was nice. It was only lonely. Mama says dragons do not get lonely. Mama has never been that far down."
            sealed: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] }
        }
    };

    // ---- Nokk and Pip, the kobold refugees of Krax's clan (cellar at step 2, tavern at step 5) ----
    MEETINGS.push(
        { id: 'nokk_home', npcId: 'nokk', levels: [], starts: [
            { if: SLAIN, to: 'slain' },
            { if: SEALED, to: 'sealed' },
            { to: 'hello' }] },
        { id: 'pip_home', npcId: 'pip', levels: [], starts: [
            { if: { item: { clan_charm: 1 } }, to: 'charm' },
            { if: SLAIN, to: 'slain' },
            { if: SEALED, to: 'sealed' },
            { if: { flag: 'pip.charm' }, to: 'happy' },
            { to: 'hello' }] });
    GRAPHS.nokk = {
        nodes: {
            // "Nokk is Nokk, old, of Krax's clan. Our tunnels lie below the old galleries. When the big fire woke, we ran. Krax holds the gate. Nokk holds the little ones. Thank you for the cellar, Berta's cellar. Warm. Only smells a bit of rats."
            hello: { choices: [
                { to: 'woke' },   // "What woke the fire?"
                { to: 'warm' },   // "Who is the Warm One?"
                { text: FAREWELL, to: null }] },
            // "The roots stopped singing. Our tunnels used to hum, a long low song, the clan's old lullaby too. Then the silence came. The little ones feared the silence. Nokk feared what came after it: breathing, deep down, big and warm."
            woke: { choices: [
                { text: 'runenpfad.npc.nokk.hello.c2', to: 'warm' },
                { text: FAREWELL, to: null }] },
            // "The Warm One. Kobold songs remember him from before the wardens came: he kept the deep hearths for us, and he never once stepped on a kobold. Nokk does not say he is kind. Nokk says he is old, and hungry, and proud. All three can be true."
            warm: { choices: [{ text: FAREWELL, to: null }] },
            // "Nokk is glad. Nokk is not glad. The Warm One kept our hearths once, long ago. Nokk will tell the little ones he was good, once, so the story is not only the end. Krax will say he would have done it faster. Krax is wrong."
            slain: { choices: [{ text: FAREWELL, to: null }] },
            // "The bell is a good sound. The tunnels hum again, Nokk heard it through the cellar floor. We go home by spring, if Berta lets us. She will not. Nokk will have to be firm. Nokk is bad at firm."
            sealed: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] }
        }
    };
    GRAPHS.pip = {
        nodes: {
            // "Pip lost the clan charm! In the old galleries, when the big noise came. Pip ran. Pip is not proud of running. Grandfather Nokk says nothing, which is worse than shouting."
            hello: { marker: 'quest', choices: [
                { to: 'where' },   // "Where did you lose it?"
                { text: FAREWELL, to: null }] },
            // "In the miners' camp west of the shaft, where the old bedrolls are. Pip put it down to tie his boot. Then the roof sang, and Pip forgot everything. It is a little thing on a string. It glows when a kobold sings near it."
            where: { choices: [{ text: FAREWELL, to: null }] },
            // "That is it! Pip smells it from here! The clan charm! Give, give, please give! Pip will be good for a hundred years."
            charm: { mood: 'happy', marker: 'heart', choices: [
                { if: { item: { clan_charm: 1 } }, effects: [{ item: { clan_charm: -1 } }, { rep: { kobolds: 1 } }, { flag: 'pip.charm' }], to: 'charm_ok' },   // "Here, it is yours."
                { text: FAREWELL, to: null }] },
            // "Pip is not crying. Pip has something in his eye. Thank you! Pip will tell Grandfather Nokk, and Nokk will say nothing, but he will say it very warmly."
            charm_ok: { mood: 'happy', choices: [{ text: 'runenpfad.npc.c.thanks', to: null }] },
            // "The charm glows when Grandfather sings! Pip sleeps with it. Do not tell the other kobolds. They will want to hold it."
            happy: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] },
            // "Pip saw the big one once, from far. Big. Pip is not scared of bones. Pip is only small. Grandfather says the Warm One sang for the clan once. Pip did not know dragons could. Pip wishes he had heard."
            slain: { choices: [{ text: FAREWELL, to: null }] },
            // "The tunnels hum! Pip heard it with his feet. Pip will teach all the little ones the hum, and when we go home, we will hum back. Maybe the Warm One will like it. Pip thinks he will."
            sealed: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] }
        }
    };

    // ---- Bram, the old miner at the corner table: the vein order and the broth ----
    MEETINGS.push({ id: 'bram_home', npcId: 'bram', levels: [], starts: [
        { if: SLAIN, to: 'slain' },
        { if: SEALED, to: 'sealed' },
        { to: 'hello' }] });
    GRAPHS.bram = {
        nodes: {
            // "Bram. I worked the galleries thirty years, until they were sealed. The chalk marks down at the shaft are mine. Do not go down without a lantern. Do not go down at all, if you have sense. You have not. Right."
            hello: { choices: [
                { to: 'rise' },   // "What do your chalk marks say?"
                { to: 'broth' },   // "What do miners eat down there?"
                { text: FAREWELL, to: null }] },
            // "Warmth rises. Every old miner knows it. Heat climbs, so you light the low lamp first, then the middle, then the top, and the draught carries it up. The old root lights down there work the same way. Light the top first and they sulk."
            rise: { choices: [
                { text: 'runenpfad.npc.bram.hello.c2', to: 'broth' },
                { text: FAREWELL, to: null }] },
            // "Miner's Broth. Turnip and honey in one pot, a day's work in one bowl, and your legs come back to you quicker. Berta has the kitchen. Tilda sells the turnips and the honey. Tell Berta Bram sent you and she will pretend she does not know me."
            broth: { choices: [
                { text: 'runenpfad.npc.bram.hello.c1', to: 'rise' },
                { text: FAREWELL, to: null }] },
            // "So the old fire is out. I worked above him thirty years and felt the floor warm on cold mornings and thought it was my knees. I would have liked to hear him once. I am glad nobody holds their breath at night now. Both. Fine. Both."
            slain: { choices: [{ text: FAREWELL, to: null }] },
            // "I heard the bell through the stone. Put a hand on a wall in the old galleries and you can feel the whole hill purr. I will go down tomorrow with my chalk. No lair to fear now. Maybe I will just sit there a while."
            sealed: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] }
        }
    };

    // ---- lore: the six wardens' tablets (in order), the cave signs and the notes of Edda's cottage ----
    const lore = (GRAPHS.lore || (GRAPHS.lore = { virtual: true, nodes: {} })).nodes;
    [
        'tab_1',   // "On this first stone the road begins. Beneath it run the roots of the Primal Rune, and they carry up the warmth of the deep. We raised a hall here and called the place Pathrest, for here the path rests, and here we keep the watch."
        'tab_2',   // "Two were born of the first fire, and they were not of one mind: Skadi, who loves the high cold, and Brandur, who loves the deep warm. She keeps the passes, he keeps the hearths. The road stands between them like a bridge between two shores. When one of them suffers, the other grieves, and the bridge shakes."
        'tab_3',   // "Brandur drank the light of the roots and found it sweet. He wanted more than his hearth: the Primal Rune itself, the road, and every warm thing upon it. Skadi sent word down from the snow and was not answered. Then the first village burned."
        'tab_4',   // "We raised four pillars of sleep runes around his hoard. Four wardens lit them together and sang: 'Four lights to keep you, one bell to call; sleep where the roots are warm, and sleep through all.' He slept. The pillars hold him only while all four burn."
        'tab_5',   // "We cast the bell from the deep bronze, so that its voice would reach him where ours cannot. Ring it, and the song rings. Let it fall silent, and the sleeper stirs. A cracked bell sings flat, and a flat song is no song. Mend the bell before you need it."
        'tab_6',   // "We who sang him down hold no hatred for him. He was our neighbour long before he was our danger. If the song fails, the keeper of the road may end the fire or sing it down again. Choose knowing his name, and do not choose alone. Walk it together, or not at all."
        'bram_chalk',   // "Chalk on the timber, in a miner's blocky hand: WARMTH RISES. Below it three arrows climbing: low, middle, top. And underneath, underlined twice: DO NOT LIGHT THE TOP ONE FIRST."
        'kobold_scratch',   // "A kobold tally on the wall: forty-three strokes for forty-three quiet nights. Then, scratched deeper, the words THE WARM ONE WAKES. Beside it a small drawing of a charm on a string, with a worried face."
        'vein_hint',   // "Three dead veins of rune light climb the wall: a low one, a middle one, a high one. Old wardens' scratching: 'Warmth rises, and so does the light. Wake it from the bottom up.'"
        'finn_song',   // "A page of verses, half crossed out. One survives: 'Four lights to keep you, one bell to call; sleep where the roots are warm, and sleep through all.' In the margin: 'My grandmother hummed this. Where did she learn it?'"
        'gundar_note',   // "A crumpled tally of debts: Finn 4, Berta 3, Tilda 0 (she keeps better score than I do). At the bottom: 'The room at the end of the hall is Edda's. Locked twenty years. She said the key lives where she keeps what she cannot throw away.'"
        'archive_hint',   // "Four faded panels on the archive wall: dawn in the east, noon in the south, dusk in the west, and in the north a single star that never sets. Beneath them: 'Pull the books the way the sun walks.'"
        'tilda_gear',   // "A dented helmet, a crossbow with one bolt, and a note in Tilda's hand: 'Retired. Reason: Mira. Do not tell her I kept the helmet.' The helmet has teeth marks. Small ones."
        'edda_letter_1',   // "'Little one: the hall key is under the third planter. The cat is not mine, whatever she says. The well was warm when I swept the yard this morning. It was always a little warm. Today it was more than a little. Back for supper. If not, there is bread in the pantry, and the cat will tell you she has not been fed.'"
        'edda_letter_2',   // "'Inside the hall bell the words are cast: SLEEP BELL. Runa laughs: who needs a bell to sleep? But it was cast in a hurry, long ago, by people who were afraid of something. If I do not come back, little one, ring it when the road goes quiet. I do not know what for. I only know they wanted it rung.'"
        'edda_notes',   // "Edda's notes, cramped and underlined. 'Hall bell: bronze of the deep galleries, cast by the first wardens. Hakon says he could pour a new one if he had the same bronze. Try the collapsed gallery behind the old rails.' Later, in different ink: 'The warmth is rising. I was right to be afraid, and wrong about what of.'"
        'edda_map'   // "Edda's map of the galleries under Pathrest, every room inked in her small hand, with arrows climbing along the roots. In the margin: 'Warmth goes up, and so, I think, does the road. Whatever lives at the bottom is warm and very old. Be polite.'"
    ].forEach(function (id) {
        lore[id] = { choices: [{ text: STEP_BACK, to: null }] };
    });
})(typeof window !== 'undefined' ? window : globalThis);

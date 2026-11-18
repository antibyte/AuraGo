/* Runenpfad folk story data: meetings and dialogue graphs of the Pathrest villagers (elder, innkeeper, smith, shopkeeper, bard), the
 * hermit, kobold barkeep, desert scribe, mountaineer and brass helper, plus one tavern node for each story NPC. DOM-free data only.
 * Comments hold the English source text for the i18n files. Appends to RunenpfadNpcStory (npc-story.js, npc-story-late.js);
 * interpreted by dialogue.js. Meetings have levels: [] and are picked through the meeting id the rooms hand to the NPC. */
(function (root) {
    'use strict';

    const S = root.RunenpfadNpcStory || (root.RunenpfadNpcStory = { MEETINGS: [], GRAPHS: {} });
    const MEETINGS = S.MEETINGS;
    const GRAPHS = S.GRAPHS;
    const FAREWELL = 'runenpfad.npc.c.farewell';
    const THANKS = 'runenpfad.npc.c.thanks';
    const TRADE = 'runenpfad.npc.c.trade';

    // ---- elder ----
    MEETINGS.push({ id: 'elder_home', npcId: 'elder', levels: [], starts: [
        { if: { flag: 'epilogue.paid' }, to: 'epilogue' },
        { if: { notFlag: 'elder.met' }, to: 'hello' },
        { if: { progress: 21 }, to: 'p21' },
        { if: { progress: 16 }, to: 'p16' },
        { if: { progress: 11 }, to: 'p11' },
        { if: { progress: 6 }, to: 'p6' },
        { to: 'again' }] });
    GRAPHS.elder = {
        nodes: {
            // "Ah, there you are. I am Runa, elder of Pathrest. The Rune Path that joined our five realms has broken, and of all its wardens only you are left. Will you walk it for us?"
            hello: { marker: 'quest', enter: [{ flag: 'elder.met' }], choices: [
                { to: 'path' },   // "What broke the path?"
                { to: 'start' },   // "Where do I begin?"
                { text: FAREWELL, to: null }] },
            // "Five waystones held it together, one in every realm. They shattered, and a keeper took up each shard. Strong, stubborn and not quite themselves anymore."
            path: { choices: [
                { to: 'keepers' },   // "Who are the keepers?"
                { to: null }] },   // "I understand."
            // "A boar, a goblin warlord, a watching orb, a frozen beast and something made of brass. Beat each one and its shard is yours. Do not take them lightly."
            keepers: { choices: [
                { to: null },   // "I will win them back."
                { text: 'runenpfad.npc.elder.hello.c2', to: 'start' }] },   // "Where do I begin?"
            // "Take the rune arch at the far end of the street and pick your road. Hakon's workshop sharpens what you carry, Berta feeds the weary, and Tilda sells what the road will not give you."
            start: { choices: [
                { text: THANKS, to: null }] },
            // "Pathrest is quiet while you are out on the path. Is there something you need, or something you wish to remember?"
            again: { choices: [
                { effects: [{ ui: 'chronicle' }] },   // "Open the chronicle."
                { to: 'keepers' },   // "Remind me of the keepers."
                { text: FAREWELL, to: null }] },
            // "The boar is down, and the first shard hums again! I can feel the path breathing. The next one is held by the goblin warlord of the Fungal Grotto. Mind his crown. It ticks."
            p6: { mood: 'happy', choices: [
                { text: 'runenpfad.npc.elder.again.c1', effects: [{ ui: 'chronicle' }] },   // "Open the chronicle."
                { text: FAREWELL, to: null }] },
            // "Two shards! The caves are quiet and the stones sing again. Now the desert, and a watching orb that sees everything. I suspect it has seen you coming."
            p11: { mood: 'happy', choices: [
                { text: 'runenpfad.npc.elder.again.c1', effects: [{ ui: 'chronicle' }] },   // "Open the chronicle."
                { text: FAREWELL, to: null }] },
            // "Three! The dunes are behind you. Ahead lies the Frozen Pass, where time itself stands still and something old sleeps in the peaks. Dress warmly."
            p16: { mood: 'happy', choices: [
                { text: 'runenpfad.npc.elder.again.c1', effects: [{ ui: 'chronicle' }] },   // "Open the chronicle."
                { text: FAREWELL, to: null }] },
            // "Four shards. Only the foundry is left, and whatever hums inside it. I know that hum from the old stories: the Chronos core. I had hoped never to say that name again."
            p21: { choices: [
                { text: 'runenpfad.npc.elder.again.c1', effects: [{ ui: 'chronicle' }] },   // "Open the chronicle."
                { text: FAREWELL, to: null }] },
            // "The path is open again. Travellers cross it, the waystones sing, and Pathrest has not been this loud in a hundred years. Thank you, from all of us."
            epilogue: { mood: 'happy', marker: 'heart', choices: [
                { text: 'runenpfad.npc.elder.again.c1', effects: [{ ui: 'chronicle' }] },   // "Open the chronicle."
                { text: FAREWELL, to: null }] }
        }
    };

    // ---- innkeeper ----
    MEETINGS.push({ id: 'innkeeper_home', npcId: 'innkeeper', levels: [], starts: [
        { if: { loot: 'tavern_cellar' }, to: 'cellar_done' },
        { if: { notFlag: 'innkeeper.met' }, to: 'hello' },
        { to: 'again' }] });
    GRAPHS.innkeeper = {
        nodes: {
            // "Welcome to the Rusty Rune! I am Berta. Hot stew, cold ale and not much else, but I make both with love."
            hello: { mood: 'happy', marker: 'trade', enter: [{ flag: 'innkeeper.met' }], choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { to: 'rats' },   // "Can I help with anything?"
                { text: FAREWELL, to: null }] },
            // "Since the path broke, rats have taken the cellar. Five at least, and they bit my last barrel boy. Clear them out and whatever you find down there is yours. The trapdoor is in the corner."
            rats: { choices: [
                { to: 'rats_ok' },   // "Leave it to me."
                { to: null }] },   // "Maybe later."
            // "You are a treasure! Mind your step, they are quick and the cellar is dark. I will keep the stew warm."
            rats_ok: { mood: 'happy', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] },
            // "Back already? Sit, eat. A full belly is the best armour I know."
            again: { marker: 'trade', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { to: 'rats' },   // "What about the cellar?"
                { text: FAREWELL, to: null }] },
            // "Not a squeak from the cellar since you went down! I could kiss you. I will not. What can I get you?"
            cellar_done: { mood: 'happy', marker: 'heart', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] }
        }
    };

    // ---- smith ----
    MEETINGS.push({ id: 'smith_home', npcId: 'smith', levels: [], starts: [
        { if: { notFlag: 'smith.met' }, to: 'hello' },
        { to: 'again' }] });
    GRAPHS.smith = {
        nodes: {
            // "Mind the sparks. Hakon, smith of Pathrest. I cannot mend a broken path, but I can make a warden's tools hit harder and last longer. Rune work, mostly."
            hello: { marker: 'talk', enter: [{ flag: 'smith.met' }], choices: [
                { effects: [{ ui: 'workshop' }] },   // "Show me the workshop."
                { to: 'runes' },   // "What kind of rune work?"
                { text: FAREWELL, to: null }] },
            // "Runes are promises hammered into metal. More hearts, a sharper edge, faster feet. You pay in coins, never in blood. I have never understood the people who pay in blood."
            runes: { choices: [
                { text: 'runenpfad.npc.smith.hello.c1', effects: [{ ui: 'workshop' }] },   // "Show me the workshop."
                { text: FAREWELL, to: null }] },
            // "Back again? The forge is hot and the anvil is hungry."
            again: { marker: 'talk', choices: [
                { text: 'runenpfad.npc.smith.hello.c1', effects: [{ ui: 'workshop' }] },   // "Show me the workshop."
                { text: FAREWELL, to: null }] }
        }
    };

    // ---- shopkeeper ----
    MEETINGS.push({ id: 'shopkeeper_home', npcId: 'shopkeeper', levels: [], starts: [
        { if: { notFlag: 'shopkeeper.met' }, to: 'hello' },
        { to: 'again' }] });
    GRAPHS.shopkeeper = {
        nodes: {
            // "Come in, come in! Tilda's has potions for the cautious and charms for the curious. Prices are fair, and I only haggle with people who can count."
            hello: { mood: 'happy', marker: 'trade', enter: [{ flag: 'shopkeeper.met' }], choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { to: 'tips' },   // "Any bargains?"
                { text: FAREWELL, to: null }] },
            // "A map charm shows you the gems. A lucky amulet makes your coins count double on the next stage. An attuning stone stretches your next rune. Everything you need to look clever."
            tips: { choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { text: THANKS, to: null }] },
            // "Back so soon? Coins talk, you know. Mine listen."
            again: { marker: 'trade', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] }
        }
    };

    // ---- bard ----
    MEETINGS.push({ id: 'bard_home', npcId: 'bard', levels: [], starts: [
        { if: { flag: 'epilogue.paid' }, to: 'song' },
        { if: { progress: 21 }, to: 'r5' },
        { if: { progress: 16 }, to: 'r4' },
        { if: { progress: 11 }, to: 'r3' },
        { if: { progress: 6 }, to: 'r2' },
        { to: 'r1' }] });
    GRAPHS.bard = {
        nodes: {
            // "Finn, bard of the Rusty Rune. Gather round for a rumour: a boar roams Whisperwood with a brass gear wedged in its side, ticking like a clock. Not natural, that."
            r1: { mood: 'happy', choices: [
                { text: THANKS, to: null }] },
            // "The goblin warlord of the Fungal Grotto wears a crown that ticks. Ticks! They say he has not slept since the path broke. Nobody dares to ask whether the crown does the thinking."
            r2: { mood: 'happy', choices: [
                { to: 'r1' },   // "And the older rumours?"
                { text: THANKS, to: null }] },
            // "Out in the dunes an orb hangs over the old observatory and watches everything that moves. Merchants swear it blinked at them. I have decided not to go there, on principle."
            r3: { mood: 'happy', choices: [
                { text: 'runenpfad.npc.bard.r2.c1', to: 'r2' },   // "And the older rumours?"
                { text: THANKS, to: null }] },
            // "In the Frozen Pass time has frozen: a waterfall hangs mid-fall, and a dragon sleeps in the peaks with its breath held. The icicles drip once an hour. I counted."
            r4: { mood: 'happy', choices: [
                { text: 'runenpfad.npc.bard.r2.c1', to: 'r3' },   // "And the older rumours?"
                { text: THANKS, to: null }] },
            // "Brass is working late in the foundry. They say he built the Chronos core to keep the path from ever running down, and now it will not stop counting. I am saving that verse for your return."
            r5: { choices: [
                { text: 'runenpfad.npc.bard.r2.c1', to: 'r4' },   // "And the older rumours?"
                { text: THANKS, to: null }] },
            // "I wrote a ballad about you. Fourteen verses, and nobody has asked me to stop yet. Sit down, this will take a while. Berta, more ale!"
            song: { mood: 'happy', marker: 'heart', choices: [
                { to: null },   // "Maybe a shorter version?"
                { text: FAREWELL, to: null }] }
        }
    };

    // ---- hermit ----
    MEETINGS.push({ id: 'hermit_home', npcId: 'hermit', levels: [], starts: [
        { if: { notFlag: 'hermit.met' }, to: 'hello' },
        { to: 'again' }] });
    GRAPHS.hermit = {
        nodes: {
            // "Hm? A visitor. Oswin. I kept the wardens' lamps for forty years, until the night the path broke. Nobody has come up here since."
            hello: { enter: [{ flag: 'hermit.met' }], choices: [
                { to: 'night' },   // "What happened that night?"
                { text: FAREWELL, to: null }] },
            // "A hum, then a crack, like a giant breaking a bone. The waystones went dark one by one. I stayed. Somebody has to trim the lamps."
            night: { choices: [
                { to: 'why' },   // "But why did it break?"
                { text: FAREWELL, to: null }] },
            // "The old wardens said the path would never wear out as long as the Primal Rune fed it. Then someone made very sure of it, and made it worse. That is all I know."
            why: { choices: [
                { if: { notFlag: 'hermit.gift' }, effects: [{ flag: 'hermit.gift' }, { give: 'potion' }], to: 'gift' },   // "(Oswin presses a flask into your hand.)"
                { text: FAREWELL, to: null }] },
            // "A potion from the days when wardens still passed by. It has not gone off. Much."
            gift: { mood: 'happy', choices: [
                { text: THANKS, to: null }] },
            // "Still on your feet? Good. The lamps need keepers, and so do the roads."
            again: { choices: [
                { to: 'night' },   // "Tell me about that night again."
                { text: FAREWELL, to: null }] }
        }
    };

    // ---- barkeep ----
    MEETINGS.push({ id: 'barkeep_home', npcId: 'barkeep', levels: [], starts: [
        { if: { notFlag: 'barkeep.met' }, to: 'hello' },
        { to: 'again' }] });
    GRAPHS.barkeep = {
        nodes: {
            // "Snik pours, Snik listens. Best mushroom ale under the mountain! Krax drinks here on quiet nights and sings, which is why the quiet nights are not so quiet."
            hello: { mood: 'happy', marker: 'trade', enter: [{ flag: 'barkeep.met' }], choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { to: 'gossip' },   // "Heard any gossip?"
                { text: FAREWELL, to: null }] },
            // "Krax guards the gate and counts his coins every night. The warlord down the tunnel, though... his crown ticks. Snik hears it through the walls. Tick, tick, tick. Bad for business."
            gossip: { choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { text: THANKS, to: null }] },
            // "Snik remembers you! Pay, drink, go. In that order."
            again: { marker: 'trade', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] }
        }
    };

    // ---- scribe ----
    MEETINGS.push({ id: 'scribe_home', npcId: 'scribe', levels: [], starts: [
        { if: { notFlag: 'scribe.met' }, to: 'hello' },
        { to: 'again' }] });
    GRAPHS.scribe = {
        nodes: {
            // "Peace, traveller. I am Nahim, scribe of the circle, keeper of the observatory's records. The great orb above us was a lens once, an eye to read the stars. Now it reads us."
            hello: { marker: 'trade', enter: [{ flag: 'scribe.met' }], choices: [
                { to: 'orb' },   // "The orb is watching us?"
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] },
            // "It turns to whatever moves. I have written eleven scrolls on how not to be seen, and I am fairly sure the orb has read them all. Shatter its eye, and the dunes sleep again."
            orb: { choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { text: THANKS, to: null }] },
            // "Back from the dunes? Sit. The ink is dry, but the tea is warm."
            again: { marker: 'trade', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] }
        }
    };

    // ---- climber ----
    MEETINGS.push({ id: 'climber_home', npcId: 'climber', levels: [], starts: [
        { if: { notFlag: 'climber.met' }, to: 'hello' },
        { to: 'again' }] });
    GRAPHS.climber = {
        nodes: {
            // "Mind the door, it sticks in the cold. I am Yrsa. I climb these peaks, or I did, until time froze up there. A dragon sleeps in the ice. There is a fire here. Rest if you must."
            hello: { enter: [{ flag: 'climber.met' }], choices: [
                { to: 'frozen' },   // "What do you mean, frozen?"
                { text: FAREWELL, to: null }] },
            // "The wind stops mid-gust. Icicles hang and never melt. The dragon over the pass does not breathe, or once a night, no more. I do not trust anything that quiet."
            frozen: { choices: [
                { text: THANKS, to: null }] },
            // "The fire is warm, the soup is hot and the dragon is still up there. Rest while you can."
            again: { choices: [
                { to: 'frozen' },   // "Tell me about the frozen peaks."
                { text: FAREWELL, to: null }] }
        }
    };

    // ---- automaton ----
    MEETINGS.push({ id: 'automaton_home', npcId: 'automaton', levels: [], starts: [
        { if: { notFlag: 'automaton.met' }, to: 'hello' },
        { to: 'again' }] });
    GRAPHS.automaton = {
        nodes: {
            // "Click. Hello. Cog is helper. Cog is brass. Whirr. Master Brass writes notes about the core. Cog keeps the notes. Click."
            hello: { enter: [{ flag: 'automaton.met' }], choices: [
                { to: 'notes' },   // "What do the notes say?"
                { text: FAREWELL, to: null }] },
            // "Click. The Chronos core never stops counting. Brass built it to keep the path young. Whirr. Now the path does not move. Cog does not understand. Cog is worried. Click."
            notes: { choices: [
                { to: 'core' },   // "And what does the core think?"
                { if: { notFlag: 'automaton.gift' }, to: 'gift' },   // "Can I help Cog?"
                { text: THANKS, to: null }] },
            // "Click. The core did a sum. A path nobody walks never wears out. So the core broke the path and gave the shards away. Whirr. Cog thinks the sum is wrong. Click."
            core: { choices: [
                { text: 'runenpfad.npc.automaton.notes.c2', if: { notFlag: 'automaton.gift' }, to: 'gift' },   // "Can I help Cog?"
                { text: THANKS, to: null }] },
            // "Click! Brave one. Cog has one gift. A token for the workshop. Fewer coins, more power. Whirr. Take it."
            gift: { mood: 'happy', choices: [
                { if: { notFlag: 'automaton.gift' }, effects: [{ item: { discount_token: 1 } }, { flag: 'automaton.gift' }], to: 'gift_ok' },   // "Take the token."
                { text: FAREWELL, to: null }] },
            // "Click. Good luck on the path. Cog will count the days. Whirr."
            gift_ok: { mood: 'happy', choices: [
                { text: THANKS, to: null }] },
            // "Click. Welcome back. Cog is polishing. Whirr. The core still counts. Brass still worries. Click."
            again: { choices: [
                { text: 'runenpfad.npc.automaton.hello.c1', to: 'notes' },   // "What do the notes say?"
                { text: FAREWELL, to: null }] }
        }
    };

    // ---- tavern guests: one node each in the existing graph of the story NPC, picked through meeting <id>_tavern ----
    [
        // "A drink at last, with nothing in it I have to stir. Thank you again for the mushrooms, wanderer. The coven pays its debts."
        'witch',
        // "Krax drinks! Krax does not remember if you paid or bonked him, but Krax remembers you. Good ale, this. Weak, but good."
        'chief',
        // "Fitz is on his break! Mostly. If you need a potion, Fitz still knows a guy. Do not tell Tilda, she thinks Fitz is competition."
        'merchant',
        // "The dunes are far away, yet I hear them in the foam of the ale. Greetings, friend. The circle sends warm wishes and one very old song."
        'seer',
        // "Aldric, off duty and very much not amused. The tower can wait one evening. The path is mending, and I would like to drink to it before the last shard is found."
        'wizard',
        // "Brass in a tavern, I know. Steam makes a man thirsty. Over the foam I will say it plainly: Brass built the core, and Brass is sorry. Finish what Brass could not."
        'alchemist'
    ].forEach(function (id) {
        if (!GRAPHS[id]) return;
        MEETINGS.push({ id: id + '_tavern', npcId: id, levels: [], starts: [{ to: 'tavern' }] });
        GRAPHS[id].nodes.tavern = { choices: [{ text: FAREWELL, to: null }] };
    });

})(typeof window !== 'undefined' ? window : globalThis);

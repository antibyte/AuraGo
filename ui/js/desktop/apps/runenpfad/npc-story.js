/* Runenpfad NPC story data: meetings and dialogue graphs (witch worlds 1-2, merchant, chief Krax). DOM-free data only.
 * Comments hold the English source text for the i18n files. Interpreted by dialogue.js; npc-story-late.js appends later arcs. */
(function (root) {
    'use strict';

    const MEETINGS = [];
    const GRAPHS = {};
    const FAREWELL = 'runenpfad.npc.c.farewell';
    const THANKS = 'runenpfad.npc.c.thanks';
    const TRADE = 'runenpfad.npc.c.trade';
    const LEAVE = 'runenpfad.npc.c.leave';
    const FIGHT = 'runenpfad.npc.c.fight';
    const BACK = 'runenpfad.npc.c.back';
    // Merchant greeting variants reuse the choice texts of merchant.hello.
    const M_RUMOURS = 'runenpfad.npc.merchant.hello.c2';
    const M_HAGGLE = 'runenpfad.npc.merchant.hello.c3';
    const M_WARES = 'runenpfad.npc.merchant.hello_quest.c2';
    const QUEST_ANY = { any: [{ quest: 'witch_mushrooms' }, { quest: 'seer_shards' }, { quest: 'alchemist_gears' }] };
    const HAGGLE_CHOICE = { text: M_HAGGLE, if: { notFlag: ['merchant.haggle', 'merchant.haggled'] }, roll: { p: 0.5, else: 'haggle_no' }, to: 'haggle_ok' };

    MEETINGS.push(
        { id: 'witch_w1', npcId: 'witch', levels: [2, 4], starts: [
            { if: { quest: 'witch_mushrooms', item: { glow_mushroom: 3 } }, to: 'turnin' },
            { if: { quest: 'witch_mushrooms' }, to: 'waiting' },
            { if: { questDone: 'witch_mushrooms' }, to: 'friend' },
            { if: { flag: 'witch.refused' }, to: 'cold' },
            { to: 'hello' }] },
        // World 2: reacts to world 1 (hostile = refused AND attacked; helped; refused; attacked; stranger).
        { id: 'witch_w2', npcId: 'witch', levels: [7], starts: [
            { if: { flag: ['witch.refused', 'witch.provoked'] }, to: 'ambush' },
            { if: { quest: 'witch_mushrooms', item: { glow_mushroom: 3 } }, to: 'turnin' },
            { if: { quest: 'witch_mushrooms' }, to: 'waiting' },
            { if: { repMin: { coven: 2 }, notFlag: 'witch.charm' }, to: 'gift' },
            { if: { questDone: 'witch_mushrooms' }, to: 'known' },
            { if: { flag: 'witch.provoked' }, to: 'wary' },
            { if: { flag: 'witch.refused' }, to: 'sour' },
            { to: 'stranger' }] },
        { id: 'merchant_campaign', npcId: 'merchant', levels: [3, 9, 12, 18, 23], starts: [
            { if: { flag: 'merchant.fake_sold' }, to: 'confront_fake' },
            { if: { repMax: { kobolds: -2 } }, to: 'wary' },
            { if: { repMin: { kobolds: 1 }, notFlag: 'merchant.krax' }, to: 'kobold_tip' },
            { if: { repMin: { kobolds: 2 } }, to: 'regular' },
            { if: QUEST_ANY, to: 'hello_quest' },
            { if: { any: [{ world: 1 }, { world: 2 }] }, to: 'hello_deep' },
            { if: { any: [{ world: 3 }, { world: 4 }] }, to: 'hello_high' },
            { to: 'hello' }] },
        { id: 'merchant_endless', npcId: 'merchant', levels: 'endless', starts: [
            { if: { flag: 'merchant.fake_sold' }, to: 'confront_fake' },
            { if: { repMax: { kobolds: -2 } }, to: 'wary' },
            { if: { repMin: { kobolds: 2 } }, to: 'regular' },
            { to: 'hello' }] },
        // Krax guards the world 2 gate (the engine's npcGate 'chief'), and waits again in world 3.
        { id: 'chief_w2', npcId: 'chief', levels: [8], starts: [
            { if: { flag: 'chief.respect' }, to: 'respect' },
            { if: { any: [{ flag: 'chief.paid' }, { flag: 'chief.charmed' }, { flag: 'chief.owed' }] }, to: 'passed' },
            { if: { repMin: { kobolds: 3 } }, to: 'friend' },
            { if: { flagMin: { 'chief.duel_lost': 2 } }, to: 'mercy' },
            { to: 'block' }] },
        { id: 'chief_w3', npcId: 'chief', levels: [13], starts: [
            { if: { flag: 'chief.respect' }, to: 'ally' },
            { if: { flag: 'chief.owed' }, to: 'debt' },
            { if: { flag: 'chief.charmed' }, to: 'grudge' },
            { to: 'meet' }] }
    );

    GRAPHS.witch = {
        hooks: {
            onAttack: [{ flag: 'witch.provoked' }],
            onDuelWon: [{ if: { flag: 'witch.refused' }, to: 'won_grudge' }, { to: 'won' }],
            onDuelLost: [{ to: 'lost' }]
        },
        nodes: {
            // "Hm! A tin-plated wanderer in my moor. I am Grimhild. My cauldron is cold for want of glow mushrooms, the blue ones that light the moss at night. Bring me three?"
            hello: { marker: 'quest', enter: [{ flag: 'witch.met' }], choices: [
                { effects: [{ quest: 'witch_mushrooms' }], to: 'accept' },            // "I'll find them."
                { to: 'reward' },                                                      // "What's in it for me?"
                { effects: [{ flag: 'witch.refused' }], to: 'refuse' },                // "Find your own weeds, witch."
                { text: TRADE, effects: [{ trade: true }] }] },
            // "A potion that mends three cuts, and the coven's goodwill. Goodwill opens doors, some in caves."
            reward: { choices: [
                { effects: [{ quest: 'witch_mushrooms' }], to: 'accept' },            // "Deal."
                { effects: [{ flag: 'witch.refused' }], to: 'refuse' }] },            // "Not interested."
            // "Now that you know to look, you will see them glowing. Three, mind you."
            accept: { mood: 'happy', choices: [{ to: 'advice' }, { text: FAREWELL, to: null }] },   // "Any advice for the road?"
            // "The moss whispers to me: {tip}"
            advice: { tip: 'secret_wall', choices: [{ text: THANKS, to: null }] },
            // "Then walk carefully. The moor remembers rudeness."
            refuse: { mood: 'angry', choices: [{ text: FAREWELL, to: null }] },
            // "Still {count} to find. They glow blue near the high ledges."
            waiting: { vars: { count: 'need:glow_mushroom:3' }, choices: [
                { to: null },                                                          // "On my way."
                { text: TRADE, effects: [{ trade: true }] }] },
            // "You again. Here to apologise, or to gawk?"
            cold: { mood: 'angry', choices: [
                { effects: [{ unflag: 'witch.refused' }, { quest: 'witch_mushrooms' }], to: 'accept' },   // "I'm sorry. I'll gather your mushrooms."
                { to: null },                                                          // "Neither."
                { if: { feature: 'duel' }, effects: [{ provoke: true }] }] },          // "Out of my way, hag!"
            // "Three glowing caps! The coven owes you, wanderer."
            turnin: { mood: 'happy', marker: 'heart', choices: [
                { if: { item: { glow_mushroom: 3 } }, effects: [{ item: { glow_mushroom: -3 } }, { questDone: 'witch_mushrooms' }, { rep: { coven: 1 } }, { give: 'potion' }, { flag: 'witch.helped' }], to: 'thanks' },   // "Here you are."
                { to: null }] },                                                       // "Not yet."
            // "Take this potion, and a secret: {tip}"
            thanks: { mood: 'happy', tip: 'gems_left', choices: [{ text: THANKS, to: null }] },
            // "My cauldron bubbles again, thanks to you."
            friend: { mood: 'happy', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { to: 'keytip' },                                                      // "Where is the key?"
                { text: FAREWELL, to: null }] },
            // "{tip}"
            keytip: { tip: 'key_dir', choices: [{ text: THANKS, to: null }] },
            // "Curse your sword arm! The coven will hear of this."
            won_grudge: { mood: 'angry', enter: [{ leave: true }], choices: [{ text: LEAVE, to: null }] },
            // "Ha! You fight like a thorn bush. Take your prize."
            won: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] },
            // "Back to the mud with you. And mind your manners."
            lost: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] },

            // ---- world 2 (cavern) ----
            // "The rude one! The moor remembered, and so do I."
            ambush: { mood: 'angry', marker: 'duel', choices: [
                { if: { feature: 'duel' }, effects: [{ duel: true }] },                // "Then let's settle it."
                { if: { coinsMin: 30 }, effects: [{ coins: -30 }, { unflag: 'witch.refused' }, { rep: { coven: 1 } }], to: 'peace' },   // "Wait! Thirty coins, as an apology."
                { effects: [{ leave: true }], to: null }] },                           // "Back away slowly."
            // "Thirty coins soothe a grudge. Barely."
            peace: { choices: [{ text: FAREWELL, to: null }] },
            // "My favourite mushroom-picker! Krax squats on the next passage. Show him this hex charm."
            gift: { mood: 'happy', marker: 'quest', choices: [
                { effects: [{ flag: 'witch.charm' }, { item: { hex_charm: 1 } }], to: 'charm' }] },   // "Thank you, Grimhild!"
            // "He will let you pass. Probably."
            charm: { choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "These caves crawl with kobolds; they respect coins and clubs. And their master: {tip}"
            known: { tip: 'boss_weakness', choices: [
                { if: { coinsMin: 25, notFlag: 'witch.charm' }, effects: [{ coins: -25 }, { rep: { coven: 1 } }, { flag: 'witch.charm' }, { item: { hex_charm: 1 } }], to: 'charm' },   // "Brew me a hex charm? (25 coins)"
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] },
            // "You struck me once. I don't forget, but I don't hold grudges forever."
            wary: { mood: 'angry', choices: [
                { effects: [{ unflag: 'witch.provoked' }, { rep: { coven: 1 } }], to: 'stranger' },   // "I'm sorry I struck you."
                { text: FAREWELL, to: null }] },
            // "Another stranger in the dark. Buying or leaving?"
            stranger: { choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "You are the one who called my herbs weeds. I have not forgotten."
            sour: { mood: 'angry', choices: [
                { effects: [{ unflag: 'witch.refused' }], to: 'stranger' },           // "I was rude. I'm sorry."
                { text: FAREWELL, to: null }] }
        }
    };

    // The kobold trader: trade, rumours, haggling, quest wares, kobold reputation and the fake-goods confrontation.
    GRAPHS.merchant = {
        hooks: {
            onAttack: [],
            onDuelWon: [{ to: 'won' }],
            onDuelLost: [{ to: 'lost' }]
        },
        nodes: {
            // "Psst, traveller! Fitz has goods, Fitz has prices. Mostly fair ones."
            hello: { marker: 'trade', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { to: 'rumour' },                                                      // "Heard any rumours?"
                { if: { notFlag: ['merchant.haggle', 'merchant.haggled'] }, roll: { p: 0.5, else: 'haggle_no' }, to: 'haggle_ok' },   // "Can we haggle?"
                { text: FAREWELL, to: null }] },
            // "A rumour costs nothing: {tip}"
            rumour: { tip: 'chest_dir', choices: [{ text: THANKS, to: null }] },
            // "Hah! You drive a hard bargain. Fifteen off, this visit only."
            haggle_ok: { mood: 'happy', enter: [{ flag: 'merchant.haggle' }], choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "No haggling. Fitz has a family to feed. Probably."
            haggle_no: { mood: 'angry', enter: [{ flag: 'merchant.haggled' }], choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "Ah. The, er, goods. A small misunderstanding, I am sure."
            confront_fake: { mood: 'angry', marker: 'duel', choices: [
                { effects: [{ confront: true }], to: 'refund' },                       // "That was a fake! Pay me back."
                { effects: [{ unflag: ['merchant.fake_sold', 'merchant.fake_price'] }], to: 'hello' }] },   // "Forget it."
            // "Half back, and not a coin more. Do not tell the other kobolds."
            refund: { choices: [{ text: FAREWELL, to: null }] },

            // "Down here Fitz sells by candlelight! Torches extra. Everything else, too."
            hello_deep: { marker: 'trade', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { text: M_RUMOURS, to: 'rumour_boss' },
                Object.assign({}, HAGGLE_CHOICE),
                { text: FAREWELL, to: null }] },
            // "Even up here, Fitz finds you. Cold air, warm prices. Mostly fair."
            hello_high: { marker: 'trade', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { text: M_RUMOURS, to: 'rumour_boss' },
                Object.assign({}, HAGGLE_CHOICE),
                { text: FAREWELL, to: null }] },
            // "Word from the road: {tip}"
            rumour_boss: { tip: 'boss_weakness', choices: [{ text: THANKS, to: null }] },
            // "Oh. You. Fitz remembers. Watch the scales, and watch your purse."
            wary: { mood: 'angry', marker: 'trade', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { if: { coinsMin: 20 }, effects: [{ coins: -20 }, { rep: { kobolds: 1 } }], to: 'forgiven' },   // "Let me make amends. (20 coins)"
                { text: FAREWELL, to: null }] },
            // "Twenty coins and a smile. Fitz forgives, a little."
            forgiven: { mood: 'happy', choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "Fitz hears Krax likes you. Between friends: Krax loves shiny things, and coins open his gate."
            kobold_tip: { mood: 'happy', marker: 'heart', enter: [{ flag: 'merchant.krax' }], choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { text: THANKS, to: null }] },
            // "My favourite customer! Fitz has goods and, for you, news."
            regular: { mood: 'happy', marker: 'trade', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { roll: { p: 0.5, else: 'nothing' }, to: 'freebie' },                  // "Any news, friend?"
                { if: QUEST_ANY, text: M_WARES, to: 'wares' },
                { text: FAREWELL, to: null }] },
            // "Free for a friend: {tip}"
            freebie: { tip: 'puzzle_hint', mood: 'happy', choices: [{ text: THANKS, to: null }] },
            // "Nothing new today. If Fitz hears something, you hear it first."
            nothing: { choices: [{ text: FAREWELL, to: null }] },
            // "An errand? Fitz knows a guy who sells what the road will not give."
            hello_quest: { marker: 'trade', choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { to: 'wares' },                                                       // "What do you have for my errand?"
                Object.assign({}, HAGGLE_CHOICE),
                { text: FAREWELL, to: null }] },
            // "Mushrooms, shards, gear parts. One at a time, honest prices."
            wares: { choices: [
                { if: { quest: 'witch_mushrooms', coinsMin: 20, not: { item: { glow_mushroom: 3 } } }, effects: [{ coins: -20 }, { item: { glow_mushroom: 1 } }], to: 'bought' },   // "Glow mushroom (20 coins)"
                { if: { quest: 'seer_shards', coinsMin: 25, not: { item: { sun_shard: 3 } } }, effects: [{ coins: -25 }, { item: { sun_shard: 1 } }], to: 'bought' },             // "Sun shard (25 coins)"
                { if: { quest: 'alchemist_gears', coinsMin: 30, not: { item: { gear_part: 3 } } }, effects: [{ coins: -30 }, { item: { gear_part: 1 } }], to: 'bought' },         // "Gear part (30 coins)"
                { text: BACK, to: 'hello_quest' }] },
            // "Pleasure! Genuine goods, no questions asked."
            bought: { mood: 'happy', choices: [
                { to: 'wares' },                                                       // "Anything else?"
                { text: FAREWELL, to: null }] },
            // "Ow, ow, ow! Fitz is leaving. Business is closed."
            won: { mood: 'angry', enter: [{ leave: true }], choices: [{ text: LEAVE, to: null }] },
            // "A fair fight, a fair win. No hard feelings. Maybe a few."
            lost: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] }
        }
    };

    // Chief Krax: the world 2 gate (bribe, witch's charm, duel, mercy) and the world 3 alliance or rematch.
    // The engine itself pays the duel coins and counts chief.beaten / chief.duel_lost.
    GRAPHS.chief = {
        hooks: {
            onAttack: [],
            onDuelWon: [{ if: { world: 1 }, to: 'won' }, { to: 'won_rematch' }],
            onDuelLost: [{ if: { world: 1, flagMin: { 'chief.duel_lost': 2 } }, to: 'mercy' }, { if: { world: 1 }, to: 'lost' }, { to: 'lost_rematch' }]
        },
        nodes: {
            // "Krax remembers your club arm. The way is open, friend."
            respect: { mood: 'happy', marker: 'heart', enter: [{ gate: 'open' }], choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "You already paid. Krax does not forget good coin. Go on through."
            passed: { mood: 'happy', enter: [{ gate: 'open' }], choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "Fitz sent word: a friend of kobolds! Krax opens the gate for you."
            friend: { mood: 'happy', marker: 'heart', enter: [{ gate: 'open' }], choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "Krax is tired of bonking you. Go through. But you owe Krax a favour."
            mercy: { enter: [{ gate: 'open' }, { flag: 'chief.owed' }], choices: [{ text: FAREWELL, to: null }] },
            // "Halt, tin wanderer! Krax guards this gate. Pay coins, or taste club."
            block: { mood: 'angry', marker: 'duel', choices: [
                { to: 'toll' },                                                        // "I'll pay the toll."
                { if: { flag: 'witch.charm', repMin: { coven: 2 }, item: { hex_charm: 1 } }, effects: [{ item: { hex_charm: -1 } }, { gate: 'open' }, { flag: 'chief.charmed' }], to: 'charmed' },   // "Grimhild sent this hex charm."
                { text: FIGHT, if: { feature: 'duel' }, effects: [{ duel: true }] },
                { to: null }] },                                                       // "Let me think."
            // "Forty coins for friends of kobolds. Sixty for everybody else."
            toll: { choices: [
                { if: { repMin: { kobolds: 0 }, coinsMin: 40 }, effects: [{ coins: -40 }, { rep: { kobolds: 1 } }, { gate: 'open' }, { flag: 'chief.paid' }], to: 'paid' },   // "Pay 40 coins."
                { if: { repMax: { kobolds: -1 }, coinsMin: 60 }, effects: [{ coins: -60 }, { rep: { kobolds: 1 } }, { gate: 'open' }, { flag: 'chief.paid' }], to: 'paid' },  // "Pay 60 coins."
                { text: BACK, to: 'block' }] },
            // "Clink, clink! Krax likes your coins. Gate is open."
            paid: { mood: 'happy', choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "A hex charm! Krax hates witch tricks, but a deal is a deal. Gate is open."
            charmed: { choices: [{ text: FAREWELL, to: null }] },
            // "Krax is beaten! You hit hard. Krax respects hard. Gate is open."
            won: { mood: 'happy', enter: [{ gate: 'open' }, { rep: { kobolds: 1 } }, { flag: 'chief.respect' }], choices: [{ text: FAREWELL, to: null }] },
            // "Ha! Small one fights bravely. Krax fights better."
            lost: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] },

            // ---- world 3 ----
            // "My friend! The cave-whispers tell Krax things: {tip}"
            ally: { mood: 'happy', marker: 'heart', tip: 'boss_weakness', choices: [
                { if: { notFlag: 'chief.gifted' }, effects: [{ coins: 20 }, { flag: 'chief.gifted' }], to: 'gifted' },   // "Accept Krax's gift."
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] },
            // "Take it! Krax has more shinies than a dragon."
            gifted: { mood: 'happy', choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "You owe Krax! Thirty coins, or we bonk again."
            debt: { mood: 'angry', marker: 'duel', choices: [
                { if: { coinsMin: 30 }, effects: [{ coins: -30 }, { unflag: 'chief.owed' }, { rep: { kobolds: 1 } }], to: 'settled' },   // "Pay 30 coins."
                { text: FIGHT, if: { feature: 'duel' }, effects: [{ duel: true }] },
                { text: LEAVE, to: null }] },
            // "Krax remembers the witch's trick. Fifty coins to forget it, or we bonk."
            grudge: { mood: 'angry', marker: 'duel', choices: [
                { if: { coinsMin: 50 }, effects: [{ coins: -50 }, { unflag: 'chief.charmed' }, { rep: { kobolds: 1 } }], to: 'settled' },   // "Pay 50 coins."
                { text: FIGHT, if: { feature: 'duel' }, effects: [{ duel: true }] },
                { text: LEAVE, to: null }] },
            // "Debt gone. Krax and tin wanderer are even."
            settled: { mood: 'happy', choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "Tin wanderer again! Krax guards the road for Fitz. Buying or leaving?"
            meet: { choices: [{ text: TRADE, effects: [{ trade: true }] }, { text: FAREWELL, to: null }] },
            // "Krax lost again! Fine, fine. Krax calls you friend now."
            won_rematch: { mood: 'happy', enter: [{ unflag: ['chief.owed', 'chief.charmed'] }, { rep: { kobolds: 1 } }, { flag: 'chief.respect' }], choices: [{ text: FAREWELL, to: null }] },
            // "Krax wins! Pay your debt next time, or bring a bigger sword."
            lost_rematch: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] }
        }
    };

    root.RunenpfadNpcStory = { MEETINGS: MEETINGS, GRAPHS: GRAPHS };
})(typeof window !== 'undefined' ? window : globalThis);

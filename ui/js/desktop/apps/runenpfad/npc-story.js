/* Runenpfad NPC story data: meetings and dialogue graphs (witch world 1, merchant base). DOM-free data only.
 * Comments hold the English source text for the i18n files. Interpreted by dialogue.js; npc-story-late.js appends later arcs. */
(function (root) {
    'use strict';

    const MEETINGS = [];
    const GRAPHS = {};
    const FAREWELL = 'runenpfad.npc.c.farewell';
    const THANKS = 'runenpfad.npc.c.thanks';
    const TRADE = 'runenpfad.npc.c.trade';

    MEETINGS.push(
        { id: 'witch_w1', npcId: 'witch', levels: [2, 4], starts: [
            { if: { quest: 'witch_mushrooms', item: { glow_mushroom: 3 } }, to: 'turnin' },
            { if: { quest: 'witch_mushrooms' }, to: 'waiting' },
            { if: { questDone: 'witch_mushrooms' }, to: 'friend' },
            { if: { flag: 'witch.refused' }, to: 'cold' },
            { to: 'hello' }] },
        { id: 'merchant_campaign', npcId: 'merchant', levels: [3, 9, 12, 18], starts: [
            { if: { flag: 'merchant.fake_sold' }, to: 'confront_fake' },
            { to: 'hello' }] },
        { id: 'merchant_endless', npcId: 'merchant', levels: 'endless', starts: [
            { if: { flag: 'merchant.fake_sold' }, to: 'confront_fake' },
            { to: 'hello' }] }
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
            won_grudge: { mood: 'angry', enter: [{ coins: 20 }, { leave: true }], choices: [{ text: 'runenpfad.npc.c.leave', to: null }] },
            // "Ha! You fight like a thorn bush. Take your prize."
            won: { mood: 'happy', enter: [{ coins: 20 }], choices: [{ text: FAREWELL, to: null }] },
            // "Back to the mud with you. And mind your manners."
            lost: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] }
        }
    };

    // The kobold trader: trade, a rumour, a haggle roll and the fake-goods confrontation. Duel results need no node.
    GRAPHS.merchant = {
        hooks: { onAttack: [], onDuelWon: [], onDuelLost: [] },
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
            refund: { choices: [{ text: FAREWELL, to: null }] }
        }
    };

    root.RunenpfadNpcStory = { MEETINGS: MEETINGS, GRAPHS: GRAPHS };
})(typeof window !== 'undefined' ? window : globalThis);

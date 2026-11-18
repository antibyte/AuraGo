/* Runenpfad NPC story data, late arcs: Saida (desert seer), Aldric (tower wizard), Brass (clockwork alchemist) and the
 * world 5 finale lines. DOM-free data only. Comments hold the English source text for the i18n files.
 * Appends to RunenpfadNpcStory (created by npc-story.js); interpreted by dialogue.js. */
(function (root) {
    'use strict';

    const S = root.RunenpfadNpcStory || (root.RunenpfadNpcStory = { MEETINGS: [], GRAPHS: {} });
    const MEETINGS = S.MEETINGS;
    const GRAPHS = S.GRAPHS;
    const FAREWELL = 'runenpfad.npc.c.farewell';
    const THANKS = 'runenpfad.npc.c.thanks';
    const TRADE = 'runenpfad.npc.c.trade';
    const LEAVE = 'runenpfad.npc.c.leave';
    const FIGHT = 'runenpfad.npc.c.fight';
    const TRADE_CHOICE = { text: TRADE, effects: [{ trade: true }] };
    const KRAX_ANY = { any: [{ flag: 'chief.paid' }, { flag: 'chief.respect' }, { flag: 'chief.charmed' }] };

    const SEER_STARTS = [
        { if: { quest: 'seer_shards', item: { sun_shard: 3 } }, to: 'turnin' },
        { if: { quest: 'seer_shards' }, to: 'waiting' },
        { if: { questDone: 'seer_shards' }, to: 'friend' },
        { if: { repMax: { circle: -2 } }, to: 'shun' },
        { if: { repMin: { circle: 2 } }, to: 'honored' },
        { to: 'hello' }];

    const ALCHEMIST_STARTS = [
        { if: { quest: 'alchemist_gears', item: { gear_part: 3 } }, to: 'turnin' },
        { if: { quest: 'alchemist_gears' }, to: 'waiting' },
        { if: { questDone: 'alchemist_gears', notFlag: 'alchemist.revealed' }, to: 'reveal' },
        { if: { questDone: 'alchemist_gears' }, to: 'known' },
        { if: { flag: 'wizard.warned', notFlag: 'alchemist.met' }, to: 'wary' },
        { to: 'hello' }];

    MEETINGS.push(
        { id: 'seer_w3a', npcId: 'seer', levels: [11], starts: SEER_STARTS },
        { id: 'seer_w3b', npcId: 'seer', levels: [14], starts: SEER_STARTS },
        { id: 'wizard_w4a', npcId: 'wizard', levels: [17], starts: [
            { if: { any: [{ repMin: { circle: 1 } }, { flag: 'seer.letter' }, { flag: 'wizard.proven' }] }, to: 'welcome' },
            { to: 'challenge' }] },
        { id: 'wizard_w4b', npcId: 'wizard', levels: [19], starts: [
            { if: { flag: 'wizard.warned' }, to: 'known' },
            { if: { any: [{ repMin: { circle: 1 } }, { flag: 'seer.letter' }, { flag: 'wizard.proven' }] }, to: 'gift' },
            { to: 'cold' }] },
        { id: 'wizard_w5', npcId: 'wizard', levels: [24], starts: [
            { if: { flag: 'alchemist.sabotage', notFlag: 'wizard.told' }, to: 'sabotage_known' },
            { if: { repMin: { circle: 2 }, notFlag: 'wizard.allied' }, to: 'ally' },
            { to: 'neutral' }] },
        { id: 'wizard_finale', npcId: 'wizard', levels: [25], starts: [
            { if: { flag: 'wizard.blessed' }, to: 'f_again' },
            { if: { repMin: { circle: 3 }, flag: 'seer.friend' }, to: 'f_honor_seer' },
            { if: { repMin: { circle: 3 } }, to: 'f_honor' },
            { if: { flag: 'wizard.proven' }, to: 'f_proven' },
            { if: { repMax: { circle: -1 } }, to: 'f_cold' },
            { to: 'f_fair' }] },
        { id: 'alchemist_w5a', npcId: 'alchemist', levels: [21], starts: ALCHEMIST_STARTS },
        { id: 'alchemist_w5b', npcId: 'alchemist', levels: [23], starts: ALCHEMIST_STARTS },
        { id: 'alchemist_finale', npcId: 'alchemist', levels: [25], starts: [
            { if: { flag: 'alchemist.blessed' }, to: 'f_again' },
            { if: { repMin: { coven: 3 }, flag: 'witch.helped' }, to: 'f_witch' },
            { if: { repMin: { coven: 3 } }, to: 'f_honor' },
            { if: KRAX_ANY, to: 'f_krax' },
            { if: { repMax: { coven: -1 } }, to: 'f_cold' },
            { to: 'f_fair' }] }
    );

    // ---- Saida, the desert seer (circle, world 3) ----
    GRAPHS.seer = {
        hooks: { onAttack: [], onDuelWon: [], onDuelLost: [] },
        nodes: {
            // "The sands told me a wanderer was coming. I am Saida. Fifteen coins and I read the dunes for you. Or do me a favour, and I read them for free."
            hello: { marker: 'quest', enter: [{ flag: 'seer.met' }], choices: [
                { if: { repMin: { circle: 0 }, coinsMin: 15 }, effects: [{ coins: -15 }, { flag: 'seer.paid' }], to: 'hint' },   // "Read the dunes for me. (15 coins)"
                { if: { repMax: { circle: -1 }, coinsMin: 30 }, effects: [{ coins: -30 }, { flag: 'seer.paid' }], to: 'hint' },   // "The circle's price, then. (30 coins)"
                { effects: [{ quest: 'seer_shards' }], to: 'favour' },               // "Is there a favour I can do instead?"
                { text: TRADE, effects: [{ trade: true }] }] },
            // "The circle's friends read the dunes for free. Sit, wanderer: the sand is listening."
            honored: { mood: 'happy', choices: [
                { to: 'hint' },                                                      // "A free reading? Gladly."
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] },
            // "Wizards' friends pay double here, and the sand has no patience for the rest. Thirty coins, or earn it."
            shun: { mood: 'angry', choices: [
                { if: { coinsMin: 30 }, effects: [{ coins: -30 }, { flag: 'seer.paid' }], to: 'hint' },   // "Fine. Thirty coins."
                { effects: [{ quest: 'seer_shards' }], to: 'favour' },               // "I'll earn it. What do you need?"
                { text: LEAVE, to: null }] },
            // "Three sun shards lie scattered in the dunes ahead, glinting where the mirage ends. Bring them to me and the sand will remember you kindly."
            favour: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] },
            // "Close your eyes. The dunes say: {tip}"
            hint: { tip: 'puzzle_hint', choices: [
                { if: { notFlag: 'seer.riddle' }, to: 'riddle' },                    // "Do you have a riddle for me?"
                { text: THANKS, to: null }] },
            // "A riddle, then. I am always ahead of you, never reached; wet to the eye, dry to the tongue. What am I?"
            riddle: { choices: [
                { to: 'riddle_no' },                                                 // "Your shadow."
                { effects: [{ flag: 'seer.riddle' }, { rep: { circle: 1 } }], to: 'riddle_ok' },   // "A mirage."
                { to: 'riddle_no' }] },                                              // "I give up."
            // "Sharp as a desert hawk! The circle will hear of it. And a gift: {tip}"
            riddle_ok: { mood: 'happy', tip: 'key_dir', choices: [{ text: THANKS, to: null }] },
            // "Not quite. It was a mirage, and you are still walking towards it."
            riddle_no: { choices: [{ text: FAREWELL, to: null }] },
            // "Still {count} sun shards out there. They glint where the mirage ends."
            waiting: { vars: { count: 'need:sun_shard:3' }, choices: [
                { to: null },                                                        // "On my way."
                { text: TRADE, effects: [{ trade: true }] }] },
            // "Three sun shards! The desert is generous, and so am I."
            turnin: { mood: 'happy', marker: 'heart', choices: [
                { if: { item: { sun_shard: 3 } }, effects: [{ item: { sun_shard: -3 } }, { questDone: 'seer_shards' }, { rep: { circle: 1 } }, { give: 'rune_magnet' }, { flag: 'seer.friend' }], to: 'thanks' },   // "Here they are."
                { to: null }] },                                                     // "Not yet."
            // "Take this magnet rune, and one last sight: {tip}"
            thanks: { mood: 'happy', tip: 'gems_left', choices: [{ text: THANKS, to: null }] },
            // "My friend! The sand sings your name. I sealed a letter for Aldric, the wizard of the glacier tower. It opens his door."
            friend: { mood: 'happy', enter: [{ flag: 'seer.letter' }], choices: [
                { to: 'hint' },                                                      // "Any hint for the puzzles?"
                { to: 'keytip' },                                                    // "Where is the key?"
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] },
            // "{tip}"
            keytip: { tip: 'key_dir', choices: [{ text: THANKS, to: null }] }
        }
    };

    // ---- Aldric, the tower wizard (circle, worlds 4 and 5) ----
    GRAPHS.wizard = {
        hooks: {
            onAttack: [{ flag: 'wizard.provoked' }],
            onDuelWon: [{ to: 'proven' }],
            onDuelLost: [{ to: 'lost' }]
        },
        nodes: {
            // "Welcome, friend of the circle! I am Aldric. The ice dragons of these peaks fear a few things, and the books know which: {tip}"
            welcome: { mood: 'happy', marker: 'talk', tip: 'boss_weakness', enter: [{ flag: 'wizard.met' }], choices: [
                { text: TRADE, effects: [{ trade: true }] },
                { to: 'lore' },                                                      // "Any rune lore?"
                { text: FAREWELL, to: null }] },
            // "Runes are older than kings, and they obey patterns. Remember this: {tip}"
            lore: { tip: 'puzzle_hint', choices: [{ text: THANKS, to: null }] },
            // "A stranger at my tower with no word from the circle? Aldric tests those he does not know. Prove yourself in a duel!"
            challenge: { mood: 'angry', marker: 'duel', choices: [
                { if: { feature: 'duel' }, text: FIGHT, effects: [{ duel: true }] },
                { to: 'study' },                                                     // "Can I earn your trust another way?"
                { text: LEAVE, to: null }] },
            // "Read the stones of this valley, and be kind to the circle's friends. Return when the circle knows your name."
            study: { choices: [{ text: FAREWELL, to: null }] },
            // "Well fought! Few hold their ground against Aldric. The circle shall hear of this, and so shall you: {tip}"
            proven: { mood: 'happy', tip: 'boss_weakness', enter: [{ rep: { circle: 1 } }, { flag: 'wizard.proven' }], choices: [{ text: THANKS, to: null }] },
            // "Study, and return. The circle does not rush, and neither should you."
            lost: { choices: [{ text: FAREWELL, to: null }] },
            // "You return! Take this frost rune: ice answers those who respect it. And a warning, wanderer: beware the alchemist in the citadel."
            gift: { mood: 'happy', marker: 'heart', enter: [{ flag: 'wizard.warned' }, { give: 'rune_frost' }], choices: [
                { to: 'rumour' },                                                    // "What is wrong with Brass?"
                { text: FAREWELL, to: null }] },
            // "Brass builds clever machines and keeps cleverer secrets. Ask what the great mech is for. I never got a straight answer."
            rumour: { choices: [{ text: THANKS, to: null }] },
            // "The ice holds its breath, wanderer. Walk lightly, and watch the citadel."
            known: { choices: [TRADE_CHOICE, { text: FAREWELL, to: null }] },
            // "The tower is closed to strangers the circle does not know. Come back with proof of good standing."
            cold: { mood: 'angry', choices: [
                { if: { coinsMin: 40 }, effects: [{ coins: -40 }, { rep: { circle: 1 } }], to: 'donated' },   // "A gift to the tower library. (40 coins)"
                { text: FAREWELL, to: null }] },
            // "The library thanks you. Consider the circle's door ajar."
            donated: { mood: 'happy', choices: [{ text: THANKS, to: null }] },
            // "You talked with Brass, and it gave up the mech's weak seam? Remarkable. The circle will remember who dared."
            sabotage_known: { mood: 'happy', enter: [{ flag: 'wizard.told' }, { rep: { circle: 1 } }], choices: [{ text: THANKS, to: null }] },
            // "Aldric the Grey greets a friend of the circle! Take this storm rune and a purse: the citadel is no place for empty pockets."
            ally: { mood: 'happy', marker: 'heart', enter: [{ flag: 'wizard.allied' }, { give: 'rune_storm' }, { coins: 30 }], choices: [
                { to: 'neutral' },                                                   // "Any advice on the great mech?"
                { text: FAREWELL, to: null }] },
            // "The citadel's guardian is a machine of brass and steam. A scholar's note: {tip}"
            neutral: { tip: 'boss_weakness', choices: [{ text: THANKS, to: null }] },

            // ---- finale (world 5, before the boss arena): the reward follows the circle's regard ----
            // "Saida's letters, the dunes' favour and the circle's trust: you have earned the tower's full blessing. Take this storm rune and forty coins. The great mech: {tip}"
            f_honor_seer: { mood: 'happy', marker: 'heart', tip: 'boss_weakness', enter: [{ flag: 'wizard.blessed' }, { give: 'rune_storm' }, { coins: 40 }], choices: [{ text: THANKS, to: null }] },
            // "The circle speaks well of you. Take forty coins and my blessing. The great mech: {tip}"
            f_honor: { mood: 'happy', marker: 'heart', tip: 'boss_weakness', enter: [{ flag: 'wizard.blessed' }, { coins: 40 }, { give: 'heart' }], choices: [{ text: THANKS, to: null }] },
            // "You proved yourself in my own tower, and I do not forget a fair fight. Take fifteen coins and a little luck. The great mech: {tip}"
            f_proven: { mood: 'happy', tip: 'boss_weakness', enter: [{ flag: 'wizard.blessed' }, { coins: 15 }, { give: 'heart' }], choices: [{ text: THANKS, to: null }] },
            // "The circle does not know you well, wanderer, but the road has been kind to you. A healing potion, then. The great mech: {tip}"
            f_fair: { tip: 'boss_weakness', enter: [{ flag: 'wizard.blessed' }, { give: 'potion' }], choices: [{ text: THANKS, to: null }] },
            // "You crossed the circle, and the circle remembers. I will not stop you. I will not help you either."
            f_cold: { mood: 'angry', enter: [{ flag: 'wizard.blessed' }], choices: [{ text: LEAVE, to: null }] },
            // "Go, wanderer. The mech will not wait, and neither will I."
            f_again: { choices: [{ text: FAREWELL, to: null }] }
        }
    };

    // ---- Brass, the clockwork alchemist (coven, world 5) ----
    GRAPHS.alchemist = {
        hooks: { onAttack: [], onDuelWon: [], onDuelLost: [] },
        nodes: {
            // "Mind the steam! I am Brass: tinkerer, brewer, the citadel's only honest alchemist. I am short of three clockwork gears. Interested?"
            hello: { marker: 'quest', enter: [{ flag: 'alchemist.met' }], choices: [
                { effects: [{ quest: 'alchemist_gears' }], to: 'accept' },           // "I'll fetch your gears."
                { to: 'parts' },                                                     // "Can you sell me parts instead?"
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] },
            // "Aldric warned you about me, did he? Wizards fear anything with gears. Ask what you like, I have nothing to hide. Mostly."
            wary: { mood: 'angry', choices: [
                { to: 'hello' },                                                     // "Then let us talk business."
                { text: FAREWELL, to: null }] },
            // "Splendid! The gears hide in the high rooms, guarded by clockwork knights. Bring me three and I will make it worth your while."
            accept: { mood: 'happy', choices: [{ text: FAREWELL, to: null }] },
            // "Gears are my bread. Twenty coins each, no haggling: the bots never pay me."
            parts: { choices: [
                { if: { coinsMin: 20 }, effects: [{ coins: -20 }, { item: { gear_part: 1 } }], to: 'parts' },   // "One gear, please. (20 coins)"
                { text: TRADE, effects: [{ trade: true }] },
                { text: FAREWELL, to: null }] },
            // "Still {count} gears missing. Check the high rooms, past the clockwork knights."
            waiting: { vars: { count: 'need:gear_part:3' }, choices: [
                { to: null },                                                        // "On my way."
                { to: 'parts' },                                                     // "Could I buy the rest?"
                { text: TRADE, effects: [{ trade: true }] }] },
            // "Three gears, still warm! Brass keeps promises: two discount tokens for the workshop, and my thanks."
            turnin: { mood: 'happy', marker: 'heart', choices: [
                { if: { item: { gear_part: 3 } }, effects: [{ item: { gear_part: -3 } }, { questDone: 'alchemist_gears' }, { rep: { coven: 1 } }, { item: { discount_token: 2 } }], to: 'reveal' },   // "Here you are."
                { to: null }] },                                                     // "Not yet."
            // "Between friends: my mech guards the citadel heart. Brass built it, and regrets it a little."
            reveal: { enter: [{ flag: 'alchemist.revealed' }], choices: [
                { if: { repMin: { coven: 2 } }, to: 'sabotage' },                    // "Can it be stopped?"
                { if: { repMax: { coven: 1 } }, to: 'deny' },                        // "Can it be stopped?"
                { text: FAREWELL, to: null }] },
            // "There is a loose seam in its left shoulder plate. One good hit and the mech loses a fifth of its strength. Do not tell Aldric I said so."
            sabotage: { mood: 'happy', marker: 'heart', enter: [{ flag: 'alchemist.sabotage' }], choices: [{ text: THANKS, to: null }] },
            // "A machine that guards a citadel does not come with a switch. Brass wishes you luck, nothing more."
            deny: { choices: [{ text: FAREWELL, to: null }] },
            // "Back with oil on your sleeves? Browse, browse."
            known: { choices: [TRADE_CHOICE, { text: FAREWELL, to: null }] },

            // ---- finale (world 5, before the boss arena): the reward follows the coven's regard ----
            // "Grimhild sent word ahead: a mushroom-fetcher with true gears. The coven pays its debts. Take forty coins and a potion."
            f_witch: { mood: 'happy', marker: 'heart', enter: [{ flag: 'alchemist.blessed' }, { coins: 40 }, { give: 'potion' }], choices: [{ text: THANKS, to: null }] },
            // "The coven speaks well of you. Take forty coins and a potion, and may the mech be kind."
            f_honor: { mood: 'happy', marker: 'heart', enter: [{ flag: 'alchemist.blessed' }, { coins: 40 }, { give: 'potion' }], choices: [{ text: THANKS, to: null }] },
            // "Krax's runners bring me scrap, and they tell me about a wanderer who dealt with their chief. Here, twenty-five coins and a heart-spring for luck."
            f_krax: { mood: 'happy', enter: [{ flag: 'alchemist.blessed' }, { coins: 25 }, { give: 'heart' }], choices: [{ text: THANKS, to: null }] },
            // "Another hero-shaped parcel of trouble. Brass will not stop you, but will not help either."
            f_cold: { mood: 'angry', enter: [{ flag: 'alchemist.blessed' }], choices: [{ text: LEAVE, to: null }] },
            // "A traveller without friends, but with a good heart, I hope. Have a potion for the road."
            f_fair: { enter: [{ flag: 'alchemist.blessed' }, { give: 'potion' }], choices: [{ text: THANKS, to: null }] },
            // "The mech waits. Try not to leave pieces of yourself behind."
            f_again: { choices: [{ text: FAREWELL, to: null }] }
        }
    };
})(typeof window !== 'undefined' ? window : globalThis);

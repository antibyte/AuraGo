import assert from 'node:assert/strict';
import { readdirSync, readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const context = { window: {}, TextEncoder };
vm.createContext(context);
for (const name of ['content', 'content-act1', 'content-act2', 'content-act3', 'content-act4', 'state', 'commands']) {
    vm.runInContext(readFileSync(resolve(root, 'ui/js/desktop/apps/goldfish-adventure', name + '.js'), 'utf8'), context);
}
const C = context.window.GoldfishContent, S = context.window.GoldfishState, K = context.window.GoldfishCommands;
const lang = (section, locale = 'de') => JSON.parse(readFileSync(resolve(root, 'ui/lang', section, locale + '.json'), 'utf8'));
const story = lang('goldfish'), strings = lang('goldfish-interface');
C.texts = story;
const t = (key, parameters = {}) => (story[key] ?? strings[key] ?? key).replace(/\{(\w+)\}/g, (match, name) => parameters[name] ?? match);
const action = id => C.actions.find(value => value.id === id);
const hotspot = (room, id) => C.rooms.find(value => value.id === room).hotspots.find(value => value.id === id);
// Arrays built inside the vm context have another prototype; copy ids into this realm before deepEqual.
const ids = list => Array.from(list, record => record.id);

// Fresh state at R01 and a state in R02 that holds the clamp.
const fresh = S.create();
let held = S.create();
for (const id of ['r01_stop', 'r01_clamp']) held = S.apply(held, id).state;
held = S.apply(S.apply(held, 'r01_brake', { answer: action('r01_brake').challenge.answer }).state, 'r01_open').state;
held = S.move(held, 'R02').state;

// Default action: one / many / item / look.
assert.equal(K.defaultAction(fresh, hotspot('R01', 'conveyor')).kind, 'one');
assert.equal(K.defaultAction(fresh, hotspot('R01', 'conveyor')).action.id, 'r01_stop');
const scrap = K.defaultAction(fresh, hotspot('R01', 'scrap'));
assert.equal(scrap.kind, 'many');
assert.deepEqual(ids(scrap.actions), ['r01_colleague', 'r01_horizon']);
assert.equal(K.defaultAction(fresh, hotspot('R01', 'grabber')).kind, 'look', 'An action behind a missing flag is not offered');
assert.equal(K.defaultAction(held, hotspot('R02', 'record')).action.id, 'r02_record');
const taken = S.apply(held, 'r02_record').state;
const gated = K.defaultAction(taken, hotspot('R02', 'record'));
assert.equal(gated.kind, 'item', 'Only the clamp action is left, and it needs an item');
assert.deepEqual(ids(gated.actions), ['r02_peel']);
assert.equal(K.defaultAction(held, { actions: [], requires: [] }).kind, 'look');
assert.equal(K.defaultAction(fresh, { ...hotspot('R01', 'conveyor'), requires: ['p01_done'] }).kind, 'look', 'A hotspot behind a missing flag offers nothing');

// Verbs: the authored verb answers, item-granting actions also answer to take (Nehmen) and use.
assert.deepEqual([K.verbOf(action('r01_stop')), K.verbOf(action('r01_colleague')), K.verbOf(action('r02_record')), K.verbOf(action('r01_clamp'))], ['use', 'talk', 'take', 'take']);
const forVerb = (state, spot, verb) => ids(K.actionsFor(state, spot, { verb }));
assert.deepEqual(forVerb(fresh, hotspot('R01', 'scrap'), 'talk'), ['r01_colleague']);
assert.deepEqual(forVerb(fresh, hotspot('R01', 'scrap'), 'use'), ['r01_horizon']);
assert.deepEqual(forVerb(fresh, hotspot('R01', 'scrap'), 'take'), [], 'Nothing on the scrap heap can be taken');
assert.deepEqual(forVerb(fresh, hotspot('R01', 'conveyor'), 'look'), [], 'Ansehen never matches an action');
const stopped = S.apply(fresh, 'r01_stop').state;
assert.deepEqual(forVerb(stopped, hotspot('R01', 'grabber'), 'take'), ['r01_clamp'], 'Nehmen matches an action that adds items');
assert.deepEqual(forVerb(stopped, hotspot('R01', 'grabber'), 'use'), ['r01_clamp'], 'The same action still answers to Benutzen');
assert.deepEqual(forVerb(stopped, hotspot('R01', 'grabber'), 'talk'), []);
assert.deepEqual(forVerb(held, hotspot('R02', 'record'), 'take'), ['r02_record'], 'An authored take action answers to Nehmen');
assert.deepEqual(forVerb(held, hotspot('R02', 'record'), 'use'), [], 'A pure take action does not answer to Benutzen');
assert.equal(C.actions.filter(record => (record.effects?.items || []).length && record.verb === 'use').every(record => K.matches(record, 'take') && K.matches(record, 'use')), true, 'Every item-granting use action answers to both');

// Items: an armed item selects only the actions that need it; verbs and items stay separate.
assert.deepEqual(ids(K.actionsFor(taken, hotspot('R02', 'record'), { item: 'tool_clamp' })), ['r02_peel']);
assert.deepEqual(ids(K.actionsFor(taken, hotspot('R02', 'record'), { item: 'own_record' })), [], 'A wrong item matches nothing');
assert.deepEqual(ids(K.actionsFor(taken, hotspot('R02', 'record'), {})), [], 'Without an item, item actions are hidden');
assert.deepEqual(ids(K.needsItem(taken, hotspot('R02', 'record'), 'use')), ['r02_peel']);
assert.deepEqual(ids(K.needsItem(taken, hotspot('R02', 'record'), 'talk')), []);
assert.deepEqual(ids(K.needsItem(fresh, hotspot('R01', 'conveyor'), 'use')), []);

// Sentence assembly from the interface strings.
const grabber = hotspot('R01', 'grabber'), label = t(grabber.label);
assert.equal(K.sentence(t, { verb: 'use', hotspot: grabber }), strings['goldfish.ui.sentence_use'].replace('{object}', label));
assert.equal(K.sentence(t, { verb: 'take', hotspot: grabber }), strings['goldfish.ui.sentence_take'].replace('{object}', label));
assert.equal(K.sentence(t, { verb: 'take' }), strings['goldfish.ui.verb_take'] + ' …', 'A verb without an object trails off');
assert.equal(K.sentence(t, { hotspot: grabber }), label, 'No verb: just the object');
assert.equal(K.sentence(t, { item: 'tool_clamp' }), strings['goldfish.ui.sentence_use_item_open'].replace('{item}', t(C.items.find(value => value.id === 'tool_clamp').name)));
assert.equal(K.sentence(t, { item: 'tool_clamp', hotspot: grabber }), strings['goldfish.ui.sentence_use_item'].replace('{item}', t(C.items[0].name)).replace('{object}', label));
assert.equal(K.sentence(t, { exit: C.rooms[0].exits[0], verb: 'use' }), t(C.rooms[0].exits[0].label), 'Exits show their own label');

// Refusals alternate between the _1 and _2 line and every key exists in all sixteen locales.
assert.deepEqual([1, 2, 3, 4].map(count => K.refusalKey('take', count)), ['goldfish.ui.refuse_take_1', 'goldfish.ui.refuse_take_2', 'goldfish.ui.refuse_take_1', 'goldfish.ui.refuse_take_2']);
const keys = [...K.VERBS.flatMap(verb => ['goldfish.ui.verb_' + verb, 'goldfish.ui.sentence_' + verb]),
    ...['take', 'use', 'talk', 'item'].flatMap(kind => [K.refusalKey(kind, 1), K.refusalKey(kind, 2)])];
const locales = readdirSync(resolve(root, 'ui/lang/goldfish-interface')).map(name => name.replace('.json', ''));
assert.equal(locales.length, 16);
for (const locale of locales) {
    const words = lang('goldfish-interface', locale);
    for (const key of keys) assert.ok(typeof words[key] === 'string' && words[key].length > 0, locale + ' lacks ' + key);
}
console.log('Goldfish commands: default actions, verb filter and Nehmen rule, item arming, sentences and refusals passed.');

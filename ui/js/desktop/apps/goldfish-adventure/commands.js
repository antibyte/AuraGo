/* Pure command logic of the SCUMM-style bar: no DOM, no timers, no storage. */
(function () {
    'use strict';

    const VERBS = Object.freeze(['look', 'take', 'use', 'talk']);
    const content = () => window.GoldfishContent;
    const logic = () => window.GoldfishState;
    const actionRecord = id => content().actions.find(value => value.id === id);
    const itemRecord = id => (content().items || []).find(value => value.id === id);
    const grantsItems = record => ((record.effects && record.effects.items) || []).length > 0;

    // The authored verb of an action, normalised to use|talk|take.
    const ownVerb = record => record.verb === 'talk' || record.verb === 'take' ? record.verb : 'use';

    // The verb an action answers to. Item-granting actions (except talk) read as "take".
    function verbOf(record) {
        if (record.verb === 'talk') return 'talk';
        return record.verb === 'take' || grantsItems(record) ? 'take' : 'use';
    }

    // Verb filter: the authored verb always matches; "take" additionally matches item-granting actions, but never
    // a conversation (you do not "take" a talk, even when it ends with someone handing you something).
    function matches(record, verb) {
        return verb === ownVerb(record) || (verb === 'take' && record.verb !== 'talk' && grantsItems(record));
    }

    function candidates(state, hotspot) {
        if (!hotspot || !logic().conditions(state, hotspot)) return [];
        return (hotspot.actions || []).map(actionRecord).filter(record => record && logic().available(state, record));
    }

    // Available actions of a hotspot for the armed verb/item. Without either: the actions that need no item.
    function actionsFor(state, hotspot, { verb, item } = {}) {
        const available = candidates(state, hotspot);
        if (item) return available.filter(record => record.item === item);
        const plain = available.filter(record => !record.item);
        if (verb === 'look') return [];
        return verb ? plain.filter(record => matches(record, verb)) : plain;
    }

    // Available item actions (optionally narrowed by verb) the player cannot run without the item.
    function needsItem(state, hotspot, verb) {
        return candidates(state, hotspot).filter(record => record.item && (!verb || verb === 'look' || matches(record, verb)));
    }

    function defaultAction(state, hotspot) {
        const available = candidates(state, hotspot);
        const plain = available.filter(record => !record.item);
        if (plain.length === 1) return { kind: 'one', action: plain[0] };
        if (plain.length > 1) return { kind: 'many', actions: plain };
        const gated = available.filter(record => record.item);
        return gated.length ? { kind: 'item', actions: gated } : { kind: 'look' };
    }

    function itemName(t, item) {
        if (!item) return '';
        if (typeof item === 'string') {
            const record = content() && itemRecord(item);
            return record ? t(record.name) : item;
        }
        return t(item.name);
    }

    // The text for the sentence line. `verb` may also be "walk"; `item` is an item record or id.
    function sentence(t, { verb, hotspot, exit, item } = {}) {
        if (exit) return t(exit.label);
        const object = hotspot ? t(hotspot.label) : '';
        if (item) {
            const name = itemName(t, item);
            return hotspot ? t('goldfish.ui.sentence_use_item', { item: name, object }) : t('goldfish.ui.sentence_use_item_open', { item: name });
        }
        if (!verb) return object;
        if (!hotspot) return t('goldfish.ui.verb_' + verb) + ' …';
        return t('goldfish.ui.sentence_' + verb, { object });
    }

    // Alternating refusal lines: goldfish.ui.refuse_<kind>_1 and _2.
    const refusalKey = (kind, count) => 'goldfish.ui.refuse_' + kind + '_' + (count % 2 ? 1 : 2);

    window.GoldfishCommands = Object.freeze({ VERBS, ownVerb, verbOf, matches, actionsFor, needsItem, defaultAction, sentence, refusalKey });
}());

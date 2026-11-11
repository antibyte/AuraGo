#!/usr/bin/env node
// A built-in trigger sample shows its text fields in the UI language; a saved one is user data.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const box = vm.createContext({ window: {}, console, Intl, Date, Map, Set, JSON });
const file = new URL('../ui/js/desktop/apps/easydrag-testing.js', import.meta.url);
vm.runInContext(fs.readFileSync(file, 'utf8'), box, { filename: file.pathname });
const ED = box.window.EasyDrag;
const words = lang => JSON.parse(fs.readFileSync(new URL('../ui/lang/easydrag/' + lang + '.json', import.meta.url), 'utf8'));
const infos = new Map([
    ['trigger.email', { type: 'trigger.email', trigger: true, sample_text_keys: { '': { subject: 'easydrag.sample.mail_subject', body: 'easydrag.sample.mail_body' } } }],
    ['trigger.planner', { type: 'trigger.planner', trigger: true, sample_text_keys: { '': { title: 'easydrag.sample.planner_appointment_title' }, todo_overdue: { title: 'easydrag.sample.planner_todo_title' } } }],
    ['trigger.person_detected', { type: 'trigger.person_detected', trigger: true, sample_text_keys: { '': { camera_name: 'easydrag.sample.person_camera', 'zones.0': 'easydrag.sample.person_camera' } } }]
]);
const editor = lang => { const dict = words(lang); return { t: key => dict[key] || key, model: { info: type => infos.get(type) } }; };
const mail = { id: 'n_mail', type: 'trigger.email', params: {} };
const builtinMail = () => ({ source: 'builtin', data: { subject: 'October invoice', from: 'shop@example.com', body: 'Hello, please find the invoice attached.' } });

assert.equal(ED.testing.localSample(editor('en'), mail, builtinMail()).subject, 'October invoice');
const de = ED.testing.localSample(editor('de'), mail, builtinMail());
assert.deepEqual([de.subject, de.body, de.from], ['Rechnung Oktober', 'Hallo, anbei die Rechnung.', 'shop@example.com']);
assert.equal(ED.testing.localSample(editor('de'), mail, { source: 'saved', data: { subject: 'Mine' } }).subject, 'Mine', 'a saved sample stays');
assert.equal(ED.testing.localSample(editor('de'), mail, { data: { subject: 'October invoice' } }).subject, 'October invoice', 'no source: left alone');
const todo = ED.testing.localSample(editor('de'), { id: 'n_p', type: 'trigger.planner', params: { event: 'todo_overdue' } }, { source: 'builtin', data: { title: 'Pay the invoice' } });
assert.equal(todo.title, 'Rechnung bezahlen', 'the event variant picks its own keys');
const person = ED.testing.localSample(editor('de'), { id: 'n_c', type: 'trigger.person_detected', params: {} }, { source: 'builtin', data: { camera_name: 'Driveway', zones: ['Driveway'] } });
assert.deepEqual([person.camera_name, person.zones[0]], ['Einfahrt', 'Einfahrt'], 'paths reach into lists');
console.log('All EasyDrag sample checks passed.');

### Task B.4: error texts, list 2

Rewrites 43 of the 47 error keys of this list (the other 4 are checked and stay as they are) so that each text follows the §4.1 rules: outcome first (rule 1), a next step or "nothing to do" (rule 2), and only what the code guarantees (rule 7). en and de are written here; the other 14 locales are translated from the new en inside the key file. Two keys of this list carry a capital "Du" inside a sentence today (`error_flow_test_data_redacted`, `error_flow_version_note_conflict`); both are rewritten with lowercase "du" here and belong to the 29 keys of §4.2 (B.1 converts the others).

How the texts are shown (read in the code, not guessed): `core.errorText` (`easydrag-core.js:372-377`) turns an API code into `easydrag.ui.error_<code>`; `core.stepErrorText` (`easydrag-core.js:393-402`) shows a failed step or run as the translated text with its final full stop replaced by ": " and the server's own message. So the run-error texts (`runner_panic` to `wait_too_long` below) are written to read well before a colon, and their next step comes before it.

**Files:**
- Modify: `ui/lang/easydrag/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json` (16 files, 43 existing values each, in place; no key is added or removed)
- Uses: `plans/2026-11-15-easydrag-w1b/keys/b-errors-2.json` (the key file, 43 keys with 16 texts each; the coordinator commits it with the other key files)
- Test: `internal/flows/catalog_i18n_test.go` `TestCatalogTranslationsAreComplete`, `ui/desktop_easydrag_test.go` `TestEasyDragUIKeysExistInAllLocales`, `ui/desktop_easydrag_voice_test.go` `TestEasyDragGermanUsesLowercaseDu` and `TestEasyDragLocalesUseLedgerWords` (from B.1 and B.2)

**Depends on:** B.1 (guide and du lint) and B.2 (ledger lint) for the two lints in step 4. The key file does not depend on them.

**Meaning of every key.** "Origin" is where the code raises the error; "state" is what the code guarantees when the text is shown (rule 7). Two keys have no raise today (`error_flow_simulation_fixture_invalid`, `error_flow_simulation_unsupported`): the server maps those causes to `FLOW_BAD_REQUEST`, so their texts cannot be shown now and stay unchanged.

| Key | Origin | State | Old en | New en | New de |
|---|---|---|---|---|---|
| `error_flow_runner_panic` | `internal/flows/runner.go:1020` | run saved as an error; earlier steps may have run | The flow runner failed unexpectedly. The AuraGo log has the details. | The run stopped because of an internal error. Run it again; if it keeps happening, the AuraGo log has the details. | Der Lauf wurde wegen eines internen Fehlers abgebrochen. Starte ihn noch einmal; tritt der Fehler wieder auf, stehen Details im AuraGo-Log. |
| `error_flow_runner_stopped` | `internal/flows/runner.go:310,393,501` via `internal/server/flows_handlers.go:304` | request refused; no run created | EasyDrag is shutting down or restarting. Try again in a moment. | EasyDrag is shutting down or restarting and refused this request. Try again in a moment. | EasyDrag fährt herunter oder startet neu und hat diese Anfrage abgelehnt. Versuche es gleich noch einmal. |
| `error_flow_runs_paused` | `internal/flows/runner.go:743` | queued run cancelled, never executed | Automatic runs are paused (missions are read-only or flows are switched off), so this queued run did not start. | This queued run was cancelled without starting, because automatic runs are paused (missions are read-only or flows are switched off). Start it again once automatic runs are allowed. | Dieser wartende Lauf wurde abgebrochen, ohne zu starten, weil automatische Läufe pausiert sind (Missionen sind schreibgeschützt oder Flows ausgeschaltet). Starte ihn erneut, sobald automatische Läufe wieder erlaubt sind. |
| `error_flow_schema` | `internal/flows/validate.go:288` | validation finding; flow unchanged | The flow document has an unknown format. | This flow uses a format that this version of EasyDrag does not support. Import it again from an export of this version, or rebuild it. | Dieser Flow hat ein Format, das diese EasyDrag-Version nicht unterstützt. Importiere ihn erneut aus einem Export dieser Version oder baue ihn neu auf. |
| `error_flow_secret_unavailable` | `internal/flows/catalog_actions_http.go:315-326` | step failed before it sent its request | A secret is missing. | A secret this step needs is missing, empty or unusable, so the step cannot send its request. Set the secret again or choose another one. | Ein Geheimnis, das dieser Schritt braucht, fehlt, ist leer oder nicht verwendbar, deshalb kann der Schritt seine Anfrage nicht senden. Hinterlege das Geheimnis neu oder wähle ein anderes. |
| `error_flow_shutdown` | `internal/flows/runner.go:1238` | run never started | AuraGo is shutting down. | AuraGo shut down before this run started, so it did not run. Start it again. | AuraGo wurde beendet, bevor dieser Lauf startete, deshalb ist er nicht gelaufen. Starte ihn erneut. |
| `error_flow_start_failed` | `internal/flows/service_start_failures.go:12,91` | failed run recorded; no step ran | The flow could not start. | The trigger could not start this run, so no step ran. Check the flow and its trigger, then run it again. | Der Auslöser konnte diesen Lauf nicht starten, deshalb ist kein Schritt gelaufen. Prüfe den Flow und seinen Auslöser und starte ihn dann erneut. |
| `error_flow_stopped` | `internal/flows/engine_state.go:810` | a Stop step set to "error" ended the run on purpose | The flow was stopped. | A Stop step ended this run with an error, as the flow is set up to do. Fix the cause given here, then run it again. | Ein Stopp-Schritt hat diesen Lauf absichtlich mit einem Fehler beendet, so wie der Flow es vorsieht. Behebe die hier genannte Ursache und starte ihn erneut. |
| `error_flow_template_error` | `internal/flows/engine.go:305`, `durable_coordinator.go:71` | step failed while reading its settings; its action was not called | A data reference could not be resolved. | A data reference in this step could not be resolved, so the step failed. Check the reference and the output of the earlier steps. | Ein Datenbezug in diesem Schritt ließ sich nicht auflösen, deshalb ist der Schritt fehlgeschlagen. Prüfe den Bezug und die Ausgabe der vorherigen Schritte. |
| `error_flow_too_large` | `internal/server/flows_handlers.go:306,308`; `internal/flows/engine_state.go:155` | request refused, or run not started | The flow or its data is too large. | The flow or its data is over the size limit, so it was refused. Remove steps or data and try again. | Der Flow oder seine Daten überschreiten die Größengrenze und wurden abgelehnt. Entferne Schritte oder Daten und versuche es erneut. |
| `error_flow_too_many_issues` | `internal/flows/validate.go:181` | issue list cut; the first errors are kept | The flow has too many problems to list them all. | The flow has more problems than can be listed; only the first ones are shown. Fix those, then check again. | Der Flow hat mehr Probleme, als sich auflisten lassen; angezeigt werden nur die ersten. Behebe diese und prüfe dann erneut. |
| `error_flow_too_many_nodes` | `internal/flows/validate.go:295,300` | validation error; flow unchanged | The flow has too many steps. | The flow has more steps or connections than EasyDrag allows. Remove some, or split the flow in two. | Der Flow hat mehr Schritte oder Verbindungen, als EasyDrag erlaubt. Entferne einige oder teile den Flow auf. |
| `error_flow_tool_denied` | `internal/flows/catalog_tools.go:170,193,272` | call denied by permissions or quarantine; the tool may have run | The agent permissions block this action. | The agent permissions or a security check blocked this action. Check the agent permissions for it, or change the step. | Die Agenten-Rechte oder eine Sicherheitsprüfung haben diese Aktion blockiert. Prüfe die Agenten-Rechte dafür oder ändere den Schritt. |
| `error_flow_tool_error` | `internal/flows/catalog_tools.go:168,280` | unchanged: the tool's own message follows the text | The action reported an error. | unchanged | unchanged |
| `error_flow_tools_unavailable` | `internal/flows/catalog_tools.go:251`; `internal/server/flows_tool_invoker.go:181` | no tool call made | The tools are not available. | Tools are not available in this run, so the step did not call its action. Try again; if it keeps happening, check the tool settings. | Werkzeuge stehen in diesem Lauf nicht zur Verfügung, deshalb hat der Schritt seine Aktion nicht aufgerufen. Versuche es noch einmal; tritt es wieder auf, prüfe die Werkzeug-Einstellungen. |
| `error_flow_trigger_disabled` | `internal/flows/engine_state.go:282` | run ended before any step | This trigger is switched off. | The trigger for this run is switched off, so no step ran. Switch the trigger on in the flow, or start from another trigger. | Der Auslöser dieses Laufs ist ausgeschaltet, deshalb ist kein Schritt gelaufen. Schalte den Auslöser im Flow ein oder starte über einen anderen. |
| `error_flow_trigger_invalid` | `internal/flows/engine_state.go:279` | run ended before any step | The trigger is not set up correctly. | The trigger for this run is not part of the flow, so no step ran. Open the flow, check its trigger, then run it again. | Der Auslöser dieses Laufs gehört nicht zum Flow, deshalb ist kein Schritt gelaufen. Öffne den Flow, prüfe seinen Auslöser und starte ihn erneut. |
| `error_flow_value_type` | `internal/flows/nodes_logic.go:450`; `ui/js/desktop/apps/easydrag-block-fields.js:102,120` | step failed, or input check beside a field | A value has the wrong type. | A value does not have the type this field expects. Change the value or the field's type. | Ein Wert hat nicht den Typ, den dieses Feld erwartet. Ändere den Wert oder den Typ des Felds. |
| `error_flow_vault_unavailable` | `internal/server/flows_handlers.go:330,932`; `flows_credentials_move.go:35` | request refused; flows without secrets still work | The Vault is not available, so flow secrets cannot be used. | The Vault is not available, so secrets and saved scenarios cannot be used right now. Flows that need no secrets still work. Check the Vault, then try again. | Der Vault ist nicht verfügbar, deshalb lassen sich Geheimnisse und gespeicherte Szenarien gerade nicht nutzen. Flows ohne Geheimnisse laufen weiter. Prüfe den Vault und versuche es dann erneut. |
| `error_flow_wait_too_long` | `internal/flows/nodes_logic.go:345`; `durable_coordinator.go:80` | step failed | The waiting time is too long. | The wait is longer than this step or run allows, so the step failed. Shorten the wait. | Die Wartezeit ist länger, als dieser Schritt oder Lauf erlaubt, deshalb ist der Schritt fehlgeschlagen. Verkürze die Wartezeit. |
| `error_flows_disabled` | `internal/server/flows_handlers.go:469` | request refused | EasyDrag is switched off. | EasyDrag is switched off, so this request was refused. Turn flows on in the settings and make sure missions are enabled. | EasyDrag ist ausgeschaltet, deshalb wurde diese Anfrage abgelehnt. Schalte Flows in den Einstellungen ein und prüfe, dass Missionen aktiviert sind. |
| `error_generic` | `ui/js/desktop/apps/easydrag-core.js:376` | unknown server code; result of the request unknown | Something went wrong. | AuraGo reported an error that EasyDrag does not recognise. Try again; if it keeps happening, the AuraGo log has the details. | AuraGo hat einen Fehler gemeldet, den EasyDrag nicht kennt. Versuche es noch einmal; tritt er wieder auf, stehen Details im AuraGo-Log. |
| `error_network` | `ui/js/desktop/apps/easydrag-core.js:375` | no answer; result of the request unknown | AuraGo is not reachable right now. | AuraGo did not answer. Check the connection and try again; if you changed something, check that it was saved. | AuraGo hat nicht geantwortet. Prüfe die Verbindung und versuche es noch einmal; hast du etwas geändert, prüfe, ob es gespeichert wurde. |
| `error_node_type_unknown` | `internal/flows/engine.go:281` | step failed at run time (tool off or type gone) | This building block is not available. | This step type is not available, so the step cannot run. Replace the step, or switch its tool back on. | Dieser Schritttyp ist nicht verfügbar, deshalb kann der Schritt nicht laufen. Ersetze den Schritt oder schalte sein Werkzeug wieder ein. |
| `load_failed` | `ui/js/desktop/apps/easydrag.js:52` | unchanged: card title; the cause and Retry follow | EasyDrag could not be loaded | unchanged | unchanged |
| `error_flow_simulation_fixture_invalid` | no raise today (`flows_handlers.go:333` sends `FLOW_BAD_REQUEST`) | unchanged: kept as is until a code raises it | A sample output does not match this step. Check its output and ports. | unchanged | unchanged |
| `error_flow_simulation_fixture_required` | `internal/flows/simulation.go:196`; `easydrag-testing.js:226` | simulation stopped at the step; no real effects | This step needs a sample output before it can be simulated. | The simulation stopped at this step because it has no sample output. Add one for it and simulate again. | Die Simulation ist an diesem Schritt stehen geblieben, weil er keine Beispielausgabe hat. Ergänze eine und simuliere erneut. |
| `error_flow_simulation_unsupported` | no raise today (`simulation.go:102` refuses by type) | unchanged: kept as is until a code raises it | This step cannot be simulated. Supply a supported sample or use a deliberate real test. | unchanged | unchanged |
| `error_flow_test_data_redacted` | `internal/server/flows_handlers.go:310`; `service_runs.go:69,462` | request refused; test not started, sample not saved | Replace redacted or omitted sample values before running a real test. | This test data contains redacted or cut-off values, so it was refused. Replace them with real values and try again. | Diese Testdaten enthalten ausgeblendete oder abgeschnittene Werte und wurden deshalb abgelehnt. Ersetze sie durch echte Werte und versuche es erneut. |
| `error_flow_data_key_invalid` | `internal/server/flows_handlers.go:328`; `flows_data_keys.go:76,81` | scenario operation refused | The private scenario data key is invalid. Restore the original key before using saved scenarios. | The scenario key is not valid, so private scenarios cannot be saved or opened. Restore the original key in the Vault. | Der Szenarioschlüssel ist ungültig, deshalb lassen sich private Szenarien nicht speichern oder öffnen. Stelle den ursprünglichen Schlüssel im Vault wieder her. |
| `error_flow_data_key_missing` | `internal/server/flows_handlers.go:326`; `flows_data_keys.go:64` | existing scenarios cannot be opened | The private scenario data key is missing from the Vault. Restore the original key before using saved scenarios. | The scenario key is missing from the Vault, so saved private scenarios cannot be opened. Restore the original key. | Der Szenarioschlüssel fehlt im Vault, deshalb lassen sich gespeicherte private Szenarien nicht öffnen. Stelle den ursprünglichen Schlüssel wieder her. |
| `error_flow_scenario_conflict` | `internal/server/flows_handlers.go:314`; `internal/flows/scenarios.go:332,640` | action not applied | This scenario changed in another window. Reload it before continuing. | This scenario was changed in another window, so your action was not applied. Reload the scenario, then try again. | Dieses Szenario wurde in einem anderen Fenster geändert, deshalb wurde deine Aktion nicht übernommen. Lade das Szenario neu und versuche es dann erneut. |
| `error_flow_scenario_invalid` | `internal/server/flows_handlers.go:320`; `scenarios.go:112` | scenario refused (save or simulate) | Check the scenario name, inputs, fixtures and expected results. | The scenario was not accepted. Check its name, inputs, sample outputs and expected results. | Das Szenario wurde nicht akzeptiert. Prüfe Name, Eingaben, Beispielausgaben und erwartete Ergebnisse. |
| `error_flow_scenario_limit` | `internal/server/flows_handlers.go:318`; `scenarios.go:467` | new scenario not saved | The private scenario storage limit was reached. Delete an unused scenario before saving another. | The limit for private scenarios is reached, so this one was not saved. Delete a scenario you no longer need, then save again. | Die Grenze für private Szenarien ist erreicht, deshalb wurde dieses nicht gespeichert. Lösche ein Szenario, das du nicht mehr brauchst, und speichere dann erneut. |
| `error_flow_scenario_not_found` | `internal/server/flows_handlers.go:312`; `scenarios.go:478` | action not applied | This private scenario no longer exists. Reload the scenario list. | This private scenario no longer exists, so your action was not applied. Reload the list of scenarios. | Dieses private Szenario gibt es nicht mehr, deshalb wurde deine Aktion nicht übernommen. Lade die Liste der Szenarien neu. |
| `error_flow_scenario_sensitive` | `internal/server/flows_handlers.go:322`; `scenarios.go:353` | scenario not saved | The scenario contains protected or credential data. Replace those values with non-sensitive samples before saving. | The scenario contains protected data or credentials, so it was not saved. Replace those values with harmless sample values. | Das Szenario enthält geschützte Daten oder Zugangsdaten und wurde deshalb nicht gespeichert. Ersetze diese Werte durch unkritische Beispielwerte. |
| `error_flow_scenario_stale` | `internal/server/flows_handlers.go:316`; `scenarios.go:643` | simulation not run | The flow configuration changed. Create a fresh scenario before simulating. | The flow has changed since this scenario was made, so it was not simulated. Start a fresh scenario with the current steps. | Der Flow hat sich geändert, seit dieses Szenario erstellt wurde, deshalb wurde nicht simuliert. Beginne ein neues Szenario mit den aktuellen Schritten. |
| `error_flow_scenario_unreadable` | `internal/server/flows_handlers.go:324`; `scenarios.go:145` | scenario cannot be opened | The private scenario cannot be decrypted. Check this server’s scenario data key. | The private scenario cannot be decrypted, so it cannot be opened. Check that this server still has the original scenario key. | Das private Szenario lässt sich nicht entschlüsseln und kann deshalb nicht geöffnet werden. Prüfe, ob dieser Server noch den ursprünglichen Szenarioschlüssel hat. |
| `error_flow_block_in_use` | `internal/server/flows_blocks_handlers.go:20`; `internal/flows/blocks_service.go:320` | block not deleted | This block is still used by a published parent version or a retained run. Inspect its dependencies before deleting it. | This block is still used by a published flow or a kept run, so it was not deleted. Check its dependencies, remove those uses, then delete it. | Dieser Baustein wird noch von einem veröffentlichten Flow oder einem gespeicherten Lauf verwendet und wurde deshalb nicht gelöscht. Prüfe seine Abhängigkeiten, entferne diese Verwendungen und lösche ihn dann. |
| `error_flow_block_unavailable` | `internal/server/flows_handlers.go:266`; `internal/flows/blocks_plan.go:130` | request refused | The selected published block version is missing or failed verification. Choose an available verified version. | The published block version this step uses is missing or no longer passes its checks, so this action was refused. Choose another published version of the block. | Die veröffentlichte Bausteinversion, die dieser Schritt nutzt, fehlt oder besteht ihre Prüfung nicht mehr, deshalb wurde diese Aktion abgelehnt. Wähle eine andere veröffentlichte Version des Bausteins. |
| `error_flow_folder_not_empty` | `internal/server/flows_handlers.go:274`; `internal/flows/library.go:285` | folder not deleted | This folder still contains flows. Move them before deleting it. | This folder still contains flows, so it was not deleted. Move the flows to another folder or take them out of this folder, then delete it. | Dieser Ordner enthält noch Flows und wurde deshalb nicht gelöscht. Verschiebe die Flows in einen anderen Ordner oder entferne ihre Zuordnung und lösche den Ordner dann. |
| `error_flow_library_conflict` | `internal/server/flows_handlers.go:272`; `library.go:282` | edit not saved; input kept in the form | This metadata changed elsewhere. Your input is kept. Reloading replaces it with the latest stored values. | This information was changed elsewhere, so your edit was not saved. Your input is still here; reloading replaces it with the saved values. | Diese Angaben wurden zwischenzeitlich geändert, deshalb wurde deine Änderung nicht gespeichert. Deine Eingabe bleibt erhalten; Neuladen ersetzt sie durch die gespeicherten Werte. |
| `error_flow_library_invalid` | `internal/server/flows_handlers.go:270`; `flows_library.go:135-157` | input not saved | Check the folder, tags and text limits. | The folder, tags or text were not accepted, so they were not saved. Check the folder, the tags and the length of the text. | Ordner, Tags oder Text wurden nicht akzeptiert und deshalb nicht gespeichert. Prüfe Ordner, Tags und die Länge des Textes. |
| `error_flow_library_sensitive` | `internal/server/flows_handlers.go:278`; `library.go:62` | text not saved | Replace secret values and redaction markers with fresh text before saving. | The text contains a secret or a redaction marker, so it was not saved. Enter new text without them. | Der Text enthält ein Geheimnis oder eine Markierung für verborgene Daten und wurde deshalb nicht gespeichert. Gib einen neuen, bereinigten Text ein. |
| `error_flow_version_note_conflict` | `internal/server/flows_handlers.go:276`; `version_notes.go:117,149` | edit not saved | This version note changed elsewhere. Reload it before saving your edit. | This version note was changed elsewhere, so your edit was not saved. Reload the note, then make your edit again. | Die Versionsnotiz wurde zwischenzeitlich geändert, deshalb wurde deine Änderung nicht gespeichert. Lade die Notiz neu und mache deine Änderung dann noch einmal. |
| `error_flow_batch_item_failed` | `internal/flows/blocks_each.go:201` | list step stopped, no output; earlier items ran | A list item failed. No partial result is available. | A list item failed, so processing stopped and returned no result. Items that ran before it are not undone. Fix the item, or set the step to collect item errors. | Ein Listeneintrag ist fehlgeschlagen, deshalb wurde die Verarbeitung abgebrochen und lieferte kein Ergebnis. Einträge, die davor liefen, werden nicht rückgängig gemacht. Behebe den Eintrag oder stelle den Schritt auf das Sammeln von Eintragsfehlern um. |
| `error_flow_extraction_conflict` | `internal/server/flows_extraction.go:50`; `extraction_service.go:48,125,176` | block not created | The draft or proposed block ID changed. Reload the draft and review the selection again. | The draft or the proposed block ID changed, so the block was not created. Reload the draft, review the selection and extract again. | Der Entwurf oder die vorgeschlagene Baustein-ID hat sich geändert, deshalb wurde der Baustein nicht erstellt. Lade den Entwurf neu, prüfe die Auswahl und extrahiere noch einmal. |

Notes on the choices behind the table:
- "Nothing was changed" is written only where the origin returns before any write or any call: the scenario, library, folder, block, version-note, test-data and extraction keys (their database work runs in a transaction that is rolled back on the error). `error_network` and `error_generic` make no such claim, because the client cannot know whether the request was carried out; `error_network` says to check that a change was saved.
- `error_flow_tool_denied` also covers a quarantined tool answer (`catalog_tools.go:193`), so the old "The agent permissions block this action" was too narrow, and the new text does not say that the tool did not run.
- `error_flow_batch_item_failed` states that earlier items are not undone because the stop policy runs items one at a time in order (`batch_policy_hint`); it names the real alternative, collecting item errors (`batch_collect`).
- "Step", not "building block": `error_node_type_unknown` is the palette sense (spec §4.4). "Block"/"Baustein" stays only in the reusable sense (`block_in_use`, `block_unavailable`, `extraction_conflict`).

- [ ] **Step 1: Check that the key file is well formed and that every key exists today**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && node -e "
const fs=require('fs');
const k=JSON.parse(fs.readFileSync('plans/2026-11-15-easydrag-w1b/keys/b-errors-2.json','utf8'));
const L='cs da de el en es fr hi it ja nl no pl pt sv zh'.split(' ');
const ph=s=>(s.match(/\\{[a-z_]+\\}/g)||[]).sort().join();
const loc=Object.fromEntries(L.map(l=>[l,JSON.parse(fs.readFileSync('ui/lang/easydrag/'+l+'.json','utf8'))]));
let bad=0;
for(const [key,v] of Object.entries(k)){
  for(const l of L){ if(typeof v[l]!=='string'||!v[l].trim()) {console.log('EMPTY',key,l);bad++;} else if(ph(v[l])!==ph(v.en)) {console.log('PLACEHOLDER',key,l);bad++;} if(!(key in loc[l])) {console.log('NEW KEY',key,l);bad++;} }
}
console.log(Object.keys(k).length+' keys, '+bad+' problems');"
```

Expected: `43 keys, 0 problems`. If it prints `NEW KEY`, the key was renamed on main since this plan was written: look the key up with `git grep -n "<key>" -- ui/lang/easydrag/en.json`, and ask the coordinator before applying (the helper would add it, and the catalog test would then need a user of it).

- [ ] **Step 2: Apply the key file**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-15-easydrag-w1b/keys/b-errors-2.json
```

Expected: the helper reports 43 keys replaced in each of the 16 files and none added. Check that nothing outside the 43 values moved:

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && git diff --numstat -- ui/lang/easydrag/ && git diff --stat -- ui/lang/easydrag/ | tail -1
```

Expected: 16 lines of `43\t43\tui/lang/easydrag/<locale>.json` and a last line `16 files changed, 688 insertions(+), 688 deletions(-)`. Re-running step 2 changes nothing (the helper is idempotent).

- [ ] **Step 3: Run the two locale tests**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestCatalogTranslations' -count=1
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragUIKeysExistInAllLocales' -count=1
```

Expected: each prints `ok  \taurago/internal/flows\t...` and `ok  \taurago/ui\t...`. A failure naming a placeholder means a locale text lost or changed a `{name}`: none of the 43 keys has a placeholder, so this would mean the step 2 diff touched another key; run the step 2 diff check again.

- [ ] **Step 4: Run the voice lints**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragGermanUsesLowercaseDu|TestEasyDragLocalesUseLedgerWords' -count=1 -v
```

Expected: `--- PASS: TestEasyDragGermanUsesLowercaseDu`, `--- PASS: TestEasyDragLocalesUseLedgerWords` and `ok`. If the ledger lint names one of this list's keys, the new en or de text contains a banned word (en: workflow, node, building block, revision, package; de: Workflow, Knoten, Node, Block, Blöcke, Blocks, Revision, Paket): replace the word in the key file with the ledger word (flow, step, block only in the reusable sense), re-run steps 2 to 4. If the lint is not in the worktree yet (B.1 and B.2 not rebased in), run it after they land and do not skip it.

- [ ] **Step 5: Commit**

Write the message file `C:/disposable/easydrag-w1b/msg-b4.txt` with the Write tool:

```
feat(easydrag): rewrite error texts, list 2 (voice rules 1, 2, 7)

Rewrites 43 of 47 error texts (run errors, scenario, library,
block and simulation errors, network, generic) in all 16 locales: the
outcome first, then the next step, and a claim of "nothing changed" or
"nothing was sent" only where the raising code guarantees it. Run-error
texts read before the ": " that stepErrorText adds. error_flow_tool_denied
now also covers a security-check quarantine; error_node_type_unknown says
"step", not "building block". load_failed, error_flow_tool_error and two
texts that no code raises today are unchanged.

Locale files change only through plans/2026-11-15-easydrag-w1b/keys/b-errors-2.json
(locale-apply.mjs, 43 keys, in place).

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && git add ui/lang/easydrag/cs.json ui/lang/easydrag/da.json ui/lang/easydrag/de.json ui/lang/easydrag/el.json ui/lang/easydrag/en.json ui/lang/easydrag/es.json ui/lang/easydrag/fr.json ui/lang/easydrag/hi.json ui/lang/easydrag/it.json ui/lang/easydrag/ja.json ui/lang/easydrag/nl.json ui/lang/easydrag/no.json ui/lang/easydrag/pl.json ui/lang/easydrag/pt.json ui/lang/easydrag/sv.json ui/lang/easydrag/zh.json plans/2026-11-15-easydrag-w1b/keys/b-errors-2.json && git commit -F C:/disposable/easydrag-w1b/msg-b4.txt
```

Expected: one commit with 17 files changed. Do not amend, rebase or push inside this task. If the commit message must be edited, write a new message file and make a follow-up commit.

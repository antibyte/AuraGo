### Task B.3: error texts, list 1

Rewrites 46 of the 47 `easydrag.ui.error_flow_*` texts of list 1 (`error_flow_action_rule_required` to `error_flow_run_timeout`, alphabetical) to rules 1, 2 and 7 of the voice guide: outcome first, a next step (or "nothing to do"), and only what the code guarantees. `error_flow_publish_incomplete` already follows the rules and stays as it is. No code changes, only locale texts. Lists 2 and 3 are other tasks with their own key files; the three key files touch disjoint keys.

**Files:**
- Modify: `ui/lang/easydrag/cs.json`, `da.json`, `de.json`, `el.json`, `en.json`, `es.json`, `fr.json`, `hi.json`, `it.json`, `ja.json`, `nl.json`, `no.json`, `pl.json`, `pt.json`, `sv.json`, `zh.json` (46 values each, overwritten in place)
- Uses: `plans/2026-11-15-easydrag-w1b/keys/b-errors-1.json` (the key file; the coordinator commits it with the plan, and it is copied from `C:/disposable/easydrag-w1b-plan/keys/b-errors-1.json` if it is not in the worktree yet)
- Test: `internal/flows/catalog_i18n_test.go` `TestCatalogTranslationsAreComplete`, `ui/desktop_easydrag_test.go` `TestEasyDragUIKeysExistInAllLocales`, and the lints `TestEasyDragGermanUsesLowercaseDu` and `TestEasyDragLocalesUseLedgerWords` of `ui/desktop_easydrag_voice_test.go` (tasks B.1 and B.2)

**Texts.** The key column drops the prefix `easydrag.ui.`. "Origin" is where the code raises the error (checked 2026-11-15; lines can drift by a few). "Guaranteed" is what the text may claim (rule 7). None of the keys has a placeholder. German uses lowercase "du"; the ledger words apply (de no "Block", "Knoten", "Workflow", "Revision", "Paket"; en no "node", "building block", "revision", "package"). `node_unavailable` loses "building block": the code raises it for a missing tool or service, not for a reusable block. The key file holds all 16 locales; the other 14 are translated from the new en in the register of each locale file (informal address in cs, da, el, es, hi, it, nl, no, pl, sv, zh, de; "tu" in fr; the "Tente" form in pt, as its other error texts). `mission_missing` names the menu item with the existing `repair_mission` label of each locale.

| Key | Origin | Guaranteed | Old en | New en | New de |
|---|---|---|---|---|---|
| `error_flow_action_rule_required` | `internal/server/flows_tool_invoker.go:237` | tool call refused, step did not run | A matching action rule is missing. Add a rule in Configuration → Danger Zone, then start again manually. No automatic retry. | This step needs an action rule from an administrator, so it did not run. Add a rule in Configuration → Danger Zone, then start the run again yourself; it is not retried automatically. | Dieser Schritt braucht eine Aktionsregel vom Administrator und wurde deshalb nicht ausgeführt. Ergänze eine Regel unter Konfiguration → Gefahrenzone und starte den Lauf danach selbst neu; er wird nicht automatisch wiederholt. |
| `error_flow_ai_output_cut` | `internal/server/flows_llm.go:338` | answer cut at token limit, step failed, no engine retry | The AI answer was cut off at its length limit. Ask for a shorter answer or fewer output fields. | The AI answer was cut off at its length limit, so the step stopped. Ask for a shorter answer or fewer output fields, then run again. | Die KI-Antwort wurde an der Längengrenze abgeschnitten, deshalb ist der Schritt gestoppt. Bitte um eine kürzere Antwort oder weniger Ausgabefelder und starte dann neu. |
| `error_flow_ai_output_invalid` | `internal/flows/catalog_ai.go:342` | answer unusable, step failed | The AI answer did not have the expected form. | The AI answered in a form the step could not use. Run it again; if it keeps happening, word the instruction more precisely. | Die KI hat in einer Form geantwortet, die der Schritt nicht verwenden konnte. Starte ihn noch einmal; passiert das öfter, formuliere die Anweisung genauer. |
| `error_flow_ai_unavailable` | `internal/flows/catalog_ai.go:279` | no model or service, step did not run | No AI model is available. | No AI model is available, so the step could not run. Check the model in Configuration, then run again. | Es ist kein KI-Modell verfügbar, deshalb konnte der Schritt nicht laufen. Prüfe das Modell in der Konfiguration und starte dann neu. |
| `error_flow_bad_request` | `internal/server/flows_ai_proposal.go:31` | request refused as invalid | The request was invalid. | AuraGo rejected the request because it was not valid. Reload the page and try again. | AuraGo hat die Anfrage abgelehnt, weil sie ungültig war. Lade die Seite neu und versuche es noch einmal. |
| `error_flow_budget_exceeded` | `internal/server/flows_llm.go:104` | daily budget used up, step did not run | The AI budget is used up. | The daily AI budget is used up, so the step could not run. Raise the budget in Configuration or try again tomorrow. | Das tägliche KI-Budget ist aufgebraucht, deshalb konnte der Schritt nicht laufen. Erhöhe das Budget in der Konfiguration oder versuche es morgen noch einmal. |
| `error_flow_cancelled` | `internal/flows/engine_state.go:825` | run stopped before it finished | The run was stopped. | The run was stopped and did not finish. Start it again if you still need it. | Der Lauf wurde gestoppt und nicht zu Ende geführt. Starte ihn neu, wenn du ihn noch brauchst. |
| `error_flow_condition_failed` | `internal/flows/nodes_logic.go:79` | condition could not be evaluated | The condition could not be checked. | The step could not check its condition. Check the values the condition uses, then run again. | Der Schritt konnte seine Bedingung nicht prüfen. Kontrolliere die Werte, die die Bedingung verwendet, und starte dann neu. |
| `error_flow_condition_invalid` | `internal/flows/nodes_logic.go:75` | condition incomplete, no decision | The condition is incomplete. | The condition is incomplete, so the step could not decide. Complete it in the step’s settings. | Die Bedingung ist unvollständig, deshalb konnte der Schritt nichts entscheiden. Vervollständige sie in den Einstellungen des Schritts. |
| `error_flow_cycle` | `internal/flows/engine_state.go:193` | refused before any step runs | The steps form a loop. | The steps form a loop, so the flow cannot run. Remove the connection that leads back to an earlier step. | Die Schritte bilden eine Schleife, deshalb kann der Flow nicht laufen. Entferne die Verbindung, die zu einem früheren Schritt zurückführt. |
| `error_flow_disabled` | `internal/server/flows_handlers.go:284` | Run now refused, no run started | The flow is paused. Switch it to Active first. | The flow is paused, so no run was started. Switch it to Active, then run it again. | Der Flow ist pausiert, deshalb wurde kein Lauf gestartet. Schalte ihn auf Aktiv und starte ihn dann erneut. |
| `error_flow_exists` | `internal/flows/store.go:492` | create refused, nothing created | A flow with this ID already exists. | A flow with this ID already exists, so no new flow was created. Choose another ID. | Ein Flow mit dieser ID existiert bereits, deshalb wurde kein neuer Flow angelegt. Wähle eine andere ID. |
| `error_flow_file_exists` | `internal/flows/catalog_actions_docs.go:461` | file kept, not overwritten | The file already exists. | The file already exists and was not overwritten. Choose another file name, or change what the step does when a file exists. | Die Datei existiert bereits und wurde nicht überschrieben. Wähle einen anderen Dateinamen oder ändere, was der Schritt bei einer vorhandenen Datei tut. |
| `error_flow_http_status` | `internal/flows/catalog_actions_http.go:590` | server answered with an error status | The web server answered with an error. | The web server answered with an error, so the step failed. Check the address and the server, then run again. | Der Webserver hat mit einem Fehler geantwortet, deshalb ist der Schritt fehlgeschlagen. Prüfe die Adresse und den Server und starte dann neu. |
| `error_flow_internal` | `internal/server/flows_handlers.go:344` | unexpected server fault, state unknown | An internal error occurred. | Something went wrong inside AuraGo. Try again; if it keeps happening, see the AuraGo log. | In AuraGo ist etwas schiefgelaufen. Versuche es noch einmal; passiert es weiter, schau ins AuraGo-Log. |
| `error_flow_invalid` | `internal/server/flows_handlers.go:240` | validation refused, issues listed | The flow has errors. | The flow has errors that must be fixed first. Fix the problems listed in the editor, then try again. | Der Flow hat Fehler, die zuerst behoben werden müssen. Behebe die im Editor aufgelisteten Probleme und versuche es dann noch einmal. |
| `error_flow_locked` | `internal/flows/service_mission.go:305` | delete stops before the flow is deleted | The flow’s mission is locked in Mission Control. Unlock it there first. | The flow was not deleted because its mission is locked in Mission Control. Unlock it there, then delete the flow again. | Der Flow wurde nicht gelöscht, weil seine Mission in Mission Control gesperrt ist. Entsperre sie dort und lösche den Flow dann noch einmal. |
| `error_flow_mission_ambiguous` | `internal/server/flows_handlers.go:288` | several flows hold one mission | Several flows are linked to the same Mission Control mission. Delete the extra flow. | Several flows are linked to the same Mission Control mission, so AuraGo cannot tell which one is meant. Delete the extra flow, then try again. | Mehrere Flows sind mit derselben Mission in Mission Control verknüpft, deshalb ist unklar, welcher gemeint ist. Lösche den überzähligen Flow und versuche es dann noch einmal. |
| `error_flow_mission_control_unavailable` | `internal/server/flows_handlers.go:293` | Mission Control cannot be reached | Mission Control is not available right now. | Mission Control is not available right now. Try again in a moment. | Mission Control ist gerade nicht verfügbar. Versuche es in einem Moment noch einmal. |
| `error_flow_mission_missing` | `internal/server/flows_handlers.go:295` | mission entry gone, repair possible | The flow’s Mission Control entry is missing. Use Repair Mission Control connection before publishing again. | The flow’s Mission Control entry is missing. Use “Repair Mission Control connection”, then try again. | Der Mission-Control-Eintrag des Flows fehlt. Nutze „Mission-Control-Verknüpfung reparieren“ und versuche es dann noch einmal. |
| `error_flow_name_required` | `internal/flows/issue.go:47` | validation issue, name empty | Give the flow a name. | The flow has no name yet. Give it one to continue. | Der Flow hat noch keinen Namen. Gib ihm einen, um fortzufahren. |
| `error_flow_no_trigger` | `internal/flows/service_runs.go:85` | no run started | The flow has no trigger. | The flow has no active trigger to start it from. Add a trigger or switch one on, then try again. | Der Flow hat keinen aktiven Auslöser, von dem aus er starten kann. Füge einen Auslöser hinzu oder schalte einen ein und versuche es dann noch einmal. |
| `error_flow_node_abandoned` | `internal/flows/engine_state.go:574` | step left running after run end, effect uncertain | The step did not stop after the run ended. | The run has ended, but this step did not stop in time and was left behind. If it changes something outside AuraGo, check that before you run again. | Der Lauf ist beendet, aber dieser Schritt hat nicht rechtzeitig aufgehört und wurde zurückgelassen. Ändert er etwas außerhalb von AuraGo, prüfe das, bevor du erneut startest. |
| `error_flow_node_failed` | `internal/flows/engine.go:273` | step failed, details in step record | The step failed. | The step failed. The step’s details show why; fix the cause, then run again. | Der Schritt ist fehlgeschlagen. Die Details des Schritts zeigen den Grund; behebe die Ursache und starte dann neu. |
| `error_flow_node_not_found` | `internal/flows/issue.go:60` | test step not in the flow | The step does not exist. | This step is not part of the flow (anymore). Select a step that exists and try again. | Dieser Schritt gehört nicht (mehr) zum Flow. Wähle einen vorhandenen Schritt und versuche es noch einmal. |
| `error_flow_node_panic` | `internal/flows/engine.go:420` | step crashed, message in step record | The step crashed. | The step crashed unexpectedly. Run it again; if it keeps happening, the step’s details show what happened. | Der Schritt ist unerwartet abgestürzt. Starte ihn noch einmal; passiert das weiter, zeigen die Details des Schritts, was geschehen ist. |
| `error_flow_node_timeout` | `internal/flows/engine.go:350` | step stopped at its time limit | The step took too long. | The step did not finish in time and was stopped. Run it again, or raise its time limit in the step’s settings. | Der Schritt wurde nicht rechtzeitig fertig und wurde gestoppt. Starte ihn noch einmal oder erhöhe sein Zeitlimit in den Einstellungen des Schritts. |
| `error_flow_node_unavailable` | `internal/flows/catalog_tools.go:274` | tool or service not set up, step did not run | The building block is not available. | The step could not run because the tool or service it needs is not available. Check that it is set up, then run again. | Der Schritt konnte nicht laufen, weil das Werkzeug oder der Dienst, den er braucht, nicht verfügbar ist. Prüfe, ob es eingerichtet ist, und starte dann neu. |
| `error_flow_not_found` | `internal/server/flows_handlers.go:246` | flow or block unknown | The flow does not exist (anymore). | This flow does not exist (anymore). Go back to the flow list. | Diesen Flow gibt es nicht (mehr). Gehe zurück zur Flow-Liste. |
| `error_flow_not_published` | `internal/server/flows_handlers.go:280` | Run now refused, nothing to run | The flow is not published yet. | The flow is not published yet, so there is nothing to run. Publish it first, or test the draft. | Der Flow ist noch nicht veröffentlicht, deshalb gibt es nichts auszuführen. Veröffentliche ihn zuerst oder teste den Entwurf. |
| `error_flow_notify_failed` | `internal/flows/catalog_actions_notify.go:56` | delivery failed or result unreadable | The notification could not be sent. | The notification could not be sent. Check the notification settings, then run again. | Die Benachrichtigung konnte nicht gesendet werden. Prüfe die Benachrichtigungseinstellungen und starte dann neu. |
| `error_flow_options_unavailable` | `internal/server/flows_handlers_catalog.go:62` | option list not loaded | The choices could not be loaded. | The list of choices could not be loaded. Check that the service behind it is reachable, then try again. | Die Auswahlliste konnte nicht geladen werden. Prüfe, ob der Dienst dahinter erreichbar ist, und versuche es dann noch einmal. |
| `error_flow_output_invalid` | `internal/flows/engine.go:363` | step result unusable, step failed | The step returned unexpected data. | The step returned data AuraGo cannot use, so the step failed. Check the step’s settings, then run again. | Der Schritt hat Daten geliefert, die AuraGo nicht verwenden kann, deshalb ist er fehlgeschlagen. Prüfe die Einstellungen des Schritts und starte dann neu. |
| `error_flow_output_too_large` | `internal/flows/engine.go:366` | step output over limit, step failed | The output is too large. | The step produced more data than AuraGo can handle, so it failed. Make the step return less, then run again. | Der Schritt hat mehr Daten erzeugt, als AuraGo verarbeiten kann, deshalb ist er fehlgeschlagen. Lass den Schritt weniger zurückgeben und starte dann neu. |
| `error_flow_param_invalid` | `internal/flows/catalog_notify_args.go:163` | setting value refused | A setting has an invalid value. | A setting of this step has an invalid value. Check the step’s settings, then run again. | Eine Einstellung dieses Schritts hat einen ungültigen Wert. Prüfe die Einstellungen des Schritts und starte dann neu. |
| `error_flow_permission_denied` | `internal/server/flows_handlers.go:478` | request refused before any change | You are not allowed to change flows. | This was not allowed, so nothing was changed. Flows may be set to read-only, or you need to sign in again. | Das war nicht erlaubt, deshalb wurde nichts geändert. Vielleicht sind Flows auf Nur-Lesen gestellt, oder du musst dich neu anmelden. |
| `error_flow_port_invalid` | `internal/flows/engine_state.go:593` | step named an unknown output | The step chose an output it does not have. | The step chose an output it does not have, which points to a fault in the step. Run it again; if it repeats, report it. | Der Schritt hat einen Ausgang gewählt, den er nicht hat; das deutet auf einen Fehler im Schritt hin. Starte ihn noch einmal; wiederholt es sich, melde es. |
| `error_flow_publish_incomplete` | `internal/server/flows_handlers.go:896` | published; Mission Control or timers not updated; publishing again heals | Published, but Mission Control or the timers could not be updated. Publish again to finish. | unchanged | unchanged |
| `error_flow_rate_limited` | `internal/server/flows_handlers.go:965` | refused before the write | Too many changes in a short time. Try again in a minute. | Too many secret changes in a short time, so this one was not saved. Wait a minute, then try again. | Zu viele Änderungen an Geheimnissen in kurzer Zeit, deshalb wurde diese nicht gespeichert. Warte eine Minute und versuche es dann noch einmal. |
| `error_flow_request_cancelled` | `internal/server/flows_handlers.go:232` | state after the cancel unknown | The request was cancelled. | The request was cancelled before AuraGo answered. Reload to see the current state, then try again if needed. | Die Anfrage wurde abgebrochen, bevor AuraGo geantwortet hat. Lade neu, um den aktuellen Stand zu sehen, und versuche es bei Bedarf noch einmal. |
| `error_flow_restarted` | `internal/flows/store_runs.go:540` | run marked cancelled at start-up | AuraGo restarted during the run. | AuraGo restarted while the run was active, so the run was ended early. Start it again if you still need it. | AuraGo wurde neu gestartet, während der Lauf aktiv war, deshalb wurde der Lauf vorzeitig beendet. Starte ihn neu, wenn du ihn noch brauchst. |
| `error_flow_revision_conflict` | `internal/server/flows_handlers.go:250` | save refused, draft changed elsewhere | The flow was changed elsewhere. | The flow was changed elsewhere, so your change was not saved. Reload the flow, then make your change again. | Der Flow wurde woanders geändert, deshalb wurde deine Änderung nicht gespeichert. Lade den Flow neu und nimm deine Änderung noch einmal vor. |
| `error_flow_run_finished` | `internal/server/flows_handlers_runs.go:356` | cancel refused, run already ended | The run has already finished. | The run has already finished, so there is nothing to stop. | Der Lauf ist bereits beendet, deshalb gibt es nichts zu stoppen. |
| `error_flow_run_limit` | `internal/server/flows_handlers.go:302` | queue full, not started | Too many runs at once. Try again shortly. | Too much is running at the same time, so this was not started. Try again in a moment. | Es läuft zu viel gleichzeitig, deshalb wurde dies nicht gestartet. Versuche es in einem Moment noch einmal. |
| `error_flow_run_not_found` | `internal/server/flows_handlers_runs.go:267` | run id unknown | The run does not exist (anymore). | This run does not exist (anymore). Choose another run in the list. | Diesen Lauf gibt es nicht (mehr). Wähle in der Liste einen anderen Lauf. |
| `error_flow_run_output_too_large` | `internal/flows/engine_state.go:598` | run failed, outputs over the total limit | The outputs of this run are too large in total. | The outputs of this run grew too large together, so the run was stopped. Make the steps return less data, then run again. | Die Ausgaben dieses Laufs sind zusammen zu groß geworden, deshalb wurde der Lauf gestoppt. Lass die Schritte weniger Daten zurückgeben und starte dann neu. |
| `error_flow_run_timeout` | `internal/flows/engine_state.go:827` | run stopped at its time limit | The run took longer than allowed. | The run took longer than its time limit, so it was stopped. Run it again, or raise the maximum run time in the flow settings. | Der Lauf hat sein Zeitlimit überschritten und wurde gestoppt. Starte ihn noch einmal oder erhöhe die maximale Laufzeit in den Flow-Einstellungen. |

- [ ] **Step 1: Create the worktree if part B has none yet, and confirm the key file is there**

```
cd C:/Users/Andi/Documents/repo/AuraGo && (git worktree list | grep easydrag-w1b-voice || git worktree add .worktrees/easydrag-w1b-voice -b feat/easydrag-w1b-voice main)
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && ls plans/2026-11-15-easydrag-w1b/keys/b-errors-1.json
```

Expected: the worktree is listed (or created) and `ls` prints the path. If the file is missing, copy it: `mkdir -p plans/2026-11-15-easydrag-w1b/keys && cp C:/disposable/easydrag-w1b-plan/keys/b-errors-1.json plans/2026-11-15-easydrag-w1b/keys/`.

- [ ] **Step 2: Apply the key file**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && node C:/disposable/easydrag-w1a/locale-apply.mjs plans/2026-11-15-easydrag-w1b/keys/b-errors-1.json
```

Expected: the helper ends without an error and changes only the 46 values in each of the 16 files (check with `git diff --stat ui/lang/easydrag`: 16 files). Running it again changes nothing (it is idempotent).

- [ ] **Step 3: Check that all 46 keys carry the new text in all 16 locales**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && node -e "const k=require('./plans/2026-11-15-easydrag-w1b/keys/b-errors-1.json');const L='cs da de el en es fr hi it ja nl no pl pt sv zh'.split(' ');let n=0;for(const l of L){const o=require('./ui/lang/easydrag/'+l+'.json');for(const [key,v] of Object.entries(k)){if(o[key]!==v[l])throw new Error(l+' '+key);n++}}console.log('applied',n,'values for',Object.keys(k).length,'keys')"
```

Expected: `applied 736 values for 46 keys`.

- [ ] **Step 4: Run the locale tests**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./internal/flows -run 'TestCatalogTranslations' -count=1
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestEasyDragUIKeysExistInAllLocales' -count=1
```

Expected: each prints `ok` followed by the package path. A placeholder mismatch would fail the first; none of these keys has a placeholder.

- [ ] **Step 5: Run the voice lints (they exist from tasks B.1 and B.2)**

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && GOTOOLCHAIN=go1.27.4 GOTMPDIR='D:\tmp\go-build' go test ./ui -run '(?i)(TestEasyDragGermanUsesLowercaseDu|TestEasyDragLocalesUseLedgerWords)' -count=1 -v
```

Expected: two `--- PASS` lines and `ok`. If the du lint fails on one of these keys, a capital "Du" slipped into the key file: fix the text in the key file, re-run step 2 and this step. If the ledger lint fails, replace the banned word in the key file the same way. Never edit a locale file by hand.

- [ ] **Step 6: Commit**

Write the message with the Write tool to `C:/disposable/easydrag-w1b/b3-errors-1.txt`:

```
feat(easydrag): rewrite error texts, list 1 (outcome first, a next step)

46 of the 47 error_flow_* texts from action_rule_required to
run_timeout now say what happened to the user, then what to do, and
claim only what the error code guarantees (voice rules 1, 2 and 7).
Each text was checked against the code that raises it.
publish_incomplete already followed the rules and is unchanged.
node_unavailable no longer says "building block". All 16 locales come
from key file b-errors-1.json.

GitNexus detect-changes unavailable in the worktree; judged by symbols and tests.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

```
cd C:/Users/Andi/Documents/repo/AuraGo/.worktrees/easydrag-w1b-voice && git add ui/lang/easydrag plans/2026-11-15-easydrag-w1b/keys/b-errors-1.json && git commit -F C:/disposable/easydrag-w1b/b3-errors-1.txt
```

Expected: one commit with the 16 locale files and the key file. Never amend, rebase or push inside this task.

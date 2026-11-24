## 1. Glossary (dominant term -> deviating keys)

- **Asset** (plural Assets, Asset-Details): dominant. Deviations: `desktop.infra_error_not_found` "Das Infra-Element", `desktop.infra_error_invalid_version` "das Element", `desktop.infra_new_node` "Infrastruktur-Asset". `desktop.infra_error_invalid_node_id` uses the technical "Asset-ID". `desktop.infra_assign_engine_host` uses the English "Engine-Host".
- **Karte / Topologie / Liste** (map views): consistent. Deviation: `desktop.infra_place_on_map` "Auf Karte platzieren" (missing article, see the "auf der Karte" wording elsewhere).
- **Ort** (generic place), **Standort / Raum / Rack / Gruppe** (place kinds): consistent. Only `desktop.infra_kind_site` / `desktop.infra_icon_site` use Standort, which is correct for the kind.
- **Fund** (discovery): consistent. "Kandidat" is never used.
- **Quelle** (source): consistent.
- **Verbindung** (connection): dominant. Deviation: `desktop.infra_connection_kind` "Verbindungsart" vs `desktop.infra_relation_kind` "Verbindungstyp" (same concept). "Link" is used only for URLs and link fields, which is fine.
- **importieren / Ignorieren / Ignoriert / Aufräumen / Bibliothek / Details / Entwurf / Betriebszustand**: consistent, apart from the deviations listed in the table.
- **Unplaced**: four variants. `desktop.infra_unplaced` "Nicht platziert", `desktop.infra_to_place` "Zu platzieren", `desktop.infra_pdf_unplaced` "Nicht auf der Karte", `desktop.infra_no_match_unplaced` "noch nicht auf der Karte". Dominant: "auf der Karte".
- **Inventar** vs **Bestand**: `desktop.infra_current_inventory` "Aktueller Bestand" and `desktop.infra_node_conflict_review` "im aktuellen Bestand" deviate from "Inventar" (used in `desktop.infra_inventory`, `desktop.infra_remove_node_confirm`).
- **Unsaved**: "Ungespeichert" (`desktop.infra_unsaved_changes`) vs "Nicht gespeichert" (`desktop.infra_source_draft`, `desktop.infra_unsaved_close`).
- **Power state**: "Betriebszustand" is consistent, except `desktop.infra_pdf_col_power` "Strom".
- **Restart / reboot**: `desktop.infra_action_restart` and `desktop.infra_action_reboot` are both "Neustart" (a noun, while the other action buttons are infinitives).
- **Escape**: strings say "Escape", while `desktop.infra_kbd_esc` says "Esc".

## 2. Form of address

Consistently du ("Dein Entwurf", "Wähle", "Lass", "Verwende", "Prüfe", "Ziehe"). I found no Sie forms.

## 3. Corrections

| key | current German | proposed German | reason |
|---|---|---|---|
| desktop.infra_unplaced | Nicht platziert | Nicht auf der Karte | glossary: "auf der Karte" |
| desktop.infra_place_on_map | Auf Karte platzieren | Auf der Karte platzieren | missing article |
| desktop.infra_connection_kind | Verbindungsart | Verbindungstyp | matches infra_relation_kind |
| desktop.infra_current_inventory | Aktueller Bestand | Aktuelles Inventar | glossary Inventar |
| desktop.infra_node_conflict_review | Dieses Asset wurde im aktuellen Bestand geändert. Vergleiche die Werte unten, bevor du den Entwurf erneut anwendest. | Dieses Asset wurde im aktuellen Inventar geändert. Vergleiche die Werte unten, bevor du den Entwurf erneut anwendest. | glossary Inventar |
| desktop.infra_error_not_found | Das Infra-Element wurde nicht gefunden. | Das Asset wurde nicht gefunden. Lade Infra neu und versuche es erneut. | glossary; gives next step |
| desktop.infra_error_invalid_version | Aktualisiere das Element und versuche es erneut. | Aktualisiere das Asset und versuche es erneut. | glossary |
| desktop.infra_error_invalid_node_id | Die Asset-ID ist ungültig. | Dieses Asset ist nicht mehr gültig. Lade Infra neu. | no technical code; next step |
| desktop.infra_error_invalid_edge_id | Die Verbindungs-ID ist ungültig. | Diese Verbindung ist nicht mehr gültig. Lade Infra neu. | no technical code; next step |
| desktop.infra_error_invalid_request | Die übermittelten Infra-Daten sind ungültig. | Die Daten sind ungültig und wurden nicht gespeichert. Prüfe die Eingaben. | plain language; "übermittelt" is stiff |
| desktop.infra_error_generic | Die Anfrage konnte nicht abgeschlossen werden. Dein Entwurf bleibt erhalten. | Die Anfrage konnte nicht abgeschlossen werden. Dein Entwurf bleibt erhalten; versuche es erneut. | next step |
| desktop.infra_error_infra_error | Infra konnte die Anfrage nicht ausführen. | Infra konnte die Anfrage nicht ausführen. Versuche es später erneut. | next step |
| desktop.infra_error_conflict | Dieses Asset wurde anderswo geändert. Dein Entwurf ist noch da; aktualisiere, um die neueste Version zu prüfen. | Dieses Asset wurde anderswo geändert. Dein Entwurf bleibt erhalten; aktualisiere, um die neueste Version zu prüfen. | colloquial "ist noch da"; matches "bleibt erhalten" elsewhere |
| desktop.infra_export_pdf_failed | Das PDF konnte nicht erstellt werden. | Das PDF konnte nicht erstellt werden. Versuche es erneut. | next step |
| desktop.infra_sync_stale | Infra konnte nicht aktualisieren. Du siehst den letzten bekannten Stand. | Infra konnte nicht aktualisiert werden. Du siehst den zuletzt bekannten Stand. | grammar: missing passive; stiff |
| desktop.infra_sync_syncing | Synchronisiere… | Wird synchronisiert… | imperative in a status label |
| desktop.infra_sync_recovered | Infra ist wieder aktuell. | Infra ist wieder aktuell | toast punctuation |
| desktop.infra_export_pdf_done | PDF erstellt. | PDF erstellt | toast punctuation |
| desktop.infra_attention | Benötigt Aufmerksamkeit | Prüfung nötig | stiff translation |
| desktop.infra_action_status_unknown | Unbekannter Ausgang | Ergebnis unbekannt | stiff; matches infra_action_outcome_unknown |
| desktop.infra_attention_draft_conflict | Entwurf kollidiert mit einer neueren Version | Der Entwurf steht im Konflikt mit einer neueren Version | machine-like "kollidiert" |
| desktop.infra_reapply_ready | Der Entwurf gilt jetzt für die aktuelle Version. Prüfe ihn und speichere, wenn du bereit bist. | Der Entwurf baut jetzt auf der aktuellen Version auf. Prüfe ihn und speichere, wenn du so weit bist. | "gilt für" reads machine-translated |
| desktop.infra_arrange_unsupported | Dieser Server kann Assets noch nicht an die automatische Anordnung zurückgeben. | Dieser Server kann Assets noch nicht automatisch anordnen. | unnatural phrasing |
| desktop.infra_reapply_draft | Entwurf behalten und erneut anwenden | Entwurf erneut anwenden | button 36 chars; desktop.infra_source_reviewed already quotes "Entwurf erneut anwenden" |
| desktop.infra_action_rearm | Gerätestatus geprüft · einen neuen Versuch erlauben | Neuen Versuch erlauben | button 50 chars |
| desktop.infra_review_pending_source | Ausstehenden Quellenstatus prüfen | Quellenstatus prüfen | button length |
| desktop.infra_link_existing | Mit vorhandenem Asset verknüpfen | Mit Asset verknüpfen | button 32 chars |
| desktop.infra_import_new | Als neues Asset importieren | Als neu importieren | button 27 chars |
| desktop.infra_cleanup_remove_duplicate | Duplikat entfernen und ignorieren | Entfernen und ignorieren | button 33 chars; label already exists |
| desktop.infra_cleanup_show_dismissed | {count} ausgeblendete Vorschläge anzeigen | Ausgeblendete ({count}) | button 41 chars |
| desktop.infra_action_restart | Neustart | Neu starten | infinitive like the other action buttons |
| desktop.infra_action_reboot | Neustart | Neu starten | infinitive; same as restart, see observations |
| desktop.infra_ungroup | Gruppierung aufheben | Gruppe auflösen | matches "aufgelöst" and desktop.infra_key_ungroup |
| desktop.infra_cleanup_keep | Behält {name} | {name} behalten | label form, not a verb phrase |
| desktop.infra_remove_from_map | Von der Karte nehmen | Von der Karte entfernen | colloquial |
| desktop.infra_removed_from_map | {count} Assets von der Karte genommen | {count} Assets von der Karte entfernt | colloquial |
| desktop.infra_key_move, desktop.infra_move_mode, desktop.infra_move_mode_place, desktop.infra_resize_mode_on | "… Escape bricht ab" | "… Esc bricht ab" | matches desktop.infra_kbd_esc |
| desktop.infra_key_tools | Werkzeug Auswählen, Verschieben oder Verbinden | Werkzeug zum Auswählen, Verschieben oder Verbinden | broken grammar |
| desktop.infra_source_draft | Nicht gespeicherte Quellenänderungen | Ungespeicherte Quellenänderungen | glossary "ungespeichert" |
| desktop.infra_unsaved_close | Infra schließen und nicht gespeicherte Änderungen verwerfen? | Infra schließen und ungespeicherte Änderungen verwerfen? | glossary "ungespeichert" |
| desktop.infra_pdf_col_power | Strom | Betrieb | "Strom" means electricity; en is "Power" |
| desktop.infra_pdf_col_to | Nach | Zu | matches desktop.infra_relation_to "Zu" |
| desktop.infra_assign_engine_host | Engine-Host zuweisen | Docker-Host zuweisen | English term |
| desktop.infra_invalid_link | Verwende eine absolute HTTP- oder HTTPS-URL ohne Benutzerangaben. | Verwende eine vollständige URL mit http:// oder https://, ohne Benutzerangaben. | "absolute" is technical jargon |

Note on the Entwurf button: the quoted label in `desktop.infra_source_reviewed` ("Entwurf erneut anwenden") does not match the current button `desktop.infra_reapply_draft` ("Entwurf behalten und erneut anwenden"). The proposed shortened button fixes this, so keep both in sync.

## 4. General observations

1. Plural gap: `desktop.infra_pdf_assets` ("{count} Assets") and `desktop.infra_pdf_guests` ("{count} Gäste") have no `_one` variant. Check the call sites so a count of 1 does not render "1 Assets" or "1 Gäste". The en file has the same gap ("1 guests").
2. Restart and reboot share the same German label ("Neustart" or, proposed, "Neu starten"). Consider a tooltip-level distinction, since en has Restart and Reboot.
3. `desktop.infra_guests_badge` uses "aktiv" for running, while `desktop.infra_power_running` uses "Läuft". Pick one, for example "laufend".
4. Keyboard hints mix forms: `desktop.infra_search_jump` says "springen", while `desktop.infra_key_search` says "anspringen". Pick one.
5. Toast punctuation is mixed. Single-clause toasts such as "Verbindung gespeichert", "Asset erstellt" and "Assets angeordnet" have no period, but "PDF erstellt.", "Infra ist wieder aktuell." and "Gespeichert." do. Adopt one rule (no period for one-clause toasts).
6. Typography is otherwise clean: "…" is used throughout (no "..."), German quotes „…“ are used, and umlauts and ß are correct. I found no missing-umlaut typos.
7. Anglicisms: "Asset", "Tags", "Host", "Link" and "Dashboard"-type terms are acceptable in German IT usage. "Engine-Host" is the only one with a clearly natural German alternative (see the table).
8. `desktop.infra_remote_removed_draft` says "an anderer Stelle", while `desktop.infra_error_conflict` says "anderswo". Harmonise on "anderswo".
9. `desktop.infra_pdf_*` and `desktop.infra_app_*` names agree with the rest of de.json ("Schnellverbindung", "Software-Store", "Netzwerkkameras"), so no change is needed there.
10. Button lengths above about 24 characters that I left alone, because they are menu items or hints rather than buttons: `desktop.infra_export_pdf` ("Karte als PDF exportieren…"), `desktop.infra_remove_app_link`, `desktop.infra_move_to_place` ("An einen Ort verschieben…").

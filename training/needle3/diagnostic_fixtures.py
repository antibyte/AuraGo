"""Synthetic development probes; these are not sealed evaluation or training data."""
from __future__ import annotations

from collections import defaultdict

from .common import ROOT, catalog, digest, read_jsonl

# Each pair is one scenario, not two independent observations. Tool names in the
# catalog probes are intentionally explicit; report them apart from natural cases.
# kind | required manuals | German request | English request
PAIRS = """
single|docker|Welche Docker-Container laufen gerade?|Which Docker containers are running right now?
single|proxmox|Zeig mir die virtuellen Maschinen auf dem Proxmox-Knoten pve1.|Show the virtual machines on Proxmox node pve1.
single|remote_execution|Führe uptime per SSH auf dem Inventarserver web01 aus.|Run uptime over SSH on inventory server web01.
single|remote_control_shell|Lies die Ausgabe der offenen Shell-Sitzung auf dem verbundenen AgoDesk-PC.|Read the output of the open shell session on the connected AgoDesk PC.
single|home_assistant|Schalte light.kitchen in Home Assistant aus.|Turn off light.kitchen in Home Assistant.
single|mqtt|Veröffentliche OFF auf dem MQTT-Topic home/kitchen/light.|Publish OFF to MQTT topic home/kitchen/light.
single|email|Lies die ungelesenen Nachrichten in meinem E-Mail-Postfach.|Read the unread messages in my email inbox.
single|discord|Poste „Backup fertig“ im Discord-Kanal #betrieb.|Post "Backup complete" in Discord channel #operations.
single|workspace_search|Suche über den Workspace-Index nach Dateien namens backup.|Use the workspace index to find files named backup.
single|paperless|Suche in Paperless nach meiner Stromrechnung.|Search Paperless for my electricity bill.
single|knowledge_graph|Zeige im Wissensgraphen die Nachbarn des Knotens server01.|Show the neighbors of node server01 in the knowledge graph.
single|remember|Merke dir dauerhaft, dass ich kurze Antworten auf Deutsch bevorzuge.|Remember that I prefer brief answers in German as a lasting preference.
single|office_workbook|Setze Zelle B2 in Documents/budget.xlsx im virtuellen Desktop auf 42.|Set cell B2 of Documents/budget.xlsx in the virtual desktop to 42.
single|office_document|Ersetze „Entwurf“ durch „Freigabe“ in Documents/bericht.docx im virtuellen Desktop.|Replace "Draft" with "Approved" in Documents/report.docx in the virtual desktop.
single|file_search|Suche mit der Dateisuche in server.log nach ERROR; der Workspace-Index ist noch nicht bereit.|Search server.log for ERROR with file search; the workspace index is not ready yet.
single|fritzbox_smarthome|Schalte die Fritz!DECT-Steckdose im Flur ein.|Turn on the Fritz!DECT outlet in the hallway.
confusable|docker|Starte den Docker-Container web neu, die Proxmox-VM soll weiterlaufen.|Restart Docker container web; leave the Proxmox VM running.
confusable|proxmox|Starte VM 104 in Proxmox neu, nicht den Docker-Container darin.|Restart VM 104 in Proxmox, not the Docker container inside it.
confusable|remote_execution|Zeige die Plattenbelegung per SSH auf dem NAS aus dem Inventar, nicht lokal.|Show disk usage over SSH on the NAS from the inventory, not locally.
confusable|mqtt|Lies die letzten Nachrichten vom MQTT-Topic home/temp, ohne Home Assistant abzufragen.|Read recent messages from MQTT topic home/temp without querying Home Assistant.
confusable|home_assistant|Lies sensor.temperature über Home Assistant, nicht direkt über MQTT.|Read sensor.temperature through Home Assistant, not directly through MQTT.
confusable|email|Schicke den Text per E-Mail an team@example.com; poste nichts in Discord.|Email the text to team@example.com; do not post anything to Discord.
confusable|onedrive|Suche den Bericht in OneDrive, nicht in Koofr.|Find the report in OneDrive, not in Koofr.
confusable|desktop_notes|Liste meine Desktop-Notizen auf, nicht die Aufgabenliste.|List my desktop notes, not the task list.
missing_parameters|email|Sende bitte eine E-Mail. Empfänger und Text liefere ich gleich nach.|Please send an email. I will provide the recipient and message next.
missing_parameters|docker|Starte einen Docker-Container neu. Den Namen muss ich noch heraussuchen.|Restart a Docker container. I still need to find its name.
missing_parameters|mqtt|Abonniere ein MQTT-Topic; den genauen Pfad sage ich dir gleich.|Subscribe to an MQTT topic; I will give you the exact path next.
missing_parameters|proxmox|Erstelle einen Snapshot einer Proxmox-VM; die ID liefere ich nach.|Create a snapshot of a Proxmox VM; I will provide its ID next.
manual_help|mqtt|Welche Parameter hat das MQTT-Publish-Werkzeug von AuraGo?|What parameters does AuraGo's MQTT publish tool take?
manual_help|office_workbook|Welche Operationen unterstützt AuraGos Workbook-Werkzeug?|Which operations does AuraGo's workbook tool support?
multi|docker,email|Liste die laufenden Docker-Container auf und sende die Liste per E-Mail.|List the running Docker containers and email the list.
multi|home_assistant,mqtt|Lies sensor.temperature in Home Assistant und veröffentliche den Wert auf MQTT-Topic room/temp.|Read sensor.temperature in Home Assistant and publish its value to MQTT topic room/temp.
multi|proxmox,discord|Lies den Zustand der Proxmox-VM 104 und poste ihn in Discord.|Read the state of Proxmox VM 104 and post it to Discord.
multi|paperless,onedrive|Lade Rechnung 12 aus Paperless herunter und lade sie in OneDrive hoch.|Download invoice 12 from Paperless and upload it to OneDrive.
multi|workspace_search,email,discord|Finde backup.md im Workspace-Index, liste die E-Mail-Konten und zeige die Discord-Kanäle.|Find backup.md in the workspace index, list email accounts, and show Discord channels.
multi|docker,proxmox,home_assistant|Liste Docker-Container, dann Proxmox-VMs und danach Home-Assistant-Entitäten auf.|List Docker containers, then Proxmox VMs, then Home Assistant entities.
typo|docker|welche dockr contaner laufen grad?|which dockr containrs are runing rn?
typo|email|ungelesne mails zeign bitte|show unread emials pls
typo|proxmox|proxmox vm 104 neustartn pls|proxmox vm 104 restrt pls
typo|mqtt|mqtt topic home/temp abonieren bitte|mqtt topic home/temp subscrbe pls
none||Was ist sieben plus acht?|What is seven plus eight?
none||Übersetze nur diesen Satz ins Englische: „Die Lampe ist aus.“|Translate only this sentence into German: "The light is off."
none||Formuliere „Gib mir den Bericht“ höflicher. Antworte direkt im Chat.|Make "Give me the report" more polite. Reply directly in chat.
none||Fasse nur diesen Text zusammen: „Der Server läuft. Das Backup ist abgeschlossen.“|Summarize only this text: "The server is running. The backup is complete."
none||Nenne drei mögliche Namen für eine fiktive Katze.|Suggest three names for a fictional cat.
none||Danke, das reicht fürs Erste.|Thanks, that is enough for now.
none||Sortiere diese Wörter alphabetisch: Birne, Apfel, Zitrone.|Sort these words alphabetically: pear, apple, lemon.
none||Schreibe einen Zweizeiler über den Herbst direkt hier im Chat.|Write a two-line poem about autumn here in the chat.
none||Korrigiere nur die Rechtschreibung: „Das Backp ist fertg.“|Only correct the spelling: "The backp is compleete."
none||Welche Zahl ist größer: 19 oder 23?|Which number is larger: 19 or 23?
""".strip()

# Context consists only of preceding user requests, matching runtime.py.
CONTEXT = [
    ("discord", "Nein, schick ihn über Discord.", "No, send it through Discord.",
     "Schicke den Statusbericht per E-Mail.", "Email the status report."),
    ("proxmox", "Nein, ich meine die VM 104 in Proxmox.", "No, I mean VM 104 in Proxmox.",
     "Starte den Docker-Container neu.", "Restart the Docker container."),
    ("mqtt", "Lies jetzt nur die letzten Nachrichten dieses Topics.", "Now only read the most recent messages for that topic.",
     "Abonniere das MQTT-Topic home/livingroom/temp.", "Subscribe to MQTT topic home/livingroom/temp."),
    ("home_assistant", "Schalte sie jetzt wieder ein.", "Turn it back on now.",
     "Schalte light.kitchen in Home Assistant aus.", "Turn off light.kitchen in Home Assistant."),
    ("email", "Ja, zeige mir die ungelesenen davon.", "Yes, show me the unread ones.",
     "Prüfe mein E-Mail-Postfach.", "Check my email inbox."),
    ("", "Stopp, sende doch nichts. Bestätige nur kurz.", "Stop, do not send anything. Just acknowledge.",
     "Schicke den Bericht per E-Mail.", "Email the report."),
    ("", "Vergiss den Neustart. Rechne nur 3 plus 4 aus.", "Forget the restart. Just calculate 3 plus 4.",
     "Starte den Docker-Container web neu.", "Restart Docker container web."),
    ("", "Bitte nichts abonnieren. Ich wollte nur Danke sagen.", "Please do not subscribe to anything. I just wanted to say thanks.",
     "Abonniere das MQTT-Topic room/temp.", "Subscribe to MQTT topic room/temp."),
]

# Four shared scenarios per language: Docker, email, MQTT, and arithmetic.
TRANSLATIONS = {
    "cs": ["Které kontejnery Docker právě běží?", "Zobraz nepřečtené e-maily v mé schránce.", "Publikuj OFF do tématu MQTT home/kitchen/light.", "Kolik je sedm plus osm?"],
    "da": ["Hvilke Docker-containere kører lige nu?", "Vis de ulæste e-mails i min indbakke.", "Publicér OFF til MQTT-emnet home/kitchen/light.", "Hvad er syv plus otte?"],
    "el": ["Ποια κοντέινερ Docker εκτελούνται τώρα;", "Δείξε τα μη αναγνωσμένα μηνύματα στο ηλεκτρονικό μου ταχυδρομείο.", "Δημοσίευσε OFF στο θέμα MQTT home/kitchen/light.", "Πόσο κάνει επτά συν οκτώ;"],
    "es": ["¿Qué contenedores de Docker están en ejecución?", "Muéstrame los correos no leídos de mi buzón.", "Publica OFF en el tema MQTT home/kitchen/light.", "¿Cuánto es siete más ocho?"],
    "fr": ["Quels conteneurs Docker sont en cours d’exécution ?", "Affiche les e-mails non lus de ma boîte de réception.", "Publie OFF sur le sujet MQTT home/kitchen/light.", "Combien font sept plus huit ?"],
    "hi": ["अभी कौन से Docker कंटेनर चल रहे हैं?", "मेरे इनबॉक्स में अपठित ईमेल दिखाओ।", "MQTT टॉपिक home/kitchen/light पर OFF प्रकाशित करो।", "सात और आठ का योग कितना है?"],
    "it": ["Quali container Docker sono attualmente in esecuzione?", "Mostrami le e-mail non lette nella mia casella.", "Pubblica OFF sul topic MQTT home/kitchen/light.", "Quanto fa sette più otto?"],
    "ja": ["現在実行中のDockerコンテナを教えて。", "受信箱の未読メールを表示して。", "MQTTトピックhome/kitchen/lightにOFFを送信して。", "7足す8はいくつ？"],
    "nl": ["Welke Docker-containers draaien er momenteel?", "Toon de ongelezen e-mails in mijn postvak.", "Publiceer OFF op het MQTT-topic home/kitchen/light.", "Hoeveel is zeven plus acht?"],
    "no": ["Hvilke Docker-containere kjører akkurat nå?", "Vis de uleste e-postene i innboksen min.", "Publiser OFF til MQTT-emnet home/kitchen/light.", "Hva er sju pluss åtte?"],
    "pl": ["Które kontenery Docker są teraz uruchomione?", "Pokaż nieprzeczytane wiadomości w mojej skrzynce e-mail.", "Opublikuj OFF w temacie MQTT home/kitchen/light.", "Ile wynosi siedem plus osiem?"],
    "pt": ["Quais contêineres Docker estão em execução agora?", "Mostra os e-mails não lidos da minha caixa de entrada.", "Publica OFF no tópico MQTT home/kitchen/light.", "Quanto é sete mais oito?"],
    "sv": ["Vilka Docker-containrar körs just nu?", "Visa de olästa mejlen i min inkorg.", "Publicera OFF på MQTT-ämnet home/kitchen/light.", "Vad är sju plus åtta?"],
    "zh": ["现在有哪些Docker容器正在运行？", "显示我收件箱中的未读邮件。", "向MQTT主题home/kitchen/light发布OFF。", "七加八等于多少？"],
}


def fixture_rows(cat=None):
    cat = cat or catalog()
    hashes = {m["id"]: m["sha256"] for m in cat["manuals"]}
    rows = []

    def add(group, lang, query, gold, kind, context=(), allowed=None):
        rows.append({"id": f"{group}-{lang}", "group_id": group, "language": lang,
                     "query": query, "context": list(context), "gold": gold,
                     "all_manual_ids": gold, "kind": kind, "cohort": "natural_challenge",
                     "challenge": kind != "single", "split": "development_diagnostic",
                     "available_manuals": allowed, "sources": {mid: hashes[mid] for mid in gold},
                     "provenance": "repository_grounded_synthetic_fixture"})

    translation_groups = {}
    for index, line in enumerate(PAIRS.splitlines()):
        kind, mids, de, en = line.split("|")
        gold = mids.split(",") if mids else []
        group = f"natural-{index:03d}"
        for lang, query in (("de", de), ("en", en)):
            add(group, lang, query, gold, kind)
        if index in (0, 5, 6, 40):
            translation_groups[index] = group
    for index, (mid, de, en, de_context, en_context) in enumerate(CONTEXT):
        gold = [mid] if mid else []
        for lang, query, context in (("de", de, de_context), ("en", en, en_context)):
            add(f"context-{index:03d}", lang, query, gold, "context" if gold else "cancelled", [context])
    for lang, queries in TRANSLATIONS.items():
        for query, pair_index, gold in zip(queries, (0, 6, 5, 40), (["docker"], ["email"], ["mqtt"], [])):
            add(translation_groups[pair_index], lang, query, gold, "single" if gold else "none")
    for lang, query in (("de", "Lies meine ungelesenen E-Mails."), ("en", "Read my unread email.")):
        add("unavailable-empty", lang, query, [], "unavailable", allowed=[])
        add("unavailable-other", lang, query, [], "unavailable", allowed=["docker", "proxmox", "mqtt"])
    return rows


def catalog_probes(cat=None):
    """Derive one DE/EN request per family from existing synthetic tool contracts.

    Default-action tools occur only in the old multi-call bank. Isolate the first
    explicitly delimited request and its first call; never carry the second label.
    These deliberately mechanical probes measure catalog coverage, not realism.
    """
    cat = cat or catalog()
    bindings = {t["name"]: t.get("manual_id") for t in cat["tools"]}
    options = defaultdict(lambda: defaultdict(dict))
    for row in read_jsonl(ROOT / "training" / "dataset_native_fc.jsonl"):
        if row["source"] not in {"operation_contract", "curated_multicall"}:
            continue
        calls = row["expectations"].get("calls", [])
        users = [m["content"] for m in row["messages"] if m["role"] == "user"]
        if not calls or len(users) != 1:
            continue
        mid = bindings.get(calls[0]["name"])
        if not mid:
            continue
        query = users[0]
        if row["source"] == "curated_multicall":
            first, second = ("Zuerst: ", " Danach: ") if row["language"] == "de" else ("First: ", " Then: ")
            if first not in query or second not in query:
                continue
            query = query.split(first, 1)[1].split(second, 1)[0]
            # The old bank prints a null operation for tools without a selector.
            query = query.replace('die Aktion „<nil>“', 'die angegebene Aktion').replace('the "<nil>" operation', 'the requested action')
            family = "default:" + calls[0]["name"]
        else:
            family = row["family"]
        options[mid][family].setdefault(row["language"], (query, row["id"]))
    probes = []
    for manual in cat["manuals"]:
        pairs = [(family, languages) for family, languages in options[manual["id"]].items() if {"de", "en"} <= languages.keys()]
        if not pairs:
            raise ValueError(f"no paired source request for {manual['id']}")
        family, languages = min(pairs, key=lambda item: (item[0].startswith("default:"), digest(item[0])))
        for lang in ("de", "en"):
            query, source_id = languages[lang]
            group = "catalog-" + manual["id"]
            probes.append({"id": group + "-" + lang, "group_id": group, "language": lang,
                           "query": query, "context": [], "gold": [manual["id"]],
                           "all_manual_ids": [manual["id"]], "kind": "catalog_probe",
                           "cohort": "catalog_probe", "split": "development_diagnostic",
                           "sources": {manual["id"]: manual["sha256"]}, "source_row_id": source_id,
                           "source_family": family, "provenance": "repository_synthetic_contract"})
    return probes

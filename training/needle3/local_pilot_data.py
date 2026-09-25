"""Small repository-grounded CPU experiment, separate from the accepted corpus.

All old natural DE/EN diagnostic variants are assigned to training. Validation
and holdout use new task goals, not new names/parameters for those same tasks.
This hand-authored development sample does not qualify the full manual catalog.
"""
from __future__ import annotations

from .diagnostic_fixtures import fixture_rows

# kind | manuals | German | English. Each line is one bilingual scenario group.
VALIDATION = """
single|docker|Zeige die letzten 30 Logzeilen des Docker-Containers api.|Show the last 30 log lines of Docker container api.
single|proxmox|Wie viel Platz ist auf dem Storage local-lvm in Proxmox frei?|How much free space does Proxmox storage local-lvm have?
single|remote_execution|Welche Inventargeräte tragen das Tag prod? Liste nur diese Geräte auf.|Which inventory devices have the prod tag? List only those devices.
single|home_assistant|Stelle climate.livingroom über Home Assistant auf 21 Grad.|Set climate.livingroom to 21 degrees through Home Assistant.
single|workspace_search|Ist der Workspace-Suchindex bereit und wie viele Dateien enthält er?|Is the workspace search index ready and how many files does it contain?
single|paperless|Welche Tags gibt es in meinem Paperless-Archiv?|Which tags exist in my Paperless archive?
single|onedrive|Wie viel von meinem OneDrive-Speicher ist noch frei?|How much of my OneDrive storage is still free?
single|office_workbook|Lies die vorhandenen Zellen des Blatts Umsatz aus Documents/verkauf.xlsx im virtuellen Desktop.|Read the existing cells of the Revenue sheet in Documents/sales.xlsx in the virtual desktop.
single|office_document|Erstelle im virtuellen Desktop ein Word-Dokument Documents/begruessung.docx mit dem Text Willkommen.|Create a Word document Documents/welcome.docx in the virtual desktop containing the text Welcome.
single|discord|Lies die letzten fünf Nachrichten im Discord-Kanal #betrieb.|Read the last five messages in Discord channel #operations.
none||Zähle nur die Wörter in diesem Satz: Der Docker Container ist ruhig.|Only count the words in this sentence: The Docker container is quiet.
none||Erkläre mir allgemein, warum Backups nützlich sind. Nutze dafür nur dein vorhandenes Wissen.|Explain in general why backups are useful. Use only your existing knowledge.
""".strip()

HOLDOUT = """
single|docker|Inspiziere den Docker-Container gateway und zeige seine Mounts.|Inspect Docker container gateway and show its mounts.
single|proxmox|Welche Snapshots existieren für VM 207 auf Proxmox pve2?|Which snapshots exist for VM 207 on Proxmox pve2?
single|remote_execution|Kopiere /tmp/diagnostic.txt per SFTP vom Inventarserver nas in die lokale Workspace-Datei diagnostic.txt.|Copy /tmp/diagnostic.txt over SFTP from inventory server nas to local workspace file diagnostic.txt.
single|home_assistant|Aktiviere scene.evening in Home Assistant.|Activate scene.evening in Home Assistant.
single|mqtt|Beende das MQTT-Abonnement für garden/moisture.|Unsubscribe from MQTT topic garden/moisture.
single|workspace_search|Stoße einen erneuten Scan für den Workspace-Suchindex an.|Trigger a rescan of the workspace search index.
single|office_workbook|Exportiere die Arbeitsmappe Documents/sales.xlsx im virtuellen Desktop als CSV.|Export workbook Documents/sales.xlsx in the virtual desktop as CSV.
single|office_document|Exportiere Documents/protokoll.docx im virtuellen Desktop als Markdown.|Export Documents/minutes.docx in the virtual desktop as Markdown.
single|paperless|Ändere den Titel von Dokument 81 in Paperless auf Wartungsbeleg.|Change the title of document 81 in Paperless to Maintenance receipt.
single|onedrive|Erzeuge einen nur lesbaren Freigabelink für reports/summary.pdf in OneDrive.|Create a read-only sharing link for reports/summary.pdf in OneDrive.
single|desktop_notes|Lege eine neue Desktop-Notiz mit dem Titel Garten und dem Inhalt Morgen gießen an.|Create a new desktop note titled Garden containing Water tomorrow.
single|knowledge_graph|Lege im Wissensgraphen die Beziehung hosted_on von app1 zu server2 an; beide Knoten existieren.|Add the hosted_on relationship from app1 to server2 in the knowledge graph; both nodes exist.
multi|docker,discord|Lies den CPU-Verbrauch des Docker-Containers worker und veröffentliche den Wert im Discord-Kanal #betrieb.|Read the CPU usage of Docker container worker and post the value in Discord channel #operations.
multi|proxmox,email|Lies das Proxmox-Task-Log zur UPID des Backups und sende eine Zusammenfassung per E-Mail an ops@example.com.|Read the Proxmox task log for the backup UPID and email a summary to ops@example.com.
multi|onedrive,desktop_notes|Lies die Metadaten von plans/roof.pdf in OneDrive und halte die Dateigröße in einer neuen Desktop-Notiz fest.|Read the metadata of plans/roof.pdf in OneDrive and record its size in a new desktop note.
multi|paperless,office_workbook|Liste die Korrespondenten aus Paperless auf und schreibe ihre Namen in Spalte A der Arbeitsmappe Documents/contacts.xlsx im virtuellen Desktop.|List correspondents from Paperless and write their names into column A of workbook Documents/contacts.xlsx in the virtual desktop.
none||Erkläre aus deinem Wissen den Unterschied zwischen einer virtuellen Maschine und einem Container. Keine Systeme abfragen.|Explain from your own knowledge the difference between a virtual machine and a container. Do not query any systems.
none||Schreibe mir hier eine kurze Definition von MQTT, ohne eine Verbindung zu einem Broker herzustellen.|Write a short definition of MQTT here without connecting to a broker.
none||Gib mir einen Reim auf Haus. Verwende keine Werkzeuge.|Give me a rhyme for house. Do not use tools.
none||Stimmt diese Logik: Alle A sind B, alle B sind C, also sind alle A auch C? Antworte hier.|Is this logic valid: All A are B, all B are C, therefore all A are C? Answer here.
none||Wandle die Binärzahl 1010 gedanklich in eine Dezimalzahl um.|Convert binary 1010 to decimal mentally.
none||Vergleiche die beiden hier angegebenen Zeichenfolgen abc und abd und nenne die abweichende Stelle.|Compare the two strings given here, abc and abd, and identify the differing position.
""".strip()

CONTEXT_HOLDOUT = [
    ("docker", "Zeige jetzt die Prozesse darin.", "Now show the processes inside it.",
     "Es geht um den Docker-Container collector.", "We are discussing Docker container collector."),
    ("proxmox", "Nein, pausiere stattdessen die virtuelle Maschine in Proxmox.", "No, suspend the virtual machine in Proxmox instead.",
     "Pausiere den Docker-Container in VM 208.", "Pause the Docker container inside VM 208."),
]


def pilot_rows(cat):
    """Training fixtures can be reused; this holdout must never become training."""
    hashes = {m["id"]: m["sha256"] for m in cat["manuals"]}
    rows = [{**r, "split": "train", "cohort": "local_cpu_pilot"} for r in fixture_rows(cat)
            if r["language"] in {"de", "en"} and r["kind"] != "unavailable"]

    def add(split, group, kind, mids, de, en, contexts=None):
        gold = mids.split(",") if mids else []
        for lang, query in (("de", de), ("en", en)):
            rows.append({"id": group + "-" + lang, "group_id": group, "split": split,
                         "language": lang, "query": query, "context": (contexts or {}).get(lang, []),
                         "kind": kind, "gold": gold, "all_manual_ids": gold,
                         "sources": {mid: hashes[mid] for mid in gold}, "cohort": "local_cpu_pilot",
                         "provenance": "repository_grounded_synthetic_fixture", "challenge": kind != "single"})

    for split, block in (("validation", VALIDATION), ("holdout", HOLDOUT)):
        for i, line in enumerate(block.splitlines()):
            kind, mids, de, en = line.split("|")
            add(split, f"pilot-{split}-{i:03d}", kind, mids, de, en)
    for i, (mid, de, en, prior_de, prior_en) in enumerate(CONTEXT_HOLDOUT):
        add("holdout", f"pilot-context-{i:03d}", "context", mid, de, en,
            {"de": [prior_de], "en": [prior_en]})
    return rows

"""Independent synthetic goals for the category experiment (DE/EN siblings)."""

# manuals | German | English. Empty manuals mean a direct conversational answer.
TRAIN = """
email|Sende eine E-Mail mit dem Betreff Termin an lea@example.com.|Email lea@example.com with the subject Appointment.
discord|Schreib im Discord-Kanal Werkstatt, dass ich später komme.|Tell the workshop Discord channel I will arrive later.
address_book|Suche Leas Telefonnummer im Adressbuch.|Find Lea's phone number in my address book.
send_notification|Zeige mir eine Benachrichtigung auf dem Desktop.|Show me a desktop notification.
manage_webhooks|Liste meine eingerichteten Webhooks auf.|List my configured webhooks.
agentmail_inboxes|Lege ein neues AgentMail-Postfach an.|Create an AgentMail inbox.
yepapi|Frag über YepAPI die verfügbaren Börsendaten ab.|Query the available stock market data through YepAPI.
huggingface|Suche auf Hugging Face nach einem kleinen Sprachmodell.|Search Hugging Face for a small language model.
composio_call|Welche Aktionen bietet meine verbundene Composio-App?|What actions does my connected Composio app offer?
virustotal_scan|Prüfe diese URL bei VirusTotal auf bekannte Bedrohungen.|Check this URL for known threats with VirusTotal.
manus|Gib den Rechercheauftrag an Manus weiter.|Delegate the research task to Manus.
filesystem|Liste alle Dateien im Ordner Berichte auf.|List every file in the Reports directory.
paperless|Suche die Stromrechnung in meinem Paperless-Archiv.|Find the electricity bill in my Paperless archive.
onedrive|Lade die Datei angebot.pdf aus OneDrive herunter.|Download quote.pdf from OneDrive.
archive|Packe den Ordner Fotos in ein ZIP-Archiv.|Put the Photos directory in a ZIP archive.
workspace_search|Durchsuche den Workspace nach Hinweisen zur Anmeldung.|Search the workspace for information about sign-in.
file_editor|Ersetze in settings.txt den Hostnamen.|Replace the hostname in settings.txt.
docker|Starte den angehaltenen Docker-Container redis.|Start the stopped redis Docker container.
proxmox|Fahre die Proxmox-VM 301 herunter.|Shut down Proxmox VM 301.
remote_execution|Führe uptime auf dem inventarisierten SSH-Server aus.|Run uptime on the inventoried SSH server.
office_workbook|Setze in meiner Desktop-Arbeitsmappe Zelle B2 auf 42.|Set cell B2 in my desktop workbook to 42.
office_document|Füge meinem Desktop-Word-Dokument eine Überschrift hinzu.|Add a heading to my desktop Word document.
sql_query|Lies die Anzahl der Bestellungen aus der verbundenen SQL-Datenbank.|Read the order count from the connected SQL database.
github|Liste die offenen Issues meines GitHub-Repositories.|List open issues in my GitHub repository.
three_d_printer|Wie weit ist mein 3D-Drucker mit dem aktuellen Druck?|How far along is the current job on my 3D printer?
desktop_notes|Erstelle eine Desktop-Notiz mit meiner Einkaufsliste.|Create a desktop note with my shopping list.
sip_phone|Rufe die Nummer über mein SIP-Telefon an.|Call the number through my SIP phone.
generate_image|Erzeuge ein Bild eines Fuchses im Schnee.|Generate an image of a fox in snow.
transcribe_audio|Transkribiere die angehängte Sprachaufnahme.|Transcribe the attached voice recording.
tts|Lies mir diesen Text laut vor.|Read this text aloud to me.
jellyfin|Suche in Jellyfin nach dem Film Dune.|Find the movie Dune in Jellyfin.
media_conversion|Wandle meine WAV-Datei in MP3 um.|Convert my WAV file to MP3.
generate_music|Komponiere ein ruhiges instrumentales Musikstück.|Compose a calm instrumental music track.
remember|Merke dir dauerhaft, dass ich vegetarisch esse.|Remember permanently that I am vegetarian.
recall_memory|Was weißt du noch über meine Ernährung?|What do you remember about my diet?
knowledge_graph|Speichere im Wissensgraphen, dass Lea an Projekt Vega arbeitet.|Store in the knowledge graph that Lea works on Project Vega.
manage_journal|Trage meinen heutigen Fortschritt ins Journal ein.|Record today's progress in my journal.
manage_notes|Speichere eine Agentennotiz zu meinem bevorzugten Arbeitsablauf.|Save an agent note about my preferred workflow.
secrets_vault|Zeige die Namen meiner im Vault hinterlegten Geheimnisse.|List the names of my secrets stored in the vault.
ddg_search|Suche im Web nach aktuellen Tests von Mini-PCs.|Search the web for current mini PC reviews.
browser_automation|Öffne die Webseite und klicke auf das Suchfeld.|Open the website and click its search field.
dns_lookup|Löse den DNS-Namen example.org auf.|Resolve the DNS name example.org.
network_ping|Prüfe per Ping, ob 192.0.2.10 erreichbar ist.|Ping 192.0.2.10 to check whether it is reachable.
web_scraper|Lies den Haupttext der angegebenen Webseite aus.|Extract the main text from the specified web page.
certificate_manager|Zeige mir die verwalteten TLS-Zertifikate.|Show my managed TLS certificates.
home_assistant|Schalte über Home Assistant das Küchenlicht ein.|Turn on the kitchen light through Home Assistant.
mqtt|Veröffentliche den Wert 18 auf dem MQTT-Topic garden/temp.|Publish the value 18 to MQTT topic garden/temp.
fritzbox_network|Zeige die WLAN-Geräte meiner FritzBox.|Show the Wi-Fi devices connected to my FritzBox.
adguard|Zeige die DNS-Statistik meines AdGuard-Servers.|Show the DNS statistics from my AdGuard server.
grafana|Liste die Dashboards in Grafana auf.|List the dashboards in Grafana.
uptime_kuma|Welche Dienste meldet Uptime Kuma als ausgefallen?|Which services does Uptime Kuma report as down?
execute_shell|Führe lokal den Befehl uname -a aus.|Run uname -a locally.
execute_python|Berechne die Werte mit einem kurzen Python-Skript.|Calculate the values with a short Python script.
manage_todos|Lege eine Aufgabe zum Wechseln des Wasserfilters an.|Add a task to replace the water filter.
manage_appointments|Trage den Zahnarzttermin in meinen Kalender ein.|Add the dentist appointment to my calendar.
cron_scheduler|Erstelle einen täglichen Zeitplan für den Statuscheck.|Create a daily schedule for the status check.
package_manager|Installiere das Paket jq auf diesem System.|Install the jq package on this system.
system_metrics|Wie hoch ist die aktuelle CPU-Auslastung des Hosts?|What is the host's current CPU usage?
docker,email|Prüfe den Status des Containers und schicke mir das Ergebnis per E-Mail.|Check the container status and email me the result.
home_assistant,manage_todos|Prüfe den Temperatursensor in Home Assistant und lege eine Aufgabe zur Batteriekontrolle an.|Check the Home Assistant temperature sensor and add a task to inspect its battery.
ddg_search,remember|Recherchiere die Öffnungszeiten im Web und merke sie dir für später.|Look up the opening hours online and remember them for later.
paperless,office_workbook|Suche meine Rechnung in Paperless und übertrage den Betrag in die Desktop-Tabelle.|Find my invoice in Paperless and enter the amount in the desktop spreadsheet.
generate_image,discord|Erzeuge ein Katzenbild und poste es in unserem Discord-Kanal.|Generate a cat image and post it to our Discord channel.
virustotal_scan,manage_notes|Prüfe den Link bei VirusTotal und speichere das Ergebnis als Agentennotiz.|Check the link with VirusTotal and save the result as an agent note.
filesystem,tts|Lies text.txt und gib den Inhalt als Sprachausgabe wieder.|Read text.txt and speak its contents aloud.
dns_lookup,manage_todos|Prüfe den DNS-Eintrag und lege eine Aufgabe für die Korrektur an.|Check the DNS record and add a task to fix it.
proxmox,mqtt,email|Lies den VM-Status, veröffentliche ihn per MQTT und maile ihn an mich.|Read the VM status, publish it through MQTT, and email it to me.
remember|Bitte vergiss die aktuelle Frage und merke dir stattdessen meine neue Lieblingsfarbe Blau.|Disregard the current question and remember that my new favorite color is blue.
home_assistant|Nicht im Browser suchen, sondern in Home Assistant den Sensor auslesen.|Read the sensor in Home Assistant instead of searching in the browser.
email|Zeige mir, wie ich mit deiner E-Mail-Integration Nachrichten versende.|Show me how to send messages using your email integration.
docker|Ich brauche Hilfe bei der Bedienung deines Docker-Tools.|I need help using your Docker tool.
|Guten Morgen, wie geht es dir?|Good morning, how are you?
|Was ist sieben mal acht?|What is seven times eight?
|Schreib mir einen Zweizeiler über den Herbst.|Write a two-line poem about autumn.
|Übersetze diesen Satz ins Englische: Der Himmel ist blau.|Translate this sentence into German: The sky is blue.
|Erkläre aus deinem Wissen den Unterschied zwischen RAM und Festplatte.|Explain the difference between RAM and a hard drive from your own knowledge.
|Ich möchte gerade nichts ausführen lassen.|I do not want anything executed right now.
|Formuliere diesen Satz höflicher: Gib mir das Buch.|Make this sentence more polite: Give me the book.
|Fasse den hier eingefügten Absatz zusammen: Ein Fuchs läuft durch den Wald.|Summarize this pasted paragraph: A fox runs through the woods.
|Was bedeutet das englische Wort container?|What does the German word Speicher mean?
|Erfinde einen Namen für mein neues WLAN.|Invent a name for my new Wi-Fi network.
|Keine Werkzeuge: Welche Farbe entsteht aus Blau und Gelb?|No tools: what color do blue and yellow make?
|Antworte nur mit ja.|Reply with yes only.
|Ich danke dir für deine Hilfe.|Thank you for your help.
|Erkläre mir allgemein, was ein Kalender ist.|Explain generally what a calendar is.
|Zähle die Buchstaben im Wort Docker ohne Programme.|Count the letters in Docker without running programs.
|Nenne drei mögliche Betreffzeilen für eine Einladung, aber sende nichts.|Suggest three subject lines for an invitation, but send nothing.
|Wie könnte eine gute Einkaufsliste aufgebaut sein? Speichere nichts.|How could a good shopping list be organized? Do not save anything.
|Bitte erkläre ohne Recherche, was eine Datenbank ist.|Please explain what a database is without researching it.
|Schreib hier einen kurzen Dialog zwischen zwei Robotern.|Write a short dialogue between two robots here.
|Ich habe meine Meinung geändert, tu bitte gar nichts.|I changed my mind; please do nothing.
""".strip()

VALIDATION = """
email|Suche in meinen E-Mails nach der Versandbestätigung von gestern.|Search my emails for yesterday's shipping confirmation.
discord|Welche Mitglieder sind auf meinem Discord-Server?|Which members are on my Discord server?
address_book|Aktualisiere die Telefonnummer von Mira im Adressbuch.|Update Mira's phone number in the address book.
yepapi|Nutze YepAPI, um Nachrichten zur Raumfahrt abzurufen.|Use YepAPI to retrieve spaceflight news.
huggingface|Zeige die Modellkarte des angegebenen Hugging-Face-Modells.|Show the model card for the specified Hugging Face model.
virustotal_scan|Lass den Hash bei VirusTotal nachschlagen.|Look up the hash in VirusTotal.
file_search|Finde im lokalen Verzeichnis die Dateien mit der Endung .log.|Find files ending in .log in the local directory.
paperless|Lies den Text des markierten Dokuments in Paperless.|Read the text of the selected document in Paperless.
webdav|Liste die Ordner auf meinem WebDAV-Speicher.|List the folders on my WebDAV storage.
docker|Welche Images liegen auf meinem Docker-Host?|Which images are stored on my Docker host?
office_workbook|Formatiere die erste Zeile der Desktop-Arbeitsmappe fett.|Make the first row of the desktop workbook bold.
github|Zeige die letzten Commits auf GitHub.|Show the latest commits on GitHub.
generate_video|Erzeuge einen kurzen Clip von Wolken über einem See.|Generate a short clip of clouds over a lake.
analyze_image|Beschreibe das angehängte Foto mithilfe der Bildanalyse.|Describe the attached photo using image analysis.
chromecast|Pausiere die Wiedergabe auf meinem Chromecast.|Pause playback on my Chromecast.
knowledge_graph|Welche Beziehungen hat der Knoten Werkstatt in meinem Wissensgraphen?|What relationships does the Workshop node have in my knowledge graph?
manage_journal|Suche den letzten Journaleintrag zur Renovierung.|Find the latest journal entry about the renovation.
recall_memory|Welche Vorlieben hast du über mich dauerhaft gespeichert?|Which preferences have you permanently stored about me?
whois_lookup|Wem gehört die Domain example.net laut Whois?|Who owns example.net according to Whois?
site_monitor|Richte eine Überwachung für Änderungen auf der Webseite ein.|Set up monitoring for changes on the web page.
api_request|Sende eine GET-Anfrage an die angegebene API.|Send a GET request to the specified API.
mqtt|Zeige die aktiven MQTT-Abonnements.|Show the active MQTT subscriptions.
fritzbox_smarthome|Lies die Temperatur des FritzBox-Thermostats.|Read the temperature of the FritzBox thermostat.
home_assistant|Welche Automationen gibt es in Home Assistant?|Which automations exist in Home Assistant?
process_management|Beende den lokalen Prozess mit PID 4321.|Terminate the local process with PID 4321.
manage_todos|Markiere meine Aufgabe Paket abholen als erledigt.|Mark my Collect parcel task as completed.
manage_updates|Prüfe, ob ein AuraGo-Update bereitsteht.|Check whether an AuraGo update is available.
docker,discord|Lies das Docker-Containerlog und stelle die Fehlermeldung in Discord.|Read the Docker container log and post the error message to Discord.
home_assistant,remember|Lies die Raumtemperatur und merke dir den Messwert dauerhaft.|Read the room temperature and remember the reading permanently.
file_reader_advanced,generate_image|Lies die Bildbeschreibung aus briefing.txt und erzeuge das Bild.|Read the image description from briefing.txt and generate the image.
yepapi,email|Rufe über YepAPI die Wetterdaten ab und sende sie per E-Mail.|Retrieve weather data through YepAPI and send it by email.
ddg_search,manage_notes|Suche aktuelle Meldungen und speichere die Zusammenfassung als Agentennotiz.|Find current news and save the summary as an agent note.
proxmox,cron_scheduler|Prüfe den Status der VM und richte einen täglichen Zeitplan für diese Prüfung ein.|Check the VM status and schedule that check daily.
office_document,onedrive|Exportiere das Desktop-Dokument und lade es in OneDrive hoch.|Export the desktop document and upload it to OneDrive.
dns_lookup,mqtt,send_notification|Löse den Hostnamen auf, publiziere die IP per MQTT und benachrichtige mich.|Resolve the hostname, publish the IP through MQTT, and notify me.
|Was ist die Hauptstadt von Italien? Antworte aus deinem Wissen.|What is the capital of Italy? Answer from your knowledge.
|Formuliere eine Entschuldigung für eine verspätete E-Mail. Nur den Text.|Write an apology for a late email. Just the text.
|Ich erwähne Docker nur als Wort. Reime etwas darauf.|I mention Docker only as a word. Make a rhyme about it.
|Erkläre allgemein, was ein Lichtschalter macht.|Explain generally what a light switch does.
|Bewerte diese Idee kurz: Mehr Pausen beim Lernen.|Briefly assess this idea: take more breaks while studying.
|Antworte ohne Tools: Ist 17 eine Primzahl?|Answer without tools: is 17 prime?
|Schreibe eine Definition von Musik, ohne Audio zu erzeugen.|Write a definition of music without generating audio.
|Hilf mir beim Formulieren einer Aufgabe, lege sie noch nicht an.|Help me phrase a task, but do not create it yet.
|Beschreibe eine imaginäre Landschaft in drei Sätzen.|Describe an imaginary landscape in three sentences.
|Lass den geplanten Versand sein und antworte nur okay.|Cancel the planned sending and reply okay only.
""".strip()

HOLDOUT = """
agentmail_messages|Zeige die ungelesenen Nachrichten meines AgentMail-Postfachs.|Show the unread messages in my AgentMail inbox.
send_agodesk_chat|Schick die Nachricht Bereit in meinen AgoDesk-Chat.|Send Ready to my AgoDesk chat.
telnyx|Versende über Telnyx eine SMS mit meiner Ankunftszeit.|Send an SMS through Telnyx with my arrival time.
yepapi|Hole über YepAPI die neuesten Meldungen zum Bahnverkehr.|Get the latest railway traffic news through YepAPI.
composio_call|Führe die angegebene Aktion meiner Composio-Verbindung aus.|Run the specified action of my Composio connection.
manus|Prüfe den Fortschritt meiner laufenden Manus-Aufgabe.|Check the progress of my running Manus task.
pdf_operations|Verbinde die beiden PDF-Dateien zu einem Dokument.|Merge the two PDF files into one document.
koofr|Suche in meinem Koofr-Speicher nach dem Mietvertrag.|Search my Koofr storage for the rental agreement.
yaml_editor|Setze in meiner YAML-Datei den Port auf 8123.|Set the port in my YAML file to 8123.
virtual_computers|Starte meinen virtuellen Computer für das Experiment.|Start my virtual computer for the experiment.
s3_storage|Liste die Objekte im S3-Bucket urlaub.|List the objects in S3 bucket vacation.
office_document|Füge dem Word-Dokument im Desktop eine Tabelle hinzu.|Insert a table into the Word document on my desktop.
video_download|Lade das Video unter diesem Link herunter.|Download the video at this link.
generate_music|Erzeuge einen zehn Sekunden langen elektronischen Jingle.|Generate a ten-second electronic jingle.
bluetooth|Welche Bluetooth-Geräte sind gerade gekoppelt?|Which Bluetooth devices are paired right now?
query_memory|Suche in deinem Langzeitgedächtnis nach meinem letzten Umzug.|Search your long-term memory for my last move.
manage_notes|Lösche deine veraltete Agentennotiz zum alten Drucker.|Delete your outdated agent note about the old printer.
knowledge_graph|Finde im Wissensgraphen den Pfad zwischen Lea und dem Server.|Find the path between Lea and the server in the knowledge graph.
port_scanner|Welche TCP-Ports sind auf dem angegebenen Host offen?|Which TCP ports are open on the specified host?
wikipedia_search|Suche bei Wikipedia nach Informationen über Gezeiten.|Search Wikipedia for information about tides.
form_automation|Fülle das Kontaktformular der Webseite mit den angegebenen Daten aus.|Fill in the website's contact form with the supplied data.
fritzbox_system|Lies die Betriebszeit meiner FritzBox aus.|Read the uptime of my FritzBox.
wake_on_lan|Wecke meinen NAS-Rechner per Wake-on-LAN.|Wake my NAS computer with Wake-on-LAN.
grafana|Rufe die Details des ausgewählten Grafana-Dashboards ab.|Retrieve details of the selected Grafana dashboard.
activate_agent_skill|Lade die Anleitung meines aktivierten Recherche-Skills.|Load the instructions for my enabled research skill.
manage_appointments|Verschiebe meinen Kalendereintrag eine Stunde nach hinten.|Move my calendar appointment one hour later.
process_analyzer|Untersuche den Speicherverbrauch des lokalen Prozesses.|Inspect the local process's memory usage.
proxmox,email,manage_todos|Prüfe den Sicherungsstatus in Proxmox, maile mir den Befund und lege eine Aufgabe zur Kontrolle an.|Check the backup status in Proxmox, email me the findings, and create a follow-up task.
paperless,tts|Suche meinen Brief in Paperless und lies ihn laut vor.|Find my letter in Paperless and read it aloud.
home_assistant,discord|Prüfe den Türsensor in Home Assistant und melde den Zustand in Discord.|Check the door sensor in Home Assistant and report its state in Discord.
virustotal_scan,filesystem|Hole den VirusTotal-Bericht und speichere ihn als lokale Datei.|Fetch the VirusTotal report and save it as a local file.
browser_automation,office_workbook|Lies die Werte aus der Webseite und trage sie in die Desktop-Arbeitsmappe ein.|Read the values from the web page and enter them into the desktop workbook.
remember,cron_scheduler|Merke dir meine bevorzugte Pausenzeit und richte dafür eine tägliche Erinnerung ein.|Remember my preferred break time and schedule a daily reminder for it.
generate_image,onedrive,email|Erzeuge ein Poster, lade es in OneDrive und maile den Link.|Generate a poster, upload it to OneDrive, and email the link.
dns_lookup,manage_notes|Prüfe den DNS-Namen und notiere das Ergebnis in deinen Agentennotizen.|Check the DNS name and record the result in your agent notes.
yepapi,mqtt,manage_todos|Hole die Wetterwerte über YepAPI, publiziere sie auf MQTT und lege eine Aufgabe zum Sensorvergleich an.|Get weather readings through YepAPI, publish them to MQTT, and add a task to compare sensors.
|Verbessere die Grammatik: Ich haben heute Zeit.|Improve the grammar: I has time today.
|Erkläre aus deinem Wissen die Redewendung den Faden verlieren.|Explain the idiom lose the thread from your own knowledge.
|Was ist die Hälfte von 90? Kein Skript nötig.|What is half of 90? No script needed.
|Schreib einen Betreff für eine Mail zur Geburtstagsfeier, ohne sie zu verschicken.|Write a subject for a birthday party email without sending it.
|Nenne kreative Namen für einen MQTT-Broker, ohne ihn zu bedienen.|Suggest creative names for an MQTT broker without operating it.
|Vergleiche die Wörter haus und maus hier im Chat.|Compare the words house and mouse here in chat.
|Bitte keine Recherche: Warum wirft ein Gegenstand einen Schatten?|No research please: why does an object cast a shadow?
|Dichte vier Zeilen über ein Raumschiff.|Write four lines of poetry about a spaceship.
|Erkläre in einem Satz, wozu man Tabellen benutzt.|Explain in one sentence what tables are used for.
|Schlage eine Ordnerstruktur vor, aber lege keine Dateien an.|Suggest a folder structure, but do not create any files.
|Das war nur ein Beispiel für eine Benachrichtigung. Zeige keine an.|That was only an example notification. Do not show one.
|Schreibe mir nur eine fiktive Wettervorhersage für einen Fantasy-Roman.|Just write a fictional weather forecast for a fantasy novel.
""".strip()

# split, manuals, current DE/EN, preceding DE/EN. Context never supplies hidden gold.
CONTEXT = [
    ("train", "docker", "Starte ihn wieder.", "Start it again.", "Mein Docker-Container ist gestoppt.", "My Docker container is stopped."),
    ("train", "email", "Schick es mir darüber.", "Send it to me that way.", "Nutze für den Versand E-Mail.", "Use email for delivery."),
    ("train", "home_assistant", "Schalte sie aus.", "Turn it off.", "Es geht um die Küchenlampe in Home Assistant.", "We are discussing the kitchen lamp in Home Assistant."),
    ("train", "manage_todos", "Markiere sie als erledigt.", "Mark it completed.", "Meine Aufgabe heißt Reifen prüfen.", "My task is called Check tires."),
    ("train", "", "Nein, erkläre es mir nur allgemein.", "No, just explain it generally.", "Suche nach aktuellen Nachrichten.", "Search for current news."),
    ("validation", "onedrive", "Zeige die Unterordner darin.", "Show its subfolders.", "Ich meine den Ordner Kunden in OneDrive.", "I mean the Customers folder in OneDrive."),
    ("validation", "mqtt", "Nein, veröffentliche den Wert stattdessen auf MQTT.", "No, publish the value to MQTT instead.", "Schreibe den Wert in meine Desktop-Tabelle.", "Write the value to my desktop spreadsheet."),
    ("validation", "email", "Nur die E-Mail, die Datei brauchst du nicht mehr.", "Just the email; you no longer need the file.", "Erstelle eine Datei und sende eine Nachricht.", "Create a file and send a message."),
    ("validation", "", "Doch nicht ausführen, ich wollte nur einen Formulierungsvorschlag.", "Do not execute it; I only wanted wording suggestions.", "Erstelle eine Aufgabe für morgen.", "Create a task for tomorrow."),
    ("holdout", "discord", "Lies dort die neueste Nachricht.", "Read the newest message there.", "Wir arbeiten im Discord-Kanal Entwicklung.", "We are working in the Development Discord channel."),
    ("holdout", "home_assistant", "Stelle ihn auf 19 Grad.", "Set it to 19 degrees.", "Der Thermostat ist in Home Assistant eingebunden.", "The thermostat is connected to Home Assistant."),
    ("holdout", "desktop_notes", "Speichere das dort, nicht in deinem Gedächtnis.", "Save it there, not in your memory.", "Ich möchte eine Desktop-Notiz anlegen.", "I want to create a desktop note."),
    ("holdout", "ddg_search", "Nein, suche jetzt aktuelle Informationen im Internet.", "No, search the internet for current information now.", "Erkläre das nur aus deinem vorhandenen Wissen.", "Explain this only from your existing knowledge."),
    ("holdout", "transcribe_audio,email", "Schreib sie ab und maile mir den Text.", "Transcribe it and email me the text.", "Ich habe eine neue Sprachaufnahme hochgeladen.", "I uploaded a new voice recording."),
    ("holdout", "", "Bitte abbrechen, keine Aktion mehr.", "Please cancel; no further action.", "Starte den Docker-Container.", "Start the Docker container."),
]

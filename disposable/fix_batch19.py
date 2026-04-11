#!/usr/bin/env python3
"""Batch 19: Config descriptions part 3 + short UI labels."""
import json
from pathlib import Path

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "Keeps the sandbox MCP server running between calls instead of restarting it each time.": {
        "cs": "Udržuje MCP server sandboxu spuštěný mezi voláními místo jeho restartování.",
        "da": "Holder sandbox-MCP-serveren kørende mellem kald i stedet for at genstarte den hver gang.",
        "de": "Hält den Sandbox-MCP-Server zwischen Aufrufen aktiv, anstatt ihn jedes Mal neu zu starten.",
        "el": "Διατηρεί τον διακομιστή MCP sandbox σε λειτουργία μεταξύ κλήσεων αντί να τον επανεκκινεί κάθε φορά.",
        "es": "Mantiene el servidor MCP del sandbox ejecutándose entre llamadas en lugar de reiniciarlo cada vez.",
        "fr": "Garde le serveur MCP du bac à sable en cours d'exécution entre les appels au lieu de le redémarrer à chaque fois.",
        "hi": "हर बार पुनरारंभ करने के बजाय कॉल के बीच सैंडबॉक्स MCP सर्वर को चलता रखता है।",
        "it": "Mantiene il server MCP sandbox in esecuzione tra le chiamate invece di riavviarlo ogni volta.",
        "ja": "毎回再起動する代わりに、呼び出し間でサンドボックスMCPサーバーを実行し続けます。",
        "nl": "Houdt de sandbox-MCP-server actief tussen aanroepen in plaats van deze telkens te herstarten.",
        "no": "Holder sandbox-MCP-serveren kjørende mellom kall i stedet for å starte den på nytt hver gang.",
        "pl": "Utrzymuje serwer MCP sandbox uruchomiony między wywołaniami zamiast restartować go za każdym razem.",
        "pt": "Mantém o servidor MCP do sandbox em execução entre chamadas em vez de reiniciá-lo a cada vez.",
        "sv": "Håller sandbox-MCP-servern igång mellan anrop istället för att starta om den varje gång.",
        "zh": "在调用之间保持沙盒 MCP 服务器运行，而不是每次都重新启动。"
    },
    "Learns from usage history which tools are used most and prioritizes them. Less-used tools are offered less frequently.": {
        "cs": "Učí se z historie používání, které nástroje jsou používány nejvíce, a upřednostňuje je. Méně používané nástroje jsou nabízeny méně často.",
        "da": "Lærer fra forbrugshistorik, hvilke værktøjer der bruges mest, og prioriterer dem. Mindre brugte værktøjer tilbydes sjældnere.",
        "de": "Lernt aus dem Nutzungsverlauf, welche Tools am meisten verwendet werden, und priorisiert diese. Weniger genutzte Tools werden seltener angeboten.",
        "el": "Μαθαίνει από το ιστορικό χρήσης ποια εργαλεία χρησιμοποιούνται περισσότερο και τα δίνει προτεραιότητα. Τα λιγότερο χρησιμοποιούμενα εργαλεία προσφέρονται λιγότερο συχνά.",
        "es": "Aprende del historial de uso qué herramientas se usan más y las prioriza. Las herramientas menos usadas se ofrecen con menos frecuencia.",
        "fr": "Apprend de l'historique d'utilisation quels outils sont les plus utilisés et les priorise. Les outils moins utilisés sont proposés moins fréquemment.",
        "hi": "उपयोग इतिहास से सीखता है कि कौन से टूल सबसे अधिक उपयोग होते हैं और उन्हें प्राथमिकता देता है। कम उपयोग किए गए टूल कम बार पेश किए जाते हैं।",
        "it": "Impara dalla cronologia d'uso quali strumenti sono più utilizzati e li priorizza. Gli strumenti meno usati vengono offerti meno frequentemente.",
        "ja": "使用履歴からどのツールが最も使用されているかを学習し、優先します。あまり使用されないツールは頻度を下げて提案されます。",
        "nl": "Leert van gebruiksgeschiedenis welke tools het meest worden gebruikt en stelt deze voorrang. Minder gebruikte tools worden minder vaak aangeboden.",
        "no": "Lærer fra bruksloggen hvilke verktøy som brukes mest og prioriterer dem. Mindre brukte verktøy tilbys sjeldnere.",
        "pl": "Uczy się z historii użycia, które narzędzia są używane najczęściej i nadaje im priorytet. Rzadziej używane narzędzia są oferowane rzadziej.",
        "pt": "Aprende com o histórico de uso quais ferramentas são mais usadas e as prioriza. Ferramentas menos usadas são oferecidas com menos frequência.",
        "sv": "Lär sig från användningshistorik vilka verktyg som används mest och prioriterar dem. Mindre använda verktyg erbjuds mer sällan.",
        "zh": "从使用历史中学习哪些工具使用最多并优先考虑它们。较少使用的工具提供频率较低。"
    },
    "List of tool names exposed via MCP. Leave empty to expose all currently enabled tools.": {
        "cs": "Seznam názvů nástrojů vystavených přes MCP. Ponechte prázdné pro vystavení všech aktuálně povolených nástrojů.",
        "da": "Liste over værktøjsnavne eksponeret via MCP. Efterlad tom for at eksponere alle aktuelt aktiverede værktøjer.",
        "de": "Liste der über MCP bereitgestellten Tool-Namen. Leer lassen, um alle aktuell aktivierten Tools bereitzustellen.",
        "el": "Λίστα ονομάτων εργαλείων που εκτίθενται μέσω MCP. Αφήστε κενό για να εκθέσετε όλα τα τρέχοντα ενεργοποιημένα εργαλεία.",
        "es": "Lista de nombres de herramientas expuestas vía MCP. Dejar vacío para exponer todas las herramientas activas.",
        "fr": "Liste des noms d'outils exposés via MCP. Laisser vide pour exposer tous les outils actuellement activés.",
        "hi": "MCP के माध्यम से साझा किए गए टूल नामों की सूची। सभी वर्तमान सक्षम टूल साझा करने के लिए खाली छोड़ें।",
        "it": "Elenco dei nomi degli strumenti esposti tramite MCP. Lasciare vuoto per esporre tutti gli strumenti attualmente abilitati.",
        "ja": "MCP経由で公開するツール名のリスト。現在有効なすべてのツールを公開するには空のままにします。",
        "nl": "Lijst met toolnamen beschikbaar via MCP. Laat leeg om alle momenteel ingeschakelde tools beschikbaar te maken.",
        "no": "Liste over verktøynavn eksponert via MCP. La stå tom for å eksponere alle aktiverte verktøy.",
        "pl": "Lista nazw narzędzi udostępnianych przez MCP. Pozostaw puste, aby udostępnić wszystkie aktualnie włączone narzędzia.",
        "pt": "Lista de nomes de ferramentas expostas via MCP. Deixe vazio para expor todas as ferramentas atualmente ativadas.",
        "sv": "Lista över verktygsnamn exponerade via MCP. Lämna tom för att exponera alla för närvarande aktiverade verktyg.",
        "zh": "通过 MCP 公开的工具名称列表。留空以公开所有当前启用的工具。"
    },
    "Maximum number of automatic retries for failed background tasks.": {
        "cs": "Maximální počet automatických opakování pro neúspěšné úlohy na pozadí.",
        "da": "Maksimalt antal automatiske genforsøg for mislykkede baggrundsopgaver.",
        "de": "Maximale Anzahl automatischer Wiederholungen für fehlgeschlagene Hintergrundaufgaben.",
        "el": "Μέγιστος αριθμός αυτόματων επαναληψών για αποτυχημένες εργασίες παρασκηνίου.",
        "es": "Número máximo de reintentos automáticos para tareas en segundo plano fallidas.",
        "fr": "Nombre maximum de tentatives automatiques pour les tâches en arrière-plan échouées.",
        "hi": "विफल बैकग्राउंड कार्यों के लिए स्वचालित पुनः प्रयासों की अधिकतम संख्या।",
        "it": "Numero massimo di tentativi automatici per le attività in background fallite.",
        "ja": "失敗したバックグラウンドタスクの自動リトライの最大回数。",
        "nl": "Maximum aantal automatische herpogingen voor mislukte achtergrondtaken.",
        "no": "Maksimalt antall automatiske forsøk for mislykkede bakgrunnsoppgaver.",
        "pl": "Maksymalna liczba automatycznych ponownych prób dla nieudanych zadań w tle.",
        "pt": "Número máximo de tentativas automáticas para tarefas em segundo plano falhadas.",
        "sv": "Maximalt antal automatiska återförsök för misslyckade bakgrundsuppgifter.",
        "zh": "失败后台任务的最大自动重试次数。"
    },
    "Maximum number of tool manuals embedded in the system prompt. Fewer = shorter prompt but less tool knowledge.": {
        "cs": "Maximální počet manuálů nástrojů vložených do systémového promptu. Méně = kratší prompt, ale méně znalostí o nástrojích.",
        "da": "Maksimalt antal værktøjsmanualer indlejret i systemprompten. Færre = kortere prompt men mindre værktøjskendskab.",
        "de": "Maximale Anzahl von Tool-Manuals, die in den System-Prompt eingebettet werden. Weniger = kürzerer Prompt, aber weniger Tool-Wissen.",
        "el": "Μέγιστος αριθμός εγχειριδίων εργαλείων ενσωματωμένων στην προτροπή συστήματος. Λιγότερα = συντομότερη προτροπή αλλά λιγότερη γνώση εργαλείων.",
        "es": "Número máximo de manuales de herramientas en el prompt del sistema. Menos = prompt más corto pero menos conocimiento de herramientas.",
        "fr": "Nombre maximum de manuels d'outils intégrés dans le prompt système. Moins = prompt plus court mais moins de connaissances d'outils.",
        "hi": "सिस्टम प्रॉम्प्ट में एम्बेड किए गए टूल मैनुअल की अधिकतम संख्या। कम = छोटा प्रॉम्प्ट लेकिन कम टूल ज्ञान।",
        "it": "Numero massimo di manuali degli strumenti incorporati nel prompt di sistema. Meno = prompt più breve ma meno conoscenza degli strumenti.",
        "ja": "システムプロンプトに埋め込むツールマニュアルの最大数。少ない＝短いプロンプトだがツール知識が減少。",
        "nl": "Maximum aantal tool-handleidingen ingebed in de systeemprompt. Minder = kortere prompt maar minder toolkennis.",
        "no": "Maksimalt antall verktøymanualer innebygd i systemprompten. Færre = kortere prompt, men mindre verktøykunnskap.",
        "pl": "Maksymalna liczba podręczników narzędzi osadzonych w prompcie systemowym. Mniej = krótszy prompt, ale mniejsza znajomość narzędzi.",
        "pt": "Número máximo de manuais de ferramentas incorporados no prompt do sistema. Menos = prompt mais curto, mas menos conhecimento de ferramentas.",
        "sv": "Maximalt antal verktygsmanualer inbäddade i systemprompten. Färre = kortare prompt men mindre verktygskunskap.",
        "zh": "嵌入系统提示中的工具手册最大数量。更少 = 更短的提示但更少的工具知识。"
    },
    "Maximum number of tools offered to the LLM at once. Reduces token consumption.": {
        "cs": "Maximální počet nástrojů nabízených LLM najednou. Snižuje spotřebu tokenů.",
        "da": "Maksimalt antal værktøjer tilbudt LLM ad gangen. Reducerer tokenforbrug.",
        "de": "Maximale Anzahl an Tools, die dem LLM gleichzeitig angeboten werden. Reduziert den Token-Verbrauch.",
        "el": "Μέγιστος αριθμός εργαλείων που προσφέρονται στο LLM ταυτόχρονα. Μειώνει την κατανάλωση tokens.",
        "es": "Número máximo de herramientas ofrecidas al LLM a la vez. Reduce el consumo de tokens.",
        "fr": "Nombre maximum d'outils proposés au LLM à la fois. Réduit la consommation de tokens.",
        "hi": "LLM को एक बार में दिए जाने वाले टूल की अधिकतम संख्या। टोकन खपत कम करता है।",
        "it": "Numero massimo di strumenti offerti all'LLM contemporaneamente. Riduce il consumo di token.",
        "ja": "LLMに一度に提供するツールの最大数。トークン消費を削減します。",
        "nl": "Maximum aantal tools tegelijk aangeboden aan de LLM. Vermindert tokenverbruik.",
        "no": "Maksimalt antall verktøy tilbudt LLM samtidig. Reduserer tokenforbruk.",
        "pl": "Maksymalna liczba narzędzi oferowanych jednocześnie do LLM. Zmniejsza zużycie tokenów.",
        "pt": "Número máximo de ferramentas oferecidas ao LLM de uma vez. Reduz o consumo de tokens.",
        "sv": "Maximalt antal verktyg som erbjuds LLM samtidigt. Minskar tokenförbrukningen.",
        "zh": "一次提供给 LLM 的工具最大数量。减少令牌消耗。"
    },
    "Maximum wait time for a background task to complete. Default: 600 seconds (10 minutes).": {
        "cs": "Maximální čekací doba na dokončení úlohy na pozadí. Výchozí: 600 sekund (10 minut).",
        "da": "Maksimal ventetid for at en baggrundsopgave afsluttes. Standard: 600 sekunder (10 minutter).",
        "de": "Maximale Wartezeit bis zum Abschluss einer Hintergrundaufgabe. Standard: 600 Sekunden (10 Minuten).",
        "el": "Μέγιστος χρόνος αναμονής για ολοκλήρωση εργασίας παρασκηνίου. Προεπιλογή: 600 δευτερόλεπτα (10 λεπτά).",
        "es": "Tiempo máximo de espera para que se complete una tarea en segundo plano. Predeterminado: 600 segundos (10 minutos).",
        "fr": "Temps d'attente maximum pour qu'une tâche en arrière-plan se termine. Par défaut : 600 secondes (10 minutes).",
        "hi": "बैकग्राउंड कार्य पूरा होने का अधिकतम प्रतीक्षा समय। डिफ़ॉल्ट: 600 सेकंड (10 मिनट)।",
        "it": "Tempo massimo di attesa per il completamento di un'attività in background. Predefinito: 600 secondi (10 minuti).",
        "ja": "バックグラウンドタスク完了の最大待機時間。デフォルト：600秒（10分）。",
        "nl": "Maximale wachttijd voor het voltooien van een achtergrondtaak. Standaard: 600 seconden (10 minuten).",
        "no": "Maksimal ventetid for at en bakgrunnsoppgave skal fullføres. Standard: 600 sekunder (10 minutter).",
        "pl": "Maksymalny czas oczekiwania na ukończenie zadania w tle. Domyślnie: 600 sekund (10 minut).",
        "pt": "Tempo máximo de espera para a conclusão de uma tarefa em segundo plano. Padrão: 600 segundos (10 minutos).",
        "sv": "Maximal väntetid för att en bakgrundsuppgift ska slutföras. Standard: 600 sekunder (10 minuter).",
        "zh": "后台任务完成的最大等待时间。默认值：600 秒（10 分钟）。"
    },
    "Maximum wait time for HTTP requests within background tasks.": {
        "cs": "Maximální čekací doba pro HTTP požadavky v rámci úloh na pozadí.",
        "da": "Maksimal ventetid for HTTP-anmodninger i baggrundsopgaver.",
        "de": "Maximale Wartezeit für HTTP-Anfragen innerhalb von Hintergrundaufgaben.",
        "el": "Μέγιστος χρόνος αναμονής για αιτήματα HTTP σε εργασίες παρασκηνίου.",
        "es": "Tiempo máximo de espera para solicitudes HTTP en tareas en segundo plano.",
        "fr": "Temps d'attente maximum pour les requêtes HTTP dans les tâches en arrière-plan.",
        "hi": "बैकग्राउंड कार्यों में HTTP अनुरोधों के लिए अधिकतम प्रतीक्षा समय।",
        "it": "Tempo massimo di attesa per le richieste HTTP nelle attività in background.",
        "ja": "バックグラウンドタスク内のHTTPリクエストの最大待機時間。",
        "nl": "Maximale wachttijd voor HTTP-verzoeken binnen achtergrondtaken.",
        "no": "Maksimal ventetid for HTTP-forespørsler i bakgrunnsoppgaver.",
        "pl": "Maksymalny czas oczekiwania na żądania HTTP w zadaniach w tle.",
        "pt": "Tempo máximo de espera para solicitações HTTP em tarefas em segundo plano.",
        "sv": "Maximal väntetid för HTTP-förfrågningar i bakgrundsuppgifter.",
        "zh": "后台任务中 HTTP 请求的最大等待时间。"
    },
    "Maximum requests per second from n8n. Set to 0 for unlimited.": {
        "cs": "Maximální počet požadavků za sekundu z n8n. Nastavte na 0 pro neomezeno.",
        "da": "Maksimalt antal anmodninger pr. sekund fra n8n. Sæt til 0 for ubegrænset.",
        "de": "Maximale Anzahl an Anfragen pro Sekunde von n8n. Auf 0 setzen für unbegrenzt.",
        "el": "Μέγιστος αριθμός αιτημάτων ανά δευτερόλεπτο από το n8n. Ορίστε στο 0 για απεριόριστα.",
        "es": "Máximo de solicitudes por segundo desde n8n. Establecer en 0 para ilimitado.",
        "fr": "Requêtes maximales par seconde depuis n8n. Mettre à 0 pour illimité.",
        "hi": "n8n से प्रति सेकंड अधिकतम अनुरोध। असीमित के लिए 0 पर सेट करें।",
        "it": "Richieste massime al secondo da n8n. Impostare a 0 per illimitate.",
        "ja": "n8nからの1秒あたり最大リクエスト数。無制限にするには0に設定。",
        "nl": "Maximum verzoeken per seconde van n8n. Stel in op 0 voor onbeperkt.",
        "no": "Maksimalt antall forespørsler per sekund fra n8n. Sett til 0 for ubegrenset.",
        "pl": "Maksymalna liczba żądań na sekundę z n8n. Ustaw na 0 dla bez limitu.",
        "pt": "Máximo de solicitações por segundo do n8n. Defina como 0 para ilimitado.",
        "sv": "Maximala förfrågningar per sekund från n8n. Ställ in på 0 för obegränsat.",
        "zh": "来自 n8n 的每秒最大请求数。设置为 0 表示无限制。"
    },
    "Optional model override for the Helper LLM. A smaller, faster and cheaper model is usually recommended here.": {
        "cs": "Volitelné přepsání modelu pro Helper LLM. Menší, rychlejší a levnější model je zde obvykle doporučován.",
        "da": "Valgfri model-tilsidesættelse for Helper LLM. En mindre, hurtigere og billigere model anbefales normalt her.",
        "de": "Optionale Modellüberschreibung für das Helper-LLM. Ein kleineres, schnelleres und günstigeres Modell wird hier meist empfohlen.",
        "el": "Προαιρετική παράκαμψη μοντέλου για το Helper LLM. Ένα μικρότερο, πιο γρήγορο και φθηνότερο μοντέλο συνιστάται συνήθως εδώ.",
        "es": "Sobrescritura opcional del modelo para el LLM Helper. Un modelo más pequeño, rápido y barato suele ser recomendado aquí.",
        "fr": "Remplacement optionnel du modèle pour le LLM Helper. Un modèle plus petit, plus rapide et moins cher est généralement recommandé ici.",
        "hi": "सहायक LLM के लिए वैकल्पिक मॉडल ओवरराइड। एक छोटा, तेज़ और सस्ता मॉडल आमतौर पर यहाँ अनुशंसित है।",
        "it": "Sovrascrittura opzionale del modello per l'LLM Helper. Un modello più piccolo, veloce ed economico è solitamente consigliato qui.",
        "ja": "Helper LLMのオプションモデルオーバーライド。より小さく、高速で安価なモデルが通常推奨されます。",
        "nl": "Optionele modeloverride voor de Helper LLM. Een kleiner, sneller en goedkoper model wordt hier meestal aanbevolen.",
        "no": "Valgfri modelloverstyring for Helper LLM. En mindre, raskere og billigere modell anbefales vanligvis her.",
        "pl": "Opcjonalne nadpisanie modelu dla Helper LLM. Mniejszy, szybszy i tańszy model jest zazwyczaj zalecany.",
        "pt": "Substituição opcional do modelo para o LLM Helper. Um modelo menor, mais rápido e mais barato é geralmente recomendado aqui.",
        "sv": "Valfri modellöverskridning för Helper LLM. En mindre, snabbare och billigare modell rekommenderas vanligtvis här.",
        "zh": "Helper LLM 的可选模型覆盖。通常建议在此使用更小、更快、更便宜的模型。"
    },
    "Password for MQTT authentication. Stored securely in the encrypted vault.": {
        "cs": "Heslo pro MQTT autentizaci. Bezpečně uloženo v šifrovaném trezoru.",
        "da": "Adgangskode til MQTT-godkendelse. Opbevares sikkert i den krypterede boks.",
        "de": "Passwort für MQTT-Authentifizierung. Sicher im verschlüsselten Tresor gespeichert.",
        "el": "Κωδικός πρόσβασης για αυθεντικοποίηση MQTT. Αποθηκεύεται με ασφάλεια στο κρυπτογραφημένο θησαυροφυλάκιο.",
        "es": "Contraseña para autenticación MQTT. Almacenada de forma segura en la bóveda cifrada.",
        "fr": "Mot de passe pour l'authentification MQTT. Stocké en toute sécurité dans le coffre chiffré.",
        "hi": "MQTT प्रमाणीकरण के लिए पासवर्ड। एन्क्रिप्टेड वॉल्ट में सुरक्षित रूप से संग्रहीत।",
        "it": "Password per l'autenticazione MQTT. Archiviata in modo sicuro nel caveau crittografato.",
        "ja": "MQTT認証のパスワード。暗号化されたボールトに安全に保存されます。",
        "nl": "Wachtwoord voor MQTT-authenticatie. Veilig opgeslagen in de versleutelde kluis.",
        "no": "Passord for MQTT-autentisering. Lagret sikkert i det krypterte hvelvet.",
        "pl": "Hasło do uwierzytelnienia MQTT. Bezpiecznie przechowywane w zaszyfrowanym sejfie.",
        "pt": "Senha para autenticação MQTT. Armazenada com segurança no cofre criptografado.",
        "sv": "Lösenord för MQTT-autentisering. Lagras säkert i det krypterade valvet.",
        "zh": "MQTT 认证密码。安全存储在加密保管库中。"
    },
    "Path to the CA certificate file for TLS verification (optional, for custom CAs).": {
        "cs": "Cesta k souboru CA certifikátu pro ověřování TLS (volitelné, pro vlastní CA).",
        "da": "Sti til CA-certifikatfil til TLS-bekræftelse (valgfrit, til brugerdefinerede CA'er).",
        "de": "Pfad zur CA-Zertifikatsdatei für TLS-Verifizierung (optional, für benutzerdefinierte CAs).",
        "el": "Διαδρομή στο αρχείο πιστοποιητικού CA για επαλήθευση TLS (προαιρετικό, για προσαρμοσμένες CA).",
        "es": "Ruta al archivo de certificado CA para verificación TLS (opcional, para CAs personalizados).",
        "fr": "Chemin vers le fichier de certificat CA pour la vérification TLS (optionnel, pour les CA personnalisés).",
        "hi": "TLS सत्यापन के लिए CA प्रमाणपत्र फ़ाइल का पथ (वैकल्पिक, कस्टम CA के लिए)।",
        "it": "Percorso al file del certificato CA per la verifica TLS (opzionale, per CA personalizzate).",
        "ja": "TLS検証用のCA証明書ファイルへのパス（オプション、カスタムCA用）。",
        "nl": "Pad naar het CA-certificaatbestand voor TLS-verificatie (optioneel, voor aangepaste CA's).",
        "no": "Sti til CA-sertifikatfilen for TLS-bekreftelse (valgfritt, for tilpassede CA-er).",
        "pl": "Ścieżka do pliku certyfikatu CA do weryfikacji TLS (opcjonalnie, dla niestandardowych CA).",
        "pt": "Caminho para o arquivo de certificado CA para verificação TLS (opcional, para CAs personalizados).",
        "sv": "Sökväg till CA-certifikatfilen för TLS-verifiering (valfritt, för anpassade CA:er).",
        "zh": "用于 TLS 验证的 CA 证书文件路径（可选，用于自定义 CA）。"
    },
    "Path to the client certificate file for mutual TLS authentication (optional).": {
        "cs": "Cesta k souboru klientského certifikátu pro vzájemné TLS ověřování (volitelné).",
        "da": "Sti til klientcertifikatfilen til gensidig TLS-godkendelse (valgfrit).",
        "de": "Pfad zur Client-Zertifikatsdatei für gegenseitige TLS-Authentifizierung (optional).",
        "el": "Διαδρομή στο αρχείο πιστοποιητικού πελάτη για αμοιβαία αυθεντικοποίηση TLS (προαιρετικό).",
        "es": "Ruta al archivo de certificado de cliente para autenticación TLS mutua (opcional).",
        "fr": "Chemin vers le fichier de certificat client pour l'authentification TLS mutuelle (optionnel).",
        "hi": "पारस्परिक TLS प्रमाणीकरण के लिए क्लाइंट प्रमाणपत्र फ़ाइल का पथ (वैकल्पिक)।",
        "it": "Percorso al file del certificato client per l'autenticazione TLS reciproca (opzionale).",
        "ja": "相互TLS認証用のクライアント証明書ファイルへのパス（オプション）。",
        "nl": "Pad naar het clientcertificaatbestand voor wederzijdse TLS-authenticatie (optioneel).",
        "no": "Sti til klientsertifikatfilen for gjensidig TLS-autentisering (valgfritt).",
        "pl": "Ścieżka do pliku certyfikatu klienta dla wzajemnego uwierzytelnienia TLS (opcjonalnie).",
        "pt": "Caminho para o arquivo de certificado do cliente para autenticação TLS mútua (opcional).",
        "sv": "Sökväg till klientcertifikatfilen för ömsesidig TLS-autentisering (valfritt).",
        "zh": "用于双向 TLS 认证的客户端证书文件路径（可选）。"
    },
    "Path to the client private key file for mutual TLS authentication (optional).": {
        "cs": "Cesta k souboru soukromého klíče klienta pro vzájemné TLS ověřování (volitelné).",
        "da": "Sti til klientens private nøglefil til gensidig TLS-godkendelse (valgfrit).",
        "de": "Pfad zur privaten Schlüsseldatei des Clients für gegenseitige TLS-Authentifizierung (optional).",
        "el": "Διαδρομή στο αρχείο ιδιωτικού κλειδιού πελάτη για αμοιβαία αυθεντικοποίηση TLS (προαιρετικό).",
        "es": "Ruta al archivo de clave privada del cliente para autenticación TLS mutua (opcional).",
        "fr": "Chemin vers le fichier de clé privée du client pour l'authentification TLS mutuelle (optionnel).",
        "hi": "पारस्परिक TLS प्रमाणीकरण के लिए क्लाइंट निजी कुंजी फ़ाइल का पथ (वैकल्पिक)।",
        "it": "Percorso al file della chiave privata del client per l'autenticazione TLS reciproca (opzionale).",
        "ja": "相互TLS認証用のクライアント秘密鍵ファイルへのパス（オプション）。",
        "nl": "Pad naar het bestand met de persoonlijke sleutel van de client voor wederzijdse TLS-authenticatie (optioneel).",
        "no": "Sti til klientens private nøkkelfil for gjensidig TLS-autentisering (valgfritt).",
        "pl": "Ścieżka do pliku klucza prywatnego klienta dla wzajemnego uwierzytelnienia TLS (opcjonalnie).",
        "pt": "Caminho para o arquivo de chave privada do cliente para autenticação TLS mútua (opcional).",
        "sv": "Sökväg till klientens privata nyckelfil för ömsesidig TLS-autentisering (valfritt).",
        "zh": "用于双向 TLS 认证的客户端私钥文件路径（可选）。"
    },
    "Prefers tools with higher success rates in adaptive selection.": {
        "cs": "Upřednostňuje nástroje s vyšší úspěšností v adaptivním výběru.",
        "da": "Foretrækker værktøjer med højere succesrater i adaptiv udvælgelse.",
        "de": "Bevorzugt Tools mit höheren Erfolgsquoten bei der adaptiven Auswahl.",
        "el": "Προτιμά εργαλεία με υψηλότερα ποσοστά επιτυχίας στην προσαρμοστική επιλογή.",
        "es": "Prefiere herramientas con mayores tasas de éxito en la selección adaptativa.",
        "fr": "Préfère les outils avec des taux de réussite plus élevés dans la sélection adaptative.",
        "hi": "अनुकूली चयन में उच्च सफलता दर वाले टूल को प्राथमिकता देता है।",
        "it": "Preferisce strumenti con tassi di successo più elevati nella selezione adattiva.",
        "ja": "適応選択で成功率の高いツールを優先します。",
        "nl": "Voorkeur voor tools met hogere slagingspercentages bij adaptieve selectie.",
        "no": "Foretrekker verktøy med høyere suksessrater i adaptiv utvelgelse.",
        "pl": "Preferuje narzędzia o wyższych wskaźnikach sukcesu w doborze adaptacyjnym.",
        "pt": "Prefere ferramentas com maiores taxas de sucesso na seleção adaptativa.",
        "sv": "Föredrar verktyg med högre framgångsgrad i adaptivt urval.",
        "zh": "在自适应选择中优先选择成功率更高的工具。"
    },
    "Removes stale tool transition data after X days.": {
        "cs": "Odstraňuje zastaralá data o přechodech nástrojů po X dnech.",
        "da": "Fjerner forældet værktøjsovergangsdata efter X dage.",
        "de": "Entfernt veraltete Tool-Übergangsdaten nach X Tagen.",
        "el": "Αφαιρεί παλιά δεδομένα μετάβασης εργαλείων μετά από X ημέρες.",
        "es": "Elimina datos de transición de herramientas obsoletos después de X días.",
        "fr": "Supprime les données de transition d'outils obsolètes après X jours.",
        "hi": "X दिनों के बाद पुराने टूल संक्रमण डेटा को हटाता है।",
        "it": "Rimuove i dati di transizione degli strumenti non aggiornati dopo X giorni.",
        "ja": "X日後に古いツール遷移データを削除します。",
        "nl": "Verwijdert verouderde tool-overgangsgegevens na X dagen.",
        "no": "Fjerner utdatert verktøyovergangsdata etter X dager.",
        "pl": "Usuwa nieaktualne dane przejść narzędzi po X dniach.",
        "pt": "Remove dados de transição de ferramentas obsoletos após X dias.",
        "sv": "Tar bort inaktuellt verktygsövergångsdata efter X dagar.",
        "zh": "在 X 天后删除过时的工具转换数据。"
    },
    "Require Bearer token authentication for all n8n requests (strongly recommended).": {
        "cs": "Vyžadovat Bearer token autentizaci pro všechny n8n požadavky (důrazně doporučeno).",
        "da": "Kræv Bearer-token-godkendelse for alle n8n-anmodninger (stærkt anbefalet).",
        "de": "Bearer-Token-Authentifizierung für alle n8n-Anforderungen erzwingen (dringend empfohlen).",
        "el": "Απαιτείται αυθεντικοποίηση διακριτικού Bearer για όλα τα αιτήματα n8n (συνιστάται έντονα).",
        "es": "Requerir autenticación de token portador para todas las solicitudes n8n (muy recomendado).",
        "fr": "Exiger l'authentification par jeton porteur pour toutes les requêtes n8n (fortement recommandé).",
        "hi": "सभी n8n अनुरोधों के लिए बियरर टोकन प्रमाणीकरण आवश्यक (दृढ़ता से अनुशंसित)।",
        "it": "Richiedere l'autenticazione con token Bearer per tutte le richieste n8n (fortemente consigliato).",
        "ja": "すべてのn8nリクエストにベアラートークン認証を要求（強く推奨）。",
        "nl": "Bearer-token-authenticatie vereisen voor alle n8n-verzoeken (sterk aanbevolen).",
        "no": "Krev bearer-token-autentisering for alle n8n-forespørsler (sterkt anbefalt).",
        "pl": "Wymagaj uwierzytelnienia tokenem Bearer dla wszystkich żądań n8n (zdecydowanie zalecane).",
        "pt": "Exigir autenticação de token portador para todas as solicitações n8n (altamente recomendado).",
        "sv": "Kräv bearer-token-autentisering för alla n8n-förfrågningar (rekommenderas starkt).",
        "zh": "要求所有 n8n 请求使用持有者令牌认证（强烈推荐）。"
    }
}

fixed = 0
for section in LANG_DIR.iterdir():
    if not section.is_dir():
        continue
    en_path = section / "en.json"
    if not en_path.exists():
        continue
    try:
        raw = en_path.read_text(encoding="utf-8-sig")
        en_data = json.loads(raw)
    except:
        continue

    for lang in LANGS:
        lang_path = section / f"{lang}.json"
        if not lang_path.exists():
            continue
        try:
            raw = lang_path.read_text(encoding="utf-8-sig")
            data = json.loads(raw)
        except:
            continue

        changed = False
        for key, en_val in en_data.items():
            if key not in data:
                continue
            if data[key] != en_val:
                continue
            if en_val in T and lang in T[en_val]:
                data[key] = T[en_val][lang]
                changed = True
                fixed += 1

        if changed:
            with open(lang_path, "w", encoding="utf-8") as f:
                json.dump(data, f, ensure_ascii=False, indent=2)
                f.write("\n")

print(f"Batch 19: Fixed {fixed} keys")

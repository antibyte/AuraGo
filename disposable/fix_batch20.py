#!/usr/bin/env python3
"""Batch 20: Config descriptions part 4 + remaining high-impact items."""
import json
from pathlib import Path

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "S3 endpoint URL (e.g. https://s3.amazonaws.com or http://minio.local:9000). Leave blank for AWS default.": {
        "cs": "S3 endpoint URL (např. https://s3.amazonaws.com nebo http://minio.local:9000). Pro AWS výchozí nechte prázdné.",
        "da": "S3 endpoint-URL (f.eks. https://s3.amazonaws.com eller http://minio.local:9000). Efterlad tom for AWS-standard.",
        "de": "S3-Endpunkt-URL (z.B. https://s3.amazonaws.com oder http://minio.local:9000). Für AWS-Standard leer lassen.",
        "el": "URL τελικού σημείου S3 (π.χ. https://s3.amazonaws.com ή http://minio.local:9000). Αφήστε κενό για προεπιλογή AWS.",
        "es": "URL del endpoint S3 (p.ej. https://s3.amazonaws.com o http://minio.local:9000). Dejar vacío para AWS predeterminado.",
        "fr": "URL de l'endpoint S3 (p.ex. https://s3.amazonaws.com ou http://minio.local:9000). Laisser vide pour AWS par défaut.",
        "hi": "S3 एंडपॉइंट URL (उदा. https://s3.amazonaws.com या http://minio.local:9000)। AWS डिफ़ॉल्ट के लिए खाली छोड़ें।",
        "it": "URL dell'endpoint S3 (es. https://s3.amazonaws.com o http://minio.local:9000). Lasciare vuoto per il default AWS.",
        "ja": "S3エンドポイントURL（例：https://s3.amazonaws.com または http://minio.local:9000）。AWSデフォルトの場合は空のまま。",
        "nl": "S3 endpoint-URL (bijv. https://s3.amazonaws.com of http://minio.local:9000). Laat leeg voor AWS-standaard.",
        "no": "S3 endepunkt-URL (f.eks. https://s3.amazonaws.com eller http://minio.local:9000). La stå tom for AWS-standard.",
        "pl": "URL punktu końcowego S3 (np. https://s3.amazonaws.com lub http://minio.local:9000). Pozostaw puste dla domyślnego AWS.",
        "pt": "URL do endpoint S3 (p.ex. https://s3.amazonaws.com ou http://minio.local:9000). Deixe vazio para o padrão AWS.",
        "sv": "S3-endpoint-URL (t.ex. https://s3.amazonaws.com eller http://minio.local:9000). Lämna tom för AWS-standard.",
        "zh": "S3 端点 URL（例如 https://s3.amazonaws.com 或 http://minio.local:9000）。AWS 默认值请留空。"
    },
    "Skip TLS certificate verification. WARNING: Use only for testing with self-signed certificates.": {
        "cs": "Přeskočit ověřování TLS certifikátů. VAROVÁNÍ: Použijte pouze pro testování s certifikáty podepsanými vlastní autoritou.",
        "da": "Spring TLS-certifikatbekræftelse over. ADVARSEL: Brug kun til test med selvsignerede certifikater.",
        "de": "TLS-Zertifikatsüberprüfung überspringen. WARNUNG: Nur für Tests mit selbstsignierten Zertifikaten verwenden.",
        "el": "Παράλειψη επαλήθευσης πιστοποιητικού TLS. ΠΡΟΕΙΔΟΠΟΙΗΣΗ: Χρησιμοποιήστε μόνο για δοκιμές με αυτο-υπογεγραμμένα πιστοποιητικά.",
        "es": "Omitir verificación de certificados TLS. ADVERTENCIA: Usar solo para pruebas con certificados autofirmados.",
        "fr": "Ignorer la vérification des certificats TLS. AVERTISSEMENT : Utiliser uniquement pour les tests avec des certificats auto-signés.",
        "hi": "TLS प्रमाणपत्र सत्यापन छोड़ें। चेतावनी: केवल स्व-हस्ताक्षरित प्रमाणपत्रों के साथ परीक्षण के लिए उपयोग करें।",
        "it": "Salta la verifica dei certificati TLS. AVVISO: Utilizzare solo per test con certificati autofirmati.",
        "ja": "TLS証明書の検証をスキップ。警告：自己署名証明書でのテストにのみ使用してください。",
        "nl": "TLS-certificaatverificatie overslaan. WAARSCHUWING: Gebruik alleen voor testen met zelfondertekende certificaten.",
        "no": "Hopp over TLS-sertifikatbekreftelse. ADVARSEL: Bruk kun for testing med selvsignerte sertifikater.",
        "pl": "Pomiń weryfikację certyfikatów TLS. OSTRZEŻENIE: Używaj tylko do testów z certyfikatami z podpisem własnym.",
        "pt": "Ignorar verificação de certificado TLS. AVISO: Use apenas para testes com certificados autoassinados.",
        "sv": "Hoppa över TLS-certifikatverifiering. VARNING: Använd endast för testning med självsignerade certifikat.",
        "zh": "跳过 TLS 证书验证。警告：仅用于自签名证书的测试。"
    },
    "The request timed out. Please try again or switch to a faster model.": {
        "cs": "Požadavek vypršel. Zkuste to znovu nebo přejděte na rychlejší model.",
        "da": "Anmodningen fik timeout. Prøv igen eller skift til en hurtigere model.",
        "de": "Die Anfrage hat das Zeitlimit überschritten. Bitte versuchen Sie es erneut oder wechseln Sie zu einem schnelleren Modell.",
        "el": "Το αίτημα έληξε. Δοκιμάστε ξανά ή μεταβείτε σε ένα πιο γρήγορο μοντέλο.",
        "es": "La solicitud expiró. Intente de nuevo o cambie a un modelo más rápido.",
        "fr": "La requête a expiré. Veuillez réessayer ou passer à un modèle plus rapide.",
        "hi": "अनुरोध का समय समाप्त हो गया। कृपया पुनः प्रयास करें या तेज़ मॉडल पर स्विच करें।",
        "it": "La richiesta è scaduta. Riprova o passa a un modello più veloce.",
        "ja": "リクエストがタイムアウトしました。再試行するか、より高速なモデルに切り替えてください。",
        "nl": "Het verzoek is verlopen. Probeer opnieuw of schakel over naar een sneller model.",
        "no": "Forespørselen fikk tidsavbrudd. Prøv igjen eller bytt til en raskere modell.",
        "pl": "Żądanie wygasło. Spróbuj ponownie lub przejdź na szybszy model.",
        "pt": "A solicitação expirou. Tente novamente ou mude para um modelo mais rápido.",
        "sv": "Begäran fick timeout. Försök igen eller byt till en snabbare modell.",
        "zh": "请求超时。请重试或切换到更快的模型。"
    },
    "Tool responses exceeding this character limit are truncated. Prevents token waste from overly long outputs.": {
        "cs": "Odpovědi nástrojů přesahující tento limit znaků jsou zkráceny. Zabraňuje plýtvání tokeny příliš dlouhými výstupy.",
        "da": "Værktøjs svar der overstiger denne tegngrænse afkortes. Forhindrer spild af tokens fra for lange output.",
        "de": "Tool-Antworten, die dieses Zeichenlimit überschreiten, werden abgeschnitten. Verhindert Token-Verschwendung durch zu lange Ausgaben.",
        "el": "Οι απαντήσεις εργαλείων που υπερβαίνουν αυτό το όριο χαρακτήρων περικόπτονται. Αποτρέπει τη σπατάλη tokens από υπερβολικά μεγάλες εξόδους.",
        "es": "Las respuestas de herramientas que excedan este límite de caracteres se truncarán. Previene el desperdicio de tokens por salidas demasiado largas.",
        "fr": "Les réponses d'outils dépassant cette limite de caractères sont tronquées. Empêche le gaspillage de tokens dû à des sorties trop longues.",
        "hi": "इस अक्षर सीमा से अधिक के टूल उत्तरों को छोटा कर दिया जाता है। अत्यधिक लंबे आउटपुट से टोकन बर्बादी रोकता है।",
        "it": "Le risposte degli strumenti che superano questo limite di caratteri vengono troncate. Previene lo spreco di token da output troppo lunghi.",
        "ja": "この文字制限を超えるツール応答は切り捨てられます。長すぎる出力によるトークン無駄遣いを防ぎます。",
        "nl": "Tool-reacties die deze tekenlimiet overschrijden worden afgekapt. Voorkomt tokenverspilling door te lange uitvoer.",
        "no": "Verktøysvar som overskrider denne tegngrensen avkortes. Forhindrer tokensløsing fra for lange utdata.",
        "pl": "Odpowiedzi narzędzi przekraczające ten limit znaków są obcinane. Zapobiega marnowaniu tokenów przez zbyt długie wyniki.",
        "pt": "Respostas de ferramentas que excedem este limite de caracteres são truncadas. Evita desperdício de tokens por saídas muito longas.",
        "sv": "Verktygssvar som överskrider denna teckengräns trunkeras. Förhindrar tokenslöseri från för långa utdata.",
        "zh": "超过此字符限制的工具响应将被截断。防止过长输出造成的令牌浪费。"
    },
    "Tool names always offered regardless of adaptive selection.": {
        "cs": "Názvy nástrojů vždy nabízeny bez ohledu na adaptivní výběr.",
        "da": "Værktøjsnavne altid tilbudt uanset adaptiv udvælgelse.",
        "de": "Tool-Namen, die immer angeboten werden, unabhängig von der adaptiven Auswahl.",
        "el": "Ονόματα εργαλείων που προσφέρονται πάντα ανεξάρτητα από την προσαρμοστική επιλογή.",
        "es": "Nombres de herramientas siempre ofrecidos independientemente de la selección adaptativa.",
        "fr": "Noms d'outils toujours proposés indépendamment de la sélection adaptative.",
        "hi": "अनुकूली चयन की परवाह किए बिना हमेशा दिए जाने वाले टूल नाम।",
        "it": "Nomi degli strumenti sempre offerti indipendentemente dalla selezione adattiva.",
        "ja": "適応選択に関係なく常に提供されるツール名。",
        "nl": "Toolnamen die altijd worden aangeboden, ongeacht adaptieve selectie.",
        "no": "Verktøynavn som alltid tilbys uavhengig av adaptiv utvelgelse.",
        "pl": "Nazwy narzędzi zawsze oferowane niezależnie od doboru adaptacyjnego.",
        "pt": "Nomes de ferramentas sempre oferecidos independentemente da seleção adaptativa.",
        "sv": "Verktygsnamn som alltid erbjuds oavsett adaptivt urval.",
        "zh": "无论自适应选择如何，始终提供的工具名称。"
    },
    "URL of the Paperless-ngx instance (e.g. https://paperless.example.com).": {
        "cs": "URL instance Paperless-ngx (např. https://paperless.example.com).",
        "da": "URL for Paperless-ngx-instansen (f.eks. https://paperless.example.com).",
        "de": "URL der Paperless-ngx-Instanz (z.B. https://paperless.example.com).",
        "el": "URL της παρουσίας Paperless-ngx (π.χ. https://paperless.example.com).",
        "es": "URL de la instancia de Paperless-ngx (p.ej. https://paperless.example.com).",
        "fr": "URL de l'instance Paperless-ngx (p.ex. https://paperless.example.com).",
        "hi": "Paperless-ngx इंस्टेंस का URL (उदा. https://paperless.example.com)।",
        "it": "URL dell'istanza Paperless-ngx (es. https://paperless.example.com).",
        "ja": "Paperless-ngxインスタンスのURL（例：https://paperless.example.com）。",
        "nl": "URL van de Paperless-ngx-instantie (bijv. https://paperless.example.com).",
        "no": "URL for Paperless-ngx-instansen (f.eks. https://paperless.example.com).",
        "pl": "URL instancji Paperless-ngx (np. https://paperless.example.com).",
        "pt": "URL da instância Paperless-ngx (p.ex. https://paperless.example.com).",
        "sv": "URL för Paperless-ngx-instansen (t.ex. https://paperless.example.com).",
        "zh": "Paperless-ngx 实例的 URL（例如 https://paperless.example.com）。"
    },
    "Wait time after the main response in seconds before a follow-up task starts.": {
        "cs": "Čekací doba po hlavní odpovědi v sekundách před spuštěním navazující úlohy.",
        "da": "Ventetid efter hovedsvaret i sekunder før en opfølgende opgave starter.",
        "de": "Wartezeit nach der Hauptantwort in Sekunden, bevor eine Folgetask startet.",
        "el": "Χρόνος αναμονής μετά την κύρια απάντηση σε δευτερόλεπτα πριν ξεκινήσει μια εργασία παρακολούθησης.",
        "es": "Tiempo de espera después de la respuesta principal en segundos antes de que comience una tarea de seguimiento.",
        "fr": "Temps d'attente après la réponse principale en secondes avant le démarrage d'une tâche de suivi.",
        "hi": "अनुवर्ती कार्य शुरू होने से पहले मुख्य प्रतिक्रिया के बाद प्रतीक्षा समय सेकंड में।",
        "it": "Tempo di attesa dopo la risposta principale in secondi prima dell'avvio di un'attività di follow-up.",
        "ja": "フォローアップタスク開始前のメイン応答後の待機時間（秒）。",
        "nl": "Wachttijd na het hoofdantwoord in seconden voordat een vervolgtaak start.",
        "no": "Ventetid etter hovedsvaret i sekunder før en oppfølgingsoppgave starter.",
        "pl": "Czas oczekiwania po głównej odpowiedzi w sekundach przed uruchomieniem zadania następczego.",
        "pt": "Tempo de espera após a resposta principal em segundos antes de iniciar uma tarefa de acompanhamento.",
        "sv": "Väntetid efter huvudsvaret i sekunder innan en uppföljningsuppgift startar.",
        "zh": "主响应后等待多少秒再启动后续任务。"
    },
    "Wait time between retry attempts in seconds.": {
        "cs": "Čekací doba mezi pokusy o opakování v sekundách.",
        "da": "Ventetid mellem genforsøg i sekunder.",
        "de": "Wartezeit zwischen Wiederholungsversuchen in Sekunden.",
        "el": "Χρόνος αναμονής μεταξύ προσπαθειών επανάληψης σε δευτερόλεπτα.",
        "es": "Tiempo de espera entre reintentos en segundos.",
        "fr": "Temps d'attente entre les tentatives de nouvelle tentative en secondes.",
        "hi": "पुनः प्रयास के बीच प्रतीक्षा समय सेकंड में।",
        "it": "Tempo di attesa tra i tentativi di riprova in secondi.",
        "ja": "リトライ間の待機時間（秒）。",
        "nl": "Wachttijd tussen herpogingen in seconden.",
        "no": "Ventetid mellom nye forsøk i sekunder.",
        "pl": "Czas oczekiwania między ponownymi próbami w sekundach.",
        "pt": "Tempo de espera entre tentativas de repetição em segundos.",
        "sv": "Väntetid mellan återförsök i sekunder.",
        "zh": "重试之间的等待时间（秒）。"
    },
    "When active, n8n can only read data — no chats, tool execution, or writes.": {
        "cs": "Pokud je aktivní, n8n může pouze číst data — žádné chaty, spouštění nástrojů nebo zápisy.",
        "da": "Når aktiv, kan n8n kun læse data — ingen chats, værktøjsudførelse eller skrivninger.",
        "de": "Wenn aktiv, kann n8n nur Daten lesen — keine Chats, Tool-Ausführung oder Schreibvorgänge.",
        "el": "Όταν είναι ενεργό, το n8n μπορεί μόνο να διαβάζει δεδομένα — χωρίς συνομιλίες, εκτέλεση εργαλείων ή εγγραφές.",
        "es": "Cuando está activo, n8n solo puede leer datos — sin chats, ejecución de herramientas o escrituras.",
        "fr": "Lorsqu'actif, n8n ne peut que lire des données — pas de chats, d'exécution d'outils ou d'écritures.",
        "hi": "सक्रिय होने पर, n8n केवल डेटा पढ़ सकता है — कोई चैट, टूल निष्पादन या लेखन नहीं।",
        "it": "Quando attivo, n8n può solo leggere dati — niente chat, esecuzione di strumenti o scritture.",
        "ja": "アクティブな場合、n8nはデータの読み取りのみ可能 — チャット、ツール実行、書き込みは不可。",
        "nl": "Indien actief, kan n8n alleen gegevens lezen — geen chats, tool-uitvoering of schrijfbewerkingen.",
        "no": "Når aktiv, kan n8n bare lese data — ingen chatter, verktøykjøring eller skriving.",
        "pl": "Gdy aktywne, n8n może tylko odczytywać dane — brak czatów, wykonywania narzędzi lub zapisów.",
        "pt": "Quando ativo, n8n só pode ler dados — sem chats, execução de ferramentas ou gravações.",
        "sv": "När aktiv kan n8n endast läsa data — inga chattar, verktygskörningar eller skrivningar.",
        "zh": "激活时，n8n 只能读取数据 — 不能聊天、执行工具或写入。"
    },
    "When enabled, MCP clients must authenticate with a Bearer token or valid session cookie.": {
        "cs": "Pokud je povoleno, MCP klienti se musí autentizovat pomocí Bearer tokenu nebo platného session cookie.",
        "da": "Når aktiveret, skal MCP-klienter godkende med et Bearer-token eller en gyldig session-cookie.",
        "de": "Wenn aktiviert, müssen sich MCP-Clients mit einem Bearer-Token oder einem gültigen Session-Cookie authentifizieren.",
        "el": "Όταν ενεργοποιηθεί, οι πελάτες MCP πρέπει να αυθεντικοποιηθούν με ένα διακριτικό Bearer ή ένα έγκυρο cookie συνεδρίας.",
        "es": "Cuando está habilitado, los clientes MCP deben autenticarse con un token portador o cookie de sesión válida.",
        "fr": "Lorsqu'activé, les clients MCP doivent s'authentifier avec un jeton porteur ou un cookie de session valide.",
        "hi": "सक्षम होने पर, MCP क्लाइंट को बियरर टोकन या मान्य सेशन कुकी के साथ प्रमाणित होना चाहिए।",
        "it": "Quando abilitato, i client MCP devono autenticarsi con un token Bearer o un cookie di sessione valido.",
        "ja": "有効な場合、MCPクライアントはベアラートークンまたは有効なセッションCookieで認証する必要があります。",
        "nl": "Indien ingeschakeld, moeten MCP-clients authenticeren met een Bearer-token of geldig sessiecookie.",
        "no": "Når aktivert, må MCP-klienter autentisere seg med et bearer-token eller en gyldig sesjons-Cookie.",
        "pl": "Po włączeniu, klienci MCP muszą uwierzytelnić się za pomocą tokenu Bearer lub prawidłowego ciasteczka sesji.",
        "pt": "Quando ativado, clientes MCP devem se autenticar com um token portador ou cookie de sessão válido.",
        "sv": "När aktiverat måste MCP-klienter autentisera med ett bearer-token eller en giltig sessionscookie.",
        "zh": "启用时，MCP 客户端必须使用持有者令牌或有效的会话 cookie 进行认证。"
    },
    "When enabled, the agent can only list and download objects but cannot upload, delete, copy, or move them.": {
        "cs": "Pokud je povoleno, agent může pouze vypsat a stáhnout objekty, ale nemůže je nahrávat, mazat, kopírovat ani přesouvat.",
        "da": "Når aktiveret, kan agenten kun liste og downloade objekter, men kan ikke uploade, slette, kopiere eller flytte dem.",
        "de": "Wenn aktiviert, kann der Agent Objekte nur auflisten und herunterladen, aber nicht hochladen, löschen, kopieren oder verschieben.",
        "el": "Όταν ενεργοποιηθεί, ο πράκτορας μπορεί μόνο να παραθέσει και να κατεβάσει αντικείμενα αλλά δεν μπορεί να τα ανεβάσει, διαγράψει, αντιγράψει ή μετακινήσει.",
        "es": "Cuando está habilitado, el agente solo puede listar y descargar objetos, pero no puede subirlos, eliminarlos, copiarlos o moverlos.",
        "fr": "Lorsqu'activé, l'agent ne peut que lister et télécharger des objets mais ne peut pas les téléverser, supprimer, copier ou déplacer.",
        "hi": "सक्षम होने पर, एजेंट केवल ऑब्जेक्ट को सूचीबद्ध और डाउनलोड कर सकता है लेकिन अपलोड, हटाना, कॉपी या स्थानांतरित नहीं कर सकता।",
        "it": "Quando abilitato, l'agente può solo elencare e scaricare oggetti ma non può caricarli, eliminarli, copiarli o spostarli.",
        "ja": "有効な場合、エージェントはオブジェクトのリストとダウンロードのみ可能で、アップロード、削除、コピー、移動はできません。",
        "nl": "Indien ingeschakeld, kan de agent alleen objecten weergeven en downloaden, maar niet uploaden, verwijderen, kopiëren of verplaatsen.",
        "no": "Når aktivert, kan agenten bare liste og laste ned objekter, men kan ikke laste opp, slette, kopiere eller flytte dem.",
        "pl": "Po włączeniu agent może tylko wyświetlać i pobierać obiekty, ale nie może ich przesyłać, usuwać, kopiować ani przenosić.",
        "pt": "Quando ativado, o agente só pode listar e baixar objetos, mas não pode enviá-los, excluí-los, copiá-los ou movê-los.",
        "sv": "När aktiverat kan agenten endast lista och ladda ner objekt men inte ladda upp, radera, kopiera eller flytta dem.",
        "zh": "启用时，代理只能列出和下载对象，但不能上传、删除、复制或移动它们。"
    },
    "When enabled, the agent can only search and read documents but cannot upload, update, or delete them.": {
        "cs": "Pokud je povoleno, agent může pouze vyhledávat a číst dokumenty, ale nemůže je nahrávat, aktualizovat ani mazat.",
        "da": "Når aktiveret, kan agenten kun søge og læse dokumenter, men kan ikke uploade, opdatere eller slette dem.",
        "de": "Wenn aktiviert, kann der Agent Dokumente nur suchen und lesen, aber nicht hochladen, aktualisieren oder löschen.",
        "el": "Όταν ενεργοποιηθεί, ο πράκτορας μπορεί μόνο να αναζητήσει και να διαβάσει έγγραφα αλλά δεν μπορεί να τα ανεβάσει, ενημερώσει ή διαγράψει.",
        "es": "Cuando está habilitado, el agente solo puede buscar y leer documentos, pero no puede subirlos, actualizarlos o eliminarlos.",
        "fr": "Lorsqu'activé, l'agent ne peut que rechercher et lire des documents mais ne peut pas les téléverser, mettre à jour ou supprimer.",
        "hi": "सक्षम होने पर, एजेंट केवल दस्तावेज़ खोज और पढ़ सकता है लेकिन अपलोड, अपडेट या हटाना नहीं कर सकता।",
        "it": "Quando abilitato, l'agente può solo cercare e leggere documenti ma non può caricarli, aggiornarli o eliminarli.",
        "ja": "有効な場合、エージェントはドキュメントの検索と読み取りのみ可能で、アップロード、更新、削除はできません。",
        "nl": "Indien ingeschakeld, kan de agent alleen documenten zoeken en lezen, maar niet uploaden, bijwerken of verwijderen.",
        "no": "Når aktivert, kan agenten bare søke i og lese dokumenter, men kan ikke laste opp, oppdatere eller slette dem.",
        "pl": "Po włączeniu agent może tylko wyszukiwać i czytać dokumenty, ale nie może ich przesyłać, aktualizować ani usuwać.",
        "pt": "Quando ativado, o agente só pode pesquisar e ler documentos, mas não pode enviá-los, atualizá-los ou excluí-los.",
        "sv": "När aktiverat kan agenten endast söka och läsa dokument men inte ladda upp, uppdatera eller radera dem.",
        "zh": "启用时，代理只能搜索和阅读文档，但不能上传、更新或删除它们。"
    },
    "Minimum messages in history before the agent retries on empty responses.": {
        "cs": "Minimální počet zpráv v historii před opakováním agenta při prázdných odpovědích.",
        "da": "Minimum antal beskeder i historikken før agenten genprøver ved tomme svar.",
        "de": "Mindestanzahl an Nachrichten im Verlauf, bevor der Agent bei leeren Antworten erneut versucht.",
        "el": "Ελάχιστος αριθμός μηνυμάτων στο ιστορικό πριν ο πράκτορας ξαναδοκιμάσει σε κενές απαντήσεις.",
        "es": "Mínimo de mensajes en el historial antes de que el agente reintente en respuestas vacías.",
        "fr": "Nombre minimum de messages dans l'historique avant que l'agent ne réessaie sur des réponses vides.",
        "hi": "रिक्त प्रतिक्रियाओं पर एजेंट द्वारा पुनः प्रयास से पहले इतिहास में न्यूनतम संदेश।",
        "it": "Numero minimo di messaggi nella cronologia prima che l'agente riprova su risposte vuote.",
        "ja": "空の応答でエージェントがリトライする前の履歴内の最小メッセージ数。",
        "nl": "Minimum aantal berichten in de geschiedenis voordat de agent het opnieuw probeert bij lege reacties.",
        "no": "Minimum antall meldinger i historikken før agenten prøver igjen ved tomme svar.",
        "pl": "Minimalna liczba wiadomości w historii przed ponowną próbą agenta przy pustych odpowiedziach.",
        "pt": "Mínimo de mensagens no histórico antes de o agente tentar novamente em respostas vazias.",
        "sv": "Minsta antal meddelanden i historiken innan agenten försöker igen vid tomma svar.",
        "zh": "代理在空响应时重试之前历史记录中的最少消息数。"
    },
    "Number of consecutive identical responses detected as a loop.": {
        "cs": "Počet po sobě jdoucích identických odpovědí detekovaných jako smyčka.",
        "da": "Antal fortløbende identiske svar opdaget som en løkke.",
        "de": "Anzahl aufeinanderfolgender identischer Antworten, die als Schleife erkannt werden.",
        "el": "Αριθμός διαδοχικών ταυτόσημων απαντήσεων που εντοπίζονται ως βρόχος.",
        "es": "Número de respuestas idénticas consecutivas detectadas como bucle.",
        "fr": "Nombre de réponses identiques consécutives détectées comme une boucle.",
        "hi": "लूप के रूप में पहचाने गए लगातार समान प्रतिक्रियाओं की संख्या।",
        "it": "Numero di risposte identiche consecutive rilevate come ciclo.",
        "ja": "ループとして検出される連続した同一応答の数。",
        "nl": "Aantal opeenvolgende identieke reacties gedetecteerd als een lus.",
        "no": "Antall etterfølgende identiske svar oppdaget som en løkke.",
        "pl": "Liczba kolejnych identycznych odpowiedzi wykrytych jako pętla.",
        "pt": "Número de respostas idênticas consecutivas detectadas como loop.",
        "sv": "Antal på varandra följande identiska svar som upptäcks som en loop.",
        "zh": "检测为循环的连续相同响应数量。"
    },
    "n8n can connect to AuraGo as an AI backend. AuraGo can also trigger n8n webhooks on events. The API token protects the AuraGo n8n endpoint — copy it into your n8n credential.": {
        "cs": "n8n se může připojit k AuraGo jako AI backend. AuraGo může také spouštět n8n webhooky na události. API token chrání AuraGo n8n endpoint — zkopírujte jej do svého n8n pověření.",
        "da": "n8n kan forbinde til AuraGo som en AI-backend. AuraGo kan også udløse n8n-webhooks ved hændelser. API-tokenet beskytter AuraGo n8n-endepunktet — kopier det til din n8n-legitimation.",
        "de": "n8n kann sich als AI-Backend mit AuraGo verbinden. AuraGo kann auch n8n-Webhooks bei Ereignissen auslösen. Das API-Token schützt den AuraGo-n8n-Endpunkt — kopieren Sie es in Ihre n8n-Zugangsdaten.",
        "el": "Το n8n μπορεί να συνδεθεί στο AuraGo ως backend AI. Το AuraGo μπορεί επίσης να ενεργοποιεί n8n webhooks σε συμβάντα. Το API token προστατεύει το endpoint AuraGo n8n — αντιγράψτε το στα διαπιστευτήρια n8n σας.",
        "es": "n8n puede conectarse a AuraGo como backend de IA. AuraGo también puede activar webhooks de n8n en eventos. El token API protege el endpoint n8n de AuraGo — cópielo en su credencial n8n.",
        "fr": "n8n peut se connecter à AuraGo comme backend IA. AuraGo peut aussi déclencher des webhooks n8n lors d'événements. Le token API protège l'endpoint n8n d'AuraGo — copiez-le dans votre identifiant n8n.",
        "hi": "n8n AuraGo से AI बैकएंड के रूप में कनेक्ट कर सकता है। AuraGo ईवेंट पर n8n वेबहुक भी ट्रिगर कर सकता है। API टोकन AuraGo n8n एंडपॉइंट की सुरक्षा करता है — इसे अपने n8n क्रेडेंशियल में कॉपी करें।",
        "it": "n8n può connettersi ad AuraGo come backend IA. AuraGo può anche attivare webhook n8n sugli eventi. Il token API protegge l'endpoint n8n di AuraGo — copialo nelle tue credenziali n8n.",
        "ja": "n8nはAIバックエンドとしてAuraGoに接続できます。AuraGoはイベントでn8nウェブフックもトリガーできます。APIトークンはAuraGo n8nエンドポイントを保護します — n8nクレデンシャルにコピーしてください。",
        "nl": "n8n kan verbinden met AuraGo als AI-backend. AuraGo kan ook n8n-webhooks activeren bij gebeurtenissen. De API-token beschermt het AuraGo n8n-endpoint — kopieer het naar uw n8n-inloggegevens.",
        "no": "n8n kan koble til AuraGo som en AI-backend. AuraGo kan også utløse n8n-webhooks ved hendelser. API-tokenet beskytter AuraGo n8n-endepunktet — kopier det til din n8n-legitimasjon.",
        "pl": "n8n może połączyć się z AuraGo jako backend AI. AuraGo może również wyzwalać webhooki n8n przy zdarzeniach. Token API chroni endpoint AuraGo n8n — skopiuj go do swoich poświadczeń n8n.",
        "pt": "n8n pode se conectar ao AuraGo como backend de IA. AuraGo também pode acionar webhooks n8n em eventos. O token API protege o endpoint n8n do AuraGo — copie-o para sua credencial n8n.",
        "sv": "n8n kan ansluta till AuraGo som en AI-backend. AuraGo kan också utlösa n8n-webhooks vid händelser. API-token skyddar AuraGo n8n-endpointen — kopiera den till dina n8n-uppgifter.",
        "zh": "n8n 可以作为 AI 后端连接到 AuraGo。AuraGo 也可以在事件上触发 n8n webhooks。API 令牌保护 AuraGo n8n 端点 — 将其复制到您的 n8n 凭据中。"
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

print(f"Batch 20: Fixed {fixed} keys")

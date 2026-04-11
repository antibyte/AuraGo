#!/usr/bin/env python3
"""Batch 18: Config descriptions part 2 - high-impact translations (14 occ each)."""
import json
from pathlib import Path

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "Enable the built-in MCP server to expose AuraGo tools for external AI agents via Streamable HTTP.": {
        "cs": "Povolit vestavěný MCP server pro zpřístupnění nástrojů AuraGo externím AI agentům přes Streamable HTTP.",
        "da": "Aktiver den indbyggede MCP-server for at eksponere AuraGo-værktøjer til eksterne AI-agenter via Streamable HTTP.",
        "de": "Den integrierten MCP-Server aktivieren, um AuraGo-Tools für externe KI-Agenten über Streamable HTTP bereitzustellen.",
        "el": "Ενεργοποιήστε τον ενσωματωμένο διακομιστή MCP για να εκθέσετε τα εργαλεία AuraGo σε εξωτερικούς πράκτορες AI μέσω Streamable HTTP.",
        "es": "Activar el servidor MCP integrado para exponer las herramientas de AuraGo a agentes IA externos mediante Streamable HTTP.",
        "fr": "Activer le serveur MCP intégré pour exposer les outils AuraGo aux agents IA externes via Streamable HTTP.",
        "hi": "बाहरी AI एजेंटों के लिए Streamable HTTP के माध्यम से AuraGo टूल साझा करने के लिए अंतर्निहित MCP सर्वर सक्षम करें।",
        "it": "Abilita il server MCP integrato per esporre gli strumenti AuraGo agli agenti IA esterni tramite Streamable HTTP.",
        "ja": "内蔵MCPサーバーを有効にして、Streamable HTTP経由で外部AIエージェントにAuraGoツールを公開します。",
        "nl": "Schakel de ingebouwde MCP-server in om AuraGo-tools beschikbaar te maken voor externe AI-agenten via Streamable HTTP.",
        "no": "Aktiver den innebygde MCP-serveren for å eksponere AuraGo-verktøy for eksterne AI-agenter via Streamable HTTP.",
        "pl": "Włącz wbudowany serwer MCP, aby udostępnić narzędzia AuraGo zewnętrznym agentom AI przez Streamable HTTP.",
        "pt": "Ativar o servidor MCP integrado para expor ferramentas AuraGo para agentes IA externos via Streamable HTTP.",
        "sv": "Aktivera den inbyggda MCP-servern för att exponera AuraGo-verktyg för externa AI-agenter via Streamable HTTP.",
        "zh": "启用内置 MCP 服务器，通过 Streamable HTTP 向外部 AI 代理公开 AuraGo 工具。"
    },
    "Enables the S3-compatible storage integration (AWS S3, MinIO, Wasabi, Backblaze B2).": {
        "cs": "Povoluje integraci S3-kompatibilního úložiště (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "da": "Aktiverer S3-kompatibel lagringsintegration (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "de": "Aktiviert die S3-kompatible Speicherintegration (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "el": "Ενεργοποιεί την ενσωμάτωση αποθήκευσης συμβατής με S3 (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "es": "Activa la integración de almacenamiento compatible con S3 (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "fr": "Active l'intégration de stockage compatible S3 (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "hi": "S3-संगत स्टोरेज एकीकरण सक्षम करता है (AWS S3, MinIO, Wasabi, Backblaze B2)।",
        "it": "Abilita l'integrazione di archiviazione compatibile S3 (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "ja": "S3互換ストレージ統合を有効にします（AWS S3、MinIO、Wasabi、Backblaze B2）。",
        "nl": "Schakelt S3-compatibele opslagintegratie in (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "no": "Aktiverer S3-kompatibel lagringsintegrasjon (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "pl": "Włącza integrację z magazynem zgodnym z S3 (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "pt": "Ativa a integração de armazenamento compatível com S3 (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "sv": "Aktiverar S3-kompatibel lagringsintegration (AWS S3, MinIO, Wasabi, Backblaze B2).",
        "zh": "启用 S3 兼容存储集成（AWS S3、MinIO、Wasabi、Backblaze B2）。"
    },
    "Enables the dedicated Helper LLM for internal analysis and background tasks. If disabled, many dependent helper features will not work fully.": {
        "cs": "Povoluje vyhrazený Helper LLM pro interní analýzu a úlohy na pozadí. Pokud je zakázán, mnoho závislých pomocných funkcí nebude plně fungovat.",
        "da": "Aktiverer den dedikerede Helper LLM til intern analyse og baggrundsopgaver. Hvis deaktiveret, vil mange afhængige hjælpefunktioner ikke fungere fuldt ud.",
        "de": "Aktiviert das dedizierte Helper-LLM für interne Analysen und Hintergrundaufgaben. Wenn deaktiviert, funktionieren viele abhängige Hilfsfunktionen nicht vollständig.",
        "el": "Ενεργοποιεί το αποκλειστικό Helper LLM για εσωτερική ανάλυση και εργασίες παρασκηνίου. Αν απενεργοποιηθεί, πολλές εξαρτώμενες βοηθητικές λειτουργίες δεν θα λειτουργούν πλήρως.",
        "es": "Activa el LLM Helper dedicado para análisis interno y tareas en segundo plano. Si se desactiva, muchas funciones auxiliares dependientes no funcionarán completamente.",
        "fr": "Active le LLM Helper dédié pour l'analyse interne et les tâches en arrière-plan. S'il est désactivé, de nombreuses fonctions auxiliaires dépendantes ne fonctionneront pas complètement.",
        "hi": "आंतरिक विश्लेषण और बैकग्राउंड कार्यों के लिए समर्पित सहायक LLM सक्षम करता है। अक्षम होने पर, कई आश्रित सहायक सुविधाएँ पूरी तरह काम नहीं करेंगी।",
        "it": "Abilita l'LLM Helper dedicato per l'analisi interna e le attività in background. Se disabilitato, molte funzioni helper dipendenti non funzioneranno completamente.",
        "ja": "内部分析とバックグラウンドタスク用の専用Helper LLMを有効にします。無効にすると、多くの依存ヘルパー機能が完全に動作しません。",
        "nl": "Schakelt de toegewezen Helper LLM in voor interne analyse en achtergrondtaken. Bij uitschakeling werken veel afhankelijke hulpfuncties niet volledig.",
        "no": "Aktiverer den dedikerte Helper LLM for intern analyse og bakgrunnsoppgaver. Hvis deaktivert, vil mange avhengige hjelpefunksjoner ikke fungere fullt ut.",
        "pl": "Włącza dedykowany Helper LLM do analizy wewnętrznej i zadań w tle. Jeśli wyłączony, wiele zależnych funkcji pomocniczych nie będzie działać w pełni.",
        "pt": "Ativa o LLM Helper dedicado para análise interna e tarefas em segundo plano. Se desativado, muitos recursos auxiliares dependentes não funcionarão completamente.",
        "sv": "Aktiverar den dedikerade Helper LLM för intern analys och bakgrundsuppgifter. Om inaktiverad kommer många beroende hjälpfunktioner inte att fungera fullt ut.",
        "zh": "启用专用 Helper LLM 用于内部分析和后台任务。如果禁用，许多依赖的辅助功能将无法完全工作。"
    },
    "Enables/disables agent debug mode (detailed error messages in system prompt).": {
        "cs": "Povoluje/zakazuje režim ladění agenta (detailní chybové zprávy v systémovém promptu).",
        "da": "Aktiverer/deaktiverer agent-fejlretningstilstand (detaljerede fejlmeddelelser i systemprompten).",
        "de": "Aktiviert/deaktiviert den Agent-Debug-Modus (detaillierte Fehlermeldungen im System-Prompt).",
        "el": "Ενεργοποιεί/απενεργοποιεί τη λειτουργία εντοπισμού σφαλμάτων του πράκτορα (αναλυτικά μηνύματα σφάλματος στην προτροπή συστήματος).",
        "es": "Activa/desactiva el modo de depuración del agente (mensajes de error detallados en el prompt del sistema).",
        "fr": "Active/désactive le mode débogage de l'agent (messages d'erreur détaillés dans le prompt système).",
        "hi": "एजेंट डीबग मोड सक्षम/अक्षम करता है (सिस्टम प्रॉम्प्ट में विस्तृत त्रुटि संदेश)।",
        "it": "Abilita/disabilita la modalità debug dell'agente (messaggi di errore dettagliati nel prompt di sistema).",
        "ja": "エージェントのデバッグモードを有効/無効にします（システムプロンプト内の詳細なエラーメッセージ）。",
        "nl": "Schakelt de debugmodus van de agent in/uit (gedetailleerde foutmeldingen in de systeemprompt).",
        "no": "Aktiverer/deaktiverer agentens feilsøkingsmodus (detaljerte feilmeldinger i systemprompten).",
        "pl": "Włącza/wyłącza tryb debugowania agenta (szczegółowe komunikaty błędów w prompcie systemowym).",
        "pt": "Ativa/desativa o modo de depuração do agente (mensagens de erro detalhadas no prompt do sistema).",
        "sv": "Aktiverar/inaktiverar agentens felsökningsläge (detaljerade felmeddelanden i systemprompten).",
        "zh": "启用/禁用代理调试模式（系统提示中的详细错误消息）。"
    },
    "Existing long-term memory vectors may no longer match the new embeddings model.": {
        "cs": "Stávající vektory dlouhodobé paměti se již nemusí shodovat s novým modelem embeddings.",
        "da": "Eksisterende langtids-hukommelsesvektorer matcher muligvis ikke længere den nye embeddings-model.",
        "de": "Bestehende Langzeitgedächtnis-Vektoren stimmen möglicherweise nicht mehr mit dem neuen Embeddings-Modell überein.",
        "el": "Υπάρχοντα διανύσματα μακροπρόθεσμης μνήμης μπορεί να μην ταιριάζουν πλέον στο νέο μοντέλο embeddings.",
        "es": "Los vectores de memoria a largo plazo existentes pueden ya no coincidir con el nuevo modelo de embeddings.",
        "fr": "Les vecteurs de mémoire à long terme existants peuvent ne plus correspondre au nouveau modèle d'embeddings.",
        "hi": "मौजूदा दीर्घकालिक मेमोरी वेक्टर नए एम्बेडिंग मॉडल से अब मेल नहीं खा सकते।",
        "it": "I vettori di memoria a lungo termine esistenti potrebbero non corrispondere più al nuovo modello di embeddings.",
        "ja": "既存の長期メモリベクトルが新しい埋め込みモデルと一致しなくなる場合があります。",
        "nl": "Bestaande langetermijngeheugen-vectoren komen mogelijk niet meer overeen met het nieuwe embeddings-model.",
        "no": "Eksisterende langtidsminne-vektorer stemmer muligens ikke lenger overens med den nye embeddings-modellen.",
        "pl": "Istniejące wektory pamięci długotrwałej mogą już nie pasować do nowego modelu embeddingów.",
        "pt": "Vetores de memória de longo prazo existentes podem não corresponder mais ao novo modelo de embeddings.",
        "sv": "Befintliga långtidsminnesvektorer kanske inte längre matchar den nya embeddings-modellen.",
        "zh": "现有的长期记忆向量可能不再与新的嵌入模型匹配。"
    },
    "Generate a bearer token before connecting an external MCP client.": {
        "cs": "Vygenerujte bearer token před připojením externího MCP klienta.",
        "da": "Generer et bearer-token inden du tilslutter en ekstern MCP-klient.",
        "de": "Generieren Sie ein Bearer-Token, bevor Sie einen externen MCP-Client verbinden.",
        "el": "Δημιουργήστε ένα διακριτικό bearer πριν συνδέσετε έναν εξωτερικό πελάτη MCP.",
        "es": "Genere un token portador antes de conectar un cliente MCP externo.",
        "fr": "Générez un jeton porteur avant de connecter un client MCP externe.",
        "hi": "बाहरी MCP क्लाइंट को कनेक्ट करने से पहले एक बियरर टोकन जनरेट करें।",
        "it": "Genera un token bearer prima di connettere un client MCP esterno.",
        "ja": "外部MCPクライアントを接続する前にベアラートークンを生成してください。",
        "nl": "Genereer een bearer-token voordat u een externe MCP-client verbindt.",
        "no": "Generer et bearer-token før du kobler til en ekstern MCP-klient.",
        "pl": "Wygeneruj token bearer przed podłączeniem zewnętrznego klienta MCP.",
        "pt": "Gere um token portador antes de conectar um cliente MCP externo.",
        "sv": "Generera ett bearer-token innan du ansluter en extern MCP-klient.",
        "zh": "在连接外部 MCP 客户端之前生成一个持有者令牌。"
    },
    "Generate a token here and enter it as the Bearer token in your n8n AuraGo credential.": {
        "cs": "Vygenerujte token zde a zadejte jej jako Bearer token ve vašem n8n AuraGo pověření.",
        "da": "Generer et token her og indtast det som Bearer-token i din n8n AuraGo-legitimation.",
        "de": "Generieren Sie hier ein Token und geben Sie es als Bearer-Token in Ihren n8n AuraGo-Zugangsdaten ein.",
        "el": "Δημιουργήστε ένα διακριτικό εδώ και εισάγετέ το ως διακριτικό Bearer στα διαπιστευτήρια AuraGo του n8n σας.",
        "es": "Genere un token aquí e introdúzcalo como token portador en su credencial n8n AuraGo.",
        "fr": "Générez un jeton ici et entrez-le comme jeton porteur dans votre identifiant n8n AuraGo.",
        "hi": "यहाँ एक टोकन जनरेट करें और इसे अपने n8n AuraGo क्रेडेंशियल में बियरर टोकन के रूप में दर्ज करें।",
        "it": "Genera un token qui e inseriscilo come token Bearer nelle tue credenziali n8n AuraGo.",
        "ja": "ここでトークンを生成し、n8n AuraGoクレデンシャルのベアラートークンとして入力してください。",
        "nl": "Genereer hier een token en voer het in als Bearer-token in uw n8n AuraGo-inloggegevens.",
        "no": "Generer et token her og skriv det inn som Bearer-token i din n8n AuraGo-legitimasjon.",
        "pl": "Wygeneruj token tutaj i wprowadź go jako token Bearer w poświadczeniu n8n AuraGo.",
        "pt": "Gere um token aqui e insira-o como token portador na sua credencial n8n AuraGo.",
        "sv": "Generera ett token här och ange det som Bearer-token i dina n8n AuraGo-uppgifter.",
        "zh": "在此生成令牌并将其作为持有者令牌输入到您的 n8n AuraGo 凭据中。"
    },
    "Generate images from text prompts using AI": {
        "cs": "Generovat obrázky z textových promptů pomocí AI",
        "da": "Generer billeder fra tekstprompts med AI",
        "de": "Bilder aus Text-Prompts mit KI generieren",
        "el": "Δημιουργία εικόνων από προτροπές κειμένου χρησιμοποιώντας AI",
        "es": "Generar imágenes a partir de prompts de texto usando IA",
        "fr": "Générer des images à partir de prompts textuels avec l'IA",
        "hi": "AI का उपयोग करके टेक्स्ट प्रॉम्प्ट से छवियाँ उत्पन्न करें",
        "it": "Genera immagini da prompt di testo utilizzando l'IA",
        "ja": "AIを使用してテキストプロンプトから画像を生成",
        "nl": "Afbeeldingen genereren uit tekstprompts met AI",
        "no": "Generer bilder fra tekstprompter med AI",
        "pl": "Generuj obrazy z promptów tekstowych za pomocą AI",
        "pt": "Gerar imagens a partir de prompts de texto usando IA",
        "sv": "Generera bilder från textpromptar med AI",
        "zh": "使用 AI 从文本提示生成图像"
    },
    "GitHub username or organization used as the default owner for API calls.": {
        "cs": "Uživatelské jméno GitHub nebo organizace použitá jako výchozí vlastník pro volání API.",
        "da": "GitHub-brugernavn eller -organisation brugt som standardejer for API-kald.",
        "de": "GitHub-Benutzername oder -Organisation, der als Standard-Eigentümer für API-Aufrufe verwendet wird.",
        "el": "Όνομα χρήστη ή οργανισμού GitHub που χρησιμοποιείται ως προεπιλεγμένος κάτοχος για κλήσεις API.",
        "es": "Nombre de usuario u organización de GitHub usado como propietario predeterminado para llamadas API.",
        "fr": "Nom d'utilisateur ou d'organisation GitHub utilisé comme propriétaire par défaut pour les appels API.",
        "hi": "API कॉल के लिए डिफ़ॉल्ट स्वामी के रूप में उपयोग किया गया GitHub उपयोगकर्ता नाम या संगठन।",
        "it": "Nome utente o organizzazione GitHub utilizzato come proprietario predefinito per le chiamate API.",
        "ja": "API呼び出しのデフォルトオーナーとして使用されるGitHubユーザー名または組織。",
        "nl": "GitHub-gebruikersnaam of -organisatie gebruikt als standaardeigenaar voor API-aanroepen.",
        "no": "GitHub-brukernavn eller -organisasjon brukt som standardeier for API-kall.",
        "pl": "Nazwa użytkownika lub organizacja GitHub używana jako domyślny właściciel dla wywołań API.",
        "pt": "Nome de usuário ou organização do GitHub usado como proprietário padrão para chamadas de API.",
        "sv": "GitHub-användarnamn eller -organisation som används som standardägare för API-anrop.",
        "zh": "GitHub 用户名或组织，用作 API 调用的默认所有者。"
    },
    "Half-life for usage scoring in days. Older usage data loses influence. Higher values = slower forgetting.": {
        "cs": "Poločas rozpadu pro hodnocení využití ve dnech. Starší data o využití ztrácejí vliv. Vyšší hodnoty = pomalejší zapomínání.",
        "da": "Halveringstid for forbrugsscoring i dage. Ældre forbrugsdata mister indflydelse. Højere værdier = langsommere glemme.",
        "de": "Halbwertszeit für Nutzungsbewertung in Tagen. Ältere Nutzungsdaten verlieren an Einfluss. Höhere Werte = langsameres Vergessen.",
        "el": "Ημιζωή για βαθμολόγηση χρήσης σε ημέρες. Τα παλαιότερα δεδομένα χρήσης χάνουν επιρροή. Υψηλότερες τιμές = πιο αργή λήθη.",
        "es": "Vida media para la puntuación de uso en días. Los datos de uso antiguos pierden influencia. Valores más altos = olvido más lento.",
        "fr": "Demi-vie pour le score d'utilisation en jours. Les données d'utilisation plus anciennes perdent de l'influence. Valeurs plus élevées = oubli plus lent.",
        "hi": "उपयोग स्कोरिंग के लिए अर्धजीवन दिनों में। पुराने उपयोग डेटा का प्रभाव कम होता है। उच्च मान = धीमा भूलना।",
        "it": "Emivita per il punteggio di utilizzo in giorni. I dati di utilizzo più vecchi perdono influenza. Valori più alti = dimenticanza più lenta.",
        "ja": "使用スコアリングの半減期（日数）。古い使用データは影響力を失います。高い値 = より遅い忘却。",
        "nl": "Halfwaardetijd voor gebruiksscore in dagen. Oudere gebruiksgegevens verliezen invloed. Hogere waarden = langzamer vergeten.",
        "no": "Halveringstid for bruksscoring i dager. Eldre bruksdata mister innflydelse. Høyere verdier = tregere glemming.",
        "pl": "Okres półtrwania dla oceny wykorzystania w dniach. Starsze dane o użyciu tracą wpływ. Wyższe wartości = wolniejsze zapominanie.",
        "pt": "Meia-vida para pontuação de uso em dias. Dados de uso mais antigos perdem influência. Valores mais altos = esquecimento mais lento.",
        "sv": "Halveringstid för användningsresultat i dagar. Äldre användningsdata förlorar inflytande. Högre värden = långsammare glömska.",
        "zh": "使用评分的半衰期（天）。较旧的使用数据失去影响力。值越高 = 遗忘越慢。"
    },
    "How many times the agent tries to recover from provider '422 Bad Request' errors before giving up.": {
        "cs": "Kolikrát se agent pokusí zotavit z chyb poskytovatele '422 Bad Request' před tím, než se vzdá.",
        "da": "Hvor mange gange agenten forsøger at komme sig fra udbyderens '422 Bad Request'-fejl før opgivelse.",
        "de": "Wie oft der Agent versucht, sich von Provider-'422 Bad Request'-Fehlern zu erholen, bevor er aufgibt.",
        "el": "Πόσες φορές ο πράκτορας προσπαθεί να ανακάμψει από σφάλματα παρόχου '422 Bad Request' πριν τα παρατήσει.",
        "es": "Cuántas veces intenta el agente recuperarse de errores '422 Bad Request' del proveedor antes de rendirse.",
        "fr": "Combien de fois l'agent tente de se remettre des erreurs '422 Bad Request' du fournisseur avant d'abandonner.",
        "hi": "एजेंट हार मानने से पहले प्रदाता '422 Bad Request' त्रुटियों से कितनी बार पुनर्प्राप्त करने का प्रयास करता है।",
        "it": "Quante volte l'agente tenta di riprendersi da errori '422 Bad Request' del provider prima di arrendersi.",
        "ja": "エージェントが諦める前にプロバイダーの「422 Bad Request」エラーからの復旧を試みる回数。",
        "nl": "Hoe vaak de agent probeert te herstellen van provider '422 Bad Request'-fouten voordat het opgeeft.",
        "no": "Hvor mange ganger agenten prøver å gjenopprette fra leverandørens '422 Bad Request'-feil før den gir opp.",
        "pl": "Ile razy agent próbuje odzyskać sprawność po błędach dostawcy '422 Bad Request' przed poddaniem się.",
        "pt": "Quantas vezes o agente tenta se recuperar de erros '422 Bad Request' do provedor antes de desistir.",
        "sv": "Hur många gånger agenten försöker återhämta sig från leverantörens '422 Bad Request'-fel innan den ger upp.",
        "zh": "代理在放弃之前尝试从提供商的 '422 Bad Request' 错误中恢复的次数。"
    },
    "How many times the same response may appear in history before detecting a loop.": {
        "cs": "Kolikrát se může stejná odpověď objevit v historii před detekcí smyčky.",
        "da": "Hvor mange gange det samme svar kan optræde i historikken før en løkke opdages.",
        "de": "Wie oft dieselbe Antwort im Verlauf erscheinen darf, bevor eine Schleife erkannt wird.",
        "el": "Πόσες φορές μπορεί να εμφανιστεί η ίδια απάντηση στο ιστορικό πριν εντοπιστεί ένας βρόχος.",
        "es": "Cuántas veces puede aparecer la misma respuesta en el historial antes de detectar un bucle.",
        "fr": "Combien de fois la même réponse peut apparaître dans l'historique avant de détecter une boucle.",
        "hi": "लूप का पता लगाने से पहले इतिहास में एक ही प्रतिक्रिया कितनी बार दिखाई दे सकती है।",
        "it": "Quante volte la stessa risposta può apparire nella cronologia prima di rilevare un ciclo.",
        "ja": "ループを検出する前に、同じ応答が履歴に何回出現できるか。",
        "nl": "Hoe vaak hetzelfde antwoord in de geschiedenis mag verschijnen voordat een lus wordt gedetecteerd.",
        "no": "Hvor mange ganger det samme svaret kan forekomme i historikken før en løkke oppdages.",
        "pl": "Ile razy ta sama odpowiedź może pojawić się w historii przed wykryciem pętli.",
        "pt": "Quantas vezes a mesma resposta pode aparecer no histórico antes de detectar um loop.",
        "sv": "Hur många gånger samma svar får förekomma i historiken innan en loop upptäcks.",
        "zh": "在检测到循环之前，相同响应在历史记录中可以出现的次数。"
    },
    "How many times the same tool error may occur consecutively before the agent aborts.": {
        "cs": "Kolikrát se může stejná chyba nástroje opakovat po sobě, než se agent přeruší.",
        "da": "Hvor mange gange den samme værktøjsfejl kan optræde i træk før agenten afbryder.",
        "de": "Wie oft derselbe Tool-Fehler aufeinanderfolgend auftreten darf, bevor der Agent abbricht.",
        "el": "Πόσες φορές μπορεί να εμφανιστεί συνεχώς το ίδιο σφάλμα εργαλείου πριν ο πράκτορας ματαιώσει.",
        "es": "Cuántas veces puede ocurrir el mismo error de herramienta consecutivamente antes de que el agente aborte.",
        "fr": "Combien de fois la même erreur d'outil peut se produire consécutivement avant que l'agent n'abandonne.",
        "hi": "एजेंट के निरस्त होने से पहले एक ही टूल त्रुटि लगातार कितनी बार हो सकती है।",
        "it": "Quante volte lo stesso errore dello strumento può verificarsi consecutivamente prima che l'agente si interrompa.",
        "ja": "エージェントが中断する前に、同じツールエラーが連続して発生できる回数。",
        "nl": "Hoe vaak dezelfde tool-fout opeenvolgend mag optreden voordat de agent afbreekt.",
        "no": "Hvor mange ganger den samme verktøyfeilen kan forekomme etter hverandre før agenten avbryter.",
        "pl": "Ile razy ten sam błąd narzędzia może wystąpić kolejno, zanim agent przerwie.",
        "pt": "Quantas vezes o mesmo erro de ferramenta pode ocorrer consecutivamente antes de o agente abortar.",
        "sv": "Hur många gånger samma verktygsfel får inträffa i följd innan agenten avbryter.",
        "zh": "在代理中止之前，同一工具错误可以连续出现的次数。"
    },
    "How often the agent checks the status of a waiting task (in seconds).": {
        "cs": "Jak často agent kontroluje stav čekající úlohy (v sekundách).",
        "da": "Hvor ofte agenten tjekker status for en ventende opgave (i sekunder).",
        "de": "Wie oft der Agent den Status einer wartenden Aufgabe prüft (in Sekunden).",
        "el": "Πόσο συχνά ο πράκτορας ελέγχει την κατάσταση μιας αναμένων εργασίας (σε δευτερόλεπτα).",
        "es": "Con qué frecuencia el agente verifica el estado de una tarea en espera (en segundos).",
        "fr": "À quelle fréquence l'agent vérifie l'état d'une tâche en attente (en secondes).",
        "hi": "एजेंट कितनी बार प्रतीक्षा कर रहे कार्य की स्थिति जांचता है (सेकंड में)।",
        "it": "Quanto spesso l'agente controlla lo stato di un'attività in attesa (in secondi).",
        "ja": "エージェントが待機中タスクのステータスを確認する頻度（秒）。",
        "nl": "Hoe vaak de agent de status van een wachtende taak controleert (in seconden).",
        "no": "Hvor ofte agenten sjekker statusen til en ventende oppgave (i sekunder).",
        "pl": "Jak często agent sprawdza stan oczekującego zadania (w sekundach).",
        "pt": "Com que frequência o agente verifica o status de uma tarefa em espera (em segundos).",
        "sv": "Hur ofta agenten kontrollerar statusen för en väntande uppgift (i sekunder).",
        "zh": "代理检查等待任务状态的频率（秒）。"
    },
    "How quickly personality traits and mood fade without new input. 0.1 = slow decay, 2 = very fast.": {
        "cs": "Jak rychle rysy osobnosti a nálada blednou bez nového vstupu. 0.1 = pomalý úbytek, 2 = velmi rychlý.",
        "da": "Hvor hurtigt personlighedstræk og humør falmer uden nye input. 0.1 = langsom forfald, 2 = meget hurtigt.",
        "de": "Wie schnell Persönlichkeitsmerkmale und Stimmung ohne neue Eingaben verblassen. 0.1 = langsamer Zerfall, 2 = sehr schnell.",
        "el": "Πόσο γρήγορα ξεθωριάζουν τα χαρακτηριστικά προσωπικότητας και η διάθεση χωρίς νέα είσοδο. 0.1 = αργή παρακμή, 2 = πολύ γρήγορη.",
        "es": "Qué tan rápido se desvanecen los rasgos de personalidad y el estado de ánimo sin nueva entrada. 0.1 = decaimiento lento, 2 = muy rápido.",
        "fr": "La vitesse à laquelle les traits de personnalité et l'humeur s'estompent sans nouvelle entrée. 0.1 = déclin lent, 2 = très rapide.",
        "hi": "बिना नए इनपुट के व्यक्तित्व लक्षण और मूड कितनी जल्दी फीके पड़ते हैं। 0.1 = धीमा क्षय, 2 = बहुत तेज़।",
        "it": "Quanto velocemente sfumano i tratti della personalità e l'umore senza nuovi input. 0.1 = decadimento lento, 2 = molto veloce.",
        "ja": "新しい入力なしに性格特性と気分がどれくらい早く薄れるか。0.1 = ゆっくり、2 = 非常に速い。",
        "nl": "Hoe snel persoonlijkheidstrekken en stemming vervagen zonder nieuwe invoer. 0.1 = langzaam verval, 2 = zeer snel.",
        "no": "Hvor raskt personlighetstrekk og humor falmer uten ny input. 0.1 = treg forfall, 2 = veldig raskt.",
        "pl": "Jak szybko znikają cechy osobowości i nastrój bez nowych danych wejściowych. 0.1 = powolny zanik, 2 = bardzo szybki.",
        "pt": "Quão rápido os traços de personalidade e humor desaparecem sem nova entrada. 0.1 = declínio lento, 2 = muito rápido.",
        "sv": "Hur snabbt personlighetsdrag och humle bleknar utan ny indata. 0.1 = långsamt förfall, 2 = mycket snabbt.",
        "zh": "在没有新输入的情况下，个性特征和情绪消退的速度。0.1 = 缓慢衰减，2 = 非常快。"
    },
    "How sensitive the persona is to lack of social interaction. 0 = immune, 2 = very sensitive.": {
        "cs": "Jak citlivá je persona na nedostatek sociální interakce. 0 = imunní, 2 = velmi citlivá.",
        "da": "Hvor følsom personaen er over for manglende social interaktion. 0 = immun, 2 = meget følsom.",
        "de": "Wie empfindlich die Persona auf fehlende soziale Interaktion reagiert. 0 = immun, 2 = sehr empfindlich.",
        "el": "Πόσο ευαίσθητο είναι το πρόσωπο στην έλλειψη κοινωνικής αλληλεπίδρασης. 0 = άνοσο, 2 = πολύ ευαίσθητο.",
        "es": "Qué tan sensible es la persona a la falta de interacción social. 0 = inmune, 2 = muy sensible.",
        "fr": "Sensibilité de la persona au manque d'interaction sociale. 0 = insensible, 2 = très sensible.",
        "hi": "पर्सोना सामाजिक बातचीत की कमी के प्रति कितना संवेदनशील है। 0 = प्रतिरक्षित, 2 = बहुत संवेदनशील।",
        "it": "Quanto è sensibile la persona alla mancanza di interazione sociale. 0 = immune, 2 = molto sensibile.",
        "ja": "ペルソナが社会的交流の欠如に対してどれくらい敏感か。0 = 免疫あり、2 = 非常に敏感。",
        "nl": "Hoe gevoelig de persona is voor gebrek aan sociale interactie. 0 = immuun, 2 = zeer gevoelig.",
        "no": "Hvor sensitiv personaen er for manglende sosial interaksjon. 0 = immun, 2 = veldig sensitiv.",
        "pl": "Jak wrażliwa jest persona na brak interakcji społecznych. 0 = odporna, 2 = bardzo wrażliwa.",
        "pt": "Quão sensível a persona é à falta de interação social. 0 = imune, 2 = muito sensível.",
        "sv": "Hur känslig personaen är för avsaknad av social interaktion. 0 = immun, 2 = mycket känslig.",
        "zh": "角色对缺乏社交互动的敏感程度。0 = 免疫，2 = 非常敏感。"
    },
    "How strongly external events shift mood. 0 = barely reactive, 2 = maximum swing.": {
        "cs": "Jak silně externí události mění náladu. 0 = stěží reaktivní, 2 = maximální výkyv.",
        "da": "Hvor stærkt eksterne begivenheder skifter humør. 0 = knap reaktiv, 2 = maksimal svingning.",
        "de": "Wie stark externe Ereignisse die Stimmung verschieben. 0 = kaum reaktiv, 2 = maximale Schwankung.",
        "el": "Πόσο έντονα τα εξωτερικά γεγονότα μετατοπίζουν τη διάθεση. 0 = μόλις αντιδραστικό, 2 = μέγιστη ταλάντωση.",
        "es": "Qué tan fuertemente los eventos externos cambian el estado de ánimo. 0 = apenas reactivo, 2 = oscilación máxima.",
        "fr": "L'intensité avec laquelle les événements externes modifient l'humeur. 0 = à peine réactif, 2 = oscillation maximale.",
        "hi": "बाहरी घटनाएँ मूड को कितनी अधिक बदलती हैं। 0 = मुश्किल से प्रतिक्रियाशील, 2 = अधिकतम उतार-चढ़ाव।",
        "it": "Quanto fortemente gli eventi esterni spostano l'umore. 0 = appena reattivo, 2 = oscillazione massima.",
        "ja": "外部イベントが気分をどれくらい強く変化させるか。0 = ほとんど反応なし、2 = 最大変動。",
        "nl": "Hoe sterk externe gebeurtenissen de stemming beïnvloeden. 0 = nauwelijks reactief, 2 = maximale schommeling.",
        "no": "Hvor sterkt eksterne hendelser endrer humøret. 0 = knapt reaktiv, 2 = maksimal svingning.",
        "pl": "Jak silnie zdarzenia zewnętrzne zmieniają nastrój. 0 = ledwo reaktywny, 2 = maksymalna zmiana.",
        "pt": "Quão fortemente eventos externos mudam o humor. 0 = mal reativo, 2 = oscilação máxima.",
        "sv": "Hur starkt externa händelser påverkar humöret. 0 = knappt reaktiv, 2 = maximal svängning.",
        "zh": "外部事件对情绪的影响程度。0 = 几乎无反应，2 = 最大波动。"
    },
    "How the persona responds to conflict or contradiction.": {
        "cs": "Jak persona reaguje na konflikt nebo rozpor.",
        "da": "Hvordan personaen reagerer på konflikt eller modsigelse.",
        "de": "Wie die Persona auf Konflikt oder Widerspruch reagiert.",
        "el": "Πώς το πρόσωπο ανταποκρίνεται σε σύγκρουση ή αντίφαση.",
        "es": "Cómo responde la persona ante conflictos o contradicciones.",
        "fr": "Comment la persona réagit au conflit ou à la contradiction.",
        "hi": "पर्सोना संघर्ष या विरोधाभास का जवाब कैसे देती है।",
        "it": "Come la persona risponde al conflitto o alla contraddizione.",
        "ja": "ペルソナが対立や矛盾にどう反応するか。",
        "nl": "Hoe de persona reageert op conflict of tegenspraak.",
        "no": "Hvordan personaen reagerer på konflikt eller motsigelse.",
        "pl": "Jak persona reaguje na konflikt lub sprzeczność.",
        "pt": "Como a persona responde a conflitos ou contradições.",
        "sv": "Hur personaen reagerar på konflikt eller motsägelse.",
        "zh": "角色如何应对冲突或矛盾。"
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

print(f"Batch 18: Fixed {fixed} keys")

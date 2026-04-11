#!/usr/bin/env python3
"""Batch 17: Config descriptions part 1 - high-impact translations (14 occ each)."""
import json
from pathlib import Path

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "Allow HTTP (non-TLS) endpoints. Only enable for local/dev environments.": {
        "cs": "Povolit HTTP (bez TLS) koncové body. Povolte pouze pro místní/vývojová prostředí.",
        "da": "Tillad HTTP (ikke-TLS) endepunkter. Aktiver kun for lokale/udviklingsmiljøer.",
        "de": "HTTP-Endpunkte (ohne TLS) zulassen. Nur für lokale/Entwicklungsumgebungen aktivieren.",
        "el": "Να επιτραπούν HTTP (μη TLS) τελικά σημεία. Ενεργοποιήστε μόνο για τοπικά/αναπτυξιακά περιβάλλοντα.",
        "es": "Permitir endpoints HTTP (sin TLS). Activar solo para entornos locales/desarrollo.",
        "fr": "Autoriser les endpoints HTTP (sans TLS). Activer uniquement pour les environnements locaux/développement.",
        "hi": "HTTP (गैर-TLS) एंडपॉइंट की अनुमति दें। केवल स्थानीय/विकास वातावरण के लिए सक्षम करें।",
        "it": "Consenti endpoint HTTP (senza TLS). Abilita solo per ambienti locali/sviluppo.",
        "ja": "HTTP（非TLS）エンドポイントを許可。ローカル/開発環境のみ有効にしてください。",
        "nl": "HTTP (zonder TLS) endpoints toestaan. Alleen inschakelen voor lokale/ontwikkelomgevingen.",
        "no": "Tillat HTTP (ikke-TLS) endepunkter. Aktiver kun for lokale/utviklingsmiljøer.",
        "pl": "Zezwalaj na endpointy HTTP (bez TLS). Włącz tylko dla środowisk lokalnych/programistycznych.",
        "pt": "Permitir endpoints HTTP (sem TLS). Ative apenas para ambientes locais/desenvolvimento.",
        "sv": "Tillåt HTTP (icke-TLS) slutpunkter. Aktivera endast för lokala/utvecklingsmiljöer.",
        "zh": "允许 HTTP（非 TLS）端点。仅在本地/开发环境中启用。"
    },
    "Allow temporary token budget overflow for homepage flows": {
        "cs": "Povolit dočasné překročení rozpočtu tokenů pro toky domovské stránky",
        "da": "Tillad midlertidigt token-budgetoverskridelse for hjemmesideflows",
        "de": "Temporäres Überschreiten des Token-Budgets für Homepage-Workflows zulassen",
        "el": "Να επιτραπεί προσωρινή υπέρβαση προϋπολογισμού tokens για ροές αρχικής σελίδας",
        "es": "Permitir desbordamiento temporal del presupuesto de tokens para flujos de homepage",
        "fr": "Autoriser le dépassement temporaire du budget de tokens pour les flux de la page d'accueil",
        "hi": "होमपेज़ प्रवाह के लिए अस्थायी टोकन बजट अतिप्रवाह की अनुमति दें",
        "it": "Consenti superamento temporaneo del budget token per i flussi della homepage",
        "ja": "ホームページフローの一時的なトークン予算超過を許可",
        "nl": "Tijdelijke token-budgetoverschrijding toestaan voor homepage-flows",
        "no": "Tillat midlertidig overskridelse av tokenbudsjett for hjemmesideflyter",
        "pl": "Zezwalaj na tymczasowe przekroczenie budżetu tokenów dla przepływów strony głównej",
        "pt": "Permitir excesso temporário do orçamento de tokens para fluxos da página inicial",
        "sv": "Tillåt tillfällig överskridning av tokenbudget för hemsidflöden",
        "zh": "允许主页流程临时超出令牌预算"
    },
    "Allows the agent to start asynchronous follow-up tasks after the main response is sent.": {
        "cs": "Umožňuje agentovi spouštět asynchronní navazující úkoly po odeslání hlavní odpovědi.",
        "da": "Lader agenten starte asynkrone opfølgende opgaver, efter hovedsvaret er sendt.",
        "de": "Ermöglicht dem Agenten, asynchrone Folgetasks zu starten, nachdem die Hauptantwort gesendet wurde.",
        "el": "Επιτρέπει στον πράκτορα να ξεκινά ασύγχρονες εργασίες παρακολούθησης μετά την αποστολή της κύριας απάντησης.",
        "es": "Permite al agente iniciar tareas asíncronas de seguimiento después de enviar la respuesta principal.",
        "fr": "Permet à l'agent de démarrer des tâches de suivi asynchrones après l'envoi de la réponse principale.",
        "hi": "एजेंट को मुख्य प्रतिक्रिया भेजने के बाद अतुल्यकालिक अनुवर्ती कार्य शुरू करने की अनुमति देता है।",
        "it": "Consente all'agente di avviare attività di follow-up asincrone dopo l'invio della risposta principale.",
        "ja": "メイン応答の送信後、エージェントが非同期フォローアップタスクを開始できるようにします。",
        "nl": "Stelt de agent in staat om asynchrone vervolgtaken te starten nadat het hoofdantwoord is verzonden.",
        "no": "Lar agenten starte asynkrone oppfølgingsoppgaver etter at hovedsvaret er sendt.",
        "pl": "Pozwala agentowi uruchamiać asynchroniczne zadania następcze po wysłaniu głównej odpowiedzi.",
        "pt": "Permite ao agente iniciar tarefas assíncronas de acompanhamento após o envio da resposta principal.",
        "sv": "Låter agenten starta asynkrona uppföljningsuppgifter efter att huvudsvaret har skickats.",
        "zh": "允许代理在发送主响应后启动异步后续任务。"
    },
    "Automatically adjusts the token budget to the model's context window size. Recommended with token budget = 0.": {
        "cs": "Automaticky přizpůsobí rozpočet tokenů velikosti kontextového okna modelu. Doporučeno s rozpočtem tokenů = 0.",
        "da": "Justerer automatisk token-budgettet til modellens kontekstvinduesstørrelse. Anbefales med token-budget = 0.",
        "de": "Passt das Token-Budget automatisch an die Kontextfenstergröße des Modells an. Empfohlen mit Token-Budget = 0.",
        "el": "Προσαρμόζει αυτόματα τον προϋπολογισμό tokens στο μέγεθος του παραθύρου περιβάλλοντος του μοντέλου. Συνιστάται με προϋπολογισμό tokens = 0.",
        "es": "Ajusta automáticamente el presupuesto de tokens al tamaño de la ventana de contexto del modelo. Recomendado con presupuesto de tokens = 0.",
        "fr": "Ajuste automatiquement le budget de tokens à la taille de la fenêtre de contexte du modèle. Recommandé avec un budget de tokens = 0.",
        "hi": "टोकन बजट को स्वचालित रूप से मॉडल के संदर्भ विंडो आकार में समायोजित करता है। टोकन बजट = 0 के साथ अनुशंसित।",
        "it": "Regola automaticamente il budget dei token in base alla dimensione della finestra di contesto del modello. Consigliato con budget token = 0.",
        "ja": "トークン予算をモデルのコンテキストウィンドウサイズに自動調整します。トークン予算 = 0 の場合に推奨。",
        "nl": "Past het token-budget automatisch aan de contextgrootte van het model aan. Aanbevolen met token-budget = 0.",
        "no": "Justerer automatisk tokenbudsjettet til modellens kontekstvindusstørrelse. Anbefalt med tokenbudsjett = 0.",
        "pl": "Automatycznie dostosowuje budżet tokenów do rozmiaru okna kontekstowego modelu. Zalecane przy budżecie tokenów = 0.",
        "pt": "Ajusta automaticamente o orçamento de tokens ao tamanho da janela de contexto do modelo. Recomendado com orçamento de tokens = 0.",
        "sv": "Justerar automatiskt tokenbudgeten till modellens kontextfönsterstorlek. Rekommenderas med tokenbudget = 0.",
        "zh": "自动将令牌预算调整为模型的上下文窗口大小。建议在令牌预算 = 0 时使用。"
    },
    "Automatically shortens the system prompt when the context window runs low. Unused manuals are removed to save space.": {
        "cs": "Automaticky zkracuje systémový prompt, když dochází místo v kontextovém okně. Nepoužité manuály jsou odebrány pro úsporu místa.",
        "da": "Forkorter automatisk systemprompten, når kontekstvinduet er ved at være fyldt. Ubrugte manualer fjernes for at spare plads.",
        "de": "Kürzt den System-Prompt automatisch, wenn das Kontextfenster knapp wird. Unbenutzte Manuals werden entfernt, um Platz zu sparen.",
        "el": "Συντομεύει αυτόματα την προτροπή συστήματος όταν το παράθυρο περιβάλλοντος εξαντλείται. Τα αχρησιμοποίητα εγχειρίδια αφαιρούνται για εξοικονόμηση χώρου.",
        "es": "Acorta automáticamente el prompt del sistema cuando la ventana de contexto se agota. Los manuales no utilizados se eliminan para ahorrar espacio.",
        "fr": "Raccourcit automatiquement le prompt système lorsque la fenêtre de contexte diminue. Les manuels inutilisés sont supprimés pour économiser de l'espace.",
        "hi": "संदर्भ विंडो कम होने पर सिस्टम प्रॉम्प्ट को स्वचालित रूप से छोटा करता है। स्थान बचाने के लिए अप्रयुक्त मैनुअल हटा दिए जाते हैं।",
        "it": "Accorcia automaticamente il prompt di sistema quando la finestra di contesto si esaurisce. I manuali inutilizzati vengono rimossi per risparmiare spazio.",
        "ja": "コンテキストウィンドウが不足した際、システムプロンプトを自動的に短縮します。未使用のマニュアルは容量節約のため削除されます。",
        "nl": "Kort de systeemprompt automatisch in wanneer het contextvenster bijna vol is. Ongebruikte manuals worden verwijderd om ruimte te besparen.",
        "no": "Korter automatisk ned systemprompten når kontekstvinduet begynner å bli fullt. Ubrukte manualer fjernes for å spare plass.",
        "pl": "Automatycznie skraca prompt systemowy, gdy okno kontekstowe się wyczerpuje. Nieużywane podręczniki są usuwane w celu oszczędności miejsca.",
        "pt": "Encurta automaticamente o prompt do sistema quando a janela de contexto está baixa. Manuais não utilizados são removidos para economizar espaço.",
        "sv": "Kortar automatiskt ner systemprompten när kontextfönstret börjar bli fullt. Oanvända manualer tas bort för att spara utrymme.",
        "zh": "当上下文窗口不足时自动缩短系统提示。未使用的手册将被移除以节省空间。"
    },
    "Changing the embeddings model or provider can make existing embeddings incompatible. AuraGo can delete the old embeddings and rebuild them after restart.": {
        "cs": "Změna modelu nebo poskytovatele embeddings může znefunkčnit stávající embeddings. AuraGo může smazat stará embeddings a přestavět je po restartu.",
        "da": "Ændring af embeddings-model eller -udbyder kan gøre eksisterende embeddings inkompatible. AuraGo kan slette de gamle embeddings og genopbygge dem efter genstart.",
        "de": "Das Ändern des Embeddings-Modells oder -Providers kann bestehende Embeddings inkompatibel machen. AuraGo kann die alten Embeddings löschen und nach dem Neustart neu aufbauen.",
        "el": "Η αλλαγή του μοντέλου ή του παρόχου embeddings μπορεί να καταστήσει τα υπάρχοντα embeddings μη συμβατά. Το AuraGo μπορεί να διαγράψει τα παλιά embeddings και να τα ανακατασκευάσει μετά την επανεκκίνηση.",
        "es": "Cambiar el modelo o proveedor de embeddings puede hacer que los embeddings existentes sean incompatibles. AuraGo puede eliminar los embeddings antiguos y reconstruirlos tras el reinicio.",
        "fr": "Changer le modèle ou le fournisseur d'embeddings peut rendre les embeddings existants incompatibles. AuraGo peut supprimer les anciens embeddings et les reconstruire après le redémarrage.",
        "hi": "एम्बेडिंग मॉडल या प्रदाता बदलने से मौजूदा एम्बेडिंग असंगत हो सकती हैं। AuraGo पुरानी एम्बेडिंग हटा कर पुनरारंभ के बाद उन्हें पुनर्निर्माण कर सकता है।",
        "it": "La modifica del modello o del provider di embeddings può rendere incompatibili quelli esistenti. AuraGo può eliminare i vecchi embeddings e ricostruirli dopo il riavvio.",
        "ja": "埋め込みモデルまたはプロバイダーの変更により、既存の埋め込みが互換性がなくなる場合があります。AuraGoは古い埋め込みを削除し、再起動後に再構築できます。",
        "nl": "Het wijzigen van het embeddings-model of de provider kan bestaande embeddings incompatibel maken. AuraGo kan de oude embeddings verwijderen en na herstart opnieuw opbouwen.",
        "no": "Å endre embeddings-modell eller -leverandør kan gjøre eksisterende embeddings inkompatible. AuraGo kan slette gamle embeddings og bygge dem opp på nytt etter omstart.",
        "pl": "Zmiana modelu lub dostawcy embeddingów może sprawić, że istniejące embeddingi staną się niekompatybilne. AuraGo może usunąć stare embeddingi i odbudować je po ponownym uruchomieniu.",
        "pt": "Alterar o modelo ou provedor de embeddings pode tornar os embeddings existentes incompatíveis. AuraGo pode excluir os embeddings antigos e reconstruí-los após a reinicialização.",
        "sv": "Att ändra embeddings-modell eller leverantör kan göra befintliga embeddings inkompatibla. AuraGo kan radera gamla embeddings och bygga upp dem igen efter omstart.",
        "zh": "更改嵌入模型或提供商可能会使现有嵌入不兼容。AuraGo 可以删除旧的嵌入并在重启后重新构建。"
    },
    "Choose a safe rollout level instead of enabling every memory-analysis path at once.": {
        "cs": "Vyberte bezpečnou úroveň nasazení místo povolení všech cest analýzy paměti najednou.",
        "da": "Vælg et sikkert udrulningsniveau i stedet for at aktivere alle hukommelsesanalyseveje på én gang.",
        "de": "Wählen Sie eine sichere Rollout-Stufe, anstatt alle Speicheranalyse-Pfade gleichzeitig zu aktivieren.",
        "el": "Επιλέξτε ένα ασφαλές επίπεδο ανάπτυξης αντί να ενεργοποιήσετε κάθε διαδρομή ανάλυσης μνήμης ταυτόχρονα.",
        "es": "Elija un nivel de despliegue seguro en lugar de habilitar todas las rutas de análisis de memoria a la vez.",
        "fr": "Choisissez un niveau de déploiement sécurisé au lieu d'activer tous les chemins d'analyse mémoire en même temps.",
        "hi": "सभी मेमोरी-विश्लेषण पथों को एक साथ सक्षम करने के बजाय एक सुरक्षित रोलआउट स्तर चुनें।",
        "it": "Scegliere un livello di rollout sicuro invece di abilitare tutti i percorsi di analisi della memoria contemporaneamente.",
        "ja": "すべてのメモリ分析パスを一度に有効にするのではなく、安全なロールアウトレベルを選択してください。",
        "nl": "Kies een veilig uitrolniveau in plaats van alle geheugenanalysepaden tegelijk in te schakelen.",
        "no": "Velg et trygt utrullingsnivå i stedet for å aktivere alle minneanalysestier på én gang.",
        "pl": "Wybierz bezpieczny poziom wdrożenia zamiast włączać wszystkie ścieżki analizy pamięci naraz.",
        "pt": "Escolha um nível de implantação seguro em vez de ativar todos os caminhos de análise de memória de uma vez.",
        "sv": "Välj en säker distributionsnivå istället för att aktivera alla minnesanalysvägar på en gång.",
        "zh": "选择一个安全的推出级别，而不是同时启用每个内存分析路径。"
    },
    "Connection timeout in seconds. Default: 15.": {
        "cs": "Časový limit připojení v sekundách. Výchozí: 15.",
        "da": "Forbindelsestimeout i sekunder. Standard: 15.",
        "de": "Verbindungs-Timeout in Sekunden. Standard: 15.",
        "el": "Χρονικό όριο σύνδεσης σε δευτερόλεπτα. Προεπιλογή: 15.",
        "es": "Tiempo de espera de conexión en segundos. Predeterminado: 15.",
        "fr": "Délai d'attente de connexion en secondes. Par défaut : 15.",
        "hi": "सेकंड में कनेक्शन टाइमआउट। डिफ़ॉल्ट: 15।",
        "it": "Timeout di connessione in secondi. Predefinito: 15.",
        "ja": "接続タイムアウト（秒）。デフォルト：15。",
        "nl": "Verbindingstimeout in seconden. Standaard: 15.",
        "no": "Tidsavbrudd for tilkobling i sekunder. Standard: 15.",
        "pl": "Limit czasu połączenia w sekundach. Domyślnie: 15.",
        "pt": "Tempo limite de conexão em segundos. Padrão: 15.",
        "sv": "Anslutningstimeout i sekunder. Standard: 15.",
        "zh": "连接超时（秒）。默认值：15。"
    },
    "Default Quality of Service level: 0 = at most once, 1 = at least once, 2 = exactly once.": {
        "cs": "Výchozí úroveň Quality of Service: 0 = nejvýše jednou, 1 = alespoň jednou, 2 = přesně jednou.",
        "da": "Standard Quality of Service-niveau: 0 = højst én gang, 1 = mindst én gang, 2 = præcis én gang.",
        "de": "Standard-QoS-Stufe: 0 = höchstens einmal, 1 = mindestens einmal, 2 = genau einmal.",
        "el": "Προεπιβεβλημένο επίπεδο Ποιότητας Εξυπηρέτησης: 0 = το πολύ μία φορά, 1 = τουλάχιστον μία φορά, 2 = ακριβώς μία φορά.",
        "es": "Nivel de Calidad de Servicio predeterminado: 0 = como máximo una vez, 1 = al menos una vez, 2 = exactamente una vez.",
        "fr": "Niveau de Qualité de Service par défaut : 0 = au plus une fois, 1 = au moins une fois, 2 = exactement une fois.",
        "hi": "डिफ़ॉल्ट सेवा की गुणवत्ता स्तर: 0 = अधिकतम एक बार, 1 = कम से कम एक बार, 2 = ठीक एक बार।",
        "it": "Livello Quality of Service predefinito: 0 = al massimo una volta, 1 = almeno una volta, 2 = esattamente una volta.",
        "ja": "デフォルトのQoSレベル：0 = 最大1回、1 = 少なくとも1回、2 = 正確に1回。",
        "nl": "Standaard Quality of Service-niveau: 0 = maximaal één keer, 1 = minimaal één keer, 2 = exact één keer.",
        "no": "Standard Quality of Service-nivå: 0 = maks én gang, 1 = minst én gang, 2 = nøyaktig én gang.",
        "pl": "Domyślny poziom Quality of Service: 0 = co najwyżej raz, 1 = co najmniej raz, 2 = dokładnie raz.",
        "pt": "Nível padrão de Qualidade de Serviço: 0 = no máximo uma vez, 1 = pelo menos uma vez, 2 = exatamente uma vez.",
        "sv": "Standard Quality of Service-nivå: 0 = högst en gång, 1 = minst en gång, 2 = exakt en gång.",
        "zh": "默认服务质量级别：0 = 最多一次，1 = 至少一次，2 = 恰好一次。"
    },
    "Default bucket name. Can also be specified per API call.": {
        "cs": "Výchozí název bucketu. Lze také zadat pro každé volání API zvlášť.",
        "da": "Standard bucket-navn. Kan også angives pr. API-kald.",
        "de": "Standard-Bucket-Name. Kann auch pro API-Aufruf angegeben werden.",
        "el": "Προεπιλεγμένο όνομα κάδου. Μπορεί επίσης να καθοριστεί ανά κλήση API.",
        "es": "Nombre de bucket predeterminado. También se puede especificar por llamada a la API.",
        "fr": "Nom de bucket par défaut. Peut également être spécifié par appel API.",
        "hi": "डिफ़ॉल्ट बकेट नाम। प्रति API कॉल भी निर्दिष्ट किया जा सकता है।",
        "it": "Nome bucket predefinito. Può anche essere specificato per ogni chiamata API.",
        "ja": "デフォルトのバケット名。API呼び出しごとに指定することもできます。",
        "nl": "Standaard bucketnaam. Kan ook per API-aanroep worden opgegeven.",
        "no": "Standard bucket-navn. Kan også angis per API-kall.",
        "pl": "Domyślna nazwa bucketu. Można ją również określić dla każdego wywołania API.",
        "pt": "Nome do bucket padrão. Também pode ser especificado por chamada de API.",
        "sv": "Standard bucket-namn. Kan också anges per API-anrop.",
        "zh": "默认存储桶名称。也可以在每次 API 调用时指定。"
    },
    "Default channel to listen on, e.g. 'general'.": {
        "cs": "Výchozí kanál pro poslech, např. 'general'.",
        "da": "Standardkanal at lytte på, f.eks. 'general'.",
        "de": "Standard-Kanal zum Abhören, z.B. 'general'.",
        "el": "Προεπιλεγμένο κανάλι ακρόασης, π.χ. 'general'.",
        "es": "Canal predeterminado para escuchar, p.ej. 'general'.",
        "fr": "Canal d'écoute par défaut, p.ex. 'general'.",
        "hi": "सुनने के लिए डिफ़ॉल्ट चैनल, उदा. 'general'।",
        "it": "Canale predefinito da ascoltare, es. 'general'.",
        "ja": "リッスンするデフォルトチャンネル（例：'general'）。",
        "nl": "Standaardkanaal om naar te luisteren, bijv. 'general'.",
        "no": "Standardkanal å lytte på, f.eks. 'general'.",
        "pl": "Domyślny kanał do nasłuchiwania, np. 'general'.",
        "pt": "Canal padrão para ouvir, p.ex. 'general'.",
        "sv": "Standardkanal att lyssna på, t.ex. 'general'.",
        "zh": "默认监听频道，例如 'general'。"
    },
    "Default node name, e.g. 'pve'. Used when no node is specified.": {
        "cs": "Výchozí název uzlu, např. 'pve'. Používá se, když není uzel zadán.",
        "da": "Standard nodenavn, f.eks. 'pve'. Bruges når ingen node er angivet.",
        "de": "Standard-Node-Name, z.B. 'pve'. Wird verwendet, wenn kein Node angegeben ist.",
        "el": "Προεπιλεγμένο όνομα κόμβου, π.χ. 'pve'. Χρησιμοποιείται όταν δεν καθορίζεται κόμβος.",
        "es": "Nombre de nodo predeterminado, p.ej. 'pve'. Se usa cuando no se especifica un nodo.",
        "fr": "Nom de nœud par défaut, p.ex. 'pve'. Utilisé lorsqu'aucun nœud n'est spécifié.",
        "hi": "डिफ़ॉल्ट नोड नाम, उदा. 'pve'। जब कोई नोड निर्दिष्ट नहीं होता है तब उपयोग किया जाता है।",
        "it": "Nome nodo predefinito, es. 'pve'. Utilizzato quando nessun nodo è specificato.",
        "ja": "デフォルトのノード名（例：'pve'）。ノードが指定されていない場合に使用されます。",
        "nl": "Standaard nodenaam, bijv. 'pve'. Wordt gebruikt wanneer geen node is opgegeven.",
        "no": "Standard nodenavn, f.eks. 'pve'. Brukes når ingen node er angitt.",
        "pl": "Domyślna nazwa węzła, np. 'pve'. Używana, gdy nie określono węzła.",
        "pt": "Nome do nó padrão, p.ex. 'pve'. Usado quando nenhum nó é especificado.",
        "sv": "Standard nodnamn, t.ex. 'pve'. Används när ingen nod har angetts.",
        "zh": "默认节点名称，例如 'pve'。在未指定节点时使用。"
    },
    "Disable TLS certificate verification. Enable for self-signed certificates.": {
        "cs": "Zakázat ověřování TLS certifikátů. Povolte pro certifikáty podepsané vlastní autoritou.",
        "da": "Deaktiver TLS-certifikatbekræftelse. Aktiver for selvsignerede certifikater.",
        "de": "TLS-Zertifikatsüberprüfung deaktivieren. Für selbstsignierte Zertifikate aktivieren.",
        "el": "Απενεργοποίηση επαλήθευσης πιστοποιητικού TLS. Ενεργοποιήστε για αυτο-υπογεγραμμένα πιστοποιητικά.",
        "es": "Desactivar la verificación de certificados TLS. Activar para certificados autofirmados.",
        "fr": "Désactiver la vérification des certificats TLS. Activer pour les certificats auto-signés.",
        "hi": "TLS प्रमाणपत्र सत्यापन अक्षम करें। स्व-हस्ताक्षरित प्रमाणपत्रों के लिए सक्षम करें।",
        "it": "Disabilita la verifica dei certificati TLS. Abilita per certificati autofirmati.",
        "ja": "TLS証明書の検証を無効化。自己署名証明書の場合に有効にしてください。",
        "nl": "TLS-certificaatverificatie uitschakelen. Inschakelen voor zelfondertekende certificaten.",
        "no": "Deaktiver TLS-sertifikatbekreftelse. Aktiver for selvsignerte sertifikater.",
        "pl": "Wyłącz weryfikację certyfikatów TLS. Włącz dla certyfikatów z podpisem własnym.",
        "pt": "Desativar verificação de certificado TLS. Ative para certificados autoassinados.",
        "sv": "Inaktivera TLS-certifikatverifiering. Aktivera för självsignerade certifikat.",
        "zh": "禁用 TLS 证书验证。自签名证书请启用此选项。"
    },
    "Display name of the bot in Rocket.Chat. Default: 'AuraGo'.": {
        "cs": "Zobrazovaný název bota v Rocket.Chat. Výchozí: 'AuraGo'.",
        "da": "Botens visningsnavn i Rocket.Chat. Standard: 'AuraGo'.",
        "de": "Anzeigename des Bots in Rocket.Chat. Standard: 'AuraGo'.",
        "el": "Εμφανιζόμενο όνομα του bot στο Rocket.Chat. Προεπιλογή: 'AuraGo'.",
        "es": "Nombre visible del bot en Rocket.Chat. Predeterminado: 'AuraGo'.",
        "fr": "Nom d'affichage du bot dans Rocket.Chat. Par défaut : 'AuraGo'.",
        "hi": "Rocket.Chat में बॉट का प्रदर्शन नाम। डिफ़ॉल्ट: 'AuraGo'।",
        "it": "Nome visualizzato del bot in Rocket.Chat. Predefinito: 'AuraGo'.",
        "ja": "Rocket.Chatでのボット表示名。デフォルト：'AuraGo'。",
        "nl": "Weergavenaam van de bot in Rocket.Chat. Standaard: 'AuraGo'.",
        "no": "Visningsnavn for boten i Rocket.Chat. Standard: 'AuraGo'.",
        "pl": "Wyświetlana nazwa bota w Rocket.Chat. Domyślnie: 'AuraGo'.",
        "pt": "Nome de exibição do bot no Rocket.Chat. Padrão: 'AuraGo'.",
        "sv": "Visningsnamn för boten i Rocket.Chat. Standard: 'AuraGo'.",
        "zh": "Rocket.Chat 中机器人的显示名称。默认值：'AuraGo'。"
    },
    "Display name of the email account.": {
        "cs": "Zobrazovaný název e-mailového účtu.",
        "da": "Visningsnavn for e-mailkontoen.",
        "de": "Anzeigename des E-Mail-Kontos.",
        "el": "Εμφανιζόμενο όνομα του λογαριασμού email.",
        "es": "Nombre visible de la cuenta de correo electrónico.",
        "fr": "Nom d'affichage du compte e-mail.",
        "hi": "ईमेल खाते का प्रदर्शन नाम।",
        "it": "Nome visualizzato dell'account email.",
        "ja": "メールアカウントの表示名。",
        "nl": "Weergavenaam van het e-mailaccount.",
        "no": "Visningsnavn for e-postkontoen.",
        "pl": "Wyświetlana nazwa konta e-mail.",
        "pt": "Nome de exibição da conta de e-mail.",
        "sv": "Visningsnamn för e-postkontot.",
        "zh": "电子邮件账户的显示名称。"
    },
    "Enable Cloudflare AI Gateway to proxy all LLM API requests. Provides caching, rate-limiting, logging, and analytics. Local providers (Ollama) are excluded.": {
        "cs": "Povolit Cloudflare AI Gateway pro proxy všech LLM API požadavků. Poskytuje mezipaměť, omezování rychlosti, protokolování a analytiku. Místní poskytovatelé (Ollama) jsou vyloučeni.",
        "da": "Aktiver Cloudflare AI Gateway til at proxy alle LLM API-anmodninger. Tilbyder caching, rate-limiting, logning og analytics. Lokale udbydere (Ollama) er ekskluderet.",
        "de": "Cloudflare AI Gateway aktivieren, um alle LLM-API-Anfragen zu proxen. Bietet Caching, Rate-Limiting, Logging und Analysen. Lokale Anbieter (Ollama) sind ausgeschlossen.",
        "el": "Ενεργοποιήστε το Cloudflare AI Gateway για να διαμεσολαβήσετε σε όλα τα LLM API αιτήματα. Παρέχει προσωρινή αποθήκευση, περιορισμό ρυθμού, καταγραφή και αναλυτικά στοιχεία. Οι τοπικοί πάροχοι (Ollama) εξαιρούνται.",
        "es": "Activar Cloudflare AI Gateway para proxificar todas las solicitudes LLM API. Proporciona caché, limitación de tasa, registro y analíticas. Los proveedores locales (Ollama) están excluidos.",
        "fr": "Activer Cloudflare AI Gateway pour proxyer toutes les requêtes LLM API. Fournit un cache, une limitation de débit, la journalisation et des analyses. Les fournisseurs locaux (Ollama) sont exclus.",
        "hi": "सभी LLM API अनुरोधों को प्रॉक्सी करने के लिए Cloudflare AI Gateway सक्षम करें। कैशिंग, रेट-लिमिटिंग, लॉगिंग और एनालिटिक्स प्रदान करता है। स्थानीय प्रदाता (Ollama) बहिष्कृत हैं।",
        "it": "Abilita Cloudflare AI Gateway per proxyare tutte le richieste LLM API. Fornisce cache, rate-limiting, logging e analisi. I provider locali (Ollama) sono esclusi.",
        "ja": "Cloudflare AI Gatewayを有効にしてすべてのLLM APIリクエストをプロキシします。キャッシュ、レート制限、ログ、分析を提供します。ローカルプロバイダー（Ollama）は除外されます。",
        "nl": "Schakel Cloudflare AI Gateway in om alle LLM API-verzoeken te proxien. Biedt caching, rate-limiting, logging en analytics. Lokale providers (Ollama) zijn uitgesloten.",
        "no": "Aktiver Cloudflare AI Gateway for å proxye alle LLM API-forespørsler. Gir caching, rate-begrensning, logging og analyse. Lokale leverandører (Ollama) er ekskludert.",
        "pl": "Włącz Cloudflare AI Gateway do proxy wszystkich żądań LLM API. Zapewnia buforowanie, ograniczanie szybkości, logowanie i analitykę. Lokalni dostawcy (Ollama) są wykluczeni.",
        "pt": "Ativar Cloudflare AI Gateway para fazer proxy de todas as solicitações LLM API. Fornece cache, limitação de taxa, registro e análises. Provedores locais (Ollama) são excluídos.",
        "sv": "Aktivera Cloudflare AI Gateway för att proxia alla LLM API-förfrågningar. Ger caching, hastighetsbegränsning, loggning och analys. Lokala leverantörer (Ollama) är undantagna.",
        "zh": "启用 Cloudflare AI Gateway 代理所有 LLM API 请求。提供缓存、速率限制、日志和分析。本地提供商（Ollama）除外。"
    },
    "Enable TLS/SSL encryption for the MQTT connection.": {
        "cs": "Povolit TLS/SSL šifrování pro MQTT připojení.",
        "da": "Aktiver TLS/SSL-kryptering for MQTT-forbindelsen.",
        "de": "TLS/SSL-Verschlüsselung für die MQTT-Verbindung aktivieren.",
        "el": "Ενεργοποίηση κρυπτογράφησης TLS/SSL για τη σύνδεση MQTT.",
        "es": "Activar cifrado TLS/SSL para la conexión MQTT.",
        "fr": "Activer le chiffrement TLS/SSL pour la connexion MQTT.",
        "hi": "MQTT कनेक्शन के लिए TLS/SSL एन्क्रिप्शन सक्षम करें।",
        "it": "Abilita la crittografia TLS/SSL per la connessione MQTT.",
        "ja": "MQTT接続のTLS/SSL暗号化を有効にします。",
        "nl": "TLS/SSL-versleuteling inschakelen voor de MQTT-verbinding.",
        "no": "Aktiver TLS/SSL-kryptering for MQTT-tilkoblingen.",
        "pl": "Włącz szyfrowanie TLS/SSL dla połączenia MQTT.",
        "pt": "Ativar criptografia TLS/SSL para a conexão MQTT.",
        "sv": "Aktivera TLS/SSL-kryptering för MQTT-anslutningen.",
        "zh": "为 MQTT 连接启用 TLS/SSL 加密。"
    },
    "Enable it to let the agent analyze conversations and extract memory-worthy content. AuraGo uses the central Helper LLM for the batched helper path when it is available.": {
        "cs": "Povolte, aby agent mohl analyzovat konverzace a extrahovat obsah hodný paměti. AuraGo používá centrální Helper LLM pro dávkovou pomocnou cestu, když je k dispozici.",
        "da": "Aktiver for at lade agenten analysere samtaler og udtrække hukommelsesværdigt indhold. AuraGo bruger den centrale Helper LLM til den batchede hjælpersti, når den er tilgængelig.",
        "de": "Aktivieren, damit der Agent Gespräche analysieren und speicherwürdige Inhalte extrahieren kann. AuraGo nutzt das zentrale Helper-LLM für den gebatchten Hilfspfad, wenn verfügbar.",
        "el": "Ενεργοποιήστε το για να επιτρέψετε στον πράκτορα να αναλύει συνομιλίες και να εξάγει περιεχόμενο αξίας μνήμης. Το AuraGo χρησιμοποιεί το κεντρικό Helper LLM για τη δέσμη βοηθητικής διαδρομής όταν είναι διαθέσιμο.",
        "es": "Actívelo para que el agente analice conversaciones y extraiga contenido digno de memoria. AuraGo usa el LLM Helper central para la ruta de ayuda por lotes cuando está disponible.",
        "fr": "Activez-le pour permettre à l'agent d'analyser les conversations et d'extraire le contenu digne de mémoire. AuraGo utilise le LLM Helper centralisé pour le chemin d'assistance par lots lorsqu'il est disponible.",
        "hi": "एजेंट को वार्तालापों का विश्लेषण करने और मेमोरी-योग्य सामग्री निकालने की अनुमति देने के लिए सक्षम करें। AuraGo बैच किए गए सहायक पथ के लिए केंद्रीय सहायक LLM का उपयोग करता है जब यह उपलब्ध हो।",
        "it": "Abilita per consentire all'agente di analizzare le conversazioni ed estrarre contenuti degni di memoria. AuraGo utilizza l'LLM Helper centrale per il percorso helper in batch quando disponibile.",
        "ja": "エージェントが会話を分析し、記憶に値するコンテンツを抽出できるように有効にします。AuraGoは利用可能な場合、バッチ処理ヘルパーパスに中央Helper LLMを使用します。",
        "nl": "Schakel in om de agent gesprekken te laten analyseren en geheugenwaardige inhoud te extraheren. AuraGo gebruikt de centrale Helper LLM voor het gebatchte hulppad wanneer beschikbaar.",
        "no": "Aktiver for å la agenten analysere samtaler og trekke ut minneverdig innhold. AuraGo bruker den sentrale Helper LLM for den satsvise hjelpestien når den er tilgjengelig.",
        "pl": "Włącz, aby agent mógł analizować rozmowy i wyodrębniać treści warte zapamiętania. AuraGo używa centralnego Helper LLM dla ścieżki wsadowej pomocnika, gdy jest dostępny.",
        "pt": "Ative para permitir que o agente analise conversas e extraia conteúdo digno de memória. AuraGo usa o LLM Helper central para o caminho de ajuda em lote quando disponível.",
        "sv": "Aktivera för att låta agenten analysera samtal och extrahera minnesvärt innehåll. AuraGo använder den centrala Helper LLM för den batchade hjälpsökvägen när den är tillgänglig.",
        "zh": "启用此选项以允许代理分析对话并提取值得记忆的内容。AuraGo 在可用时使用中央 Helper LLM 进行批处理辅助路径。"
    },
    "Enable path-style addressing (required for MinIO and most S3-compatible services).": {
        "cs": "Povolit adresování ve stylu cesty (vyžadováno pro MinIO a většinu S3-kompatibilních služeb).",
        "da": "Aktiver sti-stil adressering (kræves for MinIO og de fleste S3-kompatible tjenester).",
        "de": "Pfadstil-Adressierung aktivieren (erforderlich für MinIO und die meisten S3-kompatiblen Dienste).",
        "el": "Ενεργοποίηση διευθυνσιοδότησης στυλ διαδρομής (απαιτείται για MinIO και τις περισσότερες S3-συμβατές υπηρεσίες).",
        "es": "Activar direccionamiento estilo ruta (necesario para MinIO y la mayoría de servicios compatibles con S3).",
        "fr": "Activer l'adressage de style chemin (requis pour MinIO et la plupart des services compatibles S3).",
        "hi": "पाथ-स्टाइल एड्रेसिंग सक्षम करें (MinIO और अधिकांश S3-संगत सेवाओं के लिए आवश्यक)।",
        "it": "Abilita indirizzamento in stile percorso (richiesto per MinIO e la maggior parte dei servizi compatibili S3).",
        "ja": "パススタイルのアドレス指定を有効にします（MinIOおよびほとんどのS3互換サービスで必要）。",
        "nl": "Pad-stijl adressering inschakelen (vereist voor MinIO en de meeste S3-compatibele services).",
        "no": "Aktiver sti-stil adressering (kreves for MinIO og de fleste S3-kompatible tjenester).",
        "pl": "Włącz adresowanie w stylu ścieżki (wymagane dla MinIO i większości usług zgodnych z S3).",
        "pt": "Ativar endereçamento estilo caminho (necessário para MinIO e a maioria dos serviços compatíveis com S3).",
        "sv": "Aktivera sökvägsstil-adressering (krävs för MinIO och de flesta S3-kompatibla tjänster).",
        "zh": "启用路径样式寻址（MinIO 和大多数 S3 兼容服务需要）。"
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

print(f"Batch 17: Fixed {fixed} keys")

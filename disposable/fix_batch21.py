#!/usr/bin/env python3
"""Batch 21: Remaining config descriptions + UI labels."""
import json
from pathlib import Path

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "Access Key ID for S3 authentication. Stored encrypted in the vault.": {
        "cs": "Access Key ID pro S3 autentizaci. Uloženo šifrovaně v trezoru.",
        "da": "Access Key ID til S3-godkendelse. Krypteret i boksen.",
        "de": "Access Key ID für S3-Authentifizierung. Verschlüsselt im Tresor gespeichert.",
        "el": "Αναγνωριστικό κλειδιού πρόσβασης για αυθεντικοποίηση S3. Κρυπτογραφημένο στο θησαυροφυλάκιο.",
        "es": "Access Key ID para autenticación S3. Almacenado cifrado en la bóveda.",
        "fr": "ID de clé d'accès pour l'authentification S3. Stocké chiffré dans le coffre.",
        "hi": "S3 प्रमाणीकरण के लिए Access Key ID। वॉल्ट में एन्क्रिप्टेड संग्रहीत।",
        "it": "Access Key ID per l'autenticazione S3. Archiviato crittografato nel caveau.",
        "ja": "S3認証用のAccess Key ID。暗号化されたボールトに保存されます。",
        "nl": "Access Key ID voor S3-authenticatie. Versleuteld opgeslagen in de kluis.",
        "no": "Access Key ID for S3-autentisering. Kryptert lagret i hvelvet.",
        "pl": "Access Key ID do uwierzytelnienia S3. Zaszyfrowane w sejfie.",
        "pt": "Access Key ID para autenticação S3. Armazenado criptografado no cofre.",
        "sv": "Access Key ID för S3-autentisering. Krypterat lagrat i valvet.",
        "zh": "用于 S3 认证的 Access Key ID。加密存储在保管库中。"
    },
    "API token for the Paperless-ngx instance. Stored in the vault.": {
        "cs": "API token pro instanci Paperless-ngx. Uloženo v trezoru.",
        "da": "API-token til Paperless-ngx-instansen. Opbevares i boksen.",
        "de": "API-Token für die Paperless-ngx-Instanz. Im Tresor gespeichert.",
        "el": "Διακριτικό API για την παρουσία Paperless-ngx. Αποθηκευμένο στο θησαυροφυλάκιο.",
        "es": "Token API para la instancia de Paperless-ngx. Almacenado en la bóveda.",
        "fr": "Jeton API pour l'instance Paperless-ngx. Stocké dans le coffre.",
        "hi": "Paperless-ngx इंस्टेंस के लिए API टोकन। वॉल्ट में संग्रहीत।",
        "it": "Token API per l'istanza Paperless-ngx. Archiviato nel caveau.",
        "ja": "Paperless-ngxインスタンスのAPIトークン。ボールトに保存されます。",
        "nl": "API-token voor de Paperless-ngx-instantie. Opgeslagen in de kluis.",
        "no": "API-token for Paperless-ngx-instansen. Lagret i hvelvet.",
        "pl": "Token API dla instancji Paperless-ngx. Przechowywany w sejfie.",
        "pt": "Token API para a instância Paperless-ngx. Armazenado no cofre.",
        "sv": "API-token för Paperless-ngx-instansen. Lagrat i valvet.",
        "zh": "Paperless-ngx 实例的 API 令牌。存储在保管库中。"
    },
    "AWS region (e.g. us-east-1). Required for AWS S3, optional for MinIO.": {
        "cs": "AWS region (např. us-east-1). Vyžadováno pro AWS S3, volitelné pro MinIO.",
        "da": "AWS-region (f.eks. us-east-1). Kræves for AWS S3, valgfrit for MinIO.",
        "de": "AWS-Region (z.B. us-east-1). Erforderlich für AWS S3, optional für MinIO.",
        "el": "Περιοχή AWS (π.χ. us-east-1). Απαιτείται για AWS S3, προαιρετικό για MinIO.",
        "es": "Región de AWS (p.ej. us-east-1). Obligatorio para AWS S3, opcional para MinIO.",
        "fr": "Région AWS (p.ex. us-east-1). Requis pour AWS S3, optionnel pour MinIO.",
        "hi": "AWS क्षेत्र (उदा. us-east-1)। AWS S3 के लिए आवश्यक, MinIO के लिए वैकल्पिक।",
        "it": "Regione AWS (es. us-east-1). Richiesta per AWS S3, opzionale per MinIO.",
        "ja": "AWSリージョン（例：us-east-1）。AWS S3では必須、MinIOではオプション。",
        "nl": "AWS-regio (bijv. us-east-1). Vereist voor AWS S3, optioneel voor MinIO.",
        "no": "AWS-region (f.eks. us-east-1). Påkrevd for AWS S3, valgfritt for MinIO.",
        "pl": "Region AWS (np. us-east-1). Wymagany dla AWS S3, opcjonalny dla MinIO.",
        "pt": "Região AWS (p.ex. us-east-1). Obrigatório para AWS S3, opcional para MinIO.",
        "sv": "AWS-region (t.ex. us-east-1). Krävs för AWS S3, valfritt för MinIO.",
        "zh": "AWS 区域（例如 us-east-1）。AWS S3 必需，MinIO 可选。"
    },
    "Secret Access Key for S3 authentication. Stored encrypted in the vault.": {
        "cs": "Secret Access Key pro S3 autentizaci. Uloženo šifrovaně v trezoru.",
        "da": "Secret Access Key til S3-godkendelse. Krypteret i boksen.",
        "de": "Secret Access Key für S3-Authentifizierung. Verschlüsselt im Tresor gespeichert.",
        "el": "Μυστικό κλειδί πρόσβασης για αυθεντικοποίηση S3. Κρυπτογραφημένο στο θησαυροφυλάκιο.",
        "es": "Clave de acceso secreta para autenticación S3. Almacenada cifrada en la bóveda.",
        "fr": "Clé d'accès secrète pour l'authentification S3. Stockée chiffrée dans le coffre.",
        "hi": "S3 प्रमाणीकरण के लिए Secret Access Key। वॉल्ट में एन्क्रिप्टेड संग्रहीत।",
        "it": "Secret Access Key per l'autenticazione S3. Archiviata crittografata nel caveau.",
        "ja": "S3認証用のSecret Access Key。暗号化されたボールトに保存されます。",
        "nl": "Secret Access Key voor S3-authenticatie. Versleuteld opgeslagen in de kluis.",
        "no": "Secret Access Key for S3-autentisering. Kryptert lagret i hvelvet.",
        "pl": "Secret Access Key do uwierzytelnienia S3. Zaszyfrowane w sejfie.",
        "pt": "Chave de acesso secreta para autenticação S3. Armazenada criptografada no cofre.",
        "sv": "Secret Access Key för S3-autentisering. Krypterat lagrat i valvet.",
        "zh": "用于 S3 认证的 Secret Access Key。加密存储在保管库中。"
    },
    "Cloudflare Account ID for Workers AI providers. Required when using the workers-ai provider type.": {
        "cs": "Cloudflare Account ID pro poskytovatele Workers AI. Vyžadováno při použití typu poskytovatele workers-ai.",
        "da": "Cloudflare Account ID til Workers AI-udbydere. Kræves ved brug af workers-ai-udbydertypen.",
        "de": "Cloudflare Account-ID für Workers AI-Anbieter. Erforderlich bei Verwendung des Anbieter-Typs workers-ai.",
        "el": "Αναγνωριστικό λογαριασμού Cloudflare για παρόχους Workers AI. Απαιτείται κατά τη χρήση του τύπου παρόχου workers-ai.",
        "es": "ID de cuenta de Cloudflare para proveedores de Workers AI. Necesario al usar el tipo de proveedor workers-ai.",
        "fr": "ID de compte Cloudflare pour les fournisseurs Workers AI. Requis lors de l'utilisation du type de fournisseur workers-ai.",
        "hi": "Workers AI प्रदाताओं के लिए Cloudflare Account ID। workers-ai प्रदाता प्रकार का उपयोग करते समय आवश्यक।",
        "it": "Cloudflare Account ID per i provider Workers AI. Richiesto quando si utilizza il tipo di provider workers-ai.",
        "ja": "Workers AIプロバイダー用のCloudflare Account ID。workers-aiプロバイダータイプ使用時に必須。",
        "nl": "Cloudflare Account ID voor Workers AI-providers. Vereist bij gebruik van het workers-ai-providertype.",
        "no": "Cloudflare Account ID for Workers AI-leverandører. Påkrevd ved bruk av workers-ai-leverandørtypen.",
        "pl": "Cloudflare Account ID dla dostawców Workers AI. Wymagane przy użyciu typu dostawcy workers-ai.",
        "pt": "Cloudflare Account ID para provedores Workers AI. Obrigatório ao usar o tipo de provedor workers-ai.",
        "sv": "Cloudflare Account ID för Workers AI-leverantörer. Krävs när leverantörstypen workers-ai används.",
        "zh": "Workers AI 提供商的 Cloudflare Account ID。使用 workers-ai 提供商类型时必需。"
    },
    "Your Cloudflare Account ID. Found in the Cloudflare dashboard under Workers & Pages.": {
        "cs": "Vaše Cloudflare Account ID. Naleznete v Cloudflare dashboardu pod Workers & Pages.",
        "da": "Dit Cloudflare Account ID. Findes i Cloudflare-dashboardet under Workers & Pages.",
        "de": "Ihre Cloudflare Account-ID. Im Cloudflare-Dashboard unter Workers & Pages zu finden.",
        "el": "Το αναγνωριστικό λογαριασμού σας στο Cloudflare. Βρίσκεται στο ταμπλό Cloudflare υπό Workers & Pages.",
        "es": "Su ID de cuenta de Cloudflare. Se encuentra en el panel de Cloudflare en Workers & Pages.",
        "fr": "Votre ID de compte Cloudflare. Disponible dans le tableau de bord Cloudflare sous Workers & Pages.",
        "hi": "आपका Cloudflare Account ID। Cloudflare डैशबोर्ड में Workers & Pages के अंतर्गत मिलेगा।",
        "it": "Il tuo Cloudflare Account ID. Lo trovi nella dashboard Cloudflare sotto Workers & Pages.",
        "ja": "Cloudflare Account ID。CloudflareダッシュボードのWorkers & Pagesにあります。",
        "nl": "Uw Cloudflare Account ID. Te vinden in het Cloudflare-dashboard onder Workers & Pages.",
        "no": "Ditt Cloudflare Account ID. Finnes i Cloudflare-dashbordet under Workers & Pages.",
        "pl": "Twoje Cloudflare Account ID. Znajdziesz je w panelu Cloudflare w sekcji Workers & Pages.",
        "pt": "Seu Cloudflare Account ID. Encontrado no painel do Cloudflare em Workers & Pages.",
        "sv": "Ditt Cloudflare Account ID. Hittas i Cloudflare-instrumentpanelen under Workers & Pages.",
        "zh": "您的 Cloudflare Account ID。可在 Cloudflare 仪表板的 Workers & Pages 下找到。"
    },
    "The name or slug of your AI Gateway instance in Cloudflare.": {
        "cs": "Název nebo slug vaší instance AI Gateway v Cloudflare.",
        "da": "Navnet eller slug for din AI Gateway-instans i Cloudflare.",
        "de": "Der Name oder Slug Ihrer AI-Gateway-Instanz in Cloudflare.",
        "el": "Το όνομα ή το slug της παρουσίας AI Gateway στο Cloudflare.",
        "es": "El nombre o slug de su instancia de AI Gateway en Cloudflare.",
        "fr": "Le nom ou le slug de votre instance AI Gateway dans Cloudflare.",
        "hi": "Cloudflare में आपके AI Gateway इंस्टेंस का नाम या स्लग।",
        "it": "Il nome o lo slug della tua istanza AI Gateway in Cloudflare.",
        "ja": "Cloudflare内のAI Gatewayインスタンスの名前またはスラッグ。",
        "nl": "De naam of slug van uw AI Gateway-instantie in Cloudflare.",
        "no": "Navnet eller slug for din AI Gateway-instans i Cloudflare.",
        "pl": "Nazwa lub slug Twojej instancji AI Gateway w Cloudflare.",
        "pt": "O nome ou slug da sua instância AI Gateway no Cloudflare.",
        "sv": "Namnet eller slug för din AI Gateway-instans i Cloudflare.",
        "zh": "您在 Cloudflare 中的 AI Gateway 实例的名称或 slug。"
    },
    "Comma-separated list of topics to subscribe to on startup (e.g. 'home/#, sensors/temperature').": {
        "cs": "Seznam témat k odběru při spuštění oddělený čárkami (např. 'home/#, sensors/temperature').",
        "da": "Kommasepareret liste over emner at abonnere på ved opstart (f.eks. 'home/#, sensors/temperature').",
        "de": "Durch Kommas getrennte Liste von Topics, die beim Start abonniert werden (z.B. 'home/#, sensors/temperature').",
        "el": "Λίστα θεμάτων με κόμμα για εγγραφή κατά την εκκίνηση (π.χ. 'home/#, sensors/temperature').",
        "es": "Lista separada por comas de temas a suscribirse al inicio (p.ej. 'home/#, sensors/temperature').",
        "fr": "Liste de sujets séparés par des virgules auxquels s'abonner au démarrage (p.ex. 'home/#, sensors/temperature').",
        "hi": "स्टार्टअप पर सब्सक्राइब करने के लिए अल्पविराम से अलग विषयों की सूची (उदा. 'home/#, sensors/temperature')।",
        "it": "Elenco separato da virgole di argomenti a cui iscriversi all'avvio (es. 'home/#, sensors/temperature').",
        "ja": "起動時にサブスクライブするトピックのカンマ区切りリスト（例：'home/#, sensors/temperature'）。",
        "nl": "Kommagescheiden lijst van topics om op te abonneren bij opstarten (bijv. 'home/#, sensors/temperature').",
        "no": "Kommaseparert liste med emner å abonnere på ved oppstart (f.eks. 'home/#, sensors/temperature').",
        "pl": "Rozdzielona przecinkami lista tematów do subskrypcji przy uruchomieniu (np. 'home/#, sensors/temperature').",
        "pt": "Lista separada por vírgulas de tópicos para assinar na inicialização (p.ex. 'home/#, sensors/temperature').",
        "sv": "Kommaseparerad lista med ämnen att prenumerera på vid uppstart (t.ex. 'home/#, sensors/temperature').",
        "zh": "启动时订阅的主题逗号分隔列表（例如 'home/#, sensors/temperature'）。"
    },
    "Container backend: 'docker' (default) or 'podman'. Docker must be installed and reachable.": {
        "cs": "Backend kontejnerů: 'docker' (výchozí) nebo 'podman'. Docker musí být nainstalován a dostupný.",
        "da": "Container-backend: 'docker' (standard) eller 'podman'. Docker skal være installeret og tilgængelig.",
        "de": "Container-Backend: 'docker' (Standard) oder 'podman'. Docker muss installiert und erreichbar sein.",
        "el": "Backend κοντέινερ: 'docker' (προεπιλογή) ή 'podman'. Το Docker πρέπει να είναι εγκατεστημένο και προσβάσιμο.",
        "es": "Backend de contenedores: 'docker' (predeterminado) o 'podman'. Docker debe estar instalado y accesible.",
        "fr": "Backend de conteneurs : 'docker' (par défaut) ou 'podman'. Docker doit être installé et accessible.",
        "hi": "कंटेनर बैकएंड: 'docker' (डिफ़ॉल्ट) या 'podman'। Docker इंस्टॉल और पहुँच योग्य होना चाहिए।",
        "it": "Backend container: 'docker' (predefinito) o 'podman'. Docker deve essere installato e raggiungibile.",
        "ja": "コンテナバックエンド：'docker'（デフォルト）または'podman'。Dockerがインストールされアクセス可能である必要があります。",
        "nl": "Container-backend: 'docker' (standaard) of 'podman'. Docker moet geïnstalleerd en bereikbaar zijn.",
        "no": "Container-backend: 'docker' (standard) eller 'podman'. Docker må være installert og tilgjengelig.",
        "pl": "Backend kontenerów: 'docker' (domyślnie) lub 'podman'. Docker musi być zainstalowany i dostępny.",
        "pt": "Backend de contêineres: 'docker' (padrão) ou 'podman'. Docker deve estar instalado e acessível.",
        "sv": "Container-backend: 'docker' (standard) eller 'podman'. Docker måste vara installerat och nåbart.",
        "zh": "容器后端：'docker'（默认）或 'podman'。Docker 必须已安装且可访问。"
    },
    "Automatically install the llm-sandbox Python package (with MCP extras) on startup. Installs 'llm-sandbox[mcp-docker]' or 'llm-sandbox[mcp-podman]' depending on the backend. Requires a working Python/pip environment.": {
        "cs": "Automaticky nainstalovat Python balíček llm-sandbox (s MCP extras) při spuštění. Nainstaluje 'llm-sandbox[mcp-docker]' nebo 'llm-sandbox[mcp-podman]' v závislosti na backendu. Vyžaduje funkční prostředí Python/pip.",
        "da": "Installer automatisk llm-sandbox Python-pakken (med MCP-ekstra) ved opstart. Installerer 'llm-sandbox[mcp-docker]' eller 'llm-sandbox[mcp-podman]' afhængigt af backend. Kræver et fungerende Python/pip-miljø.",
        "de": "Das Python-Paket llm-sandbox (mit MCP-Extras) automatisch beim Start installieren. Installiert 'llm-sandbox[mcp-docker]' oder 'llm-sandbox[mcp-podman]' je nach Backend. Erfordert eine funktionierende Python/pip-Umgebung.",
        "el": "Αυτόματη εγκατάσταση του πακέτου Python llm-sandbox (με MCP πρόσθετα) κατά την εκκίνηση. Εγκαθιστά 'llm-sandbox[mcp-docker]' ή 'llm-sandbox[mcp-podman]' ανάλογα με το backend. Απαιτεί ένα λειτουργικό περιβάλλον Python/pip.",
        "es": "Instalar automáticamente el paquete Python llm-sandbox (con extras MCP) al inicio. Instala 'llm-sandbox[mcp-docker]' o 'llm-sandbox[mcp-podman]' según el backend. Requiere un entorno Python/pip funcional.",
        "fr": "Installer automatiquement le paquet Python llm-sandbox (avec extras MCP) au démarrage. Installe 'llm-sandbox[mcp-docker]' ou 'llm-sandbox[mcp-podman]' selon le backend. Nécessite un environnement Python/pip fonctionnel.",
        "hi": "स्टार्टअप पर llm-sandbox Python पैकेज (MCP एक्स्ट्रा के साथ) स्वचालित रूप से इंस्टॉल करें। बैकएंड के आधार पर 'llm-sandbox[mcp-docker]' या 'llm-sandbox[mcp-podman]' इंस्टॉल करता है। काम करने वाले Python/pip वातावरण की आवश्यकता है।",
        "it": "Installa automaticamente il pacchetto Python llm-sandbox (con extras MCP) all'avvio. Installa 'llm-sandbox[mcp-docker]' o 'llm-sandbox[mcp-podman]' a seconda del backend. Richiede un ambiente Python/pip funzionante.",
        "ja": "起動時にllm-sandbox Pythonパッケージ（MCPエクストラ付き）を自動インストールします。バックエンドに応じて'llm-sandbox[mcp-docker]'または'llm-sandbox[mcp-podman]'をインストールします。動作するPython/pip環境が必要です。",
        "nl": "Installeer automatisch het llm-sandbox Python-pakket (met MCP-extra's) bij opstarten. Installeert 'llm-sandbox[mcp-docker]' of 'llm-sandbox[mcp-podman]' afhankelijk van de backend. Vereist een werkende Python/pip-omgeving.",
        "no": "Installer automatisk llm-sandbox Python-pakken (med MCP-ekstra) ved oppstart. Installerer 'llm-sandbox[mcp-docker]' eller 'llm-sandbox[mcp-podman]' avhengig av backend. Krever et fungerende Python/pip-miljø.",
        "pl": "Automatycznie zainstaluj pakiet Python llm-sandbox (z dodatkami MCP) przy uruchomieniu. Instaluje 'llm-sandbox[mcp-docker]' lub 'llm-sandbox[mcp-podman]' w zależności od backendu. Wymaga działającego środowiska Python/pip.",
        "pt": "Instalar automaticamente o pacote Python llm-sandbox (com extras MCP) na inicialização. Instala 'llm-sandbox[mcp-docker]' ou 'llm-sandbox[mcp-podman]' dependendo do backend. Requer um ambiente Python/pip funcional.",
        "sv": "Installera automatiskt llm-sandbox Python-paketet (med MCP-tillägg) vid uppstart. Installerar 'llm-sandbox[mcp-docker]' eller 'llm-sandbox[mcp-podman]' beroende på backend. Kräver en fungerande Python/pip-miljö.",
        "zh": "启动时自动安装 llm-sandbox Python 包（带 MCP 扩展）。根据后端安装 'llm-sandbox[mcp-docker]' 或 'llm-sandbox[mcp-podman]'。需要可用的 Python/pip 环境。"
    },
    "Number of pre-warmed containers in the pool (0 = no pooling). Higher values speed up execution but use more resources.": {
        "cs": "Počet předehřátých kontejnerů ve fondu (0 = žádný fond). Vyšší hodnoty urychlují spouštění ale spotřebovávají více prostředků.",
        "da": "Antal forvarmede containere i puljen (0 = ingen pulje). Højere værdier fremskynder udførelse men bruger flere ressourcer.",
        "de": "Anzahl vorgewärmter Container im Pool (0 = kein Pooling). Höhere Werte beschleunigen die Ausführung, verbrauchen aber mehr Ressourcen.",
        "el": "Αριθμός προθερμασμένων κοντέινερ στην ομάδα (0 = καθόλου ομάδα). Υψηλότερες τιμές επιταχύνουν την εκτέλεση αλλά χρησιμοποιούν περισσότερους πόρους.",
        "es": "Número de contenedores precalentados en el pool (0 = sin pool). Valores más altos aceleran la ejecución pero usan más recursos.",
        "fr": "Nombre de conteneurs préchauffés dans le pool (0 = pas de pool). Des valeurs plus élevées accélèrent l'exécution mais consomment plus de ressources.",
        "hi": "पूल में प्री-वार्म किए गए कंटेनरों की संख्या (0 = कोई पूलिंग नहीं)। उच्च मान निष्पादन को तेज़ करते हैं लेकिन अधिक संसाधन उपयोग करते हैं।",
        "it": "Numero di container pre-riscaldati nel pool (0 = nessun pooling). Valori più alti velocizzano l'esecuzione ma usano più risorse.",
        "ja": "プール内の事前ウォームコンテナ数（0 = プールなし）。値が大きいほど実行が高速になりますが、リソースを多く消費します。",
        "nl": "Aantal voorverwarmde containers in de pool (0 = geen pooling). Hogere waarden versnellen uitvoering maar verbruiken meer middelen.",
        "no": "Antall forvarmede containere i puljen (0 = ingen pulje). Høyere verdier gjør kjøring raskere, men bruker mer ressurser.",
        "pl": "Liczba rozgrzanych kontenerów w puli (0 = brak puli). Wyższe wartości przyspieszają wykonanie, ale zużywają więcej zasobów.",
        "pt": "Número de contêineres pré-aquecidos no pool (0 = sem pool). Valores mais altos aceleram a execução mas usam mais recursos.",
        "sv": "Antal förvärmda containrar i poolen (0 = ingen poolning). Högre värden snabbar upp körningen men använder mer resurser.",
        "zh": "池中预热容器的数量（0 = 无池化）。值越高执行越快，但使用更多资源。"
    },
    "Uses the provider's native tool calls. <strong>Warning:</strong> Only enable if the model supports this (GPT-4, Claude, Gemini). Free-tier models often do NOT support this!": {
        "cs": "Používá nativní volání nástrojů poskytovatele. <strong>Varování:</strong> Povolte pouze pokud to model podporuje (GPT-4, Claude, Gemini). Bezplatné modely to často NEDOKÁŽÍ!",
        "da": "Bruger udbyderens indbyggede værktøjskald. <strong>Advarsel:</strong> Aktiver kun hvis modellen understøtter dette (GPT-4, Claude, Gemini). Gratis modeller understøtter ofte IKKE dette!",
        "de": "Verwendet die nativen Tool-Aufrufe des Anbieters. <strong>Warnung:</strong> Nur aktivieren, wenn das Modell dies unterstützt (GPT-4, Claude, Gemini). Kostenlose Modelle unterstützen dies oft NICHT!",
        "el": "Χρησιμοποιεί τις εγγενείς κλήσεις εργαλείων του παρόχου. <strong>Προειδοποίηση:</strong> Ενεργοποιήστε μόνο αν το μοντέλο το υποστηρίζει (GPT-4, Claude, Gemini). Τα δωρεάν μοντέλα συχνά ΔΕΝ το υποστηρίζουν!",
        "es": "Usa las llamadas nativas de herramientas del proveedor. <strong>Advertencia:</strong> Solo activar si el modelo lo soporta (GPT-4, Claude, Gemini). ¡Los modelos gratuitos a menudo NO lo soportan!",
        "fr": "Utilise les appels d'outils natifs du fournisseur. <strong>Avertissement :</strong> N'activer que si le modèle le prend en charge (GPT-4, Claude, Gemini). Les modèles gratuits ne le prennent souvent PAS en charge !",
        "hi": "प्रदाता के मूल टूल कॉल का उपयोग करता है। <strong>चेतावनी:</strong> केवल तभी सक्षम करें जब मॉडल इसका समर्थन करता हो (GPT-4, Claude, Gemini)। मुफ्त मॉडल अक्सर इसका समर्थन नहीं करते!",
        "it": "Utilizza le chiamate di strumenti native del provider. <strong>Avviso:</strong> Abilitare solo se il modello lo supporta (GPT-4, Claude, Gemini). I modelli gratuiti spesso NON lo supportano!",
        "ja": "プロバイダーのネイティブツール呼び出しを使用します。<strong>警告：</strong>モデルがこれをサポートしている場合のみ有効にしてください（GPT-4、Claude、Gemini）。無料ティアモデルはこれをサポートしていないことが多いです！",
        "nl": "Gebruikt de native tool-aanroepen van de provider. <strong>Waarschuwing:</strong> Alleen inschakelen als het model dit ondersteunt (GPT-4, Claude, Gemini). Gratis modellen ondersteunen dit vaak NIET!",
        "no": "Bruker leverandørens innebygde verktøykall. <strong>Advarsel:</strong> Aktiver kun hvis modellen støtter dette (GPT-4, Claude, Gemini). Gratis modeller støtter ofte IKKE dette!",
        "pl": "Używa natywnych wywołań narzędzi dostawcy. <strong>Ostrzeżenie:</strong> Włącz tylko, jeśli model to obsługuje (GPT-4, Claude, Gemini). Darmowe modele często tego NIE obsługują!",
        "pt": "Usa as chamadas de ferramentas nativas do provedor. <strong>Aviso:</strong> Ative apenas se o modelo suportar isso (GPT-4, Claude, Gemini). Modelos gratuitos frequentemente NÃO suportam isso!",
        "sv": "Använder leverantörens inbyggda verktygsanrop. <strong>Varning:</strong> Aktivera bara om modellen stöder detta (GPT-4, Claude, Gemini). Gratismodeller stöder ofta INTE detta!",
        "zh": "使用提供商的原生工具调用。<strong>警告：</strong>仅在模型支持时启用（GPT-4、Claude、Gemini）。免费模型通常不支持此功能！"
    },
    "Read-only mode for MeshCentral. Allows only read operations (list groups/devices) and blocks changes like wake, power actions, or command execution.": {
        "cs": "Režim pouze pro čtení pro MeshCentral. Umožňuje pouze operace čtení (seznam skupin/zařízení) a blokuje změny jako probuzení, napájení nebo spouštění příkazů.",
        "da": "Skrivebeskyttet tilstand for MeshCentral. Tillader kun læseoperationer (vis grupper/enheder) og blokerer ændringer som opvågning, strømhandlinger eller kommandoudførelse.",
        "de": "Schreibgeschützter Modus für MeshCentral. Erlaubt nur Leseoperationen (Gruppen/Geräte auflisten) und blockiert Änderungen wie Aufwecken, Energieaktionen oder Befehlsausführung.",
        "el": "Λειτουργία μόνο ανάγνωσης για MeshCentral. Επιτρέπει μόνο λειτουργίες ανάγνωσης (παράθεση ομάδων/συσκευών) και μπλοκάρει αλλαγές όπως αφύπνιση, ενεργειακές ενέργειες ή εκτέλεση εντολών.",
        "es": "Modo de solo lectura para MeshCentral. Permite solo operaciones de lectura (listar grupos/dispositivos) y bloquea cambios como despertar, acciones de energía o ejecución de comandos.",
        "fr": "Mode lecture seule pour MeshCentral. Autorise uniquement les opérations de lecture (lister groupes/appareils) et bloque les modifications comme le réveil, les actions d'alimentation ou l'exécution de commandes.",
        "hi": "MeshCentral के लिए केवल-पढ़ने मोड। केवल पढ़ने के ऑपरेशन (समूहों/उपकरणों की सूची) की अनुमति देता है और जागने, पावर क्रियाओं या कमांड निष्पादन जैसे परिवर्तनों को अवरुद्ध करता है।",
        "it": "Modalità sola lettura per MeshCentral. Permette solo operazioni di lettura (elenca gruppi/dispositivi) e blocca modifiche come riattivazione, azioni di alimentazione o esecuzione comandi.",
        "ja": "MeshCentralの読み取り専用モード。読み取り操作（グループ/デバイス一覧）のみを許可し、ウェイク、電源操作、コマンド実行などの変更をブロックします。",
        "nl": "Alleen-lezen modus voor MeshCentral. Staat alleen leesbewerkingen toe (groepen/apparaten weergeven) en blokkeert wijzigingen zoals ontwaken, energieacties of commando-uitvoering.",
        "no": "Skrivebeskyttet modus for MeshCentral. Tillater kun leseoperasjoner (vis grupper/enheter) og blokkerer endringer som oppvåkning, strømhandlinger eller kommandokjøring.",
        "pl": "Tryb tylko do odczytu dla MeshCentral. Zezwala tylko na operacje odczytu (lista grup/urządzeń) i blokuje zmiany takie jak wybudzanie, akcje zasilania lub wykonywanie poleceń.",
        "pt": "Modo somente leitura para MeshCentral. Permite apenas operações de leitura (listar grupos/dispositivos) e bloqueia alterações como ativação, ações de energia ou execução de comandos.",
        "sv": "Skrivskyddat läge för MeshCentral. Tillåter endast läsåtgärder (lista grupper/enheter) och blockerar ändringar som uppväckt, strömåtgärder eller kommandokörning.",
        "zh": "MeshCentral 只读模式。仅允许读取操作（列出组/设备），并阻止唤醒、电源操作或命令执行等更改。"
    },
    "Provider from provider management for the Helper LLM. This model is used for internal helper tasks and is separate from the Guardian model.": {
        "cs": "Poskytovatel ze správy poskytovatelů pro Helper LLM. Tento model se používá pro interní pomocné úlohy a je oddělený od modelu Guardian.",
        "da": "Udbyder fra udbyderstyring til Helper LLM. Denne model bruges til interne hjælpeopgaver og er adskilt fra Guardian-modellen.",
        "de": "Anbieter aus der Anbieterverwaltung für das Helper-LLM. Dieses Modell wird für interne Hilfsaufgaben verwendet und ist vom Guardian-Modell getrennt.",
        "el": "Πάροχος από τη διαχείριση παρόχων για το Helper LLM. Αυτό το μοντέλο χρησιμοποιείται για εσωτερικές βοηθητικές εργασίες και είναι ξεχωριστό από το μοντέλο Guardian.",
        "es": "Proveedor de la gestión de proveedores para el LLM Helper. Este modelo se usa para tareas auxiliares internas y es independiente del modelo Guardian.",
        "fr": "Fournisseur de la gestion des fournisseurs pour le LLM Helper. Ce modèle est utilisé pour les tâches auxiliaires internes et est distinct du modèle Guardian.",
        "hi": "सहायक LLM के लिए प्रदाता प्रबंधन से प्रदाता। यह मॉडल आंतरिक सहायक कार्यों के लिए उपयोग किया जाता है और गार्जियन मॉडल से अलग है।",
        "it": "Provider dalla gestione provider per l'LLM Helper. Questo modello viene utilizzato per le attività helper interne ed è separato dal modello Guardian.",
        "ja": "Helper LLMのプロバイダー管理からのプロバイダー。このモデルは内部ヘルパータスクに使用され、Guardianモデルとは別です。",
        "nl": "Provider uit providerbeheer voor de Helper LLM. Dit model wordt gebruikt voor interne hulptaken en is gescheiden van het Guardian-model.",
        "no": "Leverandør fra leverandørstyring for Helper LLM. Denne modellen brukes til interne hjelpeoppgaver og er atskilt fra Guardian-modellen.",
        "pl": "Dostawca z zarządzania dostawcami dla Helper LLM. Ten model jest używany do wewnętrznych zadań pomocniczych i jest oddzielony od modelu Guardian.",
        "pt": "Provedor do gerenciamento de provedores para o LLM Helper. Este modelo é usado para tarefas auxiliares internas e é separado do modelo Guardian.",
        "sv": "Leverantör från leverantörshantering för Helper LLM. Denna modell används för interna hjälparuppgifter och är separat från Guardian-modellen.",
        "zh": "来自提供商管理的 Helper LLM 提供商。此模型用于内部辅助任务，与 Guardian 模型分开。"
    },
    "The Helper LLM is disabled. Many dependent analysis and background features will not work fully, and AuraGo cannot use its full potential without it.": {
        "cs": "Helper LLM je zakázán. Mnoho závislých analytických a background funkcí nebude plně fungovat a AuraGo nemůže bez něj využít svůj plný potenciál.",
        "da": "Helper LLM er deaktiveret. Mange afhængige analyse- og baggrunds funktioner vil ikke fungere fuldt ud, og AuraGo kan ikke bruge sit fulde potentiale uden den.",
        "de": "Das Helper-LLM ist deaktiviert. Viele abhängige Analyse- und Hintergrundfunktionen werden nicht vollständig arbeiten, und AuraGo kann ohne es sein volles Potenzial nicht entfalten.",
        "el": "Το Helper LLM είναι απενεργοποιημένο. Πολλές εξαρτώμενες λειτουργίες ανάλυσης και παρασκηνίου δεν θα λειτουργούν πλήρως και το AuraGo δεν μπορεί να χρησιμοποιήσει πλήρως τις δυνατότητές του χωρίς αυτό.",
        "es": "El LLM Helper está desactivado. Muchas funciones de análisis y fondo dependientes no funcionarán completamente, y AuraGo no puede usar todo su potencial sin él.",
        "fr": "Le LLM Helper est désactivé. De nombreuses fonctions d'analyse et d'arrière-plan dépendantes ne fonctionneront pas complètement, et AuraGo ne peut pas utiliser tout son potentiel sans lui.",
        "hi": "सहायक LLM अक्षम है। कई आश्रित विश्लेषण और बैकग्राउंड सुविधाएँ पूरी तरह काम नहीं करेंगी, और AuraGo इसके बिना अपनी पूरी क्षमता का उपयोग नहीं कर सकता।",
        "it": "LLM Helper disabilitato. Molte funzionalità di analisi e background dipendenti non funzioneranno completamente e AuraGo non può sfruttare tutto il suo potenziale senza di esso.",
        "ja": "Helper LLMは無効です。多くの依存分析およびバックグラウンド機能が完全に動作せず、AuraGoはこれなしでは完全な潜在能力を発揮できません。",
        "nl": "Helper LLM is uitgeschakeld. Veel afhankelijke analyse- en achtergrondfuncties werken niet volledig, en AuraGo kan zonder dit zijn volledige potentieel niet benutten.",
        "no": "Helper LLM er deaktivert. Mange avhengige analyse- og bakgrunnsfunksjoner vil ikke fungere fullt ut, og AuraGo kan ikke bruke sitt fulle potensial uten den.",
        "pl": "Helper LLM jest wyłączony. Wiele zależnych funkcji analitycznych i w tle nie będzie działać w pełni, a AuraGo nie może bez niego wykorzystać swojego pełnego potencjału.",
        "pt": "O LLM Helper está desativado. Muitos recursos de análise e segundo plano dependentes não funcionarão completamente, e AuraGo não pode usar todo seu potencial sem ele.",
        "sv": "Helper LLM är inaktiverat. Många beroende analys- och bakgrundsfunktioner fungerar inte fullt ut, och AuraGo kan inte utnyttja sin fulla potential utan det.",
        "zh": "Helper LLM 已禁用。许多依赖的分析和后台功能将无法完全工作，AuraGo 无法在没有它的情况下发挥其全部潜力。"
    },
    "The Helper LLM is enabled. AuraGo uses it for internal analysis and background tasks and batches helper requests whenever possible. A smaller, faster and cheaper model is usually a good fit here.": {
        "cs": "Helper LLM je povolen. AuraGo ho používá pro interní analýzu a úlohy na pozadí a sdružuje pomocné požadavky, kdykoli je to možné. Menší, rychlejší a levnější model je zde obvykle dobrou volbou.",
        "da": "Helper LLM er aktiveret. AuraGo bruger den til intern analyse og baggrundsopgaver og batcher hjælpeanmodninger når det er muligt. En mindre, hurtigere og billigere model er normalt et godt valg her.",
        "de": "Das Helper-LLM ist aktiviert. AuraGo nutzt es für interne Analysen und Hintergrundaufgaben und bündelt Hilfsanfragen wann immer möglich. Ein kleineres, schnelleres und günstigeres Modell ist hier meist eine gute Wahl.",
        "el": "Το Helper LLM είναι ενεργοποιημένο. Το AuraGo το χρησιμοποιεί για εσωτερική ανάλυση και εργασίες παρασκηνίου και ομαδοποιεί τα αιτήματα βοηθού όταν είναι δυνατό. Ένα μικρότερο, πιο γρήγορο και φθηνότερο μοντέλο είναι συνήθως μια καλή επιλογή εδώ.",
        "es": "El LLM Helper está activado. AuraGo lo usa para análisis interno y tareas en segundo plano y agrupa solicitudes de ayuda cuando sea posible. Un modelo más pequeño, rápido y barato suele ser una buena opción aquí.",
        "fr": "Le LLM Helper est activé. AuraGo l'utilise pour l'analyse interne et les tâches en arrière-plan et regroupe les requêtes d'assistance lorsque c'est possible. Un modèle plus petit, plus rapide et moins cher est généralement un bon choix ici.",
        "hi": "सहायक LLM सक्षम है। AuraGo इसका उपयोग आंतरिक विश्लेषण और बैकग्राउंड कार्यों के लिए करता है और संभव होने पर सहायक अनुरोधों को बैच करता है। एक छोटा, तेज़ और सस्ता मॉडल आमतौर पर यहाँ अच्छा विकल्प है।",
        "it": "LLM Helper abilitato. AuraGo lo utilizza per l'analisi interna e le attività in background e raggruppa le richieste helper quando possibile. Un modello più piccolo, veloce ed economico è di solito una buona scelta qui.",
        "ja": "Helper LLMは有効です。AuraGoは内部分析とバックグラウンドタスクに使用し、可能な限りヘルパーリクエストをバッチ処理します。より小さく、高速で安価なモデルが通常ここに適しています。",
        "nl": "Helper LLM is ingeschakeld. AuraGo gebruikt het voor interne analyse en achtergrondtaken en batcht hulpverzoeken waar mogelijk. Een kleiner, sneller en goedkoper model is hier meestal een goede keuze.",
        "no": "Helper LLM er aktivert. AuraGo bruker den til intern analyse og bakgrunnsoppgaver og batcher hjelpeforespørsler når mulig. En mindre, raskere og billigere modell er vanligvis et godt valg her.",
        "pl": "Helper LLM jest włączony. AuraGo używa go do analizy wewnętrznej i zadań w tle oraz grupuje żądania pomocnicze, gdy to możliwe. Mniejszy, szybszy i tańszy model jest zazwyczaj dobrym wyborem.",
        "pt": "O LLM Helper está ativado. AuraGo o usa para análise interna e tarefas em segundo plano e agrupa solicitações auxiliares quando possível. Um modelo menor, mais rápido e mais barato costuma ser uma boa escolha aqui.",
        "sv": "Helper LLM är aktiverat. AuraGo använder det för intern analys och bakgrundsuppgifter och batchar hjälpförfrågningar när möjligt. En mindre, snabbare och billigare modell är vanligtvis ett bra val här.",
        "zh": "Helper LLM 已启用。AuraGo 使用它进行内部分析和后台任务，并尽可能批量处理辅助请求。通常一个更小、更快、更便宜的模型在这里是个不错的选择。"
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

print(f"Batch 21: Fixed {fixed} keys")

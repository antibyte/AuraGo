#!/usr/bin/env python3
"""Batch 22: More config descriptions + Let's Encrypt, Tailscale, webhooks."""
import json
from pathlib import Path

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "Contact email for Let's Encrypt. Used for certificate expiry warnings. Not publicly displayed.": {
        "cs": "Kontaktní e-mail pro Let's Encrypt. Používá se pro varování o vypršení certifikátu. Není veřejně zobrazen.",
        "da": "Kontakte-mail til Let's Encrypt. Bruges til advarsler om udløb af certifikater. Vises ikke offentligt.",
        "de": "Kontakt-E-Mail für Let's Encrypt. Wird für Warnungen zum Zertifikatsablauf verwendet. Wird nicht öffentlich angezeigt.",
        "el": "Email επικοινωνίας για το Let's Encrypt. Χρησιμοποιείται για προειδοποιήσεις λήξης πιστοποιητικού. Δεν εμφανίζεται δημόσια.",
        "es": "Email de contacto para Let's Encrypt. Se usa para avisos de expiración de certificados. No se muestra públicamente.",
        "fr": "E-mail de contact pour Let's Encrypt. Utilisé pour les avertissements d'expiration de certificat. Non affiché publiquement.",
        "hi": "Let's Encrypt के लिए संपर्क ईमेल। प्रमाणपत्र समाप्ति चेतावनियों के लिए उपयोग किया जाता है। सार्वजनिक रूप से प्रदर्शित नहीं।",
        "it": "Email di contatto per Let's Encrypt. Utilizzata per avvisi di scadenza certificato. Non mostrata pubblicamente.",
        "ja": "Let's Encryptの連絡先メール。証明書の有効期限警告に使用されます。公開表示されません。",
        "nl": "Contact-e-mail voor Let's Encrypt. Gebruikt voor waarschuwingen over certificaatverloop. Niet openbaar weergegeven.",
        "no": "Kontakte-post for Let's Encrypt. Brukes for advarsler om sertifikatutløp. Vises ikke offentlig.",
        "pl": "E-mail kontaktowy dla Let's Encrypt. Używany do ostrzeżeń o wygaśnięciu certyfikatu. Nie jest wyświetlany publicznie.",
        "pt": "E-mail de contato para Let's Encrypt. Usado para avisos de expiração de certificado. Não exibido publicamente.",
        "sv": "Kontakte-post för Let's Encrypt. Används för varningar om certifikatutgång. Visas inte offentligt.",
        "zh": "Let's Encrypt 的联系电子邮件。用于证书到期警告。不公开显示。"
    },
    "Enables HTTPS via Let's Encrypt. AuraGo will automatically obtain and renew a certificate for your domain. HTTP (port 80) from external sources is redirected to HTTPS. Localhost remains accessible via HTTP.": {
        "cs": "Povoluje HTTPS přes Let's Encrypt. AuraGo automaticky získá a obnoví certifikát pro vaši doménu. HTTP (port 80) z externích zdrojů je přesměrováno na HTTPS. Localhost zůstává přístupný přes HTTP.",
        "da": "Aktiverer HTTPS via Let's Encrypt. AuraGo vil automatisk indhente og forny et certifikat til dit domæne. HTTP (port 80) fra eksterne kilder omdirigeres til HTTPS. Localhost forbliver tilgængelig via HTTP.",
        "de": "Aktiviert HTTPS über Let's Encrypt. AuraGo beschafft und erneuert automatisch ein Zertifikat für Ihre Domain. HTTP (Port 80) von externen Quellen wird auf HTTPS umgeleitet. Localhost bleibt über HTTP erreichbar.",
        "el": "Ενεργοποιεί HTTPS μέσω Let's Encrypt. Το AuraGo θα αποκτήσει και θα ανανεώσει αυτόματα ένα πιστοποιητικό για τον τομέα σας. HTTP (θύρα 80) από εξωτερικές πηγές ανακατευθύνεται σε HTTPS. Το Localhost παραμένει προσβάσιμο μέσω HTTP.",
        "es": "Activa HTTPS mediante Let's Encrypt. AuraGo obtendrá y renovará automáticamente un certificado para su dominio. HTTP (puerto 80) de fuentes externas se redirige a HTTPS. Localhost sigue accesible vía HTTP.",
        "fr": "Active HTTPS via Let's Encrypt. AuraGo obtiendra et renouvellera automatiquement un certificat pour votre domaine. HTTP (port 80) des sources externes est redirigé vers HTTPS. Localhost reste accessible via HTTP.",
        "hi": "Let's Encrypt के माध्यम से HTTPS सक्षम करता है। AuraGo स्वचालित रूप से आपके डोमेन के लिए प्रमाणपत्र प्राप्त और नवीनीकरण करेगा। बाहरी स्रोतों से HTTP (पोर्ट 80) को HTTPS पर पुनर्निर्देशित किया जाता है। Localhost HTTP के माध्यम से सुलभ रहता है।",
        "it": "Abilita HTTPS tramite Let's Encrypt. AuraGo otterrà e rinnoverà automaticamente un certificato per il tuo dominio. HTTP (porta 80) da fonti esterne viene reindirizzato a HTTPS. Localhost rimane accessibile via HTTP.",
        "ja": "Let's EncryptでHTTPSを有効にします。AuraGoはドメインの証明書を自動取得・更新します。外部からのHTTP（ポート80）はHTTPSにリダイレクトされます。LocalhostはHTTPのままアクセス可能です。",
        "nl": "Schakelt HTTPS in via Let's Encrypt. AuraGo haalt en vernieuwt automatisch een certificaat voor uw domein. HTTP (poort 80) van externe bronnen wordt omgeleid naar HTTPS. Localhost blijft toegankelijk via HTTP.",
        "no": "Aktiverer HTTPS via Let's Encrypt. AuraGo vil automatisk skaffe og fornye et sertifikat for ditt domene. HTTP (port 80) fra eksterne kilder omdirigeres til HTTPS. Localhost forblir tilgjengelig via HTTP.",
        "pl": "Włącza HTTPS przez Let's Encrypt. AuraGo automatycznie uzyska i odnowi certyfikat dla Twojej domeny. HTTP (port 80) ze źródeł zewnętrznych jest przekierowany na HTTPS. Localhost pozostaje dostępny przez HTTP.",
        "pt": "Ativa HTTPS via Let's Encrypt. AuraGo obterá e renovará automaticamente um certificado para seu domínio. HTTP (porta 80) de fontes externas é redirecionado para HTTPS. Localhost permanece acessível via HTTP.",
        "sv": "Aktiverar HTTPS via Let's Encrypt. AuraGo hämtar och förnyar automatiskt ett certifikat för din domän. HTTP (port 80) från externa källor omdirigeras till HTTPS. Localhost förblir tillgängligt via HTTP.",
        "zh": "通过 Let's Encrypt 启用 HTTPS。AuraGo 将自动获取并续期您域名的证书。来自外部源的 HTTP（端口 80）被重定向到 HTTPS。Localhost 仍可通过 HTTP 访问。"
    },
    "Public domain name (e.g. aurago.mydomain.com). Both port 80 and 443 must be forwarded to AuraGo for Let's Encrypt to work.": {
        "cs": "Veřejný název domény (např. aurago.mydomain.com). Porty 80 i 443 musí být přesměrovány na AuraGo, aby Let's Encrypt fungoval.",
        "da": "Offentligt domænenavn (f.eks. aurago.mydomain.com). Både port 80 og 443 skal videresendes til AuraGo for at Let's Encrypt fungerer.",
        "de": "Öffentlicher Domainname (z.B. aurago.mydomain.com). Port 80 und 443 müssen an AuraGo weitergeleitet werden, damit Let's Encrypt funktioniert.",
        "el": "Δημόσιο όνομα τομέα (π.χ. aurago.mydomain.com). Τόσο η θύρα 80 όσο και η 443 πρέπει να προωθούνται στο AuraGo για να λειτουργήσει το Let's Encrypt.",
        "es": "Nombre de dominio público (p.ej. aurago.mydomain.com). Los puertos 80 y 443 deben redirigirse a AuraGo para que Let's Encrypt funcione.",
        "fr": "Nom de domaine public (p.ex. aurago.mydomain.com). Les ports 80 et 443 doivent être redirigés vers AuraGo pour que Let's Encrypt fonctionne.",
        "hi": "सार्वजनिक डोमेन नाम (उदा. aurago.mydomain.com)। Let's Encrypt काम करने के लिए पोर्ट 80 और 443 दोनों को AuraGo पर अग्रेषित किया जाना चाहिए।",
        "it": "Nome di dominio pubblico (es. aurago.mydomain.com). Sia la porta 80 che la 443 devono essere inoltrate ad AuraGo affinché Let's Encrypt funzioni.",
        "ja": "パブリックドメイン名（例：aurago.mydomain.com）。Let's Encryptが機能するにはポート80と443の両方をAuraGoに転送する必要があります。",
        "nl": "Openbare domeinnaam (bijv. aurago.mydomain.com). Zowel poort 80 als 443 moeten worden doorgestuurd naar AuraGo om Let's Encrypt te laten werken.",
        "no": "Offentlig domenenavn (f.eks. aurago.mydomain.com). Både port 80 og 443 må videresendes til AuraGo for at Let's Encrypt skal fungere.",
        "pl": "Publiczna nazwa domeny (np. aurago.mydomain.com). Zarówno port 80, jak i 443 muszą być przekierowane do AuraGo, aby Let's Encrypt działał.",
        "pt": "Nome de domínio público (p.ex. aurago.mydomain.com). As portas 80 e 443 devem ser encaminhadas para AuraGo para que Let's Encrypt funcione.",
        "sv": "Offentligt domännamn (t.ex. aurago.mydomain.com). Både port 80 och 443 måste vidarebefordras till AuraGo för att Let's Encrypt ska fungera.",
        "zh": "公共域名（例如 aurago.mydomain.com）。端口 80 和 443 都必须转发到 AuraGo，Let's Encrypt 才能工作。"
    },
    "Expose AuraGo securely on the public internet via Tailscale Funnel. AuraGo login stays active.": {
        "cs": "Bezpečně vystavit AuraGo na veřejném internetu přes Tailscale Funnel. Přihlášení AuraGo zůstává aktivní.",
        "da": "Eksponer AuraGo sikkert på det offentlige internet via Tailscale Funnel. AuraGo-login forbliver aktivt.",
        "de": "AuraGo sicher über das öffentliche Internet über Tailscale Funnel zugänglich machen. AuraGo-Login bleibt aktiv.",
        "el": "Εκθέστε το AuraGo με ασφάλεια στο δημόσιο διαδίκτυο μέσω Tailscale Funnel. Η σύνδεση AuraGo παραμένει ενεργή.",
        "es": "Exponer AuraGo de forma segura en el internet público mediante Tailscale Funnel. El inicio de sesión de AuraGo permanece activo.",
        "fr": "Exposer AuraGo en toute sécurité sur l'internet public via Tailscale Funnel. La connexion AuraGo reste active.",
        "hi": "Tailscale Funnel के माध्यम से AuraGo को सार्वजनिक इंटरनेट पर सुरक्षित रूप से उजागर करें। AuraGo लॉगिन सक्रिय रहता है।",
        "it": "Esporre AuraGo in modo sicuro su internet pubblico tramite Tailscale Funnel. Il login AuraGo rimane attivo.",
        "ja": "Tailscale Funnel経由でAuraGoをパブリックインターネットに安全に公開します。AuraGoログインは有効なままです。",
        "nl": "Stel AuraGo veilig beschikbaar op het openbare internet via Tailscale Funnel. AuraGo-login blijft actief.",
        "no": "Eksponer AuraGo sikkert på det offentlige internett via Tailscale Funnel. AuraGo-innlogging forblir aktiv.",
        "pl": "Udostępnij AuraGo bezpiecznie w publicznym internecie przez Tailscale Funnel. Logowanie AuraGo pozostaje aktywne.",
        "pt": "Expor AuraGo com segurança na internet pública via Tailscale Funnel. O login AuraGo permanece ativo.",
        "sv": "Exponera AuraGo säkert på offentliga internet via Tailscale Funnel. AuraGo-inloggning förblir aktiv.",
        "zh": "通过 Tailscale Funnel 在公共互联网上安全地公开 AuraGo。AuraGo 登录保持活跃。"
    },
    "Funnel additionally needs Tailscale Funnel to be enabled for this tailnet and node. Funnel only applies to the AuraGo web UI on port 443 and keeps AuraGo login protection active.": {
        "cs": "Funnel navíc vyžaduje, aby bylo Tailscale Funnel povoleno pro tento tailnet a uzel. Funnel se vztahuje pouze na webové rozhraní AuraGo na portu 443 a zachovává ochranu přihlášení AuraGo.",
        "da": "Funnel kræver desuden at Tailscale Funnel er aktiveret for dette tailnet og denne node. Funnel gælder kun for AuraGo-web-UI på port 443 og bevarer AuraGo-loginbeskyttelse.",
        "de": "Funnel erfordert zusätzlich, dass Tailscale Funnel für dieses Tailnet und diesen Node aktiviert ist. Funnel gilt nur für die AuraGo-Web-UI auf Port 443 und behält den AuraGo-Login-Schutz bei.",
        "el": "Το Funnel απαιτεί επιπλέον να είναι ενεργοποιημένο το Tailscale Funnel για αυτό το tailnet και κόμβο. Το Funnel ισχύει μόνο για το web UI του AuraGo στη θύρα 443 και διατηρεί ενεργή την προστασία σύνδεσης AuraGo.",
        "es": "Funnel además necesita que Tailscale Funnel esté habilitado para este tailnet y nodo. Funnel solo se aplica a la UI web de AuraGo en el puerto 443 y mantiene activa la protección de inicio de sesión.",
        "fr": "Funnel nécessite en plus que Tailscale Funnel soit activé pour ce tailnet et ce nœud. Funnel s'applique uniquement à l'interface web AuraGo sur le port 443 et conserve la protection de connexion AuraGo.",
        "hi": "Funnel को इस tailnet और node के लिए Tailscale Funnel सक्षम होने की भी आवश्यकता है। Funnel केवल पोर्ट 443 पर AuraGo वेब UI पर लागू होता है और AuraGo लॉगिन सुरक्षा सक्रिय रखता है।",
        "it": "Funnel richiede inoltre che Tailscale Funnel sia abilitato per questo tailnet e nodo. Funnel si applica solo all'interfaccia web di AuraGo sulla porta 443 e mantiene attiva la protezione del login AuraGo.",
        "ja": "FunnelにはこのtailnetとノードでTailscale Funnelが有効になっていることも必要です。Funnelはポート443のAuraGo Web UIにのみ適用され、AuraGoログイン保護を維持します。",
        "nl": "Funnel vereist bovendien dat Tailscale Funnel is ingeschakeld voor dit tailnet en deze node. Funnel is alleen van toepassing op de AuraGo-webinterface op poort 443 en behoudt de AuraGo-loginbeveiliging.",
        "no": "Funnel krever i tillegg at Tailscale Funnel er aktivert for dette tailnet og denne noden. Funnel gjelder kun for AuraGo-webgrensesnittet på port 443 og bevarer AuraGo-innloggingsbeskyttelsen.",
        "pl": "Funnel wymaga dodatkowo włączenia Tailscale Funnel dla tego tailnetu i węzła. Funnel dotyczy tylko interfejsu webowego AuraGo na porcie 443 i zachowuje ochronę logowania AuraGo.",
        "pt": "Funnel precisa adicionalmente que o Tailscale Funnel esteja ativado para este tailnet e nó. Funnel aplica-se apenas à UI web do AuraGo na porta 443 e mantém a proteção de login AuraGo ativa.",
        "sv": "Funnel kräver dessutom att Tailscale Funnel är aktiverat för detta tailnet och denna nod. Funnel gäller endast för AuraGo-webbgränssnittet på port 443 och behåller AuraGo-inloggningsskyddet aktivt.",
        "zh": "Funnel 还需要为此 tailnet 和节点启用 Tailscale Funnel。Funnel 仅适用于端口 443 上的 AuraGo Web UI，并保持 AuraGo 登录保护活跃。"
    },
    "HTTPS over Tailscale needs MagicDNS plus HTTPS certificates to be enabled for this node in the Tailscale admin panel. If certificates are not available yet, AuraGo falls back to HTTP on port 80.": {
        "cs": "HTTPS přes Tailscale vyžaduje MagicDNS a HTTPS certifikáty povolené pro tento uzel v administraci Tailscale. Pokud certifikáty ještě nejsou k dispozici, AuraGo se vrátí k HTTP na portu 80.",
        "da": "HTTPS over Tailscale kræver MagicDNS plus HTTPS-certifikater aktiveret for denne node i Tailscale-adminpanelet. Hvis certifikaterne endnu ikke er tilgængelige, falder AuraGo tilbage til HTTP på port 80.",
        "de": "HTTPS über Tailscale erfordert MagicDNS und aktivierte HTTPS-Zertifikate für diesen Node im Tailscale-Admin-Panel. Falls Zertifikate noch nicht verfügbar sind, fällt AuraGo auf HTTP an Port 80 zurück.",
        "el": "Το HTTPS μέσω Tailscale απαιτεί MagicDNS συν πιστοποιητικά HTTPS ενεργοποιημένα για αυτόν τον κόμβο στο Tailscale admin panel. Αν τα πιστοποιητικά δεν είναι ακόμα διαθέσιμα, το AuraGo επιστρέφει σε HTTP στη θύρα 80.",
        "es": "HTTPS sobre Tailscale necesita MagicDNS y certificados HTTPS habilitados para este nodo en el panel de administración de Tailscale. Si los certificados aún no están disponibles, AuraGo recurre a HTTP en el puerto 80.",
        "fr": "HTTPS sur Tailscale nécessite MagicDNS et les certificats HTTPS activés pour ce nœud dans le panneau d'administration Tailscale. Si les certificats ne sont pas encore disponibles, AuraGo revient à HTTP sur le port 80.",
        "hi": "Tailscale पर HTTPS के लिए Tailscale एडमिन पैनल में इस नोड के लिए MagicDNS और HTTPS प्रमाणपत्र सक्षम होने चाहिए। यदि प्रमाणपत्र अभी तक उपलब्ध नहीं हैं, तो AuraGo पोर्ट 80 पर HTTP पर वापस जाता है।",
        "it": "HTTPS su Tailscale richiede MagicDNS e certificati HTTPS abilitati per questo nodo nel pannello admin Tailscale. Se i certificati non sono ancora disponibili, AuraGo ripiega su HTTP sulla porta 80.",
        "ja": "Tailscale経由のHTTPSには、Tailscale管理パネルでこのノードに対してMagicDNSとHTTPS証明書が有効になっている必要があります。証明書がまだ利用できない場合、AuraGoはポート80のHTTPにフォールバックします。",
        "nl": "HTTPS over Tailscale vereist MagicDNS plus HTTPS-certificaten ingeschakeld voor deze node in het Tailscale-beheerpaneel. Als certificaten nog niet beschikbaar zijn, valt AuraGo terug op HTTP op poort 80.",
        "no": "HTTPS over Tailscale krever MagicDNS og HTTPS-sertifikater aktivert for denne noden i Tailscale-adminpanelet. Hvis sertifikater ikke er tilgjengelige ennå, faller AuraGo tilbake til HTTP på port 80.",
        "pl": "HTTPS przez Tailscale wymaga włączenia MagicDNS i certyfikatów HTTPS dla tego węzła w panelu administracyjnym Tailscale. Jeśli certyfikaty nie są jeszcze dostępne, AuraGo powraca do HTTP na porcie 80.",
        "pt": "HTTPS sobre Tailscale precisa de MagicDNS e certificados HTTPS ativados para este nó no painel de administração do Tailscale. Se os certificados ainda não estão disponíveis, AuraGo recua para HTTP na porta 80.",
        "sv": "HTTPS över Tailscale kräver MagicDNS och HTTPS-certifikat aktiverade för denna nod i Tailscale-adminpanelen. Om certifikaten ännu inte är tillgängliga, faller AuraGo tillbaka på HTTP på port 80.",
        "zh": "通过 Tailscale 的 HTTPS 需要在 Tailscale 管理面板中为此节点启用 MagicDNS 和 HTTPS 证书。如果证书尚不可用，AuraGo 将回退到端口 80 上的 HTTP。"
    },
    "Publishes the Homepage/Caddy web server on your tailnet via HTTPS on port 8443.": {
        "cs": "Zveřejňuje webový server Homepage/Caddy na vašem tailnetu přes HTTPS na portu 8443.",
        "da": "Udgiver Homepage/Caddy-webserveren på dit tailnet via HTTPS på port 8443.",
        "de": "Veröffentlicht den Homepage/Caddy-Webserver in Ihrem Tailnet über HTTPS auf Port 8443.",
        "el": "Δημοσιεύει τον διακομιστή Homepage/Caddy στο tailnet σας μέσω HTTPS στη θύρα 8443.",
        "es": "Publica el servidor web Homepage/Caddy en su tailnet mediante HTTPS en el puerto 8443.",
        "fr": "Publie le serveur web Homepage/Caddy sur votre tailnet via HTTPS sur le port 8443.",
        "hi": "पोर्ट 8443 पर HTTPS के माध्यम से अपने tailnet पर Homepage/Caddy वेब सर्वर प्रकाशित करता है।",
        "it": "Pubblica il server web Homepage/Caddy sulla tua tailnet tramite HTTPS sulla porta 8443.",
        "ja": "ポート8443でHTTPSを介してtailnetにHomepage/Caddyウェブサーバーを公開します。",
        "nl": "Publiceert de Homepage/Caddy-webserver op uw tailnet via HTTPS op poort 8443.",
        "no": "Publiserer Homepage/Caddy-webserveren på ditt tailnet via HTTPS på port 8443.",
        "pl": "Publikuje serwer webowy Homepage/Caddy w Twoim tailnecie przez HTTPS na porcie 8443.",
        "pt": "Publica o servidor web Homepage/Caddy no seu tailnet via HTTPS na porta 8443.",
        "sv": "Publicerar Homepage/Caddy-webbservern på ditt tailnet via HTTPS på port 8443.",
        "zh": "通过 HTTPS 在端口 8443 上发布您的 tailnet 上的 Homepage/Caddy Web 服务器。"
    },
    "Set the absolute host path that is mounted into the homepage container as /workspace.": {
        "cs": "Nastavte absolutní cestu hostitele, která je připojena do kontejneru homepage jako /workspace.",
        "da": "Indstil den absolutte værtsti der monteres i homepage-containeren som /workspace.",
        "de": "Legen Sie den absoluten Host-Pfad fest, der als /workspace in den Homepage-Container gemountet wird.",
        "el": "Ορίστε την απόλυτη διαδρομή υποδοχής που προσαρτάται στο container homepage ως /workspace.",
        "es": "Establezca la ruta absoluta del host que se monta en el contenedor homepage como /workspace.",
        "fr": "Définissez le chemin absolu de l'hôte monté dans le conteneur homepage comme /workspace.",
        "hi": "होमपेज कंटेनर में /workspace के रूप में माउंट किए जाने वाले निरपेक्ष होस्ट पथ को सेट करें।",
        "it": "Imposta il percorso assoluto dell'host montato nel container homepage come /workspace.",
        "ja": "ホームページコンテナに/workspaceとしてマウントするホストの絶対パスを設定します。",
        "nl": "Stel het absolute hostpad in dat als /workspace in de homepage-container wordt gekoppeld.",
        "no": "Angi den absolutte vertsstien som monteres i homepage-containeren som /workspace.",
        "pl": "Ustaw bezwzględną ścieżkę hosta, która jest montowana w kontenerze homepage jako /workspace.",
        "pt": "Defina o caminho absoluto do host que é montado no contêiner homepage como /workspace.",
        "sv": "Ange den absoluta värd-sökvägen som monteras i homepage-containern som /workspace.",
        "zh": "设置挂载到 homepage 容器中作为 /workspace 的绝对主机路径。"
    },
    "Homepage exposure is enabled. If no URL appears, make sure the Homepage web server is running and Tailscale HTTPS is enabled.": {
        "cs": "Vystavení domovské stránky je povoleno. Pokud se žádná URL nezobrazí, ujistěte se, že webový server Homepage běží a Tailscale HTTPS je povoleno.",
        "da": "Hjemmesideeksponering er aktiveret. Hvis ingen URL vises, skal du sikre dig at Homepage-webserveren kører og Tailscale HTTPS er aktiveret.",
        "de": "Die Homepage-Freigabe ist aktiviert. Wenn keine URL erscheint, stellen Sie sicher, dass der Homepage-Webserver läuft und Tailscale HTTPS aktiviert ist.",
        "el": "Η έκθεση της αρχικής σελίδας είναι ενεργοποιημένη. Αν δεν εμφανίζεται URL, βεβαιωθείτε ότι ο διακομιστής Homepage λειτουργεί και το Tailscale HTTPS είναι ενεργοποιημένο.",
        "es": "La exposición de Homepage está activada. Si no aparece ninguna URL, asegúrese de que el servidor web Homepage esté ejecutándose y Tailscale HTTPS esté habilitado.",
        "fr": "L'exposition de la page d'accueil est activée. Si aucune URL n'apparaît, assurez-vous que le serveur web Homepage fonctionne et que HTTPS Tailscale est activé.",
        "hi": "होमपेज़ एक्सपोज़र सक्षम है। यदि कोई URL नहीं दिखती है, तो सुनिश्चित करें कि Homepage वेब सर्वर चल रहा है और Tailscale HTTPS सक्षम है।",
        "it": "L'esposizione della homepage è abilitata. Se non compare nessun URL, assicurati che il server web Homepage sia in esecuzione e Tailscale HTTPS sia abilitato.",
        "ja": "ホームページの公開が有効です。URLが表示されない場合は、Homepageウェブサーバーが実行中でTailscale HTTPSが有効になっていることを確認してください。",
        "nl": "Homepage-expositie is ingeschakeld. Als er geen URL verschijnt, zorg ervoor dat de Homepage-webserver draait en Tailscale HTTPS is ingeschakeld.",
        "no": "Hjemmesideeksponering er aktivert. Hvis ingen URL vises, sørg for at Homepage-webserveren kjører og Tailscale HTTPS er aktivert.",
        "pl": "Udostępnianie strony głównej jest włączone. Jeśli nie pojawia się żaden URL, upewnij się, że serwer webowy Homepage działa i Tailscale HTTPS jest włączony.",
        "pt": "A exposição da página inicial está ativada. Se nenhuma URL aparecer, certifique-se de que o servidor web Homepage está em execução e Tailscale HTTPS está ativado.",
        "sv": "Hemsidexponering är aktiverad. Om ingen URL visas, se till att Homepage-webbservern körs och Tailscale HTTPS är aktiverat.",
        "zh": "主页公开已启用。如果未显示 URL，请确保 Homepage Web 服务器正在运行且 Tailscale HTTPS 已启用。"
    },
    "Homepage exposure needs the Homepage web server to be enabled first.": {
        "cs": "Vystavení domovské stránky vyžaduje nejprve povolení webového serveru Homepage.",
        "da": "Hjemmesideeksponering kræver at Homepage-webserveren aktiveres først.",
        "de": "Die Homepage-Freigabe erfordert, dass der Homepage-Webserver zuerst aktiviert wird.",
        "el": "Η έκθεση της αρχικής σελίδας απαιτεί να ενεργοποιηθεί πρώτα ο διακομιστής Homepage.",
        "es": "La exposición de Homepage necesita que el servidor web Homepage se habilite primero.",
        "fr": "L'exposition de la page d'accueil nécessite que le serveur web Homepage soit d'abord activé.",
        "hi": "होमपेज़ एक्सपोज़र के लिए पहले Homepage वेब सर्वर को सक्षम करना आवश्यक है।",
        "it": "L'esposizione della homepage richiede che il server web Homepage sia prima abilitato.",
        "ja": "ホームページの公開には、まずHomepageウェブサーバーを有効にする必要があります。",
        "nl": "Homepage-expositie vereist dat de Homepage-webserver eerst wordt ingeschakeld.",
        "no": "Hjemmesideeksponering krever at Homepage-webserveren aktiveres først.",
        "pl": "Udostępnianie strony głównej wymaga wcześniejszego włączenia serwera webowego Homepage.",
        "pt": "A exposição da página inicial precisa que o servidor web Homepage seja ativado primeiro.",
        "sv": "Hemsidexponering kräver att Homepage-webbservern aktiveras först.",
        "zh": "主页公开需要先启用 Homepage Web 服务器。"
    },
    "Loopback HTTP port for cloudflared (avoids TLS)": {
        "cs": "Loopback HTTP port pro cloudflared (vyhýbá se TLS)",
        "da": "Loopback HTTP-port til cloudflared (undgår TLS)",
        "de": "Loopback-HTTP-Port für cloudflared (vermeidet TLS)",
        "el": "Θύρα HTTP loopback για cloudflared (αποφεύγει TLS)",
        "es": "Puerto HTTP loopback para cloudflared (evita TLS)",
        "fr": "Port HTTP loopback pour cloudflared (évite TLS)",
        "hi": "cloudflared के लिए लूपबैक HTTP पोर्ट (TLS से बचता है)",
        "it": "Porta HTTP loopback per cloudflared (evita TLS)",
        "ja": "cloudflared用ループバックHTTPポート（TLSを回避）",
        "nl": "Loopback HTTP-poort voor cloudflared (vermijdt TLS)",
        "no": "Loopback HTTP-port for cloudflared (unngår TLS)",
        "pl": "Port HTTP loopback dla cloudflared (omija TLS)",
        "pt": "Porta HTTP loopback para cloudflared (evita TLS)",
        "sv": "Loopback HTTP-port för cloudflared (undviker TLS)",
        "zh": "cloudflared 的环回 HTTP 端口（避免 TLS）"
    },
    "Opens a plain HTTP port on 127.0.0.1 only. cloudflared connects to this instead of the HTTPS port — no certificate verification needed. The port is assigned automatically.": {
        "cs": "Otevírá čistý HTTP port pouze na 127.0.0.1. cloudflared se připojuje sem místo HTTPS portu — není potřeba ověřování certifikátů. Port je přiřazen automaticky.",
        "da": "Åbner en almindelig HTTP-port kun på 127.0.0.1. cloudflared forbinder til denne i stedet for HTTPS-porten — ingen certifikatbekræftelse nødvendig. Porten tildeles automatisk.",
        "de": "Öffnet einen reinen HTTP-Port nur auf 127.0.0.1. cloudflared verbindet sich damit statt mit dem HTTPS-Port — keine Zertifikatsprüfung nötig. Der Port wird automatisch zugewiesen.",
        "el": "Ανοίγει μια απλή θύρα HTTP μόνο στο 127.0.0.1. Το cloudflared συνδέεται σε αυτό αντί για τη θύρα HTTPS — δεν χρειάζεται επαλήθευση πιστοποιητικού. Η θύρα ανατίθεται αυτόματα.",
        "es": "Abre un puerto HTTP simple solo en 127.0.0.1. cloudflared se conecta a este en lugar del puerto HTTPS — no se necesita verificación de certificado. El puerto se asigna automáticamente.",
        "fr": "Ouvre un port HTTP simple uniquement sur 127.0.0.1. cloudflared s'y connecte au lieu du port HTTPS — aucune vérification de certificat nécessaire. Le port est attribué automatiquement.",
        "hi": "केवल 127.0.0.1 पर एक सादा HTTP पोर्ट खोलता है। cloudflared इससे HTTPS पोर्ट के बजाय कनेक्ट करता है — प्रमाणपत्र सत्यापन की आवश्यकता नहीं। पोर्ट स्वचालित रूप से असाइन किया जाता है।",
        "it": "Apre una porta HTTP semplice solo su 127.0.0.1. cloudflared si collega a questa invece della porta HTTPS — nessuna verifica certificato necessaria. La porta viene assegnata automaticamente.",
        "ja": "127.0.0.1でのみプレーンHTTPポートを開きます。cloudflaredはHTTPSポートの代わりにこれに接続します — 証明書検証は不要です。ポートは自動的に割り当てられます。",
        "nl": "Opent een gewone HTTP-poort alleen op 127.0.0.1. cloudflared verbindt hiermee in plaats van de HTTPS-poort — geen certificaatverificatie nodig. De poort wordt automatisch toegewezen.",
        "no": "Åpner en enkel HTTP-port kun på 127.0.0.1. cloudflared kobler til denne i stedet for HTTPS-porten — ingen sertifikatbekreftelse nødvendig. Porten tildeles automatisk.",
        "pl": "Otwiera zwykły port HTTP tylko na 127.0.0.1. cloudflared łączy się z nim zamiast z portem HTTPS — weryfikacja certyfikatu nie jest potrzebna. Port jest przypisywany automatycznie.",
        "pt": "Abre uma porta HTTP simples apenas em 127.0.0.1. cloudflared conecta-se a esta em vez da porta HTTPS — sem necessidade de verificação de certificado. A porta é atribuída automaticamente.",
        "sv": "Öppnar en vanlig HTTP-port endast på 127.0.0.1. cloudflared ansluter till denna istället för HTTPS-porten — ingen certifikatverifiering behövs. Porten tilldelas automatiskt.",
        "zh": "仅在 127.0.0.1 上打开普通 HTTP 端口。cloudflared 连接到此端口而不是 HTTPS 端口 — 无需证书验证。端口自动分配。"
    },
    "Scraped web pages are summarized through the central Helper LLM when it is enabled. The agent receives the cleaned summary instead of the full page content.<br><strong>Benefits:</strong> saves tokens, reduces duplicate helper calls and hardens the system against prompt injection from external content.": {
        "cs": "Seškrábované webové stránky jsou shrnuty přes centrální Helper LLM, pokud je povolen. Agent dostává vyčištěné shrnutí místo celého obsahu stránky.<br><strong>Výhody:</strong> šetří tokeny, snižuje duplicitní volání pomocníka a posiluje systém proti prompt injekci z externího obsahu.",
        "da": "Skrabede websider opsummeres gennem den centrale Helper LLM når den er aktiveret. Agenten modtager den rensede opsummering i stedet for det fulde sideindhold.<br><strong>Fordele:</strong> sparer tokens, reducerer duplikerede hjælperkald og hærder systemet mod prompt-injektion fra eksternt indhold.",
        "de": "Gescrapte Webseiten werden über das zentrale Helper-LLM zusammengefasst, wenn es aktiviert ist. Der Agent erhält die bereinigte Zusammenfassung statt des vollständigen Seiteninhalts.<br><strong>Vorteile:</strong> spart Tokens, reduziert doppelte Helper-Aufrufe und härtet das System gegen Prompt-Injection aus externen Inhalten.",
        "el": "Οι σελίδες ιστού που έχουν αποκοπεί συνοψίζονται μέσω του κεντρικού Helper LLM όταν είναι ενεργοποιημένο. Ο πράκτορας λαμβάνει την καθαρή σύνοψη αντί για το πλήρες περιεχόμενο της σελίδας.<br><strong>Οφέλη:</strong> εξοικονομεί tokens, μειώνει διπλές κλήσεις βοηθού και θωρακίζει το σύστημα ενάντια σε prompt injection από εξωτερικό περιεχόμενο.",
        "es": "Las páginas web scrapeadas se resumen a través del LLM Helper central cuando está habilitado. El agente recibe el resumen limpio en lugar del contenido completo de la página.<br><strong>Beneficios:</strong> ahorra tokens, reduce llamadas duplicadas de helper y endurece el sistema contra inyección de prompts desde contenido externo.",
        "fr": "Les pages web scrapées sont résumées via le LLM Helper centralisé lorsqu'il est activé. L'agent reçoit le résumé nettoyé au lieu du contenu complet de la page.<br><strong>Avantages :</strong> économise des tokens, réduit les appels d'assistant en double et renforce le système contre l'injection de prompts à partir de contenu externe.",
        "hi": "स्क्रैप किए गए वेब पेज केंद्रीय सहायक LLM के माध्यम से सारांशित किए जाते हैं जब यह सक्षम हो। एजेंट पूर्ण पृष्ठ सामग्री के बजाय साफ़ सारांश प्राप्त करता है।<br><strong>लाभ:</strong> टोकन बचाता है, डुप्लिकेट हेल्पर कॉल कम करता है और बाहरी सामग्री से प्रॉम्प्ट इंजेक्शन के खिलाफ सिस्टम को मजबूत करता है।",
        "it": "Le pagine web scrape vengono riassunte tramite l'LLM Helper centrale quando è abilitato. L'agente riceve il riassunto pulito invece del contenuto completo della pagina.<br><strong>Benefici:</strong> risparmia token, riduce le chiamate helper duplicate e rende più robusto il sistema contro l'iniezione di prompt da contenuto esterno.",
        "ja": "スクレイプされたWebページは、有効な場合、中央Helper LLMを通じて要約されます。エージェントはページ全体の代わりにクリーンな要約を受け取ります。<br><strong>利点：</strong>トークンを節約し、重複ヘルパー呼び出しを削減し、外部コンテンツからのプロンプトインジェクションに対してシステムを強化します。",
        "nl": "Gescrapte webpagina's worden samengevat via de centrale Helper LLM indien ingeschakeld. De agent ontvangt de opgeschoonde samenvatting in plaats van de volledige pagina-inhoud.<br><strong>Voordelen:</strong> bespaart tokens, vermindert dubbele helper-aanroepen en verhardt het systeem tegen prompt-injectie vanuit externe inhoud.",
        "no": "Skrapte nettsider oppsummeres gjennom den sentrale Helper LLM når den er aktivert. Agenten mottar den rensede oppsummeringen i stedet for hele sideinnholdet.<br><strong>Fordeler:</strong> sparer tokens, reduserer duplikate hjelperkall og herder systemet mot prompt-injeksjon fra eksternt innhold.",
        "pl": "Zeskrapowane strony internetowe są podsumowywane przez centralny Helper LLM, gdy jest włączony. Agent otrzymuje oczyszczone podsumowanie zamiast pełnej treści strony.<br><strong>Korzyści:</strong> oszczędza tokeny, zmniejsza zduplikowane wywołania pomocnika i wzmacnia system przeciwko iniekcji promptów z zewnętrznej zawartości.",
        "pt": "Páginas web extraídas são resumidas através do LLM Helper central quando ativado. O agente recebe o resumo limpo em vez do conteúdo completo da página.<br><strong>Benefícios:</strong> economiza tokens, reduz chamadas duplicadas de helper e fortalece o sistema contra injeção de prompts de conteúdo externo.",
        "sv": "Skrapade webbsidor sammanfattas via den centrala Helper LLM när den är aktiverad. Agenten får den rensade sammanfattningen istället för hela sidinnehållet.<br><strong>Fördelar:</strong> sparar tokens, minskar duplicerade hjälparanrop och härdar systemet mot prompt-injektion från externt innehåll.",
        "zh": "抓取的网页在启用中央 Helper LLM 时会被摘要。代理接收清理后的摘要而不是完整的页面内容。<br><strong>好处：</strong>节省令牌，减少重复的辅助调用，并加强系统防止来自外部内容的提示注入。"
    },
    "When enabled, scraped content is summarized through the central Helper LLM. This reduces tokens, batches helper work where possible, and hardens against prompt injection.": {
        "cs": "Pokud je povoleno, seškrábovaný obsah je shrnut přes centrální Helper LLM. To snižuje tokeny, sdružuje práci pomocníka kde je to možné a posiluje proti prompt injekci.",
        "da": "Når aktiveret, opsummeres skrabet indhold gennem den centrale Helper LLM. Dette reducerer tokens, batcher hjælpearbejde hvor muligt og hærder mod prompt-injektion.",
        "de": "Wenn aktiviert, wird gescrappter Inhalt über das zentrale Helper-LLM zusammengefasst. Dies reduziert Tokens, bündelt Helper-Arbeiten wo möglich und härtet gegen Prompt-Injection.",
        "el": "Όταν ενεργοποιηθεί, το αποκομμένο περιεχόμενο συνοψίζεται μέσω του κεντρικού Helper LLM. Αυτό μειώνει τα tokens, ομαδοποιεί τη βοηθητική εργασία όπου είναι δυνατό και θωρακίζει ενάντια στην prompt injection.",
        "es": "Cuando está habilitado, el contenido scrapeado se resume a través del LLM Helper central. Esto reduce tokens, procesa por lotes el trabajo del helper y endurece contra la inyección de prompts.",
        "fr": "Lorsqu'activé, le contenu scrapé est résumé via le LLM Helper centralisé. Cela réduit les tokens, regroupe le travail d'assistance lorsque possible et renforce contre l'injection de prompts.",
        "hi": "सक्षम होने पर, स्क्रैप की गई सामग्री केंद्रीय सहायक LLM के माध्यम से सारांशित होती है। यह टोकन कम करता है, जहाँ संभव हो सहायक कार्य बैच करता है, और प्रॉम्प्ट इंजेक्शन के खिलाफ मजबूत करता है।",
        "it": "Quando abilitato, il contenuto scrape viene riassunto tramite l'LLM Helper centrale. Questo riduce i token, elabora in batch il lavoro helper ove possibile e rende più robusto contro l'iniezione di prompt.",
        "ja": "有効な場合、スクレイプされたコンテンツは中央Helper LLMを通じて要約されます。これによりトークンが削減され、可能な限りヘルパー作業がバッチ処理され、プロンプトインジェクションに対して強化されます。",
        "nl": "Indien ingeschakeld, wordt gescrapte inhoud samengevat via de centrale Helper LLM. Dit vermindert tokens, batcht helperwerk waar mogelijk en verhardt tegen prompt-injectie.",
        "no": "Når aktivert, oppsummeres skrapet innhold gjennom den sentrale Helper LLM. Dette reduserer tokens, batcher hjelpearbeid der det er mulig og herder mot prompt-injeksjon.",
        "pl": "Po włączeniu, zeskrapana treść jest podsumowywana przez centralny Helper LLM. Zmniejsza to tokeny, grupuje pracę pomocnika tam gdzie to możliwe i wzmacnia ochronę przed iniekcją promptów.",
        "pt": "Quando ativado, o conteúdo extraído é resumido através do LLM Helper central. Isso reduz tokens, processa em lote o trabalho do helper quando possível e fortalece contra injeção de prompts.",
        "sv": "När aktiverat sammanfattas skrapat innehåll via den centrala Helper LLM. Detta minskar tokens, batchar hjälparbete där möjligt och härdar mot prompt-injektion.",
        "zh": "启用时，抓取的内容通过中央 Helper LLM 进行摘要。这减少了令牌，尽可能批量处理辅助工作，并加强防止提示注入。"
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

print(f"Batch 22: Fixed {fixed} keys")

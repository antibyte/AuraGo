# Part F

Tasks of batch F. Rules, commands and the order are in the plan index.

<!-- group F -->

### Task F1: Config page and 16 locales
**Batch:** F  **Depends on:** A1
**Files:**
Modify `ui/cfg/virtual_desktop.js`
Modify `ui/lang/meta.json`
Modify `ui/lang/config/common/cs.json`
Modify `ui/lang/config/common/da.json`
Modify `ui/lang/config/common/de.json`
Modify `ui/lang/config/common/el.json`
Modify `ui/lang/config/common/en.json`
Modify `ui/lang/config/common/es.json`
Modify `ui/lang/config/common/fr.json`
Modify `ui/lang/config/common/hi.json`
Modify `ui/lang/config/common/it.json`
Modify `ui/lang/config/common/ja.json`
Modify `ui/lang/config/common/nl.json`
Modify `ui/lang/config/common/no.json`
Modify `ui/lang/config/common/pl.json`
Modify `ui/lang/config/common/pt.json`
Modify `ui/lang/config/common/sv.json`
Modify `ui/lang/config/common/zh.json`
Test `ui/config_storymaker_test.go`

- [ ] **Step 1: Impact.** Run impact on `vdCfgStorymaker` and `TestConfigStorymakerBlockIsWired`. Report callers and risk.

- [ ] **Step 2: Add the agent block to the config form.** `ui/cfg/virtual_desktop.js` is CRLF in a Windows checkout, so keep CRLF on the new lines. The JSON and Go files stay LF.

**Edit** `ui/cfg/virtual_desktop.js`
Old:
```js
    const narration = data.narration || (data.narration = {});
```
New:
```js
    const narration = data.narration || (data.narration = {});
    const agent = data.agent || (data.agent = {});
    if (typeof agent.enabled !== 'boolean') agent.enabled = true;
```

**Edit** `ui/cfg/virtual_desktop.js`
Old:
```js
    html += '</div><div class="field-group"><button type="button" id="vd-storymaker-btn" class="btn-save dc-test-btn" data-config-action-draft="false" onclick="vdCfgTestStorymaker(this)">' + t(prefix + 'check') + '</button>' + cfgActionResult.html('vd-storymaker-result') + '</div>';
```
New:
```js
    html += '</div><div class="field-group"><button type="button" id="vd-storymaker-btn" class="btn-save dc-test-btn" data-config-action-draft="false" onclick="vdCfgTestStorymaker(this)">' + t(prefix + 'check') + '</button>' + cfgActionResult.html('vd-storymaker-result') + '</div>';
    html += '<h3>' + t(prefix + 'agent_title') + '</h3><p class="field-help">' + t(prefix + 'agent_help') + '</p>';
    html += '<div class="field-grid two-cols">';
    html += vdCfgToggleRow(prefix + 'agent_enabled', '', agent.enabled === true, 'storymaker.agent.enabled');
    html += '</div><div class="field-grid two-cols">';
    html += vdCfgField(prefix + 'agent_provider', prefix + 'agent_provider_help', '<select class="field-select" data-path="storymaker.agent.provider" data-config-choice="providers" aria-label="' + escapeAttr(t(prefix + 'agent_provider')) + '">' + cfgChoiceOptionsHTML('providers', helpTexts['storymaker.agent.provider'] || {}, agent.provider || '') + '</select>' + cfgChoiceHintHTML('providers'));
    html += vdCfgField(prefix + 'agent_model', prefix + 'agent_model_help', '<input class="field-input" type="text" spellcheck="false" autocomplete="off" value="' + escapeAttr(agent.model || '') + '" data-path="storymaker.agent.model">');
    html += vdCfgField(prefix + 'agent_timeout', prefix + 'agent_timeout_help', '<input class="field-input" type="number" min="120" max="7200" value="' + escapeAttr(String(agent.job_timeout_seconds || 1800)) + '" data-path="storymaker.agent.job_timeout_seconds">');
    html += vdCfgField(prefix + 'agent_max_length', prefix + 'agent_max_length_help', '<input class="field-input" type="number" min="15" max="3600" value="' + escapeAttr(String(agent.max_length_seconds || 180)) + '" data-path="storymaker.agent.max_length_seconds">');
    html += vdCfgField(prefix + 'agent_max_daily_jobs', prefix + 'agent_max_daily_jobs_help', '<input class="field-input" type="number" min="1" max="1000" value="' + escapeAttr(String(agent.max_daily_jobs || 10)) + '" data-path="storymaker.agent.max_daily_jobs">');
    html += '</div>';
```

- [ ] **Step 3: Name the empty provider option.** `refreshConfigChoiceSelects` (`ui/js/config/main.js:2736`) rebuilds the dropdown from `helpTexts`. The label therefore has to live in `lang/meta.json`, or a refresh turns it into "no provider".

**Edit** `ui/lang/meta.json`
Old:
```json
  "help.cyd_creator.model": {
```
New:
```json
  "help.storymaker.agent.provider": {
    "provider_ref": true,
    "empty_label_key": "config.storymaker.agent_provider_main"
  },
  "help.cyd_creator.model": {
```

- [ ] **Step 4: Add the texts in all 16 locales.**

**Edit** `ui/lang/config/common/en.json`
Old:
```json
  "config.refresh.load_error": "This section could not be loaded.",
```
New:
```json
  "config.storymaker.agent_title": "Storymaker agent",
  "config.storymaker.agent_help": "The agent writes a film from a topic, in the app or from chat. Its films are normal projects that you can edit and export.",
  "config.storymaker.agent_enabled": "Enable the Storymaker agent",
  "config.storymaker.agent_provider": "Agent provider",
  "config.storymaker.agent_provider_help": "The LLM provider for agent jobs. Main LLM uses the main route with its configured failover. A chosen provider is pinned to the job without failover.",
  "config.storymaker.agent_provider_main": "Main LLM",
  "config.storymaker.agent_model": "Agent model (optional)",
  "config.storymaker.agent_model_help": "Replaces the model for agent jobs. Empty means the model of the provider.",
  "config.storymaker.agent_timeout": "Agent time limit (seconds)",
  "config.storymaker.agent_timeout_help": "A job that runs longer stops, and its valid scenes stay. Between 120 and 7200.",
  "config.storymaker.agent_max_length": "Longest agent film (seconds)",
  "config.storymaker.agent_max_length_help": "The agent shortens a longer request to this length. The longest video setting also applies. Between 15 and 3600.",
  "config.storymaker.agent_max_daily_jobs": "Agent jobs per day",
  "config.storymaker.agent_max_daily_jobs_help": "New films and revisions from the app and chat, plus exports from chat. Between 1 and 1000.",
  "config.refresh.load_error": "This section could not be loaded.",
```

**Edit** `ui/lang/config/common/de.json`
Old:
```json
  "config.refresh.load_error": "Dieser Bereich konnte nicht geladen werden.",
```
New:
```json
  "config.storymaker.agent_title": "Storymaker-Agent",
  "config.storymaker.agent_help": "Der Agent schreibt einen Film aus einem Thema, in der App oder aus dem Chat. Seine Filme sind normale Projekte, die du bearbeiten und exportieren kannst.",
  "config.storymaker.agent_enabled": "Storymaker-Agent aktivieren",
  "config.storymaker.agent_provider": "Agent-Anbieter",
  "config.storymaker.agent_provider_help": "Der LLM-Anbieter für Agent-Aufträge. Haupt-LLM nutzt die Hauptroute mit ihrem eingerichteten Failover. Ein gewählter Anbieter bleibt für den Auftrag fest, ohne Failover.",
  "config.storymaker.agent_provider_main": "Haupt-LLM",
  "config.storymaker.agent_model": "Agent-Modell (optional)",
  "config.storymaker.agent_model_help": "Ersetzt das Modell für Agent-Aufträge. Leer heißt: das Modell des Anbieters.",
  "config.storymaker.agent_timeout": "Agent-Zeitlimit (Sekunden)",
  "config.storymaker.agent_timeout_help": "Ein längerer Auftrag stoppt, seine gültigen Szenen bleiben erhalten. Zwischen 120 und 7200.",
  "config.storymaker.agent_max_length": "Längster Agent-Film (Sekunden)",
  "config.storymaker.agent_max_length_help": "Der Agent kürzt eine längere Anfrage auf diese Länge. Die Einstellung für das längste Video gilt zusätzlich. Zwischen 15 und 3600.",
  "config.storymaker.agent_max_daily_jobs": "Agent-Aufträge pro Tag",
  "config.storymaker.agent_max_daily_jobs_help": "Neue Filme und Überarbeitungen aus App und Chat, dazu Exporte aus dem Chat. Zwischen 1 und 1000.",
  "config.refresh.load_error": "Dieser Bereich konnte nicht geladen werden.",
```

**Edit** `ui/lang/config/common/cs.json`
Old:
```json
  "config.refresh.load_error": "Tuto sekci se nepodařilo načíst.",
```
New:
```json
  "config.storymaker.agent_title": "Agent Storymakeru",
  "config.storymaker.agent_help": "Agent napíše film z tématu, v aplikaci nebo z chatu. Jeho filmy jsou běžné projekty, které můžete upravovat a exportovat.",
  "config.storymaker.agent_enabled": "Zapnout agenta Storymakeru",
  "config.storymaker.agent_provider": "Poskytovatel agenta",
  "config.storymaker.agent_provider_help": "Poskytovatel LLM pro úlohy agenta. Hlavní LLM používá hlavní trasu s nastaveným failoverem. Zvolený poskytovatel je pro úlohu pevně daný, bez failoveru.",
  "config.storymaker.agent_provider_main": "Hlavní LLM",
  "config.storymaker.agent_model": "Model agenta (volitelné)",
  "config.storymaker.agent_model_help": "Nahradí model pro úlohy agenta. Prázdné znamená model poskytovatele.",
  "config.storymaker.agent_timeout": "Časový limit agenta (sekundy)",
  "config.storymaker.agent_timeout_help": "Déle běžící úloha se zastaví a její platné scény zůstanou. Od 120 do 7200.",
  "config.storymaker.agent_max_length": "Nejdelší film agenta (sekundy)",
  "config.storymaker.agent_max_length_help": "Agent zkrátí delší požadavek na tuto délku. Platí také nastavení nejdelšího videa. Od 15 do 3600.",
  "config.storymaker.agent_max_daily_jobs": "Úlohy agenta za den",
  "config.storymaker.agent_max_daily_jobs_help": "Nové filmy a úpravy z aplikace a chatu a také exporty z chatu. Od 1 do 1000.",
  "config.refresh.load_error": "Tuto sekci se nepodařilo načíst.",
```

**Edit** `ui/lang/config/common/da.json`
Old:
```json
  "config.refresh.load_error": "Dette område kunne ikke indlæses.",
```
New:
```json
  "config.storymaker.agent_title": "Storymaker-agent",
  "config.storymaker.agent_help": "Agenten skriver en film ud fra et emne, i appen eller fra chatten. Dens film er almindelige projekter, som du kan redigere og eksportere.",
  "config.storymaker.agent_enabled": "Aktivér Storymaker-agenten",
  "config.storymaker.agent_provider": "Agentudbyder",
  "config.storymaker.agent_provider_help": "LLM-udbyderen til agentjob. Primær LLM bruger hovedruten med dens opsatte failover. En valgt udbyder er fast for jobbet, uden failover.",
  "config.storymaker.agent_provider_main": "Primær LLM",
  "config.storymaker.agent_model": "Agentmodel (valgfri)",
  "config.storymaker.agent_model_help": "Erstatter modellen til agentjob. Tom betyder udbyderens model.",
  "config.storymaker.agent_timeout": "Agentens tidsgrænse (sekunder)",
  "config.storymaker.agent_timeout_help": "Et job, der kører længere, stopper, og dets gyldige scener bliver. Mellem 120 og 7200.",
  "config.storymaker.agent_max_length": "Længste agentfilm (sekunder)",
  "config.storymaker.agent_max_length_help": "Agenten forkorter en længere anmodning til denne længde. Indstillingen for længste video gælder også. Mellem 15 og 3600.",
  "config.storymaker.agent_max_daily_jobs": "Agentjob pr. dag",
  "config.storymaker.agent_max_daily_jobs_help": "Nye film og revisioner fra appen og chatten samt eksporter fra chatten. Mellem 1 og 1000.",
  "config.refresh.load_error": "Dette område kunne ikke indlæses.",
```

**Edit** `ui/lang/config/common/el.json`
Old:
```json
  "config.refresh.load_error": "Δεν ήταν δυνατή η φόρτωση αυτής της ενότητας.",
```
New:
```json
  "config.storymaker.agent_title": "Πράκτορας Storymaker",
  "config.storymaker.agent_help": "Ο πράκτορας γράφει μια ταινία από ένα θέμα, στην εφαρμογή ή από τη συνομιλία. Οι ταινίες του είναι κανονικά έργα που μπορείτε να επεξεργαστείτε και να εξαγάγετε.",
  "config.storymaker.agent_enabled": "Ενεργοποίηση πράκτορα Storymaker",
  "config.storymaker.agent_provider": "Πάροχος πράκτορα",
  "config.storymaker.agent_provider_help": "Ο πάροχος LLM για τις εργασίες του πράκτορα. Το κύριο LLM χρησιμοποιεί την κύρια διαδρομή με το ρυθμισμένο failover της. Ένας επιλεγμένος πάροχος μένει σταθερός για την εργασία, χωρίς failover.",
  "config.storymaker.agent_provider_main": "Κύριο LLM",
  "config.storymaker.agent_model": "Μοντέλο πράκτορα (προαιρετικό)",
  "config.storymaker.agent_model_help": "Αντικαθιστά το μοντέλο για τις εργασίες του πράκτορα. Κενό σημαίνει το μοντέλο του παρόχου.",
  "config.storymaker.agent_timeout": "Χρονικό όριο πράκτορα (δευτερόλεπτα)",
  "config.storymaker.agent_timeout_help": "Μια εργασία που διαρκεί περισσότερο σταματά και οι έγκυρες σκηνές της μένουν. Από 120 έως 7200.",
  "config.storymaker.agent_max_length": "Μεγαλύτερη ταινία πράκτορα (δευτερόλεπτα)",
  "config.storymaker.agent_max_length_help": "Ο πράκτορας συντομεύει ένα μεγαλύτερο αίτημα σε αυτή τη διάρκεια. Ισχύει επίσης η ρύθμιση για το μεγαλύτερο βίντεο. Από 15 έως 3600.",
  "config.storymaker.agent_max_daily_jobs": "Εργασίες πράκτορα ανά ημέρα",
  "config.storymaker.agent_max_daily_jobs_help": "Νέες ταινίες και αναθεωρήσεις από την εφαρμογή και τη συνομιλία, καθώς και εξαγωγές από τη συνομιλία. Από 1 έως 1000.",
  "config.refresh.load_error": "Δεν ήταν δυνατή η φόρτωση αυτής της ενότητας.",
```

**Edit** `ui/lang/config/common/es.json`
Old:
```json
  "config.refresh.load_error": "No se pudo cargar esta sección.",
```
New:
```json
  "config.storymaker.agent_title": "Agente de Storymaker",
  "config.storymaker.agent_help": "El agente escribe una película a partir de un tema, en la aplicación o desde el chat. Sus películas son proyectos normales que puedes editar y exportar.",
  "config.storymaker.agent_enabled": "Activar el agente de Storymaker",
  "config.storymaker.agent_provider": "Proveedor del agente",
  "config.storymaker.agent_provider_help": "El proveedor de LLM para los trabajos del agente. LLM principal usa la ruta principal con su failover configurado. Un proveedor elegido queda fijado para el trabajo, sin failover.",
  "config.storymaker.agent_provider_main": "LLM principal",
  "config.storymaker.agent_model": "Modelo del agente (opcional)",
  "config.storymaker.agent_model_help": "Sustituye el modelo para los trabajos del agente. Vacío significa el modelo del proveedor.",
  "config.storymaker.agent_timeout": "Límite de tiempo del agente (segundos)",
  "config.storymaker.agent_timeout_help": "Un trabajo que dura más se detiene y sus escenas válidas se conservan. Entre 120 y 7200.",
  "config.storymaker.agent_max_length": "Película más larga del agente (segundos)",
  "config.storymaker.agent_max_length_help": "El agente acorta una petición más larga a esta duración. También se aplica el ajuste del vídeo más largo. Entre 15 y 3600.",
  "config.storymaker.agent_max_daily_jobs": "Trabajos del agente por día",
  "config.storymaker.agent_max_daily_jobs_help": "Películas nuevas y revisiones desde la aplicación y el chat, además de exportaciones desde el chat. Entre 1 y 1000.",
  "config.refresh.load_error": "No se pudo cargar esta sección.",
```

**Edit** `ui/lang/config/common/fr.json`
Old:
```json
  "config.refresh.load_error": "Impossible de charger cette section.",
```
New:
```json
  "config.storymaker.agent_title": "Agent Storymaker",
  "config.storymaker.agent_help": "L'agent écrit un film à partir d'un sujet, dans l'application ou depuis le chat. Ses films sont des projets normaux que vous pouvez modifier et exporter.",
  "config.storymaker.agent_enabled": "Activer l'agent Storymaker",
  "config.storymaker.agent_provider": "Fournisseur de l'agent",
  "config.storymaker.agent_provider_help": "Le fournisseur LLM des tâches de l'agent. LLM principal utilise la route principale avec son failover configuré. Un fournisseur choisi reste fixé pour la tâche, sans failover.",
  "config.storymaker.agent_provider_main": "LLM principal",
  "config.storymaker.agent_model": "Modèle de l'agent (facultatif)",
  "config.storymaker.agent_model_help": "Remplace le modèle des tâches de l'agent. Vide signifie le modèle du fournisseur.",
  "config.storymaker.agent_timeout": "Limite de temps de l'agent (secondes)",
  "config.storymaker.agent_timeout_help": "Une tâche plus longue s'arrête et ses scènes valides sont conservées. Entre 120 et 7200.",
  "config.storymaker.agent_max_length": "Film le plus long de l'agent (secondes)",
  "config.storymaker.agent_max_length_help": "L'agent ramène une demande plus longue à cette durée. Le réglage de la vidéo la plus longue s'applique aussi. Entre 15 et 3600.",
  "config.storymaker.agent_max_daily_jobs": "Tâches de l'agent par jour",
  "config.storymaker.agent_max_daily_jobs_help": "Nouveaux films et révisions depuis l'application et le chat, ainsi que les exports depuis le chat. Entre 1 et 1000.",
  "config.refresh.load_error": "Impossible de charger cette section.",
```

**Edit** `ui/lang/config/common/hi.json`
Old:
```json
  "config.refresh.load_error": "यह अनुभाग लोड नहीं हो सका।",
```
New:
```json
  "config.storymaker.agent_title": "Storymaker एजेंट",
  "config.storymaker.agent_help": "एजेंट किसी विषय से फ़िल्म लिखता है, ऐप में या चैट से। उसकी फ़िल्में सामान्य प्रोजेक्ट हैं, जिन्हें आप संपादित और एक्सपोर्ट कर सकते हैं।",
  "config.storymaker.agent_enabled": "Storymaker एजेंट चालू करें",
  "config.storymaker.agent_provider": "एजेंट प्रदाता",
  "config.storymaker.agent_provider_help": "एजेंट जॉब के लिए LLM प्रदाता। मुख्य LLM अपने सेट किए गए failover के साथ मुख्य रूट का उपयोग करता है। चुना गया प्रदाता जॉब के लिए तय रहता है, बिना failover के।",
  "config.storymaker.agent_provider_main": "मुख्य LLM",
  "config.storymaker.agent_model": "एजेंट मॉडल (वैकल्पिक)",
  "config.storymaker.agent_model_help": "एजेंट जॉब के लिए मॉडल बदलता है। ख़ाली का अर्थ है प्रदाता का मॉडल।",
  "config.storymaker.agent_timeout": "एजेंट समय सीमा (सेकंड)",
  "config.storymaker.agent_timeout_help": "इससे लंबा चलने वाला जॉब रुक जाता है और उसके मान्य दृश्य बने रहते हैं। 120 से 7200 के बीच।",
  "config.storymaker.agent_max_length": "एजेंट की सबसे लंबी फ़िल्म (सेकंड)",
  "config.storymaker.agent_max_length_help": "एजेंट लंबे अनुरोध को इस अवधि तक छोटा करता है। सबसे लंबे वीडियो की सेटिंग भी लागू होती है। 15 से 3600 के बीच।",
  "config.storymaker.agent_max_daily_jobs": "प्रति दिन एजेंट जॉब",
  "config.storymaker.agent_max_daily_jobs_help": "ऐप और चैट से नई फ़िल्में और संशोधन, साथ ही चैट से एक्सपोर्ट। 1 से 1000 के बीच।",
  "config.refresh.load_error": "यह अनुभाग लोड नहीं हो सका।",
```

**Edit** `ui/lang/config/common/it.json`
Old:
```json
  "config.refresh.load_error": "Impossibile caricare questa sezione.",
```
New:
```json
  "config.storymaker.agent_title": "Agente di Storymaker",
  "config.storymaker.agent_help": "L'agente scrive un film a partire da un argomento, nell'app o dalla chat. I suoi film sono normali progetti che puoi modificare ed esportare.",
  "config.storymaker.agent_enabled": "Attiva l'agente di Storymaker",
  "config.storymaker.agent_provider": "Provider dell'agente",
  "config.storymaker.agent_provider_help": "Il provider LLM per i processi dell'agente. LLM principale usa il percorso principale con il failover configurato. Un provider scelto resta fisso per il processo, senza failover.",
  "config.storymaker.agent_provider_main": "LLM principale",
  "config.storymaker.agent_model": "Modello dell'agente (facoltativo)",
  "config.storymaker.agent_model_help": "Sostituisce il modello per i processi dell'agente. Vuoto significa il modello del provider.",
  "config.storymaker.agent_timeout": "Limite di tempo dell'agente (secondi)",
  "config.storymaker.agent_timeout_help": "Un processo più lungo si ferma e le sue scene valide restano. Tra 120 e 7200.",
  "config.storymaker.agent_max_length": "Film più lungo dell'agente (secondi)",
  "config.storymaker.agent_max_length_help": "L'agente riduce una richiesta più lunga a questa durata. Vale anche l'impostazione del video più lungo. Tra 15 e 3600.",
  "config.storymaker.agent_max_daily_jobs": "Processi dell'agente al giorno",
  "config.storymaker.agent_max_daily_jobs_help": "Nuovi film e revisioni dall'app e dalla chat, più le esportazioni dalla chat. Tra 1 e 1000.",
  "config.refresh.load_error": "Impossibile caricare questa sezione.",
```

**Edit** `ui/lang/config/common/ja.json`
Old:
```json
  "config.refresh.load_error": "このセクションを読み込めませんでした。",
```
New:
```json
  "config.storymaker.agent_title": "Storymaker エージェント",
  "config.storymaker.agent_help": "エージェントはテーマから映像を作ります。アプリからでもチャットからでも使えます。作品は通常のプロジェクトなので、編集も書き出しもできます。",
  "config.storymaker.agent_enabled": "Storymaker エージェントを有効にする",
  "config.storymaker.agent_provider": "エージェントのプロバイダー",
  "config.storymaker.agent_provider_help": "エージェントジョブの LLM プロバイダーです。メイン LLM は設定済みのフェイルオーバーを含むメインルートを使います。選んだプロバイダーはジョブ中固定され、フェイルオーバーしません。",
  "config.storymaker.agent_provider_main": "メイン LLM",
  "config.storymaker.agent_model": "エージェントのモデル (任意)",
  "config.storymaker.agent_model_help": "エージェントジョブのモデルを置き換えます。空欄ならプロバイダーのモデルを使います。",
  "config.storymaker.agent_timeout": "エージェントの制限時間 (秒)",
  "config.storymaker.agent_timeout_help": "これより長いジョブは停止し、有効なシーンは残ります。120〜7200。",
  "config.storymaker.agent_max_length": "エージェント作品の最大長 (秒)",
  "config.storymaker.agent_max_length_help": "これより長い依頼はこの長さに短縮します。動画の最大長の設定も適用されます。15〜3600。",
  "config.storymaker.agent_max_daily_jobs": "1 日あたりのエージェントジョブ",
  "config.storymaker.agent_max_daily_jobs_help": "アプリとチャットからの新規作品と修正、およびチャットからの書き出しです。1〜1000。",
  "config.refresh.load_error": "このセクションを読み込めませんでした。",
```

**Edit** `ui/lang/config/common/nl.json`
Old:
```json
  "config.refresh.load_error": "Dit onderdeel kon niet worden geladen.",
```
New:
```json
  "config.storymaker.agent_title": "Storymaker-agent",
  "config.storymaker.agent_help": "De agent schrijft een film op basis van een onderwerp, in de app of vanuit de chat. Zijn films zijn gewone projecten die je kunt bewerken en exporteren.",
  "config.storymaker.agent_enabled": "Storymaker-agent inschakelen",
  "config.storymaker.agent_provider": "Agentprovider",
  "config.storymaker.agent_provider_help": "De LLM-provider voor agenttaken. Hoofd-LLM gebruikt de hoofdroute met de ingestelde failover. Een gekozen provider blijft vast voor de taak, zonder failover.",
  "config.storymaker.agent_provider_main": "Hoofd-LLM",
  "config.storymaker.agent_model": "Agentmodel (optioneel)",
  "config.storymaker.agent_model_help": "Vervangt het model voor agenttaken. Leeg betekent het model van de provider.",
  "config.storymaker.agent_timeout": "Tijdslimiet van de agent (seconden)",
  "config.storymaker.agent_timeout_help": "Een taak die langer duurt stopt, en de geldige scènes blijven bewaard. Tussen 120 en 7200.",
  "config.storymaker.agent_max_length": "Langste agentfilm (seconden)",
  "config.storymaker.agent_max_length_help": "De agent kort een langer verzoek in tot deze lengte. De instelling voor de langste video geldt ook. Tussen 15 en 3600.",
  "config.storymaker.agent_max_daily_jobs": "Agenttaken per dag",
  "config.storymaker.agent_max_daily_jobs_help": "Nieuwe films en herzieningen vanuit de app en de chat, plus exports vanuit de chat. Tussen 1 en 1000.",
  "config.refresh.load_error": "Dit onderdeel kon niet worden geladen.",
```

**Edit** `ui/lang/config/common/no.json`
Old:
```json
  "config.refresh.load_error": "Denne delen kunne ikke lastes inn.",
```
New:
```json
  "config.storymaker.agent_title": "Storymaker-agent",
  "config.storymaker.agent_help": "Agenten skriver en film ut fra et emne, i appen eller fra chatten. Filmene er vanlige prosjekter som du kan redigere og eksportere.",
  "config.storymaker.agent_enabled": "Aktiver Storymaker-agenten",
  "config.storymaker.agent_provider": "Agentleverandør",
  "config.storymaker.agent_provider_help": "LLM-leverandøren for agentjobber. Hoved-LLM bruker hovedruten med den oppsatte failoveren. En valgt leverandør er fast for jobben, uten failover.",
  "config.storymaker.agent_provider_main": "Hoved-LLM",
  "config.storymaker.agent_model": "Agentmodell (valgfritt)",
  "config.storymaker.agent_model_help": "Erstatter modellen for agentjobber. Tom betyr leverandørens modell.",
  "config.storymaker.agent_timeout": "Agentens tidsgrense (sekunder)",
  "config.storymaker.agent_timeout_help": "En jobb som varer lenger, stopper, og de gyldige scenene blir værende. Mellom 120 og 7200.",
  "config.storymaker.agent_max_length": "Lengste agentfilm (sekunder)",
  "config.storymaker.agent_max_length_help": "Agenten korter ned en lengre forespørsel til denne lengden. Innstillingen for lengste video gjelder også. Mellom 15 og 3600.",
  "config.storymaker.agent_max_daily_jobs": "Agentjobber per dag",
  "config.storymaker.agent_max_daily_jobs_help": "Nye filmer og revisjoner fra appen og chatten, pluss eksporter fra chatten. Mellom 1 og 1000.",
  "config.refresh.load_error": "Denne delen kunne ikke lastes inn.",
```

**Edit** `ui/lang/config/common/pl.json`
Old:
```json
  "config.refresh.load_error": "Nie udało się załadować tej sekcji.",
```
New:
```json
  "config.storymaker.agent_title": "Agent Storymakera",
  "config.storymaker.agent_help": "Agent pisze film na podstawie tematu, w aplikacji lub z czatu. Jego filmy to zwykłe projekty, które możesz edytować i eksportować.",
  "config.storymaker.agent_enabled": "Włącz agenta Storymakera",
  "config.storymaker.agent_provider": "Dostawca agenta",
  "config.storymaker.agent_provider_help": "Dostawca LLM dla zadań agenta. Główny LLM używa głównej trasy ze skonfigurowanym failoverem. Wybrany dostawca jest stały dla zadania, bez failovera.",
  "config.storymaker.agent_provider_main": "Główny LLM",
  "config.storymaker.agent_model": "Model agenta (opcjonalnie)",
  "config.storymaker.agent_model_help": "Zastępuje model dla zadań agenta. Puste pole oznacza model dostawcy.",
  "config.storymaker.agent_timeout": "Limit czasu agenta (sekundy)",
  "config.storymaker.agent_timeout_help": "Dłuższe zadanie zatrzymuje się, a jego poprawne sceny zostają. Od 120 do 7200.",
  "config.storymaker.agent_max_length": "Najdłuższy film agenta (sekundy)",
  "config.storymaker.agent_max_length_help": "Agent skraca dłuższe polecenie do tej długości. Obowiązuje też ustawienie najdłuższego wideo. Od 15 do 3600.",
  "config.storymaker.agent_max_daily_jobs": "Zadania agenta dziennie",
  "config.storymaker.agent_max_daily_jobs_help": "Nowe filmy i poprawki z aplikacji i czatu oraz eksporty z czatu. Od 1 do 1000.",
  "config.refresh.load_error": "Nie udało się załadować tej sekcji.",
```

**Edit** `ui/lang/config/common/pt.json`
Old:
```json
  "config.refresh.load_error": "Não foi possível carregar esta seção.",
```
New:
```json
  "config.storymaker.agent_title": "Agente do Storymaker",
  "config.storymaker.agent_help": "O agente escreve um filme a partir de um tema, no aplicativo ou pelo chat. Os filmes dele são projetos normais que você pode editar e exportar.",
  "config.storymaker.agent_enabled": "Ativar o agente do Storymaker",
  "config.storymaker.agent_provider": "Provedor do agente",
  "config.storymaker.agent_provider_help": "O provedor de LLM para as tarefas do agente. LLM principal usa a rota principal com o failover configurado. Um provedor escolhido fica fixo para a tarefa, sem failover.",
  "config.storymaker.agent_provider_main": "LLM principal",
  "config.storymaker.agent_model": "Modelo do agente (opcional)",
  "config.storymaker.agent_model_help": "Substitui o modelo das tarefas do agente. Vazio significa o modelo do provedor.",
  "config.storymaker.agent_timeout": "Limite de tempo do agente (segundos)",
  "config.storymaker.agent_timeout_help": "Uma tarefa mais longa para, e as cenas válidas permanecem. Entre 120 e 7200.",
  "config.storymaker.agent_max_length": "Filme mais longo do agente (segundos)",
  "config.storymaker.agent_max_length_help": "O agente encurta um pedido mais longo para esta duração. A configuração do vídeo mais longo também se aplica. Entre 15 e 3600.",
  "config.storymaker.agent_max_daily_jobs": "Tarefas do agente por dia",
  "config.storymaker.agent_max_daily_jobs_help": "Filmes novos e revisões do aplicativo e do chat, além de exportações do chat. Entre 1 e 1000.",
  "config.refresh.load_error": "Não foi possível carregar esta seção.",
```

**Edit** `ui/lang/config/common/sv.json`
Old:
```json
  "config.refresh.load_error": "Det gick inte att läsa in det här avsnittet.",
```
New:
```json
  "config.storymaker.agent_title": "Storymaker-agent",
  "config.storymaker.agent_help": "Agenten skriver en film utifrån ett ämne, i appen eller från chatten. Filmerna är vanliga projekt som du kan redigera och exportera.",
  "config.storymaker.agent_enabled": "Aktivera Storymaker-agenten",
  "config.storymaker.agent_provider": "Agentleverantör",
  "config.storymaker.agent_provider_help": "LLM-leverantören för agentjobb. Huvud-LLM använder huvudrutten med dess inställda failover. En vald leverantör är fast för jobbet, utan failover.",
  "config.storymaker.agent_provider_main": "Huvud-LLM",
  "config.storymaker.agent_model": "Agentmodell (valfritt)",
  "config.storymaker.agent_model_help": "Ersätter modellen för agentjobb. Tomt betyder leverantörens modell.",
  "config.storymaker.agent_timeout": "Agentens tidsgräns (sekunder)",
  "config.storymaker.agent_timeout_help": "Ett jobb som tar längre tid stoppas, och dess giltiga scener finns kvar. Mellan 120 och 7200.",
  "config.storymaker.agent_max_length": "Längsta agentfilm (sekunder)",
  "config.storymaker.agent_max_length_help": "Agenten kortar en längre begäran till den här längden. Inställningen för längsta video gäller också. Mellan 15 och 3600.",
  "config.storymaker.agent_max_daily_jobs": "Agentjobb per dag",
  "config.storymaker.agent_max_daily_jobs_help": "Nya filmer och revideringar från appen och chatten, plus exporter från chatten. Mellan 1 och 1000.",
  "config.refresh.load_error": "Det gick inte att läsa in det här avsnittet.",
```

**Edit** `ui/lang/config/common/zh.json`
Old:
```json
  "config.refresh.load_error": "无法加载此部分。",
```
New:
```json
  "config.storymaker.agent_title": "Storymaker 智能体",
  "config.storymaker.agent_help": "智能体根据主题编写影片，可在应用内或通过聊天使用。它的影片是普通项目，你可以编辑和导出。",
  "config.storymaker.agent_enabled": "启用 Storymaker 智能体",
  "config.storymaker.agent_provider": "智能体提供商",
  "config.storymaker.agent_provider_help": "智能体任务使用的 LLM 提供商。主 LLM 使用主路由及其已配置的故障转移。选定的提供商在任务期间固定，不会故障转移。",
  "config.storymaker.agent_provider_main": "主 LLM",
  "config.storymaker.agent_model": "智能体模型（可选）",
  "config.storymaker.agent_model_help": "替换智能体任务使用的模型。留空表示使用提供商的模型。",
  "config.storymaker.agent_timeout": "智能体时间限制（秒）",
  "config.storymaker.agent_timeout_help": "运行更久的任务会停止，其有效场景会保留。范围 120 到 7200。",
  "config.storymaker.agent_max_length": "智能体影片最长时长（秒）",
  "config.storymaker.agent_max_length_help": "智能体会把更长的请求缩短到此时长。最长视频设置同样适用。范围 15 到 3600。",
  "config.storymaker.agent_max_daily_jobs": "每天的智能体任务数",
  "config.storymaker.agent_max_daily_jobs_help": "来自应用和聊天的新影片与修改，以及来自聊天的导出。范围 1 到 1000。",
  "config.refresh.load_error": "无法加载此部分。",
```

- [ ] **Step 5: Test.** Extend `TestConfigStorymakerBlockIsWired`. `TestConfigStorymakerTranslations` reads the same key list, so it covers the 16 locales with no change.

**Edit** `ui/config_storymaker_test.go`
Old:
```go
	"os"
```
New:
```go
	"encoding/json"
	"os"
```

**Edit** `ui/config_storymaker_test.go`
Old:
```go
// sixteen-language texts in lang/config/common.
```
New:
```go
// sixteen-language texts in lang/config/common. The agent settings are storymaker.agent.{enabled, provider, model,
// job_timeout_seconds, max_length_seconds, max_daily_jobs}.
```

**Edit** `ui/config_storymaker_test.go`
Old:
```go
	"part_ffmpeg", "part_browser", "max_daily_chars", "max_daily_chars_help", "max_job_chars", "max_job_chars_help",
```
New:
```go
	"part_ffmpeg", "part_browser", "max_daily_chars", "max_daily_chars_help", "max_job_chars", "max_job_chars_help",
	"agent_title", "agent_help", "agent_enabled", "agent_provider", "agent_provider_help", "agent_provider_main",
	"agent_model", "agent_model_help", "agent_timeout", "agent_timeout_help", "agent_max_length", "agent_max_length_help",
	"agent_max_daily_jobs", "agent_max_daily_jobs_help",
```

**Edit** `ui/config_storymaker_test.go`
Old:
```go
		"hasUnsavedConfigChanges()",
```
New:
```go
		"hasUnsavedConfigChanges()",
		"'storymaker.agent.enabled'",
		`data-path="storymaker.agent.provider" data-config-choice="providers"`,
		"cfgChoiceOptionsHTML('providers', helpTexts['storymaker.agent.provider'] || {}, agent.provider || '')",
		`data-path="storymaker.agent.model"`,
		`data-path="storymaker.agent.job_timeout_seconds"`,
		`min="120" max="7200"`,
		`data-path="storymaker.agent.max_length_seconds"`,
		`min="15" max="3600"`,
		`data-path="storymaker.agent.max_daily_jobs"`,
		`min="1" max="1000"`,
```

**Edit** `ui/config_storymaker_test.go`
Old:
```go
	// The limits of the form are the limits of NormalizeStorymakerConfig.
```
New:
```go
	var meta map[string]map[string]any
	if err := json.Unmarshal(mustReadUIFile(t, "lang/meta.json"), &meta); err != nil {
		t.Fatal(err)
	}
	if entry := meta["help.storymaker.agent.provider"]; entry["provider_ref"] != true || entry["empty_label_key"] != "config.storymaker.agent_provider_main" {
		t.Fatalf("lang/meta.json help.storymaker.agent.provider = %v", entry)
	}
	// The limits of the form are the limits of NormalizeStorymakerConfig.
```

**Edit** `ui/config_storymaker_test.go`
Old:
```go
	for _, limit := range []string{"60", "14400", "10", "3600", "1000", "100000000", "100", "10000000"} {
```
New:
```go
	for _, limit := range []string{"60", "14400", "10", "3600", "1000", "100000000", "100", "10000000", "120", "7200", "15"} {
```

The key `agent_provider_main` is used from `lang/meta.json`, which the test checks on its own, so the source loop skips it.

**Edit** `ui/config_storymaker_test.go`
Old:
```go
		if !strings.Contains(src, "'"+key+"'") && !strings.Contains(src, "'config.storymaker."+key+"'") {
```
New:
```go
		if key == "agent_provider_main" {
			continue
		}
		if !strings.Contains(src, "'"+key+"'") && !strings.Contains(src, "'config.storymaker."+key+"'") {
```

- [ ] **Step 6: Run.**
```bash
node --check ui/cfg/virtual_desktop.js
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'TestConfigStorymaker(BlockIsWired|Translations)$' -count=1
npm run build:ui
npm run check:ui
```
Expected: the syntax check prints nothing, both tests pass, and `check:ui` exits 0. `scripts/build-ui-bundles.js` has no `cfg/` entry, so the build should change no tracked file for this task.

- [ ] **Step 7: Commit.** Stage `ui/cfg/virtual_desktop.js`, `ui/lang/meta.json`, `ui/lang/config/common/*.json` and `ui/config_storymaker_test.go`. Message: `feat(config): add the Storymaker agent settings to the desktop page`.

### Task F2: Docs
**Batch:** F  **Depends on:** E4
**Files:**
Modify `docs/superpowers/specs/2026-11-01-storymaker-design.md`
Modify `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Modify `internal/storymaker/AGENTS.md`
Modify `ui/js/desktop/apps/AGENTS.md`
Modify `documentation/storymaker.md`
Modify `documentation/manuals/de/04-webui.md`
Modify `documentation/manuals/en/04-webui.md`

- [ ] **Step 1: Record the changes in the spec.**

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-design.md`
Old:
```markdown
- The preview's duck reaches -6 dB 50 ms before speech and the export reaches it at speech start. The preview has no carve, voice match or impact dips, as the plan's E2 text says.
```
New:
```markdown
- The preview's duck reaches -6 dB 50 ms before speech and the export reaches it at speech start. The preview has no carve, voice match or impact dips, as the plan's E2 text says.

## Changes from the agent core plan

The agent core is the first slice of the Agent section, ahead of the rest of phase 3. Its plan is `docs/superpowers/plans/2026-11-06-storymaker-agent-core/`.

**Left out of this slice:** `storymaker_code`, `storymaker_asset`, stills and review sheets, `checks.js`, three directions, the rhythm plan, the message-shape table, provenance, filmprints, the cold reviewer, typed parameters, point-to-edit comments and the Assets panel.

**Scope corrections**

| # | Correction | Reason |
| --- | --- | --- |
| C1 | A compile check in Chrome, `render.Runner.CompileCheck` | It is the only source of real scene lengths and motion lint. It stops after the report, needs no FFmpeg, is bounded to 60 s and never downloads Chromium. |
| C2 | A Go job lint with `scene_unfilled` | A scene with only an id and narration passes `Validate`, so a weak model could stop after the outline with a "valid" film of empty stages. |
| C3 | `POST /projects/{id}/make-silent` | Ticking `silent` by hand breaks the film: the scene falls to its minimum length, captions vanish and cues never start. |
| C4 | No `agent.max_concurrent_jobs` | `render.Manager` allows 0 or 1 workers. The limit is fixed at 1 running and 8 queued. |
| C5 | No `music` in `create` or `set_meta` | Generation stays the user's click. A story with generated music and no file would be refused at export. |
| C6 | Narration runs after the outcome is known, not during check rounds | The audio key contains the text, so each patch during fixes would bill cloud characters again. |
| C7 | Job tools stay out of the normal catalog and go through `ExecutionHooks.HandleTool` | They have no feature flag, category, search term, manual or dispatch case. |
| C8 | No voice is not a failure | The story keeps `voice: null`, and the job reports `narration_unavailable` as an open check. |

**Corrections found in review**

| # | Correction |
| --- | --- |
| R1 | The render and narrate refusals became functions without an `http.ResponseWriter`, so the job uses the same refusal order as the routes. |
| R2 | Chat `render` takes `silent: true`, runs the `MakeSilent` path and says that speech was removed. The `narration_incomplete` answer names this option. |
| R3 | After the final compile, when model-fixable issues remain and rounds are left, one more check loop runs and narration runs again. Only changed scenes are billed. |
| R4 | Render and agent exclude each other: a render answers 409 `agent_job_active`, and an agent start or revise answers 409 `busy` while a render, narrate or music job of the project is queued or running. |
| R5 | The runner does not set `cfg.LLM.UseNativeFunctions`. It reads the mode from the tooling policy and sets `req.ToolChoice` on native routes only. |
| R6 | The check and revise packets carry the same `cast` and `environments` blocks as the compact fill packet. |
| R7 | A loop is restarted at most once after a model error or an empty end. The tool-call limit counts tool calls, so a third outline call fits inside the limit of 6. |
| R8 | `max_daily_jobs` counts every create and revise, from the app and from chat, plus chat `render`. The counter is `<data>/storymaker/agent/usage.json` and fails closed. App routes answer 429 `daily_limit_reached`. |
| R9 | Each batch compares its failures with a baseline recorded on main. |
| R10 | `make-silent` first writes `story.before-silent.json`. `POST /projects/{id}/restore-speech` puts it back. Project reads carry `silent_backup: true` while the file exists. |

**Plan sections 7, 9 and 10.** Section 7 of the plan is the list R1 to R10 above. Section 9 adds the word check (`words_tell`: a dash or a stock phrase in narration or on-screen text gets one optional fix round as advice, and never blocks) and the revise lists (`changed_scenes`, `unchanged_scenes` and `removed_scenes`, in the job and in the chat status). Section 10 settles the seams between the code parts: one routing table for Studio operations, `over_export_limit` for a film longer than the export allows, and the bundle path `ui/js/desktop/bundles/main.bundle.js`.

**Plan section 11.** The review of the applied code added corrections V1 to V9, and an agent job has no token ceiling of its own: its spend is bounded by the job timeout, the tool-call limits per step and `max_daily_jobs`.

**Changed lines of this spec**
- Line 613: one running and eight queued agent jobs are fixed, and one job per project is enforced with `Coalesce` on a third `render.Manager`.
- Line 614: "no silent switch" holds for a pinned provider or model. Without a pin the job uses the main route with its configured failover, because turning that off would make jobs fail for no gain.
- Line 617: the compact profile hides `kind=voice` in cast. The first job step resolves the voice and writes it into the story.
- Line 619: compile runs in the check step. Narration runs after it, as C6 says.
- Line 622: a scene has a hard ceiling of 8 writes (`StudioSceneWriteLimit`). Tool calls per loop are outline 6, cast 12, fill 10, check 20 and revise 30. A failed or exhausted scene keeps its outline text, is recorded as `scene_unfilled`, and the job moves on.
- Line 628: `canonicalizeStorymakerOperationCall` only routes a call to its owner. Studio wraps flat fields into `data`. The recovery nudge lives in `internal/agent/game_maker_recovery_nudge.go` and `RunConfig.RecoveryNudge`.
- Lines 635-638: this slice never emits `ready`, because layout is not verified. `failed` means no outline was saved. `incomplete` means a timeout, a model error, or any unfilled scene, model-fixable issue or `compile_failed` left. `over_target` and environment items are listed and never downgrade. The job status is `failed` only for outcome `failed`.
- Line 640: `length_seconds` is clamped, not refused, so a chat call does not fail.
- Line 654: `action_policy.go:314` is now `:321`, `agent_dispatch_exec.go:226-234` is `:228-234`, and `dispatch_platform.go:312-313` is `:307-308`. The dispatch-switch edit is dropped, as C7 says.
- Line 662: `set_meta` has no `music`, as C5 says.
- Line 671: `ProjectStore.SetAgentMeta` writes `target_seconds`, `audience` and `template`.
- Line 681: the server narrates after the check, waits at most `min(300 s, time left)`, and reports the result as an open check. It never blocks the outcome.

**User edits during a job:** nothing merges. The editor is locked, and `PUT story`, `PUT code` and `POST narrate` answer 409 `agent_job_active`. Unsaved local text stays, and the existing conflict banner offers reload or overwrite after the job.

**Still open:** the real-model acceptance on aurago-test (task F4), real text-mode replies on nested `data`, whether weak models can write `{n}` markers, and the visual quality of films, because layout stays unchecked until `checks.js`.
```

- [ ] **Step 2: Update the contract.**

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
    Narration StorymakerNarrationConfig `yaml:"narration" json:"narration"`
```
New:
```text
    Agent   StorymakerAgentConfig  `yaml:"agent" json:"agent"`
    Narration StorymakerNarrationConfig `yaml:"narration" json:"narration"`
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
func NormalizeStorymakerConfig(c *StorymakerConfig) error   // defaults, ranges, path and transport; does not change Enabled
```
New:
```text
type StorymakerAgentConfig struct {
    Enabled           bool   `yaml:"enabled" json:"enabled"`                         // loader default true
    Provider          string `yaml:"provider" json:"provider"`                       // "" = main route
    Model             string `yaml:"model" json:"model"`                             // "" = the provider's model
    JobTimeoutSeconds int    `yaml:"job_timeout_seconds" json:"job_timeout_seconds"` // default 1800, 120..7200
    MaxLengthSeconds  int    `yaml:"max_length_seconds" json:"max_length_seconds"`   // default 180, 15..3600
    MaxDailyJobs      int    `yaml:"max_daily_jobs" json:"max_daily_jobs"`           // default 10, 1..1000
}

func NormalizeStorymakerConfig(c *StorymakerConfig) error   // defaults, ranges, path and transport; does not change Enabled
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
`timeout_seconds` is the minimum render timeout; the render raises it for long videos (section 7).
```
New:
```text
`timeout_seconds` is the minimum render timeout; the render raises it for long videos (section 7).

### Agent core

- Tools: `storymaker_story`, `storymaker_catalog` and `storymaker_check` (`ToolStory`, `ToolCatalog`, `ToolCheck`). They run only inside a job, through `ExecutionHooks.HandleTool`, and bind project and job on the server.
- Phases: `outline`, `cast`, `fill`, `check`, `revise`. `PhaseTools(phase)` and `PhaseOperations(phase, tool)` hold the table. An operation outside the phase answers `operation_not_in_phase`.
- Budget constants: `StudioSceneChargedLimit` 3, `StudioSceneWriteLimit` 8, `StudioDryRunsPerScene` 2, `StudioDryRunsPerJob` 20, `StudioCheckRounds` 6.
- Session: `NewStudio(deps StudioDeps, job StudioJob) (*Studio, error)`, with `SetPhase`, `Call`, `PhaseDone`, `Progress`, `Check`, `Packet` and `DropDrafts`.
- Story helpers: `DecodeScene`, `LintForJob`, `StoryIsBlank`, `MakeSilent` and `ProjectStore.SetAgentMeta`.
- Outcome: `DecideOutcome(OutcomeInput) (string, []AgentCheck)`. The values are `ready_unverified`, `incomplete` and `failed`. `ready` is reserved until layout is verified.
- Chat seam: `AgentFacade`, `SetAgentFacade` and `CurrentAgentFacade`. The server implements the facade, and the chat tool `storymaker` calls it. Its failures are `*AgentError{Code, Message, NextAction}`.
- Render: `KindAgent`, `CodeModelError`, `Handle.SetAgent` and `Runner.CompileCheck`. `prepareStory` is split into `prepareCore` and `checkAudioGates`.
- Agent: `StorymakerPhaseToolSchemas`, `canonicalizeStorymakerOperationCall` and `RunConfig.RecoveryNudge`.
- Files under `<data>/storymaker/agent/`: `jobs.json`, `usage.json` (fails closed) and `events/<job id>.jsonl` (at most 500 lines per job, newest 200 files kept, no arguments and no model text).
- Issue codes for the job only: `scene_unfilled`, `over_target`, `over_budget`, `compile_failed`, `over_export_limit`, `words_tell`. Tool refusals: `operation_not_in_phase`, `outline_locked`, `corrections_exhausted`, `dry_runs_exhausted`, `check_rounds_exhausted`, `rev_mismatch`, `scene_not_found`, `invalid_data`.
- Open checks: `layout_unverified` (always), `compile_unverified`, `narration_unavailable`, `narration_failed`, `narration_budget`, `narration_pending`, `corrections_exhausted`, `model_error`.
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
| `GET,HEAD /projects/{id}/audio/music/{key}.{ext}` | read | 200 or 206, Range honoured. Works read-only. | 404 `audio_not_found`, 405 |
```
New:
```text
| `GET,HEAD /projects/{id}/audio/music/{key}.{ext}` | read | 200 or 206, Range honoured. Works read-only. | 404 `audio_not_found`, 405 |
| `POST /agent/jobs` `{topic, audience?, length_seconds?, aspect?, language?, voice?, project_id?}` | write | 202 `{project_id, job}` | 400 `invalid_topic`; 403 `agent_disabled`, `storymaker_disabled`, `desktop_readonly`; 404 `project_not_found`; 409 `busy` (with `job_id`), `story_not_empty`, `llm_unavailable`; 422 `model_too_small`; 429 `busy`, `daily_limit_reached` |
| `POST /projects/{id}/agent/revise` `{instruction}` | write | 202 `{job}` | 400 `invalid_instruction`; 403 `agent_disabled`, `storymaker_disabled`, `desktop_readonly`; 404 `project_not_found`; 409 `busy`; 422 `invalid_story`; 429 `daily_limit_reached` |
| `POST /projects/{id}/make-silent` | write, `If-Match` | 200 `{etag, issues, story, changed}` | 428, 412, 409 `agent_job_active` |
| `POST /projects/{id}/restore-speech` | write, `If-Match` | 200 `{etag, issues, story}` | 428, 412, 404 `no_silent_backup`, 409 `agent_job_active` |
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
Render details:
```
New:
```text
Agent rules:

- `GET /jobs/{id}`, `POST /jobs/{id}/cancel`, `GET /projects/{id}/jobs` and the event stream also cover the agent queue, so these routes read across three queues.
- `PUT .../story`, `PUT .../scenes/{s}/code`, `POST .../narrate` and `POST .../render` answer 409 `agent_job_active` while an agent job of the project is queued or running. An agent start or revise answers 409 `busy` while a render, narrate or music job of the project is queued or running.
- Project reads carry `silent_backup: true` while `story.before-silent.json` exists.
- A job's status is `failed` only for outcome `failed`. A timeout and `incomplete` are `succeeded` with `agent.outcome`. A cancel stays `cancelled`.

Render details:
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
| `music_configured` | bool | `cfg.MusicConfigured()`: music generation can run. The music panel offers mode `generated` only then. |
```
New:
```text
| `music_configured` | bool | `cfg.MusicConfigured()`: music generation can run. The music panel offers mode `generated` only then. |
| `agent` | object | `enabled`, `available`, `provider`, `model`, `compact`, `text_mode`, `max_length_seconds`, `max_daily_jobs` |
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
Job {id, project_id, kind:"render"|"narrate"|"music", status, progress:0..1, phase?, frames_done, frames_total,
```
New:
```text
Job {id, project_id, kind:"render"|"narrate"|"music"|"agent", status, progress:0..1, phase?, frames_done, frames_total,
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
     warnings?:[{code, message}], error?:{code, message}, created_at, updated_at, started_at?, finished_at?}
```
New:
```text
     warnings?:[{code, message}], error?:{code, message}, agent?:AgentJobInfo, created_at, updated_at, started_at?, finished_at?}
AgentJobInfo {kind:"create"|"revise", outcome?, phase?:"outline"|"cast"|"fill"|"check"|"revise"|"narrating", scene_id?,
     scenes_done, scenes_total, story_etag?, open_checks?:[{code, scene_id?}], summary?, provider?, model?, compact?, text_mode?,
     target_seconds?, duration_seconds?, request?, tokens?, changed_scenes?, unchanged_scenes?, removed_scenes?}
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
`narration_incomplete`, `music_failed`, `music_incomplete`.
```
New:
```text
`narration_incomplete`, `music_failed`, `music_incomplete`, `model_error`.
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
4. A comment `: ping` every 15 seconds.
```
New:
```text
4. A comment `: ping` every 15 seconds.
5. `event: agent` with `data: {job_id, seq, at, type, ...}`. On connect the events of the unfinished job, or else of the latest agent job, are replayed (at most 500), and the panel dedupes by `seq`. The types are `phase` (`phase`, `scene_id?`, `attempt`), `tool` (`tool`, `operation`, `scene_id?`, `status`: `ok`, `draft`, `refused`, `error`), `model` (`status`: `waiting`, `receiving`, `retrying`, `recovering`), `usage` (`prompt_tokens`, `completion_tokens`, `total_tokens`, `estimated`), `check` (`issues`, `advice`, `rounds_left`, `compile`) and `outcome` (`outcome`).
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
the style `desktop-app-storymaker.css` and sixteen scripts in order:
```
New:
```text
the style `desktop-app-storymaker.css` and seventeen scripts in order:
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
`storymaker-handoff.js`, `storymaker.js`. Each file is under 1100 lines.
```
New:
```text
`storymaker-handoff.js`, `storymaker-agent.js`, `storymaker.js`. Each file is under 1100 lines.
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
The "Check export setup" action reads `GET /api/desktop/storymaker/status` for the saved settings only.
```
New:
```text
The "Check export setup" action reads `GET /api/desktop/storymaker/status` for the saved settings only. The agent block has the toggle for `storymaker.agent.enabled`, the provider dropdown `storymaker.agent.provider` (empty label through `lang/meta.json`), `storymaker.agent.model`, `storymaker.agent.job_timeout_seconds` (120 to 7200), `storymaker.agent.max_length_seconds` (15 to 3600) and `storymaker.agent.max_daily_jobs` (1 to 1000).
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
### Tests
```
New:
```text
### Agent screens

- `S.createAgentPanel(inst)` returns `{el, paint, dispose}`: prompt, length, language, Start or Revise, a progress list, Cancel, the outcome card, "Export without voice" and "Restore speech". The project list has a start form of its own.
- `api.events(projectId, onJob, onState, onAgent)` and the bus event `agent`. The panel reloads when `Job.agent.story_etag` differs from `inst.etag`, throttled to once per 2 s, and once more at the end.
- While an agent job of the project is active, `inst.agentLocked` makes `inst.readonly` true and a banner shows. `handleSaveError` treats `agent_job_active` as a conflict, so unsaved text stays.
- Launch: `render` reads `ctx.projectId`, and `window.StorymakerApp.open(windowId, projectId)` serves an open window. `sdk-events-bootstrap.js` passes `event.payload.project_id` only if it matches the UUID pattern of `ValidProjectID`, and only for `storymaker`.

### Tests
```

**Edit** `docs/superpowers/specs/2026-11-01-storymaker-contract.md`
Old:
```text
Phase 1 deviations from the spec are recorded in the spec's decisions and deviations, and are folded into `docs/superpowers/specs/2026-11-01-storymaker-design.md` when the plan is committed.
```
New:
```text
Phase 1 deviations from the spec are recorded in the spec's decisions and deviations, and are folded into `docs/superpowers/specs/2026-11-01-storymaker-design.md` when the plan is committed. The agent core's changes are in that file's section "Changes from the agent core plan".
```

- [ ] **Step 3: Update the package contracts.**

**Edit** `internal/storymaker/AGENTS.md`
Old:
```markdown
Agent voice discovery (`storymaker_catalog kind=voice`) is phase 3.
```
New:
```markdown
Agent voice discovery is `storymaker_catalog kind=voice` (section Agent core).
```

**Edit** `internal/storymaker/AGENTS.md`
Old:
```markdown
## Child DOX Index
```
New:
```markdown
## Agent core

- `studio*.go` hold the job tools and call no LLM: `studio.go` (phase table, session, budget, routing, read-only gate), `studio_args.go` (flat fields, JSON-string `data`, aliases), `studio_story.go` (operations and drafts), `studio_story_scenes.go` (scene writes and `DropDrafts`), `studio_catalog.go` (search, describe, voices), `studio_check.go` (`LintForJob`, `Check`, compile cache by etag), `studio_words.go` (`LintWords`, the word check), `studio_packet.go` (context packets). `agentjob.go` holds `AgentJobInfo`, `DecideOutcome`, `StoryIsBlank`, `MakeSilent`, the backup methods and `ChangedScenes`. `agentfacade.go` holds the chat facade types.
- The tools reach a job through a closure over its `*Studio` (`ExecutionHooks.HandleTool`), so a model cannot pick a project or a job. Schemas never carry an id the server binds. The tools stay out of the normal catalog: no feature flag, category, search term, manual or dispatch case. The chat tool `storymaker` is separate and calls `CurrentAgentFacade()`.
- A loop per phase, and a loop per scene in fill. The story is the checkpoint, so a job is never resumed: a later revise finishes it. An invalid write stays a per-scene draft and is dropped at the end. A failed or exhausted scene keeps its outline text and is recorded as `scene_unfilled`.
- Charging: a drop in the issue count, a first write and a new scene are free. Limits are `StudioSceneChargedLimit`, `StudioSceneWriteLimit`, `StudioDryRunsPerScene`, `StudioDryRunsPerJob` and `StudioCheckRounds`. `IssueAdvisory` lists the four codes and `words_tell` that get one optional fix loop and never block.
- `DecideOutcome` is the one rule for outcomes. This slice never emits `ready`, and `open_checks` always holds `layout_unverified`. `over_target` and environment items never downgrade.
- The backup methods of `ProjectStore` (`SaveSilentBackup`, `LoadSilentBackup`, `HasSilentBackup`, `RemoveSilentBackup`) keep `story.before-silent.json`. `MakeSilent` is pure: it sets `silent: true` and `duration = max(1, WordCount / 2.3 + 0.4)`, removes `cue`, and turns each `say` with text into a `show`. Without the duration the film collapses to about 1 s per scene. Ticking `silent` by hand has no such repair.
- `render.Runner.CompileCheck` reuses `snapshot`, the listener, the strict proxy and the page, captures the report and answers it with an error, so the page stops. It needs no FFmpeg, is bounded to 60 s and never downloads Chromium. Without a browser the job reports `compile_unverified` and still finishes.
- Narration runs after the check, because the audio key contains the text. No voice is not a failure: the story keeps `voice: null` and reports `narration_unavailable`.
- Verify: `go test ./internal/storymaker/... -count=1` and `go test ./internal/server -run Storymaker -count=1`. `TestStorymakerAgentScriptedJobNative` and `TestStorymakerAgentScriptedJobTextMode` run the whole job with a scripted model, and no test calls a real model or voice.

## Child DOX Index
```

**Edit** `ui/js/desktop/apps/AGENTS.md`
Old:
```markdown
- `storymaker*.js` are sixteen classic scripts in this load order
```
New:
```markdown
- `storymaker*.js` are seventeen classic scripts in this load order
```

**Edit** `ui/js/desktop/apps/AGENTS.md`
Old:
```markdown
`-handoff.js` (tracks and handoff), then the composition root `storymaker.js`
```
New:
```markdown
`-handoff.js` (tracks and handoff), `-agent.js` (agent panel and project-list start form), then the composition root `storymaker.js`
```

**Edit** `ui/js/desktop/apps/AGENTS.md`
Old:
```markdown
- Verify: `go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker -count=1`
```
New:
```markdown
- Agent panel (`S.createAgentPanel(inst) -> {el, paint, dispose}`): prompt, length, language, Start or Revise, a progress list, Cancel, the outcome card, "Export without voice" and "Restore speech". The server writes the story, so nothing here goes through `editStory`. A panel start runs `flushEdits()` first. Events come through `api.events(projectId, onJob, onState, onAgent)` and the bus event `agent`, deduped by `seq`. The panel reloads when `job.agent.story_etag` differs from `inst.etag`, throttled to once per 2 s and once more at the end. Model prose never reaches the panel while a job runs, and `agent.summary` arrives only with `ready_unverified`.
- Lock during a job: `inst.agentLocked` makes the `readonly` getter true and a banner shows. `handleSaveError` treats `agent_job_active` as a conflict, so unsaved text stays and the existing banner offers reload or overwrite after the job. With no unsaved edits the app reloads by itself.
- Project launch: `render` reads `ctx.projectId`, and `window.StorymakerApp.open(windowId, projectId)` serves an open window. `sdk-events-bootstrap.js` passes `event.payload.project_id` only if it matches the UUID pattern of `ValidProjectID`, and only for `storymaker`. These shell files are parts of `main.bundle.js`, so run `npm run build:ui` after an edit.
- Verify: `go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker -count=1`
```

- [ ] **Step 4: Update the user documentation.**

**Edit** `documentation/storymaker.md`
Old:
```markdown
phases 1 and 2. The agent panel, the chat tool and the asset generator come in later phases.
```
New:
```markdown
phases 1 and 2 and the agent core. The asset generator comes in a later phase.
```

**Edit** `documentation/storymaker.md`
Old:
```markdown
| `storymaker.narration.max_job_chars` | `20000` | Characters that cloud voices may speak in one narration job (100 to 10000000). |
```
New:
```markdown
| `storymaker.narration.max_job_chars` | `20000` | Characters that cloud voices may speak in one narration job (100 to 10000000). |
| `storymaker.agent.enabled` | `true` | Allows agent jobs from the app and from chat. |
| `storymaker.agent.provider` | empty | LLM provider for agent jobs. Empty: the main route with its configured failover. A chosen provider is pinned to the job without failover. |
| `storymaker.agent.model` | empty | Model for agent jobs. Empty: the model of the provider. |
| `storymaker.agent.job_timeout_seconds` | `1800` | A job that runs longer stops, and its valid scenes stay (120 to 7200). |
| `storymaker.agent.max_length_seconds` | `180` | A longer request is shortened to this length (15 to 3600). `max_duration_seconds` also applies. |
| `storymaker.agent.max_daily_jobs` | `10` | New films and revisions from the app and chat, plus exports from chat, per day (1 to 1000). |
```

**Edit** `documentation/storymaker.md`
Old:
```markdown
## Projects
```
New:
```markdown
## The agent

The agent writes a film from a topic: first the outline, then the cast, then one scene at a time, then a check with fixes, and at the end the narration. It runs as an isolated job on one project. It can use only the Storymaker tools of its current step.

- **In the app.** The project list has a start form. Inside a project the agent panel has the prompt, length, language, Start or Revise, a progress list and Cancel.
- **From chat.** The `storymaker` tool can create a film, report its status, revise it, export it, open it in the app and list projects. "Mach mir ein Erklärvideo über Backups" is enough to start.
- **While a job runs.** The editor is locked and shows a banner. Saving, narration and export are refused for that project. A job cannot start while a render, narration or music job of the project runs. One job runs at a time, and eight can wait.
- **Language.** The request can name a language. Otherwise the job uses the language of the interface. Narration, title and on-screen text are written in that language.
- **Writing check.** Narration and on-screen text are checked for dashes everywhere, and for stock phrases in English and German. A hit gets one optional fix round and never blocks the outcome or appears among the open checks.
- **Revise.** The outcome card lists the scenes that changed, the scenes that stayed as they were and the scenes that were removed. The chat `status` answer gives the same three lists.

| Outcome | What it means |
| --- | --- |
| Your story is ready. Please watch it once. | Scenes, assets and timing were checked. Overlaps and text fit were not checked automatically. |
| Your story is partly done. | Some scenes still need work, and valid scenes are saved. The revise field is prefilled to finish them. A timeout or a model error ends here too. |
| The agent could not write this story. | No outline was saved. The card gives the reason. |

Watch every film once: nothing checks yet whether a prop covers a face or a text overflows. A film may run a little longer than asked, which is listed and is not a failure. Without a browser the compile check is skipped and reported as `compile_unverified`. The job still finishes.

**No voice.** A job on a box with no ready voice still finishes. The narration text stays in the story, and each scene gets a short on-screen text. To export, either install a voice and press **Generate narration**, or press **Export without voice**. That button marks the scenes silent, gives each a reading time and turns speech bubbles into on-screen text. **Restore speech** undoes it while its copy, `story.before-silent.json`, exists. From chat, `render` with `silent: true` does the same and says that speech was removed.

**Narration and cost.** A job narrates by itself at its end, under the character limits above, so a finished job can export. Cloud voices can cost money. **Music** is not part of the agent: a request for music gets an answer that it cannot do this, and the Music tab stays with you.

An agent job has no token limit of its own. Its cost is bounded by the job timeout (1800 seconds by default), the fixed tool-call limits per step and `storymaker.agent.max_daily_jobs`.

Job records are in `data/storymaker/agent/`: `jobs.json`, `usage.json` and `events/<job id>.jsonl`. The event files hold tool names, scene ids, token counts and statuses, but no tool arguments and no model text. `jobs.json` also keeps the request, its first 200 characters, and for a ready film the summary.

## Projects
```

**Edit** `documentation/storymaker.md`
Old:
```markdown
The agent panel and chat tool, the asset generator, charts and diagrams, most other transitions (cut, fade, slide-left and wipe
```
New:
```markdown
The asset generator, the agent's layout checks and still review, charts and diagrams, most other transitions (cut, fade, slide-left and wipe
```

**Edit** `documentation/storymaker.md`
Old:
```markdown
- Parallel workers: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render -run '^TestParallelParityBrowser$' -count=1 -timeout 20m`.
```
New:
```markdown
- Parallel workers: `AURAGO_RUN_BROWSER_SMOKE=1 go test ./internal/storymaker/render -run '^TestParallelParityBrowser$' -count=1 -timeout 20m`.
- The agent with a scripted model, in native and text mode: `go test ./internal/server -run '^TestStorymakerAgentScripted' -count=1`.
```

**Edit** `documentation/manuals/de/04-webui.md`
Old:
```markdown
danach in Video Studio weiterschneiden (der Export braucht FFmpeg und einen Browser), siehe [Storymaker](../../storymaker.md) |
```
New:
```markdown
danach in Video Studio weiterschneiden (der Export braucht FFmpeg und einen Browser). Der Storymaker-Agent schreibt den Film aus einem Thema, in der App oder aus dem Chat, siehe [Storymaker](../../storymaker.md) |
```

**Edit** `documentation/manuals/en/04-webui.md`
Old:
```markdown
then continue in Video Studio (needs FFmpeg and a browser for the export) — see [Storymaker](../../storymaker.md) |
```
New:
```markdown
then continue in Video Studio (needs FFmpeg and a browser for the export). The Storymaker agent writes the film from a topic, in the app or from chat — see [Storymaker](../../storymaker.md) |
```

- [ ] **Step 5: Run.** This task has no test. Check the one fact the docs took from task D4.
```bash
grep -n "storymaker-agent.js\|storymaker-handoff.js\|'storymaker.js'" ui/js/desktop/core/module-loader.js
git diff --check -- documentation internal/storymaker/AGENTS.md ui/js/desktop/apps/AGENTS.md
```
Expected: the loader names `storymaker-handoff.js`, then `storymaker-agent.js`, then `storymaker.js`, and `git diff --check` prints nothing. If the loader order differs, change the load-order sentences in the contract and in `ui/js/desktop/apps/AGENTS.md` to the real order before committing.

- [ ] **Step 6: Commit.** Stage `docs/superpowers/specs/2026-11-01-storymaker-design.md` and `docs/superpowers/specs/2026-11-01-storymaker-contract.md` with `git add -f`, through the temp index of the plan index. Stage `internal/storymaker/AGENTS.md`, `ui/js/desktop/apps/AGENTS.md`, `documentation/storymaker.md`, `documentation/manuals/de/04-webui.md` and `documentation/manuals/en/04-webui.md`. Message: `docs(storymaker): record the agent core in the spec, contract and manuals`.

### Task F3: Full matrix and landing
**Batch:** F  **Depends on:** F1, F2 and all tasks of batches A to E
**Files:** none. A rebuilt `ui/js/desktop/bundles/main.bundle.js` is committed only if Step 5 requires it.

- [ ] **Step 1: Check that batches A to E are on main.** Run in the batch F worktree, `C:/Users/Andi/Documents/repo/AuraGo/.worktrees/storymaker-agent-f`.
```bash
git cat-file -e main:internal/storymaker/studio.go && git cat-file -e main:internal/agent/canonical_storymaker.go && git cat-file -e main:internal/server/storymaker_agent.go && git cat-file -e main:ui/js/desktop/apps/storymaker-agent.js && git cat-file -e main:internal/agent/storymaker_tool.go && echo batches-present
git rebase main
```
Expected: `batches-present` and a clean rebase. If a file is missing, stop and tell the user which batch has not landed.

- [ ] **Step 2: Run the matrix.** Run every line in the foreground, one at a time, with the shell sandbox off for the browser lines. Keep each line's PASS and FAIL names.
```bash
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go build ./...
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go vet ./internal/storymaker/... ./internal/agent ./internal/server ./internal/config ./internal/prompts ./internal/security ./ui
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker/... ./internal/config ./internal/prompts ./internal/security ./internal/audit -count=1 -timeout 15m
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/agent -count=1 -timeout 20m
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/server -run Storymaker -count=1 -timeout 15m
GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'Storymaker|TestDesktopStorymaker|TestConfigStorymaker' -count=1 -timeout 15m
npm run check:ui
AURAGO_RUN_BROWSER_SMOKE=1 GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/server -run 'Storymaker.*(Browser|Parity)' -count=1 -timeout 15m
AURAGO_RUN_BROWSER_SMOKE=1 GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./ui -run 'Storymaker.*Browser' -count=1 -timeout 15m
AURAGO_RUN_BROWSER_SMOKE=1 GOCACHE='D:\dev-cache\go-build' GOTMPDIR='D:\tmp\go-build' go test ./internal/storymaker/render -run '^TestCompileCheckBrowser$' -count=1 -timeout 15m
git ls-files --eol | grep 'attr/text eol=lf' | grep 'w/crlf'
```
Expected: the build and vet are clean, and every test failure name is listed in `docs/superpowers/plans/2026-11-06-storymaker-agent-core/evidence/baseline.md`. `check:ui` exits 0. The last line prints nothing, because no `eol=lf` file was converted. A browser test that reports SKIP is not a pass, so report it. A failure outside the baseline stops the task: use debug-agent, and do not change the baseline.

- [ ] **Step 3: Compare the graph.** Run the detect-changes command of the plan index with `--scope compare --base-ref main`. Report the affected symbols and risk. `partial: true` or `truncated: true` is not a clean check, so say so in the report and confirm new symbols with a text search.

- [ ] **Step 4: Stop and ask the user for the go to land.** Report the matrix result, the failures matched against the baseline and the graph result. Continue only after the user says to land this batch. Do not push and do not deploy.

- [ ] **Step 5: Land after the go.** Run each line alone and read its output before the next. Do not chain the merge after the checks.
```bash
MAIN='C:/Users/Andi/Documents/repo/AuraGo'
BRANCH=$(git rev-parse --abbrev-ref HEAD)
git -C "$MAIN" branch --show-current
git rebase main
npm run check:ui
comm -12 <(git diff main...HEAD --name-only | sort) <(git -C "$MAIN" diff HEAD --name-only | sort)
git merge-base --is-ancestor main HEAD && echo main-is-ancestor
git -C "$MAIN" merge --ff-only "$BRANCH"
```
Expected: the current branch is `main`, `check:ui` exits 0, `comm` prints nothing, and the ancestor check prints `main-is-ancestor`. The merge then fast-forwards.

If `check:ui` fails after the rebase, run `npm run build:ui`, commit the bundle as in Step 7, and repeat the checks. If `comm` prints a file, another session has uncommitted edits in it: tell the user at once, and do not stash or overwrite them.

- [ ] **Step 6: Clean up after landing.** Run from outside the worktree, in PowerShell. It removes only a `node_modules` junction, never its target.
```powershell
$hits = @(git -C C:/Users/Andi/Documents/repo/AuraGo worktree list --porcelain | Select-String '^worktree .*[\\/]\.worktrees[\\/]storymaker-agent-f$' | ForEach-Object { $_.Line.Substring(9) })
if ($hits.Count -ne 1) { throw "expected one worktree, found $($hits.Count)" }
$wt = $hits[0]
$nm = Join-Path $wt 'node_modules'
if ((Test-Path -LiteralPath $nm) -and (Get-Item -LiteralPath $nm -Force).LinkType -eq 'Junction') { [System.IO.Directory]::Delete($nm, $false) }
git -C C:/Users/Andi/Documents/repo/AuraGo worktree remove $wt
```
Expected: the worktree is gone. Then run `git -C C:/Users/Andi/Documents/repo/AuraGo branch -d storymaker-agent-f`. If the pattern matches none or more than one path, stop and ask.

- [ ] **Step 7: Commit.** Nothing to commit in the normal case. Only if Step 5 rebuilt the bundle, stage `ui/js/desktop/bundles/main.bundle.js`. Message: `chore(ui): rebuild the desktop bundle after the rebase`.

### Task F4: Real-model acceptance
**Batch:** F  **Depends on:** F3
**Files:** none. The user runs this task.

- [ ] **Step 1: Hand the recipe to the user and stop.** No agent deploys, restarts or changes anything on aurago-test. An agent may only read the evidence in Step 4, and only when the user asks. Existing containers, apps and data are never deleted, recreated or updated.

- [ ] **Step 2: The user deploys.** Deploy with `update.sh`. Read `llm.provider` on the configuration page first, because the route decides which of the next steps apply.

- [ ] **Step 3: The user runs the eight checks.**
  1. **App, native route.** Storymaker, new story, topic "Wie funktioniert eine Firewall? Für Einsteiger", 45 s, German. Watch the panel: phases, scene count, no prose while running.
  2. **Text-mode route.** If a GLM or legacy MiniMax provider exists, set `storymaker.agent.provider` to it and repeat with "Was ist Phishing?". Clear the setting afterwards.
  3. **Chat.** "Mach mir ein Erklärvideo über Backups", then status, then "öffne es", then "exportiere es". The notification appears on completion.
  4. **Revise.** Panel: "Mach Szene 2 kürzer und zeige ein Schloss."
  5. **Edit lock.** With a job running, the editor is locked and the banner shows.
  6. **Cancel.** Cancel a running job. Valid scenes stay.
  7. **Restart mid-job.** Start a job and restart the service. The job reads as interrupted, and a revise finishes it.
  8. **Export.** Export one film and watch it with sound.

- [ ] **Step 4: Run.** Read the evidence without changing anything. Set `JOB` to a job id from `jobs.json`.
```bash
ssh aurago-test 'cat /home/aurago/aurago/data/storymaker/agent/jobs.json'
JOB=paste-the-job-id-here
ssh aurago-test "grep '\"type\":\"tool\"' /home/aurago/aurago/data/storymaker/agent/events/$JOB.jsonl | grep -c '\"status\":\"refused\"'"
```
Expected for each job of checks 1 to 4 that finished, to pass:
- Its `agent.outcome` is `ready_unverified`.
- `agent.open_checks` has no `scene_unfilled`.
- The events and the story show no invented ids.
- The count of refused calls is 3 or fewer.
- The export plays with speech.

The film itself is judged by eye, because layout stays unchecked.

- [ ] **Step 5: Report the result.** The user reports pass or fail per check.
  - If text mode fails on nested `data`, save the real replies as replay fixtures for a follow-up.
  - If weak models cannot write `{n}` markers, record that as a follow-up.
  - If films overlap or read badly, this decides whether `checks.js` comes first in the next slice.

- [ ] **Step 6: Commit.** Nothing to commit.


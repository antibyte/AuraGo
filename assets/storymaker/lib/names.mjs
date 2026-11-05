// Display names of the catalog entries in the 16 desktop locales.
// Order of every row: cs da de el en es fr hi it ja nl no pl pt sv zh
// The build script writes them to ui/lang/storymaker_catalog/<lang>.json as storymaker.catalog.<id with "." -> "_">.

export const LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];

export const NAMES = {
  // characters
  'char.office-woman-1': ['Kancelářská pracovnice', 'Kontoransat (kvinde)', 'Büroangestellte', 'Υπάλληλος γραφείου (γυναίκα)', 'Office woman', 'Oficinista (mujer)', 'Employée de bureau', 'ऑफिस कर्मचारी (महिला)', 'Impiegata', 'オフィスの女性', 'Kantoormedewerkster', 'Kontordame', 'Pracownica biurowa', 'Funcionária de escritório', 'Kontorsanställd (kvinna)', '办公室女职员'],
  'char.office-man-1': ['Kancelářský pracovník', 'Kontoransat (mand)', 'Büroangestellter', 'Υπάλληλος γραφείου (άνδρας)', 'Office man', 'Oficinista (hombre)', 'Employé de bureau', 'ऑफिस कर्मचारी (पुरुष)', 'Impiegato', 'オフィスの男性', 'Kantoormedewerker', 'Kontormann', 'Pracownik biurowy', 'Funcionário de escritório', 'Kontorsanställd (man)', '办公室男职员'],
  'char.it-tech-1': ['IT technik', 'IT-tekniker', 'IT-Techniker', 'Τεχνικός πληροφορικής', 'IT technician', 'Técnico informático', 'Technicien informatique', 'आईटी तकनीशियन', 'Tecnico informatico', 'ITエンジニア', 'IT-technicus', 'IT-tekniker', 'Technik IT', 'Técnico de TI', 'IT-tekniker', 'IT 技术员'],
  'char.senior-man-1': ['Starší muž', 'Ældre mand', 'Älterer Herr', 'Ηλικιωμένος άνδρας', 'Senior man', 'Hombre mayor', 'Homme âgé', 'बुज़ुर्ग पुरुष', 'Uomo anziano', '年配の男性', 'Oudere man', 'Eldre mann', 'Starszy mężczyzna', 'Homem idoso', 'Äldre man', '年长男士'],
  // props
  'prop.laptop': ['Notebook', 'Bærbar computer', 'Laptop', 'Φορητός υπολογιστής', 'Laptop', 'Portátil', 'Ordinateur portable', 'लैपटॉप', 'Portatile', 'ノートパソコン', 'Laptop', 'Bærbar PC', 'Laptop', 'Portátil', 'Bärbar dator', '笔记本电脑'],
  'prop.phone': ['Mobilní telefon', 'Mobiltelefon', 'Smartphone', 'Κινητό τηλέφωνο', 'Phone', 'Teléfono móvil', 'Téléphone', 'फ़ोन', 'Telefono', 'スマートフォン', 'Telefoon', 'Mobiltelefon', 'Telefon', 'Telefone', 'Mobiltelefon', '手机'],
  'prop.monitor': ['Monitor', 'Skærm', 'Monitor', 'Οθόνη', 'Monitor', 'Monitor', 'Écran', 'मॉनिटर', 'Monitor', 'モニター', 'Monitor', 'Skjerm', 'Monitor', 'Monitor', 'Skärm', '显示器'],
  'prop.server': ['Server', 'Server', 'Server', 'Διακομιστής', 'Server', 'Servidor', 'Serveur', 'सर्वर', 'Server', 'サーバー', 'Server', 'Server', 'Serwer', 'Servidor', 'Server', '服务器'],
  'prop.router': ['Router', 'Router', 'Router', 'Δρομολογητής', 'Router', 'Router', 'Routeur', 'राउटर', 'Router', 'ルーター', 'Router', 'Ruter', 'Router', 'Router', 'Router', '路由器'],
  'prop.cloud': ['Oblak', 'Sky', 'Wolke', 'Σύννεφο', 'Cloud', 'Nube', 'Nuage', 'क्लाउड', 'Nuvola', 'クラウド', 'Wolk', 'Sky', 'Chmura', 'Nuvem', 'Moln', '云'],
  'prop.database': ['Databáze', 'Database', 'Datenbank', 'Βάση δεδομένων', 'Database', 'Base de datos', 'Base de données', 'डेटाबेस', 'Database', 'データベース', 'Database', 'Database', 'Baza danych', 'Base de dados', 'Databas', '数据库'],
  'prop.lock': ['Zámek', 'Lås', 'Schloss', 'Λουκέτο', 'Lock', 'Candado', 'Cadenas', 'ताला', 'Lucchetto', '南京錠', 'Slot', 'Lås', 'Kłódka', 'Cadeado', 'Lås', '挂锁'],
  'prop.key': ['Klíč', 'Nøgle', 'Schlüssel', 'Κλειδί', 'Key', 'Llave', 'Clé', 'चाबी', 'Chiave', '鍵', 'Sleutel', 'Nøkkel', 'Klucz', 'Chave', 'Nyckel', '钥匙'],
  'prop.shield': ['Štít', 'Skjold', 'Schild', 'Ασπίδα', 'Shield', 'Escudo', 'Bouclier', 'ढाल', 'Scudo', 'シールド', 'Schild', 'Skjold', 'Tarcza', 'Escudo', 'Sköld', '盾牌'],
  'prop.mail': ['E-mail', 'E-mail', 'E-Mail', 'Email', 'Mail', 'Correo', 'Courriel', 'ईमेल', 'Email', 'メール', 'E-mail', 'E-post', 'E-mail', 'E-mail', 'E-post', '邮件'],
  'prop.document': ['Dokument', 'Dokument', 'Dokument', 'Έγγραφο', 'Document', 'Documento', 'Document', 'दस्तावेज़', 'Documento', '書類', 'Document', 'Dokument', 'Dokument', 'Documento', 'Dokument', '文档'],
  'prop.coffee-cup': ['Šálek kávy', 'Kaffekop', 'Kaffeetasse', 'Φλιτζάνι καφέ', 'Coffee cup', 'Taza de café', 'Tasse de café', 'कॉफ़ी का कप', 'Tazza di caffè', 'コーヒーカップ', 'Koffiekopje', 'Kaffekopp', 'Filiżanka kawy', 'Chávena de café', 'Kaffekopp', '咖啡杯'],
  'prop.desk': ['Psací stůl', 'Skrivebord', 'Schreibtisch', 'Γραφείο', 'Desk', 'Escritorio', 'Bureau', 'डेस्क', 'Scrivania', 'デスク', 'Bureau', 'Skrivebord', 'Biurko', 'Secretária', 'Skrivbord', '办公桌'],
  'prop.chair': ['Židle', 'Stol', 'Stuhl', 'Καρέκλα', 'Chair', 'Silla', 'Chaise', 'कुर्सी', 'Sedia', '椅子', 'Stoel', 'Stol', 'Krzesło', 'Cadeira', 'Stol', '椅子'],
  'prop.plant': ['Rostlina', 'Plante', 'Pflanze', 'Φυτό', 'Plant', 'Planta', 'Plante', 'पौधा', 'Pianta', '観葉植物', 'Plant', 'Plante', 'Roślina', 'Planta', 'Växt', '绿植'],
  'prop.check': ['Zaškrtnutí', 'Flueben', 'Haken', 'Σημάδι επιλογής', 'Check mark', 'Marca de verificación', 'Coche', 'सही का निशान', 'Segno di spunta', 'チェックマーク', 'Vinkje', 'Hake', 'Znacznik wyboru', 'Visto', 'Bock', '对勾'],
  'prop.cross': ['Křížek', 'Kryds', 'Kreuz', 'Σταυρός', 'Cross mark', 'Cruz', 'Croix', 'क्रॉस का निशान', 'Croce', 'バツ印', 'Kruisje', 'Kryss', 'Krzyżyk', 'Cruz', 'Kryss', '叉号'],
  'prop.warning': ['Varování', 'Advarsel', 'Warnung', 'Προειδοποίηση', 'Warning', 'Advertencia', 'Avertissement', 'चेतावनी', 'Avviso', '警告', 'Waarschuwing', 'Advarsel', 'Ostrzeżenie', 'Aviso', 'Varning', '警告'],
  'prop.idea': ['Nápad', 'Idé', 'Idee', 'Ιδέα', 'Idea', 'Idea', 'Idée', 'विचार', 'Idea', 'アイデア', 'Idee', 'Idé', 'Pomysł', 'Ideia', 'Idé', '点子'],
  'prop.question': ['Otazník', 'Spørgsmål', 'Frage', 'Ερώτηση', 'Question', 'Pregunta', 'Question', 'प्रश्न', 'Domanda', '質問', 'Vraag', 'Spørsmål', 'Pytanie', 'Pergunta', 'Fråga', '问题'],
  'prop.arrow-right': ['Šipka doprava', 'Pil mod højre', 'Pfeil nach rechts', 'Βέλος δεξιά', 'Arrow right', 'Flecha a la derecha', 'Flèche vers la droite', 'दायाँ तीर', 'Freccia a destra', '右矢印', 'Pijl naar rechts', 'Pil mot høyre', 'Strzałka w prawo', 'Seta para a direita', 'Pil åt höger', '右箭头'],
  'prop.gear': ['Ozubené kolo', 'Tandhjul', 'Zahnrad', 'Γρανάζι', 'Gear', 'Engranaje', 'Engrenage', 'गियर', 'Ingranaggio', '歯車', 'Tandwiel', 'Tannhjul', 'Koło zębate', 'Engrenagem', 'Kugghjul', '齿轮'],
  // environments
  'env.plain-stage': ['Prázdné pódium', 'Enkel scene', 'Einfache Bühne', 'Απλή σκηνή', 'Plain stage', 'Escenario liso', 'Scène simple', 'सादा मंच', 'Palco semplice', 'シンプルなステージ', 'Eenvoudig podium', 'Enkel scene', 'Prosta scena', 'Palco simples', 'Enkel scen', '纯色舞台'],
  'env.office': ['Kancelář', 'Kontor', 'Büro', 'Γραφείο', 'Office', 'Oficina', 'Bureau', 'ऑफिस', 'Ufficio', 'オフィス', 'Kantoor', 'Kontor', 'Biuro', 'Escritório', 'Kontor', '办公室'],
  'env.home-office': ['Domácí kancelář', 'Hjemmekontor', 'Homeoffice', 'Γραφείο στο σπίτι', 'Home office', 'Oficina en casa', 'Bureau à domicile', 'होम ऑफिस', 'Ufficio in casa', '在宅オフィス', 'Thuiskantoor', 'Hjemmekontor', 'Biuro domowe', 'Escritório em casa', 'Hemmakontor', '家庭办公室'],
  'env.abstract-tech': ['Abstraktní technologie', 'Abstrakt teknologi', 'Abstrakte Technik', 'Αφηρημένη τεχνολογία', 'Abstract tech', 'Tecnología abstracta', 'Technologie abstraite', 'अमूर्त तकनीक', 'Tecnologia astratta', '抽象的なテック背景', 'Abstracte tech', 'Abstrakt teknologi', 'Abstrakcyjna technologia', 'Tecnologia abstrata', 'Abstrakt teknik', '抽象科技背景'],
  // poses
  'pose.stand': ['Stoj', 'Stående', 'Stehen', 'Όρθια στάση', 'Stand', 'De pie', 'Debout', 'खड़े होना', 'In piedi', '立つ', 'Staan', 'Stå', 'Stanie', 'De pé', 'Stå', '站立'],
  'pose.sit': ['Sed', 'Sidde', 'Sitzen', 'Καθιστή στάση', 'Sit', 'Sentado', 'Assis', 'बैठना', 'Seduto', '座る', 'Zitten', 'Sitte', 'Siedzenie', 'Sentado', 'Sitta', '坐着'],
  'pose.typing': ['Psaní', 'Skrive på computer', 'Tippen', 'Πληκτρολόγηση', 'Typing', 'Escribiendo', 'Taper au clavier', 'टाइप करना', 'Digitare', 'タイピング', 'Typen', 'Skrive', 'Pisanie na klawiaturze', 'A escrever', 'Skriva', '打字'],
  'pose.point-left': ['Ukazuje vlevo', 'Peger til venstre', 'Zeigt nach links', 'Δείχνει αριστερά', 'Point left', 'Señala a la izquierda', 'Montre à gauche', 'बाईं ओर इशारा', 'Indica a sinistra', '左を指す', 'Wijst naar links', 'Peker til venstre', 'Wskazuje w lewo', 'Aponta para a esquerda', 'Pekar åt vänster', '指向左侧'],
  'pose.point-right': ['Ukazuje vpravo', 'Peger til højre', 'Zeigt nach rechts', 'Δείχνει δεξιά', 'Point right', 'Señala a la derecha', 'Montre à droite', 'दाईं ओर इशारा', 'Indica a destra', '右を指す', 'Wijst naar rechts', 'Peker til høyre', 'Wskazuje w prawo', 'Aponta para a direita', 'Pekar åt höger', '指向右侧'],
  'pose.wave': ['Mává', 'Vinker', 'Winken', 'Χαιρετισμός', 'Wave', 'Saludar', 'Saluer', 'हाथ हिलाना', 'Salutare', '手を振る', 'Zwaaien', 'Vinke', 'Machanie', 'Acenar', 'Vinka', '挥手'],
  'pose.think': ['Přemýšlí', 'Tænker', 'Nachdenken', 'Σκέψη', 'Think', 'Pensar', 'Réfléchir', 'सोचना', 'Pensare', '考える', 'Nadenken', 'Tenke', 'Myślenie', 'Pensar', 'Fundera', '思考'],
  'pose.present': ['Prezentuje', 'Præsenterer', 'Präsentieren', 'Παρουσίαση', 'Present', 'Presentar', 'Présenter', 'प्रस्तुत करना', 'Presentare', '紹介する', 'Presenteren', 'Presentere', 'Prezentowanie', 'Apresentar', 'Presentera', '展示'],
  'pose.arms-crossed': ['Zkřížené ruce', 'Korslagte arme', 'Verschränkte Arme', 'Σταυρωμένα χέρια', 'Arms crossed', 'Brazos cruzados', 'Bras croisés', 'हाथ बाँधे हुए', 'Braccia incrociate', '腕組み', 'Armen over elkaar', 'Armer i kors', 'Skrzyżowane ręce', 'Braços cruzados', 'Korslagda armar', '双臂交叉'],
  'pose.hands-on-hips': ['Ruce v bok', 'Hænderne i siden', 'Hände in die Hüften', 'Χέρια στη μέση', 'Hands on hips', 'Manos en la cintura', 'Mains sur les hanches', 'कमर पर हाथ', 'Mani sui fianchi', '腰に手を当てる', 'Handen in de zij', 'Hendene i siden', 'Ręce na biodrach', 'Mãos na cintura', 'Händerna i sidan', '双手叉腰'],
  // gestures
  'gesture.point': ['Ukázat', 'Pege', 'Zeigen', 'Δείχνω', 'Point', 'Señalar', 'Montrer', 'इशारा करना', 'Indicare', '指差し', 'Wijzen', 'Peke', 'Wskazać', 'Apontar', 'Peka', '指向'],
  'gesture.wave': ['Mávnout', 'Vinke', 'Winken', 'Χαιρετώ', 'Wave', 'Saludar', 'Saluer', 'हाथ हिलाना', 'Salutare', '手を振る', 'Zwaaien', 'Vinke', 'Pomachać', 'Acenar', 'Vinka', '挥手'],
  'gesture.shrug': ['Pokrčit rameny', 'Trække på skuldrene', 'Schulterzucken', 'Σήκωμα ώμων', 'Shrug', 'Encogerse de hombros', 'Hausser les épaules', 'कंधे उचकाना', 'Alzare le spalle', '肩をすくめる', 'Schouderophalen', 'Trekke på skuldrene', 'Wzruszenie ramionami', 'Encolher os ombros', 'Rycka på axlarna', '耸肩'],
  'gesture.think': ['Zamyslet se', 'Tænke', 'Nachdenken', 'Σκέφτομαι', 'Think', 'Pensar', 'Réfléchir', 'सोचना', 'Pensare', '考え込む', 'Nadenken', 'Tenke', 'Zamyślić się', 'Pensar', 'Fundera', '思考'],
  'gesture.present': ['Představit', 'Præsentere', 'Präsentieren', 'Παρουσιάζω', 'Present', 'Presentar', 'Présenter', 'प्रस्तुत करना', 'Presentare', '手で示す', 'Presenteren', 'Presentere', 'Zaprezentować', 'Apresentar', 'Presentera', '展示'],
  'gesture.thumbs-up': ['Palec nahoru', 'Tommelfinger op', 'Daumen hoch', 'Μπράβο', 'Thumbs up', 'Pulgar hacia arriba', 'Pouce levé', 'अंगूठा ऊपर', 'Pollice in su', 'サムズアップ', 'Duim omhoog', 'Tommel opp', 'Kciuk w górę', 'Polegar para cima', 'Tummen upp', '竖大拇指'],
  'gesture.celebrate': ['Oslavovat', 'Fejre', 'Jubeln', 'Πανηγυρίζω', 'Celebrate', 'Celebrar', 'Célébrer', 'जश्न मनाना', 'Festeggiare', '喜ぶ', 'Vieren', 'Feire', 'Świętować', 'Celebrar', 'Fira', '庆祝'],
  'gesture.facepalm': ['Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', 'फेसपाम', 'Facepalm', '顔に手を当てる', 'Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', 'Facepalm', '捂脸'],
  'gesture.count': ['Počítat', 'Tælle', 'Zählen', 'Μέτρημα', 'Count', 'Contar', 'Compter', 'गिनना', 'Contare', '数える', 'Tellen', 'Telle', 'Liczyć', 'Contar', 'Räkna', '数数'],
  // expressions
  'expr.neutral': ['Neutrální', 'Neutral', 'Neutral', 'Ουδέτερη', 'Neutral', 'Neutral', 'Neutre', 'सामान्य', 'Neutra', '無表情', 'Neutraal', 'Nøytral', 'Neutralna', 'Neutra', 'Neutral', '平静'],
  'expr.happy': ['Šťastný', 'Glad', 'Fröhlich', 'Χαρούμενος', 'Happy', 'Feliz', 'Heureux', 'खुश', 'Felice', 'うれしい', 'Blij', 'Glad', 'Szczęśliwy', 'Feliz', 'Glad', '开心'],
  'expr.laugh': ['Smích', 'Grin', 'Lachen', 'Γέλιο', 'Laugh', 'Risa', 'Rire', 'हँसी', 'Risata', '笑う', 'Lachen', 'Latter', 'Śmiech', 'Riso', 'Skratt', '大笑'],
  'expr.sad': ['Smutný', 'Trist', 'Traurig', 'Λυπημένος', 'Sad', 'Triste', 'Triste', 'उदास', 'Triste', '悲しい', 'Verdrietig', 'Trist', 'Smutny', 'Triste', 'Ledsen', '难过'],
  'expr.angry': ['Naštvaný', 'Vred', 'Wütend', 'Θυμωμένος', 'Angry', 'Enfadado', 'En colère', 'गुस्से में', 'Arrabbiato', '怒っている', 'Boos', 'Sint', 'Zły', 'Zangado', 'Arg', '生气'],
  'expr.surprised': ['Překvapený', 'Overrasket', 'Überrascht', 'Έκπληκτος', 'Surprised', 'Sorprendido', 'Surpris', 'हैरान', 'Sorpreso', '驚いた', 'Verrast', 'Overrasket', 'Zaskoczony', 'Surpreendido', 'Förvånad', '惊讶'],
  'expr.confused': ['Zmatený', 'Forvirret', 'Verwirrt', 'Μπερδεμένος', 'Confused', 'Confundido', 'Perplexe', 'उलझन में', 'Confuso', '困惑', 'Verward', 'Forvirret', 'Zdezorientowany', 'Confuso', 'Förvirrad', '困惑'],
  'expr.thinking': ['Přemýšlející', 'Tænkende', 'Nachdenklich', 'Σκεπτικός', 'Thinking', 'Pensativo', 'Pensif', 'सोच में', 'Pensieroso', '考え中', 'Nadenkend', 'Tenkende', 'Zamyślony', 'Pensativo', 'Tänkande', '思考中'],
  'expr.worried': ['Ustaraný', 'Bekymret', 'Besorgt', 'Ανήσυχος', 'Worried', 'Preocupado', 'Inquiet', 'चिंतित', 'Preoccupato', '心配', 'Bezorgd', 'Bekymret', 'Zmartwiony', 'Preocupado', 'Orolig', '担心'],
  'expr.determined': ['Odhodlaný', 'Fast besluttet', 'Entschlossen', 'Αποφασισμένος', 'Determined', 'Decidido', 'Déterminé', 'दृढ़ निश्चयी', 'Determinato', '決意', 'Vastberaden', 'Bestemt', 'Zdeterminowany', 'Determinado', 'Beslutsam', '坚定'],
  // mouth shapes
  'mouth.rest': ['Klidná ústa', 'Hvile', 'Ruhe', 'Ηρεμία', 'Rest', 'Reposo', 'Repos', 'विश्राम', 'Riposo', '閉じた口', 'Rust', 'Hvile', 'Spoczynek', 'Repouso', 'Vila', '静止'],
  'mouth.ai': ['Samohlásky A/I', 'Vokal A/I', 'Vokal A/I', 'Φωνήεν A/I', 'Vowel A/I', 'Vocal A/I', 'Voyelle A/I', 'स्वर A/I', 'Vocale A/I', '母音 A/I', 'Klinker A/I', 'Vokal A/I', 'Samogłoska A/I', 'Vogal A/I', 'Vokal A/I', '元音 A/I'],
  'mouth.e': ['Samohláska E', 'Vokal E', 'Vokal E', 'Φωνήεν E', 'Vowel E', 'Vocal E', 'Voyelle E', 'स्वर E', 'Vocale E', '母音 E', 'Klinker E', 'Vokal E', 'Samogłoska E', 'Vogal E', 'Vokal E', '元音 E'],
  'mouth.o': ['Samohláska O', 'Vokal O', 'Vokal O', 'Φωνήεν O', 'Vowel O', 'Vocal O', 'Voyelle O', 'स्वर O', 'Vocale O', '母音 O', 'Klinker O', 'Vokal O', 'Samogłoska O', 'Vogal O', 'Vokal O', '元音 O'],
  'mouth.u': ['Samohláska U', 'Vokal U', 'Vokal U', 'Φωνήεν U', 'Vowel U', 'Vocal U', 'Voyelle U', 'स्वर U', 'Vocale U', '母音 U', 'Klinker U', 'Vokal U', 'Samogłoska U', 'Vogal U', 'Vokal U', '元音 U'],
  'mouth.mbp': ['Rty M/B/P', 'Læber M/B/P', 'Lippen M/B/P', 'Χείλη M/B/P', 'Lips M/B/P', 'Labios M/B/P', 'Lèvres M/B/P', 'होंठ M/B/P', 'Labbra M/B/P', '唇 M/B/P', 'Lippen M/B/P', 'Lepper M/B/P', 'Wargi M/B/P', 'Lábios M/B/P', 'Läppar M/B/P', '双唇 M/B/P'],
  'mouth.fv': ['Zuby F/V', 'Tænder F/V', 'Zähne F/V', 'Δόντια F/V', 'Teeth F/V', 'Dientes F/V', 'Dents F/V', 'दाँत F/V', 'Denti F/V', '歯 F/V', 'Tanden F/V', 'Tenner F/V', 'Zęby F/V', 'Dentes F/V', 'Tänder F/V', '牙齿 F/V'],
  // cycles
  'cycle.walk': ['Chůze', 'Gang', 'Gehen', 'Περπάτημα', 'Walk', 'Caminar', 'Marche', 'चलना', 'Camminata', '歩く', 'Lopen', 'Gange', 'Chodzenie', 'Caminhar', 'Gå', '走路'],
  'cycle.idle': ['Klidový stav', 'Hvilestilling', 'Ruhehaltung', 'Αναμονή', 'Idle', 'En reposo', 'Au repos', 'निष्क्रिय', 'A riposo', '待機', 'Rustpose', 'Hvilestilling', 'Postawa spoczynkowa', 'Em repouso', 'Vilopose', '待机'],
  // text styles
  'text.title': ['Nadpis', 'Overskrift', 'Titel', 'Τίτλος', 'Title', 'Título', 'Titre', 'शीर्षक', 'Titolo', 'タイトル', 'Titel', 'Tittel', 'Tytuł', 'Título', 'Rubrik', '标题'],
  'text.caption': ['Titulek', 'Billedtekst', 'Untertitel', 'Λεζάντα', 'Caption', 'Subtítulo', 'Légende', 'कैप्शन', 'Didascalia', '字幕', 'Ondertitel', 'Bildetekst', 'Napis', 'Legenda', 'Bildtext', '字幕'],
  'text.label': ['Popisek', 'Etiket', 'Beschriftung', 'Ετικέτα', 'Label', 'Etiqueta', 'Étiquette', 'लेबल', 'Etichetta', 'ラベル', 'Label', 'Etikett', 'Etykieta', 'Rótulo', 'Etikett', '标签'],
  'text.speech': ['Bublina s textem', 'Talebobbel', 'Sprechblase', 'Συννεφάκι ομιλίας', 'Speech bubble', 'Bocadillo', 'Bulle de dialogue', 'संवाद बबल', 'Fumetto', '吹き出し', 'Tekstballon', 'Snakkeboble', 'Dymek', 'Balão de fala', 'Pratbubbla', '对话气泡'],
  'text.thought': ['Myšlenková bublina', 'Tankeboble', 'Gedankenblase', 'Συννεφάκι σκέψης', 'Thought bubble', 'Bocadillo de pensamiento', 'Bulle de pensée', 'विचार बबल', 'Nuvoletta di pensiero', '思考の吹き出し', 'Gedachtewolk', 'Tankeboble', 'Dymek myśli', 'Balão de pensamento', 'Tankebubbla', '思考气泡'],
  'text.subtitle': ['Titulky', 'Undertekster', 'Untertitel', 'Υπότιτλοι', 'Subtitles', 'Subtítulos', 'Sous-titres', 'उपशीर्षक', 'Sottotitoli', '字幕', 'Ondertiteling', 'Undertekster', 'Napisy', 'Legendas', 'Undertexter', '字幕'],
  // transitions
  'trans.cut': ['Střih', 'Klip', 'Schnitt', 'Κοπή', 'Cut', 'Corte', 'Coupe', 'कट', 'Taglio', 'カット', 'Cut', 'Klipp', 'Cięcie', 'Corte', 'Klipp', '切换'],
  'trans.fade': ['Prolnutí', 'Fade', 'Überblendung', 'Σβήσιμο', 'Fade', 'Fundido', 'Fondu', 'फ़ेड', 'Dissolvenza', 'フェード', 'Overvloeien', 'Toning', 'Zanikanie', 'Desvanecer', 'Toning', '淡入淡出'],
  'trans.slide-left': ['Posun doleva', 'Skub til venstre', 'Nach links schieben', 'Ολίσθηση αριστερά', 'Slide left', 'Deslizar a la izquierda', 'Glisser vers la gauche', 'बाईं ओर स्लाइड', 'Scorri a sinistra', '左へスライド', 'Naar links schuiven', 'Skyv til venstre', 'Przesunięcie w lewo', 'Deslizar para a esquerda', 'Skjut åt vänster', '向左滑动'],
  'trans.wipe': ['Stírání', 'Wipe-overgang', 'Wischblende', 'Εναλλαγή σάρωσης', 'Wipe', 'Cortinilla', 'Volet', 'वाइप', 'Tendina', 'ワイプ', 'Veegovergang', 'Wipe-overgang', 'Wycieranie', 'Cortina', 'Svepövergång', '擦除'],
  // sound effects
  'sfx.whoosh': ['Svist', 'Sus', 'Wusch', 'Σφύριγμα', 'Whoosh', 'Silbido', 'Souffle', 'सरसराहट', 'Fruscio', 'シュッという音', 'Zoef', 'Sus', 'Świst', 'Zunido', 'Sus', '嗖声'],
  'sfx.pop': ['Lupnutí', 'Plop', 'Plopp', 'Σκάσιμο', 'Pop', 'Chasquido', 'Éclatement', 'पॉप ध्वनि', 'Schiocco', 'ポンという音', 'Plop', 'Plopp', 'Pyknięcie', 'Estalo', 'Plopp', '啵声'],
  'sfx.click': ['Kliknutí', 'Klik', 'Klick', 'Κλικ', 'Click', 'Clic', 'Clic', 'क्लिक', 'Clic', 'クリック音', 'Klik', 'Klikk', 'Kliknięcie', 'Clique', 'Klick', '点击声'],
  'sfx.ding': ['Cinknutí', 'Kling', 'Ding', 'Κουδούνισμα', 'Ding', 'Campanilla', 'Tintement', 'घंटी की आवाज़', 'Rintocco', 'チーンという音', 'Ping', 'Pling', 'Dzwonek', 'Toque de sino', 'Pling', '叮声'],
  'sfx.success': ['Úspěch', 'Succes', 'Erfolg', 'Επιτυχία', 'Success', 'Éxito', 'Réussite', 'सफलता', 'Successo', '成功音', 'Succes', 'Suksess', 'Sukces', 'Sucesso', 'Framgång', '成功音'],
  'sfx.error': ['Chyba', 'Fejl', 'Fehler', 'Σφάλμα', 'Error', 'Error', 'Erreur', 'त्रुटि', 'Errore', 'エラー音', 'Fout', 'Feil', 'Błąd', 'Erro', 'Fel', '错误音'],
  'sfx.typing': ['Psaní na klávesnici', 'Tastetryk', 'Tippen', 'Πληκτρολόγηση', 'Typing', 'Tecleo', 'Frappe au clavier', 'टाइपिंग', 'Digitazione', 'タイピング音', 'Typen', 'Tastetrykk', 'Pisanie na klawiaturze', 'Digitação', 'Tangentnedslag', '打字声'],
  // palettes
  'palette.sunny': ['Slunečná', 'Solrig', 'Sonnig', 'Ηλιόλουστη', 'Sunny', 'Soleada', 'Ensoleillée', 'धूप भरी', 'Soleggiata', 'サニー', 'Zonnig', 'Solfylt', 'Słoneczna', 'Ensolarada', 'Solig', '阳光'],
  'palette.calm': ['Klidná', 'Rolig', 'Ruhig', 'Ήρεμη', 'Calm', 'Tranquila', 'Calme', 'शांत', 'Calma', 'カーム', 'Rustig', 'Rolig', 'Spokojna', 'Calma', 'Lugn', '宁静'],
};

/** storymaker.catalog.<id with "." replaced by "_"> */
export const nameKey = (id) => `storymaker.catalog.${id.replace('.', '_')}`;

/** {lang: {key: name}} for every language, keys in the order of NAMES. */
export function localeTables() {
  const out = {};
  LANGS.forEach((lang, i) => {
    const table = {};
    for (const [id, row] of Object.entries(NAMES)) table[nameKey(id)] = row[i];
    out[lang] = table;
  });
  return out;
}

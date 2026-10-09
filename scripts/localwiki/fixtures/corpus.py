"""Self-authored fixture corpus for the Local Wikipedia Xapian tests.

All article texts below were written for AuraGo; no third-party content.
Shared by make_zims.py (python-libzim container) and make_goldens.py
(Debian python3-xapian container). Plain data only - no imports beyond stdlib.
"""

# Each fixture: indexing language (ISO 639-3, like mwoffliner), articles
# (path, title, paragraphs), redirects (path, title, target path) and the
# fixed queries whose libzim results become golden data.

FIXTURES = {
    "de": {
        "language": "deu",
        "title": "AuraGo Testwiki (Deutsch)",
        "articles": [
            ("Berlin", "Berlin", [
                "Berlin ist die Hauptstadt Deutschlands und liegt an der Spree.",
                "In Berlin leben fast vier Millionen Menschen. Die Häuser in der Altstadt sind oft sehr alt.",
                "Viele Brücken verbinden die Stadtteile.",
            ]),
            ("Hamburg", "Hamburg", [
                "Hamburg ist eine große Hafenstadt im Norden. Der Hafen liegt an der Elbe.",
                "Schiffe aus aller Welt fahren nach Hamburg.",
            ]),
            ("München", "München", [
                "München ist die Hauptstadt des Freistaats Bayern. Die Stadt liegt an der Isar.",
                "Im Herbst feiern dort viele Menschen ein großes Fest.",
            ]),
            ("Köln", "Köln", [
                "Köln liegt am Rhein und ist für seinen Dom bekannt. Der Kölner Dom ist eine gotische Kirche.",
                "Die Stadt hat viele Brücken über den Rhein.",
            ]),
            ("Spree", "Spree (Fluss)", [
                "Die Spree ist ein Fluss im Osten Deutschlands. Sie fließt durch Berlin und mündet in die Havel.",
            ]),
            ("Elbe", "Elbe", [
                "Die Elbe ist ein großer Fluss. Sie fließt durch Dresden und Hamburg in die Nordsee.",
            ]),
            ("Rhein", "Rhein", [
                "Der Rhein ist einer der längsten Flüsse Europas. Er fließt an Köln und Bonn vorbei.",
            ]),
            ("Haus", "Haus", [
                "Ein Haus ist ein Gebäude, in dem Menschen wohnen.",
                "Die Wände eines Hauses bestehen oft aus Stein oder Holz. In einer Stadt stehen viele Häuser nebeneinander.",
            ]),
            ("Fachwerkhaus", "Fachwerkhaus", [
                "Ein Fachwerkhaus hat ein sichtbares Gerüst aus Holz. Solche Häuser findet man in vielen alten Städten.",
            ]),
            ("Straße", "Straße", [
                "Eine Straße ist ein Weg für Autos, Räder und Fußgänger. Breite Straßen nennt man auch Alleen.",
            ]),
            ("Brücke", "Brücke", [
                "Eine Brücke führt über einen Fluss oder ein Tal. Berlin hat mehr Brücken als Venedig.",
            ]),
            ("Bäckerei", "Bäckerei", [
                "In einer Bäckerei backt der Bäcker Brot und Brötchen. Frisches Brot duftet am Morgen besonders gut.",
            ]),
            ("Apfel", "Apfel", [
                "Der Apfel ist ein beliebtes Obst. Äpfel wachsen an Bäumen und schmecken süß oder sauer.",
            ]),
            ("Fußball", "Fußball", [
                "Fußball ist ein Ballspiel mit zwei Mannschaften. Ein Fußballverein trainiert jede Woche.",
            ]),
            ("Universität", "Universität", [
                "An einer Universität studieren junge Menschen. Die älteste Universität Deutschlands steht in Heidelberg.",
            ]),
            ("Zug", "Zug", [
                "Ein Zug fährt auf Schienen von Bahnhof zu Bahnhof. Mit dem Zug kommt man schnell von Berlin nach München.",
            ]),
            ("Zugspitze", "Zugspitze", [
                "Die Zugspitze ist der höchste Berg Deutschlands. Sie liegt in Bayern an der Grenze zu Österreich.",
            ]),
            ("Bayern", "Bayern", [
                "Bayern ist ein Bundesland im Süden. Die Hauptstadt von Bayern ist München.",
            ]),
        ],
        "redirects": [
            ("Hauptstadt_Deutschlands", "Hauptstadt Deutschlands", "Berlin"),
            ("Muenchen", "Muenchen", "München"),
            ("Strasse", "Strasse", "Straße"),
            ("Spree-Athen", "Spree-Athen", "Berlin"),
        ],
        "fulltext": [
            "berlin", "Berlin", "häuser", "hauses", "münchen", "MUNCHEN", "fluss berlin", "fluss hafen",
            "die stadt", "straße", "strasse", "brücken", "äpfel", "zug bahnhof", "hauptstadt bayern",
            "Österreich", "bäcker brot", "Spree-Athen",
        ],
        "suggest": [
            "b", "ber", "berl", "Berlin", "haupt", "hauptstadt d", "mün", "mue", "muen", "zug", "zugs",
            "str", "strasse", "brü", "spree", "spree-", "köln", "koel", "fach", "bayern m", "zu",
        ],
    },
    "en": {
        "language": "eng",
        "title": "AuraGo Test Wiki (English)",
        "articles": [
            ("London", "London", [
                "London is the capital of England and the United Kingdom. The River Thames flows through the city.",
                "Many bridges cross the river in London.",
            ]),
            ("River_Thames", "River Thames", [
                "The Thames is a river in southern England. It flows east through London to the North Sea.",
            ]),
            ("Bridge", "Bridge", [
                "A bridge is a structure that carries a road or railway over an obstacle.",
                "Engineers have built bridges from wood, stone and steel.",
            ]),
            ("Tower_Bridge", "Tower Bridge", [
                "Tower Bridge is a famous bridge in London. It opens to let tall ships pass along the Thames.",
            ]),
            ("Running", "Running", [
                "Running is a way of moving quickly on foot. A runner runs faster than a walker.",
                "Many people enjoy running in parks.",
            ]),
            ("Marathon", "Marathon", [
                "A marathon is a running race of about forty-two kilometres. Runners train for months before a marathon.",
            ]),
            ("Apple", "Apple", [
                "The apple is a sweet fruit that grows on trees. Apples are often red, green or yellow.",
            ]),
            ("Apple_pie", "Apple pie", [
                "Apple pie is a baked dessert filled with apples and sugar. It is popular in England and America.",
            ]),
            ("Pineapple", "Pineapple", [
                "The pineapple is a tropical fruit with a rough skin. Despite its name it is not related to the apple.",
            ]),
            ("Café", "Café", [
                "A café is a small restaurant that serves coffee and cakes. Many cafés in Paris have tables outside.",
            ]),
            ("Naïve_art", "Naïve art", [
                "Naïve art is created by artists without formal training. The paintings often show bright colours.",
            ]),
            ("C++", "C++", [
                "C++ is a programming language based on C. Programmers use C++ for games and operating systems.",
            ]),
            ("AT&T", "AT&T", [
                "AT&T is a large telecommunications company in the United States. It started as a telephone company.",
            ]),
            ("O'Brien", "O'Brien", [
                "O'Brien is a common Irish surname. Many people named O'Brien live in Ireland and America.",
            ]),
            ("The_Beatles", "The Beatles", [
                "The Beatles were a rock band from Liverpool. The band released many famous songs in the 1960s.",
            ]),
            ("Mount_Everest", "Mount Everest", [
                "Mount Everest is the highest mountain on Earth. Climbers need oxygen near the summit.",
            ]),
            ("2024_Summer_Olympics", "2024 Summer Olympics", [
                "The 2024 Summer Olympics took place in Paris. Athletes from many countries competed in running and swimming.",
            ]),
            ("Email", "Email", [
                "Email is a method of sending messages between computers. People check their e-mail every day.",
            ]),
            ("Swimming", "Swimming", [
                "Swimming is moving through water using arms and legs. Swimmers compete in pools and rivers.",
            ]),
        ],
        "redirects": [
            ("Big_Smoke", "Big Smoke", "London"),
            ("Cafe", "Cafe", "Café"),
            ("Beatles", "Beatles", "The_Beatles"),
            ("E-mail", "E-mail", "Email"),
            ("Thames", "Thames", "River_Thames"),
        ],
        "fulltext": [
            "london", "bridges", "bridge london", "running", "runs", "runner", "apple", "apples pie", "cafe",
            "café", "c++", "at&t", "o'brien", "the beatles", "beatles", "mount everest", "e-mail", "2024",
            "river thames", "swimming running", "colours",
        ],
        "suggest": [
            "l", "lon", "london", "tower b", "tower bridge", "app", "apple p", "pine", "caf", "café", "the b",
            "beat", "c+", "c++", "at&", "o'b", "2024", "2024 s", "mount e", "e-m", "em", "riv", "thames",
        ],
    },
    "pl": {
        "language": "pol",
        "title": "AuraGo Testowa Wiki (Polski)",
        "articles": [
            ("Warszawa", "Warszawa", [
                "Warszawa jest stolicą Polski. Miasto leży nad Wisłą.",
                "W Warszawie mieszka prawie dwa miliony ludzi.",
            ]),
            ("Kraków", "Kraków", [
                "Kraków to dawna stolica Polski. Na wzgórzu Wawel stoi zamek królewski. Kraków leży nad Wisłą.",
            ]),
            ("Łódź", "Łódź", [
                "Łódź jest dużym miastem w centrum Polski. Dawniej Łódź słynęła z fabryk włókienniczych.",
            ]),
            ("Wisła", "Wisła", [
                "Wisła jest najdłuższą rzeką Polski. Rzeka płynie przez Kraków i Warszawę do Morza Bałtyckiego.",
            ]),
            ("Gdańsk", "Gdańsk", [
                "Gdańsk to miasto portowe nad Morzem Bałtyckim. W porcie cumują duże statki.",
            ]),
            ("Poznań", "Poznań", [
                "Poznań leży nad rzeką Wartą. Na starym rynku stoi piękny ratusz.",
            ]),
            ("Wrocław", "Wrocław", [
                "Wrocław leży nad Odrą i ma wiele mostów. Miasto słynie z małych krasnali.",
            ]),
            ("Zamek_Królewski_w_Warszawie", "Zamek Królewski w Warszawie", [
                "Zamek Królewski stoi na Starym Mieście w Warszawie. Dawniej mieszkali w nim królowie Polski.",
            ]),
            ("Pierogi", "Pierogi", [
                "Pierogi to tradycyjne danie kuchni polskiej. Nadzienie może zawierać ser, ziemniaki albo kapustę.",
            ]),
            ("Bigos", "Bigos", [
                "Bigos to potrawa z kapusty i mięsa. Najlepiej smakuje po kilku dniach gotowania.",
            ]),
            ("Żubr", "Żubr", [
                "Żubr jest największym ssakiem lądowym Europy. Żubry żyją w Puszczy Białowieskiej.",
            ]),
            ("Bocian", "Bocian", [
                "Bocian biały przylatuje do Polski wiosną. Bociany budują gniazda na dachach i słupach.",
            ]),
            ("Morze_Bałtyckie", "Morze Bałtyckie", [
                "Morze Bałtyckie leży na północy Europy. Woda w tym morzu jest mało słona.",
            ]),
            ("Tatry", "Tatry", [
                "Tatry to najwyższe góry w Polsce. Najwyższym szczytem po polskiej stronie są Rysy.",
            ]),
            ("Zakopane", "Zakopane", [
                "Zakopane to miasto u stóp Tatr. Zimą przyjeżdża tu wielu narciarzy.",
            ]),
            ("Fryderyk_Chopin", "Fryderyk Chopin", [
                "Fryderyk Chopin był polskim kompozytorem i pianistą. Urodził się niedaleko Warszawy.",
            ]),
        ],
        "redirects": [
            ("Krakow", "Krakow", "Kraków"),
            ("Lodz", "Lodz", "Łódź"),
            ("Stolica_Polski", "Stolica Polski", "Warszawa"),
            ("Chopin", "Chopin", "Fryderyk_Chopin"),
        ],
        "fulltext": [
            "warszawa", "warszawie", "kraków", "krakow", "łódź", "lodz", "wisła", "wisla", "rzeka", "rzeką",
            "miasto wisłą", "stolica polski", "żubr", "zubr", "morze bałtyckie", "tatry zakopane",
        ],
        "suggest": [
            "w", "war", "warsz", "kra", "krak", "łó", "lod", "lodz", "zamek k", "pie", "żu", "zu", "morze",
            "tat", "chop", "fryderyk c",
        ],
    },
    "ja": {
        "language": "jpn",
        "title": "AuraGo テストウィキ (日本語)",
        "articles": [
            ("東京", "東京", [
                "東京は日本の首都です。東京には多くの人が住んでいます。",
                "東京タワーや東京駅が有名です。",
            ]),
            ("京都", "京都", ["京都は古い都です。京都には多くのお寺や神社があります。"]),
            ("大阪", "大阪", ["大阪は西日本の大きな都市です。大阪の人は食べ物が大好きです。"]),
            ("富士山", "富士山", ["富士山は日本で一番高い山です。晴れた日には東京からも見えます。"]),
            ("東京タワー", "東京タワー", ["東京タワーは東京にある赤い電波塔です。展望台から街を見渡せます。"]),
            ("東京駅", "東京駅", ["東京駅は東京の中心にある大きな駅です。新幹線もここから出発します。"]),
            ("寿司", "寿司", ["寿司は酢飯と魚を合わせた日本料理です。回転寿司は安くて人気があります。"]),
            ("ラーメン", "ラーメン", ["ラーメンは中華麺を使ったスープ料理です。味噌ラーメンは北海道で生まれました。"]),
            ("カレーライス", "カレーライス", ["カレーライスは日本で人気の家庭料理です。ご飯にカレーをかけて食べます。"]),
            ("新幹線", "新幹線", ["新幹線は日本の高速鉄道です。東京から大阪まで約二時間半で走ります。"]),
            ("桜", "桜", ["桜は春に咲く花です。多くの人が公園で花見を楽しみます。"]),
            ("日本語", "日本語", ["日本語は日本で話されている言語です。ひらがな、カタカナ、漢字を使います。"]),
            ("ひらがな", "ひらがな", ["ひらがなは日本語の文字の一つです。子どもは最初にひらがなを習います。"]),
            ("北海道", "北海道", ["北海道は日本の北にある大きな島です。冬はとても寒く、雪がたくさん降ります。"]),
            ("沖縄", "沖縄", ["沖縄は日本の南にある島々です。きれいな海と暖かい気候で知られています。"]),
            ("ガンダム", "ガンダム", ["ガンダムは日本のロボットアニメです。プラモデルも人気があります。"]),
            ("Tokyo_Skytree", "東京スカイツリー", ["東京スカイツリーは東京の新しい電波塔です。英語ではTokyo Skytreeと呼ばれます。"]),
        ],
        "redirects": [
            ("とうきょう", "とうきょう", "東京"),
            ("Tokyo", "Tokyo", "東京"),
            ("スカイツリー", "スカイツリー", "Tokyo_Skytree"),
        ],
        "fulltext": [
            "東京", "日本", "タワー", "東京 タワー", "東京タワー", "ラーメン", "ガンダム", "カンタム", "寿司",
            "新幹線 大阪", "skytree", "tokyo", "ひらがな",
        ],
        "suggest": [
            "東", "東京", "東京タ", "東京ス", "とう", "ラー", "tok", "ガン", "カン", "新", "スカイ",
        ],
    },
}

# Programmatic fixture that forces multi-level B-trees, continuation posting
# chunks, multi-component items and a compressed multi-component docdata tag.
BULK = {
    "language": "eng",
    "title": "AuraGo Bulk Fixture",
    "count": 1100,
    "fulltext": ["zebra", "group g3 zebra", "bulk item", "0550", "longtitleword"],
    "suggest": ["bulk item 05", "bulk item 0550", "bulk item 0550 ", "longtitleword"],
    # Terms whose complete postings go into the bulk golden (the rest only by count).
    "golden_terms": ["zebra", "bulk", "item", "g3", "0001", "0550", "1100", "Zbulk", "0posanchor"],
}

# Strings for the ICU "Lower; NFD; [:M:] remove; NFC" parity golden.
NORMALIZE_SAMPLES = [
    "Häuser in Berlin", "MÜNCHEN", "Straße", "STRASSE", "ẞ", "Œuvre", "Ångström", "Łódź", "Żubr",
    "Kraków", "naïve café", "İstanbul", "ΟΔΥΣΣΕΥΣ", "Σίσυφος", "ガンダム", "パン", "東京タワー",
    "हिन्दी", "Ελληνικά", "ﬁligree", "Ⅻ", "Ｆｕｌｌｗｉｄｔｈ", "Dvořák", "Søren Kierkegaard", "Å",
]

# Already-normalised strings for the TermGenerator (STEM_NONE, FLAG_CJK_NGRAM) parity golden.
TOKENIZE_SAMPLES = [
    "hauser in berlin", "o'brien at&t", "c++ and c# and f+++", "fish+chips", "3.14 1,000 2024-10-09",
    "e-mail spree-athen", "foo_bar baz", "東京タワー", "abc東京def", "東京 タワー と ラーメン", "한국어 텍스트",
    "中文分词测试", "word​joined", "don’t", "0posanchor berlin", "x" * 70 + " short",
    "ｆｕｌｌｗｉｄｔｈ ７", "naive cafe", "hindi हनद", "zahl 1.2.3 a1.2",
]

# Words outside the fixtures that must stem identically in Go and Xapian.
EXTRA_STEM_WORDS = {
    "de": ["häuser", "hauses", "brücken", "fußballvereine", "universitäten", "straßen", "äpfel", "laufen", "läuft", "bäckereien"],
    "en": ["running", "runs", "runner", "bridges", "generously", "happiness", "organization", "swimming", "apples", "colours"],
    "fr": ["chevaux", "continuellement", "nationalité", "été", "maisons"],
    "es": ["canciones", "rápidamente", "nacionalidad", "casas"],
    "it": ["abbandonata", "città", "nazionalità", "case"],
    "nl": ["lichamelijk", "huizen", "fietsen", "kinderen"],
    "no": ["bøkene", "huset", "kjærlighet"],
    "nb": ["bøkene", "huset", "kjærligheten", "byene"],
    "pt": ["evolução", "revoluções", "nacionalidade", "casas"],
    "sv": ["kärleken", "husen", "flickorna"],
    "da": ["kærligheden", "husene", "pigerne"],
}

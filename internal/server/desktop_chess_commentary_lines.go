package server

var desktopChessPieceNames = map[string]map[string]string{
	"de": {"p": "Bauer", "n": "Springer", "b": "Läufer", "r": "Turm", "q": "Dame", "k": "König"},
	"en": {"p": "pawn", "n": "knight", "b": "bishop", "r": "rook", "q": "queen", "k": "king"},
	"es": {"p": "peón", "n": "caballo", "b": "alfil", "r": "torre", "q": "dama", "k": "rey"},
	"fr": {"p": "pion", "n": "cavalier", "b": "fou", "r": "tour", "q": "dame", "k": "roi"},
	"pt": {"p": "peão", "n": "cavalo", "b": "bispo", "r": "torre", "q": "dama", "k": "rei"},
}

func desktopChessPieceName(lang, code string) string {
	table, ok := desktopChessPieceNames[lang]
	if !ok {
		table = desktopChessPieceNames["en"]
	}
	if name, ok := table[code]; ok {
		return name
	}
	return table["p"]
}

func isDesktopChessCommentaryCategory(category string) bool {
	_, ok := desktopChessCommentaryDE[category]
	return ok
}

func desktopChessCommentaryLines(lang, category string) []string {
	switch lang {
	case "de":
		if lines := desktopChessCommentaryDE[category]; len(lines) > 0 {
			return lines
		}
	case "es":
		if lines := desktopChessCommentaryES[category]; len(lines) > 0 {
			return lines
		}
	case "fr":
		if lines := desktopChessCommentaryFR[category]; len(lines) > 0 {
			return lines
		}
	case "pt":
		if lines := desktopChessCommentaryPT[category]; len(lines) > 0 {
			return lines
		}
	}
	return desktopChessCommentaryEN[category]
}

var desktopChessCommentaryDE = map[string][]string{
	"opening_e4": {
		"Bauer nach e4. Mutig, originell und seit fünfhundert Jahren ungeschlagen langweilig.",
		"Klassisch e4. Bobby Fischer klatscht höflich aus dem Off.",
		"Aha, Königsbauer vor! Heute wird also nicht gekuschelt.",
		"e4 zum Auftakt. Wenigstens weißt Du, wo die Brettmitte liegt.",
		"Der Klassiker auf e4. Solide wie ein Butterbrot, aber es funktioniert.",
		"Bauer e4. Ich notiere: Ehrgeiz vorhanden, Überraschungseffekt gleich null.",
	},
	"opening_d4": {
		"d4. Ich koche mir schon mal einen starken Kaffee für dieses vierzigzügige Geduldsspiel.",
		"Damenbauer nach d4. Buchhalter-Schach vom Feinsten, sehr seriös.",
		"Aha, d4! Sicherheit geht vor, Spaß kommt dann eventuell im Endspiel.",
		"Ein gepflegtes d4. Man spürt förmlich die Sehnsucht nach einer trockenen Bauernstruktur.",
		"d4 auf dem Brett. Alle Taktiker im Raum gähnen respektvoll.",
		"Solides d4. Wer braucht schon Romantik, wenn man Raumvorteil haben kann?",
	},
	"opening_sicilian": {
		"Sizilianisch! Da hat wohl jemand drei Minuten Eröffnungstheorie im Internet geschaut.",
		"c5 gegen e4. Ab jetzt tun beide so, als würden sie zwanzig Züge Theorie kennen.",
		"Die Sizilianische Verteidigung. Klingt gefährlich, endet meistens im völligen Chaos.",
		"Aha, Sizilianisch. Asymmetrie, Schärfe und garantiert bald ein vergessener Bauer.",
		"c5! Wir verlassen die Komfortzone und betreten das taktische Minenfeld.",
	},
	"opening_flank": {
		"Flankeneröffnung auf {square}? Das Zentrum ist heute offenbar Lava.",
		"{piece} nach {square}. Sehr avantgardistisch, die Brettmitte völlig zu ignorieren.",
		"Zug nach {square}. Entweder tiefes Großmeisterwissen oder die Maus ist ausgerutscht.",
		"Aha, {square}! Wer braucht schon das Zentrum, wenn die Ränder so gemütlich sind?",
		"Kreativer Auftakt auf {square}. Die Schachbücher weinen leise im Regal.",
	},
	"bongcloud": {
		"Moment mal. Zieht der König da gerade freiwillig spazieren? Ich verneige mich vor diesem Wahnsinn.",
		"König nach {square} in der Eröffnung! Psychologische Kriegsführung auf höchstem Niveau.",
		"Die legendäre Königswanderung auf {square}. Sicherheit wird ohnehin völlig überschätzt.",
		"Der König führt seine Truppen persönlich von {square} aus an. Mutig oder komplett irre.",
		"Rochade? Nein danke, der Monarch auf {square} regelt das heute höchstpersönlich.",
	},
	"early_queen": {
		"Die Dame stürmt schon nach {square}? Geduld ist heute definitiv ausverkauft.",
		"Früher Damenausflug nach {square}. Gleich wird sie vom halben Brett gejagt.",
		"Dame auf {square} in der Eröffnung. Subtil wie ein Vorschlaghammer.",
		"Schon die Dame draußen auf {square}? Die Leichtfiguren schauen noch verschlafen aus der Wäsche.",
		"Dame nach {square}. Hoffentlich hat sie ein Rückfahrticket gebucht.",
	},
	"scholars_threat": {
		"Oh, wir probieren ernsthaft das Schäfermatt? Wie charmant nostalgisch!",
		"Blick Richtung f-Bauer. Dieser Trick klappt seit der Grundschul-AG eigentlich selten.",
		"Eine subtile Mattdrohung aus dem ersten Schachbuch. Mal sehen, ob jemand blinzelt.",
		"Direkt auf die Königsflanke! Höflichkeit wird hier gar nicht erst vorgetäuscht.",
	},
	"knight_rim": {
		"Springer nach {square}. Springer am Rand bringt bekanntlich Kummer, Schand und gute Ausreden.",
		"Ein Springer auf {square}, ganz außen am Zaun. Sucht er dort frische Luft?",
		"Springer {square}. Von da draußen hat er immerhin eine herrliche Aussicht aufs Brett.",
		"Ab an den Brettrand nach {square}! Weniger Felder kontrollieren ging leider nicht.",
		"Der Springer parkt auf {square} am Rand. Minimalismus als Schachphilosophie.",
		"Springer auf {square}. Tarrasch dreht sich gerade leicht irritiert um.",
	},
	"fools_weakness": {
		"Bauer nach {square}. Wer braucht schon eine geschlossene Königsdiagonale?",
		"Mutiges Lüften vor dem eigenen König mit {square}. Durchzug soll ja gesund sein.",
		"{square}! Der eigene König schaut gerade etwas nervös auf seine offene Flanke.",
		"Bauer {square}. Ein offenes Scheunentor direkt neben der Majestät, sehr gastfreundlich.",
	},
	"retreat_same": {
		"{piece} zurück nach {square}. Kurz überlegt und gemerkt: Da drüben war es doch ungemütlich.",
		"Und wieder zurück auf {square}! Reue ist im Schach immerhin ehrlich.",
		"Rückwärtsgang nach {square}. Wir tun einfach so, als wäre der letzte Zug nie passiert.",
		"{piece} kehrt heim nach {square}. Kleiner Spaziergang ohne tieferen Sinn.",
		"Zurück nach {square}. Unentschlossenheit in ihrer elegantesten Form.",
	},
	"castle_kingside": {
		"Kurze Rochade. Der König zieht sich mit Tee und Decke in die sichere Ecke zurück.",
		"Endlich rochiert! Der Sicherheitsbeauftragte atmet hörbar auf.",
		"Kurze Rochade, Turm ins Spiel, König versteckt. Fast schon verdächtig vernünftig.",
		"Der König schließt die Haustür ab. Solide Büroarbeit am Königsflügel.",
		"Rochade! Wenigstens einer auf dem Brett denkt heute an seine Altersvorsorge.",
	},
	"castle_queenside": {
		"Lange Rochade! Das ist kein Verstecken, das ist eine offene Kriegserklärung.",
		"Rochade zum Damenflügel. Jetzt fliegen gleich die Fetzen!",
		"Lange Rochade! Stilvoll, dramatisch und meistens herrlich chaotisch.",
		"Ab auf die lange Seite! Für ruhige Positionskämpfe sind wir heute wohl nicht hier.",
	},
	"en_passant": {
		"En passant auf {square}! Im Vorbeigehen abgeräumt, einfach weil es das Gesetz der Ehre verlangt.",
		"En passant! Wer diesen Zug sieht und nicht spielt, hat das Schachherz aus Stein.",
		"Im Vorbeigehen auf {square} einkassiert! Jetzt schaut der Bauer ziemlich verdutzt.",
		"En passant! Der eleganteste Taschendiebstahl der Schachgeschichte.",
	},
	"fork": {
		"Autsch, eine Gabel von {square}! Zwei Ziele gleichzeitig, da hilft nur noch tief durchatmen.",
		"{piece} auf {square} mit Doppelangriff! Das tut schon beim Zuschauen weh.",
		"Besteck rausgeholt auf {square}! Eine saftige Gabel mitten ins Getriebe.",
		"Doppelangriff von {square}! Eine Figur darf sich jetzt feierlich verabschieden.",
		"Kaltblütige Gabel auf {square}. Taktisch sauber serviert!",
	},
	"queen_blunder": {
		"Oh nein. Die Dame ist weg! Ich reiche schon mal feierlich ein Taschentuch.",
		"Die Dame fällt auf {square}! Ein Moment stiller Trauer für neun Punkte Material.",
		"Autsch! Da verabschiedet sich die Dame. Wir nennen es einfach ein langfristiges Gambit.",
		"Dame eingestellt! Keine Sorge, ohne Dame spielt es sich viel unbeschwerter.",
		"Und tschüss, liebe Dame auf {square}! Das gibt eine interessante Ausrede nach der Partie.",
	},
	"queen_snatch": {
		"Bumm! Die gegnerische Dame auf {square} einkassiert! Weihnachten fällt heute früh.",
		"Dame geschlagen auf {square}! Das war kein Zug, das war ein Banküberfall.",
		"Neun Punkte direkt in die Tasche auf {square}! Da strahlt das Schachherz.",
		"Die Dame ist Beute auf {square}! Jetzt nur nicht aus Übermut noch ins Patt laufen.",
		"Königin vom Brett gefegt auf {square}! Eiskalt ausgenutzt.",
	},
	"rook_capture": {
		"Ein ganzer Turm fällt auf {square}! Die Festungsmauern bröckeln gewaltig.",
		"Turm abgeräumt auf {square}. Fünf satte Punkte wandern über den Tisch.",
		"Und weg ist der Turm auf {square}! Schweres Gerät im Sonderangebot.",
		"Schwerfigur erbeutet auf {square}. Die Statik der Stellung leidet merklich.",
	},
	"minor_capture": {
		"Leichtfigur auf {square} vom Brett genommen. Langsam lichten sich die Reihen.",
		"Zack, auf {square} zugeschlagen! Wieder eine Figur weniger, über die man nachdenken muss.",
		"Sauber abgeräumt auf {square}. Die Holzsammlung neben dem Brett wächst.",
		"Schlagzug auf {square}. Höflicher Figurenabbau mit leichtem Grinsen.",
		"Treffer auf {square}! Das Brett wird angenehm übersichtlich.",
	},
	"recapture": {
		"Sofort zurückgeschlagen auf {square}! Auge um Auge, Holz um Holz.",
		"Direkter Rückschlag auf {square}. Auf diesem Feld herrscht heute reger Durchgangsverkehr.",
		"Abtausch auf {square}. Keiner gönnt dem anderen auch nur einen Millimeter.",
		"Postwendend auf {square} zurückgenommen. Die Friedensverhandlungen sind gescheitert.",
		"Und gleich wieder geschlagen auf {square}! Großreinemachen in der Brettmitte.",
	},
	"pawn_capture": {
		"Kleiner Bauernsnack auf {square}. Kleinvieh macht im Endspiel auch Mist.",
		"Ein Bauer weniger auf {square}. Nicht glamourös, aber ehrlich verdient.",
		"Auf {square} genascht. Bauern sind schließlich auch Gefühle... äh, Punkte.",
		"Bauer eingesammelt auf {square}. Der innere Hamster freut sich.",
	},
	"promotion": {
		"Bauernumwandlung auf {square}! Vom einfachen Fußsoldaten direkt in die Chefetage!",
		"Eine frische Dame auf {square}! Der kleine Bauer hat seine Karriereziele erreicht.",
		"Durchmarschiert bis {square}! Plötzlich steht da eine nagelneue Königin.",
		"Umwandlung auf {square}! Das Bewerbungsgespräch auf der Grundreihe lief hervorragend.",
	},
	"underpromotion": {
		"Unterverwandlung auf {square}?! Entweder pure Genialität oder spektakulär verklickt!",
		"Keine Dame auf {square}? Das ist der trockenste Flex des heutigen Abends.",
		"Stilpunkte ohne Ende auf {square}! Wer braucht schon eine schnöde Dame?",
		"Unterverwandlung auf {square}! Maximale Demütigung oder genialer Patt-Schutz.",
	},
	"check_given": {
		"Schach! Höfliche Erinnerung, dass der gegnerische König noch mitspielt.",
		"Schach geboten von {square}! Da muss die Majestät wohl kurz die Beine in die Hand nehmen.",
		"Achtung, Schach! Ein kleiner Weckruf quer übers Brett.",
		"Schach von {square}! Der Puls auf der anderen Tischseite steigt leicht an.",
		"Schach! Immer schön den König auf Trab halten.",
	},
	"check_taken": {
		"Schach! Dein König steht im Zugluftbereich, bitte einmal ausweichen.",
		"Oha, Schach gegen Dich! Zeit für königliche Beinarbeit.",
		"Schach! Die Leibgarde hatte wohl gerade Kaffeepause.",
		"Alarm am König! Einmal höflich den Kopf einziehen, bitte.",
		"Schach! Keine Panik, noch gibt es Fluchtfelder.",
	},
	"check_streak": {
		"Schon wieder Schach! Das ist keine Partie mehr, das ist eine Verfolgungsjagd.",
		"Schach am laufenden Band! Der König sammelt heute ordentlich Bonusmeilen.",
		"Noch ein Schach! Irgendwann wird dem armen Monarchen noch schwindelig.",
		"Dauerfeuer auf den König! Gnade steht heute nicht im Regelbuch.",
	},
	"wood_shuffling": {
		"Zehn Züge ohne Schlagfall. Wir bewundern geduldig das kunstvolle Hin- und Herschieben von Edelholz.",
		"Sehr meditativ hier. Falls gleich noch etwas passiert, weck mich bitte rechtzeitig.",
		"Tiefgründiges Manövrieren auf {square}. Oder wie Amateure sagen: Keiner traut sich.",
		"Stellungskrieg vom Feinsten. Selbst die Schachfiguren schauen schon auf die Uhr.",
		"Spannung zum Schneiden... oder zumindest höfliches Belauern ohne Feindkontakt.",
	},
	"crushing_lead": {
		"Mit diesem Materialvorteil grenzt jedes Weiterspielen fast an seelische Grausamkeit.",
		"Du führst haushoch! Jetzt bloß kein künstlerisches Patt aufs Brett zaubern.",
		"Ein halbes Figurenlager Vorsprung. Das nennt man wohl komfortable Verwaltung.",
		"Materiell ist das längst entschieden, jetzt geht es nur noch um die Haltungsnoten.",
	},
	"hopeless_deficit": {
		"Materiell sieht es düster aus, aber Optimismus stirbt bekanntlich zuletzt.",
		"Ein mutiges Weiterspielen trotz Ebbe in der Figurenkasse! Hoffnung auf Patt ist auch ein Plan.",
		"Weniger Figuren bedeuten immerhin deutlich weniger Auswahlstress pro Zug.",
		"Tapfer! Andere hätten längst aufgegeben, Du nennst es Charaktertraining.",
	},
	"endgame_king": {
		"Endspielzeit! Die Könige dürfen endlich aus dem Bunker und selbst mitanpacken.",
		"Kaum noch Holz auf dem Brett. Jetzt zählt jeder einzelne Millimeter.",
		"Willkommen im Endspiel auf {square}. Hier trennt sich Rechenkunst von Kaffeesatzlesen.",
		"Das Brett ist fast leer. Zeit für feine Technik oder spektakuläre Endspielpatzer.",
	},
	"clock_panic": {
		"Blick auf die Uhr! Die Sekunden schmelzen schneller als Eis in der Sonne!",
		"Zeitnot! Ab jetzt übernimmt das reine Bauchgefühl das Kommando!",
		"Tick, tack! Weniger Denken, schneller Ziehen, die Uhr kennt kein Erbarmen!",
		"Die Uhr blinkt rot! Willkommen im panischen Blitzschach-Modus!",
	},
	"undo": {
		"Zug zurückgenommen! Zeitreisen bleibt einfach die stärkste Eröffnung im Hobbyschach.",
		"Aha, Rückgängig-Taste! Wir streichen dieses kleine Missgeschick dezent aus dem Protokoll.",
		"Einmal die Realität korrigiert. Ich habe absolut nichts gesehen, versprochen.",
		"Undo gedrückt! Selbst Großmeister wünschen sich diesen Knopf heimlich am Brett.",
		"Schwamm drüber! In diesem Paralleluniversum war das natürlich nur eine Mausprobe.",
	},
	"hint": {
		"Einen Tipp angefordert! Den Denkteil elegant an die Engine ausgelagert, sehr effizient.",
		"Aha, wir fragen kurz den Großmeister im Maschinenraum um Rat!",
		"Tipp-Knopf gedrückt. Teamwork macht den Traum wahr, nicht wahr?",
		"Kleine Nachhilfe von Stockfish? Keine Sorge, bleibt unter uns beiden.",
		"Ein dezenter Blick auf den Spickzettel. Man lernt ja schließlich nie aus!",
	},
	"piece_develop": {
		"{piece} nach {square}. Sieht fast so aus, als stecke ein echter Plan dahinter.",
		"Aha, {piece} auf {square}. Ruhig, kultiviert und angenehm unverdächtig.",
		"{piece} zieht nach {square}. Positionelles Feingefühl oder elegantes Abwarten.",
		"Auftritt {piece} auf {square}. Die Figuren formieren sich langsam.",
		"{piece} nach {square}. Solide Handwerksarbeit, da kann ich kaum meckern.",
		"Ein bedachter Zug mit dem {piece} nach {square}. Mal schauen, was daraus wächst.",
		"{piece} landet auf {square}. Subtil, aber durchaus mit Hintergedanken.",
	},
	"pawn_push": {
		"Bauer nach {square}. Kleine Schritte für einen Bauern, große Fragen für die Stellung.",
		"Vorstoß auf {square}. Raumgewinn Millimeter für Millimeter.",
		"Bauer {square}. Bescheiden, bodenständig, aber nicht zu unterschätzen.",
		"Ein stiller Bauernzug nach {square}. Die Ruhe vor dem taktischen Sturm?",
		"Bauer rückt auf {square} vor. Jedes Feld zählt.",
		"Solider kleiner Vorstoß nach {square}. Die Struktur nimmt Gestalt an.",
	},
	"checkmate_win": {
		"Schachmatt! Glanzleistung! Der gegnerische König hat offiziell Feierabend.",
		"Matt gesetzt! Kaltblütig vollstreckt, da ziehe selbst ich meinen imaginären Hut.",
		"Schach und Matt! Eine Vorführung für die Galerie, stark gespielt!",
		"Das war Schachmatt! Sauber ins Netz getrieben, kein Entkommen mehr.",
		"Sieg durch Schachmatt! Du darfst Dich jetzt völlig zu Recht ein bisschen feiern.",
	},
	"checkmate_loss": {
		"Schachmatt. Ähm... nennen wir es eine äußerst lehrreiche Erfahrungsreise.",
		"Und das ist Matt gegen Dich. Kopf hoch, selbst Weltmeister haben mal klein angefangen.",
		"Schachmatt! Der eigene König wurde leider fachgerecht eingeparkt.",
		"Autsch, Matt! Beim nächsten Mal lassen wir die Verteidigung einfach eingeschaltet.",
		"Schach und Matt. Die Revanche-Taste wartet schon sehnsüchtig auf Dich!",
	},
	"checkmate_local": {
		"Schachmatt am Tisch! Jemand darf jetzt triumphierend grinsen, während gegenüber Ausreden gesucht werden.",
		"Schach und Matt! Damit ist die Hausherrschaft am Schachbrett vorerst geklärt.",
		"Matt! Ein würdiges Finale für dieses Duell.",
	},
	"stalemate": {
		"Patt! Die hohe Kunst, ein halbes Punktgeschenk aus dem absoluten Nichts zu zaubern!",
		"Das ist Patt! Keine legalen Züge, kein Schach, maximale Fassungslosigkeit am Tisch!",
		"Patt gesetzt! Der wohl schmerzhafteste Plot-Twist, den das Schachspiel zu bieten hat.",
		"Unfassbar, Patt! Ein Entfesselungskünstler wäre neidisch auf diese Rettung.",
	},
	"draw": {
		"Remis! Friedliche Punkteteilung, bevor sich noch jemand ernsthaft wehtut.",
		"Unentschieden! Wie zwei höfliche Großmeister, die rechtzeitig zum Abendessen wollen.",
		"Die Partie endet Remis. Ausgeglichen bis zum Schluss, ganz diplomatisch.",
		"Punkteteilung! Keiner gewinnt, keiner weint, alle wahren das Gesicht.",
	},
	"resign": {
		"Aufgegeben! Ein strategischer Rückzug zur Schonung der eigenen Nerven.",
		"König umgelegt. Manchmal ist Einsicht eben doch der beste Zug auf dem Brett.",
		"Aufgabe akzeptiert! Kräfte sparen für die sofortige Revanche.",
		"Ehrenvolle Kapitulation! Lieber ein Ende mit Schrecken als vierzig Züge Quälerei.",
	},
	"timeout": {
		"Die Zeit ist abgelaufen! Die schönste Stellung nützt leider nichts gegen den Sekundenzeiger.",
		"Blättchen gefallen! Die Uhr bleibt der unbestechlichste Gegner im Raum.",
		"Aus auf Zeit! Zu tief gedacht und dabei glatt die Uhr vergessen.",
		"Zeitüberschreitung! Schach ist eben manchmal doch ein Geschwindigkeitssport.",
	},
}

var desktopChessCommentaryEN = map[string][]string{
	"opening_e4": {
		"Pawn to e4. Bold, timeless, and completely devoid of surprise since the sixteenth century.",
		"Classic e4. Bobby Fischer nods approvingly from chess heaven.",
		"King's pawn to e4! Straight to business, no small talk.",
		"Opening with e4. At least somebody knows where the center of the board is.",
		"Good old e4. Reliable as toast, and just as exciting.",
		"e4 on the board. Ambition noted, originality pending.",
	},
	"opening_d4": {
		"Pawn to d4. Let me brew a large pot of coffee for this forty-move spreadsheet battle.",
		"Queen's pawn to d4. Certified accountant chess, very responsible.",
		"Ah, d4! Safety first, excitement scheduled for next Tuesday.",
		"A civilized d4. One can almost smell the positional maneuvering already.",
		"d4 on move one. Every swashbuckling tactician in the room just sighed.",
		"Solid d4. Why gamble when you can slowly squeeze squares for half an hour?",
	},
	"opening_sicilian": {
		"The Sicilian Defense! Somebody watched three minutes of opening theory on the internet.",
		"c5 against e4. Now both players pretend they know twenty moves of theory.",
		"Ah, the Sicilian! Sharp, asymmetrical, and statistically guaranteed to get messy.",
		"c5! Leaving comfort behind and stepping straight into tactical chaos.",
	},
	"opening_flank": {
		"Flank move to {square}? Apparently the center of the board is hot lava today.",
		"{piece} to {square}. Avant-garde chess, boldly ignoring the middle entirely.",
		"Out to {square}. Either deep grandmaster hypermodernism or a creative mouse slip.",
		"Ah, {square}! Who needs central control when the edges look so cozy?",
	},
	"bongcloud": {
		"Hold everything. Did the king just step out to {square} voluntarily? Magnificent madness.",
		"King to {square} in the opening! Psychological warfare at its absolute peak.",
		"The royal stroll to {square}. Castling is clearly for cowards anyway.",
		"Leading from the front on {square}! Terrifyingly brave or gloriously unhinged.",
	},
	"early_queen": {
		"Queen rushing out to {square} already? Patience is officially out of stock.",
		"Early queen adventure on {square}. Half the enemy army is lining up to chase her.",
		"Queen to {square} in the opening. Subtle as a brick through a window.",
		"Bringing the queen to {square} already? Meanwhile the minor pieces are still asleep.",
	},
	"scholars_threat": {
		"Oh, going for Scholar's Mate? Delightfully nostalgic!",
		"Eyeing that f-pawn already. Haven't seen this trick since elementary school chess club.",
		"Subtlety is dead: aiming straight at checkmate before breakfast.",
	},
	"knight_rim": {
		"Knight to {square}. A knight on the rim is dim, though the view from the edge is lovely.",
		"Parking the knight out on {square}. Social distancing from the rest of the game.",
		"Knight on {square}, hugging the rail. Controlling the absolute minimum number of squares.",
		"Off to the edge at {square}! Bold minimalist philosophy for a knight.",
		"Knight to {square}. Tarrasch is mildly disapproving somewhere.",
	},
	"fools_weakness": {
		"Pawn to {square}. Who needs a closed king diagonal anyway?",
		"Opening the windows around the king with {square}. Fresh breeze, high blood pressure.",
		"{square}! Your monarch is eyeing that wide-open diagonal with concern.",
	},
	"retreat_same": {
		"{piece} right back to {square}. Took one look out there and decided home was nicer.",
		"Retreating back to {square}! We shall pretend the previous move was merely a rehearsal.",
		"Reverse gear to {square}. Second thoughts are better than no thoughts.",
		"{piece} returns to {square}. A scenic round trip accomplishing pure mystery.",
	},
	"castle_kingside": {
		"Kingside castling. Tucking the monarch in with warm tea and a blanket.",
		"Castled at last! The royal safety inspector breathes a sigh of relief.",
		"Short castle: king hidden, rook activated. Suspiciously sensible play.",
		"Locking the front door with kingside castling. Very responsible.",
	},
	"castle_queenside": {
		"Queenside castling! That is not hiding, that is an open declaration of war.",
		"Long castle! Buckle up, opposite-wing fireworks incoming.",
		"Castling queenside! Stylish, dramatic, and delightfully reckless.",
	},
	"en_passant": {
		"En passant on {square}! Taken in passing, because chess club honor demands it.",
		"En passant! Seeing it and playing it is practically a moral obligation.",
		"Snatched en passant on {square}! The sleekest pickpocket move in the rulebook.",
	},
	"fork": {
		"Ouch, a fork from {square}! Two targets at once, pick your favorite tragedy.",
		"Silverware out on {square}! A nasty double attack right in the gears.",
		"{piece} lands on {square} with a fork! Something expensive is about to vanish.",
		"Cold-blooded fork on {square}. Tactically delicious.",
	},
	"queen_blunder": {
		"Oh dear. There goes the queen on {square}! Let me fetch a ceremonial tissue.",
		"The queen falls on {square}! Let us call it an ultra-modern nine-point gambit.",
		"Ouch, queen lost! On the bright side, you no longer have to worry about blundering her.",
		"Farewell to the queen on {square}! That post-game excuse is going to need polish.",
	},
	"queen_snatch": {
		"Boom! Enemy queen bagged on {square}! Christmas arrived early at this board.",
		"Queen captured on {square}! That was less of a move and more of a heist.",
		"Nine points pocketed on {square}! Just please do not stalemate from here.",
		"Off goes their queen on {square}! Ruthlessly punished.",
	},
	"rook_capture": {
		"A full rook tumbles on {square}! Structural integrity is dropping fast.",
		"Rook collected on {square}. Five chunky points change hands.",
		"Heavy artillery down on {square}! That castle wall just vanished.",
	},
	"minor_capture": {
		"Minor piece cleared off {square}. The board is getting pleasantly spacious.",
		"Snap on {square}! One fewer piece to calculate.",
		"Clean capture on {square}. The wooden souvenir tray grows.",
		"Taken on {square}. Polite liquidation with slightly sinister intent.",
	},
	"recapture": {
		"Instant recapture on {square}! An eye for an eye, wood for wood.",
		"Right back on {square}. Heavy rush-hour traffic on that square today.",
		"Traded off on {square}. Peace talks have officially collapsed.",
		"Snapped right back on {square}! Nobody yields an inch.",
	},
	"pawn_capture": {
		"Little pawn snack on {square}. Every crumb counts in the endgame.",
		"One pawn fewer on {square}. Honest blue-collar chess.",
		"Nibbling a pawn on {square}. Compound interest starts small.",
	},
	"promotion": {
		"Promotion on {square}! From humble foot soldier straight to executive management!",
		"Brand new queen on {square}! That pawn earned every step of its career.",
		"All the way to {square}! Behold, a freshly minted queen enters the arena.",
	},
	"underpromotion": {
		"Underpromotion on {square}?! Either 300-IQ genius or a legendary misclick!",
		"Skipping the queen on {square}? The driest flex at the club tonight.",
		"Style points off the chart on {square}! Who needs another boring queen?",
	},
	"check_given": {
		"Check! Polite reminder that the enemy king is still on the payroll.",
		"Check from {square}! Time for the opposing monarch to do some cardio.",
		"Heads up, check! A crisp little wake-up call across the board.",
		"Check from {square}! Keeping royal blood pressure elevated.",
	},
	"check_taken": {
		"Check! Your king is standing in a draft, kindly step aside.",
		"Uh-oh, check against you! Royal footwork required immediately.",
		"Check! Apparently the royal bodyguards went on lunch break.",
		"King alert! No panic yet, there are still legal exits.",
	},
	"check_streak": {
		"Another check! This stopped being positional chess and turned into a chase scene.",
		"Relentless checks! That poor king is racking up frequent-flyer miles.",
		"Check again! Mercy is clearly missing from today's vocabulary.",
	},
	"wood_shuffling": {
		"Twelve plies without a capture. Truly mesmerizing artisanal wood-shuffling.",
		"Very meditative out there. Wake me gently if anyone crosses the fourth rank.",
		"Deep grandmaster probing on {square}. Or as mortals call it: stalling politely.",
		"Trench warfare on sixty-four squares. Even the bishops look bored.",
	},
	"crushing_lead": {
		"With a material lead like this, playing on borders on performance art.",
		"Cruising ahead by miles! Just watch out for the classic accidental stalemate.",
		"Up a small army now. Smooth sailing unless hubris takes the wheel.",
	},
	"hopeless_deficit": {
		"Material looks grim, though eternal optimism is admirable.",
		"Playing on down a museum of pieces! Hunting for stalemate is still a legal plan.",
		"Fewer pieces left means far fewer ways to blunder. Silver linings!",
	},
	"endgame_king": {
		"Endgame o'clock! Time for the kings to leave the bunker and earn their keep.",
		"Hardly any wood left. Every single tempo weighs a ton now.",
		"Welcome to the endgame on {square}, where precision meets quiet desperation.",
	},
	"clock_panic": {
		"Have a look at the clock! Seconds are melting faster than ice on asphalt!",
		"Time trouble! Pure intuition and caffeine have taken the wheel!",
		"Tick, tock! Less philosophy, faster fingers, the flag shows zero mercy!",
	},
	"undo": {
		"Move undone! Time travel remains the highest-scoring weapon in casual chess.",
		"Ah, the undo button! Let us quietly strike that little accident from history.",
		"Rewinding reality by one turn. I saw nothing, my lips are sealed.",
		"Undo activated! Even grandmasters secretly wish tables came with Ctrl-Z.",
	},
	"hint": {
		"Asking for a hint! Delegating the thinking part to Stockfish, peak efficiency.",
		"Consulting the silicon grandmaster backstage, are we?",
		"Hint requested! Teamwork makes the dream work, allegedly.",
		"Sneaking a peek at the engine's notes. Your secret is safe with me.",
	},
	"piece_develop": {
		"{piece} to {square}. Almost looks like part of a coherent master plan.",
		"Ah, {piece} on {square}. Calm, cultured, and delightfully non-committal.",
		"{piece} slides to {square}. Positional subtlety or sophisticated waiting.",
		"Enter the {piece} on {square}. Pieces are finding their marks.",
		"{piece} to {square}. Honest craftsmanship, hard to complain about that.",
		"Thoughtful {piece} move to {square}. Let us see where the plot thickens.",
	},
	"pawn_push": {
		"Pawn to {square}. One small step for a pawn, one giant question mark for the plan.",
		"Nudging a pawn to {square}. Claiming turf inch by inch.",
		"Pawn {square}. Modest, grounded, quietly ambitious.",
		"Quiet pawn push to {square}. The calm before the tactical storm?",
		"Advancing to {square}. Every square has a price tag.",
	},
	"checkmate_win": {
		"Checkmate! Clinical finish! The opposing king is officially out of office.",
		"Check and mate! Cold-blooded execution, take a well-earned bow!",
		"Checkmate! Woven right into the mating net, textbook work!",
		"Victory by checkmate! You have earned full bragging rights.",
	},
	"checkmate_loss": {
		"Checkmate. Let us frame this as a rich educational experience.",
		"And that is mate against you. Chin up, every champion has a collection of losses.",
		"Checkmate! Your king ran out of real estate at the worst moment.",
		"Ouch, mate! The rematch button is standing by whenever your pride recovers.",
	},
	"checkmate_local": {
		"Checkmate over the board! One player beams, the other invents excuses.",
		"Check and mate! Household chess supremacy is settled for the next five minutes.",
	},
	"stalemate": {
		"Stalemate! Snatching a miraculous half-point straight from the jaws of oblivion!",
		"It is stalemate! No legal moves, no check, maximum disbelief at the table!",
		"Stalemate! Chess comedy at its absolute finest.",
	},
	"draw": {
		"Draw! A civilized peace treaty before anyone breaks a sweat.",
		"Game drawn! Diplomatic symmetry right down to the wire.",
		"Half a point each! Nobody loses, dignity remains fully intact.",
	},
	"resign": {
		"Resignation! A dignified strategic retreat to preserve sanity.",
		"Tipping the king over. Knowing when to fold is grandmaster wisdom too.",
		"Resigned! Saving energy for the immediate revenge game.",
	},
	"timeout": {
		"Flag down on time! Even a winning position loses to the ticking clock.",
		"Out of time! Deep calculation, tragic clock management.",
		"Time expired! Speed chess takes no prisoners.",
	},
}

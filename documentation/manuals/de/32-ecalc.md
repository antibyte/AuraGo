# Kapitel 32: ECalc – Elektronik-Formeln im Virtual Desktop

ECalc ist eine Formelsammlung für Elektronik im Virtual Desktop. Eine Rechenseite zeigt die Schaltung, die Formel und die
Ergebnisse nebeneinander und rechnet schon während du tippst. Du gibst die Werte ein, die du kennst, und ECalc berechnet den
Rest. Bei Widerständen schlägt es außerdem den nächsten Normwert aus einer E-Reihe vor, und der Rechner **Kombination von
Normwerten** findet Normwerte, die in Reihe oder parallel einen Zielwiderstand erreichen.

ECalc enthält 77 Rechner in zehn Kategorien. Der Agent kann mit denselben Formeln rechnen und für dich einen Rechner öffnen (siehe
[ECalc mit dem Agenten nutzen](#ecalc-mit-dem-agenten-nutzen)). Nachschlagetabellen, Bauteilcodes und Pinbelegungen folgen in
späteren Versionen (siehe [Wie es weitergeht](#wie-es-weitergeht)). ECalc läuft in deinem Browser. Es braucht keinen Provider und
hat keinen eigenen Schalter unter **Config**.

## Inhalt
1. [ECalc öffnen](#ecalc-öffnen)
2. [Einen Rechner finden](#einen-rechner-finden)
3. [Die Rechenseite](#die-rechenseite)
4. [Werte eingeben](#werte-eingeben)
5. [Normwerte (E-Reihen)](#normwerte-e-reihen)
6. [Einstellungen](#einstellungen)
7. [Die Rechner](#die-rechner)
8. [ECalc mit dem Agenten nutzen](#ecalc-mit-dem-agenten-nutzen)
9. [Wie es weitergeht](#wie-es-weitergeht)
10. [Datenschutz und Sicherheit](#datenschutz-und-sicherheit)
11. [Wenn etwas schiefgeht](#wenn-etwas-schiefgeht)

---

## ECalc öffnen

Öffne **ECalc** aus dem Startmenü, in der Kategorie **Entwicklung**. Das Fenster öffnet sich mit etwa 1100 × 760 Pixeln und lässt
sich in der Größe ändern. ECalc öffnet wieder den Rechner, den du zuletzt benutzt hast.

Das Fenster hat drei Spalten: links die **Kategorieleiste**, in der Mitte die **Liste** der Rechner und rechts die
**Rechenseite**. In einem schmalen Fenster (unter etwa 720 Pixeln) wird daraus ein einziger Stapel. Wähle eine Kategorie,
dann einen Rechner, und gehe mit **Zurück** eine Ebene nach oben.

## Einen Rechner finden

- **Kategorieleiste:** Zuerst kommen **Favoriten** und **Zuletzt verwendet**, danach die Kategorien mit Rechnern: Grundlagen, Passive
  Bauteile, Filter, Halbleiter, Stromversorgung, OpAmps, Timer, HF, Digital und Leitungen & PCB. Jede Kategorie zeigt, wie viele
  Rechner sie enthält. Die Leiste zeigt eine Kategorie nur, wenn sie Inhalte hat; Tabellen, Bauteilcodes und Pinbelegungen erscheinen
  also, sobald sie hinzukommen.
- **Suche:** Tippe oben ins Suchfeld, um einen Rechner über seinen Namen in deiner Sprache, über den englischen Namen oder über
  ein Stichwort wie „Teiler“ zu finden. Jeder Rechner hat Schlagwörter, die die Suche ebenfalls durchsucht.
- **Favoriten:** Die Favoriten-Schaltfläche auf einer Rechenseite nimmt den Rechner in die **Favoriten** auf. Ein weiterer Klick
  entfernt ihn wieder.
- **Zuletzt verwendet:** ECalc merkt sich die letzten 12 Rechner, die du geöffnet hast.

## Die Rechenseite

Eine Rechenseite besteht aus diesen Teilen:

- **Titelzeile:** der Name des Rechners und die Schaltflächen **Zu Favoriten hinzufügen**, **Ergebnisse kopieren** und **Auf
  Standardwerte zurücksetzen**.
- **Schaltplan:** die Schaltung, mit einem Eingabefeld neben jedem Bauteil, zu dem es gehört. Ein Rechner ohne Schaltplan zeigt
  seine Felder in einer einfachen Liste.
- **Formel:** die gesetzte Formel und darunter eine Live-Zeile mit deinen eingesetzten Werten, zum Beispiel
  *V_out = 12 V · 2,2 kΩ / (4,7 kΩ + 2,2 kΩ) = 3,83 V*.
- **Nächste Normwerte:** die E-Reihen-Vorschläge für die Widerstände (siehe [Normwerte (E-Reihen)](#normwerte-e-reihen)).
- **Kombinationen:** nur beim Rechner **Kombination von Normwerten**. Er zeigt den nächsten Einzelwert zum Ziel sowie bis zu fünf
  Paare in Reihe und bis zu fünf Paare parallel, jeweils mit Wert und Abweichung vom Ziel. Der Block trägt die gewählte Reihe als
  Überschrift, zum Beispiel „Kombinationen aus E24-Werten“.
- **Abgeleitete Werte:** weitere Größen, die sich aus dem Ergebnis ergeben, zum Beispiel der Strom durch einen Spannungsteiler
  und seine Leistung.
- **Diagramm:** eine Kurve unter der Formel: ein Wert über einem anderen, von 0 bis zu einem Endwert. Alle übrigen Werte bleiben
  auf den Werten der Seite. Der RC-Zeitkonstanten-Rechner zeichnet zum Beispiel die Kondensatorspannung über die Zeit von 0 bis 5 τ.
  Das Diagramm bleibt ausgeblendet, solange die Werte, die es braucht, unbekannt sind.
- **Warnungen:** ein Hinweis, wenn die Werte keinen Sinn ergeben, zum Beispiel eine Ausgangsspannung, die nicht unter der
  Eingangsspannung liegt.
- **Erklärung:** ein einklappbarer Text zum Rechner mit der **Quelle**: der Norm, dem Datenblatt oder dem Lehrbuch hinter der
  Formel.

**Ergebnisse kopieren** legt die Werte der Seite in die Zwischenablage. **Auf Standardwerte zurücksetzen** stellt die
Startwerte des Rechners wieder her.

**Abgeblendete Felder:** Manche Rechner bieten eine Option, etwa die Stromart eines Kabels (Gleichstrom, einphasiger oder
dreiphasiger Wechselstrom). Ein Feld, das die gewählte Option nicht verwendet, wird abgeblendet, und sein Hinweis lautet „Bei der
gewählten Option nicht verwendet“. Beim Rechner **Spannungsfall im Kabel** ist bei Gleichstrom zum Beispiel cos φ abgeblendet.
Abgeblendete Felder fehlen in **Ergebnisse kopieren** und in den Vorschlägen der E-Reihe.

## Werte eingeben

Jeder Rechner braucht eine feste Anzahl bekannter Werte. Das Ohmsche Gesetz zum Beispiel braucht zwei der vier Größen Spannung,
Strom, Widerstand und Leistung. Die Felder, die du ausfüllst, sind **Eingaben**. ECalc füllt die übrigen Felder mit
**berechneten** Werten und färbt sie ein.

- **Ändern, was bekannt ist:** Tippe in ein berechnetes Feld. Es wird zur Eingabe, und die Eingabe, die du am längsten nicht
  mehr angefasst hast, wird berechnet. So kannst du von dem Wert ausgehen, den du gerade zur Hand hast.
- **Wert sperren:** Das Schlosssymbol an einem Feld hält es fest. Ein gesperrtes Feld bleibt eine Eingabe, bis du es wieder
  entsperrst, auch wenn du andere Felder änderst.
- **Zahlenformate:** Du kannst `4k7` (4,7 kΩ), `2u2` (2,2 µ), `10 mA`, `1e-6` oder `0,5` tippen. Dezimalkomma und Dezimalpunkt
  funktionieren beide. Eine Einheit hinter der Zahl ist optional.
- **Zahlensysteme:** Die vier Felder des Rechners **Zahlensysteme** (dec, hex, bin, oct) zeigen dieselbe ganze Zahl, jedes in
  seiner Basis. Tippe die Zahl in eines der Felder, mit den Ziffern der Basis dieses Feldes. Das Präfix dieser Basis ist optional:
  `0x` für Hexadezimal (`0xFF`), `0b` für Binär (`0b1111_1111`) und `0o` für Oktal (`0o377`). Dezimal nimmt einfache Ziffern
  (`255`), und `_` oder ein Leerzeichen dürfen die Ziffern trennen. Lange Hex- und Binärergebnisse sind zur besseren Lesbarkeit
  gruppiert: Hex zu vier Stellen (`0xDEAD_BEEF`), Binär zu acht (`0b1_00000000`).
- **Einheiten:** Das Auswahlfeld neben einem Feld wählt die Einheit und bietet nur die Einheiten an, die zum Feld passen.
  **Automatisch** wählt ein passendes Präfix, etwa mA oder kΩ. Ein Querschnitt bietet mm², AWG und kcmil, eine Temperatur °C, °F
  und K, und die Kupferdicke einer Leiterbahn oz (Kupfergewicht). Manche Felder bieten besondere Einheiten, etwa dBm, dBV und dBu
  für Pegel oder mAh, Ah und Wh für Ladung und Energie.
- **Listen:** Der Rechner für Reihen- und Parallelschaltung nimmt eine Liste wie `4k7, 10k, 2k2`. Trenne die Einträge mit
  Kommas, Semikolons oder Leerzeichen. Ein Komma direkt vor einer Ziffer ist ein Dezimalkomma: `4,7k, 10k` sind zwei Einträge
  (4,7 kΩ und 10 kΩ), `1,2,3` dagegen ein Eintrag, der sich nicht lesen lässt. Setze nach einem Komma zwischen Einträgen ein
  Leerzeichen.
- **Anzeige:** Die Ergebnisse zeigen in Deutsch, Tschechisch, Dänisch, Griechisch, Spanisch, Französisch, Italienisch,
  Niederländisch, Norwegisch, Polnisch, Portugiesisch und Schwedisch ein Dezimalkomma, in den anderen Sprachen einen
  Dezimalpunkt. Listen zeigen immer einen Dezimalpunkt, weil das Komma ihre Einträge trennt.

Wenn etwas nicht stimmt, zeigt das Feld eine Meldung statt einer Zahl:

| Meldung | Bedeutung |
|---------|-----------|
| „Gib weitere Werte ein, um das zu berechnen.“ | Die bekannten Werte bestimmen dieses Feld noch nicht. |
| „Mit diesen Werten gibt es kein endliches Ergebnis.“ | Die Rechnung teilt durch null oder verlässt den Zahlenbereich. |
| „Dieser Wert muss größer als null sein.“ | Ein Widerstand zum Beispiel kann nicht null oder negativ sein. |
| „Dieser Wert ist nicht lesbar. Beispiele: 4k7, 10 mA, 1e-6.“ | Der Text ist keine Zahl mit optionalem Präfix und optionaler Einheit. Deine bisherigen Werte bleiben unverändert. |

## Normwerte (E-Reihen)

Widerstände gibt es in Normwerten. Jedes Widerstandsfeld eines Rechners zeigt **Nächster E24-Wert**: den nächstliegenden Normwert
und wie weit er vom berechneten Wert abweicht, in Prozent. 4,7 kΩ zum Beispiel gehört zur E24-Reihe, und ein berechneter Wert von
4,6 kΩ würde 4,7 kΩ mit einer Abweichung von etwa +2,2 % zeigen.

Eine andere Reihe (E6 bis E192) wählst du im Auswahlfeld **E-Reihe**. Voreingestellt ist E24. Reihen mit mehr Werten, etwa E96,
ergeben eine kleinere Abweichung. ECalc merkt sich deine Wahl.

**Kombination von Normwerten.** Der Rechner **Kombination von Normwerten** in den Grundlagen macht aus einem Zielwiderstand
Normwerte. Er nutzt dasselbe Auswahlfeld **E-Reihe**. Er zeigt den nächsten Einzelwert der gewählten Reihe sowie bis zu fünf Paare
in Reihe und bis zu fünf Paare parallel. Jedes Paar zeigt seinen Gesamtwert und seine Abweichung vom Ziel in Prozent. Beispiel: 5 kΩ
aus der E12-Reihe ergeben 4,7 kΩ + 270 Ω (−0,6 %) oder 10 kΩ parallel zu 10 kΩ (genau 5 kΩ).

## Einstellungen

Das Einstellungsmenü hat zwei Optionen:

- **Signifikante Stellen:** wie viele Stellen die Ergebnisse zeigen, von 3 bis 6. Voreingestellt sind 4.
- **Schaltsymbole:** **IEC** zeichnet den Widerstand als Rechteck (Voreinstellung). **ANSI** zeichnet den Zickzack-Widerstand.

ECalc speichert seine Einstellungen, deine Favoriten, die Liste der zuletzt verwendeten Rechner, die gewählten Einheiten und die
letzten Werte jedes Rechners im Browser unter dem Schlüssel `aurago.ecalc.v1`. Sie bleiben in diesem Browser. Auf dem
AuraGo-Server wird nichts gespeichert, und wer die Browserdaten löscht, setzt sie zurück. ECalc funktioniert auch ohne
Browser-Speicher, vergisst dann aber alles, sobald du das Fenster schließt.

## Die Rechner

ECalc enthält 77 Rechner in zehn Kategorien. Jede Tabelle listet die Rechner einer Kategorie, in der Reihenfolge der
Kategorieleiste. Die Quelle hinter jeder Formel, also die Norm, das Datenblatt oder das Lehrbuch, zeigt die Seite des Rechners
unter **Erklärung**; deshalb steht sie nicht in den Tabellen.

### Grundlagen

| Rechner | Was er macht |
|---------|--------------|
| Ohmsches Gesetz und Leistung | Spannung, Strom, Widerstand und Leistung. Gib zwei beliebige Größen ein und erhalte die beiden anderen. |
| Spannungsteiler | Die Ausgangsspannung eines Teilers aus zwei Widerständen. Er findet auch einen Widerstand oder die Eingangsspannung für eine gewünschte Ausgangsspannung und zeigt Strom und Leistung. |
| Belasteter Spannungsteiler | Derselbe Teiler mit einem Lastwiderstand am Ausgang. Er zeigt zum Vergleich die unbelastete Ausgangsspannung und warnt, wenn die Last das Ziel unmöglich macht. |
| Stromteiler | Wie sich ein Strom auf zwei parallele Widerstände verteilt. |
| Reihen- und Parallelschaltung | Der Gesamtwiderstand einer Liste von Widerständen, in Reihe und parallel. |
| Dezibel-Verhältnis | Rechnet zwischen einem Verhältnis und Dezibel um. Wähle Spannung (20 log) oder Leistung (10 log). |
| Pegel in dB | Spannung, Widerstand und Leistung mit den Pegeln in dBm, dBV und dBu. |
| Wheatstone-Brücke | Die Brückenspannung aus der Versorgung und vier Widerständen, oder der Widerstand, der die Brücke abgleicht. |
| Stern-Dreieck-Umwandlung | Wandelt drei Dreieck-Widerstände in den gleichwertigen Stern um und umgekehrt. |
| Effektiv-, Spitzen- und Mittelwert | Rechnet zwischen Effektivwert, Spitzenwert, Spitze-Spitze-Wert und Mittelwert einer Sinus-, Rechteck- oder Dreieckschwingung um. |
| Ladung und Energie | Rechnet zwischen Ladung (zum Beispiel den mAh eines Akkus), Spannung und Energie (Wh oder J) um. |
| Kombination von Normwerten | Der nächste Normwert zu einem Zielwiderstand sowie Paare in Reihe oder parallel, die am nächsten kommen. |

### Passive Bauteile

| Rechner | Was er macht |
|---------|--------------|
| Kapazitiver Blindwiderstand | Blindwiderstand eines Kondensators. Je zwei der Größen Frequenz, Kapazität und Blindwiderstand ergeben die dritte. |
| Induktiver Blindwiderstand | Blindwiderstand einer Spule. Je zwei der Größen Frequenz, Induktivität und Blindwiderstand ergeben die dritte. |
| Impedanz der Reihenschaltung (RC, RL, RLC) | Resultierender Blindwiderstand, Impedanz und Phasenwinkel eines Widerstands in Reihe mit Spule, Kondensator oder beidem. |
| Impedanz der Parallelschaltung (RC, RL, RLC) | Impedanzbetrag, Phasenwinkel und äquivalenter Parallelblindwiderstand eines Widerstands parallel zu Spule, Kondensator oder beidem. |
| RC-Zeitkonstante | Auf- und Entladen eines Kondensators über einen Widerstand, mit der Zeitkonstante und dem Spannungsverlauf. |
| Reihen- und Parallelschaltung von Kondensatoren | Gesamtkapazität einer Liste von Kondensatoren, in Reihe und parallel. |
| Reihen- und Parallelschaltung von Spulen | Gesamtinduktivität einer Liste von Spulen, in Reihe und parallel, ohne magnetische Kopplung. |
| Kondensatorenergie und -ladung | Energie, Ladung, Spannung und Kapazität eines Kondensators. Je zwei davon ergeben die beiden anderen. |
| Luftspule (Wheeler) | Induktivität einer einlagigen Luftspule aus Durchmesser, Länge und Windungszahl, oder eine dieser Größen für eine gewünschte Induktivität. |
| Ringkerninduktivität (A_L) | Induktivität eines Ringkerns aus seinem A_L-Wert und der Windungszahl, oder eine der drei Größen. |
| Leitungswiderstand | Gleichstromwiderstand eines Kupfer-, Aluminium- oder Silberleiters, mit Temperaturkorrektur. Löst auch nach Länge, Querschnitt oder Temperatur auf. |

### Filter

| Rechner | Was er macht |
|---------|--------------|
| RC-Tiefpass | Grenzfrequenz und Verhalten eines Tiefpasses erster Ordnung bei einer Testfrequenz. |
| RC-Hochpass | Grenzfrequenz und Verhalten eines Hochpasses erster Ordnung bei einer Testfrequenz. |
| LC-Schwingkreis | Resonanzfrequenz eines LC-Kreises und sein Kennwiderstand. |
| RLC-Kreis: Güte und Bandbreite | Güte und −3-dB-Bandbreite eines Reihen- oder Parallel-RLC-Kreises. |
| Sallen-Key-Tiefpass (Verstärkung 1) | Aktiver Tiefpass zweiter Ordnung mit gleichen Widerständen und den Kondensatorwerten für einen Butterworth-Verlauf. |
| Doppel-T-Sperrfilter | Sperrfrequenz des passiven Doppel-T-Netzwerks. |
| Pi-Dämpfungsglied | Widerstände eines angepassten π-Glieds für eine Systemimpedanz und eine Dämpfung in dB. |
| T-Dämpfungsglied | Widerstände eines angepassten T-Glieds für eine Systemimpedanz und eine Dämpfung in dB. |

### Halbleiter

| Rechner | Was er macht |
|---------|--------------|
| LED-Vorwiderstand | Vorwiderstand eines LED-Strangs aus Versorgungsspannung, Flussspannung und Strom. Auch für parallele Stränge. |
| Z-Dioden-Stabilisierung | Vorwiderstand eines Z-Dioden-Parallelreglers für einen Laststrom, mit Z-Dioden-Leistung und Wirkungsgrad. |
| Transistor als Schalter | Basiswiderstand eines NPN-Transistors als gesättigter Schalter, mit dem Basisstrom. |
| Transistor-Arbeitspunkt mit Basisspannungsteiler | Gleichstrom-Arbeitspunkt einer Emitterschaltung mit Vier-Widerstands-Basisspannungsteiler. |
| MOSFET-Verluste | Durchlass- und Schaltverlust eines hart schaltenden MOSFETs und ihre Summe. |
| Kühlkörper | Wärmekette von der Sperrschicht bis zur Umgebung. Findet den fehlenden von sechs Werten, zum Beispiel den nötigen Kühlkörper. |

### Stromversorgung

| Rechner | Was er macht |
|---------|--------------|
| LM317 Ausgangsspannung | Ausgangsspannung eines LM317-Reglers aus seinen zwei Widerständen, oder der Widerstand für eine gewünschte Spannung. |
| LM317 Konstantstromquelle | Laststrom einer LM317-Stromquelle, oder der Widerstand für einen gewünschten Strom. |
| Verlustleistung Linearregler | Wärme in einem Linearregler, mit Eingangs- und Ausgangsleistung und Wirkungsgrad. |
| Tiefsetzsteller (Buck) | Idealer Tiefsetzsteller: Tastverhältnis, Rippel- und Spitzenströme, oder Induktivität bzw. Frequenz für einen gewünschten Rippel. |
| Hochsetzsteller (Boost) | Idealer Hochsetzsteller: Tastverhältnis, Eingangsstrom, Rippel und Spitzenströme, oder Induktivität bzw. Frequenz für einen gewünschten Rippel. |
| Glättungskondensator (Gleichrichter) | Kondensator hinter einem Einweg- oder Zweiweg-Gleichrichter für einen gewünschten Rippel. |
| Akku-Laufzeit | Laufzeit eines Akkus aus Kapazität und Laststrom. |
| Akku-Ladezeit | Ladezeit aus Kapazität, Ladestrom und Ladewirkungsgrad. |
| Akkupack (Reihe/parallel) | Spannung, Kapazität und Energie eines Packs aus in Reihe und parallel geschalteten Zellen, zum Beispiel 3S2P. |
| Spannungsfall im Kabel | Spannungsfall eines Kupfer- oder Aluminiumkabels bei Gleich- und Wechselstrom, mit Richtwerten für Netzinstallationen. |

### OpAmps

| Rechner | Was er macht |
|---------|--------------|
| Invertierender Verstärker | Verstärkung und Ausgang einer idealen invertierenden Operationsverstärker-Stufe. |
| Nichtinvertierender Verstärker | Verstärkung und Ausgang einer idealen nichtinvertierenden Stufe. |
| Differenzverstärker | Ausgang eines Differenzverstärkers mit gleichen Widerstandspaaren, aus seinen Eingangsspannungen. |
| Summierverstärker | Ausgang eines invertierenden Summierverstärkers für eine Liste von Eingangsspannungen. |
| Integrierer | Ausgang eines idealen invertierenden Integrierers über die Zeit, mit seiner Zeitkonstante. |
| Schmitt-Trigger | Schwellen, Hysterese und Mitte eines invertierenden Schmitt-Triggers, dazu die Widerstände für gewünschte Schwellen. |
| Instrumentenverstärker | Verstärkung und Ausgang eines Instrumentenverstärkers in der Drei-Operationsverstärker-Schaltung. |

### Timer

| Rechner | Was er macht |
|---------|--------------|
| NE555 astabil (Oszillator) | Frequenz, Highzeit, Lowzeit und Tastverhältnis des NE555-Oszillators. Bestimmt auch beide Widerstände aus Frequenz, Tastverhältnis und Kondensator. |
| NE555 monostabil (Monoflop) | Impulsbreite des NE555-Monoflops. Löst nach Widerstand, Kondensator oder Breite auf. |
| Quarz-Lastkondensatoren | Die beiden Lastkondensatoren eines Quarzes in einer Pierce-Schaltung, mit dem nächsten Normwert. |
| Frequenz, Periodendauer und Tastverhältnis | Rechnet zwischen Frequenz, Periodendauer, Tastverhältnis, Highzeit und Lowzeit eines Rechtecksignals um. |
| PWM-Mittelwert | Mittel- und Effektivwert eines PWM-Signals aus Tastverhältnis und den beiden Pegeln. |

### HF

| Rechner | Was er macht |
|---------|--------------|
| Wellenlänge | Rechnet zwischen Frequenz, Verkürzungsfaktor und Wellenlänge um, mit halber und viertel Wellenlänge. |
| Länge des Halbwellendipols | Länge eines Halbwellendipols mit Verkürzungsfaktor, dazu die Viertelwellenlänge. |
| Mikrostrip-Impedanz | Wellenwiderstand einer Mikrostrip-Leitung auf der Leiterplatte aus ihrer Geometrie und ε_r. |
| Wellenwiderstand eines Koaxialkabels | Wellenwiderstand aus den Durchmessern und ε_r, dazu der Verkürzungsfaktor. |
| SWR und Rückflussdämpfung | Rechnet zwischen SWR, Reflexionsfaktor und Rückflussdämpfung um, mit der reflektierten Leistung. |
| HF-Leistung und -Spannung (dBm, dBµV) | Leistung, Spannung und Widerstand einer Sinusschwingung, mit den Pegeln in dBm und dBµV. |
| Freiraumdämpfung | Dämpfung einer Funkstrecke aus Abstand und Frequenz, oder Abstand bzw. Frequenz für eine vorgegebene Dämpfung. |
| Skin-Tiefe (Eindringtiefe) | Skin-Tiefe von Kupfer, Silber, Gold oder Aluminium bei einer Frequenz, mit dem Oberflächenwiderstand. |

### Digital

| Rechner | Was er macht |
|---------|--------------|
| ADC (Spannung → Code) | Ausgangscode eines idealen N-Bit-Wandlers, dezimal, hexadezimal und binär. |
| DAC (Code → Spannung) | Ausgangsspannung eines idealen DAC für einen Code, oder der nächste Code für eine gewünschte Spannung. |
| I²C-Pull-up-Widerstände | Kleinster und größter Pull-up-Widerstand für einen Bus, mit den nächsten Normwerten. |
| Pegelwandler (Spannungsteiler) | Teiler von 5 V auf 3,3 V für eine Übertragungsrichtung, geprüft gegen die High-Schwelle des Empfängers. |
| UART-Baudratenfehler | Teiler und Baudratenfehler aus Takt und gewünschter Baudrate, bei 16- oder 8-facher Abtastung. |
| Zahlensysteme (dec, hex, bin, oct) | Eine ganze Zahl, dezimal, hexadezimal, binär und oktal angezeigt. |

### Leitungen & PCB

| Rechner | Was er macht |
|---------|--------------|
| Leiterbahnbreite (IPC-2221) | Breite, Strom oder Temperaturanstieg einer Kupfer-Leiterbahn auf Außen- oder Innenlage. |
| Leiterbahnwiderstand | Widerstand, Spannungsfall und Verlustleistung einer Kupfer-Leiterbahn. Löst auch nach Länge, Breite oder Dicke auf. |
| Strombelastbarkeit einer Durchkontaktierung | Kupferquerschnitt, Widerstand und Strom einer durchkontaktierten Via. |
| AWG-Umrechnung (Drahtdurchmesser und Querschnitt) | Rechnet Drahtdurchmesser und Querschnitt zwischen AWG, mm, mil, Zoll, kcmil und mil² um. |

## ECalc mit dem Agenten nutzen

Der AuraGo-Agent kann die Formeln von ECalc über das Tool `ecalc` nutzen. Das Tool rechnet auf deinem AuraGo-Server mit demselben
Katalog und denselben Formeln wie die App. Es liest weder aus dem Netzwerk noch aus Dateien, und es hat keinen eigenen Schalter unter
**Config**.

Der Agent hat vier Operationen:

- **search** findet Rechner zu einem Thema, einem Bauteil oder einer Formel, auf Deutsch oder Englisch. Ohne Suchbegriff listet es
  die Kategorien.
- **describe** zeigt die Variablen eines Rechners mit Einheiten und Vorgaben, seine Auswahlen, die Eingabesätze, die funktionieren,
  die berechneten Größen und die Prüfungen.
- **calculate** löst einen Rechner aus den Werten, die du angibst. Jeder vollständige Eingabesatz genügt, daher kann es jede fehlende
  Größe berechnen. Es nennt jeden Wert in SI-Einheiten und als formatierten Text, dazu die Warnungen des Rechners. Es meldet auch,
  wenn ein Wert, den du angegeben hast, um mehr als 1 % vom Ergebnis abweicht. Bei einem berechneten Widerstand nennt es den nächsten
  Normwert der gewählten Reihe, und der Kombinationsrechner liefert seine Paare.
- **open** öffnet ECalc auf dem Desktop bei einem Rechner, mit den Werten, als hättest du sie eingetippt. Die Antwort rechnet nichts aus;
  die Zahlen liefert **calculate**. Namen und Werte werden einzeln geprüft, die Werte aber nicht gegeneinander. Ein widersprüchlicher
  Satz von Werten öffnet sich daher so, wie er eingetragen ist.

Für **open** muss der Virtual Desktop eingeschaltet und **Agentensteuerung erlauben** unter **Config → Integrationen → Virtual
Desktop** aktiv sein. Fehlt eines davon, bekommt der Agent die Antwort „policy denied“ und nennt dir stattdessen die Zahlen aus
**calculate**. Andere Agenten, die über A2A (Agent-zu-Agent-Anfragen) mit AuraGo sprechen, dürfen suchen, beschreiben und rechnen,
aber ECalc nicht öffnen.

Zwei Beispiele, die du eintippen kannst:

- „Welchen Vorwiderstand brauche ich für eine rote LED bei 20 mA an 5 V?“ Der Agent rechnet den Vorwiderstand mit dem Rechner
  **LED-Vorwiderstand** und nennt dir den nächsten Normwert.
- „Öffne ECalc mit dem NE555 astabil für 1 kHz.“ Der Agent öffnet den Rechner **NE555 astabil (Oszillator)** im Virtual Desktop, mit
  eingetragener Frequenz, sofern Virtual Desktop und Agentensteuerung eingeschaltet sind.

## Wie es weitergeht

Der Katalog wächst in späteren Versionen:

- **Nachschlagetabellen**, zum Beispiel die Strombelastbarkeit von Kabeln und Leiterbahnen.
- **Bauteilcodes**, zum Beispiel der Widerstandsfarbcode und SMD-Codes.
- **Pinbelegungen** von Steckverbindern und ICs.
- **Auswertungen aus Tabellen**, zum Beispiel ein Kabelquerschnitt für einen Strom nach VDE und ein Leiterbahnabstand für eine
  Spannung.

Dieses Kapitel wächst mit.

## Datenschutz und Sicherheit

- **Auf deinem Gerät.** Die Berechnungen in der App laufen in deinem Browser. ECalc lädt seinen Katalog von deinem AuraGo-Server und
  schickt keinen deiner Werte irgendwohin. Es lädt keine externen Schriften, Bibliotheken oder Dienste.
- **Agent-Tool.** Das Tool `ecalc` rechnet auf dem AuraGo-Server mit dem Katalog, der in AuraGo eingebaut ist. Es liest weder aus dem
  Netzwerk noch aus Dateien. **open** funktioniert nur, solange der Virtual Desktop eingeschaltet ist und die Agentensteuerung erlaubt
  ist.
- **Schreibgeschützt.** ECalc liest nur. Ein schreibgeschützter Virtual Desktop ändert nichts an seiner Funktion.
- **Prüfe, worauf es ankommt.** Jeder Rechner nennt seine Quelle unter **Erklärung**. Die Ergebnisse sind nur so gut wie die
  Formel und deine Eingaben. Prüfe einen wichtigen Entwurf anhand der Datenblätter deiner Bauteile. Netzspannung ist
  gefährlich, und ECalc ersetzt keine Arbeit durch eine Elektrofachkraft.
- **Netzinstallationen.** Der Rechner **Spannungsfall im Kabel** liefert Richtwerte für Netzinstallationen. Seine Seite hält fest, dass
  die Auslegung einer Netzinstallation der Installationsnorm (zum Beispiel DIN VDE 0100-520 oder IEC 60364-5-52) und den örtlichen
  Vorschriften folgen muss und von einer Elektrofachkraft ausgeführt und geprüft werden muss.

## Wenn etwas schiefgeht

| Problem | Was hilft |
|---------|-----------|
| „Der ECalc-Katalog konnte nicht geladen werden.“ | Klicke auf **Erneut versuchen**. Falls es weiter scheitert, prüfe, ob AuraGo läuft und du noch angemeldet bist, und lade den Virtual Desktop neu. |
| Ein Feld zeigt eine Meldung statt eines Werts | Siehe die Tabelle unter [Werte eingeben](#werte-eingeben). |
| Ein Zahlensystem-Feld zeigt einen Fehler | Die Ziffern müssen zur Basis passen: 0 und 1 für Binär, 0 bis 7 für Oktal, 0 bis 9 und A bis F für Hex. Der Wert muss zwischen 0 und 2^53 − 1 (9 007 199 254 740 991) bleiben. Das Präfix der Basis des Feldes (`0x`, `0b` oder `0o`) ist optional. |
| Ein Feld ist abgeblendet | Die gewählte Option nutzt es nicht. Beim Spannungsfall im Kabel ist bei Gleichstrom cos φ abgeblendet. Wähle eine Option, die das Feld nutzt, wenn du es brauchst. |
| Der Agent kann ECalc nicht öffnen („policy denied“) | Schalte den Virtual Desktop ein und aktiviere **Agentensteuerung erlauben** unter **Config → Integrationen → Virtual Desktop**. Bis dahin kann der Agent dir die Zahlen aus **calculate** trotzdem nennen. |
| Favoriten oder letzte Werte sind weg | Sie liegen im Browser. Ein anderer Browser, ein privates Fenster oder gelöschte Browserdaten beginnen mit leeren. |
| Der E-Reihen-Wert weicht stark vom berechneten ab | Wähle eine Reihe mit mehr Werten, etwa E96 oder E192. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 8: Integrationen](./08-integrations.md) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md) · [Kapitel 22: Interne Tools](./22-internal-tools.md)

---

**Vorheriges Kapitel:** [Kapitel 31: Pixel](./31-pixel.md)

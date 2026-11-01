# Kapitel 32: ECalc – Elektronik-Formeln im Virtual Desktop

ECalc ist eine Formelsammlung für Elektronik im Virtual Desktop. Eine Rechenseite zeigt die Schaltung, die Formel und die
Ergebnisse nebeneinander und rechnet schon während du tippst. Du gibst die Werte ein, die du kennst, und ECalc berechnet den
Rest. Bei Widerständen schlägt es außerdem den nächsten Normwert aus einer E-Reihe vor.

Das ist die erste Version. Sie enthält 11 Rechner in der Kategorie **Grundlagen**. Tabellen, Bauteilcodes, Pinbelegungen und ein
Agent-Tool folgen in späteren Phasen (siehe [Wie es weitergeht](#wie-es-weitergeht)). ECalc läuft in deinem Browser. Es braucht
keinen Provider und hat keinen Schalter unter **Konfiguration**.

## Inhalt
1. [ECalc öffnen](#ecalc-öffnen)
2. [Einen Rechner finden](#einen-rechner-finden)
3. [Die Rechenseite](#die-rechenseite)
4. [Werte eingeben](#werte-eingeben)
5. [Normwerte (E-Reihen)](#normwerte-e-reihen)
6. [Einstellungen](#einstellungen)
7. [Die Rechner](#die-rechner)
8. [Wie es weitergeht](#wie-es-weitergeht)
9. [Datenschutz und Sicherheit](#datenschutz-und-sicherheit)
10. [Wenn etwas schiefgeht](#wenn-etwas-schiefgeht)

---

## ECalc öffnen

Öffne **ECalc** aus dem Startmenü, in der Kategorie **Entwicklung**. Das Fenster öffnet sich mit etwa 1100 × 760 Pixeln und lässt
sich in der Größe ändern. ECalc öffnet wieder den Rechner, den du zuletzt benutzt hast.

Das Fenster hat drei Spalten: links die **Kategorieleiste**, in der Mitte die **Liste** der Rechner und rechts die
**Rechenseite**. In einem schmalen Fenster (unter etwa 720 Pixeln) wird daraus ein einziger Stapel. Wähle eine Kategorie,
dann einen Rechner, und gehe mit **Zurück** eine Ebene nach oben.

## Einen Rechner finden

- **Kategorieleiste:** Zuerst kommen **Favoriten** und **Zuletzt verwendet**, danach die Rechner-Kategorien: Grundlagen, Passive
  Bauteile, Filter, Halbleiter, Stromversorgung, OpAmps, Timer, HF, Digital und Leitungen & PCB. In dieser Version enthält nur
  **Grundlagen** Rechner.
- **Suche:** Tippe oben ins Suchfeld, um einen Rechner über seinen Namen in deiner Sprache, über den englischen Namen oder über
  ein Stichwort wie „Teiler“ zu finden.
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
- **Abgeleitete Werte:** weitere Größen, die sich aus dem Ergebnis ergeben, zum Beispiel der Strom durch einen Spannungsteiler
  und seine Leistung.
- **Warnungen:** ein Hinweis, wenn die Werte keinen Sinn ergeben, zum Beispiel eine Ausgangsspannung, die nicht unter der
  Eingangsspannung liegt.
- **Erklärung:** ein einklappbarer Text zum Rechner mit der **Quelle**: der Norm, dem Datenblatt oder dem Lehrbuch hinter der
  Formel.

**Ergebnisse kopieren** legt die Werte der Seite in die Zwischenablage. **Auf Standardwerte zurücksetzen** stellt die
Startwerte des Rechners wieder her.

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
- **Einheiten:** Das Auswahlfeld neben einem Feld wählt die Einheit. **Automatisch** wählt ein passendes Präfix, etwa mA oder kΩ.
  Manche Felder bieten besondere Einheiten, etwa dBm, dBV und dBu für Pegel oder mAh, Ah und Wh für Ladung und Energie.
- **Listen:** Der Rechner für Reihen- und Parallelschaltung nimmt eine Liste wie `4k7, 10k, 2k2`. Trenne die Einträge mit
  Kommas, Semikolons oder Leerzeichen. In Listen gilt nur der Dezimalpunkt.
- **Anzeige:** Die Ergebnisse zeigen in Deutsch, Tschechisch, Dänisch, Griechisch, Spanisch, Französisch, Italienisch,
  Niederländisch, Norwegisch, Polnisch, Portugiesisch und Schwedisch ein Dezimalkomma, in den anderen Sprachen einen
  Dezimalpunkt.

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

## Einstellungen

Das Einstellungsmenü hat zwei Optionen:

- **Signifikante Stellen:** wie viele Stellen die Ergebnisse zeigen, von 3 bis 6. Voreingestellt sind 4.
- **Schaltsymbole:** **IEC** zeichnet den Widerstand als Rechteck (Voreinstellung). **ANSI** zeichnet den Zickzack-Widerstand.

ECalc speichert seine Einstellungen, deine Favoriten, die Liste der zuletzt verwendeten Rechner, die gewählten Einheiten und die
letzten Werte jedes Rechners im Browser unter dem Schlüssel `aurago.ecalc.v1`. Sie bleiben in diesem Browser. Auf dem
AuraGo-Server wird nichts gespeichert, und wer die Browserdaten löscht, setzt sie zurück. ECalc funktioniert auch ohne
Browser-Speicher, vergisst dann aber alles, sobald du das Fenster schließt.

## Die Rechner

Alle Rechner dieser Version gehören zur Kategorie **Grundlagen**.

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

## Wie es weitergeht

Der Katalog wächst in späteren Phasen:

- **Weitere Rechner** in den Kategorien Passive Bauteile, Filter, Halbleiter, Stromversorgung, OpAmps, Timer, HF, Digital und
  Leitungen & PCB.
- **Nachschlagetabellen**, zum Beispiel die Strombelastbarkeit von Kabeln und Leiterbahnen.
- **Bauteilcodes**, zum Beispiel der Widerstandsfarbcode und SMD-Codes.
- **Pinbelegungen** von Steckverbindern und ICs.
- **Das Agent-Tool `ecalc`.** Der AuraGo-Agent soll dieselben Formeln zum Rechnen nutzen und ECalc auf einer Seite mit bereits
  eingetragenen Werten öffnen können.

Dieses Kapitel wächst mit.

## Datenschutz und Sicherheit

- **Auf deinem Gerät.** Die Berechnungen laufen in deinem Browser. ECalc lädt seinen Katalog von deinem AuraGo-Server und
  schickt keinen deiner Werte irgendwohin. Es lädt keine externen Schriften, Bibliotheken oder Dienste.
- **Schreibgeschützt.** ECalc liest nur. Ein schreibgeschützter Virtual Desktop ändert nichts an seiner Funktion.
- **Prüfe, worauf es ankommt.** Jeder Rechner nennt seine Quelle unter **Erklärung**. Die Ergebnisse sind nur so gut wie die
  Formel und deine Eingaben. Prüfe einen wichtigen Entwurf anhand der Datenblätter deiner Bauteile. Netzspannung ist
  gefährlich, und ECalc ersetzt keine Arbeit durch eine Elektrofachkraft.

## Wenn etwas schiefgeht

| Problem | Was hilft |
|---------|-----------|
| „Der ECalc-Katalog konnte nicht geladen werden.“ | Klicke auf **Erneut versuchen**. Falls es weiter scheitert, prüfe, ob AuraGo läuft und du noch angemeldet bist, und lade den Virtual Desktop neu. |
| Ein Feld zeigt eine Meldung statt eines Werts | Siehe die Tabelle unter [Werte eingeben](#werte-eingeben). |
| Favoriten oder letzte Werte sind weg | Sie liegen im Browser. Ein anderer Browser, ein privates Fenster oder gelöschte Browserdaten beginnen mit leeren. |
| Der E-Reihen-Wert weicht stark vom berechneten ab | Wähle eine Reihe mit mehr Werten, etwa E96 oder E192. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 31: Pixel](./31-pixel.md)

# Kapitel 33: Aura Drift – Rennen im Virtual Desktop

Aura Drift ist ein Top-Down-Autorennen direkt auf dem Virtuellen Desktop. Es läuft komplett
im Browser: kein Konto, kein Provider, keine Netzwerkverbindung. Du fährst gegen drei
KI-Gegner, jagst deine Bestzeit oder fährst einen Cup über sechs sehr unterschiedliche
Strecken.

## Öffnen und starten

Starte **Aura Drift** über das Startmenü in der Kategorie **Spiele**. Im Menü wählst du
Modus, Strecke und Rundenzahl; beim Wechsel der Strecke wird automatisch die empfohlene
Rundenzahl (zwei Runden, etwa drei Minuten) gesetzt. **Schnellrennen** ist der schnelle
Einstieg, **Zeitfahren** fährt allein mit Geist-Fahrzeug, **Cup** verbindet alle sechs
Strecken, **Tagesroute** erzeugt aus dem Datum eine neue, geprüfte Strecke.

## Steuerung

| Eingabe | Wirkung |
| --- | --- |
| Pfeiltasten / WASD | Lenken, Gas, Bremse |
| Leertaste | Waffe abfeuern |
| Shift | Handbremse – kontrolliertes Übersteuern |
| Strg / Alt | Nitro |
| P / Escape | Pause |
| R | Rennen neu starten |

Auf Touch-Geräten erscheinen Lenk-, Gas-, Brems-, Waffen- und Handbrems-Tasten. Verliert
das Fenster den Fokus, pausiert das Rennen automatisch und alle gehaltenen Tasten werden
losgelassen.

## Fahre mit Grip, nicht gegen ihn

Das Auto folgt einem echten Reifenmodell: Vorder- und Hinterachse haben eigene
Schlupfwinkel, beim Bremsen wandert die Last nach vorn, die Handbremse nimmt der
Hinterachse Grip. Auf nassem Asphalt, Schotter, Matsch, Sand, Eis oder Öl ändert sich die
Haftung spürbar. Nimm vor der Kurve das Gas raus, lenke ein und gib am Scheitelpunkt
wieder Gas. Ein zu früher Lenkruck rächt sich mit Untersteuern – besonders auf Eis.

## Waffen

Unterwegs sammelst du Pickups ein. Ölspur, Mine, EMP, Suchrakete, MG-Salve und Schild
haben jeweils eine Ankündigung, eine Abklingzeit und eine Gegenmaßnahme. Gegner nutzen
dieselben Regeln. Im Menü kannst du Waffen komplett abschalten; beim Zeitfahren sind sie
immer aus.

## Strecken

| Strecke | Charakter |
| --- | --- |
| Alpenpass | Serpentinen, Nebel, Schotter-Auslauf |
| Wüsten-Canyon | Lange Geraden, Hitze, Sand |
| Neon-Hafen | Nacht, Regen, spiegelnder Asphalt |
| Salzsee | Weite Flächen, sehr viel Grip |
| Wald-Rallye | Schmal, Matsch, Dämmerung |
| Industrie-Ring | Oval mit Fabrik-Abkürzungen |

Die Welt wird in Kacheln um die Kamera herum erzeugt; es gibt keine riesige Bilddatei.
Vier Grafikstufen (und eine automatische Wahl) passen Partikel, Wetter, Licht und Cache an.
In den Stufen **Hoch** und **Ultra** legt sich ein WebGL2-Nachbearbeitungs-Pass über das
Bild: Bloom um Scheinwerfer, senkrechte Reflexe auf nassem Asphalt im Neon-Hafen,
Farbgrading pro Strecke und ein kurzer Farbschock bei Treffern. Fehlt WebGL2 oder bricht
der Kontext weg, läuft das Spiel ohne diesen Pass weiter.

Gegner fahren mit demselben Reifenmodell wie du. Wer deutlich zurückliegt, erhält höchstens
6 % mehr Zieltempo, aber nie mehr Grip — das steht auch im Menü. Nach einem Dreher setzt
sich die KI kurz zurück; bleibt sie hängen, holt sie der Streckenposten auf die Ideallinie.

## Einstellungen und Bestzeiten

Lautstärke, Stumm, Grafikstufe, Bewegungsreduktion, Geist-Anzeige und Waffen-Schalter
liegen im Spielmenü. Bestzeiten und Einstellungen bleiben lokal in diesem Browser. Bei
einem schreibgeschützten Desktop wird nichts gespeichert.

## Falls das Fenster leer bleibt

Die erste Version benutzte Klassennamen wie `ad-overlay` und `ad-panel`. Werbeblocker
(AdGuard, uBlock, EasyList) halten solche `ad-`-Klassen für Werbung und blenden sie
mit `display: none` aus — das Spiel lief, man sah es nur nicht, und die Console
blieb leer. Die aktuelle Version verwendet `agdrift-` und ist damit nicht mehr
betroffen. Hilft das nicht, nimm die Desktop-Adresse in deinem Werbeblocker in die
Ausnahmeliste auf und lade die Seite neu.

## Grenzen

Aura Drift ist ein Spiel mit glaubwürdiger, deterministischer Fahrphysik – keine
Fahrzeugsimulation für Ingenieure. Die Welt wird mit Canvas 2D gezeichnet, WebGL2
übernimmt nur die Nachbearbeitung in den hohen Stufen. Es gibt kein Mehrspieler und keine
Online-Rangliste. Details stehen im [technischen Handbuch](../../aura-drift.md).

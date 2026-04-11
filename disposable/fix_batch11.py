#!/usr/bin/env python3
"""Batch 11: More short UI terms, actions, and common labels."""
import json, os
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "Pause/Resume": {"cs":"Pozastavit/Pokračovat","da":"Pause/Genoptag","de":"Pause/Fortsetzen","el":"Παύση/Συνέχεια","es":"Pausar/Reanudar","fr":"Pause/Reprendre","hi":"रोकें/फिर से शुरू करें","it":"Pausa/Riprendi","ja":"一時停止/再開","nl":"Pauzeren/Hervatten","no":"Pause/Fortsett","pl":"Wstrzymaj/Wznów","pt":"Pausar/Retomar","sv":"Pausa/Återuppta","zh":"暂停/恢复"},
    "Paused": {"cs":"Pozastaveno","da":"På pause","de":"Pausiert","el":"Σε παύση","es":"En pausa","fr":"En pause","hi":"रुका हुआ","it":"In pausa","ja":"一時停止中","nl":"Gepauzeerd","no":"Satt på pause","pl":"Wstrzymane","pt":"Pausado","sv":"Pausad","zh":"已暂停"},
    "Preview": {"cs":"Náhled","da":"Forhåndsvisning","de":"Vorschau","el":"Προεπισκόπηση","es":"Vista previa","fr":"Aperçu","hi":"पूर्वावलोकन","it":"Anteprima","ja":"プレビュー","nl":"Voorbeeld","no":"Forhåndsvisning","pl":"Podgląd","pt":"Visualização","sv":"Förhandsgranskning","zh":"预览"},
    "Priority": {"cs":"Priorita","da":"Prioritet","de":"Priorität","el":"Προτεραιότητα","es":"Prioridad","fr":"Priorité","hi":"प्राथमिकता","it":"Priorità","ja":"優先度","nl":"Prioriteit","no":"Prioritet","pl":"Priorytet","pt":"Prioridade","sv":"Prioritet","zh":"优先级"},
    "Private": {"cs":"Soukromé","da":"Privat","de":"Privat","el":"Ιδιωτικό","es":"Privado","fr":"Privé","hi":"निजी","it":"Privato","ja":"プライベート","nl":"Privé","no":"Privat","pl":"Prywatny","pt":"Privado","sv":"Privat","zh":"私有"},
    "Public": {"cs":"Veřejné","da":"Offentlig","de":"Öffentlich","el":"Δημόσιο","es":"Público","fr":"Public","hi":"सार्वजनिक","it":"Pubblico","ja":"パブリック","nl":"Openbaar","no":"Offentlig","pl":"Publiczny","pt":"Público","sv":"Offentlig","zh":"公开"},
    "Queue": {"cs":"Fronta","da":"Kø","de":"Warteschlange","el":"Ουρά","es":"Cola","fr":"File d'attente","hi":"कतार","it":"Coda","ja":"キュー","nl":"Wachtrij","no":"Kø","pl":"Kolejka","pt":"Fila","sv":"Kö","zh":"队列"},
    "Rollback": {"cs":"Návrat","da":"Tilbageføring","de":"Rollback","el":"Επαναφορά","es":"Reversión","fr":"Restauration","hi":"रोलबैक","it":"Ripristino","ja":"ロールバック","nl":"Terugdraaien","no":"Tilbakerulling","pl":"Wycofanie","pt":"Reversão","sv":"Återställning","zh":"回滚"},
    "Running": {"cs":"Spuštěno","da":"Kører","de":"Läuft","el":"Εκτελείται","es":"En ejecución","fr":"En cours","hi":"चल रहा है","it":"In esecuzione","ja":"実行中","nl":"Actief","no":"Kjører","pl":"Uruchomiony","pt":"Em execução","sv":"Körs","zh":"运行中"},
    "Scheduled": {"cs":"Naplánováno","da":"Planlagt","de":"Geplant","el":"Προγραμματισμένο","es":"Programado","fr":"Planifié","hi":"निर्धारित","it":"Programmato","ja":"スケジュール済み","nl":"Gepland","no":"Planlagt","pl":"Zaplanowane","pt":"Agendado","sv":"Schemalagt","zh":"已计划"},
    "Secrets": {"cs":"Tajemství","da":"Hemmeligheder","de":"Geheimnisse","el":"Μυστικά","es":"Secretos","fr":"Secrets","hi":"रहस्य","it":"Segreti","ja":"シークレット","nl":"Geheimen","no":"Hemmeligheter","pl":"Sekrety","pt":"Segredos","sv":"Hemligheter","zh":"秘密"},
    "Select...": {"cs":"Vybrat...","da":"Vælg...","de":"Auswählen...","el":"Επιλογή...","es":"Seleccionar...","fr":"Sélectionner...","hi":"चुनें...","it":"Seleziona...","ja":"選択...","nl":"Selecteren...","no":"Velg...","pl":"Wybierz...","pt":"Selecionar...","sv":"Välj...","zh":"选择..."},
    "Shell": {"cs":"Shell","da":"Shell","de":"Shell","el":"Κέλυφος","es":"Shell","fr":"Shell","hi":"शेल","it":"Shell","ja":"シェル","nl":"Shell","no":"Shell","pl":"Powłoka","pt":"Shell","sv":"Shell","zh":"Shell"},
    "Snapshots": {"cs":"Snímky","da":"Øjebliksbilleder","de":"Snapshots","el":"Στιγμιότυπα","es":"Instantáneas","fr":"Instantanés","hi":"स्नैपशॉट","it":"Snapshot","ja":"スナップショット","nl":"Snapshots","no":"Øyeblikksbilder","pl":"Migawki","pt":"Instantâneos","sv":"Ögonblicksbilder","zh":"快照"},
    "Start": {"cs":"Spustit","da":"Start","de":"Starten","el":"Εκκίνηση","es":"Iniciar","fr":"Démarrer","hi":"शुरू करें","it":"Avvia","ja":"開始","nl":"Starten","no":"Start","pl":"Uruchom","pt":"Iniciar","sv":"Starta","zh":"启动"},
    "Stop": {"cs":"Zastavit","da":"Stop","de":"Stoppen","el":"Διακοπή","es":"Detener","fr":"Arrêter","hi":"रुकें","it":"Ferma","ja":"停止","nl":"Stoppen","no":"Stopp","pl":"Zatrzymaj","pt":"Parar","sv":"Stoppa","zh":"停止"},
    "Streaming (SSE)": {"cs":"Streamování (SSE)","da":"Streaming (SSE)","de":"Streaming (SSE)","el":"Ροή (SSE)","es":"Transmisión (SSE)","fr":"Diffusion (SSE)","hi":"स्ट्रीमिंग (SSE)","it":"Streaming (SSE)","ja":"ストリーミング (SSE)","nl":"Streaming (SSE)","no":"Strømming (SSE)","pl":"Strumieniowanie (SSE)","pt":"Transmissão (SSE)","sv":"Strömning (SSE)","zh":"流式传输 (SSE)"},
    "Successful": {"cs":"Úspěšné","da":"Vellykket","de":"Erfolgreich","el":"Επιτυχής","es":"Exitoso","fr":"Réussi","hi":"सफल","it":"Riuscito","ja":"成功","nl":"Succesvol","no":"Vellykket","pl":"Udane","pt":"Bem-sucedido","sv":"Lyckades","zh":"成功"},
    "Tokens": {"cs":"Tokeny","da":"Tokens","de":"Tokens","el":"Διακριτικά","es":"Tokens","fr":"Jetons","hi":"टोकन","it":"Token","ja":"トークン","nl":"Tokens","no":"Token","pl":"Tokeny","pt":"Tokens","sv":"Token","zh":"令牌"},
    "Total": {"cs":"Celkem","da":"Total","de":"Gesamt","el":"Σύνολο","es":"Total","fr":"Total","hi":"कुल","it":"Totale","ja":"合計","nl":"Totaal","no":"Total","pl":"Łącznie","pt":"Total","sv":"Totalt","zh":"总计"},
    "Uptime": {"cs":"Doba běhu","da":"Oppetid","de":"Betriebszeit","el":"Χρόνος λειτουργίας","es":"Tiempo activo","fr":"Disponibilité","hi":"अपटाइम","it":"Tempo di attività","ja":"稼働時間","nl":"Uptime","no":"Oppetid","pl":"Czas działania","pt":"Tempo de atividade","sv":"Drifttid","zh":"运行时间"},
    "Vault": {"cs":"Trezor","da":"Boks","de":"Vault","el":"Θησαυροφυλάκιο","es":"Cofre","fr":"Coffre","hi":"Vault","it":"Vault","ja":"Vault","nl":"Kluis","no":"Hvelv","pl":"Sejf","pt":"Cofre","sv":"Valv","zh":"保管库"},
    "Vision": {"cs":"Vidění","da":"Vision","de":"Vision","el":"Όραση","es":"Visión","fr":"Vision","hi":"दृष्टि","it":"Visione","ja":"ビジョン","nl":"Visie","no":"Visjon","pl":"Wizja","pt":"Visão","sv":"Vision","zh":"视觉"},
}

stats = defaultdict(int)
for d in sorted(str(p) for p in LANG_DIR.rglob("en.json")):
    direc = Path(d).parent
    en = json.loads(Path(d).read_text(encoding="utf-8-sig"))
    for lang in LANGS:
        lf = direc / f"{lang}.json"
        if not lf.exists():
            continue
        ld = json.loads(lf.read_text(encoding="utf-8-sig"))
        mod = False
        for k, ev in en.items():
            if k not in ld:
                continue
            if str(ld[k]).strip().lower() != str(ev).strip().lower():
                continue
            evs = ev.strip()
            if evs in T:
                t = T[evs].get(lang)
                if t:
                    ld[k] = t
                    mod = True
                    stats[lang] += 1
        if mod:
            lf.write_text(json.dumps(ld, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

print("Fixed:", dict(stats))
print("Total:", sum(stats.values()))

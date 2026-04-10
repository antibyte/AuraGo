#!/usr/bin/env python3
"""Batch 4: Backend messages and common UI terms for non-German languages."""
import json, os
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "Response complete.":{"cs":"Odpověď dokončena.","da":"Svar fuldført.","de":"Antwort abgeschlossen.","el":"Η απάντηση ολοκληρώθηκε.","es":"Respuesta completada.","fr":"Réponse terminée.","hi":"उत्तर पूर्ण।","it":"Risposta completata.","ja":"応答が完了しました。","nl":"Antwoord voltooid.","no":"Svar fullført.","pl":"Odpowiedź zakończona.","pt":"Resposta concluída.","sv":"Svar slutfört.","zh":"响应已完成。"},
    "Transcription failed":{"cs":"Přepis selhal","da":"Transkription mislykkedes","de":"Transkription fehlgeschlagen","el":"Η μεταγραφή απέτυχε","es":"Transcripción fallida","fr":"Transcription échouée","hi":"ट्रांसक्रिप्शन विफल","it":"Trascrizione fallita","ja":"文字起こしに失敗しました","nl":"Transcriptie mislukt","no":"Transkripsjon mislyktes","pl":"Transkrypcja nie powiodła się","pt":"Transcrição falhou","sv":"Transkription misslyckades","zh":"转录失败"},
    "Invalid CSRF token":{"cs":"Neplatný CSRF token","da":"Ugyldigt CSRF-token","de":"Ungültiges CSRF-Token","el":"Μη έγκυρο CSRF token","es":"Token CSRF inválido","fr":"Jeton CSRF invalide","hi":"अमान्य CSRF टोकन","it":"Token CSRF non valido","ja":"無効なCSRFトークン","nl":"Ongeldig CSRF-token","no":"Ugyldig CSRF-token","pl":"Nieprawidłowy token CSRF","pt":"Token CSRF inválido","sv":"Ogiltig CSRF-token","zh":"无效的 CSRF 令牌"},
    "Invalid JSON":{"cs":"Neplatný JSON","da":"Ugyldig JSON","de":"Ungültiges JSON","el":"Μη έγκυρο JSON","es":"JSON inválido","fr":"JSON invalide","hi":"अमान्य JSON","it":"JSON non valido","ja":"無効なJSON","nl":"Ongeldig JSON","no":"Ugyldig JSON","pl":"Nieprawidłowy JSON","pt":"JSON inválido","sv":"Ogiltig JSON","zh":"无效的 JSON"},
    "Invalid request body":{"cs":"Neplatné tělo požadavku","da":"Ugyldig anmodningskrop","de":"Ungültiger Anfragekörper","el":"Μη έγκυρο σώμα αιτήματος","es":"Cuerpo de solicitud inválido","fr":"Corps de requête invalide","hi":"अमान्य अनुरोध बॉडी","it":"Corpo della richiesta non valido","ja":"無効なリクエストボディ","nl":"Ongeldige verzoekbody","no":"Ugyldig forespørselkropp","pl":"Nieprawidłowa treść żądania","pt":"Corpo da requisição inválido","sv":"Ogiltig begärankropp","zh":"无效的请求体"},
    "Internal error":{"cs":"Vnitřní chyba","da":"Intern fejl","de":"Interner Fehler","el":"Εσωτερικό σφάλμα","es":"Error interno","fr":"Erreur interne","hi":"आंतरिक त्रुटि","it":"Errore interno","ja":"内部エラー","nl":"Interne fout","no":"Intern feil","pl":"Błąd wewnętrzny","pt":"Erro interno","sv":"Internt fel","zh":"内部错误"},
    "Internal server error":{"cs":"Vnitřní chyba serveru","da":"Intern serverfejl","de":"Interner Serverfehler","el":"Εσωτερικό σφάλμα διακομιστή","es":"Error interno del servidor","fr":"Erreur interne du serveur","hi":"आंतरिक सर्वर त्रुटि","it":"Errore interno del server","ja":"サーバー内部エラー","nl":"Interne serverfout","no":"Intern serverfeil","pl":"Wewnętrzny błąd serwera","pt":"Erro interno do servidor","sv":"Internt serverfel","zh":"内部服务器错误"},
    "Setup already completed":{"cs":"Nastavení již dokončeno","da":"Opsætning allerede fuldført","de":"Einrichtung bereits abgeschlossen","el":"Η ρύθμιση ήδη ολοκληρώθηκε","es":"Configuración ya completada","fr":"Installation déjà terminée","hi":"सेटअप पहले ही पूरा हुआ","it":"Configurazione già completata","ja":"セットアップ済みです","nl":"Installatie al voltooid","no":"Oppsett allerede fullført","pl":"Konfiguracja już zakończona","pt":"Configuração já concluída","sv":"Installation redan slutförd","zh":"设置已完成"},
    "File not found":{"cs":"Soubor nenalezen","da":"Fil ikke fundet","de":"Datei nicht gefunden","el":"Το αρχείο δεν βρέθηκε","es":"Archivo no encontrado","fr":"Fichier introuvable","hi":"फ़ाइल नहीं मिली","it":"File non trovato","ja":"ファイルが見つかりません","nl":"Bestand niet gevonden","no":"Fil ikke funnet","pl":"Plik nie znaleziony","pt":"Arquivo não encontrado","sv":"Fil hittades inte","zh":"文件未找到"},
    "Contact not found":{"cs":"Kontakt nenalezen","da":"Kontakt ikke fundet","de":"Kontakt nicht gefunden","el":"Η επαφή δεν βρέθηκε","es":"Contacto no encontrado","fr":"Contact introuvable","hi":"संपर्क नहीं मिला","it":"Contatto non trovato","ja":"連絡先が見つかりません","nl":"Contact niet gevonden","no":"Kontakt ikke funnet","pl":"Kontakt nie znaleziony","pt":"Contato não encontrado","sv":"Kontakt hittades inte","zh":"联系人未找到"},
    "Personality not found":{"cs":"Osobnost nenalezena","da":"Personlighed ikke fundet","de":"Persönlichkeit nicht gefunden","el":"Η προσωπικότητα δεν βρέθηκε","es":"Personalidad no encontrada","fr":"Personnalité introuvable","hi":"व्यक्तित्व नहीं मिला","it":"Personalità non trovata","ja":"パーソナリティが見つかりません","nl":"Persoonlijkheid niet gevonden","no":"Personlighet ikke funnet","pl":"Osobowość nie znaleziona","pt":"Personalidade não encontrada","sv":"Personlighet hittades inte","zh":"人格未找到"},
    "Config path not set":{"cs":"Cesta ke konfiguraci nenastavena","da":"Konfigurationssti ikke angivet","de":"Konfigurationspfad nicht gesetzt","el":"Η διαδρομή διαμόρφωσης δεν ορίστηκε","es":"Ruta de configuración no establecida","fr":"Chemin de configuration non défini","hi":"कॉन्फ़िगरेशन पथ सेट नहीं","it":"Percorso configurazione non impostato","ja":"設定パスが未設定","nl":"Configuratiepad niet ingesteld","no":"Konfigurasjonssti ikke angitt","pl":"Ścieżka konfiguracji nie ustawiona","pt":"Caminho da configuração não definido","sv":"Konfigurationssökväg inte angiven","zh":"配置路径未设置"},
    "Planner database not initialized":{"cs":"Databáze plánovače neinicializována","da":"Planlægger-database ikke initialiseret","de":"Planer-Datenbank nicht initialisiert","el":"Η βάση δεδομένων προγραμματισμού δεν αρχικοποιήθηκε","es":"Base de datos del planificador no inicializada","fr":"Base de données du planificateur non initialisée","hi":"प्लानर डेटाबेस प्रारंभ नहीं किया गया","it":"Database pianificatore non inizializzato","ja":"プランナーデータベースが初期化されていません","nl":"Planner-database niet geïnitialiseerd","no":"Planleggerdatabase ikke initialisert","pl":"Baza danych planisty niezainicjalizowana","pt":"Banco de dados do planejador não inicializado","sv":"Planerardatabas inte initierad","zh":"规划器数据库未初始化"},
    "Personality engine is disabled":{"cs":"Engine osobností je zakázán","da":"Personlighedsmotor er deaktiveret","de":"Persönlichkeits-Engine ist deaktiviert","el":"Η μηχανή προσωπικότητας είναι απενεργοποιημένη","es":"El motor de personalidad está desactivado","fr":"Le moteur de personnalité est désactivé","hi":"व्यक्तित्व इंजन अक्षम है","it":"Il motore della personalità è disabilitato","ja":"パーソナリティエンジンは無効です","nl":"Persoonlijkheidsengine is uitgeschakeld","no":"Personlighetsmotor er deaktivert","pl":"Silnik osobowości jest wyłączony","pt":"O motor de personalidade está desabilitado","sv":"Personlighetsmotor är inaktiverad","zh":"人格引擎已禁用"},
    "Cannot delete the currently active personality":{"cs":"Nelze smazat aktuálně aktivní osobnost","da":"Kan ikke slette den aktuelt aktive personlighed","de":"Die aktive Persönlichkeit kann nicht gelöscht werden","el":"Δεν μπορεί να διαγραφεί η ενεργή προσωπικότητα","es":"No se puede eliminar la personalidad activa","fr":"Impossible de supprimer la personnalité active","hi":"वर्तमान सक्रिय व्यक्तित्व को हटा नहीं सकते","it":"Impossibile eliminare la personalità attiva","ja":"現在アクティブなパーソナリティは削除できません","nl":"Kan de actieve persoonlijkheid niet verwijderen","no":"Kan ikke slette den aktive personligheten","pl":"Nie można usunąć aktywnej osobowości","pt":"Não é possível excluir a personalidade ativa","sv":"Kan inte ta bort den aktiva personligheten","zh":"无法删除当前活跃的人格"},
    "Sorry, I encountered an error processing your message.":{"cs":"Omlouvám se, došlo k chybě při zpracování vaší zprávy.","da":"Beklager, der opstod en fejl ved behandling af din besked.","de":"Entschuldigung, bei der Verarbeitung Ihrer Nachricht ist ein Fehler aufgetreten.","el":"Συγγνώμη, παρουσιάστηκε σφάλμα κατά την επεξεργασία του μηνύματός σας.","es":"Lo siento, ocurrió un error al procesar su mensaje.","fr":"Désolé, une erreur s'est produite lors du traitement de votre message.","hi":"क्षमा करें, आपका संदेश प्रोसेस करते समय त्रुटि हुई।","it":"Spiacente, si è verificato un errore nell'elaborazione del messaggio.","ja":"申し訳ありません、メッセージの処理中にエラーが発生しました。","nl":"Sorry, er is een fout opgetreden bij het verwerken van uw bericht.","no":"Beklager, det oppstod en feil ved behandling av meldingen din.","pl":"Przepraszam, wystąpił błąd podczas przetwarzania wiadomości.","pt":"Desculpe, ocorreu um erro ao processar sua mensagem.","sv":"Beklagar, ett fel uppstod vid bearbetning av ditt meddelande.","zh":"抱歉，处理您的消息时遇到错误。"},
    "Budget system is disabled.":{"cs":"Systém rozpočtů je zakázán.","da":"Budgetsystem er deaktiveret.","de":"Budget-System ist deaktiviert.","el":"Το σύστημα προϋπολογισμού είναι απενεργοποιημένο.","es":"El sistema de presupuesto está desactivado.","fr":"Le système de budget est désactivé.","hi":"बजट प्रणाली अक्षम है।","it":"Il sistema di budget è disabilitato.","ja":"予算システムは無効です。","nl":"Budgetsysteem is uitgeschakeld.","no":"Budsjettssystem er deaktivert.","pl":"System budżetowy jest wyłączony.","pt":"O sistema de orçamento está desabilitado.","sv":"Budgetsystemet är inaktiverat.","zh":"预算系统已禁用。"},
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

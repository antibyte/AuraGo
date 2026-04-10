#!/usr/bin/env python3
"""Generate translations.json using pattern matching + word dictionaries."""
import json, re
from pathlib import Path

LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

# Pattern templates: {pattern_id: {lang: template_with_{0}{1}}}
PT = {
 "failed": {"cs":"Nepodařilo se {0} {1}","da":"Kunne ikke {0} {1}","de":"{0} fehlgeschlagen: {1}","el":"Αποτυχία {0} {1}","es":"No se pudo {0} {1}","fr":"Échec de {0} {1}","hi":"{0} विफल: {1}","it":"Impossibile {0} {1}","ja":"{0}に失敗: {1}","nl":"Kan {1} niet {0}","no":"Kunne ikke {0} {1}","pl":"Nie udało się {0} {1}","pt":"Falha ao {0} {1}","sv":"Kunde inte {0} {1}","zh":"{0}{1}失败"},
 "loading": {"cs":"Načítám {0}...","da":"Indlæser {0}...","de":"{0} werden geladen...","el":"Φόρτωση {0}...","es":"Cargando {0}...","fr":"Chargement {0}...","hi":"{0} लोड हो रहा है...","it":"Caricamento {0}...","ja":"{0}を読み込み中...","nl":"{0} laden...","no":"Laster {0}...","pl":"Ładowanie {0}...","pt":"Carregando {0}...","sv":"Laddar {0}...","zh":"正在加载{0}..."},
 "no_found": {"cs":"Žádné {0} nenalezeny","da":"Ingen {0} fundet","de":"Keine {0} gefunden","el":"Δεν βρέθηκαν {0}","es":"No se encontraron {0}","fr":"Aucun {0} trouvé","hi":"कोई {0} नहीं मिला","it":"Nessun {0} trovato","ja":"{0}が見つかりません","nl":"Geen {0} gevonden","no":"Ingen {0} funnet","pl":"Nie znaleziono {0}","pt":"Nenhum {0} encontrado","sv":"Inga {0} hittades","zh":"未找到{0}"},
 "really_del": {"cs":"Opravdu smazat {0}?","da":"Vil du virkelig slette {0}?","de":"{0} wirklich löschen?","el":"Σίγουρα διαγραφή {0};","es":"¿Realmente eliminar {0}?","fr":"Vraiment supprimer {0} ?","hi":"क्या वाकई {0} हटाएं?","it":"Eliminare davvero {0}?","ja":"本当に{0}を削除しますか？","nl":"Echt {0} verwijderen?","no":"Vil du virkelig slette {0}?","pl":"Naprawdę usunąć {0}?","pt":"Realmente excluir {0}?","sv":"Verkligen ta bort {0}?","zh":"确定删除{0}？"},
 "error_x": {"cs":"Chyba při {0}:","da":"Fejl ved {0}:","de":"Fehler beim {0}:","el":"Σφάλμα κατά {0}:","es":"Error al {0}:","fr":"Erreur lors de {0} :","hi":"{0} में त्रुटि:","it":"Errore durante {0}:","ja":"{0}エラー:","nl":"Fout bij {0}:","no":"Feil ved {0}:","pl":"Błąd podczas {0}:","pt":"Erro ao {0}:","sv":"Fel vid {0}:","zh":"{0}时出错："},
}

# Word translations for pattern substitution
WD = {
 "save":{"cs":"ukládání","da":"gemning","de":"Speichern","el":"αποθήκευση","es":"guardar","fr":"sauvegarde","hi":"सहेजना","it":"salvataggio","ja":"保存","nl":"opslaan","no":"lagring","pl":"zapisywaniu","pt":"salvar","sv":"spara","zh":"保存"},
 "delete":{"cs":"mazání","da":"sletning","de":"Löschen","el":"διαγραφή","es":"eliminar","fr":"suppression","hi":"हटाना","it":"eliminazione","ja":"削除","nl":"verwijderen","no":"sletting","pl":"usuwaniu","pt":"excluir","sv":"borttagning","zh":"删除"},
 "create":{"cs":"vytváření","da":"oprettelse","de":"Erstellen","el":"δημιουργία","es":"crear","fr":"création","hi":"बनाना","it":"creazione","ja":"作成","nl":"aanmaken","no":"oppretting","pl":"tworzeniu","pt":"criar","sv":"skapande","zh":"创建"},
 "read":{"cs":"čtení","da":"læsning","de":"Lesen","el":"ανάγνωση","es":"leer","fr":"lecture","hi":"पढ़ना","it":"lettura","ja":"読み取り","nl":"lezen","no":"lesing","pl":"odczycie","pt":"ler","sv":"läsning","zh":"读取"},
 "load":{"cs":"načítání","da":"indlæsning","de":"Laden","el":"φόρτωση","es":"cargar","fr":"chargement","hi":"लोड","it":"caricamento","ja":"読み込み","nl":"laden","no":"lasting","pl":"ładowaniu","pt":"carregar","sv":"laddning","zh":"加载"},
 "list":{"cs":"výpis","da":"visning","de":"Auflisten","el":"παράθεση","es":"listar","fr":"liste","hi":"सूची","it":"elenco","ja":"一覧","nl":"weergeven","no":"visning","pl":"wyświetlaniu","pt":"listar","sv":"lista","zh":"列出"},
 "update":{"cs":"aktualizace","da":"opdatering","de":"Aktualisieren","el":"ενημέρωση","es":"actualizar","fr":"mise à jour","hi":"अपडेट","it":"aggiornamento","ja":"更新","nl":"bijwerken","no":"oppdatering","pl":"aktualizacji","pt":"atualizar","sv":"uppdatering","zh":"更新"},
 "parse":{"cs":"zpracování","da":"fortolkning","de":"Analysieren","el":"ανάλυση","es":"analizar","fr":"analyse","hi":"पार्स","it":"analisi","ja":"解析","nl":"verwerken","no":"tolkning","pl":"analizie","pt":"analisar","sv":"tolkning","zh":"解析"},
 "write":{"cs":"zápis","da":"skrivning","de":"Schreiben","el":"εγγραφή","es":"escribir","fr":"écriture","hi":"लेखन","it":"scrittura","ja":"書き込み","nl":"schrijven","no":"skriving","pl":"zapisie","pt":"escrever","sv":"skrivning","zh":"写入"},
 "generate":{"cs":"generování","da":"generering","de":"Generieren","el":"δημιουργία","es":"generar","fr":"génération","hi":"जनरेट","it":"generazione","ja":"生成","nl":"genereren","no":"generering","pl":"generowaniu","pt":"gerar","sv":"generering","zh":"生成"},
 "datasets":{"cs":"datové sady","da":"datasæt","de":"Datensätze","el":"σύνολα δεδομένων","es":"conjuntos de datos","fr":"jeux de données","hi":"डेटासेट","it":"dataset","ja":"データセット","nl":"datasets","no":"datasett","pl":"zbiory danych","pt":"conjuntos de dados","sv":"datauppsättningar","zh":"数据集"},
 "pools":{"cs":"fondy","da":"pools","de":"Pools","el":"ομάδες","es":"pools","fr":"pools","hi":"पूल","it":"pool","ja":"プール","nl":"pools","no":"pooler","pl":"pule","pt":"pools","sv":"pooler","zh":"存储池"},
 "shares":{"cs":"sdílené položky","da":"delinger","de":"Freigaben","el":"κοινές χρήσεις","es":"recursos compartidos","fr":"partages","hi":"शेयर","it":"condivisioni","ja":"共有","nl":"shares","no":"delinger","pl":"udziały","pt":"compartilhamentos","sv":"resurser","zh":"共享"},
 "snapshots":{"cs":"snímky","da":"snapshots","de":"Snapshots","el":"στιγμιότυπα","es":"instantáneas","fr":"instantanés","hi":"स्नैपशॉट","it":"snapshot","ja":"スナップショット","nl":"snapshots","no":"øyeblikksbilder","pl":"migawki","pt":"snapshots","sv":"ögonblicksbilder","zh":"快照"},
 "webhooks":{"cs":"webhooky","da":"webhooks","de":"Webhooks","el":"webhooks","es":"webhooks","fr":"webhooks","hi":"वेबहुक","it":"webhook","ja":"Webhook","nl":"webhooks","no":"webhooks","pl":"webhooki","pt":"webhooks","sv":"webhooks","zh":"Webhook"},
 "config":{"cs":"konfiguraci","da":"konfigurationen","de":"Konfiguration","el":"διαμόρφωση","es":"configuración","fr":"configuration","hi":"कॉन्फ़िगरेशन","it":"configurazione","ja":"設定","nl":"configuratie","no":"konfigurasjon","pl":"konfigurację","pt":"configuração","sv":"konfiguration","zh":"配置"},
 "contact":{"cs":"kontakt","da":"kontakt","de":"Kontakt","el":"επαφή","es":"contacto","fr":"contact","hi":"संपर्क","it":"contatto","ja":"連絡先","nl":"contact","no":"kontakt","pl":"kontakt","pt":"contato","sv":"kontakt","zh":"联系人"},
 "personality":{"cs":"osobnost","da":"personlighed","de":"Persönlichkeit","el":"προσωπικότητα","es":"personalidad","fr":"personnalité","hi":"व्यक्तित्व","it":"personalità","ja":"パーソナリティ","nl":"persoonlijkheid","no":"personlighet","pl":"osobowość","pt":"personalidade","sv":"personlighet","zh":"人格"},
 "file":{"cs":"soubor","da":"fil","de":"Datei","el":"αρχείο","es":"archivo","fr":"fichier","hi":"फ़ाइल","it":"file","ja":"ファイル","nl":"bestand","no":"fil","pl":"plik","pt":"arquivo","sv":"fil","zh":"文件"},
 "audio":{"cs":"zvuk","da":"lyd","de":"Audio","el":"ήχος","es":"audio","fr":"audio","hi":"ऑडियो","it":"audio","ja":"オーディオ","nl":"audio","no":"lyd","pl":"audio","pt":"áudio","sv":"ljud","zh":"音频"},
}

# Specific translations (highest priority)
SP = {
 "Cancel":{"cs":"Zrušit","da":"Annuller","de":"Abbrechen","el":"Ακύρωση","es":"Cancelar","fr":"Annuler","hi":"रद्द करें","it":"Annulla","ja":"キャンセル","nl":"Annuleren","no":"Avbryt","pl":"Anuluj","pt":"Cancelar","sv":"Avbryt","zh":"取消"},
 "Clear":{"cs":"Vymazat","da":"Ryd","de":"Leeren","el":"Εκκαθάριση","es":"Limpiar","fr":"Effacer","hi":"साफ़ करें","it":"Cancella","ja":"クリア","nl":"Wissen","no":"Tøm","pl":"Wyczyść","pt":"Limpar","sv":"Rensa","zh":"清除"},
 "Copied":{"cs":"Zkopírováno","da":"Kopieret","de":"Kopiert","el":"Αντιγράφηκε","es":"Copiado","fr":"Copié","hi":"कॉपी किया गया","it":"Copiato","ja":"コピーしました","nl":"Gekopieerd","no":"Kopiert","pl":"Skopiowano","pt":"Copiado","sv":"Kopierat","zh":"已复制"},
 "Dismiss":{"cs":"Zavřít","da":"Luk","de":"Schließen","el":"Απόρριψη","es":"Descartar","fr":"Ignorer","hi":"खारिज करें","it":"Chiudi","ja":"閉じる","nl":"Sluiten","no":"Lukk","pl":"Odrzuć","pt":"Dispensar","sv":"Avfärda","zh":"关闭"},
 "Expand":{"cs":"Rozbalit","da":"Udvid","de":"Aufklappen","el":"Ανάπτυξη","es":"Expandir","fr":"Développer","hi":"विस्तार","it":"Espandi","ja":"展開","nl":"Uitvouwen","no":"Utvid","pl":"Rozwiń","pt":"Expandir","sv":"Expandera","zh":"展开"},
 "Export":{"cs":"Exportovat","da":"Eksportér","de":"Exportieren","el":"Εξαγωγή","es":"Exportar","fr":"Exporter","hi":"निर्यात","it":"Esporta","ja":"エクスポート","nl":"Exporteren","no":"Eksporter","pl":"Eksportuj","pt":"Exportar","sv":"Exportera","zh":"导出"},
 "Import":{"cs":"Importovat","da":"Importér","de":"Importieren","el":"Εισαγωγή","es":"Importar","fr":"Importer","hi":"आयात","it":"Importa","ja":"インポート","nl":"Importeren","no":"Importer","pl":"Importuj","pt":"Importar","sv":"Importera","zh":"导入"},
 "Remove":{"cs":"Odebrat","da":"Fjern","de":"Entfernen","el":"Κατάργηση","es":"Eliminar","fr":"Supprimer","hi":"हटाएं","it":"Rimuovi","ja":"削除","nl":"Verwijderen","no":"Fjern","pl":"Usuń","pt":"Remover","sv":"Ta bort","zh":"移除"},
 "Retry":{"cs":"Zkusit znovu","da":"Prøv igen","de":"Erneut versuchen","el":"Επανάληψη","es":"Reintentar","fr":"Réessayer","hi":"पुनः प्रयास","it":"Riprova","ja":"再試行","nl":"Opnieuw proberen","no":"Prøv igjen","pl":"Ponów","pt":"Tentar novamente","sv":"Försök igen","zh":"重试"},
 "Restore":{"cs":"Obnovit","da":"Gendan","de":"Wiederherstellen","el":"Επαναφορά","es":"Restaurar","fr":"Restaurer","hi":"पुनर्स्थापना","it":"Ripristina","ja":"復元","nl":"Herstellen","no":"Gjenopprett","pl":"Przywróć","pt":"Restaurar","sv":"Återställ","zh":"恢复"},
 "Send":{"cs":"Odeslat","da":"Send","de":"Senden","el":"Αποστολή","es":"Enviar","fr":"Envoyer","hi":"भेजें","it":"Invia","ja":"送信","nl":"Verzenden","no":"Send","pl":"Wyślij","pt":"Enviar","sv":"Skicka","zh":"发送"},
 "Testing...":{"cs":"Testuji...","da":"Tester...","de":"Teste...","el":"Δοκιμή...","es":"Probando...","fr":"Test...","hi":"परीक्षण...","it":"Test in corso...","ja":"テスト中...","nl":"Testen...","no":"Tester...","pl":"Testowanie...","pt":"Testando...","sv":"Testar...","zh":"正在测试..."},
 "Connection error":{"cs":"Chyba připojení","da":"Forbindelsesfejl","de":"Verbindungsfehler","el":"Σφάλμα σύνδεσης","es":"Error de conexión","fr":"Erreur de connexion","hi":"कनेक्शन त्रुटि","it":"Errore di connessione","ja":"接続エラー","nl":"Verbindingsfout","no":"Tilkoblingsfeil","pl":"Błąd połączenia","pt":"Erro de conexão","sv":"Anslutningsfel","zh":"连接错误"},
 "Connection successful":{"cs":"Připojení úspěšné","da":"Forbindelse vellykket","de":"Verbindung erfolgreich","el":"Επιτυχής σύνδεση","es":"Conexión exitosa","fr":"Connexion réussie","hi":"कनेक्शन सफल","it":"Connessione riuscita","ja":"接続成功","nl":"Verbinding geslaagd","no":"Tilkobling vellykket","pl":"Połączenie udane","pt":"Conexão bem-sucedida","sv":"Anslutning lyckades","zh":"连接成功"},
 "Settings saved":{"cs":"Nastavení uložena","da":"Indstillinger gemt","de":"Einstellungen gespeichert","el":"Οι ρυθμίσεις αποθηκεύτηκαν","es":"Configuración guardada","fr":"Paramètres enregistrés","hi":"सेटिंग्स सहेजी गईं","it":"Impostazioni salvate","ja":"設定を保存しました","nl":"Instellingen opgeslagen","no":"Innstillinger lagret","pl":"Ustawienia zapisane","pt":"Configurações salvas","sv":"Inställningar sparade","zh":"设置已保存"},
 "Rollback successful":{"cs":"Návrat úspěšný","da":"Tilbagerulning lykkedes","de":"Rollback erfolgreich","el":"Επαναφορά επιτυχής","es":"Reversión exitosa","fr":"Restauration réussie","hi":"रोलबैक सफल","it":"Ripristino completato","ja":"ロールバック成功","nl":"Terugdraaien geslaagd","no":"Tilbakerulling vellykket","pl":"Wycofanie udane","pt":"Reversão bem-sucedida","sv":"Återställning lyckades","zh":"回滚成功"},
 "Version History":{"cs":"Historie verzí","da":"Versionshistorik","de":"Versionsverlauf","el":"Ιστορικό εκδόσεων","es":"Historial de versiones","fr":"Historique des versions","hi":"संस्करण इतिहास","it":"Cronologia versioni","ja":"バージョン履歴","nl":"Versiegeschiedenis","no":"Versjonshistorikk","pl":"Historia wersji","pt":"Histórico de versões","sv":"Versionshistorik","zh":"版本历史"},
 "Version restored":{"cs":"Verze obnovena","da":"Version gendannet","de":"Version wiederhergestellt","el":"Έκδοση αποκαταστάθηκε","es":"Versión restaurada","fr":"Version restaurée","hi":"संस्करण पुनर्स्थापित","it":"Versione ripristinata","ja":"バージョン復元済み","nl":"Versie hersteld","no":"Versjon gjenopprettet","pl":"Wersja przywrócona","pt":"Versão restaurada","sv":"Version återställd","zh":"版本已恢复"},
 "Unauthorized":{"cs":"Neautorizováno","da":"Uautoriseret","de":"Nicht autorisiert","el":"Μη εξουσιοδοτημένο","es":"No autorizado","fr":"Non autorisé","hi":"अनधिकृत","it":"Non autorizzato","ja":"認証なし","nl":"Niet geautoriseerd","no":"Uautorisert","pl":"Nieautoryzowany","pt":"Não autorizado","sv":"Obehörig","zh":"未授权"},
 "Token deleted":{"cs":"Token smazán","da":"Token slettet","de":"Token gelöscht","el":"Το διακριτικό διαγράφηκε","es":"Token eliminado","fr":"Jeton supprimé","hi":"टोकन हटाया गया","it":"Token eliminato","ja":"トークン削除済み","nl":"Token verwijderd","no":"Token slettet","pl":"Token usunięty","pt":"Token excluído","sv":"Token borttagen","zh":"令牌已删除"},
 "Read-Only":{"cs":"Jen pro čtení","da":"Skrivebeskyttet","de":"Schreibgeschützt","el":"Μόνο ανάγνωση","es":"Solo lectura","fr":"Lecture seule","hi":"केवल पठन","it":"Sola lettura","ja":"読み取り専用","nl":"Alleen-lezen","no":"Skrivebeskyttet","pl":"Tylko do odczytu","pt":"Somente leitura","sv":"Skrivskyddad","zh":"只读"},
 "Optimization":{"cs":"Optimalizace","da":"Optimering","de":"Optimierung","el":"Βελτιστοποίηση","es":"Optimización","fr":"Optimisation","hi":"अनुकूलन","it":"Ottimizzazione","ja":"最適化","nl":"Optimalisatie","no":"Optimalisering","pl":"Optymalizacja","pt":"Otimização","sv":"Optimering","zh":"优化"},
 "Overview":{"cs":"Přehled","da":"Oversigt","de":"Übersicht","el":"Επισκόπηση","es":"Resumen","fr":"Aperçu","hi":"अवलोकन","it":"Panoramica","ja":"概要","nl":"Overzicht","no":"Oversikt","pl":"Przegląd","pt":"Visão geral","sv":"Översikt","zh":"概览"},
 "Streaming unsupported":{"cs":"Streamování nepodporováno","da":"Streaming understøttes ikke","de":"Streaming nicht unterstützt","el":"Η ροή δεν υποστηρίζεται","es":"Streaming no soportado","fr":"Streaming non supporté","hi":"स्ट्रीमिंग असमर्थित","it":"Streaming non supportato","ja":"ストリーミング非対応","nl":"Streaming niet ondersteund","no":"Strømming støttes ikke","pl":"Przesyłanie nieobsługiwane","pt":"Streaming não suportado","sv":"Strömning stöds inte","zh":"不支持流式传输"},
 "Test Connection":{"cs":"Otestovat připojení","da":"Test forbindelse","de":"Verbindung testen","el":"Δοκιμή σύνδεσης","es":"Probar conexión","fr":"Tester la connexion","hi":"कनेक्शन परीक्षण","it":"Testa connessione","ja":"接続テスト","nl":"Verbinding testen","no":"Test tilkobling","pl":"Testuj połączenie","pt":"Testar conexão","sv":"Testa anslutning","zh":"测试连接"},
 "Copy config":{"cs":"Kopírovat konfiguraci","da":"Kopiér konfiguration","de":"Konfiguration kopieren","el":"Αντιγραφή διαμόρφωσης","es":"Copiar configuración","fr":"Copier la configuration","hi":"कॉन्फ़िगरेशन कॉपी","it":"Copia configurazione","ja":"設定をコピー","nl":"Configuratie kopiëren","no":"Kopier konfigurasjon","pl":"Kopiuj konfigurację","pt":"Copiar configuração","sv":"Kopiera konfiguration","zh":"复制配置"},
 "Zoom in":{"cs":"Přiblížit","da":"Zoom ind","de":"Vergrößern","el":"Μεγένθυνση","es":"Ampliar","fr":"Zoom avant","hi":"ज़ूम इन","it":"Ingrandisci","ja":"ズームイン","nl":"Inzoomen","no":"Zoom inn","pl":"Powiększ","pt":"Ampliar","sv":"Zooma in","zh":"放大"},
 "Zoom out":{"cs":"Oddálit","da":"Zoom ud","de":"Verkleinern","el":"Σμίκρυνση","es":"Reducir","fr":"Zoom arrière","hi":"ज़ूम आउट","it":"Riduci","ja":"ズームアウト","nl":"Uitzoomen","no":"Zoom ut","pl":"Pomniejsz","pt":"Reduzir","sv":"Zooma ut","zh":"缩小"},
 "Reset zoom":{"cs":"Resetovat přiblížení","da":"Nulstil zoom","de":"Zoom zurücksetzen","el":"Επαναφορά ζουμ","es":"Restablecer zoom","fr":"Réinitialiser zoom","hi":"ज़ूम रीसेट","it":"Reimposta zoom","ja":"ズームリセット","nl":"Zoom resetten","no":"Tilbakestill zoom","pl":"Resetuj powiększenie","pt":"Redefinir zoom","sv":"Återställ zoom","zh":"重置缩放"},
 "View source":{"cs":"Zobrazit zdroj","da":"Vis kilde","de":"Quelltext anzeigen","el":"Προβολή πηγής","es":"Ver fuente","fr":"Voir la source","hi":"स्रोत देखें","it":"Vedi sorgente","ja":"ソースを表示","nl":"Bron bekijken","no":"Vis kilde","pl":"Pokaż źródło","pt":"Ver fonte","sv":"Visa källa","zh":"查看源码"},
 "Fullscreen":{"cs":"Celá obrazovka","da":"Fuldskærm","de":"Vollbild","el":"Πλήρης οθόνη","es":"Pantalla completa","fr":"Plein écran","hi":"पूर्ण स्क्रीन","it":"Schermo intero","ja":"フルスクリーン","nl":"Volledig scherm","no":"Fullskjerm","pl":"Pełny ekran","pt":"Tela cheia","sv":"Fullskärm","zh":"全屏"},
 "JSON Input":{"cs":"JSON vstup","da":"JSON-input","de":"JSON-Eingabe","el":"Εισαγωγή JSON","es":"Entrada JSON","fr":"Entrée JSON","hi":"JSON इनपुट","it":"Input JSON","ja":"JSON入力","nl":"JSON-invoer","no":"JSON-inndata","pl":"Wejście JSON","pt":"Entrada JSON","sv":"JSON-indata","zh":"JSON 输入"},
}

# Pattern rules: (regex, template_key, verb_group_idx, noun_group_idx)
# verb_group_idx/noun_group_idx are 1-based regex group indices, 0 = not used
RULES = [
 (r"^Failed to (save|delete|create|read|load|list|update|parse|write|generate) (.+)$", "failed", 1, 2),
 (r"^Loading (.+)\.\.\.$", "loading", 0, 1),
 (r"^No (.+) found$", "no_found", 0, 1),
 (r"^Really delete (.+)\?$", "really_del", 0, 1),
 (r"^Error (saving|loading|creating|deleting|reading) (.+):$", "error_x", 1, 2),
]

def translate(text, lang):
    if text in SP:
        return SP[text].get(lang)
    for pattern, tkey, vidx, nidx in RULES:
        m = re.match(pattern, text, re.IGNORECASE)
        if m:
            tmpl = PT[tkey].get(lang)
            if not tmpl:
                continue
            verb = ""
            if vidx > 0:
                v = m.group(vidx).lower()
                verb = WD.get(v, {}).get(lang, m.group(vidx))
            noun = ""
            if nidx > 0 and nidx != vidx:
                n = m.group(nidx).lower().rstrip(".")
                noun = WD.get(n, {}).get(lang, m.group(nidx))
            if vidx > 0 and nidx > 0 and vidx != nidx:
                return tmpl.format(verb, noun)
            elif nidx > 0:
                return tmpl.format(noun)
            elif vidx > 0:
                return tmpl.format(verb)
            else:
                return tmpl
    return None

def generate():
    with open("disposable/untranslated_values.json", "r", encoding="utf-8") as f:
        data = json.load(f)
    values = list(data["values"].keys())
    translations = {}
    for val in values:
        entry = {}
        for lang in LANGS:
            t = translate(val, lang)
            if t:
                entry[lang] = t
        if entry:
            translations[val] = entry

    with open("disposable/translations.json", "w", encoding="utf-8") as f:
        json.dump(translations, f, ensure_ascii=False, indent=2)
    print(f"Generated {len(translations)} translation entries")
    matched = sum(1 for v in values if v in translations)
    print(f"Matched {matched}/{len(values)} untranslated values")

if __name__ == "__main__":
    generate()

#!/usr/bin/env python3
"""Batch 6: UI labels, actions, and status messages."""
import json, os
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "Checking connection...": {"cs":"Kontrola připojení...","da":"Kontrollerer forbindelse...","de":"Verbindung wird geprüft...","el":"Έλεγχος σύνδεσης...","es":"Comprobando conexión...","fr":"Vérification de la connexion...","hi":"कनेक्शन की जाँच हो रही है...","it":"Verifica connessione...","ja":"接続を確認中...","nl":"Verbinding controleren...","no":"Kontrollerer tilkobling...","pl":"Sprawdzanie połączenia...","pt":"Verificando conexão...","sv":"Kontrollerar anslutning...","zh":"正在检查连接..."},
    "Command failed": {"cs":"Příkaz selhal","da":"Kommando mislykkedes","de":"Befehl fehlgeschlagen","el":"Η εντολή απέτυχε","es":"Comando fallido","fr":"Échec de la commande","hi":"कमांड विफल","it":"Comando non riuscito","ja":"コマンド失敗","nl":"Opdracht mislukt","no":"Kommando mislyktes","pl":"Polecenie nie powiodło się","pt":"Comando falhou","sv":"Kommando misslyckades","zh":"命令失败"},
    "Confirm Rollback": {"cs":"Potvrdit návrat","da":"Bekræft tilbageføring","de":"Rollback bestätigen","el":"Επιβεβαίωση επαναφοράς","es":"Confirmar reversión","fr":"Confirmer la restauration","hi":"रोलबैक की पुष्टि करें","it":"Conferma ripristino","ja":"ロールバックを確認","nl":"Terugdraaien bevestigen","no":"Bekreft tilbakerulling","pl":"Potwierdź wycofanie","pt":"Confirmar reversão","sv":"Bekräfta återställning","zh":"确认回滚"},
    "Connection failed:": {"cs":"Připojení selhalo:","da":"Forbindelse mislykkedes:","de":"Verbindung fehlgeschlagen:","el":"Η σύνδεση απέτυχε:","es":"Conexión fallida:","fr":"Échec de la connexion :","hi":"कनेक्शन विफल:","it":"Connessione non riuscita:","ja":"接続失敗:","nl":"Verbinding mislukt:","no":"Tilkobling mislyktes:","pl":"Połączenie nie powiodło się:","pt":"Conexão falhou:","sv":"Anslutning misslyckades:","zh":"连接失败："},
    "Container Image": {"cs":"Image kontejneru","da":"Container-image","de":"Container-Image","el":"Εικόνα容器","es":"Imagen del contenedor","fr":"Image du conteneur","hi":"कंटेनर इमेज","it":"Immagine container","ja":"コンテナイメージ","nl":"Containerimage","no":"Container-image","pl":"Obraz kontenera","pt":"Imagem do contêiner","sv":"Containeravbildning","zh":"容器镜像"},
    "Copy": {"cs":"Kopírovat","da":"Kopiér","de":"Kopieren","el":"Αντιγραφή","es":"Copiar","fr":"Copier","hi":"कॉपी करें","it":"Copia","ja":"コピー","nl":"Kopiëren","no":"Kopier","pl":"Kopiuj","pt":"Copiar","sv":"Kopiera","zh":"复制"},
    "Copy diagram source": {"cs":"Kopírovat zdroj diagramu","da":"Kopiér diagramkilde","de":"Diagrammquelle kopieren","el":"Αντιγραφή πηγής διαγράμματος","es":"Copiar fuente del diagrama","fr":"Copier la source du diagramme","hi":"आरेख स्रोत कॉपी करें","it":"Copia sorgente diagramma","ja":"ダイアグラムソースをコピー","nl":"Diagrambron kopiëren","no":"Kopier diagramkilde","pl":"Kopiuj źródło diagramu","pt":"Copiar fonte do diagrama","sv":"Kopiera diagramkälla","zh":"复制图表源"},
    "Create": {"cs":"Vytvořit","da":"Opret","de":"Erstellen","el":"Δημιουργία","es":"Crear","fr":"Créer","hi":"बनाएं","it":"Crea","ja":"作成","nl":"Aanmaken","no":"Opprett","pl":"Utwórz","pt":"Criar","sv":"Skapa","zh":"创建"},
    "Create New Dataset": {"cs":"Vytvořit nový dataset","da":"Opret nyt dataset","de":"Neues Dataset erstellen","el":"Δημιουργία νέου συνόλου δεδομένων","es":"Crear nuevo conjunto de datos","fr":"Créer un nouveau jeu de données","hi":"नया डेटासेट बनाएं","it":"Crea nuovo dataset","ja":"新しいデータセットを作成","nl":"Nieuwe dataset aanmaken","no":"Opprett nytt datasett","pl":"Utwórz nowy zestaw danych","pt":"Criar novo conjunto de dados","sv":"Skapa ny dataset","zh":"创建新数据集"},
    "Create Snapshot": {"cs":"Vytvořit snímek","da":"Opret øjebliksbillede","de":"Snapshot erstellen","el":"Δημιουργία στιγμιότυπου","es":"Crear instantánea","fr":"Créer un instantané","hi":"स्नैपशॉट बनाएं","it":"Crea snapshot","ja":"スナップショットを作成","nl":"Snapshot aanmaken","no":"Opprett øyeblikksbilde","pl":"Utwórz migawkę","pt":"Criar instantâneo","sv":"Skapa ögonblicksbild","zh":"创建快照"},
    "Datasets": {"cs":"Datasety","da":"Datasets","de":"Datasets","el":"Σύνολα δεδομένων","es":"Conjuntos de datos","fr":"Jeux de données","hi":"डेटासेट","it":"Dataset","ja":"データセット","nl":"Datasets","no":"Datasett","pl":"Zestawy danych","pt":"Conjuntos de dados","sv":"Dataset","zh":"数据集"},
    "Delete Dataset": {"cs":"Smazat dataset","da":"Slet dataset","de":"Dataset löschen","el":"Διαγραφή συνόλου δεδομένων","es":"Eliminar conjunto de datos","fr":"Supprimer le jeu de données","hi":"डेटासेट हटाएं","it":"Elimina dataset","ja":"データセットを削除","nl":"Dataset verwijderen","no":"Slett datasett","pl":"Usuń zestaw danych","pt":"Excluir conjunto de dados","sv":"Ta bort dataset","zh":"删除数据集"},
    "Delete Snapshot": {"cs":"Smazat snímek","da":"Slet øjebliksbillede","de":"Snapshot löschen","el":"Διαγραφή στιγμιότυπου","es":"Eliminar instantánea","fr":"Supprimer l'instantané","hi":"स्नैपशॉट हटाएं","it":"Elimina snapshot","ja":"スナップショットを削除","nl":"Snapshot verwijderen","no":"Slett øyeblikksbilde","pl":"Usuń migawkę","pt":"Excluir instantâneo","sv":"Ta bort ögonblicksbild","zh":"删除快照"},
    "Diagram Error": {"cs":"Chyba diagramu","da":"Diagramfejl","de":"Diagrammfehler","el":"Σφάλμα διαγράμματος","es":"Error de diagrama","fr":"Erreur de diagramme","hi":"आरेख त्रुटि","it":"Errore diagramma","ja":"ダイアグラムエラー","nl":"Diagramfout","no":"Diagrammfeil","pl":"Błąd diagramu","pt":"Erro do diagrama","sv":"Diagramfel","zh":"图表错误"},
    "Diagram source copied": {"cs":"Zdroj diagramu zkopírován","da":"Diagramkilde kopieret","de":"Diagrammquelle kopiert","el":"Η πηγή διαγράμματος αντιγράφηκε","es":"Fuente del diagrama copiada","fr":"Source du diagramme copiée","hi":"आरेख स्रोत कॉपी किया गया","it":"Sorgente diagramma copiata","ja":"ダイアグラムソースをコピーしました","nl":"Diagrambron gekopieerd","no":"Diagramkilde kopiert","pl":"Źródło diagramu skopiowane","pt":"Fonte do diagrama copiada","sv":"Diagramkälla kopierad","zh":"图表源已复制"},
    "Download PNG": {"cs":"Stáhnout PNG","da":"Download PNG","de":"PNG herunterladen","el":"Λήψη PNG","es":"Descargar PNG","fr":"Télécharger PNG","hi":"PNG डाउनलोड करें","it":"Scarica PNG","ja":"PNGをダウンロード","nl":"PNG downloaden","no":"Last ned PNG","pl":"Pobierz PNG","pt":"Baixar PNG","sv":"Ladda ner PNG","zh":"下载 PNG"},
    "Download SVG": {"cs":"Stáhnout SVG","da":"Download SVG","de":"SVG herunterladen","el":"Λήψη SVG","es":"Descargar SVG","fr":"Télécharger SVG","hi":"SVG डाउनलोड करें","it":"Scarica SVG","ja":"SVGをダウンロード","nl":"SVG downloaden","no":"Last ned SVG","pl":"Pobierz SVG","pt":"Baixar SVG","sv":"Ladda ner SVG","zh":"下载 SVG"},
    "Drop files here to upload": {"cs":"Přetáhněte soubory sem pro nahrání","da":"Træk filer her for at uploade","de":"Dateien hierher ziehen zum Hochladen","el":"Τοποθετήστε αρχεία εδώ για μεταφόρτωση","es":"Suelta archivos aquí para subir","fr":"Déposez les fichiers ici pour les téléverser","hi":"अपलोड करने के लिए फ़ाइलें यहां खींचें","it":"Trascina i file qui per caricarli","ja":"アップロードするファイルをここにドロップ","nl":"Sleep bestanden hierheen om te uploaden","no":"Slipp filer her for å laste opp","pl":"Upuść pliki tutaj, aby przesłać","pt":"Solte arquivos aqui para enviar","sv":"Släpp filer här för att ladda upp","zh":"将文件拖放到此处上传"},
    "Edit": {"cs":"Upravit","da":"Rediger","de":"Bearbeiten","el":"Επεξεργασία","es":"Editar","fr":"Modifier","hi":"संपादित करें","it":"Modifica","ja":"編集","nl":"Bewerken","no":"Rediger","pl":"Edytuj","pt":"Editar","sv":"Redigera","zh":"编辑"},
    "Error saving settings": {"cs":"Chyba při ukládání nastavení","da":"Fejl ved gemning af indstillinger","de":"Fehler beim Speichern der Einstellungen","el":"Σφάλμα αποθήκευσης ρυθμίσεων","es":"Error al guardar la configuración","fr":"Erreur lors de la sauvegarde des paramètres","hi":"सेटिंग्स सहेजने में त्रुटि","it":"Errore nel salvare le impostazioni","ja":"設定の保存エラー","nl":"Fout bij opslaan instellingen","no":"Feil ved lagring av innstillinger","pl":"Błąd zapisywania ustawień","pt":"Erro ao salvar configurações","sv":"Fel vid sparning av inställningar","zh":"保存设置时出错"},
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

#!/usr/bin/env python3
"""Batch 10: Config section labels, navigation, and more UI terms."""
import json, os
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "Cheat Sheets": {"cs":"Taháky","da":"Snydeark","de":"Spickzettel","el":"Φύλλα αναφοράς","es":"Chuletas","fr":"Aide-mémoire","hi":"चीट शीट","it":"Schede di riferimento","ja":"チートシート","nl":"Spiekbriefjes","no":"Kokebøker","pl":"Ściągawki","pt":"Folhas de dicas","sv":"Fusklappar","zh":"速查表"},
    "Checking status...": {"cs":"Kontrola stavu...","da":"Kontrollerer status...","de":"Status wird geprüft...","el":"Έλεγχος κατάστασης...","es":"Comprobando estado...","fr":"Vérification du statut...","hi":"स्थिति की जाँच हो रही है...","it":"Verifica stato...","ja":"ステータス確認中...","nl":"Status controleren...","no":"Kontrollerer status...","pl":"Sprawdzanie statusu...","pt":"Verificando status...","sv":"Kontrollerar status...","zh":"正在检查状态..."},
    "Client configuration": {"cs":"Konfigurace klienta","da":"Klientkonfiguration","de":"Client-Konfiguration","el":"Διαμόρφωση πελάτη","es":"Configuración del cliente","fr":"Configuration du client","hi":"क्लाइंट कॉन्फ़िगरेशन","it":"Configurazione client","ja":"クライアント設定","nl":"Clientconfiguratie","no":"Klientkonfigurasjon","pl":"Konfiguracja klienta","pt":"Configuração do cliente","sv":"Klientkonfiguration","zh":"客户端配置"},
    "Container Logs": {"cs":"Logy kontejneru","da":"Container-logfiler","de":"Container-Logs","el":"Αρχεία καταγραφής容器","es":"Registros del contenedor","fr":"Journaux du conteneur","hi":"कंटेनर लॉग","it":"Log container","ja":"コンテナログ","nl":"Containerlogboeken","no":"Container-logger","pl":"Logi kontenera","pt":"Registros do contêiner","sv":"Containerloggar","zh":"容器日志"},
    "Core Memory": {"cs":"Základní paměť","da":"Kernehukommelse","de":"Kerngedächtnis","el":"Βασική μνήμη","es":"Memoria central","fr":"Mémoire centrale","hi":"मूख्य स्मृति","it":"Memoria principale","ja":"コアメモリ","nl":"Kerngeheugen","no":"Kjerneminne","pl":"Pamięć główna","pt":"Memória principal","sv":"Kärnminne","zh":"核心记忆"},
    "Credentials": {"cs":"Přihlašovací údaje","da":"Legitimationsoplysninger","de":"Anmeldeinformationen","el":"Διαπιστευτήρια","es":"Credenciales","fr":"Identifiants","hi":"क्रेडेंशियल","it":"Credenziali","ja":"認証情報","nl":"Inloggegevens","no":"Legitimasjon","pl":"Poświadczenia","pt":"Credenciais","sv":"Inloggningsuppgifter","zh":"凭据"},
    "Danger Zone": {"cs":"Nebezpečná zóna","da":"Farezone","de":"Gefahrenzone","el":"Επικίνδυνη ζώνη","es":"Zona de peligro","fr":"Zone de danger","hi":"खतरनाक क्षेत्र","it":"Zona pericolosa","ja":"危険地帯","nl":"Gevaarlijke zone","no":"Faresone","pl":"Strefa niebezpieczeństwa","pt":"Zona de perigo","sv":"Riskzon","zh":"危险区域"},
    "Daemon Skills": {"cs":"Démonové dovednosti","da":"Daemon-færdigheder","de":"Daemon-Skills","el":"Δεξιότητες δαίμονα","es":"Habilidades del demonio","fr":"Compétences du démon","hi":"डेमन कौशल","it":"Skill demone","ja":"デーモンスキル","nl":"Daemon-vaardigheden","no":"Daemon-ferdigheter","pl":"Umiejętności daemona","pt":"Habilidades do daemon","sv":"Daemon-färdigheter","zh":"守护进程技能"},
    "Delete Share": {"cs":"Smazat sdílení","da":"Slet deling","de":"Freigabe löschen","el":"Διαγραφή κοινής χρήσης","es":"Eliminar recurso compartido","fr":"Supprimer le partage","hi":"शेयर हटाएं","it":"Elimina condivisione","ja":"共有を削除","nl":"Deling verwijderen","no":"Slett deling","pl":"Usuń udział","pt":"Excluir compartilhamento","sv":"Ta bort delning","zh":"删除共享"},
    "Display name": {"cs":"Zobrazovaný název","da":"Visningsnavn","de":"Anzeigename","el":"Εμφανιζόμενο όνομα","es":"Nombre para mostrar","fr":"Nom d'affichage","hi":"प्रदर्शन नाम","it":"Nome visualizzato","ja":"表示名","nl":"Weergavenaam","no":"Visningsnavn","pl":"Nazwa wyświetlana","pt":"Nome de exibição","sv":"Visningsnamn","zh":"显示名称"},
    "Embeddings": {"cs":"Embeddingy","da":"Embeddings","de":"Embeddings","el":"Ενσωματώσεις","es":"Embeddings","fr":"Embeddings","hi":"एम्बेडिंग","it":"Embeddings","ja":"エンベディング","nl":"Embeddings","no":"Embeddinger","pl":"Osadzenia","pt":"Embeddings","sv":"Embeddingar","zh":"嵌入"},
    "Feedback": {"cs":"Zpětná vazba","da":"Feedback","de":"Feedback","el":"Ανατροφοδότηση","es":"Comentarios","fr":"Retour","hi":"प्रतिक्रिया","it":"Feedback","ja":"フィードバック","nl":"Feedback","no":"Tilbakemelding","pl":"Opinia","pt":"Feedback","sv":"Feedback","zh":"反馈"},
    "Folder": {"cs":"Složka","da":"Mappe","de":"Ordner","el":"Φάκελος","es":"Carpeta","fr":"Dossier","hi":"फ़ोल्डर","it":"Cartella","ja":"フォルダ","nl":"Map","no":"Mappe","pl":"Folder","pt":"Pasta","sv":"Mapp","zh":"文件夹"},
    "Job enabled.": {"cs":"Úloha povolena.","da":"Job aktiveret.","de":"Job aktiviert.","el":"Εργασία ενεργοποιήθηκε.","es":"Trabajo activado.","fr":"Tâche activée.","hi":"कार्य सक्षम किया गया।","it":"Job abilitato.","ja":"ジョブが有効になりました。","nl":"Taak ingeschakeld.","no":"Jobb aktivert.","pl":"Zadanie włączone.","pt":"Trabalho ativado.","sv":"Jobb aktiverat.","zh":"任务已启用。"},
    "Knowledge Center": {"cs":"Centrum znalostí","da":"Videnscenter","de":"Wissenszentrum","el":"Κέντρο γνώσεων","es":"Centro de conocimiento","fr":"Centre de connaissances","hi":"ज्ञान केंद्र","it":"Centro conoscenza","ja":"ナレッジセンター","nl":"Kenniscentrum","no":"Kunnskapssenter","pl":"Centrum wiedzy","pt":"Centro de conhecimento","sv":"Kunskapscenter","zh":"知识中心"},
    "Knowledge Graph": {"cs":"Znalostní graf","da":"Vidensgraf","de":"Wissensgraph","el":"Γράφος γνώσεων","es":"Grafo de conocimiento","fr":"Graphe de connaissances","hi":"ज्ञान ग्राफ","it":"Grafo della conoscenza","ja":"ナレッジグラフ","nl":"Kennisgrafiek","no":"Kunnskapsgraf","pl":"Graf wiedzy","pt":"Grafo de conhecimento","sv":"Kunskapsgraf","zh":"知识图谱"},
    "Live Log": {"cs":"Živý log","da":"Live-log","de":"Live-Log","el":"Ζωντανό αρχείο καταγραφής","es":"Registro en vivo","fr":"Journal en direct","hi":"लाइव लॉग","it":"Log live","ja":"ライブログ","nl":"Live-log","no":"Live-logg","pl":"Log na żywo","pt":"Log ao vivo","sv":"Live-logg","zh":"实时日志"},
    "Login": {"cs":"Přihlášení","da":"Login","de":"Anmelden","el":"Σύνδεση","es":"Iniciar sesión","fr":"Connexion","hi":"लॉगिन","it":"Accesso","ja":"ログイン","nl":"Inloggen","no":"Logg inn","pl":"Logowanie","pt":"Login","sv":"Logga in","zh":"登录"},
    "Logout": {"cs":"Odhlášení","da":"Log ud","de":"Abmelden","el":"Αποσύνδεση","es":"Cerrar sesión","fr":"Déconnexion","hi":"लॉगआउट","it":"Esci","ja":"ログアウト","nl":"Uitloggen","no":"Logg ut","pl":"Wyloguj","pt":"Sair","sv":"Logga ut","zh":"登出"},
    "Logs": {"cs":"Logy","da":"Logfiler","de":"Logs","el":"Αρχεία καταγραφής","es":"Registros","fr":"Journaux","hi":"लॉग","it":"Log","ja":"ログ","nl":"Logboeken","no":"Logger","pl":"Logi","pt":"Registros","sv":"Loggar","zh":"日志"},
    "Maintenance": {"cs":"Údržba","da":"Vedligeholdelse","de":"Wartung","el":"Συντήρηση","es":"Mantenimiento","fr":"Maintenance","hi":"रखरखाव","it":"Manutenzione","ja":"メンテナンス","nl":"Onderhoud","no":"Vedlikehold","pl":"Konserwacja","pt":"Manutenção","sv":"Underhåll","zh":"维护"},
    "Memory": {"cs":"Paměť","da":"Hukommelse","de":"Speicher","el":"Μνήμη","es":"Memoria","fr":"Mémoire","hi":"स्मृति","it":"Memoria","ja":"メモリ","nl":"Geheugen","no":"Minne","pl":"Pamięć","pt":"Memória","sv":"Minne","zh":"内存"},
    "Navigation": {"cs":"Navigace","da":"Navigation","de":"Navigation","el":"Πλοήγηση","es":"Navegación","fr":"Navigation","hi":"नेविगेशन","it":"Navigazione","ja":"ナビゲーション","nl":"Navigatie","no":"Navigasjon","pl":"Nawigacja","pt":"Navegação","sv":"Navigering","zh":"导航"},
    "Notifications": {"cs":"Oznámení","da":"Notifikationer","de":"Benachrichtigungen","el":"Ειδοποιήσεις","es":"Notificaciones","fr":"Notifications","hi":"सूचनाएं","it":"Notifiche","ja":"通知","nl":"Meldingen","no":"Varsler","pl":"Powiadomienia","pt":"Notificações","sv":"Aviseringar","zh":"通知"},
    "Permissions": {"cs":"Oprávnění","da":"Tilladelser","de":"Berechtigungen","el":"Δικαιώματα","es":"Permisos","fr":"Permissions","hi":"अनुमतियां","it":"Permessi","ja":"権限","nl":"Rechten","no":"Tillatelser","pl":"Uprawnienia","pt":"Permissões","sv":"Rättigheter","zh":"权限"},
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

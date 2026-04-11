#!/usr/bin/env python3
"""Batch 14: Emoji-prefixed backend messages and high-impact labels."""
import json, os
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "+ Create Snapshot": {"cs":"+ Vytvořit snímek","da":"+ Opret øjebliksbillede","de":"+ Snapshot erstellen","el":"+ Δημιουργία στιγμιότυπου","es":"+ Crear instantánea","fr":"+ Créer un instantané","hi":"+ स्नैपशॉट बनाएं","it":"+ Crea snapshot","ja":"+ スナップショットを作成","nl":"+ Snapshot aanmaken","no":"+ Opprett øyeblikksbilde","pl":"+ Utwórz migawkę","pt":"+ Criar instantâneo","sv":"+ Skapa ögonblicksbild","zh":"+ 创建快照"},
    "+ New Dataset": {"cs":"+ Nový dataset","da":"+ Nyt dataset","de":"+ Neues Dataset","el":"+ Νέο σύνολο δεδομένων","es":"+ Nuevo conjunto de datos","fr":"+ Nouveau jeu de données","hi":"+ नया डेटासेट","it":"+ Nuovo dataset","ja":"+ 新しいデータセット","nl":"+ Nieuwe dataset","no":"+ Nytt datasett","pl":"+ Nowy zestaw danych","pt":"+ Novo conjunto de dados","sv":"+ Ny dataset","zh":"+ 新数据集"},
    "+ New Share": {"cs":"+ Nové sdílení","da":"+ Ny deling","de":"+ Neue Freigabe","el":"+ Νέα κοινή χρήση","es":"+ Nuevo recurso compartido","fr":"+ Nouveau partage","hi":"+ नया शेयर","it":"+ Nuova condivisione","ja":"+ 新しい共有","nl":"+ Nieuwe deling","no":"+ Ny deling","pl":"+ Nowy udział","pt":"+ Novo compartilhamento","sv":"+ Ny delning","zh":"+ 新共享"},
    "+ Field": {"cs":"+ Pole","da":"+ Felt","de":"+ Feld","el":"+ Πεδίο","es":"+ Campo","fr":"+ Champ","hi":"+ फ़ील्ड","it":"+ Campo","ja":"+ フィールド","nl":"+ Veld","no":"+ Felt","pl":"+ Pole","pt":"+ Campo","sv":"+ Fält","zh":"+ 字段"},
    "Allow destructive operations (delete, rollback)": {"cs":"Povolit destruktivní operace (smazání, návrat)","da":"Tillad destruktive handlinger (slet, tilbagefør)","de":"Zerstörerische Operationen erlauben (Löschen, Rollback)","el":"Αποδοχή καταστροφικών λειτουργιών (διαγραφή, επαναφορά)","es":"Permitir operaciones destructivas (eliminar, revertir)","fr":"Autoriser les opérations destructrices (suppression, restauration)","hi":"विनाशकारी संचालन की अनुमति दें (हटाना, रोलबैक)","it":"Consenti operazioni distruttive (elimina, ripristino)","ja":"破壊的操作を許可（削除、ロールバック）","nl":"Destructieve bewerkingen toestaan (verwijderen, terugdraaien)","no":"Tillat destruktive operasjoner (slett, tilbakerull)","pl":"Zezwalaj na operacje niszczące (usuwanie, wycofywanie)","pt":"Permitir operações destrutivas (excluir, reverter)","sv":"Tillåt destruktiva åtgärder (ta bort, återställ)","zh":"允许破坏性操作（删除、回滚）"},
    "Allow guest access": {"cs":"Povolit přístup hostům","da":"Tillad gæsteadgang","de":"Gastzugriff erlauben","el":"Αποδοχή πρόσβασης επισκεπτών","es":"Permitir acceso de invitados","fr":"Autoriser l'accès invité","hi":"अतिथि पहुंच की अनुमति दें","it":"Consenti accesso ospite","ja":"ゲストアクセスを許可","nl":"Gasttoegang toestaan","no":"Tillat gjestetilgang","pl":"Zezwalaj na dostęp gości","pt":"Permitir acesso de convidados","sv":"Tillåt gäståtkomst","zh":"允许访客访问"},
    "Budget": {"cs":"Rozpočet","da":"Budget","de":"Budget","el":"Προϋπολογισμός","es":"Presupuesto","fr":"Budget","hi":"बजट","it":"Budget","ja":"予算","nl":"Budget","no":"Budsjett","pl":"Budżet","pt":"Orçamento","sv":"Budget","zh":"预算"},
    "Budget & Tokens": {"cs":"Rozpočet a tokeny","da":"Budget & Tokens","de":"Budget & Tokens","el":"Προϋπολογισμός & Διακριτικά","es":"Presupuesto y Tokens","fr":"Budget et Jetons","hi":"बजट और टोकन","it":"Budget e Token","ja":"予算とトークン","nl":"Budget & Tokens","no":"Budsjett og Token","pl":"Budżet i Tokeny","pt":"Orçamento e Tokens","sv":"Budget och Token","zh":"预算和令牌"},
    "Budget Shed": {"cs":"Shed rozpočtu","da":"Budget-shed","de":"Budget-Shed","el":"Αποβολή προϋπολογισμού","es":"Shed de presupuesto","fr":"Shed de budget","hi":"बजट शेड","it":"Shed budget","ja":"予算シェッド","nl":"Budget-shed","no":"Budsjettskjul","pl":"Shed budżetu","pt":"Shed de orçamento","sv":"Budget-shed","zh":"预算脱落"},
    "Budget Sheds": {"cs":"Shedy rozpočtu","da":"Budget-sheds","de":"Budget-Sheds","el":"Αποβολές προϋπολογισμού","es":"Sheds de presupuesto","fr":"Sheds de budget","hi":"बजट शेड्स","it":"Shed budget","ja":"予算シェッド","nl":"Budget-sheds","no":"Budsjettskjul","pl":"Shedy budżetu","pt":"Sheds de orçamento","sv":"Budget-sheds","zh":"预算脱落"},
    "Burst": {"cs":"Burst","da":"Burst","de":"Burst","el":"Burst","es":"Ráfaga","fr":"Rafale","hi":"बर्स्ट","it":"Burst","ja":"バースト","nl":"Burst","no":"Burst","pl":"Burst","pt":"Rajada","sv":"Burst","zh":"突发"},
    "Category": {"cs":"Kategorie","da":"Kategori","de":"Kategorie","el":"Κατηγορία","es":"Categoría","fr":"Catégorie","hi":"श्रेणी","it":"Categoria","ja":"カテゴリ","nl":"Categorie","no":"Kategori","pl":"Kategoria","pt":"Categoria","sv":"Kategori","zh":"分类"},
    "Changelog:": {"cs":"Seznam změn:","da":"Ændringslog:","de":"Änderungsprotokoll:","el":"Αρχείο αλλαγών:","es":"Registro de cambios:","fr":"Journal des modifications :","hi":"बदलाव लॉग:","it":"Registro modifiche:","ja":"変更履歴:","nl":"Wijzigingslog:","no":"Endringslogg:","pl":"Dziennik zmian:","pt":"Registro de alterações:","sv":"Ändringslogg:","zh":"更新日志："},
    "Circuit Breaker": {"cs":"Jistič","da":"Circuit Breaker","de":"Circuit Breaker","el":"Ασφαλής διακοπή","es":"Cortacircuitos","fr":"Disjoncteur","hi":"सर्किट ब्रेकर","it":"Circuit Breaker","ja":"サーキットブレーカー","nl":"Circuit Breaker","no":"Circuit Breaker","pl":"Bezpiecznik","pt":"Disjuntor","sv":"Circuit Breaker","zh":"熔断器"},
    "Collection": {"cs":"Kolekce","da":"Samling","de":"Sammlung","el":"Συλλογή","es":"Colección","fr":"Collection","hi":"संग्रह","it":"Collezione","ja":"コレクション","nl":"Collectie","no":"Samling","pl":"Kolekcja","pt":"Coleção","sv":"Samling","zh":"集合"},
    "Collections": {"cs":"Kolekce","da":"Samlinger","de":"Sammlungen","el":"Συλλογές","es":"Colecciones","fr":"Collections","hi":"संग्रह","it":"Collezioni","ja":"コレクション","nl":"Collecties","no":"Samlinger","pl":"Kolekcje","pt":"Coleções","sv":"Samlingar","zh":"集合"},
    "Compression": {"cs":"Komprese","da":"Komprimering","de":"Kompression","el":"Συμπίεση","es":"Compresión","fr":"Compression","hi":"संपीडन","it":"Compressione","ja":"圧縮","nl":"Compressie","no":"Komprimering","pl":"Kompresja","pt":"Compressão","sv":"Komprimering","zh":"压缩"},
    "Conservative: weekly reflection only": {"cs":"Konzervativní: pouze týdenní reflexe","da":"Konservativ: kun ugentlig refleksion","de":"Konservativ: nur wöchentliche Reflexion","el":"Συντηρητικό: μόνο εβδομαδιαία αναστοχασμός","es":"Conservador: solo reflexión semanal","fr":"Conservateur : réflexion hebdomadaire uniquement","hi":"रूढ़िवादी: केवल साप्ताहिक प्रतिबिंब","it":"Conservativo: solo riflessione settimanale","ja":"控えめ：週次リフレクションのみ","nl":"Conservatief: alleen wekelijkse reflectie","no":"Konservativ: kun ukentlig refleksjon","pl":"Konserwatywny: tylko tygodniowa refleksja","pt":"Conservador: apenas reflexão semanal","sv":"Konservativ: endast veckovis reflektion","zh":"保守：仅每周反思"},
    "Custom / Other": {"cs":"Vlastní / Jiné","da":"Brugerdefineret / Andet","de":"Benutzerdefiniert / Andere","el":"Προσαρμοσμένο / Άλλο","es":"Personalizado / Otro","fr":"Personnalisé / Autre","hi":"कस्टम / अन्य","it":"Personalizzato / Altro","ja":"カスタム / その他","nl":"Aangepast / Overig","no":"Tilpasset / Annet","pl":"Niestandardowy / Inny","pt":"Personalizado / Outro","sv":"Anpassat / Annat","zh":"自定义 / 其他"},
    "Daemon: Error": {"cs":"Démon: Chyba","da":"Daemon: Fejl","de":"Daemon: Fehler","el":"Δαίμονας: Σφάλμα","es":"Demonio: Error","fr":"Démon : Erreur","hi":"डेमन: त्रुटि","it":"Demone: Errore","ja":"デーモン: エラー","nl":"Daemon: Fout","no":"Daemon: Feil","pl":"Demon: Błąd","pt":"Daemon: Erro","sv":"Daemon: Fel","zh":"守护进程：错误"},
    "Daemons": {"cs":"Démoni","da":"Daemons","de":"Daemons","el":"Δαίμονες","es":"Demonios","fr":"Démons","hi":"डेमन","it":"Demoni","ja":"デーモン","nl":"Daemons","no":"Daemoner","pl":"Demony","pt":"Daemons","sv":"Daemoner","zh":"守护进程"},
    "Dependencies": {"cs":"Závislosti","da":"Afhængigheder","de":"Abhängigkeiten","el":"Εξαρτήσεις","es":"Dependencias","fr":"Dépendances","hi":"निर्भरताएं","it":"Dipendenze","ja":"依存関係","nl":"Afhankelijkheden","no":"Avhengigheter","pl":"Zależności","pt":"Dependências","sv":"Beroenden","zh":"依赖"},
    "Designer": {"cs":"Návrhář","da":"Designer","de":"Designer","el":"Σχεδιαστής","es":"Diseñador","fr":"Concepteur","hi":"डिज़ाइनर","it":"Designer","ja":"デザイナー","nl":"Ontwerper","no":"Designer","pl":"Projektant","pt":"Designer","sv":"Designer","zh":"设计器"},
    "Details": {"cs":"Detaily","da":"Detaljer","de":"Details","el":"Λεπτομέρειες","es":"Detalles","fr":"Détails","hi":"विवरण","it":"Dettagli","ja":"詳細","nl":"Details","no":"Detaljer","pl":"Szczegóły","pt":"Detalhes","sv":"Detaljer","zh":"详情"},
    "Direct": {"cs":"Přímý","da":"Direkte","de":"Direkt","el":"Άμεσο","es":"Directo","fr":"Direct","hi":"प्रत्यक्ष","it":"Diretto","ja":"ダイレクト","nl":"Direct","no":"Direkte","pl":"Bezpośredni","pt":"Direto","sv":"Direkt","zh":"直接"},
    "Disk": {"cs":"Disk","da":"Disk","de":"Festplatte","el":"Δίσκος","es":"Disco","fr":"Disque","hi":"डिस्क","it":"Disco","ja":"ディスク","nl":"Schijf","no":"Disk","pl":"Dysk","pt":"Disco","sv":"Disk","zh":"磁盘"},
    "Database": {"cs":"Databáze","da":"Database","de":"Datenbank","el":"Βάση δεδομένων","es":"Base de datos","fr":"Base de données","hi":"डेटाबेस","it":"Database","ja":"データベース","nl":"Database","no":"Database","pl":"Baza danych","pt":"Banco de dados","sv":"Databas","zh":"数据库"},
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

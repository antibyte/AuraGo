#!/usr/bin/env python3
"""Batch 7: More UI labels, config sections, and common terms."""
import json, os
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
LANGS = ["cs","da","de","el","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]

T = {
    "Error creating": {"cs":"Chyba při vytváření","da":"Fejl ved oprettelse","de":"Fehler beim Erstellen","el":"Σφάλμα δημιουργίας","es":"Error al crear","fr":"Erreur lors de la création","hi":"बनाते समय त्रुटि","it":"Errore nella creazione","ja":"作成エラー","nl":"Fout bij aanmaken","no":"Feil ved opprettelse","pl":"Błąd tworzenia","pt":"Erro ao criar","sv":"Fel vid skapande","zh":"创建时出错"},
    "Executing Python script...": {"cs":"Spouští se Python skript...","da":"Kører Python-script...","de":"Python-Skript wird ausgeführt...","el":"Εκτέλεση σεναρίου Python...","es":"Ejecutando script Python...","fr":"Exécution du script Python...","hi":"Python स्क्रिप्ट निष्पादित हो रही है...","it":"Esecuzione script Python...","ja":"Pythonスクリプトを実行中...","nl":"Python-script uitvoeren...","no":"Kjører Python-skript...","pl":"Wykonywanie skryptu Python...","pt":"Executando script Python...","sv":"Kör Python-skript...","zh":"正在执行 Python 脚本..."},
    "Generate Skill Draft": {"cs":"Vygenerovat návrh dovednosti","da":"Generér færdighedsudkast","de":"Skill-Entwurf generieren","el":"Δημιουργία προχείρου δεξιότητας","es":"Generar borrador de habilidad","fr":"Générer un brouillon de compétence","hi":"कौशल ड्राफ्ट उत्पन्न करें","it":"Genera bozza skill","ja":"スキルドラフトを生成","nl":"Vaardigheidsconcept genereren","no":"Generer ferdighetsutkast","pl":"Generuj szkic umiejętności","pt":"Gerar rascunho de habilidade","sv":"Generera färdighetsutkast","zh":"生成技能草稿"},
    "Generate Token": {"cs":"Vygenerovat token","da":"Generér token","de":"Token generieren","el":"Δημιουργία διακριτικού","es":"Generar token","fr":"Générer un jeton","hi":"टोकन उत्पन्न करें","it":"Genera token","ja":"トークンを生成","nl":"Token genereren","no":"Generer token","pl":"Generuj token","pt":"Gerar token","sv":"Generera token","zh":"生成令牌"},
    "Refresh": {"cs":"Obnovit","da":"Opdater","de":"Aktualisieren","el":"Ανανέωση","es":"Actualizar","fr":"Actualiser","hi":"रीफ़्रेश करें","it":"Aggiorna","ja":"更新","nl":"Vernieuwen","no":"Oppdater","pl":"Odśwież","pt":"Atualizar","sv":"Uppdatera","zh":"刷新"},
    "Settings": {"cs":"Nastavení","da":"Indstillinger","de":"Einstellungen","el":"Ρυθμίσεις","es":"Configuración","fr":"Paramètres","hi":"सेटिंग्स","it":"Impostazioni","ja":"設定","nl":"Instellingen","no":"Innstillinger","pl":"Ustawienia","pt":"Configurações","sv":"Inställningar","zh":"设置"},
    "Download": {"cs":"Stáhnout","da":"Download","de":"Herunterladen","el":"Λήψη","es":"Descargar","fr":"Télécharger","hi":"डाउनलोड करें","it":"Scarica","ja":"ダウンロード","nl":"Downloaden","no":"Last ned","pl":"Pobierz","pt":"Baixar","sv":"Ladda ner","zh":"下载"},
    "Test": {"cs":"Test","da":"Test","de":"Testen","el":"Δοκιμή","es":"Probar","fr":"Tester","hi":"परीक्षण","it":"Test","ja":"テスト","nl":"Testen","no":"Test","pl":"Testuj","pt":"Testar","sv":"Testa","zh":"测试"},
    "Test failed": {"cs":"Test selhal","da":"Test mislykkedes","de":"Test fehlgeschlagen","el":"Η δοκιμή απέτυχε","es":"Prueba fallida","fr":"Test échoué","hi":"परीक्षण विफल","it":"Test non riuscito","ja":"テスト失敗","nl":"Test mislukt","no":"Test mislyktes","pl":"Test nie powiódł się","pt":"Teste falhou","sv":"Test misslyckades","zh":"测试失败"},
    "Test finished": {"cs":"Test dokončen","da":"Test afsluttet","de":"Test abgeschlossen","el":"Η δοκιμή ολοκληρώθηκε","es":"Prueba finalizada","fr":"Test terminé","hi":"परीक्षण पूरा","it":"Test completato","ja":"テスト完了","nl":"Test voltooid","no":"Test fullført","pl":"Test zakończony","pt":"Teste concluído","sv":"Test slutfört","zh":"测试完成"},
    "Run Test": {"cs":"Spustit test","da":"Kør test","de":"Test ausführen","el":"Εκτέλεση δοκιμής","es":"Ejecutar prueba","fr":"Exécuter le test","hi":"टेस्ट चलाएं","it":"Esegui test","ja":"テストを実行","nl":"Test uitvoeren","no":"Kjør test","pl":"Uruchom test","pt":"Executar teste","sv":"Kör test","zh":"运行测试"},
    "Storage": {"cs":"Úložiště","da":"Lagring","de":"Speicher","el":"Αποθήκευση","es":"Almacenamiento","fr":"Stockage","hi":"भंडारण","it":"Archiviazione","ja":"ストレージ","nl":"Opslag","no":"Lagring","pl":"Pamięć masowa","pt":"Armazenamento","sv":"Lagring","zh":"存储"},
    "Size": {"cs":"Velikost","da":"Størrelse","de":"Größe","el":"Μέγεθος","es":"Tamaño","fr":"Taille","hi":"आकार","it":"Dimensione","ja":"サイズ","nl":"Grootte","no":"Størrelse","pl":"Rozmiar","pt":"Tamanho","sv":"Storlek","zh":"大小"},
    "Status": {"cs":"Stav","da":"Status","de":"Status","el":"Κατάσταση","es":"Estado","fr":"Statut","hi":"स्थिति","it":"Stato","ja":"ステータス","nl":"Status","no":"Status","pl":"Status","pt":"Status","sv":"Status","zh":"状态"},
    "Version": {"cs":"Verze","da":"Version","de":"Version","el":"Έκδοση","es":"Versión","fr":"Version","hi":"संस्करण","it":"Versione","ja":"バージョン","nl":"Versie","no":"Versjon","pl":"Wersja","pt":"Versão","sv":"Version","zh":"版本"},
    "Error": {"cs":"Chyba","da":"Fejl","de":"Fehler","el":"Σφάλμα","es":"Error","fr":"Erreur","hi":"त्रुटि","it":"Errore","ja":"エラー","nl":"Fout","no":"Feil","pl":"Błąd","pt":"Erro","sv":"Fel","zh":"错误"},
    "Name is required": {"cs":"Název je povinný","da":"Navn er påkrævet","de":"Name ist erforderlich","el":"Το όνομα είναι υποχρεωτικό","es":"El nombre es obligatorio","fr":"Le nom est requis","hi":"नाम आवश्यक है","it":"Il nome è obbligatorio","ja":"名前は必須です","nl":"Naam is vereist","no":"Navn er påkrevd","pl":"Nazwa jest wymagana","pt":"O nome é obrigatório","sv":"Namn är obligatoriskt","zh":"名称为必填项"},
    "Active": {"cs":"Aktivní","da":"Aktiv","de":"Aktiv","el":"Ενεργό","es":"Activo","fr":"Actif","hi":"सक्रिय","it":"Attivo","ja":"アクティブ","nl":"Actief","no":"Aktiv","pl":"Aktywny","pt":"Ativo","sv":"Aktiv","zh":"活动"},
    "Inactive": {"cs":"Neaktivní","da":"Inaktiv","de":"Inaktiv","el":"Ανενεργό","es":"Inactivo","fr":"Inactif","hi":"निष्क्रिय","it":"Inattivo","ja":"非アクティブ","nl":"Inactief","no":"Inaktiv","pl":"Nieaktywny","pt":"Inativo","sv":"Inaktiv","zh":"不活动"},
    "Disabled": {"cs":"Zakázáno","da":"Deaktiveret","de":"Deaktiviert","el":"Απενεργοποιημένο","es":"Desactivado","fr":"Désactivé","hi":"अक्षम","it":"Disabilitato","ja":"無効","nl":"Uitgeschakeld","no":"Deaktivert","pl":"Wyłączone","pt":"Desativado","sv":"Inaktiverat","zh":"已禁用"},
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

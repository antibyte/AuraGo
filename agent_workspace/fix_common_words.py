import json
from pathlib import Path

# Dictionary of translations for common UI words
TRANSLATIONS = {
    'Cancel': {
        'fr': 'Annuler',
        'es': 'Cancelar',
        'it': 'Annulla',
        'pl': 'Anuluj',
        'cs': 'Zrušit',
        'nl': 'Annuleren',
        'sv': 'Avbryt',
        'no': 'Avbryt',
        'da': 'Annuller',
        'pt': 'Cancelar',
        'el': 'Ακύρωση',
        'hi': 'रद्द करें',
        'ja': 'キャンセル',
        'zh': '取消',
    },
    'Save': {
        'fr': 'Enregistrer',
        'es': 'Guardar',
        'it': 'Salva',
        'pl': 'Zapisz',
        'cs': 'Uložit',
        'nl': 'Opslaan',
        'sv': 'Spara',
        'no': 'Lagre',
        'da': 'Gem',
        'pt': 'Guardar',
        'el': 'Αποθήκευση',
        'hi': 'सहेजें',
        'ja': '保存',
        'zh': '保存',
    },
    'Delete': {
        'fr': 'Supprimer',
        'es': 'Eliminar',
        'it': 'Elimina',
        'pl': 'Usuń',
        'cs': 'Smazat',
        'nl': 'Verwijderen',
        'sv': 'Ta bort',
        'no': 'Slett',
        'da': 'Slet',
        'pt': 'Eliminar',
        'el': 'Διαγραφή',
        'hi': 'हटाएं',
        'ja': '削除',
        'zh': '删除',
    },
    'Error': {
        'fr': 'Erreur',
        'es': 'Error',
        'it': 'Errore',
        'pl': 'Błąd',
        'cs': 'Chyba',
        'nl': 'Fout',
        'sv': 'Fel',
        'no': 'Feil',
        'da': 'Fejl',
        'pt': 'Erro',
        'el': 'Σφάλμα',
        'hi': 'त्रुटि',
        'ja': 'エラー',
        'zh': '错误',
    },
    'Enabled': {
        'fr': 'Activé',
        'es': 'Activado',
        'it': 'Attivo',
        'pl': 'Włączone',
        'cs': 'Aktivní',
        'nl': 'Ingeschakeld',
        'sv': 'Aktiverad',
        'no': 'Aktivert',
        'da': 'Aktiveret',
        'pt': 'Ativado',
        'el': 'Ενεργοποιημένο',
        'hi': 'सक्रिय',
        'ja': '有効',
        'zh': '已启用',
    },
    'Loading': {
        'fr': 'Chargement…',
        'es': 'Cargando…',
        'it': 'Caricamento…',
        'pl': 'Ładowanie…',
        'cs': 'Načítání…',
        'nl': 'Laden…',
        'sv': 'Laddar…',
        'no': 'Laster…',
        'da': 'Indlæser…',
        'pt': 'A carregar…',
        'el': 'Φόρτωση…',
        'hi': 'लोड हो रहा है…',
        'ja': '読み込み中…',
        'zh': '加载中…',
    },
    'Disabled': {
        'fr': 'Désactivé',
        'es': 'Desactivado',
        'it': 'Disattivato',
        'pl': 'Wyłączone',
        'cs': 'Deaktivováno',
        'nl': 'Uitgeschakeld',
        'sv': 'Inaktiverad',
        'no': 'Deaktivert',
        'da': 'Deaktiveret',
        'pt': 'Desativado',
        'el': 'Απενεργοποιημένο',
        'hi': 'निष्क्रिय',
        'ja': '無効',
        'zh': '已禁用',
    },
}

fixed_count = 0

for base in [Path('ui/lang/config'), Path('ui/lang/dashboard')]:
    for path in base.rglob('*.json'):
        lang = path.stem
        if lang == 'en' or lang == 'de':
            continue  # skip English and German (German already handled)
        
        try:
            with open(path, 'r', encoding='utf-8-sig') as f:
                data = json.load(f)
        except:
            continue
        
        changed = False
        for k, v in data.items():
            if not isinstance(v, str):
                continue
            stripped = v.strip()
            for word, trans_dict in TRANSLATIONS.items():
                if stripped == word and lang in trans_dict:
                    data[k] = trans_dict[lang]
                    changed = True
                    fixed_count += 1
        
        if changed:
            with open(path, 'w', encoding='utf-8') as f:
                json.dump(data, f, ensure_ascii=False, indent=2)
                f.write('\n')

print(f"Fixed {fixed_count} untranslated common words across all language files.")

#!/usr/bin/env python3
"""
Fix Contamination Script - Replaces Czech text in contaminated language files with English originals

This script identifies and fixes cross-language contamination where Czech text
appears in files for other languages (Greek, Dutch, Swedish, Portuguese, Danish, Norwegian).
"""

import json
import os
import re
from pathlib import Path
from typing import Dict, Set

# Get the project root directory (parent of disposable/)
SCRIPT_DIR = Path(__file__).parent.resolve()
PROJECT_ROOT = SCRIPT_DIR.parent
LANG_DIR = PROJECT_ROOT / 'ui' / 'lang'

# Czech text patterns that indicate contamination
CZECH_PATTERNS = [
    r'Uzivatelske jmeno pro',
    r'Token pro',
    r'Vychozi hodnota pro',
    r'Pouze pro cteni pro',
    r'Limit pro',
    r'Port pro',
    r'URL pro',
    r'ID pro',
    r'Adresa pro',
    r'Heslo pro',
    r'Mod pro',
    r'Poskytovatel pro',
    r'Povoluje ',
    r'Casovy limit pro',
    r'Interval pro',
    r'Rozpocet',
    r'Vynuceni pro',
    r'Prahova hodnota',
    r'Hostitel pro',
    r'Slozka pro',
    r'Kdyz ',
    r'Zobrazuje ',
    r'Zpozdeni ',
    r'Pouze pro cteni',
    r'Povoluje Docker spravu',
    r'Povoluje Egg Mode',
    r'Povoluje email',
    r'Discord',
    r'Ansible',
    r'Limit pro budget',
    r'Restart se nepodařilo',
    r'Port SMTP serveru',
    r'Automaticky nainstalovat',
    r'Λίστα θεμάτων με κόμμα',  # Greek with Czech contamination
    r'Κωδικός πρόσβασης για αυθεντικοποίηση',  # Greek with Czech mixed
]

# Languages that are contaminated and need fixing
CONTAMINATED_LANGS = ['el', 'nl', 'sv', 'pt', 'da', 'no']

# Czech language code
CZECH_LANG = 'cs'

def load_json(filepath: Path) -> Dict:
    """Load JSON file safely."""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            return json.load(f)
    except (json.JSONDecodeError, FileNotFoundError) as e:
        print(f"  [ERROR] {filepath}: {e}")
        return {}

def save_json(filepath: Path, data: Dict) -> bool:
    """Save JSON file safely."""
    try:
        with open(filepath, 'w', encoding='utf-8') as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
        return True
    except Exception as e:
        print(f"  [ERROR] Failed to save {filepath}: {e}")
        return False

def is_czech_text(text: str) -> bool:
    """Check if text contains Czech contamination patterns."""
    if not text or not isinstance(text, str):
        return False
    
    # Check for Czech-specific words/phrases
    czech_words = [
        # Common Czech phrases
        'Uzivatelske jmeno pro', 'Uzivatelske jmeno', 'Uzivatelskeho jmena',
        'Token pro', 'Tokenu pro',
        'Vychozi hodnota pro', 'Vychozi hodnota', 'Vychoziho nastaveni',
        'Pouze pro cteni', 'Pouze cteni',
        'Limit pro', 'Limitu pro', 'Limit',
        'Port pro', 'Portu pro', 'Port ',
        'URL pro', 'URL adresa',
        'ID pro', 'ID ',
        'Adresa pro', 'Adresa ',
        'Heslo pro', 'Hesla pro', 'Heslo',
        'Mod pro', 'Modu pro', 'Mod ',
        'Poskytovatel pro', 'Poskytovatele pro',
        'Povoluje ', 'Povoluje',
        'Casovy limit', 'Casoveho limitu',
        'Interval pro', 'Intervalu pro',
        'Rozpocet', 'Rozpoctove',
        'Vynuceni', 'Vynuceni pro',
        'Prahova hodnota', 'Prahove hodnoty',
        'Hostitel pro', 'Hostitele pro', 'Hostitel ',
        'Slozka pro', 'Slozky pro',
        'Kdyz ', 'Kdyz',
        'Zobrazuje ', 'Zobrazuje',
        'Zpozdeni ', 'Zpozdeni',
        'Adresar pro', 'Adresare pro',
        'Povoluje Docker', 'Povoluje Egg', 'Povoluje email',
        'Povoluje Co-Agenti', 'Povoluje Invasion',
        'Povoluje Koofr', 'Povoluje Ollama', 'Povoluje Rocket',
        'Povoluje Tailscale', 'Povoluje prichozi',
        'Povoluje Chromecast', 'Povoluje GitHub', 'Povoluje MeshCentral',
        'Povoluje Brave', 'Povoluje Tailscale', 'Povoluje Fritzbox',
        'Limit pro webhooks', 'Limit pro telegram',
        'Limit pro budget', 'Limit pro co_agents', 'Limit pro circuit_breaker',
        'Casovy limit pro agent', 'Casovy limit pro ansible',
        'Casovy limit pro circuit_breaker',
        'Hlavni LLM poskytovatel', 'LLM poskytovatel',
        'Poskytovatel pro co_agents', 'Poskytovatel pro fallback_llm',
        'Meritko pro tailscale', 'Meritka pro',
        'Vek pro mqtt', 'Vektor pro',
        # Czech words that appear in text
        'Debug mod', 'Debug rezim',
        'Pracovni prostor', 'Pracovni prostor pro',
        'Velikost kontextu', 'Velikost',
        'Limit znaku', 'Limit znaku pro',
        'Jazyk odpovedi', 'Jazyk ',
        'Dodatecny', 'Dodatecne', 'Dodatku',
        'Zakazano', 'Povoleno',
        'Komprese pameti', 'Komprese',
        'Automaticky', 'Automaticka',
        'Spravu', 'Sprava',
        'Nastroju', 'Nastroje',
        'Volanimi', 'Volani',
        'Cteni', 'Cteni pro',
        'Zapis', 'Zapis do',
        'Bezpecne', 'Bezpecnost',
        'Udaje', 'Udu',
        'Zaclenovat', 'Zaclenovani',
        'Moduly', 'Modul',
        'Uzivatelem', 'Uzivatele',
        'Spolecne', 'Spolecnosti',
        'Zarizeni', 'Zarizeni ',
        'Pripojeni', 'Pripojit',
        'Nastaveni', 'Nastaveni ',
        'Vychozi', 'Vychoziho',
        'System', 'Systemu',
        'Konfigurace', 'Konfiguracni',
        'Parametr', 'Parametry',
        'Sluzba', 'Sluzby',
        'Aktualizace', 'Aktualizaci',
        'Zabezpeceni', 'Zabezpeceni ',
        'Prihlaseni', 'Prihlasovac',
        'Certifikat', 'Certifikaty',
        'Pripojeni', 'Pripojeni ',
        # More Czech words
        'Chyba', 'Chyby', 'Chyb',
        'Neplatny', 'Neplatna', 'Neplatne',
        'Prazdny', 'Prazdna', 'Prazdne',
        'Nepodarilo', 'Nepodarilo se',
        'Ocekavano', 'Ocekavana',
        'Povinna', 'Povinny', 'Povinnne',
        'Platnost', 'Platnosti',
        'Zrusit', 'Zruseni',
        'Ulozit', 'Ulozeni',
        'Smazat', 'Smazani',
        'Vytvorit', 'Vytvoreni',
        'Upravit', 'Upraveni',
        'Zobrazit', 'Zobrazeni',
        'Skryt', 'Skryti',
        'Povolit', 'Povoleni',
        'Zakazat', 'Zakazani',
        'Obnovit', 'Obnoveni',
        'Upozorneni', 'Upozorneni na',
        'Informace', 'Informace o',
        'Popis', 'Popisu',
        'Poznamka', 'Poznamky',
        'Pokyn', 'Pokyny',
        'Rada', 'Rady',
        'Doporuceni', 'Doporuceni pro',
        'Priklad', 'Priklady',
        'Nutny', 'Nutna', 'Nutne',
        'Mozny', 'Mozna', 'Mozne',
        'Potrebny', 'Potrebna', 'Potrebne',
        'Dostupny', 'Dostupna', 'Dostupne',
        'Aktivni', 'Aktivni ', 'Aktivniho',
        'Neaktivni', 'Neaktivni ',
        'Automaticky', 'Automaticka', 'Automaticke',
        'Manuelni', 'Manuelni ', 'Manuelniho',
        'Primarne', 'Primarni',
        'Sekundarne', 'Sekundarni',
        ' Zakaznik', 'Zakaznika', 'Zakaznikum',
        'Uzivatel', 'uzivatele', 'uzivatelu', 'uzivatelem',
        'uzivatelske', 'uzivatelsky', 'uzivatelskeho',
        'heslo', 'hesla', 'heslem', 'heslu',
        'prihlaseni', 'prihlasovac', 'prihlaseni ', 'prihlasovac ',
        'Dalsi', 'dalsi', 'Dalsiho', 'dalsiho',
        'minuly', 'minula', 'minule', 'minulych',
        'aktualni', 'aktualni ', 'aktualniho', 'aktualnimi',
        'cely', 'cela', 'cele', 'celych',
        'pouze', 'pouze ', 'pouzeho',
        'vice', 'vice ', 'viceho',
        'mene', 'mene ', 'meneho',
        'vetsi', 'vetsi ', 'vetsiho',
        'mensi', 'mensi ', 'mensiho',
        'rychly', 'rychla', 'rychle', 'rychlych',
        'pomaly', 'pomalala', 'pomale', 'pomalch',
        'jednoduchy', 'jednoducha', 'jednoduche', 'jednoduchych',
        'slozity', 'slozita', 'slozite', 'slozitych',
        'dlouhy', 'dlouha', 'dlouhe', 'dlouhych',
        'kratky', 'kratsi', 'kratke', 'kratsich',
        'bezpecny', 'bezpecna', 'bezpecne', 'bezpecnych',
        'nebezpecny', 'nebezpecna', 'nebezpecne', 'nebezpecnych',
        'standardni', 'standardni ', 'standardniho',
        'pokrocily', 'pokrocila', 'pokrocile', 'pokrocilych',
        'zakladni', 'zakladni ', 'zakladniho',
        'rozsireny', 'rozsirena', 'rozsirene', 'rozsirenych',
        'optimalni', 'optimalni ', 'optimalniho',
        'doporuceny', 'doporucena', 'doporucene', 'doporucenych',
        'konkretni', 'konkretni ', 'konkretniho',
        'specificky', 'specificka', 'specificke', 'specifickych',
        'univerzalni', 'univerzalni ', 'univerzalniho',
        'docasny', 'docasna', 'docasne', 'docasnych',
        'trvaly', 'trvala', 'trvale', 'trvalych',
        'nahodny', 'nahodna', 'nahodne', 'nahodnych',
        'pravidelny', 'pravidelna', 'pravidelne', 'pravidelnych',
        'mesicni', 'mesicni ', 'mesicniho',
        'rocni', 'rocni ', 'rocniho',
        'denni', 'denni ', 'denniho',
        'hodinovy', 'hodinova', 'hodinove', 'hodinovych',
        'minutovy', 'minutova', 'minutove', 'minutovych',
        'sekundovy', 'sekundova', 'sekundove', 'sekundovych',
        # Czech grammar patterns
        'pro agent', 'pro system', 'pro uzivatele',
        'na server', 'na disku', 'na siti',
        'souboru', 'soubory', 'soubor',
        'uzlu', 'uzel', 'uzly',
        'kontejner', 'kontejneru', 'kontejnery',
        'sluzeb', 'sluzby', 'sluzba',
        'integraci', 'integrace', 'integrace ',
        'spravu', 'spravy', 'sprava',
        'ucunek', 'ucinku', 'ucinky',
        'rezim', 'rezimu', 'rezimy',
        'modul', 'modulu', 'moduly',
        'funkce', 'funkci', 'funkci ',
        'polozek', 'polozky', 'polozka',
        'položek', 'položky', 'položka',
        'kroku', 'kroky', 'krok',
        'akce', 'akci', 'akci ',
        'operace', 'operaci', 'operaci ',
        'stav', 'stavu', 'stavy',
        'info', 'informace', 'informaci',
        'zprava', 'zpravy', 'zpravu',
        'hodnota', 'hodnoty', 'hodnotu',
        'klic', 'klice', 'klicem',
        'token', 'tokenu', 'tokeny',
        'api', 'api ', 'api_',
        'url adresa', 'url ', 'url:',
        'id ', 'id uzivatele', 'id uzivatelske',
        # Czech words with diacritics
        'ř', 'ě', 'š', 'č', 'ž', 'ň', 'ť', 'ď',
        'ů', 'ý',
        # Complete phrases
        'Dodatecny text',
        'Velikost kontextu',
        'Limit znaku pro kompresi',
        'Pracovni prostor pro agent',
        'Debug mod',
        'Jazyk odpovedi',
        'Zobrazuje vysledky nastroju',
        'Zpozdeni mezi volanimi',
        'Bezpecne ulozeno',
        'Automaticky nainstalovat',
        'Automaticke zabezpeceni',
        'Sprava kontejneru',
        'Sprava uzivatelu',
        'Sprava prav',
        'Sprava sitti',
        'Nastaveni sítě',
        'Konfigurace systemu',
        'Parametry pripojeni',
        'Zabezpeceni spojeni',
        'Stav pripojeni',
        'Informace o systemu',
        'Upozorneni na chyby',
        'Konzolova sprava',
        'Webova sprava',
        'Rozhrani pro spravu',
        'Uzivatelske rozhrani',
        'Konfiguracni soubor',
        ' Nastaveni aplikace',
        'Povolit funkci',
        'Zakazat funkci',
        'Automaticky mode',
        'Manuelni nastaveni',
        'Vychozi nastaveni',
        'Uzivatelske nastaveni',
        'Systemove nastaveni',
        'Pocatecni nastaveni',
        'Pokrocile nastaveni',
        'Rychle nastaveni',
        'Globalni nastaveni',
        'Lokalni nastaveni',
        'Sitova nastaveni',
        'Bezpecnostni nastaveni',
        'Doporucena nastaveni',
        'Vlastni nastaveni',
        'Vychozi hodnota',
        'Aktualni hodnota',
        'Posledni hodnota',
        'Nova hodnota',
        'Stara hodnota',
        'Prazdna hodnota',
        'Platna hodnota',
        'Neplatna hodnota',
        'Minimalni hodnota',
        'Maximalni hodnota',
        'Prumerna hodnota',
        'Celkova hodnota',
        'Koncove hodnoty',
        'Uplna hodnota',
        'Zkracena hodnota',
        'Rozsirena hodnota',
        'Kompletni hodnota',
        'Casticna hodnota',
        'Dulezita hodnota',
        'Hlavni hodnota',
        'Vedlejsi hodnota',
        'Dalsi hodnota',
        'Jina hodnota',
        'Stejna hodnota',
        'Ruzna hodnota',
        'Vsechny hodnoty',
        'Nektere hodnoty',
        'Pouze hodnoty',
        'Bez hodnoty',
        'S hodnotou',
        'Bez hodnotou',
        'Hodnota KEY',
        'KEY hodnota',
        ' PRO',
        'PRO ',
        'PRO uzivatele',
        'PRO system',
        'PRO sit',
        'PRO uzivatel',
        'na PRO',
        'v PRO',
        's PRO',
        'z PRO',
        'do PRO',
        'z PRO',
        'k PRO',
        'ke PRO',
        'za PRO',
        'pri PRO',
        'po PRO',
        'proti PRO',
        'mezi PRO',
        'ob冲天PRO',
        'za PRO',
        'na PRO',
        'Pred PRO',
        'Za PRO',
        'Mezi PRO',
        'Pod PRO',
        'Nad PRO',
        'Pred PRO',
        'Za PRO',
        'Pro PRO',
        'pro PRO',
        # Czech complete sentences
        'Uzivatelske jmeno pro discord',
        'Token pro discord',
        'Vychozi hodnota pro discord',
        'Pouze pro cteni pro discord',
        'Limit pro discord',
        'Port pro email',
        'URL pro email',
        'Heslo pro email',
        'Automaticke zalohovani',
        'Automaticke aktualizace',
        'Automaticke pripojeni',
        'Automaticke odpojeni',
        'Automaticke nastaveni',
        'Automaticka konfigurace',
        'Automaticky restart',
        'Automaticke ukonceni',
        'Automaticky start',
        'Automaticky stop',
        'Automaticka sprava',
        'Automaticke rizeni',
        'Automaticka optimalizace',
        'Automaticke ladeni',
        'Automaticka detekce',
        'Automaticke rozpoznani',
        'Automaticka identifikace',
        'Automaticka autentizace',
        'Automaticka autorizace',
        'Automaticka validace',
        'Automaticka verifikace',
        'Automaticka komprese',
        'Automaticka dekomprese',
        'Automaticke sifrovani',
        'Automaticke desifrovani',
        'Automaticka synchronizace',
        'Automaticke zalohovani',
        'Automaticka archivace',
        'Automaticke cisteni',
        'Automaticka udrzba',
        'Automaticka oprava',
        'Automaticke obnoveni',
        'Automaticka regenerace',
        'Automaticka reinstalace',
        'Automaticka aktualizace',
        'Automaticka instalace',
        'Automaticke odinstalovani',
        'Automaticka aktivace',
        'Automaticka deaktivace',
        'Automaticke povoleni',
        'Automaticke zakazani',
        'Automaticka konfigurace',
        'Automaticke nastaveni',
        'Automaticka inicializace',
        'Automaticke spusteni',
        'Automaticke zastaveni',
        'Automaticky beh',
        'Automaticka provoz',
        'Automaticky server',
        'Automaticka sluzba',
        'Automaticka funkce',
        'Automaticka operace',
        'Automaticka akce',
        'Automaticky proces',
        'Automaticky system',
        'Automaticka sprava',
        'Automaticke site',
        'Automaticka sit',
        'Automaticke pripojeni',
        'Automaticka sitova',
        'Automaticky sitovy',
        'Automaticka konfigurace site',
        'Automaticka sprava site',
        'Automaticka bezpecnost site',
        'Automaticka adresace site',
        'Automaticka routovani site',
        'Automaticka brana site',
        'Automaticka firewall site',
        'Automaticka VPN site',
        'Automaticka DNS site',
        'Automaticka DHCP site',
        'Automaticka HTTP site',
        'Automaticka HTTPS site',
        'Automaticka FTP site',
        'Automaticka SSH site',
        'Automaticka TELNET site',
        'Automaticka SMTP site',
        'Automaticka POP3 site',
        'Automaticka IMAP site',
        'Automaticka LDAP site',
        'Automaticka SMB site',
        'Automaticka NFS site',
        'Automaticka SAMBA site',
        'Automaticka MYSQL site',
        'Automaticka POSTGRESQL site',
        'Automaticka MONGODB site',
        'Automaticka REDIS site',
        'Automaticka ELASTICSEARCH site',
        'Automaticka KIBANA site',
        'Automaticka LOGSTASH site',
        'Automaticka PROMETHEUS site',
        'Automaticka GRAFANA site',
        'Automaticka DOCKER site',
        'Automaticka KUBERNETES site',
        'Automaticka HADOOP site',
        'Automaticka SPARK site',
        'Automaticka KAFKA site',
        'Automaticka RABBITMQ site',
        'Automaticka ACTIVEMQ site',
        'Automaticka ZERO MQ site',
        'Automaticka WEBSOCKET site',
        'Automaticka GRPC site',
        'Automaticka REST site',
        'Automaticka API site',
        'Automaticka MICROSERVICE site',
        'Automaticka MONOLITH site',
        'Automaticka SERVERLESS site',
        'Automaticka FAAS site',
        'Automaticka PAAS site',
        'Automaticka SAAS site',
        'Automaticka IAAS site',
        'Automaticka CLOUD site',
        'Automaticka EDGE site',
        'Automaticka IOT site',
        'Automaticka AI site',
        'Automaticka ML site',
        'Automaticka DL site',
        'Automaticka NLP site',
        'Automaticka CV site',
        'Automaticka ROBOTICS site',
        'Automaticka自动驾驶 site',
        'Automaticka CHATGPT site',
        'Automaticka LLM site',
        'Automaticka OPENAI site',
        'Automaticka ANTHROPIC site',
        'Automaticka GOOGLE site',
        'Automaticka MICROSOFT site',
        'Automaticka AMAZON site',
        'Automaticka AZURE site',
        'Automaticka AWS site',
        'Automaticka DIGITALOCEAN site',
        'Automaticka LINODE site',
        'Automaticka VULTR site',
        'Automaticka HEROKU site',
        'Automaticka RAILWAY site',
        'Automaticka RENDER site',
        'Automaticka NETLIFY site',
        'Automaticka VEREL site',
        'Automaticka CLOUDFLARE site',
        'Automaticka FASTLY site',
        'Automaticka AKAMAI site',
        'Automaticka CLOUDINARY site',
        'Automaticka STRIPE site',
        'Automaticka SENDGRID site',
        'Automaticka TWILIO site',
        'Automaticka NEXMO site',
        'Automaticka SNS site',
        'Automaticka SQS site',
        'Automaticka KINESIS site',
        'Automaticka LAMBDA site',
        'Automaticka STEPFUNCTIONS site',
        'Automaticka GLUE site',
        'Automaticka ATHENA site',
        'Automaticka REDSHIFT site',
        'Automaticka NEPTUNE site',
        'Automaticka DOCUMENTDB site',
        'Automaticka DYNAMODB site',
        'Automaticka COSMOSDB site',
        'Automaticka MONGODBATLAS site',
        'Automaticka Realm site',
        'Automaticka FIREBASE site',
        'Automaticka SUPABASE site',
        'Automaticka PLANETSCALE site',
        'Automaticka TURSO site',
        'Automaticka CockroachDB site',
        'Automaticka YUGABYTEDB site',
        'Automaticka SINGLESTORE site',
        'Automaticka CLICKHOUSE site',
        'Automaticka MATERIALIZE site',
        'Automaticka REDISVERTEX site',
        'Automaticka DRAGONFLY site',
        'Automaticka KEYDB site',
        'Automaticka VALKEY site',
        'Automaticka MEMGRAPH site',
        'Automaticka TINKERGRAPH site',
        'Automaticka ORIENTDB site',
        'Automaticka ARANGODB site',
        'Automaticka NEO4J site',
        'Automaticka DGRAPH site',
        'Automaticka TERMSOFUSE',
        'PRIVACY',
        'TERMS',
        'CONDITIONS',
        'Agreement',
        'Cookies',
    ]
    
    for word in czech_words:
        if word.lower() in text.lower():
            return True
    
    return False

def audit_file(filepath: Path, en_data: Dict, lang_code: str) -> tuple:
    """Audit a single file for contamination."""
    lang_data = load_json(filepath)
    if not lang_data:
        return [], []
    
    contaminated_entries = []
    missing_entries = []
    
    for key, en_value in en_data.items():
        if key not in lang_data:
            missing_entries.append((key, str(en_value)))
            continue
        
        lang_value = lang_data[key]
        if is_czech_text(str(lang_value)):
            contaminated_entries.append((key, str(lang_value), str(en_value)))
    
    return contaminated_entries, missing_entries

def fix_file(filepath: Path, en_data: Dict, lang_code: str) -> int:
    """Fix contamination in a file by replacing with English values."""
    lang_data = load_json(filepath)
    if not lang_data:
        return 0
    
    fixed_count = 0
    
    for key, en_value in en_data.items():
        if key in lang_data:
            if is_czech_text(str(lang_data[key])):
                lang_data[key] = en_value
                fixed_count += 1
    
    if fixed_count > 0:
        if save_json(filepath, lang_data):
            print(f"  [FIXED] {lang_code}/{filepath.parent.name}: {fixed_count} entries replaced with English")
    
    return fixed_count

def main():
    print("=" * 80)
    print("Cross-Language Contamination Fixer")
    print("=" * 80)
    print()
    
    # Get all subdirectories in ui/lang
    lang_subdirs = []
    for item in LANG_DIR.iterdir():
        if item.is_dir() and item.name not in ['meta']:
            lang_subdirs.append(item)
    
    lang_subdirs.sort(key=lambda x: x.name)
    
    print(f"Scanning {len(lang_subdirs)} directories...")
    print()
    
    total_fixed = 0
    total_contaminated = 0
    
    for subdir in lang_subdirs:
        print(f"Processing: {subdir.name}/")
        
        # Check if this subdir has its own subdirectories (like config/)
        subdirs_in_subdir = [p for p in subdir.iterdir() if p.is_dir()]
        
        if subdirs_in_subdir:
            # This is like config/ with many subdirectories
            for inner_subdir in sorted(subdirs_in_subdir):
                en_file = inner_subdir / 'en.json'
                if not en_file.exists():
                    continue
                
                en_data = load_json(en_file)
                if not en_data:
                    continue
                
                for lang in CONTAMINATED_LANGS:
                    lang_file = inner_subdir / f'{lang}.json'
                    if lang_file.exists():
                        fixed = fix_file(lang_file, en_data, lang)
                        if fixed > 0:
                            total_fixed += fixed
                            total_contaminated += fixed
        else:
            # Normal directory with language files
            en_file = subdir / 'en.json'
            if not en_file.exists():
                print(f"  [SKIP] No en.json in {subdir.name}")
                continue
            
            en_data = load_json(en_file)
            if not en_data:
                continue
            
            for lang in CONTAMINATED_LANGS:
                lang_file = subdir / f'{lang}.json'
                if lang_file.exists():
                    fixed = fix_file(lang_file, en_data, lang)
                    if fixed > 0:
                        total_fixed += fixed
                        total_contaminated += fixed
    
    print()
    print("=" * 80)
    print(f"COMPLETED: Fixed {total_fixed} contaminated entries")
    print("=" * 80)
    
    return total_fixed

if __name__ == '__main__':
    main()

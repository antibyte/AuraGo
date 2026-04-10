#!/usr/bin/env python3
"""
Comprehensive translation fix script for AuraGo UI.
Reads the v2 audit report, fixes all untranslated keys in all languages.
"""

import json
import os
import re
from pathlib import Path
from collections import defaultdict

LANG_DIR = Path("ui/lang")
LANGUAGES = ["cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"]
NON_EN_LANGS = [l for l in LANGUAGES if l != "en"]

# ── Comprehensive translation dictionary ──
# Maps English value -> {lang_code: translated_value}
# Covers backend messages, tools messages, config labels, and common UI strings

TRANSLATIONS = {
    # ═══════════════════════════════════════════════════════════════
    # TOOLS: process management
    # ═══════════════════════════════════════════════════════════════
    "Listed {0} processes (top {1} by CPU)": {
        "cs": "Seznam {0} procesů (top {1} podle CPU)",
        "da": "Vist {0} processer (top {1} efter CPU)",
        "de": "{0} Prozesse aufgelistet (Top {1} nach CPU)",
        "el": "Καταγράφηκαν {0} διεργασίες (κορυφαίες {1} κατά CPU)",
        "es": "Se listaron {0} procesos (top {1} por CPU)",
        "fr": "{0} processus listés (top {1} par CPU)",
        "hi": "{0} प्रक्रियाएँ सूचीबद्ध (CPU के अनुसार शीर्ष {1})",
        "it": "{0} processi elencati (top {1} per CPU)",
        "ja": "{0}プロセスをリスト（CPU上位{1}）",
        "nl": "{0} processen weergegeven (top {1} op CPU)",
        "no": "Viste {0} prosesser (topp {1} etter CPU)",
        "pl": "Wylistowano {0} procesów (top {1} wg CPU)",
        "pt": "Listados {0} processos (top {1} por CPU)",
        "sv": "Listade {0} processer (topp {1} efter CPU)",
        "zh": "已列出 {0} 个进程（按 CPU 排名前 {1}）",
    },
    "Unknown operation: {0}. Valid: list, kill, stats": {
        "cs": "Neznámá operace: {0}. Platné: list, kill, stats",
        "da": "Ukendt handling: {0}. Gyldige: list, kill, stats",
        "de": "Unbekannte Operation: {0}. Gültig: list, kill, stats",
        "el": "Άγνωστη λειτουργία: {0}. Έγκυρες: list, kill, stats",
        "es": "Operación desconocida: {0}. Válidas: list, kill, stats",
        "fr": "Opération inconnue : {0}. Validées : list, kill, stats",
        "hi": "अज्ञात ऑपरेशन: {0}। मान्य: list, kill, stats",
        "it": "Operazione sconosciuta: {0}. Valide: list, kill, stats",
        "ja": "不明な操作: {0}。有効: list, kill, stats",
        "nl": "Onbekende bewerking: {0}. Geldig: list, kill, stats",
        "no": "Ukjent operasjon: {0}. Gyldige: list, kill, stats",
        "pl": "Nieznana operacja: {0}. Prawidłowe: list, kill, stats",
        "pt": "Operação desconhecida: {0}. Válidas: list, kill, stats",
        "sv": "Okänd operation: {0}. Giltiga: list, kill, stats",
        "zh": "未知操作: {0}。有效值: list, kill, stats",
    },
    "PID {0} is protected and cannot be killed": {
        "cs": "PID {0} je chráněn a nelze jej ukončit",
        "da": "PID {0} er beskyttet og kan ikke afsluttes",
        "de": "PID {0} ist geschützt und kann nicht beendet werden",
        "el": "Το PID {0} είναι προστατευμένο και δεν μπορεί να τερματιστεί",
        "es": "PID {0} está protegido y no se puede terminar",
        "fr": "Le PID {0} est protégé et ne peut pas être arrêté",
        "hi": "PID {0} सुरक्षित है और बंद नहीं किया जा सकता",
        "it": "PID {0} è protetto e non può essere terminato",
        "ja": "PID {0} は保護されており終了できません",
        "nl": "PID {0} is beveiligd en kan niet worden beëindigd",
        "no": "PID {0} er beskyttet og kan ikke termineres",
        "pl": "PID {0} jest chroniony i nie może zostać zakończony",
        "pt": "PID {0} está protegido e não pode ser encerrado",
        "sv": "PID {0} är skyddad och kan inte avslutas",
        "zh": "PID {0} 受保护，无法终止",
    },
    "Failed to kill process {0}: {1}": {
        "cs": "Nepodařilo se ukončit proces {0}: {1}",
        "da": "Kunne ikke afslutte proces {0}: {1}",
        "de": "Prozess {0} konnte nicht beendet werden: {1}",
        "el": "Αποτυχία τερματισμού διεργασίας {0}: {1}",
        "es": "No se pudo terminar el proceso {0}: {1}",
        "fr": "Échec de l'arrêt du processus {0} : {1}",
        "hi": "प्रक्रिया {0} बंद करने में विफल: {1}",
        "it": "Impossibile terminare il processo {0}: {1}",
        "ja": "プロセス {0} の終了に失敗: {1}",
        "nl": "Kan proces {0} niet beëindigen: {1}",
        "no": "Kunne ikke terminere prosess {0}: {1}",
        "pl": "Nie udało się zakończyć procesu {0}: {1}",
        "pt": "Falha ao encerrar processo {0}: {1}",
        "sv": "Kunde inte avsluta process {0}: {1}",
        "zh": "终止进程 {0} 失败: {1}",
    },
    "Failed to list processes: {0}": {
        "cs": "Nepodařilo se vypsat procesy: {0}",
        "da": "Kunne ikke vise processer: {0}",
        "de": "Prozesse konnten nicht aufgelistet werden: {0}",
        "el": "Αποτυχία λίστας διεργασιών: {0}",
        "es": "No se pudieron listar los procesos: {0}",
        "fr": "Échec du listage des processus : {0}",
        "hi": "प्रक्रियाओं को सूचीबद्ध करने में विफल: {0}",
        "it": "Impossibile elencare i processi: {0}",
        "ja": "プロセス一覧の取得に失敗: {0}",
        "nl": "Kan processen niet weergeven: {0}",
        "no": "Kunne ikke liste prosesser: {0}",
        "pl": "Nie udało się wylistować procesów: {0}",
        "pt": "Falha ao listar processos: {0}",
        "sv": "Kunde inte lista processer: {0}",
        "zh": "列出进程失败: {0}",
    },
    "Process {0} terminated": {
        "cs": "Proces {0} ukončen",
        "da": "Proces {0} afsluttet",
        "de": "Prozess {0} wurde beendet",
        "el": "Η διεργασία {0} τερματίστηκε",
        "es": "Proceso {0} terminado",
        "fr": "Processus {0} arrêté",
        "hi": "प्रक्रिया {0} समाप्त",
        "it": "Processo {0} terminato",
        "ja": "プロセス {0} を終了しました",
        "nl": "Proces {0} beëindigd",
        "no": "Prosess {0} terminert",
        "pl": "Proces {0} zakończony",
        "pt": "Processo {0} encerrado",
        "sv": "Process {0} avslutad",
        "zh": "进程 {0} 已终止",
    },

    # ═══════════════════════════════════════════════════════════════
    # TOOLS: DNS
    # ═══════════════════════════════════════════════════════════════
    "partial results": {
        "cs": "částečné výsledky",
        "da": "delvise resultater",
        "de": "Teilergebnisse",
        "el": "μερικά αποτελέσματα",
        "es": "resultados parciales",
        "fr": "résultats partiels",
        "hi": "आंशिक परिणाम",
        "it": "risultati parziali",
        "ja": "部分結果",
        "nl": "gedeeltelijke resultaten",
        "no": "delvise resultater",
        "pl": "częściowe wyniki",
        "pt": "resultados parciais",
        "sv": "ofullständiga resultat",
        "zh": "部分结果",
    },
    "MX lookup: {0}": {
        "cs": "MX vyhledávání: {0}",
        "da": "MX-opslag: {0}",
        "de": "MX-Lookup: {0}",
        "el": "Αναζήτηση MX: {0}",
        "es": "Búsqueda MX: {0}",
        "fr": "Recherche MX : {0}",
        "hi": "MX लुकअप: {0}",
        "it": "Ricerca MX: {0}",
        "ja": "MXルックアップ: {0}",
        "nl": "MX-zoekactie: {0}",
        "no": "MX-oppslag: {0}",
        "pl": "Wyszukiwanie MX: {0}",
        "pt": "Pesquisa MX: {0}",
        "sv": "MX-uppslag: {0}",
        "zh": "MX 查找: {0}",
    },
    "PTR lookup: {0}": {
        "cs": "PTR vyhledávání: {0}",
        "da": "PTR-opslag: {0}",
        "de": "PTR-Lookup: {0}",
        "el": "Αναζήτηση PTR: {0}",
        "es": "Búsqueda PTR: {0}",
        "fr": "Recherche PTR : {0}",
        "hi": "PTR लुकअप: {0}",
        "it": "Ricerca PTR: {0}",
        "ja": "PTRルックアップ: {0}",
        "nl": "PTR-zoekactie: {0}",
        "no": "PTR-oppslag: {0}",
        "pl": "Wyszukiwanie PTR: {0}",
        "pt": "Pesquisa PTR: {0}",
        "sv": "PTR-uppslag: {0}",
        "zh": "PTR 查找: {0}",
    },
    "CNAME lookup: {0}": {
        "cs": "CNAME vyhledávání: {0}",
        "da": "CNAME-opslag: {0}",
        "de": "CNAME-Lookup: {0}",
        "el": "Αναζήτηση CNAME: {0}",
        "es": "Búsqueda CNAME: {0}",
        "fr": "Recherche CNAME : {0}",
        "hi": "CNAME लुकअप: {0}",
        "it": "Ricerca CNAME: {0}",
        "ja": "CNAMEルックアップ: {0}",
        "nl": "CNAME-zoekactie: {0}",
        "no": "CNAME-oppslag: {0}",
        "pl": "Wyszukiwanie CNAME: {0}",
        "pt": "Pesquisa CNAME: {0}",
        "sv": "CNAME-uppslag: {0}",
        "zh": "CNAME 查找: {0}",
    },
    "IP lookup: {0}": {
        "cs": "IP vyhledávání: {0}",
        "da": "IP-opslag: {0}",
        "de": "IP-Lookup: {0}",
        "el": "Αναζήτηση IP: {0}",
        "es": "Búsqueda IP: {0}",
        "fr": "Recherche IP : {0}",
        "hi": "IP लुकअप: {0}",
        "it": "Ricerca IP: {0}",
        "ja": "IPルックアップ: {0}",
        "nl": "IP-zoekactie: {0}",
        "no": "IP-oppslag: {0}",
        "pl": "Wyszukiwanie IP: {0}",
        "pt": "Pesquisa IP: {0}",
        "sv": "IP-uppslag: {0}",
        "zh": "IP 查找: {0}",
    },

    # ═══════════════════════════════════════════════════════════════
    # TOOLS: cron
    # ═══════════════════════════════════════════════════════════════
    "id required for enable": {
        "cs": "ID je vyžadováno pro povolení",
        "da": "ID påkrævet for at aktivere",
        "de": "ID zum Aktivieren erforderlich",
        "el": "Απαιτείται id για ενεργοποίηση",
        "es": "ID requerido para habilitar",
        "fr": "ID requis pour activer",
        "hi": "सक्षम करने के लिए ID आवश्यक",
        "it": "ID richiesto per l'abilitazione",
        "ja": "有効化にはIDが必要です",
        "nl": "ID vereist om in te schakelen",
        "no": "ID kreves for å aktivere",
        "pl": "ID wymagane do włączenia",
        "pt": "ID obrigatório para ativar",
        "sv": "ID krävs för att aktivera",
        "zh": "启用需要 ID",
    },
    "id required for disable": {
        "cs": "ID je vyžadováno pro zakázání",
        "da": "ID påkrævet for at deaktivere",
        "de": "ID zum Deaktivieren erforderlich",
        "el": "Απαιτείται id για απενεργοποίηση",
        "es": "ID requerido para deshabilitar",
        "fr": "ID requis pour désactiver",
        "hi": "अक्षम करने के लिए ID आवश्यक",
        "it": "ID richiesto per la disabilitazione",
        "ja": "無効化にはIDが必要です",
        "nl": "ID vereist om uit te schakelen",
        "no": "ID kreves for å deaktivere",
        "pl": "ID wymagane do wyłączenia",
        "pt": "ID obrigatório para desativar",
        "sv": "ID krävs för att inaktivera",
        "zh": "禁用需要 ID",
    },
    "id required for remove": {
        "cs": "ID je vyžadováno pro odstranění",
        "da": "ID påkrævet for at fjerne",
        "de": "ID zum Entfernen erforderlich",
        "el": "Απαιτείται id για αφαίρεση",
        "es": "ID requerido para eliminar",
        "fr": "ID requis pour supprimer",
        "hi": "हटाने के लिए ID आवश्यक",
        "it": "ID richiesto per la rimozione",
        "ja": "削除にはIDが必要です",
        "nl": "ID vereist om te verwijderen",
        "no": "ID kreves for å fjerne",
        "pl": "ID wymagane do usunięcia",
        "pt": "ID obrigatório para remover",
        "sv": "ID krävs för att ta bort",
        "zh": "移除需要 ID",
    },
    "Invalid cron expression: {0}": {
        "cs": "Neplatný cron výraz: {0}",
        "da": "Ugyldigt cron-udtryk: {0}",
        "de": "Ungültiger Cron-Ausdruck: {0}",
        "el": "Μη έγκυρη παράσταση cron: {0}",
        "es": "Expresión cron no válida: {0}",
        "fr": "Expression cron invalide : {0}",
        "hi": "अमान्य cron अभिव्यक्ति: {0}",
        "it": "Espressione cron non valida: {0}",
        "ja": "無効なcron式: {0}",
        "nl": "Ongeldige cron-expressie: {0}",
        "no": "Ugyldig cron-uttrykk: {0}",
        "pl": "Nieprawidłowe wyrażenie cron: {0}",
        "pt": "Expressão cron inválida: {0}",
        "sv": "Ogiltigt cron-uttryck: {0}",
        "zh": "无效的 cron 表达式: {0}",
    },
    "cron_expr and task_prompt required for add": {
        "cs": "cron_expr a task_prompt jsou vyžadovány pro přidání",
        "da": "cron_expr og task_prompt påkrævet for at tilføje",
        "de": "cron_expr und task_prompt zum Hinzufügen erforderlich",
        "el": "cron_expr και task_prompt απαιτούνται για προσθήκη",
        "es": "cron_expr y task_prompt requeridos para añadir",
        "fr": "cron_expr et task_prompt requis pour ajouter",
        "hi": "जोड़ने के लिए cron_expr और task_prompt आवश्यक",
        "it": "cron_expr e task_prompt richiesti per l'aggiunta",
        "ja": "追加にはcron_exprとtask_promptが必要です",
        "nl": "cron_expr en task_prompt vereist om toe te voegen",
        "no": "cron_expr og task_prompt kreves for å legge til",
        "pl": "cron_expr i task_prompt wymagane do dodania",
        "pt": "cron_expr e task_prompt obrigatórios para adicionar",
        "sv": "cron_expr och task_prompt krävs för att lägga till",
        "zh": "添加需要 cron_expr 和 task_prompt",
    },
    "Job ID not found": {
        "cs": "ID úlohy nenalezeno",
        "da": "Job-ID ikke fundet",
        "de": "Job-ID nicht gefunden",
        "el": "Το ID εργασίας δεν βρέθηκε",
        "es": "ID de trabajo no encontrado",
        "fr": "ID de tâche introuvable",
        "hi": "जॉब ID नहीं मिला",
        "it": "ID lavoro non trovato",
        "ja": "ジョブIDが見つかりません",
        "nl": "Taak-ID niet gevonden",
        "no": "Jobb-ID ikke funnet",
        "pl": "Nie znaleziono ID zadania",
        "pt": "ID do trabalho não encontrado",
        "sv": "Jobb-ID hittades inte",
        "zh": "未找到作业 ID",
    },
    "Job already disabled.": {
        "cs": "Úloha je již zakázána.",
        "da": "Job er allerede deaktiveret.",
        "de": "Job ist bereits deaktiviert.",
        "el": "Η εργασία είναι ήδη απενεργοποιημένη.",
        "es": "El trabajo ya está deshabilitado.",
        "fr": "La tâche est déjà désactivée.",
        "hi": "जॉब पहले से अक्षम है।",
        "it": "Il lavoro è già disabilitato.",
        "ja": "ジョブはすでに無効です。",
        "nl": "Taak is al uitgeschakeld.",
        "no": "Jobben er allerede deaktivert.",
        "pl": "Zadanie jest już wyłączone.",
        "pt": "O trabalho já está desativado.",
        "sv": "Jobbet är redan inaktiverat.",
        "zh": "作业已被禁用。",
    },
    "Job already enabled.": {
        "cs": "Úloha je již povolena.",
        "da": "Job er allerede aktiveret.",
        "de": "Job ist bereits aktiviert.",
        "el": "Η εργασία είναι ήδη ενεργοποιημένη.",
        "es": "El trabajo ya está habilitado.",
        "fr": "La tâche est déjà activée.",
        "hi": "जॉब पहले से सक्षम है।",
        "it": "Il lavoro è già abilitato.",
        "ja": "ジョブはすでに有効です。",
        "nl": "Taak is al ingeschakeld.",
        "no": "Jobben er allerede aktivert.",
        "pl": "Zadanie jest już włączone.",
        "pt": "O trabalho já está ativo.",
        "sv": "Jobbet är redan aktiverat.",
        "zh": "作业已被启用。",
    },
    "Job scheduled.": {
        "cs": "Úloha naplánována.",
        "da": "Job planlagt.",
        "de": "Job geplant.",
        "el": "Η εργασία προγραμματίστηκε.",
        "es": "Trabajo programado.",
        "fr": "Tâche planifiée.",
        "hi": "जॉब निर्धारित।",
        "it": "Lavoro pianificato.",
        "ja": "ジョブをスケジュールしました。",
        "nl": "Taak ingepland.",
        "no": "Jobb planlagt.",
        "pl": "Zadanie zaplanowane.",
        "pt": "Trabalho agendado.",
        "sv": "Jobb schemalagt.",
        "zh": "作业已计划。",
    },
    "Job disabled.": {
        "cs": "Úloha zakázána.",
        "da": "Job deaktiveret.",
        "de": "Job deaktiviert.",
        "el": "Η εργασία απενεργοποιήθηκε.",
        "es": "Trabajo deshabilitado.",
        "fr": "Tâche désactivée.",
        "hi": "जॉब अक्षम।",
        "it": "Lavoro disabilitato.",
        "ja": "ジョブを無効化しました。",
        "nl": "Taak uitgeschakeld.",
        "no": "Jobb deaktivert.",
        "pl": "Zadanie wyłączone.",
        "pt": "Trabalho desativado.",
        "sv": "Jobb inaktiverat.",
        "zh": "作业已禁用。",
    },
    "Job removed.": {
        "cs": "Úloha odebrána.",
        "da": "Job fjernet.",
        "de": "Job entfernt.",
        "el": "Η εργασία αφαιρέθηκε.",
        "es": "Trabajo eliminado.",
        "fr": "Tâche supprimée.",
        "hi": "जॉब हटाया गया।",
        "it": "Lavoro rimosso.",
        "ja": "ジョブを削除しました。",
        "nl": "Taak verwijderd.",
        "no": "Jobb fjernet.",
        "pl": "Zadanie usunięte.",
        "pt": "Trabalho removido.",
        "sv": "Jobb borttaget.",
        "zh": "作业已移除。",
    },

    # ═══════════════════════════════════════════════════════════════
    # TOOLS: core memory
    # ═══════════════════════════════════════════════════════════════
    "'fact' text is required for operation 'remove'.": {
        "cs": "Text 'fact' je vyžadován pro operaci 'remove'.",
        "da": "'fact'-tekst er påkrævet for handlingen 'remove'.",
        "de": "Text 'fact' ist für die Operation 'remove' erforderlich.",
        "el": "Το κείμενο 'fact' απαιτείται για τη λειτουργία 'remove'.",
        "es": "El texto 'fact' es obligatorio para la operación 'remove'.",
        "fr": "Le texte 'fact' est requis pour l'opération 'remove'.",
        "hi": "ऑपरेशन 'remove' के लिए 'fact' पाठ आवश्यक है।",
        "it": "Il testo 'fact' è richiesto per l'operazione 'remove'.",
        "ja": "操作 'remove' には 'fact' テキストが必要です。",
        "nl": "'fact'-tekst is vereist voor bewerking 'remove'.",
        "no": "'fact'-tekst kreves for operasjonen 'remove'.",
        "pl": "Tekst 'fact' jest wymagany dla operacji 'remove'.",
        "pt": "O texto 'fact' é obrigatório para a operação 'remove'.",
        "sv": "'fact'-text krävs för operationen 'remove'.",
        "zh": "操作 'remove' 需要提供 'fact' 文本。",
    },
    "'fact' is required for operation 'update'.": {
        "cs": "'fact' je vyžadováno pro operaci 'update'.",
        "da": "'fact' er påkrævet for handlingen 'update'.",
        "de": "'fact' ist für die Operation 'update' erforderlich.",
        "el": "Το 'fact' απαιτείται για τη λειτουργία 'update'.",
        "es": "'fact' es obligatorio para la operación 'update'.",
        "fr": "'fact' est requis pour l'opération 'update'.",
        "hi": "ऑपरेशन 'update' के लिए 'fact' आवश्यक है।",
        "it": "'fact' è richiesto per l'operazione 'update'.",
        "ja": "操作 'update' には 'fact' が必要です。",
        "nl": "'fact' is vereist voor bewerking 'update'.",
        "no": "'fact' kreves for operasjonen 'update'.",
        "pl": "'fact' jest wymagany dla operacji 'update'.",
        "pt": "'fact' é obrigatório para a operação 'update'.",
        "sv": "'fact' krävs för operationen 'update'.",
        "zh": "操作 'update' 需要提供 'fact'。",
    },
    "'id' is required for operation 'delete'. Use the numeric id shown in [brackets] before each memory entry.": {
        "cs": "'id' je vyžadováno pro operaci 'delete'. Použijte číselné ID zobrazené v [závorkách] před každou položkou paměti.",
        "da": "'id' er påkrævet for handlingen 'delete'. Brug det numeriske ID vist i [parenteser] foran hver hukommelsespost.",
        "de": "'id' ist für die Operation 'delete' erforderlich. Verwende die numerische ID in [Klammern] vor jedem Speichereintrag.",
        "el": "Το 'id' απαιτείται για τη λειτουργία 'delete'. Χρησιμοποιήστε το αριθμητικό id που εμφανίζεται σε [αγκύλες] πριν από κάθε καταχώρηση μνήμης.",
        "es": "'id' es obligatorio para la operación 'delete'. Use el ID numérico mostrado entre [corchetes] antes de cada entrada de memoria.",
        "fr": "'id' est requis pour l'opération 'delete'. Utilisez l'ID numérique affiché entre [crochets] avant chaque entrée mémoire.",
        "hi": "ऑपरेशन 'delete' के लिए 'id' आवश्यक है। प्रत्येक मेमोरी प्रविष्टि से पहले [कोष्ठक] में दिखाई गई संख्यात्मक ID का उपयोग करें।",
        "it": "'id' è richiesto per l'operazione 'delete'. Usa l'ID numerico mostrato tra [parentesi] prima di ogni voce di memoria.",
        "ja": "操作 'delete' には 'id' が必要です。各メモリエントリの前の[括弧]に表示される数値IDを使用してください。",
        "nl": "'id' is vereist voor bewerking 'delete'. Gebruik het numerieke ID weergegeven tussen [haakjes] voor elke geheugenvermelding.",
        "no": "'id' kreves for operasjonen 'delete'. Bruk den numeriske ID-en vist i [hakeparenteser] foran hver minneoppføring.",
        "pl": "'id' jest wymagane dla operacji 'delete'. Użyj numerycznego ID wyświetlanego w [nawiasach] przed każdym wpisem pamięci.",
        "pt": "'id' é obrigatório para a operação 'delete'. Use o ID numérico exibido entre [colchetes] antes de cada entrada de memória.",
        "sv": "'id' krävs för operationen 'delete'. Använd det numeriska ID som visas i [hakparenteser] framför varje minnespost.",
        "zh": "操作 'delete' 需要提供 'id'。请使用每个内存条目前[方括号]中显示的数字 ID。",
    },
    "'id' is required for operation 'update'. Use the numeric id shown in [brackets] before each memory entry.": {
        "cs": "'id' je vyžadováno pro operaci 'update'. Použijte číselné ID zobrazené v [závorkách] před každou položkou paměti.",
        "da": "'id' er påkrævet for handlingen 'update'. Brug det numeriske ID vist i [parenteser] foran hver hukommelsespost.",
        "de": "'id' ist für die Operation 'update' erforderlich. Verwende die numerische ID in [Klammern] vor jedem Speichereintrag.",
        "el": "Το 'id' απαιτείται για τη λειτουργία 'update'. Χρησιμοποιήστε το αριθμητικό id που εμφανίζεται σε [αγκύλες] πριν από κάθε καταχώρηση μνήμης.",
        "es": "'id' es obligatorio para la operación 'update'. Use el ID numérico mostrado entre [corchetes] antes de cada entrada de memoria.",
        "fr": "'id' est requis pour l'opération 'update'. Utilisez l'ID numérique affiché entre [crochets] avant chaque entrée mémoire.",
        "hi": "ऑपरेशन 'update' के लिए 'id' आवश्यक है। प्रत्येक मेमोरी प्रविष्टि से पहले [कोष्ठक] में दिखाई गई संख्यात्मक ID का उपयोग करें।",
        "it": "'id' è richiesto per l'operazione 'update'. Usa l'ID numerico mostrato tra [parentesi] prima di ogni voce di memoria.",
        "ja": "操作 'update' には 'id' が必要です。各メモリエントリの前の[括弧]に表示される数値IDを使用してください。",
        "nl": "'id' is vereist voor bewerking 'update'. Gebruik het numerieke ID weergegeven tussen [haakjes] voor elke geheugenvermelding.",
        "no": "'id' kreves for operasjonen 'update'. Bruk den numeriske ID-en vist i [hakeparenteser] foran hver minneoppføring.",
        "pl": "'id' jest wymagane dla operacji 'update'. Użyj numerycznego ID wyświetlanego w [nawiasach] przed każdym wpisem pamięci.",
        "pt": "'id' é obrigatório para a operação 'update'. Use o ID numérico exibido entre [colchetes] antes de cada entrada de memória.",
        "sv": "'id' krävs för operationen 'update'. Använd det numeriska ID som visas i [hakparenteser] framför varje minnespost.",
        "zh": "操作 'update' 需要提供 'id'。请使用每个内存条目前[方括号]中显示的数字 ID。",
    },
    "Failed to serialize entries.": {
        "cs": "Nepodařilo se serializovat položky.",
        "da": "Kunne ikke serialisere poster.",
        "de": "Einträge konnten nicht serialisiert werden.",
        "el": "Αποτυχία σειριοποίησης καταχωρήσεων.",
        "es": "No se pudieron serializar las entradas.",
        "fr": "Échec de la sérialisation des entrées.",
        "hi": "प्रविष्टियों को क्रमबद्ध करने में विफल।",
        "it": "Impossibile serializzare le voci.",
        "ja": "エントリのシリアライズに失敗しました。",
        "nl": "Kan vermeldingen niet serialiseren.",
        "no": "Kunne ikke serialisere oppføringer.",
        "pl": "Nie udało się zserializować wpisów.",
        "pt": "Falha ao serializar entradas.",
        "sv": "Kunde inte serialisera poster.",
        "zh": "序列化条目失败。",
    },
    "Core memory entry deleted.": {
        "cs": "Položka jádrové paměti smazána.",
        "da": "Kernehukommelsespost slettet.",
        "de": "Kernspeicher-Eintrag gelöscht.",
        "el": "Η καταχώρηση πυρηνικής μνήμης διαγράφηκε.",
        "es": "Entrada de memoria central eliminada.",
        "fr": "Entrée de mémoire centrale supprimée.",
        "hi": "कोर मेमोरी प्रविष्टि हटाई गई।",
        "it": "Voce di memoria centrale eliminata.",
        "ja": "コアメモリエントリを削除しました。",
        "nl": "Kerngeheugenvermelding verwijderd.",
        "no": "Kjerneminneoppføring slettet.",
        "pl": "Wpis pamięci głównej usunięty.",
        "pt": "Entrada da memória central excluída.",
        "sv": "Kärnminnespost borttagen.",
        "zh": "核心内存条目已删除。",
    },
    "Core memory entry updated.": {
        "cs": "Položka jádrové paměti aktualizována.",
        "da": "Kernehukommelsespost opdateret.",
        "de": "Kernspeicher-Eintrag aktualisiert.",
        "el": "Η καταχώρηση πυρηνικής μνήμης ενημερώθηκε.",
        "es": "Entrada de memoria central actualizada.",
        "fr": "Entrée de mémoire centrale mise à jour.",
        "hi": "कोर मेमोरी प्रविष्टि अपडेट की गई।",
        "it": "Voce di memoria centrale aggiornata.",
        "ja": "コアメモリエントリを更新しました。",
        "nl": "Kerngeheugenvermelding bijgewerkt.",
        "no": "Kjerneminneoppføring oppdatert.",
        "pl": "Wpis pamięci głównej zaktualizowany.",
        "pt": "Entrada da memória central atualizada.",
        "sv": "Kärnminnespost uppdaterad.",
        "zh": "核心内存条目已更新。",
    },
    "Core memory is full ({0}/{1} entries). Delete outdated entries first.": {
        "cs": "Jádrová paměť je plná ({0}/{1} položek). Nejprve smažte zastaralé položky.",
        "da": "Kernehukommelsen er fuld ({0}/{1} poster). Slet forældede poster først.",
        "de": "Kernspeicher ist voll ({0}/{1} Einträge). Bitte veraltete Einträge zuerst löschen.",
        "el": "Η πυρηνική μνήμη είναι γεμάτη ({0}/{1} καταχωρήσεις). Διαγράψτε παλιές καταχωρήσεις πρώτα.",
        "es": "La memoria central está llena ({0}/{1} entradas). Elimine entradas obsoletas primero.",
        "fr": "La mémoire centrale est pleine ({0}/{1} entrées). Supprimez d'abord les entrées obsolètes.",
        "hi": "कोर मेमोरी भरी हुई है ({0}/{1} प्रविष्टियाँ)। पहले पुरानी प्रविष्टियाँ हटाएँ।",
        "it": "La memoria centrale è piena ({0}/{1} voci). Elimina prima le voci obsolete.",
        "ja": "コアメモリが満杯です（{0}/{1}エントリ）。古いエントリを先に削除してください。",
        "nl": "Kerngeheugen is vol ({0}/{1} vermeldingen). Verwijder eerst verouderde vermeldingen.",
        "no": "Kjerneminnet er fullt ({0}/{1} oppføringer). Slett utdaterte oppføringer først.",
        "pl": "Pamięć główna jest pełna ({0}/{1} wpisów). Usuń najpierw nieaktualne wpisy.",
        "pt": "A memória central está cheia ({0}/{1} entradas). Exclua entradas obsoletas primeiro.",
        "sv": "Kärnminnet är fullt ({0}/{1} poster). Ta bort föråldrade poster först.",
        "zh": "核心内存已满（{0}/{1} 条）。请先删除过时条目。",
    },
    "Fact already exists in core memory.": {
        "cs": "Fakt již v jádrové paměti existuje.",
        "da": "Fakta findes allerede i kernehukommelsen.",
        "de": "Fakt existiert bereits im Kernspeicher.",
        "el": "Το γεγονός υπάρχει ήδη στη πυρηνική μνήμη.",
        "es": "El dato ya existe en la memoria central.",
        "fr": "Le fait existe déjà dans la mémoire centrale.",
        "hi": "तथ्य पहले से कोर मेमोरी में मौजूद है।",
        "it": "Il fatto esiste già nella memoria centrale.",
        "ja": "ファクトはすでにコアメモリに存在します。",
        "nl": "Feit bestaat al in kerngeheugen.",
        "no": "Fakta finnes allerede i kjerneminnet.",
        "pl": "Fakt już istnieje w pamięci głównej.",
        "pt": "O fato já existe na memória central.",
        "sv": "Fakta finns redan i kärnminnet.",
        "zh": "事实已存在于核心内存中。",
    },
    "Fact permanently added to core memory.": {
        "cs": "Fakt trvale přidán do jádrové paměti.",
        "da": "Fakta permanent tilføjet til kernehukommelsen.",
        "de": "Fakt dauerhaft zum Kernspeicher hinzugefügt.",
        "el": "Το γεγονός μονίμως προστέθηκε στη πυρηνική μνήμη.",
        "es": "Dato añadido permanentemente a la memoria central.",
        "fr": "Fait ajouté définitivement à la mémoire centrale.",
        "hi": "तथ्य स्थायी रूप से कोर मेमोरी में जोड़ा गया।",
        "it": "Fatto aggiunto permanentemente alla memoria centrale.",
        "ja": "ファクトをコアメモリに永久追加しました。",
        "nl": "Feit permanent toegevoegd aan kerngeheugen.",
        "no": "Fakta permanent lagt til i kjerneminnet.",
        "pl": "Fakt trwale dodany do pamięci głównej.",
        "pt": "Fato adicionado permanentemente à memória central.",
        "sv": "Fakta permanent tillagt i kärnminnet.",
        "zh": "事实已永久添加到核心内存。",
    },
    "Fact added (soft-cap warning: {0}/{1} entries used). Consider removing outdated entries.": {
        "cs": "Fakt přidán (varování soft-cap: {0}/{1} položek použito). Zvažte odstranění zastaralých položek.",
        "da": "Fakta tilføjet (soft-cap advarsel: {0}/{1} poster brugt). Overvej at fjerne forældede poster.",
        "de": "Fakt hinzugefügt (Soft-Cap-Warnung: {0}/{1} Einträge belegt). Veraltete Einträge sollten entfernt werden.",
        "el": "Το γεγονός προστέθηκε (προειδοποίηση soft-cap: {0}/{1} καταχωρήσεις σε χρήση). Εξετάστε την αφαίρεση παλιών καταχωρήσεων.",
        "es": "Dato añadido (aviso de límite suave: {0}/{1} entradas usadas). Considere eliminar entradas obsoletas.",
        "fr": "Fait ajouté (avertissement de limite douce : {0}/{1} entrées utilisées). Pensez à supprimer les entrées obsolètes.",
        "hi": "तथ्य जोड़ा गया (सॉफ्ट-कैप चेतावनी: {0}/{1} प्रविष्टियाँ उपयोग में)। पुरानी प्रविष्टियाँ हटाने पर विचार करें।",
        "it": "Fatto aggiunto (avviso soft-cap: {0}/{1} voci usate). Valutare la rimozione di voci obsolete.",
        "ja": "ファクトを追加しました（ソフト上限警告: {0}/{1}エントリ使用中）。古いエントリの削除を検討してください。",
        "nl": "Feit toegevoegd (soft-cap waarschuwing: {0}/{1} vermeldingen gebruikt). Overweeg verouderde vermeldingen te verwijderen.",
        "no": "Fakta lagt til (soft-cap advarsel: {0}/{1} oppføringer brukt). Vurder å fjerne utdaterte oppføringer.",
        "pl": "Fakt dodany (ostrzeżenie soft-cap: {0}/{1} wpisów użytych). Rozważ usunięcie nieaktualnych wpisów.",
        "pt": "Fato adicionado (aviso de limite flexível: {0}/{1} entradas usadas). Considere remover entradas obsoletas.",
        "sv": "Fakta tillagt (soft-cap varning: {0}/{1} poster använda). Överväg att ta bort föråldrade poster.",
        "zh": "事实已添加（软上限警告：已使用 {0}/{1} 条）。建议删除过时条目。",
    },

    # ═══════════════════════════════════════════════════════════════
    # TOOLS: ping
    # ═══════════════════════════════════════════════════════════════
    "failed to create pinger: {0}": {
        "cs": "nepodařilo se vytvořit pinger: {0}",
        "da": "kunne ikke oprette pinger: {0}",
        "de": "Pinger konnte nicht erstellt werden: {0}",
        "el": "αποτυχία δημιουργίας pinger: {0}",
        "es": "no se pudo crear el pinger: {0}",
        "fr": "échec de la création du pinger : {0}",
        "hi": "पिंगर बनाने में विफल: {0}",
        "it": "impossibile creare il pinger: {0}",
        "ja": "pingerの作成に失敗: {0}",
        "nl": "kan pinger niet maken: {0}",
        "no": "kunne ikke opprette pinger: {0}",
        "pl": "nie udało się utworzyć pinger: {0}",
        "pt": "falha ao criar pinger: {0}",
        "sv": "kunde inte skapa pinger: {0}",
        "zh": "创建 pinger 失败: {0}",
    },
    "ping failed (privileged: {0}; unprivileged: {1}) — ensure the host is reachable and ICMP is allowed": {
        "cs": "ping selhal (privilegovaný: {0}; neprivilegovaný: {1}) — ujistěte se, že hostitel je dostupný a ICMP je povoleno",
        "da": "ping mislykkedes (privilegeret: {0}; uprivilegeret: {1}) — sikr at værten er tilgængelig og ICMP er tilladt",
        "de": "Ping fehlgeschlagen (privilegiert: {0}; unprivilegiert: {1}) — Stelle sicher, dass der Host erreichbar ist und ICMP erlaubt ist",
        "el": "το ping απέτυχε (προνομιούχο: {0}; μη προνομιούχο: {1}) — βεβαιωθείτε ότι ο κόμβος είναι προσβάσιμος και το ICMP επιτρέπεται",
        "es": "ping fallido (privilegiado: {0}; no privilegiado: {1}) — asegúrese de que el host sea accesible e ICMP esté permitido",
        "fr": "échec du ping (privilégié : {0} ; non privilégié : {1}) — vérifiez que l'hôte est accessible et que ICMP est autorisé",
        "hi": "पिंग विफल (विशेषाधिकार प्राप्त: {0}; अविशेषाधिकार प्राप्त: {1}) — सुनिश्चित करें कि होस्ट पहुँच योग्य है और ICMP अनुमत है",
        "it": "ping fallito (privilegiato: {0}; non privilegiato: {1}) — assicurarsi che l'host sia raggiungibile e ICMP sia consentito",
        "ja": "ping失敗（特権: {0}; 非特権: {1}）— ホストが到達可能でICMPが許可されていることを確認してください",
        "nl": "ping mislukt (privileged: {0}; unprivileged: {1}) — zorg dat de host bereikbaar is en ICMP is toegestaan",
        "no": "ping mislyktes (privilegert: {0}; uprivilegert: {1}) — sikre at verten er tilgjengelig og ICMP er tillatt",
        "pl": "ping nie powiódł się (uprzywilejowany: {0}; nieuprzywilejowany: {1}) — upewnij się, że host jest osiągalny a ICMP jest dozwolony",
        "pt": "ping falhou (privilegiado: {0}; não privilegiado: {1}) — certifique-se de que o host está acessível e ICMP está permitido",
        "sv": "ping misslyckades (privilegierad: {0}; opriviligierad: {1}) — säkerställ att värden är nåbar och ICMP är tillåtet",
        "zh": "ping 失败（特权: {0}；非特权: {1}）— 请确保主机可达且 ICMP 已允许",
    },
    "host is required": {
        "cs": "hostitel je vyžadován",
        "da": "vært er påkrævet",
        "de": "Host ist erforderlich",
        "el": "απαιτείται κόμβος",
        "es": "host es obligatorio",
        "fr": "hôte requis",
        "hi": "होस्ट आवश्यक है",
        "it": "host è obbligatorio",
        "ja": "ホストは必須です",
        "nl": "host is vereist",
        "no": "vert kreves",
        "pl": "host jest wymagany",
        "pt": "host é obrigatório",
        "sv": "värd krävs",
        "zh": "主机为必填项",
    },
}


def flatten(obj, prefix=""):
    items = {}
    if isinstance(obj, dict):
        for k, v in obj.items():
            new_key = f"{prefix}.{k}" if prefix else k
            if isinstance(v, dict):
                items.update(flatten(v, new_key))
            else:
                items[new_key] = v
    return items


def unflatten(flat_dict):
    """Convert flat dot-notation dict back to nested dict."""
    result = {}
    for key, value in flat_dict.items():
        parts = key.split(".")
        d = result
        for part in parts[:-1]:
            if part not in d:
                d[part] = {}
            d = d[part]
        d[parts[-1]] = value
    return result


def read_json(path: Path) -> tuple[dict | None, str | None]:
    try:
        with open(path, "r", encoding="utf-8-sig") as f:
            return json.load(f), None
    except Exception as e:
        return None, str(e)


def write_json(path: Path, data: dict):
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
        f.write("\n")


def fix_translations():
    """Main fix function: reads all files, applies translations, writes back."""
    stats = defaultdict(lambda: {"fixed": 0, "unchanged": 0, "errors": 0})

    # Collect all directories with en.json
    dirs = []
    for root, _, files in os.walk(LANG_DIR):
        if "en.json" in files:
            dirs.append(Path(root))

    for directory in sorted(dirs):
        rel_dir = directory.relative_to(LANG_DIR)
        en_path = directory / "en.json"
        en_data_raw, err = read_json(en_path)
        if err:
            print(f"  ERROR reading {en_path}: {err}")
            continue

        en_flat = flatten(en_data_raw)

        for lang in NON_EN_LANGS:
            lang_path = directory / f"{lang}.json"
            if not lang_path.exists():
                continue

            lang_data_raw, err = read_json(lang_path)
            if err:
                print(f"  ERROR reading {lang_path}: {err}")
                stats[lang]["errors"] += 1
                continue

            lang_flat = flatten(lang_data_raw)
            modified = False

            for key, en_value in en_flat.items():
                if key not in lang_flat:
                    continue

                lang_value = str(lang_flat[key]).strip()
                en_value_str = str(en_value).strip()

                # Skip if already translated
                if lang_value.lower() != en_value_str.lower():
                    continue

                # Skip empty values
                if not en_value_str:
                    continue

                # Look up translation
                if en_value_str in TRANSLATIONS:
                    trans = TRANSLATIONS[en_value_str].get(lang)
                    if trans:
                        lang_flat[key] = trans
                        modified = True
                        stats[lang]["fixed"] += 1

            if modified:
                # Write back as flat dict (files use dot-notation keys)
                write_json(lang_path, lang_flat)

    # Print summary
    print("\n=== Translation Fix Summary ===")
    total_fixed = 0
    for lang in NON_EN_LANGS:
        s = stats[lang]
        if s["fixed"] > 0 or s["errors"] > 0:
            print(f"  {lang}: {s['fixed']} fixed, {s['errors']} errors")
            total_fixed += s["fixed"]
    print(f"\nTotal keys fixed: {total_fixed}")


if __name__ == "__main__":
    fix_translations()

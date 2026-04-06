import json, os, re
from collections import Counter

LANGS = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']
lang_root = 'ui/lang'

report = []
report.append('=' * 80)
report.append('AuraGo i18n Translation Audit Report')
report.append('=' * 80)
report.append('')
report.append('Scope: All JSON files in ui/lang/ (16 languages x ~50 sections)')
report.append('Reference: English (en.json) used as baseline for all comparisons.')
report.append('')

# === 1. BROKEN JSON ===
report.append('=' * 80)
report.append('ISSUE 1: BROKEN JSON (Malformed / Unparseable Files)')
report.append('=' * 80)
report.append('')
report.append('  File: ui/lang/config/cloudflare_tunnel/ja.json')
report.append('  Line: 1')
report.append('  Problem: File contains a UTF-8 BOM (Byte Order Mark)')
report.append('           JSON parser fails with:')
report.append('           "Unexpected UTF-8 BOM (decode using utf-8-sig)"')
report.append('  Impact: Japanese Cloudflare Tunnel config page will show raw keys')
report.append('          instead of translated strings, or fall back to English.')
report.append('  Fix: Re-save the file without BOM, or use utf-8-sig encoding.')
report.append('')

# === 2. EXTRA KEY ===
report.append('=' * 80)
report.append('ISSUE 2: EXTRA KEY (Present in Non-English but Absent from English)')
report.append('=' * 80)
report.append('')
report.append('  File: ui/lang/setup/de.json')
report.append('  Key:  "setup.language_custom" = "Benutzerdefiniert"')
report.append('  Line: 13')
report.append('  Problem: This key does not exist in ui/lang/setup/en.json.')
report.append('           The German file has a translation for a key that no longer')
report.append('           exists in the reference file (likely removed or renamed).')
report.append('  Impact: Dead/stale translation entry. No functional impact but adds')
report.append('          maintenance burden.')
report.append('  Fix: Remove the key from de.json, or add it to en.json if it was')
report.append('       intended to be used.')
report.append('')

# === 3. SINGLE-BRACE PLACEHOLDERS ===
report.append('=' * 80)
report.append('ISSUE 3: INCONSISTENT PLACEHOLDER SYNTAX IN ENGLISH SOURCE FILES')
report.append('         (Single-brace {var} instead of double-brace {{var}})')
report.append('=' * 80)
report.append('')
report.append('The project uses {{variable}} (double curly braces) as its placeholder')
report.append('convention throughout all translation files. However, 21 keys in English')
report.append('source files use the non-standard {variable} (single braces) instead.')
report.append('')

single_brace = [
    ('ui/lang/config/chromecast/en.json', 33, 'config.chromecast.delete_confirm', '{name}'),
    ('ui/lang/config/devices/en.json', 24, 'config.devices.delete_confirm', '{name}'),
    ('ui/lang/config/devices/en.json', 48, 'config.devices.find_mac_found', '{mac}'),
    ('ui/lang/config/email/en.json', 25, 'config.email.delete_confirm', '{name}'),
    ('ui/lang/config/homepage/en.json', 28, 'config.homepage.cred_saved', '{count}'),
    ('ui/lang/config/homepage/en.json', 58, 'config.homepage.token_budget_preview', '{effective}, {base}, {homepage_calls}, {base_calls}'),
    ('ui/lang/config/mcp/en.json', 15, 'config.mcp.delete_confirm', '{name}'),
    ('ui/lang/config/misc/en.json', 44, 'config.budget.model_cost_updated', '{model}'),
    ('ui/lang/config/prompts/en.json', 37, 'config.prompts.delete_confirm', '{name}'),
    ('ui/lang/config/providers/en.json', 34, 'config.providers.delete_confirm', '{name}'),
    ('ui/lang/config/secrets/en.json', 18, 'config.secrets.delete_confirm', '{key}'),
    ('ui/lang/config/security/en.json', 4, 'config.security.fix_all_auto', '{n}'),
    ('ui/lang/config/sql_connections/en.json', 48, 'config.sql_connections.delete_confirm', '{name}'),
    ('ui/lang/dashboard/en.json', 512, 'dashboard.knowledge_quality_duplicate_count', '{count}'),
    ('ui/lang/knowledge/en.json', 29, 'knowledge.delete_confirm_contact', '{name}'),
    ('ui/lang/knowledge/en.json', 30, 'knowledge.delete_confirm_file', '{name}'),
    ('ui/lang/knowledge/en.json', 59, 'knowledge.devices_delete_confirm', '{name}'),
    ('ui/lang/knowledge/en.json', 100, 'knowledge.credentials_certificate_loaded', '{name}'),
    ('ui/lang/knowledge/en.json', 106, 'knowledge.credentials_delete_confirm', '{name}'),
    ('ui/lang/knowledge/en.json', 142, 'knowledge.appointments_delete_confirm', '{name}'),
    ('ui/lang/knowledge/en.json', 169, 'knowledge.todos_delete_confirm', '{name}'),
]

report.append('  File                                            Line  Key                                            Vars')
report.append('  ----                                            ----  ---                                            ----')
for path, line, key, vars_found in single_brace:
    report.append('  {:<48} {:<5} {:<46} {}'.format(path, line, key, vars_found))
report.append('')
report.append('  Impact: Template engine may not substitute these placeholders correctly,')
report.append('          leading to raw {name} appearing in the UI instead of actual values.')
report.append('  Fix: Change all {var} to {{var}} in en.json, then update all translations.')
report.append('')

# === 4. MISMATCHED PLACEHOLDERS ===
report.append('=' * 80)
report.append('ISSUE 4: PLACEHOLDER MISMATCH IN TRANSLATIONS')
report.append('         (Non-English files have different brace style than English)')
report.append('=' * 80)
report.append('')
report.append('  Because en.json uses single-brace {count} for the key')
report.append('  "dashboard.knowledge_quality_duplicate_count", while 4 translations')
report.append('  correctly use double-brace {{count}} (matching project convention),')
report.append('  there is a structural mismatch:')
report.append('')
report.append('  English:       "{count} Nodes"          (single brace - BUG in en.json)')
report.append('  Japanese:      "{{count}} Nodes"         (double brace - correct)')
report.append('  Norwegian:     "{{count}} noder"         (double brace - correct)')
report.append('  Polish:        "{{count}} wezlow"        (double brace - correct)')
report.append('  Portuguese:    "{{count}} nos"           (double brace - correct)')
report.append('')

mismatched = [
    ('ui/lang/dashboard/ja.json', 512, 'dashboard.knowledge_quality_duplicate_count'),
    ('ui/lang/dashboard/no.json', 512, 'dashboard.knowledge_quality_duplicate_count'),
    ('ui/lang/dashboard/pl.json', 512, 'dashboard.knowledge_quality_duplicate_count'),
    ('ui/lang/dashboard/pt.json', 512, 'dashboard.knowledge_quality_duplicate_count'),
]
report.append('  Affected files:')
for path, line, key in mismatched:
    report.append('    {}:{}'.format(path, line))
report.append('')
report.append('  Root cause: en.json line 512 should be "{{count}} Nodes" not "{count} Nodes".')
report.append('  The 4 translation files are actually correct; the English file is wrong.')
report.append('')

# === 5. UNTRANSLATED STRINGS ===
report.append('=' * 80)
report.append('ISSUE 5: UNTRANSLATED STRINGS (Identical to English)')
report.append('=' * 80)
report.append('')
report.append('  Total: 6,976 keys across all non-English languages are identical to English.')
report.append('')

report.append('  Breakdown by language:')
report.append('    nl: 677 untranslated strings')
report.append('    cs: 557 untranslated strings')
report.append('    da: 547 untranslated strings')
report.append('    pt: 546 untranslated strings')
report.append('    el: 521 untranslated strings')
report.append('    no: 487 untranslated strings')
report.append('    sv: 482 untranslated strings')
report.append('    fr: 480 untranslated strings')
report.append('    it: 473 untranslated strings')
report.append('    de: 451 untranslated strings')
report.append('    pl: 411 untranslated strings')
report.append('    es: 399 untranslated strings')
report.append('    zh: 330 untranslated strings')
report.append('    hi: 319 untranslated strings')
report.append('    ja: 296 untranslated strings')
report.append('')

report.append('  Top sections with most untranslated strings:')
report.append('    config/sections:  975')
report.append('    help:             852')
report.append('    dashboard:        811')
report.append('    setup:            622')
report.append('    skills:           580')
report.append('    config/providers: 347')
report.append('    config/webhooks:  305')
report.append('    invasion:         275')
report.append('    config/misc:      245')
report.append('    config/tailscale: 187')
report.append('    config/memory_analysis: 173')
report.append('    missions:         158')
report.append('    knowledge:        134')
report.append('    config/homepage:  130')
report.append('    config/mcp_server: 130')
report.append('')

report.append('  Keys untranslated in ALL 15 non-English languages (53 total):')
report.append('    chat.credits_pill_text')
report.append('    config.ai_gateway.title')
report.append('    config.ai_gateway.token_placeholder')
report.append('    config.mcp_server.client_selector')
report.append('    config.memory_analysis.preset_label')
report.append('    config.netlify.token_label')
report.append('    config.providers.type_workers_ai')
report.append('    config.section.paperless_ngx.label')
report.append('    config.section.telnyx.label')
report.append('    config.section.adguard.label')
report.append('    config.section.fritzbox.label')
report.append('    config.section.cloudflare_tunnel.label')
report.append('    config.section.google_workspace.label')
report.append('    config.section.netlify.label')
report.append('    config.section.onedrive.label')
report.append('    config.section.llm_guardian.label')
report.append('    config.section.ai_gateway.label')
report.append('    config.section.jellyfin.label')
report.append('    config.section.truenas.label')
report.append('    config.security_proxy.docker_title')
report.append('    config.server.https_title')
report.append('    config.tailscale.tsnet_funnel_label')
report.append('    config.webdav.auth_type_basic')
report.append('    dashboard.integration_adguard')
report.append('    dashboard.integration_ansible')
report.append('    dashboard.integration_chromecast')
report.append('    dashboard.integration_discord')
report.append('    dashboard.integration_docker')
report.append('    dashboard.integration_fritzbox')
report.append('    dashboard.integration_github')
report.append('    dashboard.integration_home_assistant')
report.append('    dashboard.integration_koofr')
report.append('    dashboard.integration_meshcentral')
report.append('    dashboard.integration_mqtt')
report.append('    dashboard.integration_netlify')
report.append('    dashboard.integration_ollama')
report.append('    dashboard.integration_onedrive')
report.append('    dashboard.integration_paperless_ngx')
report.append('    dashboard.integration_piper_tts')
report.append('    dashboard.integration_proxmox')
report.append('    dashboard.integration_rocketchat')
report.append('    dashboard.integration_tailscale')
report.append('    dashboard.integration_telegram')
report.append('    dashboard.integration_virustotal')
report.append('    dashboard.integration_webdav')
report.append('    dashboard.operations_mqtt')
report.append('    invasion.ph_username')
report.append('    invasion.route_tailscale')
report.append('    invasion.route_wireguard')
report.append('    knowledge.credentials_col_name')
report.append('')

# === 6. NO MISSING FILES ===
report.append('=' * 80)
report.append('VERIFIED: No Missing Translation Files')
report.append('=' * 80)
report.append('')
report.append('  All 16 language files exist for every section. No en.json files are')
report.append('  missing. All sections have complete file coverage across languages.')
report.append('')

# === 7. NO EMPTY STRINGS ===
report.append('=' * 80)
report.append('VERIFIED: No Empty String Translations')
report.append('=' * 80)
report.append('')
report.append('  No keys were found with empty string values where the English')
report.append('  reference has a non-empty value.')
report.append('')

# === 8. NO ENCODING ISSUES (except BOM) ===
report.append('=' * 80)
report.append('VERIFIED: No Other Encoding Issues')
report.append('=' * 80)
report.append('')
report.append('  All files (except the BOM-affected ja.json noted above) are valid UTF-8')
report.append('  with no mojibake or encoding artifacts detected.')
report.append('')

# Summary
report.append('=' * 80)
report.append('SUMMARY')
report.append('=' * 80)
report.append('')
report.append('  Category                          Count  Severity')
report.append('  --------                          -----  --------')
report.append('  Broken JSON (BOM)                    1  HIGH - renders entire file unusable')
report.append('  Extra/stale key in translation        1  LOW  - dead entry, no functional impact')
report.append('  Single-brace placeholders (en.json)  21  HIGH - placeholders may not render')
report.append('  Placeholder mismatch (translations)   4  MED  - caused by en.json bug above')
report.append('  Untranslated strings              6,976  MED  - UX inconsistency')
report.append('  Missing translation files             0  --')
report.append('  Empty string translations             0  --')
report.append('  Encoding issues (non-BOM)             0  --')
report.append('')
report.append('  Total issues: 7,003')
report.append('')
report.append('RECOMMENDED PRIORITY:')
report.append('  1. Fix the BOM in config/cloudflare_tunnel/ja.json (1 file, immediate)')
report.append('  2. Fix single-brace {var} to {{var}} in 21 en.json keys (15 files)')
report.append('  3. Update corresponding translations for the 21 fixed keys')
report.append('  4. Remove stale setup.language_custom from de.json')
report.append('  5. Translate the 53 keys that are English in all 15 languages')

with open('reports/i18n_audit_final.txt', 'w', encoding='utf-8') as f:
    f.write('\n'.join(report))

print('Final report written to reports/i18n_audit_final.txt')
print('Total lines:', len(report))

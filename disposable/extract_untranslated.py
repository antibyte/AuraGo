#!/usr/bin/env python3
"""Extract all unique untranslated English values from the translation files."""
import json, os, re
from pathlib import Path

LANG_DIR = Path("ui/lang")
OUT = Path("disposable/untranslated_values.json")
LANGUAGES = ["cs","da","de","el","en","es","fr","hi","it","ja","nl","no","pl","pt","sv","zh"]
NON_EN = [l for l in LANGUAGES if l != "en"]

IDENTITY_WORDS = {
    "ok","token","api","url","ip","mac","nas","iot","vm","mqtt","tls","ssl","sse","grpc",
    "http","https","ssh","docker","podman","ollama","netlify","gotenberg","chromecast",
    "telnyx","discord","github","virustotal","brave","oauth2","totp","csrf","json","yaml",
    "csv","html","svg","png","model","models","port","host","backend","provider","client",
    "server","container","router","switch","printer","camera","generic","system","info",
    "id","name","type","key","value","description","password","username","input","output",
    "context","tags","actions","deploy","deployment","sandbox","landlock","openrouter",
    "openai","google","anthropic","workers-ai","custom","restart","config","dashboard",
    "invasion","chat","missions","plans","containers","loading...","loading…","sites",
    "system","timeout","pool size","no pooling","native host","delete","set","unchanged",
    "sk-or-...","cf-aig-...","my-gateway",
}

def is_allowed(val):
    v = val.strip().lower()
    if not v: return True
    if v in IDENTITY_WORDS: return True
    if len(v) <= 3 and v.isascii(): return True
    if re.match(r'^[\$\{\}\(\)\/\s\.\:]+$', val): return True
    if re.match(r'^\$\{\{.*\}\}', val): return True
    if re.match(r'^\{\{.*\}\}$', val): return True
    return False

# Collect: english_value -> list of (section, lang, key) 
untranslated = {}
for root, _, files in os.walk(LANG_DIR):
    if "en.json" not in files: continue
    d = Path(root)
    section = str(d.relative_to(LANG_DIR))
    with open(d / "en.json", "r", encoding="utf-8-sig") as f:
        en = json.load(f)
    for lang in NON_EN:
        lp = d / f"{lang}.json"
        if not lp.exists(): continue
        try:
            with open(lp, "r", encoding="utf-8-sig") as f:
                ld = json.load(f)
        except: continue
        for k, ev in en.items():
            if k in ld:
                lv = str(ld[k]).strip()
                evs = str(ev).strip()
                if lv.lower() == evs.lower() and evs and not is_allowed(evs):
                    if evs not in untranslated:
                        untranslated[evs] = []
                    untranslated[evs].append({"section": section, "lang": lang, "key": k})

# Write output
vals = sorted(untranslated.keys())
result = {
    "total_unique_values": len(vals),
    "total_occurrences": sum(len(v) for v in untranslated.values()),
    "values": {v: len(untranslated[v]) for v in vals}
}
with open(OUT, "w", encoding="utf-8") as f:
    json.dump(result, f, ensure_ascii=False, indent=2)

print(f"Unique untranslated English values: {len(vals)}")
print(f"Total occurrences across all files: {result['total_occurrences']}")
print(f"Output written to: {OUT}")

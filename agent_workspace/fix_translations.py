import json
from pathlib import Path

# Languages with BOM issues in cloudflare_tunnel
BOM_LANGS = ['cs', 'da', 'el', 'es', 'fr', 'hi', 'it', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']

# Missing keys to add to cloudflare_tunnel files (from de.json translations)
MISSING_LOOPBACK = {
    "config.cloudflare_tunnel.loopback_label": "Loopback HTTP port for cloudflared (avoids TLS)",
    "config.cloudflare_tunnel.loopback_hint": "Opens a plain HTTP port on 127.0.0.1 only. cloudflared connects to this instead of the HTTPS port — no certificate verification needed. The port is assigned automatically."
}

# Fix cloudflare_tunnel files
for lang in BOM_LANGS:
    path = Path(f'ui/lang/config/cloudflare_tunnel/{lang}.json')
    # Read with utf-8-sig to handle BOM
    with open(path, 'r', encoding='utf-8-sig') as f:
        data = json.load(f)
    
    # Add missing keys
    for k, v in MISSING_LOOPBACK.items():
        if k not in data:
            data[k] = v
    
    # Write back without BOM, using utf-8
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
        f.write('\n')
    print(f"Fixed {path}")

# Fix dashboard/cs.json typo
path = Path('ui/lang/dashboard/cs.json')
with open(path, 'r', encoding='utf-8') as f:
    data = json.load(f)

if 'dashboard.memory_curar_contradictions' in data:
    val = data.pop('dashboard.memory_curar_contradictions')
    data['dashboard.memory_curator_fact_contradictions'] = val
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
        f.write('\n')
    print(f"Fixed typo in {path}")

# Fix truenas extra keys - remove port_label, port_placeholder, saving from all non-en files
TRUENAS_EXTRA_KEYS = [
    'config.truenas.port_label',
    'config.truenas.port_placeholder', 
    'config.truenas.saving'
]

for lang in ['cs', 'da', 'de', 'el', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']:
    path = Path(f'ui/lang/config/truenas/{lang}.json')
    with open(path, 'r', encoding='utf-8') as f:
        data = json.load(f)
    
    removed = False
    for k in TRUENAS_EXTRA_KEYS:
        if k in data:
            del data[k]
            removed = True
    
    if removed:
        with open(path, 'w', encoding='utf-8') as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
            f.write('\n')
        print(f"Removed extra keys from {path}")

print("Done!")

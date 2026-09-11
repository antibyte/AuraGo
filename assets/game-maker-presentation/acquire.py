"""Fetch reviewed CC0 production inputs. Runtime games never call these URLs."""
import hashlib
import json
from pathlib import Path
import re
import urllib.request
from urllib.parse import urljoin

ROOT = Path(__file__).resolve().parent / 'sources'
SOURCES = {
    'impact': ('https://kenney.nl/assets/impact-sounds', '.zip', 'Kenney'),
    'rpg': ('https://kenney.nl/assets/rpg-audio', '.zip', 'Kenney'),
    'rain': ('https://opengameart.org/content/amb-rain-loop-2', '.ogg', 'Kresiek The Furry'),
    'crickets': ('https://opengameart.org/content/crickets-ambient-noise-loopable', '.mp3', 'Ted Kerr / Wolfgang_'),
    'forest': ('https://opengameart.org/content/ambient-bird-sounds', '.ogg', 'isaiah658'),
}

def main():
    ROOT.mkdir(parents=True, exist_ok=True)
    lock_path = ROOT / 'sources.json'
    locked = json.loads(lock_path.read_text()) if lock_path.exists() else {}
    for name, (page, ext, author) in SOURCES.items():
        path = ROOT / (name + ext)
        if path.exists() and name in locked:
            assert hashlib.sha256(path.read_bytes()).hexdigest() == locked[name]['sha256'], name
            continue
        html = urllib.request.urlopen(page, timeout=40).read().decode()
        if not re.search(r'CC0', html, re.I):
            raise ValueError('CC0 declaration missing: ' + page)
        links = re.findall(r'href=[\"\x27]([^\"\x27]+)[\"\x27]', html)
        url = next(urljoin(page, link) for link in links if link.endswith(ext))
        data = urllib.request.urlopen(url, timeout=120).read()
        path.write_bytes(data)
        locked[name] = dict(page=page, url=url, author=author, license='CC0-1.0', sha256=hashlib.sha256(data).hexdigest())
        lock_path.write_text(json.dumps(locked, indent=2) + '\n', encoding='utf-8')
        print(name, len(data), flush=True)

if __name__ == '__main__':
    main()

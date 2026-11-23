"""Explicit refresh of public, mode-scoped augment names for match history.

Run from the repository root. Application rendering uses the bundled names.
"""
import hashlib
import html
import json
from pathlib import Path
import re
import urllib.request

ROOT = Path(__file__).resolve().parent.parent


def collect_names(value, names, depth=0):
    if depth > 35:
        return
    if isinstance(value, dict):
        rarity = value.get('rarity', value.get('rareity'))
        if isinstance(value.get('id'), int) and rarity in (0, 1, 4, 8) and 'desc' in value:
            name = value.get('name')
            if isinstance(name, str):
                name = html.unescape(re.sub('<[^>]*>', '', name)).strip()
                if name and len(name) < 100:
                    names[str(value['id'])] = name
        for child in value.values():
            collect_names(child, names, depth + 1)
    elif isinstance(value, list):
        for child in value:
            collect_names(child, names, depth + 1)


def fetch_names(mode):
    url = f'https://op.gg/zh-cn/lol/modes/{mode}/thresh/augments'
    request = urllib.request.Request(url, headers={
        'User-Agent': 'LeaguePingo/1 (+https://github.com/ZJing1125/LeaguePingo)',
    })
    with urllib.request.urlopen(request, timeout=12) as response:
        raw = response.read(3 * 1024 * 1024 + 1)
    if len(raw) > 3 * 1024 * 1024:
        raise ValueError('Public augment page exceeds size limit')
    names = {}
    for script in re.findall(r'self\.__next_f\.push\((\[.*?\])\)</script>', raw.decode()):
        data = json.loads(script)
        if len(data) < 2 or not isinstance(data[1], str):
            continue
        for line in data[1].split('\n'):
            record = line.partition(':')[2]
            try:
                collect_names(json.loads(record), names)
            except (ValueError, TypeError):
                continue
    if len(names) < 20:
        raise ValueError(f'No verified augment names for {mode}')
    return names, {'url': url, 'pageSha256': hashlib.sha256(raw).hexdigest(), 'count': len(names)}


if __name__ == '__main__':
    catalog, sources = {}, {}
    for mode in ('aram-mayhem', 'arena'):
        catalog[mode], sources[mode] = fetch_names(mode)
        print(mode, len(catalog[mode]))
    output = ROOT / 'frontend/src/assets/game'
    (output / 'match-augments.json').write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    (output / 'match-augments-sources.json').write_text(json.dumps(sources, indent=2) + '\n', encoding='utf-8')

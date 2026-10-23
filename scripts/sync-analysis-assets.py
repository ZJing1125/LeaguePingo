"""Explicit maintenance of official rune art and Summoner's Rift minimap.

Python standard library only. Never invoked by the application or build.
"""
import concurrent.futures
import hashlib
import json
from pathlib import Path
import urllib.request

root = Path(__file__).resolve().parent.parent / "frontend/src/assets/game"
version = json.loads((root / "catalog.json").read_text())["version"]
base = "https://ddragon.leagueoflegends.com/cdn"

def fetch(url):
    with urllib.request.urlopen(url, timeout=35) as response:
        data = response.read(2 * 1024 * 1024 + 1)
    if len(data) > 2 * 1024 * 1024:
        raise ValueError("Asset exceeds size bound")
    return data

source = f"{base}/{version}/data/zh_CN/runesReforged.json"
styles = json.loads(fetch(source))
icons = {style["id"]: style["icon"] for style in styles}
for style in styles:
    for slot in style["slots"]:
        for rune in slot["runes"]:
            icons[rune["id"]] = rune["icon"]

out = root / "analysis"
out.mkdir(parents=True, exist_ok=True)

def save(task):
    key, url = task
    data = fetch(url)
    if not data.startswith(b"\x89PNG\r\n\x1a\n"):
        raise ValueError(f"Not an original PNG: {key}")
    (out / (key + ".png")).write_bytes(data)
    return key, {"url": url, "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}

tasks = [(str(key), f"{base}/img/{icon}") for key, icon in icons.items()]
tasks.append(("map11", f"{base}/{version}/img/map/map11.png"))
with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
    images = dict(pool.map(save, tasks))
(out / "runes.json").write_text(json.dumps(styles, ensure_ascii=False, indent=2) + "\n")
(out / "manifest.json").write_text(json.dumps({"version": version, "source": source, "images": images}, indent=2) + "\n")
print(f"Bundled {len(images)} original analysis images, {sum(x['bytes'] for x in images.values())} bytes")

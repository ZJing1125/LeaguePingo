"""Explicit maintenance: download original Riot UI art; never run at app/build time.

Python 3 standard library only. Images are preserved byte-for-byte, not redrawn.
"""
import argparse
import concurrent.futures
import hashlib
import io
import json
from pathlib import Path
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parent.parent / "frontend/src/assets/game"
parser = argparse.ArgumentParser()
parser.add_argument("--rank-archive", type=Path)
args = parser.parse_args()
catalog = json.loads((ROOT / "catalog.json").read_text())
rank_url = "https://static.developer.riotgames.com/docs/lol/ranked-emblems-latest.zip"
def fetch(url):
    with urllib.request.urlopen(url, timeout=40) as response:
        return response.read(80 * 1024 * 1024 + 1)

def save(folder, filename, data, source):
    (ROOT / folder).mkdir(parents=True, exist_ok=True)
    (ROOT / folder / filename).write_bytes(data)
    return {"url": source, "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}

def splash(entry):
    key, champion = entry
    name = champion["full"].removesuffix(".png")
    if not name.isalnum():
        raise ValueError("Invalid champion asset name")
    url = f"https://ddragon.leagueoflegends.com/cdn/img/champion/splash/{name}_0.jpg"
    data = fetch(url)
    if len(data) > 2 * 1024 * 1024 or not data.startswith(b"\xff\xd8"):
        raise ValueError(f"Invalid splash: {key}")
    return key, save("splashes", f"{key}.jpg", data, url)

with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
    images = dict(pool.map(splash, catalog["champions"].items()))
(ROOT / "splashes/manifest.json").write_text(json.dumps({
    "catalogVersion": catalog["version"], "source": "Riot Data Dragon classic splash art", "images": images,
}, indent=2) + "\n")
archive = args.rank_archive.read_bytes() if args.rank_archive else fetch(rank_url)
emblems = {}
with zipfile.ZipFile(io.BytesIO(archive)) as bundle:
    for tier in ["Iron", "Bronze", "Silver", "Gold", "Platinum", "Emerald", "Diamond", "Master", "Grandmaster", "Challenger"]:
        member = f"Ranked Emblems Latest/Rank={tier}.png"
        data = bundle.read(member)
        if len(data) > 2 * 1024 * 1024 or not data.startswith(b"\x89PNG\r\n\x1a\n"):
            raise ValueError(f"Invalid emblem: {tier}")
        emblems[tier.upper()] = save("ranks", tier.upper() + ".png", data, rank_url + "#" + member)
(ROOT / "ranks/manifest.json").write_text(json.dumps({
    "source": "Riot official ranked-emblems-latest.zip", "images": emblems,
}, indent=2) + "\n")
print(f"Bundled {len(images)} original splash images and {len(emblems)} official ranked emblems")
print(f"Total: {sum(i['bytes'] for i in images.values()) + sum(i['bytes'] for i in emblems.values())} bytes")

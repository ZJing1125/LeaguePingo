// Explicit asset maintenance only. No application/build-time network requests.
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { createHash } from "node:crypto";
const root = new URL("../frontend/src/assets/game/", import.meta.url);
const catalog = JSON.parse(await readFile(new URL("catalog.json", root)));
const dir = new URL("portraits/", root);
await mkdir(dir, { recursive: true });
const entries = Object.entries(catalog.champions);
const manifest = {
  catalogVersion: catalog.version,
  source: "Riot Games Data Dragon classic loading art",
  images: {},
};
async function worker() {
  while (entries.length) {
    const [id, champion] = entries.shift();
    const name = champion.full.replace(/\.png$/, "");
    if (!/^[A-Za-z0-9]+$/.test(name))
      throw Error("Unexpected champion filename");
    const url = `https://ddragon.leagueoflegends.com/cdn/img/champion/loading/${name}_0.jpg`;
    const response = await fetch(url, {
      signal: AbortSignal.timeout(30000),
      redirect: "error",
    });
    if (!response.ok) throw Error(`${name}: HTTP ${response.status}`);
    const bytes = Buffer.from(await response.arrayBuffer());
    if (bytes.length > 512 * 1024 || bytes[0] !== 0xff || bytes[1] !== 0xd8)
      throw Error(`${name}: invalid JPEG`);
    await writeFile(new URL(`${id}.jpg`, dir), bytes);
    manifest.images[id] = {
      url,
      bytes: bytes.length,
      sha256: createHash("sha256").update(bytes).digest("hex"),
    };
  }
}
await Promise.all(Array.from({ length: 4 }, worker));
await writeFile(
  new URL("manifest.json", dir),
  JSON.stringify(manifest, null, 2) + "\n",
);
console.log(
  `Bundled ${Object.keys(manifest.images).length} portraits, ${Object.values(manifest.images).reduce((a, i) => a + i.bytes, 0)} bytes`,
);

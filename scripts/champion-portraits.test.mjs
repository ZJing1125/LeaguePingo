import { test } from "node:test";
import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { createHash } from "node:crypto";
test("packaged portraits match the supported champion catalog and integrity manifest", async () => {
  const root = new URL("../frontend/src/assets/game/", import.meta.url);
  const catalog = JSON.parse(await readFile(new URL("catalog.json", root)));
  const manifest = JSON.parse(
    await readFile(new URL("portraits/manifest.json", root)),
  );
  assert.equal(manifest.catalogVersion, catalog.version);
  assert.deepEqual(
    Object.keys(manifest.images).sort(),
    Object.keys(catalog.champions).sort(),
  );
  assert.equal(
    (await readdir(new URL("portraits/", root))).length,
    Object.keys(manifest.images).length + 1,
  );
  let total = 0;
  for (const [id, entry] of Object.entries(manifest.images)) {
    const bytes = await readFile(new URL(`portraits/${id}.jpg`, root));
    assert.equal(bytes.length, entry.bytes);
    assert.equal(bytes.readUInt16BE(0), 0xffd8);
    assert.equal(
      createHash("sha256").update(bytes).digest("hex"),
      entry.sha256,
    );
    assert.equal(
      entry.url,
      `https://ddragon.leagueoflegends.com/cdn/img/champion/loading/${catalog.champions[id].full.replace(/\.png$/, "")}_0.jpg`,
    );
    total += bytes.length;
  }
  assert(total < 12 * 1024 * 1024, "portrait bundle exceeds its size budget");
});

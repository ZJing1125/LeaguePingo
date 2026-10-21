import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
test('official emblems and HD splash images retain source integrity within the UI asset budget', async () => {
  const root = new URL('../frontend/src/assets/game/', import.meta.url);
  const catalog = JSON.parse(await readFile(new URL('catalog.json', root)));
  let total = 0;
  for (const kind of ['splashes', 'ranks']) {
    const manifest = JSON.parse(await readFile(new URL(`${kind}/manifest.json`, root)));
    if (kind === 'splashes') { assert.equal(manifest.catalogVersion, catalog.version); assert.deepEqual(Object.keys(manifest.images).sort(), Object.keys(catalog.champions).sort()); }
    else assert.deepEqual(Object.keys(manifest.images).sort(), ['IRON','BRONZE','SILVER','GOLD','PLATINUM','EMERALD','DIAMOND','MASTER','GRANDMASTER','CHALLENGER'].sort());
    for (const [id, entry] of Object.entries(manifest.images)) {
      const b = await readFile(new URL(`${kind}/${id}.${kind === 'ranks' ? 'png' : 'jpg'}`, root));
      assert.equal(b.length, entry.bytes); assert.equal(createHash('sha256').update(b).digest('hex'), entry.sha256); total += b.length;
      if (kind === 'ranks') { assert(entry.url.startsWith('https://static.developer.riotgames.com/docs/lol/ranked-emblems-latest.zip#')); assert.equal(b.readUInt32BE(16),1000); assert.equal(b.readUInt32BE(20),1000); }
      else {
        assert.equal(entry.url, `https://ddragon.leagueoflegends.com/cdn/img/champion/splash/${catalog.champions[id].full.replace(/\.png$/, '')}_0.jpg`); assert.equal(b.readUInt16BE(0),0xffd8);
        let found = false;
        for (let at = 2; at + 9 < b.length;) {
          assert.equal(b[at++],0xff); while(b[at]===0xff) at++;
          const marker=b[at++]; if(marker===0xda||marker===0xd9) break;
          const size=b.readUInt16BE(at);
          if([0xc0,0xc1,0xc2].includes(marker)) { assert(b.readUInt16BE(at+5)>=1000 && b.readUInt16BE(at+3)>=600, 'banner source is not HD landscape art'); found=true;break; }
          at+=size;
        }
        assert(found,'JPEG dimensions unavailable');
      }
    }
  }
  assert(total < 50*1024*1024, 'UI art bundle exceeds 50 MiB');
});

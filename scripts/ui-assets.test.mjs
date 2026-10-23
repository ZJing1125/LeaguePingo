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

test('analysis rune trees and minimap are local original official images with a bounded budget',async()=>{
 const root=new URL('../frontend/src/assets/game/analysis/',import.meta.url);
 const manifest=JSON.parse(await readFile(new URL('manifest.json',root)));
 const catalog=JSON.parse(await readFile(new URL('../catalog.json',root)));
 const styles=JSON.parse(await readFile(new URL('runes.json',root)));
 assert.equal(manifest.version,catalog.version);
 assert.equal(manifest.source,`https://ddragon.leagueoflegends.com/cdn/${catalog.version}/data/zh_CN/runesReforged.json`);
 const ids=styles.flatMap(style=>[style.id,...style.slots.flatMap(slot=>slot.runes.map(r=>r.id))]).map(String);
 assert.deepEqual(Object.keys(manifest.images).sort(),[...ids,'map11'].sort());let bytes=0;
 for(const [id,entry] of Object.entries(manifest.images)){
  const image=await readFile(new URL(`${id}.png`,root));assert.equal(image.length,entry.bytes);assert.equal(createHash('sha256').update(image).digest('hex'),entry.sha256);assert.equal(image.subarray(0,8).toString('hex'),'89504e470d0a1a0a');assert(entry.url.startsWith('https://ddragon.leagueoflegends.com/cdn/'));bytes+=image.length;
 }
 assert(bytes<3*1024*1024,'analysis assets exceed 3MiB');
});

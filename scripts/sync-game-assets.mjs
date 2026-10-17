// Explicit maintenance step, never run at application startup or in a match.
// Downloads public Riot Data Dragon metadata and sprites, not executable code.
import { mkdir, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { createHash } from 'node:crypto'
const version = process.argv[2]
if (!/^\d+\.\d+\.\d+$/.test(version ?? '')) throw Error('Pass a verified Data Dragon version, e.g. 16.20.1')
const root = new URL('../frontend/src/assets/game/', import.meta.url)
await mkdir(root, { recursive: true })
async function download(path) {
  const response = await fetch(`https://ddragon.leagueoflegends.com/cdn/${version}/${path}`, { signal: AbortSignal.timeout(30000) })
  if (!response.ok) throw Error(`Data Dragon returned ${response.status}`)
  const bytes = Buffer.from(await response.arrayBuffer())
  if (bytes.length > 8 * 1024 * 1024) throw Error('Resource size limit exceeded')
  return bytes
}
const catalog = { version, locale: 'zh_CN', champions: {}, items: {}, spells: {}, sheets: {} }
for (const [kind, file] of [['champions', 'champion'], ['items', 'item'], ['spells', 'summoner']]) {
  const data = JSON.parse(await download(`data/zh_CN/${file}.json`)).data
  for (const [key, entry] of Object.entries(data)) {
    const id = kind === 'items' ? key : entry.key
    if (!entry.image || !/^\w+\.png$/.test(entry.image.sprite)) continue
    catalog[kind][id] = { name: entry.name, title: kind === 'champions' ? entry.title : undefined, ...entry.image }
  }
}
const sprites = [...new Set(['champions', 'items', 'spells'].flatMap(k => Object.values(catalog[k]).map(e => e.sprite)))]
for (const sprite of sprites) {
  const bytes = await download(`img/sprite/${sprite}`)
  if (bytes.subarray(0, 8).toString('hex') !== '89504e470d0a1a0a') throw Error('Expected PNG')
  catalog.sheets[sprite] = { width: bytes.readUInt32BE(16), height: bytes.readUInt32BE(20), sha256: createHash('sha256').update(bytes).digest('hex') }
  await writeFile(new URL(sprite, root), bytes)
}
await writeFile(new URL('catalog.json', root), JSON.stringify(catalog))
console.log(`Synced ${version}: ${Object.keys(catalog.champions).length} champions, ${sprites.length} sprite sheets to ${fileURLToPath(root)}`)

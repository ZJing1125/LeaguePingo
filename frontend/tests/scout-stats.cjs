// Actual production TS modules, using the project's existing compiler.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const Module = require('node:module');
const ts = require('typescript');
const cache = new Map();
function load(file) {
  const filename = path.resolve(__dirname, '../src', file + '.ts');
  if (cache.has(filename)) return cache.get(filename).exports;
  const mod = new Module(filename, module); cache.set(filename, mod); mod.filename = filename;
  mod.require = id => { assert.equal(id, './matchStats'); return load(id.slice(2)); };
  mod._compile(ts.transpileModule(fs.readFileSync(filename, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, filename);
  return mod.exports;
}
const { scoutStats, rankedMode } = load('scoutStats');
const stats = (n, rate = 60) => ({ games: n, wins: n * rate / 100, losses: n * (100-rate) / 100, outcomeGames: n, kdaGames: n, winRate: rate, kda: 3 });
const player = {
  championId: 23, identityState: 'known', history: Array.from({ length: 20 }, () => ({ championId: 25, queueId: 450, win: false })),
  scout: {
    state: 'ready', fetchedAt: '2026-10-21T12:00:00Z', season: { name: '2026' }, complete: true, complete12: true, complete30: true, read: 1600,
    champions: [{ championId: 23, games: 1100, mastery: { championId: 23, level: 33, points: 900000 }, queues: [{ queueId: 420, stats: stats(800), average: [8, 3, 7] }, { queueId: 440, stats: stats(200, 40), average: [6, 4, 5] }] }],
    queues: [{ queueId: 420, twelve: stats(10), month: stats(100), positions: [{ position: 'TOP', games: 70 }, { position: 'JUNGLE', games: 30 }] }, { queueId: 440, twelve: stats(5, 40), month: stats(50, 40), positions: [{ position: 'JUNGLE', games: 30 }, { position: 'MIDDLE', games: 20 }] }],
  },
};
let d = scoutStats(player, 420);
assert.equal(d.hero.games, 1100); assert.equal(d.champion.games, 800); assert.equal(d.periods[1].games, 100); assert.equal(d.role.key, 'TOP'); assert.equal(d.role.share, 70);
assert(d.tags.some(t => t.label.includes('1100'))); assert(d.tags.some(t => t.label.includes('33')));
d = scoutStats(player, 440); assert.equal(d.champion.games, 200); assert.equal(d.periods[0].games, 5); assert.equal(d.role.key, 'JUNGLE'); assert.equal(d.overall.winRate, 40);
assert.equal(rankedMode(440), '灵活组排');
d = scoutStats({ ...player, scout: undefined }); assert.equal(d.champion.games, 0); assert.equal(d.overall.winRate, null); assert.equal(d.tags.length, 0, 'recent history cannot stand in for hero experience');
d = scoutStats({ ...player, identityState: 'unavailable' }); assert.equal(d.hero, undefined); assert.equal(d.tags.length, 0, 'unknown identity cannot display retained experience');
const copy = structuredClone(player); copy.scout.queues[0].positions = [{ position: 'TOP', games: 5 }, { position: 'JUNGLE', games: 5 }];
assert.equal(scoutStats(copy).role.key, '', 'a tied position is not a main role');
copy.scout.queues[0].month = { ...stats(3, 0), outcomeGames: 0, winRate: null }; assert.equal(scoutStats(copy).overall.winRate, null);
copy.scout.queues[0].month = stats(3, 0); assert.equal(scoutStats(copy).overall.winRate, 0, 'real zero differs from missing outcomes');
const unknownSeason = structuredClone(player); unknownSeason.scout.season = null;
assert.equal(scoutStats(unknownSeason).champion.games, 0, 'unverified season cannot label hero ranked records as current season');
assert.equal(scoutStats(unknownSeason).periods[1].games, 100, 'verified time-window report remains independent of season metadata');
const sparse = structuredClone(player); sparse.scout.queues[0].positions = [{ position: 'TOP', games: 2 }];
assert.equal(scoutStats(sparse).role.label, '位置记录较少', 'two records in the same role are not a scattered role distribution');
console.log('PASS: independent season hero experience, mastery, ranked queue/window/role switching, no list-sample fallback, unknown identity and zero vs missing.');

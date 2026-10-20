// Pure production TS modules, compiled with the project's existing compiler.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const Module = require("node:module");
const ts = require("typescript");
const cache = new Map();
function load(file) {
  const filename = path.resolve(__dirname, "../src", file + ".ts");
  if (cache.has(filename)) return cache.get(filename).exports;
  const mod = new Module(filename, module);
  cache.set(filename, mod);
  mod.filename = filename;
  mod.require = id => {
    assert(["./matchStats"].includes(id), `Unexpected runtime dependency: ${id}`);
    return load(id.slice(2));
  };
  mod._compile(ts.transpileModule(fs.readFileSync(filename, "utf8"), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText, filename);
  return mod.exports;
}
const { scoutStats } = load("scoutStats");
const anchor = Date.parse("2026-10-20T12:00:00Z");
function match(i, overrides = {}) {
  return { gameId: i, championId: 23, queueId: 420, mapId: 11, position: "TOP", playedAt: new Date(anchor - i * 3600000).toISOString(), win: true, kills: 6, deaths: 3, assists: 9, duration: 1800, cs: 210, ...overrides };
}
function player(history, overrides = {}) {
  return { championId: 23, history, historyMeta: { fetchedAt: new Date(anchor).toISOString() }, ...overrides };
}
let data = scoutStats(player([
  match(0), match(12, { win: false }), match(13, { win: null }),
  match(720), match(721), match(-1), match(1, { playedAt: "invalid", win: null }),
]));
assert.equal(data.periods[0].games, 2, "12h boundary inclusive, future/invalid dates excluded");
assert.equal(data.periods[0].winRate, 50);
assert.equal(data.periods[1].games, 4, "30d boundary inclusive, outside sample excluded");
assert.equal(data.periods[1].outcomeGames, 3, "unknown result cannot count as a loss");
assert.equal(data.periods[1].losses, 1);
assert.equal(data.periods[1].winRate, 200 / 3);
data = scoutStats(player([match(0, { win: null, kills: null }), match(1, { win: null, deaths: null })]));
assert.equal(data.overall.winRate, null);
assert.equal(data.champion.average, null);
assert.equal(data.role.key, "", "two games must not assert a main role");
data = scoutStats(player([match(0, { win: false }), match(1, { win: false })]));
assert.equal(data.overall.winRate, 0, "actual 0% must remain distinct from unknown");
data = scoutStats(player([match(0), match(1), match(2)], { historyMeta: {} }));
assert.equal(data.periods[0].timed, false);
assert.equal(data.periods[0].winRate, null, "missing fetch time cannot use wall clock or latest match as a substitute");
data = scoutStats(player([
  match(0, { position: "", lane: "TOP" }), match(1, { position: "", lane: "TOP" }), match(2, { position: "", lane: "TOP" }),
  match(3, { queueId: 450, mapId: 12 }), match(4, { queueId: 450, mapId: 12 }), match(5, { queueId: 450, mapId: 12 }),
]));
assert.equal(data.role.known, 0, "client lane and non-rift games cannot establish a historical role");
data = scoutStats(player([match(0), match(1), match(2, { position: "JUNGLE" }), match(3, { position: "JUNGLE" })]));
assert.equal(data.role.key, "", "tied roles cannot be resolved by source ordering");
data = scoutStats(player(Array.from({ length: 30 }, (_, i) => match(i))));
assert.equal(data.overall.games, 20);
assert.equal(data.role.key, "TOP");
assert.equal(data.role.share, 100);
assert.equal(data.periods[0].games, 13);
assert.equal(data.periods[1].games, 20, "period count must not claim complete 30-day history");
assert(data.tags.some(t => t.label === "近期 20 连胜"));
assert(data.tags.some(t => t.label === "补刀 7.0/分"));
data = scoutStats(player([match(0), match(1, { win: null }), match(2)]));
assert(!data.tags.some(t => t.label.includes("连胜")), "unknown result must interrupt the winning streak");
data = scoutStats(player([match(0, { kills: 10, deaths: 1, assists: 10 }), match(1, { kills: 0, deaths: 9, assists: 0 })]));
assert.deepEqual(data.champion.average, [5, 5, 5]);
assert.equal(data.champion.kda, 2, "aggregate KDA must not become average of individual ratios");
assert.equal(scoutStats(player([match(0)], { championId: 0 })).champion.games, 0);
assert(scoutStats(player(Array.from({ length: 10 }, (_, i) => match(i)), { championId: 25 })).tags.some(t => t.heroId === 23 && t.label === "常用 10 场"));
assert.equal(scoutStats(player([])).tags.length, 0);
console.log("PASS: scout sample boundaries, unknown vs zero, time anchors, role evidence/ties, first-20 limits, KDA and factual tags.");

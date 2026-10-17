// Explicit development maintenance; no assets are downloaded at app runtime.
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
const catalog = JSON.parse(
  await readFile(
    new URL("../frontend/src/assets/game/catalog.json", import.meta.url),
    "utf8",
  ),
);
if (!/^\d+\.\d+\.\d+$/.test(catalog.version))
  throw Error("Invalid catalog version");
const response = await fetch(
  `https://ddragon.leagueoflegends.com/cdn/${catalog.version}/data/zh_CN/runesReforged.json`,
  { signal: AbortSignal.timeout(30000) },
);
if (!response.ok)
  throw Error(`Data Dragon metadata returned ${response.status}`);
const raw = await response.text();
if (raw.length > 1024 * 1024) throw Error("Metadata size exceeded");
const trees = JSON.parse(raw),
  names = {};
function add(entry) {
  if (
    !Number.isSafeInteger(entry.id) ||
    entry.id <= 0 ||
    typeof entry.name !== "string" ||
    !entry.name.trim() ||
    entry.name.length > 100
  )
    throw Error("Invalid rune entry");
  names[entry.id] = entry.name;
}
for (const tree of trees) {
  add(tree);
  for (const slot of tree.slots) for (const rune of slot.runes) add(rune);
}
// Emit an apply_patch patch; only the normal patch tool writes repository files.
const target = new URL(
  "../frontend/src/assets/game/rune-names.json",
  import.meta.url,
);
let previous;
try {
  previous = await readFile(target, "utf8");
} catch (error) {
  if (error.code !== "ENOENT") throw error;
}
const next = JSON.stringify({ version: catalog.version, names }, null, 2);
const header = previous === undefined ? "*** Add File: " : "*** Update File: ";
const removed =
  previous === undefined
    ? ""
    : "@@\n" +
      previous
        .trimEnd()
        .split("\n")
        .map((line) => "-" + line)
        .join("\n") +
      "\n";
process.stdout.write(
  "*** Begin Patch\n" +
    header +
    fileURLToPath(target).replaceAll("\\", "/") +
    "\n" +
    removed +
    next
      .split("\n")
      .map((line) => "+" + line)
      .join("\n") +
    "\n*** End Patch\n",
);

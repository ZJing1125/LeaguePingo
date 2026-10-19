import catalog from "./assets/game/catalog.json";
export type AssetKind = "champions" | "items" | "spells";
interface Entry {
  name: string;
  title?: string;
  sprite: string;
  x: number;
  y: number;
  w: number;
  h: number;
}
const sheets = import.meta.glob("./assets/game/*.png", {
  eager: true,
  query: "?url",
  import: "default",
}) as Record<string, string>;
export function gameAsset(kind: AssetKind, id: number | null | undefined) {
  return id == null ? undefined : (catalog[kind] as Record<string, Entry>)[id];
}
export function championName(id: number) {
  const e = gameAsset("champions", id);
  return e ? `${e.name}·${e.title}` : id ? `英雄 #${id}` : "英雄未知";
}
export function iconStyle(
  kind: AssetKind,
  id: number | null | undefined,
  size: number,
) {
  const e = gameAsset(kind, id);
  if (!e) return { width: `${size}px`, height: `${size}px` };
  const sheet = (
    catalog.sheets as Record<string, { width: number; height: number }>
  )[e.sprite]!;
  return {
    width: `${size}px`,
    height: `${size}px`,
    backgroundImage: `url("${sheets[`./assets/game/${e.sprite}`]}")`,
    backgroundPosition: `${(-e.x * size) / e.w}px ${(-e.y * size) / e.h}px`,
    backgroundSize: `${(sheet.width * size) / e.w}px ${(sheet.height * size) / e.h}px`,
  };
}
export const assetVersion = catalog.version;

// All artwork is packaged. Missing/new champions keep the local fallback.
const portraits = import.meta.glob("./assets/game/portraits/*.jpg", {
  eager: true,
  query: "?url",
  import: "default",
}) as Record<string, string>;
export function championPortrait(id: number | undefined) {
  return id ? portraits[`./assets/game/portraits/${id}.jpg`] : undefined;
}

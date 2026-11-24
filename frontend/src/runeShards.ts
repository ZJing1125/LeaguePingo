import catalog from './assets/game/shards/config.json';
import { uiText } from './uiText';

export function shardConfiguration(version?: string) {
  const parts = /^(\d{1,2})\.(\d{1,2})(?:\.\d+)*$/.exec(version ?? '');
  if (!parts) return undefined;
  let major = Number(parts[1]);
  const minor = Number(parts[2]);
  if (minor < 1 || minor > 24) return undefined;
  if ([25,26].includes(major)) major -= 10;
  const patch = major * 100 + minor;
  return catalog.versions.find(v => patch >= v.min[0]*100+v.min[1] && patch <= v.max[0]*100+v.max[1]);
}

function baseName(id: number) {
  const text = uiText.runePage;
  const names: Record<number,string> = {
    5001:text.scalingHealth, 5002:text.armor, 5003:text.magicResistance,
    5005:text.attackSpeed, 5007:text.haste, 5008:text.adaptiveForce,
    5010:text.movementSpeed, 5011:text.health, 5013:text.tenacity,
  };
  return id ? names[id] ?? text.stat(id) : uiText.common.missing;
}

export function shardName(id: number, version?: string, slot?: number) {
  const config = shardConfiguration(version);
  const options = slot == null ? config?.rows.flat() : config?.rows[slot];
  const value = options?.includes(id) ? (config?.values as Record<string,string|undefined>)?.[String(id)] : undefined;
  return value ? `${baseName(id)} ${value}` : baseName(id);
}

export function shardRows(shards: number[], version?: string) {
  const config = shardConfiguration(version);
  return [0,1,2].map(slot => {
    const selected = shards[slot] ?? 0;
    const options = config?.rows[slot];
    // Conflicting or unknown configurations retain only the actual recorded ID.
    return options && (!selected || options.includes(selected)) ? options : selected ? [selected] : [];
  });
}

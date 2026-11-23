import type { Match } from '../bindings/leaguepingo/internal/core/models';
import names from './assets/game/match-augments.json';
import modes from '../../internal/core/scout_modes.json';

export function matchMode(match?: Match | null) {
  const mode = modes.find(mode => mode.queueId === match?.queueId);
  const known = !!mode && (!match?.mapId || match.mapId === mode.mapId);
  const roles = known && !!mode.roles;
  return {
    known,
    roles,
    vision: roles,
    objectives: roles,
    runes: known && ![2400, 1700, 1710].includes(mode.queueId),
    augments: known && [2400, 1700, 1710].includes(mode.queueId),
  };
}

export function matchAugmentName(match: Match, id: number) {
  if (!matchMode(match).augments) return undefined;
  const mode = match.queueId === 2400 ? 'aram-mayhem' : 'arena';
  const catalog = names as Record<string, Record<string, string>>;
  return catalog[mode]?.[String(id)];
}

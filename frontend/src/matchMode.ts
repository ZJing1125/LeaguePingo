import type { Match } from '../bindings/leaguepingo/internal/core/models';
import names from './assets/game/match-augments.json';
import modes from '../../internal/core/scout_modes.json';

export function matchMode(match?: Match | null) {
  const mode = modes.find(mode => mode.queueId === match?.queueId);
  const known = !!mode && (!match?.mapId || match.mapId === mode.mapId);
  const map = known ? mode.mapId : 0;
  const roles = known && mode.roles;
  const arena = known && [1700,1710].includes(mode.queueId);
  const family = !known ? 'unknown' : arena ? 'arena' : mode.queueId===2400 ? 'mayhem' : mode.queueId===450 ? 'aram' : mode.guide==='urf' ? 'urf' : mode.queueId===1300 ? 'nexus-blitz' : 'rift';
  const metrics: Record<string,string[]> = {
    rift:['kills','gold','damage','visionScore','damageTaken','cs'],
    aram:['damage','damageTaken','kills','gold'],
    mayhem:['damage','damageTaken','kills','gold'],
    urf:['damage','kills','damageTaken','gold'],
    'nexus-blitz':['kills','damage','gold','damageTaken'],
    arena:[],unknown:[],
  };
  return {
    known,
    family,
    metrics:metrics[family],
    roles,
    arena,
    cs: roles,
    vision: roles,
    objectives: known && !arena,
    structures: known && !arena,
    neutralObjectives: known && [11,21].includes(map),
    riftObjectives: known && map === 11,
    team: known && !arena,
    timeline: known && !arena,
    map: known && map === 11,
    runes: known && ![2400,1700,1710].includes(mode.queueId),
    augments: known && [2400,1700,1710].includes(mode.queueId),
  };
}

export function matchAugmentName(match: Match, id: number) {
  if (!matchMode(match).augments) return undefined;
  const mode = match.queueId === 2400 ? 'aram-mayhem' : 'arena';
  const catalog = names as Record<string, Record<string, string>>;
  return catalog[mode]?.[String(id)];
}

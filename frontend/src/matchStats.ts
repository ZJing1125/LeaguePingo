import type { Match, Stats } from "../bindings/leaguepingo/internal/core/models";
import modes from "../../internal/core/scout_modes.json";

export const isArena = (match: Match) => modes.some(mode => mode.queueId === match.queueId && 'arena' in mode && mode.arena);
export function arenaResult(match: Match) {
  const result = match.arena;
  if (!isArena(match) || match.mapId != null && match.mapId !== 30 || !result || !Number.isInteger(result.placement) || result.placement < 1 || result.placement > 32) return null;
  if (result.teamCount != null && (!Number.isInteger(result.teamCount) || result.teamCount < 2 || result.teamCount > 32 || result.placement > result.teamCount)) return null;
  return result;
}

export function matchTone(match: Match) {
  if (isArena(match)) return arenaResult(match)?.placement === 1 ? 'won' : 'unknown';
  return isRemake(match) ? 'remake' : match.win == null ? 'unknown' : match.win ? 'won' : 'lost';
}

function arenaStats(matches: Match[]): Stats['arena'] {
  if (!matches.some(isArena)) return undefined;
  const results = matches.map(arenaResult).filter(result => result != null);
  const teams = results.filter(result => result.teamCount != null);
  const firstPlaces = results.filter(result => result.placement === 1).length;
  const topHalf = teams.filter(result => result.placement <= Math.floor(result.teamCount! / 2)).length;
  return {
    placementGames: results.length, firstPlaces,
    averagePlace: results.length ? results.reduce((n, result) => n + result.placement, 0) / results.length : null,
    firstPlaceRate: results.length ? firstPlaces * 100 / results.length : null,
    topHalfGames: teams.length, topHalf, topHalfRate: teams.length ? topHalf * 100 / teams.length : null,
  };
}

export function isRemake(match: Match) {
  return (
    !isArena(match) &&
    match.earlyEnd === true &&
    match.duration != null &&
    match.duration >= 0 &&
    match.duration <= 5 * 60
  );
}

export function sampleStats(matches: Match[]): Stats {
  const statistical = matches.filter((m) => !isRemake(m));
  const outcomes = statistical.filter((m) => m.win != null && !isArena(m));
  const complete = statistical.filter(
    (m) => m.kills != null && m.deaths != null && m.assists != null,
  );
  const wins = outcomes.filter((m) => m.win).length;
  const kills = complete.reduce((a, m) => a + m.kills!, 0),
    deaths = complete.reduce((a, m) => a + m.deaths!, 0),
    assists = complete.reduce((a, m) => a + m.assists!, 0);
  return {
    arena: arenaStats(matches),
    games: matches.length,
    wins,
    losses: outcomes.length - wins,
    outcomeGames: outcomes.length,
    kdaGames: complete.length,
    winRate: outcomes.length ? (wins * 100) / outcomes.length : null,
    kda: complete.length ? (kills + assists) / Math.max(1, deaths) : null,
  };
}

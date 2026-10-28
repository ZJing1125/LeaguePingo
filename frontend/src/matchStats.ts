import type { Match, Stats } from "../bindings/leaguepingo/internal/core/models";

export function isRemake(match: Match) {
  return (
    match.earlyEnd === true &&
    match.duration != null &&
    match.duration >= 0 &&
    match.duration <= 5 * 60
  );
}

export function sampleStats(matches: Match[]): Stats {
  const statistical = matches.filter((m) => !isRemake(m));
  const outcomes = statistical.filter((m) => m.win != null);
  const complete = statistical.filter(
    (m) => m.kills != null && m.deaths != null && m.assists != null,
  );
  const wins = outcomes.filter((m) => m.win).length;
  const kills = complete.reduce((a, m) => a + m.kills!, 0),
    deaths = complete.reduce((a, m) => a + m.deaths!, 0),
    assists = complete.reduce((a, m) => a + m.assists!, 0);
  return {
    games: matches.length,
    wins,
    losses: outcomes.length - wins,
    outcomeGames: outcomes.length,
    kdaGames: complete.length,
    winRate: outcomes.length ? (wins * 100) / outcomes.length : null,
    kda: complete.length ? (kills + assists) / Math.max(1, deaths) : null,
  };
}

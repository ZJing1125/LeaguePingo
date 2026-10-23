import type { Participant, Rank } from "../bindings/leaguepingo/internal/core/models";

// Rank position and match-derived results have different scopes. Prefer the
// independent current-season report, never the user's filtered history list.
export function rankRecord(player: Participant, queue: string, rank?: Rank) {
  if (player.identityState !== "known") return null;
  const report = player.scout;
  const mode = report?.season && report.queues?.find(q => q.queueId === (queue === "RANKED_FLEX_SR" ? 440 : 420));
  const stats = mode?.season;
  if (stats && (stats.outcomeGames > 0 || report?.complete && stats.games === 0)) {
    const complete = report?.complete && stats.outcomeGames === stats.games;
    const scope = `${report?.retained ? "上次" : ""}${complete ? "本季" : "本季已读"}`;
    const total = stats.wins + stats.losses;
    return { wins: stats.wins, losses: stats.losses, rate: total ? stats.wins * 100 / total : null, scope,
      title: `${report?.season?.name} · ${scope}胜负，有效 ${stats.outcomeGames}/${stats.games} 场；独立于战绩列表筛选` };
  }
  if (rank?.wins == null || rank.losses == null) return null;
  const total = rank.wins + rank.losses;
  return { wins: rank.wins, losses: rank.losses, rate: total ? rank.wins * 100 / total : null, scope: "排位",
    title: "段位资料中的排位胜负；与最近30天及本季已读记录的范围不同" };
}

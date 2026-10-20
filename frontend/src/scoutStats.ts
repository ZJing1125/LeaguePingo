import type { Match, Participant } from "../bindings/leaguepingo/internal/core/models";
import { sampleStats } from "./matchStats";

export const roleNames: Record<string, string> = {
  TOP: "上路", JUNGLE: "打野", MIDDLE: "中路", BOTTOM: "下路", UTILITY: "辅助",
};
const riftQueues = new Set([400, 420, 430, 440, 490, 700]);
function onRift(m: Match) {
  return m.mapId === 11 || (m.mapId == null && riftQueues.has(m.queueId));
}
function averageKDA(matches: Match[]) {
  const valid = matches.filter(m => m.kills != null && m.deaths != null && m.assists != null);
  return valid.length ? ["kills", "deaths", "assists"].map(key =>
    valid.reduce((sum, m) => sum + (m[key as "kills" | "deaths" | "assists"] ?? 0), 0) / valid.length,
  ) : null;
}

// All periods are subsets of the same first-page sample, anchored to its fetch
// time. A timer, inferred lane, additional page or client request is never used.
export function scoutStats(player: Participant) {
  const matches = (player.history ?? []).slice(0, 20);
  const overall = sampleStats(matches);
  const championMatches = player.championId ? matches.filter(m => m.championId === player.championId) : [];
  const champion = { ...sampleStats(championMatches), average: averageKDA(championMatches) };
  const anchor = Date.parse(player.historyMeta?.fetchedAt ?? "");
  const periods = [12, 24 * 30].map(hours => {
    const subset = Number.isFinite(anchor) ? matches.filter(m => {
      const played = Date.parse(m.playedAt ?? "");
      return Number.isFinite(played) && played <= anchor && played >= anchor - hours * 3600000;
    }) : [];
    return { hours, timed: Number.isFinite(anchor), ...sampleStats(subset) };
  });
  const positions = matches.filter(m => onRift(m) && !!roleNames[m.position]);
  const counts = Object.keys(roleNames).map(key => ({ key, count: positions.filter(m => m.position === key).length }))
    .sort((a, b) => b.count - a.count);
  const first = counts[0]!;
  const clear = positions.length >= 3 && first.count > counts[1]!.count && first.count / positions.length >= 0.5;
  const role = {
    key: clear ? first.key : "",
    label: !positions.length ? "位置暂缺" : !clear ? "位置样本不足" : roleNames[first.key]!,
    count: clear ? first.count : 0,
    known: positions.length,
    share: clear ? first.count * 100 / positions.length : null,
  };
  const mains = [...new Set(matches.map(m => m.championId).filter(Boolean))]
    .map(id => ({ id, count: matches.filter(m => m.championId === id).length }))
    .sort((a, b) => b.count - a.count).slice(0, 3);
  const tags: { label: string; title: string; tone: string; heroId?: number }[] = [];
  let streak = 0;
  for (const m of matches) { if (m.win !== true) break; streak++; }
  if (streak >= 3) tags.push({ label: `近期 ${streak} 连胜`, title: `已获取记录按新到旧，连续 ${streak} 场明确胜利；未知胜负会中断连胜`, tone: "positive" });
  else {
    const recent = sampleStats(matches.slice(0, 5));
    if (recent.outcomeGames >= 3) tags.push({ label: `近5场 ${recent.wins}胜${recent.losses}负`, title: `近五场中胜负有效 ${recent.outcomeGames}/${recent.games} 场`, tone: recent.wins >= 3 ? "positive" : "neutral" });
  }
  if (champion.games >= 3) tags.push({ label: `本英雄 ${champion.games} 场`, title: `最近 ${matches.length} 场中的本英雄样本，不代表熟练度或赛季场次`, tone: "accent" });
  if (mains[0] && mains[0].id !== player.championId && mains[0].count >= 4)
    tags.push({ label: `常用 ${mains[0].count} 场`, heroId: mains[0].id, title: `最近 ${matches.length} 场中使用最多的英雄；不代表赛季英雄专精`, tone: "accent" });
  const farming = matches.filter(m => onRift(m) && m.cs != null && m.duration != null && m.duration >= 600);
  if (farming.length >= 3) {
    const perMinute = farming.reduce((n, m) => n + m.cs!, 0) * 60 / farming.reduce((n, m) => n + m.duration!, 0);
    tags.push({ label: `补刀 ${perMinute.toFixed(1)}/分`, title: `召唤师峡谷中，${farming.length} 场有效补刀 / 总时长；排除时长不足 10 分钟和缺失字段`, tone: "neutral" });
  }
  if (clear && tags.length < 3) tags.push({ label: `近期${role.label}`, title: `历史记录明确提供的位置，${role.count}/${role.known} 场；不代表本局分路`, tone: "neutral" });
  if (!tags.length && matches.length) tags.push({ label: `近期样本 ${matches.length} 场`, title: "仅使用已经取得的首批战绩", tone: "neutral" });
  return { matches, overall, champion, periods, role, mains, tags: tags.slice(0, 3) };
}

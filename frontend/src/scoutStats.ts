import type { Participant, Snapshot } from "../bindings/leaguepingo/internal/core/models";
import modes from "../../internal/core/scout_modes.json";
import { sampleStats } from "./matchStats";

export const roleNames: Record<string, string> = { TOP: "上路", JUNGLE: "打野", MIDDLE: "中路", BOTTOM: "下路", UTILITY: "辅助" };
export const scoutModes = modes;
export function scoutMode(queue: number) { return modes.find(m => m.queueId === queue); }
export function rankedMode(queue: number) { return scoutMode(queue)?.label ?? "模式待确认"; }

export function liveScoutMode(snapshot?: Snapshot | null) {
  const mode = scoutMode(snapshot?.queueId ?? 0);
  if (!mode || (snapshot?.mapId && snapshot.mapId !== mode.mapId)
    || (snapshot?.gameMode && !mode.gameModes.includes(snapshot.gameMode.toUpperCase()))) return 0;
  return mode.queueId;
}

// Uses only the independent report. The user's recent-history sample is never
// a fallback for champion experience, a time window or ranked role distribution.
export function scoutStats(player: Participant, queue = 420) {
  const rawReport = player.identityState === "known" ? player.scout : undefined;
  const mode = rawReport?.queues?.find(q => q.queueId === queue);
  const rating = player.teamRating;
  const ratingAnalysis = rating ? rawReport?.queues?.find(q => q.queueId === rating.queueId)?.analysis : undefined;
  const report = mode?.coverage && rawReport ? { ...rawReport, ...mode.coverage } : rawReport;
  const hero = report?.champions?.find(h => h.championId === player.championId);
  const definition = scoutMode(queue);
  const ranked = !!definition?.ranked;
  const selectedHero = (!ranked || report?.season) ? hero?.queues?.find(q => q.queueId === queue) : undefined;
  const empty = sampleStats([]);
  const periods = [
    { hours: 12, complete: !!mode && (report?.complete12 ?? false), ...(mode?.twelve ?? empty) },
    { hours: 720, complete: !!mode && (report?.complete30 ?? false), ...(mode?.month ?? empty) },
  ];
  const positions = [...(definition?.roles ? mode?.positions ?? [] : [])].filter(p => !!roleNames[p.position] && p.games > 0).sort((a, b) => b.games - a.games);
  const known = positions.reduce((n, p) => n + p.games, 0);
  const first = positions[0];
  const clear = known >= 3 && first && first.games > (positions[1]?.games ?? 0) && first.games / known >= 0.5;
  const role = { key: clear ? first.position : "", label: clear ? roleNames[first.position]! : known >= 3 ? "位置较分散" : known ? "位置记录较少" : "位置未记录", known, count: clear ? first.games : 0, share: clear ? first.games * 100 / known : null, positions };
  const champion = { ...(selectedHero?.stats ?? empty), average: selectedHero?.average?.length === 3 ? selectedHero.average : null };
  const tags: { label: string; title: string; tone: string; heroId?: number }[] = [];
  if (hero?.games != null && report?.season) tags.push({ label: `${report.complete ? '本季全模式' : '本季已读'} ${hero.games} 场`, title: `${report.season.name} · 本英雄全部模式${report.complete ? '' : '已读取范围'}；不受战绩列表筛选影响`, tone: "accent" });
  if (hero?.mastery) tags.push({ label: `熟练度 ${hero.mastery.level} 级`, title: `全部模式英雄熟练度 ${hero.mastery.points.toLocaleString()} 分；熟练度不换算为场次`, tone: "positive" });
  return { report, mode, rating, ratingAnalysis, analysis: mode?.analysis, hero, champion, periods, role, ranked, arena: !!(definition && 'arena' in definition && definition.arena), roles: !!definition?.roles, heroComplete: ranked ? !!report?.complete : !!report?.complete30, overall: mode?.month ?? empty, tags: tags.slice(0, 3), queueLabel: rankedMode(queue) };
}

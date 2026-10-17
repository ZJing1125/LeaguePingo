import type {
  Match,
  Stats,
  Participant,
  HistoryMeta,
} from "../bindings/leaguepingo/internal/core/models";
import runeCatalog from "./assets/game/rune-names.json";
export function runeName(id: number) {
  return (runeCatalog.names as Record<string, string>)[id] ?? `符文 #${id}`;
}
export function mapName(id: number | null | undefined) {
  return id == null
    ? "地图未知"
    : ((
        { 11: "召唤师峡谷", 12: "嚎哭深渊", 30: "斗魂竞技场" } as Record<
          number,
          string
        >
      )[id] ?? `地图 #${id}`);
}
export function positionName(position: string | undefined) {
  return (
    (
      {
        TOP: "上路",
        JUNGLE: "打野",
        MIDDLE: "中路",
        BOTTOM: "下路",
        UTILITY: "辅助",
      } as Record<string, string>
    )[position ?? ""] ?? "位置未获取"
  );
}

export function percent(value: number | null | undefined) {
  return value == null ? "—" : `${Math.round(value)}%`;
}
export function decimal(value: number | null | undefined) {
  return value == null ? "—" : value.toFixed(2);
}
export function number(value: number | null | undefined) {
  return value == null ? "—" : String(value);
}
export function kda(m: Match) {
  return [m.kills, m.deaths, m.assists].map(number).join(" / ");
}
export function duration(seconds: number | null | undefined) {
  return seconds == null
    ? "—"
    : `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}
const tiers: Record<string, string> = {
  NONE: "未定级",
  UNRANKED: "未定级",
  IRON: "黑铁",
  BRONZE: "青铜",
  SILVER: "白银",
  GOLD: "黄金",
  PLATINUM: "铂金",
  EMERALD: "翡翠",
  DIAMOND: "钻石",
  MASTER: "大师",
  GRANDMASTER: "宗师",
  CHALLENGER: "王者",
};
export function tierName(tier: string) {
  return tiers[tier] ?? "段位未知";
}
export function outcome(m: Match) {
  return m.win == null ? "未知" : m.win ? "胜利" : "失败";
}
export function date(value: string | null | undefined) {
  if (!value) return "日期未知";
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? "日期未知"
    : d.toLocaleString("zh-CN", {
        month: "numeric",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
}
const queues: Record<number, string> = {
  0: "自定义 / 其他",
  400: "匹配征召",
  420: "单双排",
  430: "匹配",
  440: "灵活排位",
  450: "极地大乱斗",
  490: "快速匹配",
  700: "冠军杯赛",
  1700: "斗魂竞技场",
  1900: "无限火力",
  2400: "海克斯大乱斗",
};
export function queueName(id: number) {
  return queues[id] ?? `模式 ${id}`;
}
export function sampleStats(matches: Match[]): Stats {
  const outcomes = matches.filter((m) => m.win != null);
  const complete = matches.filter(
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
export function errorMessage(reason: unknown) {
  const text = (
    reason instanceof Error ? reason.message : String(reason)
  ).replace(/^RuntimeError:\s*/, "");
  if (/<!DOCTYPE|<html/i.test(text))
    return "桌面服务暂时不可用，请重新打开应用。";
  return text.slice(0, 220);
}
export function preparationLabel(p: Participant) {
  if (p.identityState === "unavailable") return "身份未提供";
  if (p.stale) return "旧资料 · 可主动更新";
  if (p.historyState === "loading") return "正在准备";
  if (p.historyState === "failed") return "准备失败 · 可重试";
  if (p.historyState === "pending") return p.error ? "准备已暂停" : "尚未准备";
  if (p.historyState === "empty") return "暂无战绩";
  return p.cached ? "缓存资料" : "已准备";
}
export function recentOutcomes(p: Participant) {
  return (
    (p.history ?? [])
      .slice(0, 5)
      .map((m) => (m.win == null ? "?" : m.win ? "胜" : "负"))
      .join(" · ") || "暂无样本"
  );
}
export function championSample(p: Participant) {
  const matches = (p.history ?? []).filter(
    (m) => m.championId === p.championId,
  );
  return !p.championId || !matches.length
    ? "当前英雄暂无样本"
    : `当前英雄 ${matches.length} 场 · ${percent(sampleStats(matches).winRate)}`;
}
export interface PlayerViewState {
  queue: string;
  expanded: number[];
  sourceStamp?: string;
  extra?: Match[];
  pageMeta?: HistoryMeta[];
  nextOffset?: number;
  hasMore?: boolean;
  pageBusy?: boolean;
  pageError?: string;
  page?: number;
  sampleSize?: number;
  browseNotice?: string;
  browseTarget?: number;
  stopRequested?: boolean;
  sourceLimit?: string;
}

import type {
  Match,
  Participant,
  HistoryMeta,
} from "../bindings/leaguepingo/internal/core/models";
import { isRemake, isArena, arenaResult } from "./matchStats";
import { uiText } from "./uiText";
import runeCatalog from "./assets/game/rune-names.json";
import { gameAsset } from "./gameAssets";
import { hasPlayerName } from "./playerState";
import { sampleStats } from "./matchStats";

export function playerName(player: Participant, position?: number) {
  if (hasPlayerName(player)) return player.displayName;
  if (player.identityState === "unavailable") return player.historyState === "loading" ? "正在确认玩家身份" : "身份暂不可用";
  const hero = gameAsset("champions", player.championId)?.name;
  return `${hero || (position ? `玩家 ${position}` : "玩家")} · 姓名未获取`;
}
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
  if (isArena(m)) {
    const result = arenaResult(m);
    return result ? uiText.arena.place(result.placement) : uiText.arena.pending;
  }
  return isRemake(m)
    ? "重开"
    : m.win == null
      ? "未知"
      : m.win
        ? "胜利"
        : "失败";
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
  1710: "斗魂竞技场（1710）",
  1900: "无限火力",
  2400: "海克斯大乱斗",
  3140: "训练模式",
};
export function queueName(id: number) {
  return queues[id] ?? `模式 ${id}`;
}
export { sampleStats } from "./matchStats";
// Match core.NormalizeRiotID's clipboard cleanup; keep name-significant
// joiners, accents, variation selectors and inner spaces unchanged.
export function normalizeRiotIDInput(value: string) {
  return value
    .replace(/[\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069]/g, "")
    .split("#")
    .map((part) =>
      part.replace(/^[\s\u0085\u200b\ufeff]+|[\s\u0085\u200b\ufeff]+$/g, ""),
    )
    .join("#");
}

export { errorMessage } from './errors';
export function preparationLabel(p: Participant, frozen = false) {
  if (p.identityState === "unavailable") return p.historyState === "loading" ? "正在确认玩家身份" : "身份未提供";
  if (p.stale) return "旧资料 · 可主动更新";
  if (p.historyState === "loading") return "正在准备";
  if (p.historyState === "failed") return "准备失败 · 可重试";
  if (p.historyState === "pending")
    return frozen ? "未获取 · 可手动查询" : p.error ? "准备已暂停" : "尚未准备";
  if (p.historyState === "empty") return "暂无战绩";
  return p.cached ? "缓存资料" : "已准备";
}
export function recentOutcomes(p: Participant) {
  return (
    (p.history ?? [])
      .slice(0, 5)
      .map((m) => (isRemake(m) ? "重" : m.win == null ? "?" : m.win ? "胜" : "负"))
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
  sourceHead?: string;
  mixedHasMore?: boolean;
  queueFallbacks?: string[];
  queuePages?: Record<string, { matches: Match[]; nextOffset: number; hasMore: boolean; direct: boolean; meta: HistoryMeta[] }>;
  extra?: Match[];
  pageMeta?: HistoryMeta[];
  nextOffset?: number;
  hasMore?: boolean;
  pageBusy?: boolean;
  warmBusy?: boolean;
  pageError?: string;
  page?: number;
  sampleSize?: number;
  browseNotice?: string;
  browseTarget?: number;
  browseKind?: "samples" | "page";
  stopRequested?: boolean;
  sourceLimit?: string;
}

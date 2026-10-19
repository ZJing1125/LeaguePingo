import type { Participant } from "../bindings/leaguepingo/internal/core/models";
import { date, tierName } from "./presentation";

export function rankPresentation(
  player: Participant,
  queue = "RANKED_SOLO_5x5",
) {
  const profile = player.profile;
  const rank =
    profile?.rankState === "ready"
      ? profile.ranks?.find((r) => r.queue === queue)
      : undefined;
  const retained = !!rank && profile?.fields?.rank?.state === "retained";
  let label = "段位待查询";
  let title = "打开玩家详情后点击“获取段位”；战绩与段位分别获取";
  if (player.identityState !== "known") {
    label = "身份未提供";
    title = "客户端未提供有效身份，无法查询段位";
  } else if (rank) {
    label = ["NONE", "UNRANKED"].includes(rank.tier)
      ? "未定级"
      : `${tierName(rank.tier)} ${rank.division || ""}${rank.lp != null ? ` · ${rank.lp} LP` : ""}`.trim();
    title = `${retained ? "保留上次段位" : "段位获取时间"} · ${date(profile?.fields?.rank?.fetchedAt)}${profile?.rankMessage ? ` · ${profile.rankMessage}` : ""}`;
  } else if (profile?.rankState === "failed") {
    label = profile.rankMessage?.includes("超时")
      ? "段位查询超时"
      : "段位查询失败";
    title = profile.rankMessage || "查询失败，可重试；战绩仍可独立使用";
  } else if (profile?.rankState === "ready") {
    label = "该模式段位未返回";
    title = "客户端未返回此模式的可核验段位，不能据此判断未定级";
  } else if (profile?.rankState === "unavailable") {
    label = "来源未提供段位";
    title = profile.rankMessage || "客户端未提供可核验段位，可手动重试";
  }
  return { rank, retained, label, title };
}

export function rankActionLabel(player: Participant) {
  if (
    player.profile?.rankMessage ||
    ["failed", "unavailable"].includes(player.profile?.rankState ?? "")
  )
    return "重试段位";
  return player.profile?.rankState === "ready" ? "更新段位" : "获取段位";
}

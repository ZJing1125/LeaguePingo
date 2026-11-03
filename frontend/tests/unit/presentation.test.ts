import { describe, expect, it } from "vitest";
import { HistoryState, IdentityState } from "../../bindings/leaguepingo/internal/core/models";
import { championSample, decimal, duration, errorMessage, kda, normalizeRiotIDInput, number, outcome, percent, playerName, preparationLabel, queueName, recentOutcomes, sampleStats, tierName } from "../../src/presentation";
import { codedError } from "../../src/errors";
import { match, player } from "./fixtures";

describe("player and match presentation", () => {
  it("keeps unknown numeric values distinct from actual zero", () => {
    expect([number(null), decimal(undefined), percent(null), duration(undefined)]).toEqual(["—", "—", "—", "—"]);
    expect([number(0), decimal(0), percent(0), duration(0)]).toEqual(["0", "0.00", "0%", "0:00"]);
    expect(kda(match(1, 420, { kills: 0, deaths: null, assists: 3 }))).toBe("0 / — / 3");
  });

  it("labels verified short remakes without relabeling normal surrender or unknown results", () => {
    expect(outcome(match(1, 420, { win: false, earlyEnd: true, duration: 103 }))).toBe("重开");
    expect(outcome(match(1, 420, { win: false, earlyEnd: true, duration: 1500 }))).toBe("失败");
    expect(outcome(match(1, 420, { win: false, earlyEnd: null, duration: 103 }))).toBe("失败");
    expect(outcome(match(1, 420, { win: null }))).toBe("未知");
  });

  it("excludes remakes and incomplete fields from statistical denominators", () => {
    const result = sampleStats([
      match(1, 420, { win: true, kills: 4, deaths: 2, assists: 6 }),
      match(2, 420, { win: false }),
      match(3),
      match(4, 420, { win: false, earlyEnd: true, duration: 90, kills: 0, deaths: 0, assists: 0 }),
    ]);
    expect(result).toMatchObject({ games: 4, outcomeGames: 2, wins: 1, losses: 1, winRate: 50, kdaGames: 1, kda: 5 });
    expect(sampleStats([match(1)])).toMatchObject({ winRate: null, kda: null });
  });

  it("cleans clipboard direction markers while preserving meaningful Unicode and spaces", () => {
    expect(normalizeRiotIDInput("\u2066 \ufeffCafé A\u200dB ♥️\u200b # \u0085TEST \u2069")).toBe("Café A\u200dB ♥️#TEST");
  });

  it("uses safe fallback labels without inventing a name, rank, or mode", () => {
    expect(playerName(player([], { displayName: "", championId: 0 }), 3)).toBe("玩家 3 · 姓名未获取");
    expect(playerName(player([], { displayName: "", identityState: IdentityState.IdentityUnavailable }))).toBe("身份暂不可用");
    expect([queueName(420), queueName(440), queueName(450), queueName(2400)]).toEqual(["单双排", "灵活排位", "极地大乱斗", "海克斯大乱斗"]);
    expect(tierName("unknown-tier")).toBe("段位未知");
    expect(queueName(99999)).toBe("模式 99999");
  });

  it("distinguishes stale data and frozen pending data from a successful empty history", () => {
    expect(preparationLabel(player([], { stale: true, cached: true }))).toBe("旧资料 · 可主动更新");
    expect(preparationLabel(player([], { historyState: HistoryState.HistoryPending }), true)).toBe("未获取 · 可手动查询");
    expect(preparationLabel(player([], { historyState: HistoryState.HistoryEmpty }))).toBe("暂无战绩");
  });

  it("limits recent outcomes and current-champion samples to their actual records", () => {
    const records = [match(1, 420, { championId: 1, win: true }), match(2, 420, { championId: 2, win: false }), match(3), match(4, 420, { earlyEnd: true, duration: 90 }), match(5, 420, { win: true }), match(6, 420, { win: false })];
    const target = player(records, { championId: 2 });
    expect(recentOutcomes(target)).toBe("胜 · 负 · ? · 重 · 胜");
    expect(championSample(target)).toBe("当前英雄 1 场 · 0%");
    expect(championSample(player([], { championId: 2 }))).toBe("当前英雄暂无样本");
  });

  it("never displays arbitrary transport or bridge error text", () => {
    const unsafe = "synthetic-private-response-and-token";
    expect(errorMessage(new Error(unsafe))).toBe(errorMessage(null));
    expect(errorMessage(codedError("HistoryRepeated"))).toContain("数据源重复返回");
    expect(errorMessage({ message: unsafe, cause: { code: "not-a-known-code", message: unsafe } })).not.toContain(unsafe);
  });
});

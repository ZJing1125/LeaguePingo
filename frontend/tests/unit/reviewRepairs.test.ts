import { describe, expect, it } from "vitest";
import { Snapshot, MatchDetail } from "../../bindings/leaguepingo/internal/core/models";
import { liveScoutMode } from "../../src/scoutStats";
import { historyRoster, peerShortName } from "../../src/historyMatch";
import { analysisCache, acquireTimeline } from "../../src/analysisCache";
import { match } from "./fixtures";

describe("room and review regressions", () => {
  it("uses the engine mode decision and distinguishes room queue from active-game conflicts", () => {
    expect(liveScoutMode(({ queueId: 2400, actualScoutQueue: 2400, stage: "party", gameMode: "UNKNOWN" } as Snapshot))).toBe(2400);
    expect(liveScoutMode(({ queueId: 2400, actualScoutQueue: 0, stage: "in-game", gameMode: "CLASSIC" } as Snapshot))).toBe(0);
  });
  it("fills anonymous same-game rosters from verified detail without guessing champion names", () => {
    const game = match(91234, 420, { roster: [ { participantId: 1, teamId: 100, championId: 81, displayName: "", target: true } ] });
    expect(peerShortName(game.roster![0]!)).toBe("姓名未提供");
    analysisCache.setDetail("89:player:91234", ({ gameId: 91234, connectionId: 89, players: [ { participantId: 1, teamId: 100, displayName: "测试玩家#AB", target: true, summary: { ...game, championId: 81 } } ] } as MatchDetail));
    expect(historyRoster(game, 89)[0]!.peers[0]!.displayName).toBe("测试玩家#AB");
    expect(historyRoster(game, 90)[0]!.peers[0]!.displayName).toBe("");
    expect(historyRoster({ ...game, roster: [ { ...game.roster![0]!, championId: 82 } ] }, 89)[0]!.peers[0]!.displayName).toBe("");
  });
  it("shares a timeline until its final consumer leaves", () => {
    let calls = 0, cancelled = 0;
    const fetch = () => { calls++; return Object.assign(new Promise<never>(() => {}), { cancel: () => { cancelled++; } }); };
    const first = acquireTimeline("shared", fetch);
    const second = acquireTimeline("shared", fetch);
    first.release();
    expect([calls, cancelled]).toEqual([1, 0]);
    second.release(); second.release();
    expect(cancelled).toBe(1);
  });
});

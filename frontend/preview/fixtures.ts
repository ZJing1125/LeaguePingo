// Synthetic, fixed-time examples only. No player IDs, client access or remote assets.
import type {
  Match,
  Participant,
  Snapshot,
  IdentityState,
  HistoryState,
} from "../bindings/leaguepingo/internal/core/models";
import type { OverlayState } from "../bindings/leaguepingo/models";
import { sampleStats } from "../src/presentation";

export const exampleTime = "2026-10-18T01:00:00Z";
export function makeMatch(index = 0, overrides: Partial<Match> = {}): Match {
  return {
    gameId: 900000 + index,
    playedAt: exampleTime,
    queueId: 420,
    championId: [23, 24, 25][index % 3]!,
    win: index % 3 !== 0,
    kills: 6,
    deaths: 3,
    assists: 9,
    duration: 1920,
    gold: 12345,
    damage: 23456,
    cs: 204,
    items: [3006, 3078, 0, null, 0, 0, 3364],
    spell1: 4,
    spell2: 14,
    mapId: 11,
    position: "TOP",
    lane: "TOP",
    earlyEnd: null,
    runeIds: [8005],
    runePrimary: 8000,
    runeSecondary: 8100,
    ...overrides,
  };
}
export function makePlayer(
  index = 0,
  overrides: Partial<Participant> = {},
): Participant {
  const history = Array.from({ length: 20 }, (_, i) => makeMatch(i));
  return {
    connectionId: 1,
    key: `synthetic-${index}`,
    displayName: `合成玩家${index + 1}#DEMO`,
    team: index < 5 ? "ally" : "enemy",
    championId: [23, 24, 25][index % 3],
    championState: "selected",
    identityState: "known" as IdentityState,
    privacyState: "public",
    historyState: "ready" as HistoryState,
    history,
    historyMeta: {
      returned: 20,
      parsed: 20,
      excluded: 0,
      fetchedAt: exampleTime,
      scope: "合成示例 · 最近 20 场",
    },
    cached: false,
    stale: false,
    stats: sampleStats(history),
    profile: {
      updatedAt: exampleTime,
      fields: {
        level: { state: "current", fetchedAt: exampleTime },
        rank: { state: "current", fetchedAt: exampleTime },
      },
      privacyState: "public",
      profileIconId: null,
      profileIconData: "",
      level: 260,
      rankState: "ready",
      ranks: [
        {
          queue: "RANKED_SOLO_5x5",
          tier: "DIAMOND",
          division: "III",
          lp: 74,
          wins: 24,
          losses: 21,
        },
      ],
    },
    ...overrides,
  };
}
export function makeSnapshot(overrides: Partial<Snapshot> = {}): Snapshot {
  return {
    connectionId: 1,
    generation: 1,
    selfKey: "synthetic-0",
    connected: true,
    phase: "InProgress",
    stage: "in-game",
    knownCount: 10,
    readyCount: 10,
    rosterError: "",
    region: "synthetic",
    gameId: 900000,
    frozen: true,
    updatedAt: exampleTime,
    participants: Array.from({ length: 10 }, (_, i) => makePlayer(i)),
    ...overrides,
  };
}
export function makeOverlayState(): OverlayState {
  return {
    revision: 1,
    visible: true,
    pinned: true,
    shortcutAvailable: true,
    warning: "",
    shortcut: "Ctrl+Shift+P",
    mode: "persistent",
    holdEnabled: true,
    holdKey: "Backquote",
    holdAvailable: true,
    holdWarning: "",
  };
}

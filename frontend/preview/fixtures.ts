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
  const history = Array.from({ length: 20 }, (_, i) => makeMatch(i, {
    position: ["TOP", "JUNGLE", "MIDDLE", "BOTTOM", "UTILITY"][index % 5],
    playedAt: new Date(Date.parse(exampleTime) - i * 7200000).toISOString(),
  }));
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
    scout: {
      state: "ready", fetchedAt: exampleTime,
      season: { name: "2026 合成赛季", start: "2026-01-01T00:00:00Z", end: "2027-01-01T00:00:00Z" },
      complete: true, complete12: true, complete30: true, read: 1200,
      champions: [23,24,25,107].map(championId => ({ championId, games: 180 + index * 11,
        mastery: { championId, level: 22 + index, points: 180000 + index * 15000 },
        queues: [420,440].map(queueId => ({ queueId, stats: { games: 90, wins: 54, losses: 36, outcomeGames: 90, kdaGames: 90, winRate: 60, kda: 3.5 }, average: [6,3,4.5] })),
      })),
      queues: [420,440].map(queueId => ({ queueId,
        season: { games: 500, wins: 300, losses: 200, outcomeGames: 500, kdaGames: 500, winRate: 60, kda: 3.5 },
        twelve: { games: 8, wins: 5, losses: 3, outcomeGames: 8, kdaGames: 8, winRate: 62.5, kda: 3.5 },
        month: { games: 100, wins: 60, losses: 40, outcomeGames: 100, kdaGames: 100, winRate: 60, kda: 3.5 },
        positions: [{ position: ["TOP","JUNGLE","MIDDLE","BOTTOM","UTILITY"][index % 5]!, games: 70 }, { position: index % 5 === 2 ? "JUNGLE" : "MIDDLE", games: 30 }],
      })),
    },
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
    queueId: 420,
    scoutQueue: 420,
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
    sidebarEnabled:true,
    previewEnabled:true,
    sidebarWidth:320,
    sidebarSide:'left',
    collapsed:false,
  };
}

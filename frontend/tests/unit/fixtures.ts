import { HistoryState, IdentityState, type Match, type Participant, type HistoryPage } from "../../bindings/leaguepingo/internal/core/models";

// Synthetic fixtures intentionally contain only the fields exercised by the
// display logic. They never connect to the client or contain a real identity.
export function match(gameId: number, queueId = 420, fields: Partial<Match> = {}): Match {
  return { gameId, queueId, championId: 1, win: null, kills: null, deaths: null, assists: null, duration: null, earlyEnd: null, ...fields } as Match;
}

export function matches(start: number, count = 20, queue = 420) {
  return Array.from({ length: count }, (_, i) => match(start + i, queue));
}

export function page(offset: number, records = matches(offset + 1), hasMore = true): HistoryPage {
  return {
    offset, nextOffset: offset + records.length, matches: records, hasMore,
    meta: { returned: records.length, parsed: records.length, excluded: 0, fetchedAt: "2026-01-01T00:00:00Z", scope: "synthetic" },
  };
}

export function player(history = matches(1), fields: Partial<Participant> = {}): Participant {
  return {
    key: "synthetic-player", connectionId: 1, displayName: "Synthetic#TEST",
    identityState: IdentityState.IdentityKnown, historyState: HistoryState.HistoryReady,
    history, historyMeta: page(0, history).meta, ...fields,
  } as Participant;
}

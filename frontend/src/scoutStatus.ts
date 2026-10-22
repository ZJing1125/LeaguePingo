import type { Participant } from "../bindings/leaguepingo/internal/core/models";

// Coverage and freshness belong to the independent report, never the history list.
export function scoutStatus(players: Participant[], queue = 420) {
  const known = players.filter(p => p.identityState === "known");
  const reports = known.map(p => p.scout);
  const hasMode = (p: Participant) => p.scout?.queues?.some(q => q.queueId === queue);
  const complete = known.filter(p => hasMode(p) && p.scout?.complete && p.scout.complete12 && p.scout.complete30).length;
  const loading = reports.filter(r => r?.state === "loading").length;
  const partial = known.filter(p => hasMode(p) && !(p.scout?.complete && p.scout.complete12 && p.scout.complete30)).length;
  const times = known.filter(hasMode).map(p => p.scout?.fetchedAt).filter((t): t is string => !!t && Number.isFinite(Date.parse(t)) && Date.parse(t) > 0).sort((a, b) => Date.parse(a) - Date.parse(b));
  return {
    total: players.length, known: known.length, complete, loading, partial,
    recent: known.filter(p => ["ready", "empty"].includes(p.historyState)).length,
    retained: reports.filter(r => r?.retained).length,
    earliest: times[0], latest: times.at(-1), untimed: known.length - times.length,
  };
}

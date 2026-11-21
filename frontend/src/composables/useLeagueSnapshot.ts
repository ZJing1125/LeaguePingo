import { recordVisibleResponse } from "../responseMetrics";
import { applyAnalysisBrief } from "../analysisBriefState";
import { computed, ref } from "vue";
import type { Snapshot as SnapshotModel } from "../../bindings/leaguepingo/internal/core/models";

// Snapshot ordering is independent from the workspace that presents it.
export function useLeagueSnapshot() {
  const snapshot = ref<SnapshotModel | null>(null);
  let observedReports = new Map<string, number>();
  function acceptSnapshot(next: SnapshotModel) {
    const current = snapshot.value;
    if (
      current &&
      ((next.generation ?? 0) < (current.generation ?? 0) ||
        (next.generation === current.generation &&
          Date.parse(next.updatedAt) < Date.parse(current.updatedAt)))
    )
      return false;
    const started = performance.now();
    snapshot.value = next;
    if (["party", "preparing"].includes(next.stage)) {
      const category = typeof location !== "undefined" && new URLSearchParams(location.search).get("view") === "companion" ? "sidebar-render" : "room-render";
      recordVisibleResponse(category, started, () => snapshot.value === next || snapshot.value?.updatedAt === next.updatedAt);
      const reports = new Map<string, number>();
      for (const player of next.participants ?? []) {
        const id = player.scout?.responseId;
        const key = `${next.connectionId}:${player.key}`;
        if (!id || typeof document === "undefined" || document.visibilityState !== "visible") continue;
        reports.set(key, id);
        if (observedReports.get(key) === id) continue;
        recordVisibleResponse(null, started, () => snapshot.value?.connectionId === next.connectionId && snapshot.value.participants?.some(p => p.key === player.key && p.scout?.responseId === id) === true, id);
      }
      observedReports = reports;
    }
    applyAnalysisBrief(next.analysisBrief);
    return true;
  }
  const canRefresh = computed(
    () =>
      snapshot.value?.connected &&
      [
        "Lobby",
        "Matchmaking",
        "ReadyCheck",
        "ChampSelect",
        "GameStart",
        "InProgress",
        "Reconnect",
      ].includes(snapshot.value.phase),
  );
  const status = computed(() => {
    const s = snapshot.value;
    if (!s?.connected) return s?.frozen ? "连接中断 · 保留资料" : "等待客户端";
    if (
      s.stage === "in-game" &&
      s.participants?.some((p) => p.historyState === "loading")
    )
      return "对局中 · 开局补齐资料";
    const phases: Record<string, string> = {
      Lobby: "组队大厅",
      Matchmaking: "正在排队",
      ReadyCheck: "确认对局",
      ChampSelect: "选人 · 准备资料",
      GameStart: "对局载入 · 准备玩家分析",
      InProgress: "对局中 · 玩家分析",
      Reconnect: "重连中",
      WatchInProgress: "观战中",
      WaitingForStats: "等待结算",
      PreEndOfGame: "正在结算",
      EndOfGame: "对局结束",
    };
    return phases[s.phase] ?? "客户端已连接";
  });

  return { snapshot, acceptSnapshot, canRefresh, status };
}

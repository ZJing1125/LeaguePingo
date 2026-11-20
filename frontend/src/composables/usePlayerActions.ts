import { computed, onUnmounted, ref, watch, type Ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import { HistoryState } from "../../bindings/leaguepingo/internal/core/models";
import type {
  Participant,
  Snapshot as SnapshotModel,
} from "../../bindings/leaguepingo/internal/core/models";
import { errorMessage, normalizeRiotIDInput } from "../presentation";
import { completeRiotID, type PlayerHistoryTarget } from "../playerHistoryNavigation";
import { recordVisibleResponse } from "../responseMetrics";
import { newestPlayer, operationKey } from "../playerState";
import type { usePlayerWorkspace } from "./usePlayerWorkspace";

// All manual operations still share the same Go scheduler and UI busy lane.
export function usePlayerActions(
  snapshot: Ref<SnapshotModel | null>,
  workspace: ReturnType<typeof usePlayerWorkspace>,
  acceptSnapshot: (next: SnapshotModel) => void,
  setMessage: (key: string, message: string) => void,
) {
  const { tabs, selectedKey, selectedResult, selected, openPlayer } = workspace;
  const query = ref("");
  const searching = ref(false);
  const rosterBusy = ref(false);
  const playerOperation = ref<{
    key: string;
    kind: "history" | "profile";
  } | null>(null);
  const manualBusy = computed(
    () => searching.value || playerOperation.value?.kind === "history",
  );
  const pendingProfile = ref<Participant | null>(null);
  const profileOpening = ref(false);
  let componentAlive = true;
  let navigationRevision = 0;
  let navigationTarget = "";
  let profileInterrupted = false;
  let profileCall: ReturnType<typeof Service.EnsureProfile> | undefined;
  let navigationCall: ReturnType<typeof Service.Lookup> | undefined;
  let navigationDone: Promise<void> = Promise.resolve();
  const actionBusy = computed(() => manualBusy.value || rosterBusy.value);
  function requestMissingProfile(player: Participant) {
    pendingProfile.value = null;
    if (
      profileOpening.value &&
      playerOperation.value?.key === operationKey(player)
    )
      return;
    if (
      player.identityState !== "known" ||
      player.stale ||
      !snapshot.value?.connected
    )
      return;
    if (
      player.profile?.profileIconData &&
      player.profile?.level != null &&
      player.profile?.rankState === "ready"
    )
      return;
    pendingProfile.value = player;
    void flushMissingProfile();
  }
  function acceptProfile(result: Participant) {
    if (
      !componentAlive ||
      !result?.key ||
      !snapshot.value?.connected ||
      (result.connectionId ?? 0) !== (snapshot.value.connectionId ?? 0)
    )
      return;
    const merge = (p: Participant) =>
      newestPlayer(
        { ...p, displayName: result.displayName },
        {
          ...p,
          profile: result.profile,
          privacyState: result.privacyState,
        },
      )!;
    for (let i = 0; i < tabs.value.length; i++) {
      if (operationKey(tabs.value[i]!) === operationKey(result))
        tabs.value[i] = merge(tabs.value[i]!);
    }
    if (
      operationKey(selected.value ?? result) === operationKey(result) &&
      selectedKey.value === result.key
    ) {
      selectedResult.value = merge(selected.value ?? result);
    }
    snapshot.value.participants = (snapshot.value.participants ?? []).map(
      (p) => (operationKey(p) === operationKey(result) ? merge(p) : p),
    );
  }
  async function flushMissingProfile() {
    if (
      !componentAlive ||
      !pendingProfile.value ||
      actionBusy.value ||
      profileOpening.value
    )
      return;
    const target = pendingProfile.value;
    pendingProfile.value = null;
    if (
      !snapshot.value?.connected ||
      (target.connectionId ?? 0) !== (snapshot.value.connectionId ?? 0)
    )
      return;
    profileOpening.value = true;
    profileInterrupted = false;
    playerOperation.value = { key: operationKey(target), kind: "profile" };
    try {
      const call = Service.EnsureProfile(target.key, target.connectionId);
      profileCall = call;
      acceptProfile(await call);
    } catch (reason) {
      if (
        !profileInterrupted &&
        componentAlive &&
        snapshot.value?.connected &&
        (target.connectionId ?? 0) === (snapshot.value.connectionId ?? 0)
      )
        setMessage(operationKey(target), errorMessage(reason));
    } finally {
      profileCall = undefined;
      playerOperation.value = null;
      profileOpening.value = false;
      void flushMissingProfile();
    }
  }
  watch(actionBusy, (busy) => {
    if (!busy) void flushMissingProfile();
  });
  function openPlayerDetails(player: Participant) {
    openPlayer(player);
    requestMissingProfile(player);
  }
  function refreshing(player: Participant) {
    return (
      playerOperation.value?.key === operationKey(player) &&
      playerOperation.value.kind === "history"
    );
  }
  watch(workspace.view, () => setMessage("historyJump", ""));
  function canOpenHistory(target?: PlayerHistoryTarget): target is PlayerHistoryTarget {
    return !!target && !!snapshot.value?.connected && target.connectionId === snapshot.value.connectionId;
  }

  watch(() => [workspace.activeKey.value, workspace.view.value, snapshot.value?.connected, snapshot.value?.connectionId], () => {
    if (navigationCall && (workspace.activeKey.value !== navigationTarget || workspace.view.value !== "players" || !snapshot.value?.connected))
      navigationCall.cancel();
  }, { flush: "sync" });

  async function openHistory(target: PlayerHistoryTarget) {
    if (!canOpenHistory(target)) return;
    const previousWorking = searching.value;
    const canceled = navigationCall?.cancel();
    const request = ++navigationRevision;
    profileInterrupted = true;
    profileCall?.cancel();
    const existing = tabs.value.find(player => !player.stale && player.connectionId === target.connectionId && (
      target.kind === "livePlayer" ? player.key === target.key
        : completeRiotID(player.displayName).toLowerCase() === target.riotId.toLowerCase()
    ));
    setMessage("historyJump", "");
    setMessage("search", "");
    const live = target.kind === "livePlayer"
      ? snapshot.value?.participants?.find(p => p.key === target.key) : undefined;
    const placeholder = existing ?? live ?? ({
      key: `query-${target.connectionId}-${target.kind === "riotId" ? target.riotId : target.key}`,
      puuid: "", displayName: target.label, connectionId: target.connectionId,
      identityState: "unavailable", historyState: "loading", history: [], historyMeta: {},
    } as unknown as Participant);
    const started = performance.now();
    navigationTarget = placeholder.key;
    openPlayer(placeholder);
    recordVisibleResponse("history-feedback", started, () => componentAlive && request === navigationRevision);
    const stillCurrent = () => componentAlive && request === navigationRevision && canOpenHistory(target)
      && workspace.view.value === "players" && workspace.activeKey.value === placeholder.key;
    if (existing && existing.historyState !== "loading" && existing.historyState !== "pending") {
      searching.value = false;
      requestMissingProfile(existing);
      recordVisibleResponse("player-history-ready", started, () => workspace.activeKey.value === existing.key);
      return existing;
    }
    const previous = navigationDone;
    let finish!: () => void;
    navigationDone = new Promise(resolve => { finish = resolve; });
    searching.value = true;
    try {
      await canceled;
      await previous;
      if (previousWorking && stillCurrent()) await Service.WaitForQueryIdle();
      if (!stillCurrent()) return;
      const call = target.kind === "livePlayer" ? Service.LookupPlayer(target.key) : Service.Lookup(target.riotId);
      navigationCall = call;
      const player = await call;
      if (!stillCurrent() || player.connectionId !== target.connectionId) return;
      navigationCall = undefined;
      if (placeholder.key !== player.key) workspace.closeTab(placeholder.key);
      openPlayer(player);
      // Optional profile is scheduled after the history becomes usable.
      requestMissingProfile(player);
      recordVisibleResponse("player-history-ready", started, () => componentAlive && request === navigationRevision && workspace.activeKey.value === player.key);
      return player;
    } catch (reason) {
      if (stillCurrent()) {
        const pending = tabs.value.find(p => p.key === placeholder.key);
        if (pending) {
          pending.historyState = HistoryState.HistoryFailed;
          pending.error = errorMessage(reason);
        }
        setMessage("historyJump", errorMessage(reason));
      }
    } finally {
      finish();
      if (request === navigationRevision) {
        navigationCall = undefined;
        searching.value = false;
        void flushMissingProfile();
      }
    }
  }

  async function search() {
    if (manualBusy.value) return;
    profileInterrupted = true;
    profileCall?.cancel();
    const id = normalizeRiotIDInput(query.value);
    query.value = id;
    setMessage("historyJump", "");
    if (!completeRiotID(id)) {
      setMessage("search", "请输入完整的 玩家名#标签");
      return;
    }
    searching.value = true;
    setMessage("search", "");
    try {
      const player = await Service.Lookup(id);
      openPlayer(player);
      requestMissingProfile(player);
    } catch (reason) {
      setMessage("search", errorMessage(reason));
    } finally {
      searching.value = false;
    }
  }
  async function refreshPlayer(player: Participant) {
    if (manualBusy.value) return;
    const requestKey = operationKey(player);
    playerOperation.value = { key: requestKey, kind: "history" };
    setMessage(requestKey, "");
    try {
      const result = await Service.RefreshData(player.key, player.connectionId);
      acceptProfile(result);
      if (selectedKey.value === player.key) selectedResult.value = result;
      const existing = tabs.value.findIndex(
        (p) => operationKey(p) === operationKey(result),
      );
      if (existing >= 0)
        tabs.value[existing] = newestPlayer(result, tabs.value[existing]!)!;
      if (result.error) setMessage(requestKey, result.error);
      return result;
    } catch (reason) {
      setMessage(requestKey, errorMessage(reason));
    } finally {
      playerOperation.value = null;
    }
  }
  function selectPlayer(player: Participant) {
    selectedKey.value = player.key;
    selectedResult.value = null;
    void Service.PrioritizePlayer(player.key).catch((reason) => {
      if (componentAlive)
        setMessage(operationKey(player), errorMessage(reason));
    });
    requestMissingProfile(player);
  }
  async function refreshRoster() {
    if (rosterBusy.value) return;
    rosterBusy.value = true;
    setMessage("roster", "");
    try {
      acceptSnapshot(await Service.RefreshRoster());
    } catch (reason) {
      setMessage("roster", errorMessage(reason));
    } finally {
      rosterBusy.value = false;
    }
  }

  onUnmounted(() => {
    componentAlive = false;
    navigationRevision++;
    navigationCall?.cancel();
    profileCall?.cancel();
    pendingProfile.value = null;
  });
  return {
    query,
    searching,
    rosterBusy,
    playerOperation,
    manualBusy,
    refreshing,
    openPlayerDetails,
    search,
    canOpenHistory,
    openHistory,
    refreshPlayer,
    selectPlayer,
    refreshRoster,
  };
}

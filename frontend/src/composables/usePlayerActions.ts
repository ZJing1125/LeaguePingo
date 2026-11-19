import { computed, onUnmounted, ref, watch, type Ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type {
  Participant,
  Snapshot as SnapshotModel,
} from "../../bindings/leaguepingo/internal/core/models";
import { errorMessage, normalizeRiotIDInput } from "../presentation";
import { completeRiotID, type PlayerHistoryTarget } from "../playerHistoryNavigation";
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
    () => searching.value || playerOperation.value !== null,
  );
  const pendingProfile = ref<Participant | null>(null);
  const profileOpening = ref(false);
  let componentAlive = true;
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
    playerOperation.value = { key: operationKey(target), kind: "profile" };
    try {
      acceptProfile(
        await Service.EnsureProfile(target.key, target.connectionId),
      );
    } catch (reason) {
      if (
        componentAlive &&
        snapshot.value?.connected &&
        (target.connectionId ?? 0) === (snapshot.value.connectionId ?? 0)
      )
        setMessage(operationKey(target), errorMessage(reason));
    } finally {
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

  async function openHistory(target: PlayerHistoryTarget) {
    if (manualBusy.value || !canOpenHistory(target)) return;
    const existing = tabs.value.find(player => !player.stale && player.connectionId === target.connectionId && (
      target.kind === "livePlayer" ? player.key === target.key
        : completeRiotID(player.displayName).toLowerCase() === target.riotId.toLowerCase()
    ));
    setMessage("historyJump", "");
    setMessage("search", "");
    if (existing) {
      openPlayer(existing);
      return existing;
    }
    const origin = workspace.view.value;
    const tab = workspace.activeKey.value;
    const stillCurrent = () => componentAlive && canOpenHistory(target)
      && workspace.view.value === origin && workspace.activeKey.value === tab;
    searching.value = true;
    try {
      const player = target.kind === "livePlayer" ? await Service.LookupPlayer(target.key) : await Service.Lookup(target.riotId);
      if (!stillCurrent() || player.connectionId !== target.connectionId) return;
      openPlayer(player);
      return player;
    } catch (reason) {
      if (stillCurrent()) setMessage("historyJump", errorMessage(reason));
    } finally {
      searching.value = false;
    }
  }

  async function search() {
    if (manualBusy.value) return;
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
      openPlayer(await Service.Lookup(id));
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

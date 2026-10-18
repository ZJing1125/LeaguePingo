import { computed, onUnmounted, ref, watch, type Ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type {
  Participant,
  Snapshot as SnapshotModel,
} from "../../bindings/leaguepingo/internal/core/models";
import { historyBrowseBusy } from "../historyBrowser";
import { errorMessage, normalizeRiotIDInput } from "../presentation";
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
    kind: "history" | "profile" | "lookup";
  } | null>(null);
  const manualBusy = computed(
    () =>
      searching.value ||
      playerOperation.value !== null ||
      historyBrowseBusy.value,
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
    if (player.profile?.profileIconData && player.profile?.level != null)
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
  async function search() {
    if (manualBusy.value) return;
    const id = normalizeRiotIDInput(query.value);
    query.value = id;
    if (!/^[^#]+#[^#]+$/.test(id) || /[\u0000-\u001f\u007f-\u009f]/.test(id)) {
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
      const result = await Service.RefreshHistory(
        player.key,
        player.connectionId,
      );
      if (selectedKey.value === player.key) selectedResult.value = result;
      openPlayer(result, false);
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
  async function lookupSelected() {
    if (!selected.value || manualBusy.value) return;
    const target = selected.value;
    const requestKey = operationKey(target);
    playerOperation.value = { key: requestKey, kind: "lookup" };
    setMessage(requestKey, "");
    try {
      const result = await Service.LookupPlayer(target.key);
      if (selectedKey.value === target.key) selectedResult.value = result;
      openPlayer(result, false);
    } catch (reason) {
      setMessage(requestKey, errorMessage(reason));
    } finally {
      playerOperation.value = null;
    }
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
  async function loadProfile(player: Participant) {
    if (manualBusy.value) return;
    const requestKey = operationKey(player);
    playerOperation.value = { key: requestKey, kind: "profile" };
    setMessage(requestKey, "");
    try {
      const result = await Service.LoadProfile(player.key, player.connectionId);
      openPlayer(result, false);
      if (selectedKey.value === player.key) selectedResult.value = result;
    } catch (reason) {
      setMessage(requestKey, errorMessage(reason));
    } finally {
      playerOperation.value = null;
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
    refreshPlayer,
    selectPlayer,
    lookupSelected,
    refreshRoster,
    loadProfile,
  };
}

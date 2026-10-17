import { computed, reactive, ref, type Ref } from "vue";
import type {
  Participant,
  Snapshot as SnapshotModel,
} from "../../bindings/leaguepingo/internal/core/models";
import type { PlayerViewState } from "../presentation";
import { newestPlayer } from "../playerState";

export function usePlayerWorkspace(
  snapshot: Ref<SnapshotModel | null>,
  onClose?: (player: Participant) => void,
) {
  const view = ref<"players" | "match">("players");
  const tabs = ref<Participant[]>([]),
    activeKey = ref(""),
    selectedKey = ref(""),
    selectedResult = ref<Participant | null>(null);
  const viewStates = reactive<Record<string, PlayerViewState>>({});
  const players = computed(() => snapshot.value?.participants ?? []);
  const active = computed(
    () => tabs.value.find((p) => p.key === activeKey.value) ?? null,
  );
  const selected = computed(() =>
    newestPlayer(
      players.value.find((p) => p.key === selectedKey.value) ?? null,
      selectedResult.value?.key === selectedKey.value
        ? selectedResult.value
        : null,
    ),
  );
  function acceptSnapshot(next: SnapshotModel) {
    for (const tab of tabs.value) {
      if (
        !next.connected ||
        (tab.connectionId ?? 0) !== (next.connectionId ?? 0)
      )
        tab.stale = true;
    }
    if (
      selectedResult.value &&
      (!next.connected ||
        (selectedResult.value.connectionId ?? 0) !== (next.connectionId ?? 0))
    )
      selectedResult.value.stale = true;
    // Update only newer verified histories; a stage change cannot erase tabs.
    for (const player of next.participants ?? []) {
      const index = tabs.value.findIndex((p) => p.key === player.key);
      if (index >= 0)
        tabs.value[index] = newestPlayer(player, tabs.value[index]!)!;
    }
  }
  function stateFor(key: string) {
    return (
      viewStates[key] ?? (viewStates[key] = { queue: "all", expanded: [] })
    );
  }
  function openPlayer(player: Participant, activate = true) {
    const index = tabs.value.findIndex((p) => p.key === player.key);
    if (index >= 0)
      tabs.value[index] = newestPlayer(player, tabs.value[index]!)!;
    else {
      if (tabs.value.length >= 8) {
        const removed = tabs.value.shift();
        if (removed) {
          delete viewStates[removed.key];
          onClose?.(removed);
          if (activeKey.value === removed.key)
            activeKey.value = tabs.value[0]?.key ?? "";
        }
      }
      tabs.value.push(player);
    }
    stateFor(player.key);
    if (!activeKey.value) activeKey.value = player.key;
    if (activate) {
      activeKey.value = player.key;
      view.value = "players";
    }
  }
  function closeTab(key: string) {
    const old = tabs.value.find((p) => p.key === key);
    if (old) onClose?.(old);
    const i = tabs.value.findIndex((p) => p.key === key);
    tabs.value = tabs.value.filter((p) => p.key !== key);
    delete viewStates[key];
    if (activeKey.value === key)
      activeKey.value =
        tabs.value[Math.min(i, tabs.value.length - 1)]?.key ?? "";
  }

  return {
    view,
    tabs,
    activeKey,
    selectedKey,
    selectedResult,
    active,
    selected,
    players,
    stateFor,
    openPlayer,
    closeTab,
    acceptSnapshot,
  };
}

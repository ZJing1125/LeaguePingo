import { computed, onUnmounted, ref, watch, type Ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { Participant, Snapshot } from "../../bindings/leaguepingo/internal/core/models";
import { newestPlayer } from "../playerState";
import { errorMessage } from "../presentation";

export function useHomePlayer(snapshot: Ref<Snapshot | null>, view: Ref<string>, busy: Ref<boolean>) {
  const result = ref<Participant | null>(null);
  const loading = ref(false), error = ref("");
  let attempted = "", alive = true;
  const player = computed(() => {
    const s = snapshot.value;
    if (!s?.selfKey) return null;
    const cached = result.value?.key === s.selfKey && result.value.connectionId === s.connectionId ? result.value : null;
    const value = newestPlayer(s.participants?.find(p => p.key === s.selfKey) ?? null, cached);
    return value && !s.connected ? { ...value, stale: true } : value;
  });
  watch(() => snapshot.value?.connectionId, () => {
    result.value = null;
    error.value = "";
  });
  async function load(explicit = false) {
    const s = snapshot.value;
    if (!s?.connected || !s.selfKey || busy.value || loading.value) return;
    if (!explicit && (view.value !== "home" || s.stage !== "idle")) return;
    const token = `${s.connectionId}:${s.selfKey}`;
    if (!explicit && attempted === token) return;
    attempted = token;
    loading.value = true;
    error.value = "";
    try {
      const p = await Service.CurrentPlayer(s.connectionId);
      if (alive && snapshot.value?.connected && snapshot.value.connectionId === s.connectionId && snapshot.value.selfKey === p.key)
        result.value = newestPlayer(p, result.value);
    } catch (reason) {
      if (alive && snapshot.value?.connectionId === s.connectionId && (explicit || snapshot.value.stage === "idle")) error.value = errorMessage(reason);
    } finally { loading.value = false; }
  }
  watch(() => [view.value, snapshot.value?.connected, snapshot.value?.connectionId, snapshot.value?.selfKey, snapshot.value?.stage, busy.value, loading.value], () => { void load(); }, { immediate: true });
  function accept(player: Participant) {
    if (player.key === snapshot.value?.selfKey && player.connectionId === snapshot.value.connectionId)
      result.value = newestPlayer(player, result.value);
  }
  onUnmounted(() => { alive = false; });
  return { player, loading, error, accept, retry: () => load(true) };
}

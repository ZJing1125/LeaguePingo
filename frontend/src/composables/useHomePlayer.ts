import { computed, onUnmounted, ref, watch, type Ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { Participant, Snapshot } from "../../bindings/leaguepingo/internal/core/models";
import { newestPlayer, playerProfileComplete } from "../playerState";
import { errorMessage } from "../presentation";

const retryDelays = [2000, 5000];
const recoveryWindow = 60000;

export function useHomePlayer(snapshot: Ref<Snapshot | null>, view: Ref<string>, busy: Ref<boolean>) {
  const result = ref<Participant | null>(null);
  const loading = ref(false), error = ref("");
  const scope = computed(() => `${snapshot.value?.connectionId}:${snapshot.value?.selfKey}`);
  const eligible = computed(() => view.value === "home" && snapshot.value?.connected &&
    !!snapshot.value.selfKey && snapshot.value.stage === "idle" && !busy.value);
  let alive = true, epoch = 0, attempts = 0, profileAttempts = 0, started = 0;
  let retryReady = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let homeCall: ReturnType<typeof Service.CurrentPlayer> | undefined;
  let profileCall: ReturnType<typeof Service.EnsureProfile> | undefined;

  function clearTimer() {
    clearTimeout(timer);
    timer = undefined;
  }
  function cancelCalls() {
    epoch++;
    homeCall?.cancel?.();
    profileCall?.cancel?.();
    homeCall = undefined;
    profileCall = undefined;
    loading.value = false;
  }
  function current(token: string) {
    return alive && scope.value === token && !!snapshot.value?.connected;
  }
  function scheduleRetry() {
    if (timer || attempts > retryDelays.length || Date.now() - started >= recoveryWindow) return;
    const token = scope.value;
    timer = setTimeout(() => {
      timer = undefined;
      if (!current(token)) return;
      retryReady = true;
      void load();
    }, retryDelays[Math.max(0, attempts - 1)]);
  }
  async function completeProfile(p: Participant) {
    if (playerProfileComplete(p) || !eligible.value || profileAttempts >= 2 || profileCall) return;
    profileAttempts++;
    const token = scope.value, request = epoch;
    const call = Service.EnsureProfile(p.key, p.connectionId);
    profileCall = call;
    try {
      const updated = await call;
      if (current(token) && epoch === request && updated.key === p.key &&
        updated.puuid === p.puuid && updated.connectionId === p.connectionId) {
        result.value = newestPlayer(updated, result.value);
        if (!playerProfileComplete(updated) && profileAttempts < 2) scheduleRetry();
      }
    } catch {
      // A profile failure retains the already visible history.
      if (current(token) && epoch === request) scheduleRetry();
    } finally {
      if (profileCall === call) profileCall = undefined;
    }
  }
  async function load(explicit = false) {
    const s = snapshot.value;
    if (!s?.connected || !s.selfKey || busy.value || loading.value) return;
    if (!explicit && (!eligible.value || attempts && !retryReady)) return;
    if (!explicit && attempts && Date.now() - started >= recoveryWindow) return;
    if (!explicit && attempts >= 3) return;
    retryReady = false;
    if (!explicit && attempts === 0) started = Date.now();
    if (!explicit) attempts++;
    const token = scope.value, request = epoch;
    if (!explicit && result.value) {
      void completeProfile(result.value);
      return;
    }
    loading.value = true;
    error.value = "";
    const call = Service.CurrentPlayer(s.connectionId);
    homeCall = call;
    try {
      const p = await call;
      if (current(token) && epoch === request && s.selfKey === p.key && s.connectionId === p.connectionId) {
        result.value = newestPlayer(p, result.value);
        void completeProfile(p);
      }
    } catch (reason) {
      if (current(token) && epoch === request) {
        error.value = errorMessage(reason);
        if (!explicit) scheduleRetry();
      }
    } finally {
      if (homeCall === call) {
        homeCall = undefined;
        loading.value = false;
      }
    }
  }
  watch(scope, () => {
    clearTimer();
    cancelCalls();
    result.value = null;
    error.value = "";
    attempts = profileAttempts = started = 0;
    retryReady = false;
  }, { flush: "sync" });
  watch([scope, eligible], ([, value]) => {
    if (!value) {
      const interrupted = !!homeCall || !!profileCall;
      cancelCalls();
      if (interrupted) scheduleRetry();
    } else void load();
  }, { immediate: true });
  const player = computed(() => {
    const s = snapshot.value;
    if (!s?.selfKey) return null;
    const cached = result.value?.key === s.selfKey && result.value.connectionId === s.connectionId ? result.value : null;
    const value = newestPlayer(s.participants?.find(p => p.key === s.selfKey) ?? null, cached);
    return value && !s.connected ? { ...value, stale: true } : value;
  });
  function accept(p: Participant) {
    if (p.key === snapshot.value?.selfKey && p.connectionId === snapshot.value.connectionId) {
      result.value = newestPlayer(p, result.value);
      if (playerProfileComplete(p)) clearTimer();
    }
  }
  onUnmounted(() => { alive = false; clearTimer(); cancelCalls(); });
  return { player, loading, error, accept, retry: () => load(true) };
}

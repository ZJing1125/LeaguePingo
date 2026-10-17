<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from "vue";
import { Events } from "@wailsio/runtime";
import * as Service from "../bindings/leaguepingo/appservice";
import type {
  Participant,
  Snapshot as SnapshotModel,
} from "../bindings/leaguepingo/internal/core/models";
import type { OverlayState } from "../bindings/leaguepingo/models";
import PlayerView from "./components/PlayerView.vue";
import OverlayView from "./components/OverlayView.vue";
import ParticipantAvatar from "./components/ParticipantAvatar.vue";
import TransientNotice from "./components/TransientNotice.vue";
import SettingsPanel from "./components/SettingsPanel.vue";
import AppIcon from "./components/AppIcon.vue";
import RecentForm from "./components/RecentForm.vue";
import { historyBrowseBusy } from "./historyBrowser";
import { useTransientMessages } from "./transientMessages";
import {
  errorMessage,
  normalizeRiotIDInput,
  percent,
  decimal,
  preparationLabel,
  type PlayerViewState,
} from "./presentation";

const isOverlay =
  new URLSearchParams(location.search).get("view") === "overlay";
const settingsOpen = ref(false);
const navCollapsed = ref(true);
const snapshot = ref<SnapshotModel | null>(null),
  overlay = ref<OverlayState | null>(null);
const view = ref<"players" | "match">("players"),
  query = ref(""),
  searching = ref(false);
const { messages: playerErrors, setMessage } = useTransientMessages();
const searchError = computed(() => playerErrors.search || "");
const rosterError = computed(() => playerErrors.roster || "");
const rosterBusy = ref(false),
  overlayBusy = ref(false),
  overlayError = ref("");
const tabs = ref<Participant[]>([]),
  activeKey = ref(""),
  selectedKey = ref(""),
  selectedResult = ref<Participant | null>(null);
const viewStates = reactive<Record<string, PlayerViewState>>({});
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
  if (player.profile?.profileIconData && player.profile?.level != null) return;
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
    newestPlayer(p, {
      ...p,
      profile: result.profile,
      privacyState: result.privacyState,
    })!;
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
  snapshot.value.participants = (snapshot.value.participants ?? []).map((p) =>
    operationKey(p) === operationKey(result) ? merge(p) : p,
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
    acceptProfile(await Service.EnsureProfile(target.key, target.connectionId));
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
function operationKey(player: Participant) {
  return `${player.connectionId ?? 0}:${player.key}`;
}
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
function newestPlayer(
  current: Participant | null,
  retained: Participant | null,
) {
  if (!retained) return current;
  if (!current) return retained;
  if ((current.connectionId ?? 0) !== (retained.connectionId ?? 0))
    return (current.connectionId ?? 0) > (retained.connectionId ?? 0)
      ? current
      : retained;
  const currentTime = Date.parse(current.historyMeta?.fetchedAt ?? "") || 0;
  const retainedTime = Date.parse(retained.historyMeta?.fetchedAt ?? "") || 0;
  const history = currentTime >= retainedTime ? current : retained;
  const currentProfileTime = Date.parse(current.profile?.updatedAt ?? "") || 0;
  const retainedProfileTime =
    Date.parse(retained.profile?.updatedAt ?? "") || 0;
  const profile =
    (currentProfileTime >= retainedProfileTime
      ? current.profile
      : retained.profile) ??
    current.profile ??
    retained.profile;
  return profile
    ? { ...history, profile, privacyState: profile.privacyState }
    : history;
}
function acceptSnapshot(next: SnapshotModel) {
  const current = snapshot.value;
  if (
    current &&
    ((next.generation ?? 0) < (current.generation ?? 0) ||
      (next.generation === current.generation &&
        Date.parse(next.updatedAt) < Date.parse(current.updatedAt)))
  )
    return;
  snapshot.value = next;
  for (const tab of tabs.value) {
    if (!next.connected || (tab.connectionId ?? 0) !== (next.connectionId ?? 0))
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
  const phases: Record<string, string> = {
    Lobby: "组队大厅",
    Matchmaking: "正在排队",
    ReadyCheck: "确认对局",
    ChampSelect: "选人 · 准备资料",
    GameStart: "对局载入 · 自动预取暂停",
    InProgress: "对局中 · 自动预取暂停",
    Reconnect: "重连中",
    WatchInProgress: "观战中",
    WaitingForStats: "等待结算",
    PreEndOfGame: "正在结算",
    EndOfGame: "对局结束",
  };
  return phases[s.phase] ?? "客户端已连接";
});
function stateFor(key: string) {
  return viewStates[key] ?? (viewStates[key] = { queue: "all", expanded: [] });
}
function openPlayer(player: Participant, activate = true) {
  const index = tabs.value.findIndex((p) => p.key === player.key);
  if (index >= 0) tabs.value[index] = newestPlayer(player, tabs.value[index]!)!;
  else {
    if (tabs.value.length >= 8) {
      const removed = tabs.value.shift();
      if (removed) {
        delete viewStates[removed.key];
        delete playerErrors[operationKey(removed)];
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
  if (old) delete playerErrors[operationKey(old)];
  const i = tabs.value.findIndex((p) => p.key === key);
  tabs.value = tabs.value.filter((p) => p.key !== key);
  delete viewStates[key];
  if (activeKey.value === key)
    activeKey.value = tabs.value[Math.min(i, tabs.value.length - 1)]?.key ?? "";
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
  void Service.PrioritizePlayer(player.key);
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
function acceptOverlay(state: OverlayState) {
  if (!overlay.value || state.revision >= overlay.value.revision)
    overlay.value = state;
}
async function control(action: () => Promise<OverlayState>) {
  if (overlayBusy.value) return;
  overlayBusy.value = true;
  overlayError.value = "";
  try {
    acceptOverlay(await action());
  } catch (reason) {
    overlayError.value = errorMessage(reason);
  } finally {
    overlayBusy.value = false;
  }
}
function hideOverlay() {
  return control(() => Service.HideOverlay());
}
function pinOverlay() {
  return control(() => Service.SetOverlayPinned(!overlay.value?.pinned));
}
async function toggleOverlay() {
  try {
    await Service.ToggleOverlay();
  } catch (reason) {
    overlayError.value = errorMessage(reason);
  }
}
function onKey(event: KeyboardEvent) {
  if (event.key === "Escape") {
    if (settingsOpen.value) {
      settingsOpen.value = false;
      return;
    }
    if (isOverlay) void hideOverlay();
    else selectedKey.value = "";
  }
}
let stopSnapshot: (() => void) | undefined,
  stopOverlay: (() => void) | undefined;
onMounted(async () => {
  window.addEventListener("keydown", onKey);
  stopSnapshot = Events.On("leaguepingo:snapshot", (event) => {
    acceptSnapshot(event.data as SnapshotModel);
  });
  stopOverlay = Events.On("leaguepingo:overlay", (event) =>
    acceptOverlay(event.data as OverlayState),
  );
  const initial = await Promise.allSettled([
    Service.Snapshot(),
    Service.OverlayStatus(),
  ]);
  if (initial[0].status === "fulfilled") acceptSnapshot(initial[0].value);
  else setMessage("search", errorMessage(initial[0].reason));
  if (initial[1].status === "fulfilled") acceptOverlay(initial[1].value);
  else overlayError.value = errorMessage(initial[1].reason);
});
onUnmounted(() => {
  componentAlive = false;
  pendingProfile.value = null;
  stopSnapshot?.();
  stopOverlay?.();
  window.removeEventListener("keydown", onKey);
});
</script>

<template>
  <OverlayView
    v-if="isOverlay"
    :snapshot="snapshot"
    :state="overlay"
    :status="status"
    :busy="overlayBusy"
    :error="overlayError"
    @hide="hideOverlay"
    @pin="pinOverlay"
  />
  <main v-else class="desktop-app" :class="{ 'nav-collapsed': navCollapsed }">
    <aside class="app-sidebar">
      <div class="app-brand">
        <span class="brand-mark">P</span>
        <div><strong>LeaguePingo</strong><small>战绩助手</small></div>
      </div>
      <nav aria-label="工作区">
        <button
          :class="{ active: view === 'players' }"
          :aria-current="view === 'players' ? 'page' : undefined"
          aria-label="玩家战绩"
          title="玩家战绩"
          @click="view = 'players'"
        >
          <AppIcon name="history" /><span>玩家战绩</span></button
        ><button
          :class="{ active: view === 'match' }"
          :aria-current="view === 'match' ? 'page' : undefined"
          aria-label="当前对局"
          title="当前对局"
          @click="view = 'match'"
        >
          <AppIcon name="match" /><span>当前对局</span
          ><i v-if="players.length">{{ players.length }}</i>
        </button>
      </nav>
      <div class="sidebar-bottom">
        <button
          class="icon-button nav-toggle square"
          :aria-label="navCollapsed ? '展开导航' : '收起导航'"
          :title="navCollapsed ? '展开导航' : '收起导航'"
          :aria-expanded="!navCollapsed"
          @click="navCollapsed = !navCollapsed"
        >
          <AppIcon name="panel" />
        </button>
        <button
          class="text-button settings-trigger"
          aria-label="设置与诊断"
          title="设置与诊断"
          @click="settingsOpen = true"
        >
          <AppIcon name="settings" /><span>设置与诊断</span>
        </button>
        <div class="sidebar-connection">
          <span
            class="connection-light"
            :class="{ online: snapshot?.connected }"
          ></span
          >{{ status }}<small>局内仅主动查询战绩</small>
        </div>
      </div>
    </aside>
    <div class="app-main">
      <header class="app-toolbar">
        <strong>{{ view === "players" ? "玩家战绩" : "当前对局" }}</strong>
        <span class="toolbar-status" :title="status"
          ><span
            class="connection-light"
            :class="{ online: snapshot?.connected }"
          ></span
          >{{ status.split(" · ")[0] }}</span
        >
        <form class="global-search" @submit.prevent="search">
          <AppIcon name="search" :size="16" />
          <input
            v-model="query"
            aria-label="玩家完整 ID"
            placeholder="搜索 玩家名#标签"
            autocomplete="off"
          /><button class="primary" :disabled="manualBusy">
            {{ searching ? "查询中…" : "查询战绩" }}
          </button>
        </form>
        <button
          class="icon-button"
          aria-label="显示／隐藏快览"
          title="显示／隐藏快览"
          @click="toggleOverlay"
        >
          <AppIcon name="panel" /><span>对局快览</span></button
        ><button class="icon-button small-control" @click="hideOverlay">
          隐藏快览
        </button>
      </header>
      <p
        v-if="overlayError || overlay?.warning"
        class="error-banner workspace-message"
        role="status"
      >
        {{ overlayError || overlay?.warning }}
      </p>
      <p
        v-if="!snapshot?.connected && snapshot?.message"
        class="error-banner workspace-message"
      >
        {{ snapshot.message }}
      </p>
      <TransientNotice
        :message="searchError"
        banner-class="workspace-message"
      />
      <section v-show="view === 'players'" class="players-workspace">
        <div
          v-if="tabs.length"
          class="player-tabs"
          role="tablist"
          aria-label="已查看玩家"
        >
          <div
            v-for="player in tabs"
            :key="player.key"
            class="player-tab"
            :class="{ active: activeKey === player.key }"
          >
            <button
              role="tab"
              :aria-selected="activeKey === player.key"
              :title="player.displayName"
              @click="activeKey = player.key"
            >
              {{ player.displayName }}</button
            ><button
              class="tab-close"
              :aria-label="`关闭 ${player.displayName}`"
              @click="closeTab(player.key)"
            >
              ×
            </button>
          </div>
        </div>
        <PlayerView
          v-if="active"
          :player="active"
          :state="stateFor(active.key)"
          :busy="manualBusy"
          :refreshing="refreshing(active)"
          :connected="snapshot?.connected"
          :error="playerErrors[operationKey(active)] || active.error"
          @refresh="refreshPlayer"
          @profile="loadProfile"
        />
        <div v-else class="empty-state initial-empty">
          <AppIcon name="search" :size="42" />
          <h1>查看玩家近期表现</h1>
          <p>输入完整 ID 查询战绩，或从当前对局中选择玩家。</p>
          <button class="secondary" @click="view = 'match'">
            查看当前对局
          </button>
        </div>
      </section>
      <section v-show="view === 'match'" class="match-workspace">
        <header class="roster-toolbar">
          <div>
            <h1>
              {{ snapshot?.stage === "party" ? "当前房间成员" : "本局名单" }}
            </h1>
            <span class="muted"
              >{{ status }} · 身份 {{ snapshot?.knownCount ?? 0 }}/{{
                players.length
              }}
              · 战绩 {{ snapshot?.readyCount ?? 0 }}/{{
                players.length
              }}
              已准备</span
            >
          </div>
          <button
            class="secondary"
            :disabled="rosterBusy || !canRefresh"
            @click="refreshRoster"
          >
            {{ rosterBusy ? "刷新中…" : "刷新对局名单" }}
          </button>
        </header>
        <TransientNotice :message="rosterError" />
        <p
          v-if="!rosterError && snapshot?.rosterError"
          class="error-banner"
          role="alert"
        >
          {{ snapshot?.rosterError }}
        </p>
        <div class="match-layout">
          <div v-if="!players.length" class="empty-state">
            <h2>尚无对局名单</h2>
            <p>选人或进入对局后，显示客户端提供的身份。</p>
            <p class="muted">排队和接受确认阶段通常无法取得双方十人的身份。</p>
          </div>
          <div v-else class="team-columns">
            <section
              v-for="team in ['ally', 'enemy']"
              :key="team"
              class="team-section"
            >
              <h2>
                {{
                  snapshot?.stage === "party"
                    ? team === "ally"
                      ? "房间成员"
                      : "对手未确定"
                    : team === "ally"
                      ? "我方"
                      : "对方"
                }}<small
                  >{{ players.filter((p) => p.team === team).length }} 人</small
                >
              </h2>
              <button
                v-for="player in players.filter((p) => p.team === team)"
                :key="player.key"
                class="player-row"
                :class="{ selected: selectedKey === player.key }"
                @click="selectPlayer(player)"
              >
                <ParticipantAvatar :player="player" />
                <span class="roster-identity"
                  ><strong :title="player.displayName">{{
                    player.displayName
                  }}</strong
                  ><small v-if="player.championState === 'intent'"
                    >预选英雄</small
                  ><small v-if="!player.championId">{{
                    player.championState === "unselected"
                      ? "暂未选择英雄"
                      : "英雄信息暂不可用"
                  }}</small
                  ><small
                    >{{ preparationLabel(player)
                    }}<template v-if="player.historyState === 'ready'">
                      · {{ player.stats.games }} 场样本</template
                    ></small
                  ><RecentForm
                    v-if="player.historyState === 'ready'"
                    :matches="player.history"
                /></span>
                <span class="roster-metrics" :title="preparationLabel(player)"
                  ><template v-if="player.historyState === 'ready'"
                    ><b>{{ percent(player.stats.winRate) }}</b
                    ><small
                      >{{ decimal(player.stats.kda) }} KDA</small
                    ></template
                  ><span v-else class="status-badge">{{
                    player.identityState === "unavailable"
                      ? "身份未知"
                      : "待准备"
                  }}</span></span
                >
              </button>
            </section>
          </div>
          <aside v-if="selected" class="player-drawer">
            <div class="drawer-toolbar">
              <button class="text-button" @click="openPlayerDetails(selected)">
                在战绩页打开 ↗</button
              ><button
                class="icon-button"
                aria-label="关闭玩家详情"
                @click="selectedKey = ''"
              >
                ×
              </button>
            </div>
            <p
              v-if="selected.identityState === 'unavailable'"
              class="quality-warning"
            >
              客户端未提供身份，可刷新名单或输入完整 ID。
            </p>
            <button
              v-else-if="!['ready', 'empty'].includes(selected.historyState)"
              class="primary drawer-query"
              :disabled="manualBusy"
              @click="lookupSelected"
            >
              {{
                playerOperation?.key === operationKey(selected) &&
                playerOperation?.kind === "lookup"
                  ? "查询中…"
                  : "手动查询此玩家"
              }}
            </button>
            <p
              v-if="!players.some((p) => p.key === selectedKey)"
              class="quality-warning"
            >
              该玩家不在当前名单，保留上次查看资料。
            </p>
            <PlayerView
              :player="selected"
              :state="stateFor(selected.key)"
              :busy="manualBusy"
              :refreshing="refreshing(selected)"
              :connected="snapshot?.connected"
              :error="playerErrors[operationKey(selected)] || selected.error"
              compact
              @refresh="refreshPlayer"
              @profile="loadProfile"
            />
          </aside>
        </div>
      </section>
    </div>
    <SettingsPanel v-if="settingsOpen" @close="settingsOpen = false" />
  </main>
</template>

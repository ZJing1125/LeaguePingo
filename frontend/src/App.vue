<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from "vue";
import { Events } from "@wailsio/runtime";
import * as Service from "../bindings/leaguepingo/appservice";
import type {
  Participant,
  Snapshot as SnapshotModel,
} from "../bindings/leaguepingo/internal/core/models";
import type { OverlayState } from "../bindings/leaguepingo/models";
import PlayerView from "./components/PlayerView.vue";
import OverlayView from "./components/OverlayView.vue";
import GameIcon from "./components/GameIcon.vue";
import SettingsPanel from "./components/SettingsPanel.vue";
import { errorMessage, percent, recentOutcomes, preparationLabel, type PlayerViewState } from "./presentation";

const isOverlay =
  new URLSearchParams(location.search).get("view") === "overlay";
const settingsOpen = ref(false);
const snapshot = ref<SnapshotModel | null>(null),
  overlay = ref<OverlayState | null>(null);
const view = ref<"players" | "match">("players"),
  query = ref(""),
  searching = ref(false),
  searchError = ref("");
const rosterBusy = ref(false),
  rosterError = ref(""),
  overlayBusy = ref(false),
  overlayError = ref("");
const tabs = ref<Participant[]>([]),
  activeKey = ref(""),
  selectedKey = ref(""),
  selectedResult = ref<Participant | null>(null),
  selectedError = ref("");
const viewStates = reactive<Record<string, PlayerViewState>>({});
const players = computed(() => snapshot.value?.participants ?? []);
const active = computed(
  () => tabs.value.find((p) => p.key === activeKey.value) ?? null,
);
const selected = computed(() =>
  newestPlayer(
    players.value.find((p) => p.key === selectedKey.value) ?? null,
    selectedResult.value?.key === selectedKey.value ? selectedResult.value : null,
  ),
);
function newestPlayer(current: Participant | null, retained: Participant | null) {
  if (!retained) return current;
  if (!current) return retained;
  if ((current.connectionId ?? 0) !== (retained.connectionId ?? 0))
    return (current.connectionId ?? 0) > (retained.connectionId ?? 0) ? current : retained;
  const currentTime = Date.parse(current.historyMeta?.fetchedAt ?? "") || 0;
  const retainedTime = Date.parse(retained.historyMeta?.fetchedAt ?? "") || 0;
  return currentTime >= retainedTime ? current : retained;
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
    if (!next.connected || (tab.connectionId ?? 0) !== (next.connectionId ?? 0)) tab.stale = true;
  }
  if (selectedResult.value && (!next.connected || (selectedResult.value.connectionId ?? 0) !== (next.connectionId ?? 0))) selectedResult.value.stale = true;
  // Update only newer verified histories; a stage change cannot erase tabs.
  for (const player of next.participants ?? []) {
    const index = tabs.value.findIndex((p) => p.key === player.key);
    if (index >= 0) tabs.value[index] = newestPlayer(player, tabs.value[index]!)!;
  }
}
const canRefresh = computed(
  () =>
    snapshot.value?.connected &&
    ["Lobby", "Matchmaking", "ReadyCheck", "ChampSelect", "GameStart", "InProgress", "Reconnect"].includes(
      snapshot.value.phase,
    ),
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
        if (activeKey.value === removed.key) activeKey.value = tabs.value[0]?.key ?? '';
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
  const i = tabs.value.findIndex((p) => p.key === key);
  tabs.value = tabs.value.filter((p) => p.key !== key);
  delete viewStates[key];
  if (activeKey.value === key)
    activeKey.value = tabs.value[Math.min(i, tabs.value.length - 1)]?.key ?? "";
}
async function search() {
  if (searching.value) return;
  const id = query.value.trim();
  if (!/^[^#]+#[^#]+$/.test(id)) {
    searchError.value = "请输入完整的 玩家名#标签";
    return;
  }
  searching.value = true;
  searchError.value = "";
  try {
    openPlayer(await Service.Lookup(id));
  } catch (reason) {
    searchError.value = errorMessage(reason);
  } finally {
    searching.value = false;
  }
}
async function refreshPlayer(player: Participant) {
  if (searching.value) return;
  const workspace = view.value;
  searching.value = true;
  searchError.value = "";
  selectedError.value = "";
  try {
    const result = await (player.displayName.includes("#")
      ? Service.RefreshLookup(player.displayName)
      : Service.RefreshPlayer(player.key));
    if (workspace === "match" && selectedKey.value === player.key)
      selectedResult.value = result;
    openPlayer(result, false);
  } catch (reason) {
    if (workspace === "match") {
      if (selectedKey.value === player.key) selectedError.value = errorMessage(reason);
    } else searchError.value = errorMessage(reason);
  } finally {
    searching.value = false;
  }
}
function selectPlayer(player: Participant) {
  selectedKey.value = player.key;
  selectedResult.value = null;
  selectedError.value = "";
  void Service.PrioritizePlayer(player.key);
}
async function lookupSelected() {
  if (!selected.value || searching.value) return;
  const target = selected.value;
  searching.value = true;
  selectedError.value = "";
  try {
    const result = await Service.LookupPlayer(target.key);
    if (selectedKey.value === target.key) selectedResult.value = result;
    openPlayer(result, false);
  } catch (reason) {
    if (selectedKey.value === target.key) selectedError.value = errorMessage(reason);
  } finally {
    searching.value = false;
  }
}
async function refreshRoster() {
  if (rosterBusy.value) return;
  rosterBusy.value = true;
  rosterError.value = "";
  try {
    acceptSnapshot(await Service.RefreshRoster());
  } catch (reason) {
    rosterError.value = errorMessage(reason);
  } finally {
    rosterBusy.value = false;
  }
}
async function loadProfile(player:Participant) {
  if(searching.value) return;
  searching.value=true;
  selectedError.value='';searchError.value='';
  const workspace=view.value;
  try {
    const result=await Service.LoadProfile(player.key,player.connectionId);
    openPlayer(result,false);
    if(selectedKey.value===player.key) selectedResult.value=result;
  } catch(reason) {
    if(workspace==='match') {if(selectedKey.value===player.key) selectedError.value=errorMessage(reason);}
    else searchError.value=errorMessage(reason);
  } finally {searching.value=false;}
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
  else searchError.value = errorMessage(initial[0].reason);
  if (initial[1].status === "fulfilled") acceptOverlay(initial[1].value);
  else overlayError.value = errorMessage(initial[1].reason);
});
onUnmounted(() => {
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
  <main v-else class="desktop-app">
    <aside class="app-sidebar">
      <div class="app-brand">
        <span class="brand-mark">P</span>
        <div><strong>LeaguePingo</strong><small>战绩助手</small></div>
      </div>
      <nav aria-label="工作区">
        <button
          :class="{ active: view === 'players' }"
          :aria-current="view === 'players' ? 'page' : undefined"
          @click="view = 'players'"
        >
          ◷ <span>玩家战绩</span></button
        ><button
          :class="{ active: view === 'match' }"
          :aria-current="view === 'match' ? 'page' : undefined"
          @click="view = 'match'"
        >
          ◈ <span>当前对局</span
          ><i v-if="players.length">{{ players.length }}</i>
        </button>
      </nav>
      <div class="sidebar-bottom">
        <button
          class="text-button settings-trigger"
          @click="settingsOpen = true"
        >
          设置与诊断
        </button>
        <span
          class="connection-light"
          :class="{ online: snapshot?.connected }"
        ></span
        >{{ status }}<small>局内仅主动查询战绩</small>
      </div>
    </aside>
    <div class="app-main">
      <header class="app-toolbar">
        <strong>{{ view === "players" ? "玩家战绩" : "当前对局" }}</strong>
        <form class="global-search" @submit.prevent="search">
          <input
            v-model="query"
            aria-label="玩家完整 ID"
            placeholder="搜索 玩家名#标签"
            autocomplete="off"
          /><button class="primary" :disabled="searching">
            {{ searching ? "查询中…" : "查询战绩" }}
          </button>
        </form>
        <button class="icon-button" @click="toggleOverlay">
          显示／隐藏快览</button
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
      <p v-if="searchError" class="error-banner workspace-message" role="alert">
        {{ searchError }}<span v-if="active"> · 已有资料保留</span>
      </p>
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
          :busy="searching"
          :connected="snapshot?.connected"
          @refresh="refreshPlayer"
          @profile="loadProfile"
        />
        <div v-else class="empty-state initial-empty">
          <span class="empty-icon">⌕</span>
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
        <p
          v-if="rosterError || snapshot?.rosterError"
          class="error-banner"
          role="alert"
        >
          {{ rosterError || snapshot?.rosterError }}
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
                {{ team === "ally" ? "我方" : "对方"
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
                <GameIcon :id="player.championId" />
                <span class="roster-identity"
                  ><strong :title="player.displayName">{{
                    player.displayName
                  }}</strong
                  ><small>{{
                    player.identityState === "unavailable"
                      ? "身份不可获取"
                      : player.historyState === "ready"
                        ? `${player.stats.games} 场 · ${percent(player.stats.winRate)} 胜率`
                        : player.historyState === "empty"
                          ? "暂无战绩"
                          : player.historyState === "loading"
                            ? "正在准备"
                            : player.historyState === "failed"
                              ? "准备失败 · 可手动重试"
                              : "尚未准备"
                  }}</small></span
                ><span :title="preparationLabel(player)"
                  >{{
                    player.historyState === "ready"
                      ? recentOutcomes(player)
                      : preparationLabel(player)
                  }}
                  ›</span
                >
              </button>
            </section>
          </div>
          <aside v-if="selected" class="player-drawer">
            <div class="drawer-toolbar">
              <button class="text-button" @click="openPlayer(selected)">
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
              :disabled="searching"
              @click="lookupSelected"
            >
              {{ searching ? "查询中…" : "手动查询此玩家" }}
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
              :busy="searching"
              :connected="snapshot?.connected"
              :error="selectedError || selected.error"
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

<script setup lang="ts">
import { useGuide } from "./useGuide";
import AutoAcceptNotice from "./components/AutoAcceptNotice.vue";
import PostGameNotice from "./components/PostGameNotice.vue";
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { Events } from "@wailsio/runtime";
import * as Service from "../bindings/leaguepingo/appservice";
import type { Snapshot as SnapshotModel } from "../bindings/leaguepingo/internal/core/models";
import type { OverlayState } from "../bindings/leaguepingo/models";
import PlayerView from "./components/PlayerView.vue";
import PlayerTabs from "./components/PlayerTabs.vue";
import TransientNotice from "./components/TransientNotice.vue";
import SettingsPanel from "./components/SettingsPanel.vue";
import AppIcon from "./components/AppIcon.vue";
import BrandIcon from "./components/BrandIcon.vue";
import DesktopTitleBar from "./components/DesktopTitleBar.vue";
import HomeView from "./components/HomeView.vue";
import MatchRoom from "./components/MatchRoom.vue";
import { useTransientMessages } from "./transientMessages";
import { errorMessage, normalizeRiotIDInput } from "./presentation";
import { newestPlayer, operationKey, playerTabId } from "./playerState";
import { useLeagueSnapshot } from "./composables/useLeagueSnapshot";
import { usePlayerWorkspace } from "./composables/usePlayerWorkspace";
import { usePlayerActions } from "./composables/usePlayerActions";
import { useOverlayControls } from "./composables/useOverlayControls";
import { useTheme, type ThemePreference } from "./composables/useTheme";
import { useHomePlayer } from "./composables/useHomePlayer";

const { preference: themePreference, resolved: resolvedTheme, saveError: themeError, setTheme } = useTheme();
const settingsOpen = ref(false);
const mainElement = ref<HTMLElement | null>(null);
const { messages: playerErrors, setMessage } = useTransientMessages();
const searchError = computed(() => playerErrors.search || "");
const league = useLeagueSnapshot();
useGuide(); // Subscribe once at startup; navigating to the guide is a memory-only view.
const { snapshot, status } = league;
const workspace = usePlayerWorkspace(snapshot, (player) => {
  delete playerErrors[operationKey(player)];
});
const {
  view,
  tabs,
  activeKey,
  active,
  stateFor,
  closeTab,
} = workspace;
const reviewModalOpen = ref(false);
const homeModalOpen = ref(false);
const matchModalOpen = computed(() => view.value === "match" && reviewModalOpen.value || view.value === "home" && homeModalOpen.value);
function queryReviewPlayer(player: import("../bindings/leaguepingo/internal/core/models").Participant) {
  query.value = player.displayName;
  view.value = "players";
  submitSearch();
}
function acceptSnapshot(next: SnapshotModel) {
  if (league.acceptSnapshot(next)) workspace.acceptSnapshot(next);
}
const {
  query,
  searching,
  rosterBusy,
  manualBusy,
  refreshing,
  openPlayerDetails,
  search,
  refreshPlayer,
} = usePlayerActions(snapshot, workspace, acceptSnapshot, setMessage);
const home = useHomePlayer(snapshot, view, computed(() => manualBusy.value || rosterBusy.value));
const { player: homePlayer, loading: homeLoading, error: homeError } = home;
const queuedSearch = ref(false);
let queuedConnection = 0;
function submitSearch() {
  if (queuedSearch.value) { queuedSearch.value = false; setMessage("search", ""); return; }
  if (homeLoading.value && !manualBusy.value && /^[^#]+#[^#]+$/.test(normalizeRiotIDInput(query.value))) {
    queuedSearch.value = true;
    queuedConnection = snapshot.value?.connectionId ?? 0;
    setMessage("search", "查询已排队，首页资料准备完成后自动开始；再次点击可取消等待。");
    return;
  }
  void search();
}
watch(query, () => { if (queuedSearch.value) setMessage("search", ""); queuedSearch.value = false; });
watch([homeLoading, manualBusy], ([homeBusy, actionBusy]) => {
  if (!homeBusy && !actionBusy && queuedSearch.value) {
    queuedSearch.value = false;
    if (snapshot.value?.connected && queuedConnection === snapshot.value.connectionId) void search();
  }
});
watch(() => [snapshot.value?.connected, snapshot.value?.connectionId], () => {
  if (!snapshot.value?.connected || queuedConnection !== snapshot.value.connectionId) queuedSearch.value = false;
});
async function refreshHome() {
  if (homePlayer.value) {
    const result = await refreshPlayer(homePlayer.value);
    if (result) home.accept(result);
  } else await home.retry();
}
const {
  overlay,
  overlayError,
  acceptOverlay,
  toggleOverlay,
} = useOverlayControls();
async function showCompanion() {
  if (['Lobby','Matchmaking','ReadyCheck','ChampSelect'].includes(snapshot.value?.phase || '')) {
    if (overlay.value?.sidebarEnabled === false) {setMessage('search','客户端侧栏已关闭，可在游戏辅助设置中启用。');return;}
    try {acceptOverlay(await Service.SetSidebarCollapsed(false));}
    catch (reason) {setMessage('search',errorMessage(reason));}
  } else await toggleOverlay();
}
function onKey(event: KeyboardEvent) {
  if (event.key === "Escape") {
    if (settingsOpen.value) {
      settingsOpen.value = false;
      return;
    }
  }
}
let componentAlive = true;
const pendingPlayers=new Map<string,import('../bindings/leaguepingo/internal/core/models').Participant>();
function readingPlayer(key:string){
  return homeModalOpen.value && key===snapshot.value?.selfKey || view.value==='players' && active.value?.key===key && workspace.stateFor(key).expanded.length>0;
}
function playerUpdate(p:import('../bindings/leaguepingo/internal/core/models').Participant){
  if(!snapshot.value?.connected || p.connectionId!==snapshot.value.connectionId)return;
  if(readingPlayer(p.key)){
    if(!pendingPlayers.has(p.key) && pendingPlayers.size>=64)pendingPlayers.delete(pendingPlayers.keys().next().value!);
    pendingPlayers.set(p.key,newestPlayer(p,pendingPlayers.get(p.key)??null)!);return;
  }
  workspace.acceptRecovery(p);home.accept(p);
}
watch(()=>[homeModalOpen.value,view.value,active.value?.key,active.value?workspace.stateFor(active.value.key).expanded.length:0,snapshot.value?.connected,snapshot.value?.connectionId],()=>{
  for(const [key,p] of pendingPlayers){
    if(!snapshot.value?.connected || p.connectionId!==snapshot.value.connectionId){pendingPlayers.delete(key);continue;}
    if(!readingPlayer(key)){pendingPlayers.delete(key);playerUpdate(p);}
  }
});
let stopPlayer: (()=>void)|undefined;
let stopSnapshot: (() => void) | undefined,
  stopOverlay: (() => void) | undefined;
onMounted(async () => {
  window.addEventListener("keydown", onKey);
  stopPlayer=Events.On('leaguepingo:player-updated',event=>{
    const p=event.data as import('../bindings/leaguepingo/internal/core/models').Participant;
    playerUpdate(p);
  });
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
  if (!componentAlive) return;
  if (initial[0].status === "fulfilled") acceptSnapshot(initial[0].value);
  else setMessage("search", errorMessage(initial[0].reason));
  if (initial[1].status === "fulfilled") acceptOverlay(initial[1].value);
  else overlayError.value = errorMessage(initial[1].reason);
});
onUnmounted(() => {
  componentAlive = false;
  stopSnapshot?.();
  stopPlayer?.();
  stopOverlay?.();
  window.removeEventListener("keydown", onKey);
});
</script>

<template>
  <main class="desktop-app">
    <DesktopTitleBar :modal-open="settingsOpen || matchModalOpen">
        <span class="nav-wordmark window-brand"
          ><BrandIcon />League<span>Pingo</span></span
        >
        <nav class="workspace-nav" aria-label="工作区">
          <button
            :class="{ active: view === 'home' }"
            :aria-current="view === 'home' ? 'page' : undefined"
            aria-label="首页"
            @click="view = 'home'"
          >首页</button>
          <button
            :class="{ active: view === 'match' }"
            :aria-current="view === 'match' ? 'page' : undefined"
            aria-label="对局室"
            @click="view = 'match'"
          >
            对局室</button
          ><button
            :class="{ active: view === 'players' }"
            :aria-current="view === 'players' ? 'page' : undefined"
            aria-label="玩家战绩"
            @click="view = 'players'"
          >
            玩家战绩
          </button>
        </nav>
        <form class="global-search" @submit.prevent="submitSearch()" :aria-busy="searching || queuedSearch" :title="queuedSearch ? '等待首页资料完成后查询；点击取消等待' : manualBusy && !searching ? '正在完成当前操作，请稍候再查询' : '输入玩家名#标签，按 Enter 查询'">
          <AppIcon name="search" :size="16" />
          <input
            v-model="query"
            aria-label="玩家完整 ID"
            placeholder="搜索 玩家名#标签"
            autocomplete="off"
          /><button class="primary" :disabled="manualBusy && !queuedSearch">
            {{ searching ? "查询中…" : queuedSearch ? "取消等待" : manualBusy ? "请稍候…" : "查询战绩" }}
          </button>
        </form>
        <select class="theme-select" aria-label="界面主题" :value="themePreference" @change="setTheme(($event.target as HTMLSelectElement).value as ThemePreference)">
          <option value="light">浅色</option><option value="dark">深色</option><option value="system">跟随系统</option>
        </select>
        <span class="toolbar-status" :title="status"
          ><span
            class="connection-light"
            :class="{ online: snapshot?.connected }"
          ></span
          >{{ status.split(" · ")[0] }}</span
        >
        <button
          class="icon-button settings-trigger"
          aria-label="设置与诊断"
          title="设置与诊断"
          @click="settingsOpen = true"
        >
          <AppIcon name="settings" />
        </button>
        <button
          class="icon-button"
          aria-label="显示／收起游戏助手"
          title="显示／收起游戏助手"
          @click="toggleOverlay"
        >
          <AppIcon name="panel" />
        </button>
    </DesktopTitleBar>
    <div
      ref="mainElement"
      class="app-main"
      :inert="settingsOpen || matchModalOpen"
    >
      <p
        v-if="overlayError || overlay?.warning || overlay?.holdWarning"
        class="error-banner workspace-message"
        role="status"
      >
        {{ overlayError || overlay?.warning || overlay?.holdWarning }}
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
      <AutoAcceptNotice class="workspace-message" />
      <PostGameNotice class="workspace-message" />
      <HomeView
        v-show="view === 'home'"
        :active="view === 'home'"
        :player="homePlayer"
        :snapshot="snapshot"
        :status="status"
        :loading="homeLoading"
        :busy="manualBusy"
        :error="homeError || (homePlayer ? playerErrors[operationKey(homePlayer)] || '' : '')"
        @modal="homeModalOpen = $event" @open="openPlayerDetails"
        @match="view = 'match'"
        @refresh="refreshHome"
        @recovered="home.accept"
      />
      <section v-show="view === 'players'" class="players-workspace">
        <PlayerTabs
          v-if="tabs.length"
          :players="tabs"
          :active-key="activeKey"
          @select="activeKey = $event"
          @close="closeTab"
        />
        <PlayerView
          v-if="active"
          id="player-panel"
          role="tabpanel"
          :aria-labelledby="playerTabId(active.key)"
          tabindex="0"
          :player="active"
          :state="stateFor(active.key)"
          :active="view === 'players'"
          :busy="manualBusy"
          :refreshing="refreshing(active)"
          :connected="snapshot?.connected"
          :automatic-history="snapshot?.stage==='idle'"
          :error="playerErrors[operationKey(active)] || active.error"
          @refresh="refreshPlayer"
          @recovered="workspace.acceptRecovery"
        />
        <div v-else class="empty-state initial-empty">
          <AppIcon name="search" :size="42" />
          <h1>查看玩家近期表现</h1>
          <p>输入完整 ID 查询战绩，或从当前对局中选择玩家。</p>
          <button class="secondary" @click="view = 'match'">
            查看对局室
          </button>
        </div>
      </section>
      <section v-show="view === 'match'" class="match-workspace review-host">
        <MatchRoom :snapshot="snapshot" :active="view === 'match'" @snapshot="acceptSnapshot" @query="queryReviewPlayer" @modal="reviewModalOpen = $event" @companion="showCompanion"/>
      </section>
    </div>
    <SettingsPanel
      :overlay-state="overlay"
      :theme="themePreference"
      :resolved-theme="resolvedTheme"
      :theme-error="themeError"
      @theme="setTheme"
      v-if="settingsOpen"
      @close="settingsOpen = false"
    />
  </main>
</template>

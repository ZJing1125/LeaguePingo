<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { Events } from "@wailsio/runtime";
import * as Service from "../bindings/leaguepingo/appservice";
import type { Snapshot as SnapshotModel } from "../bindings/leaguepingo/internal/core/models";
import type { OverlayState } from "../bindings/leaguepingo/models";
import PlayerView from "./components/PlayerView.vue";
import PlayerTabs from "./components/PlayerTabs.vue";
import OverlayView from "./components/OverlayView.vue";
import MatchRoster from "./components/MatchRoster.vue";
import PlayerDrawer from "./components/PlayerDrawer.vue";
import TransientNotice from "./components/TransientNotice.vue";
import SettingsPanel from "./components/SettingsPanel.vue";
import AppIcon from "./components/AppIcon.vue";
import BrandIcon from "./components/BrandIcon.vue";
import DesktopTitleBar from "./components/DesktopTitleBar.vue";
import HomeView from "./components/HomeView.vue";
import QueueSwitch from "./components/QueueSwitch.vue";
import RosterProgress from "./components/RosterProgress.vue";
import { useTransientMessages } from "./transientMessages";
import { errorMessage, normalizeRiotIDInput } from "./presentation";
import { operationKey, playerTabId } from "./playerState";
import { useLeagueSnapshot } from "./composables/useLeagueSnapshot";
import { usePlayerWorkspace } from "./composables/usePlayerWorkspace";
import { usePlayerActions } from "./composables/usePlayerActions";
import { useOverlayControls } from "./composables/useOverlayControls";
import { useTheme, type ThemePreference } from "./composables/useTheme";
import { useHomePlayer } from "./composables/useHomePlayer";

const isOverlay =
  new URLSearchParams(location.search).get("view") === "overlay";
const { preference: themePreference, resolved: resolvedTheme, saveError: themeError, setTheme } = useTheme();
const settingsOpen = ref(false);
const mainElement = ref<HTMLElement | null>(null);
const { messages: playerErrors, setMessage } = useTransientMessages();
const searchError = computed(() => playerErrors.search || "");
const rosterError = computed(() => playerErrors.roster || "");
const league = useLeagueSnapshot();
const { snapshot, canRefresh, status } = league;
const workspace = usePlayerWorkspace(snapshot, (player) => {
  delete playerErrors[operationKey(player)];
});
const {
  view,
  tabs,
  activeKey,
  selectedKey,
  active,
  selected,
  players,
  stateFor,
  matchStateFor,
  closeTab,
} = workspace;
const matchModalOpen = computed(
  () => view.value === "match" && !!selected.value,
);
const scoutQueueBusy = ref(false);
async function changeScoutQueue(queue: number) {
  if (scoutQueueBusy.value) return;
  scoutQueueBusy.value = true;
  try { acceptSnapshot(await Service.SetScoutQueue(queue)); }
  catch (reason) { setMessage("roster", errorMessage(reason)); }
  finally { scoutQueueBusy.value = false; }
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
  selectPlayer,
  refreshRoster,
} = usePlayerActions(snapshot, workspace, acceptSnapshot, setMessage);
const home = useHomePlayer(snapshot, view, computed(() => manualBusy.value || rosterBusy.value || isOverlay));
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
  overlayBusy,
  overlayError,
  acceptOverlay,
  hideOverlay,
  pinOverlay,
  toggleOverlay,
} = useOverlayControls();
function selectAdjacent(direction: number) {
  const index = players.value.findIndex((p) => p.key === selectedKey.value);
  const next =
    players.value[
      (index + direction + players.value.length) % players.value.length
    ];
  if (next) selectPlayer(next);
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
let componentAlive = true;
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
  if (!componentAlive) return;
  if (initial[0].status === "fulfilled") acceptSnapshot(initial[0].value);
  else setMessage("search", errorMessage(initial[0].reason));
  if (initial[1].status === "fulfilled") acceptOverlay(initial[1].value);
  else overlayError.value = errorMessage(initial[1].reason);
});
onUnmounted(() => {
  componentAlive = false;
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
            aria-label="当前对局"
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
          aria-label="显示／隐藏快览"
          title="显示／隐藏快览"
          @click="toggleOverlay"
        >
          <AppIcon name="panel" /><span>对局快览</span></button
        ><button class="icon-button small-control" @click="hideOverlay">
          隐藏快览
        </button>
    </DesktopTitleBar>
    <div
      ref="mainElement"
      class="app-main"
      :inert="settingsOpen || matchModalOpen"
    >
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
      <HomeView
        v-show="view === 'home'"
        :active="view === 'home'"
        :player="homePlayer"
        :snapshot="snapshot"
        :status="status"
        :loading="homeLoading"
        :busy="manualBusy"
        :error="homeError || (homePlayer ? playerErrors[operationKey(homePlayer)] || '' : '')"
        @open="openPlayerDetails"
        @match="view = 'match'"
        @refresh="refreshHome"
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
          :error="playerErrors[operationKey(active)] || active.error"
          @refresh="refreshPlayer"
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
      <section
        v-show="view === 'match'"
        class="match-workspace"
        :class="{ 'has-inspector': !!selected }"
      >
        <header class="roster-toolbar">
          <div>
            <span class="section-eyebrow">MATCH ROOM / 对局室</span>
            <h1>
              {{ snapshot?.stage === "party" ? "当前房间成员" : "本局名单" }}
            </h1>
            <p class="roster-subtitle muted">{{ status }}</p>
          </div>
          <QueueSwitch :value="snapshot?.scoutQueue ?? 420" :busy="scoutQueueBusy" @change="changeScoutQueue" />
          <span class="hold-hint" :title="overlay?.holdWarning"
            ><kbd>{{
              overlay?.holdKey === "Backquote" ? "~" : overlay?.holdKey || "~"
            }}</kbd>
            {{
              overlay?.holdEnabled && overlay?.holdAvailable
                ? "局内按住显示快览"
                : "按住快览未启用"
            }}<small>仅游戏前台 · 松开隐藏</small></span
          >
          <button
            class="secondary"
            :disabled="rosterBusy || manualBusy || !canRefresh"
            @click="refreshRoster"
          >
            {{ rosterBusy ? "刷新中…" : "刷新对局数据" }}
          </button>
        </header>
        <RosterProgress v-if="players.length" :players="players" :queue="snapshot?.scoutQueue ?? 420" />
        <p
          v-if="snapshot?.connected && snapshot?.message"
          class="roster-message muted"
          role="status"
        >
          {{ snapshot.message }}
        </p>
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
          <MatchRoster
            :queue="snapshot?.scoutQueue ?? 420"
            v-else
            :players="players"
            :selected-key="selectedKey"
            :party="snapshot?.stage === 'party'"
            :frozen="snapshot?.frozen"
            @select="selectPlayer"
          />
          <PlayerDrawer
            v-if="selected"
            :player-key="selected.key"
            modal
            :active="view === 'match'"
            @close="selectedKey = ''"
          >
            <div class="drawer-toolbar">
              <button class="text-button" @click="openPlayerDetails(selected)">
                在战绩页打开 ↗
              </button>
              <div class="drawer-player-switch">
                <button
                  class="text-button"
                  aria-label="上一位玩家"
                  @click="selectAdjacent(-1)"
                >
                  ← 上一位
                </button>
                <button
                  class="text-button"
                  aria-label="下一位玩家"
                  @click="selectAdjacent(1)"
                >
                  下一位 →
                </button>
              </div>
              <button
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
            <p
              v-if="!players.some((p) => p.key === selectedKey)"
              class="quality-warning"
            >
              该玩家不在当前名单，保留上次查看资料。
            </p>
            <PlayerView
              :player="selected"
              :state="matchStateFor(selected.key)"
              :scout-queue="snapshot?.scoutQueue ?? 420"
              :active="view === 'match'"
              :busy="manualBusy"
              :refreshing="refreshing(selected)"
              :connected="snapshot?.connected"
              :error="playerErrors[operationKey(selected)] || selected.error"
              compact
              @refresh="refreshPlayer"
            />
          </PlayerDrawer>
        </div>
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

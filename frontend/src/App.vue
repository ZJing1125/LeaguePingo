<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
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
import DesktopTitleBar from "./components/DesktopTitleBar.vue";
import { useTransientMessages } from "./transientMessages";
import { errorMessage } from "./presentation";
import { operationKey, playerTabId } from "./playerState";
import { useLeagueSnapshot } from "./composables/useLeagueSnapshot";
import { usePlayerWorkspace } from "./composables/usePlayerWorkspace";
import { usePlayerActions } from "./composables/usePlayerActions";
import { useOverlayControls } from "./composables/useOverlayControls";

const isOverlay =
  new URLSearchParams(location.search).get("view") === "overlay";
const settingsOpen = ref(false);
const mainElement = ref<HTMLElement | null>(null);
const mainWidth = ref(window.innerWidth);
// Measure the actual workspace so resizing moves the existing inspector instance.
const drawerModal = computed(() => mainWidth.value < 1000);
let resizeObserver: ResizeObserver | undefined;
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
  () => drawerModal.value && view.value === "match" && !!selected.value,
);
const historyReady = computed(
  () =>
    players.value.filter((player) =>
      ["ready", "empty"].includes(player.historyState),
    ).length,
);
const historyLoading = computed(
  () =>
    players.value.filter((player) => player.historyState === "loading").length,
);
const historyUnknown = computed(
  () =>
    players.value.filter((player) => player.identityState === "unavailable")
      .length,
);
function acceptSnapshot(next: SnapshotModel) {
  if (league.acceptSnapshot(next)) workspace.acceptSnapshot(next);
}
const {
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
  loadRanks,
} = usePlayerActions(snapshot, workspace, acceptSnapshot, setMessage);
const {
  overlay,
  overlayBusy,
  overlayError,
  acceptOverlay,
  hideOverlay,
  pinOverlay,
  toggleOverlay,
} = useOverlayControls();
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
  resizeObserver = new ResizeObserver(([entry]) => {
    if (entry) mainWidth.value = entry.contentRect.width;
  });
  if (mainElement.value) resizeObserver.observe(mainElement.value);
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
  resizeObserver?.disconnect();
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
    <DesktopTitleBar :modal-open="settingsOpen || matchModalOpen" />
    <div
      ref="mainElement"
      class="app-main"
      :inert="settingsOpen || matchModalOpen"
    >
      <header class="app-toolbar">
        <span class="nav-wordmark"
          ><span class="brand-mark">P</span>League<span>Pingo</span></span
        >
        <nav class="workspace-nav" aria-label="工作区">
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
          :rank-loading="
            playerOperation?.key === operationKey(active) &&
            playerOperation?.kind === 'rank'
          "
          :connected="snapshot?.connected"
          :error="playerErrors[operationKey(active)] || active.error"
          @refresh="refreshPlayer"
          @profile="loadProfile"
          @ranks="loadRanks"
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
            :disabled="rosterBusy || !canRefresh"
            @click="refreshRoster"
          >
            {{ rosterBusy ? "刷新中…" : "刷新对局名单" }}
          </button>
        </header>
        <div
          v-if="players.length"
          class="roster-progress"
          aria-label="本局资料准备进度"
          role="status"
        >
          <span class="progress-summary"
            ><b
              >{{ historyReady }}<small>/ {{ players.length }}</small></b
            >
            战绩已准备</span
          >
          <span class="roster-progress-track" aria-hidden="true"
            ><i :style="{ width: `${(historyReady / players.length) * 100}%` }"
          /></span>
          <span v-if="historyLoading">{{ historyLoading }} 人正在获取</span>
          <span v-else-if="historyReady < players.length">{{
            historyUnknown
              ? `${historyUnknown} 人身份未提供`
              : `${players.length - historyReady} 人未就绪`
          }}</span>
          <span v-else>双方资料已就绪</span>
          <span class="roster-known"
            >身份 {{ snapshot?.knownCount ?? 0 }}/{{ players.length }}</span
          >
        </div>
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
            :modal="drawerModal"
            :active="view === 'match'"
            @close="selectedKey = ''"
          >
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
              :state="matchStateFor(selected.key)"
              :active="view === 'match'"
              :busy="manualBusy"
              :refreshing="refreshing(selected)"
              :rank-loading="
                playerOperation?.key === operationKey(selected) &&
                playerOperation?.kind === 'rank'
              "
              :connected="snapshot?.connected"
              :error="playerErrors[operationKey(selected)] || selected.error"
              compact
              @refresh="refreshPlayer"
              @profile="loadProfile"
              @ranks="loadRanks"
            />
          </PlayerDrawer>
        </div>
      </section>
    </div>
    <SettingsPanel
      :overlay-state="overlay"
      v-if="settingsOpen"
      @close="settingsOpen = false"
    />
  </main>
</template>

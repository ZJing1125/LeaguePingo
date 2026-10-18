<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { Events } from "@wailsio/runtime";
import * as Service from "../bindings/leaguepingo/appservice";
import type { Snapshot as SnapshotModel } from "../bindings/leaguepingo/internal/core/models";
import type { OverlayState } from "../bindings/leaguepingo/models";
import PlayerView from "./components/PlayerView.vue";
import PlayerTabs from "./components/PlayerTabs.vue";
import OverlayView from "./components/OverlayView.vue";
import ParticipantAvatar from "./components/ParticipantAvatar.vue";
import TransientNotice from "./components/TransientNotice.vue";
import SettingsPanel from "./components/SettingsPanel.vue";
import AppIcon from "./components/AppIcon.vue";
import DesktopTitleBar from "./components/DesktopTitleBar.vue";
import RecentForm from "./components/RecentForm.vue";
import { useTransientMessages } from "./transientMessages";
import {
  errorMessage,
  percent,
  decimal,
  preparationLabel,
} from "./presentation";
import { operationKey, playerTabId } from "./playerState";
import { useLeagueSnapshot } from "./composables/useLeagueSnapshot";
import { usePlayerWorkspace } from "./composables/usePlayerWorkspace";
import { usePlayerActions } from "./composables/usePlayerActions";
import { useOverlayControls } from "./composables/useOverlayControls";

const isOverlay =
  new URLSearchParams(location.search).get("view") === "overlay";
const settingsOpen = ref(false);
const navCollapsed = ref(true);
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
  closeTab,
} = workspace;
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
  <main v-else class="desktop-app" :class="{ 'nav-collapsed': navCollapsed }">
    <DesktopTitleBar :modal-open="settingsOpen" />
    <aside class="app-sidebar" :inert="settingsOpen">
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
    <div class="app-main" :inert="settingsOpen">
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

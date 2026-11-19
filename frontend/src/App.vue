<script setup lang="ts">
import { uiText } from "./uiText";
import { useGuide } from "./useGuide";
import AutoAcceptNotice from "./components/AutoAcceptNotice.vue";
import PostGameNotice from "./components/PostGameNotice.vue";
import { computed, nextTick, onMounted, onUnmounted, provide, ref, watch } from "vue";
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
import AutomationView from "./components/AutomationView.vue";
import { useAutomation } from "./composables/useAutomation";
import { useTransientMessages } from "./transientMessages";
import { errorMessage, normalizeRiotIDInput } from "./presentation";
import { newestPlayer, operationKey, playerTabId } from "./playerState";
import { useLeagueSnapshot } from "./composables/useLeagueSnapshot";
import { usePlayerWorkspace } from "./composables/usePlayerWorkspace";
import { usePlayerActions } from "./composables/usePlayerActions";
import { useOverlayControls } from "./composables/useOverlayControls";
import { useTheme, type ThemePreference } from "./composables/useTheme";
import { playerHistoryNavigation } from "./playerHistoryNavigation";
import { useHomePlayer } from "./composables/useHomePlayer";

const { preference: themePreference, resolved: resolvedTheme, saveError: themeError, setTheme } = useTheme();
const settingsOpen = ref(false);
const mainElement = ref<HTMLElement | null>(null);
const { messages: playerErrors, setMessage } = useTransientMessages();
const historyJumpError = computed(() => playerErrors.historyJump || "");
const searchError = computed(() => playerErrors.search || "");
const league = useLeagueSnapshot();
useGuide(); // Subscribe once at startup; navigating to the guide is a memory-only view.
const { activeCount: automationCount, warning: automationWarning } = useAutomation();
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
  canOpenHistory,
  openHistory,
  refreshPlayer,
} = usePlayerActions(snapshot, workspace, acceptSnapshot, setMessage);
provide(playerHistoryNavigation, {
  available: canOpenHistory,
  busy: manualBusy,
  error: historyJumpError,
  async open(target) {
    const player = await openHistory(target);
    if (player) {
      await nextTick();
      document.getElementById(playerTabId(player.key))?.focus({ preventScroll: true });
    }
  },
});
const home = useHomePlayer(snapshot, view, computed(() => manualBusy.value || rosterBusy.value));
const { player: homePlayer, loading: homeLoading, error: homeError } = home;
const queuedSearch = ref(false);
let queuedConnection = 0;
function submitSearch() {
  if (queuedSearch.value) { queuedSearch.value = false; setMessage("search", ""); return; }
  if (homeLoading.value && !manualBusy.value && /^[^#]+#[^#]+$/.test(normalizeRiotIDInput(query.value))) {
    queuedSearch.value = true;
    queuedConnection = snapshot.value?.connectionId ?? 0;
    setMessage("search", uiText.app.queuedSearch);
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
    if (overlay.value?.sidebarEnabled === false) {setMessage('search',uiText.app.sidebarDisabled);return;}
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
        <nav class="workspace-nav" :aria-label="uiText.app.workspace">
          <button
            :class="{ active: view === 'home' }"
            :aria-current="view === 'home' ? 'page' : undefined"
            :aria-label="uiText.common.home"
            @click="view = 'home'"
          >{{ uiText.common.home }}</button>
          <button
            :class="{ active: view === 'match' }"
            :aria-current="view === 'match' ? 'page' : undefined"
            :aria-label="uiText.common.matchRoom"
            @click="view = 'match'"
          >{{ uiText.app.matchRoomNav }}</button
          ><button
            :class="{ active: view === 'players' }"
            :aria-current="view === 'players' ? 'page' : undefined"
            :aria-label="uiText.common.playerHistory"
            @click="view = 'players'"
          >{{ uiText.app.playerHistoryNav }}</button
          ><button
            :class="{ active: view === 'automation' }"
            :aria-current="view === 'automation' ? 'page' : undefined"
            :aria-label="uiText.common.automation"
            @click="view = 'automation'"
          >
            <span class="nav-full">{{ uiText.common.automation }}</span><span class="nav-short">{{ uiText.app.automationShort }}</span>
          </button>
        </nav>
        <form class="global-search" @submit.prevent="submitSearch()" :aria-busy="searching || queuedSearch" :title="queuedSearch ? uiText.app.queuedSearchHint : manualBusy && !searching ? uiText.app.busySearchHint : uiText.app.searchHint">
          <AppIcon name="search" :size="16" />
          <input
            v-model="query"
            :aria-label="uiText.common.playerId"
            :placeholder="uiText.app.searchPlaceholder"
            autocomplete="off"
          /><button class="primary" :disabled="manualBusy && !queuedSearch">
            {{ searching ? uiText.common.searching : queuedSearch ? uiText.app.cancelWait : manualBusy ? uiText.app.pleaseWait : uiText.common.search }}
          </button>
        </form>
        <select class="theme-select" :aria-label="uiText.common.theme" :value="themePreference" @change="setTheme(($event.target as HTMLSelectElement).value as ThemePreference)">
          <option value="light">{{ uiText.common.light }}</option><option value="dark">{{ uiText.common.dark }}</option><option value="system">{{ uiText.common.system }}</option>
        </select>
        <span class="toolbar-status" :title="status"
          ><span
            class="connection-light"
            :class="{ online: snapshot?.connected }"
          ></span
          >{{ status.split(" · ")[0] }}</span
        >
        <button
          v-if="automationCount > 0"
          class="automation-chip"
          :class="{ warn: automationWarning }"
          :aria-label="uiText.app.automationEnabledHint(automationCount)"
          :title="automationWarning ? uiText.app.automationWarning : uiText.app.automationEnabled"
          @click="view = 'automation'"
        >
          <span class="dot" />{{ uiText.app.automationCount(automationCount) }}</button>
        <button
          class="icon-button settings-trigger"
          :aria-label="uiText.common.settings"
          :title="uiText.common.settings"
          @click="settingsOpen = true"
        >
          <AppIcon name="settings" />
        </button>
        <button
          class="icon-button"
          :aria-label="uiText.common.toggleAssistant"
          :title="uiText.common.toggleAssistant"
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
        :message="searchError || historyJumpError"
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
          :self="active.key === snapshot?.selfKey && active.connectionId === snapshot?.connectionId"
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
          <h1>{{ uiText.app.playerOverview }}</h1>
          <p>{{ uiText.app.emptyWorkspace }}</p>
          <button class="secondary" @click="view = 'match'">{{ uiText.app.openMatchRoom }}</button>
        </div>
      </section>
      <section v-show="view === 'match'" class="match-workspace review-host">
        <MatchRoom :snapshot="snapshot" :active="view === 'match'" @snapshot="acceptSnapshot" @modal="reviewModalOpen = $event" @companion="showCompanion"/>
      </section>
      <section v-if="view === 'automation'" class="automation-host">
        <AutomationView @settings="settingsOpen = true" />
      </section>
    </div>
    <SettingsPanel
      :overlay-state="overlay"
      :theme="themePreference"
      :resolved-theme="resolvedTheme"
      :theme-error="themeError"
      @theme="setTheme"
      @automation="settingsOpen = false; view = 'automation'"
      v-if="settingsOpen"
      @close="settingsOpen = false"
    />
  </main>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.workspace-message):is(.workspace-message) {
  margin: 8px 20px 0;
}
:where(.window-brand):is(.window-brand) {
  display: flex;
  align-items: center;
  gap: 9px;
  padding-left: 16px;
  min-width: 0;
}
:where(.window-brand):is(.window-inactive .window-brand) {
  opacity: 0.6;
}
:where(.settings-trigger):is(.settings-trigger) {
  justify-content: flex-start;
  padding: 7px 0;
}
:where(.toolbar-status):is(.toolbar-status) {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 11px;
  color: var(--muted);
  white-space: nowrap;
}
:where(.app-main):is(.app-main) {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  position: relative;
}
:where(.global-search):is(.global-search) {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
  width: min(450px, 55%);
  padding: 3px 4px 3px 12px;
  border: 1px solid var(--border);
  background: #171b24;
  border-radius: 7px;
}
:where(.global-search):is(.global-search) :deep(input) {
  background: none;
  border: 0;
  padding: 4px 0;
  min-width: 80px;
  width: 100%;
  font-size: 12px;
}
:where(.global-search):is(.global-search) :deep(button) {
  padding: 4px 10px;
}
:where(.players-workspace):is(.players-workspace) {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
}
:where(.initial-empty):is(.initial-empty) {
  flex: 1;
}
:where(.match-workspace):is(.match-workspace) {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 18px 20px 0;
}
:where(.match-workspace):is(.match-workspace) :deep(>.error-banner) {
  margin-bottom: 8px;
}
@media (max-width: 1000px) {
  :where(.settings-trigger):is(.settings-trigger) {
    justify-content: center;
  }
}
@media (max-width: 760px) {
  :where(.global-search):is(.global-search) {
    width: auto;
    flex: 1;
  }
}
@media (max-width: 760px) {
  :where(.toolbar-status):is(.toolbar-status) {
    display: none;
  }
}
@media (max-width: 760px) {
  :where(.match-workspace):is(.match-workspace) {
    padding: 15px 12px 0;
  }
}
:where(.theme-select):is(.theme-select) {
  flex: none;
  font-size: 12px;
  max-width: 102px;
  padding: 6px 5px;
  cursor: pointer;
}
@media (max-width: 1150px) {
  :where(.workspace-nav):is(.workspace-nav) { gap: 14px; }
}
@media (max-width: 1150px) {
  :where(.global-search):is(.global-search) { flex: 1; min-width: 200px; }
}
@media (max-width: 999px) {
  :where(.nav-wordmark):is(.app-toolbar .nav-wordmark) { font-size: 13px; }
}
@media (max-width: 999px) {
  :where(.workspace-nav):is(.app-toolbar .workspace-nav) { gap: 10px; }
}
@media (max-width: 999px) {
  :where(.global-search):is(.app-toolbar .global-search) { min-width: 180px; }
}
@media (max-width: 999px) {
  :where(.theme-select):is(.theme-select) { max-width: 88px; font-size: 11px; }
}
:where(.match-workspace):is(.desktop-app .match-workspace) :deep(.roster-toolbar) { height: 46px; padding-block: 3px; }
:where(.match-workspace):is(.match-workspace) :deep(.roster-toolbar .section-eyebrow) { display: none; }
:where(.match-workspace):is(.match-workspace) :deep(.roster-toolbar h1) { font-size: 23px; line-height: 26px; margin: 0 0 2px; }
:where(.match-workspace):is(.desktop-app .match-workspace) :deep(.roster-progress) { height: 22px; min-height: 22px; margin-bottom: 2px; padding-block: 2px; }
:where(.window-brand):is(.window-brand) {
  gap: 0;
  padding-left: 0;
}
:where(.window-brand):is(.window-inactive .window-brand) {
  opacity: 0.65;
}
:where(.app-main):is(.app-main) {
  width: 100%;
  overflow: hidden;
  background:
    radial-gradient(ellipse at 55% 0, var(--page-glow), transparent 65%), var(--surface);
}
:where(.nav-wordmark):is(.nav-wordmark) {
  display: flex;
  align-items: center;
  font-size: 18px;
  letter-spacing: -0.6px;
  font-weight: 750;
  white-space: nowrap;
}
:where(.nav-wordmark):is(.nav-wordmark) :deep(>span:last-child) {
  color: var(--accent-art);
}
:where(.workspace-nav):is(.workspace-nav) {
  display: flex;
  height: 100%;
  gap: 22px;
  flex: none;
  margin-left: 14px;
}
:where(.workspace-nav):is(.workspace-nav) :deep(button) {
  position: relative;
  padding: 0 2px;
  font-weight: 600;
  font-size: 12px;
  color: var(--muted);
  background: none;
  border: 0;
}
:where(.workspace-nav):is(.workspace-nav) :deep(button.active) {
  color: var(--text);
}
:where(.workspace-nav):is(.workspace-nav) :deep(button.active:after) {
  content: "";
  height: 3px;
  background: var(--accent-art);
  position: absolute;
  bottom: 0;
  left: 0;  right: 0;
}
:where(.nav-short):is(.workspace-nav .nav-short) { display: none; }
:where(.automation-chip):is(.automation-chip) {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex: none;
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 4px 10px;
  font-size: 12px;
  color: var(--text);
  background: none;
}
:where(.automation-chip):is(.automation-chip) :deep(.dot) { width: 7px; height: 7px; border-radius: 50%; background: var(--accent); }
:where(.automation-chip.warn):is(.automation-chip.warn) { color: var(--danger, #e06c6c); border-color: currentColor; }
:where(.automation-chip.warn):is(.automation-chip.warn) :deep(.dot) { background: currentColor; }
:where(.automation-host):is(.automation-host) { display: flex; flex-direction: column; min-height: 0; flex: 1; }
:where(.global-search):is(.global-search) {
  flex: 1;
  width: auto;
  min-width: 180px;
  max-width: 360px;
  background: var(--search-bg);
  border: 0;
  border-radius: 3px;
  margin-left: auto;
}
:where(.global-search):is(.global-search) :deep(button) {
  font-size: 11px;
}
:where(.toolbar-status):is(.toolbar-status) {
  font-size: 11px;
  white-space: nowrap;
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
}
:where(.match-workspace):is(.match-workspace) {
  padding: 0 28px 14px;
  gap: 0;
  overflow: hidden;
}
:where(.match-workspace):is(.match-workspace) :deep(>.roster-message) {
  font-size: 10px;
  margin: 0 0 4px;
  line-height: 16px;
  flex: none;
}
:where(.players-workspace):is(.players-workspace) {
  overflow: hidden;
}
@media (max-width: 1150px) {
  :where(.workspace-nav):is(.workspace-nav) {
    margin-left: 2px;
    gap: 15px;
  }

}
@media (max-width: 1150px) {
  :where(.nav-wordmark):is(.nav-wordmark) {
    font-size: 16px;
  }

}
@media (max-width: 1150px) {
  :where(.global-search):is(.global-search) {
    width: auto;
  }

}
@media (max-width: 1150px) {
  :where(.toolbar-status):is(.toolbar-status) {
    display: none;
  }

}
@media (max-width: 1150px) {
  :where(.match-workspace):is(.match-workspace) {
    padding-inline: 20px;
  }

}
@media (max-width: 999px) {
  :where(.nav-wordmark):is(.nav-wordmark) {
    font-size: 14px;
  }
}
@media (max-width: 999px) {
  :where(.workspace-nav):is(.workspace-nav) {
    gap: 13px;
  }
}
@media (max-width: 999px) {
  :where(.workspace-nav):is(.workspace-nav) :deep(button) {
    font-size: 11px;
  }
}
@media (max-width: 999px) {
  :where(.global-search):is(.global-search) {
    width: auto;
    gap: 5px;
  }
}
@media (max-width: 999px) {
  :where(.global-search):is(.global-search) :deep(button) {
    padding: 4px 6px;
    font-size: 11px;
  }
}
@media (max-width: 999px) {
  :where(.match-workspace):is(.match-workspace) {
    padding-inline: 18px;
  }
}
:where(.players-workspace):is(.players-workspace) :deep(.rank-query) {
  color: #f4e5c8;
  border-color: #a1ad8e80;
  background: #243b34;
}
:where(.match-workspace):is(.match-workspace) :deep(.card-intent) {
  top: 58px;
  right: 10px;
  font-size: 10px;
}
:where(.match-workspace):is(.match-workspace) :deep(.current-hero) {
  min-height: 25px;
  gap: 5px;
  font-size: 12px;
  padding: 4px 0;
}
:where(.match-workspace):is(.match-workspace) :deep(.current-hero>span) {
  font-size: 12px;
}
:where(.match-workspace):is(.match-workspace) :deep(.current-hero>b) {
  font-size: 12px;
}
:where(.match-workspace):is(.match-workspace) :deep(.current-hero>small) {
  display: none;
}
:where(.match-workspace):is(.match-workspace) :deep(.block-caption) {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
}
:where(.match-workspace):is(.match-workspace) :deep(.block-caption small) {
  font-size: 10px;
}
:where(.match-workspace):is(.match-workspace) :deep(.recent-games) {
  gap: 5px;
}
:where(.match-workspace):is(.match-workspace) :deep(.recent-game>small) {
  font-size: 10px;
}
:where(.match-workspace):is(.match-workspace) :deep(.main-heroes-row) {
  display: flex;
  min-height: 24px;
  gap: 6px;
}
:where(.match-workspace):is(.match-workspace) :deep(.main-heroes-row>span:first-child) {
  font-size: 11px;
}
:where(.match-workspace):is(.match-workspace) :deep(.main-hero small) {
  font-size: 11px;
}
:where(.match-workspace):is(.match-workspace) :deep(.main-heroes) {
  gap: 7px;
}
:where(.match-workspace):is(.match-workspace) :deep(.card-freshness) {
  font-size: 11px;
}
@media (max-width: 1150px) { :where(.nav-full):is(.workspace-nav .nav-full) { display: none; } }
@media (max-width: 1150px) { :where(.nav-short):is(.workspace-nav .nav-short) { display: inline; } }
:where(.match-workspace.review-host):is(.match-workspace.review-host) {padding:0;overflow:hidden;}
</style>

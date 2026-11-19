<script setup lang="ts">
import { uiText } from "../uiText";
import GuidePanel from "./GuidePanel.vue";
import { useGuide } from "../useGuide";
import { computed, ref, watch } from "vue";
import type { Snapshot } from "../../bindings/leaguepingo/internal/core/models";
import type { OverlayState } from "../../bindings/leaguepingo/models";
import { date } from "../presentation";
import AppIcon from "./AppIcon.vue";
import BrandIcon from "./BrandIcon.vue";
import PlayerScoutCard from "./PlayerScoutCard.vue";
import { rankedMode, liveScoutMode } from "../scoutStats";
import { rankedPlayers } from "../playerAnalysis";
import { scoutStatus } from "../scoutStatus";
import RosterProgress from "./RosterProgress.vue";
import ExperimentalBadge from "./ExperimentalBadge.vue";
const props = defineProps<{
  snapshot: Snapshot | null;
  state: OverlayState | null;
  status: string;
  busy: boolean;
  error: string;
}>();
const guideState=useGuide(),guideOpen=ref(false);
watch(()=>`${props.snapshot?.connectionId}:${props.snapshot?.selfKey}:${props.snapshot?.gameId}`,()=>guideOpen.value=false);
const hasGuide=computed(()=>guideState.value?.query.champion===props.snapshot?.participants?.find(p=>p.key===props.snapshot?.selfKey)?.championId&&guideState.value?.scope===`${props.snapshot?.connectionId}:${props.snapshot?.selfKey||''}:${props.snapshot?.gameId||0}`&&!!(guideState.value?.base||guideState.value?.matchup));
const displayGuide=computed(()=>guideOpen.value&&hasGuide.value&&!['hold','preview'].includes(props.state?.mode||''));
const players = computed(() => props.snapshot?.participants ?? []);
const teamPlayers = (team: string) => rankedPlayers(players.value.filter(p => p.team === team));
const modeLabel = computed(() => {
  const actual = liveScoutMode(props.snapshot);
  const view = props.snapshot?.scoutQueue ?? 0;
  return view && view !== actual
    ? `${uiText.modeScouting.current(rankedMode(actual))} · ${uiText.modeScouting.viewing(rankedMode(view))}`
    : rankedMode(actual);
});
const reports = computed(() => scoutStatus(players.value, props.snapshot?.scoutQueue ?? 0));
const reportTime = computed(() => {
  const p = reports.value;
  if (!p.earliest) return uiText.overlayView.unknownTime;
  const first = date(p.earliest), last = date(p.latest);
  return uiText.overlayView.cutoff(first, last !== first ? `–${last}` : '', p.untimed ? uiText.overlayView.partial : '');
});
defineEmits<{ hide: []; pin: [] }>();
</script>
<template>
  <main class="overlay-shell" :class="{ 'hold-mode': ['hold','preview'].includes(state?.mode || ''), 'preview-mode':state?.mode==='preview' }">
    <header class="overlay-head">
      <div class="overlay-title">
        <BrandIcon />
        <div><h1>LeaguePingo</h1><small>{{ uiText.overlayView.title }}</small></div>
      </div>
      <div v-if="!['hold','preview'].includes(state?.mode || '')" class="overlay-controls">
        <button class="guide-switch" v-if="hasGuide" :aria-pressed="displayGuide" @click="guideOpen=!guideOpen">{{displayGuide?uiText.overlayView.players:uiText.clientSidebar.guide}}</button>
        <button
          :disabled="busy || !state"
          :aria-label="state?.pinned ? uiText.overlayView.unpin : uiText.overlayView.pin"
          :title="state?.pinned ? uiText.overlayView.unpin : uiText.overlayView.pin"
          :aria-pressed="state?.pinned"
          @click="$emit('pin')"
        >
          <AppIcon name="pin" :size="16" /></button
        ><button
          class="overlay-close"
          :aria-label="uiText.overlayView.close"
          :disabled="busy"
          @click="$emit('hide')"
        >
          <AppIcon name="close" :size="18" />
        </button>
      </div>
    </header>
    <div class="overlay-content">
      <div class="overlay-status">
        <span
          class="connection-light"
          :class="{ online: snapshot?.connected }"
        ></span
        >{{ status }} · {{ modeLabel }}
        <ExperimentalBadge v-if="players.some(p => p.premadeGroup && (p.premadeSize || 0) > 1)" :feature="uiText.clientSidebar.groups" :label="uiText.clientSidebar.groupExperimental"/>
      </div>
      <p v-if="state?.warning || error" class="error-banner" role="status">
        {{ error || state?.warning }}
      </p>
      <GuidePanel v-if="displayGuide" :snapshot="snapshot" summary/>
      <RosterProgress v-if="players.length && !displayGuide" :players="players" :queue="snapshot?.scoutQueue ?? 0" compact />
      <div v-if="!players.length" class="empty-state">
        <AppIcon name="match" :size="30" />
        <p>{{ uiText.overlayView.waitingRoster }}</p>
      </div>
      <div v-else-if="!displayGuide" class="overlay-groups">
        <section v-for="team in ['ally', 'enemy']" :key="team">
          <h2>
            {{
              snapshot?.stage === "party"
                ? team === "ally"
                  ? uiText.common.party
                  : uiText.matchRoster.noOpponent
                : team === "ally"
                  ? uiText.common.ally
                  : uiText.common.enemy
            }}<span>{{ players.filter((p) => p.team === team).length }}</span>
          </h2>
          <div
            v-for="p in teamPlayers(team)"
            :key="p.key"
            class="overlay-player scout-card"
            :class="{
              'scout-enemy': team === 'enemy',
              unavailable: p.identityState === 'unavailable',
              outdated: p.stale,
            }"
          >
            <PlayerScoutCard :player="p" :frozen="snapshot?.frozen" :party="snapshot?.stage === 'party'" :queue="snapshot?.scoutQueue ?? 0" />
          </div>
        </section>
      </div>
    </div>
    <footer class="overlay-footer">
      <span v-if="state?.mode==='preview'">{{ uiText.overlayView.autoHide(snapshot?.phase==='GameStart' ? uiText.overlayView.loadingPreview : uiText.overlayView.startPreview, state?.holdEnabled && state?.holdAvailable ? uiText.overlayView.hold(state?.holdKey==='Backquote'?'~':state?.holdKey || '~') : state?.shortcutAvailable ? uiText.overlayView.shortcut(state.shortcut) : uiText.overlayView.manualPreview) }}</span>
      <span v-else-if="state?.mode === 'hold'"
        >{{ uiText.overlayView.holdHelp(state?.holdKey === "Backquote" ? "~" : state?.holdKey) }}</span
      ><span v-else
        >{{ uiText.overlayView.manualHelp(state?.shortcut || "Ctrl+Shift+P", state?.shortcutAvailable === false ? uiText.overlayView.unregistered : uiText.overlayView.visibility) }}</span
      ><span :title="uiText.overlayView.cutoffHint"
        >{{ reportTime }}</span
      >
    </footer>
    <div v-if="state?.mode==='preview'" class="preview-timer" aria-hidden="true"><i/></div>
  </main>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.overlay-shell):is(.overlay-shell) {
  height: 100dvh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: #121820;
  border: 1px solid #3a514b;
  border-radius: 10px;
  padding: 15px 16px;
}
:where(.overlay-head):is(.overlay-head) {
  --wails-draggable: drag;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  cursor: move;
  user-select: none;
  flex: none;
  border-bottom: 1px solid var(--border);
  padding-bottom: 12px;
}
:where(.overlay-title):is(.overlay-title) {
  display: flex;
  align-items: center;
  gap: 10px;
}
:where(.overlay-controls):is(.overlay-controls) {
  display: flex;
  align-items: center;
  gap: 10px;
}
:where(.overlay-title):is(.overlay-title) :deep(h1) {
  display: inline;
  margin: 0;
  font-size: 13px;
  font-weight: 700;
}
:where(.overlay-title):is(.overlay-title) :deep(small) {
  display: block;
  font-size: 11px;
  color: var(--muted);
  margin-top: 3px;
}
:where(.overlay-controls):is(.overlay-controls) {
  --wails-draggable: no-drag;
}
:where(.overlay-controls):is(.overlay-controls) :deep(button) {
  --wails-draggable: no-drag;
}
:where(.overlay-controls):is(.overlay-controls) :deep(button) {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  padding: 5px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--muted);
}
:where(.overlay-controls):is(.overlay-controls) :deep(button[aria-pressed="true"]) {
  color: var(--accent);
  border-color: #42695b;
}
:where(.overlay-content):is(.overlay-content) {
  --wails-draggable: no-drag;
  flex: 1;
  min-height: 0;
  overflow: auto;
}
:where(.overlay-content):is(.overlay-content) :deep(*) {
  --wails-draggable: no-drag;
}
:where(.overlay-status):is(.overlay-status) {
  display: flex;
  align-items: center;
  gap: 7px;
  margin: 13px 0;
  color: #c6d0dc;
  font-size: 12px;
}
:where(.overlay-groups):is(.overlay-groups) {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  margin-top: 13px;
}
:where(.overlay-groups):is(.overlay-groups) :deep(h2) {
  display: flex;
  justify-content: space-between;
  color: #aad7c7;
  font-size: 12px;
  margin: 0 2px 10px;
}
:where(.overlay-groups):is(.overlay-groups) :deep(section:nth-child(2) h2) {
  color: #d4b2c1;
}
:where(.overlay-groups):is(.overlay-groups) :deep(h2 span) {
  color: var(--muted);
  font-weight: 400;
}
:where(.overlay-player):is(.overlay-player) {
  background: #1a222d;
  border: 1px solid #2c3845;
  border-radius: 7px;
  padding: 9px;
  margin-bottom: 7px;
  min-width: 0;
  min-height: 76px;
}
:where(.overlay-player.outdated):is(.overlay-player.outdated) {
  border-color: #665232;
}
:where(.overlay-player):is(.overlay-player) :deep(>.warning-text) {
  display: block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11px;
  margin-top: 5px;
}
:where(.overlay-footer):is(.overlay-footer) {
  flex: none;
  display: flex;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  border-top: 1px solid var(--border);
  margin-top: 10px;
  padding-top: 11px;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.6;
}
@media (max-width: 440px) {
  :where(.overlay-groups):is(.overlay-groups) {
    gap: 8px;
  }
}
@media (max-width: 440px) {
  :where(.overlay-shell):is(.overlay-shell) {
    padding: 10px;
  }
}
@media (max-width: 440px) {
  :where(.overlay-footer):is(.overlay-footer) {
    font-size: 10px;
  }
}
@media (max-width: 440px) {
  :where(.overlay-player):is(.overlay-player) {
    padding: 7px;
  }
}
:where(.overlay-shell):is(.overlay-shell) {
  --scout-card: #344052;
  --scout-head: #2b3545;
  --scout-cell: #374456;
  --scout-inset: #2d394a;
  --scout-border: #18223380;
  --scout-text: #f2f5fa;
  --scout-muted: #b3c0d1;
  --scout-good: #4cdea8;
  --scout-bad: #ff8b9a;
  --scout-warning: #ecc17b;
  --scout-track: #223043;
  --scout-accent: #bdacff;
  --scout-ally: #50c6e3;
  --scout-enemy: #ef8b9b;
}
:where(.overlay-player.scout-card):is(.overlay-shell .overlay-player.scout-card) {
  --scout-team: var(--scout-ally);
  position: relative;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  gap: 0;
  padding: 0;
  color: var(--scout-text);
  background: var(--scout-card);
  border: 1px solid var(--scout-border);
  border-radius: 5px;
  clip-path: none;
  overflow: hidden;
  box-shadow: 0 2px 5px #11182725;
  min-width: 0;
  min-height: 0;
  height: 300px;
}
:where(.overlay-player.scout-enemy):is(.overlay-shell .overlay-player.scout-enemy) { --scout-team: var(--scout-enemy); }
:where(.overlay-player.scout-card):is(.overlay-shell .overlay-player.scout-card)::before { content: none; display: none; }
:where(.overlay-groups):is(.overlay-shell .overlay-groups) { gap: 8px; margin-top: 0; }
:where(main.overlay-shell):is(main.overlay-shell) { padding-block: 8px; }
:where(.overlay-head):is(.overlay-shell .overlay-head) { min-height: 34px; padding-bottom: 2px; }
:where(.overlay-title):is(.overlay-shell .overlay-title) :deep(h1) { font-size: 16px; line-height: 18px; }
:where(.overlay-title):is(.overlay-shell .overlay-title) :deep(small) { line-height: 11px; margin-top: 2px; }
:where(.overlay-shell):is(.overlay-shell) :deep(.overlay-readiness) { padding-block: 2px; font-size: 11px; line-height: 12px; }
:where(.overlay-content):is(.overlay-shell .overlay-content) { gap: 2px; }
:where(.overlay-footer):is(.overlay-shell .overlay-footer) { min-height: 16px; padding-top: 2px; line-height: 14px; margin-top: 0; }
:where(.overlay-groups):is(.overlay-shell .overlay-groups) :deep(>section) { gap: 6px; }
:where(.overlay-player.scout-card):is(.overlay-shell .overlay-player.scout-card) { height: 236px; min-height: 236px; width: calc((100% - 24px) / 5); margin: 0; }
@media (max-width: 680px) {
  :where(.overlay-groups):is(.overlay-shell .overlay-groups) { display: flex; flex-direction: column; }
}
@media (max-width: 680px) {
  :where(.overlay-groups):is(.overlay-shell .overlay-groups) :deep(>section) { flex-direction: row; flex-wrap: wrap; }
}
@media (max-width: 680px) {
  :where(.overlay-groups):is(.overlay-shell .overlay-groups) :deep(h2) { flex-basis: 100%; }
}
@media (max-width: 680px) {
  :where(.overlay-player.scout-card):is(.overlay-shell .overlay-player.scout-card) { width: calc((100% - 6px) / 2); height: auto; min-height: 250px; }
}
@media (max-width: 680px) {
  :where(.overlay-head):is(.overlay-shell .overlay-head) { min-height: 34px; }
}
:where(.overlay-status):is(.overlay-shell .overlay-status) { font-size: 11px; }
:where(.overlay-title):is(.overlay-shell .overlay-title) :deep(small) { font-size: 11px; }
:where(.overlay-footer):is(.overlay-shell .overlay-footer) { font-size: 11px; }
:where(.overlay-shell):is(.overlay-shell) :deep(button) { font-family: inherit; transition: border-color .12s, box-shadow .12s; }
:where(.overlay-controls):is(.overlay-controls) :deep(button) { border-radius: var(--control-radius); }
@media (prefers-reduced-motion: reduce) { :where(.overlay-shell):is(.overlay-shell) :deep(button) { transition: none; } }
:where(.overlay-player):is(.overlay-player):before {
  position: absolute;
  content: "";
  inset: 0;
  z-index: 1;
  pointer-events: none;
  background:
    linear-gradient(90deg, #10212bef 0%, #10212bcc 44%, #10212b20 100%),
    linear-gradient(0deg, #10212bf2 0%, #10212b88 30%, #10212b0a 100%);
}
:where(.overlay-player):is(.overlay-player) :deep(>.card-illustration) {
  inset: 8px;
  width: calc(100% - 16px);
  height: calc(100% - 16px);
  object-fit: contain;
  object-position: right center;
  z-index: 0;
}
:where(.overlay-shell):is(.overlay-shell) {
  color-scheme: dark;
  color: #e5eadb;
  background: #141c20;
  --text: #e5eadb;
  --muted: #a0b39f;
  --border: #ffffff20;
  --surface: #141c20;
  --raised: #253531;
  --accent: #ec875e;
  --win: #99c9b0;
  --loss: #d3938c;
  padding: 14px 22px;
  gap: 0;
}
:where(.overlay-head):is(.overlay-head) {
  background: none;
  border: 0;
  padding: 0 0 10px;
  min-height: 42px;
}
:where(.overlay-title):is(.overlay-title) :deep(h1) {
  font-size: 17px;
}
:where(.overlay-title):is(.overlay-title) :deep(small) {
  font-size: 9px;
  color: #93a68e;
}
:where(.overlay-controls):is(.overlay-controls) :deep(button) {
  background: #25322e;
  color: #cbd7c4;
  border-color: #ffffff15;
}
:where(.overlay-content):is(.overlay-content) {
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-height: 0;
  overflow: auto;
}
:where(.overlay-status):is(.overlay-status) {
  font-size: 10px;
  margin: 0;
}
:where(.overlay-groups):is(.overlay-groups) {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 8px;
  min-height: 0;
  flex: 1;
}
:where(.overlay-groups):is(.overlay-groups) :deep(>section) {
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  align-content: start;
  gap: 8px;
}
:where(.overlay-groups):is(.overlay-groups) :deep(h2) {
  width: 100%;
  font-size: 10px;
  letter-spacing: 2px;
  flex: none;
  margin: 0;
  height: 15px;
  color: #9abdb6;
}
:where(.overlay-groups):is(.overlay-groups) :deep(section:nth-child(2) h2) {
  color: #d4a19b;
}
:where(.overlay-groups):is(.overlay-groups) :deep(h2 span) {
  font-size: 9px;
  margin-left: 10px;
}
:where(.overlay-player):is(.overlay-player) {
  position: relative;
  isolation: isolate;
  display: flex;
  flex-direction: column;
  padding: 0;
  gap: 0;
  border: 1px solid #ffffff15;
  min-width: 0;
  width: calc((100% - 32px) / 5);
  min-height: 182px;
  margin: 0;
  border-radius: 0;
  background: #1b2b2e;
  overflow: hidden;
}
:where(.overlay-player.outdated):is(.overlay-player.outdated) {
  border-color: #a48c5f;
}
:where(.overlay-player):is(.overlay-player) :deep(.main-heroes-row) {
  display: none;
}
:where(.overlay-player):is(.overlay-player) :deep(.current-hero) {
  min-height: 19px;
  padding: 3px 0;
}
:where(.overlay-footer):is(.overlay-footer) {
  padding: 9px 0 0;
  border: 0;
  background: none;
  font-size: 9px;
  min-height: 22px;
  flex: none;
  color: #94a48d;
}
:where(.overlay-shell.hold-mode):is(.overlay-shell.hold-mode) {
  user-select: none;
  pointer-events: none;
}
:where(.overlay-head):is(.overlay-shell.hold-mode .overlay-head) {
  --wails-draggable: no-drag;
}
:where(.overlay-footer):is(.overlay-shell.hold-mode .overlay-footer) {
  color: #e0b49b;
}
@media (max-width: 999px) {

  :where(.overlay-shell):is(.overlay-shell) {
    padding: 12px 15px;
  }
}
@media (max-width: 999px) {
  :where(.overlay-player):is(.overlay-player) :deep(.block-caption small) {
    display: none;
  }
}
@media (max-width: 680px) {
  :where(.overlay-groups):is(.overlay-groups) {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }
}
@media (max-width: 680px) {
  :where(.overlay-groups):is(.overlay-groups) :deep(>section) {
    flex-direction: column;
    flex-wrap: nowrap;
    gap: 5px;
  }
}
@media (max-width: 680px) {
  :where(.overlay-player):is(.overlay-player) {
    width: 100%;
    min-height: 94px;
    flex: none;
  }
}
@media (max-width: 680px) {
  :where(.overlay-player):is(.overlay-player) :deep(.current-hero) {
    display: none;
  }
}
@media (max-width: 680px) {
  :where(.overlay-footer):is(.overlay-footer) :deep(>span:last-child) {
    display: none;
  }
}
@media (max-width: 680px) {
  :where(.overlay-title):is(.overlay-title) :deep(h1) {
    font-size: 15px;
  }
}
:where(.overlay-shell):is(.overlay-shell) {position:relative;}
:where(.preview-timer):is(.preview-timer) {position:absolute;bottom:0;left:0;right:0;height:2px;background:var(--border);}
:where(.preview-timer):is(.preview-timer) :deep(i) {display:block;height:100%;background:var(--ally);transform-origin:left;animation:preview-countdown 6s linear forwards;}
@keyframes preview-countdown {to {transform:scaleX(0)}}
@media(prefers-reduced-motion:reduce){:where(.preview-timer):is(.preview-timer) :deep(i){animation:none;}}
</style>
<style scoped>.overlay-controls .guide-switch{width:auto;min-width:76px;font-size:12px;white-space:nowrap;}</style>

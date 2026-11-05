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
import { rankedMode } from "../scoutStats";
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
const reports = computed(() => scoutStatus(players.value, props.snapshot?.scoutQueue ?? 420));
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
        <div><strong>LeaguePingo</strong><small>{{ uiText.overlayView.title }}</small></div>
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
        >{{ status }} · {{ rankedMode(snapshot?.scoutQueue ?? 420) }}
        <ExperimentalBadge v-if="players.some(p => p.premadeGroup && (p.premadeSize || 0) > 1)" :feature="uiText.clientSidebar.groups" :label="uiText.clientSidebar.groupExperimental"/>
      </div>
      <p v-if="state?.warning || error" class="error-banner" role="status">
        {{ error || state?.warning }}
      </p>
      <GuidePanel v-if="displayGuide" :snapshot="snapshot" summary/>
      <RosterProgress v-if="players.length && !displayGuide" :players="players" :queue="snapshot?.scoutQueue ?? 420" compact />
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
            v-for="p in players.filter((p) => p.team === team)"
            :key="p.key"
            class="overlay-player scout-card"
            :class="{
              'scout-enemy': team === 'enemy',
              unavailable: p.identityState === 'unavailable',
              outdated: p.stale,
            }"
          >
            <PlayerScoutCard :player="p" :frozen="snapshot?.frozen" :party="snapshot?.stage === 'party'" :queue="snapshot?.scoutQueue ?? 420" />
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

<style scoped>.overlay-controls .guide-switch{width:auto;min-width:76px;font-size:12px;white-space:nowrap;}</style>

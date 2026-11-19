<script setup lang="ts">
import { uiText } from "../uiText";
import AutoAcceptNotice from "./AutoAcceptNotice.vue";
import PostGameNotice from "./PostGameNotice.vue";
import { computed, ref, watch } from 'vue';
import GuidePanel from './GuidePanel.vue';
import type { Snapshot } from '../../bindings/leaguepingo/internal/core/models';
import type { OverlayState } from '../../bindings/leaguepingo/models';
import BrandIcon from './BrandIcon.vue';
import AppIcon from './AppIcon.vue';
import QueueSwitch from './QueueSwitch.vue';
import { rankedPlayers } from '../playerAnalysis';
import { liveScoutMode } from '../scoutStats';
import SidebarPlayer from './SidebarPlayer.vue';
import ExperimentalBadge from './ExperimentalBadge.vue';
const props=defineProps<{snapshot:Snapshot|null;state:OverlayState|null;status:string;error:string}>();
defineEmits<{collapse:[value:boolean];queue:[value:number]}>();
const tab=ref('players');
const activeTeam = ref<'ally' | 'enemy'>('ally');
const hasEnemies = computed(() => props.snapshot?.stage === 'in-game' && !!props.snapshot.participants?.some(p => p.team === 'enemy'));
const players = computed(() => rankedPlayers(
  props.snapshot?.participants?.filter(p => p.team === (hasEnemies.value ? activeTeam.value : 'ally')) ?? [],
));
const hasGroups=computed(()=>players.value.some(p=>p.premadeGroup && (p.premadeSize||0)>1));
function expandPlayer(event:Event){const item=event.target as HTMLDetailsElement;if(item.open) item.parentElement?.querySelectorAll('details[open]').forEach(other=>{if(other!==item)other.removeAttribute('open');});}
const picking=computed(()=>props.snapshot?.phase==='ChampSelect');
watch([() => props.snapshot?.connectionId, () => props.snapshot?.gameId, hasEnemies], () => { activeTeam.value = 'ally'; });

</script>
<template>
  <main v-if="state?.mode==='rail'" class="sidebar-rail desktop-app">
    <button :aria-label="uiText.clientSidebar.expand" :title="uiText.clientSidebar.expand" @click="$emit('collapse',false)"><BrandIcon/><span>{{ picking?uiText.clientSidebar.selection:uiText.common.party }}</span><b>{{ players.length }}</b><AppIcon name="panel" :size="14"/></button>
  </main>
  <main v-else class="client-sidebar desktop-app" :data-side="state?.sidebarSide" :aria-label="uiText.clientSidebar.label">
    <header class="sidebar-header"><BrandIcon/><strong>LeaguePingo</strong><button class="icon-button" :aria-label="uiText.clientSidebar.collapse" :title="uiText.clientSidebar.collapseShort" @click="$emit('collapse',true)"><AppIcon name="minimise" :size="17"/></button></header>
    <div class="sidebar-heading"><div><h1><button class="sidebar-tab" :aria-label="uiText.clientSidebar.playerData" :aria-pressed="tab==='players'" @click="tab='players'">{{ hasEnemies ? uiText.clientSidebar.playerData : picking ? uiText.common.allies : uiText.common.party }}</button></h1><button class="sidebar-tab guide-tab" :aria-pressed="tab==='guide'" @click="tab='guide'">{{ uiText.clientSidebar.guide }}</button><small><ExperimentalBadge v-if="hasGroups" :feature="uiText.clientSidebar.groups" :label="uiText.clientSidebar.groupExperimental"/><template v-else>{{ status.split(' · ')[0] }}</template>{{ uiText.clientSidebar.members(players.length) }}</small></div><QueueSwitch v-if="tab==='players'" :current="liveScoutMode(snapshot)" :value="snapshot?.scoutQueue ?? 0" compact @change="$emit('queue',$event)"/></div>
    <p v-if="error || state?.warning || snapshot?.rosterError" class="quality-warning" role="status">{{ error || state?.warning || snapshot?.rosterError }}</p>
    <AutoAcceptNotice compact />
    <PostGameNotice compact />

    <GuidePanel v-show="tab==='guide'" :snapshot="snapshot" compact/>
    <div v-if="tab === 'players' && hasEnemies" class="sidebar-team-switch" role="group" :aria-label="uiText.matchRoster.title">
      <button v-for="team in ['ally', 'enemy'] as const" :key="team" type="button" :aria-pressed="activeTeam === team" @click="activeTeam = team">{{ uiText.common[team] }}</button>
    </div>
    <section v-show="tab==='players'" class="sidebar-players" @toggle.capture="expandPlayer" :aria-label="hasEnemies ? uiText.common[activeTeam] : uiText.clientSidebar.alliesLabel">
      <SidebarPlayer v-for="p in players" :key="p.key" :player="p" :queue="snapshot?.scoutQueue ?? 0" :picking="picking" :show-hero="hasEnemies"/>
      <p v-if="!players.length" class="sidebar-empty">{{ picking?uiText.clientSidebar.loadingAllies:uiText.clientSidebar.waitingParty }}</p>
    </section>
    <footer class="sidebar-footer"><span>{{ picking?uiText.clientSidebar.expandHelp:uiText.clientSidebar.selectionHelp }}</span><small>{{ state?.holdEnabled && state?.holdAvailable ? uiText.clientSidebar.holdHelp(state?.holdKey==='Backquote'?'~':state?.holdKey || '~') : state?.shortcutAvailable ? uiText.clientSidebar.shortcutHelp(state.shortcut) : uiText.clientSidebar.previewHelp }}</small></footer>
  </main>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.client-sidebar):is(.client-sidebar) { height:100dvh; width:100%; background:var(--surface); color:var(--text); border:1px solid var(--border); font-variant-numeric:tabular-nums; overflow:hidden; }
:where(.sidebar-rail):is(.sidebar-rail) { height:100dvh; width:100%; background:var(--surface); color:var(--text); border:1px solid var(--border); font-variant-numeric:tabular-nums; overflow:hidden; }
:where(.client-sidebar):is(.client-sidebar) { display:flex;flex-direction:column;font-size:12px;line-height:1.25; }
:where(.sidebar-header):is(.sidebar-header) { display:flex;align-items:center;gap:8px;padding:6px 12px;flex:none;border-bottom:1px solid var(--border);height:38px; }
:where(.sidebar-header):is(.sidebar-header) :deep(strong) {font-size:13px;letter-spacing:-.3px;}
:where(.sidebar-header):is(.sidebar-header) :deep(button) {margin-left:auto;padding:4px;}
:where(.sidebar-heading):is(.sidebar-heading) {flex:none;display:flex;flex-direction:column;padding:9px 10px 8px;gap:7px;}
:where(.sidebar-heading):is(.sidebar-heading) :deep(>div) {display:flex;align-items:baseline;justify-content:space-between;gap:8px;}
:where(.sidebar-heading):is(.sidebar-heading) :deep(h1) {font-size:18px;letter-spacing:-.5px;margin:0;}
:where(.sidebar-heading):is(.sidebar-heading) :deep(small) {font-size:11px;color:var(--muted);}
:where(.sidebar-footer):is(.sidebar-footer) {font-size:11px;color:var(--muted);}
:where(.sidebar-players):is(.sidebar-players) {flex:1;min-height:0;overflow:auto;overscroll-behavior:contain;padding:0 10px 10px;scrollbar-gutter:stable;}
:where(.sidebar-footer):is(.sidebar-footer) {display:flex;flex-direction:column;border-top:1px solid var(--border);flex:none;padding:6px 10px;gap:2px;}
:where(.sidebar-footer):is(.sidebar-footer) :deep(small) {font-size:11px;}
:where(.sidebar-empty):is(.sidebar-empty) {color:var(--muted);font-size:13px;padding:24px 6px;}
:where(.client-sidebar):is(.client-sidebar) :deep(>.quality-warning) {margin:0 10px 8px;font-size:11px;}
:where(.sidebar-rail):is(.sidebar-rail) :deep(>button) {display:flex;flex-direction:column;align-items:center;justify-content:center;gap:15px;width:100%;height:100%;padding:3px;background:var(--surface);color:var(--text);border:0;}
:where(.sidebar-rail):is(.sidebar-rail) :deep(span) {writing-mode:vertical-rl;letter-spacing:4px;font-size:12px;}
:where(.sidebar-rail):is(.sidebar-rail) :deep(b) {font-size:12px;color:var(--accent);}
</style>
<style scoped>.sidebar-tab{border:0;border-radius:0;background:none;color:var(--muted);font:inherit;padding:0;min-height:22px;cursor:pointer;}.sidebar-tab[aria-pressed="true"]{color:var(--text);border-bottom:1px solid var(--accent);}.guide-tab{font-size:12px;}.client-sidebar > .guide-panel{flex:1;min-height:0;overflow:auto;overscroll-behavior:contain;}</style>


<style scoped>
.sidebar-team-switch { display: flex; gap: 6px; padding: 0 10px 8px; }
.sidebar-team-switch button { flex: 1; min-height: 28px; background: var(--surface); color: var(--muted); border: 1px solid var(--border); border-radius: 6px; }
.sidebar-team-switch button[aria-pressed="true"] { background: var(--raised); color: var(--text); }
</style>

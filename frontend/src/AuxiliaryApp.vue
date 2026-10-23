<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted } from 'vue';
import { Events } from '@wailsio/runtime';
import * as Service from '../bindings/leaguepingo/appservice';
import type { Snapshot } from '../bindings/leaguepingo/internal/core/models';
import type { OverlayState } from '../bindings/leaguepingo/models';
import { useLeagueSnapshot } from './composables/useLeagueSnapshot';
import { useOverlayControls } from './composables/useOverlayControls';
import { useTheme } from './composables/useTheme';
import { errorMessage } from './presentation';
import OverlayView from './components/OverlayView.vue';
import ClientSidebar from './components/ClientSidebar.vue';
useTheme();
const { snapshot,acceptSnapshot,status }=useLeagueSnapshot();
const {overlay,overlayBusy,overlayError,acceptOverlay,hideOverlay,pinOverlay}=useOverlayControls();
const sidebar=computed(()=>['sidebar','rail'].includes(overlay.value?.mode || ''));
let alive=true; const stops:(()=>void)[]=[];
function onKey(e:KeyboardEvent){if(e.key==='Escape') void hideOverlay();}
onMounted(async()=>{
  window.addEventListener('keydown',onKey);
  stops.push(Events.On('leaguepingo:snapshot',e=>acceptSnapshot(e.data as Snapshot)));
  stops.push(Events.On('leaguepingo:overlay',e=>acceptOverlay(e.data as OverlayState)));
  const initial=await Promise.allSettled([Service.Snapshot(),Service.OverlayStatus()]);
  if(!alive)return;
  if(initial[0].status==='fulfilled')acceptSnapshot(initial[0].value);else overlayError.value=errorMessage(initial[0].reason);
  if(initial[1].status==='fulfilled')acceptOverlay(initial[1].value);else overlayError.value=errorMessage(initial[1].reason);
  await nextTick();
  if(alive && new URLSearchParams(location.search).get('view')==='companion'){
    try {await Service.AuxiliaryReady();}catch(reason){overlayError.value=errorMessage(reason);}
  }
});
onUnmounted(()=>{alive=false;stops.forEach(stop=>stop());window.removeEventListener('keydown',onKey);});
async function collapse(value:boolean){try{acceptOverlay(await Service.SetSidebarCollapsed(value));}catch(reason){overlayError.value=errorMessage(reason);}}
async function queue(value:number){try{acceptSnapshot(await Service.SetScoutQueue(value));}catch(reason){overlayError.value=errorMessage(reason);}}
</script>
<template>
  <ClientSidebar v-if="sidebar" :snapshot="snapshot" :state="overlay" :status="status" :error="overlayError" @collapse="collapse" @queue="queue" />
  <OverlayView v-else :snapshot="snapshot" :state="overlay" :status="status" :busy="overlayBusy" :error="overlayError" @hide="hideOverlay" @pin="pinOverlay" />
</template>

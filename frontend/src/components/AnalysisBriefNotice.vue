<script setup lang="ts">
import { computed, ref } from 'vue';
import * as Service from '../../bindings/leaguepingo/appservice';
import type { Snapshot } from '../../bindings/leaguepingo/internal/core/models';
import { brief, applyAnalysisBrief } from '../analysisBriefState';
import { uiText } from '../uiText';
import { errorMessage } from '../presentation';
const props = defineProps<{ snapshot?: Snapshot | null }>();
const busy = ref(false), error = ref('');
const state = computed(() => brief.value ?? props.snapshot?.analysisBrief);
const visible = computed(() => props.snapshot?.phase === 'ChampSelect' && state.value?.enabled && ['waiting','queued','waiting-chat','sending','muted'].includes(state.value.status));
const cancellable = computed(() => ['waiting','queued','waiting-chat'].includes(state.value?.status ?? ''));
const status = computed(() => uiText.analysisBrief.state[state.value?.status as keyof typeof uiText.analysisBrief.state] ?? '');
async function mute() {
 busy.value = true;
 try { applyAnalysisBrief(await Service.MuteAnalysisBrief()); }
 catch (reason) { error.value = errorMessage(reason); }
 finally { busy.value = false; }
}
</script>
<template>
 <div v-if="visible" class="brief-notice" role="status">
  <span>{{ error || status }}</span>
  <button v-if="cancellable" type="button" :disabled="busy" @click="mute">{{uiText.analysisBrief.mute}}</button>
 </div>
</template>
<style scoped>
.brief-notice {display:flex;align-items:center;justify-content:space-between;gap:8px;flex-wrap:wrap;margin:4px 10px 8px;font-size:11px;color:var(--muted);}
.brief-notice button {font-size:11px;padding:3px 6px;min-height:26px;}
</style>

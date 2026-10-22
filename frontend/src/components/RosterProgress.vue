<script setup lang="ts">
import { computed } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { scoutStatus } from "../scoutStatus";
const props = defineProps<{ players: Participant[]; queue: number; compact?: boolean }>();
const progress = computed(() => scoutStatus(props.players, props.queue));
</script>
<template>
  <div class="scout-progress" :class="{ 'scout-progress-compact': compact }" role="status" aria-label="本局资料准备进度">
    <span>身份 <b>{{ progress.known }}/{{ progress.total }}</b></span>
    <span>近期战绩 <b>{{ progress.recent }}/{{ progress.total }}</b></span>
    <span class="scout-progress-coverage">统计完整 <b>{{ progress.complete }}/{{ progress.total }}</b><i class="scout-progress-track" aria-hidden="true"><i :style="{ width: `${progress.total ? progress.complete / progress.total * 100 : 0}%` }" /></i></span>
    <span v-if="progress.loading" class="scout-progress-note">正在补齐 {{ progress.loading }} 人统计</span>
    <span v-else-if="progress.partial" class="scout-progress-note">{{ progress.partial }} 人为已读范围</span>
    <span v-else-if="progress.known > progress.complete" class="scout-progress-note">部分统计待获取，可刷新重试</span>
    <span v-if="progress.retained" class="scout-progress-note">{{ progress.retained }} 人保留上次统计</span>
  </div>
</template>
<style scoped>
.scout-progress { display: flex; align-items: center; flex-wrap: wrap; gap: 6px 18px; min-height: 28px; padding: 3px 2px; margin-bottom: 4px; font-size: 12px; color: var(--muted); font-variant-numeric: tabular-nums; }
.scout-progress b { color: var(--text); font-weight: 600; margin-left: 4px; }
.scout-progress-coverage { display: inline-flex; align-items: center; gap: 5px; }
.scout-progress-track { display: inline-flex; width: 64px; height: 3px; background: var(--border); overflow: hidden; border-radius: 2px; }
.scout-progress-track > i { height: 100%; background: var(--ally); }
.scout-progress-note { color: var(--muted); }
.scout-progress-compact { min-height: 22px; font-size: 11px; gap: 4px 14px; margin: 0; padding: 1px 0; color: var(--scout-muted); }
.scout-progress-compact b { color: var(--scout-text); }
.scout-progress-compact .scout-progress-note { color: var(--scout-muted); }
.scout-progress-compact .scout-progress-track { width: 48px; background: var(--scout-track); }
</style>

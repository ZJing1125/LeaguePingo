<script setup lang="ts">
import { uiText } from "../uiText";
// Only in-flight or actionable steps surface here; a static "已开启" row is gone
// because the navigation status chip and the automation page already say it.
import { useAutomation } from '../composables/useAutomation';
defineProps<{ compact?: boolean }>();
const { acceptVisible, acceptText, accept, skipAccept, busy, error } = useAutomation();
</script>
<template>
  <div v-if="acceptVisible" class="auto-accept-notice" :class="{ compact }" :data-status="accept?.status">
    <span role="status">{{ error || acceptText }}</span>
    <button v-if="accept?.status === 'countdown'" class="secondary" :disabled="busy" @click="skipAccept">{{ uiText.common.skipOnce }}</button>
  </div>
</template>
<style scoped>
.auto-accept-notice{display:flex;align-items:center;justify-content:space-between;gap:8px;min-height:32px;margin:0 0 12px;padding:6px 10px;border:1px solid var(--border);border-radius:6px;color:var(--text);font-size:12px;font-variant-numeric:tabular-nums;}
.auto-accept-notice[data-status="failed"],.auto-accept-notice[data-status="unsupported"]{color:var(--danger, #e06c6c);}
.auto-accept-notice button{flex:none;min-height:28px;padding:3px 8px;font-size:12px;}
.auto-accept-notice.workspace-message{margin:8px 20px 0;flex:none;}
.compact{margin:0 8px 8px;flex-wrap:wrap;}
</style>

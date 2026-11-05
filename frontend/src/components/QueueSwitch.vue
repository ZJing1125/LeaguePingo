<script setup lang="ts">
import { uiText } from "../uiText";
defineProps<{ value: number; busy?: boolean; compact?: boolean }>();
defineEmits<{ change: [queue: number] }>();
</script>
<template>
  <div class="queue-switch" :class="{ compact }" role="group" :aria-label="uiText.queueSwitch.label">
    <button v-for="mode in [{ id: 420, label: uiText.common.solo }, { id: 440, label: uiText.queueSwitch.flex }]" :key="mode.id" type="button" :aria-pressed="value === mode.id" :disabled="busy" @click="$emit('change', mode.id)">{{ mode.label }}</button>
  </div>
</template>
<style scoped>
/* Preserve source cascade and contextual sizing; shared tokens remain global. */
/* Scope inside :where; :is retains one class weight against the sidebar layout. */
:where(.queue-switch):is(.queue-switch) { display: inline-flex; padding: 3px; gap: 3px; background: var(--raised); border: 1px solid var(--border); border-radius: 9px; align-self: center; flex: none; }
.queue-switch button { border: 1px solid transparent; color: var(--muted); background: transparent; min-height: 30px; padding: 5px 13px; font-size: 12px; border-radius: 6px; white-space: nowrap; }
.queue-switch button:hover:not(:disabled) { background: var(--hover); color: var(--text); }
.queue-switch button[aria-pressed="true"] { color: var(--on-accent); background: var(--primary-bg); box-shadow: 0 2px 5px #0002; }
@media (max-width: 999px) { .queue-switch button { padding-inline: 10px; } }
.queue-switch button[aria-pressed="true"]:hover:not(:disabled) { background: var(--primary-bg); color: var(--on-accent); filter: brightness(1.07); }
.sidebar-heading .queue-switch {display:flex;}
.sidebar-heading .queue-switch button {flex:1;font-size:12px;padding:5px 9px;}
</style>

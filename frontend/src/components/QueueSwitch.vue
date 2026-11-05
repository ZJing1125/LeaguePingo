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

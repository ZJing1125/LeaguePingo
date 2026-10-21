<script setup lang="ts">
defineProps<{ value: number; busy?: boolean; compact?: boolean }>();
defineEmits<{ change: [queue: number] }>();
</script>
<template>
  <div class="queue-switch" :class="{ compact }" role="group" aria-label="对局统计排位模式">
    <button v-for="mode in [{ id: 420, label: '单双排' }, { id: 440, label: '灵活组排' }]" :key="mode.id" type="button" :aria-pressed="value === mode.id" :disabled="busy" @click="$emit('change', mode.id)">{{ mode.label }}</button>
  </div>
</template>

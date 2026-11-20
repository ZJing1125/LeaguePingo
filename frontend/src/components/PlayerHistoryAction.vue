<script setup lang="ts">
import { computed, inject } from "vue";
import { playerHistoryNavigation, type PlayerHistoryTarget } from "../playerHistoryNavigation";
import { uiText } from "../uiText";

const props = defineProps<{ target?: PlayerHistoryTarget; label?: string; showDisabled?: boolean }>();
const navigation = inject(playerHistoryNavigation, null);
const available = computed(() => navigation?.available(props.target) ?? false);
const label = computed(() => uiText.playerHistoryNavigation.open(props.target?.label || props.label || uiText.playerHistoryNavigation.player));

function open() {
  if (navigation?.available(props.target)) void navigation.open(props.target);
}
</script>

<template>
  <button v-if="available || showDisabled" type="button" class="player-history-action" :disabled="!available"
    :title="available ? label : uiText.playerHistoryNavigation.unavailable" :aria-label="label" @click.stop="open">
    <slot/>
  </button>
  <span v-else :title="target ? uiText.playerHistoryNavigation.unavailable : undefined"><slot/></span>
</template>

<style scoped>
.player-history-action {
  min-width:0;
  border:0;
  margin:0;
  padding:0;
  background:none;
  color:inherit;
  font:inherit;
  text-align:inherit;
}
.player-history-action:not(:disabled):hover { text-decoration:underline; color:var(--accent); }
.player-history-action:focus-visible { outline-offset:1px; }
</style>

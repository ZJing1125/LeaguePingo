<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { championPortrait } from "../gameAssets";
const props = defineProps<{ id?: number }>();
const broken = ref(false);
watch(
  () => props.id,
  () => (broken.value = false),
);
const src = computed(() => championPortrait(props.id));
</script>
<template>
  <img
    v-if="src && !broken"
    class="hero-art"
    :src="src"
    alt=""
    decoding="async"
    @error="broken = true"
  /><span v-else class="portrait-fallback" aria-hidden="true">P</span>
</template>

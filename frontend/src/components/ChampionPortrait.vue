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
<style scoped>
/* Preserve source cascade and contextual sizing; shared tokens remain global. */
.hero-art {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: 50% 23%;
  z-index: -2;
}
.portrait-fallback {
  position: absolute;
  inset: 0;
  z-index: -2;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, #4a605d, #192e35);
  font:
    bold italic 70px "Segoe UI",
    sans-serif;
  color: #ffffff12;
}
</style>

<script setup lang="ts">
import { ref, watch } from "vue";
import { championSplash } from "../gameAssets";
const props = defineProps<{ id?: number }>();
const broken = ref(false);
watch(() => props.id, () => broken.value = false);
</script>
<template>
  <img v-if="championSplash(id) && !broken" class="hero-splash" :src="championSplash(id)" alt="" decoding="async" @error="broken = true" />
  <span v-else class="splash-fallback" aria-hidden="true" />
</template>
<style scoped>
/* Preserve source cascade and contextual sizing; shared tokens remain global. */
.home-hero-art .hero-splash { width: 100%; height: 100%; object-fit: cover; object-position: 50% 30%; }
</style>

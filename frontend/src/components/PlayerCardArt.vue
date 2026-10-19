<script setup lang="ts">
import ChampionPortrait from "./ChampionPortrait.vue";
defineProps<{ id?: number }>();
</script>
<template>
  <span v-if="id" class="card-artwork" aria-hidden="true">
    <ChampionPortrait :id="id" class="card-art-blur" />
    <ChampionPortrait :id="id" class="card-art-figure" />
  </span>
</template>

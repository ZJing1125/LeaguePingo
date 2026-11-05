<script setup lang="ts">
defineProps<{ role?: string }>();
</script>
<template>
  <!-- Geometry follows the five League client position glyphs; colour stays in our theme. -->
  <svg class="role-icon" viewBox="0 0 34 34" fill="currentColor" aria-hidden="true">
    <template v-if="role==='TOP'">
      <path opacity=".38" fill-rule="evenodd" d="M21 14h-7v7h7v-7Zm5-3v15H11l-4 4h23V7Z"/>
      <path d="M4 4v24l5-5V9h14l5-5H4Z"/>
    </template>
    <template v-else-if="role==='JUNGLE'">
      <path fill-rule="evenodd" d="M25 3c-2.13 3.3-5.15 6.85-6.97 11.47A42.4 42.4 0 0 1 20 20c.2-1.03.53-2.03 1-3 0-4.98 1.86-8.72 4-14ZM13 20c-1.49-4.49-4.76-6.97-9-9 3.87 3.14 4.42 7.52 5 12l3.74 3.31C14.22 27.92 16.53 30.45 17 31c4.56-9.45-3.37-20.8-8-28 2.67 6.57 4.72 10.34 4 17Zm8 5c.08.67.08 1.33 0 2l4-4c.58-4.48 1.13-8.86 5-12-5.29 2.54-7.87 7.85-9 14Z"/>
    </template>
    <template v-else-if="role==='MIDDLE'">
      <path opacity=".38" fill-rule="evenodd" d="M30 13l-4 4v9h-9l-4 4h17ZM17 8l4-4H4v17l4-4V8h9Z"/>
      <path d="M25 4 4 25v5h5L30 9V4h-5Z"/>
    </template>
    <template v-else-if="role==='BOTTOM'">
      <path opacity=".38" fill-rule="evenodd" d="M13 20h7v-7h-7v7ZM4 4v23l4-4V8h15l4-4H4Z"/>
      <path d="m30 6-5 5v14H11l-5 5h24V6Z"/>
    </template>
    <template v-else-if="role==='UTILITY'">
      <path fill-rule="evenodd" d="M26 13c3.54 0 8-4 8-4H23l-3 3 2 7 5-2-3-4h2ZM22 5l-1.17-2h-7.77L12 5l5 6 5-6Zm-5 9-1-1-3 15 4 3 4-3-3-15-1 1ZM11 9H0s4.47 4 8 4h2l-3 4 5 2 2-7-3-3Z"/>
    </template>
    <path v-else fill-rule="evenodd" d="M6 6h22v22H6V6Zm4 4v14h14V10H10Zm5 4a4 4 0 1 1 5 3.87V21h-3v-5h1a1 1 0 1 0-1-1h-3c0-.34.04-.68.12-1H15Zm2 9h3v3h-3v-3Z"/>
  </svg>
</template>
<style scoped>
/* Preserve source cascade and contextual sizing; shared tokens remain global. */
.scout-ring .role-icon { width: 23px; height: 23px; color: var(--scout-warning); }
</style>

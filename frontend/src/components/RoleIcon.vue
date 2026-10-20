<script setup lang="ts">
defineProps<{ role?: string }>();
const paths: Record<string, string> = {
  TOP: "M4 4h16l-5 5H9v6l-5 5zM13 20h7v-7",
  JUNGLE: "m12 21-5-6L4 5l6 6-1-8 5 10 6-8-3 11z",
  MIDDLE: "M4 4h8L4 12zm16 16h-8l8-8zM5 19 19 5l-3-1L4 16z",
  BOTTOM: "M20 20H4l5-5h6V9l5-5zM4 11V4h7",
  UTILITY: "m12 4 3 5 6 2-5 3-1 6-3-3-3 3-1-6-5-3 6-2z",
};
</script>
<template>
  <svg class="role-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round" aria-hidden="true">
    <path :d="paths[role ?? ''] ?? 'M5 5h14v14H5zM9 10a3 3 0 1 1 4 3v2M12 18h.01'" />
  </svg>
</template>

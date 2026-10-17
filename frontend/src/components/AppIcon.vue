<script setup lang="ts">
withDefaults(
  defineProps<{
    name:
      | "history"
      | "match"
      | "search"
      | "settings"
      | "panel"
      | "copy"
      | "refresh"
      | "chevron"
      | "close"
      | "user"
      | "shield"
      | "pin";
    size?: number;
  }>(),
  { size: 18 },
);
const paths = {
  history: "M3 12a9 9 0 1 0 3-6.7M3 4v5h5M12 7v5l3 2",
  match:
    "M4 4l6 6m4 4 6 6M20 4l-6 6m-4 4-6 6M3 8V3h5m8 0h5v5M3 16v5h5m8 0h5v-5",
  search: "M16 16l5 5M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0",
  settings:
    "M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8M9 3l-1 3-3 1-2 3 2 2-1 3 2 3 3-1 2 3h3l1-3 3-1 2-3-2-2 1-3-2-3-3 1-2-3z",
  panel: "M3 4h18v16H3zM8 4v16M12 8h5m-5 4h5",
  copy: "M8 8h12v13H8zM16 8V3H3v13h5",
  refresh: "M20 8a8 8 0 0 0-14-3L3 8m0-5v5h5M4 16a8 8 0 0 0 14 3l3-3m0 5v-5h-5",
  chevron: "M9 5l7 7-7 7",
  close: "M6 6l12 12M6 18 18 6",
  user: "M20 21v-2a7 7 0 0 0-14 0v2M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0",
  shield: "M12 2l8 4v6c0 5-8 10-8 10S4 17 4 12V6zM8 12l3 3 5-6",
  pin: "M8 3h8l-1 7 4 4H5l4-4zM12 14v8",
};
</script>
<template>
  <svg
    class="app-icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.7"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="paths[name]" />
  </svg>
</template>

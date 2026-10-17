<script setup lang="ts">
defineProps<{ message?: string; bannerClass?: string }>();
</script>
<template>
  <Transition name="notice">
    <div v-if="message" class="transient-notice" role="alert">
      <div class="notice-clip">
        <p class="error-banner" :class="bannerClass">{{ message }}</p>
      </div>
    </div>
  </Transition>
</template>
<style scoped>
.transient-notice {
  display: grid;
  grid-template-rows: 1fr;
  opacity: 1;
  flex: none;
}
.notice-clip {
  min-height: 0;
  overflow: hidden;
}
.notice-enter-active {
  transition:
    opacity 150ms ease-out,
    grid-template-rows 150ms ease-out;
}
.notice-leave-active {
  transition:
    opacity 200ms ease-in,
    grid-template-rows 200ms ease-in;
}
.notice-enter-from,
.notice-leave-to {
  opacity: 0;
  grid-template-rows: 0fr;
}
@media (prefers-reduced-motion: reduce) {
  .notice-enter-active,
  .notice-leave-active {
    transition: none;
  }
}
</style>

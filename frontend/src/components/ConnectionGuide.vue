<script setup lang="ts">
import { uiText } from "../uiText";
const copy = uiText.connectionGuide;
</script>

<template>
  <section class="connection-guide" :aria-label="copy.title">
    <h2>{{ copy.title }}</h2>
    <ol>
      <li v-for="(step, index) in copy.steps" :key="step">
        <span class="step-number" aria-hidden="true">{{ index + 1 }}</span>
        <span>{{ step }}</span>
      </li>
    </ol>
  </section>
</template>

<style scoped>
.connection-guide { margin-top: 12px; padding: 14px 18px; border: 1px solid var(--border); border-radius: 8px; background: var(--panel); }
h2 { margin: 0 0 12px; font-size: 14px; color: var(--text); }
ol { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; margin: 0; padding: 0; list-style: none; }
li { display: flex; align-items: flex-start; gap: 8px; font-size: 12px; line-height: 1.6; color: var(--text); }
.step-number { display: inline-grid; place-items: center; flex: 0 0 22px; height: 22px; border: 1px solid var(--border); border-radius: 50%; color: var(--accent); font-weight: 700; }
@media (max-width: 680px) { ol { grid-template-columns: 1fr; gap: 10px; } }
</style>

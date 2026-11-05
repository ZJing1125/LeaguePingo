<script setup lang="ts">
import { uiText } from "../uiText";
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { OverlayState, AppInfo } from "../../bindings/leaguepingo/models";
import { errorMessage } from "../presentation";
import { assetVersion } from "../gameAssets";
import type { ThemePreference } from "../composables/useTheme";
import AppIcon from "./AppIcon.vue";
defineProps<{ overlayState?: OverlayState | null; theme: ThemePreference; resolvedTheme: string; themeError: string }>();
defineEmits<{ close: []; theme: [value: ThemePreference]; automation: [] }>();
const areas = [{ id: 'appearance', label: uiText.settingsPanel.appearance }, { id: 'game', label: uiText.common.automation }, { id: 'diagnostics', label: uiText.settingsPanel.about }];
const area = ref('appearance');
const holdEnabled = ref(true), holdKey = ref('Backquote'), shortcut = ref('Ctrl+Shift+P');
const sidebarEnabled = ref(true), previewEnabled = ref(true), sidebarWidth = ref(320);
const guidesEnabled = ref(true);
const info = ref<AppInfo | null>(null), busy = ref(false), message = ref(''), error = ref('');
const dirty = computed(() => !!info.value && (shortcut.value !== info.value.shortcut || holdEnabled.value !== (info.value.holdEnabled ?? true) || holdKey.value !== (info.value.holdKey || 'Backquote') || sidebarEnabled.value !== (info.value.sidebarEnabled ?? true) || previewEnabled.value !== (info.value.previewEnabled ?? true) || sidebarWidth.value !== (info.value.sidebarWidth || 320) || guidesEnabled.value !== (info.value.guidesEnabled ?? true)));
const metrics = computed(() => Object.entries(info.value?.requests ?? {}).flatMap(([category, metric]) => metric ? [{ category, ...metric }] : []));
const panel = ref<HTMLElement | null>(null);
const opener = document.activeElement;
let alive = true;
function trapFocus(event: KeyboardEvent) {
  if (event.key !== 'Tab' || !panel.value) return;
  const controls = Array.from(panel.value.querySelectorAll<HTMLElement>('button:not(:disabled), select:not(:disabled), input:not(:disabled), summary, [tabindex="0"]')).filter(e => e.getClientRects().length);
  const first = controls[0], last = controls.at(-1);
  if (event.shiftKey && (document.activeElement === first || document.activeElement === panel.value)) { event.preventDefault(); last?.focus(); }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
}
async function moveArea(event: KeyboardEvent, index: number) {
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? areas.length - 1 : event.key === 'ArrowRight' ? (index + 1) % areas.length : event.key === 'ArrowLeft' ? (index + areas.length - 1) % areas.length : -1;
  if (next < 0) return;
  event.preventDefault(); area.value = areas[next]!.id;
  await nextTick(); panel.value?.querySelector<HTMLElement>(`#settings-tab-${area.value}`)?.focus();
}
async function load() {
  busy.value = true; error.value = '';
  try {
    const result = await Service.Info();
    if (!alive) return;
    info.value = result; shortcut.value = result.shortcut;
    holdEnabled.value = result.holdEnabled ?? true; holdKey.value = result.holdKey || 'Backquote';
    sidebarEnabled.value=result.sidebarEnabled ?? true;previewEnabled.value=result.previewEnabled ?? true;sidebarWidth.value=result.sidebarWidth || 320;
    guidesEnabled.value=result.guidesEnabled ?? true;
  } catch (reason) { if (alive) error.value = errorMessage(reason); }
  finally { busy.value = false; }
}
async function saveGame() {
  if (busy.value || !info.value) return;
  busy.value = true; error.value = ''; message.value = '';
  let applied = false;
  try {
    if (shortcut.value !== info.value.shortcut) { info.value = await Service.ConfigureShortcut(shortcut.value); applied = true; }
    if (holdEnabled.value !== (info.value.holdEnabled ?? true) || holdKey.value !== (info.value.holdKey || 'Backquote')) {
      info.value = await Service.ConfigureHold(holdEnabled.value, holdKey.value); applied = true;
    }
    if(sidebarEnabled.value !== (info.value.sidebarEnabled ?? true) || previewEnabled.value !== (info.value.previewEnabled ?? true) || sidebarWidth.value !== (info.value.sidebarWidth || 320)){
      info.value=await Service.ConfigureCompanion(sidebarEnabled.value,previewEnabled.value,sidebarWidth.value);applied=true;
    }
    if(guidesEnabled.value !== (info.value.guidesEnabled ?? true)){info.value=await Service.ConfigureGuides(guidesEnabled.value);applied=true;}
    message.value = uiText.settingsPanel.saved;
  } catch (reason) { error.value = `${applied ? uiText.common.partiallyApplied : ''}${errorMessage(reason)}`; }
  finally { busy.value = false; }
}
function defaults() { shortcut.value = 'Ctrl+Shift+P'; holdEnabled.value = true; holdKey.value = 'Backquote'; sidebarEnabled.value=true;previewEnabled.value=true;sidebarWidth.value=320;guidesEnabled.value=true; message.value = uiText.settingsPanel.defaults; }
async function exportReport() {
  busy.value = true; error.value = ''; message.value = '';
  try { message.value = uiText.settingsPanel.diagnosticSaved(await Service.ExportDiagnostics()); }
  catch (reason) { error.value = errorMessage(reason); }
  finally { busy.value = false; }
}
onMounted(() => { panel.value?.focus(); void load(); });
onUnmounted(() => { alive = false; void nextTick(() => { if (opener instanceof HTMLElement && opener.isConnected) opener.focus({ preventScroll: true }); }); });
</script>
<template>
  <div class="settings-backdrop" @click.self="$emit('close')">
    <section ref="panel" tabindex="-1" class="settings-panel" role="dialog" aria-modal="true" :aria-label="uiText.common.settings" @keydown="trapFocus" @keydown.esc.stop="$emit('close')">
      <header><h1>{{ uiText.common.settings }}</h1><button class="icon-button" :aria-label="uiText.settingsPanel.close" @click="$emit('close')"><AppIcon name="close" :size="18" /></button></header>
      <nav class="settings-tabs" role="tablist" :aria-label="uiText.settingsPanel.categories">
        <button v-for="(item, index) in areas" :id="`settings-tab-${item.id}`" :key="item.id" role="tab" :aria-selected="area === item.id" :aria-controls="`settings-area-${item.id}`" :tabindex="area === item.id ? 0 : -1" @click="area = item.id" @keydown="moveArea($event, index)">{{ item.label }}<span v-if="item.id === 'game' && dirty" class="settings-unsaved" :aria-label="uiText.settingsPanel.unsaved"> ·</span></button>
      </nav>
      <div :id="`settings-area-${area}`" class="settings-area" role="tabpanel" :aria-labelledby="`settings-tab-${area}`">
        <template v-if="area === 'appearance'">
          <fieldset class="theme-choices">
            <legend>{{ uiText.settingsPanel.theme }}</legend>
            <label v-for="choice in [{ value: 'light', name: uiText.common.light, caption: uiText.settingsPanel.lightCaption }, { value: 'dark', name: uiText.common.dark, caption: uiText.settingsPanel.darkCaption }, { value: 'system', name: uiText.common.system, caption: uiText.settingsPanel.systemCaption }]" :key="choice.value" :class="{ selected: theme === choice.value }">
              <input type="radio" name="theme" :value="choice.value" :checked="theme === choice.value" @change="$emit('theme', choice.value as ThemePreference)" />
              <span class="theme-swatch" :class="choice.value" aria-hidden="true"><i /><i /><i /></span>
              <strong>{{ choice.name }}</strong><small>{{ choice.caption }}</small>
            </label>
          </fieldset>
          <p class="muted">{{ uiText.settingsPanel.previewTheme(theme === 'system' ? uiText.settingsPanel.systemTheme(resolvedTheme === 'dark' ? uiText.common.dark : uiText.common.light) : uiText.settingsPanel.themeSaved) }}</p>
          <p v-if="themeError" class="quality-warning" role="status">{{ themeError }}</p>
        </template>
        <form v-else-if="area === 'game'" class="settings-game" @submit.prevent="saveGame">
          <section class="settings-group"><h2>{{ uiText.settingsPanel.automation }}</h2><p class="muted">{{ uiText.settingsPanel.automationHelp }}</p><div class="settings-actions"><button type="button" class="secondary" @click="$emit('automation')">{{ uiText.settingsPanel.openAutomation }}</button></div></section>
          <section class="settings-group"><h2>{{ uiText.settingsPanel.companion }}</h2><p class="muted">{{ uiText.settingsPanel.companionHelp }}</p><div class="settings-companion"><label><input v-model="sidebarEnabled" type="checkbox" :disabled="busy"/>{{ uiText.settingsPanel.enableSidebar }}</label><label><input v-model="previewEnabled" type="checkbox" :disabled="busy"/>{{ uiText.settingsPanel.autoPreview }}</label><label>{{ uiText.settingsPanel.sidebarWidth }}<select v-model.number="sidebarWidth" :disabled="busy" :aria-label="uiText.settingsPanel.sidebarWidth"><option v-for="width in [280,300,320,340,360,380,400,420]" :key="width" :value="width">{{ width }} px</option></select></label></div></section>
          <section class="settings-group"><h2>{{ uiText.common.guide }}</h2><p class="muted">{{ uiText.settingsPanel.guidesHelp }}</p><div class="settings-companion"><label><input v-model="guidesEnabled" type="checkbox" :disabled="busy"/>{{ uiText.settingsPanel.enableGuides }}</label></div></section>
          <section class="settings-group"><h2>{{ uiText.settingsPanel.hold }}</h2><p class="muted">{{ uiText.settingsPanel.holdHelp }}</p>
            <p class="muted">{{ uiText.settingsPanel.inputPrivacy }}</p>
            <div class="settings-hold"><label><input v-model="holdEnabled" type="checkbox" :aria-label="uiText.settingsPanel.holdEnabledLabel" :disabled="busy" />{{ uiText.settingsPanel.holdEnabled }}</label><select v-model="holdKey" :aria-label="uiText.settingsPanel.holdKey" :disabled="busy"><option value="Backquote">{{ uiText.settingsPanel.backquote }}</option><option v-for="key in ['F8', 'F9', 'F10']" :key="key" :value="key">{{ key }}</option></select></div>
            <p v-if="overlayState?.holdWarning" class="quality-warning" role="status">{{ overlayState.holdWarning }}</p>
          </section>
          <section class="settings-group"><h2>{{ uiText.settingsPanel.shortcut }}</h2><p class="muted">{{ uiText.settingsPanel.shortcutHelp }}</p><select v-model="shortcut" :aria-label="uiText.settingsPanel.shortcutLabel" :disabled="busy"><option v-for="letter in 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')" :key="letter" :value="`Ctrl+Shift+${letter}`">Ctrl+Shift+{{ letter }}</option></select></section>
          <p class="settings-applied muted" v-if="info">{{ uiText.settingsPanel.applied((info.holdEnabled ?? true) ? ((info.holdKey || 'Backquote') === 'Backquote' ? '~' : info.holdKey) : uiText.settingsPanel.holdDisabled, info.shortcut) }}</p>
          <details class="settings-help"><summary>{{ uiText.settingsPanel.keyHelp }}</summary><p>{{ uiText.settingsPanel.keyHelpDescription }}</p></details>
          <div class="settings-actions"><button type="button" class="secondary" :disabled="busy" @click="defaults">{{ uiText.settingsPanel.restore }}</button><span class="muted">{{ dirty ? uiText.common.unsaved : uiText.common.synced }}</span><button class="primary" :disabled="busy || !dirty">{{ busy ? uiText.common.saving : uiText.settingsPanel.save }}</button></div>
        </form>
        <template v-else>
          <h2>LeaguePingo {{ info?.version ?? uiText.common.loading }} · {{ info?.commit ?? '—' }}</h2>
          <p class="muted">{{ uiText.settingsPanel.build(info?.buildTime ?? '—', assetVersion) }}</p>
          <section class="settings-validation" :aria-label="uiText.settingsPanel.validationLabel"><h2>{{ uiText.settingsPanel.validation }}</h2><p class="muted">{{ uiText.settingsPanel.validationHelp }}</p></section>
          <h2>{{ uiText.settingsPanel.diagnostics }}</h2><p class="muted">{{ uiText.settingsPanel.diagnosticsHelp }}</p>
          <div v-if="info" class="runtime-summary"><span>{{ uiText.settingsPanel.heap((info.goHeapBytes / 1048576).toFixed(1)) }}</span><span>{{ uiText.settingsPanel.goroutines(info.goroutines) }}</span><small>{{ info.performanceScope }}</small></div>
          <table v-if="metrics.length" class="diagnostic-table"><thead><tr><th>{{ uiText.settingsPanel.category }}</th><th>{{ uiText.settingsPanel.requests }}</th><th>{{ uiText.settingsPanel.latency }}</th></tr></thead><tbody><tr v-for="metric in metrics" :key="metric.category"><td>{{ metric.category }}</td><td>{{ metric.calls }} / {{ metric.failed }}</td><td>{{ metric.calls ? Math.round(metric.totalMs / metric.calls) : '—' }} / {{ metric.maxMs }}</td></tr></tbody></table>
          <div class="settings-actions"><button class="secondary" :disabled="busy" @click="load">{{ uiText.settingsPanel.refreshDiagnostics }}</button><button class="secondary" :disabled="busy" @click="exportReport">{{ uiText.settingsPanel.exportDiagnostics }}</button></div>
        </template>
        <p v-if="info?.settingsWarning" class="quality-warning" role="status">{{ info.settingsWarning }}</p>
        <p v-if="error" class="error-banner" role="alert">{{ error }}</p><p v-if="message" class="settings-message" role="status">{{ message }}</p>
      </div>
    </section>
  </div>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.theme-choices):is(.theme-choices) {
  border: 0;
  padding: 0;
  margin: 8px 0 0;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}
:where(.theme-choices):is(.theme-choices) :deep(legend) { font-weight: 600; margin-bottom: 12px; }
:where(.theme-choices):is(.theme-choices) :deep(label) {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 7px;
  padding: 10px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 7px;
  cursor: pointer;
}
:where(.theme-choices):is(.theme-choices) :deep(label.selected) { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent); }
:where(.theme-choices):is(.theme-choices) :deep(input) { position: absolute; right: 9px; bottom: 30px; accent-color: var(--accent); }
:where(.theme-choices):is(.theme-choices) :deep(strong) { font-size: 13px; }
:where(.theme-choices):is(.theme-choices) :deep(small) { color: var(--muted); font-size: 11px; }
:where(.theme-swatch):is(.theme-swatch) { position: relative; width: 100%; height: 76px; padding: 28px 8px 8px; display: flex; gap: 5px; background: #f2f1eb; border: 1px solid #8883; border-radius: 4px; }
:where(.theme-swatch):is(.theme-swatch) :deep(i) { width: 24%; height: 100%; background: #d75530; border-radius: 2px; }
:where(.theme-swatch):is(.theme-swatch) :deep(i+i) { background: #d6d9cd; }
:where(.theme-swatch.dark):is(.theme-swatch.dark) { background: #1c1c1f; }
:where(.theme-swatch.dark):is(.theme-swatch.dark) :deep(i) { background: #7145db; }
:where(.theme-swatch.dark):is(.theme-swatch.dark) :deep(i+i) { background: #3b3b44; }
:where(.theme-swatch.system):is(.theme-swatch.system) { background: linear-gradient(90deg, #f2f1eb 50%, #1c1c1f 50%); }
:where(.theme-swatch.system):is(.theme-swatch.system) :deep(i) { background: #d75530; }
:where(.theme-swatch.system):is(.theme-swatch.system) :deep(i+i) { background: #7145db; }
:where(.theme-swatch):is(.theme-swatch)::before { content: ""; position: absolute; inset: 7px 8px auto; height: 12px; background: #dddeda; border-bottom: 2px solid #bf4724; }
:where(.theme-swatch.dark):is(.theme-swatch.dark)::before { background: #29292e; border-bottom-color: #9876ff; }
:where(.theme-swatch.system):is(.theme-swatch.system)::before { background: #a8a8b6; border-bottom-color: #7145db; }
:where(.theme-swatch):is(.theme-swatch) :deep(i) { width: 33.333%; background: #e8e8e0; border: 1px solid #0002; }
:where(.theme-swatch):is(.theme-swatch) :deep(i+i) { background: #e8e8e0; }
:where(.theme-swatch.dark):is(.theme-swatch.dark) :deep(i) { background: #344052; border-color: #fff1; }
</style>
<style scoped>
/* Component-owned rules, retaining foundation/style cascade order. */
.settings-backdrop {
  position: fixed;
  inset: var(--titlebar-height) 0 0;
  z-index: 20;
  background: #0009;
  display: grid;
  place-items: center;
  padding: 24px;
}
.settings-panel {
  display: flex;
  flex-direction: column;
  gap: 15px;
  width: min(620px, 100%);
  max-height: 100%;
  overflow: auto;
  padding: 24px;
  background: var(--surface);
  border: 1px solid #3c4759;
  border-radius: 10px;
  box-shadow: 0 18px 60px #0008;
}
.settings-panel header,
.settings-shortcut,
.settings-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.settings-shortcut select {
  flex: 1;
}
.settings-panel h2 {
  margin-top: 10px;
}
.runtime-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  font-size: 12px;
}
.runtime-summary small {
  width: 100%;
  color: var(--muted);
  line-height: 1.6;
}
.diagnostic-table {
  border-collapse: collapse;
  width: 100%;
  font-size: 12px;
}
.diagnostic-table th,
.diagnostic-table td {
  border-bottom: 1px solid var(--border);
  padding: 8px 5px;
  text-align: left;
}
.diagnostic-table th {
  color: var(--muted);
}
.settings-backdrop {
  background: #1d282f88;
}
.settings-panel {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 4px;
}
.runtime-summary {
  background: var(--raised);
}
.diagnostic-table th,
.diagnostic-table td {
  border-color: var(--border);
}
.settings-hold {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
.settings-hold label {
  display: flex;
  align-items: center;
  gap: 6px;
}
.settings-hold input {
  accent-color: var(--primary-bg);
}
.settings-hold select {
  max-width: 230px;
}

.settings-companion {display:flex;flex-direction:column;gap:10px;font-size:13px;}
.settings-companion label {display:flex;align-items:center;gap:8px;}
.settings-companion input {accent-color:var(--accent);}
.settings-tabs { display: flex; gap: 8px; border-bottom: 1px solid var(--border); margin-top: 20px; flex: none; }
.settings-tabs button { border: 0; border-bottom: 2px solid transparent; border-radius: 0; padding: 10px 14px; color: var(--muted); background: transparent; font-size: 13px; }
.settings-tabs button[aria-selected="true"] { color: var(--text); border-bottom-color: var(--accent); }
.settings-unsaved { color: var(--accent); }
.settings-area { padding-top: 20px; overflow-y: auto; min-height: 0; }
.settings-area h2 { font-size: 15px; margin: 0 0 10px; }
.settings-area p { font-size: 12px; line-height: 1.7; }
.settings-group + .settings-group { margin-top: 24px; padding-top: 20px; border-top: 1px solid var(--border); }
.settings-group .muted { margin: 0 0 12px; }
.settings-help { margin-top: 18px; color: var(--muted); font-size: 12px; }
.settings-help summary { cursor: pointer; }
.settings-help p { margin: 8px 0; }
.settings-actions { gap: 10px; flex-wrap: wrap; align-items: center; }
.settings-actions > .muted { margin-left: auto; font-size: 12px; }
.settings-applied { padding-top: 12px; }
.diagnostic-table :is(th, td):not(:first-child) { text-align: right; font-variant-numeric: tabular-nums; }
</style>

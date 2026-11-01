<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { OverlayState, AppInfo } from "../../bindings/leaguepingo/models";
import { errorMessage } from "../presentation";
import { assetVersion } from "../gameAssets";
import type { ThemePreference } from "../composables/useTheme";
import AppIcon from "./AppIcon.vue";
defineProps<{ overlayState?: OverlayState | null; theme: ThemePreference; resolvedTheme: string; themeError: string }>();
defineEmits<{ close: []; theme: [value: ThemePreference]; automation: [] }>();
const areas = [{ id: 'appearance', label: '外观' }, { id: 'game', label: '游戏辅助' }, { id: 'diagnostics', label: '诊断与关于' }];
const area = ref('appearance');
const holdEnabled = ref(true), holdKey = ref('Backquote'), shortcut = ref('Ctrl+Shift+P');
const sidebarEnabled = ref(true), previewEnabled = ref(true), sidebarWidth = ref(320);
const info = ref<AppInfo | null>(null), busy = ref(false), message = ref(''), error = ref('');
const dirty = computed(() => !!info.value && (shortcut.value !== info.value.shortcut || holdEnabled.value !== (info.value.holdEnabled ?? true) || holdKey.value !== (info.value.holdKey || 'Backquote') || sidebarEnabled.value !== (info.value.sidebarEnabled ?? true) || previewEnabled.value !== (info.value.previewEnabled ?? true) || sidebarWidth.value !== (info.value.sidebarWidth || 320)));
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
    message.value = '游戏辅助设置已保存。';
  } catch (reason) { error.value = `${applied ? '部分设置已生效；' : ''}${errorMessage(reason)}`; }
  finally { busy.value = false; }
}
function defaults() { shortcut.value = 'Ctrl+Shift+P'; holdEnabled.value = true; holdKey.value = 'Backquote'; sidebarEnabled.value=true;previewEnabled.value=true;sidebarWidth.value=320; message.value = '已恢复默认选项，保存后生效。'; }
async function exportReport() {
  busy.value = true; error.value = ''; message.value = '';
  try { message.value = `已保存本地诊断：${await Service.ExportDiagnostics()}`; }
  catch (reason) { error.value = errorMessage(reason); }
  finally { busy.value = false; }
}
onMounted(() => { panel.value?.focus(); void load(); });
onUnmounted(() => { alive = false; void nextTick(() => { if (opener instanceof HTMLElement && opener.isConnected) opener.focus({ preventScroll: true }); }); });
</script>
<template>
  <div class="settings-backdrop" @click.self="$emit('close')">
    <section ref="panel" tabindex="-1" class="settings-panel" role="dialog" aria-modal="true" aria-label="设置与诊断" @keydown="trapFocus" @keydown.esc.stop="$emit('close')">
      <header><h1>设置与诊断</h1><button class="icon-button" aria-label="关闭设置" @click="$emit('close')"><AppIcon name="close" :size="18" /></button></header>
      <nav class="settings-tabs" role="tablist" aria-label="设置分类">
        <button v-for="(item, index) in areas" :id="`settings-tab-${item.id}`" :key="item.id" role="tab" :aria-selected="area === item.id" :aria-controls="`settings-area-${item.id}`" :tabindex="area === item.id ? 0 : -1" @click="area = item.id" @keydown="moveArea($event, index)">{{ item.label }}<span v-if="item.id === 'game' && dirty" class="settings-unsaved" aria-label="有未保存设置"> ·</span></button>
      </nav>
      <div :id="`settings-area-${area}`" class="settings-area" role="tabpanel" :aria-labelledby="`settings-tab-${area}`">
        <template v-if="area === 'appearance'">
          <fieldset class="theme-choices">
            <legend>外观主题</legend>
            <label v-for="choice in [{ value: 'light', name: '浅色', caption: '暖纸白 · 活力橙' }, { value: 'dark', name: '深色', caption: '炭灰 · 星夜紫' }, { value: 'system', name: '跟随系统', caption: '随系统外观自动切换' }]" :key="choice.value" :class="{ selected: theme === choice.value }">
              <input type="radio" name="theme" :value="choice.value" :checked="theme === choice.value" @change="$emit('theme', choice.value as ThemePreference)" />
              <span class="theme-swatch" :class="choice.value" aria-hidden="true"><i /><i /><i /></span>
              <strong>{{ choice.name }}</strong><small>{{ choice.caption }}</small>
            </label>
          </fieldset>
          <p class="muted">{{ theme === 'system' ? `当前跟随系统：${resolvedTheme === 'dark' ? '深色' : '浅色'}。` : '选择后立即生效并自动保存。' }}局内快览使用深色。</p>
          <p v-if="themeError" class="quality-warning" role="status">{{ themeError }}</p>
        </template>
        <form v-else-if="area === 'game'" class="settings-game" @submit.prevent="saveGame">
          <section class="settings-group"><h2>自动化</h2><p class="muted">自动接受、赛后跳过点赞、返回与排队已集中在左侧导航的「游戏辅助」页，开关与实时状态都在那里；此处只保留侧栏、快览与快捷键。</p><div class="settings-actions"><button type="button" class="secondary" @click="$emit('automation')">打开游戏辅助页</button></div></section>
          <section class="settings-group"><h2>客户端侧栏与开局快览</h2><p class="muted">侧栏优先靠左，空间不足时换右侧或折叠。加载游戏时自动展示一次，6 秒后收起。</p><div class="settings-companion"><label><input v-model="sidebarEnabled" type="checkbox" :disabled="busy"/>启用客户端侧栏</label><label><input v-model="previewEnabled" type="checkbox" :disabled="busy"/>开局自动展示快览</label><label>侧栏宽度<select v-model.number="sidebarWidth" :disabled="busy" aria-label="侧栏宽度"><option v-for="width in [280,300,320,340,360,380,400,420]" :key="width" :value="width">{{ width }} px</option></select></label></div></section>
          <section class="settings-group"><h2>局内按住快览</h2><p class="muted">加载和对局中，游戏前台按住显示，松开隐藏。鼠标和按键继续交给游戏。</p>
            <p class="muted">只监听已配置的按键，不记录其他输入。不可用时，可使用主窗口或托盘的“显示／收起游戏助手”。</p>
            <div class="settings-hold"><label><input v-model="holdEnabled" type="checkbox" aria-label="启用局内按住快览" :disabled="busy" />启用按住快览</label><select v-model="holdKey" aria-label="局内按住键" :disabled="busy"><option value="Backquote">~ / 反引号（无需 Shift）</option><option v-for="key in ['F8', 'F9', 'F10']" :key="key" :value="key">{{ key }}</option></select></div>
            <p v-if="overlayState?.holdWarning" class="quality-warning" role="status">{{ overlayState.holdWarning }}</p>
          </section>
          <section class="settings-group"><h2>固定快览快捷键</h2><p class="muted">切换可交互的快览窗口，可拖动、置顶或按 Esc 隐藏。</p><select v-model="shortcut" aria-label="快览快捷键" :disabled="busy"><option v-for="letter in 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')" :key="letter" :value="`Ctrl+Shift+${letter}`">Ctrl+Shift+{{ letter }}</option></select></section>
          <p class="settings-applied muted" v-if="info">当前生效：{{ (info.holdEnabled ?? true) ? ((info.holdKey || 'Backquote') === 'Backquote' ? '~' : info.holdKey) : '按住快览已关闭' }} · {{ info.shortcut }}</p>
          <details class="settings-help"><summary>按键冲突与窗口使用</summary><p>如与“只以英雄为目标”等游戏绑定冲突，请更换按住键。建议使用无边框或窗口模式。关闭主窗口会收起到托盘，窗口位置与置顶状态会在正常退出时保存。</p></details>
          <div class="settings-actions"><button type="button" class="secondary" :disabled="busy" @click="defaults">恢复默认</button><span class="muted">{{ dirty ? '有未保存修改' : '设置已同步' }}</span><button class="primary" :disabled="busy || !dirty">{{ busy ? '保存中…' : '保存游戏辅助设置' }}</button></div>
        </form>
        <template v-else>
          <h2>LeaguePingo {{ info?.version ?? '加载中' }} · {{ info?.commit ?? '—' }}</h2>
          <p class="muted">构建 {{ info?.buildTime ?? '—' }} · 游戏资源 {{ assetVersion }}</p>
          <section class="settings-validation" aria-label="国服实机验证状态"><h2>实机验证</h2><p class="muted">以下能力仍属实验性：自动接受、赛后跳过点赞、返回与排队、双方同组标识、英雄攻略、本地战绩缓存与恢复。自动化检查通过后，仍需在实际客户端中逐项核对。</p></section>
          <h2>本地诊断</h2><p class="muted">导出版本、请求汇总与运行状态，便于排查问题。文件不含玩家身份、战绩或鉴权信息，由你选择是否分享。</p>
          <div v-if="info" class="runtime-summary"><span>Go 堆 {{ (info.goHeapBytes / 1048576).toFixed(1) }} MiB</span><span>协程 {{ info.goroutines }}</span><small>{{ info.performanceScope }}</small></div>
          <table v-if="metrics.length" class="diagnostic-table"><thead><tr><th>类别</th><th>请求 / 失败</th><th>平均 / 最慢 ms</th></tr></thead><tbody><tr v-for="metric in metrics" :key="metric.category"><td>{{ metric.category }}</td><td>{{ metric.calls }} / {{ metric.failed }}</td><td>{{ metric.calls ? Math.round(metric.totalMs / metric.calls) : '—' }} / {{ metric.maxMs }}</td></tr></tbody></table>
          <div class="settings-actions"><button class="secondary" :disabled="busy" @click="load">刷新诊断</button><button class="secondary" :disabled="busy" @click="exportReport">导出脱敏诊断</button></div>
        </template>
        <p v-if="info?.settingsWarning" class="quality-warning" role="status">{{ info.settingsWarning }}</p>
        <p v-if="error" class="error-banner" role="alert">{{ error }}</p><p v-if="message" class="settings-message" role="status">{{ message }}</p>
      </div>
    </section>
  </div>
</template>
<style scoped>
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

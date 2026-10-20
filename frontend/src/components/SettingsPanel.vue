<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { OverlayState, AppInfo } from "../../bindings/leaguepingo/models";
import { errorMessage } from "../presentation";
import { assetVersion } from "../gameAssets";
import type { ThemePreference } from "../composables/useTheme";
defineProps<{ overlayState?: OverlayState | null; theme: ThemePreference; resolvedTheme: string; themeError: string }>();
defineEmits<{ close: []; theme: [value: ThemePreference] }>();
const holdEnabled = ref(true),
  holdKey = ref("Backquote");
const info = ref<AppInfo | null>(null),
  shortcut = ref("Ctrl+Shift+P"),
  busy = ref(false),
  message = ref(""),
  error = ref("");
const metrics = computed(() =>
  Object.entries(info.value?.requests ?? {}).flatMap(([category, metric]) =>
    metric ? [{ category, ...metric }] : [],
  ),
);
const panel = ref<HTMLElement | null>(null);
const opener = document.activeElement;
function trapFocus(event: KeyboardEvent) {
  if (event.key !== "Tab" || !panel.value) return;
  const controls = Array.from(
    panel.value.querySelectorAll<HTMLElement>(
      'button:not(:disabled), select:not(:disabled), input:not(:disabled), [tabindex="0"]',
    ),
  );
  const first = controls[0],
    last = controls[controls.length - 1];
  if (
    event.shiftKey &&
    (document.activeElement === first || document.activeElement === panel.value)
  ) {
    event.preventDefault();
    last?.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first?.focus();
  }
}
async function load() {
  try {
    info.value = await Service.Info();
    shortcut.value = info.value.shortcut;
    holdEnabled.value = info.value.holdEnabled ?? true;
    holdKey.value = info.value.holdKey || "Backquote";
  } catch (reason) {
    error.value = errorMessage(reason);
  }
}
async function save() {
  busy.value = true;
  error.value = "";
  message.value = "";
  try {
    info.value = await Service.ConfigureShortcut(shortcut.value);
    message.value = "快捷键已保存；窗口位置和置顶状态也会记住。";
  } catch (reason) {
    error.value = errorMessage(reason);
  } finally {
    busy.value = false;
  }
}
async function saveHold() {
  busy.value = true;
  error.value = "";
  message.value = "";
  try {
    info.value = await Service.ConfigureHold(holdEnabled.value, holdKey.value);
    message.value = "局内按住快览设置已保存。";
  } catch (reason) {
    error.value = errorMessage(reason);
  } finally {
    busy.value = false;
  }
}
async function exportReport() {
  busy.value = true;
  error.value = "";
  message.value = "";
  try {
    message.value = `已保存本地诊断：${await Service.ExportDiagnostics()}`;
  } catch (reason) {
    error.value = errorMessage(reason);
  } finally {
    busy.value = false;
  }
}
onMounted(() => {
  panel.value?.focus();
  void load();
});
onUnmounted(() => {
  // Wait until the parent removes inert from the underlying workspace.
  void nextTick(() => {
    if (opener instanceof HTMLElement && opener.isConnected)
      opener.focus({ preventScroll: true });
  });
});
</script>
<template>
  <div class="settings-backdrop" @click.self="$emit('close')">
    <section
      ref="panel"
      tabindex="-1"
      class="settings-panel"
      role="dialog"
      aria-modal="true"
      aria-label="设置与诊断"
      @keydown="trapFocus"
      @keydown.esc.stop="$emit('close')"
    >
      <header>
        <h1>设置与诊断</h1>
        <button
          class="icon-button"
          aria-label="关闭设置"
          @click="$emit('close')"
        >
          ×
        </button>
      </header>
      <p class="muted">
        LeaguePingo {{ info?.version ?? "加载中" }} · {{ info?.commit ?? "—"
        }}<br />构建 {{ info?.buildTime ?? "—" }} · 游戏资源 {{ assetVersion }}
      </p>
      <fieldset class="theme-choices">
        <legend>外观主题</legend>
        <label v-for="choice in [{ value: 'light', name: '浅色', caption: '暖纸白 · 活力橙' }, { value: 'dark', name: '深色', caption: '炭灰 · 星夜紫' }, { value: 'system', name: '跟随系统', caption: '随系统外观自动切换' }]" :key="choice.value" :class="{ selected: theme === choice.value }">
          <input type="radio" name="theme" :value="choice.value" :checked="theme === choice.value" @change="$emit('theme', choice.value as ThemePreference)" />
          <span class="theme-swatch" :class="choice.value" aria-hidden="true"><i /><i /><i /></span>
          <strong>{{ choice.name }}</strong><small>{{ choice.caption }}</small>
        </label>
      </fieldset>
      <p class="muted">{{ theme === 'system' ? `当前跟随系统：${resolvedTheme === 'dark' ? '深色' : '浅色'}。` : '选择后立即生效并自动保存。' }}局内快览保持深色，方便对局中阅读。</p>
      <p v-if="themeError" class="quality-warning" role="status">{{ themeError }}</p>
      <h2>局内按住快览</h2>
      <form class="settings-hold" @submit.prevent="saveHold">
        <label
          ><input
            v-model="holdEnabled"
            type="checkbox"
            aria-label="启用局内按住快览"
          />启用按住显示、松开隐藏</label
        ><select v-model="holdKey" aria-label="局内按住键">
          <option value="Backquote">~ / 反引号（无需 Shift）</option>
          <option v-for="key in ['F8', 'F9', 'F10']" :key="key" :value="key">
            {{ key }}
          </option></select
        ><button class="primary" :disabled="busy">保存按住设置</button>
      </form>
      <p class="muted">
        仅在正式对局、游戏窗口处于前台时生效。临时快览不抢焦点，鼠标和按键继续传给游戏；切出游戏会隐藏。若与“只以英雄为目标”等游戏按键冲突，请在此改键。建议使用无边框或窗口模式，独占全屏兼容性待实机验证。
      </p>
      <p v-if="overlayState?.holdWarning" class="quality-warning">
        {{ overlayState.holdWarning }}
      </p>
      <h2>手动快览快捷键</h2>
      <form class="settings-shortcut" @submit.prevent="save">
        <select v-model="shortcut" aria-label="快览快捷键">
          <option
            v-for="letter in 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')"
            :key="letter"
            :value="`Ctrl+Shift+${letter}`"
          >
            Ctrl+Shift+{{ letter }}
          </option></select
        ><button class="primary" :disabled="busy">保存快捷键</button>
      </form>
      <p class="muted">
        注册冲突时保留原快捷键，不抢占其他程序。主窗口和快览的位置/大小、置顶状态会在正常退出时保存。关闭主窗口仅隐藏到托盘。
      </p>
      <p v-if="info?.settingsWarning" class="quality-warning">
        {{ info.settingsWarning }}
      </p>
      <h2>本地诊断</h2>
      <p class="muted">
        只包含版本、汇总请求计数/耗时/状态码和 Go 运行时数据；不包含玩家
        ID、战绩、鉴权信息或原始响应。不自动上传。
      </p>
      <div v-if="info" class="runtime-summary">
        <span>Go 堆 {{ (info.goHeapBytes / 1048576).toFixed(1) }} MiB</span
        ><span>协程 {{ info.goroutines }}</span
        ><small>{{ info.performanceScope }}</small>
      </div>
      <table v-if="metrics.length" class="diagnostic-table">
        <thead>
          <tr>
            <th>类别</th>
            <th>请求 / 失败</th>
            <th>平均 / 最慢 ms</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="metric in metrics" :key="metric.category">
            <td>{{ metric.category }}</td>
            <td>{{ metric.calls }} / {{ metric.failed }}</td>
            <td>
              {{
                metric.calls ? Math.round(metric.totalMs / metric.calls) : "—"
              }}
              / {{ metric.maxMs }}
            </td>
          </tr>
        </tbody>
      </table>
      <div class="settings-actions">
        <button class="secondary" :disabled="busy" @click="load">
          刷新诊断</button
        ><button class="secondary" :disabled="busy" @click="exportReport">
          导出脱敏诊断
        </button>
      </div>
      <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
      <p v-if="message" class="settings-message" role="status">{{ message }}</p>
      <p class="muted">
        性能结论需包含 WebView2 进程树和真实游戏对照。此处不显示推测的
        FPS、延迟或“零开销”。
      </p>
    </section>
  </div>
</template>

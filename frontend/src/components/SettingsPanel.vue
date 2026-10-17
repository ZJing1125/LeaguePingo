<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { AppInfo } from "../../bindings/leaguepingo/models";
import { errorMessage } from "../presentation";
import { assetVersion } from "../gameAssets";
defineEmits<{ close: [] }>();
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
function trapFocus(event: KeyboardEvent) {
  if (event.key !== "Tab" || !panel.value) return;
  const controls = Array.from(
    panel.value.querySelectorAll<HTMLElement>(
      'button:not(:disabled), select, [tabindex="0"]',
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
      <h2>快览快捷键</h2>
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

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { Events, Window } from "@wailsio/runtime";
import AppIcon from "./AppIcon.vue";

defineProps<{ modalOpen: boolean }>();
const maximised = ref(false);
const focused = ref(true);
const busy = ref(false);
const error = ref("");
const subscriptions: (() => void)[] = [];
let alive = true;
let revision = 0;
let focusRevision = 0;

async function syncMaximised() {
  const request = ++revision;
  try {
    const value = await Window.IsMaximised();
    if (alive && request === revision) maximised.value = value === true;
  } catch {
    // The next native state event can still update the controls.
  }
}

async function control(action: "Minimise" | "ToggleMaximise" | "Close") {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    // Close follows the Go WindowClosing hook, including close-to-tray.
    await Window[action]();
    if (action === "ToggleMaximise") await syncMaximised();
  } catch {
    if (alive) error.value = "窗口操作未完成，请重试";
  } finally {
    if (alive) busy.value = false;
  }
}

function doubleClick(event: MouseEvent) {
  if ((event.target as Element).closest("button")) return;
  void control("ToggleMaximise");
}

onMounted(() => {
  const common = Events.Types.Common;
  function listen(name: string, update: () => void) {
    subscriptions.push(
      Events.On(name, (event) => {
        if (!event.sender || event.sender === "main") update();
      }),
    );
  }
  listen(common.WindowMaximise, () => {
    revision++;
    maximised.value = true;
  });
  listen(common.WindowUnMaximise, () => {
    revision++;
    maximised.value = false;
  });
  listen(common.WindowRestore, () => {
    void syncMaximised();
  });
  listen(common.WindowFocus, () => {
    focusRevision++;
    focused.value = true;
    void syncMaximised();
  });
  listen(common.WindowLostFocus, () => {
    focusRevision++;
    focused.value = false;
  });
  void syncMaximised();
  void Window.IsFocused()
    .then((value) => {
      if (alive && focusRevision === 0) focused.value = value === true;
    })
    .catch(() => {});
});
onUnmounted(() => {
  alive = false;
  revision++;
  subscriptions.forEach((stop) => stop());
});
</script>

<template>
  <header
    class="desktop-titlebar"
    :class="{ 'window-inactive': !focused, 'window-maximised': maximised }"
    aria-label="LeaguePingo 窗口标题栏"
    @dblclick="doubleClick"
  >
    <div class="window-brand">
      <span class="brand-mark" aria-hidden="true">P</span>
      <strong>LeaguePingo</strong>
      <span class="window-subtitle">英雄联盟助手</span>
    </div>
    <span v-if="error" class="window-control-error" role="status">{{
      error
    }}</span>
    <div
      class="window-controls"
      role="group"
      aria-label="窗口控制"
      :aria-busy="busy"
    >
      <button
        type="button"
        aria-label="最小化窗口"
        title="最小化"
        :tabindex="modalOpen ? -1 : 0"
        :aria-disabled="busy"
        @mousedown.prevent
        @click="control('Minimise')"
      >
        <AppIcon name="minimise" :size="16" />
      </button>
      <button
        type="button"
        :aria-label="maximised ? '还原窗口' : '最大化窗口'"
        :title="maximised ? '还原' : '最大化'"
        :tabindex="modalOpen ? -1 : 0"
        :aria-disabled="busy"
        @mousedown.prevent
        @click="control('ToggleMaximise')"
      >
        <AppIcon :name="maximised ? 'restore' : 'maximise'" :size="16" />
      </button>
      <button
        type="button"
        class="window-close"
        aria-label="关闭窗口并收起到托盘"
        title="关闭到托盘 · 可从托盘重新打开"
        :tabindex="modalOpen ? -1 : 0"
        :aria-disabled="busy"
        @mousedown.prevent
        @click="control('Close')"
      >
        <AppIcon name="close" :size="17" />
      </button>
    </div>
  </header>
</template>

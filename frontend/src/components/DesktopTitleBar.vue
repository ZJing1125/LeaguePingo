<script setup lang="ts">
import { uiText } from "../uiText";
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
    if (alive) error.value = uiText.common.actionFailed;
  } finally {
    if (alive) busy.value = false;
  }
}

function doubleClick(event: MouseEvent) {
  if ((event.target as Element).closest("button, input, select, textarea, a, form, nav, [contenteditable='true']")) return;
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
    class="desktop-titlebar app-toolbar"
    :class="{ 'window-inactive': !focused, 'window-maximised': maximised }"
    :aria-label="uiText.desktopTitleBar.title"
    @dblclick="doubleClick"
  >
    <div class="toolbar-content" :inert="modalOpen">
      <slot />
    </div>
    <span v-if="error" class="window-control-error" role="status">{{
      error
    }}</span>
    <div
      class="window-controls"
      role="group"
      :aria-label="uiText.desktopTitleBar.controls"
      :aria-busy="busy"
    >
      <button
        type="button"
        :aria-label="uiText.desktopTitleBar.minimizeLabel"
        :title="uiText.desktopTitleBar.minimize"
        :tabindex="modalOpen ? -1 : 0"
        :aria-disabled="busy"
        @mousedown.prevent
        @click="control('Minimise')"
      >
        <AppIcon name="minimise" :size="16" />
      </button>
      <button
        type="button"
        :aria-label="maximised ? uiText.desktopTitleBar.restoreLabel : uiText.desktopTitleBar.maximizeLabel"
        :title="maximised ? uiText.desktopTitleBar.restore : uiText.desktopTitleBar.maximize"
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
        :aria-label="uiText.desktopTitleBar.closeLabel"
        :title="uiText.desktopTitleBar.closeHint"
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
<style scoped>
/* Retain original global import order; shared controls and ancestor contexts stay in effect. */
.window-control-error {
  margin-left: 20px;
  color: var(--loss);
  font-size: 11px;
}
.window-controls {
  display: flex;
  align-self: stretch;
  margin-left: auto;
  --wails-draggable: no-drag;
}
.window-controls button {
  width: 46px;
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: var(--muted);
}
.window-controls button:hover:not(:disabled) {
  background: var(--raised);
  color: var(--text);
}
.window-controls .window-close:hover:not(:disabled) {
  background: #c44459;
  color: #fff;
}
.window-controls button:focus-visible {
  outline-offset: -4px;
}
.window-controls button[aria-disabled="true"] {
  opacity: 0.45;
  cursor: wait;
}
.window-controls .app-icon {
  pointer-events: none;
}
:is(.desktop-app, .drawer-floating) .window-controls { gap: 3px; padding: 3px 6px; align-items: center; }
:is(.desktop-app, .drawer-floating) .window-controls button { position: relative; width: 38px; height: 28px; min-height: 0; border: 1px solid transparent; border-radius: 6px; color: var(--muted); background: transparent; padding: 0; }
:is(.desktop-app, .drawer-floating) .window-controls button:hover:not(:disabled) { color: var(--text); background: color-mix(in srgb, var(--accent) 13%, transparent); border-color: color-mix(in srgb, var(--accent) 22%, transparent); }
:is(.desktop-app, .drawer-floating) .window-controls button:active:not(:disabled) { background: color-mix(in srgb, var(--accent) 25%, transparent); }
:is(.desktop-app, .drawer-floating) .window-controls .window-close:hover:not(:disabled) { background: #d85e6d; border-color: #d85e6d; color: #fff; }
:is(.desktop-app, .drawer-floating) .window-controls .app-icon { width: 14px; height: 14px; }
.window-controls button {
  color: var(--muted);
}
.window-controls button:hover {
  background: var(--hover);
}
.window-controls .window-close:hover {
  background: #c84040;
  color: white;
}
.desktop-titlebar .window-controls {
  flex: none;
  margin-left: 0;
}
.desktop-titlebar .window-control-error {
  position: absolute;
  top: calc(100% + 6px);
  right: 12px;
  z-index: 30;
  margin: 0;
  padding: 8px 12px;
  max-width: min(340px, calc(100vw - 24px));
  border: 1px solid var(--warning-border);
  border-radius: 6px;
  background: var(--warning-bg);
  color: var(--warning-text);
  --wails-draggable: no-drag;
}
</style>

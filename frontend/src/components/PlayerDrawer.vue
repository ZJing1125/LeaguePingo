<script setup lang="ts">
import { uiText } from "../uiText";
import { inject, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { playerHistoryNavigation } from "../playerHistoryNavigation";
import TransientNotice from "./TransientNotice.vue";

const navigation = inject(playerHistoryNavigation, null);
const props = defineProps<{
  modal: boolean;
  active: boolean;
  playerKey: string;
  label?: string;
  fit?: boolean;
}>();
const emit = defineEmits<{ close: [] }>();
const panel = ref<HTMLElement | null>(null);
let opener: HTMLElement | null =
  document.activeElement instanceof HTMLElement ? document.activeElement : null;
let disposed = false;

function focusable() {
  return Array.from(
    panel.value?.querySelectorAll<HTMLElement>(
      'button:not(:disabled), select:not(:disabled), input:not(:disabled), summary, [tabindex="0"], a[href]',
    ) ?? [],
  ).filter(
    (control) => control.getClientRects().length && !control.closest("[inert]"),
  );
}
function trapFocus(event: KeyboardEvent) {
  if (!props.modal || event.key !== "Tab") return;
  const controls = focusable();
  const first = controls[0],
    last = controls.at(-1);
  if (!first || !last) {
    event.preventDefault();
    panel.value?.focus();
  } else if (
    event.shiftKey &&
    (document.activeElement === first || document.activeElement === panel.value)
  ) {
    event.preventDefault();
    last.focus();
  } else if (
    !event.shiftKey &&
    (document.activeElement === last || document.activeElement === panel.value)
  ) {
    event.preventDefault();
    first.focus();
  }
}
function restoreFocus() {
  void nextTick(() => {
    const selectedRow = Array.from(
      document.querySelectorAll<HTMLElement>(".player-row"),
    ).find((row) => row.dataset.playerKey === props.playerKey);
    const target = [
      selectedRow?.querySelector<HTMLElement>(".player-row-preview") ?? selectedRow,
      opener,
      document.querySelector<HTMLElement>(
        '.players-workspace [role="tab"][aria-selected="true"]',
      ),
      document.querySelector<HTMLElement>(".roster-toolbar button"),
      document.querySelector<HTMLElement>(uiText.playerDrawer.playerIdSelector),
    ].find(
      (element) =>
        element?.isConnected &&
        element.getClientRects().length &&
        !element.closest("[inert]") &&
        !element.matches(":disabled"),
    );
    target?.focus({ preventScroll: true });
  });
}
watch(
  () => [props.active, props.modal] as const,
  async ([active, modal], previous) => {
    if (!active) {
      if (previous?.[0]) restoreFocus();
      return;
    }
    if (!previous?.[0] && document.activeElement instanceof HTMLElement)
      opener = document.activeElement;
    // Inline details leave focus on the roster so players can be compared quickly.
    if (modal) {
      await nextTick();
      if (
        !disposed &&
        props.active &&
        props.modal &&
        !panel.value?.contains(document.activeElement)
      )
        panel.value?.focus({ preventScroll: true });
    } else if (previous?.[1]) {
      restoreFocus();
    }
  },
  { immediate: true, flush: "post" },
);
onBeforeUnmount(() => {
  disposed = true;
  restoreFocus();
});
</script>
<template>
  <Teleport to="body" :disabled="!modal">
    <div
      v-show="active"
      class="drawer-shell"
      :class="{ 'drawer-floating': modal }"
      @click.self="modal && emit('close')"
    >
      <aside
        ref="panel"
        class="player-drawer"
        :class="{'drawer-fit':fit}"
        :role="modal ? 'dialog' : 'complementary'"
        :aria-modal="modal ? true : undefined"
        :aria-label="label || uiText.playerDrawer.title"
        tabindex="-1"
        @keydown="trapFocus"
        @keydown.esc.stop="emit('close')"
      >
        <TransientNotice v-if="modal && active && navigation" :message="navigation.error.value" banner-class="quality-warning"/>
        <slot />
      </aside>
    </div>
  </Teleport>
</template>
<style scoped>
/* Scope inside :where and retain original specificity with :is; child boundaries use :deep. */
:where(.drawer-shell):is(.drawer-shell) {
  display: contents;
}
:where(.player-drawer):is(.player-drawer) {
  width: 400px;
  min-width: 0;
  flex: none;
  display: flex;
  flex-direction: column;
  background: #141820;
  border: 1px solid var(--border);
  border-bottom: 0;
  border-radius: 9px 9px 0 0;
  overflow: hidden;
}
:where(.drawer-floating):is(.drawer-floating) {
  position: fixed;
  inset: var(--titlebar-height) 0 0;
  z-index: 30;
  display: flex;
  justify-content: flex-end;
  padding: 12px;
  background: #05090db3;
}
:where(.player-drawer):is(.drawer-floating .player-drawer) {
  width: min(440px, 100%);
  height: 100%;
  border-bottom: 1px solid var(--border);
  border-radius: 10px;
  box-shadow: -15px 0 40px #0007;
}
:where(.player-drawer):is(.player-drawer) :deep(>.quality-warning) {
  margin: 8px;
}
@media (max-width: 620px) {
  :where(.drawer-floating):is(.drawer-floating) {
    padding: 8px;
  }
}
:where(.drawer-shell):is(.drawer-shell) {
  display: contents;
}
:where(.player-drawer):is(.player-drawer) {
  width: 100%;
  height: 212px;
  flex: none;
  background: var(--raised);
  border: 1px solid var(--border);
  border-radius: 0;
  box-shadow: none;
}
:where(.player-drawer):is(.player-drawer) :deep(>.quality-warning) {
  margin: 2px 10px;
  padding: 2px 6px;
  font-size: 10px;
}
:where(.player-drawer):is(.player-drawer) :deep(.profile-update) {
  font-size: 9px;
}
:where(.drawer-floating):is(.drawer-floating) {
  inset: var(--titlebar-height) 0 0;
  background: #1c2b2e88;
  padding: 18px;
  align-items: flex-end;
  justify-content: center;
  display: flex;
}
:where(.player-drawer):is(.drawer-floating .player-drawer) {
  width: 100%;
  height: min(450px, 85%);
  border: 1px solid var(--border);
  border-radius: 3px;
  background: var(--surface);
}
@media (min-width: 1000px) and (max-height: 780px) {
  :where(.player-drawer):is(.player-drawer) {
    height: 188px;
  }
}
:where(.drawer-floating):is(.drawer-floating) {
  inset: var(--titlebar-height) 0 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: #15252b80;
  backdrop-filter: blur(3px);
}
:where(.player-drawer):is(.drawer-floating .player-drawer) {
  width: min(1100px, 100%);
  height: min(640px, 100%);
  border-radius: 10px;
  box-shadow: 0 24px 70px #0c171840;
  overflow: hidden;
}
:where(.drawer-floating):is(.drawer-floating) :deep(.drawer-toolbar) {
  height: 42px;
  min-height: 42px;
  padding: 5px 14px;
  display: flex;
  align-items: center;
  gap: 10px;
}
:where(.drawer-floating):is(.drawer-floating) :deep(.drawer-toolbar button) {
  font-size: 12px;
}
:where(.drawer-floating):is(.drawer-floating) :deep(.profile-freshness) {
  left: 16px;
  right: auto;
  max-width: 220px;
  top: auto;
  bottom: 12px;
  font-size: 11px;
}
@media (max-width: 1000px) {
  :where(.drawer-floating):is(.drawer-floating) {
    padding: 14px;
  }
}
</style>
<style scoped>
.drawer-floating .player-drawer.drawer-fit {height:auto;min-height:0;max-height:calc(100dvh - var(--titlebar-height) - 48px);}
</style>

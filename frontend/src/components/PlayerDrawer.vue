<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from "vue";
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
      selectedRow,
      opener,
      document.querySelector<HTMLElement>(
        '.players-workspace [role="tab"][aria-selected="true"]',
      ),
      document.querySelector<HTMLElement>(".roster-toolbar button"),
      document.querySelector<HTMLElement>('[aria-label="玩家完整 ID"]'),
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
        :aria-label="label || '玩家详情'"
        tabindex="-1"
        @keydown="trapFocus"
        @keydown.esc.stop="emit('close')"
      >
        <slot />
      </aside>
    </div>
  </Teleport>
</template>
<style scoped>
.drawer-floating .player-drawer.drawer-fit {height:auto;min-height:0;max-height:calc(100dvh - var(--titlebar-height) - 48px);}
</style>

import { ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { OverlayState } from "../../bindings/leaguepingo/models";
import { errorMessage } from "../presentation";

export function useOverlayControls() {
  const overlay = ref<OverlayState | null>(null);
  const overlayBusy = ref(false);
  const overlayError = ref("");
  function acceptOverlay(state: OverlayState) {
    if (!overlay.value || state.revision >= overlay.value.revision)
      overlay.value = state;
  }
  async function control(action: () => Promise<OverlayState>) {
    if (overlayBusy.value) return;
    overlayBusy.value = true;
    overlayError.value = "";
    try {
      acceptOverlay(await action());
    } catch (reason) {
      overlayError.value = errorMessage(reason);
    } finally {
      overlayBusy.value = false;
    }
  }
  function hideOverlay() {
    return control(() => Service.HideOverlay());
  }
  function pinOverlay() {
    return control(() => Service.SetOverlayPinned(!overlay.value?.pinned));
  }
  async function toggleOverlay() {
    try {
      await Service.ToggleOverlay();
    } catch (reason) {
      overlayError.value = errorMessage(reason);
    }
  }

  return {
    overlay,
    overlayBusy,
    overlayError,
    acceptOverlay,
    hideOverlay,
    pinOverlay,
    toggleOverlay,
  };
}

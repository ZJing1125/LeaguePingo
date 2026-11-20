import { nextTick } from "vue";
import * as Service from "../bindings/leaguepingo/appservice";

export type ResponseCategory = "player-history-ready" | "history-feedback" | "history-cache-ready" | "history-first-ready" | "rating-render" | "guide-render";

// Two animation frames place the measurement after a visible DOM update.
// This is a local scheduling measurement; Windows presentation is checked on-device.
export function recordVisibleResponse(category: ResponseCategory, started: number, valid: () => boolean) {
  if (typeof document === "undefined" || document.visibilityState !== "visible") return;
  void nextTick().then(() => requestAnimationFrame(() => requestAnimationFrame(() => {
    if (document.visibilityState !== "visible" || !valid()) return;
    void Service.RecordUIResponse?.(category, performance.now() - started).catch(() => {});
  })));
}

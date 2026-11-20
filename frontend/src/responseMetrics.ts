import { nextTick } from "vue";
import * as Service from "../bindings/leaguepingo/appservice";

export type ResponseCategory = "player-history-ready" | "history-feedback" | "history-cache-ready" | "history-first-ready" | "rating-render" | "guide-render";

// Two animation frames place the measurement after a visible DOM update.
// This is a local scheduling measurement; Windows presentation is checked on-device.
export function recordVisibleResponse(category: ResponseCategory | null, started: number, valid: () => boolean, responseID = 0) {
  const received = performance.now();
  if (typeof document === "undefined" || document.visibilityState !== "visible") return;
  void nextTick().then(() => requestAnimationFrame(() => requestAnimationFrame(() => {
    if (document.visibilityState !== "visible" || !valid()) return;
    if (responseID) void Service.AcknowledgeResponse?.(responseID, performance.now() - received).catch(() => {});
    if (category) void Service.RecordUIResponse?.(category, performance.now() - started).catch(() => {});
  })));
}

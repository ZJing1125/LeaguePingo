import type { InjectionKey, Ref } from "vue";
import type { Participant } from "../bindings/leaguepingo/internal/core/models";
import { normalizeRiotIDInput } from "./presentation";

export type PlayerHistoryTarget = { connectionId: number; label: string } & (
  { kind: "riotId"; riotId: string } | { kind: "livePlayer"; key: string }
);

export const playerHistoryNavigation: InjectionKey<{
  available: (target?: PlayerHistoryTarget) => target is PlayerHistoryTarget;
  open: (target: PlayerHistoryTarget) => Promise<void>;
  busy: Readonly<Ref<boolean>>;
  error: Readonly<Ref<string>>;
}> = Symbol("player history navigation");

export function completeRiotID(value?: string | null) {
  if (!value) return "";
  const id = normalizeRiotIDInput(value);
  return /^[^#]+#[^#]+$/.test(id) && !/[\u0000-\u001f\u007f-\u009f]/.test(id) ? id : "";
}

export function historyTarget(displayName: string, connectionId?: number): PlayerHistoryTarget | undefined {
  const riotId = completeRiotID(displayName);
  if (riotId && connectionId != null) return { kind: "riotId", riotId, label: riotId, connectionId };
}

export function roomHistoryTarget(player: Participant, cached: boolean): PlayerHistoryTarget | undefined {
  if (player.identityState !== "known") return;
  if (!cached && !player.stale && player.key) {
    return { kind: "livePlayer", key: player.key, label: player.displayName, connectionId: player.connectionId };
  }
  return historyTarget(player.displayName, player.connectionId);
}

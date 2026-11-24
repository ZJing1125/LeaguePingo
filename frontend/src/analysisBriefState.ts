import { ref } from "vue";
import type { AnalysisBriefState } from "../bindings/leaguepingo/internal/core/models";
import type { AppInfo } from "../bindings/leaguepingo/models";
export const brief = ref<AnalysisBriefState | null>(null);
export function applyAnalysisBrief(state?: AnalysisBriefState | null) { brief.value = state ?? null; }
export function applyBriefConfiguration(info: AppInfo) {
  brief.value = {cachedSeats:brief.value?.cachedSeats ?? 0,readySeats:brief.value?.readySeats ?? 0,totalSeats:brief.value?.totalSeats ?? 0,freezeMs:brief.value?.freezeMs ?? 0,queuedToPostMs:brief.value?.queuedToPostMs ?? 0,postMs:brief.value?.postMs ?? 0,enabled: info.autoAnalysisBrief ?? false, audience: info.analysisAudience ?? 'team', includeSelf: info.analysisIncludeSelf ?? true, status: info.autoAnalysisBrief ? brief.value?.enabled ? brief.value.status : 'idle' : 'off', updatedAt: brief.value?.updatedAt ?? ''};
}

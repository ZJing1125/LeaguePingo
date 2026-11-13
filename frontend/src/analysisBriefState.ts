import { ref } from "vue";
import type { AnalysisBriefState } from "../bindings/leaguepingo/internal/core/models";
import type { AppInfo } from "../bindings/leaguepingo/models";
export const brief = ref<AnalysisBriefState | null>(null);
export function applyAnalysisBrief(state?: AnalysisBriefState | null) { brief.value = state ?? null; }
export function applyBriefConfiguration(info: AppInfo) {
  brief.value = {enabled: info.autoAnalysisBrief ?? false, audience: info.analysisAudience ?? 'team', includeSelf: info.analysisIncludeSelf ?? true, status: info.autoAnalysisBrief ? brief.value?.enabled ? brief.value.status : 'idle' : 'off', updatedAt: brief.value?.updatedAt ?? ''};
}

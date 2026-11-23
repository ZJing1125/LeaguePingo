<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { AppInfo } from "../../bindings/leaguepingo/models";
import { errorMessage } from "../presentation";
import { applyBriefConfiguration, useAutomation } from "../composables/useAutomation";
import AppIcon from "./AppIcon.vue";
import ExperimentalBadge from "./ExperimentalBadge.vue";
import { uiText } from "../uiText";

const emit = defineEmits<{ settings: [] }>();
const info = ref<AppInfo | null>(null);
const busy = ref(false);
const loading = ref(true);
const unavailable = computed(() => loading.value || busy.value || !info.value);
const text = uiText.automationState;
const message = ref("");
const error = ref("");
const acceptEnabled = ref(false), acceptDelay = ref(1);
const briefEnabled = ref(false), briefAudience = ref("team"), briefSelf = ref(true), briefPreview = ref("");
const skipHonor = ref(false), returnEnabled = ref(false), queueEnabled = ref(false), queueDelay = ref(10);
const live = useAutomation();
const { acceptText, postText, acceptVisible, postVisible, accept, post, skipAccept, cancelPost, error: liveError, activeCount, warning } = live;

type AutomationDraft = {
  briefEnabled: boolean;
  briefAudience: string;
  briefSelf: boolean;
  acceptEnabled: boolean;
  acceptDelay: number;
  skipHonor: boolean;
  returnEnabled: boolean;
  queueEnabled: boolean;
  queueDelay: number;
};

function apply(next: AppInfo) {
  info.value = next;
  applyBriefConfiguration(next);
  briefEnabled.value = next.autoAnalysisBrief ?? false;
  briefAudience.value = next.analysisAudience ?? 'team';
  briefSelf.value = next.analysisIncludeSelf ?? true;
  acceptEnabled.value = next.autoAcceptEnabled ?? false;
  acceptDelay.value = next.autoAcceptDelay ?? 1;
  skipHonor.value = next.autoSkipHonor ?? false;
  returnEnabled.value = next.autoReturnEnabled ?? false;
  queueEnabled.value = next.autoQueueEnabled ?? false;
  queueDelay.value = next.autoQueueDelay ?? 10;
}
function currentDraft(): AutomationDraft {
  return {
    briefEnabled: briefEnabled.value, briefAudience: briefAudience.value, briefSelf: briefSelf.value,
    acceptEnabled: acceptEnabled.value,
    acceptDelay: acceptDelay.value,
    skipHonor: skipHonor.value,
    returnEnabled: returnEnabled.value,
    queueEnabled: queueEnabled.value,
    queueDelay: queueDelay.value,
  };
}
function acceptChanged(target: AutomationDraft, current: AppInfo) {
  return target.acceptEnabled !== (current.autoAcceptEnabled ?? false) ||
    target.acceptDelay !== (current.autoAcceptDelay ?? 1);
}
function postChanged(target: AutomationDraft, current: AppInfo) {
  return target.skipHonor !== (current.autoSkipHonor ?? false) ||
    target.returnEnabled !== (current.autoReturnEnabled ?? false) ||
    target.queueEnabled !== (current.autoQueueEnabled ?? false) ||
    target.queueDelay !== (current.autoQueueDelay ?? 10);
}
function briefChanged(target: AutomationDraft, current: AppInfo) {
  return target.briefEnabled !== (current.autoAnalysisBrief ?? false) || target.briefAudience !== (current.analysisAudience ?? 'team') || target.briefSelf !== (current.analysisIncludeSelf ?? true);
}
const briefStatus = computed(() => {
  if (!info.value) return "";
  if (!briefChanged(currentDraft(), info.value)) return live.briefText.value;
  if (info.value.autoAnalysisBrief) return uiText.analysisBrief.afterSave;
  return briefEnabled.value ? uiText.analysisBrief.enableAfterSave : "";
});
let previewRevision = 0;
watch([briefEnabled, briefSelf, briefAudience], () => { briefPreview.value = ""; previewRevision++; });
const dirty = computed(() => !!info.value && (
  briefChanged(currentDraft(), info.value) ||
  acceptEnabled.value !== (info.value.autoAcceptEnabled ?? false) ||
  acceptDelay.value !== (info.value.autoAcceptDelay ?? 1) ||
  skipHonor.value !== (info.value.autoSkipHonor ?? false) ||
  returnEnabled.value !== (info.value.autoReturnEnabled ?? false) ||
  queueEnabled.value !== (info.value.autoQueueEnabled ?? false) ||
  queueDelay.value !== (info.value.autoQueueDelay ?? 10)
));
const anyEnabled = computed(() => !!info.value && (
  briefEnabled.value || (info.value.autoAnalysisBrief ?? false) || acceptEnabled.value || skipHonor.value || returnEnabled.value || queueEnabled.value ||
  (info.value.autoAcceptEnabled ?? false) || (info.value.autoSkipHonor ?? false) ||
  (info.value.autoReturnEnabled ?? false) || (info.value.autoQueueEnabled ?? false)
));
async function load() {
  loading.value = true; error.value = "";
  try { apply(await Service.Info()); }
  catch (reason) { error.value = errorMessage(reason); }
  finally { loading.value = false; }
}
async function persist(target: AutomationDraft, successMessage: string) {
  if (busy.value || loading.value || !info.value) return;
  busy.value = true; error.value = ""; message.value = "";
  let applied = false;
  let latest = info.value;
  try {
    // Keep the requested values stable across both calls. Applying the first
    // response here would otherwise erase unsaved values from the other card.
    if (briefChanged(target, latest) && !target.briefEnabled) {
      latest = await Service.ConfigureAnalysisBrief(target.briefEnabled, target.briefAudience, target.briefSelf);
      applied = true;
    }
    if (acceptChanged(target, latest)) {
      latest = await Service.ConfigureAutoAccept(target.acceptEnabled, target.acceptDelay);
      applied = true;
    }
    if (postChanged(target, latest)) {
      latest = await Service.ConfigurePostGame(target.returnEnabled, target.queueEnabled, target.queueDelay, target.skipHonor);
      applied = true;
    }
    if (briefChanged(target, latest)) {
      latest = await Service.ConfigureAnalysisBrief(target.briefEnabled, target.briefAudience, target.briefSelf);
      applied = true;
    }
    apply(latest);
    message.value = successMessage;
    await live.refresh();
  } catch (reason) {
    // Reflect any first call that succeeded instead of restoring stale drafts.
    if (applied) apply(latest);
    error.value = `${applied ? uiText.common.partiallyApplied : ""}${errorMessage(reason)}`;
    await live.refresh();
  } finally { busy.value = false; }
}
async function save() {
  if (!dirty.value) return;
  await persist(currentDraft(), uiText.automationView.saved);
}
async function disableAll() {
  const target = currentDraft();
  target.briefEnabled = false;
  target.acceptEnabled = false;
  target.skipHonor = false;
  target.returnEnabled = false;
  target.queueEnabled = false;
  await persist(target, uiText.automationView.disabledAll);
}
async function previewBrief() {
  error.value = '';
  const revision = ++previewRevision;
  try {
    const text = await Service.PreviewAnalysisBriefWithOptions(briefSelf.value);
    if (revision === previewRevision) briefPreview.value = text || uiText.analysisBrief.emptyPreview;
  } catch (reason) { if (revision === previewRevision) error.value = errorMessage(reason); }
}
onMounted(() => { void load(); });
</script>
<template>
  <section class="automation-workspace" :aria-label="uiText.common.automation" :aria-busy="loading || busy">
    <header class="automation-head">
      <div>
        <h1>{{ uiText.common.automation }}</h1>
        <p class="muted">{{ uiText.automationView.introduction }}</p>
        <p class="automation-boundary">{{ uiText.automationView.boundaries }}</p>
      </div>
      <span class="automation-state" :class="{ busy: activeCount > 0, warn: warning }">
        <span class="dot" />{{ activeCount ? uiText.automationView.enabledCount(activeCount) : uiText.automationView.allDisabled }}
      </span>
    </header>

    <article class="automation-card" :class="{ on: acceptEnabled }">
      <header>
        <h2>{{ uiText.automationView.acceptHeading }}<ExperimentalBadge :feature="uiText.automationView.acceptTitle" /></h2>
        <label class="automation-switch"><input v-model="acceptEnabled" type="checkbox" :aria-label="uiText.automationView.acceptEnabled" :disabled="unavailable" />{{ uiText.common.enable }}</label>
      </header>
      <p class="muted">{{ uiText.automationView.acceptHelp }}</p>
      <label class="automation-field">{{ uiText.automationView.acceptDelayLabel }}<select v-model.number="acceptDelay" :aria-label="uiText.automationView.acceptDelay" :disabled="unavailable">
          <option v-for="seconds in [0, 1, 3, 5]" :key="seconds" :value="seconds">{{ seconds ? uiText.common.seconds(seconds) : uiText.automationView.immediately }}</option>
        </select>
      </label>
      <p v-if="acceptEnabled && acceptVisible" class="automation-live" data-live="accept">
        <span>{{ liveError || acceptText }}</span>
        <button v-if="accept?.status === 'countdown'" class="secondary" :disabled="busy" @click="skipAccept">{{ uiText.common.skipOnce }}</button>
      </p>
    </article>

    <article class="automation-card" :class="{ on: skipHonor || returnEnabled || queueEnabled }">
      <header>
        <h2>{{ uiText.automationView.postgameHeading }}<ExperimentalBadge :feature="uiText.automationView.postgameTitle" /></h2>
      </header>
      <div class="automation-rows">
        <label class="automation-switch"><input v-model="skipHonor" type="checkbox" :aria-label="uiText.automationView.skipHonor" :disabled="unavailable" />{{ uiText.automationView.skipHonor }}</label>
        <small class="muted">{{ uiText.automationView.skipHonorHelp }}</small>
        <label class="automation-switch"><input v-model="returnEnabled" type="checkbox" :aria-label="uiText.automationView.returnLobby" :disabled="unavailable" />{{ uiText.automationView.returnLobby }}</label>
        <label class="automation-switch"><input v-model="queueEnabled" type="checkbox" :aria-label="uiText.automationView.queueAfterReturn" :disabled="unavailable" />{{ uiText.automationView.queueAfterReturn }}</label>
        <label class="automation-field">{{ uiText.automationView.queueDelayLabel }}<select v-model.number="queueDelay" :aria-label="uiText.automationView.queueDelay" :disabled="unavailable">
            <option v-for="seconds in [5, 10, 15, 20]" :key="seconds" :value="seconds">{{ uiText.automationView.secondsText(seconds) }}</option>
          </select>
        </label>
        <small v-if="queueEnabled && !returnEnabled" class="muted">{{ uiText.automationView.manualReturnHelp }}</small>
      </div>
      <p v-if="(skipHonor || returnEnabled || queueEnabled) && postVisible" class="automation-live" data-live="post">
        <span>{{ liveError || postText }}</span>
        <button v-if="['honor-countdown','waiting-honor','return-countdown','queue-countdown','waiting-result','waiting-return','waiting-ready','waiting-postgame','continuing-postgame'].includes(post?.status ?? '')" class="secondary" :disabled="busy" @click="cancelPost">{{ uiText.automationView.cancelOnce }}</button>
      </p>
    </article>

    <article class="automation-card" :class="{on:briefEnabled}">
      <header><h2>{{ uiText.analysisBrief.title }}<ExperimentalBadge :feature="uiText.analysisBrief.title"/></h2><label class="automation-switch"><input v-model="briefEnabled" type="checkbox" :aria-label="uiText.analysisBrief.enabled" :disabled="unavailable"/>{{ uiText.common.enable }}</label></header>
      <p class="muted">{{ uiText.analysisBrief.help }}</p>
      <div class="automation-rows"><label class="automation-field">{{ uiText.analysisBrief.audience }}<select v-model="briefAudience" :aria-label="uiText.analysisBrief.audience" :disabled="unavailable"><option value="team">{{ uiText.analysisBrief.team }}</option><option value="self">{{ uiText.analysisBrief.self }}</option></select></label><label class="automation-switch"><input v-model="briefSelf" type="checkbox" :disabled="unavailable"/>{{ uiText.analysisBrief.includeSelf }}</label></div>
      <p v-if="briefEnabled || info?.autoAnalysisBrief" class="automation-live" data-live="brief">{{ briefStatus }}</p>
      <button class="secondary" :disabled="unavailable" @click="previewBrief">{{ uiText.analysisBrief.preview }}</button>
      <textarea v-if="briefPreview" class="brief-preview" :aria-label="uiText.analysisBrief.previewLabel" :value="briefPreview" readonly rows="6"/>
    </article>

    <footer class="automation-actions">
      <button class="secondary" type="button" @click="emit('settings')"><AppIcon name="settings" :size="14" />{{ uiText.automationView.otherSettings }}</button>
      <button class="secondary stop-all" type="button" :disabled="unavailable || !anyEnabled" @click="disableAll">{{ uiText.automationView.stopAll }}</button>
      <span class="muted">{{ loading ? text.loading : !info ? text.unavailable : dirty ? uiText.common.unsaved : uiText.common.synced }}</span>
      <button class="primary" type="button" :disabled="unavailable || !dirty" @click="save">{{ busy ? uiText.common.saving : uiText.automationView.save }}</button>
    </footer>
    <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
    <p v-else-if="message" class="settings-message" role="status">{{ message }}</p>
    <button v-if="!info && !loading" class="secondary" type="button" @click="load">{{ text.retry }}</button>
  </section>
</template>
<style scoped>
.brief-preview{display:block;width:100%;margin-top:10px;font-size:12px;resize:vertical;}
.automation-workspace { padding: 20px 24px 28px; overflow-y: auto; min-height: 0; }
.automation-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.automation-head h1 { font-size: 20px; margin: 0 0 6px; }
.automation-head p { margin: 0; }
.automation-boundary { max-width: 760px; margin-top: 7px !important; color: var(--muted); font-size: 12px; line-height: 1.6; }
.automation-state { display: inline-flex; align-items: center; gap: 7px; flex: none; font-size: 12px; color: var(--muted); border: 1px solid var(--border); border-radius: 999px; padding: 5px 11px; }
.automation-state .dot { width: 7px; height: 7px; border-radius: 50%; background: var(--muted); }
.automation-state.busy { color: var(--text); }
.automation-state.busy .dot { background: var(--accent); }
.automation-state.warn { color: var(--danger, #e06c6c); border-color: currentColor; }
.automation-state.warn .dot { background: currentColor; }
.automation-card { border: 1px solid var(--border); border-radius: 8px; padding: 16px 18px; margin-bottom: 14px; background: var(--raised); }
.automation-card.on { border-color: var(--accent); }
.automation-card header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.automation-card h2 { font-size: 15px; margin: 0; display: flex; align-items: center; gap: 8px; }
.automation-card > p { margin: 8px 0 12px; }
.automation-rows { display: flex; flex-direction: column; gap: 8px; margin-top: 10px; }
.automation-switch { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.automation-switch input { accent-color: var(--accent); }
.automation-field { display: inline-flex; align-items: center; gap: 8px; font-size: 13px; margin-top: 4px; }
.automation-field select { min-height: 30px; }
.automation-live { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin: 12px 0 0; padding: 8px 10px; border: 1px solid var(--border); border-radius: 6px; font-size: 12px; font-variant-numeric: tabular-nums; color: var(--text); }
.automation-actions { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-top: 18px; }
.automation-actions .muted { margin-left: auto; font-size: 12px; }
.automation-actions .stop-all:not(:disabled) { color: var(--danger, #e06c6c); border-color: currentColor; }
@media (max-width: 880px) { .automation-workspace { padding: 16px; } .automation-head { flex-direction: column; gap: 10px; } }
</style>

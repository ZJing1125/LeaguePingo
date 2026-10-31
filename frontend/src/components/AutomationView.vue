<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { AppInfo } from "../../bindings/leaguepingo/models";
import { errorMessage } from "../presentation";
import { useAutomation } from "../composables/useAutomation";
import AppIcon from "./AppIcon.vue";
import ExperimentalBadge from "./ExperimentalBadge.vue";

const emit = defineEmits<{ settings: [] }>();
const info = ref<AppInfo | null>(null);
const busy = ref(false);
const message = ref("");
const error = ref("");
const acceptEnabled = ref(false), acceptDelay = ref(1);
const skipHonor = ref(false), returnEnabled = ref(false), queueEnabled = ref(false), queueDelay = ref(10);
const live = useAutomation();
const { acceptText, postText, acceptVisible, postVisible, accept, post, skipAccept, cancelPost, error: liveError, activeCount, warning } = live;

function apply(next: AppInfo) {
  info.value = next;
  acceptEnabled.value = next.autoAcceptEnabled ?? false;
  acceptDelay.value = next.autoAcceptDelay ?? 1;
  skipHonor.value = next.autoSkipHonor ?? false;
  returnEnabled.value = next.autoReturnEnabled ?? false;
  queueEnabled.value = next.autoQueueEnabled ?? false;
  queueDelay.value = next.autoQueueDelay ?? 10;
}
const dirty = computed(() => !!info.value && (
  acceptEnabled.value !== (info.value.autoAcceptEnabled ?? false) ||
  acceptDelay.value !== (info.value.autoAcceptDelay ?? 1) ||
  skipHonor.value !== (info.value.autoSkipHonor ?? false) ||
  returnEnabled.value !== (info.value.autoReturnEnabled ?? false) ||
  queueEnabled.value !== (info.value.autoQueueEnabled ?? false) ||
  queueDelay.value !== (info.value.autoQueueDelay ?? 10)
));
async function load() {
  try { apply(await Service.Info()); }
  catch (reason) { error.value = errorMessage(reason); }
}
async function save() {
  if (busy.value || !info.value || !dirty.value) return;
  busy.value = true; error.value = ""; message.value = "";
  let applied = false;
  try {
    if (acceptEnabled.value !== (info.value.autoAcceptEnabled ?? false) || acceptDelay.value !== (info.value.autoAcceptDelay ?? 1)) {
      apply(await Service.ConfigureAutoAccept(acceptEnabled.value, acceptDelay.value));
      applied = true;
    }
    if (skipHonor.value !== (info.value.autoSkipHonor ?? false) || returnEnabled.value !== (info.value.autoReturnEnabled ?? false) || queueEnabled.value !== (info.value.autoQueueEnabled ?? false) || queueDelay.value !== (info.value.autoQueueDelay ?? 10)) {
      apply(await Service.ConfigurePostGame(returnEnabled.value, queueEnabled.value, queueDelay.value, skipHonor.value));
      applied = true;
    }
    message.value = "自动化设置已保存。";
    await live.refresh();
  } catch (reason) {
    error.value = `${applied ? "部分设置已生效；" : ""}${errorMessage(reason)}`;
  } finally { busy.value = false; }
}
onMounted(() => { void load(); });
</script>
<template>
  <section class="automation-workspace" aria-label="游戏辅助">
    <header class="automation-head">
      <div>
        <h1>游戏辅助</h1>
        <p class="muted">自动接受、赛后点赞与返回排队。开关默认关闭，保存成功后才会生效。</p>
      </div>
      <span class="automation-state" :class="{ busy: activeCount > 0, warn: warning }">
        <span class="dot" />{{ activeCount ? `${activeCount} 项开启` : '全部关闭' }}
      </span>
    </header>

    <article class="automation-card" :class="{ on: acceptEnabled }">
      <header>
        <h2>自动接受对局 <ExperimentalBadge feature="自动接受对局" /></h2>
        <label class="automation-switch"><input v-model="acceptEnabled" type="checkbox" aria-label="启用自动接受对局" />启用</label>
      </header>
      <p class="muted">按设定延迟接受对局；可跳过本次，不发送拒绝、不模拟点击。</p>
      <label class="automation-field">接受延迟
        <select v-model.number="acceptDelay" aria-label="自动接受延迟" :disabled="busy">
          <option v-for="seconds in [0, 1, 3, 5]" :key="seconds" :value="seconds">{{ seconds ? `${seconds} 秒` : '立即' }}</option>
        </select>
      </label>
      <p v-if="acceptEnabled && acceptVisible" class="automation-live" data-live="accept">
        <span>{{ liveError || acceptText }}</span>
        <button v-if="accept?.status === 'countdown'" class="secondary" :disabled="busy" @click="skipAccept">跳过本次</button>
      </p>
    </article>

    <article class="automation-card" :class="{ on: skipHonor || returnEnabled || queueEnabled }">
      <header>
        <h2>赛后流程 <ExperimentalBadge feature="赛后返回与自动排队" /></h2>
      </header>
      <div class="automation-rows">
        <label class="automation-switch"><input v-model="skipHonor" type="checkbox" aria-label="自动跳过点赞" />自动跳过点赞</label>
        <small class="muted">可单独开启，3 秒倒计时内可取消；不会替你选择点赞对象。</small>
        <label class="automation-switch"><input v-model="returnEnabled" type="checkbox" aria-label="赛后自动返回房间" />赛后自动返回房间</label>
        <label class="automation-switch"><input v-model="queueEnabled" type="checkbox" aria-label="返回后自动排队" />返回后自动排队</label>
        <label class="automation-field">排队倒计时
          <select v-model.number="queueDelay" aria-label="自动排队倒计时" :disabled="busy">
            <option v-for="seconds in [5, 10, 15, 20]" :key="seconds" :value="seconds">{{ seconds }} 秒</option>
          </select>
        </label>
        <small v-if="queueEnabled && !returnEnabled" class="muted">手动返回房间后，才会开始排队倒计时。</small>
      </div>
      <p v-if="(skipHonor || returnEnabled || queueEnabled) && postVisible" class="automation-live" data-live="post">
        <span>{{ liveError || postText }}</span>
        <button v-if="['honor-countdown','waiting-honor','return-countdown','queue-countdown','waiting-result','waiting-return','waiting-ready','waiting-postgame','continuing-postgame'].includes(post?.status ?? '')" class="secondary" :disabled="busy" @click="cancelPost">取消本次</button>
      </p>
    </article>

    <footer class="automation-actions">
      <button class="secondary" type="button" @click="emit('settings')"><AppIcon name="settings" :size="14" /> 其他设置</button>
      <span class="muted">{{ dirty ? '有未保存修改' : '设置已同步' }}</span>
      <button class="primary" type="button" :disabled="busy || !dirty" @click="save">{{ busy ? '保存中…' : '保存自动化设置' }}</button>
    </footer>
    <p v-if="error" class="error-banner" role="alert">{{ error }}</p>
    <p v-else-if="message" class="settings-message" role="status">{{ message }}</p>
  </section>
</template>
<style scoped>
.automation-workspace { padding: 20px 24px 28px; overflow-y: auto; min-height: 0; }
.automation-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 18px; }
.automation-head h1 { font-size: 20px; margin: 0 0 6px; }
.automation-head p { margin: 0; }
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
@media (max-width: 880px) { .automation-workspace { padding: 16px; } .automation-head { flex-direction: column; gap: 10px; } }
</style>

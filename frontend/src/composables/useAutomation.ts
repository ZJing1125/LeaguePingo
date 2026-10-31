// One subscription for the two automation channels. Both the title-bar chip and
// the two notice components read the same state; nothing renders a static
// "已开启" banner any more, only in-flight or actionable steps.
import { computed, onMounted, onUnmounted, ref } from "vue";
import { Events } from "@wailsio/runtime";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { State as ReadyState } from "../../bindings/leaguepingo/internal/readycheck/models";
import type { State as PostState } from "../../bindings/leaguepingo/internal/postgame/models";
import { errorMessage } from "../presentation";

// Statuses worth a visible line: something is counting down, working, or needs
// the player's attention. Terminal successes stay in the automation page only.
const acceptVisibleStatuses = ["countdown", "accepting", "failed", "unsupported"];
const postVisibleStatuses = [
  "honor-countdown", "waiting-honor", "skipping-honor",
  "return-countdown", "queue-countdown",
  "waiting-result", "waiting-return", "waiting-ready", "waiting-postgame",
  "continuing-postgame", "returning", "queueing",
  "honor-unavailable", "honor-unconfirmed", "honor-mismatch",
  "continue-unavailable", "continue-unconfirmed", "timeout", "failed",
];
const countingStatuses = ["honor-countdown", "return-countdown", "queue-countdown"];

export function useAutomation() {
  const accept = ref<ReadyState | null>(null);
  const post = ref<PostState | null>(null);
  const now = ref(Date.now());
  const busy = ref(false);
  const error = ref("");
  let stopAccept: (() => void) | undefined;
  let stopPost: (() => void) | undefined;
  let clock: ReturnType<typeof setInterval> | undefined;
  let alive = true;

  function syncClock() {
    if (clock) { clearInterval(clock); clock = undefined; }
    const counting = accept.value?.status === "countdown" || countingStatuses.includes(post.value?.status ?? "");
    if (counting) {
      now.value = Date.now();
      clock = setInterval(() => { now.value = Date.now(); }, 100);
    }
  }
  function applyAccept(next?: ReadyState | null) {
    if (!alive || !next || (accept.value && next.revision < accept.value.revision)) return;
    accept.value = next;
    error.value = "";
    syncClock();
  }
  function applyPost(next?: PostState | null) {
    if (!alive || !next || (post.value && next.revision < post.value.revision)) return;
    post.value = next;
    error.value = "";
    syncClock();
  }
  const secondsLeft = (due?: string) => Math.max(0, (Date.parse(due ?? "") - now.value) / 1000);

  const acceptEnabled = computed(() => !!accept.value?.enabled);
  const postEnabled = computed(() => !!(post.value?.returnEnabled || post.value?.queueEnabled || post.value?.skipHonor));
  const activeCount = computed(() => Number(acceptEnabled.value) + Number(postEnabled.value));
  const acceptVisible = computed(() => acceptEnabled.value && acceptVisibleStatuses.includes(accept.value!.status));
  const postVisible = computed(() => postEnabled.value && postVisibleStatuses.includes(post.value!.status));
  const warning = computed(() =>
    ["failed", "unsupported"].includes(accept.value?.status ?? "") ||
    ["failed", "timeout", "honor-unavailable", "honor-unconfirmed", "honor-mismatch", "continue-unavailable", "continue-unconfirmed"].includes(post.value?.status ?? ""),
  );

  const acceptText = computed(() => {
    const s = accept.value;
    if (!s) return "";
    switch (s.status) {
      case "countdown": return `${secondsLeft(s.dueAt).toFixed(1)} 秒后自动接受`;
      case "accepting": return "正在确认接受结果…";
      case "accepted": return "已接受对局";
      case "skipped": return "本次已跳过，请手动接受";
      case "manual": return "本次已手动响应";
      case "failed": return "未确认接受结果，请在客户端检查";
      case "unsupported": return "当前客户端暂不支持自动接受";
      default: return "自动接受已开启";
    }
  });
  const postText = computed(() => {
    const s = post.value;
    if (!s) return "";
    switch (s.status) {
      case "honor-countdown": return `${secondsLeft(s.dueAt).toFixed(0)} 秒后跳过点赞`;
      case "waiting-honor": return "等待点赞界面就绪或手动点赞完成";
      case "skipping-honor": return "正在跳过点赞并确认结果…";
      case "honor-skipped": return "已跳过点赞";
      case "honor-not-needed": return "本局无需跳过点赞或已手动完成";
      case "honor-unavailable": return "无法自动跳过点赞，请在客户端手动处理";
      case "honor-mismatch": return "点赞页与本局不匹配，本次自动流程已停止";
      case "honor-unconfirmed": return "未确认点赞页已结束，请在客户端检查";
      case "return-countdown": return `${secondsLeft(s.dueAt).toFixed(0)} 秒后返回房间`;
      case "queue-countdown": return `${secondsLeft(s.dueAt).toFixed(0)} 秒后开始排队`;
      case "waiting-result": return "等待赛后结算与复盘保留";
      case "waiting-return": return "等待返回房间后自动排队";
      case "waiting-ready": return "等待队友返回或房间就绪";
      case "waiting-postgame": return "等待赛后页面完成";
      case "continuing-postgame": return "正在完成赛后继续步骤…";
      case "continue-unavailable": return "暂时无法继续赛后页面，请在客户端完成本次流程";
      case "continue-unconfirmed": return "未确认赛后继续步骤已完成，请在客户端检查";
      case "returning": return "正在返回房间…";
      case "queueing": return "正在确认排队状态…";
      case "returned": return "已返回房间";
      case "queued": return "已开始排队";
      case "not-leader": return "已返回房间，等待队长开始";
      case "changed": return "房间或模式已变化，本次自动排队已停止";
      case "cancelled": return "本次赛后自动操作已取消";
      case "timeout": return "等待房间超时，请手动操作";
      case "failed": return "未确认赛后操作结果，请在客户端检查";
      default: return "赛后自动流程已开启";
    }
  });

  async function skipAccept() {
    busy.value = true;
    try { applyAccept(await Service.SkipAutoAccept()); }
    catch (reason) { error.value = errorMessage(reason); }
    finally { busy.value = false; }
  }
  async function cancelPost() {
    busy.value = true;
    try { applyPost(await Service.CancelPostGame()); }
    catch (reason) { error.value = errorMessage(reason); }
    finally { busy.value = false; }
  }
  async function refresh() {
    try { applyAccept(await Service.AutoAcceptStatus()); } catch { /* connection status lives in the title bar */ }
    try { applyPost(await Service.PostGameStatus()); } catch { /* same */ }
  }

  onMounted(() => {
    stopAccept = Events.On("leaguepingo:auto-accept", (event) => applyAccept(event.data as ReadyState));
    stopPost = Events.On("leaguepingo:post-game", (event) => applyPost(event.data as PostState));
    void refresh();
  });
  onUnmounted(() => {
    alive = false;
    stopAccept?.();
    stopPost?.();
    if (clock) clearInterval(clock);
  });

  return {
    accept, post, error, busy,
    acceptEnabled, postEnabled, activeCount, warning,
    acceptVisible, postVisible, acceptText, postText,
    skipAccept, cancelPost, refresh,
  };
}

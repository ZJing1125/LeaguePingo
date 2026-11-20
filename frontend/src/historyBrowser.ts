import { computed, nextTick, onBeforeUnmount, ref, watch, type Ref } from "vue";
import * as Service from "../bindings/leaguepingo/appservice";
import type { Participant } from "../bindings/leaguepingo/internal/core/models";
import { errorMessage, queueName, type PlayerViewState } from "./presentation";
import { recordVisibleResponse } from "./responseMetrics";
import { codedError, retryTime } from './errors';

export const HISTORY_PAGE_SIZE = 20;
export const MAX_HISTORY_PAGES = 50;
export const SAMPLE_SIZES = [10, 20, 30, 40, 50];
export const SAMPLE_SCAN_BUDGET = 90_000;

export function useHistoryBrowser(
  props: {
    player: Participant;
    state: PlayerViewState;
    busy?: boolean;
    connected?: boolean;
    active?: boolean;
    compact?: boolean;
  },
  scroll: Ref<HTMLElement | null>,
  reveal?: () => void,
) {
  let criteriaRevision = 0;
  let responseStarted = performance.now();
  let measuredResponse = -1;
  let pendingPage: ReturnType<typeof Service.HistoryPage> | undefined;
  watch(
    () => [
      props.state,
      props.player.key,
      props.player.connectionId,
      props.player.historyMeta?.fetchedAt,
    ],
    () => {
      const stamp = `${props.player.connectionId}:${props.player.key}:${props.player.historyMeta?.fetchedAt}`;
      const state = props.state;
      state.sampleSize ??= 20;
      state.page ??= 1;
      if (state.sourceStamp === stamp) return;
      const head = (props.player.history ?? []).map(m => m.gameId).join(",");
      if (state.sourceHead === head && state.sourceStamp?.startsWith(`${props.player.connectionId}:${props.player.key}:`)) {
        pendingPage?.cancel();
        state.sourceStamp = stamp;
        state.pageBusy = false;
        state.warmBusy = false;
        return;
      }
      Object.assign(state, {
        sourceHead: head,
        queuePages: {},
        queueFallbacks: [],
        mixedHasMore: (props.player.historyMeta?.returned ?? props.player.history?.length ?? 0) >= 20,
        sourceStamp: stamp,
        extra: [],
        pageMeta: [],
        page: 1,
        nextOffset:
          props.player.historyMeta?.returned ??
          props.player.history?.length ??
          0,
        hasMore:
          (props.player.historyMeta?.returned ??
            props.player.history?.length ??
            0) >= 20,
        pageBusy: false,
        warmBusy: false,
        pageError: "",
        browseNotice: "",
        sourceLimit: "",
        browseTarget: undefined,
        browseKind: "samples",
        stopRequested: false,
      });
      pendingPage?.cancel();
    },
    { immediate: true },
  );
  // A request already sent may finish, but changing players must not keep
  // scanning their history in the background or apply it to the new player.
  watch(
    () => props.state,
    (_, old) => {
      if (old) old.stopRequested = true;
    },
  );
  onBeforeUnmount(() => {
    criteriaRevision++;
    pendingPage?.cancel();
    props.state.stopRequested = true;
  });
  watch(
    () => [props.active, props.state, props.player.key, props.player.connectionId, props.state.queue],
    () => {
      pendingPage?.cancel();
      if (props.active === false) props.state.stopRequested = true;
    },
    { flush: "sync" },
  );

  const allMatchesFor = (player: Participant, state: PlayerViewState) => {
    const seen = new Set<number>();
    return [...(player.history ?? []), ...(state.extra ?? [])].filter(
      (match) => {
        if (seen.has(match.gameId)) return false;
        seen.add(match.gameId);
        return true;
      },
    );
  };
  const allMatches = computed(() => allMatchesFor(props.player, props.state));
  const matchesFor = (player: Participant, state: PlayerViewState, queue: string) => {
    if (queue === "all") return allMatchesFor(player, state);
    const seen = new Set<number>();
    const refreshed = new Map((player.history ?? []).map(m => [m.gameId,m]));
    return [...(state.queuePages?.[queue]?.matches ?? []), ...allMatchesFor(player, state)]
      .map(m => refreshed.get(m.gameId) ?? m)
      .filter(m => {
        if (String(m.queueId) !== queue || seen.has(m.gameId)) return false;
        seen.add(m.gameId);
        return true;
      });
  };
  const matches = computed(() => matchesFor(props.player, props.state, props.state.queue));
  const analysisMatches = computed(() =>
    matches.value.slice(0, props.state.sampleSize ?? 20),
  );
  const pageMatches = computed(() =>
    matches.value.slice(
      ((props.state.page ?? 1) - 1) * HISTORY_PAGE_SIZE,
      (props.state.page ?? 1) * HISTORY_PAGE_SIZE,
    ),
  );
  watch(pageMatches, rows => {
    if (!rows.length || measuredResponse === responseStarted) return;
    const started = responseStarted;
    measuredResponse = started;
    const stamp = props.state.sourceStamp, page = props.state.page;
    recordVisibleResponse(props.player.cached ? "history-cache-ready" : "history-first-ready", started,
      () => workspaceActive() && props.state.sourceStamp === stamp && props.state.page === page && responseStarted === started);
  }, { immediate: true, flush: "post" });
  watch(() => props.player.responseId, id => {
    if (!id || !pageMatches.value.length) return;
    recordVisibleResponse(null,performance.now(),() => workspaceActive() && props.player.responseId===id,id);
  }, { immediate:true, flush:"post" });
  const knownPages = computed(() =>
    Math.max(1, Math.ceil(matches.value.length / HISTORY_PAGE_SIZE)),
  );
  const canFetch = computed(
    () =>
      props.connected &&
      props.player.identityState === "known",
  );
  const maxJumpPage = computed(() =>
    props.state.hasMore && canFetch.value
      ? MAX_HISTORY_PAGES
      : knownPages.value,
  );
  const jumpDraft = ref(String(props.state.page ?? 1));
  const jumpError = ref("");
  watch(
    () => [
      props.state,
      props.state.page,
      props.state.queue,
      props.state.sourceStamp,
    ],
    () => {
      jumpDraft.value = String(props.state.page ?? 1);
      jumpError.value = "";
    },
  );
  watch(jumpDraft, () => {
    jumpError.value = "";
  });
  const sampleLabel = computed(
    () =>
      `${props.state.queue === "all" ? "全部模式" : queueName(Number(props.state.queue))} · 最近 ${analysisMatches.value.length} / ${props.state.sampleSize ?? 20} 场`,
  );
  const needsSamples = computed(
    () =>
      analysisMatches.value.length < (props.state.sampleSize ?? 20) &&
      props.state.hasMore,
  );
  const noticePlayer = computed(() => {
    const scoped = props.state.queuePages?.[props.state.queue];
    const metas = [props.player.historyMeta, ...(props.state.pageMeta ?? []), ...(scoped?.meta ?? [])].filter(Boolean);
    const duplicates =
      metas.reduce((n, m) => n + m.parsed, 0) - allMatches.value.length;
    return {
      ...props.player,
      historyMeta: {
        ...props.player.historyMeta,
        returned: metas.reduce((n, m) => n + (m.returned ?? 0), 0),
        parsed: allMatches.value.length,
        excluded:
          metas.reduce((n, m) => n + (m.excluded ?? 0), 0) +
          Math.max(0, duplicates),
        scope: `${sampleLabel.value}用于分析；列表每页 ${HISTORY_PAGE_SIZE} 场，翻页不改变分析范围。已核验 ${allMatches.value.length} 场混合模式记录${scoped?.direct ? `；当前模式直查 ${scoped.matches.length} 场` : ""}。`,
      },
    };
  });
  async function scrollTop() {
    await nextTick();
    if (reveal) {
      reveal();
      return;
    }
    scroll.value?.scrollTo({ top: 0 });
  }
  const workspaceActive = () => props.active !== false;

  async function fill(
    page = props.state.page ?? 1,
    kind = props.state.browseKind ?? "samples",
    queueOverride = props.state.queue,
    background = false,
  ) {
    if (
      !workspaceActive() ||
      props.busy ||
      props.state.pageBusy ||
      props.state.warmBusy
    )
      return;
    const target = props.player,
      state = props.state,
      stamp = state.sourceStamp;
    const queue = queueOverride;
    const required = kind === "samples" ? (state.sampleSize ?? 20) : page * HISTORY_PAGE_SIZE;
    const filtered = () => matchesFor(target, state, queue);
    const current = () => props.state === state && state.sourceStamp === stamp && props.player.key === target.key;
    if (!background) {
      state.browseTarget = page;
      state.browseKind = kind;
      state.pageError = "";
      state.browseNotice = "";
    }
    state.stopRequested = false;
    if (background) state.warmBusy = true;
    else state.pageBusy = true;
    const started = Date.now();
    let requests = 0;
    let retries=0;
    const budget=kind==='samples'?SAMPLE_SCAN_BUDGET:15000;
    const pageBudget=kind==='samples'?MAX_HISTORY_PAGES:5;
    async function fetchPage(offset:number) {
      for(;;){
        if(!current() || state.stopRequested || !workspaceActive() || props.busy)throw codedError('RequestCanceled');
        if(Date.now()-started>=budget)throw codedError('RequestTimeout');
        requests++;
        const scoped = state.queuePages?.[queue];
        const call=queue !== "all" && Service.HistoryPageForQueue
          ? Service.HistoryPageForQueue(target.key,target.connectionId,{queueId:Number(queue),offset:scoped?.nextOffset??0,mixedOffset:offset,remainingPages:pageBudget-requests+1,mixedOnly:state.queueFallbacks?.includes(queue)??false})
          : Service.HistoryPage(target.key,target.connectionId,offset);
        pendingPage = call;
        let expired=false;
        const timeout=setTimeout(()=>{expired=true;call.cancel();},Math.max(0,budget-(Date.now()-started)));
        try { return await call; }
        catch(reason){
          if(expired)throw codedError('RequestTimeout');
          const at=retryTime(reason),delay=Math.max(500,at-Date.now());
          if(background || !at || retries>=2 || requests>=pageBudget || Date.now()-started+delay>=budget)throw reason;
          retries++;
          state.browseNotice=`战绩服务暂不可用，稍后自动重试（${retries}/2）；已有记录保留。`;
          const until=Date.now()+delay;
          while(Date.now()<until){
            if(!current() || state.stopRequested || !workspaceActive() || props.busy || !props.connected)throw codedError('RequestCanceled');
            await new Promise(resolve=>setTimeout(resolve,Math.min(100,until-Date.now())));
          }
        }
        finally{clearTimeout(timeout);if(pendingPage===call)pendingPage=undefined;}
      }
    }
    let pageShown = state.page === page;
    async function showAvailablePage() {
      if (background || pageShown || queue !== props.state.queue || filtered().length <= (page - 1) * HISTORY_PAGE_SIZE) return;
      state.page = page;
      pageShown = true;
      await scrollTop();
    }
    try {
      if (!pageShown && filtered().length > (page - 1) * HISTORY_PAGE_SIZE) await showAvailablePage();
      while (
        current() &&
        !state.stopRequested &&
        filtered().length < required &&
        state.hasMore
      ) {
        if (!workspaceActive()) {
          state.stopRequested = true;
          break;
        }
        if (
          !props.connected ||
          target.identityState !== "known"
        ) {
          state.browseNotice =
            "当前仅能浏览已获取的记录，连接恢复后可继续查找。";
          break;
        }
        // A selected analysis size is one explicit operation, including all
        // source pages needed for that mode. Far page jumps keep a smaller budget.
        const sampleScan = kind === "samples";
        if (
          requests >= (sampleScan ? MAX_HISTORY_PAGES : 5) ||
          Date.now() - started >= (sampleScan ? SAMPLE_SCAN_BUDGET : 15000)
        ) {
          state.browseNotice = sampleScan
            ? "样本查找已达到本次 90 秒上限，已获取记录保留；可继续查找。"
            : `本轮查找已暂停，已获取记录保留；可继续查找${page > 1 ? `第 ${page} 页` : "所选模式"}。`;
          break;
        }
        const offset = state.nextOffset ?? 0;
        if (offset > 980 && (queue === "all" || state.queueFallbacks?.includes(queue))) {
          state.hasMore = false;
          break;
        }
        const result = await fetchPage(offset);
        if (!current() || queue !== props.state.queue) return;
        requests += Math.max(0, (result.requests ?? 1) - 1);
        const scopeQueue = result.queueId ?? 0;
        const expectedOffset = scopeQueue ? (state.queuePages?.[queue]?.nextOffset ?? 0) : offset;
        if (
          scopeQueue !== 0 && scopeQueue !== Number(queue) ||
          (scopeQueue !== 0 && (result.matches ?? []).some(m => m.queueId !== scopeQueue)) ||
          result.offset !== expectedOffset ||
          !Number.isSafeInteger(result.nextOffset) ||
          result.nextOffset < expectedOffset ||
          (result.hasMore && result.nextOffset <= expectedOffset)
        ) {
          if (scopeQueue) { state.queueFallbacks ??= []; state.queueFallbacks.push(queue); continue; }
          throw codedError('HistoryPageInvalid');
        }
        const existing = new Set(
          (scopeQueue ? state.queuePages?.[queue]?.matches ?? [] : allMatchesFor(target, state)).map((m) => m.gameId),
        );
        const repeated = result.matches?.length && (scopeQueue
          ? result.matches.some(m => existing.has(m.gameId))
          : result.matches.every(m => existing.has(m.gameId)));
        if (repeated) {
          if (scopeQueue) { state.queueFallbacks ??= []; state.queueFallbacks.push(queue); continue; }
          throw codedError('HistoryRepeated');
        }
        if (scopeQueue) {
          state.queuePages ??= {};
          const previous = state.queuePages[queue];
          state.queuePages[queue] = {
            matches: [...(previous?.matches ?? []), ...(result.matches ?? [])], nextOffset: result.nextOffset,
            hasMore: result.hasMore && result.nextOffset <= 980, direct: true, meta: [...(previous?.meta ?? []), result.meta],
          };
        } else {
          state.extra = [...(state.extra ?? []), ...(result.matches ?? [])];
          state.pageMeta = [...(state.pageMeta ?? []), result.meta];
          state.nextOffset = result.nextOffset;
          state.mixedHasMore = result.hasMore && result.nextOffset <= 980;
          if (state.queuePages?.[queue]) state.queuePages[queue].direct = false;
        }
        state.hasMore = scopeQueue ? state.queuePages![queue]!.hasMore : state.mixedHasMore;
        state.sourceLimit = result.limitReason || "";
        await showAvailablePage();
        if (result.responseId) recordVisibleResponse(null,performance.now(),
          () => current() && workspaceActive() && queue===props.state.queue,result.responseId);
        if (result.hasMore && result.nextOffset > 980 && !state.sourceLimit) {
          state.sourceLimit = scopeQueue
            ? "已达到当前模式最近 1000 条记录的查询上限，按已找到的模式样本分析。"
            : "已达到最近 1000 条混合记录的查询上限，按已找到的模式样本分析。";
        }
      }
      if (!current()) return;
      // Keep the existing page on failure/no new matches. A short last page is
      // valid; the known total is only final once the source is exhausted.
      const count = filtered().length;
      if (!background && (page === 1 || count > (page - 1) * HISTORY_PAGE_SIZE)) {
        state.page = page;
        if (props.state === state && !pageShown) await scrollTop();
      }
      if (!background && state.stopRequested)
        state.browseNotice = "已停止查找，已获取记录保留。";
      if (!background && state.sourceLimit) {
        state.browseNotice = state.sourceLimit;
      } else if (!background && !state.hasMore && count < (state.sampleSize ?? 20)) {
        state.browseNotice = `当前可查询范围内仅找到 ${count} 场该模式记录，按实际样本分析。`;
      } else if (!background && !state.hasMore && count <= (page - 1) * HISTORY_PAGE_SIZE) {
        state.browseNotice = `当前可查询范围内没有第 ${page} 页，已保留当前页。`;
      }
    } catch (reason) {
      if (current() && !background && workspaceActive()) {
        if(state.stopRequested)state.browseNotice='已停止查找，已获取记录保留。';
        else state.pageError = errorMessage(reason);
      }
    } finally {
      if (state.sourceStamp === stamp) {
        if (state.stopRequested && !state.browseNotice) state.browseNotice = "已停止查找，已获取记录保留。";
        if (background) state.warmBusy = false;
        else state.pageBusy = false;
      }
    }
  }
  function changeCriteria(kind: "page" | "samples" = "page") {
    responseStarted = performance.now();
    const revision = ++criteriaRevision;
    pendingPage?.cancel();
    props.state.stopRequested = true;
    const scoped = props.state.queuePages?.[props.state.queue];
    props.state.hasMore = scoped?.direct ? scoped.hasMore : props.state.mixedHasMore ||
      props.state.nextOffset === 1000 && props.state.queue !== "all" && !props.state.queueFallbacks?.includes(props.state.queue);
    props.state.page = 1;
    props.state.browseTarget = 1;
    void scrollTop();
    void (async () => {
      while (props.state.pageBusy || props.state.warmBusy) {
        await new Promise((resolve) => setTimeout(resolve, 25));
        if (revision !== criteriaRevision || !workspaceActive()) return;
      }
      if (revision !== criteriaRevision || !workspaceActive()) return;
      props.state.stopRequested = false;
      await fill(1, kind);
    })();
  }
  async function goPage(page: number) {
    responseStarted = performance.now();
    if (
      props.busy ||
      !Number.isSafeInteger(page) ||
      page < 1 ||
      page > maxJumpPage.value
    )
      return;
    jumpError.value = "";
    if (
      page <= knownPages.value
    ) {
      props.state.page = page;
      props.state.browseTarget = page;
      await scrollTop();
    } else if (props.state.hasMore && canFetch.value) {
      await fill(page, "page");
    }
  }
  async function jumpPage() {
    if (props.busy) return;
    const input = jumpDraft.value.trim();
    const page = Number(input);
    if (
      !/^\d+$/.test(input) ||
      !Number.isSafeInteger(page) ||
      page < 1 ||
      page > maxJumpPage.value
    ) {
      jumpError.value = `请输入 1–${maxJumpPage.value} 的整数页码`;
      return;
    }
    await goPage(page);
  }
  return {
    allMatches,
    matches,
    analysisMatches,
    pageMatches,
    knownPages,
    canFetch,
    maxJumpPage,
    jumpDraft,
    jumpError,
    jumpPage,
    sampleLabel,
    needsSamples,
    noticePlayer,
    changeCriteria,
    goPage,
    fill,
  };
}

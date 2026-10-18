import { computed, nextTick, onBeforeUnmount, ref, watch, type Ref } from "vue";
import * as Service from "../bindings/leaguepingo/appservice";
import type { Participant } from "../bindings/leaguepingo/internal/core/models";
import { errorMessage, queueName, type PlayerViewState } from "./presentation";

export const HISTORY_PAGE_SIZE = 20;
export const MAX_HISTORY_PAGES = 50;
export const SAMPLE_SIZES = [10, 20, 30, 40, 50];
export const SAMPLE_SCAN_BUDGET = 90_000;
// All explicit browsing uses the same manual lane as search/profile/refresh.
export const historyBrowseBusy = ref(false);

export function useHistoryBrowser(
  props: {
    player: Participant;
    state: PlayerViewState;
    busy?: boolean;
    connected?: boolean;
    active?: boolean;
  },
  scroll: Ref<HTMLElement | null>,
) {
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
      Object.assign(state, {
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
        pageError: "",
        browseNotice: "",
        sourceLimit: "",
        browseTarget: undefined,
        browseKind: "samples",
        stopRequested: false,
      });
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
    props.state.stopRequested = true;
  });
  watch(
    () => props.active,
    (active) => {
      if (active === false) props.state.stopRequested = true;
    },
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
  const matches = computed(() =>
    allMatches.value.filter(
      (m) =>
        props.state.queue === "all" || String(m.queueId) === props.state.queue,
    ),
  );
  const analysisMatches = computed(() =>
    matches.value.slice(0, props.state.sampleSize ?? 20),
  );
  const pageMatches = computed(() =>
    matches.value.slice(
      ((props.state.page ?? 1) - 1) * HISTORY_PAGE_SIZE,
      (props.state.page ?? 1) * HISTORY_PAGE_SIZE,
    ),
  );
  const knownPages = computed(() =>
    Math.max(1, Math.ceil(matches.value.length / HISTORY_PAGE_SIZE)),
  );
  const canFetch = computed(
    () =>
      props.connected &&
      !props.player.stale &&
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
    const metas = [
      props.player.historyMeta,
      ...(props.state.pageMeta ?? []),
    ].filter(Boolean);
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
        scope: `${sampleLabel.value}用于分析；列表每页 ${HISTORY_PAGE_SIZE} 场，翻页不改变分析范围。已核验 ${allMatches.value.length} 场混合模式记录。`,
      },
    };
  });
  async function scrollTop() {
    await nextTick();
    scroll.value?.scrollTo({ top: 0 });
  }
  const workspaceActive = () => props.active !== false;

  async function fill(
    page = props.state.page ?? 1,
    kind = props.state.browseKind ?? "samples",
  ) {
    if (
      !workspaceActive() ||
      props.busy ||
      historyBrowseBusy.value ||
      props.state.pageBusy
    )
      return;
    const target = props.player,
      state = props.state,
      stamp = state.sourceStamp;
    const queue = state.queue;
    const required = Math.max(state.sampleSize ?? 20, page * HISTORY_PAGE_SIZE);
    const filtered = () =>
      allMatchesFor(target, state).filter(
        (m) => queue === "all" || String(m.queueId) === queue,
      );
    const current = () => state.sourceStamp === stamp;
    state.browseTarget = page;
    state.browseKind = kind;
    state.pageError = "";
    state.browseNotice = "";
    state.stopRequested = false;
    state.pageBusy = true;
    historyBrowseBusy.value = true;
    const started = Date.now();
    let requests = 0;
    try {
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
          target.stale ||
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
        if (offset > 980) {
          state.hasMore = false;
          break;
        }
        const result = await Service.HistoryPage(
          target.key,
          target.connectionId,
          offset,
        );
        if (!current()) return;
        if (
          result.offset !== offset ||
          !Number.isSafeInteger(result.nextOffset) ||
          result.nextOffset < offset ||
          (result.hasMore && result.nextOffset <= offset)
        ) {
          throw new Error("战绩分页位置无效，已有记录保留，请刷新后重试。");
        }
        const existing = new Set(
          allMatchesFor(target, state).map((m) => m.gameId),
        );
        if (
          result.matches?.length &&
          result.matches.every((m) => existing.has(m.gameId))
        ) {
          throw new Error(
            "数据源重复返回已获取的战绩，已停止补取；未证明历史已查完，请稍后刷新重试。",
          );
        }
        requests++;
        state.extra = [...(state.extra ?? []), ...(result.matches ?? [])];
        state.pageMeta = [...(state.pageMeta ?? []), result.meta];
        state.nextOffset = result.nextOffset;
        state.hasMore = result.hasMore && result.nextOffset <= 980;
        state.sourceLimit = result.limitReason || "";
        if (result.hasMore && result.nextOffset > 980 && !state.sourceLimit) {
          state.sourceLimit =
            "已达到最近 1000 条混合记录的查询上限，按已找到的模式样本分析。";
        }
      }
      if (!current()) return;
      // Keep the existing page on failure/no new matches. A short last page is
      // valid; the known total is only final once the source is exhausted.
      const count = filtered().length;
      if (page === 1 || count > (page - 1) * HISTORY_PAGE_SIZE) {
        state.page = page;
        if (props.state === state) await scrollTop();
      }
      if (state.stopRequested)
        state.browseNotice = "已停止查找，已获取记录保留。";
      if (state.sourceLimit) {
        state.browseNotice = state.sourceLimit;
      } else if (!state.hasMore && count < (state.sampleSize ?? 20)) {
        state.browseNotice = `当前可查询范围内仅找到 ${count} 场该模式记录，按实际样本分析。`;
      } else if (!state.hasMore && count <= (page - 1) * HISTORY_PAGE_SIZE) {
        state.browseNotice = `当前可查询范围内没有第 ${page} 页，已保留当前页。`;
      }
    } catch (reason) {
      if (current()) state.pageError = errorMessage(reason);
    } finally {
      if (current()) state.pageBusy = false;
      historyBrowseBusy.value = false;
    }
  }
  async function changeCriteria() {
    props.state.page = 1;
    props.state.browseTarget = 1;
    await scrollTop();
    await fill(1, "samples");
  }
  async function goPage(page: number) {
    if (
      props.busy ||
      historyBrowseBusy.value ||
      !Number.isSafeInteger(page) ||
      page < 1 ||
      page > maxJumpPage.value
    )
      return;
    jumpError.value = "";
    if (
      page <= knownPages.value &&
      (page * HISTORY_PAGE_SIZE <= matches.value.length ||
        !props.state.hasMore ||
        !canFetch.value)
    ) {
      props.state.page = page;
      props.state.browseTarget = page;
      await scrollTop();
    } else if (props.state.hasMore && canFetch.value) {
      await fill(page, "page");
    }
  }
  async function jumpPage() {
    if (props.busy || historyBrowseBusy.value) return;
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

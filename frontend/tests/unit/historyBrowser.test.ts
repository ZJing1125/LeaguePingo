import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createRenderer, defineComponent, nextTick, reactive, ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { HistoryPage, Participant } from "../../bindings/leaguepingo/internal/core/models";
import type { PlayerViewState } from "../../src/presentation";
import { codedError } from "../../src/errors";
import { MAX_HISTORY_PAGES, SAMPLE_SCAN_BUDGET, useHistoryBrowser } from "../../src/historyBrowser";
import { matches, page, player } from "./fixtures";

vi.mock("../../bindings/leaguepingo/appservice", () => ({ HistoryPage: vi.fn() }));

// Mount the actual composable in Vue's lifecycle without a DOM/browser and
// without loading the Wails runtime. Only the bridge boundary is mocked.
const renderer = createRenderer<object, object>({
  createElement: () => ({}), createText: () => ({}), createComment: () => ({}),
  insert() {}, remove() {}, setText() {}, setElementText() {}, patchProp() {},
  parentNode: () => null, nextSibling: () => null,
});
const dispose: Array<() => void> = [];
function setup(records = matches(1), state: Partial<PlayerViewState> = {}) {
  const props = reactive({
    player: player(records), state: { queue: "all", expanded: [], ...state } as PlayerViewState,
    connected: true, active: true, compact: true, busy: false,
  });
  let browser!: ReturnType<typeof useHistoryBrowser>;
  const app = renderer.createApp(defineComponent({
    setup() { browser = useHistoryBrowser(props, ref(null)); return () => null; },
  }));
  app.mount({});
  dispose.push(() => app.unmount());
  return { props, browser };
}
function response(value: HistoryPage | Promise<HistoryPage>) {
  return Object.assign(Promise.resolve(value), { cancel: vi.fn() }) as unknown as ReturnType<typeof Service.HistoryPage>;
}
function deferred() {
  let resolve!: (value: HistoryPage) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<HistoryPage>((yes, no) => { resolve = yes; reject = no; });
  return { resolve, reject, call: response(promise) };
}

beforeEach(() => { vi.useFakeTimers(); vi.mocked(Service.HistoryPage).mockReset(); });
afterEach(() => { dispose.splice(0).forEach(stop => stop()); vi.clearAllTimers(); vi.useRealTimers(); });

describe("history browser request and state boundaries", () => {
  it("deduplicates cached matches, pages locally and preserves the analysis sample", async () => {
    const { props, browser } = setup(matches(1), { sampleSize: 10 });
    props.state.extra = matches(20, 21);
    props.state.hasMore = false;
    expect(browser.allMatches.value).toHaveLength(40);
    const sample = browser.analysisMatches.value.map(m => m.gameId);
    await browser.goPage(2);
    expect(browser.pageMatches.value.map(m => m.gameId)).toEqual(matches(21).map(m => m.gameId));
    expect(browser.analysisMatches.value.map(m => m.gameId)).toEqual(sample);
    expect(Service.HistoryPage).not.toHaveBeenCalled();
  });

  it("shows the first available target-page row before the rest of the page arrives", async () => {
    const { props, browser } = setup(matches(1), { queue: "420" });
    const pending = deferred();
    vi.mocked(Service.HistoryPage)
      .mockReturnValueOnce(response(page(20, [...matches(21, 1), ...matches(22, 19, 440)])))
      .mockReturnValueOnce(pending.call);
    const filling = browser.goPage(2);
    await vi.waitFor(() => expect(Service.HistoryPage).toHaveBeenCalledTimes(2));
    expect(props.state.page).toBe(2);
    expect(browser.pageMatches.value.map(m => m.gameId)).toEqual([21]);
    pending.resolve(page(40, matches(41, 19)));
    await filling;
    expect(browser.pageMatches.value).toHaveLength(20);
  });

  it("does not prefetch ranked queues when opening or returning to history", async () => {
    const { props } = setup(matches(1, 20, 2400));
    props.compact = false;
    await vi.advanceTimersByTimeAsync(2000);
    props.active = false;
    await nextTick();
    props.active = true;
    await nextTick();
    await vi.advanceTimersByTimeAsync(2000);
    expect(Service.HistoryPage).not.toHaveBeenCalled();
  });

  it("keeps prepared pages when a refresh confirms the same first-page matches", async () => {
    const { props, browser } = setup();
    props.state.extra = matches(21);
    props.state.page = 2;
    props.player = player(matches(1), { historyMeta: { ...props.player.historyMeta, fetchedAt: "2026-11-19T12:00:00Z" } });
    await nextTick();
    expect(props.state.page).toBe(2);
    expect(browser.allMatches.value).toHaveLength(40);
    expect(Service.HistoryPage).not.toHaveBeenCalled();
  });

  it("finds the selected mode across mixed pages rather than filtering only the initial twenty", async () => {
    const { props, browser } = setup(matches(1), { queue: "440" });
    vi.mocked(Service.HistoryPage)
      .mockReturnValueOnce(response(page(20, [...matches(21, 5, 440), ...matches(26, 15)])))
      .mockReturnValueOnce(response(page(40, [...matches(41, 15, 440), ...matches(56, 5)])));
    await browser.fill();
    expect(vi.mocked(Service.HistoryPage).mock.calls.map(call => call[2])).toEqual([20, 40]);
    expect(browser.analysisMatches.value).toHaveLength(20);
    expect(browser.analysisMatches.value.every(m => m.queueId === 440)).toBe(true);
    expect(props.state.nextOffset).toBe(60);
    expect(props.player.history).toHaveLength(20);
  });

  it.each(["cursor", "duplicate"])("rejects a %s page without advancing or discarding records", async (kind) => {
    const { props, browser } = setup();
    vi.mocked(Service.HistoryPage).mockReturnValue(response(kind === "cursor" ? { ...page(20), nextOffset: 20 } : page(20, matches(1))));
    await browser.fill(2, "page");
    expect(props.state.nextOffset).toBe(20);
    expect(props.state.page).toBe(1);
    expect(browser.allMatches.value).toHaveLength(20);
    expect(props.state.pageError).toContain(kind === "cursor" ? "战绩分页位置无效" : "数据源重复返回");
    expect(Service.HistoryPage).toHaveBeenCalledTimes(1);
  });

  it("stops a pending scan after the current page and retains the completed records", async () => {
    const { props, browser } = setup(matches(1), { sampleSize: 50 });
    const pending = deferred();
    vi.mocked(Service.HistoryPage).mockReturnValue(pending.call);
    const filling = browser.fill();
    props.state.stopRequested = true;
    pending.resolve(page(20));
    await filling;
    expect(Service.HistoryPage).toHaveBeenCalledTimes(1);
    expect(browser.allMatches.value).toHaveLength(40);
    expect(props.state.browseNotice).toContain("已停止查找");
    expect(props.state.pageBusy).toBe(false);
  });

  it("rejects a late page when the same player's source snapshot was replaced", async () => {
    const { props, browser } = setup(matches(1), { sampleSize: 50 });
    const pending = deferred();
    vi.mocked(Service.HistoryPage).mockReturnValue(pending.call);
    const filling = browser.fill();
    props.player = player(matches(101), { historyMeta: { ...props.player.historyMeta, fetchedAt: "2026-01-02T00:00:00Z" } });
    await nextTick();
    pending.resolve(page(20));
    await filling;
    expect(browser.allMatches.value.map(m => m.gameId)).toEqual(matches(101).map(m => m.gameId));
    expect(props.state.nextOffset).toBe(20);
    expect(props.state.pageBusy).toBe(false);
  });

  it("does not apply another player's pending page to the new player or continue scanning", async () => {
    const { props, browser } = setup(matches(1), { sampleSize: 50 });
    const oldState = props.state;
    const pending = deferred();
    vi.mocked(Service.HistoryPage).mockReturnValue(pending.call);
    const filling = browser.fill();
    props.player = player(matches(101), { key: "other-synthetic" }) as Participant;
    props.state = { queue: "all", expanded: [] };
    await nextTick();
    pending.resolve(page(20));
    await filling;
    expect(oldState.stopRequested).toBe(true);
    expect(oldState.pageBusy).toBe(false);
    expect(browser.allMatches.value.map(m => m.gameId)).toEqual(matches(101).map(m => m.gameId));
    expect(Service.HistoryPage).toHaveBeenCalledTimes(1);
  });

  it("caps a far page jump at five requests and keeps the requested target", async () => {
    const { props, browser } = setup();
    vi.mocked(Service.HistoryPage).mockImplementation((_key, _connection, offset) => response(page(offset)));
    await browser.goPage(10);
    expect(Service.HistoryPage).toHaveBeenCalledTimes(5);
    expect(props.state.page).toBe(1);
    expect(props.state.browseTarget).toBe(10);
    expect(props.state.browseNotice).toContain("继续查找第 10 页");
  });

  it("enforces the mixed-history thousand-record boundary", async () => {
    const { props, browser } = setup(matches(1), { queue: "440" });
    props.state.nextOffset = 980;
    vi.mocked(Service.HistoryPage).mockReturnValue(response(page(980)));
    await browser.fill();
    expect(Service.HistoryPage).toHaveBeenCalledExactlyOnceWith("synthetic-player", 1, 980);
    expect(props.state.hasMore).toBe(false);
    expect(props.state.sourceLimit).toContain("1000");
  });

  it("cancels a source call when the sample scan budget expires", async () => {
    const { props, browser } = setup(matches(1), { sampleSize: 50 });
    const pending = deferred();
    vi.mocked(pending.call.cancel).mockImplementation(() => {
      pending.reject(codedError("RequestCanceled"));
      return Promise.resolve() as unknown as ReturnType<typeof pending.call.cancel>;
    });
    vi.mocked(Service.HistoryPage).mockReturnValue(pending.call);
    const filling = browser.fill();
    await vi.advanceTimersByTimeAsync(SAMPLE_SCAN_BUDGET + 1);
    await filling;
    expect(pending.call.cancel).toHaveBeenCalledOnce();
    expect(Service.HistoryPage).toHaveBeenCalledOnce();
    expect(props.state.pageError).toContain("超时");
    expect(browser.allMatches.value).toHaveLength(20);
  });

  it("allows offline cached navigation while rejecting invalid jump input without requests", async () => {
    const { props, browser } = setup();
    props.state.extra = matches(21);
    props.connected = false;
    for (const input of ["", "0", "-1", "1.5", "abc", String(MAX_HISTORY_PAGES + 1)]) {
      browser.jumpDraft.value = input;
      await nextTick();
      await browser.jumpPage();
      expect(browser.jumpError.value).toContain("整数页码");
    }
    browser.jumpDraft.value = "2";
    await nextTick();
    await browser.jumpPage();
    expect(props.state.page).toBe(2);
    expect(Service.HistoryPage).not.toHaveBeenCalled();
  });
});

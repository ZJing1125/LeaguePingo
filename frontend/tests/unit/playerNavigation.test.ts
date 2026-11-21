import { afterEach, describe, expect, it, vi } from "vitest";
import { createRenderer, defineComponent, nextTick, ref } from "vue";
import * as Service from "../../bindings/leaguepingo/appservice";
import type { Participant, Snapshot } from "../../bindings/leaguepingo/internal/core/models";
import { usePlayerActions } from "../../src/composables/usePlayerActions";
import { usePlayerWorkspace } from "../../src/composables/usePlayerWorkspace";
import { player } from "./fixtures";

vi.mock("../../bindings/leaguepingo/appservice", () => ({
  Lookup: vi.fn(), LookupPlayer: vi.fn(), EnsureProfile: vi.fn(), PrioritizePlayer: vi.fn(), WaitForQueryIdle: vi.fn().mockResolvedValue(undefined),
}));
const renderer = createRenderer<object, object>({
  createElement: () => ({}), createText: () => ({}), createComment: () => ({}),
  insert() {}, remove() {}, setText() {}, setElementText() {}, patchProp() {},
  parentNode: () => null, nextSibling: () => null,
});
const cleanup: Array<() => void> = [];
function pending() {
  let resolve!: (p: Participant) => void;
  let reject!: (e: Error) => void;
  const promise = new Promise<Participant>((yes, no) => { resolve = yes; reject = no; });
  const cancel = vi.fn(() => { reject(new Error("canceled")); return Promise.resolve(); });
  return { resolve, reject, call: Object.assign(promise, { cancel }) as unknown as ReturnType<typeof Service.Lookup> };
}
function setup() {
  const snapshot = ref({ connected: true, connectionId: 1, participants: [] } as unknown as Snapshot);
  let actions!: ReturnType<typeof usePlayerActions>, workspace!: ReturnType<typeof usePlayerWorkspace>;
  const app = renderer.createApp(defineComponent({ setup() {
    workspace = usePlayerWorkspace(snapshot);
    actions = usePlayerActions(snapshot, workspace, () => {}, () => {});
    return () => null;
  }}));
  app.mount({});
  cleanup.push(() => app.unmount());
  return { actions, workspace, snapshot };
}
afterEach(() => { cleanup.splice(0).forEach(stop => stop()); vi.clearAllMocks(); });

describe("player history navigation", () => {
  it("opens the target immediately and shows history while profile is still pending", async () => {
    const history = pending(), profile = pending();
    vi.mocked(Service.Lookup).mockReturnValue(history.call);
    vi.mocked(Service.EnsureProfile).mockReturnValue(profile.call);
    const { actions, workspace } = setup();
    const opening = actions.openHistory({ kind: "riotId", riotId: "Alice#TEST", label: "Alice#TEST", connectionId: 1 });
    expect(workspace.view.value).toBe("players");
    expect(workspace.active.value?.displayName).toBe("Alice#TEST");
    expect(workspace.active.value?.historyState).toBe("loading");
    await nextTick();
    history.resolve(player(undefined, { key: "alice", displayName: "Alice#TEST" }));
    await opening;
    expect(workspace.active.value?.key).toBe("alice");
    expect(workspace.active.value?.history).toHaveLength(20);
    expect(Service.EnsureProfile).toHaveBeenCalledOnce();
    profile.resolve(player(undefined, { key: "alice" }));
    await nextTick();
  });

  it("keeps the latest player intent after canceling the previous lookup", async () => {
    const first = pending(), second = pending();
    vi.mocked(Service.Lookup).mockReturnValueOnce(first.call).mockReturnValueOnce(second.call);
    vi.mocked(Service.EnsureProfile).mockReturnValue(Object.assign(Promise.resolve(player()), { cancel: vi.fn() }) as unknown as ReturnType<typeof Service.EnsureProfile>);
    const { actions, workspace } = setup();
    const openingA = actions.openHistory({ kind: "riotId", riotId: "Alice#TEST", label: "Alice#TEST", connectionId: 1 });
    await vi.waitFor(() => expect(Service.Lookup).toHaveBeenCalledOnce());
    const openingB = actions.openHistory({ kind: "riotId", riotId: "Bob#TEST", label: "Bob#TEST", connectionId: 1 });
    expect(workspace.active.value?.displayName).toBe("Bob#TEST");
    await openingA;
    await nextTick();
    expect(first.call.cancel).toHaveBeenCalledOnce();
    second.resolve(player(undefined, { key: "bob", displayName: "Bob#TEST" }));
    await openingB;
    expect(workspace.active.value?.key).toBe("bob");
    expect(actions.searching.value).toBe(false);
  });

  it("does not reopen a closed pending player view when the response arrives", async () => {
    const history = pending();
    vi.mocked(Service.Lookup).mockReturnValue(history.call);
    const { actions, workspace } = setup();
    const opening = actions.openHistory({ kind: "riotId", riotId: "Alice#TEST", label: "Alice#TEST", connectionId: 1 });
    await nextTick();
    workspace.closeTab(workspace.activeKey.value);
    history.resolve(player(undefined, { key: "alice" }));
    await opening;
    expect(workspace.tabs.value).toHaveLength(0);
    expect(Service.EnsureProfile).not.toHaveBeenCalled();
  });
});

it("repairs missing profiles once while repeatedly reactivating a player", async () => {
  const update = pending();
  vi.mocked(Service.EnsureProfile).mockReturnValue(update.call);
  const { workspace } = setup();
  const p = player([], { key: "missing", puuid: "missing", connectionId: 1, displayName: "Missing#TEST", profile: undefined });
  workspace.openPlayer(p);
  await nextTick();
  update.resolve(p);
  await nextTick(); await nextTick();
  workspace.activeKey.value = "";
  await nextTick();
  workspace.activeKey.value = p.key;
  await nextTick();
  expect(Service.EnsureProfile).toHaveBeenCalledTimes(1);
});

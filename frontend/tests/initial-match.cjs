// Entirely synthetic. Go owns the bounded startup batch; UI reacts to snapshots.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const { mockWindowRuntime } = require("./window-runtime.cjs");

async function main() {
  const browser = await chromium.launch({
    headless: true,
    executablePath: process.env.BROWSER_EXECUTABLE,
  });
  try {
    const page = await browser.newPage({
      viewport: { width: 1100, height: 760 },
    });
    page.setDefaultTimeout(10000);
    const errors = [],
      calls = [],
      external = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const desktop = mockWindowRuntime(page);
    const binding = await fs.readFile(
      path.join(__dirname, "../bindings/leaguepingo/appservice.ts"),
      "utf8",
    );
    const methods = Object.fromEntries(
      [
        ...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g),
      ].map((m) => [m[2], m[1]]),
    );
    let snapshot = {
      connectionId: 1,
      generation: 1,
      gameId: 42,
      connected: true,
      stage: "in-game",
      phase: "InProgress",
      frozen: true,
      knownCount: 10,
      readyCount: 0,
      updatedAt: "2026-10-18T10:00:00Z",
      message: "正在补齐本局开局资料；每局仅一轮，最多 10 位玩家、45 秒。",
      participants: Array.from({ length: 10 }, (_, i) => ({
        connectionId: 1,
        key: `fixture-${i}`,
        puuid: `fixture-${i}`,
        displayName: "玩家名称暂不可用",
        team: i < 5 ? "ally" : "enemy",
        championId: 23 + i,
        championState: "selected",
        identityState: "known",
        privacyState: "unknown",
        historyState: "loading",
        history: [],
        historyMeta: {},
        stats: { games: 0 },
      })),
    };
    await page.route("**/*", (route) => {
      external.push(route.request().url());
      return route.abort();
    });
    await page.route("http://127.0.0.1:43198/**", async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname === "/wails/custom.js")
        return route.fulfill({ contentType: "text/javascript", body: "" });
      if (url.pathname === "/wails/runtime") {
        const request = route.request().postDataJSON();
        if (await desktop.handle(route, request)) return;
        const method = methods[request.args?.methodID];
        const args = request.args?.args ?? [];
        calls.push(method);
        let result = null;
        if (method === "Snapshot") result = snapshot;
        else if (method === "OverlayStatus")
          result = { visible: false, pinned: true, revision: 0 };
        else if (method === "EnsureProfile") {
          const player = snapshot.participants.find((p) => p.key === args[0]);
          result = {
            ...player,
            displayName:
              args[0] === "fixture-0"
                ? "手动补齐姓名#TEST"
                : player.displayName,
            profile: {
              updatedAt: "2026-10-18T10:00:01Z",
              level: 100,
              privacyState: "unknown",
              rankState: "unavailable",
              ranks: [],
            },
          };
        } else if (method !== "PrioritizePlayer")
          throw new Error(`Unexpected service request: ${method}`);
        return route.fulfill({ json: result });
      }
      const relative =
        url.pathname === "/" ? "index.html" : url.pathname.slice(1);
      if (relative.includes("..")) return route.abort();
      return route.fulfill({
        body: await fs.readFile(path.join(__dirname, "../dist", relative)),
        contentType: relative.endsWith(".js")
          ? "text/javascript"
          : relative.endsWith(".css")
            ? "text/css"
            : relative.endsWith(".png")
              ? "image/png"
              : "text/html",
      });
    });
    await page.goto("http://127.0.0.1:43198/");
    const navigation = page.getByRole("navigation", { name: "工作区" });
    await navigation.getByRole("button", { name: "当前对局" }).click();
    await page.getByText("正在获取", { exact: true }).first().waitFor();
    assert.equal(await page.getByText("正在获取", { exact: true }).count(), 10);
    assert(
      (await page.locator(".roster-toolbar").textContent()).includes(
        "开局补齐",
      ),
    );
    assert.equal(
      await page.getByText("玩家名称暂不可用", { exact: true }).count(),
      0,
    );
    if (process.env.UI_SCREENSHOT_PREFIX)
      await page.screenshot({
        path: process.env.UI_SCREENSHOT_PREFIX + "-initial-loading.png",
      });
    for (let i = 0; i < 8; i++) {
      await page.locator(".player-row").nth(i).click();
      if (i === 0)
        await page
          .locator(".player-drawer h1")
          .filter({ hasText: "手动补齐姓名#TEST" })
          .waitFor();
      await page.getByRole("button", { name: "在战绩页打开 ↗" }).click();
      if (i === 1)
        assert(
          await page
            .getByRole("button", { name: "复制玩家完整 ID" })
            .isDisabled(),
          "missing name must not be copied as an ID",
        );
      await navigation.getByRole("button", { name: "当前对局" }).click();
    }
    async function publish() {
      snapshot = { ...snapshot, generation: snapshot.generation + 1 };
      await page.evaluate(
        (s) =>
          window._wails.dispatchWailsEvent({
            name: "leaguepingo:snapshot",
            data: s,
          }),
        snapshot,
      );
    }
    snapshot.participants = snapshot.participants.map((p, i) => ({
      ...p,
      displayName: i === 0 ? "手动补齐姓名#TEST" : `自动玩家${i}#TEST`,
      historyState: "empty",
      historyMeta: { fetchedAt: "2026-10-18T10:01:00Z" },
    }));
    snapshot.readyCount = 10;
    snapshot.message =
      "本局开局资料补齐已结束；未完成的玩家可手动查询，本局不再自动重试。";
    await publish();
    await navigation.getByRole("button", { name: "玩家战绩" }).click();
    assert.equal(await page.getByRole("tab").count(), 8);
    for (let i = 1; i < 8; i++)
      assert.equal(
        await page
          .getByRole("tab", { name: `自动玩家${i}#TEST`, exact: true })
          .count(),
        1,
      );
    const visibleTab = page.getByRole("tab", { selected: true });
    assert(
      await visibleTab.evaluate((el) => {
        const b = el.getBoundingClientRect(),
          p = el.closest(".player-tabs").getBoundingClientRect();
        return (
          b.top >= p.top &&
          b.bottom <= p.bottom &&
          b.height >= 24 &&
          b.left >= p.left &&
          b.right <= p.right
        );
      }),
      "horizontal scrollbar must not clip tab captions",
    );
    snapshot.participants[7] = {
      ...snapshot.participants[7],
      displayName: "新确认姓名#TEST",
      historyMeta: { fetchedAt: "2026-10-18T09:59:00Z" },
    };
    await publish();
    await page
      .getByRole("heading", { name: "新确认姓名#TEST", exact: true })
      .waitFor();
    assert.equal(
      await page
        .getByRole("tab", { name: "新确认姓名#TEST", exact: true })
        .count(),
      1,
      "name must update independently of retained history",
    );
    if (process.env.UI_SCREENSHOT_PREFIX)
      await page.screenshot({
        path: process.env.UI_SCREENSHOT_PREFIX + "-name-tabs.png",
      });
    snapshot.participants[9] = {
      ...snapshot.participants[9],
      historyState: "pending",
      error: "首次补齐已结束，可手动查询",
    };
    await publish();
    await navigation.getByRole("button", { name: "当前对局" }).click();
    await page.getByText("未获取 · 可手动查询", { exact: true }).waitFor();
    assert.equal(await page.getByText("正在获取", { exact: true }).count(), 0);
    assert(
      !calls.includes("LookupPlayer") && !calls.includes("RefreshHistory"),
      "UI must not start a parallel batch of player queries",
    );
    assert.deepEqual(errors, []);
    assert.deepEqual(external, []);
    console.log(
      "PASS: initial-match loading/completion/timeout UI, name repair across profile/roster/tabs, independent name freshness, copy guard and tab overflow (synthetic runtime).",
    );
  } finally {
    await browser.close();
  }
}
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

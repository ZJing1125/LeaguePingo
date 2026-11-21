// Synthetic desktop runtime only; no OS window or game client is controlled.
const fs = require("node:fs");
const path = require("node:path");
const runtimeRoot = path.dirname(require.resolve("@wailsio/runtime"));
const source = fs.readFileSync(path.join(runtimeRoot, "window.js"), "utf8");
const methods = Object.fromEntries(
  [...source.matchAll(/const (\w+)Method = (\d+);/g)].map((m) => [m[2], m[1]]),
);

const binding = fs.readFileSync(path.join(__dirname, "../bindings/leaguepingo/appservice.ts"), "utf8");
const responseMethod = binding.match(/export function RecordUIResponse[\s\S]*?\$Call\.ByID\((\d+)/)?.[1];

const ackMethod = binding.match(/export function AcknowledgeResponse[\s\S]*?\$Call\.ByID\((\d+)/)?.[1];

function mockWindowRuntime(page) {
  return {
    calls: [],
    maximised: false,
    focused: true,
    failNext: "",
    async emit(name, sender = "main") {
      await page.evaluate(
        ({ name, sender }) => {
          window._wails.dispatchWailsEvent({ name: `common:${name}`, sender });
        },
        { name, sender },
      );
    },
    async handle(route, request) {
      if (request.object === 10) {
        await route.fulfill({ json: null });
        return true;
      }
      if (String(request.args?.methodID) === responseMethod) {
        const [category, elapsed] = request.args.args;
        if (!["room-render", "sidebar-render", "player-history-ready", "history-feedback", "history-cache-ready", "history-first-ready", "rating-render", "guide-render"].includes(category) || !Number.isFinite(elapsed) || elapsed < 0) {
          throw new Error("Invalid anonymous response measurement");
        }
        await route.fulfill({ json: null });
        return true;
      }
      if (String(request.args?.methodID) === ackMethod) {
        const [id,elapsed] = request.args.args;
        if (!Number.isSafeInteger(id) || id<=0 || !Number.isFinite(elapsed) || elapsed<0) throw new Error("Invalid response acknowledgement");
        await route.fulfill({json:null}); return true;
      }
      // These browser fixtures model an unverified source. At the mixed
      // limit Engine returns a bounded page without another source request.
      if (String(request.args?.methodID) === queueMethod && request.args.args[2].mixedOffset === 1000) {
        await route.fulfill({json:{queueId:0,offset:1000,nextOffset:1000,hasMore:false,requests:1,matches:[],meta:{returned:0,parsed:0,excluded:0},limitReason:"已达到最近 1000 条混合记录的查询上限，按已找到的模式样本分析。"}});
        return true;
      }
      if (request.object !== 6) return false; // Wails beta.29 Window object.
      const method = methods[request.method];
      this.calls.push(method);
      if (this.failNext === method) {
        this.failNext = "";
        await route.fulfill({
          status: 500,
          json: { message: "synthetic window failure" },
        });
        return true;
      }
      let result = null;
      if (method === "IsMaximised") result = this.maximised;
      else if (method === "IsFocused") result = this.focused;
      else if (method === "ToggleMaximise") {
        this.maximised = !this.maximised;
        await this.emit(this.maximised ? "WindowMaximise" : "WindowUnMaximise");
      } else if (!["Minimise", "Close"].includes(method)) {
        throw new Error(`Unexpected window method: ${method}`);
      }
      await route.fulfill({ json: result });
      return true;
    },
  };
}

module.exports = { mockWindowRuntime };

const queueMethod = binding.match(/export function HistoryPageForQueue[\s\S]*?\$Call\.ByID\((\d+)/)?.[1];
const historyMethod = binding.match(/export function HistoryPage\([\s\S]*?\$Call\.ByID\((\d+)/)?.[1];
function normalizeServiceRequest(request) {
  if (String(request.args?.methodID) === queueMethod) {
    const [key,connection,scope] = request.args.args;
    if (scope.mixedOffset === 1000) return request;
    return {...request,args:{...request.args,methodID:Number(historyMethod),args:[key,connection,scope.mixedOffset]}};
  }
  return request;
}
module.exports.normalizeServiceRequest = normalizeServiceRequest;

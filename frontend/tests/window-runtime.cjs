// Synthetic desktop runtime only; no OS window or game client is controlled.
const fs = require("node:fs");
const path = require("node:path");
const runtimeRoot = path.dirname(require.resolve("@wailsio/runtime"));
const source = fs.readFileSync(path.join(runtimeRoot, "window.js"), "utf8");
const methods = Object.fromEntries(
  [...source.matchAll(/const (\w+)Method = (\d+);/g)].map((m) => [m[2], m[1]]),
);

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

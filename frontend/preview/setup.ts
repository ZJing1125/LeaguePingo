import productStyles from "../src/style.css?inline";
import { defineSetupVue3 } from "@histoire/plugin-vue";

export const setupVue3 = defineSetupVue3(() => {
  // The product's global tokens and element styles belong to the story iframe,
  // not Histoire's light/dark navigation shell or server-side collection.
  if (
    typeof document === "undefined" ||
    !location.pathname.endsWith("/__sandbox.html")
  )
    return;
  if (!document.getElementById("leaguepingo-preview-style")) {
    const style = document.createElement("style");
    style.id = "leaguepingo-preview-style";
    style.textContent = productStyles;
    document.head.append(style);
  }
});

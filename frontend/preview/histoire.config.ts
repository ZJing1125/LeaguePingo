import { defineConfig } from "histoire";
import { HstVue } from "@histoire/plugin-vue";
import vue from "@vitejs/plugin-vue";
import { fileURLToPath } from "node:url";

// Histoire's published beta uses Vite 7. Keep it out of the Vite 8 product lock.
export default defineConfig({
  plugins: [HstVue()],
  theme: { title: "LeaguePingo 组件预览" },
  setupFile: "./setup.ts",
  storyMatch: ["./stories/**/*.story.vue"],
  outDir: "../../artifacts/component-preview",
  vite: {
    plugins: [
      {
        name: "leaguepingo-offline-preview-assets",
        enforce: "pre",
        transform(code, id) {
          if (
            id
              .replaceAll("\\", "/")
              .includes("/@histoire/vendors/dist/client/b-iconify.js")
          ) {
            const preload = fileURLToPath(
              new URL("./icons.ts", import.meta.url),
            ).replaceAll("\\", "/");
            return `import ${JSON.stringify(preload)};\n${code}`;
          }
          if (
            id.replaceAll("\\", "/").includes("/@histoire/app/dist/style.css")
          )
            return code.replace(
              /@import url\(['"]https:\/\/fonts\.googleapis\.com[^;]+;/g,
              "",
            );
        },
      },
      vue(),
    ],
    resolve: {
      // Product components and the preview renderer must share one Vue runtime.
      alias: [
        {
          find: /^vue$/,
          replacement: fileURLToPath(
            new URL(
              "../node_modules/vue/dist/vue.runtime.esm-bundler.js",
              import.meta.url,
            ),
          ),
        },
      ],
      dedupe: ["vue"],
    },
    // beta.1's pre-bundled app no longer imports these old optimizer hints.
    optimizeDeps: {
      exclude: ["flexsearch", "vscode-oniguruma", "vscode-textmate"],
    },
    server: {
      host: "127.0.0.1",
      fs: { allow: [fileURLToPath(new URL("..", import.meta.url))] },
    },
  },
});

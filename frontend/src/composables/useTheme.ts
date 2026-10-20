import { computed, onMounted, onUnmounted, ref } from "vue";

export type ThemePreference = "light" | "dark" | "system";
const key = "leaguepingo.theme";
const defaultTheme: ThemePreference = "dark";
function valid(value: unknown): value is ThemePreference {
  return value === "light" || value === "dark" || value === "system";
}
function saved(): ThemePreference {
  try {
    const value = localStorage.getItem(key);
    return valid(value) ? value : defaultTheme;
  } catch {
    return defaultTheme;
  }
}
const preference = ref<ThemePreference>(saved());
const systemDark = ref(matchMedia("(prefers-color-scheme: dark)").matches);
const resolved = computed(() => preference.value === "system" ? (systemDark.value ? "dark" : "light") : preference.value);
const saveError = ref("");
function apply() {
  document.documentElement.dataset.theme = resolved.value;
  document.documentElement.style.colorScheme = resolved.value;
}

export function useTheme() {
  function setTheme(value: ThemePreference) {
    preference.value = value;
    saveError.value = "";
    try { localStorage.setItem(key, value); }
    catch { saveError.value = "主题已应用，本次无法保存选择。"; }
    apply();
  }
  const media = matchMedia("(prefers-color-scheme: dark)");
  function onSystem(event: MediaQueryListEvent) {
    systemDark.value = event.matches;
    apply();
  }
  function onStorage(event: StorageEvent) {
    if (event.key !== key && event.key !== null) return;
    preference.value = valid(event.newValue) ? event.newValue : defaultTheme;
    saveError.value = "";
    apply();
  }
  apply();
  onMounted(() => {
    media.addEventListener("change", onSystem);
    window.addEventListener("storage", onStorage);
  });
  onUnmounted(() => {
    media.removeEventListener("change", onSystem);
    window.removeEventListener("storage", onStorage);
  });
  return { preference, resolved, saveError, setTheme };
}

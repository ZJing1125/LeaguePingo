import { onScopeDispose, reactive } from "vue";

// Timers belong to the operation key, not the currently visible player tab.
export function useTransientMessages() {
  const messages = reactive<Record<string, string>>({});
  const timers = new Map<string, ReturnType<typeof setTimeout>>();
  function setMessage(key: string, message: string) {
    clearTimeout(timers.get(key));
    timers.delete(key);
    messages[key] = message;
    if (!message) return;
    const timer = setTimeout(() => {
      if (timers.get(key) !== timer) return;
      delete messages[key];
      timers.delete(key);
    }, 2000);
    timers.set(key, timer);
  }
  onScopeDispose(() => {
    for (const timer of timers.values()) clearTimeout(timer);
    timers.clear();
  });
  return { messages, setMessage };
}
